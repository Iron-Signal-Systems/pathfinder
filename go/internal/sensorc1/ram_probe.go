package sensorc1

import (
 "crypto/rand"
 "crypto/sha256"
 "encoding/hex"
 "errors"
 "fmt"
 "io"
 "os"
 "path/filepath"
 "regexp"
 "syscall"
)

// RAM mount probes are disposable test-only objects. They contain no sensor
// evidence, receiver custody, SQL records, private keys, or timestamps.
// The FreeBSD CLI refuses to run unless the actual jail mount is nullfs.
const RAMProbeBytes = 64
var ramProbeName = regexp.MustCompile(`^pf-c1-mount-probe-[0-9a-f]{32}\.ready$`)

type RAMProbeRef struct{
 Name string
 SHA256 string
}

// PublishRAMProbe writes 64 random bytes using the same read-only-group
// permission model as a real prepared bundle. This is a temporary RAM test,
// never receiver custody or SQL acceptance.
func PublishRAMProbe(root string,sharedGID int)(RAMProbeRef,error){
 var zero RAMProbeRef
 if err:=checkRoot(root);err!=nil{return zero,err}
 if sharedGID<1{return zero,errors.New("C1 RAM probe: non-root shared GID required")}
 var body [RAMProbeBytes]byte
 if _,err:=rand.Read(body[:]);err!=nil{return zero,err}
 var nonce [16]byte
 if _,err:=rand.Read(nonce[:]);err!=nil{return zero,err}
 name:="pf-c1-mount-probe-"+hex.EncodeToString(nonce[:])+".ready"
 sum:=sha256.Sum256(body[:])
 ref:=RAMProbeRef{Name:name,SHA256:hex.EncodeToString(sum[:])}
 temp,err:=os.CreateTemp(root,".pf-c1-probe-part-")
 if err!=nil{return zero,err}
 defer os.Remove(temp.Name())
 if err:=temp.Chown(-1,sharedGID);err!=nil{temp.Close();return zero,fmt.Errorf("C1 RAM probe: set shared group: %w",err)}
 if err:=temp.Chmod(0640);err!=nil{temp.Close();return zero,err}
 if n,err:=temp.Write(body[:]);err!=nil||n!=RAMProbeBytes{
  temp.Close()
  return zero,fmt.Errorf("C1 RAM probe: short or failed write: %d: %v",n,err)
 }
 if err:=temp.Close();err!=nil{return zero,err}
 if err:=os.Link(temp.Name(),filepath.Join(root,name));err!=nil{
  return zero,fmt.Errorf("C1 RAM probe: atomic link: %w",err)
 }
 return ref,nil
}

// ReadRAMProbe is intended to run as the SoR service identity from its
// read-only nullfs view, using the name/digest emitted by the producer.
func ReadRAMProbe(root string,ref RAMProbeRef)(error){
 if err:=checkRoot(root);err!=nil{return err}
 if !ramProbeName.MatchString(ref.Name)||len(ref.SHA256)!=64{
  return errors.New("C1 RAM probe: unsafe name or hash")
 }
 _,err:=hex.DecodeString(ref.SHA256)
 if err!=nil{return errors.New("C1 RAM probe: invalid digest")}
 path:=filepath.Join(root,ref.Name)
 stat,err:=os.Lstat(path)
 if err!=nil{return err}
 if !stat.Mode().IsRegular()||stat.Size()!=RAMProbeBytes||stat.Mode().Perm()!=0640{
  return errors.New("C1 RAM probe: object type, size or mode differs")
 }
 f,err:=os.Open(path)
 if err!=nil{return fmt.Errorf("C1 RAM probe: SoR cannot open file: %w",err)}
 defer f.Close()
 content,err:=io.ReadAll(io.LimitReader(f,RAMProbeBytes+1))
 if err!=nil{return err}
 if len(content)!=RAMProbeBytes{return errors.New("C1 RAM probe: incorrect content size")}
 actual:=sha256.Sum256(content)
 if hex.EncodeToString(actual[:])!=ref.SHA256{
  return errors.New("C1 RAM probe: contents do not match producer SHA-256")
 }
 return nil
}

// VerifyRAMProbeSharedGID checks producer-to-SoR filesystem group alignment.
// It is Unix-specific and does not create, modify, or delete files.
func VerifyRAMProbeSharedGID(root string,ref RAMProbeRef,sharedGID int)error{
 if sharedGID<1{return errors.New("C1 RAM probe: invalid shared GID")}
 if err:=ReadRAMProbe(root,ref);err!=nil{return err}
 st,err:=os.Lstat(filepath.Join(root,ref.Name))
 if err!=nil{return err}
 native,ok:=st.Sys().(*syscall.Stat_t)
 if !ok || int(native.Gid)!=sharedGID{
  return errors.New("C1 RAM probe: published file has wrong shared group")
 }
 return nil
}

// RemoveRAMProbe requires the producer's writable mount and the matching
// full digest. The consumer has no delete capability. Removal of probes is
// independent from SQL commit cleanup.
func RemoveRAMProbe(root string,ref RAMProbeRef,sharedGID int)error{
 if err:=VerifyRAMProbeSharedGID(root,ref,sharedGID);err!=nil{return err}
 return os.Remove(filepath.Join(root,ref.Name))
}
