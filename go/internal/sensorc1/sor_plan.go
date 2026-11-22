package sensorc1

import (
    "errors"
    "fmt"
    "reflect"
    "strings"
    "time"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// OriginalBatch is SoR-verified source provenance, NOT data supplied by the
// ingest process over RAM or mTLS. The future SoR source resolver must read
// these values directly from immutable receipt/custody and validate the source
// segment. A caller constructing this from ingest's claims breaks the boundary.
type OriginalBatch struct {
    Input Input
    SegmentUUID string
    SegmentFilename string
    SegmentSHA256 string
    SegmentReference string
    SegmentStoredBytes int64
    SegmentRecordCount int64
    BatchCustodyReference string
    PreservedAt time.Time
}

// BuildSoRBatch checks the complete prepared evidence against an independently
// opened original batch BEFORE constructing the existing transactional SoR
// SQL plan. It does not execute SQL, record a commit, or acknowledge a sensor.
func BuildSoRBatch(prepared Prepared,original OriginalBatch)(sensorsor.BatchSQL,error){
    var zero sensorsor.BatchSQL
    if err:=ValidatePrepared(prepared);err!=nil{return zero,fmt.Errorf("C1 SoR: prepared input: %w",err)}
    if original.SegmentUUID==""||original.SegmentFilename==""||
       original.SegmentSHA256==""||original.SegmentReference==""||
       original.BatchCustodyReference==""||original.SegmentStoredBytes<=0||
       original.SegmentRecordCount<=0||original.PreservedAt.IsZero(){
        return zero,errors.New("C1 SoR: missing original receiver/segment provenance")
    }
    if !uuidPattern.MatchString(original.SegmentUUID) ||
       len(original.SegmentSHA256)!=64 ||
       strings.Trim(original.SegmentSHA256,"0123456789abcdef")!=""{
        return zero,errors.New("C1 SoR: invalid source segment identity/hash")
    }
    if original.Input.SensorID!=prepared.SensorID ||
       original.Input.SensorUUID!=prepared.SensorUUID ||
       original.Input.Stream!=prepared.Stream ||
       original.Input.SchemaVersion!=prepared.SchemaVersion ||
       original.Input.WireBatchID!=prepared.WireBatchID ||
       original.Input.BatchUUID!=prepared.BatchUUID ||
       original.Input.SegmentOrdinalStart!=prepared.SegmentOrdinalStart{
        return zero,errors.New("C1 SoR: original source identity differs from handoff")
    }
    fresh,err:=Prepare(original.Input)
    if err!=nil{return zero,fmt.Errorf("C1 SoR: original custody invalid: %w",err)}
    if fresh.PayloadSHA256!=prepared.PayloadSHA256 ||
       fresh.PayloadBytes!=prepared.PayloadBytes ||
       fresh.Evidence.Count!=prepared.Evidence.Count ||
       !fresh.Evidence.First.Equal(prepared.Evidence.First) ||
       !fresh.Evidence.Last.Equal(prepared.Evidence.Last) ||
       !reflect.DeepEqual(fresh.Evidence.Tables,prepared.Evidence.Tables) {
        return zero,errors.New("C1 SoR: prepared evidence does not match original custody")
    }
    plan:=sensorsor.BatchSQL{
        SensorUUID:prepared.SensorUUID,
        SegmentUUID:original.SegmentUUID,
        BatchUUID:prepared.BatchUUID,
        Stream:string(prepared.Stream),
        WireBatchID:prepared.WireBatchID,
        SchemaVersion:prepared.SchemaVersion,
        SegmentFilename:original.SegmentFilename,
        SegmentSHA:original.SegmentSHA256,
        SegmentReference:original.SegmentReference,
        SegmentBytes:original.SegmentStoredBytes,
        SegmentRecords:original.SegmentRecordCount,
        SHA:prepared.PayloadSHA256,
        Reference:original.BatchCustodyReference,
        Bytes:prepared.PayloadBytes,
        PreservedAt:original.PreservedAt,
        Evidence:prepared.Evidence,
    }
    if err:=plan.Validate();err!=nil{return zero,fmt.Errorf("C1 SoR: SQL batch plan invalid: %w",err)}
    return plan,nil
}

// BuildSoRSQL is the development's final offline preparation-to-transaction
// boundary, reusing the already established atomic COPY + completion marker.
// A real SoR worker will have to check duplicate/dirty SQL state before
// execution, commit every batch durably, and issue only matching SQL ACKs.
func BuildSoRSQL(prepared Prepared,original OriginalBatch)([]byte,error){
    plan,err:=BuildSoRBatch(prepared,original)
    if err!=nil{return nil,err}
    return plan.SQLScript()
}
