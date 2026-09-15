package storage

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	activeSuffix          = ".jsonl.active"
	completedSuffix       = ".jsonl"
	compressedSuffix      = ".gz"
	compressionTempSuffix = ".gz.active"
	recoveredSuffix       = ".jsonl.recovered"

	CompressionGzip = "gzip"
	CompressionNone = "none"
)

type completedSegment struct {
	modTime time.Time
	path    string
	size    uint64
}

// RingOptions controls rolling observation storage.
type RingOptions struct {
	Compression        string
	Directory          string
	MaxBytes           uint64
	MinFreeBytes       uint64
	OnCompressionError func(error)
	Prefix             string
	SegmentBytes       uint64
}

// Stats is a point-in-time snapshot of rolling-storage health.
type Stats struct {
	ActiveBytes          uint64 `json:"active_bytes"`
	CompletedBytes       uint64 `json:"completed_bytes"`
	CompletedSegments    int    `json:"completed_segments"`
	CompressedSegments   int    `json:"compressed_segments"`
	CompressionActive    int    `json:"compression_active"`
	CompressionBacklog   int    `json:"compression_backlog"`
	CompressionCompleted uint64 `json:"compression_completed"`
	CompressionFailures  uint64 `json:"compression_failures"`
	CompressionMode      string `json:"compression_mode"`
	RawCompletedSegments int    `json:"raw_completed_segments"`
	RetentionDeletes     uint64 `json:"retention_deletes"`
}

// Ring stores whole JSONL observation records in immutable rolling segments.
type Ring struct {
	activeBytes        atomic.Uint64
	completed          []completedSegment
	completedBytes     uint64
	compressionDone    atomic.Uint64
	compressionErrors  atomic.Uint64
	compressQueue      chan string
	compressStop       chan struct{}
	compressWG         sync.WaitGroup
	compressing        map[string]struct{}
	compression        string
	compressorStarted  bool
	currentBytes       uint64
	directory          string
	file               *os.File
	lockFile           *os.File
	maxBytes           uint64
	minFreeBytes       uint64
	mu                 sync.Mutex
	onCompressionError func(error)
	prefix             string
	retentionDeletes   atomic.Uint64
	segmentBytes       uint64
}

// NewRing creates a rolling JSONL observation store.
func NewRing(options RingOptions) (*Ring, error) {
	if options.Directory == "" {
		return nil, fmt.Errorf("storage directory is required")
	}
	if options.SegmentBytes == 0 {
		return nil, fmt.Errorf("segment size must be greater than zero")
	}
	if options.MaxBytes < options.SegmentBytes {
		return nil, fmt.Errorf(
			"max size %d must be at least one segment (%d)",
			options.MaxBytes,
			options.SegmentBytes,
		)
	}

	compression := options.Compression
	if compression == "" {
		compression = CompressionNone
	}

	prefix := options.Prefix
	if prefix == "" {
		prefix = "observations"
	}
	if strings.ContainsAny(prefix, `/\\`) || strings.TrimSpace(prefix) != prefix || prefix == "" {
		return nil, fmt.Errorf("invalid storage prefix %q", prefix)
	}
	if compression != CompressionNone && compression != CompressionGzip {
		return nil, fmt.Errorf("unsupported compression %q", compression)
	}

	if err := os.MkdirAll(options.Directory, 0o750); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	ring := &Ring{
		compressing:        make(map[string]struct{}),
		compression:        compression,
		directory:          options.Directory,
		maxBytes:           options.MaxBytes,
		minFreeBytes:       options.MinFreeBytes,
		onCompressionError: options.OnCompressionError,
		prefix:             prefix,
		segmentBytes:       options.SegmentBytes,
	}

	if err := ring.acquireDirectoryLock(); err != nil {
		return nil, err
	}

	ok := false
	defer func() {
		if !ok {
			ring.releaseDirectoryLock()
		}
	}()

	if err := ring.recoverCompressionTemps(); err != nil {
		return nil, err
	}
	if err := ring.recoverActiveSegments(); err != nil {
		return nil, err
	}
	if err := ring.resolveCompressionDuplicates(); err != nil {
		return nil, err
	}
	if err := ring.scanCompleted(); err != nil {
		return nil, err
	}
	if err := ring.enforceRetention(0); err != nil {
		return nil, err
	}
	if err := ring.openSegment(); err != nil {
		return nil, err
	}

	ring.startCompressor()
	ring.queueExistingRawSegments()

	ok = true
	return ring, nil
}

// Close finalizes the active segment and stops background compression.
// The just-finalized current segment intentionally remains raw; it can be
// compressed on the next start. This keeps shutdown bounded and preserves the
// most recent observations in directly readable JSONL.
func (ring *Ring) Close() error {
	if ring.file != nil {
		if ring.currentBytes == 0 {
			path := ring.file.Name()
			closeErr := ring.file.Close()
			ring.file = nil
			ring.currentBytes = 0
			ring.activeBytes.Store(0)

			removeErr := os.Remove(path)
			if closeErr != nil {
				ring.stopCompressor()
				ring.releaseDirectoryLock()
				return fmt.Errorf("close empty active segment: %w", closeErr)
			}
			if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				ring.stopCompressor()
				ring.releaseDirectoryLock()
				return fmt.Errorf("remove empty active segment: %w", removeErr)
			}
		} else {
			if _, err := ring.finalizeCurrent(); err != nil {
				ring.stopCompressor()
				ring.releaseDirectoryLock()
				return err
			}
		}
	}

	ring.stopCompressor()

	retentionErr := ring.enforceRetention(0)
	ring.releaseDirectoryLock()

	return retentionErr
}

// Write writes exactly one complete JSONL observation record.
func (ring *Ring) Write(record []byte) (int, error) {
	if ring.file == nil {
		return 0, fmt.Errorf("storage ring is closed")
	}
	if len(record) == 0 {
		return 0, nil
	}
	if uint64(len(record)) > ring.segmentBytes {
		return 0, fmt.Errorf(
			"observation size %d exceeds segment size %d",
			len(record),
			ring.segmentBytes,
		)
	}

	if ring.currentBytes > 0 &&
		ring.currentBytes+uint64(len(record)) > ring.segmentBytes {
		if err := ring.rotate(); err != nil {
			return 0, err
		}
	}

	written := 0
	for written < len(record) {
		n, err := ring.file.Write(record[written:])
		if err != nil {
			return written, fmt.Errorf("write observation segment: %w", err)
		}
		if n == 0 {
			return written, ioErrShortWrite()
		}
		written += n
	}

	ring.currentBytes += uint64(written)
	ring.activeBytes.Store(ring.currentBytes)

	ring.mu.Lock()
	totalBytes := ring.completedBytes + ring.currentBytes
	ring.mu.Unlock()

	if totalBytes > ring.maxBytes {
		if err := ring.enforceRetention(ring.currentBytes); err != nil {
			return written, err
		}
	}

	return written, nil
}

// Stats returns a concurrency-safe storage-health snapshot.
func (ring *Ring) Stats() Stats {
	if ring == nil {
		return Stats{}
	}

	ring.mu.Lock()
	completedBytes := ring.completedBytes
	completedSegments := len(ring.completed)
	compressionActive := len(ring.compressing)
	compressedSegments := 0
	rawSegments := 0
	for _, segment := range ring.completed {
		if isCompressedCompletedName(filepath.Base(segment.path)) {
			compressedSegments++
		} else if isRawCompletedName(filepath.Base(segment.path)) {
			rawSegments++
		}
	}
	ring.mu.Unlock()

	backlog := 0
	if ring.compressQueue != nil {
		backlog = len(ring.compressQueue)
	}

	return Stats{
		ActiveBytes:          ring.activeBytes.Load(),
		CompletedBytes:       completedBytes,
		CompletedSegments:    completedSegments,
		CompressedSegments:   compressedSegments,
		CompressionActive:    compressionActive,
		CompressionBacklog:   backlog,
		CompressionCompleted: ring.compressionDone.Load(),
		CompressionFailures:  ring.compressionErrors.Load(),
		CompressionMode:      ring.compression,
		RawCompletedSegments: rawSegments,
		RetentionDeletes:     ring.retentionDeletes.Load(),
	}
}

func (ring *Ring) acquireDirectoryLock() error {
	path := filepath.Join(ring.directory, ".pathfinder-sensor.lock")

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return fmt.Errorf("open storage lock: %w", err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return fmt.Errorf("storage directory %s is already in use: %w", ring.directory, err)
	}

	ring.lockFile = file
	return nil
}

func (ring *Ring) compressSegment(path string) {
	if !isRawCompletedPath(path) {
		return
	}

	ring.mu.Lock()
	if _, active := ring.compressing[path]; active {
		ring.mu.Unlock()
		return
	}

	found := false
	for _, segment := range ring.completed {
		if segment.path == path {
			found = true
			break
		}
	}
	if !found {
		ring.mu.Unlock()
		return
	}

	ring.compressing[path] = struct{}{}
	ring.mu.Unlock()

	defer func() {
		ring.mu.Lock()
		delete(ring.compressing, path)
		ring.mu.Unlock()
	}()

	sourceInfo, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			ring.reportCompressionError(fmt.Errorf("stat segment %s: %w", path, err))
		}
		return
	}

	source, err := os.Open(path)
	if err != nil {
		ring.reportCompressionError(fmt.Errorf("open segment %s: %w", path, err))
		return
	}
	defer source.Close()

	finalPath := path + compressedSuffix
	tempPath := path + compressionTempSuffix

	_ = os.Remove(tempPath)

	destination, err := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		ring.reportCompressionError(fmt.Errorf("create compressed segment %s: %w", tempPath, err))
		return
	}

	success := false
	defer func() {
		if !success {
			destination.Close()
			_ = os.Remove(tempPath)
		}
	}()

	writer, err := gzip.NewWriterLevel(destination, gzip.BestSpeed)
	if err != nil {
		ring.reportCompressionError(fmt.Errorf("create gzip writer for %s: %w", path, err))
		return
	}
	writer.Name = filepath.Base(path)
	writer.ModTime = sourceInfo.ModTime()

	buffer := make([]byte, 256*1024)
	if _, err := io.CopyBuffer(writer, source, buffer); err != nil {
		writer.Close()
		ring.reportCompressionError(fmt.Errorf("compress segment %s: %w", path, err))
		return
	}
	if err := writer.Close(); err != nil {
		ring.reportCompressionError(fmt.Errorf("finish gzip segment %s: %w", path, err))
		return
	}
	if err := destination.Sync(); err != nil {
		ring.reportCompressionError(fmt.Errorf("sync compressed segment %s: %w", path, err))
		return
	}
	if err := destination.Close(); err != nil {
		ring.reportCompressionError(fmt.Errorf("close compressed segment %s: %w", path, err))
		return
	}

	if err := os.Rename(tempPath, finalPath); err != nil {
		ring.reportCompressionError(fmt.Errorf("finalize compressed segment %s: %w", path, err))
		return
	}
	if err := os.Chtimes(finalPath, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		_ = os.Remove(finalPath)
		ring.reportCompressionError(fmt.Errorf("preserve compressed segment time %s: %w", path, err))
		return
	}

	compressedInfo, err := os.Stat(finalPath)
	if err != nil {
		_ = os.Remove(finalPath)
		ring.reportCompressionError(fmt.Errorf("stat compressed segment %s: %w", finalPath, err))
		return
	}

	if err := os.Remove(path); err != nil {
		_ = os.Remove(finalPath)
		ring.reportCompressionError(fmt.Errorf("remove raw segment %s: %w", path, err))
		return
	}

	ring.mu.Lock()
	for index := range ring.completed {
		if ring.completed[index].path != path {
			continue
		}

		ring.completedBytes -= ring.completed[index].size
		ring.completed[index].path = finalPath
		ring.completed[index].size = uint64(compressedInfo.Size())
		ring.completed[index].modTime = sourceInfo.ModTime()
		ring.completedBytes += ring.completed[index].size
		break
	}
	sortCompleted(ring.completed)
	ring.mu.Unlock()

	ring.compressionDone.Add(1)
	success = true
}

func (ring *Ring) enforceRetention(activeBytes uint64) error {
	for {
		freeBytes, err := filesystemFreeBytes(ring.directory)
		if err != nil {
			return fmt.Errorf("read filesystem free space: %w", err)
		}

		ring.mu.Lock()

		overMax := ring.completedBytes+activeBytes > ring.maxBytes
		belowFree := ring.minFreeBytes > 0 && freeBytes < ring.minFreeBytes

		if !overMax && !belowFree {
			ring.mu.Unlock()
			return nil
		}

		removeIndex := -1
		for index, segment := range ring.completed {
			if _, active := ring.compressing[segment.path]; active {
				continue
			}
			removeIndex = index
			break
		}

		if removeIndex < 0 {
			completedCount := len(ring.completed)
			ring.mu.Unlock()

			if completedCount == 0 {
				if belowFree {
					return fmt.Errorf(
						"filesystem free space %d is below minimum %d and no completed segment can be removed",
						freeBytes,
						ring.minFreeBytes,
					)
				}
				return fmt.Errorf(
					"rolling storage exceeds maximum %d and no completed segment can be removed",
					ring.maxBytes,
				)
			}

			return fmt.Errorf("retention cannot remove a segment while all completed segments are being compressed")
		}

		oldest := ring.completed[removeIndex]
		if err := os.Remove(oldest.path); err != nil {
			ring.mu.Unlock()
			return fmt.Errorf("remove oldest segment %s: %w", oldest.path, err)
		}

		ring.completed = append(
			ring.completed[:removeIndex],
			ring.completed[removeIndex+1:]...,
		)
		ring.completedBytes -= oldest.size
		ring.mu.Unlock()
		ring.retentionDeletes.Add(1)
	}
}

func (ring *Ring) finalizeCurrent() (string, error) {
	if ring.file == nil {
		return "", nil
	}

	activePath := ring.file.Name()

	if err := ring.file.Sync(); err != nil {
		return "", fmt.Errorf("sync active segment: %w", err)
	}
	if err := ring.file.Close(); err != nil {
		return "", fmt.Errorf("close active segment: %w", err)
	}
	ring.file = nil

	completedPath := strings.TrimSuffix(activePath, ".active")
	if err := os.Rename(activePath, completedPath); err != nil {
		return "", fmt.Errorf("finalize active segment: %w", err)
	}

	info, err := os.Stat(completedPath)
	if err != nil {
		return "", fmt.Errorf("stat completed segment: %w", err)
	}

	size := uint64(info.Size())

	ring.mu.Lock()
	ring.completed = append(ring.completed, completedSegment{
		modTime: info.ModTime(),
		path:    completedPath,
		size:    size,
	})
	ring.completedBytes += size
	sortCompleted(ring.completed)
	ring.mu.Unlock()

	ring.currentBytes = 0
	ring.activeBytes.Store(0)
	return completedPath, nil
}

func (ring *Ring) openSegment() error {
	name := fmt.Sprintf(
		"%s-%s%s",
		ring.prefix,
		time.Now().UTC().Format("20060102T150405.000000000Z"),
		activeSuffix,
	)
	path := filepath.Join(ring.directory, name)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("create active observation segment: %w", err)
	}

	ring.file = file
	ring.currentBytes = 0
	ring.activeBytes.Store(0)

	return nil
}

func (ring *Ring) queueCompression(path string) {
	if ring.compression != CompressionGzip || path == "" {
		return
	}

	select {
	case <-ring.compressStop:
		return
	case ring.compressQueue <- path:
	default:
		ring.reportCompressionError(
			fmt.Errorf("compression queue full; leaving segment uncompressed: %s", path),
		)
	}
}

func (ring *Ring) queueExistingRawSegments() {
	if ring.compression != CompressionGzip {
		return
	}

	ring.mu.Lock()
	paths := make([]string, 0, len(ring.completed))
	for _, segment := range ring.completed {
		if isRawCompletedPath(segment.path) {
			paths = append(paths, segment.path)
		}
	}
	ring.mu.Unlock()

	for _, path := range paths {
		ring.queueCompression(path)
	}
}

func (ring *Ring) recoverActiveSegments() error {
	entries, err := os.ReadDir(ring.directory)
	if err != nil {
		return fmt.Errorf("list storage directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), activeSuffix) {
			continue
		}

		oldPath := filepath.Join(ring.directory, entry.Name())
		completeBytes, err := truncateToLastCompleteLine(oldPath)
		if err != nil {
			return fmt.Errorf("recover active segment %s: %w", oldPath, err)
		}

		if completeBytes == 0 {
			if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove empty recovered segment %s: %w", oldPath, err)
			}
			continue
		}

		newPath := strings.TrimSuffix(oldPath, activeSuffix) + recoveredSuffix
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("recover active segment %s: %w", oldPath, err)
		}
	}

	return nil
}

func (ring *Ring) recoverCompressionTemps() error {
	entries, err := os.ReadDir(ring.directory)
	if err != nil {
		return fmt.Errorf("list compression temporary files: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), compressionTempSuffix) {
			continue
		}

		path := filepath.Join(ring.directory, entry.Name())
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale compression temporary file %s: %w", path, err)
		}
	}

	return nil
}

func (ring *Ring) releaseDirectoryLock() {
	if ring.lockFile == nil {
		return
	}

	_ = syscall.Flock(int(ring.lockFile.Fd()), syscall.LOCK_UN)
	_ = ring.lockFile.Close()
	ring.lockFile = nil
}

func (ring *Ring) reportCompressionError(err error) {
	if err == nil {
		return
	}

	ring.compressionErrors.Add(1)
	if ring.onCompressionError != nil {
		ring.onCompressionError(err)
	}
}

func (ring *Ring) resolveCompressionDuplicates() error {
	entries, err := os.ReadDir(ring.directory)
	if err != nil {
		return fmt.Errorf("list compression duplicates: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !isCompressedCompletedName(entry.Name()) {
			continue
		}

		compressedPath := filepath.Join(ring.directory, entry.Name())
		rawPath := strings.TrimSuffix(compressedPath, compressedSuffix)

		if _, err := os.Stat(rawPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("stat possible raw duplicate %s: %w", rawPath, err)
		}

		if err := verifyGzip(compressedPath); err != nil {
			if removeErr := os.Remove(compressedPath); removeErr != nil {
				return fmt.Errorf(
					"remove invalid compressed duplicate %s after verification error %v: %w",
					compressedPath,
					err,
					removeErr,
				)
			}
			continue
		}

		if err := os.Remove(rawPath); err != nil {
			return fmt.Errorf("remove raw compression duplicate %s: %w", rawPath, err)
		}
	}

	return nil
}

func (ring *Ring) rotate() error {
	completedPath, err := ring.finalizeCurrent()
	if err != nil {
		return err
	}

	if err := ring.enforceRetention(0); err != nil {
		return err
	}
	if err := ring.openSegment(); err != nil {
		return err
	}

	ring.queueCompression(completedPath)
	return nil
}

func (ring *Ring) scanCompleted() error {
	entries, err := os.ReadDir(ring.directory)
	if err != nil {
		return fmt.Errorf("list completed segments: %w", err)
	}

	ring.completed = nil
	ring.completedBytes = 0

	for _, entry := range entries {
		if entry.IsDir() || !isCompletedName(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat segment %s: %w", entry.Name(), err)
		}

		size := uint64(info.Size())
		ring.completed = append(ring.completed, completedSegment{
			modTime: info.ModTime(),
			path:    filepath.Join(ring.directory, entry.Name()),
			size:    size,
		})
		ring.completedBytes += size
	}

	sortCompleted(ring.completed)
	return nil
}

func (ring *Ring) startCompressor() {
	if ring.compression != CompressionGzip {
		return
	}

	ring.compressQueue = make(chan string, 256)
	ring.compressStop = make(chan struct{})
	ring.compressorStarted = true

	ring.compressWG.Add(1)
	go func() {
		defer ring.compressWG.Done()

		for {
			select {
			case <-ring.compressStop:
				return
			case path := <-ring.compressQueue:
				ring.compressSegment(path)
			}
		}
	}()
}

func (ring *Ring) stopCompressor() {
	if !ring.compressorStarted {
		return
	}

	close(ring.compressStop)
	ring.compressWG.Wait()
	ring.compressorStarted = false
}

func filesystemFreeBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}

	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}

func ioErrShortWrite() error {
	return &fs.PathError{
		Op:   "write",
		Path: "observation segment",
		Err:  errors.New("short write"),
	}
}

func isCompletedName(name string) bool {
	return isRawCompletedName(name) || isCompressedCompletedName(name)
}

func isCompressedCompletedName(name string) bool {
	return strings.HasSuffix(name, completedSuffix+compressedSuffix) ||
		strings.HasSuffix(name, recoveredSuffix+compressedSuffix)
}

func isRawCompletedName(name string) bool {
	return strings.HasSuffix(name, completedSuffix) ||
		strings.HasSuffix(name, recoveredSuffix)
}

func isRawCompletedPath(path string) bool {
	return isRawCompletedName(filepath.Base(path))
}

func sortCompleted(segments []completedSegment) {
	sort.Slice(segments, func(i, j int) bool {
		if segments[i].modTime.Equal(segments[j].modTime) {
			return segments[i].path < segments[j].path
		}
		return segments[i].modTime.Before(segments[j].modTime)
	})
}

func truncateToLastCompleteLine(path string) (int64, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	size := info.Size()
	if size == 0 {
		return 0, nil
	}

	const chunkSize int64 = 64 * 1024
	buffer := make([]byte, chunkSize)

	for end := size; end > 0; {
		start := end - chunkSize
		if start < 0 {
			start = 0
		}

		length := end - start
		if _, err := file.ReadAt(buffer[:length], start); err != nil && !errors.Is(err, io.EOF) {
			return 0, err
		}

		for index := length - 1; index >= 0; index-- {
			if buffer[index] != '\n' {
				continue
			}

			completeBytes := start + index + 1
			if completeBytes < size {
				if err := file.Truncate(completeBytes); err != nil {
					return 0, err
				}
				if err := file.Sync(); err != nil {
					return 0, err
				}
			}

			return completeBytes, nil
		}

		end = start
	}

	if err := file.Truncate(0); err != nil {
		return 0, err
	}
	if err := file.Sync(); err != nil {
		return 0, err
	}

	return 0, nil
}

func verifyGzip(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(io.Discard, reader)
	closeErr := reader.Close()

	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
