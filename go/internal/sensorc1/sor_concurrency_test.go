package sensorc1

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "sync"
 "testing"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
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
