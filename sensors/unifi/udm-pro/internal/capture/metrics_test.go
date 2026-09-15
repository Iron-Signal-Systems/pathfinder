package capture

import "testing"

func TestMetricsTracksQueueHighWater(t *testing.T) {
	metrics := NewMetrics([]string{"br0"})
	metrics.configureObservationQueue(8192)

	metrics.addQueued("br0", 1)
	metrics.addQueued("br0", 17)
	metrics.addQueued("br0", 4)

	snapshot := metrics.Snapshot()

	if snapshot.ObservationQueueCapacity != 8192 {
		t.Fatalf(
			"queue capacity = %d, want 8192",
			snapshot.ObservationQueueCapacity,
		)
	}
	if snapshot.ObservationQueueHighWater != 17 {
		t.Fatalf(
			"queue high water = %d, want 17",
			snapshot.ObservationQueueHighWater,
		)
	}
	if len(snapshot.Interfaces) != 1 {
		t.Fatalf("interfaces = %d, want 1", len(snapshot.Interfaces))
	}
	if snapshot.Interfaces[0].ObservationsQueued != 3 {
		t.Fatalf(
			"queued = %d, want 3",
			snapshot.Interfaces[0].ObservationsQueued,
		)
	}
}

func TestMetricsTracksSocketBuffer(t *testing.T) {
	metrics := NewMetrics([]string{"eth8"})
	metrics.setSocketReceiveBuffer("eth8", 8*1024*1024, 16*1024*1024, "forced")

	snapshot := metrics.Snapshot()
	current := snapshot.Interfaces[0]

	if current.SocketReceiveBufferRequested != 8*1024*1024 {
		t.Fatalf(
			"requested = %d, want %d",
			current.SocketReceiveBufferRequested,
			8*1024*1024,
		)
	}
	if current.SocketReceiveBufferBytes != 16*1024*1024 {
		t.Fatalf(
			"actual = %d, want %d",
			current.SocketReceiveBufferBytes,
			16*1024*1024,
		)
	}
	if current.SocketReceiveBufferMode != "forced" {
		t.Fatalf("mode = %q, want forced", current.SocketReceiveBufferMode)
	}
}

func TestMetricsTracksTimestampSources(t *testing.T) {
	metrics := NewMetrics([]string{"br0"})
	metrics.setTimestampMode("br0", "kernel_software_ns")

	metrics.addTimestampObservation("br0", "kernel_software_ns")
	metrics.addTimestampObservation("br0", "kernel_software_ns")
	metrics.addTimestampObservation("br0", "userspace_receive_fallback")
	metrics.addTimestampControlFailure("br0")

	snapshot := metrics.Snapshot()
	current := snapshot.Interfaces[0]

	if current.TimestampMode != "kernel_software_ns" {
		t.Fatalf(
			"timestamp mode = %q, want kernel_software_ns",
			current.TimestampMode,
		)
	}
	if current.KernelTimestampedFrames != 2 {
		t.Fatalf(
			"kernel timestamped = %d, want 2",
			current.KernelTimestampedFrames,
		)
	}
	if current.UserspaceTimestampFallbackFrames != 1 {
		t.Fatalf(
			"userspace timestamp fallbacks = %d, want 1",
			current.UserspaceTimestampFallbackFrames,
		)
	}
	if current.TimestampControlFailures != 1 {
		t.Fatalf(
			"timestamp control failures = %d, want 1",
			current.TimestampControlFailures,
		)
	}
}
