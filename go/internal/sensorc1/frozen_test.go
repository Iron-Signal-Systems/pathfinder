package sensorc1

import (
    "compress/gzip"
    "crypto/sha256"
    "encoding/hex"
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

func TestFrozenChunkBoundaryKeepsOriginalSourceOrdinalAndBatchID(t *testing.T){
    root:=t.TempDir()
    dir:=filepath.Join(root,"health")
    if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
    originalRecord:=cases[sensoringest.Health].raw+"\n"
    file:=filepath.Join(dir,"health-boundary.jsonl.active")
    if err:=os.WriteFile(file,[]byte(strings.Repeat(originalRecord,2501)),0600);err!=nil{t.Fatal(err)}
    audit,err:=AuditFrozen(root,sensorID)
    if err!=nil{t.Fatal(err)}
    if audit.Files!=1||audit.Records!=2501||audit.ByStream[sensoringest.Health]!=2501{
        t.Fatalf("invalid complete audit at 2500-row boundary: %+v",audit)
    }
    var batches []Prepared
    summary,err:=ReplayFrozen(root,sensorID,sensorUUID,func(p Prepared)error{
        batches=append(batches,p)
        return nil
    })
    if err!=nil{t.Fatal(err)}
    if summary.Files!=1||summary.Batches!=2||summary.Records!=2501||len(batches)!=2{
        t.Fatalf("incorrect original chunking: %+v",summary)
    }
    if batches[0].Evidence.Count!=2500||batches[0].SegmentOrdinalStart!=1{
        t.Fatalf("first original chunk metadata wrong: %+v",batches[0])
    }
    if batches[1].Evidence.Count!=1||batches[1].SegmentOrdinalStart!=2501{
        t.Fatalf("second original chunk metadata wrong: %+v",batches[1])
    }
    for _,b:=range batches{
        want:=sensorsor.ID("frozen-import-batch-v1",sensorUUID,string(b.Stream),b.WireBatchID)
        if b.BatchUUID!=want{t.Fatalf("original frozen importer ID changed: %s",b.BatchUUID)}
        if err:=ValidatePrepared(b);err!=nil{t.Fatal(err)}
    }
    if batches[0].BatchUUID==batches[1].BatchUUID {
        t.Fatal("separate source chunks reused the same durable batch ID")
    }
}

func TestFrozenArchiveFingerprintDistinguishesSameCountChangedSource(t *testing.T){
 root:=t.TempDir()
 dir:=filepath.Join(root,"health")
 if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
 path:=filepath.Join(dir,"health-same-record-count.jsonl.active")
 original:=cases[sensoringest.Health].raw+"\n"
 if err:=os.WriteFile(path,[]byte(original),0600);err!=nil{t.Fatal(err)}
 baseline,err:=AuditFrozen(root,sensorID);if err!=nil{t.Fatal(err)}
 pinned,err:=FrozenArchiveFingerprint(baseline);if err!=nil{t.Fatal(err)}
 changed:=strings.ReplaceAll(original,"2026-11-20T10:00:00Z","2026-11-20T10:00:01Z")
 if len(changed)!=len(original){t.Fatal("fixture source size changed")}
 if err:=os.WriteFile(path,[]byte(changed),0600);err!=nil{t.Fatal(err)}
 newer,err:=AuditFrozen(root,sensorID);if err!=nil{t.Fatal(err)}
 if newer.Files!=baseline.Files||newer.Records!=baseline.Records{
  t.Fatal("fixture no longer has identical counts")
 }
 fingerprint,err:=FrozenArchiveFingerprint(newer)
 if err!=nil{t.Fatal(err)}
 if fingerprint==pinned{t.Fatal("changed source with same files/records accepted pinned hash")}
}
func TestFrozenArchiveFingerprintOrderIndependentAndStrict(t *testing.T){
 hash1:=strings.Repeat("a",64)
 hash2:=strings.Repeat("b",64)
 a:=FrozenSummary{StoredSHA256:map[string]string{"health/b.jsonl.gz":hash2,"identity/a.jsonl.gz":hash1}}
 b:=FrozenSummary{StoredSHA256:map[string]string{"identity/a.jsonl.gz":hash1,"health/b.jsonl.gz":hash2}}
 got,err:=FrozenArchiveFingerprint(a);if err!=nil{t.Fatal(err)}
 same,err:=FrozenArchiveFingerprint(b);if err!=nil{t.Fatal(err)}
 if got!=same{t.Fatal("archive fingerprint changed with map iteration order")}
 for _,invalid:=range []FrozenSummary{
  {},
  {StoredSHA256:map[string]string{"../escape.jsonl":hash1}},
  {StoredSHA256:map[string]string{"health/a.jsonl":"not-sha256"}},
  {StoredSHA256:map[string]string{"health/a.jsonl":strings.ToUpper(hash1)}},
 }{
  if _,err:=FrozenArchiveFingerprint(invalid);err==nil{
   t.Fatal("accepted unsafe/invalid archive manifest")
  }
 }
}

func TestFrozenArchiveFingerprintUsesCanonicalBinaryDelimiters(t *testing.T){
 digest:=strings.Repeat("a",64)
 summary:=FrozenSummary{StoredSHA256:map[string]string{"health/health-original.jsonl.gz":digest}}
 got,err:=FrozenArchiveFingerprint(summary)
 if err!=nil{t.Fatal(err)}
 // Deliberately independent expected bytes. A literal "\\x00" or "\\n"
 // MUST NOT be accepted as the canonical full-source fingerprint encoding.
 exact:=[]byte("health/health-original.jsonl.gz\x00"+digest+"\n")
 sha:=sha256.Sum256(exact)
 expected:=hex.EncodeToString(sha[:])
 if got!=expected{t.Fatalf("manifest delimiter encoding drift got=%s expected=%s",got,expected)}
}
