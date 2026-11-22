// Package sensorc1 is Pathfinder's receiver-independent C1 preparation boundary.
// It validates frozen source bytes and reuses the established typed SoR projector.
// It has no database client, receiver, or custody authority.
package sensorc1

import (
    "bufio"
    "bytes"
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "fmt"
    "regexp"
    "reflect"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

const PreparedFormat = "pathfinder-c1-prepared-v1"

var safeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Input comes from an operator-controlled frozen archive for development.
// SensorID and SensorUUID are provided by the trusted replay controller.
// Live input must first be bound to an immutable receiver custody receipt.
type Input struct {
    SensorID      string
    SensorUUID    string
    Stream        sensoringest.Stream
    SchemaVersion string
    WireBatchID   string
    BatchUUID     string // Original receiver/frozen-import batch UUID; never silently replace it.
    SegmentOrdinalStart int64
    Payload       []byte
}

// Prepared is SQL-ready evidence, not a PostgreSQL commit, receiver receipt,
// or an accepted delivery checkpoint.
type Prepared struct {
    Format         string             `json:"format"`
    SensorID       string             `json:"sensor_id"`
    SensorUUID     string             `json:"sensor_uuid"`
    Stream         sensoringest.Stream `json:"stream"`
    SchemaVersion  string             `json:"schema_version"`
    WireBatchID    string             `json:"wire_batch_id"`
    BatchUUID      string             `json:"batch_uuid"`
    PayloadSHA256  string             `json:"payload_sha256"`
    PayloadBytes   int                `json:"payload_bytes"`
    SegmentOrdinalStart int64          `json:"segment_ordinal_start"`
    Evidence       *sensorsor.Evidence `json:"evidence"`
}

func Prepare(in Input) (Prepared, error) {
    var zero Prepared
    if !safeID.MatchString(in.SensorID) || !safeID.MatchString(in.WireBatchID) || !uuidPattern.MatchString(in.SensorUUID) || !uuidPattern.MatchString(in.BatchUUID) {
        return zero, errors.New("C1 prepare: invalid operator-provided source identity")
    }
    if in.SegmentOrdinalStart < 1 || in.SegmentOrdinalStart > int64(^uint64(0)>>1)-sensoringest.MaxBatchRecords {
        return zero, errors.New("C1 prepare: invalid source segment ordinal")
    }
    if len(in.Payload) == 0 || len(in.Payload) > sensoringest.MaxBatchBytes || in.Payload[len(in.Payload)-1] != '\n' {
        return zero, errors.New("C1 prepare: batch must be nonempty, bounded, newline-terminated JSONL")
    }
    // Scanner.ScanLines would remove CR from CRLF; do not silently alter
    // the exact source bytes used for original-payload SHA-256.
    if bytes.IndexByte(in.Payload, '\r') >= 0 {
        return zero, errors.New("C1 prepare: expected LF-only original JSONL")
    }
    expectedSchema := map[sensoringest.Stream]string{
        sensoringest.Observations:"5", sensoringest.Conntrack:"1",
        sensoringest.Identity:"1", sensoringest.Health:"2",
    }[in.Stream]
    if expectedSchema == "" || in.SchemaVersion != expectedSchema {
        return zero, errors.New("C1 prepare: unsupported stream/schema")
    }
    evidence := sensorsor.New()
    ctx := sensorsor.Context{SensorUUID:in.SensorUUID, BatchUUID:in.BatchUUID, StartOrdinal:1}
    scanner := bufio.NewScanner(bytes.NewReader(in.Payload))
    scanner.Buffer(make([]byte, 64<<10), sensoringest.MaxRecordBytes+2)
    for scanner.Scan() {
        if evidence.Count >= sensoringest.MaxBatchRecords {
            return zero, errors.New("C1 prepare: record limit exceeded")
        }
        raw := scanner.Bytes()
        record, err := sensoringest.Validate(in.Stream, raw, in.SensorID)
        if err != nil { return zero, fmt.Errorf("C1 prepare: record %d: %w", evidence.Count+1, err) }
        if record.SchemaVersion != in.SchemaVersion {
            return zero, fmt.Errorf("C1 prepare: schema mismatch at record %d", evidence.Count+1)
        }
        if err := evidence.Append(string(in.Stream), ctx, in.SegmentOrdinalStart+int64(evidence.Count), raw); err != nil {
            return zero, fmt.Errorf("C1 prepare: projection record %d: %w", evidence.Count+1, err)
        }
    }
    if err := scanner.Err(); err != nil { return zero, fmt.Errorf("C1 prepare: scan: %w", err) }
    if evidence.Count == 0 { return zero, errors.New("C1 prepare: empty input") }
    digest := sha256.Sum256(in.Payload)
    return Prepared{Format:PreparedFormat, SensorID:in.SensorID, SensorUUID:in.SensorUUID,
        Stream:in.Stream, SchemaVersion:in.SchemaVersion, WireBatchID:in.WireBatchID,
        BatchUUID:in.BatchUUID, PayloadSHA256:hex.EncodeToString(digest[:]),
        PayloadBytes:len(in.Payload), SegmentOrdinalStart:in.SegmentOrdinalStart,
        Evidence:evidence}, nil
}

// ValidatePrepared checks that the handoff contains every original JSONL
// source record with exact original bytes and contiguous source ordinals.
// It is not independent verifier-B or PostgreSQL commit verification.
func ValidatePrepared(p Prepared) error {
    if p.Format != PreparedFormat || !safeID.MatchString(p.SensorID) ||
        !safeID.MatchString(p.WireBatchID) || !uuidPattern.MatchString(p.SensorUUID) || !uuidPattern.MatchString(p.BatchUUID) ||
        p.Evidence == nil || p.Evidence.Count < 1 || p.Evidence.Count > sensoringest.MaxBatchRecords ||
        p.PayloadBytes < 1 || p.PayloadBytes > sensoringest.MaxBatchBytes ||
        p.SegmentOrdinalStart < 1 || p.SegmentOrdinalStart > int64(^uint64(0)>>1)-sensoringest.MaxBatchRecords {
        return errors.New("C1 prepared: invalid metadata or limits")
    }
    var tableName string
    switch p.Stream {
    case sensoringest.Observations: tableName="sensor_observation"
    case sensoringest.Conntrack: tableName="sensor_conntrack_event"
    case sensoringest.Identity: tableName="sensor_identity_snapshot"
    case sensoringest.Health: tableName="sensor_health"
    default: return errors.New("C1 prepared: unsupported stream")
    }
    table := p.Evidence.Tables[tableName]
    if table == nil || len(table.Rows) != p.Evidence.Count || len(table.Names) != len(sensorsor.Schema[tableName]) {
        return errors.New("C1 prepared: missing source evidence rows")
    }
    idxRaw := -1
    for i, name := range sensorsor.Schema[tableName] { if name == "raw_record" { idxRaw=i; break } }
    if idxRaw < 0 { return errors.New("C1 prepared: missing raw record column") }
    h := sha256.New()
    size := 0
    // Rebuild the entire typed projection from original raw-record bytes. A
    // valid handoff digest alone does not bind DNS/identity child rows, IPs,
    // timestamps, or record family to the untrusted prepared JSON object.
    expected := sensorsor.New()
    ctx := sensorsor.Context{SensorUUID:p.SensorUUID,BatchUUID:p.BatchUUID,StartOrdinal:1}
    for i,row := range table.Rows {
        if len(row) != len(sensorsor.Schema[tableName]) { return errors.New("C1 prepared: malformed source row") }
        raw := []byte(row[idxRaw])
        rec,err:=sensoringest.Validate(p.Stream,raw,p.SensorID)
        if err != nil || rec.SchemaVersion!=p.SchemaVersion { return fmt.Errorf("C1 prepared: invalid raw record %d",i+1) }
        if err:=assertSourceOrdinal(p.Stream,row,i+1,p.SegmentOrdinalStart+int64(i),p.SensorUUID,p.BatchUUID);err!=nil{return err}
        if err:=expected.Append(string(p.Stream),ctx,p.SegmentOrdinalStart+int64(i),raw);err!=nil {
            return fmt.Errorf("C1 prepared: source re-projection %d: %w",i+1,err)
        }
        h.Write(raw);h.Write([]byte{'\n'});size+=len(raw)+1
    }
    if size != p.PayloadBytes || hex.EncodeToString(h.Sum(nil)) != p.PayloadSHA256 {
        return errors.New("C1 prepared: original payload hash or byte count mismatch")
    }
    if p.Evidence.Count!=expected.Count ||
       !p.Evidence.First.Equal(expected.First) || !p.Evidence.Last.Equal(expected.Last) ||
       !reflect.DeepEqual(p.Evidence.Tables,expected.Tables) {
        return errors.New("C1 prepared: projected source or child evidence differs from original")
    }
    return nil
}

func assertSourceOrdinal(stream sensoringest.Stream,row []string, ordinal int,segmentOrdinal int64,sensorUUID,batchUUID string)error{
    // Identity and health rows place identity fields ahead of timestamps.
    idColumn,batchColumn,ordinalColumn,segmentColumn:=1,3,4,5
    if stream==sensoringest.Identity || stream==sensoringest.Health {
        idColumn,batchColumn,ordinalColumn,segmentColumn=0,2,3,4
    }
    if row[idColumn]!=sensorsor.ID("sensor-source-record-v1",batchUUID,fmt.Sprint(ordinal)) ||
        row[batchColumn]!=batchUUID || row[ordinalColumn]!=fmt.Sprint(ordinal) ||
        row[segmentColumn]!=fmt.Sprint(segmentOrdinal) {
        return fmt.Errorf("C1 prepared: source identity/ordinal mismatch at record %d",ordinal)
    }
    sensorColumn:=2
    if stream==sensoringest.Identity || stream==sensoringest.Health { sensorColumn=1 }
    if row[sensorColumn]!=sensorUUID {return fmt.Errorf("C1 prepared: sensor UUID mismatch at record %d",ordinal)}
    return nil
}
