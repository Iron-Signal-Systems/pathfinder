//go:build pathfinder_pg18_integration

package sensorc1

import (
 "context"
 "fmt"
 "os"
 "strings"
 "sync"
 "testing"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// Two or more authenticated clients can retry the same original batch using
// different RAM object names and control nonces. The SoR must read the
// original source independently, commit once and acknowledge every exact
// replay, WITHOUT involving B verification in the critical ACK path.
func TestPG18ConcurrentAuthenticatedSameSourceExactlyOneSQLCommit(t *testing.T){
 pg:=pg18RecoveryExecutor(t)
 original:=originalFor(sensoringest.Observations)
 original.Input.WireBatchID="pg18-same-source-parallel-v1"
 original.Input.BatchUUID=sensorsor.ID("frozen-import-batch-v1",
  original.Input.SensorUUID,string(original.Input.Stream),original.Input.WireBatchID)
 p,err:=Prepare(original.Input)
 if err!=nil{t.Fatal(err)}
 ram:=t.TempDir()
 if err:=os.Chmod(ram,0700);err!=nil{t.Fatal(err)}
 processor:=&SoRProcessor{
  ReadRoot:ram,SQL:pg,
  Originals:testOriginalReader{values:map[string]OriginalBatch{p.BatchUUID:original}},
 }
 pki:=makeControlTestPKI(t)
 server:=makeControlTestServer(t,processor,pki)
 transport,err:=NewC1HTTPTransport(server.Listener.Addr().String(),"pathfinder-sor.internal",pki.client)
 if err!=nil{t.Fatal(err)}
 bundle:=Bundle{Format:BundleFormat,Stream:p.Stream,
  Batches:[]Prepared{p},SourceBytes:p.PayloadBytes,Records:p.Evidence.Count}
 refs:=make([]BundleReference,8)
 for i:=range refs{
  refs[i],err=PublishBundle(ram,bundle)
  if err!=nil{t.Fatal(err)}
 }
 start:=make(chan struct{})
 var wg sync.WaitGroup
 results:=make(chan string,len(refs))
 failures:=make(chan error,len(refs))
 for _,ref:=range refs{
  wg.Add(1)
  go func(ref BundleReference){
   defer wg.Done()
   <-start
   future,err:=transport.Submit(context.Background(),ref)
   if err!=nil{failures<-err;return}
   ack,ok:=<-future
   if !ok{failures<-fmt.Errorf("missing exact SQL ACK for %s",ref.Name);return}
   if err:=validateCommitAck(ref,[]string{p.BatchUUID},ack);err!=nil{
    failures<-err;return
   }
   results<-ack.Outcome
  }(ref)
 }
 close(start)
 wg.Wait()
 close(results);close(failures)
 for err:=range failures{t.Errorf("parallel mTLS SQL handoff: %v",err)}
 committed,replayed:=0,0
 for outcome:=range results{
  if outcome=="committed"{committed++}else if outcome=="already_committed"{replayed++}else{
   t.Errorf("unrecognized outcome %s",outcome)
  }
 }
 if committed!=1||replayed!=len(refs)-1{
  t.Errorf("concurrent commit/replay count incorrect committed=%d replayed=%d",committed,replayed)
 }
 plan,err:=BuildSoRBatch(p,original)
 if err!=nil{t.Fatal(err)}
 state,err:=pg.Inspect(context.Background(),plan)
 if err!=nil||state!=SQLComplete{
  t.Fatalf("parallel source not committed: state=%s err=%v",state,err)
 }
 if err:=pg.VerifyComplete(context.Background(),plan);err!=nil{t.Fatal(err)}
 completion:=pgAdminIntegration(t,fmt.Sprintf(
  "SELECT count(*) FROM pathfinder.sensor_batch_index_completion WHERE batch_id='%s'::uuid;",p.BatchUUID))
 if strings.TrimSpace(completion)!="1"{
  t.Fatalf("concurrent retries produced %s durable SQL completion records",completion)
 }
}
