package storage

import (
	"fmt"
	"strconv"
	"strings"
)

var sizeMultipliers = map[string]uint64{
	"B":   1,
	"KB":  1000,
	"MB":  1000 * 1000,
	"GB":  1000 * 1000 * 1000,
	"KIB": 1024,
	"MIB": 1024 * 1024,
	"GIB": 1024 * 1024 * 1024,
}

// ParseBytes parses byte sizes such as 1GiB, 500MiB, 100GB, or 4096.
func ParseBytes(value string) (uint64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("size is empty")
	}

	upper := strings.ToUpper(value)

	unit := "B"
	number := upper

	for _, candidate := range []string{"GIB", "MIB", "KIB", "GB", "MB", "KB", "B"} {
		if strings.HasSuffix(upper, candidate) {
			unit = candidate
			number = strings.TrimSpace(upper[:len(upper)-len(candidate)])
			break
		}
	}

	if number == "" {
		return 0, fmt.Errorf("missing numeric size in %q", value)
	}

	parsed, err := strconv.ParseUint(number, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", value, err)
	}

	multiplier := sizeMultipliers[unit]
	if parsed > ^uint64(0)/multiplier {
		return 0, fmt.Errorf("size %q overflows uint64", value)
	}

	return parsed * multiplier, nil
}
