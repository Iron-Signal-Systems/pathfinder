package decode

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"

	"github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro/internal/observation"
)

func TestFrameIPv4IGMP(t *testing.T) {
	frame := make([]byte, 14+20+8)
	binary.BigEndian.PutUint16(frame[12:14], etherTypeIPv4)

	offset := 14
	frame[offset] = 0x45
	binary.BigEndian.PutUint16(frame[offset+2:offset+4], 28)
	frame[offset+8] = 1
	frame[offset+9] = 2
	copy(frame[offset+12:offset+16], []byte{192, 168, 1, 3})
	copy(frame[offset+16:offset+20], []byte{224, 0, 0, 22})

	observed, ok := Frame(frame)
	if !ok {
		t.Fatal("Frame() did not decode Ethernet frame")
	}
	if observed.Network.Protocol != "igmp" {
		t.Fatalf("protocol = %q, want igmp", observed.Network.Protocol)
	}
}

func TestFrameIPv4TCP(t *testing.T) {
	frame := make([]byte, 14+20+20)
	copy(frame[0:6], []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff})
	copy(frame[6:12], []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55})
	binary.BigEndian.PutUint16(frame[12:14], etherTypeIPv4)

	offset := 14
	frame[offset] = 0x45
	binary.BigEndian.PutUint16(frame[offset+2:offset+4], 40)
	frame[offset+8] = 64
	frame[offset+9] = 6
	copy(frame[offset+12:offset+16], []byte{192, 168, 1, 25})
	copy(frame[offset+16:offset+20], []byte{93, 184, 216, 34})

	tcp := offset + 20
	binary.BigEndian.PutUint16(frame[tcp:tcp+2], 50000)
	binary.BigEndian.PutUint16(frame[tcp+2:tcp+4], 443)
	frame[tcp+12] = 0x50
	frame[tcp+13] = 0x02

	observed, ok := Frame(frame)
	if !ok {
		t.Fatal("Frame() did not decode Ethernet frame")
	}
	if observed.CapturedLength != uint32(len(frame)) {
		t.Fatalf("captured length = %d, want %d", observed.CapturedLength, len(frame))
	}
	if observed.Ethernet.FrameFormat != "ethernet_ii" {
		t.Fatalf("frame format = %q, want ethernet_ii", observed.Ethernet.FrameFormat)
	}
	if observed.Network.SourceIP != "192.168.1.25" {
		t.Fatalf("source IP = %q", observed.Network.SourceIP)
	}
	if observed.Transport.DestinationPort != 443 {
		t.Fatalf("destination port = %d", observed.Transport.DestinationPort)
	}
	if len(observed.Transport.TCPFlags) != 1 || observed.Transport.TCPFlags[0] != "SYN" {
		t.Fatalf("TCP flags = %#v", observed.Transport.TCPFlags)
	}
}

func TestFrameIEEE8023STP(t *testing.T) {
	frame := make([]byte, 14+39)
	copy(frame[0:6], []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x00})
	copy(frame[6:12], []byte{0x64, 0x9d, 0x99, 0x27, 0xf7, 0xc5})
	binary.BigEndian.PutUint16(frame[12:14], 39)

	frame[14] = 0x42
	frame[15] = 0x42
	frame[16] = 0x03

	observed, ok := Frame(frame)
	if !ok {
		t.Fatal("Frame() did not decode Ethernet frame")
	}
	if observed.Ethernet.FrameFormat != "ieee_802_3" {
		t.Fatalf("frame format = %q, want ieee_802_3", observed.Ethernet.FrameFormat)
	}
	if observed.Ethernet.IEEE8023Length != 39 {
		t.Fatalf("802.3 length = %d, want 39", observed.Ethernet.IEEE8023Length)
	}
	if observed.Ethernet.EtherType != "no_record" {
		t.Fatalf("ethertype = %q, want no_record", observed.Ethernet.EtherType)
	}
	if observed.Ethernet.LLCProtocol != "stp" {
		t.Fatalf("LLC protocol = %q, want stp", observed.Ethernet.LLCProtocol)
	}
}

func TestDecodeDNSEDNSPreservesMetadataWithoutPseudoRecord(t *testing.T) {
	message := make([]byte, 12)
	binary.BigEndian.PutUint16(message[0:2], 0x1111)
	binary.BigEndian.PutUint16(message[2:4], 0x0100)
	binary.BigEndian.PutUint16(message[4:6], 1)
	binary.BigEndian.PutUint16(message[10:12], 1)

	message = append(message, encodeDNSNameForTest("www.example.com")...)
	message = appendUint16ForTest(message, 1)
	message = appendUint16ForTest(message, 1)

	message = append(message, 0)
	message = appendUint16ForTest(message, 41)
	message = appendUint16ForTest(message, 4096)
	message = appendUint32ForTest(message, 0x00008000)
	message = appendUint16ForTest(message, 4)
	message = appendUint16ForTest(message, 10)
	message = appendUint16ForTest(message, 0)

	observed := observation.New().Observed
	decodeDNSMessage(message, &observed)

	if !observed.DNS.EDNS.Present {
		t.Fatal("EDNS present = false, want true")
	}
	if observed.DNS.EDNS.UDPSize != 4096 {
		t.Fatalf("EDNS UDP size = %d, want 4096", observed.DNS.EDNS.UDPSize)
	}
	if !observed.DNS.EDNS.DNSSECOK {
		t.Fatal("EDNS DNSSEC OK = false, want true")
	}
	if observed.DNS.EDNS.OptionCount != 1 {
		t.Fatalf("EDNS option count = %d, want 1", observed.DNS.EDNS.OptionCount)
	}
	if len(observed.DNS.Additionals) != 0 {
		t.Fatalf("additional records = %#v, want OPT omitted from ordinary RR list", observed.DNS.Additionals)
	}
}

func TestDecodeDNSHTTPSIsNamedAndPreservesTarget(t *testing.T) {
	message := make([]byte, 12)
	binary.BigEndian.PutUint16(message[0:2], 0x2222)
	binary.BigEndian.PutUint16(message[2:4], 0x8180)
	binary.BigEndian.PutUint16(message[4:6], 1)
	binary.BigEndian.PutUint16(message[6:8], 1)

	message = append(message, encodeDNSNameForTest("example.com")...)
	message = appendUint16ForTest(message, 65)
	message = appendUint16ForTest(message, 1)

	message = append(message, encodeDNSNameForTest("example.com")...)
	message = appendUint16ForTest(message, 65)
	message = appendUint16ForTest(message, 1)
	message = appendUint32ForTest(message, 60)

	rdata := appendUint16ForTest(nil, 1)
	rdata = append(rdata, encodeDNSNameForTest("svc.example.net")...)
	message = appendUint16ForTest(message, uint16(len(rdata)))
	message = append(message, rdata...)

	observed := observation.New().Observed
	decodeDNSMessage(message, &observed)

	if observed.DNS.QueryType != "HTTPS" {
		t.Fatalf("query type = %q, want HTTPS", observed.DNS.QueryType)
	}
	if len(observed.DNS.Answers) != 1 {
		t.Fatalf("answer count = %d, want 1", len(observed.DNS.Answers))
	}
	if observed.DNS.Answers[0].Type != "HTTPS" {
		t.Fatalf("answer type = %q, want HTTPS", observed.DNS.Answers[0].Type)
	}
	if observed.DNS.Answers[0].Value != "priority=1 target=svc.example.net" {
		t.Fatalf("HTTPS value = %q", observed.DNS.Answers[0].Value)
	}
	if len(observed.DNS.ResponseNames) != 1 ||
		observed.DNS.ResponseNames[0] != "svc.example.net" {
		t.Fatalf("response names = %#v", observed.DNS.ResponseNames)
	}
}

func TestDecodeDNSQueryPreservesQuestion(t *testing.T) {
	message := make([]byte, 12)
	binary.BigEndian.PutUint16(message[0:2], 0x4321)
	binary.BigEndian.PutUint16(message[2:4], 0x0100)
	binary.BigEndian.PutUint16(message[4:6], 1)

	message = append(message, encodeDNSNameForTest("www.example.com")...)
	message = appendUint16ForTest(message, 1)
	message = appendUint16ForTest(message, 1)

	observed := observation.New().Observed
	decodeDNSMessage(message, &observed)

	if observed.DNS.MessageType != "query" {
		t.Fatalf("message type = %q, want query", observed.DNS.MessageType)
	}
	if observed.DNS.TransactionID != "0x4321" {
		t.Fatalf("transaction ID = %q, want 0x4321", observed.DNS.TransactionID)
	}
	if observed.DNS.QueryName != "www.example.com" {
		t.Fatalf("query name = %q", observed.DNS.QueryName)
	}
	if observed.DNS.QueryType != "A" {
		t.Fatalf("query type = %q", observed.DNS.QueryType)
	}
	if observed.DNS.ResponseCode != "no_record" {
		t.Fatalf("response code = %q, want no_record", observed.DNS.ResponseCode)
	}
}

func TestDecodeDNSResponsePreservesPointInTimeAnswers(t *testing.T) {
	message := make([]byte, 12)
	binary.BigEndian.PutUint16(message[0:2], 0x1234)
	binary.BigEndian.PutUint16(message[2:4], 0x8180)
	binary.BigEndian.PutUint16(message[4:6], 1)
	binary.BigEndian.PutUint16(message[6:8], 2)

	message = append(message, encodeDNSNameForTest("www.example.com")...)
	message = appendUint16ForTest(message, 1)
	message = appendUint16ForTest(message, 1)

	message = append(message, encodeDNSNameForTest("www.example.com")...)
	message = appendUint16ForTest(message, 5)
	message = appendUint16ForTest(message, 1)
	message = appendUint32ForTest(message, 300)
	cname := encodeDNSNameForTest("edge.example.net")
	message = appendUint16ForTest(message, uint16(len(cname)))
	message = append(message, cname...)

	message = append(message, encodeDNSNameForTest("edge.example.net")...)
	message = appendUint16ForTest(message, 1)
	message = appendUint16ForTest(message, 1)
	message = appendUint32ForTest(message, 120)
	message = appendUint16ForTest(message, 4)
	message = append(message, 203, 0, 113, 7)

	observed := observation.New().Observed
	decodeDNSMessage(message, &observed)

	if observed.DNS.MessageType != "response" {
		t.Fatalf("message type = %q, want response", observed.DNS.MessageType)
	}
	if observed.DNS.TransactionID != "0x1234" {
		t.Fatalf("transaction ID = %q, want 0x1234", observed.DNS.TransactionID)
	}
	if observed.DNS.ResponseCode != "NOERROR" {
		t.Fatalf("response code = %q, want NOERROR", observed.DNS.ResponseCode)
	}
	if len(observed.DNS.Answers) != 2 {
		t.Fatalf("answer count = %d, want 2", len(observed.DNS.Answers))
	}
	if observed.DNS.Answers[0].Type != "CNAME" ||
		observed.DNS.Answers[0].Value != "edge.example.net" ||
		observed.DNS.Answers[0].TTL != 300 {
		t.Fatalf("CNAME answer = %#v", observed.DNS.Answers[0])
	}
	if observed.DNS.Answers[1].Type != "A" ||
		observed.DNS.Answers[1].Value != "203.0.113.7" ||
		observed.DNS.Answers[1].TTL != 120 {
		t.Fatalf("A answer = %#v", observed.DNS.Answers[1])
	}
	if len(observed.DNS.ResponseNames) != 1 || observed.DNS.ResponseNames[0] != "edge.example.net" {
		t.Fatalf("response names = %#v", observed.DNS.ResponseNames)
	}
	if len(observed.DNS.ResponseAddresses) != 1 || observed.DNS.ResponseAddresses[0] != "203.0.113.7" {
		t.Fatalf("response addresses = %#v", observed.DNS.ResponseAddresses)
	}
}

func TestDecodeDNSTXTDoesNotRetainPayload(t *testing.T) {
	message := make([]byte, 12)
	binary.BigEndian.PutUint16(message[0:2], 0xbeef)
	binary.BigEndian.PutUint16(message[2:4], 0x8180)
	binary.BigEndian.PutUint16(message[4:6], 1)
	binary.BigEndian.PutUint16(message[6:8], 1)

	message = append(message, encodeDNSNameForTest("example.com")...)
	message = appendUint16ForTest(message, 16)
	message = appendUint16ForTest(message, 1)

	message = append(message, encodeDNSNameForTest("example.com")...)
	message = appendUint16ForTest(message, 16)
	message = appendUint16ForTest(message, 1)
	message = appendUint32ForTest(message, 60)
	txtPayload := []byte{6, 's', 'e', 'c', 'r', 'e', 't'}
	message = appendUint16ForTest(message, uint16(len(txtPayload)))
	message = append(message, txtPayload...)

	observed := observation.New().Observed
	decodeDNSMessage(message, &observed)

	if len(observed.DNS.Answers) != 1 {
		t.Fatalf("answer count = %d, want 1", len(observed.DNS.Answers))
	}
	if observed.DNS.Answers[0].Type != "TXT" {
		t.Fatalf("answer type = %q, want TXT", observed.DNS.Answers[0].Type)
	}
	if observed.DNS.Answers[0].Value != "not_recorded" {
		t.Fatalf("TXT value = %q, want not_recorded", observed.DNS.Answers[0].Value)
	}
}

func appendUint16ForTest(buffer []byte, value uint16) []byte {
	var encoded [2]byte
	binary.BigEndian.PutUint16(encoded[:], value)
	return append(buffer, encoded[:]...)
}

func appendUint32ForTest(buffer []byte, value uint32) []byte {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	return append(buffer, encoded[:]...)
}

func encodeDNSNameForTest(name string) []byte {
	var encoded []byte
	for _, label := range strings.Split(name, ".") {
		encoded = append(encoded, byte(len(label)))
		encoded = append(encoded, label...)
	}
	return append(encoded, 0)
}

func TestFrameTLSClientHelloExtractsObservedSNI(t *testing.T) {
	tlsPayload := tlsClientHelloForTest("www.example.com", true)
	frame := ipv4TCPFrameForTest(
		"192.168.1.193",
		"203.0.113.44",
		50000,
		443,
		tlsPayload,
	)

	observed, ok := Frame(frame)
	if !ok {
		t.Fatal("Frame() did not decode Ethernet frame")
	}
	if !observed.TLS.ClientHelloObserved {
		t.Fatal("ClientHelloObserved = false, want true")
	}
	if observed.TLS.SNI != "www.example.com" {
		t.Fatalf(
			"TLS SNI = %q, want www.example.com",
			observed.TLS.SNI,
		)
	}
}

func TestFrameTLSClientHelloWithoutSNIPreservesNoRecord(t *testing.T) {
	tlsPayload := tlsClientHelloForTest("", false)
	frame := ipv4TCPFrameForTest(
		"192.168.1.193",
		"203.0.113.44",
		50000,
		443,
		tlsPayload,
	)

	observed, ok := Frame(frame)
	if !ok {
		t.Fatal("Frame() did not decode Ethernet frame")
	}
	if !observed.TLS.ClientHelloObserved {
		t.Fatal("ClientHelloObserved = false, want true")
	}
	if observed.TLS.SNI != observation.ValueNoRecord {
		t.Fatalf(
			"TLS SNI = %q, want %q",
			observed.TLS.SNI,
			observation.ValueNoRecord,
		)
	}
}

func TestFrameIncompleteTLSClientHelloDoesNotClaimSNI(t *testing.T) {
	tlsPayload := tlsClientHelloForTest("www.example.com", true)
	tlsPayload = tlsPayload[:len(tlsPayload)-5]

	frame := ipv4TCPFrameForTest(
		"192.168.1.193",
		"203.0.113.44",
		50000,
		443,
		tlsPayload,
	)

	observed, ok := Frame(frame)
	if !ok {
		t.Fatal("Frame() did not decode Ethernet frame")
	}
	if observed.TLS.ClientHelloObserved {
		t.Fatal("ClientHelloObserved = true, want false")
	}
	if observed.TLS.SNI != observation.ValueNoRecord {
		t.Fatalf(
			"TLS SNI = %q, want %q",
			observed.TLS.SNI,
			observation.ValueNoRecord,
		)
	}
}

func ipv4TCPFrameForTest(
	sourceIP string,
	destinationIP string,
	sourcePort uint16,
	destinationPort uint16,
	payload []byte,
) []byte {
	source := net.ParseIP(sourceIP).To4()
	destination := net.ParseIP(destinationIP).To4()

	frame := make([]byte, 14+20+20+len(payload))
	copy(frame[0:6], []byte{0x68, 0xd7, 0x9a, 0x35, 0xbb, 0x7a})
	copy(frame[6:12], []byte{0x44, 0xa9, 0x2c, 0x50, 0x80, 0x76})
	binary.BigEndian.PutUint16(frame[12:14], etherTypeIPv4)

	ip := 14
	frame[ip] = 0x45
	binary.BigEndian.PutUint16(
		frame[ip+2:ip+4],
		uint16(20+20+len(payload)),
	)
	frame[ip+8] = 64
	frame[ip+9] = 6
	copy(frame[ip+12:ip+16], source)
	copy(frame[ip+16:ip+20], destination)

	tcp := ip + 20
	binary.BigEndian.PutUint16(frame[tcp:tcp+2], sourcePort)
	binary.BigEndian.PutUint16(frame[tcp+2:tcp+4], destinationPort)
	frame[tcp+12] = 0x50
	frame[tcp+13] = 0x18

	copy(frame[tcp+20:], payload)

	return frame
}

func tlsClientHelloForTest(
	serverName string,
	includeSNI bool,
) []byte {
	body := make([]byte, 0)

	// legacy_version TLS 1.2 encoding used by modern ClientHello.
	body = append(body, 0x03, 0x03)

	// random
	body = append(body, make([]byte, 32)...)

	// legacy_session_id
	body = append(body, 0)

	// cipher_suites: TLS_AES_128_GCM_SHA256 encoded only as parser input.
	body = append(body, 0, 2, 0x13, 0x01)

	// legacy_compression_methods: null compression
	body = append(body, 1, 0)

	extensions := make([]byte, 0)
	if includeSNI {
		name := []byte(serverName)

		serverNameList := make([]byte, 0)
		serverNameList = append(
			serverNameList,
			byte((3+len(name))>>8),
			byte(3+len(name)),
		)
		serverNameList = append(serverNameList, 0)
		serverNameList = append(
			serverNameList,
			byte(len(name)>>8),
			byte(len(name)),
		)
		serverNameList = append(serverNameList, name...)

		extensions = append(
			extensions,
			0x00,
			0x00,
			byte(len(serverNameList)>>8),
			byte(len(serverNameList)),
		)
		extensions = append(extensions, serverNameList...)
	}

	body = append(
		body,
		byte(len(extensions)>>8),
		byte(len(extensions)),
	)
	body = append(body, extensions...)

	handshake := []byte{
		0x01,
		byte(len(body) >> 16),
		byte(len(body) >> 8),
		byte(len(body)),
	}
	handshake = append(handshake, body...)

	record := []byte{
		0x16,
		0x03,
		0x01,
		byte(len(handshake) >> 8),
		byte(len(handshake)),
	}
	record = append(record, handshake...)

	return record
}
