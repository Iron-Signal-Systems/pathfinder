package sensorc1

import (
 "context"
 "crypto/sha256"
 "fmt"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "sync"
 "testing"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

// SQL operations are individually thread-safe, but intentionally NOT atomic
// across Inspect / Execute / Inspect. The SoR per-natural-source gate must
// coordinate that sequence; otherwise simultaneous retries both see "new".
type concurrentTestSQL struct{
 mu sync.Mutex
 states map[string]SQLBatchState
 writes int
}
func (s *concurrentTestSQL)Inspect(_ context.Context,p sensorsor.BatchSQL)(SQLBatchState,error){
 s.mu.Lock();defer s.mu.Unlock()
 if state,ok:=s.states[p.BatchUUID];ok{return state,nil}
 return SQLNew,nil
}
func (s *concurrentTestSQL)Execute(ctx context.Context,sql []byte)error{
 if !strings.Contains(string(sql),"COMMIT;") {
  return errors.New("missing commit")
 }
 // Encourage concurrent contenders to observe "new" on an unfenced SoR.
 select {
 case <-time.After(15*time.Millisecond):
 case <-ctx.Done():return ctx.Err()
 }
 s.mu.Lock()
 defer s.mu.Unlock()
 // Test fixture contains a single original batch, supplied by fixture.
 // SQL's real uniqueness checks remain independently tested in PG18.
 for id,state:=range s.states{
  if state!=SQLNew{return errors.New("duplicate SQL execution")}
  s.states[id]=SQLComplete
  s.writes++
  return nil
 }
 return errors.New("missing source key")
}
func (s *concurrentTestSQL)Verify(_ context.Context,_ sensorsor.BatchSQL)error{return nil}

func TestConcurrentDistinctRAMReferencesSameOriginalOnlyCommitOnce(t *testing.T){
 p,base,_,bundle:=processorFixture(t)
 if len(bundle.Batches)!=2{t.Fatal("fixture expected two batches")}
 // Test with one batch to prove exact natural-key coordination.
 first:=bundle.Batches[0]
 single:=Bundle{
  Format:BundleFormat,Stream:first.Stream,
  Batches:[]Prepared{first},SourceBytes:first.PayloadBytes,Records:first.Evidence.Count,
 }
 sql:=&concurrentTestSQL{states:map[string]SQLBatchState{first.BatchUUID:SQLNew}}
 p.SQL=sql
 refs:=make([]BundleReference,12)
 for i:=range refs{
  var err error
  refs[i],err=PublishBundle(p.ReadRoot,single)
  if err!=nil{t.Fatal(err)}
 }
 var wg sync.WaitGroup
 start:=make(chan struct{})
 errs:=make(chan error,len(refs))
 outcomes:=make(chan string,len(refs))
 for _,ref:=range refs{
  wg.Add(1)
  go func(ref BundleReference){
   defer wg.Done()
   <-start
   ack,err:=p.ProcessBundle(context.Background(),ref)
   if err!=nil{errs<-err;return}
   if err:=validateCommitAck(ref,[]string{first.BatchUUID},ack);err!=nil{
    errs<-err
    return
   }
   outcomes<-ack.Outcome
  }(ref)
 }
 close(start)
 wg.Wait()
 close(errs);close(outcomes)
 for err:=range errs{t.Errorf("concurrent SoR retry failed: %v",err)}
 committed,duplicate:=0,0
 for outcome:=range outcomes{
  switch outcome{
  case "committed":committed++
  case "already_committed":duplicate++
  default:t.Errorf("unknown SQL acknowledgment %s",outcome)
  }
 }
 if sql.writes!=1||committed!=1||duplicate!=len(refs)-1{
  t.Fatalf("duplicate retry changed SQL writes=%d committed=%d duplicate=%d",
   sql.writes,committed,duplicate)
 }
 if _,err:=os.Stat(filepath.Join(p.ReadRoot,base.Name));err!=nil{
  t.Fatal("SoR prematurely unlinked an original RAM source")
 }
}

// overlapSQL deliberately prevents the first SQL writer from finishing
// until a second independent source enters SQL Execute. It proves C1 gate
// striping preserves true parallelism for unrelated family/source keys.
type overlapSQL struct{
 mu sync.Mutex
 ids []string
 states map[string]SQLBatchState
 entered chan string
 release chan struct{}
}
func(s *overlapSQL)Inspect(_ context.Context,p sensorsor.BatchSQL)(SQLBatchState,error){
 s.mu.Lock();defer s.mu.Unlock()
 if state,ok:=s.states[p.BatchUUID];ok{return state,nil}
 return SQLNew,nil
}
func(s *overlapSQL)Execute(ctx context.Context,script []byte)error{
 key:=""
 for _,id:=range s.ids{
  if strings.Contains(string(script),id){key=id;break}
 }
 if key==""{return errors.New("SQL source batch ID missing")}
 select{
 case s.entered<-key:
 case <-ctx.Done():return ctx.Err()
 }
 select{
 case <-s.release:
 case <-ctx.Done():return ctx.Err()
 }
 s.mu.Lock()
 s.states[key]=SQLComplete
 s.mu.Unlock()
 return nil
}
func(s *overlapSQL)Verify(_ context.Context,_ sensorsor.BatchSQL)error{return nil}

func TestSoRUnrelatedFamiliesCanExecuteSQLInParallel(t *testing.T){
 left:=originalFor(sensoringest.Observations)
 right:=originalFor(sensoringest.Health)
 pickShard:=func(o OriginalBatch)byte{
  id:=sha256.Sum256([]byte(o.Input.SensorUUID+"\x00"+string(o.Input.Stream)+"\x00"+o.Input.WireBatchID))
  return id[0]
 }
 // The keys are chosen from distinct streams and must occupy different
 // bounded gates; adjust wire identity deterministically on a hash collision.
 for n:=0;n<32 && pickShard(left)==pickShard(right);n++ {
  right.Input.WireBatchID=fmt.Sprintf("independent-health-%d",n)
  right.Input.BatchUUID=sensorsor.ID("frozen-import-batch-v1",
   right.Input.SensorUUID,string(right.Input.Stream),right.Input.WireBatchID)
 }
 if pickShard(left)==pickShard(right){t.Fatal("unable to select two independent source gates")}
 a,err:=Prepare(left.Input);if err!=nil{t.Fatal(err)}
 b,err:=Prepare(right.Input);if err!=nil{t.Fatal(err)}
 ram:=t.TempDir()
 if err:=os.Chmod(ram,0700);err!=nil{t.Fatal(err)}
 fake:=&overlapSQL{
  ids:[]string{a.BatchUUID,b.BatchUUID},
  states:map[string]SQLBatchState{},
  entered:make(chan string,2),release:make(chan struct{}),
 }
 proc:=&SoRProcessor{
  ReadRoot:ram,SQL:fake,
  Originals:testOriginalReader{values:map[string]OriginalBatch{
   a.BatchUUID:left,b.BatchUUID:right,
  }},
 }
 one:=Bundle{Format:BundleFormat,Stream:a.Stream,Batches:[]Prepared{a},
  SourceBytes:a.PayloadBytes,Records:a.Evidence.Count}
 two:=Bundle{Format:BundleFormat,Stream:b.Stream,Batches:[]Prepared{b},
  SourceBytes:b.PayloadBytes,Records:b.Evidence.Count}
 refs:=make([]BundleReference,2)
 for i,bundle:=range []Bundle{one,two}{
  refs[i],err=PublishBundle(ram,bundle)
  if err!=nil{t.Fatal(err)}
 }
 ctx,cancel:=context.WithTimeout(context.Background(),4*time.Second)
 defer cancel()
 errs:=make(chan error,2)
 var wg sync.WaitGroup
 for _,ref:=range refs{
  wg.Add(1)
  go func(ref BundleReference){
   defer wg.Done()
   ack,err:=proc.ProcessBundle(ctx,ref)
   if err!=nil{errs<-err;return}
   if ack.Outcome!="committed"{errs<-fmt.Errorf("unrelated source was not newly committed: %s",ack.Outcome)}
  }(ref)
 }
 arrived:=map[string]bool{}
 for len(arrived)<2{
  select{
  case id:=<-fake.entered:
   arrived[id]=true
  case <-ctx.Done():
   close(fake.release)
   wg.Wait()
   t.Fatalf("unrelated SQL streams could not overlap: %v",ctx.Err())
  }
 }
 close(fake.release)
 wg.Wait()
 close(errs)
 for err:=range errs{t.Error(err)}
 if !arrived[a.BatchUUID]||!arrived[b.BatchUUID]{
  t.Fatalf("expected both independent SQL writers, saw %+v",arrived)
 }
}
