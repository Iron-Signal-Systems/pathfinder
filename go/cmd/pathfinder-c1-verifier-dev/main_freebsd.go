//go:build freebsd && pathfinder_c1_dev

// pathfinder-c1-verifier-dev is a DISTINCT FreeBSD read-only OS process B.
// It verifies committed PostgreSQL against frozen original sensor source
// while the separate pathfinder-ingest and pathfinder-sor writers continue.
package main

import (
 "context"
 "errors"
 "flag"
 "fmt"
 "os"
 "os/signal"
 "syscall"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorc1"
)

func main(){
 if err:=run();err!=nil{
  fmt.Fprintln(os.Stderr,"PATHFINDER C1 INDEPENDENT B STOP:",err)
  os.Exit(1)
 }
}
func run()error{
 if os.Geteuid()==0{return errors.New("refuse root; use dedicated unprivileged read-only verifier identity")}
 flags:=flag.NewFlagSet("pathfinder-c1-verifier-dev",flag.ExitOnError)
 archive:=flags.String("frozen-root","","read-only original frozen source archive")
 sensor:=flags.String("sensor-id","","registered source identity")
 uuid:=flags.String("sensor-uuid","","registered SQL sensor UUID")
 socket:=flags.String("postgres-socket","","local PostgreSQL Unix socket directory")
 sqlRole:=flags.String("postgres-role","","read-only PostgreSQL verifier role")
 psql:=flags.String("psql","/usr/local/bin/psql","absolute PostgreSQL psql path")
 expectedFiles:=flags.Int("expect-files",0,"exact expected source file count")
 expectedRows:=flags.Int64("expect-records",0,"exact expected source record count")
 poll:=flags.Duration("poll",5*time.Second,"verification progress poll interval")
 maxWait:=flags.Duration("max-wait-per-batch",90*time.Minute,"bounded SQL completion wait")
 flags.Parse(os.Args[1:])
 if flags.NArg()!=0{return errors.New("no positional arguments permitted")}
 if *archive==""||*sensor==""||*uuid==""||*socket==""||*sqlRole==""||
    *expectedFiles<1||*expectedRows<1||*poll<=0||*maxWait<=0{
  return errors.New("frozen source, local read-only PostgreSQL role and exact expected counts are mandatory")
 }
 ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGTERM,syscall.SIGINT)
 defer stop()
 original,err:=sensorc1.NewFrozenOriginalReader(*archive,*sensor,*uuid)
 if err!=nil{return fmt.Errorf("independent original source audit: %w",err)}
 postgres,err:=sensorc1.NewPGCloneExecutor(sensorc1.PGCloneConfig{
  PSQLBinary:*psql,SocketDirectory:*socket,Role:*sqlRole,
  Database:sensorc1.CloneDatabase, // No WriteAuthorization, EVER.
 },nil)
 if err!=nil{return err}
 if err:=postgres.VerifyReadOnlyRole(ctx);err!=nil{return err}
 verifier:=sensorc1.IndependentVerifierB{
  Originals:original,SQL:postgres,
  PollInterval:*poll,MaxWait:*maxWait,
 }
 result,err:=verifier.VerifyFrozen(ctx,*archive,*sensor,*uuid,*expectedFiles,*expectedRows)
 if err!=nil{return err}
 fmt.Printf("PATHFINDER_C1_B_VERIFICATION_PASS files=%d batches=%d records=%d duration=%s clone=%s writes=0 canonical_accepted=false\n",
  result.Files,result.Batches,result.Records,
  result.CompletedAt.Sub(result.StartedAt),sensorc1.CloneDatabase)
 return nil
}
