package sensorc1

import (
    "context"
    "errors"
    "fmt"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// OriginalSourceReader runs in pathfinder-sor. It must re-open actual
// immutable source custody (or the operator-controlled frozen replay source
// in clone tests) independently of the RAM contents.
type OriginalSourceReader interface {
    OpenOriginal(context.Context,Prepared)(OriginalBatch,error)
}

// SQLBatchState comes from PostgreSQL, never from a process-local flag.
type SQLBatchState string
const (
    SQLNew SQLBatchState="new"
    SQLComplete SQLBatchState="complete"
    SQLPartial SQLBatchState="partial"
    SQLConflict SQLBatchState="conflict"
)

// SoRSQLExecutor is an in-jail PostgreSQL SQL owner. Implementations must
// enforce exact sensor, stream, batch ID, hash, bytes, record count, child
// coverage and receipt provenance against committed SQL, not only marker
// existence. Production implementations must use only local PostgreSQL.
type SoRSQLExecutor interface {
    Inspect(context.Context,sensorsor.BatchSQL)(SQLBatchState,error)
    Execute(context.Context,[]byte)error
    Verify(context.Context,sensorsor.BatchSQL)error
}

type SoRProcessor struct {
    ReadRoot string
    Originals OriginalSourceReader
    SQL SoRSQLExecutor
}

// ProcessBundle returns an exact SQL COMMIT ACK only after every batch has
// independently checked ORIGINAL source provenance and written/confirmed a
// durable PostgreSQL transaction. Independent verifier B performs full
// readback in PARALLEL, not on the SQL ACK critical path. SQL failure or
// uncertain state stops the group with NO ACK. No batch rows are deleted.
func(s *SoRProcessor)ProcessBundle(ctx context.Context,ref BundleReference)(SQLCommitAck,error){
    var zero SQLCommitAck
    if s==nil||ctx==nil||s.Originals==nil||s.SQL==nil{
        return zero,errors.New("C1 SoR: original custody reader and SQL owner required")
    }
    if err:=ctx.Err();err!=nil{return zero,err}
    bundle,err:=ReadBundle(s.ReadRoot,ref)
    if err!=nil{return zero,fmt.Errorf("C1 SoR: RAM input invalid: %w",err)}
    committed:=0
    ids:=make([]string,0,len(bundle.Batches))
    for i,p:=range bundle.Batches {
        if err:=ctx.Err();err!=nil{return zero,err}
        original,err:=s.Originals.OpenOriginal(ctx,p)
        if err!=nil{return zero,fmt.Errorf("C1 SoR: custody batch %d: %w",i,err)}
        plan,err:=BuildSoRBatch(p,original)
        if err!=nil{return zero,fmt.Errorf("C1 SoR: original proof batch %d: %w",i,err)}
        state,err:=s.SQL.Inspect(ctx,plan)
        if err!=nil{return zero,fmt.Errorf("C1 SoR: SQL preflight batch %d: %w",i,err)}
        switch state {
        case SQLComplete:
            // A completed record may be an exact replay, but readback remains
            // mandatory. Never infer integrity from completion marker alone.
        case SQLNew:
            script,err:=plan.SQLScript()
            if err!=nil{return zero,fmt.Errorf("C1 SoR: SQL plan %d: %w",i,err)}
            if err:=s.SQL.Execute(ctx,script);err!=nil{
                return zero,fmt.Errorf("C1 SoR: SQL commit uncertain for batch %d: %w",i,err)
            }
            committed++
        case SQLPartial,SQLConflict:
            return zero,fmt.Errorf("C1 SoR: %s PostgreSQL state at batch %d; manual reconciliation required",state,i)
        default:
            return zero,fmt.Errorf("C1 SoR: unexpected PostgreSQL state %q",state)
        }
        // SQL ACK reflects the durable completion marker, NOT independent
        // verifier B acceptance. A lost/uncertain SQL response fails closed.
        committedState,err:=s.SQL.Inspect(ctx,plan)
        if err!=nil || committedState!=SQLComplete {
            return zero,fmt.Errorf("C1 SoR: batch %d durable completion not confirmed (state=%s): %w",i,committedState,err)
        }
        ids=append(ids,p.BatchUUID)
    }
    outcome:="already_committed"
    if committed>0{outcome="committed"}
    return SQLCommitAck{Name:ref.Name,SHA256:ref.SHA256,Stream:ref.Stream,
        Records:ref.Records,BatchUUIDs:ids,Outcome:outcome},nil
}
