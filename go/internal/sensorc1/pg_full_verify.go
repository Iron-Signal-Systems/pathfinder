package sensorc1

import (
 "context"
 "errors"
 "fmt"
 "strings"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

// Verifier B requires every typed parent column, not only raw JSON, source
// identity and relational child rows. Use PG's OWN catalog types so database
// upgrades cannot silently drift from the established typed projectors.
var allowedPGType = map[string]string{
 "uuid":"uuid", "timestamptz":"timestamptz",
 "timestamp":"timestamp", "text":"text",
 "int4":"integer", "int8":"bigint",
 "bool":"boolean", "inet":"inet", "macaddr":"macaddr",
 "jsonb":"jsonb", "bpchar":"character",
}

func(p *PGCloneExecutor) VerifyComplete(ctx context.Context,b sensorsor.BatchSQL)error{
 // Exact durable metadata, source raw bytes and ALL child fields are already
 // checked by Verify. This slow extra proof belongs only in independent B.
 if err:=p.Verify(ctx,b);err!=nil{return err}
 name,err:=readbackBase(b.Stream)
 if err!=nil{return err}
 fields,err:=p.parentCatalogColumns(ctx,name)
 if err!=nil{return err}
 source:=b.Evidence.Tables[name]
 if source==nil||!sameReadbackSchema(source,name){
  return errors.New("C1 independent B: missing original typed source schema")
 }
 table:=readbackTable{table:name,columns:fields}
 query,err:=makeReadbackQuery(table,b.BatchUUID,source.Rows)
 if err!=nil{return err}
 got,err:=p.run(ctx,[]byte(query))
 if err!=nil{return fmt.Errorf("C1 independent B: full relational readback: %w",err)}
 if got!="PASS"{
  return fmt.Errorf("C1 independent B: full %s typed columns diverge from original (%q)",name,got)
 }
 return nil
}

func(p *PGCloneExecutor)parentCatalogColumns(ctx context.Context,table string)([]readbackColumn,error){
 schema,ok:=sensorsor.Schema[table]
 if !ok{return nil,errors.New("C1 independent B: unknown SQL source table")}
 if err:=p.VerifyCloneTarget(ctx);err!=nil{return nil,err}
 q:=fmt.Sprintf(`SELECT column_name || '|' || udt_name
 FROM information_schema.columns
 WHERE table_schema='pathfinder' AND table_name=%s
 ORDER BY ordinal_position;`,sensorsor.Quote(table))
 output,err:=p.run(ctx,[]byte(q))
 if err!=nil{return nil,err}
 actual:=map[string]string{}
 for _,line:=range strings.Split(output,"\n"){
  if line==""{continue}
  name,udt,ok:=strings.Cut(line,"|")
  mapped,allowed:=allowedPGType[udt]
  if !ok||!allowed||mapped==""||name==""{
   return nil,fmt.Errorf("C1 independent B: unexpected SQL column type %q",line)
  }
  if _,seen:=actual[name];seen{
   return nil,fmt.Errorf("C1 independent B: duplicate catalog column %s",name)
  }
  actual[name]=mapped
 }
 cols:=make([]readbackColumn,0,len(schema))
 for _,name:=range schema{
  typ,ok:=actual[name]
  if !ok{return nil,fmt.Errorf("C1 independent B: missing typed column %s.%s",table,name)}
  cols=append(cols,rb(name,typ))
 }
 return cols,nil
}
