package sensorc1

import (
 "bytes"
 "context"
 "crypto/rand"
 "crypto/sha256"
 "crypto/tls"
 "crypto/x509"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "os"
 "regexp"
 "strings"
 "time"
)

// This is the DEVELOPMENT inter-jail control protocol. Prepared row bytes
// never traverse HTTPS: the authenticated request carries only the exact RAM
// reference, and only a successful original-custody + PostgreSQL readback
// can produce an SQLCommitAck. This does not replace the receiver protocol.
const ControlVersion = 1
const ControlPath = "/v1/c1/sql-commit"
// Requests carry only a RAM reference. SQL ACKs may carry up to 256 exact
// original UUIDs, so the RESPONSE needs a larger independent bounded cap.
const MaxControlBytes = 8192
const MaxControlResponseBytes = 64 << 10
const MaxControlInflight = MaxOutstandingHandoffs

var noncePattern=regexp.MustCompile(`^[0-9a-f]{32}$`)

type ControlRequest struct{
 Version int `json:"version"`
 Nonce string `json:"nonce"`
 Reference BundleReference `json:"reference"`
}
type ControlResponse struct{
 Version int `json:"version"`
 Nonce string `json:"nonce"`
 ACK SQLCommitAck `json:"sql_commit_ack"`
}

// C1ControlServer executes exclusively within pathfinder-sor. A configured
// process MUST also require TLS 1.3, an explicit private client CA and a
// registered ingest certificate identity, as installed by NewServerTLS.
type C1ControlServer struct{
 Processor *SoRProcessor
 ExpectedClientDNS string
 permits chan struct{}
 replay controlReplayFence
}
func NewC1ControlServer(proc *SoRProcessor,expectedClientDNS string)(*C1ControlServer,error){
 if proc==nil||proc.SQL==nil||proc.Originals==nil||proc.ReadRoot==""||expectedClientDNS==""{
  return nil,errors.New("C1 control server: SoR, independent source and trusted ingest DNS required")
 }
 return &C1ControlServer{Processor:proc,ExpectedClientDNS:expectedClientDNS,
  permits:make(chan struct{},MaxControlInflight)},nil
}
func(s *C1ControlServer)ServeHTTP(w http.ResponseWriter,r *http.Request){
 if r.URL.Path!=ControlPath{http.NotFound(w,r);return}
 if r.Method!=http.MethodPost{
  w.Header().Set("Allow","POST")
  http.Error(w,"POST required",http.StatusMethodNotAllowed);return
 }
 if r.TLS==nil||len(r.TLS.VerifiedChains)==0||len(r.TLS.PeerCertificates)==0||
    !exactDNS(r.TLS.PeerCertificates[0],s.ExpectedClientDNS){
  http.Error(w,"authenticated ingest identity required",http.StatusForbidden);return
 }
 if r.Header.Get("Content-Type")!="application/json"{
  http.Error(w,"application/json required",http.StatusUnsupportedMediaType);return
 }
 if r.ContentLength>MaxControlBytes{
  http.Error(w,"request too large",http.StatusRequestEntityTooLarge);return
 }
 raw,err:=io.ReadAll(io.LimitReader(r.Body,MaxControlBytes+1))
 if err!=nil||len(raw)>MaxControlBytes{
  http.Error(w,"control frame too large",http.StatusRequestEntityTooLarge);return
 }
 dec:=json.NewDecoder(bytes.NewReader(raw))
 dec.DisallowUnknownFields()
 var req ControlRequest
 if err:=dec.Decode(&req);err!=nil{
  http.Error(w,"invalid control frame",http.StatusUnprocessableEntity);return
 }
 var trailer any
 if err:=dec.Decode(&trailer);err!=io.EOF{
  http.Error(w,"multiple control frames forbidden",http.StatusUnprocessableEntity);return
 }
 if req.Version!=ControlVersion||!noncePattern.MatchString(req.Nonce)||req.Reference.validate()!=nil{
  http.Error(w,"invalid control identity/reference",http.StatusUnprocessableEntity);return
 }
 select{
 case s.permits<-struct{}{}:defer func(){<-s.permits}()
 default:http.Error(w,"SoR capacity exhausted",http.StatusServiceUnavailable);return
 }
 if err:=s.replay.remember(r.TLS.PeerCertificates[0].Raw,req.Nonce,time.Now());err!=nil{
  if errors.Is(err,ErrControlReplay){
   http.Error(w,"replayed authenticated control nonce",http.StatusConflict)
  }else{
   http.Error(w,"control replay protection unavailable",http.StatusServiceUnavailable)
  }
  return
 }
 ack,err:=s.Processor.ProcessBundle(r.Context(),req.Reference)
 if err!=nil{
  // Uncertain SQL outcomes must NEVER return a successful commit ACK.
  http.Error(w,"SQL commit not confirmed; reconcile original custody",http.StatusServiceUnavailable)
  return
 }
 response:=ControlResponse{Version:ControlVersion,Nonce:req.Nonce,ACK:ack}
 w.Header().Set("Content-Type","application/json")
 w.Header().Set("Cache-Control","no-store")
 if err:=json.NewEncoder(w).Encode(response);err!=nil{return}
}
func exactDNS(cert *x509.Certificate,dns string)bool{
 if cert==nil||dns==""{return false}
 for _,name:=range cert.DNSNames{if name==dns{return true}}
 return false
}

type C1TLSFiles struct{
 Certificate string
 PrivateKey string
 TrustCA string
}

// NewServerTLS requires a dedicated mTLS server certificate and a pinned CA
// for the ingest client. No system trust roots or insecure skip verification.
func NewServerTLS(files C1TLSFiles)(*tls.Config,error){
 cert,ca,err:=loadTLS(files)
 if err!=nil{return nil,err}
 return &tls.Config{MinVersion:tls.VersionTLS13,
  Certificates:[]tls.Certificate{cert},ClientAuth:tls.RequireAndVerifyClientCert,
  ClientCAs:ca},nil
}
func loadTLS(files C1TLSFiles)(tls.Certificate,*x509.CertPool,error){
 if files.Certificate==""||files.PrivateKey==""||files.TrustCA==""{
  return tls.Certificate{},nil,errors.New("C1 mTLS: explicit key, certificate and CA required")
 }
 cert,err:=tls.LoadX509KeyPair(files.Certificate,files.PrivateKey)
 if err!=nil{return tls.Certificate{},nil,err}
 caRaw,err:=os.ReadFile(files.TrustCA)
 if err!=nil{return tls.Certificate{},nil,err}
 pool:=x509.NewCertPool()
 if !pool.AppendCertsFromPEM(caRaw){
  return tls.Certificate{},nil,errors.New("C1 mTLS: invalid dedicated trust CA")
 }
 return cert,pool,nil
}

// NewC1HTTPTransport connects to ONE explicitly configured private jail IP.
// It uses TLS 1.3, pinned trust, a client certificate and the expected SoR
// DNS SAN for certificate validation. It does not send prepared SQL rows.
type C1HTTPTransport struct {
 client *http.Client
 endpoint string
}
func NewC1HTTPTransport(address,serverDNS string,files C1TLSFiles)(*C1HTTPTransport,error){
 host,port,err:=net.SplitHostPort(address)
 if err!=nil||port==""||net.ParseIP(host)==nil||net.ParseIP(host).IsUnspecified()||serverDNS==""{
  return nil,errors.New("C1 mTLS: literal non-wildcard IP:port and exact SoR SAN required")
 }
 cert,ca,err:=loadTLS(files)
 if err!=nil{return nil,err}
 tr:=&http.Transport{
  TLSClientConfig:&tls.Config{
   MinVersion:tls.VersionTLS13,RootCAs:ca,
   Certificates:[]tls.Certificate{cert},ServerName:serverDNS,
  },
  MaxIdleConns:MaxControlInflight,
  MaxIdleConnsPerHost:MaxControlInflight,
  MaxConnsPerHost:MaxControlInflight,
  IdleConnTimeout:30*time.Second,
  DisableCompression:true,
 }
 return &C1HTTPTransport{client:&http.Client{Transport:tr},
  endpoint:"https://"+address+ControlPath},nil
}
func(t *C1HTTPTransport)Submit(ctx context.Context,ref BundleReference)(<-chan SQLCommitAck,error){
 if t==nil||t.client==nil||ctx==nil{return nil,errors.New("C1 mTLS: unavailable")}
 if err:=ctx.Err();err!=nil{return nil,err}
 if err:=ref.validate();err!=nil{return nil,err}
 ch:=make(chan SQLCommitAck,1)
 go func(){
  defer close(ch)
  var n [16]byte
  if _,err:=rand.Read(n[:]);err!=nil{return}
  nonce:=hex.EncodeToString(n[:])
  body,err:=json.Marshal(ControlRequest{Version:ControlVersion,Nonce:nonce,Reference:ref})
  if err!=nil||len(body)>MaxControlBytes{return}
  req,err:=http.NewRequestWithContext(ctx,http.MethodPost,t.endpoint,bytes.NewReader(body))
  if err!=nil{return}
  req.Header.Set("Content-Type","application/json")
  resp,err:=t.client.Do(req)
  if err!=nil{return}
  defer resp.Body.Close()
  if resp.StatusCode!=http.StatusOK||!strings.HasPrefix(resp.Header.Get("Content-Type"),"application/json"){
   return
  }
  raw,err:=io.ReadAll(io.LimitReader(resp.Body,MaxControlResponseBytes+1))
  if err!=nil||len(raw)>MaxControlResponseBytes{return}
  var result ControlResponse
  if err:=json.Unmarshal(raw,&result);err!=nil{return}
  if result.Version!=ControlVersion||result.Nonce!=nonce{return}
  select {
  case ch<-result.ACK:
  case <-ctx.Done():
  }
 }()
 return ch,nil
}

// ProveControlIdentity is a development-only diagnostic function to expose
// the exact certificate digest without exporting any TLS private key.
func ProveControlIdentity(certDER []byte)string{
 d:=sha256.Sum256(certDER)
 return fmt.Sprintf("%x",d[:])
}
var _ SQLCommitTransport=(*C1HTTPTransport)(nil)
