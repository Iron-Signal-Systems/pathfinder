//go:build linux

package capture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/decode"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/interfaces"
	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

const (
	packetStatistics       = 6
	solPacket              = 263
	statisticsPollInterval = 5 * time.Second
)

type capturedObservation struct {
	Observation observation.Observation
}

type tpacketStats struct {
	Packets uint32
	Drops   uint32
}

// Observe captures Ethernet frames from the requested interfaces, extracts
// metadata, writes one JSON observation per line, and discards packet contents.
func Observe(parent context.Context, options Options) error {
	if len(options.Interfaces) == 0 {
		return fmt.Errorf("at least one --interface is required")
	}
	if options.Output == nil {
		options.Output = os.Stdout
	}
	if options.Count < 0 {
		return fmt.Errorf("count must be zero or greater")
	}
	if options.Metrics == nil {
		options.Metrics = NewMetrics(options.Interfaces)
	}
	if options.ObservationQueue <= 0 {
		options.ObservationQueue = 8192
	}
	if options.SocketBufferBytes <= 0 {
		options.SocketBufferBytes = 8 * 1024 * 1024
	}
	options.Metrics.configureObservationQueue(options.ObservationQueue)

	discovered, err := interfaces.Discover()
	if err != nil {
		return fmt.Errorf("discover interfaces: %w", err)
	}

	byName := make(map[string]interfaces.Interface, len(discovered))
	for _, networkInterface := range discovered {
		byName[networkInterface.Name] = networkInterface
	}

	selected := make([]interfaces.Interface, 0, len(options.Interfaces))
	seen := make(map[string]struct{}, len(options.Interfaces))
	for _, name := range options.Interfaces {
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}

		networkInterface, ok := byName[name]
		if !ok {
			return fmt.Errorf("interface %q not found", name)
		}
		selected = append(selected, networkInterface)
	}

	sensorID, err := os.Hostname()
	if err != nil || sensorID == "" {
		sensorID = observation.ValueNotKnown
	}

	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	observations := make(chan capturedObservation, options.ObservationQueue)
	errorsCh := make(chan error, len(selected))

	var captureWG sync.WaitGroup
	for _, networkInterface := range selected {
		networkInterface := networkInterface
		captureWG.Add(1)
		go func() {
			defer captureWG.Done()
			captureInterface(
				ctx,
				networkInterface,
				byName,
				sensorID,
				options.Metrics,
				options.SocketBufferBytes,
				observations,
				errorsCh,
			)
		}()
	}

	go func() {
		captureWG.Wait()
		close(observations)
	}()

	count := 0
	parentDone := parent.Done()
	writeEnabled := true
	var resultErr error

	for {
		select {
		case <-parentDone:
			parentDone = nil
			cancel()

		case err := <-errorsCh:
			if err != nil && resultErr == nil {
				resultErr = err
			}
			cancel()

		case captured, ok := <-observations:
			if !ok {
				if resultErr != nil {
					return resultErr
				}
				if err := parent.Err(); err != nil && !errors.Is(err, context.Canceled) {
					return err
				}
				return nil
			}

			if !writeEnabled {
				options.Metrics.addDiscarded()
				continue
			}

			data, err := json.Marshal(captured.Observation)
			if err != nil {
				resultErr = fmt.Errorf("encode observation: %w", err)
				writeEnabled = false
				cancel()
				options.Metrics.addDiscarded()
				continue
			}

			data = append(data, '\n')
			if _, err := options.Output.Write(data); err != nil {
				options.Metrics.addWriteFailure()
				resultErr = fmt.Errorf("write observation: %w", err)
				writeEnabled = false
				cancel()
				options.Metrics.addDiscarded()
				continue
			}
			options.Metrics.addWritten()

			count++
			if options.Count > 0 && count >= options.Count {
				writeEnabled = false
				cancel()
			}
		}
	}
}

func captureInterface(
	ctx context.Context,
	networkInterface interfaces.Interface,
	topology map[string]interfaces.Interface,
	sensorID string,
	metrics *Metrics,
	socketBufferBytes int,
	observations chan<- capturedObservation,
	errorsCh chan<- error,
) {
	protocol := int(hostToNetworkShort(syscall.ETH_P_ALL))
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, protocol)
	if err != nil {
		sendError(ctx, errorsCh, fmt.Errorf("open raw socket on %s: %w", networkInterface.Name, err))
		return
	}
	defer syscall.Close(fd)

	address := &syscall.SockaddrLinklayer{
		Protocol: hostToNetworkShort(syscall.ETH_P_ALL),
		Ifindex:  networkInterface.Index,
	}
	if err := syscall.Bind(fd, address); err != nil {
		sendError(ctx, errorsCh, fmt.Errorf("bind raw socket on %s: %w", networkInterface.Name, err))
		return
	}

	timestampMode, kernelTimestampingEnabled := configureKernelTimestamping(fd)
	metrics.setTimestampMode(networkInterface.Name, timestampMode)

	receiveBuffer, receiveBufferMode := configureSocketReceiveBuffer(
		fd,
		socketBufferBytes,
	)
	metrics.setSocketReceiveBuffer(
		networkInterface.Name,
		socketBufferBytes,
		receiveBuffer,
		receiveBufferMode,
	)

	timeout := syscall.NsecToTimeval(time.Second.Nanoseconds())
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeout); err != nil {
		sendError(ctx, errorsCh, fmt.Errorf("set receive timeout on %s: %w", networkInterface.Name, err))
		return
	}

	statisticsAvailable := true
	nextStatisticsPoll := time.Now().Add(statisticsPollInterval)
	defer func() {
		if !statisticsAvailable {
			return
		}

		packets, drops, err := readPacketStatistics(fd)
		if err != nil {
			metrics.markKernelStatisticsUnavailable(networkInterface.Name)
			return
		}
		metrics.addKernelStatistics(networkInterface.Name, packets, drops)
	}()

	// AF_PACKET may deliver GRO/GSO-coalesced frames larger than the interface
	// MTU. Keep enough receive space to preserve the kernel-delivered metadata
	// so those observations can be identified rather than silently truncated.
	buffer := make([]byte, 65536+256)
	control := make([]byte, syscall.CmsgSpace(16))

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, controlLength, receiveFlags, from, err := syscall.Recvmsg(
			fd,
			buffer,
			control,
			0,
		)
		receivedAt := time.Now().UTC()
		if err != nil {
			if errors.Is(err, syscall.EAGAIN) ||
				errors.Is(err, syscall.EWOULDBLOCK) ||
				errors.Is(err, syscall.EINTR) {
				pollPacketStatisticsIfDue(
					fd,
					networkInterface.Name,
					metrics,
					&statisticsAvailable,
					&nextStatisticsPoll,
				)
				continue
			}

			sendError(ctx, errorsCh, fmt.Errorf("receive frame on %s: %w", networkInterface.Name, err))
			return
		}
		if n <= 0 {
			continue
		}

		observedAt := receivedAt
		timestampSource := observation.TimestampSourceUserspaceFallback

		if kernelTimestampingEnabled {
			if receiveFlags&syscall.MSG_CTRUNC != 0 {
				metrics.addTimestampControlFailure(networkInterface.Name)
			} else {
				kernelObservedAt, ok, timestampErr := kernelTimestampFromControl(
					control[:controlLength],
				)
				if timestampErr != nil {
					metrics.addTimestampControlFailure(networkInterface.Name)
				} else if ok {
					observedAt = kernelObservedAt
					timestampSource = observation.TimestampSourceKernelSoftwareNS
				}
			}
		}

		metrics.addTimestampObservation(
			networkInterface.Name,
			timestampSource,
		)
		metrics.addUserspaceFrame(networkInterface.Name)

		observed, ok := decode.Frame(buffer[:n])
		if !ok {
			metrics.addRejected(networkInterface.Name)
			pollPacketStatisticsIfDue(
				fd,
				networkInterface.Name,
				metrics,
				&statisticsAvailable,
				&nextStatisticsPoll,
			)
			continue
		}
		metrics.addDecoded(networkInterface.Name)

		record := observation.New()
		record.ObservedAt = observedAt
		record.SensorID = sensorID
		record.TimestampSource = timestampSource
		record.Observed = observed
		record.Observed.Interface = networkInterface.Name
		record.Observed.InterfaceIndex = networkInterface.Index
		record.Context.Direction = packetDirection(from)
		record.Context.VLANID = networkInterface.AssociatedVLANID

		if (record.Observed.Network.Family == "ipv4" ||
			record.Observed.Network.Family == "ipv6") &&
			networkInterface.MTU > 0 &&
			record.Observed.Network.Length > uint32(networkInterface.MTU) {
			record.Context.KernelOffloadSuspected = true
		}

		if networkInterface.Kind == "bridge" {
			record.Context.SVI = networkInterface.Name
		} else if networkInterface.Master != interfaces.ValueNoRecord {
			if master, ok := topology[networkInterface.Master]; ok && master.Kind == "bridge" {
				record.Context.SVI = master.Name
			}
		}

		observations <- capturedObservation{Observation: record}
		metrics.addQueued(networkInterface.Name, len(observations))

		pollPacketStatisticsIfDue(
			fd,
			networkInterface.Name,
			metrics,
			&statisticsAvailable,
			&nextStatisticsPoll,
		)
	}
}

func configureSocketReceiveBuffer(fd int, requested int) (int, string) {
	mode := "default"

	if requested > 0 {
		if err := syscall.SetsockoptInt(
			fd,
			syscall.SOL_SOCKET,
			syscall.SO_RCVBUFFORCE,
			requested,
		); err == nil {
			mode = "forced"
		} else if err := syscall.SetsockoptInt(
			fd,
			syscall.SOL_SOCKET,
			syscall.SO_RCVBUF,
			requested,
		); err == nil {
			mode = "requested"
		} else {
			mode = "tuning_failed"
		}
	}

	actual, err := syscall.GetsockoptInt(
		fd,
		syscall.SOL_SOCKET,
		syscall.SO_RCVBUF,
	)
	if err != nil {
		return 0, mode + "_unreadable"
	}

	return actual, mode
}

func hostToNetworkShort(value uint16) uint16 {
	return (value<<8)&0xff00 | value>>8
}

func packetDirection(address syscall.Sockaddr) string {
	linkLayer, ok := address.(*syscall.SockaddrLinklayer)
	if !ok {
		return observation.ValueNotKnown
	}

	if linkLayer.Pkttype == syscall.PACKET_OUTGOING {
		return "egress"
	}

	return "ingress"
}

func pollPacketStatisticsIfDue(
	fd int,
	interfaceName string,
	metrics *Metrics,
	available *bool,
	nextPoll *time.Time,
) {
	if !*available || time.Now().Before(*nextPoll) {
		return
	}

	packets, drops, err := readPacketStatistics(fd)
	if err != nil {
		metrics.markKernelStatisticsUnavailable(interfaceName)
		*available = false
		return
	}

	metrics.addKernelStatistics(interfaceName, packets, drops)
	*nextPoll = time.Now().Add(statisticsPollInterval)
}

func readPacketStatistics(fd int) (uint64, uint64, error) {
	statistics := tpacketStats{}
	length := uint32(unsafe.Sizeof(statistics))

	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		uintptr(solPacket),
		uintptr(packetStatistics),
		uintptr(unsafe.Pointer(&statistics)),
		uintptr(unsafe.Pointer(&length)),
		0,
	)
	if errno != 0 {
		return 0, 0, errno
	}

	if length < uint32(unsafe.Sizeof(statistics)) {
		return 0, 0, fmt.Errorf("PACKET_STATISTICS returned %d bytes, expected at least %d", length, unsafe.Sizeof(statistics))
	}

	return uint64(statistics.Packets), uint64(statistics.Drops), nil
}

func sendError(ctx context.Context, errorsCh chan<- error, err error) {
	select {
	case <-ctx.Done():
	case errorsCh <- err:
	}
}
