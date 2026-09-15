//go:build linux

package interfaces

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadVLANConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")

	content := `VLAN Dev name	 | VLAN ID
Name-Type: VLAN_NAME_TYPE_RAW_PLUS_VID_NO_PAD
eth0           | 4094  | switch0
eth9.20        | 20  | eth9
switch0.20     | 20  | switch0
`

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	vlans, err := readVLANConfig(path)
	if err != nil {
		t.Fatalf("readVLANConfig() error = %v", err)
	}

	if vlans["eth9.20"].ID != 20 {
		t.Fatalf("eth9.20 VLAN = %d, want 20", vlans["eth9.20"].ID)
	}

	if vlans["eth9.20"].Parent != "eth9" {
		t.Fatalf(
			"eth9.20 parent = %q, want %q",
			vlans["eth9.20"].Parent,
			"eth9",
		)
	}

	if vlans["eth0"].ID != 4094 {
		t.Fatalf("eth0 VLAN = %d, want 4094", vlans["eth0"].ID)
	}
}
