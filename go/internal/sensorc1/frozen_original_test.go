package sensorc1

import (
 "context"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

func TestIndependentFrozenSourceReaderNeverTrustsRAMBatchIdentity(t *testing.T){
 root:=t.TempDir()
 source:=filepath.Join(root,"observations-20261120.jsonl.active")
 raw:=[]byte(cases[sensoringest.Observations].raw+"\n")
 if err:=os.WriteFile(source,raw,0600);err!=nil{t.Fatal(err)}
 reader,err:=NewFrozenOriginalReader(root,sensorID,sensorUUID)
 if err!=nil{t.Fatal(err)}
 var prepared Prepared
 _,err=ReplayFrozen(root,sensorID,sensorUUID,func(p Prepared)error{
  prepared=p
  return nil
 })
 if err!=nil{t.Fatal(err)}
 original,err:=reader.OpenOriginal(context.Background(),prepared)
 if err!=nil{t.Fatal(err)}
 if original.Input.BatchUUID!=prepared.BatchUUID ||
    original.SegmentSHA256=="" ||
    original.Input.SegmentOrdinalStart==0 || !strings.HasPrefix(original.BatchCustodyReference,"frozen-replay:") {
  t.Fatalf("missing independently sourced original evidence: %+v",original)
 }
 if string(original.Input.Payload)!=string(raw){t.Fatal("original bytes changed")}
 // A forged unknown batch UUID is not an authority to read arbitrary files.
 forged:=prepared
 forged.BatchUUID="11111111-2222-4333-8444-111111111111"
 if _,err:=reader.OpenOriginal(context.Background(),forged);err==nil{
  t.Fatal("unregistered batch accepted as original custody")
 }
 // Cached bytes cannot be changed through the returned snapshot.
 original.Input.Payload[0]='?'
 again,err:=reader.OpenOriginal(context.Background(),prepared)
 if err!=nil{t.Fatal(err)}
 if string(again.Input.Payload)!=string(raw){
  t.Fatal("reader exposed mutable source-owned memory")
 }
 if err:=os.WriteFile(source,append(raw,raw...),0600);err!=nil{t.Fatal(err)}
 if _,err:=reader.OpenOriginal(context.Background(),prepared);err==nil{
  t.Fatal("changed archived source accepted after original audit")
 }
}

func TestFrozenSourceReaderRejectsMissingOrModifiedEvidence(t *testing.T){
 root:=t.TempDir()
 source:=filepath.Join(root,"observations-1.jsonl.active")
 if err:=os.WriteFile(source,[]byte("{bad json}\n"),0600);err!=nil{t.Fatal(err)}
 if _,err:=NewFrozenOriginalReader(root,sensorID,sensorUUID);err==nil{
  t.Fatal("malformed archive accepted")
 }
}

func TestFrozenOriginalCacheHasExplicitFailClosedPayloadBudget(t *testing.T){
 root:=t.TempDir()
 source:=filepath.Join(root,"observations-20261122.jsonl.active")
 raw:=[]byte(cases[sensoringest.Observations].raw+"\n")
 if err:=os.WriteFile(source,raw,0600);err!=nil{t.Fatal(err)}
 if _,err:=NewFrozenOriginalReaderWithLimit(root,sensorID,sensorUUID,0);err==nil{
  t.Fatal("unbounded or zero source-cache allowance accepted")
 }
 if _,err:=NewFrozenOriginalReaderWithLimit(root,sensorID,sensorUUID,DefaultFrozenCacheBytes+1);err==nil{
  t.Fatal("source-cache allowance above audited upper bound accepted")
 }
 if _,err:=NewFrozenOriginalReaderWithLimit(root,sensorID,sensorUUID,int64(len(raw)-1));err==nil||
    !strings.Contains(err.Error(),"cache budget exceeded") {
  t.Fatalf("truncated original source cache did not stop: %v",err)
 }
 reader,err:=NewFrozenOriginalReaderWithLimit(root,sensorID,sensorUUID,int64(len(raw)))
 if err!=nil{t.Fatal(err)}
 batches,bytes,max:=reader.CachedSourceStats()
 if batches!=1||bytes!=int64(len(raw))||max!=int64(len(raw)){
  t.Fatalf("cache retained unexpected evidence batches=%d bytes=%d limit=%d",batches,bytes,max)
 }
}
