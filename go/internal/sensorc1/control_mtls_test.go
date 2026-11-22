package sensorc1

import (
 "context"
 "crypto/ecdsa"
 "crypto/elliptic"
 "crypto/rand"
 "crypto/tls"
 "crypto/x509"
 "crypto/x509/pkix"
 "encoding/pem"
 "math/big"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "testing"
 "time"
)

type testPKI struct {ca string; server,client,badClient C1TLSFiles}

func makeControlTestPKI(t *testing.T)testPKI{
 t.Helper()
 dir:=t.TempDir()
 caKey,err:=ecdsa.GenerateKey(elliptic.P256(),rand.Reader);if err!=nil{t.Fatal(err)}
 now:=time.Now()
 caTemplate:=&x509.Certificate{
  SerialNumber:big.NewInt(1),Subject:pkix.Name{CommonName:"Pathfinder-C1-test-CA"},
  NotBefore:now.Add(-time.Hour),NotAfter:now.Add(time.Hour),
  IsCA:true,BasicConstraintsValid:true,
  KeyUsage:x509.KeyUsageCertSign|x509.KeyUsageCRLSign,
 }
 caDER,err:=x509.CreateCertificate(rand.Reader,caTemplate,caTemplate,&caKey.PublicKey,caKey)
 if err!=nil{t.Fatal(err)}
 caCert,err:=x509.ParseCertificate(caDER);if err!=nil{t.Fatal(err)}
 caPath:=filepath.Join(dir,"ca.pem")
 if err:=os.WriteFile(caPath,pem.EncodeToMemory(&pem.Block{Type:"CERTIFICATE",Bytes:caDER}),0600);err!=nil{t.Fatal(err)}
 makeCert:=func(name,dns string,eku x509.ExtKeyUsage)C1TLSFiles{
  key,err:=ecdsa.GenerateKey(elliptic.P256(),rand.Reader);if err!=nil{t.Fatal(err)}
  der,err:=x509.CreateCertificate(rand.Reader,&x509.Certificate{
   SerialNumber:big.NewInt(int64(len(name)+4)),
   Subject:pkix.Name{CommonName:name},
   DNSNames:[]string{dns},
   NotBefore:now.Add(-time.Hour),NotAfter:now.Add(time.Hour),
   KeyUsage:x509.KeyUsageDigitalSignature,
   ExtKeyUsage:[]x509.ExtKeyUsage{eku},
  },caCert,&key.PublicKey,caKey)
  if err!=nil{t.Fatal(err)}
  crt:=filepath.Join(dir,name+".crt")
  keyPath:=filepath.Join(dir,name+".key")
  if err:=os.WriteFile(crt,pem.EncodeToMemory(&pem.Block{Type:"CERTIFICATE",Bytes:der}),0600);err!=nil{t.Fatal(err)}
  pemKey,err:=x509.MarshalECPrivateKey(key);if err!=nil{t.Fatal(err)}
  if err:=os.WriteFile(keyPath,pem.EncodeToMemory(&pem.Block{Type:"EC PRIVATE KEY",Bytes:pemKey}),0600);err!=nil{t.Fatal(err)}
  return C1TLSFiles{Certificate:crt,PrivateKey:keyPath,TrustCA:caPath}
 }
 return testPKI{ca:caPath,
  server:makeCert("server","pathfinder-sor.internal",x509.ExtKeyUsageServerAuth),
  client:makeCert("client","pathfinder-ingest.internal",x509.ExtKeyUsageClientAuth),
  badClient:makeCert("wrong-client","unexpected-ingest.internal",x509.ExtKeyUsageClientAuth),
 }
}
func makeControlTestServer(t *testing.T,proc *SoRProcessor,pki testPKI)*httptest.Server{
 t.Helper()
 controller,err:=NewC1ControlServer(proc,"pathfinder-ingest.internal")
 if err!=nil{t.Fatal(err)}
 server:=httptest.NewUnstartedServer(controller)
 server.TLS,err=NewServerTLS(pki.server)
 if err!=nil{t.Fatal(err)}
 server.StartTLS()
 t.Cleanup(server.Close)
 return server
}
func TestC1TLSReferenceOnlyCommitACK(t *testing.T){
 proc,ref,_,bundle:=processorFixture(t)
 pki:=makeControlTestPKI(t)
 server:=makeControlTestServer(t,proc,pki)
 transport,err:=NewC1HTTPTransport(server.Listener.Addr().String(),"pathfinder-sor.internal",pki.client)
 if err!=nil{t.Fatal(err)}
 future,err:=transport.Submit(context.Background(),ref)
 if err!=nil{t.Fatal(err)}
 ack,ok:=<-future
 if !ok{t.Fatal("mTLS SQL commit ACK absent")}
 ids:=[]string{bundle.Batches[0].BatchUUID,bundle.Batches[1].BatchUUID}
 if err:=validateCommitAck(ref,ids,ack);err!=nil{t.Fatal(err)}
}
func TestC1TLSRejectsUnauthorizedClientDNS(t *testing.T){
 proc,ref,_,_:=processorFixture(t)
 pki:=makeControlTestPKI(t)
 server:=makeControlTestServer(t,proc,pki)
 transport,err:=NewC1HTTPTransport(server.Listener.Addr().String(),"pathfinder-sor.internal",pki.badClient)
 if err!=nil{t.Fatal(err)}
 future,err:=transport.Submit(context.Background(),ref)
 if err!=nil{t.Fatal(err)}
 if _,ok:=<-future;ok{t.Fatal("unauthorized client received SQL ACK")}
}
func TestC1TLSRejectsWrongServerSAN(t *testing.T){
 proc,ref,_,_:=processorFixture(t)
 pki:=makeControlTestPKI(t)
 server:=makeControlTestServer(t,proc,pki)
 transport,err:=NewC1HTTPTransport(server.Listener.Addr().String(),"wrong-sor.internal",pki.client)
 if err!=nil{t.Fatal(err)}
 future,err:=transport.Submit(context.Background(),ref)
 if err!=nil{t.Fatal(err)}
 if _,ok:=<-future;ok{t.Fatal("untrusted server yielded SQL ACK")}
}
func TestC1TLSNoACKOnUnknownPostgresCommit(t *testing.T){
 proc,ref,sql,_:=processorFixture(t)
 sql.failExecute=true
 pki:=makeControlTestPKI(t)
 server:=makeControlTestServer(t,proc,pki)
 transport,err:=NewC1HTTPTransport(server.Listener.Addr().String(),"pathfinder-sor.internal",pki.client)
 if err!=nil{t.Fatal(err)}
 future,err:=transport.Submit(context.Background(),ref)
 if err!=nil{t.Fatal(err)}
 if _,ok:=<-future;ok{t.Fatal("uncertain SQL commit incorrectly acknowledged")}
}
func TestC1TLSRequiresTrustedIdentityAndNoPlainHTTP(t *testing.T){
 proc,_,_,_:=processorFixture(t)
 ctl,err:=NewC1ControlServer(proc,"pathfinder-ingest.internal")
 if err!=nil{t.Fatal(err)}
 recorder:=httptest.NewRecorder()
 req:=httptest.NewRequest(http.MethodPost,ControlPath,nil)
 req.Header.Set("Content-Type","application/json")
 ctl.ServeHTTP(recorder,req)
 if recorder.Code!=http.StatusForbidden{t.Fatalf("plaintext request status %d",recorder.Code)}
 if _,err:=NewC1HTTPTransport("0.0.0.0:9443","pathfinder-sor.internal",C1TLSFiles{});err==nil{
  t.Fatal("wildcard or missing TLS accepted")
 }
 // The server's pinned CA chain must be usable by the Go standard verifier.
 pki:=makeControlTestPKI(t)
 cfg,err:=NewServerTLS(pki.server)
 if err!=nil{t.Fatal(err)}
 if cfg.MinVersion!=tls.VersionTLS13||cfg.ClientAuth!=tls.RequireAndVerifyClientCert{
  t.Fatal("server TLS downgraded or client authentication disabled")
 }
}
