package sensorc1

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "net/http"
 "strings"
 "testing"
 "time"
)

func TestControlReplayFenceRejectsSameCertNonce(t *testing.T){
 var f controlReplayFence
 now:=time.Date(2026,11,22,15,0,0,0,time.UTC)
 nonce:=strings.Repeat("a",32)
 if err:=f.remember([]byte("registered-ingest-der"),nonce,now);err!=nil{t.Fatal(err)}
 if err:=f.remember([]byte("registered-ingest-der"),nonce,now.Add(time.Second));!errors.Is(err,ErrControlReplay){
  t.Fatalf("replayed control nonce was not rejected: %v",err)
 }
 // Certificate-scoped: a distinct authenticated identity has its own
 // nonce space. The TLS handler must still enforce its configured DNS SAN.
 if err:=f.remember([]byte("different-ingest-der"),nonce,now);err!=nil{t.Fatal(err)}
 if err:=f.remember([]byte("registered-ingest-der"),nonce,now.Add(ControlReplayWindow+time.Second));err!=nil{
  t.Fatalf("expired nonce could not be reclaimed: %v",err)
 }
}

func TestControlReplayFenceFailsClosedAtCapacity(t *testing.T){
 var f controlReplayFence
 now:=time.Date(2026,11,22,15,0,0,0,time.UTC)
 for i:=0;i<MaxControlReplayEntries;i++{
  var key [32]byte
  key[0]=byte(i)
  key[1]=byte(i>>8)
  key[2]=byte(i>>16)
  f.used=appendControlReplayFixture(f.used,key,now)
 }
 if len(f.used)!=MaxControlReplayEntries{t.Fatal("test registry did not fill")}
 if err:=f.remember([]byte("registered-ingest-der"),strings.Repeat("b",32),now);!errors.Is(err,ErrControlReplayCapacity){
  t.Fatalf("bounded registry accepted additional work: %v",err)
 }
 if err:=f.remember([]byte("registered-ingest-der"),strings.Repeat("b",32),
  now.Add(ControlReplayWindow+time.Second));err!=nil{
  t.Fatalf("expired entries not removed: %v",err)
 }
}
func appendControlReplayFixture(m map[[32]byte]time.Time,k [32]byte,at time.Time)map[[32]byte]time.Time{
 if m==nil{m=make(map[[32]byte]time.Time,MaxControlReplayEntries)}
 m[k]=at
 return m
}

func TestC1TLSRejectsReplayedControlRequestWithoutSQL(t *testing.T){
 proc,ref,sql,b:=processorFixture(t)
 pki:=makeControlTestPKI(t)
 server:=makeControlTestServer(t,proc,pki)
 transport,err:=NewC1HTTPTransport(server.Listener.Addr().String(),
  "pathfinder-sor.internal",pki.client)
 if err!=nil{t.Fatal(err)}
 request:=func(nonce string)(int,ControlResponse){
  t.Helper()
  encoded,err:=json.Marshal(ControlRequest{
   Version:ControlVersion,Nonce:nonce,Reference:ref,
  })
  if err!=nil{t.Fatal(err)}
  req,err:=http.NewRequestWithContext(context.Background(),http.MethodPost,
   transport.endpoint,bytes.NewReader(encoded))
  if err!=nil{t.Fatal(err)}
  req.Header.Set("Content-Type","application/json")
  response,err:=transport.client.Do(req)
  if err!=nil{t.Fatal(err)}
  defer response.Body.Close()
  var out ControlResponse
  if response.StatusCode==http.StatusOK{
   if err:=json.NewDecoder(response.Body).Decode(&out);err!=nil{t.Fatal(err)}
  }
  return response.StatusCode,out
 }
 nonce:=strings.Repeat("c",32)
 code,first:=request(nonce)
 if code!=http.StatusOK||first.ACK.Outcome!="committed"{
  t.Fatalf("first authenticated control result: %d %+v",code,first)
 }
 expectedIDs:=[]string{b.Batches[0].BatchUUID,b.Batches[1].BatchUUID}
 if err:=validateCommitAck(ref,expectedIDs,first.ACK);err!=nil{t.Fatal(err)}
 code,_=request(nonce)
 if code!=http.StatusConflict{
  t.Fatalf("duplicate nonce was not fenced; HTTP status=%d",code)
 }
 if sql.writes!=2{t.Fatalf("replayed nonce retriggered SQL writes=%d",sql.writes)}
 // Legitimate retry after a lost response must use a fresh nonce and rely
 // on the durable PostgreSQL batch keys, not the volatile replay table.
 code,again:=request(strings.Repeat("d",32))
 if code!=http.StatusOK||again.ACK.Outcome!="already_committed"{
  t.Fatalf("new-nonce durable replay result: %d %+v",code,again)
 }
 if sql.writes!=2{t.Fatal("safe SQL replay introduced duplicate writes")}
}
