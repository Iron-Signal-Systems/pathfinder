package capture

import "io"

// Options controls live observation capture.
type Options struct {
	Count             int
	Interfaces        []string
	Metrics           *Metrics
	ObservationQueue  int
	Output            io.Writer
	SocketBufferBytes int
}
