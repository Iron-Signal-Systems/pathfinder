package sensorc1

import (
    "strings"
    "testing"
    "time"

    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
    "github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorsor"
)

func originalFor(stream sensoringest.Stream)OriginalBatch{
    input:=inputFor(stream)
    return OriginalBatch{
        Input:input,
        SegmentUUID:sensorsor.ID("frozen-import-segment-v1",input.SensorUUID,string(stream),strings.Repeat("a",64)),
        SegmentFilename:"test.jsonl.active",
        SegmentSHA256:strings.Repeat("a",64),
        SegmentReference:"/readonly/frozen/test.jsonl.active",
        SegmentStoredBytes:int64(len(input.Payload)),
        SegmentRecordCount:1,
        BatchCustodyReference:"/readonly/custody/batch.json",
        PreservedAt:time.Date(2026,11,20,12,0,0,0,time.UTC),
    }
}

func TestSoRSQLPlanAllFourFamilies(t *testing.T){
    for stream:=range cases{
        t.Run(string(stream),func(t *testing.T){
            original:=originalFor(stream)
            p,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
            plan,err:=BuildSoRBatch(p,original);if err!=nil{t.Fatal(err)}
            if plan.BatchUUID!=p.BatchUUID || plan.Reference!=original.BatchCustodyReference{
                t.Fatal("SoR SQL did not preserve custody identity")
            }
            sql,err:=BuildSoRSQL(p,original);if err!=nil{t.Fatal(err)}
            script:=string(sql)
            if !strings.Contains(script,"BEGIN;") ||
               !strings.Contains(script,"COMMIT;") ||
               !strings.Contains(script,"sensor_batch_index_completion"){
                t.Fatal("missing atomic batch/complete transaction")
            }
            if !strings.Contains(script,"COPY pathfinder."+cases[stream].table) {
                t.Fatal("wrong primary source table")
            }
            if cases[stream].child!="" &&
               !strings.Contains(script,"COPY pathfinder."+cases[stream].child){
                t.Fatal("missing atomic source child")
            }
        })
    }
}

func TestSoRSQLRejectsUnverifiedOrConflictingCustody(t *testing.T){
    original:=originalFor(sensoringest.Observations)
    p,err:=Prepare(original.Input);if err!=nil{t.Fatal(err)}
    t.Run("different source bytes",func(t *testing.T){
        changed:=original
        changed.Input=original.Input
        changed.Input.Payload=[]byte(strings.ReplaceAll(string(original.Input.Payload),"example.org","substituted.invalid"))
        // The fixture may not contain that text; explicitly change the raw
        // observation record's timestamp while keeping JSON valid.
        changed.Input.Payload=[]byte(strings.ReplaceAll(string(original.Input.Payload),"2026-11-20T10:00:00.123456789Z","2026-11-20T10:00:01.123456789Z"))
        if _,err:=BuildSoRBatch(p,changed);err==nil{t.Fatal("changed source accepted")}
    })
    t.Run("different identity",func(t *testing.T){
        changed:=original
        changed.Input.BatchUUID=sensorsor.ID("other",p.BatchUUID)
        if _,err:=BuildSoRBatch(p,changed);err==nil{t.Fatal("changed custody batch accepted")}
    })
    t.Run("missing provenance",func(t *testing.T){
        changed:=original
        changed.BatchCustodyReference=""
        if _,err:=BuildSoRBatch(p,changed);err==nil{t.Fatal("no custody ref accepted")}
    })
    t.Run("invalid segment hash",func(t *testing.T){
        changed:=original
        changed.SegmentSHA256="BAD"
        if _,err:=BuildSoRBatch(p,changed);err==nil{t.Fatal("invalid source hash accepted")}
    })
}
