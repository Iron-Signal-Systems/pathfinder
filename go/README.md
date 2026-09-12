# Pathfinder Go Runtime

Phase 1.1 validated the initial Pathfinder runtime with Go 1.25.

The current command lives at:

```text
cmd/pathfinder/
```

Validate from this directory:

```sh
gofmt -w cmd/pathfinder/*.go
go vet ./...
go test ./...
```

Build the FreeBSD runtime candidate on the supported FreeBSD build environment:

```sh
CGO_ENABLED=0 go build \
    -trimpath \
    -o pathfinder \
    ./cmd/pathfinder
```

The installed runtime appliance does not retain the Go toolchain after the validated binary is installed.

Phase 1 installation automation must own installing any temporary build toolchain, validating the candidate, installing the final artifact, and removing temporary build dependencies from the runtime jail.
