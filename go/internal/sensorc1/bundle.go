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

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

// A Bundle contains complete batch projections from exactly one processing
// family. There is no batch-boundary flush: each family's buffer may contain
// several independently attributable source batches.
const BundleFormat = "pathfinder-c1-family-bundle-v1"
const MaxBundleBatches = 256

type Bundle struct {
    Format       string                `json:"format"`
    Stream       sensoringest.Stream    `json:"stream"`
    Batches      []Prepared            `json:"batches"`
    SourceBytes  int                   `json:"source_bytes"`
    Records      int                   `json:"records"`
}

func (b Bundle) Validate() error {
    if b.Format!=BundleFormat || len(b.Batches)<1 || len(b.Batches)>MaxBundleBatches ||
       b.SourceBytes<1 || b.Records<1 || b.Records>MaxBundleBatches*sensoringest.MaxBatchRecords {
        return errors.New("C1 bundle: invalid format or bounds")
    }
    sourceBytes,records:=0,0
    seen:=make(map[string]struct{},len(b.Batches))
    for i,p:=range b.Batches {
        if p.Stream!=b.Stream { return fmt.Errorf("C1 bundle: cross-family batch at index %d",i) }
        if _,ok:=seen[p.BatchUUID];ok{return fmt.Errorf("C1 bundle: duplicate source batch at index %d",i)}
        seen[p.BatchUUID]=struct{}{}
        if err:=ValidatePrepared(p);err!=nil{return fmt.Errorf("C1 bundle: batch %d: %w",i,err)}
        sourceBytes+=p.PayloadBytes
        records+=p.Evidence.Count
    }
    if sourceBytes!=b.SourceBytes || records!=b.Records{return errors.New("C1 bundle: source byte/record totals differ")}
    return nil
}

// BundleReference is an authenticated-control-plane value in the eventual
// two-jail service. It does NOT represent SQL COMMIT or source custody.
type BundleReference struct {
    Name    string             `json:"name"`
    SHA256  string             `json:"sha256"`
    Bytes   int                `json:"bytes"`
    Stream  sensoringest.Stream `json:"stream"`
    Batches int                `json:"batches"`
    Records int                `json:"records"`
}

func (r BundleReference) validate()error{
    if !handoffName.MatchString(r.Name) || r.Bytes<=0 || r.Bytes>MaxHandoffBytes ||
       r.Batches<=0 || r.Batches>MaxBundleBatches ||
       r.Records<=0 || r.Records>MaxBundleBatches*sensoringest.MaxBatchRecords ||
       len(r.SHA256)!=64 { return errors.New("C1 bundle: invalid reference") }
    _,err:=hex.DecodeString(r.SHA256)
    if err!=nil{return err}
    if hex.EncodeToString(mustSHABytes(r.SHA256))!=r.SHA256 {
        return errors.New("C1 bundle: noncanonical checksum")
    }
    return nil
}
func mustSHABytes(hexDigest string)[]byte{
    b,_:=hex.DecodeString(hexDigest);return b
}

// PublishBundle is intentionally volatile, with an atomic no-overwrite link.
// Before jail deployment, an independent native FreeBSD check must prove
// root is host tmpfs presented RW only to the ingest jail.
// PublishBundle is for single-process isolated tests; it inherits the
// producer group. FreeBSD cross-jail services use PublishBundleWithGroup.
func PublishBundle(root string,b Bundle)(BundleReference,error){
    return publishBundleWithGroup(root,b,-1)
}

// PublishBundleWithGroup assigns the SoR-readable shared group before the
// object becomes visible. Never fall back to world-readable permissions.
func PublishBundleWithGroup(root string,b Bundle,sharedGID int)(BundleReference,error){
    if sharedGID<1{return BundleReference{},errors.New("C1 bundle: non-root shared RAM GID required")}
    return publishBundleWithGroup(root,b,sharedGID)
}
func publishBundleWithGroup(root string,b Bundle,sharedGID int)(BundleReference,error){
    var zero BundleReference
    if err:=checkRoot(root);err!=nil{return zero,err}
    if err:=b.Validate();err!=nil{return zero,err}
    raw,err:=json.Marshal(b)
    if err!=nil{return zero,err}
    if len(raw)==0 || len(raw)>MaxHandoffBytes{return zero,errors.New("C1 bundle: 128 MiB encoded object ceiling")}
    var nonce [16]byte
    if _,err=rand.Read(nonce[:]);err!=nil{return zero,err}
    name:="pf-c1-"+hex.EncodeToString(nonce[:])+".ready"
    digest:=sha256.Sum256(raw)
    ref:=BundleReference{Name:name,SHA256:hex.EncodeToString(digest[:]),Bytes:len(raw),
        Stream:b.Stream,Batches:len(b.Batches),Records:b.Records}
    tmp,err:=os.CreateTemp(root,".pf-c1-part-")
    if err!=nil{return zero,err}
    defer os.Remove(tmp.Name())
    if sharedGID>=0 {
        if err:=tmp.Chown(-1,sharedGID);err!=nil{
            tmp.Close()
            return zero,fmt.Errorf("C1 bundle: cannot set shared RAM group: %w",err)
        }
    }
    if err:=tmp.Chmod(0640);err!=nil{tmp.Close();return zero,err}
    n,err:=tmp.Write(raw)
    if err!=nil||n!=len(raw){tmp.Close();return zero,fmt.Errorf("C1 bundle: temporary write failed: %v",err)}
    if err:=tmp.Close();err!=nil{return zero,err}
    if err:=os.Link(tmp.Name(),filepath.Join(root,name));err!=nil{return zero,err}
    return ref,nil
}

// ReadBundle runs in the eventual SoR service, using its RO nullfs view.
// Rechecking typed evidence binds all projected children to original JSONL.
func ReadBundle(root string,r BundleReference)(Bundle,error){
    var zero Bundle
    if err:=checkRoot(root);err!=nil{return zero,err}
    if err:=r.validate();err!=nil{return zero,err}
    path:=filepath.Join(root,r.Name)
    st,err:=os.Lstat(path);if err!=nil{return zero,err}
    if !st.Mode().IsRegular() || st.Size()!=int64(r.Bytes){
        return zero,errors.New("C1 bundle: missing or changed object")
    }
    f,err:=os.Open(path);if err!=nil{return zero,err}
    defer f.Close()
    raw,err:=io.ReadAll(io.LimitReader(f,int64(r.Bytes)+1))
    if err!=nil{return zero,err}
    if len(raw)!=r.Bytes{return zero,errors.New("C1 bundle: truncated or grown object")}
    h:=sha256.Sum256(raw)
    if hex.EncodeToString(h[:])!=r.SHA256{return zero,errors.New("C1 bundle: object hash mismatch")}
    var b Bundle
    if err:=json.Unmarshal(raw,&b);err!=nil{return zero,err}
    if b.Stream!=r.Stream || len(b.Batches)!=r.Batches || b.Records!=r.Records{
        return zero,errors.New("C1 bundle: reference and payload identity mismatch")
    }
    if err:=b.Validate();err!=nil{return zero,err}
    return b,nil
}

// ReclaimCommittedBundle is the ONLY publication-side unlink path. It must
// be called exclusively after verifying an authentic matching SoR SQL COMMIT
// ACK for the exact reference. It does not itself verify that external ACK.
func ReclaimCommittedBundle(root string,ref BundleReference)error{
    if err:=checkRoot(root);err!=nil{return err}
    if err:=ref.validate();err!=nil{return err}
    return os.Remove(filepath.Join(root,ref.Name))
}
