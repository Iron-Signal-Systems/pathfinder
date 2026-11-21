package sensoringest

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func goodBatch() Batch {
	return Batch{SensorID: "sensor-a", Stream: Observations, SchemaVersion: "5", BatchID: "batch-0001",
		Payload: []byte(`{"observed_at":"2026-11-20T10:45:25Z","schema_version":"5","sensor_id":"sensor-a","observed":{},"context":{},"enrichment":{}}` + "\n")}
}

func TestCustodyPreserveReplayAndConflict(t *testing.T) {
	store, err := OpenCustody(filepath.Join(t.TempDir(), "custody"))
	if err != nil { t.Fatal(err) }
	batch := goodBatch()
	first, err := store.Preserve(batch)
	if err != nil { t.Fatal(err) }
	if first.AlreadyPreserved || first.RecordCount != 1 { t.Fatalf("first %#v", first) }
	again, err := store.Preserve(batch)
	if err != nil || !again.AlreadyPreserved || again.SHA256 != first.SHA256 { t.Fatalf("replay %#v %v", again, err) }
	batch.Payload = append(batch.Payload, []byte(`{"observed_at":"2026-11-20T10:45:25Z","schema_version":"5","sensor_id":"sensor-a","observed":{},"context":{},"enrichment":{}}`+"\n")...)
	_, err = store.Preserve(batch)
	if !errors.Is(err, ErrBatchConflict) { t.Fatalf("wanted conflict, got %v", err) }
}

func TestCustodyRejectsBadInput(t *testing.T) {
	store, err := OpenCustody(filepath.Join(t.TempDir(), "custody"))
	if err != nil { t.Fatal(err) }
	base := goodBatch()
	cases := []Batch{}
	b:=base; b.SensorID="../evil"; cases=append(cases,b)
	b=base; b.BatchID="../../root"; cases=append(cases,b)
	b=base; b.Payload=[]byte(`{"schema_version":"5"}`+"\n"); cases=append(cases,b)
	b=base; b.SchemaVersion="6"; cases=append(cases,b)
	b=base; b.Payload=bytes.TrimSuffix(base.Payload, []byte("\n")); cases=append(cases,b)
	b=base; b.Payload=[]byte(strings.Repeat("x",MaxBatchBytes+1)); cases=append(cases,b)
	for i, candidate := range cases {
		if _, err := store.Preserve(candidate); err == nil { t.Errorf("case %d accepted", i) }
	}
}

func TestCustodyConcurrentReplay(t *testing.T) {
	store, err := OpenCustody(filepath.Join(t.TempDir(), "custody"))
	if err != nil { t.Fatal(err) }
	const n=8
	var wg sync.WaitGroup
	errs := make(chan error,n)
	counts := make(chan bool,n)
	for i:=0;i<n;i++ { wg.Add(1); go func(){defer wg.Done(); r,e:=store.Preserve(goodBatch());errs<-e;counts<-r.AlreadyPreserved}() }
	wg.Wait();close(errs);close(counts)
	fresh:=0
	for e:=range errs { if e!=nil {t.Fatalf("concurrent failed %v",e)} }
	for duplicated:=range counts {if !duplicated {fresh++}}
	if fresh!=1 {t.Fatalf("fresh=%d expected 1",fresh)}
}

func TestCustodyRejectsWorldReadableDirectory(t *testing.T) {
	dir:=filepath.Join(t.TempDir(),"open")
	if err:=os.Mkdir(dir,0755);err!=nil {t.Fatal(err)}
	if _,err:=OpenCustody(dir);err==nil {t.Fatal("accepted permissive directory")}
}
