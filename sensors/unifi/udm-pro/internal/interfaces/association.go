package interfaces

import "sort"

func associate(discovered []Interface) {
	byName := make(map[string]*Interface, len(discovered))

	for i := range discovered {
		networkInterface := &discovered[i]
		byName[networkInterface.Name] = networkInterface
	}

	for i := range discovered {
		networkInterface := &discovered[i]

		if networkInterface.Kind == "vlan" && networkInterface.VLANID != 0 {
			networkInterface.AssociatedVLANID = networkInterface.VLANID
			networkInterface.AssociatedVLANSource = "interface_vlan"
		}

		if networkInterface.Master == ValueNoRecord {
			continue
		}

		master, ok := byName[networkInterface.Master]
		if !ok {
			continue
		}

		master.Members = append(master.Members, networkInterface.Name)
	}

	for i := range discovered {
		networkInterface := &discovered[i]

		sort.Strings(networkInterface.Members)

		if networkInterface.Kind != "bridge" {
			continue
		}

		vlanID := 0
		conflict := false

		for _, memberName := range networkInterface.Members {
			member, ok := byName[memberName]
			if !ok || member.VLANID == 0 {
				continue
			}

			if vlanID == 0 {
				vlanID = member.VLANID
				continue
			}

			if vlanID != member.VLANID {
				conflict = true
				break
			}
		}

		if vlanID != 0 && !conflict {
			networkInterface.AssociatedVLANID = vlanID
			networkInterface.AssociatedVLANSource = "bridge_membership"
		}
	}
}
