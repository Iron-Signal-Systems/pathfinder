package sensorc1

import (
    "bufio"
    "compress/gzip"
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// Exactly match sensor-sor-import's frozen-capture chunking and identity.
// Replaying previously indexed evidence must not invent new source IDs.
const FrozenChunkBytes = 4 << 20
const FrozenChunkRecords = 2500

type FrozenSummary struct {
    Files        int
    Batches      int
    Records      int64
    ByStream     map[sensoringest.Stream]int64
    StoredSHA256 map[string]string
}
type frozenSegment struct {
    path   string
    stream sensoringest.Stream
    audit  sensoringest.Audit
}

// ReplayFrozen audits and prepares EVERY original record in registered source
// files, without writing to the source, receiver or database. Callback errors
// stop replay. Callback output is not authoritative custody or SQL acceptance.
func ReplayFrozen(root,sensorID,sensorUUID string,emit func(Prepared)error)(FrozenSummary,error){
    summary:=FrozenSummary{ByStream:map[sensoringest.Stream]int64{},StoredSHA256:map[string]string{}}
    if !filepath.IsAbs(root)||!safeID.MatchString(sensorID)||!uuidPattern.MatchString(sensorUUID)||emit==nil{
        return summary,errors.New("C1 frozen replay: absolute source root, sensor identity, UUID and consumer required")
    }
    segs,err:=collectFrozen(root,sensorID)
    if err!=nil{return summary,err}
    for _,seg:=range segs {
        n,err:=readFrozenSegment(seg,sensorID,sensorUUID,emit,&summary)
        if err!=nil{return summary,fmt.Errorf("C1 frozen replay: %s: %w",seg.path,err)}
        if n!=int64(seg.audit.Total){return summary,fmt.Errorf("C1 frozen replay: source segment count drift in %s: %d vs %d",seg.path,n,seg.audit.Total)}
        summary.Files++
        rel,_:=filepath.Rel(root,seg.path)
        summary.StoredSHA256[filepath.ToSlash(rel)]=seg.audit.SHA256
    }
    return summary,nil
}

func collectFrozen(root,sensorID string)([]frozenSegment,error){
    var segs []frozenSegment
    err:=filepath.WalkDir(root,func(path string,d os.DirEntry,err error)error{
        if err!=nil{return err}
        if d.IsDir(){return nil}
        if !d.Type().IsRegular(){return fmt.Errorf("non-regular archive entry %s",path)}
        base:=filepath.Base(path)
        valid:=strings.HasSuffix(base,".jsonl.gz")||strings.HasSuffix(base,".jsonl.active")
        if !valid{
            if strings.Contains(base,".jsonl")||strings.HasPrefix(base,"observations-")||
               strings.HasPrefix(base,"conntrack-")||strings.HasPrefix(base,"endpoint-identity-")||strings.HasPrefix(base,"health-"){
                return fmt.Errorf("unsupported sensor data file %s",path)
            }
            return nil // archive manifest and packaging metadata are not evidence
        }
        stream,err:=frozenStream(root,path)
        if err!=nil{return err}
        a,err:=sensoringest.AuditFile(path,stream,sensorID,time.Time{},time.Time{})
        if err!=nil{return fmt.Errorf("source audit %s: %w",path,err)}
        segs=append(segs,frozenSegment{path:path,stream:stream,audit:a})
        return nil
    })
    if err!=nil{return nil,err}
    if len(segs)==0{return nil,errors.New("C1 frozen replay: no sensor segments")}
    sort.Slice(segs,func(i,j int)bool{return segs[i].path<segs[j].path})
    return segs,nil
}

func frozenStream(root,path string)(sensoringest.Stream,error){
    rel,err:=filepath.Rel(root,path);if err!=nil{return "",err}
    parts:=strings.Split(filepath.ToSlash(rel),"/")
    if len(parts)==1&&strings.HasPrefix(parts[0],"observations-"){return sensoringest.Observations,nil}
    if len(parts)==2{
        switch parts[0]{
        case "conntrack":if strings.HasPrefix(parts[1],"conntrack-"){return sensoringest.Conntrack,nil}
        case "identity":if strings.HasPrefix(parts[1],"endpoint-identity-"){return sensoringest.Identity,nil}
        case "health":if strings.HasPrefix(parts[1],"health-"){return sensoringest.Health,nil}
        }
    }
    return "",fmt.Errorf("unsupported source location %s",rel)
}

func readFrozenSegment(seg frozenSegment,sensorID,sensorUUID string,emit func(Prepared)error,summary *FrozenSummary)(int64,error){
    f,err:=os.Open(seg.path);if err!=nil{return 0,err}
    defer f.Close()
    st,err:=f.Stat();if err!=nil{return 0,err}
    if !st.Mode().IsRegular(){return 0,errors.New("source is no longer a regular file")}
    h:=sha256.New()
    var input io.Reader=io.TeeReader(f,h)
    if strings.HasSuffix(seg.path,".jsonl.gz"){
        gz,err:=gzip.NewReader(input);if err!=nil{return 0,err}
        defer gz.Close()
        input=gz
    }
    reader:=bufio.NewReaderSize(input,64<<10)
    var ordinal int64
    start:=int64(1)
    chunk:=make([]byte,0,FrozenChunkBytes)
    rows:=0
    chunkNum:=0
    flush:=func()error{
        if rows==0{return nil}
        chunkNum++
        wireID:=fmt.Sprintf("frozen-v1-%s-%04d",seg.audit.SHA256[:32],chunkNum)
        batchUUID:=sensorsor.ID("frozen-import-batch-v1",sensorUUID,string(seg.stream),wireID)
        version:=map[sensoringest.Stream]string{
            sensoringest.Observations:"5",sensoringest.Conntrack:"1",
            sensoringest.Identity:"1",sensoringest.Health:"2",
        }[seg.stream]
        p,err:=Prepare(Input{SensorID:sensorID,SensorUUID:sensorUUID,
            Stream:seg.stream,SchemaVersion:version,WireBatchID:wireID,
            BatchUUID:batchUUID,SegmentOrdinalStart:start,Payload:chunk})
        if err!=nil{return err}
        if err:=ValidatePrepared(p);err!=nil{return err}
        if err:=emit(p);err!=nil{return err}
        summary.Batches++
        summary.Records+=int64(rows)
        summary.ByStream[seg.stream]+=int64(rows)
        start=ordinal+1
        chunk=make([]byte,0,FrozenChunkBytes)
        rows=0
        return nil
    }
    for {
        line,e:=reader.ReadBytes('\n')
        if e==io.EOF&&len(line)==0{break}
        if e!=nil{return ordinal,fmt.Errorf("truncated line at source ordinal %d: %w",ordinal+1,e)}
        if len(line)>FrozenChunkBytes{return ordinal,fmt.Errorf("source record exceeds frozen chunk size at %d",ordinal+1)}
        if rows>0&&(len(chunk)+len(line)>FrozenChunkBytes||rows>=FrozenChunkRecords){
            if err:=flush();err!=nil{return ordinal,err}
        }
        ordinal++
        rows++
        chunk=append(chunk,line...)
    }
    if err:=flush();err!=nil{return ordinal,err}
    if got:=hex.EncodeToString(h.Sum(nil));got!=seg.audit.SHA256{
        return ordinal,fmt.Errorf("stored source bytes changed: got %s audited %s",got,seg.audit.SHA256)
    }
    return ordinal,nil
}
