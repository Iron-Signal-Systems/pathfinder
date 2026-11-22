//go:build pathfinder_pg18_integration

package sensorc1

import (
 "bytes"
 "context"
 "fmt"
 "os"
 "os/exec"
 "strings"
 "testing"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

func requireIntegration(t *testing.T)string{
 t.Helper()
 wrapper:=os.Getenv("PATHFINDER_C1_PG_TEST_WRAPPER")
 if wrapper==""{t.Skip("disposable PostgreSQL 18 clone psql wrapper not supplied")}
 return wrapper
}
func pgAdminIntegration(t *testing.T,script string)string{
 t.Helper()
 cmd:=exec.Command("docker","exec","-i","pathfinder-c1-pg18",
  "psql","-X","-q","-A","-t","-v","ON_ERROR_STOP=1",
  "-h","/var/run/postgresql","-U","postgres","-d",CloneDatabase)
 cmd.Stdin=strings.NewReader(script)
 data,err:=cmd.CombinedOutput()
 if err!=nil{t.Fatalf("disposable PG admin SQL failed: %v: %s",err,data)}
 return strings.TrimSpace(string(data))
}

func TestActualPostgres18FourStreamCommitReplayAndTamper(t *testing.T){
 wrapper:=requireIntegration(t)
 ctx:=context.Background()
 config:=testCloneConfig()
 config.PSQLBinary=wrapper
 config.WriteAuthorization=CloneWriteAuthorization
 pg,err:=NewPGCloneExecutor(config,nil)
 if err!=nil{t.Fatal(err)}
 if err:=pg.VerifyCloneTarget(ctx);err!=nil{t.Fatal(err)}
 pgAdminIntegration(t,fmt.Sprintf(`INSERT INTO pathfinder.sensor (sensor_id,sensor_key,sensor_type,display_name)
  VALUES ('%s','udm-test','unifi','Pathfinder C1 PG18 isolated test')
  ON CONFLICT (sensor_key) DO NOTHING;`,sensorUUID))
 for stream:=range cases{
  t.Run(string(stream),func(t *testing.T){
   original:=originalFor(stream)
   p,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
   plan,err:=BuildSoRBatch(p,original);if err!=nil{t.Fatal(err)}
   state,err:=pg.Inspect(ctx,plan)
   if err!=nil||state!=SQLNew{t.Fatalf("first SQL batch state=%q: %v",state,err)}
   sql,err:=plan.SQLScript();if err!=nil{t.Fatal(err)}
   if err:=pg.Execute(ctx,sql);err!=nil{t.Fatal(err)}
   state,err=pg.Inspect(ctx,plan)
   if err!=nil||state!=SQLComplete{t.Fatalf("postcommit state=%q: %v",state,err)}
   if err:=pg.Verify(ctx,plan);err!=nil{t.Fatal(err)}
   // A complete duplicate replay must not emit another INSERT/COPY.
   state,err=pg.Inspect(ctx,plan)
   if err!=nil||state!=SQLComplete{t.Fatalf("replay state=%q: %v",state,err)}
   if err:=pg.Verify(ctx,plan);err!=nil{t.Fatal(err)}
  })
 }
 t.Run("detect modified committed DNS child",func(t *testing.T){
  original:=originalFor(sensoringest.Observations)
  prepared,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
  plan,err:=BuildSoRBatch(prepared,original);if err!=nil{t.Fatal(err)}
  pgAdminIntegration(t,fmt.Sprintf(`UPDATE pathfinder.sensor_dns_question
    SET question_name='tampered.invalid'
    WHERE observation_id IN (
      SELECT observation_id FROM pathfinder.sensor_observation
      WHERE batch_id='%s'::uuid);`,plan.BatchUUID))
  if err:=pg.Verify(ctx,plan);err==nil{
   t.Fatal("modified committed DNS child incorrectly passed exact readback")
  }
 })
 t.Run("clone only is fail closed",func(t *testing.T){
  bad:=config;bad.Database="pathfinder"
  if _,err:=NewPGCloneExecutor(bad,nil);err==nil{
   t.Fatal("real populated database target was accepted")
  }
 })
}

func TestRealSQLPlansAreDeterministicForSameSource(t *testing.T){
 for stream:=range cases{
  original:=originalFor(stream)
  p,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
  a,err:=BuildSoRSQL(p,original);if err!=nil{t.Fatal(err)}
  b,err:=BuildSoRSQL(p,original);if err!=nil{t.Fatal(err)}
  if !bytes.Equal(a,b){t.Fatal("original SQL projection is nondeterministic")}
 }
}
