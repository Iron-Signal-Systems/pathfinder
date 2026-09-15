package interfaces

import (
	"reflect"
	"testing"
)

func TestAssociateBridgeWithVLAN(t *testing.T) {
	discovered := []Interface{
		{
			Kind:    "vlan",
			Master:  "br20",
			Name:    "eth9.20",
			VLANID:  20,
			Members: []string{},
		},
		{
			Kind:    "vlan",
			Master:  "br20",
			Name:    "switch0.20",
			VLANID:  20,
			Members: []string{},
		},
		{
			Kind:    "bridge",
			Master:  ValueNoRecord,
			Name:    "br20",
			Members: []string{},
		},
	}

	associate(discovered)

	bridge := discovered[2]

	if bridge.AssociatedVLANID != 20 {
		t.Fatalf(
			"AssociatedVLANID = %d, want 20",
			bridge.AssociatedVLANID,
		)
	}

	if bridge.AssociatedVLANSource != "bridge_membership" {
		t.Fatalf(
			"AssociatedVLANSource = %q, want %q",
			bridge.AssociatedVLANSource,
			"bridge_membership",
		)
	}

	wantMembers := []string{
		"eth9.20",
		"switch0.20",
	}

	if !reflect.DeepEqual(bridge.Members, wantMembers) {
		t.Fatalf(
			"Members = %#v, want %#v",
			bridge.Members,
			wantMembers,
		)
	}
}

func TestAssociateVLANInterface(t *testing.T) {
	discovered := []Interface{
		{
			Kind:    "vlan",
			Master:  "br20",
			Name:    "eth9.20",
			VLANID:  20,
			Members: []string{},
		},
		{
			Kind:    "bridge",
			Master:  ValueNoRecord,
			Name:    "br20",
			Members: []string{},
		},
	}

	associate(discovered)

	if discovered[0].AssociatedVLANID != 20 {
		t.Fatalf(
			"AssociatedVLANID = %d, want 20",
			discovered[0].AssociatedVLANID,
		)
	}

	if discovered[0].AssociatedVLANSource != "interface_vlan" {
		t.Fatalf(
			"AssociatedVLANSource = %q, want %q",
			discovered[0].AssociatedVLANSource,
			"interface_vlan",
		)
	}
}
