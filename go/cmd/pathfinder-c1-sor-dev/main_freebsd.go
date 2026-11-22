//go:build freebsd && pathfinder_c1_dev

// pathfinder-c1-sor-dev is the FreeBSD DEVELOPMENT SoR control process.
// It can write ONLY to an explicitly authorized isolated PostgreSQL clone.
// It never changes the populated Pathfinder database or receiver custody.
package main

import (
 "context"
 "errors"
 "flag"
 "fmt"
 "net"
 "net/http"
 "os"
 "os/signal"
 "syscall"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorc1"
)

func main(){
 if err:=run();err!=nil{
  fmt.Fprintln(os.Stderr,"PATHFINDER C1 SOR DEVELOPMENT STOP:",err)
  os.Exit(1)
 }
}
func run()error{
 if os.Geteuid()==0{return errors.New("refuse root; dedicated non-root SoR SQL identity required")}
 flags:=flag.NewFlagSet("pathfinder-c1-sor-dev",flag.ExitOnError)
 listen:=flags.String("listen","","literal private SoR jail IP:port (no wildcard)")
 archive:=flags.String("frozen-root","","separate read-only original UDM-Pro evidence view inside SoR")
 sensor:=flags.String("sensor-id","","trusted source sensor key")
 uuid:=flags.String("sensor-uuid","","original registered PostgreSQL sensor UUID")
 ram:=flags.String("ram-dir","","SoR read-only nullfs view of host tmpfs")
 pgSocket:=flags.String("postgres-socket","","local PostgreSQL Unix socket directory")
 pgRole:=flags.String("postgres-role","","unprivileged SoR SQL role")
 psql:=flags.String("psql","/usr/local/bin/psql","absolute PostgreSQL 18 psql binary")
 cert:=flags.String("server-cert","","dedicated SoR server TLS certificate")
 key:=flags.String("server-key","","dedicated SoR TLS private key")
 trust:=flags.String("client-ca","","pinned ingest-client issuing CA")
 clientDNS:=flags.String("ingest-client-dns","","exact permitted ingest client DNS SAN")
 flags.Parse(os.Args[1:])
 if flags.NArg()!=0{return errors.New("unexpected positional arguments")}
 host,_,err:=net.SplitHostPort(*listen)
 if err!=nil||net.ParseIP(host)==nil||net.ParseIP(host).IsUnspecified(){
  return errors.New("refuse wildcard/non-literal SoR listening address")
 }
 if *archive==""||*sensor==""||*uuid==""||*ram==""||*pgSocket==""||
    *pgRole==""||*cert==""||*key==""||*trust==""||*clientDNS==""{
  return errors.New("SoR original custody, local SQL, RAM and mTLS options are mandatory")
 }
 if err:=sensorc1.RequireFreeBSDJailRAM(*ram,true);err!=nil{return err}
 approval:=os.Getenv("PATHFINDER_C1_SQL_CLONE_APPROVED")
 if approval!=sensorc1.CloneWriteAuthorization{
  return errors.New("C1 development SoR clone write authorization absent; do not start")
 }
 original,err:=sensorc1.NewFrozenOriginalReader(*archive,*sensor,*uuid)
 if err!=nil{return fmt.Errorf("independent frozen original source: %w",err)}
 postgres,err:=sensorc1.NewPGCloneExecutor(sensorc1.PGCloneConfig{
  PSQLBinary:*psql,SocketDirectory:*pgSocket,Database:sensorc1.CloneDatabase,
  Role:*pgRole,WriteAuthorization:approval,
 },nil)
 if err!=nil{return err}
 ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGTERM,syscall.SIGINT)
 defer stop()
 if err:=postgres.VerifyCloneTarget(ctx);err!=nil{return err}
 processor:=&sensorc1.SoRProcessor{ReadRoot:*ram,Originals:original,SQL:postgres}
 controller,err:=sensorc1.NewC1ControlServer(processor,*clientDNS)
 if err!=nil{return err}
 tlsConfig,err:=sensorc1.NewServerTLS(sensorc1.C1TLSFiles{
  Certificate:*cert,PrivateKey:*key,TrustCA:*trust,
 })
 if err!=nil{return err}
 server:=&http.Server{
  Addr:*listen,Handler:controller,TLSConfig:tlsConfig,
  ReadHeaderTimeout:5*time.Second,ReadTimeout:40*time.Second,
  // A slow SQL commit may outlast the ordinary request read deadline.
  WriteTimeout:10*time.Minute,IdleTimeout:60*time.Second,
  MaxHeaderBytes:16<<10,
 }
 errs:=make(chan error,1)
 go func(){errs<-server.ListenAndServeTLS("","")}()
 fmt.Printf("PATHFINDER_C1_SOR_DEV_LISTENING clone=%s populated_sor_write=false\n",sensorc1.CloneDatabase)
 select{
 case err:=<-errs:
  if errors.Is(err,http.ErrServerClosed){return nil}
  return err
 case <-ctx.Done():
  closeCtx,finish:=context.WithTimeout(context.Background(),20*time.Second)
  defer finish()
  return server.Shutdown(closeCtx)
 }
}
