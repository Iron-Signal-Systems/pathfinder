//go:build pathfinder_pg18_integration

package sensorc1

import (
 "compress/gzip"
 "context"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

// This test traverses C1 with REAL PG18 and independent original evidence.
// It exercises two separate directory-backed RAM control directions,
// not TCP. CI directories are not FreeBSD tmpfs/nullfs acceptance.
func TestFullC1FrozenPipelineIntoActualPG18(t *testing.T){
 wrapper:=requireIntegration(t)
 root:=t.TempDir()
 testFiles:=map[string]sensoringest.Stream{
  "observations-20261120.jsonl.active":sensoringest.Observations,
  "conntrack/conntrack-20261120.jsonl.gz":sensoringest.Conntrack,
  "identity/endpoint-identity-20261120.jsonl.gz":sensoringest.Identity,
  "health/health-20261120.jsonl.active":sensoringest.Health,
 }
 for name,stream:=range testFiles{
  path:=filepath.Join(root,name)
  if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{t.Fatal(err)}
  f,err:=os.Create(path);if err!=nil{t.Fatal(err)}
  raw:=[]byte(cases[stream].raw+"\n")
  if strings.HasSuffix(name,".gz"){
   g:=gzip.NewWriter(f)
   if _,err:=g.Write(raw);err!=nil{f.Close();t.Fatal(err)}
   if err:=g.Close();err!=nil{f.Close();t.Fatal(err)}
  }else{
   if _,err:=f.Write(raw);err!=nil{f.Close();t.Fatal(err)}
  }
  if err:=f.Close();err!=nil{t.Fatal(err)}
 }
 orig,err:=NewFrozenOriginalReader(root,sensorID,sensorUUID)
 if err!=nil{t.Fatal(err)}
 pgAdminIntegration(t,fmt.Sprintf(`INSERT INTO pathfinder.sensor (sensor_id,sensor_key,sensor_type,display_name)
 VALUES ('%s','udm-test','unifi','Pathfinder C1 PG18 isolated test')
 ON CONFLICT (sensor_key) DO NOTHING;`,sensorUUID))
 config:=testCloneConfig()
 config.PSQLBinary=wrapper
 config.WriteAuthorization=CloneWriteAuthorization
 pg,err:=NewPGCloneExecutor(config,nil)
 if err!=nil{t.Fatal(err)}
 ram:=t.TempDir()
 if err:=os.Chmod(ram,0700);err!=nil{t.Fatal(err)}
 processor:=&SoRProcessor{ReadRoot:ram,Originals:orig,SQL:pg}
 // Prove the actual two-directory, entirely filesystem-based control
 // path against PostgreSQL 18. CI directories stand in for tmpfs/nullfs;
 // native C220 mount permissions have a separate acceptance gate.
 gid:=os.Getgid()
 if gid<1{t.Skip("cannot exercise a non-root shared group as gid 0")}
 receipts:=t.TempDir()
 if err:=os.Chmod(receipts,0750);err!=nil{t.Fatal(err)}
 ctx,cancel:=context.WithCancel(context.Background())
 defer cancel()
 sorDone:=make(chan error,1)
 go func(){sorDone<-ServeRAMWork(ctx,ram,receipts,gid,processor,5*time.Millisecond)}()
 transport,err:=NewRAMReceiptTransport(receipts,gid,5*time.Millisecond)
 if err!=nil{t.Fatal(err)}
 dispatcher,err:=NewDispatcherForJails(ram,transport,gid)
 if err!=nil{t.Fatal(err)}
 scheduler,err:=NewScheduler(DefaultSchedulerOptions(),dispatcher)
 if err!=nil{t.Fatal(err)}
 emitted:=0
 total,err:=ReplayFrozen(root,sensorID,sensorUUID,func(p Prepared)error{
  emitted++
  return scheduler.Push(ctx,p,time.Now())
 })
 if err!=nil{t.Fatal(err)}
 if total.Files!=4||total.Batches!=4||total.Records!=4||emitted!=4{
  t.Fatalf("unexpected replay counts %+v",total)
 }
 if err:=scheduler.Drain(ctx);err!=nil{t.Fatal(err)}
 if err:=dispatcher.Wait(ctx);err!=nil{t.Fatal(err)}
 // The SoR's independent PostgreSQL write/commit/readback takes ownership
 // of the indexed evidence. Receiver custody is not created or acknowledged.
 q:=fmt.Sprintf(`SELECT count(*) FROM pathfinder.sensor_batch_index_completion c
 JOIN pathfinder.sensor_batch b ON b.batch_id=c.batch_id
 WHERE b.sensor_id='%s'::uuid AND b.wire_batch_id LIKE 'frozen-v1-%%';`,sensorUUID)
 got:=pgAdminIntegration(t,q)
 if got!="4"{t.Fatalf("not all four committed historical batches: %s",got)}
 // Replay is duplicate-safe: exact original batches do not add SQL rows.
 count:=0
 _,err=ReplayFrozen(root,sensorID,sensorUUID,func(p Prepared)error{
  count++
  return scheduler.Push(ctx,p,time.Now())
 })
 if err!=nil{t.Fatal(err)}
 if err:=scheduler.Drain(ctx);err!=nil{t.Fatal(err)}
 if err:=dispatcher.Wait(ctx);err!=nil{t.Fatal(err)}
 if count!=4{t.Fatalf("duplicate replay lost batches: %d",count)}
 if got2:=pgAdminIntegration(t,q);got2!="4"{t.Fatalf("duplicate replay changed indexed count: %s",got2)}
 cancel()
 select{
 case err:=<-sorDone:
   if err!=nil{t.Fatalf("filesystem SoR stopped: %v",err)}
 case <-time.After(3*time.Second):t.Fatal("filesystem SoR loop did not shut down")
 }
}
