//go:build linux

package capture

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"time"
)

const timestampModeKernelSoftwareNS = "kernel_software_ns"

func configureKernelTimestamping(fd int) (string, bool) {
	if err := syscall.SetsockoptInt(
		fd,
		syscall.SOL_SOCKET,
		syscall.SO_TIMESTAMPNS,
		1,
	); err != nil {
		return "userspace_fallback_socket_option_failed", false
	}

	return timestampModeKernelSoftwareNS, true
}

func kernelTimestampFromControl(
	control []byte,
) (time.Time, bool, error) {
	if len(control) == 0 {
		return time.Time{}, false, nil
	}

	messages, err := syscall.ParseSocketControlMessage(control)
	if err != nil {
		return time.Time{}, false, fmt.Errorf(
			"parse socket control message: %w",
			err,
		)
	}

	return kernelTimestampFromMessages(messages)
}

func kernelTimestampFromMessages(
	messages []syscall.SocketControlMessage,
) (time.Time, bool, error) {
	for _, message := range messages {
		if message.Header.Level != syscall.SOL_SOCKET ||
			message.Header.Type != syscall.SO_TIMESTAMPNS {
			continue
		}

		if len(message.Data) < 16 {
			return time.Time{}, false, fmt.Errorf(
				"SO_TIMESTAMPNS control data is %d bytes; expected at least 16",
				len(message.Data),
			)
		}

		seconds := int64(binary.NativeEndian.Uint64(message.Data[0:8]))
		nanoseconds := int64(binary.NativeEndian.Uint64(message.Data[8:16]))

		if nanoseconds < 0 || nanoseconds >= int64(time.Second) {
			return time.Time{}, false, fmt.Errorf(
				"SO_TIMESTAMPNS nanoseconds out of range: %d",
				nanoseconds,
			)
		}

		return time.Unix(seconds, nanoseconds).UTC(), true, nil
	}

	return time.Time{}, false, nil
}
