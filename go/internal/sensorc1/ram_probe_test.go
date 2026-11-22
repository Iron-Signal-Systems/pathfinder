package sensorc1

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestRAMProbeExplicitGroupAndExactConsumerRead(t *testing.T){
 root:=t.TempDir()
 if err:=os.Chmod(root,0750);err!=nil{t.Fatal(err)}
 if _,err:=PublishRAMProbe(root,0);err==nil{
  t.Fatal("accepted shared root group")
 }
 gid:=os.Getgid()
 if gid<1{t.Skip("test runner uses group zero")}
 ref,err:=PublishRAMProbe(root,gid)
 if err!=nil{t.Fatal(err)}
 if err:=VerifyRAMProbeSharedGID(root,ref,gid);err!=nil{t.Fatal(err)}
 bad:=ref
 bad.SHA256=strings.Repeat("a",64)
 if err:=ReadRAMProbe(root,bad);err==nil{t.Fatal("accepted false source digest")}
 bad=ref
 bad.Name="../other"
 if err:=ReadRAMProbe(root,bad);err==nil{t.Fatal("accepted path escape")}
 if err:=RemoveRAMProbe(root,bad,gid);err==nil{t.Fatal("removed unverified probe")}
 if err:=ReadRAMProbe(root,ref);err!=nil{t.Fatal("probe was prematurely removed")}
 if err:=RemoveRAMProbe(root,ref,gid);err!=nil{t.Fatal(err)}
 if _,err:=os.Stat(filepath.Join(root,ref.Name));!os.IsNotExist(err){
  t.Fatalf("producer cleanup did not remove probe: %v",err)
 }
}

func TestRAMProbeRejectsTampering(t *testing.T){
 root:=t.TempDir()
 if err:=os.Chmod(root,0750);err!=nil{t.Fatal(err)}
 gid:=os.Getgid()
 if gid<1{t.Skip("test runner uses group zero")}
 ref,err:=PublishRAMProbe(root,gid)
 if err!=nil{t.Fatal(err)}
 filename:=filepath.Join(root,ref.Name)
 f,err:=os.OpenFile(filename,os.O_WRONLY,0)
 if err!=nil{t.Fatal(err)}
 if _,err:=f.Write([]byte("changed"));err!=nil{t.Fatal(err)}
 if err:=f.Close();err!=nil{t.Fatal(err)}
 if err:=ReadRAMProbe(root,ref);err==nil{
  t.Fatal("consumer accepted tampered RAM probe")
 }
 if err:=RemoveRAMProbe(root,ref,gid);err==nil{
  t.Fatal("producer deleted a probe despite digest mismatch")
 }
}
