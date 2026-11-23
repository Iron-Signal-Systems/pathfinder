package sensorc1

import (
    "context"
    "errors"
    "fmt"
    "os"
    "crypto/sha256"
    "sync"

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

// A fixed-size set of in-process SQL gates coordinates the exact
// Inspect→Execute→Inspect sequence for duplicate natural source keys.
// No per-batch map, journal, or unbounded lock collection is retained.
// Database advisory transaction locks still protect against OTHER processes.
const sorSQLGateCount = 256

type SoRProcessor struct {
    ReadRoot string
    Originals OriginalSourceReader
    SQL SoRSQLExecutor
    sqlGates [sorSQLGateCount]sync.Mutex
}

// ErrVolatileRAMWorkGone is limited to a missing work object on the RAM
// handoff path. Missing ORIGINAL custody is always a distinct fatal error.
var ErrVolatileRAMWorkGone = errors.New("C1 SoR: volatile work object disappeared")

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
    if errors.Is(err,os.ErrNotExist){
        return zero,fmt.Errorf("%w: %w",ErrVolatileRAMWorkGone,err)
    }
    if err!=nil{return zero,fmt.Errorf("C1 SoR: RAM input invalid: %w",err)}
    committed:=0
    ids:=make([]string,0,len(bundle.Batches))
    for i,p:=range bundle.Batches {
        if err:=ctx.Err();err!=nil{return zero,err}
        original,err:=s.Originals.OpenOriginal(ctx,p)
        if err!=nil{return zero,fmt.Errorf("C1 SoR: custody batch %d: %w",i,err)}
        plan,err:=BuildSoRBatch(p,original)
        if err!=nil{return zero,fmt.Errorf("C1 SoR: original proof batch %d: %w",i,err)}
        newlyCommitted,err:=s.commitOne(ctx,plan)
        if err!=nil{
            return zero,fmt.Errorf("C1 SoR: batch %d: %w",i,err)
        }
        if newlyCommitted{committed++}
        ids=append(ids,p.BatchUUID)
    }
    outcome:="already_committed"
    if committed>0{outcome="committed"}
    return SQLCommitAck{Name:ref.Name,SHA256:ref.SHA256,Stream:ref.Stream,
        Records:ref.Records,BatchUUIDs:ids,Outcome:outcome},nil
}

 
// commitOne is the ONLY SQL work path from the SoR processor. Lock by the
// verified natural key, not the RAM object name, so two differently named
// bundles containing the same original batch cannot both observe "new".
// The lock is released on every error and does not hold verifier B.
func(s *SoRProcessor)commitOne(ctx context.Context,plan sensorsor.BatchSQL)(bool,error){
    key:=sha256.Sum256([]byte(plan.SensorUUID+"\x00"+plan.Stream+"\x00"+plan.WireBatchID))
    gate:=&s.sqlGates[key[0]]
    gate.Lock()
    defer gate.Unlock()
    if err:=ctx.Err();err!=nil{return false,err}
    state,err:=s.SQL.Inspect(ctx,plan)
    if err!=nil{return false,fmt.Errorf("SQL preflight: %w",err)}
    wrote:=false
    switch state{
    case SQLComplete:
        // Durable metadata matches the original replay identity.
        // Independent verifier B checks all typed rows in parallel.
    case SQLNew:
        script,err:=plan.SQLScript()
        if err!=nil{return false,fmt.Errorf("SQL plan: %w",err)}
        if executeErr:=s.SQL.Execute(ctx,script);executeErr!=nil{
            // Another SoR process can commit this SAME original key while
            // this process waits on the SQLScript transaction advisory lock.
            // The second transaction then fails its replay guard, although
            // the first transaction is already durable. Re-inspect the exact
            // original identity after any uncertain execution outcome:
            // "complete" is a safe replay, never a second write. Any other
            // state (or failed read) stays unacknowledged.
            stateAfter,inspectErr:=s.SQL.Inspect(ctx,plan)
            if inspectErr!=nil{
                return false,fmt.Errorf("SQL commit uncertain: %w; durable replay inspection failed: %v",executeErr,inspectErr)
            }
            switch stateAfter {
            case SQLComplete:
                return false,nil // an exact durable replay, not this writer's commit
            case SQLNew,SQLPartial,SQLConflict:
                return false,fmt.Errorf("SQL commit uncertain: %w; durable state=%s",executeErr,stateAfter)
            default:
                return false,fmt.Errorf("SQL commit uncertain: %w; invalid durable state=%q",executeErr,stateAfter)
            }
        }
        wrote=true
    case SQLPartial,SQLConflict:
        return false,fmt.Errorf("%s PostgreSQL state; manual reconciliation required",state)
    default:
        return false,fmt.Errorf("unexpected PostgreSQL state %q",state)
    }
    // An SQL ACK means every batch has a confirmed durable completion,
    // not that the separately scheduled B verifier has completed.
    committedState,err:=s.SQL.Inspect(ctx,plan)
    if err!=nil{return false,fmt.Errorf("postcommit SQL state read: %w",err)}
    if committedState!=SQLComplete{
        return false,fmt.Errorf("postcommit SQL state %q is not complete",committedState)
    }
    return wrote,nil
}
