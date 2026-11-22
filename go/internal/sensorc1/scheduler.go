package sensorc1

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

// FamilySink receives complete, checksummed SQL-ready source batches. A
// successful Submit is only *admission* to a later RAM/SoR handoff. It is NOT
// a PostgreSQL commit and MUST NOT release any receiver custody.
type FamilySink interface {
    Submit(context.Context,Bundle) error
}

type SchedulerOptions struct {
    MaxFamilySourceBytes int
    MaxFamilyRecords int
    MaxFamilyBatches int
    Inactivity time.Duration
}

func DefaultSchedulerOptions()SchedulerOptions{
    return SchedulerOptions{
        MaxFamilySourceBytes:16<<20,
        MaxFamilyRecords:8192,
        MaxFamilyBatches:16,
        Inactivity:30*time.Second,
    }
}

type familyBuffer struct {
    batches []Prepared
    bytes,records int
    lastActivity time.Time
}

type FamilyCounts struct {
    Batches int
    Records int
    SourceBytes int
}

// Scheduler is deliberately single-owner. The ingest process invokes Push,
// Tick and Drain sequentially; asynchronous SQL commitments belong to the
// downstream SoR controller and cannot mutate these buffers.
type Scheduler struct {
    options SchedulerOptions
    sink FamilySink
    buffers map[sensoringest.Stream]*familyBuffer
}

var orderedFamilies=[]sensoringest.Stream{
    sensoringest.Observations,sensoringest.Conntrack,
    sensoringest.Identity,sensoringest.Health,
}

func NewScheduler(options SchedulerOptions,sink FamilySink)(*Scheduler,error){
    if sink==nil{return nil,errors.New("C1 scheduler: sink required")}
    if options.MaxFamilySourceBytes<sensoringest.MaxBatchBytes ||
       options.MaxFamilySourceBytes>64<<20 ||
       options.MaxFamilyRecords<sensoringest.MaxBatchRecords ||
       options.MaxFamilyRecords>MaxBundleBatches*sensoringest.MaxBatchRecords ||
       options.MaxFamilyBatches<1 || options.MaxFamilyBatches>MaxBundleBatches ||
       options.Inactivity<=0 {
        return nil,errors.New("C1 scheduler: invalid capacity or idle bounds")
    }
    return &Scheduler{options:options,sink:sink,buffers:make(map[sensoringest.Stream]*familyBuffer)},nil
}

// Push may flush an existing family only when that family's own capacity is
// reached. Other families continue accumulating. Batch/file boundaries never
// cause a flush. Caller transfers ownership of Prepared on successful Push
// and MUST NOT modify the pointed-to Evidence tables afterward.
func(s *Scheduler)Push(ctx context.Context,p Prepared,at time.Time)error{
    if s==nil||ctx==nil{return errors.New("C1 scheduler: uninitialized")}
    if err:=ctx.Err();err!=nil{return err}
    if at.IsZero(){return errors.New("C1 scheduler: missing clock")}
    if err:=ValidatePrepared(p);err!=nil{return err}
    if p.PayloadBytes>s.options.MaxFamilySourceBytes ||
       p.Evidence.Count>s.options.MaxFamilyRecords {
        return errors.New("C1 scheduler: single source batch exceeds family limit")
    }
    if err:=s.Tick(ctx,at);err!=nil{return err}
    b:=s.buffers[p.Stream]
    if b==nil{
        b=&familyBuffer{}
        s.buffers[p.Stream]=b
    }
    if len(b.batches)>0 && (b.bytes+p.PayloadBytes>s.options.MaxFamilySourceBytes ||
       b.records+p.Evidence.Count>s.options.MaxFamilyRecords ||
       len(b.batches)+1>s.options.MaxFamilyBatches){
        if err:=s.flush(ctx,p.Stream);err!=nil{return err}
    }
    b.batches=append(b.batches,p)
    b.records+=p.Evidence.Count
    b.bytes+=p.PayloadBytes
    b.lastActivity=at
    return nil
}

// Tick flushes *only* families idle for the configured interval.
func(s *Scheduler)Tick(ctx context.Context,at time.Time)error{
    if s==nil||ctx==nil||at.IsZero(){return errors.New("C1 scheduler: invalid tick")}
    if err:=ctx.Err();err!=nil{return err}
    for _,stream:=range orderedFamilies{
        b:=s.buffers[stream]
        if b!=nil&&len(b.batches)>0&&!at.Before(b.lastActivity)&&at.Sub(b.lastActivity)>=s.options.Inactivity{
            if err:=s.flush(ctx,stream);err!=nil{return err}
        }
    }
    return nil
}

// Drain is a true end-of-input/shutdown barrier, not a source batch boundary.
// A failed Submit leaves the exact family intact for controlled retry.
func(s *Scheduler)Drain(ctx context.Context)error{
    if s==nil||ctx==nil{return errors.New("C1 scheduler: uninitialized")}
    if err:=ctx.Err();err!=nil{return err}
    for _,stream:=range orderedFamilies{
        if err:=s.flush(ctx,stream);err!=nil{return err}
    }
    return nil
}
func(s *Scheduler)Pending(stream sensoringest.Stream)FamilyCounts{
    if s==nil{return FamilyCounts{}}
    b:=s.buffers[stream]
    if b==nil{return FamilyCounts{}}
    return FamilyCounts{Batches:len(b.batches),Records:b.records,SourceBytes:b.bytes}
}
func(s *Scheduler)flush(ctx context.Context,stream sensoringest.Stream)error{
    b:=s.buffers[stream]
    if b==nil||len(b.batches)==0{return nil}
    if err:=ctx.Err();err!=nil{return err}
    bundle:=Bundle{Format:BundleFormat,Stream:stream,
        Batches:append([]Prepared(nil),b.batches...),
        SourceBytes:b.bytes,Records:b.records}
    if err:=s.sink.Submit(ctx,bundle);err!=nil{
        return fmt.Errorf("C1 scheduler: %s handoff failed, original buffer retained: %w",stream,err)
    }
    b.batches=nil
    b.bytes=0
    b.records=0
    b.lastActivity=time.Time{}
    return nil
}
