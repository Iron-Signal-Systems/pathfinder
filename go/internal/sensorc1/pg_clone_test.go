package sensorc1

import (
 "context"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

func testCloneConfig()PGCloneConfig{
 return PGCloneConfig{
  PSQLBinary:"/usr/local/bin/psql",
  SocketDirectory:"/var/run/postgresql",
  Database:CloneDatabase,
  Role:"pathfinder_app",
 }
}

func TestCloneDatabaseGuardRefusesLiveTarget(t *testing.T){
 c:=testCloneConfig()
 for _,bad:=range []string{"pathfinder","postgres","", "pathfinder_c1_clone_evil"}{
  c.Database=bad
  if _,err:=NewPGCloneExecutor(c,func(context.Context,[]byte)(string,error){return "",nil});err==nil{
   t.Fatalf("accepted database %q",bad)
  }
 }
 c=testCloneConfig()
 c.SocketDirectory="localhost"
 if _,err:=NewPGCloneExecutor(c,func(context.Context,[]byte)(string,error){return "",nil});err==nil{
  t.Fatal("accepted TCP Postgres address")
 }
}

func TestClonePreflightFailsClosed(t *testing.T){
 c:=testCloneConfig()
 for _,response:=range []string{"", "C1_CLONE_TARGET_FAIL", "C1_CLONE_TARGET_PASS\nunexpected"}{
  p,err:=NewPGCloneExecutor(c,func(_ context.Context,_ []byte)(string,error){return response,nil})
  if err!=nil{t.Fatal(err)}
  if err:=p.VerifyCloneTarget(context.Background());err==nil{
   t.Fatalf("accepted invalid preflight %q",response)
  }
 }
}

func TestCloneExecutorInspectsExactMetadataBeforeReplay(t *testing.T){
 original:=originalFor(sensoringest.Observations)
 prepared,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
 batch,err:=BuildSoRBatch(prepared,original);if err!=nil{t.Fatal(err)}
 var queries []string
 result:="partial"
 runner:=func(_ context.Context,sql []byte)(string,error){
  q:=string(sql)
  queries=append(queries,q)
  if strings.Contains(q,"C1_CLONE_TARGET_PASS"){return "C1_CLONE_TARGET_PASS",nil}
  if strings.Contains(q,"WITH candidates AS"){return result,nil}
  return "",errors.New("unrecognized SQL")
 }
 pg,err:=NewPGCloneExecutor(testCloneConfig(),runner);if err!=nil{t.Fatal(err)}
 for _,state:=range []SQLBatchState{SQLNew,SQLComplete,SQLPartial,SQLConflict}{
  result=string(state)
  got,err:=pg.Inspect(context.Background(),batch)
  if err!=nil||got!=state{t.Fatalf("state=%q got=%q error=%v",state,got,err)}
 }
 if len(queries)!=8{t.Fatalf("wrong preflight and state queries %d",len(queries))}
 q:=queries[1]
 for _,needle:=range []string{
  "sensor_batch_index_completion","WITH candidates AS",
  "pathfinder.sensor_segment","payload_sha256","custody_reference",
  "first_observed_at","last_observed_at","record_count",
  batch.BatchUUID,batch.WireBatchID,batch.SegmentSHA,
 }{
  if !strings.Contains(q,needle){t.Fatalf("SQL state missing %q",needle)}
 }
}

func TestCloneNoExecuteUntilExplicitCloneApproval(t *testing.T){
 c:=testCloneConfig()
 executions:=0
 runner:=func(_ context.Context,sql []byte)(string,error){
  q:=string(sql)
  if strings.Contains(q,"C1_CLONE_TARGET_PASS"){return "C1_CLONE_TARGET_PASS",nil}
  executions++
  return "",nil
 }
 pg,err:=NewPGCloneExecutor(c,runner);if err!=nil{t.Fatal(err)}
 script:=[]byte("BEGIN;\nINSERT INTO pathfinder.sensor_batch_index_completion (batch_id) VALUES (NULL);\nCOMMIT;")
 if err:=pg.Execute(context.Background(),script);err==nil{t.Fatal("unapproved SQL write executed")}
 if executions!=0{t.Fatalf("unapproved SQL calls=%d",executions)}
 c.WriteAuthorization=CloneWriteAuthorization
 pg,err=NewPGCloneExecutor(c,runner);if err!=nil{t.Fatal(err)}
 if err:=pg.Execute(context.Background(),[]byte("SELECT 1;"));err==nil{t.Fatal("noncanonical SQL accepted")}
 if err:=pg.Execute(context.Background(),script);err!=nil{t.Fatal(err)}
 if executions!=1{t.Fatalf("expected one SQL call, got %d",executions)}
}

func TestPGReadbackIncludesOriginalAndEveryChild(t *testing.T){
 original:=originalFor(sensoringest.Observations)
 p,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
 batch,err:=BuildSoRBatch(p,original);if err!=nil{t.Fatal(err)}
 seen:=map[string]bool{}
 runner:=func(_ context.Context,sql []byte)(string,error){
  q:=string(sql)
  if strings.Contains(q,"C1_CLONE_TARGET_PASS"){return "C1_CLONE_TARGET_PASS",nil}
  if strings.Contains(q,"WITH candidates AS"){return "complete",nil}
  if strings.Contains(q,"EXCEPT ALL"){
   for _,name:=range []string{"sensor_observation","sensor_dns_question","sensor_dns_record"}{
    if strings.Contains(q,"FROM pathfinder."+name+" d"){seen[name]=true}
   }
   return "PASS",nil
  }
  return "",errors.New("unexpected SQL")
 }
 pg,err:=NewPGCloneExecutor(testCloneConfig(),runner);if err!=nil{t.Fatal(err)}
 if err:=pg.Verify(context.Background(),batch);err!=nil{t.Fatal(err)}
 for _,name:=range []string{"sensor_observation","sensor_dns_question","sensor_dns_record"}{
  if !seen[name]{t.Fatalf("readback skipped %s",name)}
 }
}

func TestPGReadbackMismatchBlocksCommitACK(t *testing.T){
 original:=originalFor(sensoringest.Identity)
 p,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
 batch,err:=BuildSoRBatch(p,original);if err!=nil{t.Fatal(err)}
 runner:=func(_ context.Context,sql []byte)(string,error){
  q:=string(sql)
  if strings.Contains(q,"C1_CLONE_TARGET_PASS"){return "C1_CLONE_TARGET_PASS",nil}
  if strings.Contains(q,"WITH candidates AS"){return "complete",nil}
  if strings.Contains(q,"EXCEPT ALL"){
   if strings.Contains(q,"sensor_identity_endpoint"){return "FAIL",nil}
   return "PASS",nil
  }
  return "",errors.New("unexpected SQL")
 }
 pg,err:=NewPGCloneExecutor(testCloneConfig(),runner);if err!=nil{t.Fatal(err)}
 if err:=pg.Verify(context.Background(),batch);err==nil{
  t.Fatal("verified complete batch with corrupt identity child data")
 }
}

func TestPGReadbackQuotesUntrustedJSON(t *testing.T){
 c:=sqlReadback["sensor_dns_question"]
 sql,err:=makeReadbackQuery(c,"11111111-2222-4333-8444-555555555555",[][]string{
  {"2026-11-20T10:00:00Z","11111111-2222-4333-8444-555555555555","0","example'quote","A"},
 })
 if err!=nil{t.Fatal(err)}
 if !strings.Contains(sql,"'example''quote'::text")||!strings.Contains(sql,"EXCEPT ALL"){
  t.Fatal("missing SQL string quote or exact-set comparison")
 }
}

func TestPGCredentialFileMustBeRegularAndOwnerOnly(t *testing.T){
 root:=t.TempDir()
 secret:=filepath.Join(root,".pgpass")
 if err:=os.WriteFile(secret,[]byte("localhost:5432:*:test:testing\n"),0600);err!=nil{t.Fatal(err)}
 c:=testCloneConfig()
 c.PasswordFile=secret
 if _,err:=NewPGCloneExecutor(c,func(context.Context,[]byte)(string,error){return "",nil});err!=nil{
  t.Fatalf("refused owner-only PostgreSQL credential file: %v",err)
 }
 if err:=os.Chmod(secret,0640);err!=nil{t.Fatal(err)}
 if _,err:=NewPGCloneExecutor(c,func(context.Context,[]byte)(string,error){return "",nil});err==nil{
  t.Fatal("accepted group-readable PostgreSQL credential file")
 }
 if err:=os.Chmod(secret,0600);err!=nil{t.Fatal(err)}
 alias:=filepath.Join(root,"alias-pgpass")
 if err:=os.Symlink(secret,alias);err!=nil{t.Fatal(err)}
 c.PasswordFile=alias
 if _,err:=NewPGCloneExecutor(c,func(context.Context,[]byte)(string,error){return "",nil});err==nil{
  t.Fatal("accepted symlinked PostgreSQL credentials")
 }
 c.PasswordFile=filepath.Join(root,"missing")
 if _,err:=NewPGCloneExecutor(c,func(context.Context,[]byte)(string,error){return "",nil});err==nil{
  t.Fatal("accepted missing PostgreSQL credential file")
 }
}
