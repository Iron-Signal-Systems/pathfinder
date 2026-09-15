package storage

import "testing"

func TestParseBytes(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"1GiB", 1024 * 1024 * 1024},
		{"100MiB", 100 * 1024 * 1024},
		{"2GB", 2 * 1000 * 1000 * 1000},
		{"4096", 4096},
	}

	for _, test := range tests {
		got, err := ParseBytes(test.input)
		if err != nil {
			t.Fatalf("ParseBytes(%q) error = %v", test.input, err)
		}
		if got != test.want {
			t.Fatalf(
				"ParseBytes(%q) = %d, want %d",
				test.input,
				got,
				test.want,
			)
		}
	}
}
