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

## Validation

Server startup validates:

- `server.port` must be 1-65535
- `sandbox.max_sandboxes` must be positive
- `sandbox.default_memory` must be ≥ 4MB
- `sandbox.default_timeout` must be valid Go duration
