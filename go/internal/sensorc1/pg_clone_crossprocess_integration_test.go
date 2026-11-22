//go:build pathfinder_pg18_integration

package sensorc1

import (
 "context"
 "errors"
 "fmt"
 "os"
 "sync"
 "strings"
 "testing"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// Each collisionProcessSQL belongs to an independently allocated SoR
// controller. Their only cross-process coordination is the PostgreSQL
// advisory transaction lock within sensorsor.BatchSQL.SQLScript().
type collisionProcessSQL struct {
 *PGCloneExecutor
 arrived chan<- struct{}
 release <-chan struct{}
}
func(p *collisionProcessSQL)Execute(ctx context.Context,script []byte)error{
 select{
 case p.arrived<-struct{}{}:
 case <-ctx.Done():return ctx.Err()
 }
 select{
 case <-p.release:
 case <-ctx.Done():return ctx.Err()
 }
 return p.PGCloneExecutor.Execute(ctx,script)
}

// This uses actual PostgreSQL 18 transactions with TWO distinct SoR
// controllers, two independent TLS listeners, separate connections,
// different RAM object references and different request nonces. It forces
// both to observe 'new' before either can write. One SQLScript should commit,
// the other must rediscover that exact durable completion after its duplicate
// transaction fails. Neither may return a false commit acknowledgment.
func TestPG18TwoIndependentSoRControllersRecoverConcurrentSQLCollision(t *testing.T){
 ctx,cancel:=context.WithTimeout(context.Background(),50*time.Second)
 defer cancel()
 pg:=pg18RecoveryExecutor(t)
 original:=originalFor(sensoringest.Health)
 original.Input.WireBatchID="pg18-cross-process-advisory-collision-v1"
 original.Input.BatchUUID=sensorsor.ID("frozen-import-batch-v1",
  original.Input.SensorUUID,string(original.Input.Stream),original.Input.WireBatchID)
 prepared,err:=Prepare(original.Input)
 if err!=nil{t.Fatal(err)}
 ram:=t.TempDir()
 if err:=os.Chmod(ram,0700);err!=nil{t.Fatal(err)}
 bundle:=Bundle{Format:BundleFormat,Stream:prepared.Stream,
  Batches:[]Prepared{prepared},SourceBytes:prepared.PayloadBytes,Records:prepared.Evidence.Count}
 refs:=make([]BundleReference,2)
 for i:=range refs {
  refs[i],err=PublishBundle(ram,bundle)
  if err!=nil{t.Fatal(err)}
 }
 arrived:=make(chan struct{},2)
 release:=make(chan struct{})
 pki:=makeControlTestPKI(t)
 transports:=make([]*C1HTTPTransport,2)
 for i:=range transports {
  // Separate service objects (not shared Go locks); each opens independent
  // PostgreSQL psql sessions against the SAME disposable PostgreSQL 18 clone.
  sql:=&collisionProcessSQL{PGCloneExecutor:pg,arrived:arrived,release:release}
  controller:=&SoRProcessor{ReadRoot:ram,SQL:sql,
   Originals:testOriginalReader{values:map[string]OriginalBatch{prepared.BatchUUID:original}}}
  server:=makeControlTestServer(t,controller,pki)
  transports[i],err=NewC1HTTPTransport(server.Listener.Addr().String(),
   "pathfinder-sor.internal",pki.client)
  if err!=nil{t.Fatal(err)}
 }
 type outcome struct{ack SQLCommitAck;err error;ok bool;ref BundleReference}
 answers:=make(chan outcome,2)
 var wg sync.WaitGroup
 for i,ref:=range refs {
  wg.Add(1)
  go func(transport *C1HTTPTransport,ref BundleReference){
   defer wg.Done()
   future,err:=transport.Submit(ctx,ref)
   if err!=nil{answers<-outcome{err:err,ref:ref};return}
   select{
   case ack,ok:=<-future:answers<-outcome{ack:ack,ok:ok,ref:ref}
   case <-ctx.Done():answers<-outcome{err:ctx.Err(),ref:ref}
   }
  }(transports[i],ref)
 }
 // Force both clients to finish independent Inspect(new) before either SQL
 // transaction gets a chance to run. Do not let an unexpected deadlock
 // hang GitHub Actions indefinitely.
 for i:=0;i<2;i++ {
  select{
  case <-arrived:
  case <-ctx.Done():
   close(release)
   wg.Wait()
   t.Fatalf("two independent SoR controllers did not both reach SQL Execute: %v",ctx.Err())
  }
 }
 close(release)
 wg.Wait()
 close(answers)
 committed,replayed:=0,0
 for result:=range answers {
  if result.err!=nil||!result.ok{
   t.Errorf("independent SoR lost SQL ACK: %v (stream closed=%v)",result.err,!result.ok)
   continue
  }
  if err:=validateCommitAck(result.ref,[]string{prepared.BatchUUID},result.ack);err!=nil{
   t.Error(err)
   continue
  }
  switch result.ack.Outcome{
  case "committed":committed++
  case "already_committed":replayed++
  default:t.Errorf("invalid cross-process SQL outcome %q",result.ack.Outcome)
  }
 }
 if committed!=1||replayed!=1{
  t.Errorf("cross-process SQL collision not resolved: new=%d replay=%d",committed,replayed)
 }
 plan,err:=BuildSoRBatch(prepared,original)
 if err!=nil{t.Fatal(err)}
 state,err:=pg.Inspect(ctx,plan)
 if err!=nil||state!=SQLComplete{
  t.Fatalf("original batch not durably completed after collision: %v %v",state,err)
 }
 if err:=pg.VerifyComplete(ctx,plan);err!=nil{t.Fatal(err)}
 count:=pgAdminIntegration(t,fmt.Sprintf(
  "SELECT count(*) FROM pathfinder.sensor_batch_index_completion WHERE batch_id='%s'::uuid;",
  prepared.BatchUUID))
 if count!="1"{t.Fatalf("duplicate SQL completion rows after two-process collision: %s",count)}
 if errors.Is(ctx.Err(),context.DeadlineExceeded){t.Fatal("SQL collision exceeded deadline")}
}


// Same durable batch key with DIFFERENT original bytes must NOT be interpreted
// as an idempotent replay, even if two independent controllers enter their SQL
// transactions concurrently. The winning original is immutable; the
// conflicting source gets no SQL commit acknowledgment.
func TestPG18TwoIndependentSoRControllersRejectConflictingOriginals(t *testing.T){
 ctx,cancel:=context.WithTimeout(context.Background(),50*time.Second)
 defer cancel()
 pg:=pg18RecoveryExecutor(t)
 first:=originalFor(sensoringest.Health)
 first.Input.WireBatchID="pg18-cross-process-conflict-v1"
 first.Input.BatchUUID=sensorsor.ID("frozen-import-batch-v1",
  first.Input.SensorUUID,string(first.Input.Stream),first.Input.WireBatchID)
 second:=first
 second.Input=first.Input
 second.Input.Payload=[]byte(strings.Replace(
  string(first.Input.Payload),
  "\"expected_interval_seconds\":60","\"expected_interval_seconds\":61",1))
 if string(second.Input.Payload)==string(first.Input.Payload) {
  t.Fatal("conflicting source fixture did not change original bytes")
 }
 second.SegmentSHA256=strings.Repeat("b",64)
 second.SegmentUUID=sensorsor.ID("frozen-import-segment-v1",
  second.Input.SensorUUID,string(second.Input.Stream),second.SegmentSHA256)
 second.SegmentFilename="health-conflicting-source.jsonl.active"
 second.SegmentReference="/readonly/frozen/health-conflicting-source.jsonl.active"
 second.BatchCustodyReference="/readonly/custody/conflicting-source.json"
 inputs:=[]OriginalBatch{first,second}
 ram:=t.TempDir()
 if err:=os.Chmod(ram,0700);err!=nil{t.Fatal(err)}
 arrived:=make(chan struct{},2)
 release:=make(chan struct{})
 pki:=makeControlTestPKI(t)
 refs:=make([]BundleReference,2)
 transports:=make([]*C1HTTPTransport,2)
 plans:=make([]sensorsor.BatchSQL,2)
 for i,original:=range inputs{
  prepared,err:=Prepare(original.Input)
  if err!=nil{t.Fatal(err)}
  plans[i],err=BuildSoRBatch(prepared,original)
  if err!=nil{t.Fatal(err)}
  bundle:=Bundle{Format:BundleFormat,Stream:prepared.Stream,
   Batches:[]Prepared{prepared},SourceBytes:prepared.PayloadBytes,Records:prepared.Evidence.Count}
  refs[i],err=PublishBundle(ram,bundle)
  if err!=nil{t.Fatal(err)}
  sql:=&collisionProcessSQL{PGCloneExecutor:pg,arrived:arrived,release:release}
  controller:=&SoRProcessor{ReadRoot:ram,SQL:sql,
   Originals:testOriginalReader{values:map[string]OriginalBatch{prepared.BatchUUID:original}}}
  server:=makeControlTestServer(t,controller,pki)
  transports[i],err=NewC1HTTPTransport(server.Listener.Addr().String(),
   "pathfinder-sor.internal",pki.client)
  if err!=nil{t.Fatal(err)}
 }
 type result struct{index int; ack SQLCommitAck; ok bool; err error}
 outcomes:=make(chan result,2)
 var wg sync.WaitGroup
 for i:=range refs{
  wg.Add(1)
  go func(i int){
   defer wg.Done()
   future,err:=transports[i].Submit(ctx,refs[i])
   if err!=nil{outcomes<-result{index:i,err:err};return}
   select{
   case ack,ok:=<-future:outcomes<-result{index:i,ack:ack,ok:ok}
   case <-ctx.Done():outcomes<-result{index:i,err:ctx.Err()}
   }
  }(i)
 }
 for i:=0;i<2;i++{
  select{
  case <-arrived:
  case <-ctx.Done():
   close(release)
   wg.Wait()
   t.Fatalf("conflicting SoR processes did not reach SQL concurrently: %v",ctx.Err())
  }
 }
 close(release)
 wg.Wait()
 close(outcomes)
 winner:=-1
 rejected:=0
 for res:=range outcomes{
  if res.err!=nil{t.Errorf("control transport failed unexpectedly: %v",res.err);continue}
  if !res.ok{rejected++;continue}
  if winner>=0{t.Error("both conflicting original sources received a commit ACK")}
  winner=res.index
  if res.ack.Outcome!="committed"{
   t.Errorf("conflicting source was falsely marked replay: %q",res.ack.Outcome)
  }
  if err:=validateCommitAck(refs[res.index],[]string{inputs[res.index].Input.BatchUUID},res.ack);err!=nil{
   t.Error(err)
  }
 }
 if winner<0||rejected!=1{
  t.Fatalf("expected one winner and one rejection, winner=%d rejected=%d",winner,rejected)
 }
 for i,plan:=range plans{
  state,err:=pg.Inspect(ctx,plan)
  if err!=nil{t.Fatal(err)}
  expected:=SQLConflict
  if i==winner{expected=SQLComplete}
  if state!=expected{t.Fatalf("source %d state=%s wanted=%s",i,state,expected)}
 }
 if err:=pg.VerifyComplete(ctx,plans[winner]);err!=nil{t.Fatal(err)}
 count:=pgAdminIntegration(t,fmt.Sprintf(
  "SELECT count(*) FROM pathfinder.sensor_batch_index_completion WHERE batch_id='%s'::uuid;",
  first.Input.BatchUUID))
 if count!="1"{t.Fatalf("conflicting source inserted duplicate completion: %s",count)}
}
