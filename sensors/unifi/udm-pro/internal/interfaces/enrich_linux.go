//go:build linux

package interfaces

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type vlanInfo struct {
	ID     int
	Parent string
}

func enrich(discovered []Interface) error {
	vlans, err := readVLANConfig("/proc/net/vlan/config")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read VLAN configuration: %w", err)
	}

	indexNames := make(map[int]string, len(discovered))
	for _, networkInterface := range discovered {
		indexNames[networkInterface.Index] = networkInterface.Name
	}

	for i := range discovered {
		networkInterface := &discovered[i]

		base := filepath.Join("/sys/class/net", networkInterface.Name)

		if value, err := readTrimmed(filepath.Join(base, "operstate")); err == nil {
			networkInterface.OperState = value
		}

		if value, err := readTrimmed(filepath.Join(base, "carrier")); err == nil {
			switch value {
			case "1":
				networkInterface.Carrier = "present"
			case "0":
				networkInterface.Carrier = "absent"
			default:
				networkInterface.Carrier = ValueNotKnown
			}
		}

		masterPath := filepath.Join(base, "master")
		if info, err := os.Lstat(masterPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
			if target, err := filepath.EvalSymlinks(masterPath); err == nil {
				networkInterface.Master = filepath.Base(target)
			}
		}

		if vlan, ok := vlans[networkInterface.Name]; ok {
			networkInterface.Kind = "vlan"
			networkInterface.Parent = vlan.Parent
			networkInterface.VLANID = vlan.ID
			continue
		}

		if info, err := os.Stat(filepath.Join(base, "bridge")); err == nil && info.IsDir() {
			networkInterface.Kind = "bridge"
			continue
		}

		if networkInterface.Name == "lo" {
			networkInterface.Kind = "loopback"
			continue
		}

		if iflinkText, err := readTrimmed(filepath.Join(base, "iflink")); err == nil {
			iflink, err := strconv.Atoi(iflinkText)
			if err == nil && iflink != networkInterface.Index {
				if parent, ok := indexNames[iflink]; ok {
					networkInterface.Parent = parent
				}
			}
		}
	}

	return nil
}

func readTrimmed(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

func readVLANConfig(path string) (map[string]vlanInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]vlanInfo)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" ||
			strings.HasPrefix(line, "VLAN Dev name") ||
			strings.HasPrefix(line, "Name-Type:") {
			continue
		}

		fields := strings.Split(line, "|")
		if len(fields) != 3 {
			continue
		}

		name := strings.TrimSpace(fields[0])
		idText := strings.TrimSpace(fields[1])
		parent := strings.TrimSpace(fields[2])

		id, err := strconv.Atoi(idText)
		if err != nil {
			continue
		}

		result[name] = vlanInfo{
			ID:     id,
			Parent: parent,
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
