package sensorc1

import (
    "context"
    "errors"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

type futureTransport struct {
    future chan SQLCommitAck
    last BundleReference
    fail error
}
func(t *futureTransport)Submit(_ context.Context,r BundleReference)(<-chan SQLCommitAck,error){
    t.last=r
    if t.fail!=nil{return nil,t.fail}
    return t.future,nil
}

func oneBundle(t *testing.T)Bundle{
    t.Helper()
    p,err:=Prepare(inputFor(sensoringest.Observations))
    if err!=nil{t.Fatal(err)}
    return Bundle{Format:BundleFormat,Stream:p.Stream,
        Batches:[]Prepared{p},SourceBytes:p.PayloadBytes,Records:p.Evidence.Count}
}
func testAck(ref BundleReference,b Bundle)SQLCommitAck{
    ids:=make([]string,len(b.Batches))
    for i,p:=range b.Batches{ids[i]=p.BatchUUID}
    return SQLCommitAck{Name:ref.Name,SHA256:ref.SHA256,Stream:ref.Stream,
        Records:ref.Records,BatchUUIDs:ids,Outcome:"committed"}
}
func TestRAMNotReleasedBeforeExactSQLCommit(t *testing.T){
    dir:=t.TempDir()
    if err:=os.Chmod(dir,0700);err!=nil{t.Fatal(err)}
    transport:=&futureTransport{future:make(chan SQLCommitAck,1)}
    disp,err:=NewDispatcher(dir,transport);if err!=nil{t.Fatal(err)}
    b:=oneBundle(t)
    if err:=disp.Submit(context.Background(),b);err!=nil{t.Fatal(err)}
    path:=filepath.Join(dir,transport.last.Name)
    if _,err:=os.Stat(path);err!=nil{t.Fatal("RAM disappeared before ACK")}
    ctx,cancel:=context.WithTimeout(context.Background(),15*time.Millisecond)
    defer cancel()
    if err:=disp.Wait(ctx);err==nil{t.Fatal("Wait claimed SQL commit without ACK")}
    if _,err:=os.Stat(path);err!=nil{t.Fatal("timeout reclaimed uncommitted RAM")}
    transport.future<-testAck(transport.last,b)
    if err:=disp.Wait(context.Background());err!=nil{t.Fatal(err)}
    if _,err:=os.Stat(path);!errors.Is(err,os.ErrNotExist){
        t.Fatalf("committed RAM not reclaimed: %v",err)
    }
}
func TestWrongCommitAckRetainsRamAndBlocksNext(t *testing.T){
    dir:=t.TempDir()
    os.Chmod(dir,0700)
    transport:=&futureTransport{future:make(chan SQLCommitAck,1)}
    disp,err:=NewDispatcher(dir,transport);if err!=nil{t.Fatal(err)}
    b:=oneBundle(t)
    if err:=disp.Submit(context.Background(),b);err!=nil{t.Fatal(err)}
    ref:=transport.last
    bad:=testAck(ref,b)
    bad.BatchUUIDs=[]string{"forged-source-batch"}
    transport.future<-bad
    if err:=disp.Wait(context.Background());err==nil{t.Fatal("invalid SQL ACK accepted")}
    if _,err:=os.Stat(filepath.Join(dir,ref.Name));err!=nil{t.Fatal("uncertain RAM removed")}
    if err:=disp.Submit(context.Background(),b);err==nil{t.Fatal("accepted new dispatch after uncertain SQL")}
}
func TestClosedCommitChannelRetainsRAM(t *testing.T){
    dir:=t.TempDir()
    os.Chmod(dir,0700)
    transport:=&futureTransport{future:make(chan SQLCommitAck)}
    disp,err:=NewDispatcher(dir,transport);if err!=nil{t.Fatal(err)}
    b:=oneBundle(t)
    if err:=disp.Submit(context.Background(),b);err!=nil{t.Fatal(err)}
    path:=filepath.Join(dir,transport.last.Name)
    close(transport.future)
    if err:=disp.Wait(context.Background());err==nil{t.Fatal("missing commit ACK accepted")}
    if _,err:=os.Stat(path);err!=nil{t.Fatal("RAM removed on missing ACK")}
}
func TestSixteenCreditBound(t *testing.T){
    dir:=t.TempDir()
    os.Chmod(dir,0700)
    transport:=&futureTransport{future:make(chan SQLCommitAck)}
    disp,err:=NewDispatcher(dir,transport);if err!=nil{t.Fatal(err)}
    b:=oneBundle(t)
    ctx,cancel:=context.WithCancel(context.Background())
    defer cancel()
    for i:=0;i<MaxOutstandingHandoffs;i++{
        if err:=disp.Submit(ctx,b);err!=nil{t.Fatal(err)}
    }
    timeout,stop:=context.WithTimeout(ctx,20*time.Millisecond)
    defer stop()
    if err:=disp.Submit(timeout,b);!errors.Is(err,context.DeadlineExceeded){
        t.Fatalf("seventeenth RAM handoff bypassed credit cap: %v",err)
    }
    cancel()
    if err:=disp.Wait(context.Background());err==nil{t.Fatal("canceled uncommitted SQL reported success")}
}
