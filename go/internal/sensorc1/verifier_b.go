package sensorc1

import (
 "context"
 "errors"
 "fmt"
 "time"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// BSQLReadOnly exposes NO SQL Execute method. Verifier B must never become
// an SQL writer, alter Source of Record evidence, or acknowledge a sensor.
type BSQLReadOnly interface{
 Inspect(context.Context,SQLBatch) (SQLBatchState,error)
 Verify(context.Context,SQLBatch)error
}

// SQLBatch is the established sensorsor.BatchSQL wire-agnostic SQL plan.
type SQLBatch = sensorsor.BatchSQL

type IndependentVerifierB struct{
 Originals OriginalSourceReader
 SQL BSQLReadOnly
 PollInterval time.Duration
 MaxWait time.Duration
}

type BVerificationResult struct{
 Files int
 Batches int
 Records int64
 StartedAt time.Time
 CompletedAt time.Time
}

func(v IndependentVerifierB)VerifyBatch(ctx context.Context,p Prepared)error{
 if ctx==nil||v.Originals==nil||v.SQL==nil {
  return errors.New("C1 independent B: original custody and read-only SQL required")
 }
 if v.PollInterval<=0||v.MaxWait<=0||v.PollInterval>v.MaxWait{
  return errors.New("C1 independent B: bounded poll schedule required")
 }
 original,err:=v.Originals.OpenOriginal(ctx,p)
 if err!=nil{return fmt.Errorf("C1 independent B: original source: %w",err)}
 plan,err:=BuildSoRBatch(p,original)
 if err!=nil{return fmt.Errorf("C1 independent B: source proof: %w",err)}
 timeoutCtx,finish:=context.WithTimeout(ctx,v.MaxWait)
 defer finish()
 tick:=time.NewTicker(v.PollInterval)
 defer tick.Stop()
 for {
  state,err:=v.SQL.Inspect(timeoutCtx,plan)
  if err!=nil{return fmt.Errorf("C1 independent B: PostgreSQL readback preflight: %w",err)}
  switch state{
  case SQLComplete:
   if err:=v.SQL.Verify(timeoutCtx,plan);err!=nil{
    return fmt.Errorf("C1 independent B: committed original evidence differs: %w",err)
   }
   return nil
  case SQLNew:
   // A still-ingesting source is not automatically a verification failure;
   // this process waits independently while SoR continues writing.
  case SQLPartial,SQLConflict:
   return fmt.Errorf("C1 independent B: %s database state requires reconciliation",state)
  default:
   return fmt.Errorf("C1 independent B: unsupported database state %q",state)
  }
  select{
  case <-timeoutCtx.Done():
   return fmt.Errorf("C1 independent B: SQL completion not observed: %w",timeoutCtx.Err())
  case <-tick.C:
  }
 }
}

// VerifyFrozen is run only by the independent verifier OS process. It makes
// an entirely separate read-only pass over all original frozen files and
// checks each exact committed batch against PostgreSQL evidence. The ingest
// process can operate in parallel with B; B does not issue any SQL writes.
func(v IndependentVerifierB)VerifyFrozen(ctx context.Context,root,sensorID,sensorUUID string,
 expectFiles int,expectRecords int64)(BVerificationResult,error){
 var result BVerificationResult
 if expectFiles<1||expectRecords<1{
  return result,errors.New("C1 independent B: exact expected source counts required")
 }
 result.StartedAt=time.Now()
 summary,err:=ReplayFrozen(root,sensorID,sensorUUID,func(p Prepared)error{
  if err:=v.VerifyBatch(ctx,p);err!=nil{return err}
  result.Batches++
  result.Records+=int64(p.Evidence.Count)
  return nil
 })
 if err!=nil{return result,err}
 if summary.Files!=expectFiles||summary.Records!=expectRecords||
    summary.Batches!=result.Batches||summary.Records!=result.Records{
  return result,fmt.Errorf("C1 independent B: source/SQL verification count mismatch expected=%d/%d got=%d/%d",
   expectFiles,expectRecords,summary.Files,summary.Records)
 }
 result.Files=summary.Files
 result.CompletedAt=time.Now()
 return result,nil
}
