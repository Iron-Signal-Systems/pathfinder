package sensorc1

import (
    "bytes"
    "crypto/sha256"
    "encoding/hex"
    "strings"
    "testing"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

const sensorUUID = "11111111-2222-4333-8444-555555555555"
const sensorID = "udm-test"

var cases = map[sensoringest.Stream]struct{schema,raw,table,child string}{
 sensoringest.Observations:{"5",`{"sensor_id":"udm-test","schema_version":"5","observed_at":"2026-11-20T10:00:00.123456789Z","timestamp_source":"kernel_software_ns","context":{"direction":"ingress"},"observed":{"network":{"source_ip":"192.0.2.10","destination_ip":"198.51.100.2","protocol":"tcp"},"dns":{"questions":[{"name":"example.org","type":"A"}],"answers":[{"name":"example.org","type":"A","class":"IN","ttl":30,"value":"198.51.100.2"}]},"tls":{"client_hello_observed":true,"sni":"example.org"}},"enrichment":{}}`,"sensor_observation","sensor_dns_question"},
 sensoringest.Conntrack:{"1",`{"record_type":"conntrack_event","schema_version":"1","observed_at":"2026-11-20T10:00:00Z","event_type":"new","original":{},"reply":{},"original_counters":{},"reply_counters":{}}`,"sensor_conntrack_event",""},
 sensoringest.Identity:{"1",`{"record_type":"endpoint_identity_snapshot","schema_version":"1","observed_at":"2026-11-20T10:00:00Z","expected_interval_seconds":60,"endpoints":[{"ip":"192.0.2.3","mac":"aa:bb:cc:dd:ee:ff","hostname":"test"}]}`,"sensor_identity_snapshot","sensor_identity_endpoint"},
 sensoringest.Health:{"2",`{"record_type":"sensor_health","schema_version":"2","observed_at":"2026-11-20T10:00:00Z","expected_interval_seconds":60,"capture":{"started_at":"2026-11-20T09:00:00Z"},"conntrack":{},"identity":{},"storage":{}}`,"sensor_health",""},
}

func inputFor(s sensoringest.Stream) Input {
    c:=cases[s]
    return Input{SensorID:sensorID,SensorUUID:sensorUUID,Stream:s,
        SchemaVersion:c.schema,WireBatchID:"batch-a",
        BatchUUID:sensorsor.ID("frozen-import-batch-v1",sensorUUID,string(s),"batch-a"),SegmentOrdinalStart:17,
        Payload:[]byte(c.raw+"\n")}
}

func TestPrepareAllFourFamilies(t *testing.T) {
    for stream,c:=range cases { t.Run(string(stream),func(t *testing.T) {
        in:=inputFor(stream)
        in.Payload=append(in.Payload,[]byte(c.raw+"\n")...)
        p,err:=Prepare(in);if err!=nil{t.Fatal(err)}
        if p.Evidence.Count!=2 {t.Fatalf("count=%d",p.Evidence.Count)}
        if got:=len(p.Evidence.Tables[c.table].Rows);got!=2 {t.Fatalf("base rows=%d",got)}
        if c.child!="" && len(p.Evidence.Tables[c.child].Rows)!=2 {t.Fatal("missing ordered child rows")}
        if err:=ValidatePrepared(p);err!=nil{t.Fatal(err)}
        h:=sha256.Sum256(in.Payload)
        if p.PayloadSHA256!=hex.EncodeToString(h[:]){t.Fatal("source hash mismatch")}
        if stream==sensoringest.Observations && len(p.Evidence.Tables["sensor_dns_record"].Rows)!=2 {
            t.Fatal("DNS resource-record children lost")
        }
    })}
}

func TestPrepareRejectsInvalidOrConflictingSource(t *testing.T) {
    good:=inputFor(sensoringest.Observations)
    checks:=map[string]func(*Input){
        "missing newline":func(i *Input){i.Payload=bytes.TrimSuffix(i.Payload,[]byte("\n"))},
        "CRLF changes exact bytes":func(i *Input){i.Payload=bytes.ReplaceAll(i.Payload,[]byte("\n"),[]byte("\r\n"))},
        "sensor mismatch":func(i *Input){i.SensorID="different"},
        "schema mismatch":func(i *Input){i.SchemaVersion="99"},
        "unsafe batch identifier":func(i *Input){i.WireBatchID="../escape"},
        "bad segment ordinal":func(i *Input){i.SegmentOrdinalStart=0},
        "missing record":func(i *Input){i.Payload=[]byte("\n")},
        "corrupt json":func(i *Input){i.Payload=[]byte("{not-json}\n")},
    }
    for name,change:=range checks { t.Run(name,func(t *testing.T) {
        i:=good;change(&i)
        if _,err:=Prepare(i);err==nil{t.Fatal("accepted invalid input")}
    })}
}

func TestPreparedTamperRejected(t *testing.T) {
    p,err:=Prepare(inputFor(sensoringest.Observations));if err!=nil{t.Fatal(err)}
    t.Run("digest",func(t *testing.T){
        q:=p;q.PayloadSHA256=strings.Repeat("0",64)
        if err:=ValidatePrepared(q);err==nil{t.Fatal("changed source hash accepted")}
    })
    t.Run("ordinal",func(t *testing.T){
        q:=p;table:=*p.Evidence.Tables["sensor_observation"]
        table.Rows=append([][]string(nil),table.Rows...)
        table.Rows[0]=append([]string(nil),table.Rows[0]...)
        table.Rows[0][4]="7"
        ev:=*p.Evidence
        ev.Tables=map[string]*sensorsor.Columns{}
        for k,v:=range p.Evidence.Tables{ev.Tables[k]=v}
        ev.Tables["sensor_observation"]=&table
        q.Evidence=&ev
        if err:=ValidatePrepared(q);err==nil{t.Fatal("changed source ordinal accepted")}
    })
}
