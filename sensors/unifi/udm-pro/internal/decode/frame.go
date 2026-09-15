package decode

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

const (
	ieee8023MaxLength = 1500

	etherTypeIPv4 = 0x0800
	etherTypeARP  = 0x0806
	etherTypeVLAN = 0x8100
	etherTypeQinQ = 0x88a8
	etherTypeIPv6 = 0x86dd
)

// Frame decodes metadata from one Ethernet frame. Packet payload is never
// retained in the returned observation.
func Frame(frame []byte) (observation.Observed, bool) {
	observed := observation.New().Observed
	if len(frame) < 14 {
		return observed, false
	}

	observed.CapturedLength = uint32(len(frame))
	observed.Ethernet.DestinationMAC = net.HardwareAddr(frame[0:6]).String()
	observed.Ethernet.SourceMAC = net.HardwareAddr(frame[6:12]).String()

	offset := 14
	typeOrLength := binary.BigEndian.Uint16(frame[12:14])

	if typeOrLength == etherTypeVLAN || typeOrLength == etherTypeQinQ {
		if len(frame) < offset+4 {
			return observed, true
		}

		tci := binary.BigEndian.Uint16(frame[offset : offset+2])
		observed.Ethernet.VLANPriority = uint8((tci >> 13) & 0x7)
		observed.Ethernet.VLANID = int(tci & 0x0fff)
		typeOrLength = binary.BigEndian.Uint16(frame[offset+2 : offset+4])
		offset += 4
	}

	if typeOrLength <= ieee8023MaxLength {
		observed.Ethernet.FrameFormat = "ieee_802_3"
		observed.Ethernet.EtherType = observation.ValueNoRecord
		observed.Ethernet.IEEE8023Length = typeOrLength
		decodeLLC(frame, offset, &observed)
		return observed, true
	}

	observed.Ethernet.FrameFormat = "ethernet_ii"
	observed.Ethernet.EtherType = etherTypeName(typeOrLength)

	switch typeOrLength {
	case etherTypeIPv4:
		decodeIPv4(frame, offset, &observed)
	case etherTypeIPv6:
		decodeIPv6(frame, offset, &observed)
	case etherTypeARP:
		decodeARP(frame, offset, &observed)
	}

	return observed, true
}

func decodeLLC(frame []byte, offset int, observed *observation.Observed) {
	if len(frame) < offset+3 {
		return
	}

	dsap := frame[offset]
	ssap := frame[offset+1]
	control := frame[offset+2]

	switch {
	case dsap == 0x42 && ssap == 0x42 && control == 0x03:
		observed.Ethernet.LLCProtocol = "stp"
	default:
		observed.Ethernet.LLCProtocol = fmt.Sprintf(
			"llc_dsap_0x%02x_ssap_0x%02x_control_0x%02x",
			dsap,
			ssap,
			control,
		)
	}
}

func decodeARP(frame []byte, offset int, observed *observation.Observed) {
	if len(frame) < offset+28 {
		return
	}

	hardwareType := binary.BigEndian.Uint16(frame[offset : offset+2])
	protocolType := binary.BigEndian.Uint16(frame[offset+2 : offset+4])
	hardwareLength := int(frame[offset+4])
	protocolLength := int(frame[offset+5])
	operation := binary.BigEndian.Uint16(frame[offset+6 : offset+8])

	if hardwareType != 1 || protocolType != etherTypeIPv4 || hardwareLength != 6 || protocolLength != 4 {
		return
	}

	observed.Network.Family = "arp"
	observed.Network.Protocol = "arp"

	switch operation {
	case 1:
		observed.ARP.Operation = "request"
	case 2:
		observed.ARP.Operation = "reply"
	default:
		observed.ARP.Operation = fmt.Sprintf("operation_%d", operation)
	}

	observed.ARP.SenderMAC = net.HardwareAddr(frame[offset+8 : offset+14]).String()
	observed.ARP.SenderIP = net.IP(frame[offset+14 : offset+18]).String()
	observed.ARP.TargetMAC = net.HardwareAddr(frame[offset+18 : offset+24]).String()
	observed.ARP.TargetIP = net.IP(frame[offset+24 : offset+28]).String()
}

func decodeIPv4(frame []byte, offset int, observed *observation.Observed) {
	if len(frame) < offset+20 {
		return
	}

	versionIHL := frame[offset]
	if versionIHL>>4 != 4 {
		return
	}

	headerLength := int(versionIHL&0x0f) * 4
	if headerLength < 20 || len(frame) < offset+headerLength {
		return
	}

	dscpECN := frame[offset+1]
	fragment := binary.BigEndian.Uint16(frame[offset+6 : offset+8])
	fragmentOffset := fragment & 0x1fff
	moreFragments := fragment&0x2000 != 0
	protocolNumber := frame[offset+9]

	observed.Network.Family = "ipv4"
	observed.Network.SourceIP = net.IP(frame[offset+12 : offset+16]).String()
	observed.Network.DestinationIP = net.IP(frame[offset+16 : offset+20]).String()
	observed.Network.DSCP = dscpECN >> 2
	observed.Network.ECN = dscpECN & 0x03
	observed.Network.Fragmented = moreFragments || fragmentOffset != 0
	observed.Network.HopLimit = frame[offset+8]
	observed.Network.Length = uint32(binary.BigEndian.Uint16(frame[offset+2 : offset+4]))
	observed.Network.Protocol = protocolName(protocolNumber, false)

	if fragmentOffset != 0 {
		return
	}

	decodeTransport(frame, offset+headerLength, protocolNumber, observed)
}

func decodeIPv6(frame []byte, offset int, observed *observation.Observed) {
	if len(frame) < offset+40 {
		return
	}

	if frame[offset]>>4 != 6 {
		return
	}

	trafficClass := ((frame[offset] & 0x0f) << 4) | (frame[offset+1] >> 4)
	nextHeader := frame[offset+6]
	payloadLength := binary.BigEndian.Uint16(frame[offset+4 : offset+6])

	observed.Network.Family = "ipv6"
	observed.Network.SourceIP = net.IP(frame[offset+8 : offset+24]).String()
	observed.Network.DestinationIP = net.IP(frame[offset+24 : offset+40]).String()
	observed.Network.DSCP = trafficClass >> 2
	observed.Network.ECN = trafficClass & 0x03
	observed.Network.HopLimit = frame[offset+7]
	observed.Network.Length = uint32(payloadLength) + 40
	observed.Network.Protocol = protocolName(nextHeader, true)

	decodeTransport(frame, offset+40, nextHeader, observed)
}

func decodeTransport(frame []byte, offset int, protocol uint8, observed *observation.Observed) {
	switch protocol {
	case 6:
		decodeTCP(frame, offset, observed)
	case 17:
		decodeUDP(frame, offset, observed)
	case 1, 58:
		decodeICMP(frame, offset, observed)
	}
}

func decodeTCP(frame []byte, offset int, observed *observation.Observed) {
	if len(frame) < offset+20 {
		return
	}

	observed.Transport.SourcePort = binary.BigEndian.Uint16(frame[offset : offset+2])
	observed.Transport.DestinationPort = binary.BigEndian.Uint16(frame[offset+2 : offset+4])
	observed.Transport.TCPFlags = tcpFlags(frame[offset+13])

	headerLength := int(frame[offset+12]>>4) * 4
	if headerLength < 20 || len(frame) < offset+headerLength {
		return
	}

	payloadOffset := offset + headerLength

	if observed.Transport.SourcePort == 53 || observed.Transport.DestinationPort == 53 {
		if len(frame) >= payloadOffset+2 {
			messageLength := int(binary.BigEndian.Uint16(frame[payloadOffset : payloadOffset+2]))
			dnsOffset := payloadOffset + 2
			if messageLength > 0 && len(frame) >= dnsOffset+messageLength {
				decodeDNSMessage(frame[dnsOffset:dnsOffset+messageLength], observed)
			}
		}
	}

	if len(frame) > payloadOffset {
		decodeTLSClientHello(frame[payloadOffset:], observed)
	}
}

func decodeTLSClientHello(
	payload []byte,
	observed *observation.Observed,
) {
	const (
		tlsContentTypeHandshake = 22
		tlsHandshakeClientHello = 1
	)

	// A ClientHello can span multiple TLS records while still being contained
	// in one TCP segment. Reassemble only those record payloads transiently.
	// Nothing from the TLS payload is retained after metadata extraction.
	handshake := make([]byte, 0, len(payload))

	for len(payload) >= 5 {
		contentType := payload[0]
		versionMajor := payload[1]
		recordLength := int(binary.BigEndian.Uint16(payload[3:5]))

		if contentType != tlsContentTypeHandshake || versionMajor != 3 {
			return
		}
		if recordLength <= 0 || len(payload) < 5+recordLength {
			// No TCP stream reassembly: an incomplete TLS record is simply not
			// enough authority to claim a ClientHello or SNI.
			return
		}

		handshake = append(
			handshake,
			payload[5:5+recordLength]...,
		)

		if len(handshake) >= 4 {
			if handshake[0] != tlsHandshakeClientHello {
				return
			}

			handshakeLength := int(handshake[1])<<16 |
				int(handshake[2])<<8 |
				int(handshake[3])

			if handshakeLength <= 0 {
				return
			}
			if len(handshake) >= 4+handshakeLength {
				observed.TLS.ClientHelloObserved = true
				observed.TLS.SNI = tlsClientHelloSNI(
					handshake[4 : 4+handshakeLength],
				)
				return
			}
		}

		payload = payload[5+recordLength:]
	}
}

func tlsClientHelloSNI(body []byte) string {
	// legacy_version(2) + random(32)
	if len(body) < 34 {
		return observation.ValueNoRecord
	}

	offset := 34

	// legacy_session_id
	if offset >= len(body) {
		return observation.ValueNoRecord
	}
	sessionIDLength := int(body[offset])
	offset++
	if offset+sessionIDLength > len(body) {
		return observation.ValueNoRecord
	}
	offset += sessionIDLength

	// cipher_suites
	if offset+2 > len(body) {
		return observation.ValueNoRecord
	}
	cipherSuitesLength := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if cipherSuitesLength == 0 ||
		cipherSuitesLength%2 != 0 ||
		offset+cipherSuitesLength > len(body) {
		return observation.ValueNoRecord
	}
	offset += cipherSuitesLength

	// legacy_compression_methods
	if offset >= len(body) {
		return observation.ValueNoRecord
	}
	compressionLength := int(body[offset])
	offset++
	if compressionLength == 0 ||
		offset+compressionLength > len(body) {
		return observation.ValueNoRecord
	}
	offset += compressionLength

	// Extensions are optional in older ClientHello formats.
	if offset == len(body) {
		return observation.ValueNoRecord
	}
	if offset+2 > len(body) {
		return observation.ValueNoRecord
	}

	extensionsLength := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if extensionsLength == 0 || offset+extensionsLength > len(body) {
		return observation.ValueNoRecord
	}

	end := offset + extensionsLength
	for offset+4 <= end {
		extensionType := binary.BigEndian.Uint16(body[offset : offset+2])
		extensionLength := int(
			binary.BigEndian.Uint16(body[offset+2 : offset+4]),
		)
		offset += 4

		if offset+extensionLength > end {
			return observation.ValueNoRecord
		}

		if extensionType == 0 {
			return tlsServerNameFromExtension(
				body[offset : offset+extensionLength],
			)
		}

		offset += extensionLength
	}

	return observation.ValueNoRecord
}

func tlsServerNameFromExtension(extension []byte) string {
	if len(extension) < 2 {
		return observation.ValueNoRecord
	}

	listLength := int(binary.BigEndian.Uint16(extension[0:2]))
	if listLength == 0 || 2+listLength > len(extension) {
		return observation.ValueNoRecord
	}

	offset := 2
	end := 2 + listLength

	for offset+3 <= end {
		nameType := extension[offset]
		nameLength := int(
			binary.BigEndian.Uint16(extension[offset+1 : offset+3]),
		)
		offset += 3

		if nameLength == 0 || offset+nameLength > end {
			return observation.ValueNoRecord
		}

		if nameType == 0 {
			nameBytes := extension[offset : offset+nameLength]
			if !validObservedSNI(nameBytes) {
				return observation.ValueNoRecord
			}
			return string(nameBytes)
		}

		offset += nameLength
	}

	return observation.ValueNoRecord
}

func validObservedSNI(value []byte) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}

	for _, current := range value {
		// SNI host_name is normally an ASCII A-label. Reject whitespace,
		// control bytes, DEL, and non-ASCII bytes so metadata output cannot
		// contain control sequences or arbitrary payload content.
		if current < 0x21 || current > 0x7e {
			return false
		}
	}

	return true
}

func decodeUDP(frame []byte, offset int, observed *observation.Observed) {
	if len(frame) < offset+8 {
		return
	}

	observed.Transport.SourcePort = binary.BigEndian.Uint16(frame[offset : offset+2])
	observed.Transport.DestinationPort = binary.BigEndian.Uint16(frame[offset+2 : offset+4])

	if observed.Transport.SourcePort == 53 || observed.Transport.DestinationPort == 53 {
		udpLength := int(binary.BigEndian.Uint16(frame[offset+4 : offset+6]))
		if udpLength >= 8 && len(frame) >= offset+udpLength {
			decodeDNSMessage(frame[offset+8:offset+udpLength], observed)
		}
	}
}

func decodeICMP(frame []byte, offset int, observed *observation.Observed) {
	if len(frame) < offset+2 {
		return
	}

	observed.Transport.ICMPType = frame[offset]
	observed.Transport.ICMPCode = frame[offset+1]
}

func decodeDNSMessage(message []byte, observed *observation.Observed) {
	if len(message) < 12 {
		return
	}

	flags := binary.BigEndian.Uint16(message[2:4])
	questionCount := int(binary.BigEndian.Uint16(message[4:6]))
	answerCount := int(binary.BigEndian.Uint16(message[6:8]))
	authorityCount := int(binary.BigEndian.Uint16(message[8:10]))
	additionalCount := int(binary.BigEndian.Uint16(message[10:12]))

	observed.DNS.TransactionID = fmt.Sprintf("0x%04x", binary.BigEndian.Uint16(message[0:2]))
	observed.DNS.Authoritative = flags&0x0400 != 0
	observed.DNS.Truncated = flags&0x0200 != 0
	observed.DNS.RecursionDesired = flags&0x0100 != 0
	observed.DNS.RecursionAvailable = flags&0x0080 != 0

	if flags&0x8000 != 0 {
		observed.DNS.MessageType = "response"
		observed.DNS.ResponseCode = dnsResponseCodeName(uint8(flags & 0x000f))
	} else {
		observed.DNS.MessageType = "query"
	}

	offset := 12
	for i := 0; i < questionCount; i++ {
		name, next, ok := decodeDNSName(message, offset)
		if !ok || next+4 > len(message) {
			return
		}

		question := observation.DNSQuestion{
			Name: name,
			Type: dnsTypeName(binary.BigEndian.Uint16(message[next : next+2])),
		}
		observed.DNS.Questions = append(observed.DNS.Questions, question)

		if i == 0 {
			observed.DNS.QueryName = question.Name
			observed.DNS.QueryType = question.Type
		}

		offset = next + 4
	}

	var ok bool
	offset, ok = decodeDNSRecords(message, offset, answerCount, &observed.DNS.Answers, observed, true)
	if !ok {
		return
	}

	offset, ok = decodeDNSRecords(message, offset, authorityCount, &observed.DNS.Authorities, observed, false)
	if !ok {
		return
	}

	_, _ = decodeDNSRecords(message, offset, additionalCount, &observed.DNS.Additionals, observed, false)
}

func decodeDNSRecords(
	message []byte,
	offset int,
	count int,
	destination *[]observation.DNSRecord,
	observed *observation.Observed,
	collectResponse bool,
) (int, bool) {
	for i := 0; i < count; i++ {
		record, includeRecord, next, address, relatedName, ok := decodeDNSRecord(
			message,
			offset,
			&observed.DNS.EDNS,
		)
		if !ok {
			return offset, false
		}

		if includeRecord {
			*destination = append(*destination, record)
		}

		if collectResponse {
			if address != "" {
				observed.DNS.ResponseAddresses = appendUnique(observed.DNS.ResponseAddresses, address)
			}
			if relatedName != "" {
				observed.DNS.ResponseNames = appendUnique(observed.DNS.ResponseNames, relatedName)
			}
		}

		offset = next
	}

	return offset, true
}

func decodeDNSRecord(
	message []byte,
	offset int,
	edns *observation.DNSEDNS,
) (observation.DNSRecord, bool, int, string, string, bool) {
	record := observation.DNSRecord{
		Class: observation.ValueNotKnown,
		Name:  observation.ValueNotKnown,
		Type:  observation.ValueNotKnown,
		Value: observation.ValueNoRecord,
	}

	name, next, ok := decodeDNSName(message, offset)
	if !ok || next+10 > len(message) {
		return record, false, offset, "", "", false
	}

	typeValue := binary.BigEndian.Uint16(message[next : next+2])
	classValue := binary.BigEndian.Uint16(message[next+2 : next+4])
	ttl := binary.BigEndian.Uint32(message[next+4 : next+8])
	rdataLength := int(binary.BigEndian.Uint16(message[next+8 : next+10]))
	rdataOffset := next + 10
	recordEnd := rdataOffset + rdataLength
	if recordEnd > len(message) {
		return record, false, offset, "", "", false
	}

	if typeValue == 41 {
		edns.Present = true
		edns.UDPSize = classValue
		edns.ExtendedRCode = uint8(ttl >> 24)
		edns.Version = uint8((ttl >> 16) & 0xff)
		edns.DNSSECOK = ttl&0x00008000 != 0
		edns.OptionCount = countEDNSOptions(message[rdataOffset:recordEnd])

		return record, false, recordEnd, "", "", true
	}

	record.Name = name
	record.Type = dnsTypeName(typeValue)
	record.Class = dnsClassName(classValue)
	record.TTL = ttl

	address := ""
	relatedName := ""

	switch typeValue {
	case 1:
		if rdataLength == 4 {
			address = net.IP(message[rdataOffset:recordEnd]).String()
			record.Value = address
		}

	case 2, 5, 12:
		value, _, ok := decodeDNSName(message, rdataOffset)
		if ok {
			record.Value = value
			relatedName = value
		}

	case 15:
		if rdataLength >= 3 {
			preference := binary.BigEndian.Uint16(message[rdataOffset : rdataOffset+2])
			exchange, _, ok := decodeDNSName(message, rdataOffset+2)
			if ok {
				record.Value = fmt.Sprintf("preference=%d exchange=%s", preference, exchange)
				relatedName = exchange
			}
		}

	case 28:
		if rdataLength == 16 {
			address = net.IP(message[rdataOffset:recordEnd]).String()
			record.Value = address
		}

	case 33:
		if rdataLength >= 7 {
			priority := binary.BigEndian.Uint16(message[rdataOffset : rdataOffset+2])
			weight := binary.BigEndian.Uint16(message[rdataOffset+2 : rdataOffset+4])
			port := binary.BigEndian.Uint16(message[rdataOffset+4 : rdataOffset+6])
			target, _, ok := decodeDNSName(message, rdataOffset+6)
			if ok {
				record.Value = fmt.Sprintf(
					"priority=%d weight=%d port=%d target=%s",
					priority,
					weight,
					port,
					target,
				)
				relatedName = target
			}
		}

	case 64, 65:
		if rdataLength >= 3 {
			priority := binary.BigEndian.Uint16(message[rdataOffset : rdataOffset+2])
			target, _, ok := decodeDNSName(message, rdataOffset+2)
			if ok {
				displayTarget := target
				if displayTarget == "" {
					displayTarget = "."
				}
				record.Value = fmt.Sprintf(
					"priority=%d target=%s",
					priority,
					displayTarget,
				)
				if target != "" {
					relatedName = target
				}
			}
		}

	case 16:
		// TXT content is arbitrary application payload. Preserve that a TXT
		// record existed and its TTL, but deliberately do not retain RDATA.
		record.Value = "not_recorded"

	default:
		record.Value = "not_recorded"
	}

	return record, true, recordEnd, address, relatedName, true
}

func countEDNSOptions(data []byte) uint16 {
	var count uint16

	for len(data) >= 4 {
		optionLength := int(binary.BigEndian.Uint16(data[2:4]))
		if optionLength > len(data)-4 {
			break
		}

		count++
		data = data[4+optionLength:]
	}

	return count
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func dnsClassName(value uint16) string {
	// The high bit is used as the cache-flush bit by mDNS. Port 53 DNS does
	// not normally use it, but masking it keeps class interpretation correct.
	base := value & 0x7fff

	switch base {
	case 1:
		return "IN"
	case 3:
		return "CH"
	case 4:
		return "HS"
	case 255:
		return "ANY"
	default:
		return fmt.Sprintf("CLASS%d", base)
	}
}

func dnsResponseCodeName(value uint8) string {
	switch value {
	case 0:
		return "NOERROR"
	case 1:
		return "FORMERR"
	case 2:
		return "SERVFAIL"
	case 3:
		return "NXDOMAIN"
	case 4:
		return "NOTIMP"
	case 5:
		return "REFUSED"
	default:
		return fmt.Sprintf("RCODE%d", value)
	}
}

func decodeDNSName(message []byte, offset int) (string, int, bool) {
	labels := make([]string, 0, 4)
	cursor := offset
	jumps := 0
	consumed := 0
	jumped := false

	for {
		if cursor >= len(message) || jumps > 16 {
			return "", 0, false
		}

		length := int(message[cursor])
		if length == 0 {
			if !jumped {
				consumed++
			}
			return strings.Join(labels, "."), offset + consumed, true
		}

		if length&0xc0 == 0xc0 {
			if cursor+1 >= len(message) {
				return "", 0, false
			}
			pointer := int(binary.BigEndian.Uint16(message[cursor:cursor+2]) & 0x3fff)
			if !jumped {
				consumed += 2
			}
			cursor = pointer
			jumped = true
			jumps++
			continue
		}

		if length > 63 || cursor+1+length > len(message) {
			return "", 0, false
		}

		labels = append(labels, string(message[cursor+1:cursor+1+length]))
		if !jumped {
			consumed += 1 + length
		}
		cursor += 1 + length
	}
}

func dnsTypeName(value uint16) string {
	switch value {
	case 1:
		return "A"
	case 2:
		return "NS"
	case 5:
		return "CNAME"
	case 6:
		return "SOA"
	case 12:
		return "PTR"
	case 15:
		return "MX"
	case 16:
		return "TXT"
	case 28:
		return "AAAA"
	case 33:
		return "SRV"
	case 41:
		return "OPT"
	case 43:
		return "DS"
	case 46:
		return "RRSIG"
	case 47:
		return "NSEC"
	case 48:
		return "DNSKEY"
	case 64:
		return "SVCB"
	case 65:
		return "HTTPS"
	case 257:
		return "CAA"
	default:
		return fmt.Sprintf("TYPE%d", value)
	}
}

func etherTypeName(value uint16) string {
	switch value {
	case etherTypeIPv4:
		return "ipv4"
	case etherTypeARP:
		return "arp"
	case etherTypeIPv6:
		return "ipv6"
	default:
		return fmt.Sprintf("0x%04x", value)
	}
}

func protocolName(value uint8, ipv6 bool) string {
	switch value {
	case 1:
		return "icmp"
	case 2:
		return "igmp"
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 58:
		return "icmpv6"
	default:
		if ipv6 {
			return fmt.Sprintf("next_header_%d", value)
		}
		return fmt.Sprintf("ip_protocol_%d", value)
	}
}

func tcpFlags(flags uint8) []string {
	result := make([]string, 0, 8)

	if flags&0x01 != 0 {
		result = append(result, "FIN")
	}
	if flags&0x02 != 0 {
		result = append(result, "SYN")
	}
	if flags&0x04 != 0 {
		result = append(result, "RST")
	}
	if flags&0x08 != 0 {
		result = append(result, "PSH")
	}
	if flags&0x10 != 0 {
		result = append(result, "ACK")
	}
	if flags&0x20 != 0 {
		result = append(result, "URG")
	}
	if flags&0x40 != 0 {
		result = append(result, "ECE")
	}
	if flags&0x80 != 0 {
		result = append(result, "CWR")
	}

	return result
}
