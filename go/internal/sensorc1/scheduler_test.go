package sensorc1

import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

type testSink struct {
    bundles []Bundle
    reject bool
}
func(s *testSink)Submit(_ context.Context,b Bundle)error{
    if s.reject{return errors.New("downstream unavailable")}
    if err:=b.Validate();err!=nil{return err}
    s.bundles=append(s.bundles,b)
    return nil
}

func secondPrepared(t *testing.T,stream sensoringest.Stream)Prepared{
    t.Helper()
    in:=inputFor(stream)
    in.WireBatchID="batch-b"
    in.BatchUUID=sensorsor.ID("frozen-import-batch-v1",in.SensorUUID,string(stream),in.WireBatchID)
    p,err:=Prepare(in)
    if err!=nil{t.Fatal(err)}
    return p
}

func TestIndependentContinuousBuffersAndIdle(t *testing.T){
    sink:=&testSink{}
    s,err:=NewScheduler(DefaultSchedulerOptions(),sink)
    if err!=nil{t.Fatal(err)}
    ctx:=context.Background()
    now:=time.Date(2026,11,22,12,0,0,0,time.UTC)
    one,err:=Prepare(inputFor(sensoringest.Observations));if err!=nil{t.Fatal(err)}
    if err:=s.Push(ctx,one,now);err!=nil{t.Fatal(err)}
    two:=secondPrepared(t,sensoringest.Observations)
    if err:=s.Push(ctx,two,now.Add(10*time.Second));err!=nil{t.Fatal(err)}
    conn,err:=Prepare(inputFor(sensoringest.Conntrack));if err!=nil{t.Fatal(err)}
    if err:=s.Push(ctx,conn,now.Add(5*time.Second));err!=nil{t.Fatal(err)}
    if len(sink.bundles)!=0{t.Fatal("batch boundary forced a flush")}
    if got:=s.Pending(sensoringest.Observations);got.Batches!=2 || got.Records!=2 {
        t.Fatalf("observation family not continuous: %+v",got)
    }
    if err:=s.Tick(ctx,now.Add(35*time.Second));err!=nil{t.Fatal(err)}
    if len(sink.bundles)!=1 || sink.bundles[0].Stream!=sensoringest.Conntrack{
        t.Fatalf("wrong independent idle flush: %+v",sink.bundles)
    }
    if s.Pending(sensoringest.Observations).Batches!=2{t.Fatal("unrelated family was drained")}
    if err:=s.Drain(ctx);err!=nil{t.Fatal(err)}
    if len(sink.bundles)!=2 || sink.bundles[1].Stream!=sensoringest.Observations ||
       len(sink.bundles[1].Batches)!=2{t.Fatal("true end did not flush cumulative observations")}
    if s.Pending(sensoringest.Observations).Records!=0{t.Fatal("drained rows still pending")}
}

func TestFamilyCapacityDoesNotFlushOthers(t *testing.T){
    opt:=DefaultSchedulerOptions()
    opt.MaxFamilyBatches=2
    sink:=&testSink{}
    s,err:=NewScheduler(opt,sink);if err!=nil{t.Fatal(err)}
    ctx:=context.Background()
    now:=time.Now()
    health,err:=Prepare(inputFor(sensoringest.Health));if err!=nil{t.Fatal(err)}
    if err:=s.Push(ctx,health,now);err!=nil{t.Fatal(err)}
    for _,wire:=range []string{"batch-a","batch-b","batch-c"}{
        in:=inputFor(sensoringest.Conntrack)
        in.WireBatchID=wire
        in.BatchUUID=sensorsor.ID("frozen-import-batch-v1",in.SensorUUID,string(in.Stream),wire)
        p,err:=Prepare(in);if err!=nil{t.Fatal(err)}
        if err:=s.Push(ctx,p,now.Add(1*time.Second));err!=nil{t.Fatal(err)}
    }
    if len(sink.bundles)!=1 || sink.bundles[0].Stream!=sensoringest.Conntrack ||
       len(sink.bundles[0].Batches)!=2{t.Fatal("capacity did not flush exact same family")}
    if s.Pending(sensoringest.Health).Batches!=1{t.Fatal("unrelated health family drained")}
    if s.Pending(sensoringest.Conntrack).Batches!=1{t.Fatal("third batch missing")}
}

func TestFailedSubmissionPreservesExactBufferedWork(t *testing.T){
    sink:=&testSink{reject:true}
    s,err:=NewScheduler(DefaultSchedulerOptions(),sink);if err!=nil{t.Fatal(err)}
    ctx:=context.Background()
    at:=time.Now()
    p,err:=Prepare(inputFor(sensoringest.Identity));if err!=nil{t.Fatal(err)}
    if err:=s.Push(ctx,p,at);err!=nil{t.Fatal(err)}
    if err:=s.Tick(ctx,at.Add(time.Minute));err==nil{t.Fatal("lost rejected submission")}
    if got:=s.Pending(sensoringest.Identity);got.Batches!=1 || got.Records!=1 {
        t.Fatalf("failed buffer not retained: %+v",got)
    }
    sink.reject=false
    if err:=s.Drain(ctx);err!=nil{t.Fatal(err)}
    if len(sink.bundles)!=1 || sink.bundles[0].Batches[0].BatchUUID!=p.BatchUUID{
        t.Fatal("retry lost original identity")
    }
}

func TestCanceledContextKeepsPending(t *testing.T){
    sink:=&testSink{}
    s,err:=NewScheduler(DefaultSchedulerOptions(),sink);if err!=nil{t.Fatal(err)}
    at:=time.Now()
    p,err:=Prepare(inputFor(sensoringest.Health));if err!=nil{t.Fatal(err)}
    if err:=s.Push(context.Background(),p,at);err!=nil{t.Fatal(err)}
    ctx,cancel:=context.WithCancel(context.Background());cancel()
    if err:=s.Drain(ctx);err==nil{t.Fatal("canceled drain succeeded")}
    if s.Pending(sensoringest.Health).Batches!=1{t.Fatal("cancel discarded buffer")}
}
