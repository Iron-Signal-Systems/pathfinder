//go:build linux

package conntrackhistory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/storage"
)

const (
	conntrackGroupNew     = 1
	conntrackGroupUpdate  = 2
	conntrackGroupDestroy = 3

	defaultReceiveBuffer = 8 * 1024 * 1024
	receiveBufferBytes   = 1024 * 1024
)

type Recorder struct {
	fd      int
	metrics *metrics
	ring    *storage.Ring
}

// NewRecorder opens a read-only conntrack netlink subscription and a bounded
// immutable event store. It does not alter firewall rules or conntrack state.
func NewRecorder(options RecorderOptions) (*Recorder, error) {
	if options.Directory == "" {
		return nil, fmt.Errorf("conntrack history directory is required")
	}
	if options.SegmentBytes == 0 {
		return nil, fmt.Errorf("conntrack segment size must be greater than zero")
	}
	if options.MaxBytes < options.SegmentBytes {
		return nil, fmt.Errorf(
			"conntrack max size must be at least one segment",
		)
	}
	if options.SocketBufferBytes <= 0 {
		options.SocketBufferBytes = defaultReceiveBuffer
	}

	fd, err := syscall.Socket(
		syscall.AF_NETLINK,
		syscall.SOCK_RAW,
		syscall.NETLINK_NETFILTER,
	)
	if err != nil {
		return nil, fmt.Errorf("open NETLINK_NETFILTER socket: %w", err)
	}

	ok := false
	defer func() {
		if !ok {
			_ = syscall.Close(fd)
		}
	}()

	actualBuffer, bufferMode := configureReceiveBuffer(
		fd,
		options.SocketBufferBytes,
	)

	groups := uint32(
		1<<(conntrackGroupNew-1) |
			1<<(conntrackGroupUpdate-1) |
			1<<(conntrackGroupDestroy-1),
	)
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Groups: groups,
	}); err != nil {
		return nil, fmt.Errorf(
			"bind conntrack multicast groups: %w",
			err,
		)
	}

	timeout := syscall.NsecToTimeval(int64(time.Second))
	if err := syscall.SetsockoptTimeval(
		fd,
		syscall.SOL_SOCKET,
		syscall.SO_RCVTIMEO,
		&timeout,
	); err != nil {
		return nil, fmt.Errorf(
			"configure conntrack receive timeout: %w",
			err,
		)
	}

	ring, err := storage.NewRing(storage.RingOptions{
		Compression:  options.Compression,
		Directory:    options.Directory,
		MaxBytes:     options.MaxBytes,
		MinFreeBytes: options.MinFreeBytes,
		OnCompressionError: func(err error) {
			if options.OnError != nil {
				options.OnError(fmt.Errorf(
					"conntrack compression: %w",
					err,
				))
			}
		},
		Prefix:       "conntrack",
		SegmentBytes: options.SegmentBytes,
	})
	if err != nil {
		return nil, err
	}

	recorder := &Recorder{
		fd:      fd,
		metrics: newMetrics(),
		ring:    ring,
	}
	recorder.metrics.setSocketBuffer(
		options.SocketBufferBytes,
		actualBuffer,
		bufferMode,
	)

	ok = true
	return recorder, nil
}

// Close finalizes conntrack history storage and closes the netlink socket.
func (recorder *Recorder) Close() error {
	if recorder == nil {
		return nil
	}

	var socketErr error
	if recorder.fd >= 0 {
		if err := syscall.Close(recorder.fd); err != nil &&
			!errors.Is(err, syscall.EBADF) {
			socketErr = fmt.Errorf(
				"close conntrack netlink socket: %w",
				err,
			)
		}
		recorder.fd = -1
	}

	var storageErr error
	if recorder.ring != nil {
		storageErr = recorder.ring.Close()
		recorder.ring = nil
	}

	return errors.Join(socketErr, storageErr)
}

// Run receives conntrack multicast events until the context is cancelled.
func (recorder *Recorder) Run(ctx context.Context) error {
	if recorder == nil || recorder.fd < 0 || recorder.ring == nil {
		return fmt.Errorf("conntrack recorder is not initialized")
	}

	buffer := make([]byte, receiveBufferBytes)

	for {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}

		count, _, receiveFlags, _, err := syscall.Recvmsg(
			recorder.fd,
			buffer,
			nil,
			0,
		)
		if err != nil {
			switch {
			case errors.Is(err, syscall.EINTR):
				continue

			case errors.Is(err, syscall.EAGAIN),
				errors.Is(err, syscall.EWOULDBLOCK):
				continue

			case errors.Is(err, syscall.ENOBUFS):
				recorder.metrics.addReceiveOverrun()
				continue

			default:
				recorder.metrics.addReceiveError()
				return fmt.Errorf(
					"receive conntrack netlink event: %w",
					err,
				)
			}
		}
		if count == 0 {
			continue
		}
		if receiveFlags&syscall.MSG_TRUNC != 0 {
			recorder.metrics.addDatagramTruncated()
			continue
		}

		receivedAt := time.Now().UTC()
		messages, err := syscall.ParseNetlinkMessage(
			buffer[:count],
		)
		if err != nil {
			recorder.metrics.addParseRejected()
			continue
		}

		for _, message := range messages {
			event, recognized, parseErr := parseConntrackMessage(
				message.Header.Type,
				message.Header.Flags,
				message.Data,
			)
			if parseErr != nil {
				recorder.metrics.addParseRejected()
				continue
			}
			if !recognized {
				recorder.metrics.addIgnored()
				continue
			}

			event.ObservedAt = receivedAt
			recorder.metrics.addEvent(event.EventType)

			if err := recorder.write(event); err != nil {
				recorder.metrics.addWriteFailure()
				return err
			}
			recorder.metrics.addWritten()
		}
	}
}

// Stats returns a concurrency-safe point-in-time recorder snapshot.
func (recorder *Recorder) Stats() Stats {
	if recorder == nil {
		return DisabledStats()
	}
	return recorder.metrics.snapshot(recorder.ring)
}

func configureReceiveBuffer(
	fd int,
	requested int,
) (int, string) {
	mode := "standard"

	if err := syscall.SetsockoptInt(
		fd,
		syscall.SOL_SOCKET,
		syscall.SO_RCVBUFFORCE,
		requested,
	); err == nil {
		mode = "forced"
	} else {
		_ = syscall.SetsockoptInt(
			fd,
			syscall.SOL_SOCKET,
			syscall.SO_RCVBUF,
			requested,
		)
	}

	actual, err := syscall.GetsockoptInt(
		fd,
		syscall.SOL_SOCKET,
		syscall.SO_RCVBUF,
	)
	if err != nil {
		return 0, mode + "_actual_unknown"
	}
	return actual, mode
}

func (recorder *Recorder) write(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode conntrack event: %w", err)
	}
	data = append(data, '\n')

	if _, err := recorder.ring.Write(data); err != nil {
		return fmt.Errorf("write conntrack event: %w", err)
	}
	return nil
}
