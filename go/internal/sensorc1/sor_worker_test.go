package sensorc1

import (
    "context"
    "errors"
    "os"
    "strings"
    "testing"
    "time"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

type testOriginalReader struct {
    values map[string]OriginalBatch
    fail error
}
func(r testOriginalReader)OpenOriginal(_ context.Context,p Prepared)(OriginalBatch,error){
    if r.fail!=nil{return OriginalBatch{},r.fail}
    o,ok:=r.values[p.BatchUUID]
    if !ok{return OriginalBatch{},errors.New("custody batch absent")}
    return o,nil
}

type testSQLExecutor struct {
    states map[string]SQLBatchState
    last string
    writes,verifications int
    failExecute,failVerify bool
}
func(s *testSQLExecutor)Inspect(_ context.Context,p sensorsor.BatchSQL)(SQLBatchState,error){
    s.last=p.BatchUUID
    state,ok:=s.states[p.BatchUUID]
    if !ok{return SQLNew,nil}
    return state,nil
}
func(s *testSQLExecutor)Execute(_ context.Context,sql []byte)error{
    if s.failExecute{return errors.New("commit status uncertain")}
    if !strings.Contains(string(sql),"COMMIT;"){return errors.New("no SQL commit transaction")}
    s.writes++
    s.states[s.last]=SQLComplete
    return nil
}
func(s *testSQLExecutor)Verify(_ context.Context,p sensorsor.BatchSQL)error{
    if s.failVerify{return errors.New("source evidence readback rejected")}
    if s.states[p.BatchUUID]!=SQLComplete{return errors.New("not committed")}
    s.verifications++
    return nil
}

func processorFixture(t *testing.T)(*SoRProcessor,BundleReference,*testSQLExecutor,Bundle){
    t.Helper()
    root:=t.TempDir()
    if err:=os.Chmod(root,0700);err!=nil{t.Fatal(err)}
    in1:=inputFor(sensoringest.Observations)
    in2:=inputFor(sensoringest.Observations)
    in2.WireBatchID="batch-b"
    in2.BatchUUID=sensorsor.ID("frozen-import-batch-v1",in2.SensorUUID,string(in2.Stream),in2.WireBatchID)
    p1,err:=Prepare(in1);if err!=nil{t.Fatal(err)}
    p2,err:=Prepare(in2);if err!=nil{t.Fatal(err)}
    bundle:=Bundle{Format:BundleFormat,Stream:sensoringest.Observations,
        Batches:[]Prepared{p1,p2},SourceBytes:p1.PayloadBytes+p2.PayloadBytes,
        Records:p1.Evidence.Count+p2.Evidence.Count}
    ref,err:=PublishBundle(root,bundle);if err!=nil{t.Fatal(err)}
    originals:=map[string]OriginalBatch{}
    for _,input:=range []Input{in1,in2}{
        proof:=originalFor(input.Stream)
        proof.Input=input
        originals[input.BatchUUID]=proof
    }
    sql:=&testSQLExecutor{states:map[string]SQLBatchState{}}
    proc:=&SoRProcessor{ReadRoot:root,
        Originals:testOriginalReader{values:originals},SQL:sql}
    return proc,ref,sql,bundle
}

func TestSoRACKRequiresEveryPostgresCommitCheckpoint(t *testing.T){
    proc,ref,sql,b:=processorFixture(t)
    ack,err:=proc.ProcessBundle(context.Background(),ref)
    if err!=nil{t.Fatal(err)}
    if ack.Outcome!="committed"||len(ack.BatchUUIDs)!=2||sql.writes!=2||sql.verifications!=0{
        t.Fatalf("incomplete SQL ACK: %+v, writes=%d verify=%d",ack,sql.writes,sql.verifications)
    }
    if err:=validateCommitAck(ref,[]string{b.Batches[0].BatchUUID,b.Batches[1].BatchUUID},ack);err!=nil{t.Fatal(err)}
    // Processing is read-only with respect to RAM: only the ingest owner may
    // reclaim after receiving this exact authenticated commit acknowledgment.
    if _,err:=ReadBundle(proc.ReadRoot,ref);err!=nil{t.Fatal("SoR deleted producer RAM")}
    // A real duplicate must be verified against durable SQL, not re-inserted.
    ack,err=proc.ProcessBundle(context.Background(),ref)
    if err!=nil{t.Fatal(err)}
    if ack.Outcome!="already_committed"||sql.writes!=2||sql.verifications!=0{
        t.Fatal("exact replay performed SQL writes or blocked on independent B")
    }
}

func TestSoRPartialAndUnknownOutcomeReturnNoACK(t *testing.T){
    for _,state:=range []SQLBatchState{SQLPartial,SQLConflict,"unexpected"}{
        t.Run(string(state),func(t *testing.T){
            proc,ref,sql,b:=processorFixture(t)
            sql.states[b.Batches[1].BatchUUID]=state
            ack,err:=proc.ProcessBundle(context.Background(),ref)
            if err==nil||ack.Outcome!=""{t.Fatalf("false SQL ACK: %+v %v",ack,err)}
            if sql.writes!=1{t.Fatal("first transaction should be independently committed")}
            // No RAM deletion; recovered SoR may inspect first complete batch.
            if _,err:=ReadBundle(proc.ReadRoot,ref);err!=nil{t.Fatal("lost original RAM")}
        })
    }
}
func TestIndependentBFailureDoesNotBlockDurableSQLCommitACK(t *testing.T){
    proc,ref,sql,bundle:=processorFixture(t)
    sql.failVerify=true
    ack,err:=proc.ProcessBundle(context.Background(),ref)
    if err!=nil||ack.Outcome!="committed"{
        t.Fatalf("independent B blocked durable SQL commit: %+v %v",ack,err)
    }
    if sql.verifications!=0{t.Fatal("B verification ran serially in SQL worker")}
    b:=IndependentVerifierB{Originals:proc.Originals,SQL:sql,
        PollInterval:time.Millisecond,MaxWait:time.Second}
    if err:=b.VerifyBatch(context.Background(),bundle.Batches[0]);err==nil{
        t.Fatal("independent B accepted tampered committed evidence")
    }
}

type inMemorySoRTransport struct{ processor *SoRProcessor }
func(t inMemorySoRTransport)Submit(ctx context.Context,ref BundleReference)(<-chan SQLCommitAck,error){
    ch:=make(chan SQLCommitAck,1)
    // This test transport is NOT authenticated and MUST NOT be used in a
    // running jail. Real services require mTLS and session replay fencing.
    go func(){
        ack,err:=t.processor.ProcessBundle(ctx,ref)
        if err==nil { ch<-ack }
        close(ch)
    }()
    return ch,nil
}
func TestEndToEndInMemorySoRWithoutReceiverOrDatabase(t *testing.T){
    root:=t.TempDir()
    if err:=os.Chmod(root,0700);err!=nil{t.Fatal(err)}
    original:=originalFor(sensoringest.Health)
    p,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
    sql:=&testSQLExecutor{states:map[string]SQLBatchState{}}
    proc:=&SoRProcessor{
        ReadRoot:root,
        Originals:testOriginalReader{values:map[string]OriginalBatch{p.BatchUUID:original}},
        SQL:sql,
    }
    dispatch,err:=NewDispatcher(root,inMemorySoRTransport{processor:proc})
    if err!=nil{t.Fatal(err)}
    scheduler,err:=NewScheduler(DefaultSchedulerOptions(),dispatch)
    if err!=nil{t.Fatal(err)}
    ctx:=context.Background()
    if err:=scheduler.Push(ctx,p,time.Now());err!=nil{t.Fatal(err)}
    if err:=scheduler.Drain(ctx);err!=nil{t.Fatal(err)}
    if err:=dispatch.Wait(ctx);err!=nil{t.Fatal(err)}
    if sql.writes!=1||sql.verifications!=0{t.Fatal("SQL commit incomplete or B verifier incorrectly serialized")}
}
