package sensorc1

import (
 "context"
 "errors"
 "testing"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

type stagedBSQL struct{
 polls int
 verifyCalls int
 firstState SQLBatchState
 corrupt bool
}
func(s *stagedBSQL)Inspect(_ context.Context,_ sensorsor.BatchSQL)(SQLBatchState,error){
 s.polls++
 if s.polls<=2{return s.firstState,nil}
 return SQLComplete,nil
}
func(s *stagedBSQL)Verify(_ context.Context,_ sensorsor.BatchSQL)error{
 s.verifyCalls++
 if s.corrupt{return errors.New("original source differs from SQL")}
 return nil
}
func TestIndependentBWaitsForCommittedSQLWithoutWriting(t *testing.T){
 original:=originalFor(sensoringest.Observations)
 p,err:=Prepare(original.Input)
 if err!=nil{t.Fatal(err)}
 sql:=&stagedBSQL{firstState:SQLNew}
 b:=IndependentVerifierB{
  Originals:testOriginalReader{values:map[string]OriginalBatch{p.BatchUUID:original}},
  SQL:sql,PollInterval:time.Millisecond,MaxWait:time.Second,
 }
 if err:=b.VerifyBatch(context.Background(),p);err!=nil{t.Fatal(err)}
 if sql.polls!=3||sql.verifyCalls!=1{
  t.Fatalf("independent B did not poll and prove committed SQL: %+v",sql)
 }
}
func TestIndependentBRejectsPartialOrCorruptCommittedSQL(t *testing.T){
 original:=originalFor(sensoringest.Identity)
 p,err:=Prepare(original.Input)
 if err!=nil{t.Fatal(err)}
 for _,state:=range []SQLBatchState{SQLPartial,SQLConflict}{
  sql:=&stagedBSQL{firstState:state}
  b:=IndependentVerifierB{
   Originals:testOriginalReader{values:map[string]OriginalBatch{p.BatchUUID:original}},
   SQL:sql,PollInterval:time.Millisecond,MaxWait:time.Second,
  }
  if err:=b.VerifyBatch(context.Background(),p);err==nil{
   t.Fatalf("independent B accepted %s state",state)
  }
  if sql.verifyCalls!=0{t.Fatal("B attempted proof over incomplete database")}
 }
 sql:=&stagedBSQL{firstState:SQLComplete,corrupt:true}
 b:=IndependentVerifierB{
  Originals:testOriginalReader{values:map[string]OriginalBatch{p.BatchUUID:original}},
  SQL:sql,PollInterval:time.Millisecond,MaxWait:time.Second,
 }
 if err:=b.VerifyBatch(context.Background(),p);err==nil{
  t.Fatal("independent B falsely accepted corrupt SQL source evidence")
 }
}
func TestIndependentBRequiresBoundedWaitAndOriginals(t *testing.T){
 p,err:=Prepare(inputFor(sensoringest.Health));if err!=nil{t.Fatal(err)}
 b:=IndependentVerifierB{}
 if err:=b.VerifyBatch(context.Background(),p);err==nil{
  t.Fatal("unconfigured verification accepted")
 }
}
