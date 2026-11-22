//go:build pathfinder_pg18_integration

package sensorc1

import (
 "bytes"
 "context"
 "crypto/sha256"
 "encoding/hex"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// A real isolated PostgreSQL 18 database must be used for every test here.
// The only lost object is disposable handoff RAM. Original evidence is always
// reread from frozen source, and the populated Pathfinder SoR is never opened.
func pg18RecoveryExecutor(t *testing.T)*PGCloneExecutor {
 t.Helper()
 wrapper:=requireIntegration(t)
 config:=testCloneConfig()
 config.PSQLBinary=wrapper
 config.WriteAuthorization=CloneWriteAuthorization
 pg,err:=NewPGCloneExecutor(config,nil)
 if err!=nil{t.Fatal(err)}
 if err:=pg.VerifyCloneTarget(context.Background());err!=nil{t.Fatal(err)}
 pgAdminIntegration(t,fmt.Sprintf(`INSERT INTO pathfinder.sensor (sensor_id,sensor_key,sensor_type,display_name)
 VALUES ('%s','udm-test','unifi','Pathfinder C1 PG18 isolated test')
 ON CONFLICT (sensor_key) DO NOTHING;`,sensorUUID))
 return pg
}

func TestPG18RecoveryLostACKAndDestroyedRAM(t *testing.T){
 pg:=pg18RecoveryExecutor(t)
 sourceRoot:=t.TempDir()
 // Different source bytes prevent a historical fixture from aliasing another
 // test's source segment identity; the actual segment digest is audited.
 raw:=strings.ReplaceAll(cases[sensoringest.Observations].raw,
  "2026-11-20T10:00:00.123456789Z","2026-11-22T15:12:00.123456789Z")+"\n"
 if err:=os.WriteFile(filepath.Join(sourceRoot,"observations-recovery.jsonl.active"),
  []byte(raw),0600);err!=nil{t.Fatal(err)}
 original,err:=NewFrozenOriginalReader(sourceRoot,sensorID,sensorUUID)
 if err!=nil{t.Fatal(err)}
 var prepared Prepared
 _,err=ReplayFrozen(sourceRoot,sensorID,sensorUUID,func(p Prepared)error{prepared=p;return nil})
 if err!=nil{t.Fatal(err)}
 if prepared.BatchUUID==""{t.Fatal("missing frozen original batch")}
 oldRAM:=t.TempDir()
 if err:=os.Chmod(oldRAM,0700);err!=nil{t.Fatal(err)}
 first:=&SoRProcessor{ReadRoot:oldRAM,Originals:original,SQL:pg}
 b:=Bundle{Format:BundleFormat,Stream:prepared.Stream,
  Batches:[]Prepared{prepared},SourceBytes:prepared.PayloadBytes,Records:prepared.Evidence.Count}
 ref,err:=PublishBundle(oldRAM,b)
 if err!=nil{t.Fatal(err)}
 ack,err:=first.ProcessBundle(context.Background(),ref)
 if err!=nil||ack.Outcome!="committed"{t.Fatalf("first SQL commit: %+v %v",ack,err)}
 // Inject a transport failure AFTER PostgreSQL COMMIT: no response reaches
 // ingest and the complete volatile handoff disappears in a simulated reboot.
 // We intentionally do not call the ACK receiver or RAM reclamation.
 if err:=os.RemoveAll(oldRAM);err!=nil{t.Fatal(err)}
 freshRAM:=t.TempDir()
 if err:=os.Chmod(freshRAM,0700);err!=nil{t.Fatal(err)}
 restarted:=&SoRProcessor{ReadRoot:freshRAM,Originals:original,SQL:pg}
 pki:=makeControlTestPKI(t)
 server:=makeControlTestServer(t,restarted,pki)
 transport,err:=NewC1HTTPTransport(server.Listener.Addr().String(),"pathfinder-sor.internal",pki.client)
 if err!=nil{t.Fatal(err)}
 dispatcher,err:=NewDispatcher(freshRAM,transport)
 if err!=nil{t.Fatal(err)}
 scheduler,err:=NewScheduler(DefaultSchedulerOptions(),dispatcher)
 if err!=nil{t.Fatal(err)}
 _,err=ReplayFrozen(sourceRoot,sensorID,sensorUUID,func(p Prepared)error{
  return scheduler.Push(context.Background(),p,time.Now())
 })
 if err!=nil{t.Fatal(err)}
 if err:=scheduler.Drain(context.Background());err!=nil{t.Fatal(err)}
 if err:=dispatcher.Wait(context.Background());err!=nil{t.Fatal(err)}
 originalPlan,err:=original.OpenOriginal(context.Background(),prepared)
 if err!=nil{t.Fatal(err)}
 sqlPlan,err:=BuildSoRBatch(prepared,originalPlan)
 if err!=nil{t.Fatal(err)}
 state,err:=pg.Inspect(context.Background(),sqlPlan)
 if err!=nil||state!=SQLComplete{t.Fatalf("lost ACK recovery=%s %v",state,err)}
 if err:=pg.VerifyComplete(context.Background(),sqlPlan);err!=nil{t.Fatal(err)}
 got:=pgAdminIntegration(t,fmt.Sprintf(`SELECT count(*) FROM pathfinder.sensor_batch_index_completion
  WHERE batch_id='%s'::uuid;`,prepared.BatchUUID))
 if got!="1"{t.Fatalf("duplicate post-recovery completion count=%s",got)}
}

func TestPG18RecoveryAbortedTransactionLeavesNoCheckpoint(t *testing.T){
 pg:=pg18RecoveryExecutor(t)
 original:=originalFor(sensoringest.Health)
 original.Input.WireBatchID="pg18-recovery-abort-v1"
 original.Input.BatchUUID=sensorsor.ID("frozen-import-batch-v1",sensorUUID,
  string(original.Input.Stream),original.Input.WireBatchID)
 p,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
 plan,err:=BuildSoRBatch(p,original);if err!=nil{t.Fatal(err)}
 script,err:=plan.SQLScript();if err!=nil{t.Fatal(err)}
 if !bytes.Contains(script,[]byte("COMMIT;\n\\echo")){t.Fatal("missing final COMMIT")}
 broken:=bytes.Replace(script,[]byte("COMMIT;\n\\echo"),[]byte("SELECT 1/0;\nCOMMIT;\n\\echo"),1)
 if err:=pg.Execute(context.Background(),broken);err==nil{
  t.Fatal("injected SQL failure was incorrectly accepted")
 }
 state,err:=pg.Inspect(context.Background(),plan)
 if err!=nil||state!=SQLNew{t.Fatalf("aborted transaction leaked durable batch metadata: %s %v",state,err)}
 if err:=pg.Execute(context.Background(),script);err!=nil{t.Fatal(err)}
 state,err=pg.Inspect(context.Background(),plan)
 if err!=nil||state!=SQLComplete{t.Fatalf("replay after abort=%s %v",state,err)}
 if err:=pg.VerifyComplete(context.Background(),plan);err!=nil{t.Fatal(err)}
}

func TestPG18RecoveryPartialMetadataNeverProducesACK(t *testing.T){
 pg:=pg18RecoveryExecutor(t)
 original:=originalFor(sensoringest.Health)
 original.Input.WireBatchID="pg18-recovery-partial-v1"
 original.Input.BatchUUID=sensorsor.ID("frozen-import-batch-v1",sensorUUID,
  string(original.Input.Stream),original.Input.WireBatchID)
 p,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
 plan,err:=BuildSoRBatch(p,original);if err!=nil{t.Fatal(err)}
 sql,err:=plan.SQLScript();if err!=nil{t.Fatal(err)}
 if err:=pg.Execute(context.Background(),sql);err!=nil{t.Fatal(err)}
 // Simulate damaged legacy or operator-created state; the normal C1 writer
 // never creates this because all source records commit atomically.
 pgAdminIntegration(t,fmt.Sprintf(`DELETE FROM pathfinder.sensor_batch_index_completion
  WHERE batch_id='%s'::uuid;`,p.BatchUUID))
 state,err:=pg.Inspect(context.Background(),plan)
 if err!=nil||state!=SQLPartial{t.Fatalf("partial-state detection=%s %v",state,err)}
 ram:=t.TempDir()
 if err:=os.Chmod(ram,0700);err!=nil{t.Fatal(err)}
 ref,err:=PublishBundle(ram,Bundle{Format:BundleFormat,
  Stream:p.Stream,Batches:[]Prepared{p},
  SourceBytes:p.PayloadBytes,Records:p.Evidence.Count})
 if err!=nil{t.Fatal(err)}
 processor:=&SoRProcessor{ReadRoot:ram,SQL:pg,
  Originals:testOriginalReader{values:map[string]OriginalBatch{p.BatchUUID:original}}}
 ack,err:=processor.ProcessBundle(context.Background(),ref)
 if err==nil||ack.Outcome!=""{t.Fatalf("partial SQL state falsely acknowledged: %+v %v",ack,err)}
 if _,err:=os.Stat(filepath.Join(ram,ref.Name));err!=nil{t.Fatal("uncertain RAM removed")}
}

// Wire identity must be bound to exact original bytes even if all source
// counts and file names match. Preparation hash is not a SQL completion proof.
func TestRecoveryHashIsIndependentOfBatchIdentifier(t *testing.T){
 a:=[]byte(cases[sensoringest.Health].raw+"\n")
 b:=[]byte(strings.ReplaceAll(string(a),"2026-11-20T10:00:00Z","2026-11-20T10:00:02Z"))
 aa:=sha256.Sum256(a);bb:=sha256.Sum256(b)
 if hex.EncodeToString(aa[:])==hex.EncodeToString(bb[:]){t.Fatal("changed source preserved identical hash")}
}
