package interfaces

import "testing"

func TestDiscoverReturnsInterfaces(t *testing.T) {
	discovered, err := Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(discovered) == 0 {
		t.Fatal("Discover() returned no interfaces")
	}
}

func TestDiscoverInterfacesHaveNames(t *testing.T) {
	discovered, err := Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	for _, networkInterface := range discovered {
		if networkInterface.Name == "" {
			t.Fatalf(
				"interface index %d has empty name",
				networkInterface.Index,
			)
		}
	}
}
