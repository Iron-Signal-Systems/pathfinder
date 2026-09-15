package observation

import "time"

const (
	SchemaVersion = "5"

	TimestampSourceKernelSoftwareNS  = "kernel_software_ns"
	TimestampSourceUserspaceFallback = "userspace_receive_fallback"

	ValueNoRecord = "no_record"
	ValueNotKnown = "not_known"
)

// ARP contains metadata observed from an ARP frame.
type ARP struct {
	Operation string `json:"operation"`
	SenderIP  string `json:"sender_ip"`
	SenderMAC string `json:"sender_mac"`
	TargetIP  string `json:"target_ip"`
	TargetMAC string `json:"target_mac"`
}

// Context describes network configuration associated with an observation.
// These values provide environmental context and are not assumed to have
// originated from the observed frame itself.
type Context struct {
	Direction              string `json:"direction"`
	KernelOffloadSuspected bool   `json:"kernel_offload_suspected"`
	NetworkName            string `json:"network_name"`
	SVI                    string `json:"svi"`
	VLANID                 int    `json:"vlan_id"`
	Zone                   string `json:"zone"`
}

// DNS contains DNS metadata extracted directly from an observed DNS message.
// DNS payload content such as TXT values is not retained.
type DNS struct {
	Additionals        []DNSRecord   `json:"additionals"`
	Answers            []DNSRecord   `json:"answers"`
	Authoritative      bool          `json:"authoritative"`
	Authorities        []DNSRecord   `json:"authorities"`
	EDNS               DNSEDNS       `json:"edns"`
	MessageType        string        `json:"message_type"`
	QueryName          string        `json:"query_name"`
	QueryType          string        `json:"query_type"`
	Questions          []DNSQuestion `json:"questions"`
	RecursionAvailable bool          `json:"recursion_available"`
	RecursionDesired   bool          `json:"recursion_desired"`
	ResponseAddresses  []string      `json:"response_addresses"`
	ResponseCode       string        `json:"response_code"`
	ResponseNames      []string      `json:"response_names"`
	TransactionID      string        `json:"transaction_id"`
	Truncated          bool          `json:"truncated"`
}

// DNSEDNS contains metadata from an observed EDNS OPT pseudo-record. EDNS
// option payload bytes are not retained.
type DNSEDNS struct {
	DNSSECOK      bool   `json:"dnssec_ok"`
	ExtendedRCode uint8  `json:"extended_rcode"`
	OptionCount   uint16 `json:"option_count"`
	Present       bool   `json:"present"`
	UDPSize       uint16 `json:"udp_size"`
	Version       uint8  `json:"version"`
}

// DNSQuestion is one question from an observed DNS message.
type DNSQuestion struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DNSRecord is one observed DNS resource record. Value is deliberately not
// retained for record types whose RDATA may contain arbitrary payload content.
type DNSRecord struct {
	Class string `json:"class"`
	Name  string `json:"name"`
	TTL   uint32 `json:"ttl"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type TLS struct {
	ClientHelloObserved bool   `json:"client_hello_observed"`
	SNI                 string `json:"sni"`
}

// Enrichment contains information learned after the original observation.
type Enrichment struct {
	DestinationNames []NameRecord `json:"destination_names"`
	SourceNames      []NameRecord `json:"source_names"`
}

// Ethernet contains observed Layer 2 metadata.
type Ethernet struct {
	DestinationMAC string `json:"destination_mac"`
	EtherType      string `json:"ethertype"`
	FrameFormat    string `json:"frame_format"`
	IEEE8023Length uint16 `json:"ieee_802_3_length"`
	LLCProtocol    string `json:"llc_protocol"`
	SourceMAC      string `json:"source_mac"`
	VLANID         int    `json:"vlan_id"`
	VLANPriority   uint8  `json:"vlan_priority"`
}

// NameRecord preserves a discovered name and where that name came from.
type NameRecord struct {
	ObservedAt time.Time `json:"observed_at"`
	Source     string    `json:"source"`
	Value      string    `json:"value"`
}

// Network contains observed Layer 3 metadata.
type Network struct {
	DestinationIP string `json:"destination_ip"`
	DSCP          uint8  `json:"dscp"`
	ECN           uint8  `json:"ecn"`
	Family        string `json:"family"`
	Fragmented    bool   `json:"fragmented"`
	HopLimit      uint8  `json:"hop_limit"`
	Length        uint32 `json:"length"`
	Protocol      string `json:"protocol"`
	SourceIP      string `json:"source_ip"`
}

// Observation is one original metadata observation from the sensor.
type Observation struct {
	Context         Context    `json:"context"`
	Enrichment      Enrichment `json:"enrichment"`
	Observed        Observed   `json:"observed"`
	ObservedAt      time.Time  `json:"observed_at"`
	SchemaVersion   string     `json:"schema_version"`
	SensorID        string     `json:"sensor_id"`
	TimestampSource string     `json:"timestamp_source"`
}

// Observed contains information obtained directly from the observed frame.
type Observed struct {
	ARP            ARP       `json:"arp"`
	CapturedLength uint32    `json:"captured_length"`
	DNS            DNS       `json:"dns"`
	Ethernet       Ethernet  `json:"ethernet"`
	Interface      string    `json:"interface"`
	InterfaceIndex int       `json:"interface_index"`
	Network        Network   `json:"network"`
	TLS            TLS       `json:"tls"`
	Transport      Transport `json:"transport"`
}

// Transport contains observed Layer 4 metadata.
type Transport struct {
	DestinationPort uint16   `json:"destination_port"`
	ICMPCode        uint8    `json:"icmp_code"`
	ICMPType        uint8    `json:"icmp_type"`
	SourcePort      uint16   `json:"source_port"`
	TCPFlags        []string `json:"tcp_flags"`
}

// New returns an initialized observation without null-valued collection fields.
func New() Observation {
	return Observation{
		Context: Context{
			Direction:   ValueNotKnown,
			NetworkName: ValueNotKnown,
			SVI:         ValueNotKnown,
			Zone:        ValueNotKnown,
		},
		Enrichment: Enrichment{
			DestinationNames: []NameRecord{},
			SourceNames:      []NameRecord{},
		},
		Observed: Observed{
			ARP: ARP{
				Operation: ValueNoRecord,
				SenderIP:  ValueNoRecord,
				SenderMAC: ValueNoRecord,
				TargetIP:  ValueNoRecord,
				TargetMAC: ValueNoRecord,
			},
			DNS: DNS{
				Additionals:       []DNSRecord{},
				Answers:           []DNSRecord{},
				Authorities:       []DNSRecord{},
				MessageType:       ValueNoRecord,
				QueryName:         ValueNoRecord,
				QueryType:         ValueNoRecord,
				Questions:         []DNSQuestion{},
				ResponseAddresses: []string{},
				ResponseCode:      ValueNoRecord,
				ResponseNames:     []string{},
				TransactionID:     ValueNoRecord,
			},
			Ethernet: Ethernet{
				DestinationMAC: ValueNotKnown,
				EtherType:      ValueNotKnown,
				FrameFormat:    ValueNotKnown,
				LLCProtocol:    ValueNoRecord,
				SourceMAC:      ValueNotKnown,
			},
			Interface: ValueNotKnown,
			Network: Network{
				DestinationIP: ValueNoRecord,
				Family:        ValueNotKnown,
				Protocol:      ValueNotKnown,
				SourceIP:      ValueNoRecord,
			},
			TLS: TLS{
				SNI: ValueNoRecord,
			},
			Transport: Transport{
				TCPFlags: []string{},
			},
		},
		SchemaVersion:   SchemaVersion,
		SensorID:        ValueNotKnown,
		TimestampSource: ValueNotKnown,
	}
}
