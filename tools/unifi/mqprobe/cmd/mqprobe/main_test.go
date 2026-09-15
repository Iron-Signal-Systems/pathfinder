package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

func TestParseDelivery(t *testing.T) {
	var body bytes.Buffer
	writeShortstr(&body, "ctag-test")
	if err := binary.Write(&body, binary.BigEndian, uint64(42)); err != nil {
		t.Fatal(err)
	}
	body.WriteByte(1)
	writeShortstr(&body, "unifi_flow_exchange")
	writeShortstr(&body, "flow_event")

	got, err := parseDelivery(body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got.ConsumerTag != "ctag-test" || got.DeliveryTag != 42 || !got.Redelivered || got.Exchange != "unifi_flow_exchange" || got.RoutingKey != "flow_event" {
		t.Fatalf("unexpected delivery: %+v", got)
	}
}

func TestParseContentHeader(t *testing.T) {
	var payload bytes.Buffer
	if err := binary.Write(&payload, binary.BigEndian, uint16(basicClass)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&payload, binary.BigEndian, uint16(0)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&payload, binary.BigEndian, uint64(123)); err != nil {
		t.Fatal(err)
	}

	const flags = uint16(0x8000 | 0x2000 | 0x1000 | 0x0080 | 0x0040 | 0x0020 | 0x0008)
	if err := binary.Write(&payload, binary.BigEndian, flags); err != nil {
		t.Fatal(err)
	}
	writeShortstr(&payload, "application/json")
	writeTable(&payload, map[string]tableValue{"source": {kind: 'S', value: "test"}})
	payload.WriteByte(2)
	writeShortstr(&payload, "message-1")
	if err := binary.Write(&payload, binary.BigEndian, uint64(1_700_000_000)); err != nil {
		t.Fatal(err)
	}
	writeShortstr(&payload, "flow_event")
	writeShortstr(&payload, "unifi-mq-broker")

	props, bodySize, err := parseContentHeader(payload.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if bodySize != 123 {
		t.Fatalf("body size: got %d want 123", bodySize)
	}
	if props.ContentType != "application/json" || props.DeliveryMode != 2 || props.MessageID != "message-1" || props.Type != "flow_event" || props.AppID != "unifi-mq-broker" {
		t.Fatalf("unexpected properties: %+v", props)
	}
	if props.Headers["source"] != "test" {
		t.Fatalf("unexpected headers: %#v", props.Headers)
	}
	wantTime := time.Unix(1_700_000_000, 0).UTC()
	if props.Timestamp == nil || !props.Timestamp.Equal(wantTime) {
		t.Fatalf("timestamp: got %v want %v", props.Timestamp, wantTime)
	}
}

func TestReadTablePreservesRawBytes(t *testing.T) {
	var encoded bytes.Buffer
	writeTable(&encoded, map[string]tableValue{
		"enabled": {kind: 't', value: true},
		"name":    {kind: 'S', value: "pathfinder"},
	})

	decoded, raw, err := readTable(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("expected raw table bytes")
	}
	if decoded["enabled"] != true || decoded["name"] != "pathfinder" {
		t.Fatalf("unexpected decoded table: %#v", decoded)
	}
}

func TestChannelOpenUsesShortstrReservedField(t *testing.T) {
	var body bytes.Buffer
	writeShortstr(&body, "")

	got := body.Bytes()
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("channel.open reserved field must be one-byte empty shortstr, got %x", got)
	}
}

func TestReadMethodSurfacesConnectionCloseOnChannelZero(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	c := &amqpClient{conn: client, reader: bufio.NewReader(client), frameMax: 131072}

	go func() {
		var payload bytes.Buffer
		_ = binary.Write(&payload, binary.BigEndian, uint16(501))
		writeShortstr(&payload, "FRAME_ERROR - bad channel.open")
		_ = binary.Write(&payload, binary.BigEndian, uint16(channelClass))
		_ = binary.Write(&payload, binary.BigEndian, uint16(10))

		methodPayload := make([]byte, 4+payload.Len())
		binary.BigEndian.PutUint16(methodPayload[0:2], connectionClass)
		binary.BigEndian.PutUint16(methodPayload[2:4], 50)
		copy(methodPayload[4:], payload.Bytes())

		var header [7]byte
		header[0] = frameMethod
		binary.BigEndian.PutUint16(header[1:3], 0)
		binary.BigEndian.PutUint32(header[3:7], uint32(len(methodPayload)))
		_, _ = server.Write(header[:])
		_, _ = server.Write(methodPayload)
		_, _ = server.Write([]byte{frameEnd})
	}()

	_, _, _, err := c.readMethod(1)
	if err == nil || !strings.Contains(err.Error(), "code=501") || !strings.Contains(err.Error(), "bad channel.open") {
		t.Fatalf("expected broker connection.close details, got %v", err)
	}
}
