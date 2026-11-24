package sensorc1

import (
 "context"
 "errors"
 "fmt"
 "sync"
 "sync/atomic"
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
 // Workers is the maximum number of independently verified batches in
 // flight. Zero preserves the historical single-worker library behavior.
 // The FreeBSD verifier entrypoint explicitly defaults to four.
 Workers int
 OnProgress func(BVerificationProgress)
}

type BVerificationProgress struct {
 Batches int64
 Records int64
 ExpectedRecords int64
 Elapsed time.Duration
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
   // A real PostgreSQL executor proves ALL typed parent columns, while
   // the minimal read-only test double uses the smaller Verify contract.
   // A real PostgreSQL executor has already proved the exact durable
   // source identity in Inspect. Keep that proof and check all typed parent
   // and relational child evidence in one read-only SQL snapshot. The
   // generic fallback and original full verifier remain unchanged.
   if full,ok:=v.SQL.(interface{VerifyBCompleteAfterInspect(context.Context,SQLBatch)error});ok{
    if err:=full.VerifyBCompleteAfterInspect(timeoutCtx,plan);err!=nil{
     return fmt.Errorf("C1 independent B: snapshot SQL proof failed: %w",err)
    }
   }else if full,ok:=v.SQL.(interface{VerifyComplete(context.Context,SQLBatch)error});ok{
    if err:=full.VerifyComplete(timeoutCtx,plan);err!=nil{
     return fmt.Errorf("C1 independent B: full typed SQL proof failed: %w",err)
    }
   }else{
    if err:=v.SQL.Verify(timeoutCtx,plan);err!=nil{
     return fmt.Errorf("C1 independent B: committed original evidence differs: %w",err)
    }
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
 if ctx==nil || expectFiles<1 || expectRecords<1 {
  return result,errors.New("C1 independent B: context and exact expected source counts required")
 }
 workers:=v.Workers
 if workers==0{workers=1}
 if workers<1||workers>16 {
  return result,errors.New("C1 independent B: verification workers must be 1..16")
 }
 result.StartedAt=time.Now()
 // No SQL evidence may be accepted until ALL nine original source files
 // have independently passed their complete read-only audit.
 audit,err:=AuditFrozen(root,sensorID)
 if err!=nil{return result,fmt.Errorf("C1 independent B: full original archive preflight: %w",err)}
 if audit.Files!=expectFiles || audit.Records!=expectRecords {
  return result,fmt.Errorf("C1 independent B: source count preflight failed files=%d/%d records=%d/%d; no SQL evidence accepted",
   audit.Files,expectFiles,audit.Records,expectRecords)
 }
 // Only complete prepared batches enter this bounded queue. ReplayFrozen
 // runs on this producer goroutine, preserving its original stored-file
 // hash and chunk-identity proofs. Workers cannot acknowledge or write SQL.
 verifyCtx,cancel:=context.WithCancel(ctx)
 defer cancel()
 jobs:=make(chan Prepared,workers)
 firstFailure:=make(chan error,1)
 var wg sync.WaitGroup
 var verifiedBatches,verifiedRecords atomic.Int64
 for i:=0;i<workers;i++{
  wg.Add(1)
  go func(){
   defer wg.Done()
   for{
    select{
    case <-verifyCtx.Done():return
    case prepared,ok:=<-jobs:
     if !ok{return}
     if err:=v.VerifyBatch(verifyCtx,prepared);err!=nil{
      select{
      case firstFailure<-fmt.Errorf("C1 independent B: batch %s: %w",prepared.BatchUUID,err):
      default:
      }
      cancel()
      return
     }
     count:=verifiedBatches.Add(1)
     rows:=verifiedRecords.Add(int64(prepared.Evidence.Count))
     if v.OnProgress!=nil && (count%10==0 || rows==expectRecords){
      v.OnProgress(BVerificationProgress{
       Batches:count,Records:rows,ExpectedRecords:expectRecords,
       Elapsed:time.Since(result.StartedAt),
      })
     }
    }
   }
  }()
 }
 summary,replayErr:=ReplayFrozen(root,sensorID,sensorUUID,func(p Prepared)error{
  // Source is already validated by ReplayFrozen, but no worker may claim
  // final acceptance until the producer finishes the exact source hash pass.
  select{
  case <-verifyCtx.Done():return verifyCtx.Err()
  case jobs<-p:return nil
  }
 })
 close(jobs)
 wg.Wait()
 select{
 case workerErr:=<-firstFailure:
  return result,workerErr
 default:
 }
 if replayErr!=nil{return result,replayErr}
 if err:=ctx.Err();err!=nil{return result,err}
 for path,digest:=range audit.StoredSHA256 {
  if summary.StoredSHA256[path]!=digest {
   return result,fmt.Errorf("C1 independent B: source %s differs from preflight audit",path)
  }
 }
 if summary.Files!=expectFiles||summary.Records!=expectRecords||
    summary.Batches!=int(verifiedBatches.Load())||
    summary.Records!=verifiedRecords.Load(){
  return result,fmt.Errorf("C1 independent B: source/SQL verification count mismatch expected=%d/%d got=%d/%d verified=%d/%d",
   expectFiles,expectRecords,summary.Files,summary.Records,
   verifiedBatches.Load(),verifiedRecords.Load())
 }
 // Any schema type drift during the run invalidates the entire B result.
 // This is also checked if the SQL executor's catalog was pinned only once
 // per family, rather than querying information_schema for each batch.
 if pinned,ok:=v.SQL.(interface{VerifyBPinnedCatalog(context.Context)error});ok{
  if err:=pinned.VerifyBPinnedCatalog(ctx);err!=nil{
   return result,fmt.Errorf("C1 independent B: final SQL catalog changed: %w",err)
  }
 }
 result.Files=summary.Files
 result.Batches=int(verifiedBatches.Load())
 result.Records=verifiedRecords.Load()
 result.CompletedAt=time.Now()
 return result,nil
}
