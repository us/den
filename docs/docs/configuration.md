# Configuration

Configuration merges in order: **defaults → YAML file → environment variables**.

## den.yaml

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  allowed_origins:
    - "http://localhost:8080"
  rate_limit_rps: 10
  rate_limit_burst: 20
  tls:
    enabled: false
    cert_file: ""
    key_file: ""

runtime:
  backend: "docker"
  docker_host: ""
  network_id: "den-net"

sandbox:
  default_image: "ubuntu:22.04"
  default_timeout: "30m"
  max_sandboxes: 50
  default_cpu: 1000000000        # NanoCPUs (1e9 = 1 core)
  default_memory: 536870912      # bytes (512MB)
  default_pid_limit: 256
  warm_pool_size: 0
  allow_volumes: true
  allow_shared_volumes: true
  allow_s3: true
  allow_s3_fuse: false
  allow_host_binds: false        # NEVER enable in production
  max_volumes_per_sandbox: 5
  default_tmpfs:
    - path: "/tmp"
      size: "256m"
    - path: "/home/sandbox"
      size: "512m"
    - path: "/run"
      size: "64m"
    - path: "/var/tmp"
      size: "128m"

s3:
  endpoint: ""
  region: "us-east-1"
  access_key: ""
  secret_key: ""

resource:
  overcommit_ratio: 10.0          # Memory overcommit multiplier
  pressure_threshold: 0.80        # Warning level (80%)
  critical_threshold: 0.90        # Critical level (90%)
  monitor_interval: "5s"          # Pressure sampling interval
  enable_auto_throttle: true      # Dynamic memory.high adjustment
  min_memory_floor: 33554432      # 32MB minimum per container

store:
  path: "den.db"

auth:
  enabled: false
  api_keys:
    - "your-secret-key-here"

log:
  level: "info"                  # debug, info, warn, error
  format: "text"                 # text, json
```

## Environment Variables

Prefix `DEN_` with `__` as nesting separator:

| Config | Environment Variable |
|--------|---------------------|
| `server.port` | `DEN_SERVER__PORT` |
| `server.host` | `DEN_SERVER__HOST` |
| `sandbox.default_image` | `DEN_SANDBOX__DEFAULT_IMAGE` |
| `sandbox.default_timeout` | `DEN_SANDBOX__DEFAULT_TIMEOUT` |
| `sandbox.max_sandboxes` | `DEN_SANDBOX__MAX_SANDBOXES` |
| `sandbox.default_memory` | `DEN_SANDBOX__DEFAULT_MEMORY` |
| `sandbox.default_cpu` | `DEN_SANDBOX__DEFAULT_CPU` |
| `auth.enabled` | `DEN_AUTH__ENABLED` |
| `log.level` | `DEN_LOG__LEVEL` |
| `s3.endpoint` | `DEN_S3__ENDPOINT` |
| `s3.access_key` | `DEN_S3__ACCESS_KEY` |
| `s3.secret_key` | `DEN_S3__SECRET_KEY` |

## Resource Management

Controls host memory pressure monitoring and dynamic container throttling.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `resource.overcommit_ratio` | float | `10.0` | Memory overcommit multiplier. Higher = more sandboxes per server, but higher pressure risk |
| `resource.pressure_threshold` | float | `0.80` | Memory usage ratio that triggers Warning level |
| `resource.critical_threshold` | float | `0.90` | Memory usage ratio that triggers Critical level (blocks new sandboxes) |
| `resource.monitor_interval` | duration | `5s` | How often to sample host memory pressure |
| `resource.enable_auto_throttle` | bool | `true` | Automatically apply cgroup v2 `memory.high` limits when pressure rises |
| `resource.min_memory_floor` | int | `33554432` (32MB) | Minimum memory per container, even under maximum pressure |

### Environment Variables

| Config | Environment Variable |
|--------|---------------------|
| `resource.overcommit_ratio` | `DEN_RESOURCE__OVERCOMMIT_RATIO` |
| `resource.pressure_threshold` | `DEN_RESOURCE__PRESSURE_THRESHOLD` |
| `resource.critical_threshold` | `DEN_RESOURCE__CRITICAL_THRESHOLD` |
| `resource.monitor_interval` | `DEN_RESOURCE__MONITOR_INTERVAL` |
| `resource.enable_auto_throttle` | `DEN_RESOURCE__ENABLE_AUTO_THROTTLE` |
| `resource.min_memory_floor` | `DEN_RESOURCE__MIN_MEMORY_FLOOR` |

### Notes

- Pressure levels use hysteresis: 2 consecutive readings are required before the level changes, preventing flapping
- On Linux, throttling uses direct cgroup v2 `memory.high` writes (sub-ms). On macOS, it falls back to Docker API
- When `enable_auto_throttle` is false, pressure is still monitored and reported via `GET /api/v1/resources`, but no limits are applied

## Validation

Server startup validates:

- `server.port` must be 1-65535
- `sandbox.max_sandboxes` must be positive
- `sandbox.default_memory` must be ≥ 4MB
- `sandbox.default_timeout` must be valid Go duration
