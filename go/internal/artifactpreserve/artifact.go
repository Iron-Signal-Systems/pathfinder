package artifactpreserve

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	DefaultSocketPath      = "/var/run/pathfinder-artifact/preserve.sock"
	defaultSocketMode      = 0660
	digestAlgorithm        = "sha256"
	protocolProbeHeader    = "PATHFINDER/1 PROBE\n"
	protocolPreserveHeader = "PATHFINDER/1 PRESERVE\n"
	requestHeaderBuffer    = 128
)

type Receipt struct {
	ByteLength       int64  `json:"byte_length"`
	Reused           bool   `json:"reused"`
	SHA256           string `json:"sha256"`
	StorageReference string `json:"storage_reference"`
}

type response struct {
	ByteLength       int64  `json:"byte_length,omitempty"`
	Error            string `json:"error,omitempty"`
	OK               bool   `json:"ok"`
	Reused           bool   `json:"reused,omitempty"`
	SHA256           string `json:"sha256,omitempty"`
	StorageReference string `json:"storage_reference,omitempty"`
}

type Server struct {
	MaxBytes   int64
	ObjectsDir string
	SocketPath string
}

func Probe(
	ctx context.Context,
	socketPath string,
) error {
	connection, err := dialUnix(
		ctx,
		socketPath,
	)
	if err != nil {
		return err
	}
	defer connection.Close()

	if _, err := io.WriteString(
		connection,
		protocolProbeHeader,
	); err != nil {
		return fmt.Errorf(
			"send artifact preservation probe: %w",
			err,
		)
	}

	if err := connection.CloseWrite(); err != nil {
		return fmt.Errorf(
			"finish artifact preservation probe: %w",
			err,
		)
	}

	result, err := readResponse(connection)
	if err != nil {
		return err
	}

	if !result.OK {
		if result.Error == "" {
			result.Error = "preservation service rejected probe"
		}

		return errors.New(result.Error)
	}

	return nil
}

func Preserve(
	ctx context.Context,
	socketPath string,
	source io.Reader,
) (Receipt, error) {
	connection, err := dialUnix(
		ctx,
		socketPath,
	)
	if err != nil {
		return Receipt{}, err
	}
	defer connection.Close()

	if _, err := io.WriteString(
		connection,
		protocolPreserveHeader,
	); err != nil {
		return Receipt{}, fmt.Errorf(
			"send artifact preservation request header: %w",
			err,
		)
	}

	if _, err := io.Copy(connection, source); err != nil {
		return Receipt{}, fmt.Errorf(
			"stream artifact bytes: %w",
			err,
		)
	}

	if err := connection.CloseWrite(); err != nil {
		return Receipt{}, fmt.Errorf(
			"finish artifact byte stream: %w",
			err,
		)
	}

	result, err := readResponse(connection)
	if err != nil {
		return Receipt{}, err
	}

	if !result.OK {
		if result.Error == "" {
			result.Error = "preservation service rejected artifact"
		}

		return Receipt{}, errors.New(result.Error)
	}

	receipt := Receipt{
		ByteLength:       result.ByteLength,
		Reused:           result.Reused,
		SHA256:           result.SHA256,
		StorageReference: result.StorageReference,
	}

	if err := validateReceipt(receipt); err != nil {
		return Receipt{}, fmt.Errorf(
			"invalid artifact preservation receipt: %w",
			err,
		)
	}

	return receipt, nil
}

func dialUnix(
	ctx context.Context,
	socketPath string,
) (*net.UnixConn, error) {
	if !filepath.IsAbs(socketPath) {
		return nil, fmt.Errorf(
			"artifact socket path must be absolute",
		)
	}

	dialer := net.Dialer{}

	rawConnection, err := dialer.DialContext(
		ctx,
		"unix",
		socketPath,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"connect to artifact preservation service: %w",
			err,
		)
	}

	connection, ok := rawConnection.(*net.UnixConn)
	if !ok {
		_ = rawConnection.Close()

		return nil, fmt.Errorf(
			"artifact preservation connection is not a Unix socket",
		)
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			_ = connection.Close()

			return nil, fmt.Errorf(
				"set artifact preservation connection deadline: %w",
				err,
			)
		}
	}

	return connection, nil
}

func readResponse(
	connection *net.UnixConn,
) (response, error) {
	var result response

	decoder := json.NewDecoder(
		bufio.NewReader(connection),
	)

	if err := decoder.Decode(&result); err != nil {
		return response{}, fmt.Errorf(
			"read artifact preservation response: %w",
			err,
		)
	}

	return result, nil
}

func (server Server) Serve(ctx context.Context) error {
	if err := server.validate(); err != nil {
		return err
	}

	if err := cleanupIncoming(server.ObjectsDir); err != nil {
		return err
	}

	if err := prepareSocket(server.SocketPath); err != nil {
		return err
	}

	address := &net.UnixAddr{
		Name: server.SocketPath,
		Net:  "unix",
	}

	listener, err := net.ListenUnix("unix", address)
	if err != nil {
		return fmt.Errorf(
			"listen on artifact socket: %w",
			err,
		)
	}

	if err := os.Chmod(
		server.SocketPath,
		defaultSocketMode,
	); err != nil {
		_ = listener.Close()
		_ = os.Remove(server.SocketPath)

		return fmt.Errorf(
			"set artifact socket mode: %w",
			err,
		)
	}

	defer func() {
		_ = listener.Close()
		_ = os.Remove(server.SocketPath)
	}()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		connection, err := listener.AcceptUnix()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return fmt.Errorf(
				"accept artifact connection: %w",
				err,
			)
		}

		go server.handle(connection)
	}
}

func commitObject(
	temp *os.File,
	objectsDir string,
	digest string,
	byteLength int64,
) (Receipt, error) {
	prefix := digest[:2]

	algorithmDir := filepath.Join(
		objectsDir,
		digestAlgorithm,
	)

	prefixDir := filepath.Join(
		algorithmDir,
		prefix,
	)

	if err := ensureDirectory(
		algorithmDir,
		objectsDir,
		0750,
	); err != nil {
		return Receipt{}, err
	}

	if err := ensureDirectory(
		prefixDir,
		algorithmDir,
		0750,
	); err != nil {
		return Receipt{}, err
	}

	finalPath := filepath.Join(
		prefixDir,
		digest,
	)

	if err := temp.Chmod(0440); err != nil {
		return Receipt{}, fmt.Errorf(
			"set committed object mode: %w",
			err,
		)
	}

	if err := temp.Sync(); err != nil {
		return Receipt{}, fmt.Errorf(
			"sync committed object metadata: %w",
			err,
		)
	}

	reused := false

	err := os.Link(
		temp.Name(),
		finalPath,
	)

	switch {
	case err == nil:
		if err := syncDirectory(prefixDir); err != nil {
			return Receipt{}, err
		}

	case errors.Is(err, os.ErrExist):
		if err := verifyExistingObject(
			finalPath,
			digest,
			byteLength,
		); err != nil {
			return Receipt{}, err
		}

		reused = true

	default:
		return Receipt{}, fmt.Errorf(
			"commit exact-byte object without replacement: %w",
			err,
		)
	}

	storageReference := filepath.ToSlash(
		filepath.Join(
			"objects",
			digestAlgorithm,
			prefix,
			digest,
		),
	)

	return Receipt{
		ByteLength:       byteLength,
		Reused:           reused,
		SHA256:           digest,
		StorageReference: storageReference,
	}, nil
}

func cleanupIncoming(objectsDir string) error {
	entries, err := os.ReadDir(objectsDir)
	if err != nil {
		return fmt.Errorf(
			"read artifact objects directory: %w",
			err,
		)
	}

	removed := false

	for _, entry := range entries {
		if !strings.HasPrefix(
			entry.Name(),
			".incoming-",
		) {
			continue
		}

		if entry.IsDir() {
			return fmt.Errorf(
				"unexpected incoming artifact directory: %s",
				entry.Name(),
			)
		}

		path := filepath.Join(
			objectsDir,
			entry.Name(),
		)

		if err := os.Remove(path); err != nil {
			return fmt.Errorf(
				"remove stale incoming artifact %s: %w",
				entry.Name(),
				err,
			)
		}

		removed = true
	}

	if removed {
		if err := syncDirectory(objectsDir); err != nil {
			return err
		}
	}

	return nil
}

func ensureDirectory(
	path string,
	parent string,
	mode os.FileMode,
) error {
	created := false

	err := os.Mkdir(path, mode)

	switch {
	case err == nil:
		created = true

	case errors.Is(err, os.ErrExist):
		info, statErr := os.Stat(path)
		if statErr != nil {
			return fmt.Errorf(
				"inspect artifact object directory %s: %w",
				path,
				statErr,
			)
		}

		if !info.IsDir() {
			return fmt.Errorf(
				"artifact object path exists and is not a directory: %s",
				path,
			)
		}

	default:
		return fmt.Errorf(
			"create artifact object directory %s: %w",
			path,
			err,
		)
	}

	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf(
			"set artifact object directory mode %s: %w",
			path,
			err,
		)
	}

	if created {
		if err := syncDirectory(parent); err != nil {
			return err
		}
	}

	return nil
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hasher := sha256.New()

	byteLength, err := io.Copy(
		hasher,
		file,
	)
	if err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(hasher.Sum(nil)), byteLength, nil
}

func probeAuthority(objectsDir string) error {
	info, err := os.Stat(objectsDir)
	if err != nil {
		return fmt.Errorf(
			"artifact objects directory unavailable: %w",
			err,
		)
	}

	if !info.IsDir() {
		return fmt.Errorf(
			"artifact objects path is not a directory",
		)
	}

	const writeAccess = 2 // POSIX W_OK.

	if err := syscall.Access(objectsDir, writeAccess); err != nil {
		return fmt.Errorf(
			"artifact objects directory is not writable by preservation authority: %w",
			err,
		)
	}

	return nil
}

func prepareSocket(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf(
			"artifact socket path must be absolute",
		)
	}

	info, err := os.Lstat(path)

	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil

	case err != nil:
		return fmt.Errorf(
			"inspect artifact socket path: %w",
			err,
		)

	case info.Mode()&os.ModeSocket == 0:
		return fmt.Errorf(
			"artifact socket path exists and is not a socket: %s",
			path,
		)

	default:
		if err := os.Remove(path); err != nil {
			return fmt.Errorf(
				"remove stale artifact socket: %w",
				err,
			)
		}

		return nil
	}
}

func sendResponse(
	connection *net.UnixConn,
	result response,
) {
	encoder := json.NewEncoder(connection)
	_ = encoder.Encode(result)
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf(
			"open artifact directory for sync %s: %w",
			path,
			err,
		)
	}
	defer directory.Close()

	if err := directory.Sync(); err != nil {
		return fmt.Errorf(
			"sync artifact directory %s: %w",
			path,
			err,
		)
	}

	return nil
}

func validateReceipt(receipt Receipt) error {
	if receipt.ByteLength < 0 {
		return fmt.Errorf(
			"negative artifact byte length",
		)
	}

	if len(receipt.SHA256) != sha256.Size*2 {
		return fmt.Errorf(
			"invalid SHA-256 length",
		)
	}

	if _, err := hex.DecodeString(receipt.SHA256); err != nil {
		return fmt.Errorf(
			"invalid SHA-256: %w",
			err,
		)
	}

	expectedPrefix := filepath.ToSlash(
		filepath.Join(
			"objects",
			digestAlgorithm,
			receipt.SHA256[:2],
			receipt.SHA256,
		),
	)

	if receipt.StorageReference != expectedPrefix {
		return fmt.Errorf(
			"unexpected storage reference",
		)
	}

	return nil
}

func verifyExistingObject(
	path string,
	expectedDigest string,
	expectedLength int64,
) error {
	actualDigest, actualLength, err := hashFile(path)
	if err != nil {
		return fmt.Errorf(
			"verify existing content-addressed object: %w",
			err,
		)
	}

	if actualDigest != expectedDigest {
		return fmt.Errorf(
			"existing content-addressed object digest mismatch",
		)
	}

	if actualLength != expectedLength {
		return fmt.Errorf(
			"existing content-addressed object length mismatch",
		)
	}

	return nil
}

func (server Server) handle(
	connection *net.UnixConn,
) {
	defer connection.Close()

	reader := bufio.NewReaderSize(
		connection,
		requestHeaderBuffer,
	)

	header, err := reader.ReadSlice('\n')
	if err != nil {
		sendResponse(
			connection,
			response{
				Error: fmt.Sprintf(
					"read artifact request header: %v",
					err,
				),
				OK: false,
			},
		)
		return
	}

	switch string(header) {
	case protocolProbeHeader:
		if err := probeAuthority(server.ObjectsDir); err != nil {
			sendResponse(
				connection,
				response{
					Error: err.Error(),
					OK:    false,
				},
			)
			return
		}

		sendResponse(
			connection,
			response{
				OK: true,
			},
		)
		return

	case protocolPreserveHeader:
		receipt, err := server.receive(reader)
		if err != nil {
			sendResponse(
				connection,
				response{
					Error: err.Error(),
					OK:    false,
				},
			)
			return
		}

		sendResponse(
			connection,
			response{
				ByteLength:       receipt.ByteLength,
				OK:               true,
				Reused:           receipt.Reused,
				SHA256:           receipt.SHA256,
				StorageReference: receipt.StorageReference,
			},
		)
		return

	default:
		sendResponse(
			connection,
			response{
				Error: "unsupported artifact preservation protocol request",
				OK:    false,
			},
		)
	}
}

func (server Server) receive(
	source io.Reader,
) (Receipt, error) {
	temp, err := os.CreateTemp(
		server.ObjectsDir,
		".incoming-*",
	)
	if err != nil {
		return Receipt{}, fmt.Errorf(
			"create incoming artifact object: %w",
			err,
		)
	}

	tempPath := temp.Name()

	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()

	hasher := sha256.New()

	limited := io.LimitReader(
		source,
		server.MaxBytes+1,
	)

	byteLength, err := io.Copy(
		io.MultiWriter(temp, hasher),
		limited,
	)
	if err != nil {
		return Receipt{}, fmt.Errorf(
			"receive artifact bytes: %w",
			err,
		)
	}

	if byteLength > server.MaxBytes {
		return Receipt{}, fmt.Errorf(
			"artifact exceeds maximum size of %d bytes",
			server.MaxBytes,
		)
	}

	if err := temp.Sync(); err != nil {
		return Receipt{}, fmt.Errorf(
			"sync incoming artifact bytes: %w",
			err,
		)
	}

	digest := hex.EncodeToString(
		hasher.Sum(nil),
	)

	receipt, err := commitObject(
		temp,
		server.ObjectsDir,
		digest,
		byteLength,
	)
	if err != nil {
		return Receipt{}, err
	}

	return receipt, nil
}

func (server Server) validate() error {
	if server.MaxBytes < 1 {
		return fmt.Errorf(
			"artifact maximum byte count must be positive",
		)
	}

	for _, path := range []string{
		server.ObjectsDir,
		server.SocketPath,
	} {
		if !filepath.IsAbs(path) {
			return fmt.Errorf(
				"artifact preservation path must be absolute: %s",
				path,
			)
		}

		if strings.ContainsRune(path, '\x00') {
			return fmt.Errorf(
				"artifact preservation path contains NUL",
			)
		}
	}

	info, err := os.Stat(server.ObjectsDir)
	if err != nil {
		return fmt.Errorf(
			"artifact objects directory: %w",
			err,
		)
	}

	if !info.IsDir() {
		return fmt.Errorf(
			"artifact objects path is not a directory",
		)
	}

	socketDir := filepath.Dir(server.SocketPath)

	info, err = os.Stat(socketDir)
	if err != nil {
		return fmt.Errorf(
			"artifact socket directory: %w",
			err,
		)
	}

	if !info.IsDir() {
		return fmt.Errorf(
			"artifact socket parent is not a directory",
		)
	}

	return nil
}
