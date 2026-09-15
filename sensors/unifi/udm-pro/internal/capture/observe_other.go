//go:build !linux

package capture

import (
	"context"
	"fmt"
)

// Observe captures live network metadata on supported Linux systems.
func Observe(context.Context, Options) error {
	return fmt.Errorf("live observation capture is supported only on linux")
}
