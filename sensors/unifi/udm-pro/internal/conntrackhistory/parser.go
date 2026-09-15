package conntrackhistory

import (
	"encoding/binary"
	"fmt"
	"net"
	"sort"
)

const (
	nfNetlinkSubsystemConntrack = 1

	ipctnlMessageNew    = 0
	ipctnlMessageDelete = 2

	netlinkFlagCreate = 0x400
	netlinkFlagExcl   = 0x200

	netlinkAttributeHeaderLength = 4
	netlinkAttributeTypeMask     = 0x3fff

	ctaTupleOriginal = 1
	ctaTupleReply    = 2
	ctaStatus        = 3
	ctaProtoInfo     = 4
	ctaTimeout       = 7
	ctaMark          = 8
	ctaCountersOrig  = 9
	ctaCountersReply = 10
	ctaID            = 12

	ctaTupleIP    = 1
	ctaTupleProto = 2

	ctaIPv4Source      = 1
	ctaIPv4Destination = 2
	ctaIPv6Source      = 3
	ctaIPv6Destination = 4

	ctaProtoNumber          = 1
	ctaProtoSourcePort      = 2
	ctaProtoDestinationPort = 3
	ctaProtoICMPID          = 4
	ctaProtoICMPType        = 5
	ctaProtoICMPCode        = 6
	ctaProtoICMPv6ID        = 7
	ctaProtoICMPv6Type      = 8
	ctaProtoICMPv6Code      = 9

	ctaProtoInfoTCP      = 1
	ctaProtoInfoTCPState = 1

	ctaCountersPackets = 1
	ctaCountersBytes   = 2

	addressFamilyIPv4 = 2
	addressFamilyIPv6 = 10

	statusExpected        = 1 << 0
	statusSeenReply       = 1 << 1
	statusAssured         = 1 << 2
	statusConfirmed       = 1 << 3
	statusSourceNAT       = 1 << 4
	statusDestinationNAT  = 1 << 5
	statusDying           = 1 << 9
	statusOffload         = 1 << 14
	statusHardwareOffload = 1 << 15
)

type netlinkAttribute struct {
	data []byte
	kind uint16
}

func parseConntrackMessage(
	messageType uint16,
	flags uint16,
	payload []byte,
) (Event, bool, error) {
	if int(messageType>>8) != nfNetlinkSubsystemConntrack {
		return Event{}, false, nil
	}

	eventType := ""
	switch int(messageType & 0x00ff) {
	case ipctnlMessageNew:
		if flags&netlinkFlagCreate != 0 &&
			flags&netlinkFlagExcl != 0 {
			eventType = EventNew
		} else {
			eventType = EventUpdate
		}

	case ipctnlMessageDelete:
		eventType = EventDestroy

	default:
		return Event{}, false, nil
	}

	if len(payload) < 4 {
		return Event{}, false, fmt.Errorf(
			"conntrack payload is %d bytes; expected nfgenmsg",
			len(payload),
		)
	}

	family := payload[0]
	attributes, err := parseAttributes(payload[4:])
	if err != nil {
		return Event{}, false, fmt.Errorf(
			"parse conntrack attributes: %w",
			err,
		)
	}

	event := Event{
		EventType:       eventType,
		RecordType:      RecordType,
		SchemaVersion:   SchemaVersion,
		TimestampSource: TimestampSourceUserspaceReceive,
		TCPState:        "not_known",
	}

	for _, attribute := range attributes {
		switch attribute.kind {
		case ctaTupleOriginal:
			event.Original, err = parseTuple(family, attribute.data)
			if err != nil {
				return Event{}, false, fmt.Errorf(
					"parse original tuple: %w",
					err,
				)
			}

		case ctaTupleReply:
			event.Reply, err = parseTuple(family, attribute.data)
			if err != nil {
				return Event{}, false, fmt.Errorf(
					"parse reply tuple: %w",
					err,
				)
			}

		case ctaStatus:
			if len(attribute.data) < 4 {
				return Event{}, false, fmt.Errorf(
					"CTA_STATUS has %d bytes",
					len(attribute.data),
				)
			}
			event.Status = binary.BigEndian.Uint32(attribute.data[:4])

		case ctaProtoInfo:
			event.TCPState, err = parseTCPState(attribute.data)
			if err != nil {
				return Event{}, false, err
			}

		case ctaTimeout:
			if len(attribute.data) < 4 {
				return Event{}, false, fmt.Errorf(
					"CTA_TIMEOUT has %d bytes",
					len(attribute.data),
				)
			}
			event.TimeoutSeconds = binary.BigEndian.Uint32(
				attribute.data[:4],
			)

		case ctaMark:
			if len(attribute.data) < 4 {
				return Event{}, false, fmt.Errorf(
					"CTA_MARK has %d bytes",
					len(attribute.data),
				)
			}
			event.Mark = binary.BigEndian.Uint32(attribute.data[:4])

		case ctaCountersOrig:
			event.OriginalCounters, err = parseCounters(
				attribute.data,
			)
			if err != nil {
				return Event{}, false, fmt.Errorf(
					"parse original counters: %w",
					err,
				)
			}

		case ctaCountersReply:
			event.ReplyCounters, err = parseCounters(
				attribute.data,
			)
			if err != nil {
				return Event{}, false, fmt.Errorf(
					"parse reply counters: %w",
					err,
				)
			}

		case ctaID:
			if len(attribute.data) < 4 {
				return Event{}, false, fmt.Errorf(
					"CTA_ID has %d bytes",
					len(attribute.data),
				)
			}
			event.ID = binary.BigEndian.Uint32(attribute.data[:4])
		}
	}

	if event.Original.SourceIP == "" ||
		event.Original.DestinationIP == "" {
		return Event{}, false, fmt.Errorf(
			"conntrack event has no complete original tuple",
		)
	}
	if event.Reply.SourceIP == "" ||
		event.Reply.DestinationIP == "" {
		return Event{}, false, fmt.Errorf(
			"conntrack event has no complete reply tuple",
		)
	}

	event.SeenReply = event.Status&statusSeenReply != 0
	event.Assured = event.Status&statusAssured != 0
	event.Confirmed = event.Status&statusConfirmed != 0
	event.SourceNAT = event.Status&statusSourceNAT != 0
	event.DestinationNAT = event.Status&statusDestinationNAT != 0
	event.Dying = event.Status&statusDying != 0
	event.Offload = event.Status&statusOffload != 0
	event.HardwareOffload = event.Status&statusHardwareOffload != 0
	event.Unreplied = !event.SeenReply

	return event, true, nil
}

func parseAttributes(data []byte) ([]netlinkAttribute, error) {
	result := make([]netlinkAttribute, 0, 8)

	for len(data) > 0 {
		if len(data) < netlinkAttributeHeaderLength {
			if allZero(data) {
				break
			}
			return nil, fmt.Errorf(
				"trailing netlink attribute bytes: %d",
				len(data),
			)
		}

		length := int(binary.NativeEndian.Uint16(data[0:2]))
		rawType := binary.NativeEndian.Uint16(data[2:4])

		if length < netlinkAttributeHeaderLength {
			return nil, fmt.Errorf(
				"invalid netlink attribute length %d",
				length,
			)
		}
		if length > len(data) {
			return nil, fmt.Errorf(
				"netlink attribute length %d exceeds %d available bytes",
				length,
				len(data),
			)
		}

		payload := append(
			[]byte(nil),
			data[netlinkAttributeHeaderLength:length]...,
		)
		result = append(result, netlinkAttribute{
			data: payload,
			kind: rawType & netlinkAttributeTypeMask,
		})

		aligned := align4(length)
		if aligned > len(data) {
			if length == len(data) {
				break
			}
			return nil, fmt.Errorf(
				"aligned netlink attribute length %d exceeds %d",
				aligned,
				len(data),
			)
		}
		data = data[aligned:]
	}

	return result, nil
}

func parseCounters(data []byte) (Counters, error) {
	attributes, err := parseAttributes(data)
	if err != nil {
		return Counters{}, err
	}

	result := Counters{}
	for _, attribute := range attributes {
		switch attribute.kind {
		case ctaCountersPackets:
			if len(attribute.data) < 8 {
				return Counters{}, fmt.Errorf(
					"CTA_COUNTERS_PACKETS has %d bytes",
					len(attribute.data),
				)
			}
			result.Packets = binary.BigEndian.Uint64(
				attribute.data[:8],
			)
			result.Present = true

		case ctaCountersBytes:
			if len(attribute.data) < 8 {
				return Counters{}, fmt.Errorf(
					"CTA_COUNTERS_BYTES has %d bytes",
					len(attribute.data),
				)
			}
			result.Bytes = binary.BigEndian.Uint64(
				attribute.data[:8],
			)
			result.Present = true
		}
	}

	return result, nil
}

func parseTCPState(data []byte) (string, error) {
	attributes, err := parseAttributes(data)
	if err != nil {
		return "", fmt.Errorf("parse CTA_PROTOINFO: %w", err)
	}

	for _, attribute := range attributes {
		if attribute.kind != ctaProtoInfoTCP {
			continue
		}

		tcpAttributes, tcpErr := parseAttributes(attribute.data)
		if tcpErr != nil {
			return "", fmt.Errorf(
				"parse CTA_PROTOINFO_TCP: %w",
				tcpErr,
			)
		}
		for _, tcpAttribute := range tcpAttributes {
			if tcpAttribute.kind != ctaProtoInfoTCPState {
				continue
			}
			if len(tcpAttribute.data) < 1 {
				return "", fmt.Errorf(
					"CTA_PROTOINFO_TCP_STATE is empty",
				)
			}
			return tcpStateName(tcpAttribute.data[0]), nil
		}
	}

	return "not_known", nil
}

func parseTuple(family uint8, data []byte) (Tuple, error) {
	attributes, err := parseAttributes(data)
	if err != nil {
		return Tuple{}, err
	}

	result := Tuple{
		Family: familyName(family),
	}

	for _, attribute := range attributes {
		switch attribute.kind {
		case ctaTupleIP:
			if err := parseTupleIP(
				family,
				attribute.data,
				&result,
			); err != nil {
				return Tuple{}, err
			}

		case ctaTupleProto:
			if err := parseTupleProtocol(
				attribute.data,
				&result,
			); err != nil {
				return Tuple{}, err
			}
		}
	}

	return result, nil
}

func parseTupleIP(
	family uint8,
	data []byte,
	result *Tuple,
) error {
	attributes, err := parseAttributes(data)
	if err != nil {
		return err
	}

	for _, attribute := range attributes {
		switch attribute.kind {
		case ctaIPv4Source:
			if len(attribute.data) < net.IPv4len {
				return fmt.Errorf(
					"CTA_IP_V4_SRC has %d bytes",
					len(attribute.data),
				)
			}
			result.SourceIP = net.IP(
				attribute.data[:net.IPv4len],
			).String()

		case ctaIPv4Destination:
			if len(attribute.data) < net.IPv4len {
				return fmt.Errorf(
					"CTA_IP_V4_DST has %d bytes",
					len(attribute.data),
				)
			}
			result.DestinationIP = net.IP(
				attribute.data[:net.IPv4len],
			).String()

		case ctaIPv6Source:
			if len(attribute.data) < net.IPv6len {
				return fmt.Errorf(
					"CTA_IP_V6_SRC has %d bytes",
					len(attribute.data),
				)
			}
			result.SourceIP = net.IP(
				attribute.data[:net.IPv6len],
			).String()

		case ctaIPv6Destination:
			if len(attribute.data) < net.IPv6len {
				return fmt.Errorf(
					"CTA_IP_V6_DST has %d bytes",
					len(attribute.data),
				)
			}
			result.DestinationIP = net.IP(
				attribute.data[:net.IPv6len],
			).String()
		}
	}

	if family == addressFamilyIPv4 {
		if result.SourceIP == "" || result.DestinationIP == "" {
			return fmt.Errorf("IPv4 tuple is incomplete")
		}
	}
	if family == addressFamilyIPv6 {
		if result.SourceIP == "" || result.DestinationIP == "" {
			return fmt.Errorf("IPv6 tuple is incomplete")
		}
	}

	return nil
}

func parseTupleProtocol(data []byte, result *Tuple) error {
	attributes, err := parseAttributes(data)
	if err != nil {
		return err
	}

	for _, attribute := range attributes {
		switch attribute.kind {
		case ctaProtoNumber:
			if len(attribute.data) < 1 {
				return fmt.Errorf("CTA_PROTO_NUM is empty")
			}
			result.ProtocolNumber = attribute.data[0]
			result.Protocol = protocolName(result.ProtocolNumber)

		case ctaProtoSourcePort:
			if len(attribute.data) < 2 {
				return fmt.Errorf(
					"CTA_PROTO_SRC_PORT has %d bytes",
					len(attribute.data),
				)
			}
			result.SourcePort = binary.BigEndian.Uint16(
				attribute.data[:2],
			)

		case ctaProtoDestinationPort:
			if len(attribute.data) < 2 {
				return fmt.Errorf(
					"CTA_PROTO_DST_PORT has %d bytes",
					len(attribute.data),
				)
			}
			result.DestinationPort = binary.BigEndian.Uint16(
				attribute.data[:2],
			)

		case ctaProtoICMPID, ctaProtoICMPv6ID:
			if len(attribute.data) < 2 {
				return fmt.Errorf(
					"CTA_PROTO_ICMP_ID has %d bytes",
					len(attribute.data),
				)
			}
			result.ICMPID = binary.BigEndian.Uint16(
				attribute.data[:2],
			)

		case ctaProtoICMPType, ctaProtoICMPv6Type:
			if len(attribute.data) < 1 {
				return fmt.Errorf(
					"CTA_PROTO_ICMP_TYPE is empty",
				)
			}
			result.ICMPType = attribute.data[0]

		case ctaProtoICMPCode, ctaProtoICMPv6Code:
			if len(attribute.data) < 1 {
				return fmt.Errorf(
					"CTA_PROTO_ICMP_CODE is empty",
				)
			}
			result.ICMPCode = attribute.data[0]
		}
	}

	if result.Protocol == "" {
		result.Protocol = protocolName(result.ProtocolNumber)
	}

	return nil
}

func align4(value int) int {
	return (value + 3) &^ 3
}

func allZero(data []byte) bool {
	for _, value := range data {
		if value != 0 {
			return false
		}
	}
	return true
}

func familyName(family uint8) string {
	switch family {
	case addressFamilyIPv4:
		return "ipv4"
	case addressFamilyIPv6:
		return "ipv6"
	default:
		return fmt.Sprintf("AF%d", family)
	}
}

func protocolName(protocol uint8) string {
	switch protocol {
	case 1:
		return "icmp"
	case 2:
		return "igmp"
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 41:
		return "ipv6"
	case 47:
		return "gre"
	case 50:
		return "esp"
	case 51:
		return "ah"
	case 58:
		return "icmpv6"
	default:
		return fmt.Sprintf("ipproto-%d", protocol)
	}
}

func tcpStateName(state uint8) string {
	names := map[uint8]string{
		0:  "NONE",
		1:  "SYN_SENT",
		2:  "SYN_RECV",
		3:  "ESTABLISHED",
		4:  "FIN_WAIT",
		5:  "CLOSE_WAIT",
		6:  "LAST_ACK",
		7:  "TIME_WAIT",
		8:  "CLOSE",
		9:  "SYN_SENT2",
		10: "MAX",
		11: "IGNORE",
		12: "RETRANS",
		13: "UNACK",
		14: "TIMEOUT_MAX",
	}
	if name, ok := names[state]; ok {
		return name
	}
	return fmt.Sprintf("STATE_%d", state)
}

// statusNames is used only by text rendering and tests; Event preserves the
// original numeric status value.
func statusNames(status uint32) []string {
	type statusName struct {
		bit  uint32
		name string
	}
	values := []statusName{
		{statusExpected, "EXPECTED"},
		{statusSeenReply, "SEEN_REPLY"},
		{statusAssured, "ASSURED"},
		{statusConfirmed, "CONFIRMED"},
		{statusSourceNAT, "SRC_NAT"},
		{statusDestinationNAT, "DST_NAT"},
		{statusDying, "DYING"},
		{statusOffload, "OFFLOAD"},
		{statusHardwareOffload, "HW_OFFLOAD"},
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		if status&value.bit != 0 {
			result = append(result, value.name)
		}
	}
	sort.Strings(result)
	return result
}
