package sensorc1

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

func TestRAMHandoffRoundTripAndTamper(t *testing.T) {
    root:=t.TempDir()
    if err:=os.Chmod(root,0700);err!=nil{t.Fatal(err)}
    p,err:=Prepare(inputFor(sensoringest.Observations))
    if err!=nil{t.Fatal(err)}
    r,err:=Publish(root,p);if err!=nil{t.Fatal(err)}
    if r.RecordCount!=1 || r.BatchUUID!=p.BatchUUID {t.Fatalf("wrong handoff reference: %+v",r)}
    got,err:=Read(root,r);if err!=nil{t.Fatal(err)}
    if got.PayloadSHA256!=p.PayloadSHA256 || len(got.Evidence.Tables["sensor_dns_question"].Rows)!=1 {
        t.Fatal("roundtrip lost original source or DNS children")
    }
    // Read never consumes custody or the volatile object. Only a matching
    // durable SQL COMMIT ACK can authorize future producer-side deletion.
    if _,err:=os.Stat(filepath.Join(root,r.Name));err!=nil{t.Fatal(err)}
    t.Run("wrong digest",func(t *testing.T){
        bad:=r;bad.SHA256="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        if _,err:=Read(root,bad);err==nil{t.Fatal("wrong checksum accepted")}
    })
    t.Run("path traversal",func(t *testing.T){
        bad:=r;bad.Name="../outside"
        if _,err:=Read(root,bad);err==nil{t.Fatal("unsafe reference accepted")}
    })
    t.Run("changed bytes",func(t *testing.T){
        f,err:=os.OpenFile(filepath.Join(root,r.Name),os.O_WRONLY|os.O_APPEND,0)
        if err!=nil{t.Fatal(err)}
        _,err=f.Write([]byte("x"));f.Close();if err!=nil{t.Fatal(err)}
        if _,err:=Read(root,r);err==nil{t.Fatal("changed object accepted")}
    })
}

func TestHandoffRequiresPrivateDirectory(t *testing.T) {
    root:=t.TempDir()
    if err:=os.Chmod(root,0777);err!=nil{t.Fatal(err)}
    p,err:=Prepare(inputFor(sensoringest.Health));if err!=nil{t.Fatal(err)}
    if _,err:=Publish(root,p);err==nil{t.Fatal("accepted world-writeable handoff")}
}
