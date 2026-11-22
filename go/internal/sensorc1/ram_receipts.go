package sensorc1

// Pathfinder C1 native control plane: two distinct, permissioned volatile
// tmpfs mounts. Ingest writes work; SoR reads work. SoR writes SQL-commit
// receipts; ingest reads receipts. No TCP, HTTP, TLS, SQL in ingest, or disk
// journal. PostgreSQL and independently preserved original source provide
// the durable recovery authority.
import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "syscall"
    "time"
)

const RAMReceiptVersion = 1
const MaxRAMReceiptBytes = 64 << 10
const MaxRAMReceiptWorkers = MaxOutstandingHandoffs

type RAMCommitReceipt struct {
    Version int
    Reference BundleReference
    ACK SQLCommitAck
}

func receiptFileName(ref BundleReference) string { return ref.Name + ".ack" }

func sameBundleRef(a, b BundleReference) bool {
    return a.Name==b.Name && a.SHA256==b.SHA256 && a.Bytes==b.Bytes &&
        a.Stream==b.Stream && a.Batches==b.Batches && a.Records==b.Records
}

func validateRAMCommitReceipt(record RAMCommitReceipt, expected BundleReference) error {
    if err:=expected.validate();err!=nil{return err}
    if record.Version!=RAMReceiptVersion || !sameBundleRef(record.Reference,expected) {
        return errors.New("C1 RAM receipt: version or original work identity mismatch")
    }
    a:=record.ACK
    if a.Name!=expected.Name || a.SHA256!=expected.SHA256 ||
       a.Stream!=expected.Stream || a.Records!=expected.Records ||
       len(a.BatchUUIDs)!=expected.Batches ||
       (a.Outcome!="committed" && a.Outcome!="already_committed") {
        return errors.New("C1 RAM receipt: missing exact durable commit evidence")
    }
    return nil
}

func checkFileGID(info os.FileInfo, gid int) error {
    if info.Mode().Perm()!=0640 || !info.Mode().IsRegular() {
        return errors.New("C1 RAM IPC: require regular 0640 file")
    }
    stat,ok:=info.Sys().(*syscall.Stat_t)
    if !ok || int(stat.Gid)!=gid {
        return errors.New("C1 RAM IPC: file group differs from approved shared GID")
    }
    return nil
}

// ReadRAMCommitReceipt accepts a receipt ONLY from the SoR-writable,
// ingest-read-only dedicated tmpfs. Ingest also checks original batch IDs
// against this ACK before removing the matching work object.
func ReadRAMCommitReceipt(root string, ref BundleReference, gid int)(RAMCommitReceipt,error) {
    var zero RAMCommitReceipt
    if gid<1{return zero,errors.New("C1 RAM receipt: dedicated group required")}
    if err:=checkRoot(root);err!=nil{return zero,err}
    if err:=ref.validate();err!=nil{return zero,err}
    path:=filepath.Join(root,receiptFileName(ref))
    st,err:=os.Lstat(path)
    if err!=nil{return zero,err}
    if err:=checkFileGID(st,gid);err!=nil{return zero,err}
    if st.Size()<1 || st.Size()>MaxRAMReceiptBytes{
        return zero,errors.New("C1 RAM receipt: invalid bounded size")
    }
    f,err:=os.Open(path)
    if err!=nil{return zero,err}
    defer f.Close()
    fst,err:=f.Stat()
    if err!=nil{return zero,err}
    if !os.SameFile(st,fst){return zero,errors.New("C1 RAM receipt: replaced after lstat")}
    raw,err:=io.ReadAll(io.LimitReader(f,MaxRAMReceiptBytes+1))
    if err!=nil || len(raw)!=int(st.Size()){return zero,errors.New("C1 RAM receipt: incomplete or changed contents")}
    decoder:=json.NewDecoder(strings.NewReader(string(raw)))
    decoder.DisallowUnknownFields()
    var rec RAMCommitReceipt
    if err:=decoder.Decode(&rec);err!=nil{return zero,fmt.Errorf("C1 RAM receipt: malformed JSON: %w",err)}
    var trailing any
    if err:=decoder.Decode(&trailing);err!=io.EOF{return zero,errors.New("C1 RAM receipt: trailing JSON")}
    if err:=validateRAMCommitReceipt(rec,ref);err!=nil{return zero,err}
    return rec,nil
}

// WriteRAMCommitReceipt publishes a single immutable SoR-owned receipt,
// AFTER all original-source SQL transactions are durably confirmed. It
// never overwrites an existing receipt (including on duplicate retry).
func WriteRAMCommitReceipt(root string,ref BundleReference,ack SQLCommitAck,gid int)error {
    if gid<1{return errors.New("C1 RAM receipt: dedicated non-root group required")}
    if err:=checkRoot(root);err!=nil{return err}
    rec:=RAMCommitReceipt{Version:RAMReceiptVersion,Reference:ref,ACK:ack}
    if err:=validateRAMCommitReceipt(rec,ref);err!=nil{return err}
    raw,err:=json.Marshal(rec)
    if err!=nil{return err}
    if len(raw)==0 || len(raw)>MaxRAMReceiptBytes{return errors.New("C1 RAM receipt: 64 KiB ceiling exceeded")}
    tmp,err:=os.CreateTemp(root,".pf-c1-ack-part-")
    if err!=nil{return err}
    defer os.Remove(tmp.Name())
    if err:=tmp.Chown(-1,gid);err!=nil{tmp.Close();return fmt.Errorf("C1 RAM receipt: set group: %w",err)}
    if err:=tmp.Chmod(0640);err!=nil{tmp.Close();return err}
    if n,err:=tmp.Write(raw);err!=nil||n!=len(raw){
        tmp.Close()
        return errors.New("C1 RAM receipt: short write; no acknowledgment")
    }
    if err:=tmp.Close();err!=nil{return err}
    final:=filepath.Join(root,receiptFileName(ref))
    if err:=os.Link(tmp.Name(),final);err!=nil{
        if !os.IsExist(err){return err}
        prior,readErr:=ReadRAMCommitReceipt(root,ref,gid)
        if readErr!=nil{return fmt.Errorf("C1 RAM receipt: conflicting existing receipt: %w",readErr)}
        if prior.ACK.Outcome!=ack.Outcome || len(prior.ACK.BatchUUIDs)!=len(ack.BatchUUIDs){
            return errors.New("C1 RAM receipt: conflicting immutable SQL acknowledgment")
        }
        for i:=range ack.BatchUUIDs{
            if prior.ACK.BatchUUIDs[i]!=ack.BatchUUIDs[i]{
                return errors.New("C1 RAM receipt: existing SQL batch IDs differ")
            }
        }
    }
    return nil
}

type RAMReceiptTransport struct{
    root string
    gid int
    poll time.Duration
}

func NewRAMReceiptTransport(root string, gid int, poll time.Duration)(*RAMReceiptTransport,error){
    if gid<1{return nil,errors.New("C1 RAM receipt: shared non-root GID required")}
    if poll<time.Millisecond || poll>time.Second{return nil,errors.New("C1 RAM receipt: polling interval must be 1ms..1s")}
    if err:=checkRoot(root);err!=nil{return nil,err}
    return &RAMReceiptTransport{root:root,gid:gid,poll:poll},nil
}

// Submit does not send payload or control bytes. The SoR independently
// discovers .ready work objects on its RO tmpfs and writes the exact SQL
// completion receipt on a separate SoR-RW / ingest-RO tmpfs.
func(t *RAMReceiptTransport)Submit(ctx context.Context,ref BundleReference)(<-chan SQLCommitAck,error){
    if t==nil||ctx==nil{return nil,errors.New("C1 RAM receipt: transport unavailable")}
    if err:=ctx.Err();err!=nil{return nil,err}
    if err:=ref.validate();err!=nil{return nil,err}
    results:=make(chan SQLCommitAck,1)
    go func(){
        defer close(results)
        ticker:=time.NewTicker(t.poll)
        defer ticker.Stop()
        for {
            rec,err:=ReadRAMCommitReceipt(t.root,ref,t.gid)
            if err==nil{
                select{case results<-rec.ACK:case <-ctx.Done():}
                return
            }
            if !errors.Is(err,os.ErrNotExist){return} // malformed/forged ACK: fail closed
            select {
            case <-ctx.Done():return
            case <-ticker.C:
            }
        }
    }()
    return results,nil
}
var _ SQLCommitTransport=(*RAMReceiptTransport)(nil)

// DiscoverBundleFromRAM independently reconstructs a reference from a
// complete immutable work object visible to SoR. Do not trust directory
// names or ingest metadata; ProcessBundle still rechecks ORIGINAL source.
func DiscoverBundleFromRAM(root,name string,gid int)(BundleReference,error){
    var zero BundleReference
    if gid<1{return zero,errors.New("C1 RAM work: shared GID required")}
    if !handoffName.MatchString(name){return zero,errors.New("C1 RAM work: untrusted object name")}
    if err:=checkRoot(root);err!=nil{return zero,err}
    path:=filepath.Join(root,name)
    st,err:=os.Lstat(path)
    if err!=nil{return zero,err}
    if err:=checkFileGID(st,gid);err!=nil{return zero,err}
    if st.Size()<1||st.Size()>MaxHandoffBytes{return zero,errors.New("C1 RAM work: invalid bounded object size")}
    f,err:=os.Open(path)
    if err!=nil{return zero,err}
    defer f.Close()
    fst,err:=f.Stat()
    if err!=nil{return zero,err}
    if !os.SameFile(st,fst){return zero,errors.New("C1 RAM work: file replaced after lstat")}
    raw,err:=io.ReadAll(io.LimitReader(f,MaxHandoffBytes+1))
    if err!=nil||len(raw)!=int(st.Size()){return zero,errors.New("C1 RAM work: truncated while reading")}
    var b Bundle
    if err:=json.Unmarshal(raw,&b);err!=nil{return zero,fmt.Errorf("C1 RAM work: invalid bundle JSON: %w",err)}
    if err:=b.Validate();err!=nil{return zero,err}
    digest:=sha256.Sum256(raw)
    ref:=BundleReference{Name:name,SHA256:hex.EncodeToString(digest[:]),Bytes:len(raw),
        Stream:b.Stream,Batches:len(b.Batches),Records:b.Records}
    return ref,ref.validate()
}

type c1WorkResult struct{ name string; err error }

// ServeRAMWork is the native SoR-side loop. Work is never unlinked by SoR;
// only ingest can reclaim it after independently validating the exact
// SoR-created receipt. Failure stops processing without an acknowledgment.
func ServeRAMWork(ctx context.Context,workRoot,receiptRoot string,gid int,proc *SoRProcessor,poll time.Duration)error{
    if ctx==nil||proc==nil||proc.SQL==nil||proc.Originals==nil{
        return errors.New("C1 RAM SoR: SQL processor and source reader required")
    }
    if gid<1||poll<time.Millisecond||poll>time.Second{return errors.New("C1 RAM SoR: invalid shared group or poll interval")}
    if workRoot==receiptRoot{return errors.New("C1 RAM SoR: work and receipt mounts must be separate")}
    if err:=checkRoot(workRoot);err!=nil{return err}
    if err:=checkRoot(receiptRoot);err!=nil{return err}
    ownedCtx,cancel:=context.WithCancel(ctx)
    var wg sync.WaitGroup
    defer func(){cancel();wg.Wait()}()
    ticker:=time.NewTicker(poll)
    defer ticker.Stop()
    active:=make(map[string]bool)
    finished:=make(chan c1WorkResult,MaxRAMReceiptWorkers)
    scan:=func()error{
        entries,err:=os.ReadDir(workRoot)
        if err!=nil{return err}
        for _,item:=range entries{
            name:=item.Name()
            if !handoffName.MatchString(name)||active[name]{continue}
            if len(active)>=MaxRAMReceiptWorkers{break}
            active[name]=true
            wg.Add(1)
            go func(name string){
                defer wg.Done()
                err:=serveOneRAMBundle(ownedCtx,workRoot,receiptRoot,name,gid,proc)
                select{case finished<-c1WorkResult{name:name,err:err}:case <-ownedCtx.Done():}
            }(name)
        }
        return cleanupOrphanRAMReceipts(workRoot,receiptRoot)
    }
    if err:=scan();err!=nil{return err}
    for {
        select{
        case <-ctx.Done():return nil
        case result:=<-finished:
            delete(active,result.name)
            if result.err!=nil{return fmt.Errorf("C1 RAM SoR work %s: %w",result.name,result.err)}
        case <-ticker.C:
            if err:=scan();err!=nil{return err}
        }
    }
}

func serveOneRAMBundle(ctx context.Context,workRoot,receiptRoot,name string,gid int,proc *SoRProcessor)error{
    if err:=ctx.Err();err!=nil{return err}
    ref,err:=DiscoverBundleFromRAM(workRoot,name,gid)
    if err!=nil{return err}
    if _,err:=os.Lstat(filepath.Join(receiptRoot,receiptFileName(ref)));err==nil{
        _,err=ReadRAMCommitReceipt(receiptRoot,ref,gid)
        return err // already committed to SQL; never overwrite an existing ACK
    }else if !errors.Is(err,os.ErrNotExist){return err}
    ack,err:=proc.ProcessBundle(ctx,ref)
    if err!=nil{return err}
    return WriteRAMCommitReceipt(receiptRoot,ref,ack,gid)
}

func cleanupOrphanRAMReceipts(workRoot,receiptRoot string)error{
    entries,err:=os.ReadDir(receiptRoot)
    if err!=nil{return err}
    for _,item:=range entries{
        name:=item.Name()
        if !strings.HasSuffix(name,".ack"){continue}
        work:=strings.TrimSuffix(name,".ack")
        if !handoffName.MatchString(work){continue}
        if _,err:=os.Lstat(filepath.Join(workRoot,work));err==nil{continue
        }else if !errors.Is(err,os.ErrNotExist){return err}
        // A work file disappears ONLY after ingest validated its receipt.
        // Stale receipts may be removed by the SoR owner, never by ingest.
        if err:=os.Remove(filepath.Join(receiptRoot,name));err!=nil&&!errors.Is(err,os.ErrNotExist){return err}
    }
    return nil
}
