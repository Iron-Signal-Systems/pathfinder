package sensorc1

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

// Volatile RAM transport: never fsync, journal, ZFS-store or back up.
// Only receiver custody and PostgreSQL commit state are durable.
// A future FreeBSD jail deployment MUST prove host tmpfs and RW/RO nullfs.
const MaxHandoffBytes = 128 << 20

var handoffName = regexp.MustCompile(`^pf-c1-[0-9a-f]{32}\.ready$`)

type Reference struct {
    Name        string `json:"name"`
    SHA256      string `json:"sha256"`
    Bytes       int    `json:"bytes"`
    BatchUUID   string `json:"batch_uuid"`
    RecordCount int    `json:"record_count"`
}

func checkRoot(root string) error {
    if !filepath.IsAbs(root) || filepath.Clean(root)=="/" {
        return errors.New("C1 RAM: require a non-root absolute mount")
    }
    info,err:=os.Lstat(root)
    if err != nil { return fmt.Errorf("C1 RAM: mount: %w",err) }
    if !info.IsDir() || info.Mode()&os.ModeSymlink!=0 || info.Mode().Perm()&0007!=0 {
        return errors.New("C1 RAM: mount must be a real non-world-accessible directory")
    }
    return nil
}

func (r Reference) validate() error {
    if !handoffName.MatchString(r.Name) || r.Bytes<1 || r.Bytes>MaxHandoffBytes ||
       r.RecordCount<1 || r.RecordCount>25000 || !uuidPattern.MatchString(r.BatchUUID) ||
       len(r.SHA256)!=64 || strings.ToLower(r.SHA256)!=r.SHA256 {
        return errors.New("C1 RAM: invalid handoff reference")
    }
    _,err:=hex.DecodeString(r.SHA256)
    return err
}

// Publish atomically makes one SQL-ready prepared object available to the
// SoR consumer. The future mTLS control channel must bind this exact reference.
// Publishing is NOT a SQL commit or a receiver ACK.
func Publish(root string,p Prepared)(Reference,error){
    var zero Reference
    if err:=checkRoot(root);err!=nil{return zero,err}
    if err:=ValidatePrepared(p);err!=nil{return zero,err}
    encoded,err:=json.Marshal(p)
    if err!=nil{return zero,err}
    if len(encoded)==0 || len(encoded)>MaxHandoffBytes {
        return zero,errors.New("C1 RAM: prepared object exceeds 128 MiB")
    }
    var nonce [16]byte
    if _,err:=rand.Read(nonce[:]);err!=nil{return zero,err}
    name:="pf-c1-"+hex.EncodeToString(nonce[:])+".ready"
    sum:=sha256.Sum256(encoded)
    reference:=Reference{Name:name,SHA256:hex.EncodeToString(sum[:]),Bytes:len(encoded),BatchUUID:p.BatchUUID,RecordCount:p.Evidence.Count}
    temp,err:=os.CreateTemp(root,".pf-c1-part-")
    if err!=nil{return zero,err}
    defer os.Remove(temp.Name())
    if err:=temp.Chmod(0640);err!=nil{temp.Close();return zero,err}
    n,err:=temp.Write(encoded)
    if err!=nil || n!=len(encoded){temp.Close();return zero,fmt.Errorf("C1 RAM: write interrupted: %v",err)}
    if err:=temp.Close();err!=nil{return zero,err}
    if err:=os.Rename(temp.Name(),filepath.Join(root,name));err!=nil{return zero,err}
    return reference,nil
}

// Read verifies exact size, SHA-256, source identity and record count before
// returning SQL-ready evidence. The eventual fi-style SoR jail must see a
// read-only nullfs mapping of the same host tmpfs directory.
func Read(root string,r Reference)(Prepared,error){
    var zero Prepared
    if err:=checkRoot(root);err!=nil{return zero,err}
    if err:=r.validate();err!=nil{return zero,err}
    path:=filepath.Join(root,r.Name)
    info,err:=os.Lstat(path)
    if err!=nil{return zero,err}
    if !info.Mode().IsRegular() || info.Size()!=int64(r.Bytes) {
        return zero,errors.New("C1 RAM: missing, unsafe or changed object")
    }
    f,err:=os.Open(path)
    if err!=nil{return zero,err}
    defer f.Close()
    contents,err:=io.ReadAll(io.LimitReader(f,int64(r.Bytes)+1))
    if err!=nil{return zero,err}
    if len(contents)!=r.Bytes{return zero,errors.New("C1 RAM: wrong object size")}
    digest:=sha256.Sum256(contents)
    if hex.EncodeToString(digest[:])!=r.SHA256 {
        return zero,errors.New("C1 RAM: handoff SHA-256 mismatch")
    }
    var prepared Prepared
    if err:=json.Unmarshal(contents,&prepared);err!=nil{
        return zero,fmt.Errorf("C1 RAM: malformed prepared object: %w",err)
    }
    if prepared.BatchUUID!=r.BatchUUID || prepared.Evidence==nil || prepared.Evidence.Count!=r.RecordCount {
        return zero,errors.New("C1 RAM: handoff identity/count mismatch")
    }
    if err:=ValidatePrepared(prepared);err!=nil{return zero,err}
    return prepared,nil
}

// No general-purpose delete API: the producer may only reclaim after a
// separately authenticated matching SoR SQL COMMIT ACK, not after Read.
