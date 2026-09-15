//go:build !linux

package interfaces

func enrich(discovered []Interface) error {
	return nil
}
