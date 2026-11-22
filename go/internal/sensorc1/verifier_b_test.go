package sensorc1

import (
 "context"
 "errors"
 "testing"
 "os"
 "path/filepath"
 "strings"
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

func TestIndependentBRefusesPartialFrozenSourceBeforeAnySQL(t *testing.T){
 root:=t.TempDir()
 path:=filepath.Join(root,"health-ignored.jsonl.active")
 // Use the canonical source path for the original sensor health family.
 path=filepath.Join(root,"health","health-valid.jsonl.active")
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(path,[]byte(cases[sensoringest.Health].raw+"\n"),0600);err!=nil{t.Fatal(err)}
 sql:=&stagedBSQL{firstState:SQLComplete}
 b:=IndependentVerifierB{SQL:sql,PollInterval:time.Millisecond,MaxWait:time.Second}
 _,err:=b.VerifyFrozen(context.Background(),root,sensorID,sensorUUID,9,685218)
 if err==nil||!strings.Contains(err.Error(),"source count preflight failed"){
  t.Fatalf("missing incomplete-archive refusal: %v",err)
 }
 if sql.polls!=0||sql.verifyCalls!=0{
  t.Fatalf("partial original archive reached SQL readback: %+v",sql)
 }
}
