# Pathfinder UDM-Pro Sensor

The Pathfinder UDM-Pro Sensor observes network activity visible from a UniFi
UDM-Pro and preserves those observations for later Pathfinder analysis.

The sensor is intentionally conservative about what it claims. Direct packet
observations, DNS observations, TLS observations, endpoint identity,
connection-tracking state, and derived relationships remain distinct.

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
- historical queries

The currently validated UDM-Pro observation points are:

```text
br0
    LAN-side observation

eth8
    WAN-side observation
```

Interface names are deployment-specific and must remain explicitly
configured.

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

A DNS name, SNI value, endpoint identity, or NAT relationship does not replace
the underlying packet observation.

Missing information is not manufactured.

## UDM-Pro Limitations

The sensor does not claim:

- authoritative UniFi firewall-decision history
- complete WAN-local processing visibility
- authoritative reconstruction of unsolicited WAN ingress
- application identity when it was not directly observable
- hostname identity merely because an IP address was contacted
- forwarding latency from differences between observation timestamps

Observed timestamp differences between interfaces are observation-point
deltas, not forwarding-latency measurements.

## UniFi Internal Telemetry

The production sensor does not depend on UniFi's internal RabbitMQ / AMQP
telemetry.

Diagnostic research for that interface is maintained separately under:

```text
tools/unifi/mqprobe
```

Failure or absence of that proprietary telemetry therefore does not prevent
the core sensor from collecting its supported observations.

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

Its presence does not make Pathfinder core a firewall, enforcement system, or
authoritative packet-capture platform. Observations remain attributable to the
sensor and its observation points, while later Pathfinder interpretation
remains distinct from what the sensor directly observed.
