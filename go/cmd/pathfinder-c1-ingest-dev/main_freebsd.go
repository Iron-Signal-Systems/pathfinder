//go:build freebsd && pathfinder_c1_dev

// pathfinder-c1-ingest-dev is an isolated FreeBSD DEVELOPMENT replay process.
// It reads original frozen UDM-Pro data, never connects to PostgreSQL and
// never acknowledges live devices. No receiver is required.
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
 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

func main(){
 if err:=run();err!=nil{
  fmt.Fprintln(os.Stderr,"PATHFINDER C1 INGEST DEVELOPMENT STOP:",err)
  os.Exit(1)
 }
}
func run()error{
 if os.Geteuid()==0{return errors.New("refuse root; use dedicated pathfinder-ingest service identity")}
 flags:=flag.NewFlagSet("pathfinder-c1-ingest-dev",flag.ExitOnError)
 archive:=flags.String("frozen-root","","read-only frozen UDM-Pro capture accessible in ingest jail")
 sensor:=flags.String("sensor-id","","existing trusted frozen source key")
 uuid:=flags.String("sensor-uuid","","existing registered PostgreSQL sensor UUID")
 ram:=flags.String("ram-dir","","ingest-writable nullfs view of host tmpfs")
 sharedGID:=flags.Int("ram-shared-gid",0,"dedicated non-root shared group GID for RAM objects")
 addr:=flags.String("sor-address","","private literal IP:port for SoR jail")
 serverDNS:=flags.String("sor-server-dns","","exact SoR mTLS certificate DNS SAN")
 cert:=flags.String("client-cert","","dedicated ingest client certificate")
 key:=flags.String("client-key","","dedicated ingest client private key")
 trust:=flags.String("server-ca","","pinned dedicated SoR CA certificate")
 expectedFiles:=flags.Int("expect-files",0,"required expected original source segments")
 expectedRecords:=flags.Int64("expect-records",0,"required expected original source records")
 expectedManifest:=flags.String("expect-manifest-sha256","","required full original archive manifest SHA-256 from read-only baseline")
 flags.Parse(os.Args[1:])
 if flags.NArg()!=0{return errors.New("unexpected positional arguments")}
 if *archive==""||*sensor==""||*uuid==""||*ram==""||*addr==""||*serverDNS==""||
    *cert==""||*key==""||*trust==""||*expectedFiles<1||*expectedRecords<1||*sharedGID<1||*expectedManifest==""{
  return errors.New("all source, RAM, mTLS and exact source-count settings required")
 }
 // Validate the ENTIRE frozen source before initiating any RAM/SQL
 // handoff. Fail closed on a missing stream, missing file or altered count.
 sourceAudit,err:=sensorc1.AuditFrozen(*archive,*sensor)
 if err!=nil{return fmt.Errorf("C1 ingest original source preflight: %w",err)}
 if sourceAudit.Files!=*expectedFiles||sourceAudit.Records!=*expectedRecords{
  return fmt.Errorf("C1 ingest source count preflight FAILED files=%d/%d records=%d/%d; no SQL work submitted",
    sourceAudit.Files,*expectedFiles,sourceAudit.Records,*expectedRecords)
 }
 fingerprint,err:=sensorc1.FrozenArchiveFingerprint(sourceAudit)
 if err!=nil{return err}
 if fingerprint!=*expectedManifest{
  return fmt.Errorf("C1 ingest source manifest SHA-256 differs from operator-pinned capture; SQL submissions=0")
 }
 for _,family:=range []sensoringest.Stream{
  sensoringest.Observations,sensoringest.Conntrack,
  sensoringest.Identity,sensoringest.Health,
 }{
  if sourceAudit.ByStream[family]<1{return fmt.Errorf("C1 ingest preflight: source family %s missing",family)}
 }
 fmt.Printf("PATHFINDER_C1_INGEST_SOURCE_PREFLIGHT_PASS files=%d records=%d archive_manifest_sha256=%s SQL_handoffs=0\n",
   sourceAudit.Files,sourceAudit.Records,fingerprint)
 if err:=sensorc1.RequireFreeBSDJailRAM(*ram,false);err!=nil{return err}
 tlsTransport,err:=sensorc1.NewC1HTTPTransport(*addr,*serverDNS,
  sensorc1.C1TLSFiles{Certificate:*cert,PrivateKey:*key,TrustCA:*trust})
 if err!=nil{return err}
 dispatch,err:=sensorc1.NewDispatcherForJails(*ram,tlsTransport,*sharedGID)
 if err!=nil{return err}
 scheduler,err:=sensorc1.NewScheduler(sensorc1.DefaultSchedulerOptions(),dispatch)
 if err!=nil{return err}
 ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGTERM,syscall.SIGINT)
 defer stop()
 began:=time.Now()
 summary,err:=sensorc1.ReplayFrozen(*archive,*sensor,*uuid,func(p sensorc1.Prepared)error{
  return scheduler.Push(ctx,p,time.Now())
 })
 if err!=nil{return err}
 if summary.Files!=*expectedFiles||summary.Records!=*expectedRecords{
  return fmt.Errorf("frozen archive count mismatch files=%d/%d records=%d/%d",
   summary.Files,*expectedFiles,summary.Records,*expectedRecords)
 }
 replayManifest,err:=sensorc1.FrozenArchiveFingerprint(summary)
 if err!=nil{return err}
 if replayManifest!=fingerprint {
  return errors.New("C1 ingest: source archive changed after pinned preflight; NO acceptance")
 }
 if err:=scheduler.Drain(ctx);err!=nil{return err}
 if err:=dispatch.Wait(ctx);err!=nil{return err}
 fmt.Printf("PATHFINDER_C1_DEV_SQL_COMMIT_ACKS_PASS files=%d batches=%d rows=%d elapsed=%s sql_hosted_in_sor=true canonical_accepted=false\n",
  summary.Files,summary.Batches,summary.Records,time.Since(began))
 return nil
}
