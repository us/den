---
title: Configuration
---

# Configuration

Den is configured via YAML file, environment variables, or CLI flags. Values are merged in this order (later overrides earlier):

1. Built-in defaults
2. YAML config file
3. Environment variables

## Config File

Pass a config file with the `--config` flag:

```bash
den serve --config den.yaml
```

## Full Configuration Reference

```yaml
# den.yaml — all options with defaults

server:
  host: "0.0.0.0"            # Listen address
  port: 8080                  # Listen port
  allowed_origins:            # CORS and WebSocket origins
    - "http://localhost:8080"
    - "http://127.0.0.1:8080"
  rate_limit_rps: 10          # Requests per second per key/IP
  rate_limit_burst: 20        # Burst allowance
  tls:
    cert_file: ""             # TLS certificate path
    key_file: ""              # TLS private key path

runtime:
  backend: "docker"           # Runtime backend (only "docker" supported)
  docker_host: ""             # Docker socket (empty = default)
  network_id: ""              # Custom Docker network (empty = auto-create)

sandbox:
  default_image: "ubuntu:22.04"  # Default container image
  default_timeout: "30m"         # Default sandbox lifetime
  max_sandboxes: 50              # Maximum concurrent sandboxes
  default_cpu: 1000000000        # CPU limit in NanoCPUs (1 core)
  default_memory: 536870912      # Memory limit in bytes (512MB)
  default_pid_limit: 256         # Max processes per sandbox
  warm_pool_size: 0              # Pre-created standby sandboxes
  allow_volumes: true            # Enable Docker named volumes
  allow_shared_volumes: true     # Allow same volume on multiple sandboxes
  allow_s3: true                 # Enable S3 sync features
  allow_s3_fuse: false           # Enable S3 FUSE mounts (requires SYS_ADMIN)
  allow_host_binds: false        # Allow host directory bind mounts (DANGEROUS)
  max_volumes_per_sandbox: 5     # Max volume mounts per sandbox
  default_tmpfs:                 # Default tmpfs mounts (overridable per-sandbox)
    - path: "/tmp"
      size: "256m"
    - path: "/home/sandbox"
      size: "512m"
    - path: "/run"
      size: "64m"
    - path: "/var/tmp"
      size: "128m"

s3:
  endpoint: ""                   # S3-compatible endpoint (e.g. MinIO)
  region: "us-east-1"            # AWS region
  access_key: ""                 # Default access key (per-sandbox override possible)
  secret_key: ""                 # Default secret key (per-sandbox override possible)

store:
  path: "den.db"       # BoltDB database file path

auth:
  enabled: false              # Enable API key authentication
  api_keys:                   # List of valid API keys
    - "your-secret-key-here"

log:
  level: "info"               # Log level: debug, info, warn, error
  format: "text"              # Log format: text, json
```

## Environment Variables

Every config option can be set via environment variable. Use the prefix `DEN_` with `__` (double underscore) as the nesting separator:

| Config Path | Environment Variable |
|-------------|---------------------|
| `server.host` | `DEN_SERVER__HOST` |
| `server.port` | `DEN_SERVER__PORT` |
| `server.rate_limit_rps` | `DEN_SERVER__RATE_LIMIT_RPS` |
| `server.rate_limit_burst` | `DEN_SERVER__RATE_LIMIT_BURST` |
| `sandbox.default_image` | `DEN_SANDBOX__DEFAULT_IMAGE` |
| `sandbox.default_timeout` | `DEN_SANDBOX__DEFAULT_TIMEOUT` |
| `sandbox.max_sandboxes` | `DEN_SANDBOX__MAX_SANDBOXES` |
| `sandbox.default_memory` | `DEN_SANDBOX__DEFAULT_MEMORY` |
| `sandbox.default_cpu` | `DEN_SANDBOX__DEFAULT_CPU` |
| `sandbox.default_pid_limit` | `DEN_SANDBOX__DEFAULT_PID_LIMIT` |
| `sandbox.allow_volumes` | `DEN_SANDBOX__ALLOW_VOLUMES` |
| `sandbox.allow_shared_volumes` | `DEN_SANDBOX__ALLOW_SHARED_VOLUMES` |
| `sandbox.allow_s3` | `DEN_SANDBOX__ALLOW_S3` |
| `sandbox.allow_s3_fuse` | `DEN_SANDBOX__ALLOW_S3_FUSE` |
| `sandbox.max_volumes_per_sandbox` | `DEN_SANDBOX__MAX_VOLUMES_PER_SANDBOX` |
| `s3.endpoint` | `DEN_S3__ENDPOINT` |
| `s3.region` | `DEN_S3__REGION` |
| `s3.access_key` | `DEN_S3__ACCESS_KEY` |
| `s3.secret_key` | `DEN_S3__SECRET_KEY` |
| `auth.enabled` | `DEN_AUTH__ENABLED` |
| `store.path` | `DEN_STORE__PATH` |
| `log.level` | `DEN_LOG__LEVEL` |
| `log.format` | `DEN_LOG__FORMAT` |

Example:

```bash
DEN_SERVER__PORT=9090 \
DEN_SANDBOX__MAX_SANDBOXES=100 \
DEN_AUTH__ENABLED=true \
  den serve
```

## Option Details

### Server

#### `server.host`
IP address to bind to. Use `0.0.0.0` to listen on all interfaces, `127.0.0.1` for local only.

#### `server.port`
TCP port for the HTTP API and dashboard. Default `8080`.

#### `server.allowed_origins`
List of origins allowed for CORS requests and WebSocket connections. Used for browser-based dashboard access.

#### `server.rate_limit_rps`
Maximum sustained requests per second per API key (or per client IP when auth is disabled). Set to `0` to disable.

#### `server.rate_limit_burst`
Maximum burst of requests allowed above the sustained rate. Allows brief spikes in traffic.

#### `server.tls`
Enable HTTPS by providing certificate and key file paths. When set, the server listens on HTTPS only.

### Sandbox

#### `sandbox.default_image`
Docker image used when no `image` is specified in the create request. Must be pre-pulled or available in configured registries.

#### `sandbox.default_timeout`
How long a sandbox lives before automatic destruction. Accepts Go duration format: `30m`, `1h`, `24h`. Each sandbox's timeout starts from creation time.

#### `sandbox.max_sandboxes`
Hard limit on concurrent sandboxes. Returns `503 Service Unavailable` when exceeded.

#### `sandbox.default_cpu`
CPU limit in Docker NanoCPUs. `1000000000` = 1 CPU core. `500000000` = 0.5 cores.

#### `sandbox.default_memory`
Memory limit in bytes. `536870912` = 512MB. `1073741824` = 1GB. The container is OOM-killed if it exceeds this.

#### `sandbox.default_pid_limit`
Maximum number of processes inside the container. Prevents fork bombs. Default `256`.

### Storage

#### `sandbox.allow_volumes`
When `true`, sandboxes can request Docker named volume mounts. Volumes are namespaced with a `den-` prefix. Default `true`.

#### `sandbox.allow_shared_volumes`
When `true`, the same named volume can be mounted by multiple sandboxes simultaneously. When `false`, a volume already in use by another sandbox will be rejected. Default `true`.

#### `sandbox.allow_s3`
When `true`, sandboxes can use S3 sync features (hooks mode and on-demand API). Default `true`.

#### `sandbox.allow_s3_fuse`
When `true`, sandboxes can use S3 FUSE mounts. This grants `SYS_ADMIN` capability and `/dev/fuse` device access to the container — a significant security escalation. Default `false`.

#### `sandbox.allow_host_binds`
When `true`, host directory bind mounts are allowed. **Never enable in production** — this gives sandboxes access to the host filesystem. Default `false`.

#### `sandbox.max_volumes_per_sandbox`
Maximum number of volume mounts allowed per sandbox. Default `5`.

#### `sandbox.default_tmpfs`
Default tmpfs mounts applied to every sandbox. Each entry has `path` and `size`. Per-sandbox overrides can change sizes or add new mount points.

Size format: `256m`, `1g`, `512k`. Maximum: `4g`.

### S3

Server-wide S3 defaults. Per-sandbox configs can override these values.

#### `s3.endpoint`
S3-compatible endpoint URL. Required for MinIO, LocalStack, or other S3-compatible services. Leave empty for AWS S3.

#### `s3.region`
AWS region. Default `us-east-1`.

#### `s3.access_key` / `s3.secret_key`
Default credentials used when per-sandbox credentials are not provided. The secret key is masked in logs.

### Authentication

#### `auth.enabled`
When `true`, all API requests must include a valid `X-API-Key` header. Keys are compared using constant-time comparison to prevent timing attacks.

#### `auth.api_keys`
List of valid API keys. Use strong, randomly generated keys in production:

```bash
# Generate a secure key
openssl rand -hex 32
```

### Store

#### `store.path`
Path to the BoltDB database file. Stores sandbox metadata and snapshot records. Created automatically if it doesn't exist.

### Logging

#### `log.level`
Minimum log level. Options: `debug`, `info`, `warn`, `error`.

- `debug` — Verbose, includes request/response details
- `info` — Standard operations (default)
- `warn` — Potential issues
- `error` — Failures only

#### `log.format`
Log output format:
- `text` — Human-readable, colored output (default, good for development)
- `json` — Structured JSON lines (good for log aggregation)

## Validation

Config is validated on startup. The server refuses to start with invalid configuration:

- `server.port` must be between 1 and 65535
- `sandbox.max_sandboxes` must be positive
- `sandbox.default_memory` must be at least 4MB
- `sandbox.default_timeout` must be a valid Go duration

## Example Configs

### Development (minimal)

```yaml
log:
  level: debug
```

### Production

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  rate_limit_rps: 20
  rate_limit_burst: 40
  tls:
    cert_file: /etc/ssl/certs/den.pem
    key_file: /etc/ssl/private/den.key

sandbox:
  default_timeout: "1h"
  max_sandboxes: 100
  default_memory: 1073741824

auth:
  enabled: true
  api_keys:
    - "prod-key-abc123..."

log:
  level: info
  format: json

store:
  path: /var/lib/den/den.db
```

### Resource-Constrained

```yaml
sandbox:
  max_sandboxes: 10
  default_memory: 268435456   # 256MB
  default_cpu: 500000000      # 0.5 cores
  default_pid_limit: 128
  default_timeout: "10m"
```

### With S3 Storage (MinIO)

```yaml
sandbox:
  allow_volumes: true
  allow_s3: true
  max_volumes_per_sandbox: 5
  default_tmpfs:
    - path: "/tmp"
      size: "512m"
    - path: "/home/sandbox"
      size: "1g"

s3:
  endpoint: "http://minio:9000"
  region: "us-east-1"
  access_key: "minioadmin"
  secret_key: "minioadmin"
```
