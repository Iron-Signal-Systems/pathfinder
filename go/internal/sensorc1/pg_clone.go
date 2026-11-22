package sensorc1

import (
 "bytes"
 "context"
 "errors"
 "fmt"
 "os"
 "os/exec"
 "path/filepath"
 "strings"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// CloneDatabase is the only database that this development executor accepts.
// A correctly named clone still requires independently verified isolation,
// restricted SQL credentials, and an explicit SQL-commit authorization.
const CloneDatabase = "pathfinder_c1_clone"
const CloneWriteAuthorization = "I_VERIFIED_AN_ISOLATED_PATHFINDER_C1_CLONE"

// PGRun executes exactly one PostgreSQL command, returning only query stdout.
// Implementations must return an error for a failed SQL statement/exit status.
type PGRun func(context.Context, []byte) (string,error)

type PGCloneConfig struct{
 PSQLBinary string
 SocketDirectory string
 Role string
 PasswordFile string
 Database string
 WriteAuthorization string
}

type PGCloneExecutor struct{
 config PGCloneConfig
 run PGRun
}

// NewPGCloneExecutor does not connect, modify a database or start a service.
// The caller must validate the actual FreeBSD environment before approval.
func NewPGCloneExecutor(c PGCloneConfig,run PGRun)(*PGCloneExecutor,error){
 if c.Database!=CloneDatabase {
  return nil,fmt.Errorf("C1 PostgreSQL: refuse database %q; only isolated clone %q supported",c.Database,CloneDatabase)
 }
 if c.PSQLBinary==""{c.PSQLBinary="/usr/local/bin/psql"}
 if !filepath.IsAbs(c.PSQLBinary) || !filepath.IsAbs(c.SocketDirectory) ||
    !strings.HasPrefix(c.SocketDirectory,"/") || c.Role=="" ||
    strings.ContainsAny(c.Role," \t\n\r") {
  return nil,errors.New("C1 PostgreSQL: require absolute psql/socket paths and an explicit role")
 }
 if c.PasswordFile!="" && !filepath.IsAbs(c.PasswordFile) {
  return nil,errors.New("C1 PostgreSQL: password file must use absolute path")
 }
 if run==nil {
  run=makeLocalPGRun(c)
 }
 return &PGCloneExecutor{config:c,run:run},nil
}

// Executed only by pathfinder-sor; -h is a Unix socket directory, never a
// hostname. No arbitrary PG* environment variables are inherited.
func makeLocalPGRun(c PGCloneConfig)PGRun{
 return func(ctx context.Context,script []byte)(string,error){
  cmd:=exec.CommandContext(ctx,c.PSQLBinary,
   "-X","-q","-A","-t","-P","pager=off","-v","ON_ERROR_STOP=1",
   "-h",c.SocketDirectory,"-U",c.Role,"-d",c.Database)
  cmd.Stdin=bytes.NewReader(script)
  cmd.Env=make([]string,0)
  for _,entry:=range os.Environ(){
   key,_,_:=strings.Cut(entry,"=")
   if !strings.HasPrefix(key,"PG"){cmd.Env=append(cmd.Env,entry)}
  }
  cmd.Env=append(cmd.Env,"PGCONNECT_TIMEOUT=6","PGAPPNAME=pathfinder-c1-clone-sor")
  if c.PasswordFile!=""{cmd.Env=append(cmd.Env,"PGPASSFILE="+c.PasswordFile)}
  out,err:=cmd.CombinedOutput()
  if err!=nil{return "",fmt.Errorf("C1 PostgreSQL psql failed: %w: %s",err,strings.TrimSpace(string(out)))}
  return strings.TrimSpace(string(out)),nil
 }
}

// VerifyCloneTarget is performed for EACH operation, including read-only
// probes. An app-role local socket alone is not a clone isolation proof;
// it must also identify the exact database and expected installed tables.
// No check writes to PostgreSQL.
func(p *PGCloneExecutor) VerifyCloneTarget(ctx context.Context)error{
 if p==nil||p.run==nil{return errors.New("C1 PostgreSQL: executor missing")}
 query:=`SELECT CASE WHEN current_database() = 'pathfinder_c1_clone'
 AND to_regclass('pathfinder.sensor_batch') IS NOT NULL
 AND to_regclass('pathfinder.sensor_batch_index_completion') IS NOT NULL
 AND to_regclass('pathfinder.sensor_segment') IS NOT NULL
 AND to_regclass('pathfinder.sensor_observation') IS NOT NULL
 AND to_regclass('pathfinder.sensor_dns_question') IS NOT NULL
 AND to_regclass('pathfinder.sensor_dns_record') IS NOT NULL
 AND to_regclass('pathfinder.sensor_conntrack_event') IS NOT NULL
 AND to_regclass('pathfinder.sensor_identity_snapshot') IS NOT NULL
 AND to_regclass('pathfinder.sensor_identity_endpoint') IS NOT NULL
 AND to_regclass('pathfinder.sensor_health') IS NOT NULL
 THEN 'C1_CLONE_TARGET_PASS' ELSE 'C1_CLONE_TARGET_FAIL' END;`
 got,err:=p.run(ctx,[]byte(query))
 if err!=nil{return fmt.Errorf("C1 PostgreSQL clone preflight: %w",err)}
 if got!="C1_CLONE_TARGET_PASS"{return fmt.Errorf("C1 PostgreSQL: clone preflight rejected: %q",got)}
 return nil
}

func(p *PGCloneExecutor) Inspect(ctx context.Context,b sensorsor.BatchSQL)(SQLBatchState,error){
 if err:=b.Validate();err!=nil{return "",err}
 if err:=p.VerifyCloneTarget(ctx);err!=nil{return "",err}
 // Same triple of source/sensor/stream identity as the historical importer.
 // Any partial metadata or completion mismatch must fail CLOSED. No SQL
 // state is inferred from a receiver receipt or from RAM.
 q:=pgInspectQuery(b)
 result,err:=p.run(ctx,[]byte(q))
 if err!=nil{return "",fmt.Errorf("C1 PostgreSQL: inspect: %w",err)}
 switch SQLBatchState(result){
 case SQLNew,SQLComplete,SQLPartial,SQLConflict:return SQLBatchState(result),nil
 default:return "",fmt.Errorf("C1 PostgreSQL: invalid batch state %q",result)
 }
}

// VerifyReadOnlyRole must be called by independent verifier B before it
// evaluates any evidence. The verifier role must have SELECT authority but
// NO table- or column-level ability to insert/update/delete/truncate or alter evidence through
// REFERENCES/TRIGGER, and no CREATE privilege in the evidence schema.
func(p *PGCloneExecutor)VerifyReadOnlyRole(ctx context.Context)error{
 if err:=p.VerifyCloneTarget(ctx);err!=nil{return err}
 query:=`SELECT CASE WHEN
  NOT has_schema_privilege(current_user,'pathfinder','CREATE')
  AND has_table_privilege(current_user,'pathfinder.sensor','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_segment','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_batch','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_batch_index_completion','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_observation','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_dns_question','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_dns_record','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_conntrack_event','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_identity_snapshot','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_identity_endpoint','SELECT')
  AND has_table_privilege(current_user,'pathfinder.sensor_health','SELECT')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_segment','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_batch','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_batch_index_completion','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_observation','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_dns_question','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_dns_record','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_conntrack_event','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_identity_snapshot','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_identity_endpoint','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_table_privilege(current_user,'pathfinder.sensor_health','INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_segment','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_batch','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_batch_index_completion','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_observation','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_dns_question','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_dns_record','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_conntrack_event','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_identity_snapshot','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_identity_endpoint','INSERT, UPDATE, REFERENCES')
  AND NOT has_any_column_privilege(current_user,'pathfinder.sensor_health','INSERT, UPDATE, REFERENCES')
  THEN 'C1_B_READONLY_ROLE_PASS' ELSE 'C1_B_READONLY_ROLE_FAIL' END;`
 result,err:=p.run(ctx,[]byte(query))
 if err!=nil{return fmt.Errorf("C1 independent B: read-only SQL role check: %w",err)}
 if result!="C1_B_READONLY_ROLE_PASS"{
  return fmt.Errorf("C1 independent B: SQL role is not read-only (%q)",result)
 }
 return nil
}

// Execute is deliberately prohibited unless the explicitly isolated clone
// has been approved by the operator. It never creates a schema or role.
func(p *PGCloneExecutor) Execute(ctx context.Context,script []byte)error{
 if p==nil||p.config.WriteAuthorization!=CloneWriteAuthorization {
  return errors.New("C1 PostgreSQL: writes disabled until isolated clone is explicitly approved")
 }
 if len(script)==0||!bytes.Contains(script,[]byte("sensor_batch_index_completion")) ||
    !bytes.Contains(script,[]byte("COMMIT;")) {
  return errors.New("C1 PostgreSQL: reject incomplete/noncanonical SQL script")
 }
 if err:=p.VerifyCloneTarget(ctx);err!=nil{return err}
 _,err:=p.run(ctx,script)
 if err!=nil{return fmt.Errorf("C1 PostgreSQL: SQL outcome uncertain; reconcile before retry: %w",err)}
 return nil
}

// CheckReference only verifies the exact clone through PostgreSQL; a separate
// independent custody resolver and FreeBSD jail mount proof are still required.
func(p *PGCloneExecutor) Verify(ctx context.Context,b sensorsor.BatchSQL)error{
 if err:=b.Validate();err!=nil{return err}
 state,err:=p.Inspect(ctx,b)
 if err!=nil{return err}
 if state!=SQLComplete{return fmt.Errorf("C1 PostgreSQL: no exact committed batch (state=%s)",state)}
 if err:=verifyRows(ctx,p.run,b);err!=nil{return fmt.Errorf("C1 PostgreSQL: committed row readback: %w",err)}
 return nil
}
var _ SoRSQLExecutor=(*PGCloneExecutor)(nil)
