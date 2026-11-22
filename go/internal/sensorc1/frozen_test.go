package sensorc1

import (
    "compress/gzip"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

func TestFrozenReplayPreservesImportedIDsAndCounts(t *testing.T){
    root:=t.TempDir()
    files:=map[string]sensoringest.Stream{
        "observations-1.jsonl.active":sensoringest.Observations,
        "conntrack/conntrack-1.jsonl.gz":sensoringest.Conntrack,
        "identity/endpoint-identity-1.jsonl.gz":sensoringest.Identity,
        "health/health-1.jsonl.active":sensoringest.Health,
    }
    for rel,stream:=range files{
        if err:=os.MkdirAll(filepath.Dir(filepath.Join(root,rel)),0700);err!=nil{t.Fatal(err)}
        f,err:=os.Create(filepath.Join(root,rel));if err!=nil{t.Fatal(err)}
        var input interface{Write([]byte)(int,error)}=f
        var g *gzip.Writer
        if strings.HasSuffix(rel,".gz"){g=gzip.NewWriter(f);input=g}
        raw:=[]byte(cases[stream].raw+"\n")
        if _,err:=input.Write(raw);err!=nil{t.Fatal(err)}
        if g!=nil{if err:=g.Close();err!=nil{t.Fatal(err)}}
        if err:=f.Close();err!=nil{t.Fatal(err)}
    }
    if err:=os.WriteFile(filepath.Join(root,"file-manifest.txt"),[]byte("metadata"),0600);err!=nil{t.Fatal(err)}
    audit,err:=AuditFrozen(root,sensorID)
    if err!=nil{t.Fatal(err)}
    if audit.Files!=4||audit.Records!=4||len(audit.StoredSHA256)!=4 {
        t.Fatalf("incorrect full source preflight before SQL: %+v",audit)
    }
    count:=0
    observed:=map[sensoringest.Stream]int{}
    summary,err:=ReplayFrozen(root,sensorID,sensorUUID,func(p Prepared)error{
        count++
        observed[p.Stream]++
        if err:=ValidatePrepared(p);err!=nil{return err}
        want:=sensorsor.ID("frozen-import-batch-v1",sensorUUID,string(p.Stream),p.WireBatchID)
        if p.BatchUUID!=want{t.Fatalf("source batch UUID differs from existing historical importer")}
        return nil
    })
    if err!=nil{t.Fatal(err)}
    for sourceName,sourceSHA:=range audit.StoredSHA256{
        if summary.StoredSHA256[sourceName]!=sourceSHA {
            t.Fatalf("archive source changed between preflight and playback: %s",sourceName)
        }
    }
    if summary.Files!=4 || summary.Records!=4 || summary.Batches!=4 || count!=4 || len(summary.StoredSHA256)!=4 {
        t.Fatalf("invalid archive summary %+v",summary)
    }
    for stream:=range cases{
        if observed[stream]!=1 || summary.ByStream[stream]!=1 {t.Fatalf("missing stream %q: %+v",stream,summary)}
    }
}
func TestFrozenReplayRejectsUnexpectedSensorFiles(t *testing.T){
    root:=t.TempDir()
    if err:=os.WriteFile(filepath.Join(root,"observations-1.jsonl"),[]byte("discard"),0600);err!=nil{t.Fatal(err)}
    if _,err:=ReplayFrozen(root,sensorID,sensorUUID,func(Prepared)error{return nil});err==nil {
        t.Fatal("unsupported evidence file silently skipped")
    }
}
func TestFrozenReplayPropagatesConsumerFailure(t *testing.T){
    root:=t.TempDir()
    if err:=os.WriteFile(filepath.Join(root,"observations-1.jsonl.active"),
        []byte(cases[sensoringest.Observations].raw+"\n"),0600);err!=nil{t.Fatal(err)}
    expected:=os.ErrPermission
    _,err:=ReplayFrozen(root,sensorID,sensorUUID,func(Prepared)error{return expected})
    if err==nil||!strings.Contains(err.Error(),expected.Error()){t.Fatalf("consumer failure ignored: %v",err)}
}

func TestFrozenPreflightRejectsMissingInvalidOrEmptySources(t *testing.T){
    root:=t.TempDir()
    if _,err:=AuditFrozen(root,sensorID);err==nil{t.Fatal("accepted missing original archive")}
    source:=filepath.Join(root,"observations-1.jsonl.active")
    if err:=os.WriteFile(source,[]byte(""),0600);err!=nil{t.Fatal(err)}
    if _,err:=AuditFrozen(root,sensorID);err==nil{t.Fatal("accepted empty original source")}
    if err:=os.WriteFile(source,[]byte("{bad json}\n"),0600);err!=nil{t.Fatal(err)}
    if _,err:=AuditFrozen(root,sensorID);err==nil{t.Fatal("accepted malformed original source")}
}
