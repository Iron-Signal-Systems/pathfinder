// Copyright (c) Iron Signal Systems.
//
// mqprobe is a deliberately small, dependency-free AMQP 0-9-1 observation
// utility for the Pathfinder UniFi sensor lab. It creates a server-named,
// exclusive, auto-delete queue and binds it to an existing exchange. It never
// consumes from UniFi's existing queue.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	frameMethod    = 1
	frameHeader    = 2
	frameBody      = 3
	frameHeartbeat = 8
	frameEnd       = 0xCE

	connectionClass = 10
	channelClass    = 20
	queueClass      = 50
	basicClass      = 60
)

type config struct {
	Exchange   string
	Host       string
	Limit      uint64
	OutputPath string
	Password   string
	Port       int
	RoutingKey string
	Timeout    time.Duration
	Username   string
	VHost      string
}

type frame struct {
	Channel uint16
	Payload []byte
	Type    byte
}

type delivery struct {
	ConsumerTag string
	DeliveryTag uint64
	Exchange    string
	Redelivered bool
	RoutingKey  string
}

type properties struct {
	AppID           string         `json:"app_id,omitempty"`
	ContentEncoding string         `json:"content_encoding,omitempty"`
	ContentType     string         `json:"content_type,omitempty"`
	CorrelationID   string         `json:"correlation_id,omitempty"`
	DeliveryMode    uint8          `json:"delivery_mode,omitempty"`
	Expiration      string         `json:"expiration,omitempty"`
	Headers         map[string]any `json:"headers,omitempty"`
	HeadersRawB64   string         `json:"headers_raw_base64,omitempty"`
	MessageID       string         `json:"message_id,omitempty"`
	Priority        uint8          `json:"priority,omitempty"`
	ReplyTo         string         `json:"reply_to,omitempty"`
	Timestamp       *time.Time     `json:"timestamp,omitempty"`
	Type            string         `json:"type,omitempty"`
	UserID          string         `json:"user_id,omitempty"`
}

type observation struct {
	BodyBase64 string     `json:"raw_body_base64"`
	BodyLength int        `json:"body_length"`
	BodySHA256 string     `json:"body_sha256"`
	BodyUTF8   *string    `json:"raw_body_utf8,omitempty"`
	Delivery   delivery   `json:"delivery"`
	Properties properties `json:"properties"`
	ReceivedAt time.Time  `json:"received_at"`
}

type amqpClient struct {
	conn      net.Conn
	frameMax  uint32
	heartbeat uint16
	reader    *bufio.Reader
	writeMu   sync.Mutex
}

func main() {
	cfg := parseFlags()

	connAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := net.DialTimeout("tcp", connAddr, 5*time.Second)
	if err != nil {
		fatalf("connect %s: %v", connAddr, err)
	}
	defer conn.Close()

	client := &amqpClient{
		conn:     conn,
		frameMax: 131072,
		reader:   bufio.NewReader(conn),
	}

	if err := client.handshake(cfg); err != nil {
		fatalf("AMQP handshake: %v", err)
	}

	queueName, err := client.declareEphemeralQueue(1)
	if err != nil {
		fatalf("declare temporary queue: %v", err)
	}
	if err := client.bindQueue(1, queueName, cfg.Exchange, cfg.RoutingKey); err != nil {
		fatalf("bind temporary queue: %v", err)
	}
	consumerTag, err := client.consume(1, queueName)
	if err != nil {
		fatalf("consume temporary queue: %v", err)
	}

	var output io.Writer = os.Stdout
	var outputFile *os.File
	if cfg.OutputPath != "" {
		outputFile, err = os.OpenFile(cfg.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			fatalf("open output %q: %v", cfg.OutputPath, err)
		}
		defer outputFile.Close()
		output = io.MultiWriter(os.Stdout, outputFile)
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)

	fmt.Fprintf(os.Stderr, "mqprobe connected host=%s port=%d vhost=%q exchange=%q routing_key=%q\n", cfg.Host, cfg.Port, cfg.VHost, cfg.Exchange, cfg.RoutingKey)
	fmt.Fprintf(os.Stderr, "temporary_queue=%s durable=false auto_delete=true exclusive=true consumer_tag=%s no_ack=true\n", queueName, consumerTag)
	if cfg.OutputPath != "" {
		fmt.Fprintf(os.Stderr, "output=%s\n", cfg.OutputPath)
	}

	ctxDone := make(chan struct{})
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			close(ctxDone)
			_ = conn.Close()
		})
	}

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		stop()
	}()

	if cfg.Timeout > 0 {
		timer := time.NewTimer(cfg.Timeout)
		defer timer.Stop()
		go func() {
			select {
			case <-timer.C:
				fmt.Fprintf(os.Stderr, "timeout reached after %s\n", cfg.Timeout)
				stop()
			case <-ctxDone:
			}
		}()
	}

	if client.heartbeat > 0 {
		go client.heartbeatLoop(ctxDone)
	}

	var count uint64
	for {
		obs, err := client.readObservation(1)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) || strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			fatalf("consume: %v", err)
		}
		count++
		if err := encoder.Encode(obs); err != nil {
			fatalf("write observation: %v", err)
		}
		if outputFile != nil {
			if err := outputFile.Sync(); err != nil {
				fatalf("sync output: %v", err)
			}
		}
		if cfg.Limit > 0 && count >= cfg.Limit {
			fmt.Fprintf(os.Stderr, "message limit reached: %d\n", count)
			stop()
			break
		}
	}

	fmt.Fprintf(os.Stderr, "mqprobe stopped messages=%d temporary queue is removed when the AMQP connection closes\n", count)
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.Host, "host", "127.0.0.1", "RabbitMQ host")
	flag.IntVar(&cfg.Port, "port", 5672, "RabbitMQ port")
	flag.StringVar(&cfg.Username, "username", "ui", "RabbitMQ username")
	flag.StringVar(&cfg.Password, "password", envOrDefault("PATHFINDER_MQ_PASSWORD", "ui"), "RabbitMQ password; PATHFINDER_MQ_PASSWORD is preferred")
	flag.StringVar(&cfg.VHost, "vhost", "/", "RabbitMQ vhost")
	flag.StringVar(&cfg.Exchange, "exchange", "unifi_flow_exchange", "existing exchange to observe")
	flag.StringVar(&cfg.RoutingKey, "routing-key", "flow_event", "routing key to bind")
	flag.Uint64Var(&cfg.Limit, "limit", 0, "stop after N messages; 0 means unlimited")
	flag.DurationVar(&cfg.Timeout, "timeout", 0, "stop after duration such as 30s; 0 means unlimited")
	flag.StringVar(&cfg.OutputPath, "output", "", "optional NDJSON output file; stdout is always written")
	flag.Parse()

	if cfg.Port < 1 || cfg.Port > 65535 {
		fatalf("invalid port: %d", cfg.Port)
	}
	if cfg.Host == "" || cfg.Username == "" || cfg.Exchange == "" || cfg.RoutingKey == "" {
		fatalf("host, username, exchange, and routing-key must be non-empty")
	}
	return cfg
}

func envOrDefault(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "mqprobe: "+format+"\n", args...)
	os.Exit(1)
}

func (c *amqpClient) handshake(cfg config) error {
	if _, err := c.conn.Write([]byte("AMQP\x00\x00\x09\x01")); err != nil {
		return err
	}

	class, method, payload, err := c.readMethod(0)
	if err != nil {
		return err
	}
	if class != connectionClass || method != 10 { // connection.start
		return fmt.Errorf("expected connection.start, got class=%d method=%d", class, method)
	}
	mechanisms, err := parseStart(payload)
	if err != nil {
		return err
	}
	if !strings.Contains(" "+mechanisms+" ", " PLAIN ") && mechanisms != "PLAIN" {
		return fmt.Errorf("broker does not advertise PLAIN authentication: %q", mechanisms)
	}

	var startOK bytes.Buffer
	writeTable(&startOK, map[string]tableValue{
		"product":  {kind: 'S', value: "pathfinder-mqprobe"},
		"version":  {kind: 'S', value: "v0"},
		"platform": {kind: 'S', value: "golang-stdlib"},
	})
	writeShortstr(&startOK, "PLAIN")
	response := "\x00" + cfg.Username + "\x00" + cfg.Password
	writeLongstr(&startOK, []byte(response))
	writeShortstr(&startOK, "en_US")
	if err := c.writeMethod(0, connectionClass, 11, startOK.Bytes()); err != nil { // start-ok
		return err
	}

	class, method, payload, err = c.readMethod(0)
	if err != nil {
		return err
	}
	if class == connectionClass && method == 50 { // connection.close
		return parseConnectionClose(payload)
	}
	if class != connectionClass || method != 30 { // tune
		return fmt.Errorf("expected connection.tune, got class=%d method=%d", class, method)
	}
	if len(payload) < 8 {
		return io.ErrUnexpectedEOF
	}
	channelMax := binary.BigEndian.Uint16(payload[0:2])
	frameMax := binary.BigEndian.Uint32(payload[2:6])
	heartbeat := binary.BigEndian.Uint16(payload[6:8])
	if frameMax == 0 {
		frameMax = 131072
	}
	c.frameMax = frameMax
	c.heartbeat = heartbeat

	var tuneOK bytes.Buffer
	_ = binary.Write(&tuneOK, binary.BigEndian, channelMax)
	_ = binary.Write(&tuneOK, binary.BigEndian, frameMax)
	_ = binary.Write(&tuneOK, binary.BigEndian, heartbeat)
	if err := c.writeMethod(0, connectionClass, 31, tuneOK.Bytes()); err != nil {
		return err
	}

	var open bytes.Buffer
	writeShortstr(&open, cfg.VHost)
	writeShortstr(&open, "")
	open.WriteByte(0) // insist=false
	if err := c.writeMethod(0, connectionClass, 40, open.Bytes()); err != nil {
		return err
	}

	class, method, payload, err = c.readMethod(0)
	if err != nil {
		return err
	}
	if class == connectionClass && method == 50 {
		return parseConnectionClose(payload)
	}
	if class != connectionClass || method != 41 { // open-ok
		return fmt.Errorf("expected connection.open-ok, got class=%d method=%d", class, method)
	}

	if err := c.openChannel(1); err != nil {
		return err
	}
	return nil
}

func parseStart(payload []byte) (string, error) {
	r := bytes.NewReader(payload)
	if _, err := r.ReadByte(); err != nil { // version major
		return "", err
	}
	if _, err := r.ReadByte(); err != nil { // version minor
		return "", err
	}
	if err := skipTable(r); err != nil {
		return "", fmt.Errorf("server properties: %w", err)
	}
	mechanisms, err := readLongstr(r)
	if err != nil {
		return "", err
	}
	if _, err := readLongstr(r); err != nil { // locales
		return "", err
	}
	return string(mechanisms), nil
}

func parseConnectionClose(payload []byte) error {
	r := bytes.NewReader(payload)
	var code uint16
	if err := binary.Read(r, binary.BigEndian, &code); err != nil {
		return err
	}
	text, err := readShortstr(r)
	if err != nil {
		return err
	}
	return fmt.Errorf("broker closed connection: code=%d text=%q", code, text)
}

func (c *amqpClient) openChannel(channel uint16) error {
	var body bytes.Buffer
	// channel.open reserved-1 is an AMQP shortstr, not a longstr.
	writeShortstr(&body, "")
	if err := c.writeMethod(channel, channelClass, 10, body.Bytes()); err != nil {
		return err
	}
	class, method, payload, err := c.readMethod(channel)
	if err != nil {
		return err
	}
	if class == channelClass && method == 40 {
		return parseChannelClose(payload)
	}
	if class != channelClass || method != 11 {
		return fmt.Errorf("expected channel.open-ok, got class=%d method=%d", class, method)
	}
	return nil
}

func parseChannelClose(payload []byte) error {
	r := bytes.NewReader(payload)
	var code uint16
	if err := binary.Read(r, binary.BigEndian, &code); err != nil {
		return err
	}
	text, err := readShortstr(r)
	if err != nil {
		return err
	}
	return fmt.Errorf("broker closed channel: code=%d text=%q", code, text)
}

func (c *amqpClient) declareEphemeralQueue(channel uint16) (string, error) {
	var body bytes.Buffer
	_ = binary.Write(&body, binary.BigEndian, uint16(0))
	writeShortstr(&body, "")
	body.WriteByte(0x0C) // exclusive=true, auto-delete=true
	writeEmptyTable(&body)

	if err := c.writeMethod(channel, queueClass, 10, body.Bytes()); err != nil {
		return "", err
	}
	class, method, payload, err := c.readMethod(channel)
	if err != nil {
		return "", err
	}
	if class == channelClass && method == 40 {
		return "", parseChannelClose(payload)
	}
	if class != queueClass || method != 11 {
		return "", fmt.Errorf("expected queue.declare-ok, got class=%d method=%d", class, method)
	}
	r := bytes.NewReader(payload)
	queueName, err := readShortstr(r)
	if err != nil {
		return "", err
	}
	return queueName, nil
}

func (c *amqpClient) bindQueue(channel uint16, queueName, exchange, routingKey string) error {
	var body bytes.Buffer
	_ = binary.Write(&body, binary.BigEndian, uint16(0))
	writeShortstr(&body, queueName)
	writeShortstr(&body, exchange)
	writeShortstr(&body, routingKey)
	body.WriteByte(0) // no-wait=false
	writeEmptyTable(&body)

	if err := c.writeMethod(channel, queueClass, 20, body.Bytes()); err != nil {
		return err
	}
	class, method, payload, err := c.readMethod(channel)
	if err != nil {
		return err
	}
	if class == channelClass && method == 40 {
		return parseChannelClose(payload)
	}
	if class != queueClass || method != 21 {
		return fmt.Errorf("expected queue.bind-ok, got class=%d method=%d", class, method)
	}
	return nil
}

func (c *amqpClient) consume(channel uint16, queueName string) (string, error) {
	var body bytes.Buffer
	_ = binary.Write(&body, binary.BigEndian, uint16(0))
	writeShortstr(&body, queueName)
	writeShortstr(&body, "")
	body.WriteByte(0x02) // no-ack=true; no-local/exclusive/no-wait=false
	writeEmptyTable(&body)

	if err := c.writeMethod(channel, basicClass, 20, body.Bytes()); err != nil {
		return "", err
	}
	class, method, payload, err := c.readMethod(channel)
	if err != nil {
		return "", err
	}
	if class == channelClass && method == 40 {
		return "", parseChannelClose(payload)
	}
	if class != basicClass || method != 21 {
		return "", fmt.Errorf("expected basic.consume-ok, got class=%d method=%d", class, method)
	}
	r := bytes.NewReader(payload)
	return readShortstr(r)
}

func (c *amqpClient) readObservation(channel uint16) (observation, error) {
	for {
		f, err := c.readFrame()
		if err != nil {
			return observation{}, err
		}
		if f.Type == frameHeartbeat {
			continue
		}
		if f.Type != frameMethod || f.Channel != channel {
			continue
		}
		if len(f.Payload) < 4 {
			return observation{}, io.ErrUnexpectedEOF
		}
		class := binary.BigEndian.Uint16(f.Payload[0:2])
		method := binary.BigEndian.Uint16(f.Payload[2:4])
		payload := f.Payload[4:]

		if class == connectionClass && method == 50 {
			return observation{}, parseConnectionClose(payload)
		}
		if class == channelClass && method == 40 {
			return observation{}, parseChannelClose(payload)
		}
		if class != basicClass || method != 60 { // basic.deliver
			continue
		}

		d, err := parseDelivery(payload)
		if err != nil {
			return observation{}, err
		}
		header, err := c.readExpectedFrame(frameHeader, channel)
		if err != nil {
			return observation{}, err
		}
		props, bodySize, err := parseContentHeader(header.Payload)
		if err != nil {
			return observation{}, err
		}
		body, err := c.readBody(channel, bodySize)
		if err != nil {
			return observation{}, err
		}
		digest := sha256.Sum256(body)
		obs := observation{
			BodyBase64: base64.StdEncoding.EncodeToString(body),
			BodyLength: len(body),
			BodySHA256: hex.EncodeToString(digest[:]),
			Delivery:   d,
			Properties: props,
			ReceivedAt: time.Now().UTC(),
		}
		if utf8.Valid(body) {
			text := string(body)
			obs.BodyUTF8 = &text
		}
		return obs, nil
	}
}

func parseDelivery(payload []byte) (delivery, error) {
	r := bytes.NewReader(payload)
	consumerTag, err := readShortstr(r)
	if err != nil {
		return delivery{}, err
	}
	var deliveryTag uint64
	if err := binary.Read(r, binary.BigEndian, &deliveryTag); err != nil {
		return delivery{}, err
	}
	bits, err := r.ReadByte()
	if err != nil {
		return delivery{}, err
	}
	exchange, err := readShortstr(r)
	if err != nil {
		return delivery{}, err
	}
	routingKey, err := readShortstr(r)
	if err != nil {
		return delivery{}, err
	}
	return delivery{
		ConsumerTag: consumerTag,
		DeliveryTag: deliveryTag,
		Exchange:    exchange,
		Redelivered: bits&0x01 != 0,
		RoutingKey:  routingKey,
	}, nil
}

func parseContentHeader(payload []byte) (properties, uint64, error) {
	r := bytes.NewReader(payload)
	var classID, weight uint16
	var bodySize uint64
	if err := binary.Read(r, binary.BigEndian, &classID); err != nil {
		return properties{}, 0, err
	}
	if err := binary.Read(r, binary.BigEndian, &weight); err != nil {
		return properties{}, 0, err
	}
	if classID != basicClass || weight != 0 {
		return properties{}, 0, fmt.Errorf("unexpected content header class=%d weight=%d", classID, weight)
	}
	if err := binary.Read(r, binary.BigEndian, &bodySize); err != nil {
		return properties{}, 0, err
	}

	var flags uint16
	if err := binary.Read(r, binary.BigEndian, &flags); err != nil {
		return properties{}, 0, err
	}
	if flags&0x0001 != 0 {
		return properties{}, 0, fmt.Errorf("multi-word basic property flags are not supported by mqprobe")
	}

	var p properties
	var err error
	if flags&0x8000 != 0 {
		p.ContentType, err = readShortstr(r)
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x4000 != 0 {
		p.ContentEncoding, err = readShortstr(r)
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x2000 != 0 {
		decoded, raw, tableErr := readTable(r)
		if tableErr != nil {
			return properties{}, 0, tableErr
		}
		p.Headers = decoded
		p.HeadersRawB64 = base64.StdEncoding.EncodeToString(raw)
	}
	if flags&0x1000 != 0 {
		p.DeliveryMode, err = r.ReadByte()
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x0800 != 0 {
		p.Priority, err = r.ReadByte()
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x0400 != 0 {
		p.CorrelationID, err = readShortstr(r)
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x0200 != 0 {
		p.ReplyTo, err = readShortstr(r)
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x0100 != 0 {
		p.Expiration, err = readShortstr(r)
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x0080 != 0 {
		p.MessageID, err = readShortstr(r)
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x0040 != 0 {
		var unix uint64
		if err := binary.Read(r, binary.BigEndian, &unix); err != nil {
			return properties{}, 0, err
		}
		ts := time.Unix(int64(unix), 0).UTC()
		p.Timestamp = &ts
	}
	if flags&0x0020 != 0 {
		p.Type, err = readShortstr(r)
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x0010 != 0 {
		p.UserID, err = readShortstr(r)
		if err != nil {
			return properties{}, 0, err
		}
	}
	if flags&0x0008 != 0 {
		p.AppID, err = readShortstr(r)
		if err != nil {
			return properties{}, 0, err
		}
	}
	return p, bodySize, nil
}

func (c *amqpClient) readBody(channel uint16, bodySize uint64) ([]byte, error) {
	if bodySize == 0 {
		return []byte{}, nil
	}
	if bodySize > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("message body too large: %d", bodySize)
	}
	body := make([]byte, 0, int(bodySize))
	for uint64(len(body)) < bodySize {
		f, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		if f.Type == frameHeartbeat {
			continue
		}
		if f.Type != frameBody || f.Channel != channel {
			return nil, fmt.Errorf("expected body frame on channel %d, got type=%d channel=%d", channel, f.Type, f.Channel)
		}
		remaining := int(bodySize) - len(body)
		if len(f.Payload) > remaining {
			return nil, fmt.Errorf("body frame exceeds declared body size")
		}
		body = append(body, f.Payload...)
	}
	return body, nil
}

func (c *amqpClient) heartbeatLoop(done <-chan struct{}) {
	interval := time.Duration(c.heartbeat) * time.Second / 2
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.writeFrame(frameHeartbeat, 0, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func (c *amqpClient) readExpectedFrame(frameType byte, channel uint16) (frame, error) {
	for {
		f, err := c.readFrame()
		if err != nil {
			return frame{}, err
		}
		if f.Type == frameHeartbeat {
			continue
		}
		if f.Type != frameType || f.Channel != channel {
			return frame{}, fmt.Errorf("expected frame type=%d channel=%d, got type=%d channel=%d", frameType, channel, f.Type, f.Channel)
		}
		return f, nil
	}
}

func (c *amqpClient) readMethod(channel uint16) (uint16, uint16, []byte, error) {
	for {
		f, err := c.readFrame()
		if err != nil {
			return 0, 0, nil, err
		}
		if f.Type == frameHeartbeat {
			continue
		}
		if f.Type != frameMethod {
			return 0, 0, nil, fmt.Errorf("expected method frame channel=%d, got type=%d channel=%d", channel, f.Type, f.Channel)
		}
		if len(f.Payload) < 4 {
			return 0, 0, nil, io.ErrUnexpectedEOF
		}

		class := binary.BigEndian.Uint16(f.Payload[0:2])
		method := binary.BigEndian.Uint16(f.Payload[2:4])
		payload := f.Payload[4:]

		// A connection.close is always sent on channel 0 and may arrive while
		// waiting for a channel-scoped reply. Surface the broker's actual
		// reason instead of reporting a misleading channel mismatch.
		if f.Channel == 0 && class == connectionClass && method == 50 {
			return 0, 0, nil, parseConnectionClose(payload)
		}

		// connection.blocked / connection.unblocked are asynchronous
		// connection-level notifications. They do not satisfy a channel RPC.
		if f.Channel == 0 && class == connectionClass && (method == 60 || method == 61) {
			continue
		}

		if f.Channel != channel {
			return 0, 0, nil, fmt.Errorf("expected method frame channel=%d, got class=%d method=%d channel=%d", channel, class, method, f.Channel)
		}
		return class, method, payload, nil
	}
}

func (c *amqpClient) readFrame() (frame, error) {
	header := make([]byte, 7)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return frame{}, err
	}
	size := binary.BigEndian.Uint32(header[3:7])
	if c.frameMax > 0 && size+8 > c.frameMax {
		return frame{}, fmt.Errorf("received frame larger than negotiated frame max: size=%d frame_max=%d", size+8, c.frameMax)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return frame{}, err
	}
	end, err := c.reader.ReadByte()
	if err != nil {
		return frame{}, err
	}
	if end != frameEnd {
		return frame{}, fmt.Errorf("invalid frame end marker: 0x%02x", end)
	}
	return frame{Type: header[0], Channel: binary.BigEndian.Uint16(header[1:3]), Payload: payload}, nil
}

func (c *amqpClient) writeMethod(channel, class, method uint16, payload []byte) error {
	body := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint16(body[0:2], class)
	binary.BigEndian.PutUint16(body[2:4], method)
	copy(body[4:], payload)
	return c.writeFrame(frameMethod, channel, body)
}

func (c *amqpClient) writeFrame(frameType byte, channel uint16, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var header [7]byte
	header[0] = frameType
	binary.BigEndian.PutUint16(header[1:3], channel)
	binary.BigEndian.PutUint32(header[3:7], uint32(len(payload)))
	if _, err := c.conn.Write(header[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := c.conn.Write(payload); err != nil {
			return err
		}
	}
	_, err := c.conn.Write([]byte{frameEnd})
	return err
}

type tableValue struct {
	kind  byte
	value any
}

func writeTable(w io.Writer, values map[string]tableValue) {
	var body bytes.Buffer
	for key, value := range values {
		writeShortstr(&body, key)
		body.WriteByte(value.kind)
		switch value.kind {
		case 'S':
			writeLongstr(&body, []byte(value.value.(string)))
		case 't':
			if value.value.(bool) {
				body.WriteByte(1)
			} else {
				body.WriteByte(0)
			}
		}
	}
	_ = binary.Write(w, binary.BigEndian, uint32(body.Len()))
	_, _ = w.Write(body.Bytes())
}

func writeEmptyTable(w io.Writer) {
	_ = binary.Write(w, binary.BigEndian, uint32(0))
}

func writeShortstr(w io.Writer, value string) {
	if len(value) > 255 {
		panic("AMQP shortstr exceeds 255 bytes")
	}
	_, _ = w.Write([]byte{byte(len(value))})
	_, _ = io.WriteString(w, value)
}

func writeLongstr(w io.Writer, value []byte) {
	_ = binary.Write(w, binary.BigEndian, uint32(len(value)))
	_, _ = w.Write(value)
}

func readShortstr(r *bytes.Reader) (string, error) {
	length, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	value := make([]byte, int(length))
	if _, err := io.ReadFull(r, value); err != nil {
		return "", err
	}
	return string(value), nil
}

func readLongstr(r *bytes.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if uint64(length) > uint64(r.Len()) {
		return nil, io.ErrUnexpectedEOF
	}
	value := make([]byte, int(length))
	if _, err := io.ReadFull(r, value); err != nil {
		return nil, err
	}
	return value, nil
}

func skipTable(r *bytes.Reader) error {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return err
	}
	if uint64(length) > uint64(r.Len()) {
		return io.ErrUnexpectedEOF
	}
	_, err := r.Seek(int64(length), io.SeekCurrent)
	return err
}

func readTable(r *bytes.Reader) (map[string]any, []byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, nil, err
	}
	if uint64(length) > uint64(r.Len()) {
		return nil, nil, io.ErrUnexpectedEOF
	}
	raw := make([]byte, int(length))
	if _, err := io.ReadFull(r, raw); err != nil {
		return nil, nil, err
	}
	decoded, err := decodeTableBody(bytes.NewReader(raw))
	if err != nil {
		return map[string]any{"decode_error": err.Error()}, raw, nil
	}
	return decoded, raw, nil
}

func decodeTableBody(r *bytes.Reader) (map[string]any, error) {
	result := make(map[string]any)
	for r.Len() > 0 {
		key, err := readShortstr(r)
		if err != nil {
			return nil, err
		}
		kind, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		value, err := decodeFieldValue(r, kind)
		if err != nil {
			return nil, fmt.Errorf("header %q type %q: %w", key, kind, err)
		}
		result[key] = value
	}
	return result, nil
}

func decodeFieldValue(r *bytes.Reader, kind byte) (any, error) {
	switch kind {
	case 't':
		b, err := r.ReadByte()
		return b != 0, err
	case 'b':
		b, err := r.ReadByte()
		return int8(b), err
	case 'B':
		b, err := r.ReadByte()
		return b, err
	case 'U':
		var v int16
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 'u':
		var v uint16
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 'I':
		var v int32
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 'i':
		var v uint32
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 'L':
		var v int64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 'l':
		var v uint64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 'f':
		var bits uint32
		if err := binary.Read(r, binary.BigEndian, &bits); err != nil {
			return nil, err
		}
		return fmt.Sprintf("float32_bits:0x%08x", bits), nil
	case 'd':
		var bits uint64
		if err := binary.Read(r, binary.BigEndian, &bits); err != nil {
			return nil, err
		}
		return fmt.Sprintf("float64_bits:0x%016x", bits), nil
	case 'D':
		scale, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		var value uint32
		if err := binary.Read(r, binary.BigEndian, &value); err != nil {
			return nil, err
		}
		return map[string]any{"scale": scale, "value": value}, nil
	case 's':
		var length uint16
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		b := make([]byte, int(length))
		_, err := io.ReadFull(r, b)
		return string(b), err
	case 'S':
		b, err := readLongstr(r)
		return string(b), err
	case 'x':
		b, err := readLongstr(r)
		if err != nil {
			return nil, err
		}
		return map[string]any{"base64": base64.StdEncoding.EncodeToString(b)}, nil
	case 'T':
		var unix uint64
		if err := binary.Read(r, binary.BigEndian, &unix); err != nil {
			return nil, err
		}
		return time.Unix(int64(unix), 0).UTC(), nil
	case 'F':
		var length uint32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		if uint64(length) > uint64(r.Len()) {
			return nil, io.ErrUnexpectedEOF
		}
		b := make([]byte, int(length))
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}
		return decodeTableBody(bytes.NewReader(b))
	case 'A':
		var length uint32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return nil, err
		}
		if uint64(length) > uint64(r.Len()) {
			return nil, io.ErrUnexpectedEOF
		}
		b := make([]byte, int(length))
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err
		}
		ar := bytes.NewReader(b)
		var values []any
		for ar.Len() > 0 {
			k, err := ar.ReadByte()
			if err != nil {
				return nil, err
			}
			v, err := decodeFieldValue(ar, k)
			if err != nil {
				return nil, err
			}
			values = append(values, v)
		}
		return values, nil
	case 'V':
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported AMQP field type 0x%02x", kind)
	}
}
