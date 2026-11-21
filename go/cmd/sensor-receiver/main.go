// sensor-receiver is a development-only mTLS custody endpoint. It does not
// provide database indexing, a delivery checkpoint, or full ingestion acceptance.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sensoringest"
	"github.com/Iron-Signal-Systems/pathfinder/go/internal/sensorreceiver"
)

func main() {
	if err := run(); err != nil { log.Fatalf("sensor receiver stopped: %v", err) }
}

func run() error {
	listen := flag.String("listen", "127.0.0.1:9443", "literal IPv4/IPv6 listening address (no wildcard)")
	certFile := flag.String("server-cert", "", "PEM server certificate path")
	keyFile := flag.String("server-key", "", "PEM server private key path")
	caFile := flag.String("client-ca", "", "dedicated sensor-client trust CA PEM")
	allowFile := flag.String("identities", "", "JSON map of allowed URI SAN -> sensor ID")
	custodyDir := flag.String("custody-dir", "", "private on-disk batch custody directory")
	flag.Parse()
	if flag.NArg() > 0 { return fmt.Errorf("unexpected positional arguments") }
	if *certFile == "" || *keyFile == "" || *caFile == "" || *allowFile == "" || *custodyDir == "" {
		return fmt.Errorf("server-cert, server-key, client-ca, identities and custody-dir are required")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || net.ParseIP(host) == nil || net.ParseIP(host).IsUnspecified() {
		return fmt.Errorf("listen must be a literal, non-wildcard IP address and port")
	}
	pair, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil { return fmt.Errorf("load TLS server certificate: %w", err) }
	caPEM, err := os.ReadFile(*caFile)
	if err != nil { return fmt.Errorf("read dedicated client CA: %w", err) }
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) { return fmt.Errorf("client CA file has no usable certificates") }
	identities, err := readIdentities(*allowFile)
	if err != nil { return err }
	custody, err := sensoringest.OpenCustody(*custodyDir)
	if err != nil { return fmt.Errorf("open custody: %w",err) }
	handler, err := sensorreceiver.NewHandler(custody,identities)
	if err != nil { return err }
	server := &http.Server{
		Addr: *listen,
		Handler: handler,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs: pool,
			Certificates: []tls.Certificate{pair},
		},
		ReadHeaderTimeout: 5*time.Second,
		ReadTimeout: 30*time.Second,
		WriteTimeout: 45*time.Second,
		IdleTimeout: 60*time.Second,
		MaxHeaderBytes: 16<<10,
	}
	ctx, cancel := signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM)
	defer cancel()
	errorsCh := make(chan error,1)
	go func(){errorsCh<-server.ListenAndServeTLS("","")}()
	log.Printf("Pathfinder development sensor custody receiver listening on %s (mTLS)",*listen)
	select {
	case err := <-errorsCh:
		if errors.Is(err,http.ErrServerClosed) { return nil }
		return err
	case <-ctx.Done():
		closeCtx, done := context.WithTimeout(context.Background(),10*time.Second)
		defer done()
		return server.Shutdown(closeCtx)
	}
}

func readIdentities(path string) (sensorreceiver.IdentityPolicy,error) {
	info, err := os.Lstat(path)
	if err != nil { return nil,fmt.Errorf("identity map: %w",err) }
	if !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return nil,fmt.Errorf("identity map must be regular and not writable by group/other")
	}
	raw, err := os.ReadFile(path)
	if err != nil { return nil,fmt.Errorf("identity map: %w",err) }
	var m sensorreceiver.IdentityPolicy
	if err := json.Unmarshal(raw,&m); err != nil { return nil,fmt.Errorf("invalid identity map JSON") }
	return m,nil
}
