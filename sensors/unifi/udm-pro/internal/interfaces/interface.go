package interfaces

import (
	"fmt"
	"net"
	"sort"
)

const (
	ValueNoRecord = "no_record"
	ValueNotKnown = "not_known"
)

// Interface describes one network interface visible to the sensor.
type Interface struct {
	Addresses            []string `json:"addresses"`
	AdminUp              bool     `json:"admin_up"`
	AssociatedVLANID     int      `json:"associated_vlan_id"`
	AssociatedVLANSource string   `json:"associated_vlan_source"`
	Carrier              string   `json:"carrier"`
	HardwareAddr         string   `json:"hardware_addr"`
	Index                int      `json:"index"`
	Kind                 string   `json:"kind"`
	Master               string   `json:"master"`
	Members              []string `json:"members"`
	MTU                  int      `json:"mtu"`
	Name                 string   `json:"name"`
	OperState            string   `json:"oper_state"`
	Parent               string   `json:"parent"`
	VLANID               int      `json:"vlan_id"`
}

// Discover returns the network interfaces visible to the operating system.
func Discover() ([]Interface, error) {
	systemInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}

	discovered := make([]Interface, 0, len(systemInterfaces))

	for _, systemInterface := range systemInterfaces {
		addresses, err := systemInterface.Addrs()
		if err != nil {
			return nil, fmt.Errorf(
				"list addresses for interface %s: %w",
				systemInterface.Name,
				err,
			)
		}

		addressStrings := make([]string, 0, len(addresses))
		for _, address := range addresses {
			addressStrings = append(addressStrings, address.String())
		}

		sort.Strings(addressStrings)

		hardwareAddr := ValueNoRecord
		if len(systemInterface.HardwareAddr) != 0 {
			hardwareAddr = systemInterface.HardwareAddr.String()
		}

		discovered = append(discovered, Interface{
			Addresses:            addressStrings,
			AdminUp:              systemInterface.Flags&net.FlagUp != 0,
			AssociatedVLANID:     0,
			AssociatedVLANSource: ValueNoRecord,
			Carrier:              ValueNotKnown,
			HardwareAddr:         hardwareAddr,
			Index:                systemInterface.Index,
			Kind:                 ValueNotKnown,
			Master:               ValueNoRecord,
			Members:              []string{},
			MTU:                  systemInterface.MTU,
			Name:                 systemInterface.Name,
			OperState:            ValueNotKnown,
			Parent:               ValueNoRecord,
			VLANID:               0,
		})
	}

	if err := enrich(discovered); err != nil {
		return nil, err
	}

	associate(discovered)

	sort.Slice(discovered, func(i, j int) bool {
		return discovered[i].Index < discovered[j].Index
	})

	return discovered, nil
}
