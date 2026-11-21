// Package sensorsor projects validated immutable sensor JSONL evidence into the
// relational SoR schema. It does not authenticate a sensor or acknowledge custody.
package sensorsor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

type Columns struct {
	Table string
	Names []string
	Rows  [][]string
}
type Evidence struct {
	Tables      map[string]*Columns
	First, Last time.Time
	Count       int
}
type Context struct {
	SensorUUID, BatchUUID string
	StartOrdinal          int
}

var Schema = map[string][]string{
	"sensor_observation":       {"observed_at", "observation_id", "sensor_id", "batch_id", "record_ordinal", "segment_record_ordinal", "observed_at_ns", "timestamp_source", "interface_name", "interface_index", "direction", "vlan_id", "zone", "network_name", "svi", "network_family", "protocol", "source_ip", "destination_ip", "source_port", "destination_port", "source_mac", "destination_mac", "ethertype", "captured_length", "dns_message_type", "dns_query_name", "dns_query_type", "tls_client_hello_observed", "tls_sni", "arp_operation", "kernel_offload_suspected", "raw_record"},
	"sensor_dns_question":      {"observation_at", "observation_id", "question_ordinal", "question_name", "question_type"},
	"sensor_dns_record":        {"observation_at", "observation_id", "section", "record_ordinal", "name", "record_type", "record_class", "ttl_seconds", "value", "raw_record"},
	"sensor_conntrack_event":   {"observed_at", "event_id", "sensor_id", "batch_id", "record_ordinal", "segment_record_ordinal", "observed_at_ns", "timestamp_source", "conntrack_id", "event_type", "original_family", "original_protocol", "original_source_ip", "original_destination_ip", "original_source_port", "original_destination_port", "reply_family", "reply_protocol", "reply_source_ip", "reply_destination_ip", "reply_source_port", "reply_destination_port", "source_nat", "destination_nat", "tcp_state", "assured", "confirmed", "seen_reply", "dying", "offload", "hardware_offload", "timeout_seconds", "conntrack_status", "mark", "original_counters_present", "original_packets", "original_bytes", "reply_counters_present", "reply_packets", "reply_bytes", "raw_record"},
	"sensor_identity_snapshot": {"snapshot_id", "sensor_id", "batch_id", "record_ordinal", "segment_record_ordinal", "observed_at", "observed_at_ns", "expected_interval_seconds", "identity_source", "identity_source_path", "raw_record"},
	"sensor_identity_endpoint": {"snapshot_id", "endpoint_ordinal", "ip", "mac", "hostname", "fqdn", "lease_expires_at", "raw_endpoint"},
	"sensor_health":            {"health_id", "sensor_id", "batch_id", "record_ordinal", "segment_record_ordinal", "observed_at", "observed_at_ns", "capture_started_at", "expected_interval_seconds", "final", "observations_written", "observations_discarded", "capture_write_failures", "conntrack_enabled", "conntrack_events_received", "conntrack_receive_overruns", "raw_record"},
}

func ID(parts ...string) string {
	d := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	d[6] = (d[6] & 0x0f) | 0x40 // deterministic UUIDv4 form, not random UUID
	d[8] = (d[8] & 0x3f) | 0x80
	s := hex.EncodeToString(d[:16])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func New() *Evidence { return &Evidence{Tables: map[string]*Columns{}} }
func (e *Evidence) add(table string, row ...string) error {
	if len(row) != len(Schema[table]) {
		return fmt.Errorf("%s projection fields=%d want=%d", table, len(row), len(Schema[table]))
	}
	dest := e.Tables[table]
	if dest == nil {
		dest = &Columns{Table: table, Names: Schema[table]}
		e.Tables[table] = dest
	}
	dest.Rows = append(dest.Rows, row)
	return nil
}
func null(s string) string {
	if s == "" || s == "no_record" || s == "not_known" {
		return "\\N"
	}
	return s
}
func num(v interface{}) string {
	if v == nil {
		return "\\N"
	}
	return fmt.Sprint(v)
}
func b(v bool) string {
	if v {
		return "t"
	}
	return "f"
}
func int64s(n int64) string { return strconv.FormatInt(n, 10) }
func optionalPort(proto string, p int) string {
	if p == 0 && proto != "tcp" && proto != "udp" {
		return "\\N"
	}
	return strconv.Itoa(p)
}
func validIP(s string) (string, error) {
	if null(s) == "\\N" {
		return "\\N", nil
	}
	ip, err := netip.ParseAddr(s)
	if err != nil {
		return "", fmt.Errorf("invalid IP %q: %w", s, err)
	}
	return ip.String(), nil
}
func validMAC(s string) (string, error) {
	if null(s) == "\\N" {
		return "\\N", nil
	}
	mac, err := net.ParseMAC(s)
	if err != nil {
		return "", fmt.Errorf("invalid MAC %q: %w", s, err)
	}
	if len(mac) != 6 {
		return "", fmt.Errorf("macaddr requires 6 bytes")
	}
	return mac.String(), nil
}
func optionalTime(s string) (string, error) {
	if null(s) == "\\N" {
		return "\\N", nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return "", err
	}
	return t.UTC().Format(time.RFC3339Nano), nil
}
func timestamp(s string) (time.Time, string, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil || t.IsZero() {
		return time.Time{}, "", fmt.Errorf("invalid observed_at %q", s)
	}
	return t, t.UTC().Format(time.RFC3339Nano), nil
}
func (e *Evidence) Append(stream string, ctx Context, segmentOrdinal int64, raw []byte) error {
	ordinal := ctx.StartOrdinal + e.Count
	if ordinal < 1 || segmentOrdinal < 1 {
		return fmt.Errorf("invalid source ordinals")
	}
	var base struct {
		ObservedAt      string `json:"observed_at"`
		TimestampSource string `json:"timestamp_source"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		return err
	}
	observed, at, err := timestamp(base.ObservedAt)
	if err != nil {
		return err
	}
	if e.Count == 0 || observed.Before(e.First) {
		e.First = observed
	}
	if e.Count == 0 || observed.After(e.Last) {
		e.Last = observed
	}
	id := ID("sensor-source-record-v1", ctx.BatchUUID, strconv.Itoa(ordinal))
	order := strconv.Itoa(ordinal)
	segOrd := int64s(segmentOrdinal)
	common := []string{at, id, ctx.SensorUUID, ctx.BatchUUID, order, segOrd, int64s(observed.UnixNano()), null(base.TimestampSource)}
	switch stream {
	case "observations":
		var r struct {
			SensorID string `json:"sensor_id"`
			Context  struct {
				Direction              string `json:"direction"`
				VLANID                 int    `json:"vlan_id"`
				Zone                   string `json:"zone"`
				NetworkName            string `json:"network_name"`
				SVI                    string `json:"svi"`
				KernelOffloadSuspected bool   `json:"kernel_offload_suspected"`
			} `json:"context"`
			Observed struct {
				Interface      string `json:"interface"`
				InterfaceIndex int    `json:"interface_index"`
				CapturedLength int    `json:"captured_length"`
				Network        struct {
					Family        string `json:"family"`
					Protocol      string `json:"protocol"`
					SourceIP      string `json:"source_ip"`
					DestinationIP string `json:"destination_ip"`
				} `json:"network"`
				Transport struct {
					SourcePort      int `json:"source_port"`
					DestinationPort int `json:"destination_port"`
				} `json:"transport"`
				Ethernet struct {
					SourceMAC      string `json:"source_mac"`
					DestinationMAC string `json:"destination_mac"`
					EtherType      string `json:"ethertype"`
				} `json:"ethernet"`
				TLS struct {
					ClientHelloObserved bool   `json:"client_hello_observed"`
					SNI                 string `json:"sni"`
				} `json:"tls"`
				DNS struct {
					MessageType string `json:"message_type"`
					QueryName   string `json:"query_name"`
					QueryType   string `json:"query_type"`
					Questions   []struct {
						Name string `json:"name"`
						Type string `json:"type"`
					} `json:"questions"`
					Answers     []json.RawMessage `json:"answers"`
					Authorities []json.RawMessage `json:"authorities"`
					Additionals []json.RawMessage `json:"additionals"`
				} `json:"dns"`
				ARP struct {
					Operation string `json:"operation"`
				} `json:"arp"`
			} `json:"observed"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		o := r.Observed
		c := r.Context
		n := o.Network
		sip, err := validIP(n.SourceIP)
		if err != nil {
			return err
		}
		dip, err := validIP(n.DestinationIP)
		if err != nil {
			return err
		}
		sm, err := validMAC(o.Ethernet.SourceMAC)
		if err != nil {
			return err
		}
		dm, err := validMAC(o.Ethernet.DestinationMAC)
		if err != nil {
			return err
		}
		row := append(common,
			null(o.Interface), strconv.Itoa(o.InterfaceIndex), null(c.Direction), strconv.Itoa(c.VLANID), null(c.Zone), null(c.NetworkName), null(c.SVI),
			null(n.Family), null(n.Protocol), sip, dip, optionalPort(n.Protocol, o.Transport.SourcePort), optionalPort(n.Protocol, o.Transport.DestinationPort),
			sm, dm, null(o.Ethernet.EtherType), strconv.Itoa(o.CapturedLength), null(o.DNS.MessageType), null(o.DNS.QueryName), null(o.DNS.QueryType),
			b(o.TLS.ClientHelloObserved), null(o.TLS.SNI), null(o.ARP.Operation), b(c.KernelOffloadSuspected), string(raw))
		if err := e.add("sensor_observation", row...); err != nil {
			return err
		}
		for i, q := range o.DNS.Questions {
			if err := e.add("sensor_dns_question", at, id, strconv.Itoa(i), q.Name, q.Type); err != nil {
				return err
			}
		}
		for _, section := range []struct {
			name string
			rows []json.RawMessage
		}{{"answer", o.DNS.Answers}, {"authority", o.DNS.Authorities}, {"additional", o.DNS.Additionals}} {
			for i, v := range section.rows {
				var dr struct {
					Name  string `json:"name"`
					Type  string `json:"type"`
					Class string `json:"class"`
					TTL   int64  `json:"ttl"`
					Value string `json:"value"`
				}
				if err := json.Unmarshal(v, &dr); err != nil {
					return err
				}
				if err := e.add("sensor_dns_record", at, id, section.name, strconv.Itoa(i), dr.Name, dr.Type, null(dr.Class), int64s(dr.TTL), null(dr.Value), string(v)); err != nil {
					return err
				}
			}
		}
	case "conntrack":
		var r struct {
			ID              uint32 `json:"id"`
			EventType       string `json:"event_type"`
			SourceNAT       bool   `json:"source_nat"`
			DestinationNAT  bool   `json:"destination_nat"`
			TCPState        string `json:"tcp_state"`
			Assured         bool   `json:"assured"`
			Confirmed       bool   `json:"confirmed"`
			SeenReply       bool   `json:"seen_reply"`
			Dying           bool   `json:"dying"`
			Offload         bool   `json:"offload"`
			HardwareOffload bool   `json:"hardware_offload"`
			TimeoutSeconds  uint64 `json:"timeout_seconds"`
			Status          uint64 `json:"status"`
			Mark            uint64 `json:"mark"`
			Original        struct {
				Family          string `json:"family"`
				Protocol        string `json:"protocol"`
				SourceIP        string `json:"source_ip"`
				DestinationIP   string `json:"destination_ip"`
				SourcePort      int    `json:"source_port"`
				DestinationPort int    `json:"destination_port"`
			} `json:"original"`
			Reply struct {
				Family          string `json:"family"`
				Protocol        string `json:"protocol"`
				SourceIP        string `json:"source_ip"`
				DestinationIP   string `json:"destination_ip"`
				SourcePort      int    `json:"source_port"`
				DestinationPort int    `json:"destination_port"`
			} `json:"reply"`
			OriginalCounters struct {
				Present bool   `json:"present"`
				Packets uint64 `json:"packets"`
				Bytes   uint64 `json:"bytes"`
			} `json:"original_counters"`
			ReplyCounters struct {
				Present bool   `json:"present"`
				Packets uint64 `json:"packets"`
				Bytes   uint64 `json:"bytes"`
			} `json:"reply_counters"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		oi, err := validIP(r.Original.SourceIP)
		if err != nil {
			return err
		}
		od, err := validIP(r.Original.DestinationIP)
		if err != nil {
			return err
		}
		ri, err := validIP(r.Reply.SourceIP)
		if err != nil {
			return err
		}
		rd, err := validIP(r.Reply.DestinationIP)
		if err != nil {
			return err
		}
		oc := r.OriginalCounters
		rc := r.ReplyCounters
		op, ob, rp, rb := "\\N", "\\N", "\\N", "\\N"
		if oc.Present {
			op = strconv.FormatUint(oc.Packets, 10)
			ob = strconv.FormatUint(oc.Bytes, 10)
		}
		if rc.Present {
			rp = strconv.FormatUint(rc.Packets, 10)
			rb = strconv.FormatUint(rc.Bytes, 10)
		}
		row := append(common,
			strconv.FormatUint(uint64(r.ID), 10), r.EventType,
			null(r.Original.Family), null(r.Original.Protocol), oi, od, optionalPort(r.Original.Protocol, r.Original.SourcePort), optionalPort(r.Original.Protocol, r.Original.DestinationPort),
			null(r.Reply.Family), null(r.Reply.Protocol), ri, rd, optionalPort(r.Reply.Protocol, r.Reply.SourcePort), optionalPort(r.Reply.Protocol, r.Reply.DestinationPort),
			b(r.SourceNAT), b(r.DestinationNAT), null(r.TCPState), b(r.Assured), b(r.Confirmed), b(r.SeenReply), b(r.Dying), b(r.Offload), b(r.HardwareOffload),
			strconv.FormatUint(r.TimeoutSeconds, 10), strconv.FormatUint(r.Status, 10), strconv.FormatUint(r.Mark, 10), b(oc.Present), op, ob, b(rc.Present), rp, rb, string(raw))
		if err := e.add("sensor_conntrack_event", row...); err != nil {
			return err
		}
	case "identity":
		var r struct {
			ExpectedIntervalSeconds int64  `json:"expected_interval_seconds"`
			Source                  string `json:"source"`
			SourcePath              string `json:"source_path"`
			Endpoints               []struct {
				IP             string `json:"ip"`
				MAC            string `json:"mac"`
				Hostname       string `json:"hostname"`
				FQDN           string `json:"fqdn"`
				LeaseExpiresAt string `json:"lease_expires_at"`
			} `json:"endpoints"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		if r.ExpectedIntervalSeconds <= 0 {
			return fmt.Errorf("identity expected_interval_seconds missing/invalid")
		}
		snapshot := id
		if err := e.add("sensor_identity_snapshot", snapshot, ctx.SensorUUID, ctx.BatchUUID, order, segOrd, at, int64s(observed.UnixNano()), int64s(r.ExpectedIntervalSeconds), null(r.Source), null(r.SourcePath), string(raw)); err != nil {
			return err
		}
		for i, ep := range r.Endpoints {
			ip, err := validIP(ep.IP)
			if err != nil {
				return err
			}
			mac, err := validMAC(ep.MAC)
			if err != nil {
				return err
			}
			expires, err := optionalTime(ep.LeaseExpiresAt)
			if err != nil {
				return err
			}
			original, err := json.Marshal(ep)
			if err != nil {
				return err
			}
			if err := e.add("sensor_identity_endpoint", snapshot, strconv.Itoa(i), ip, mac, null(ep.Hostname), null(ep.FQDN), expires, string(original)); err != nil {
				return err
			}
		}
	case "health":
		var r struct {
			ExpectedIntervalSeconds int64 `json:"expected_interval_seconds"`
			Final                   bool  `json:"final"`
			Capture                 struct {
				StartedAt             string `json:"started_at"`
				ObservationsWritten   uint64 `json:"observations_written"`
				ObservationsDiscarded uint64 `json:"observations_discarded"`
				WriteFailures         uint64 `json:"write_failures"`
			} `json:"capture"`
			Conntrack struct {
				Enabled         bool   `json:"enabled"`
				EventsReceived  uint64 `json:"events_received"`
				ReceiveOverruns uint64 `json:"receive_overruns"`
			} `json:"conntrack"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		if r.ExpectedIntervalSeconds <= 0 {
			return fmt.Errorf("health expected_interval_seconds missing/invalid")
		}
		capture, err := optionalTime(r.Capture.StartedAt)
		if err != nil {
			return err
		}
		if capture == "\\N" {
			return fmt.Errorf("health missing capture.started_at")
		}
		if err := e.add("sensor_health", id, ctx.SensorUUID, ctx.BatchUUID, order, segOrd, at, int64s(observed.UnixNano()), capture, int64s(r.ExpectedIntervalSeconds), b(r.Final), strconv.FormatUint(r.Capture.ObservationsWritten, 10), strconv.FormatUint(r.Capture.ObservationsDiscarded, 10), strconv.FormatUint(r.Capture.WriteFailures, 10), b(r.Conntrack.Enabled), strconv.FormatUint(r.Conntrack.EventsReceived, 10), strconv.FormatUint(r.Conntrack.ReceiveOverruns, 10), string(raw)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown stream %q", stream)
	}
	e.Count++
	return nil
}
