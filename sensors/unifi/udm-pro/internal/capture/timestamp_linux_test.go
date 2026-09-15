//go:build linux

package capture

import (
	"encoding/binary"
	"syscall"
	"testing"
	"time"
)

func TestKernelTimestampFromMessages(t *testing.T) {
	data := make([]byte, 16)
	binary.NativeEndian.PutUint64(data[0:8], uint64(1770000000))
	binary.NativeEndian.PutUint64(data[8:16], uint64(123456789))

	got, ok, err := kernelTimestampFromMessages(
		[]syscall.SocketControlMessage{
			{
				Header: syscall.Cmsghdr{
					Level: syscall.SOL_SOCKET,
					Type:  syscall.SO_TIMESTAMPNS,
				},
				Data: data,
			},
		},
	)
	if err != nil {
		t.Fatalf("kernelTimestampFromMessages() error = %v", err)
	}
	if !ok {
		t.Fatal("kernelTimestampFromMessages() ok = false, want true")
	}

	want := time.Unix(1770000000, 123456789).UTC()
	if !got.Equal(want) {
		t.Fatalf("timestamp = %s, want %s", got, want)
	}
}

func TestKernelTimestampFromMessagesIgnoresOtherControl(t *testing.T) {
	_, ok, err := kernelTimestampFromMessages(
		[]syscall.SocketControlMessage{
			{
				Header: syscall.Cmsghdr{
					Level: syscall.SOL_SOCKET,
					Type:  syscall.SO_RCVBUF,
				},
				Data: make([]byte, 16),
			},
		},
	)
	if err != nil {
		t.Fatalf("kernelTimestampFromMessages() error = %v", err)
	}
	if ok {
		t.Fatal("kernelTimestampFromMessages() ok = true, want false")
	}
}

func TestKernelTimestampFromMessagesRejectsShortTimestamp(t *testing.T) {
	_, _, err := kernelTimestampFromMessages(
		[]syscall.SocketControlMessage{
			{
				Header: syscall.Cmsghdr{
					Level: syscall.SOL_SOCKET,
					Type:  syscall.SO_TIMESTAMPNS,
				},
				Data: make([]byte, 8),
			},
		},
	)
	if err == nil {
		t.Fatal("kernelTimestampFromMessages() error = nil, want error")
	}
}

func TestKernelTimestampFromMessagesRejectsBadNanoseconds(t *testing.T) {
	data := make([]byte, 16)
	binary.NativeEndian.PutUint64(data[0:8], uint64(1770000000))
	binary.NativeEndian.PutUint64(data[8:16], uint64(time.Second))

	_, _, err := kernelTimestampFromMessages(
		[]syscall.SocketControlMessage{
			{
				Header: syscall.Cmsghdr{
					Level: syscall.SOL_SOCKET,
					Type:  syscall.SO_TIMESTAMPNS,
				},
				Data: data,
			},
		},
	)
	if err == nil {
		t.Fatal("kernelTimestampFromMessages() error = nil, want error")
	}
}
