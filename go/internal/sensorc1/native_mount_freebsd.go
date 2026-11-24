//go:build freebsd && pathfinder_c1_dev

package sensorc1

import (
 "errors"
 "fmt"
 "strings"
 "syscall"
)

// RequireFreeBSDJailRAM enforces the expected FreeBSD nullfs jail view.
// The host MUST separately prove the underlying source is a correctly
// mounted volatile tmpfs. A nullfs type alone does not prove durability,
// isolation, host ownership or a working reboot configuration.
func RequireFreeBSDJailRAM(path string,readOnly bool)error{
 if err:=checkRoot(path);err!=nil{return err}
 var st syscall.Statfs_t
 if err:=syscall.Statfs(path,&st);err!=nil{
  return fmt.Errorf("C1 RAM: FreeBSD statfs: %w",err)
 }
 var typ strings.Builder
 for _,character:=range st.Fstypename{
  if character==0{break}
  typ.WriteByte(byte(character))
 }
 if typ.String()!="nullfs"{
  return fmt.Errorf("C1 RAM: expected FreeBSD jail nullfs, got %q",typ.String())
 }
 // FreeBSD sys/mount.h MNT_RDONLY=0x00000001; Go's stdlib syscall
 // package does not export this flag on the cross-compilation toolchain.
 const freeBSDMountReadOnly=0x00000001
 mountedReadOnly:=st.Flags&freeBSDMountReadOnly!=0
 if mountedReadOnly!=readOnly{
  return fmt.Errorf("C1 RAM: jail nullfs RO=%t expected=%t",mountedReadOnly,readOnly)
 }
 if st.Bsize<=0||st.Blocks<=0{return errors.New("C1 RAM: invalid mounted filesystem geometry")}
 return nil
}
