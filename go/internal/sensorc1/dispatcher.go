package sensorc1

import (
    "context"
    "errors"
    "fmt"
    "sync"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

const MaxOutstandingHandoffs = 16

// SQLCommitAck is a completed PostgreSQL transaction acknowledgement, not
// permission to acknowledge the original sensor. In production this message
// MUST arrive over the authenticated, replay-protected SoR control channel.
// No caller may infer durable SQL commit from transport admission alone.
type SQLCommitAck struct {
    Name        string
    SHA256      string
    Stream      sensoringest.Stream
    Records     int
    BatchUUIDs  []string
    Outcome     string // "committed" or "already_committed"
}

// SQLCommitTransport is the authenticated control path to pathfinder-sor.
// Submit returns a future for SQL COMMIT, not a pre-commit "accepted" signal.
// This interface deliberately cannot execute SQL from pathfinder-ingest.
type SQLCommitTransport interface {
    Submit(context.Context,BundleReference)(<-chan SQLCommitAck,error)
}

type Dispatcher struct {
    RAMRoot string
    transport SQLCommitTransport
    credits chan struct{}
    mu sync.Mutex
    pending sync.WaitGroup
    failure error
    sharedGID int // -1 permitted only by the isolated single-process test constructor
}

func NewDispatcher(root string,transport SQLCommitTransport)(*Dispatcher,error){
    if err:=checkRoot(root);err!=nil{return nil,err}
    if transport==nil{return nil,errors.New("C1 SQL dispatch: transport required")}
    return &Dispatcher{
        RAMRoot:root,transport:transport,
        credits:make(chan struct{},MaxOutstandingHandoffs),sharedGID:-1,
    },nil
}

// NewDispatcherForJails is the ONLY supported constructor for actual
// cross-jail development. The SoR user needs this dedicated shared group;
// the source tmpfs and both nullfs views must be independently preflighted.
func NewDispatcherForJails(root string,transport SQLCommitTransport,sharedGID int)(*Dispatcher,error){
    if sharedGID<1{return nil,errors.New("C1 SQL dispatch: dedicated shared RAM group ID required")}
    d,err:=NewDispatcher(root,transport)
    if err!=nil{return nil,err}
    d.sharedGID=sharedGID
    return d,nil
}

// Submit is compatible with Scheduler.FamilySink. It blocks when sixteen
// handoffs are still waiting on SQL commit; no unbounded RAM backlog can be
// admitted. On an uncertain SQL outcome it retains the RAM file and records
// failure. Only original custody plus PostgreSQL reconciliation can replay.
func(d *Dispatcher)Submit(ctx context.Context,b Bundle)error{
    if d==nil||ctx==nil{return errors.New("C1 SQL dispatch: unavailable")}
    if err:=ctx.Err();err!=nil{return err}
    if err:=b.Validate();err!=nil{return err}
    d.mu.Lock()
    previous:=d.failure
    d.mu.Unlock()
    if previous!=nil{return fmt.Errorf("C1 SQL dispatch: prior uncertain commit: %w",previous)}
    select{
    case d.credits<-struct{}{}:
    case <-ctx.Done():return ctx.Err()
    }
    // A prior worker may have failed while this producer waited for its
    // credit. Recheck after acquiring the credit, before publishing work.
    d.mu.Lock()
    previous=d.failure
    d.mu.Unlock()
    if previous!=nil{
        <-d.credits
        return fmt.Errorf("C1 SQL dispatch: prior uncertain commit: %w",previous)
    }
    ref,err:=publishBundleWithGroup(d.RAMRoot,b,d.sharedGID)
    if err!=nil{
        <-d.credits
        return err
    }
    ackFuture,err:=d.transport.Submit(ctx,ref)
    if err!=nil||ackFuture==nil{
        // Once a control send is attempted, failure can be an uncertain SQL
        // outcome. Never unlink the RAM object without recovery audit.
        if err==nil{err=errors.New("SQL future missing")}
        d.markFailure(err)
        <-d.credits
        return fmt.Errorf("C1 SQL dispatch: handoff uncertain, RAM retained: %w",err)
    }
    batchIDs:=make([]string,0,len(b.Batches))
    for _,p:=range b.Batches{batchIDs=append(batchIDs,p.BatchUUID)}
    d.pending.Add(1)
    go func(){
        defer d.pending.Done()
        defer func(){<-d.credits}()
        select {
        case ack,ok:=<-ackFuture:
            if !ok{
                d.markFailure(errors.New("SQL commit channel closed without ACK"))
                return
            }
            if err:=validateCommitAck(ref,batchIDs,ack);err!=nil{
                d.markFailure(err)
                return
            }
            if err:=ReclaimCommittedBundle(d.RAMRoot,ref);err!=nil{
                d.markFailure(fmt.Errorf("SQL committed but RAM cleanup failed: %w",err))
            }
        case <-ctx.Done():
            d.markFailure(fmt.Errorf("SQL ACK interrupted: %w",ctx.Err()))
        }
    }()
    return nil
}

func validateCommitAck(ref BundleReference,batchIDs []string,ack SQLCommitAck)error{
    if ack.Outcome!="committed"&&ack.Outcome!="already_committed"{
        return errors.New("C1 SQL dispatch: no durable COMMIT outcome")
    }
    if ack.Name!=ref.Name||ack.SHA256!=ref.SHA256||
       ack.Stream!=ref.Stream||ack.Records!=ref.Records||
       len(ack.BatchUUIDs)!=len(batchIDs){
        return errors.New("C1 SQL dispatch: commit ACK identity/count mismatch")
    }
    for i,id:=range batchIDs {
        if id!=ack.BatchUUIDs[i]{
            return fmt.Errorf("C1 SQL dispatch: commit ACK batch %d mismatch",i)
        }
    }
    return nil
}
func(d *Dispatcher)markFailure(err error){
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.failure==nil{d.failure=err}
}

// Wait must be called after the single ingest owner has stopped submitting.
// Context expiry is NOT permission to reclaim or acknowledge anything.
func(d *Dispatcher)Wait(ctx context.Context)error{
    if d==nil||ctx==nil{return errors.New("C1 SQL dispatch: unavailable")}
    done:=make(chan struct{})
    go func(){d.pending.Wait();close(done)}()
    select {
    case <-ctx.Done():return ctx.Err()
    case <-done:
    }
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.failure!=nil{return fmt.Errorf("C1 SQL dispatch: SQL confirmation failed: %w",d.failure)}
    return nil
}
var _ FamilySink=(*Dispatcher)(nil)
