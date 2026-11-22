package sensorc1

import (
 "context"
 "errors"
 "fmt"
 "strings"

 "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

type readbackColumn struct {name,sqlType string}
type readbackTable struct {table string; columns []readbackColumn; join string}

func rb(name,t string)readbackColumn{return readbackColumn{name,t}}
var sqlReadback = map[string]readbackTable{
 "sensor_observation":{
  table:"sensor_observation",
  columns:[]readbackColumn{
   rb("observed_at","timestamptz"),rb("observation_id","uuid"),
   rb("sensor_id","uuid"),rb("batch_id","uuid"),
   rb("record_ordinal","integer"),rb("segment_record_ordinal","bigint"),
   rb("observed_at_ns","bigint"),rb("raw_record","jsonb"),
  },
 },
 "sensor_conntrack_event":{
  table:"sensor_conntrack_event",
  columns:[]readbackColumn{
   rb("observed_at","timestamptz"),rb("event_id","uuid"),
   rb("sensor_id","uuid"),rb("batch_id","uuid"),
   rb("record_ordinal","integer"),rb("segment_record_ordinal","bigint"),
   rb("observed_at_ns","bigint"),rb("raw_record","jsonb"),
  },
 },
 "sensor_identity_snapshot":{
  table:"sensor_identity_snapshot",
  columns:[]readbackColumn{
   rb("snapshot_id","uuid"),rb("sensor_id","uuid"),rb("batch_id","uuid"),
   rb("record_ordinal","integer"),rb("segment_record_ordinal","bigint"),
   rb("observed_at","timestamptz"),rb("observed_at_ns","bigint"),
   rb("raw_record","jsonb"),
  },
 },
 "sensor_health":{
  table:"sensor_health",
  columns:[]readbackColumn{
   rb("health_id","uuid"),rb("sensor_id","uuid"),rb("batch_id","uuid"),
   rb("record_ordinal","integer"),rb("segment_record_ordinal","bigint"),
   rb("observed_at","timestamptz"),rb("observed_at_ns","bigint"),
   rb("raw_record","jsonb"),
  },
 },
 "sensor_dns_question":{
  table:"sensor_dns_question",
  columns:[]readbackColumn{
   rb("observation_at","timestamptz"),rb("observation_id","uuid"),
   rb("question_ordinal","integer"),rb("question_name","text"),rb("question_type","text"),
  },
  join:"JOIN pathfinder.sensor_observation o ON d.observation_at=o.observed_at AND d.observation_id=o.observation_id",
 },
 "sensor_dns_record":{
  table:"sensor_dns_record",
  columns:[]readbackColumn{
   rb("observation_at","timestamptz"),rb("observation_id","uuid"),
   rb("section","text"),rb("record_ordinal","integer"),rb("name","text"),
   rb("record_type","text"),rb("record_class","text"),rb("ttl_seconds","bigint"),
   rb("value","text"),rb("raw_record","jsonb"),
  },
  join:"JOIN pathfinder.sensor_observation o ON d.observation_at=o.observed_at AND d.observation_id=o.observation_id",
 },
 "sensor_identity_endpoint":{
  table:"sensor_identity_endpoint",
  columns:[]readbackColumn{
   rb("snapshot_id","uuid"),rb("endpoint_ordinal","integer"),rb("ip","inet"),
   rb("mac","macaddr"),rb("hostname","text"),rb("fqdn","text"),
   rb("lease_expires_at","timestamptz"),rb("raw_endpoint","jsonb"),
  },
  join:"JOIN pathfinder.sensor_identity_snapshot o ON d.snapshot_id=o.snapshot_id",
 },
}

// verifyRows issues read-only EXCEPT ALL comparisons against PostgreSQL for
// exact source identity/ordinals/observed time/original JSON and every DNS /
// identity child field. Both difference directions are required to detect
// missing, extra, duplicated, or modified evidence. Source projections beyond
// these columns still require a SEPARATE independent verifier-B acceptance.
func verifyRows(ctx context.Context,run PGRun,b sensorsor.BatchSQL)error{
 if b.Evidence==nil || run==nil{return errors.New("C1 SQL readback: missing evidence")}
 base,err:=readbackBase(b.Stream)
 if err!=nil{return err}
 names:=[]string{base}
 if base=="sensor_observation"{
  names=append(names,"sensor_dns_question","sensor_dns_record")
 } else if base=="sensor_identity_snapshot" {
  names=append(names,"sensor_identity_endpoint")
 }
 for _,name:=range names{
  table:=sqlReadback[name]
  var rows [][]string
  if expected:=b.Evidence.Tables[name];expected!=nil{
   if !sameReadbackSchema(expected,name) {
    return fmt.Errorf("C1 SQL readback: untrusted schema %s",name)
   }
   rows=expected.Rows
  }
  query,err:=makeReadbackQuery(table,b.BatchUUID,rows)
  if err!=nil{return err}
  result,err:=run(ctx,[]byte(query))
  if err!=nil{return fmt.Errorf("C1 SQL readback: %s: %w",name,err)}
  if result!="PASS" {
   return fmt.Errorf("C1 SQL readback: %s exact evidence mismatch (%q)",name,result)
  }
 }
 return nil
}
func readbackBase(stream string)(string,error){
 switch stream{
 case "observations":return "sensor_observation",nil
 case "conntrack":return "sensor_conntrack_event",nil
 case "identity":return "sensor_identity_snapshot",nil
 case "health":return "sensor_health",nil
 default:return "",errors.New("C1 SQL readback: invalid source family")
 }
}
func sameReadbackSchema(table *sensorsor.Columns,name string)bool{
 if table==nil||table.Table!=name{return false}
 expected:=sensorsor.Schema[name]
 if len(table.Names)!=len(expected){return false}
 for i,n:=range expected {if table.Names[i]!=n{return false}}
 return true
}
func makeReadbackQuery(table readbackTable,batchUUID string,rows [][]string)(string,error){
 schema,ok:=sensorsor.Schema[table.table]
 if !ok||!uuidPattern.MatchString(batchUUID)||len(table.columns)==0 {
  return "",errors.New("C1 SQL readback: unsupported table or batch identity")
 }
 indices:=make([]int,len(table.columns))
 for i,c:=range table.columns{
  indices[i]=-1
  for j,name:=range schema{if name==c.name {indices[i]=j;break}}
  if indices[i]<0{return "",fmt.Errorf("C1 SQL readback: missing %s.%s",table.table,c.name)}
 }
 columns:=make([]string,len(table.columns))
 for i,c:=range table.columns{columns[i]=c.name}
 // Every identifier and type below originates from a trusted static table,
 // not from a remote ingest worker. Source cell data is SQL-quoted.
 query:=new(strings.Builder)
 query.WriteString("WITH actual AS (SELECT ")
 for i,c:=range table.columns{
  if i>0{query.WriteByte(',')}
  fmt.Fprintf(query,"d.%s",c.name)
 }
 fmt.Fprintf(query," FROM pathfinder.%s d %s WHERE ",table.table,table.join)
 if table.join==""{query.WriteString("d.batch_id=")}else{query.WriteString("o.batch_id=")}
 query.WriteString(sensorsor.Quote(batchUUID))
 query.WriteString("::uuid)")
 if len(rows)==0{
  query.WriteString(" SELECT CASE WHEN EXISTS (SELECT 1 FROM actual) THEN 'FAIL' ELSE 'PASS' END;")
  return query.String(),nil
 }
 query.WriteString(", expected(")
 query.WriteString(strings.Join(columns,","))
 query.WriteString(") AS (VALUES\n")
 for rowNum,row:=range rows{
  if len(row)!=len(schema){return "",errors.New("C1 SQL readback: wrong source row width")}
  if rowNum>0{query.WriteString(",\n")}
  query.WriteByte('(')
  for colNum,c:=range table.columns{
   if colNum>0{query.WriteByte(',')}
   cell:=row[indices[colNum]]
   if cell=="\\N"{query.WriteString("NULL::"+c.sqlType)}else{
    fmt.Fprintf(query,"%s::%s",sensorsor.Quote(cell),c.sqlType)
   }
  }
  query.WriteByte(')')
 }
 query.WriteString(")\nSELECT CASE WHEN\n")
 query.WriteString(" EXISTS (SELECT * FROM expected EXCEPT ALL SELECT * FROM actual)\n")
 query.WriteString(" OR EXISTS (SELECT * FROM actual EXCEPT ALL SELECT * FROM expected)\n")
 query.WriteString(" THEN 'FAIL' ELSE 'PASS' END;")
 return query.String(),nil
}
