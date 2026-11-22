package sensorc1

import (
    "context"
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
