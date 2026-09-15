# UniFi MQ Probe

`mqprobe` is a Pathfinder engineering and diagnostic utility for controlled
investigation of UniFi's local AMQP flow-event interface.

It is not required by the Pathfinder UDM-Pro Sensor and is not part of the
sensor's observation authority.

## Purpose

The utility exists to support investigation of UniFi internal telemetry while
keeping proprietary appliance plumbing outside the production sensor
dependency chain.

It should be treated as research and diagnostic tooling, not as an
authoritative packet, firewall-decision, or path-observation source.

## Module

```text
github.com/Iron-Signal-Systems/pathfinder/tools/unifi/mqprobe
```

## Validation

```sh
go test ./...
go vet ./...
```

Linux ARM64 build:

```sh
CGO_ENABLED=0 \
GOOS=linux \
GOARCH=arm64 \
go build \
    -trimpath \
    -o pathfinder-mqprobe \
    ./cmd/mqprobe
```
