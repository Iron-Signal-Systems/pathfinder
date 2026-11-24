package sensorc1

import (
    "os"
    "path/filepath"
    "testing"
    "syscall"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

func TestFamilyBundleRAMPreserveIdentity(t *testing.T){
    root:=t.TempDir()
    if err:=os.Chmod(root,0700);err!=nil{t.Fatal(err)}
    one,err:=Prepare(inputFor(sensoringest.Observations));if err!=nil{t.Fatal(err)}
    two:=secondPrepared(t,sensoringest.Observations)
    bundle:=Bundle{Format:BundleFormat,Stream:sensoringest.Observations,
        Batches:[]Prepared{one,two},SourceBytes:one.PayloadBytes+two.PayloadBytes,
        Records:one.Evidence.Count+two.Evidence.Count}
    ref,err:=PublishBundle(root,bundle);if err!=nil{t.Fatal(err)}
    if ref.Batches!=2||ref.Records!=2{t.Fatalf("bad reference %+v",ref)}
    got,err:=ReadBundle(root,ref);if err!=nil{t.Fatal(err)}
    if got.Batches[0].BatchUUID!=one.BatchUUID||got.Batches[1].BatchUUID!=two.BatchUUID{
        t.Fatal("source batches were merged or reordered")
    }
    for _,p:=range got.Batches{
        if len(p.Evidence.Tables["sensor_dns_question"].Rows)!=1 ||
           len(p.Evidence.Tables["sensor_dns_record"].Rows)!=1{
            t.Fatal("source observation DNS child evidence not retained")
        }
    }
    // RAM publication/reading is not SQL COMMIT and must not reclaim files.
    if _,err:=os.Stat(filepath.Join(root,ref.Name));err!=nil{t.Fatal(err)}
    bad:=ref
    bad.Name="../outside"
    if _,err:=ReadBundle(root,bad);err==nil{t.Fatal("path traversal accepted")}
    bad=ref
    bad.SHA256="ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
    if _,err:=ReadBundle(root,bad);err==nil{t.Fatal("checksum mismatch accepted")}
    bad=ref
    bad.Records=3
    if _,err:=ReadBundle(root,bad);err==nil{t.Fatal("record mismatch accepted")}
}

func TestFamilyBundleRejectsConflicts(t *testing.T){
    a,err:=Prepare(inputFor(sensoringest.Conntrack));if err!=nil{t.Fatal(err)}
    b,err:=Prepare(inputFor(sensoringest.Health));if err!=nil{t.Fatal(err)}
    for name,bundle:=range map[string]Bundle{
        "duplicate original batch":{Format:BundleFormat,Stream:sensoringest.Conntrack,Batches:[]Prepared{a,a},SourceBytes:2*a.PayloadBytes,Records:2},
        "mixed family":{Format:BundleFormat,Stream:sensoringest.Conntrack,Batches:[]Prepared{a,b},SourceBytes:a.PayloadBytes+b.PayloadBytes,Records:2},
        "incorrect row count":{Format:BundleFormat,Stream:sensoringest.Conntrack,Batches:[]Prepared{a},SourceBytes:a.PayloadBytes,Records:2},
    }{
        t.Run(name,func(t *testing.T){
            if err:=bundle.Validate();err==nil{t.Fatal("invalid bundle accepted")}
        })
    }
}

func TestFreeBSDStyleSharedRAMGroupPublication(t *testing.T){
    root:=t.TempDir()
    if err:=os.Chmod(root,0750);err!=nil{t.Fatal(err)}
    p,err:=Prepare(inputFor(sensoringest.Health))
    if err!=nil{t.Fatal(err)}
    b:=Bundle{Format:BundleFormat,Stream:p.Stream,Batches:[]Prepared{p},
        SourceBytes:p.PayloadBytes,Records:p.Evidence.Count}
    if _,err:=PublishBundleWithGroup(root,b,0);err==nil{
        t.Fatal("accepted root/wheel as shared RAM GID")
    }
    if _,err:=NewDispatcherForJails(root,nil,0);err==nil{
        t.Fatal("accepted missing dedicated shared RAM group")
    }
    gid:=os.Getgid()
    if gid<1{t.Skip("CI is root group: no safe non-root shared GID to exercise")}
    ref,err:=PublishBundleWithGroup(root,b,gid)
    if err!=nil{t.Fatal(err)}
    st,err:=os.Stat(filepath.Join(root,ref.Name))
    if err!=nil{t.Fatal(err)}
    if st.Mode().Perm()!=0640{t.Fatalf("group-only RAM object permissions: %04o",st.Mode().Perm())}
    native,ok:=st.Sys().(*syscall.Stat_t)
    if !ok{t.Fatal("no native Unix file group metadata")}
    if int(native.Gid)!=gid{t.Fatalf("published file GID=%d want=%d",native.Gid,gid)}
    got,err:=ReadBundle(root,ref)
    if err!=nil{t.Fatal(err)}
    if len(got.Batches)!=1||got.Batches[0].BatchUUID!=p.BatchUUID{
        t.Fatal("shared-group publication lost source identity")
    }
}
