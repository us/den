# Installation

## Requirements

- Docker running locally (accessible via Docker socket)
- Go 1.21+ (for building from source)

## Build from Source

```bash
git clone https://github.com/getden/den.git
cd den
go build -o den ./cmd/den
./den serve
```

Or use the Makefile:

```bash
make build    # CGO_ENABLED=0 go build -o bin/den ./cmd/den
make run      # ./bin/den serve
```

## Docker

```bash
docker build -t den/den:latest .
```

Build the default sandbox image:

```bash
docker build -t den/default:latest images/default/
```

## Verify

```bash
den version
# den v0.1.0 (commit: abc1234, built: 2026-03-03)
```
