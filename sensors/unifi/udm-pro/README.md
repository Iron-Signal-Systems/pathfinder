# Pathfinder UDM-Pro Sensor

The Pathfinder UDM-Pro Sensor observes network activity visible from a UniFi UDM-Pro and preserves those observations for later Pathfinder analysis and threat-intelligence correlation.

The sensor is intentionally conservative about what it claims.

Direct packet observations, DNS observations, TLS observations, endpoint identity, connection-tracking state, and derived relationships remain distinct.

## Role in Pathfinder

The sensor answers:

> **What did this observation point actually see?**

Pathfinder core later answers:

> **Does what the sensor saw intersect with threat intelligence, does that intelligence apply, and what should Pathfinder conclude?**

The sensor does not classify traffic as malicious merely because it observes a domain, IP, connection pattern, or future threat-intelligence match.

```text
sensor observation
    != threat assertion
    != maliciousness
    != compromise
    != campaign attribution
```

## Observation Sources

The sensor currently supports:

- packet observation on explicitly configured interfaces
- DNS questions and responses observed in packet traffic
- plaintext TLS SNI when visible in a complete ClientHello
- endpoint identity history
- Linux conntrack history
- NAT correlation
- observed LAN-to-WAN path correlation
- collection-health history
- bounded and compressed local storage
- historical diagnostic queries

The currently validated UDM-Pro observation points are:

```text
br0
    LAN-side observation

eth8
    WAN-side observation
```

Interface names are deployment-specific and must remain explicitly configured.

## Interpretation Boundary

Pathfinder preserves the distinction between:

```text
What the source said
What was actually observed
What Pathfinder concluded
```

For this sensor:

```text
packet observation
    destination IP, protocol, ports, interface, direction, and timestamp
    directly observed by the sensor

DNS observation
    DNS question or answer directly observed by the sensor

TLS observation
    plaintext SNI directly observed in a complete TLS ClientHello

identity observation
    endpoint identity information available at the recorded time

conntrack observation
    connection state reported by Linux conntrack

correlation
    relationship derived between independent observations
```

A DNS name, SNI value, endpoint identity, or NAT relationship does not replace the underlying packet observation.

Missing information is not manufactured.

## Local Ring

The bounded compressed local ring is not intended to become Pathfinder's primary historical query platform.

Its primary purposes are:

```text
delivery resilience
short-term source record
sensor diagnostics
collector validation
troubleshooting
replay/reconciliation after delivery interruption
```

Commands such as local connection/path queries are therefore valuable diagnostic tools.

Normal broad historical queries are intended to run against the Pathfinder server's indexed sensor observation store once server ingestion is implemented.

See [`../../../docs/SENSOR-OBSERVATION-INGESTION.md`](../../../docs/SENSOR-OBSERVATION-INGESTION.md).

## Threat-Intelligence Correlation

The sensor may observe something such as:

```text
endpoint:     Living-Room-2
source:       192.168.1.172
destination:  207.121.116.20:443
TLS SNI:      siloh-ns1.plutotv.net
time:         T
```

That observation says the SNI was visible in a TLS ClientHello for that connection.

It does not say the hostname is good, bad, C2, compromised, or associated with a campaign.

If threat intelligence later contains an attributable Assertion about that Observable, Pathfinder core may correlate the records, evaluate time/platform/role applicability, and create a new Assessment.

The original sensor record remains unchanged.

See [`../../../docs/CORRELATION-HISTORICAL-REPROCESSING.md`](../../../docs/CORRELATION-HISTORICAL-REPROCESSING.md).

## UDM-Pro Limitations

The sensor does not claim:

- authoritative UniFi firewall-decision history
- complete WAN-local processing visibility
- authoritative reconstruction of unsolicited WAN ingress
- application identity when it was not directly observable
- hostname identity merely because an IP address was contacted
- forwarding latency from differences between observation timestamps
- authoritative full packet capture
- payload-level reconstruction after the fact

Observed timestamp differences between interfaces are observation-point deltas, not forwarding-latency measurements.

The sensor can preserve high-fidelity communication context without preserving full payloads. Full PCAP remains a different observation authority.

## Capture Health

Sensor health is part of the observation context.

Kernel capture/socket drops, queue pressure, decoding failures, storage failures, and coverage interruptions must remain visible because they affect what Pathfinder can safely claim about absence or completeness.

```text
no matching observation under degraded coverage
    !=
proof that no activity occurred
```

## UniFi Internal Telemetry

The production sensor does not depend on UniFi's internal RabbitMQ / AMQP telemetry.

Diagnostic research for that interface is maintained separately under:

```text
tools/unifi/mqprobe
```

Failure or absence of that proprietary telemetry therefore does not prevent the core sensor from collecting its supported observations.

## Server Ingestion Direction

The intended direction is:

```text
UDM-Pro sensor
    |
    | authenticated, versioned, idempotent delivery
    v
Pathfinder server
    |
    +--> preserved/indexed sensor observation store
    |
    +--> versioned Sighting derivation
    |
    +--> threat-intelligence correlation
    |
    +--> applicability / Assessment
```

Sensors will not write directly into Pathfinder database tables.

The transport/schema contract will be implemented under the current roadmap.

## Module

```text
github.com/Iron-Signal-Systems/pathfinder/sensors/unifi/udm-pro
```

## Validation

Native development-host validation:

```sh
go test ./...
go vet ./...
```

UDM-Pro target build:

```sh
CGO_ENABLED=0 \
GOOS=linux \
GOARCH=arm64 \
go build \
    -trimpath \
    -o pathfinder-sensor \
    ./cmd/pathfinder-sensor
```

## Pathfinder Boundary

This sensor is an observation source hosted in the Pathfinder repository.

Its presence does not make Pathfinder core a firewall, SIEM, EDR, enforcement system, or authoritative packet-capture platform.

Observations remain attributable to the sensor and its observation points, while later Pathfinder interpretation remains distinct from what the sensor directly observed.
