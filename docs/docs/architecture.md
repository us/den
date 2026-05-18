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

State machine: `creating → running → stopped / error`. Exec, ReadFile, WriteFile require `running` status.

### 3. Docker Runtime (`internal/runtime/docker/`)

Uses the official Docker SDK. Implements the `Runtime` interface:

- **docker.go** — Container lifecycle (create, start, stop, remove, inspect)
- **exec.go** — Execution via `ContainerExecCreate` + `ContainerExecAttach`
- **fs.go** — File I/O via exec-based approach (`cat` for read, `tee` for write)
- **snapshot.go** — Snapshots via `docker commit`
- **docker.go (`EnsureNetwork`/`Reconcile`)** — Mode-aware managed network (`internal`/`bridge`/`none`)

Port mappings are applied **at container-create time** via Docker-native
`HostConfig.PortBindings` (bound to `127.0.0.1`) and are published **only in
`network_mode=bridge`** — they are inert in `internal` and rejected in `none`.
There is no userspace port-forwarder and no runtime add/remove: `POST`/`DELETE
/api/v1/sandboxes/{id}/ports` permanently return `501`. **Docker-out-of-Docker
(DooD): when den's Docker client points at a remote or socket-proxied daemon,
`127.0.0.1` host bindings land on the *daemon* host, not the den host — DooD
port access is unsupported.** The legacy in-process `PortForwarder`
(`network.go`) was removed in v9.

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

### 6. Resource Management (`internal/engine/`)

Manages host memory pressure and dynamic container throttling.

**Components:**

- **PressureMonitor** — Observer pattern with 5-level pressure system and hysteresis (2 consecutive readings required to change level). Runs a sampling goroutine at configurable intervals (default 5s).
- **MemoryBackend** — Platform abstraction interface:
  - **Linux**: `/proc/meminfo` for host memory, direct cgroup v2 file reads for container limits
  - **macOS**: `sysctl` for host memory, Docker API for container stats (development fallback)
- **CgroupManager** — Direct cgroup v2 `memory.high` writes for sub-millisecond limit application. Falls back to Docker API `UpdateContainerResources` when direct access unavailable.

**Auto-throttle flow:**

```
Host pressure rises → PressureMonitor detects level change (with hysteresis)
  → Calculate per-container memory.high = (available memory) / (active containers)
  → Apply floor (min 32MB per container)
  → Write memory.high to each container's cgroup
  → Containers slow down (throttled, not killed)
  → Host pressure drops → Remove memory.high limits (write "max")
```

**Pressure levels and actions:**

| Level | Memory Usage | Action |
|-------|-------------|--------|
| Normal | < 80% | No limits applied |
| Warning | 80-85% | Log warning |
| High | 85-90% | Apply `memory.high` to containers |
| Critical | 90-95% | Aggressive throttle + reject new sandboxes (503) |
| Emergency | > 95% | Maximum throttle + reject new sandboxes (503) |

### 7. MCP Server (`internal/mcp/`)

JSON-RPC 2.0 over stdio. Creates its own Engine + Docker Runtime instance (does not use HTTP API). Binary file content auto-base64-encoded. Logs to stderr.

## Container Isolation

| Control | Setting |
|---------|---------|
| Capabilities | `ALL` dropped, `NET_BIND_SERVICE` added |
| Root filesystem | Read-only (`ReadonlyRootfs: true`) |
| Privileges | `no-new-privileges` security option |
| PID limit | Default 256 (prevents fork bombs) |
| Memory limit | Default 512MB (soft throttle via `memory.high`; OOM kill only at `memory.max`) |
| CPU limit | Default 1 core |
| Network | Managed `den-net`: `internal` (default, no egress) / `bridge` (egress + published ports) / `none` (no interface). **Only `none` is a tenant boundary** |
| Port binding | Docker-native, `127.0.0.1` only, fixed at creation, **published only in `bridge`** (`POST`/`DELETE /ports` → `501`) |
| Path validation | Null byte rejection, traversal protection |

## Concurrency Model

- **sync.Map** — Thread-safe sandbox storage (lock-free reads)
- **sync.RWMutex** — Per-sandbox status access
- **sync.Mutex** — Sandbox counter for limit enforcement
- **sync.Once** — Shutdown guard (prevents double-close); also used for `PressureMonitor.Start()` (double-start protection)
- **doneCh (chan struct{})** — `PressureMonitor.Stop()` blocks until goroutine finishes via `doneCh` (prevents send-on-closed-channel panic)
- **Goroutines** — Reaper, rate limiter cleanup (5 min), PressureMonitor sampler, ExecStream readers, WebSocket handlers

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
