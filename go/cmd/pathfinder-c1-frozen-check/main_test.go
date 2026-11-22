package main

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestC1FrozenCheckReadOnly(t *testing.T){
    root:=t.TempDir()
    raw:=`{"sensor_id":"Gateway","schema_version":"5","observed_at":"2026-11-20T10:00:00Z","context":{},"observed":{},"enrichment":{}}`+"\n"
    path:=filepath.Join(root,"observations-1.jsonl.active")
    if err:=os.WriteFile(path,[]byte(raw),0600);err!=nil{t.Fatal(err)}
    var out bytes.Buffer
    err:=run([]string{"-root",root,"-sensor-id","Gateway",
        "-sensor-uuid","11111111-2222-4333-8444-555555555555",
        "-expect-files","1","-expect-records","1"},&out)
    if err!=nil{t.Fatal(err)}
    if !strings.Contains(out.String(),"PATHFINDER_C1_FROZEN_PREP_PASS files=1 batches=1 records=1"){
        t.Fatal(out.String())
    }
    got,err:=os.ReadFile(path);if err!=nil{t.Fatal(err)}
    if string(got)!=raw{t.Fatal("original evidence changed")}
}
func TestC1FrozenCheckRefusesWrongExpectedCounts(t *testing.T){
    root:=t.TempDir()
    raw:=`{"sensor_id":"Gateway","schema_version":"5","observed_at":"2026-11-20T10:00:00Z","context":{},"observed":{},"enrichment":{}}`+"\n"
    if err:=os.WriteFile(filepath.Join(root,"observations-1.jsonl.active"),[]byte(raw),0600);err!=nil{t.Fatal(err)}
    var out bytes.Buffer
    err:=run([]string{"-root",root,"-sensor-id","Gateway",
        "-sensor-uuid","11111111-2222-4333-8444-555555555555",
        "-expect-files","9","-expect-records","685218"},&out)
    if err==nil{t.Fatal("false acceptance of incomplete source")}
    if strings.Contains(out.String(),"PASS"){t.Fatal("reported false PASS")}
}
