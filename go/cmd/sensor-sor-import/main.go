// sensor-sor-import is an operator-only historical frozen-sensor import tool.
// It does not implement a live sensor receiver, registration, or acknowledgements.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

const chunkLimit = 4 << 20
const archiveDefault = "/mnt/udm-pro-20261120-0500EST"

type settings struct {
	Root, Custody, SensorKey, Mode, PSQL, Host, Port, DB, User, QueryFile string
	ExpectFiles, ExpectRows                                               int
}
type segment struct {
	Path, Stream, SHA string
	Length, Count     int64
}

type database struct{ opt settings }

func (db database) cmd(script []byte) (string, error) {
	args := []string{"-X", "-A", "-t", "-v", "ON_ERROR_STOP=1"}
	if db.opt.Host != "" {
		args = append(args, "-h", db.opt.Host)
	}
	if db.opt.Port != "" {
		args = append(args, "-p", db.opt.Port)
	}
	if db.opt.User != "" {
		args = append(args, "-U", db.opt.User)
	}
	args = append(args, "-d", db.opt.DB)
	command := exec.Command(db.opt.PSQL, args...)
	command.Stdin = bytes.NewReader(script)
	command.Env = append(os.Environ(), "PGCONNECT_TIMEOUT=6", "PGAPPNAME=pathfinder-frozen-sensor-import")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("psql: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}
func (db database) query(sql string) (string, error) { return db.cmd([]byte(sql + "\n")) }
func (db database) registerLookup() (string, error) {
	if !safeName.MatchString(db.opt.SensorKey) {
		return "", fmt.Errorf("invalid sensor key")
	}
	query := fmt.Sprintf("SELECT sensor_id FROM pathfinder.sensor WHERE sensor_key=%s;", sensorsor.Quote(db.opt.SensorKey))
	result, err := db.query(query)
	if err != nil {
		return "", err
	}
	if len(result) != 36 {
		return "", fmt.Errorf("sensor %q is not registered (DB returned %q)", db.opt.SensorKey, result)
	}
	return result, nil
}
func (db database) checkMigration() error {
	// Runtime role deliberately has NO SELECT access to schema_migration.
	// Inspect only the public schema catalog for the required source tables
	// and indexed batch lookup paths; do not weaken migration authority.
	const query = `SELECT CASE WHEN
      to_regclass('pathfinder.sensor') IS NOT NULL AND
      to_regclass('pathfinder.sensor_segment') IS NOT NULL AND
      to_regclass('pathfinder.sensor_batch') IS NOT NULL AND
      to_regclass('pathfinder.sensor_batch_index_completion') IS NOT NULL AND
      to_regclass('pathfinder.sensor_observation') IS NOT NULL AND
      to_regclass('pathfinder.sensor_conntrack_event') IS NOT NULL AND
      to_regclass('pathfinder.sensor_identity_snapshot') IS NOT NULL AND
      to_regclass('pathfinder.sensor_health') IS NOT NULL AND
      to_regclass('pathfinder.sensor_observation_batch_lookup_idx') IS NOT NULL AND
      to_regclass('pathfinder.sensor_conntrack_batch_lookup_idx') IS NOT NULL
      THEN 'PASS' ELSE 'FAIL' END;`
	got, err := db.query(query)
	if err != nil {
		return err
	}
	if got != "PASS" {
		return fmt.Errorf("required sensor SoR schema or batch indexes are missing (migrations 0008/0009)")
	}
	return nil
}
func (db database) indexed(meta sensorsor.BatchSQL) (bool, error) {
	// Compare ALL replay-critical properties in the same DB query, not only the batch ID.
	q := fmt.Sprintf(`SELECT CASE
      WHEN b.batch_id IS NULL THEN 'NEW'
      WHEN b.batch_id<>%s OR b.payload_sha256<>%s OR b.payload_bytes<>%d
        OR b.record_count<>%d OR b.segment_id IS DISTINCT FROM %s
        OR b.custody_reference<>%s OR b.schema_version<>%s
        OR b.first_observed_at<>%s OR b.last_observed_at<>%s
        THEN 'CONFLICT'
      WHEN c.batch_id IS NULL THEN 'INCOMPLETE'
      WHEN c.indexed_record_count<>%d THEN 'CONFLICT'
      ELSE 'COMPLETE'
    END
    FROM (SELECT 1) x
    LEFT JOIN pathfinder.sensor_batch b
      ON b.sensor_id=%s AND b.stream=%s AND b.wire_batch_id=%s
    LEFT JOIN pathfinder.sensor_batch_index_completion c ON c.batch_id=b.batch_id;`,
		sensorsor.Quote(meta.BatchUUID), sensorsor.Quote(meta.SHA), meta.Bytes, meta.Evidence.Count,
		sensorsor.Quote(meta.SegmentUUID), sensorsor.Quote(meta.Reference), sensorsor.Quote(meta.SchemaVersion),
		sensorsor.Quote(meta.Evidence.First.UTC().Format(time.RFC3339Nano)),
		sensorsor.Quote(meta.Evidence.Last.UTC().Format(time.RFC3339Nano)), meta.Evidence.Count,
		sensorsor.Quote(meta.SensorUUID), sensorsor.Quote(meta.Stream), sensorsor.Quote(meta.WireBatchID))
	got, err := db.query(q)
	if err != nil {
		return false, err
	}
	switch got {
	case "NEW":
		return false, nil
	case "COMPLETE":
		q = fmt.Sprintf("SELECT count(*) FROM pathfinder.%s WHERE batch_id=%s;", baseTable(meta.Stream), sensorsor.Quote(meta.BatchUUID))
		n, err := db.query(q)
		if err != nil {
			return false, err
		}
		if n != strconv.Itoa(meta.Evidence.Count) {
			return false, fmt.Errorf("corrupt indexed batch %s count=%s want=%d", meta.WireBatchID, n, meta.Evidence.Count)
		}
		return true, nil
	case "CONFLICT", "INCOMPLETE":
		return false, fmt.Errorf("existing batch %s: %s; aborting", meta.WireBatchID, got)
	default:
		return false, fmt.Errorf("unexpected replay status %q", got)
	}
}
func baseTable(stream string) string {
	switch stream {
	case "observations":
		return "sensor_observation"
	case "conntrack":
		return "sensor_conntrack_event"
	case "identity":
		return "sensor_identity_snapshot"
	default:
		return "sensor_health"
	}
}

func classify(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 1 && strings.HasPrefix(parts[0], "observations-") {
		return "observations", nil
	}
	if len(parts) == 2 {
		switch parts[0] {
		case "conntrack":
			if strings.HasPrefix(parts[1], "conntrack-") {
				return "conntrack", nil
			}
		case "identity":
			if strings.HasPrefix(parts[1], "endpoint-identity-") {
				return "identity", nil
			}
		case "health":
			if strings.HasPrefix(parts[1], "health-") {
				return "health", nil
			}
		}
	}
	return "", fmt.Errorf("unexpected archive path %q", rel)
}
func audit(root, key string) ([]segment, map[string]int64, error) {
	var files []segment
	totals := map[string]int64{"observations": 0, "conntrack": 0, "identity": 0, "health": 0}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("non-regular path in archive: %s", path)
		}
		// This exact root-level file is a frozen-capture transfer artifact,
		// not a sensor JSONL segment. Do not ignore other unexpected files.
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".pathfinder-transfer.tar" {
			fmt.Printf("SKIP_TRANSFER_ARTIFACT %s\n", rel)
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl.gz") && !strings.HasSuffix(path, ".jsonl.active") {
			return fmt.Errorf("unexpected input file: %s", path)
		}
		stream, err := classify(root, path)
		if err != nil {
			return err
		}
		a, err := sensoringest.AuditFile(path, sensoringest.Stream(stream), key, time.Time{}, time.Time{})
		if err != nil {
			return fmt.Errorf("audit %s: %w", path, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		totals[stream] += int64(a.Total)
		files = append(files, segment{Path: path, Stream: stream, SHA: a.SHA256, Length: info.Size(), Count: int64(a.Total)})
		fmt.Printf("AUDIT %-12s %8d %s %s\n", stream, a.Total, a.SHA256, filepath.Base(path))
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("no frozen sensor files found")
	}
	return files, totals, nil
}
func openLines(path string, compressed bool) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if compressed {
		reader, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, err
		}
		return multiCloser{Reader: reader, close: func() error {
			err := reader.Close()
			e2 := file.Close()
			if err != nil {
				return err
			}
			return e2
		}}, nil
	}
	return file, nil
}

type multiCloser struct {
	io.Reader
	close func() error
}

func (m multiCloser) Close() error { return m.close() }

func privateDir(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("custody path must be absolute")
	}
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("custody dir must be private, non-symlink")
	}
	return nil
}
func syncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func shaFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
func preserveSegment(source, directory, digest string, length int64) (string, error) {
	if err := privateDir(directory); err != nil {
		return "", err
	}
	dest := filepath.Join(directory, digest+".segment")
	verify := func() error {
		sum, n, err := shaFile(dest)
		if err != nil {
			return err
		}
		if sum != digest || n != length {
			return fmt.Errorf("conflicting or corrupt frozen segment custody")
		}
		return nil
	}
	if _, err := os.Lstat(dest); err == nil {
		if err := verify(); err != nil {
			return "", err
		}
		return dest, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	input, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer input.Close()
	out, err := os.CreateTemp(directory, ".pending-segment-")
	if err != nil {
		return "", err
	}
	name := out.Name()
	defer os.Remove(name)
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), input)
	if err != nil {
		out.Close()
		return "", err
	}
	if hex.EncodeToString(h.Sum(nil)) != digest || n != length {
		out.Close()
		return "", fmt.Errorf("source segment changed after audit")
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	if err := os.Link(name, dest); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
		if err := verify(); err != nil {
			return "", err
		}
	}
	if err := syncDir(directory); err != nil {
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	if err := syncDir(directory); err != nil {
		return "", err
	}
	return dest, nil
}

func run(opt settings) error {
	if opt.Mode == "sni" {
		db := database{opt}
		if err := db.checkMigration(); err != nil {
			return err
		}
		result, err := os.ReadFile(opt.QueryFile)
		if err != nil {
			return err
		}
		names, err := db.cmd(result)
		if err != nil {
			return err
		}
		fmt.Println("===== UNIQUE SNI HOSTNAMES FROM POSTGRESQL =====")
		if names == "" {
			fmt.Println("(no SNI rows)")
			return nil
		}
		fmt.Println(names)
		fmt.Printf("UNIQUE_SNI_COUNT %d\n", len(strings.Split(names, "\n")))
		return nil
	}
	if opt.SensorKey == "" || !safeName.MatchString(opt.SensorKey) {
		return fmt.Errorf("-sensor-key must be an exact registered sensor identifier")
	}
	files, totals, err := audit(opt.Root, opt.SensorKey)
	if err != nil {
		return err
	}
	actual := int64(0)
	for _, n := range totals {
		actual += n
	}
	fmt.Printf("AUDIT_TOTAL files=%d records=%d per_stream=%v\n", len(files), actual, totals)
	if opt.ExpectFiles > 0 && len(files) != opt.ExpectFiles {
		return fmt.Errorf("archive file count=%d expected %d", len(files), opt.ExpectFiles)
	}
	if opt.ExpectRows > 0 && actual != int64(opt.ExpectRows) {
		return fmt.Errorf("archive records=%d expected %d", actual, opt.ExpectRows)
	}
	if opt.Mode != "import" && opt.Mode != "dry-run" {
		return fmt.Errorf("unsupported mode %q", opt.Mode)
	}
	var db database
	var sensorUUID string
	var custody *sensoringest.Custody
	if opt.Mode == "import" {
		db = database{opt}
		if err := db.checkMigration(); err != nil {
			return err
		}
		sensorUUID, err = db.registerLookup()
		if err != nil {
			return err
		}
		if err := privateDir(opt.Custody); err != nil {
			return err
		}
		custody, err = sensoringest.OpenCustody(filepath.Join(opt.Custody, "batches"))
		if err != nil {
			return err
		}
	} else {
		sensorUUID = sensorsor.ID("dry-run-only", opt.SensorKey)
	}
	processed := map[string]int64{}
	batches := 0
	for _, seg := range files {
		var segmentReference string
		if opt.Mode == "import" {
			segmentReference, err = preserveSegment(seg.Path, filepath.Join(opt.Custody, "segments"), seg.SHA, seg.Length)
			if err != nil {
				return err
			}
		} else {
			segmentReference = seg.Path
		}
		reader, err := openLines(segmentReference, strings.HasSuffix(seg.Path, ".gz"))
		if err != nil {
			return err
		}
		segmentOrdinal := int64(0)
		ordinalStart := int64(1)
		chunkNumber := 0
		chunk := make([]byte, 0, chunkLimit)
		batchLines := make([][]byte, 0, 2500)
		flush := func() error {
			if len(batchLines) == 0 {
				return nil
			}
			chunkNumber++
			batchID := fmt.Sprintf("frozen-v1-%s-%04d", seg.SHA[:32], chunkNumber)
			buuid := sensorsor.ID("frozen-import-batch-v1", sensorUUID, seg.Stream, batchID)
			e := sensorsor.New()
			ctx := sensorsor.Context{SensorUUID: sensorUUID, BatchUUID: buuid, StartOrdinal: 1}
			for i, line := range batchLines {
				if _, err := sensoringest.Validate(sensoringest.Stream(seg.Stream), bytes.TrimSuffix(line, []byte("\n")), opt.SensorKey); err != nil {
					return fmt.Errorf("batch %s record %d: %w", batchID, i+1, err)
				}
				if err := e.Append(seg.Stream, ctx, ordinalStart+int64(i), bytes.TrimSuffix(line, []byte("\n"))); err != nil {
					return fmt.Errorf("projection %s record %d: %w", batchID, i+1, err)
				}
			}
			hash := sha256.Sum256(chunk)
			digest := hex.EncodeToString(hash[:])
			reference := filepath.Join(opt.Custody, "batches", "dry-run")
			if opt.Mode == "import" {
				b := sensoringest.Batch{SensorID: opt.SensorKey, Stream: sensoringest.Stream(seg.Stream), SchemaVersion: schemaFor(seg.Stream), BatchID: batchID, Payload: chunk}
				receipt, err := custody.Preserve(b)
				if err != nil {
					return fmt.Errorf("preserve %s: %w", batchID, err)
				}
				if receipt.SHA256 != digest || receipt.RecordCount != len(batchLines) {
					return fmt.Errorf("custody receipt mismatch")
				}
				reference, err = custody.ReceiptPath(receipt)
				if err != nil {
					return err
				}
			}
			meta := sensorsor.BatchSQL{SensorUUID: sensorUUID, SegmentUUID: sensorsor.ID("frozen-import-segment-v1", sensorUUID, seg.Stream, seg.SHA), BatchUUID: buuid,
				Stream: seg.Stream, WireBatchID: batchID, SchemaVersion: schemaFor(seg.Stream), SegmentFilename: filepath.Base(seg.Path), SegmentSHA: seg.SHA, SegmentReference: segmentReference, SegmentBytes: seg.Length, SegmentRecords: seg.Count,
				SHA: digest, Reference: reference, Bytes: len(chunk), PreservedAt: time.Now().UTC(), Evidence: e}
			if opt.Mode == "import" {
				indexed, err := db.indexed(meta)
				if err != nil {
					return err
				}
				if indexed {
					fmt.Printf("SKIP_REPLAY %-12s count=%d batch=%s\n", seg.Stream, e.Count, batchID)
				} else {
					script, err := meta.SQLScript()
					if err != nil {
						return err
					}
					if _, err := db.cmd(script); err != nil {
						return fmt.Errorf("index batch %s: %w", batchID, err)
					}
					fmt.Printf("INDEXED %-12s count=%d batch=%s\n", seg.Stream, e.Count, batchID)
				}
			}
			batches++
			processed[seg.Stream] += int64(e.Count)
			chunk = make([]byte, 0, chunkLimit)
			batchLines = make([][]byte, 0, 2500)
			ordinalStart = segmentOrdinal + 1
			return nil
		}
		buf := bufio.NewReaderSize(reader, 64*1024)
		for {
			line, readErr := buf.ReadBytes('\n')
			if readErr == io.EOF && len(line) == 0 {
				break
			}
			if readErr != nil {
				reader.Close()
				return fmt.Errorf("read %s at line %d: %w", seg.Path, segmentOrdinal+1, readErr)
			}
			if len(line) > sensoringest.MaxRecordBytes+1 {
				reader.Close()
				return fmt.Errorf("oversized record in %s", seg.Path)
			}
			if len(chunk)+len(line) > chunkLimit || len(batchLines) >= 2500 {
				if err := flush(); err != nil {
					reader.Close()
					return err
				}
			}
			segmentOrdinal++
			chunk = append(chunk, line...)
			batchLines = append(batchLines, line)
		}
		if err := flush(); err != nil {
			reader.Close()
			return err
		}
		if err := reader.Close(); err != nil {
			return err
		}
		if segmentOrdinal != seg.Count {
			return fmt.Errorf("source segment changed during import %s: %d != %d", seg.Path, segmentOrdinal, seg.Count)
		}
	}
	for stream, n := range totals {
		if processed[stream] != n {
			return fmt.Errorf("projected source count %s: %d != %d", stream, processed[stream], n)
		}
	}
	fmt.Printf("SOURCE_RECONCILED batches=%d total=%d streams=%v\n", batches, actual, processed)
	if opt.Mode == "import" {
		if err := verifyArchive(db, sensorUUID, files, totals); err != nil {
			return err
		}
		fmt.Println("DATABASE_ARCHIVE_RECONCILIATION_PASS")
		result, err := os.ReadFile(opt.QueryFile)
		if err != nil {
			return err
		}
		names, err := db.cmd(result)
		if err != nil {
			return err
		}
		fmt.Println("===== DISTINCT SNI FROM COMMITTED POSTGRESQL TABLE =====")
		if names != "" {
			fmt.Println(names)
		}
		if names == "" {
			fmt.Println("(no SNI values)")
			fmt.Println("UNIQUE_SNI_COUNT 0")
		} else {
			fmt.Printf("UNIQUE_SNI_COUNT %d\n", len(strings.Split(names, "\n")))
		}
	}
	return nil
}
func schemaFor(stream string) string {
	switch stream {
	case "observations":
		return "5"
	case "health":
		return "2"
	default:
		return "1"
	}
}

func verifyArchive(db database, sensorUUID string, files []segment, expected map[string]int64) error {
	for _, stream := range []string{"observations", "conntrack", "identity", "health"} {
		shas := []string{}
		for _, seg := range files {
			if seg.Stream == stream {
				shas = append(shas, sensorsor.Quote(seg.SHA))
			}
		}
		if len(shas) == 0 {
			return fmt.Errorf("missing stream %s", stream)
		}
		// Read actual evidence rows, not merely completed batch counters.
		sql := fmt.Sprintf(`SELECT count(*) FROM pathfinder.%s e WHERE e.sensor_id=%s AND e.batch_id IN (
          SELECT b.batch_id FROM pathfinder.sensor_batch b JOIN pathfinder.sensor_segment s ON b.segment_id=s.segment_id
          WHERE s.sensor_id=%s AND s.stream=%s AND s.stored_sha256 IN (%s));`,
			baseTable(stream), sensorsor.Quote(sensorUUID), sensorsor.Quote(sensorUUID), sensorsor.Quote(stream), strings.Join(shas, ","))
		got, err := db.query(sql)
		if err != nil {
			return err
		}
		if got != strconv.FormatInt(expected[stream], 10) {
			return fmt.Errorf("DB source count mismatch %s got=%s want=%d", stream, got, expected[stream])
		}
		fmt.Printf("DB_RECONCILED %-12s count=%s\n", stream, got)
	}
	return nil
}

func main() {
	var opt settings
	flag.StringVar(&opt.Mode, "mode", "dry-run", "dry-run, import, or sni")
	flag.StringVar(&opt.Root, "root", archiveDefault, "frozen snapshot directory")
	flag.StringVar(&opt.SensorKey, "sensor-key", "", "registered/authenticated sensor key; required for dry-run/import")
	flag.StringVar(&opt.Custody, "custody", "", "private absolute custody directory; required for import")
	flag.StringVar(&opt.PSQL, "psql", "psql", "PostgreSQL psql executable")
	flag.StringVar(&opt.Host, "db-host", "", "PostgreSQL host or local socket default")
	flag.StringVar(&opt.Port, "db-port", "5432", "PostgreSQL port")
	flag.StringVar(&opt.DB, "db-name", "pathfinder", "PostgreSQL database name")
	flag.StringVar(&opt.User, "db-user", "", "database login; omitted uses libpq default")
	flag.StringVar(&opt.QueryFile, "sni-query", "queries/sensor_unique_sni.sql", "file holding the approved unique-SNI SQL query")
	flag.IntVar(&opt.ExpectFiles, "expect-files", 9, "expected frozen segment count (0=unrestricted)")
	flag.IntVar(&opt.ExpectRows, "expect-rows", 685218, "expected frozen source-row count (0=unrestricted)")
	flag.Parse()
	if len(flag.Args()) != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	if opt.Mode == "import" && opt.Custody == "" {
		fmt.Fprintln(os.Stderr, "-custody is required for import")
		os.Exit(2)
	}
	if err := run(opt); err != nil {
		fmt.Fprintln(os.Stderr, "SENSOR_SOR_IMPORT_FAILED:", err)
		os.Exit(1)
	}
}
