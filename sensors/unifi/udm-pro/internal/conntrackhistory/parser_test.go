package conntrackhistory

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestParseConntrackNewTCPIPv4(t *testing.T) {
	original := tupleAttr(
		addressFamilyIPv4,
		net.ParseIP("192.168.1.193").To4(),
		net.ParseIP("172.64.148.235").To4(),
		6,
		55387,
		443,
	)
	reply := tupleAttr(
		addressFamilyIPv4,
		net.ParseIP("172.64.148.235").To4(),
		net.ParseIP("100.65.193.174").To4(),
		6,
		443,
		55387,
	)

	payload := nfgen(
		addressFamilyIPv4,
		attrNested(ctaTupleOriginal, original),
		attrNested(ctaTupleReply, reply),
		attrU32(
			ctaStatus,
			statusConfirmed|statusSourceNAT,
		),
		attrU32(ctaMark, 1703936),
		attrU32(ctaTimeout, 120),
		attrNested(
			ctaProtoInfo,
			attrNested(
				ctaProtoInfoTCP,
				attrBytes(
					ctaProtoInfoTCPState,
					[]byte{1},
				),
			),
		),
	)

	event, recognized, err := parseConntrackMessage(
		uint16(nfNetlinkSubsystemConntrack<<8|ipctnlMessageNew),
		netlinkFlagCreate|netlinkFlagExcl,
		payload,
	)
	if err != nil {
		t.Fatalf("parseConntrackMessage() error = %v", err)
	}
	if !recognized {
		t.Fatal("recognized = false, want true")
	}

	if event.EventType != EventNew {
		t.Fatalf("event type = %q, want new", event.EventType)
	}
	if event.Original.SourceIP != "192.168.1.193" ||
		event.Original.DestinationIP != "172.64.148.235" {
		t.Fatalf("unexpected original tuple: %+v", event.Original)
	}
	if event.Original.SourcePort != 55387 ||
		event.Original.DestinationPort != 443 {
		t.Fatalf("unexpected original ports: %+v", event.Original)
	}
	if event.Reply.DestinationIP != "100.65.193.174" ||
		event.Reply.DestinationPort != 55387 {
		t.Fatalf("unexpected reply tuple: %+v", event.Reply)
	}
	if event.TCPState != "SYN_SENT" {
		t.Fatalf("TCP state = %q, want SYN_SENT", event.TCPState)
	}
	if !event.Confirmed {
		t.Fatal("Confirmed = false, want true")
	}
	if !event.SourceNAT {
		t.Fatal("SourceNAT = false, want true")
	}
	if event.DestinationNAT {
		t.Fatal("DestinationNAT = true, want false")
	}
	if !event.Unreplied {
		t.Fatal("Unreplied = false, want true")
	}
	if event.Mark != 1703936 {
		t.Fatalf("mark = %d, want 1703936", event.Mark)
	}
}

func TestParseConntrackUpdateClassification(t *testing.T) {
	payload := basicTCPPayload(t)

	event, recognized, err := parseConntrackMessage(
		uint16(nfNetlinkSubsystemConntrack<<8|ipctnlMessageNew),
		0,
		payload,
	)
	if err != nil {
		t.Fatalf("parseConntrackMessage() error = %v", err)
	}
	if !recognized {
		t.Fatal("recognized = false, want true")
	}
	if event.EventType != EventUpdate {
		t.Fatalf(
			"event type = %q, want %q",
			event.EventType,
			EventUpdate,
		)
	}
}

func TestParseConntrackDestroyClassification(t *testing.T) {
	payload := basicTCPPayload(t)

	event, recognized, err := parseConntrackMessage(
		uint16(
			nfNetlinkSubsystemConntrack<<8|
				ipctnlMessageDelete,
		),
		0,
		payload,
	)
	if err != nil {
		t.Fatalf("parseConntrackMessage() error = %v", err)
	}
	if !recognized {
		t.Fatal("recognized = false, want true")
	}
	if event.EventType != EventDestroy {
		t.Fatalf(
			"event type = %q, want %q",
			event.EventType,
			EventDestroy,
		)
	}
}

func TestParseConntrackRejectsIncompleteTuple(t *testing.T) {
	payload := nfgen(
		addressFamilyIPv4,
		attrNested(
			ctaTupleOriginal,
			attrNested(
				ctaTupleIP,
				attrBytes(
					ctaIPv4Source,
					net.ParseIP("192.168.1.1").To4(),
				),
			),
		),
	)

	_, _, err := parseConntrackMessage(
		uint16(nfNetlinkSubsystemConntrack<<8|ipctnlMessageNew),
		netlinkFlagCreate|netlinkFlagExcl,
		payload,
	)
	if err == nil {
		t.Fatal("parseConntrackMessage() error = nil, want error")
	}
}

func basicTCPPayload(t *testing.T) []byte {
	t.Helper()
	return nfgen(
		addressFamilyIPv4,
		attrNested(
			ctaTupleOriginal,
			tupleAttr(
				addressFamilyIPv4,
				net.ParseIP("192.168.1.193").To4(),
				net.ParseIP("172.64.148.235").To4(),
				6,
				55387,
				443,
			),
		),
		attrNested(
			ctaTupleReply,
			tupleAttr(
				addressFamilyIPv4,
				net.ParseIP("172.64.148.235").To4(),
				net.ParseIP("100.65.193.174").To4(),
				6,
				443,
				55387,
			),
		),
	)
}

func attrBytes(kind uint16, payload []byte) []byte {
	length := netlinkAttributeHeaderLength + len(payload)
	aligned := align4(length)
	result := make([]byte, aligned)
	binary.NativeEndian.PutUint16(result[0:2], uint16(length))
	binary.NativeEndian.PutUint16(result[2:4], kind)
	copy(result[4:length], payload)
	return result
}

func attrNested(kind uint16, children ...[]byte) []byte {
	payload := []byte{}
	for _, child := range children {
		payload = append(payload, child...)
	}
	return attrBytes(kind|0x8000, payload)
}

func attrU32(kind uint16, value uint32) []byte {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, value)
	return attrBytes(kind, payload)
}

func nfgen(family uint8, attributes ...[]byte) []byte {
	result := []byte{family, 0, 0, 0}
	for _, attribute := range attributes {
		result = append(result, attribute...)
	}
	return result
}

func tupleAttr(
	family uint8,
	sourceIP []byte,
	destinationIP []byte,
	protocol uint8,
	sourcePort uint16,
	destinationPort uint16,
) []byte {
	sourceKind := uint16(ctaIPv4Source)
	destinationKind := uint16(ctaIPv4Destination)
	if family == addressFamilyIPv6 {
		sourceKind = ctaIPv6Source
		destinationKind = ctaIPv6Destination
	}

	sourcePortBytes := make([]byte, 2)
	destinationPortBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(sourcePortBytes, sourcePort)
	binary.BigEndian.PutUint16(
		destinationPortBytes,
		destinationPort,
	)

	return append(
		attrNested(
			ctaTupleIP,
			attrBytes(sourceKind, sourceIP),
			attrBytes(destinationKind, destinationIP),
		),
		attrNested(
			ctaTupleProto,
			attrBytes(ctaProtoNumber, []byte{protocol}),
			attrBytes(ctaProtoSourcePort, sourcePortBytes),
			attrBytes(
				ctaProtoDestinationPort,
				destinationPortBytes,
			),
		)...,
	)
}
