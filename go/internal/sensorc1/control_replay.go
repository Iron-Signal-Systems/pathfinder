package sensorc1

import (
 "crypto/sha256"
 "errors"
 "sync"
 "time"
)

// Bounded transport replay defense. This is per-service-process session
// fencing, NOT durable SQL progress. Across server restarts, PostgreSQL's
// immutable batch UUID and exact completion state provide durable idempotency.
const ControlReplayWindow = 30 * time.Minute
const MaxControlReplayEntries = 65536

var ErrControlReplay=errors.New("C1 control: client nonce already used")
var ErrControlReplayCapacity=errors.New("C1 control: replay fence at capacity; fail closed")

type controlReplayFence struct {
 mu sync.Mutex
 used map[[32]byte]time.Time
 sincePrune uint32
}

// remember accepts a nonce ONCE for an authenticated peer certificate,
// including on a failed SQL attempt. A client must use a fresh nonce for a
// retry; it must not infer SQL commit from a transport failure.
func(f *controlReplayFence)remember(certDER []byte,nonce string,now time.Time)error{
 if f==nil||len(certDER)==0||!noncePattern.MatchString(nonce)||now.IsZero(){
  return errors.New("C1 control: invalid authenticated replay identity")
 }
 payload:=make([]byte,0,len(certDER)+len(nonce))
 payload=append(payload,certDER...)
 payload=append(payload,nonce...)
 id:=sha256.Sum256(payload)
 f.mu.Lock()
 defer f.mu.Unlock()
 if f.used==nil{f.used=make(map[[32]byte]time.Time)}
 if first,ok:=f.used[id];ok{
  if now.Before(first.Add(ControlReplayWindow)){
   return ErrControlReplay
  }
  delete(f.used,id)
 }
 // At high traffic we expire old nonces periodically. If an authenticated
 // client exceeds the bounded capacity before expiration, reject new work,
 // rather than allocate an unlimited replay registry.
 f.sincePrune++
 if len(f.used)>=MaxControlReplayEntries || f.sincePrune>=1024{
  for k,first:=range f.used{
   if !now.Before(first.Add(ControlReplayWindow)){delete(f.used,k)}
  }
  f.sincePrune=0
 }
 if len(f.used)>=MaxControlReplayEntries{return ErrControlReplayCapacity}
 f.used[id]=now
 return nil
}
