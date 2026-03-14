<p align="center">
  <h1 align="center">Den</h1>
  <p align="center">Self-hosted sandbox runtime for AI agents</p>
  <p align="center">
    <a href="docs/docs/quick-start.md">Getting Started</a> &bull;
    <a href="docs/api-reference.md">API Reference</a> &bull;
    <a href="docs/docs/sdks.md">SDKs</a> &bull;
    <a href="docs/docs/mcp.md">MCP Integration</a> &bull;
    <a href="docs/docs/configuration.md">Configuration</a>
  </p>
  <p align="center">
    <b>English</b> | <a href="README.zh-CN.md">中文</a>
  </p>
</p>

---

Den gives AI agents secure, isolated sandbox environments to execute code. It's the open-source, self-hosted alternative to E2B and similar cloud sandbox services.

**Single binary. Zero config. Works with any AI framework.**

> **100 sandboxes on E2B = ~$600/hour. 100 sandboxes on Den = one $5/month server.**

```
curl -sSL https://get.den.dev | sh
den serve
```

## What's New

### Shared Resource Management (v0.0.6)

- **Memory pressure monitoring** — Real-time 5-level pressure system (Normal → Warning → High → Critical → Emergency) with hysteresis
- **Dynamic memory throttling** — Automatic per-container cgroup v2 `memory.high` adjustment based on host pressure
- **Pressure-aware scheduling** — New sandboxes rejected at Critical/Emergency (HTTP 503)
- **Resource status API** — `GET /api/v1/resources` for host memory, pressure level, and sandbox metrics
- **Platform support** — Linux (direct cgroup v2, `/proc/meminfo`) and macOS (Docker API fallback)
- **Auto-recovery** — Memory limits automatically removed when pressure drops

### Storage Layer (v0.0.5)

- **Persistent & shared volumes** — Docker named volumes, cross-sandbox mounting (RW/RO)
- **S3 integration** — Hooks sync, on-demand import/export, FUSE mount
- **Go, TypeScript (`@us4/den`), Python (`den-sdk`) SDKs** — Full storage type support

See [CHANGELOG.md](CHANGELOG.md) for the full release history.

## Why Den?

AI agents need to run code, but running untrusted code on your machine is dangerous. Den solves this by providing:

- **Isolated containers** — Each sandbox runs in its own Docker container with dropped capabilities, read-only rootfs, PID limits, and resource constraints
- **Shared resource model** — Containers share host memory intelligently instead of fixed allocation. Dynamic pressure monitoring with auto-throttle (Google Borg / AWS Firecracker approach). 10x overcommit = 10x more sandboxes per dollar
- **Simple REST API** — Create sandboxes, execute commands, read/write files, manage snapshots — all via HTTP
- **WebSocket streaming** — Real-time command output for interactive use cases
- **MCP server** — Native Model Context Protocol support for Claude, Cursor, and other AI tools
- **Snapshot/Restore** — Save sandbox state and restore it later for reproducible environments
- **Storage** — Persistent volumes, shared volumes, configurable tmpfs, and S3 integration
- **Go + TypeScript + Python SDKs** — First-class client libraries

## Installation

```bash
# Go
go get github.com/us/den@latest

# TypeScript
bun add @us4/den
# or: npm install @us4/den

# Python
pip install den-sdk
# or: uv add den-sdk
```

## Quick Start

### Prerequisites

- Docker running locally
- Go 1.21+ (to build from source)

### Run the Server

```bash
# Build and run
go build -o den ./cmd/den
./den serve

# Or with custom config
./den serve --config den.yaml
```

### Create a Sandbox and Run Code

```bash
# Create a sandbox
curl -X POST http://localhost:8080/api/v1/sandboxes \
  -H 'Content-Type: application/json' \
  -d '{"image": "ubuntu:22.04"}'
# → {"id":"abc123","status":"running",...}

# Execute a command
curl -X POST http://localhost:8080/api/v1/sandboxes/abc123/exec \
  -H 'Content-Type: application/json' \
  -d '{"cmd": ["python3", "-c", "print(2+2)"]}'
# → {"exit_code":0,"stdout":"4\n","stderr":""}

# Write a file
curl -X PUT 'http://localhost:8080/api/v1/sandboxes/abc123/files?path=/tmp/hello.py' \
  -d 'print("Hello from sandbox!")'

# Read a file
curl 'http://localhost:8080/api/v1/sandboxes/abc123/files?path=/tmp/hello.py'

# Destroy the sandbox
curl -X DELETE http://localhost:8080/api/v1/sandboxes/abc123
```

### Use with Go SDK

```go
package main

import (
    "context"
    "fmt"

    client "github.com/us/den/pkg/client"
)

func main() {
    c := client.New("http://localhost:8080", client.WithAPIKey("your-key"))
    ctx := context.Background()

    // Create sandbox
    sb, _ := c.CreateSandbox(ctx, client.SandboxConfig{
        Image: "ubuntu:22.04",
    })

    // Run code
    result, _ := c.Exec(ctx, sb.ID, client.ExecOpts{
        Cmd: []string{"echo", "Hello from Go SDK!"},
    })
    fmt.Println(result.Stdout)

    // Clean up
    c.DestroySandbox(ctx, sb.ID)
}
```

### Use with MCP (Claude Code, Cursor)

```bash
# Start the MCP server (stdio mode)
den mcp
```

Add to your Claude Code config (`~/.claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "den": {
      "command": "den",
      "args": ["mcp"]
    }
  }
}
```

Now Claude can create sandboxes, run code, and manage files directly.

## Features

| Feature | Description |
|---------|-------------|
| **Sandbox CRUD** | Create, list, get, stop, destroy containers |
| **Command Execution** | Sync exec with exit code, stdout, stderr |
| **Streaming Exec** | WebSocket-based real-time output |
| **File Operations** | Read, write, list, mkdir, delete files inside sandboxes |
| **File Upload/Download** | Multipart upload and direct download |
| **Snapshots** | Save and restore sandbox state via `docker commit` |
| **Persistent Volumes** | Docker named volumes that survive sandbox destruction |
| **Shared Volumes** | Mount the same volume across sandboxes (RW or RO) |
| **Configurable Tmpfs** | Per-sandbox tmpfs size and option overrides |
| **S3 Sync** | Import/export files via hooks, on-demand API, or FUSE mount |
| **Port Forwarding** | Expose sandbox ports to host (bound to 127.0.0.1) |
| **Resource Limits** | CPU, memory, PID limits per sandbox |
| **Pressure Monitoring** | Host memory pressure detection with dynamic throttling |
| **Auto-Expiry** | Sandboxes auto-destroy after configurable timeout |
| **Rate Limiting** | Per-key rate limiting on all API endpoints |
| **API Key Auth** | Header-based authentication with constant-time comparison |
| **MCP Server** | stdio-based Model Context Protocol for AI tool integration |
| **Dashboard** | Embedded web UI for monitoring and management |

## Security

Den takes security seriously. Every sandbox runs with:

- **Dropped capabilities** — `ALL` capabilities dropped, minimal set added back
- **Read-only root filesystem** — Only tmpfs mounts and explicit volumes are writable
- **PID limits** — Default 256 processes per container
- **No new privileges** — `no-new-privileges` security option
- **Network isolation** — Containers on internal Docker network
- **Port binding** — Forwarded ports bind to `127.0.0.1` only
- **Path validation** — Null byte and traversal protection on all file operations
- **Dynamic memory throttling** — cgroup v2 `memory.high` based throttling instead of hard kills; 5-level pressure system with auto-recovery
- **Constant-time auth** — API key comparison resistant to timing attacks
- **No error leaking** — Internal errors are logged, generic messages returned to clients

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                    Clients                           │
│  CLI  │  Go SDK  │  TS SDK  │  Python SDK  │  MCP   │
└───────┴──────────┴──────────┴──────────────┴────────┘
                          │
                    ┌─────┴─────┐
                    │  HTTP API  │  chi router + middleware
                    │  WebSocket │  gorilla/websocket
                    └─────┬─────┘
                          │
                    ┌─────┴─────┐
                    │  Engine   │  Lifecycle, reaper, pressure
                    └──┬────┬──┘
                       │    │
          ┌────────────┘    └────────────┐
  ┌───────┴───────┐           ┌──────────┴─────────┐
  │ Docker Runtime│           │  Storage Layer     │
  │  Docker SDK   │           │  Volumes, S3, Tmpfs│
  └───────┬───────┘           └──────────┬─────────┘
          │                              │
  ┌───────┴───────┐           ┌──────────┴─────────┐
  │   Containers  │           │  S3 / MinIO        │
  │  (sandboxes)  │           │  Docker Volumes    │
  └───────────────┘           └────────────────────┘
```

## Performance

Benchmarked on Apple Silicon (M-series):

| Operation | Latency | Notes |
|-----------|---------|-------|
| API health check | < 1ms | Near-zero overhead |
| Create sandbox | ~100ms | Cold start; warm pool brings this to ~5ms |
| Execute command | ~20-30ms | Including Docker exec round-trip |
| Read file | ~28-30ms | Exec-based file I/O |
| Write file | ~56-70ms | Exec-based with auto-mkdir |
| Destroy sandbox | ~1s | SIGTERM + cleanup |
| Parallel create (5x) | ~42ms/each | Concurrent container creation |
| Parallel exec (10x) | ~7ms/each | Concurrent command execution |

### vs. Alternatives

| | **Den** | E2B | Daytona | Modal |
|---|---|---|---|---|
| Sandbox create | **~100ms** | ~150ms | ~90ms | 2-5s |
| Pricing | **Free** | $0.10/min+ | Free (complex) | $0.10/min+ |
| Max sandboxes/server | **100+ (shared resources)** | ~10 (dedicated) | ~10 (K8s pods) | N/A (cloud) |
| Setup | **`curl \| sh`** | SDK + API key | Docker + K8s | SDK + API key |
| Self-hosted | **Easy (single binary)** | Hard (Firecracker+Nomad) | Heavy (K8s) | No |
| Offline | **Yes** | No | Partial | No |
| License | AGPL-3.0 | Apache-2.0 | Apache-2.0 | Proprietary |

## Documentation

- [Getting Started](docs/docs/quick-start.md) — Installation, first sandbox, basic usage
- [API Reference](docs/api-reference.md) — Complete REST API documentation
- [Configuration](docs/docs/configuration.md) — All config options explained
- [SDK Guide](docs/docs/sdks.md) — Go, TypeScript, and Python client libraries
- [MCP Integration](docs/docs/mcp.md) — Using Den with AI tools
- [Architecture](docs/docs/architecture.md) — Internal design and security model
- [CLI Reference](docs/cli.md) — Command-line interface

## CLI

```
den serve                         # Start API server
den create --image ubuntu:22.04   # Create sandbox
den ls                            # List sandboxes
den exec <id> -- echo hello       # Execute command
den rm <id>                       # Destroy sandbox
den snapshot create <id>          # Create snapshot
den snapshot restore <snap-id>    # Restore snapshot
den stats                         # System stats
den mcp                           # Start MCP server
den version                       # Version info
```

## Configuration

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  rate_limit_rps: 10
  rate_limit_burst: 20

sandbox:
  default_image: "ubuntu:22.04"
  default_timeout: "30m"
  max_sandboxes: 50
  default_memory: 536870912  # 512MB
  allow_volumes: true
  allow_s3: true
  max_volumes_per_sandbox: 5

s3:
  endpoint: "http://localhost:9000"  # MinIO or S3-compatible
  region: "us-east-1"
  access_key: "minioadmin"
  secret_key: "minioadmin"

auth:
  enabled: true
  api_keys:
    - "your-secret-key"

resource:
  overcommit_ratio: 10.0
  monitor_interval: "5s"
  enable_auto_throttle: true
```

See [Configuration Guide](docs/docs/configuration.md) for all options.

## Contributing

```bash
# Clone and build
git clone https://github.com/us/den
cd den
go build ./cmd/den

# Run tests
go test ./internal/... -race

# Run with race detector
go test ./internal/... -count=1 -race -v
```

## License

AGPL-3.0 — See [LICENSE](LICENSE) for details.
