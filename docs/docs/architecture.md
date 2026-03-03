# Architecture

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

HTTP server using chi router. Requests flow through a middleware stack:

```
Request → RequestID → RealIP → Logger → Recoverer → RateLimit → CORS → Auth → Handler
```

- **RequestID** — Unique ID per request for tracing
- **RealIP** — Client IP from proxy headers
- **Logger** — Structured logging via `slog`
- **Recoverer** — Catches panics, returns 500
- **RateLimit** — Per-key token bucket (`golang.org/x/time/rate`)
- **CORS** — Configurable allowed origins
- **Auth** — API key with constant-time comparison (`crypto/subtle` + SHA-256 pre-hash)

Handlers organized by domain: `sandbox.go`, `exec.go`, `fs.go`, `snapshot.go`, `port.go`, `stats.go`, `s3.go`. WebSocket streaming in `ws/exec.go`.

### 2. Engine (`internal/engine/`)

Orchestrates sandbox lifecycle and enforces business rules:

- **Tracking** — `sync.Map` for thread-safe concurrent access (lock-free reads)
- **IDs** — Short, sortable IDs via xid
- **Limits** — Max sandboxes cap with atomic counter + mutex
- **Reaper** — Goroutine runs every 30s, destroys expired sandboxes
- **Persistence** — Saves sandbox metadata to BoltDB on create/update
- **Recovery** — On startup, loads from BoltDB, validates against Docker state, removes stale records
- **Storage** — S3 hook init/cleanup, volume validation, tmpfs configuration

Status machine: `creating → running → stopped / error`. Exec, ReadFile, WriteFile require `running` status.

### 3. Docker Runtime (`internal/runtime/docker/`)

Uses the official Docker SDK. Implements the `Runtime` interface:

- **docker.go** — Container lifecycle (create, start, stop, remove, inspect)
- **exec.go** — Execution via `ContainerExecCreate` + `ContainerExecAttach`
- **fs.go** — File I/O via exec-based approach (`cat` for read, `tee` for write)
- **snapshot.go** — Snapshots via `docker commit`
- **network.go** — Port forwarding via Docker port bindings

File I/O uses exec-based approach instead of `CopyToContainer`/`CopyFromContainer` because the latter fails with read-only rootfs. Exec works through the container's mount namespace and handles tmpfs correctly.

### 4. Storage Layer (`internal/storage/`)

| Type | Description |
|------|-------------|
| **tmpfs** | Writable in-memory mounts (`/tmp`, `/home/sandbox`, `/run`, `/var/tmp`) |
| **Named volumes** | Docker managed, persistent, namespaced with `den-` prefix |
| **Shared volumes** | Same volume mountable on multiple sandboxes |
| **S3 hooks** | Auto-download on create, auto-upload on destroy |
| **S3 on-demand** | Manual import/export via REST API |
| **S3 FUSE** | s3fs mount inside container (requires `SYS_ADMIN` + `/dev/fuse`) |

Security: path traversal protection, object size limits (100MB), SSRF protection on endpoints, credential cleanup via `defer`.

### 5. Store Layer (`internal/store/`)

BoltDB embedded key-value persistence:

- **Sandboxes bucket** — Metadata (ID, image, status, timestamps)
- **Snapshots bucket** — Metadata (ID, sandbox ID, name, image ID)

Used for crash recovery only. Engine validates store records against Docker state on startup.

### 6. MCP Server (`internal/mcp/`)

JSON-RPC 2.0 over stdio. Creates its own Engine + Docker Runtime instance (does not use HTTP API). Binary file content auto-base64-encoded. Logs to stderr.

## Container Isolation

| Control | Setting |
|---------|---------|
| Capabilities | `ALL` dropped, `NET_BIND_SERVICE` added |
| Root filesystem | Read-only (`ReadonlyRootfs: true`) |
| Privileges | `no-new-privileges` security option |
| PID limit | Default 256 (prevents fork bombs) |
| Memory limit | Default 512MB (OOM kill on exceed) |
| CPU limit | Default 1 core |
| Network | Internal Docker network (`den-net`) |
| Port binding | `127.0.0.1` only |
| Path validation | Null byte rejection, traversal protection |

## Concurrency Model

- **sync.Map** — Thread-safe sandbox storage (lock-free reads)
- **sync.RWMutex** — Per-sandbox status access
- **sync.Mutex** — Sandbox counter for limit enforcement
- **sync.Once** — Shutdown guard (prevents double-close)
- **Goroutines** — Reaper, rate limiter cleanup (5 min), ExecStream readers, WebSocket handlers

## Performance

Benchmarked on Apple Silicon (M-series):

| Operation | Latency |
|-----------|---------|
| Health check | < 1ms |
| Create sandbox | ~100-160ms |
| Execute command | ~20-30ms |
| Read file | ~28-30ms |
| Write file | ~56-70ms |
| Concurrent throughput | ~66 req/s |

Primary bottleneck is Docker API overhead. Exec-based file I/O adds ~20ms vs direct filesystem access.

## Persistence & Recovery

1. **Normal** — Metadata saved to BoltDB on create, updated on status change
2. **Graceful shutdown** — Destroys all running sandboxes, closes store
3. **Crash recovery** — Loads from BoltDB, validates against Docker, removes stale records
4. **Snapshots** — Docker images with `den.snapshot` label, metadata in BoltDB
