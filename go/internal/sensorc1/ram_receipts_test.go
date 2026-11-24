package sensorc1

import (
    "context"
    "errors"
    "sync/atomic"
    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

func RAMTestRef() BundleReference {
    return BundleReference{
        Name:"pf-c1-"+strings.Repeat("a",32)+".ready",
        SHA256:strings.Repeat("b",64),
        Bytes:128,Stream:sensoringest.Health,Batches:1,Records:1,
    }
}
func RAMTestAck(r BundleReference)SQLCommitAck{
    return SQLCommitAck{Name:r.Name,SHA256:r.SHA256,Stream:r.Stream,Records:r.Records,
        BatchUUIDs:[]string{"01234567-89ab-4cde-8fab-0123456789ab"},Outcome:"committed"}
}
func TestRAMReceiptTransportExactCommit(t *testing.T){
    gid:=os.Getgid()
    if gid<1{t.Skip("unit runner has no non-root shared group")}
    root:=t.TempDir()
    if err:=os.Chmod(root,0750);err!=nil{t.Fatal(err)}
    ref:=RAMTestRef()
    transport,err:=NewRAMReceiptTransport(root,gid,5*time.Millisecond)
    if err!=nil{t.Fatal(err)}
    ctx,cancel:=context.WithTimeout(context.Background(),time.Second)
    defer cancel()
    future,err:=transport.Submit(ctx,ref)
    if err!=nil{t.Fatal(err)}
    ack:=RAMTestAck(ref)
    if err:=WriteRAMCommitReceipt(root,ref,ack,gid);err!=nil{t.Fatal(err)}
    select{
    case got,ok:=<-future:
        if !ok||got.SHA256!=ack.SHA256||got.Outcome!="committed"{t.Fatalf("incorrect SQL receipt: %+v, ok=%t",got,ok)}
    case <-ctx.Done():t.Fatal("valid receipt not observed")
    }
    st,err:=os.Lstat(filepath.Join(root,ref.Name+".ack"))
    if err!=nil{t.Fatal(err)}
    if st.Mode().Perm()!=0640{t.Fatalf("receipt mode=%o; want 0640",st.Mode().Perm())}
    if err:=WriteRAMCommitReceipt(root,ref,ack,gid);err!=nil{t.Fatalf("idempotent same receipt rejected: %v",err)}
    wrong:=ack
    wrong.Outcome="already_committed"
    if err:=WriteRAMCommitReceipt(root,ref,wrong,gid);err==nil{t.Fatal("conflicting immutable ACK accepted")}
}
func TestRAMReceiptRejectForgedAndWrongIdentity(t *testing.T){
    gid:=os.Getgid()
    if gid<1{t.Skip("unit runner has no non-root shared group")}
    root:=t.TempDir()
    if err:=os.Chmod(root,0750);err!=nil{t.Fatal(err)}
    ref:=RAMTestRef()
    ack:=RAMTestAck(ref)
    ack.Name="pf-c1-"+strings.Repeat("f",32)+".ready"
    if err:=WriteRAMCommitReceipt(root,ref,ack,gid);err==nil{t.Fatal("wrong work identity accepted")}
    ack=RAMTestAck(ref)
    if err:=WriteRAMCommitReceipt(root,ref,ack,gid);err!=nil{t.Fatal(err)}
    bad:=ref
    bad.SHA256=strings.Repeat("c",64)
    if _,err:=ReadRAMCommitReceipt(root,bad,gid);err==nil{t.Fatal("wrong SHA accepted")}
    if _,err:=ReadRAMCommitReceipt(root,ref,gid+1);err==nil{t.Fatal("wrong shared group accepted")}
    if err:=os.Remove(filepath.Join(root,ref.Name+".ack"));err!=nil{t.Fatal(err)}
    if err:=os.Symlink("/etc/passwd",filepath.Join(root,ref.Name+".ack"));err!=nil{t.Fatal(err)}
    if _,err:=ReadRAMCommitReceipt(root,ref,gid);err==nil{t.Fatal("symlink receipt accepted")}
}
func TestRAMReceiptAbsentAndCanceled(t *testing.T){
    gid:=os.Getgid()
    if gid<1{t.Skip("unit runner has no non-root shared group")}
    root:=t.TempDir()
    if err:=os.Chmod(root,0750);err!=nil{t.Fatal(err)}
    ref:=RAMTestRef()
    if _,err:=ReadRAMCommitReceipt(root,ref,gid);!os.IsNotExist(err){
        t.Fatalf("absent receipt should be retryable, got %v",err)
    }
    ctx,cancel:=context.WithCancel(context.Background())
    transport,err:=NewRAMReceiptTransport(root,gid,time.Millisecond)
    if err!=nil{t.Fatal(err)}
    future,err:=transport.Submit(ctx,ref)
    if err!=nil{t.Fatal(err)}
    cancel()
    select{
    case _,ok:=<-future:
        if ok{t.Fatal("canceled pending SQL receipt was accepted")}
    case <-time.After(time.Second):t.Fatal("transport did not stop after cancellation")
    }
}
func TestRAMReceiptRejectUnsafeSourceSeparation(t *testing.T){
    root:=t.TempDir()
    if err:=os.Chmod(root,0750);err!=nil{t.Fatal(err)}
    if err:=ServeRAMWork(context.Background(),root,root,1,&SoRProcessor{},time.Millisecond);err==nil{
        t.Fatal("same read/write root must be rejected")
    }
}

type countedC1SoRSQL struct {
    SoRSQLExecutor
    inspections atomic.Int64
}
func(s *countedC1SoRSQL)Inspect(ctx context.Context,b sensorsor.BatchSQL)(SQLBatchState,error){
    s.inspections.Add(1)
    return s.SoRSQLExecutor.Inspect(ctx,b)
}

// A completed SoR worker must not be scheduled again while the producer
// reads its valid SQL ACK and removes its .ready object. Re-dispatch was
// observed on native C220 after four durable batch commits.
func TestServeRAMWorkRetiresAcknowledgedObjectUntilReclaimed(t *testing.T){
    gid:=os.Getgid()
    if gid<1{t.Skip("unit runner has no non-root shared group")}
    proc,ref,sql,bundle:=processorFixture(t)
    if err:=os.Chmod(proc.ReadRoot,0750);err!=nil{t.Fatal(err)}
    receipts:=t.TempDir()
    if err:=os.Chmod(receipts,0750);err!=nil{t.Fatal(err)}
    counted:=&countedC1SoRSQL{SoRSQLExecutor:sql}
    proc.SQL=counted
    ctx,cancel:=context.WithCancel(context.Background())
    done:=make(chan error,1)
    go func(){
        done<-ServeRAMWork(ctx,proc.ReadRoot,receipts,gid,proc,time.Millisecond)
    }()
    defer func(){
        cancel()
        select{
        case err:=<-done:
            if err!=nil{t.Errorf("unexpected SoR stop: %v",err)}
        case <-time.After(3*time.Second):t.Error("SoR did not stop on cancellation")
        }
    }()
    awaitReceipt:=func(ref BundleReference){
        t.Helper()
        deadline:=time.Now().Add(5*time.Second)
        for time.Now().Before(deadline){
            if _,err:=ReadRAMCommitReceipt(receipts,ref,gid);err==nil{return}
            select{
            case err:=<-done:t.Fatalf("SoR stopped before receipt: %v",err)
            default:
            }
            time.Sleep(5*time.Millisecond)
        }
        t.Fatal("missing SoR SQL receipt")
    }
    awaitReceipt(ref)
    time.Sleep(75*time.Millisecond) // allow completed work to retire
    before:=counted.inspections.Load()
    if before!=4{t.Fatalf("unexpected first bundle inspections: %d",before)}
    // Simulate receipt tmpfs loss while work is still visible. An already
    // finished worker must not spin up another SQL operation or forge a
    // replacement receipt; recovery requires source/SQL reconciliation.
    if err:=os.Remove(filepath.Join(receipts,ref.Name+".ack"));err!=nil{t.Fatal(err)}
    time.Sleep(75*time.Millisecond)
    if counted.inspections.Load()!=before{t.Fatal("completed work dispatched again after receipt vanished")}
    if _,err:=os.Stat(filepath.Join(receipts,ref.Name+".ack"));!os.IsNotExist(err){
        t.Fatalf("SoR silently reissued lost receipt, stat=%v",err)
    }
    if err:=os.Remove(filepath.Join(proc.ReadRoot,ref.Name));err!=nil{t.Fatal(err)}
    time.Sleep(20*time.Millisecond)
    // A different immutable object for the same original is legitimate
    // recovery and must be inspected against durable SQL, not retired.
    retry,err:=PublishBundleWithGroup(proc.ReadRoot,bundle,gid)
    if err!=nil{t.Fatal(err)}
    awaitReceipt(retry)
    recovered,err:=ReadRAMCommitReceipt(receipts,retry,gid)
    if err!=nil||recovered.ACK.Outcome!="already_committed"{
        t.Fatalf("same-original recovery rejected: %+v, %v",recovered,err)
    }
    if sql.writes!=2{t.Fatalf("recovery duplicated SQL commits: %d",sql.writes)}
}

// A directory listing is only a hint. A vanished work name is not an ACK,
// but it must not terminate an otherwise healthy SoR consumer.
func TestServeOneRAMBundleSkipsReclaimedWorkWithoutACK(t *testing.T){
    gid:=os.Getgid()
    if gid<1{t.Skip("unit runner has no non-root shared group")}
    proc,ref,_,_:=processorFixture(t)
    if err:=os.Chmod(proc.ReadRoot,0750);err!=nil{t.Fatal(err)}
    receipts:=t.TempDir()
    if err:=os.Chmod(receipts,0750);err!=nil{t.Fatal(err)}
    if err:=os.Remove(filepath.Join(proc.ReadRoot,ref.Name));err!=nil{t.Fatal(err)}
    if err:=serveOneRAMBundle(context.Background(),proc.ReadRoot,receipts,ref.Name,gid,proc);err!=nil{
        t.Fatalf("stale work entry stopped SoR: %v",err)
    }
    files,err:=os.ReadDir(receipts)
    if err!=nil||len(files)!=0{t.Fatal("missing work unexpectedly produced SQL receipt")}
}

func TestMissingRAMWorkIsNotMissingOriginalCustody(t *testing.T){
    proc,ref,_,_:=processorFixture(t)
    if err:=os.Remove(filepath.Join(proc.ReadRoot,ref.Name));err!=nil{t.Fatal(err)}
    if _,err:=proc.ProcessBundle(context.Background(),ref);!errors.Is(err,ErrVolatileRAMWorkGone){
        t.Fatalf("missing volatile work lacks distinct classification: %v",err)
    }
    // Restore valid work and deliberately remove independently verified
    // original custody. That must still terminate SoR without an ACK.
    original:=originalFor(sensoringest.Health)
    p,err:=Prepare(original.Input)
    if err!=nil{t.Fatal(err)}
    bundle:=Bundle{Format:BundleFormat,Stream:p.Stream,
        Batches:[]Prepared{p},SourceBytes:p.PayloadBytes,Records:p.Evidence.Count}
    ref2,err:=PublishBundle(proc.ReadRoot,bundle)
    if err!=nil{t.Fatal(err)}
    proc.Originals=testOriginalReader{fail:os.ErrNotExist}
    if _,err:=proc.ProcessBundle(context.Background(),ref2);err==nil||
        errors.Is(err,ErrVolatileRAMWorkGone){
        t.Fatalf("lost original custody was incorrectly treated as volatile race: %v",err)
    }
}
