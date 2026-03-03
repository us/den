# Quick Start

## 1. Start the Server

```bash
den serve
# Or with a config file
den serve --config den.yaml
```

The API server starts on `http://localhost:8080` with an embedded dashboard.

## 2. Create a Sandbox

```bash
den create --image ubuntu:22.04 --timeout 1800 --memory 536870912
# → cq4hsj3k...
```

Or via the API:

```bash
curl -X POST http://localhost:8080/api/v1/sandboxes \
  -H 'Content-Type: application/json' \
  -d '{"image": "ubuntu:22.04"}'
# → {"id":"cq4hsj3k","status":"running",...}
```

## 3. Execute Commands

```bash
den exec cq4hsj3k -- python3 -c "print(2+2)"
# 4
```

```bash
curl -X POST http://localhost:8080/api/v1/sandboxes/cq4hsj3k/exec \
  -H 'Content-Type: application/json' \
  -d '{"cmd": ["python3", "-c", "print(2+2)"]}'
# → {"exit_code":0,"stdout":"4\n","stderr":""}
```

## 4. File Operations

```bash
# Write a file
curl -X PUT 'http://localhost:8080/api/v1/sandboxes/cq4hsj3k/files?path=/tmp/hello.py' \
  -d 'print("Hello from sandbox!")'

# Read a file
curl 'http://localhost:8080/api/v1/sandboxes/cq4hsj3k/files?path=/tmp/hello.py'
```

## 5. Snapshots

```bash
# Create a snapshot
den snapshot create cq4hsj3k --name "after-setup"

# List snapshots
den snapshot ls cq4hsj3k

# Restore from snapshot (creates new sandbox)
den snapshot restore <snapshot-id>
```

## 6. List & Destroy

```bash
# List all sandboxes
den ls
# ID        IMAGE           STATUS    AGE
# cq4hsj3k  ubuntu:22.04    running   5m

# Resource stats
den stats cq4hsj3k
# CPU: 2.5%  Memory: 15 MB / 512 MB  PIDs: 3

# Destroy
den rm cq4hsj3k
```

## Environment Variables

Instead of `--config`:

```bash
DEN_SERVER__PORT=9090 \
DEN_SANDBOX__MAX_SANDBOXES=100 \
DEN_AUTH__ENABLED=true \
den serve
```

## Next Steps

- [Architecture](#architecture) — How isolation works
- [Configuration](#configuration) — Full `den.yaml` reference
- [REST API](#rest-api) — All API endpoints
- [MCP Server](#mcp) — Connect to Claude Code
