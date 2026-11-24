//go:build freebsd && pathfinder_c1_dev

// pathfinder-c1-ram-probe-dev proves the actual group-readable host tmpfs
// handoff through the two separate FreeBSD jail identities. It creates only
// a disposable 64-byte random probe, never receiver/SQL/sensor data.
package main

import (
 "errors"
 "flag"
 "fmt"
 "os"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorc1"
)

func main(){
 if err:=run();err!=nil{
  fmt.Fprintln(os.Stderr,"PATHFINDER_C1_RAM_PROBE_FAIL:",err)
  os.Exit(1)
 }
}
func run()error{
 if os.Geteuid()==0{return errors.New("run as the unprivileged ingest or SoR service identity, never root")}
 flags:=flag.NewFlagSet("pathfinder-c1-ram-probe-dev",flag.ExitOnError)
 op:=flags.String("operation","","publish, read, or remove")
 path:=flags.String("ram-dir","","absolute jail-visible shared RAM nullfs directory")
 gid:=flags.Int("shared-gid",0,"dedicated shared RAM group, not root")
 name:=flags.String("name","","exact published random probe name")
 digest:=flags.String("sha256","","exact SHA-256 received from ingest probe")
 flags.Parse(os.Args[1:])
 if flags.NArg()!=0{return errors.New("positional arguments forbidden")}
 if *path==""||*gid<1{return errors.New("RAM directory and dedicated positive shared GID required")}
 readOnly:=*op=="read"
 if *op!="publish"&&*op!="read"&&*op!="remove"{
  return errors.New("operation must be publish, read, or remove")
 }
 if err:=sensorc1.RequireFreeBSDJailRAM(*path,readOnly);err!=nil{return err}
 if *op=="publish"{
  if *name!=""||*digest!=""{return errors.New("publishing does not accept an existing reference")}
  ref,err:=sensorc1.PublishRAMProbe(*path,*gid)
  if err!=nil{return err}
  fmt.Printf("PATHFINDER_C1_RAM_PROBE_PUBLISHED name=%s sha256=%s bytes=64 custody=false sql=false\n",ref.Name,ref.SHA256)
  return nil
 }
 ref:=sensorc1.RAMProbeRef{Name:*name,SHA256:*digest}
 if *op=="read"{
  if err:=sensorc1.VerifyRAMProbeSharedGID(*path,ref,*gid);err!=nil{return err}
  fmt.Printf("PATHFINDER_C1_RAM_PROBE_READ_PASS name=%s group=%d sql_writes=0\n",ref.Name,*gid)
  return nil
 }
 if err:=sensorc1.RemoveRAMProbe(*path,ref,*gid);err!=nil{return err}
 fmt.Printf("PATHFINDER_C1_RAM_PROBE_REMOVED name=%s remaining=0\n",ref.Name)
 return nil
}
