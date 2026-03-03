---
title: Architecture
---

# Architecture

This document describes the internal design, data flow, and security model of Den.

## System Overview

```
┌──────────────────────────────────────────────────────┐
│                      Clients                          │
│  CLI  │  Go SDK  │  TS SDK  │  Python SDK  │  MCP    │
└───────┴──────────┴──────────┴──────────────┴─────────┘
                          │
                    ┌─────┴─────┐
                    │  HTTP API  │  chi router + middleware
                    │  WebSocket │  gorilla/websocket
                    └─────┬─────┘
                          │
                    ┌─────┴─────┐
                    │  Engine   │  Lifecycle, reaper, limits
                    └─────┬─────┘
                          │
                ┌─────────┴─────────┐
                │  Docker Runtime   │  Docker SDK
                └─────────┬─────────┘
                          │
              ┌───────────┴───────────┐
              │  Docker Containers    │
              │  (isolated sandboxes) │
              └───────────────────────┘
```

## Layers

### 1. API Layer (`internal/api/`)

The HTTP server uses [chi](https://github.com/go-chi/chi) as the router. Requests flow through a middleware stack before reaching handlers:

```
Request → RequestID → RealIP → Logger → Recoverer → RateLimit → CORS → Auth → Handler
```

**Middleware:**
- **RequestID** — Assigns a unique ID to each request for tracing
- **RealIP** — Extracts client IP from proxy headers
- **Logger** — Structured request/response logging via `slog`
- **Recoverer** — Catches panics, returns 500 instead of crashing
- **RateLimit** — Per-key token bucket rate limiting (`golang.org/x/time/rate`)
- **CORS** — Cross-Origin Resource Sharing headers
- **Auth** — API key validation with constant-time comparison

**Handlers** are organized by domain:
- `handlers/sandbox.go` — CRUD operations
- `handlers/exec.go` — Command execution
- `handlers/fs.go` — File operations
- `handlers/snapshot.go` — Snapshot management
- `handlers/port.go` — Port forwarding
- `handlers/stats.go` — Resource statistics

**WebSocket** streaming is handled separately in `ws/exec.go` for real-time command output.

### 2. Engine Layer (`internal/engine/`)

The Engine orchestrates sandbox lifecycle and enforces business rules:

- **Sandbox tracking** — `sync.Map` for thread-safe concurrent access
- **ID generation** — Short, sortable IDs via [xid](https://github.com/rs/xid)
- **Limit enforcement** — Max sandboxes cap with atomic counter
- **Auto-expiry** — Reaper goroutine runs every 30 seconds, destroys expired sandboxes
- **State persistence** — Saves sandbox metadata to BoltDB store
- **Startup recovery** — Restores sandboxes from store on restart, validates against Docker state

**Status machine:**
```
creating → running → stopped
                  → error
```

Operations like `Exec`, `ReadFile`, `WriteFile` are gated by status — they require `running` status and return `ErrNotRunning` otherwise.

### 3. Runtime Layer (`internal/runtime/`)

The Runtime interface abstracts the container backend:

```go
type Runtime interface {
    Ping(ctx context.Context) error
    Create(ctx context.Context, config SandboxConfig) (string, error)
    Start(ctx context.Context, id string) error
    Stop(ctx context.Context, id string) error
    Remove(ctx context.Context, id string) error
    Info(ctx context.Context, id string) (*SandboxInfo, error)
    List(ctx context.Context) ([]SandboxInfo, error)
    Exec(ctx context.Context, id string, opts ExecOpts) (*ExecResult, error)
    ExecStream(ctx context.Context, id string, opts ExecOpts) (ExecStream, error)
    ReadFile(ctx context.Context, id string, path string) ([]byte, error)
    WriteFile(ctx context.Context, id string, path string, content []byte) error
    ListDir(ctx context.Context, id string, path string) ([]FileInfo, error)
    MkDir(ctx context.Context, id string, path string) error
    RemoveFile(ctx context.Context, id string, path string) error
    Snapshot(ctx context.Context, id string) (*SnapshotInfo, error)
    Restore(ctx context.Context, imageID string) (string, error)
    ListSnapshots(ctx context.Context) ([]SnapshotInfo, error)
    RemoveSnapshot(ctx context.Context, imageID string) error
    Stats(ctx context.Context, id string) (*SandboxStats, error)
}
```

Currently only Docker is implemented (`runtime/docker/`), but the interface allows adding other backends (e.g., Firecracker, gVisor).

### 4. Docker Runtime (`internal/runtime/docker/`)

Uses the official Docker SDK (`github.com/docker/docker/client`):

- **docker.go** — Container lifecycle (create, start, stop, remove, inspect)
- **exec.go** — Command execution via `ContainerExecCreate` + `ContainerExecAttach`
- **fs.go** — File I/O via exec-based approach (`cat` for read, `tee` for write)
- **snapshot.go** — Snapshots via `docker commit`
- **network.go** — Port forwarding via Docker port bindings

**Why exec-based file I/O?** Docker's `CopyToContainer`/`CopyFromContainer` operates at the overlay filesystem level, which fails with read-only root filesystem (`ReadonlyRootfs: true`). The exec-based approach works through the container's mount namespace and correctly handles tmpfs mounts.

### 5. Store Layer (`internal/store/`)

BoltDB provides embedded key-value persistence:

- **Sandboxes bucket** — Sandbox metadata (ID, image, status, timestamps)
- **Snapshots bucket** — Snapshot metadata (ID, sandbox ID, name, image ID)

The store is used for crash recovery. On startup, the engine loads sandboxes from the store and validates each against Docker state, removing stale records.

### 6. MCP Server (`internal/mcp/`)

Implements JSON-RPC 2.0 over stdio for Model Context Protocol:

- Creates its own Engine + Docker Runtime (does not use the HTTP API)
- Exposes sandbox operations as MCP tools
- Binary file content automatically base64-encoded
- Logs to stderr (stdout reserved for protocol)

## Data Flow

### Create Sandbox

```
Client → POST /api/v1/sandboxes
  → Auth middleware validates API key
  → Handler parses request body
  → Engine.CreateSandbox()
    → Check sandbox limit
    → Generate xid
    → Runtime.Create() → Docker ContainerCreate
      → Security config (caps, rootfs, limits)
      → Network config (den-net)
      → Port bindings
    → Runtime.Start() → Docker ContainerStart
    → Store.SaveSandbox() → BoltDB
    → Track in sync.Map
  ← Return sandbox JSON
```

### Execute Command

```
Client → POST /api/v1/sandboxes/{id}/exec
  → Handler parses cmd, timeout
  → Engine.Exec()
    → Engine.getRunning() → status check
    → Runtime.Exec()
      → Docker ContainerExecCreate (cmd, env, workdir)
      → Docker ContainerExecAttach
      → stdcopy.StdCopy → separate stdout/stderr
      → ContainerExecInspect → exit code
  ← Return {exit_code, stdout, stderr}
```

### WebSocket Streaming

```
Client → WS /api/v1/sandboxes/{id}/exec/stream
  → Origin check
  → Upgrade to WebSocket
  → Read JSON message (cmd, timeout)
  → Engine.ExecStream()
    → Runtime.ExecStream()
      → Docker ContainerExecCreate
      → Docker ContainerExecAttach
      → io.Pipe for stdout/stderr separation
      → Goroutines push to channel
  → Stream messages: {type, data}
  → Send exit message
  → Close connection
```

## Concurrency Model

- **sync.Map** — Thread-safe sandbox storage (lock-free reads)
- **sync.RWMutex** — Per-sandbox status access
- **sync.Mutex** — Sandbox counter for limit enforcement
- **sync.Once** — Engine shutdown (prevents double-close panic)
- **Goroutines:**
  - Reaper: periodic expired sandbox cleanup
  - Rate limiter cleanup: removes stale limiters every 5 minutes
  - ExecStream: per-stream goroutines for stdout/stderr reading
  - WebSocket: per-connection handler goroutine

## Security Model

### Container Isolation

Every sandbox container runs with hardened security:

| Control | Setting |
|---------|---------|
| Capabilities | `ALL` dropped, `NET_BIND_SERVICE` added |
| Root filesystem | Read-only (`ReadonlyRootfs: true`) |
| Writable mounts | tmpfs: `/tmp`, `/home/sandbox`, `/run`, `/var/tmp` |
| Privileges | `no-new-privileges` security option |
| PID limit | Default 256 (prevents fork bombs) |
| Memory limit | Default 512MB (OOM kill on exceed) |
| CPU limit | Default 1 core |
| Network | Internal Docker network (`den-net`) |
| Port binding | Forwarded ports bind to `127.0.0.1` only |

### API Security

| Control | Implementation |
|---------|---------------|
| Authentication | API key via `X-API-Key` header |
| Key comparison | Constant-time (`crypto/subtle` + SHA-256 pre-hash) |
| Rate limiting | Per-key token bucket (configurable RPS + burst) |
| Body limits | 1MB for JSON payloads, 100MB for file uploads |
| Path validation | Null byte rejection, traversal protection |
| Error handling | Generic messages to clients, details in server logs |
| CORS | Configurable allowed origins |
| WebSocket origins | Config-based origin checking |

### Network Security

- Containers communicate via internal Docker network
- Container-to-container traffic is possible within `den-net` but sandboxes cannot reach the host network
- Forwarded ports bind to `127.0.0.1` only (not exposed externally)
- Outbound internet access depends on Docker network configuration

## Performance Characteristics

Benchmarked on Apple Silicon (M-series):

| Operation | Latency |
|-----------|---------|
| API health check | < 1ms |
| Create sandbox | ~100-160ms |
| Execute command | ~20-30ms |
| Read file | ~28-30ms |
| Write file | ~56-70ms |
| Concurrent throughput | ~66 req/s |

The primary bottleneck is Docker API overhead for container operations. Exec-based file I/O adds ~20ms compared to direct filesystem access but is necessary for read-only rootfs compatibility.

## Persistence & Recovery

1. **Normal operation:** Sandbox metadata saved to BoltDB on create, updated on status change
2. **Graceful shutdown:** Engine destroys all running sandboxes, closes store
3. **Crash recovery:** On startup, engine loads sandboxes from BoltDB, validates each against Docker:
   - If container exists in Docker: restore sandbox with Docker's reported status
   - If container is gone: delete stale record from store
4. **Snapshots:** Stored as Docker images with `den.snapshot` label, metadata in BoltDB
