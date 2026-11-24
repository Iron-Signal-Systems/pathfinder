package sensorc1

import (
 "context"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "sync/atomic"
 "testing"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// A clean eight-segment archive produces independently identifiable batches.
// Unknown JSON keys are permitted source evidence and still bound by hashes.
func makeEightHealthSegments(t *testing.T)string{
 t.Helper()
 root:=t.TempDir()
 path:=filepath.Join(root,"health")
 if err:=os.MkdirAll(path,0700);err!=nil{t.Fatal(err)}
 raw:=cases[sensoringest.Health].raw
 if !strings.HasSuffix(raw,"}"){t.Fatal("malformed health fixture")}
 for i:=0;i<8;i++{
  name:=filepath.Join(path,fmt.Sprintf("health-%02d.jsonl.active",i))
  unique:=raw[:len(raw)-1]+fmt.Sprintf(",\"c1_parallel_nonce\":%d}\n",i)
  if err:=os.WriteFile(name,[]byte(unique),0600);err!=nil{t.Fatal(err)}
 }
 return root
}
type parallelReadOnlySQL struct{
 active atomic.Int64
 peak atomic.Int64
 verifies atomic.Int64
 fail bool
}
func(s *parallelReadOnlySQL)Inspect(_ context.Context,_ sensorsor.BatchSQL)(SQLBatchState,error){
 return SQLComplete,nil
}
func(s *parallelReadOnlySQL)Verify(ctx context.Context,_ sensorsor.BatchSQL)error{
 current:=s.active.Add(1)
 defer s.active.Add(-1)
 for {
  peak:=s.peak.Load()
  if current<=peak || s.peak.CompareAndSwap(peak,current){break}
 }
 select{
 case <-ctx.Done():return ctx.Err()
 case <-time.After(25*time.Millisecond):
 }
 s.verifies.Add(1)
 if s.fail{return fmt.Errorf("intentional evidence mismatch")}
 return nil
}
func TestIndependentBParallelFrozenBoundedAndExact(t *testing.T){
 root:=makeEightHealthSegments(t)
 originals,err:=NewFrozenOriginalReader(root,sensorID,sensorUUID)
 if err!=nil{t.Fatal(err)}
 sql:=&parallelReadOnlySQL{}
 var progress atomic.Int64
 verifier:=IndependentVerifierB{
  Originals:originals,SQL:sql,
  PollInterval:time.Millisecond,MaxWait:5*time.Second,Workers:4,
  OnProgress:func(p BVerificationProgress){
   if p.Batches<1||p.Records<1||p.Records>8||p.ExpectedRecords!=8{
    t.Errorf("bad progress %+v",p)
   }
   progress.Add(1)
  },
 }
 got,err:=verifier.VerifyFrozen(context.Background(),root,sensorID,sensorUUID,8,8)
 if err!=nil{t.Fatal(err)}
 if got.Files!=8||got.Batches!=8||got.Records!=8{
  t.Fatalf("missing independently verified source evidence: %+v",got)
 }
 if sql.verifies.Load()!=8||sql.peak.Load()<2||sql.peak.Load()>4{
  t.Fatalf("bounded concurrent B proof not exercised: verified=%d peak=%d",
   sql.verifies.Load(),sql.peak.Load())
 }
 if progress.Load()!=1{t.Fatalf("no final B progress: %d",progress.Load())}
}
func TestIndependentBParallelFailureNeverReturnsAcceptedSource(t *testing.T){
 root:=makeEightHealthSegments(t)
 originals,err:=NewFrozenOriginalReader(root,sensorID,sensorUUID)
 if err!=nil{t.Fatal(err)}
 verifier:=IndependentVerifierB{
  Originals:originals,SQL:&parallelReadOnlySQL{fail:true},
  PollInterval:time.Millisecond,MaxWait:5*time.Second,Workers:4,
 }
 result,err:=verifier.VerifyFrozen(context.Background(),root,sensorID,sensorUUID,8,8)
 if err==nil||result.CompletedAt.After(result.StartedAt){
  t.Fatalf("corrupt evidence returned final B acceptance: result=%+v err=%v",result,err)
 }
}
func TestIndependentBParallelRejectsUnsafeWorkerCount(t *testing.T){
 root:=makeEightHealthSegments(t)
 verifier:=IndependentVerifierB{Workers:17}
 if _,err:=verifier.VerifyFrozen(context.Background(),root,sensorID,sensorUUID,8,8);err==nil{
  t.Fatal("unbounded verification worker count accepted")
 }
}
