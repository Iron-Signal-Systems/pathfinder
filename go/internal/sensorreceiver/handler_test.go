package sensorreceiver

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
)

const observationRow = `{"observed_at":"2026-11-20T10:45:25Z","schema_version":"5","sensor_id":"sensor-a","observed":{},"context":{},"enrichment":{}}` + "\n"

func certificate(t *testing.T, parent *x509.Certificate, signingKey *ecdsa.PrivateKey, template *x509.Certificate) (tls.Certificate, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil { t.Fatal(err) }
	if parent == nil { parent = template; signingKey = key }
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, signingKey)
	if err != nil { t.Fatal(err) }
	cert, err := x509.ParseCertificate(der)
	if err != nil { t.Fatal(err) }
	derKey, err := x509.MarshalECPrivateKey(key)
	if err != nil { t.Fatal(err) }
	pair, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: derKey}))
	if err != nil { t.Fatal(err) }
	return pair, cert, key
}

func TestMutualTLSIdentityAndReplay(t *testing.T) {
	now := time.Now()
	caT := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Pathfinder sensor test CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	_, ca, caKey := certificate(t, nil, nil, caT)
	pool := x509.NewCertPool(); pool.AddCert(ca)
	serverT := &x509.Certificate{SerialNumber: big.NewInt(2), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	serverPair, _, _ := certificate(t, ca, caKey, serverT)
	san, err := url.Parse("spiffe://pathfinder.test/sensor/sensor-a")
	if err != nil { t.Fatal(err) }
	clientT := &x509.Certificate{SerialNumber: big.NewInt(3), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, URIs: []*url.URL{san}}
	clientPair, _, _ := certificate(t, ca, caKey, clientT)
	unregisteredURI, _ := url.Parse("spiffe://pathfinder.test/sensor/not-authorized")
	unregisteredT := &x509.Certificate{SerialNumber: big.NewInt(4), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, URIs: []*url.URL{unregisteredURI}}
	unregisteredPair, _, _ := certificate(t, ca, caKey, unregisteredT)

	dir := filepath.Join(t.TempDir(), "custody")
	store, err := sensoringest.OpenCustody(dir)
	if err != nil { t.Fatal(err) }
	handler, err := NewHandler(store, IdentityPolicy{san.String(): "sensor-a"})
	if err != nil { t.Fatal(err) }
	srv := httptest.NewUnstartedServer(handler)
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool, Certificates: []tls.Certificate{serverPair}}
	srv.StartTLS(); defer srv.Close()
	makeClient := func(certificates []tls.Certificate) *http.Client {
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pool, Certificates: certificates}}}
	}
	authorized := makeClient([]tls.Certificate{clientPair}); defer authorized.CloseIdleConnections()
	unregistered := makeClient([]tls.Certificate{unregisteredPair}); defer unregistered.CloseIdleConnections()
	anonymous := makeClient(nil); defer anonymous.CloseIdleConnections()
	send := func(client *http.Client, claimed, version, batchID, stream, payload string) (int, string, error) {
		req, err := http.NewRequest(http.MethodPost, srv.URL+BatchPath, strings.NewReader(payload))
		if err != nil { return 0, "", err }
		req.Header.Set("Content-Type", "application/x-ndjson")
		req.Header.Set("X-Pathfinder-Sensor-ID", claimed)
		req.Header.Set("X-Pathfinder-Stream", stream)
		req.Header.Set("X-Pathfinder-Schema-Version", version)
		req.Header.Set("X-Pathfinder-Batch-ID", batchID)
		res, err := client.Do(req)
		if err != nil { return 0, "", err }
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		return res.StatusCode, string(body), err
	}
	status, body, err := send(authorized, "sensor-a", "5", "batch-a", "observations", observationRow)
	if err != nil || status != 201 || !strings.Contains(body, `"status":"preserved"`) || !strings.Contains(body, `"indexed":false`) { t.Fatalf("initial status=%d body=%s err=%v", status, body, err) }
	status, body, err = send(authorized, "sensor-a", "5", "batch-a", "observations", observationRow)
	if err != nil || status != 200 || !strings.Contains(body, `"status":"already_preserved"`) { t.Fatalf("replay status=%d body=%s err=%v", status, body, err) }
	status, _, err = send(authorized, "sensor-a", "5", "batch-a", "observations", observationRow+observationRow)
	if err != nil || status != 409 { t.Fatalf("conflict status=%d err=%v", status, err) }
	status, _, err = send(authorized, "sensor-b", "5", "batch-b", "observations", observationRow)
	if err != nil || status != 403 { t.Fatalf("forged sensor status=%d err=%v", status, err) }
	status, _, err = send(unregistered, "sensor-a", "5", "batch-b", "observations", observationRow)
	if err != nil || status != 401 { t.Fatalf("unregistered client status=%d err=%v", status, err) }
	if _, _, err = send(anonymous, "sensor-a", "5", "batch-b", "observations", observationRow); err == nil { t.Fatal("anonymous client succeeded without mTLS") }
	status, body, err = send(authorized, "sensor-a", "6", "batch-b", "observations", strings.Replace(observationRow, `"schema_version":"5"`, `"schema_version":"6"`, 1))
	if err != nil || status != 422 || !strings.Contains(body, "unsupported_schema") { t.Fatalf("unknown version status=%d body=%s err=%v", status, body, err) }
	status, _, err = send(authorized, "sensor-a", "5", "batch-c", "observations", strings.Replace(observationRow, `"sensor_id":"sensor-a"`, `"sensor_id":"sensor-b"`, 1))
	if err != nil || status != 422 { t.Fatalf("packet identity mismatch status=%d err=%v", status, err) }
	status, _, err = send(authorized, "sensor-a", "5", "batch-c", "observations", string(bytes.Repeat([]byte("x"), sensoringest.MaxBatchBytes+1)))
	if err != nil || status != 413 { t.Fatalf("oversized status=%d err=%v", status, err) }
	conntrack := `{"observed_at":"2026-11-20T10:45:25Z","schema_version":"1","record_type":"conntrack_event","event_type":"new","original":{},"reply":{},"original_counters":{},"reply_counters":{}}` + "\n"
	status, _, err = send(authorized, "sensor-a", "1", "ct-a", "conntrack", conntrack)
	if err != nil || status != 201 { t.Fatalf("conntrack status=%d err=%v", status, err) }

	// A freshly constructed listener with the same custody recognizes replay.
	reopened, err := sensoringest.OpenCustody(dir)
	if err != nil { t.Fatal(err) }
	newHandler, err := NewHandler(reopened, IdentityPolicy{san.String(): "sensor-a"})
	if err != nil { t.Fatal(err) }
	srv.Close()
	restarted := httptest.NewUnstartedServer(newHandler)
	restarted.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool, Certificates: []tls.Certificate{serverPair}}
	restarted.StartTLS(); defer restarted.Close()
	newReq, err := http.NewRequest(http.MethodPost, restarted.URL+BatchPath, strings.NewReader(observationRow))
	if err != nil { t.Fatal(err) }
	newReq.Header.Set("Content-Type", "application/x-ndjson")
	newReq.Header.Set("X-Pathfinder-Sensor-ID", "sensor-a")
	newReq.Header.Set("X-Pathfinder-Stream", "observations")
	newReq.Header.Set("X-Pathfinder-Schema-Version", "5")
	newReq.Header.Set("X-Pathfinder-Batch-ID", "batch-a")
	response, err := authorized.Do(newReq)
	if err != nil { t.Fatal(err) }
	defer response.Body.Close()
	if response.StatusCode != 200 { t.Fatalf("restart replay status=%d", response.StatusCode) }
}
