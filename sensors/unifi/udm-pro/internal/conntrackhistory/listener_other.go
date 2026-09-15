//go:build !linux

package conntrackhistory

import (
	"context"
	"fmt"
)

type Recorder struct{}

// NewRecorder reports that live conntrack capture requires Linux.
func NewRecorder(RecorderOptions) (*Recorder, error) {
	return nil, fmt.Errorf("conntrack history is supported only on linux")
}

// Close is a no-op on unsupported platforms.
func (recorder *Recorder) Close() error {
	return nil
}

// Run reports that live conntrack capture requires Linux.
func (recorder *Recorder) Run(context.Context) error {
	return fmt.Errorf("conntrack history is supported only on linux")
}

// Stats returns disabled state on unsupported platforms.
func (recorder *Recorder) Stats() Stats {
	return DisabledStats()
}
