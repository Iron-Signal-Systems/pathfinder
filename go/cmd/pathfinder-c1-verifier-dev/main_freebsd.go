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
 pgPassfile:=flags.String("postgres-passfile","","optional absolute owner-only PostgreSQL PGPASSFILE; no password in environment")
 psql:=flags.String("psql","/usr/local/bin/psql","absolute PostgreSQL psql path")
 expectedFiles:=flags.Int("expect-files",0,"exact expected source file count")
 expectedRows:=flags.Int64("expect-records",0,"exact expected source record count")
 expectedManifest:=flags.String("expect-manifest-sha256","","required full original archive manifest fingerprint")
 poll:=flags.Duration("poll",5*time.Second,"verification progress poll interval")
 maxWait:=flags.Duration("max-wait-per-batch",90*time.Minute,"bounded SQL completion wait")
 maxCacheMiB:=flags.Int64("max-original-cache-mib",4096,"maximum frozen original payload bytes retained in MiB, 1-4096")
 flags.Parse(os.Args[1:])
 if flags.NArg()!=0{return errors.New("no positional arguments permitted")}
 if *archive==""||*sensor==""||*uuid==""||*socket==""||*sqlRole==""||
    *expectedFiles<1||*expectedRows<1||*poll<=0||*maxWait<=0||*expectedManifest==""{
  return errors.New("frozen source, local read-only PostgreSQL role and exact expected counts are mandatory")
 }
 if *maxCacheMiB<1||*maxCacheMiB>4096{return errors.New("source-cache-MiB must be 1..4096")}
 audit,err:=sensorc1.AuditFrozen(*archive,*sensor)
 if err!=nil{return fmt.Errorf("C1 B pinned original archive preflight: %w",err)}
 if audit.Files!=*expectedFiles||audit.Records!=*expectedRows{
  return fmt.Errorf("C1 B original archive count mismatch files=%d/%d records=%d/%d",
   audit.Files,*expectedFiles,audit.Records,*expectedRows)
 }
 fingerprint,err:=sensorc1.FrozenArchiveFingerprint(audit)
 if err!=nil{return err}
 if fingerprint!=*expectedManifest {
  return errors.New("C1 B: independently audited original archive SHA-256 differs from pinned capture")
 }
 fmt.Printf("PATHFINDER_C1_B_SOURCE_PREFLIGHT_PASS files=%d records=%d archive_manifest_sha256=%s SQL_reads=0\n",
   audit.Files,audit.Records,fingerprint)
 ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGTERM,syscall.SIGINT)
 defer stop()
 original,err:=sensorc1.NewFrozenOriginalReaderWithLimit(*archive,*sensor,*uuid,*maxCacheMiB<<20)
 if err!=nil{return fmt.Errorf("independent original source audit: %w",err)}
 postgres,err:=sensorc1.NewPGCloneExecutor(sensorc1.PGCloneConfig{
  PSQLBinary:*psql,SocketDirectory:*socket,Role:*sqlRole,
  PasswordFile:*pgPassfile,
  Database:sensorc1.CloneDatabase, // No WriteAuthorization, EVER.
 },nil)
 if err!=nil{return err}
 batches,sourceBytes,limitBytes:=original.CachedSourceStats()
 fmt.Printf("PATHFINDER_C1_B_SOURCE_CACHE batches=%d original_source_bytes=%d budget_bytes=%d authoritative=false\n",batches,sourceBytes,limitBytes)
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
