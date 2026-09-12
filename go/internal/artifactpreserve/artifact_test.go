package artifactpreserve

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPreserveAndReuse(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	objects := filepath.Join(root, "objects")
	run := filepath.Join(root, "run")

	if err := os.Mkdir(objects, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(run, 0750); err != nil {
		t.Fatal(err)
	}

	socket := filepath.Join(run, "preserve.sock")

	server := Server{
		MaxBytes:   1024,
		ObjectsDir: objects,
		SocketPath: socket,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)

	go func() {
		result <- server.Serve(ctx)
	}()

	waitForSocket(t, socket)

	content := []byte("exact source bytes\n")

	first, err := Preserve(
		context.Background(),
		socket,
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(content)
	expectedDigest := fmt.Sprintf("%x", sum)

	if first.SHA256 != expectedDigest {
		t.Fatalf(
			"digest=%s expected=%s",
			first.SHA256,
			expectedDigest,
		)
	}

	if first.Reused {
		t.Fatal("first preservation unexpectedly reused an object")
	}

	if first.ByteLength != int64(len(content)) {
		t.Fatalf(
			"length=%d expected=%d",
			first.ByteLength,
			len(content),
		)
	}

	final := filepath.Join(
		objects,
		"sha256",
		expectedDigest[:2],
		expectedDigest,
	)

	stored, err := os.ReadFile(final)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(stored, content) {
		t.Fatal("stored object differs from exact input bytes")
	}

	second, err := Preserve(
		context.Background(),
		socket,
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !second.Reused {
		t.Fatal("second preservation did not reuse matching object")
	}

	if second != (Receipt{
		ByteLength:       first.ByteLength,
		Reused:           true,
		SHA256:           first.SHA256,
		StorageReference: first.StorageReference,
	}) {
		t.Fatalf(
			"unexpected reuse receipt: %#v",
			second,
		)
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}

	case <-time.After(3 * time.Second):
		t.Fatal("artifact server did not stop")
	}
}

func TestStartupRemovesStaleIncomingFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	objects := filepath.Join(root, "objects")
	run := filepath.Join(root, "run")

	if err := os.Mkdir(objects, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(run, 0750); err != nil {
		t.Fatal(err)
	}

	stale := filepath.Join(
		objects,
		".incoming-stale",
	)

	if err := os.WriteFile(
		stale,
		[]byte("partial"),
		0600,
	); err != nil {
		t.Fatal(err)
	}

	socket := filepath.Join(run, "preserve.sock")

	server := Server{
		MaxBytes:   1024,
		ObjectsDir: objects,
		SocketPath: socket,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)

	go func() {
		result <- server.Serve(ctx)
	}()

	waitForSocket(t, socket)

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf(
			"stale incoming file still exists: %v",
			err,
		)
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}

	case <-time.After(3 * time.Second):
		t.Fatal("artifact server did not stop")
	}
}

func TestRejectsOversize(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	objects := filepath.Join(root, "objects")
	run := filepath.Join(root, "run")

	if err := os.Mkdir(objects, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(run, 0750); err != nil {
		t.Fatal(err)
	}

	socket := filepath.Join(run, "preserve.sock")

	server := Server{
		MaxBytes:   4,
		ObjectsDir: objects,
		SocketPath: socket,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)

	go func() {
		result <- server.Serve(ctx)
	}()

	waitForSocket(t, socket)

	_, err := Preserve(
		context.Background(),
		socket,
		bytes.NewReader([]byte("12345")),
	)
	if err == nil {
		t.Fatal("oversize artifact unexpectedly accepted")
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}

	case <-time.After(3 * time.Second):
		t.Fatal("artifact server did not stop")
	}
}

func TestProbeFailsWhenObjectStoreUnavailable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	objects := filepath.Join(root, "objects")
	run := filepath.Join(root, "run")

	if err := os.Mkdir(objects, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(run, 0750); err != nil {
		t.Fatal(err)
	}

	socket := filepath.Join(run, "preserve.sock")
	server := Server{
		MaxBytes:   1024,
		ObjectsDir: objects,
		SocketPath: socket,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)
	go func() {
		result <- server.Serve(ctx)
	}()

	waitForSocket(t, socket)

	if err := os.Remove(objects); err != nil {
		t.Fatal(err)
	}

	probeCtx, probeCancel := context.WithTimeout(
		context.Background(),
		500*time.Millisecond,
	)
	defer probeCancel()

	if err := Probe(probeCtx, socket); err == nil {
		t.Fatal("probe unexpectedly passed without artifact object store")
	}

	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("artifact server did not stop")
	}
}

func TestProbeHonorsContextDeadlineAfterDial(t *testing.T) {
	t.Parallel()

	socket := filepath.Join(t.TempDir(), "stall.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	accepted := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		close(accepted)
		time.Sleep(750 * time.Millisecond)
	}()

	probeCtx, cancel := context.WithTimeout(
		context.Background(),
		100*time.Millisecond,
	)
	defer cancel()

	started := time.Now()
	err = Probe(probeCtx, socket)
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("probe unexpectedly succeeded against stalled service")
	}

	if elapsed > 400*time.Millisecond {
		t.Fatalf("probe exceeded context deadline window: %v", elapsed)
	}

	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("stalled test server never accepted connection")
	}
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for {
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"socket did not appear: %s",
				path,
			)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestProbeDoesNotCreateObject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	objects := filepath.Join(root, "objects")
	run := filepath.Join(root, "run")

	if err := os.Mkdir(objects, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(run, 0750); err != nil {
		t.Fatal(err)
	}

	socket := filepath.Join(run, "preserve.sock")

	server := Server{
		MaxBytes:   1024,
		ObjectsDir: objects,
		SocketPath: socket,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)

	go func() {
		result <- server.Serve(ctx)
	}()

	waitForSocket(t, socket)

	if err := Probe(context.Background(), socket); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(objects)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatalf(
			"probe created object-store entries: %d",
			len(entries),
		)
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}

	case <-time.After(3 * time.Second):
		t.Fatal("artifact server did not stop")
	}
}

func TestRejectsUnframedLegacyRequestWithoutObject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	objects := filepath.Join(root, "objects")
	run := filepath.Join(root, "run")

	if err := os.Mkdir(objects, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(run, 0750); err != nil {
		t.Fatal(err)
	}

	socket := filepath.Join(run, "preserve.sock")

	server := Server{
		MaxBytes:   1024,
		ObjectsDir: objects,
		SocketPath: socket,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan error, 1)

	go func() {
		result <- server.Serve(ctx)
	}()

	waitForSocket(t, socket)

	raw, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}

	connection, ok := raw.(*net.UnixConn)
	if !ok {
		_ = raw.Close()
		t.Fatal("test connection is not Unix socket")
	}

	if _, err := connection.Write([]byte("legacy raw bytes")); err != nil {
		_ = connection.Close()
		t.Fatal(err)
	}

	if err := connection.CloseWrite(); err != nil {
		_ = connection.Close()
		t.Fatal(err)
	}

	var resultResponse response

	if err := json.NewDecoder(connection).Decode(&resultResponse); err != nil {
		_ = connection.Close()
		t.Fatal(err)
	}

	_ = connection.Close()

	if resultResponse.OK {
		t.Fatal("legacy unframed request unexpectedly succeeded")
	}

	entries, err := os.ReadDir(objects)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Fatalf(
			"legacy request created object-store entries: %d",
			len(entries),
		)
	}

	cancel()

	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}

	case <-time.After(3 * time.Second):
		t.Fatal("artifact server did not stop")
	}
}
