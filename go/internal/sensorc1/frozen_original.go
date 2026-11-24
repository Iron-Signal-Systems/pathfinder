package sensorc1

import (
 "context"
 "errors"
 "fmt"
 "os"
 "path/filepath"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// FrozenOriginalReader is ONLY for the isolated historical JSONL development
// replay. It is not a receiver custody trust authority or production service.
// It independently audits, scans and reconstructs batches from immutable
// operator-provided archive files. It does not trust an ingest RAM reference
// as original source provenance.
type FrozenOriginalReader struct {
 sensorID,sensorUUID string
 entries map[string]frozenOriginal
 payloadBytes int64
 maxPayloadBytes int64
}
type frozenOriginal struct {
 original OriginalBatch
 path string
 fileInfo os.FileInfo
}

// DefaultFrozenCacheBytes limits raw JSON source bytes held by each dev SoR
// or verifier process. This is a payload cap, NOT a total RSS ceiling.
const DefaultFrozenCacheBytes int64 = 4 << 30

// NewFrozenOriginalReader is the default bounded source cache for the
// development-only replay. No persistent sidecar or database is created.
func NewFrozenOriginalReader(root,sensorID,sensorUUID string)(*FrozenOriginalReader,error){
 return NewFrozenOriginalReaderWithLimit(root,sensorID,sensorUUID,DefaultFrozenCacheBytes)
}

// NewFrozenOriginalReaderWithLimit is suitable for deterministic limit tests
// and explicit operational memory budgets. The source remains authoritative;
// cached original bytes are disposable and must never authorize a receiver ACK.
func NewFrozenOriginalReaderWithLimit(root,sensorID,sensorUUID string,maxSourceBytes int64)(*FrozenOriginalReader,error){
 if maxSourceBytes<1 || maxSourceBytes>DefaultFrozenCacheBytes{
  return nil,errors.New("C1 frozen SoR: positive cache limit must be <= 4 GiB")
 }
 if !filepath.IsAbs(root)||!safeID.MatchString(sensorID)||!uuidPattern.MatchString(sensorUUID){
  return nil,errors.New("C1 frozen SoR: absolute root and trusted registered sensor identity required")
 }
 segments,err:=collectFrozen(root,sensorID)
 if err!=nil{return nil,err}
 out:=&FrozenOriginalReader{sensorID:sensorID,sensorUUID:sensorUUID,entries:map[string]frozenOriginal{},
  maxPayloadBytes:maxSourceBytes}
 for _,seg:=range segments{
  info,err:=os.Stat(seg.path)
  if err!=nil{return nil,err}
  if !info.Mode().IsRegular(){return nil,errors.New("C1 frozen SoR: nonregular source segment")}
  var batchKeys []string
  sink:=func(p Prepared)error{
   base,err:=readbackBase(string(p.Stream))
   if err!=nil{return err}
   table:=p.Evidence.Tables[base]
   if table==nil{return errors.New("C1 frozen SoR: missing original primary evidence")}
   idx:=-1
   for i,name:=range table.Names{
    if name=="raw_record"{idx=i;break}
   }
   if idx<0{return errors.New("C1 frozen SoR: no raw source column")}
   if int64(p.PayloadBytes)>out.maxPayloadBytes-out.payloadBytes {
    return fmt.Errorf("C1 frozen SoR: source payload cache budget exceeded at %s: used=%d next=%d limit=%d",
     seg.path,out.payloadBytes,p.PayloadBytes,out.maxPayloadBytes)
   }
   payload:=make([]byte,0,p.PayloadBytes)
   for _,row:=range table.Rows{
    if idx>=len(row){return errors.New("C1 frozen SoR: corrupt original row")}
    payload=append(payload,row[idx]...)
    payload=append(payload,'\n')
   }
   input:=Input{SensorID:sensorID,SensorUUID:sensorUUID,
    Stream:p.Stream,SchemaVersion:p.SchemaVersion,WireBatchID:p.WireBatchID,
    BatchUUID:p.BatchUUID,SegmentOrdinalStart:p.SegmentOrdinalStart,
    Payload:payload}
   // A complete independent raw-byte check; not a copy of prepared
   // evidence or source metadata from the untrusted inter-jail handoff.
   again,err:=Prepare(input)
   if err!=nil{return err}
   if again.PayloadSHA256!=p.PayloadSHA256 || again.Evidence.Count!=p.Evidence.Count ||
      again.PayloadBytes!=p.PayloadBytes{
    return errors.New("C1 frozen SoR: original payload reconstruction mismatch")
   }
   original:=OriginalBatch{
    Input:input,
    SegmentUUID:sensorsor.ID("frozen-import-segment-v1",sensorUUID,string(p.Stream),seg.audit.SHA256),
    SegmentFilename:filepath.Base(seg.path),
    SegmentSHA256:seg.audit.SHA256,
    SegmentReference:seg.path,
    SegmentStoredBytes:info.Size(),
    SegmentRecordCount:int64(seg.audit.Total),
    BatchCustodyReference:fmt.Sprintf("frozen-replay:%s#%s",seg.path,p.WireBatchID),
    PreservedAt:info.ModTime().UTC(),
   }
   if _,ok:=out.entries[p.BatchUUID];ok{
    return fmt.Errorf("C1 frozen SoR: duplicate original batch ID %s",p.BatchUUID)
   }
   out.entries[p.BatchUUID]=frozenOriginal{original:original,path:seg.path,fileInfo:info}
   out.payloadBytes+=int64(len(input.Payload))
   batchKeys=append(batchKeys,p.BatchUUID)
   return nil
  }
  summary:=FrozenSummary{ByStream:map[sensoringest.Stream]int64{}}
  count,err:=readFrozenSegment(seg,sensorID,sensorUUID,sink,&summary)
  if err!=nil{return nil,err}
  if count!=int64(seg.audit.Total){return nil,fmt.Errorf("C1 frozen SoR: record count drift %s",seg.path)}
  // No index entries are accepted if the stored-file SHA-256 changed during
  // this independent second scan; readFrozenSegment checks its original hash.
  if len(batchKeys)==0{return nil,fmt.Errorf("C1 frozen SoR: zero batches in %s",seg.path)}
 }
 return out,nil
}

var _ OriginalSourceReader=(*FrozenOriginalReader)(nil)

func(r *FrozenOriginalReader)OpenOriginal(ctx context.Context,p Prepared)(OriginalBatch,error){
 var zero OriginalBatch
 if r==nil||ctx==nil{return zero,errors.New("C1 frozen SoR: reader unavailable")}
 if err:=ctx.Err();err!=nil{return zero,err}
 entry,ok:=r.entries[p.BatchUUID]
 if !ok{return zero,errors.New("C1 frozen SoR: batch is not in independently audited source")}
 // A changed source file invalidates the cached scan. The separate B
 // verifier will always re-open original bytes for final acceptance.
 current,err:=os.Stat(entry.path)
 if err!=nil{return zero,err}
 if !os.SameFile(entry.fileInfo,current)||current.Size()!=entry.fileInfo.Size()||
    !current.ModTime().Equal(entry.fileInfo.ModTime()){
  return zero,errors.New("C1 frozen SoR: archive was replaced/modified after audit")
 }
 original:=entry.original
 original.Input.Payload=append([]byte(nil),original.Input.Payload...)
 return original,nil
}


// CachedSourceStats reports only source-payload retention (not process RSS,
// transient projector allocations, or Go map overhead). No mutable data leaks.
func(r *FrozenOriginalReader)CachedSourceStats()(batches int,sourceBytes int64,limitBytes int64){
 if r==nil{return 0,0,0}
 return len(r.entries),r.payloadBytes,r.maxPayloadBytes
}
