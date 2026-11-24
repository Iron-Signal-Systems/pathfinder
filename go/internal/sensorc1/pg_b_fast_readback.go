package sensorc1

// PostgreSQL-only verifier B accelerated readback. Exact source identity,
// typed parent values and every relational child are checked against ONE
// read-only transaction snapshot. This avoids repeating the parent subset
// EXCEPT ALL proof and launching psql separately for every child statement.
// Neither the SoR writer nor the independent source/custody reader changes.

import (
 "context"
 "errors"
 "fmt"
 "reflect"
 "strings"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

func(p *PGCloneExecutor) bPinnedColumns(ctx context.Context,table string)([]readbackColumn,error){
 if p==nil{return nil,errors.New("C1 B: unavailable SQL executor")}
 if cached,ok:=p.bColumnCache.Load(table);ok{
  return cached.([]readbackColumn),nil
 }
 // Check the existing schema, not a hardcoded replacement. Only catalog
 // columns approved by parentCatalogColumns can be pinned in memory.
 cols,err:=p.parentCatalogColumns(ctx,table)
 if err!=nil{return nil,err}
 existing,loaded:=p.bColumnCache.LoadOrStore(table,cols)
 if loaded && !reflect.DeepEqual(existing.([]readbackColumn),cols){
  return nil,fmt.Errorf("C1 B: SQL catalog changed while pinning %s",table)
 }
 return existing.([]readbackColumn),nil
}

// VerifyBPinnedCatalog runs after all 281 original batches have passed.
// A changed PostgreSQL parent column type during this verification session
// must not produce B acceptance, even if its initial type was cached.
func(p *PGCloneExecutor) VerifyBPinnedCatalog(ctx context.Context)error{
 if p==nil{return errors.New("C1 B: missing SQL executor")}
 var result error
 p.bColumnCache.Range(func(key,value any)bool{
  table:=key.(string)
  current,err:=p.parentCatalogColumns(ctx,table)
  if err!=nil{result=err;return false}
  if !reflect.DeepEqual(current,value.([]readbackColumn)){
   result=fmt.Errorf("C1 B: PostgreSQL typed catalog drift for %s",table)
   return false
  }
  return true
 })
 return result
}

// VerifyBCompleteAfterInspect is available only to the independent
// PostgreSQL verifier after VerifyBatch has confirmed SQLComplete with the
// exact original-source provenance. Re-inspect the committed source again
// inside a single READ ONLY transaction with complete typed and child
// evidence, keeping the result bound to one database snapshot.
func(p *PGCloneExecutor) VerifyBCompleteAfterInspect(ctx context.Context,b sensorsor.BatchSQL)error{
 if p==nil{return errors.New("C1 B: missing SQL executor")}
 if err:=b.Validate();err!=nil{return err}
 if b.Evidence==nil{return errors.New("C1 B: missing source evidence")}
 base,err:=readbackBase(b.Stream)
 if err!=nil{return err}
 fields,err:=p.bPinnedColumns(ctx,base)
 if err!=nil{return err}
 names:=[]string{base}
 if base=="sensor_observation"{
  names=append(names,"sensor_dns_question","sensor_dns_record")
 }else if base=="sensor_identity_snapshot"{
  names=append(names,"sensor_identity_endpoint")
 }
 // Full parent has ALL columns, including the subset already checked by
 // Verify's basic parent readback. No fields are dropped by omitting that
 // redundant subset. DNS and identity children still use full bidirectional
 // EXCEPT ALL comparisons, even when no rows are expected.
 queries:=make([]string,0,len(names))
 for _,name:=range names{
  table:=sqlReadback[name]
  if name==base{
   table=readbackTable{table:base,columns:fields}
  }
  var rows [][]string
  if expected:=b.Evidence.Tables[name];expected!=nil{
   if !sameReadbackSchema(expected,name){
    return fmt.Errorf("C1 B: original typed evidence schema differs for %s",name)
   }
   rows=expected.Rows
  }else if name==base {
   return errors.New("C1 B: missing original typed parent rows")
  }
  query,err:=makeReadbackQuery(table,b.BatchUUID,rows)
  if err!=nil{return err}
  queries=append(queries,query)
 }
 script:=new(strings.Builder)
 script.WriteString("BEGIN TRANSACTION READ ONLY;\n")
 script.WriteString("SELECT CASE WHEN current_database()='pathfinder_c1_clone' THEN 'C1_CLONE_TARGET_PASS' ELSE 'C1_CLONE_TARGET_FAIL' END;\n")
 script.WriteString(pgInspectQuery(b))
 for _,query:=range queries {
  script.WriteByte('\n')
  script.WriteString(query)
 }
 script.WriteString("\nROLLBACK;\n")
 output,err:=p.run(ctx,[]byte(script.String()))
 if err!=nil{return fmt.Errorf("C1 B: read-only snapshot query failed: %w",err)}
 values:=strings.Split(strings.TrimSpace(output),"\n")
 if len(values)!=2+len(queries){
  return fmt.Errorf("C1 B: incomplete read-only proof replies (%d/%d)",len(values),2+len(queries))
 }
 if strings.TrimSpace(values[0])!="C1_CLONE_TARGET_PASS" {
  return errors.New("C1 B: snapshot is not the approved isolated clone")
 }
 if strings.TrimSpace(values[1])!=string(SQLComplete){
  return fmt.Errorf("C1 B: snapshot source batch is not exactly committed (%q)",values[1])
 }
 for i,name:=range names{
  if strings.TrimSpace(values[i+2])!="PASS"{
   return fmt.Errorf("C1 B: snapshot typed evidence mismatch for %s (%q)",name,values[i+2])
  }
 }
 return nil
}
