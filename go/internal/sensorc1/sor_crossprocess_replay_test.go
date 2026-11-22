package sensorc1

import (
 "context"
 "errors"
 "fmt"
 "testing"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// scriptedCommitCollision models a second SoR process that commits the same
// original key during the first process's Execute call.
type scriptedCommitCollision struct {
 states []SQLBatchState
 reads int
 writes int
 executeErr error
}
func(s *scriptedCommitCollision)Inspect(_ context.Context,_ sensorsor.BatchSQL)(SQLBatchState,error){
 if s.reads>=len(s.states){return "",errors.New("unexpected extra SQL query")}
 x:=s.states[s.reads]
 s.reads++
 return x,nil
}
func(s *scriptedCommitCollision)Execute(_ context.Context,_ []byte)error{
 s.writes++
 return s.executeErr
}
func(s *scriptedCommitCollision)Verify(_ context.Context,_ sensorsor.BatchSQL)error{
 return errors.New("independent verifier B must not be called in SQL ACK path")
}

func TestSQLExecutionCollisionRechecksExactCommittedSource(t *testing.T){
 for _,tc:=range []struct{
  name string
  states []SQLBatchState
  success bool
 }{
  {"committed by another process",[]SQLBatchState{SQLNew,SQLComplete},true},
  {"not committed",[]SQLBatchState{SQLNew,SQLNew},false},
  {"partial metadata",[]SQLBatchState{SQLNew,SQLPartial},false},
  {"conflicting metadata",[]SQLBatchState{SQLNew,SQLConflict},false},
 }{
  t.Run(tc.name,func(t *testing.T){
   original:=originalFor(sensoringest.Health)
   p,err:=Prepare(original.Input)
   if err!=nil{t.Fatal(err)}
   plan,err:=BuildSoRBatch(p,original)
   if err!=nil{t.Fatal(err)}
   sql:=&scriptedCommitCollision{
    states:tc.states,executeErr:fmt.Errorf("concurrent SQL replay guard failed"),
   }
   proc:=&SoRProcessor{SQL:sql}
   newCommit,err:=proc.commitOne(context.Background(),plan)
   if tc.success {
    if err!=nil{t.Fatalf("replayed durable commit rejected: %v",err)}
    if newCommit{t.Fatal("process falsely claimed other process's new commit")}
   }else if err==nil{
    t.Fatal("uncertain or uncommitted database state was acknowledged")
   }
   if sql.writes!=1||sql.reads!=2{
    t.Fatalf("expected Execute then exact durable inspection; writes=%d inspections=%d",
     sql.writes,sql.reads)
   }
  })
 }
}
