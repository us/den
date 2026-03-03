---
title: Getting Started
---

# Getting Started

## Prerequisites

- **Docker** — Running and accessible (Den manages containers via Docker API)
- **Go 1.21+** — For building from source

Verify Docker is running:

```bash
docker info
```

## Installation

### Build from Source

```bash
git clone https://github.com/den/den
cd den
go build -o den ./cmd/den
```

### Binary Releases

```bash
# macOS / Linux
curl -sSL https://get.den.dev | sh

# Or download from GitHub Releases
```

## Start the Server

```bash
# With defaults (port 8080, no auth)
./den serve

# With config file
./den serve --config den.yaml
```

The server will:
1. Connect to Docker
2. Create the `den-net` network (if needed)
3. Restore any persisted sandboxes from the BoltDB store
4. Start the HTTP API on port 8080

## Your First Sandbox

### 1. Create a Sandbox

```bash
curl -s -X POST http://localhost:8080/api/v1/sandboxes \
  -H 'Content-Type: application/json' \
  -d '{"image": "ubuntu:22.04"}' | jq
```

Response:
```json
{
  "id": "d6jcj6a9qf76oti2r2sg",
  "image": "ubuntu:22.04",
  "status": "running",
  "created_at": "2026-03-03T11:44:25.809Z",
  "expires_at": "2026-03-03T12:14:25.809Z"
}
```

Save the `id` for the next steps:
```bash
export SB_ID="d6jcj6a9qf76oti2r2sg"
```

### 2. Execute a Command

```bash
curl -s -X POST "http://localhost:8080/api/v1/sandboxes/$SB_ID/exec" \
  -H 'Content-Type: application/json' \
  -d '{"cmd": ["echo", "Hello from sandbox!"]}' | jq
```

Response:
```json
{
  "exit_code": 0,
  "stdout": "Hello from sandbox!\n",
  "stderr": ""
}
```

### 3. Write a File

```bash
curl -s -X PUT "http://localhost:8080/api/v1/sandboxes/$SB_ID/files?path=/tmp/hello.py" \
  -d 'print("Hello World!")'
```

### 4. Read It Back

```bash
curl -s "http://localhost:8080/api/v1/sandboxes/$SB_ID/files?path=/tmp/hello.py"
# → print("Hello World!")
```

### 5. Run Your Script

> **Note:** The default `ubuntu:22.04` image does not include Python. Use `python:3.12` or install it with `apt-get install -y python3` first.

```bash
curl -s -X POST "http://localhost:8080/api/v1/sandboxes/$SB_ID/exec" \
  -H 'Content-Type: application/json' \
  -d '{"cmd": ["bash", "-c", "cat /tmp/hello.py"]}' | jq .stdout
# → "print(\"Hello World!\")\n"
```

### 6. Clean Up

```bash
curl -s -X DELETE "http://localhost:8080/api/v1/sandboxes/$SB_ID"
# → 204 No Content
```

## Enable Authentication

For production use, enable API key authentication:

```yaml
# den.yaml
auth:
  enabled: true
  api_keys:
    - "your-secret-key-here"
```

Then pass the key in requests:

```bash
curl -H 'X-API-Key: your-secret-key-here' \
  http://localhost:8080/api/v1/sandboxes
```

## Using the CLI

Den includes a built-in CLI client:

```bash
# Create sandbox
den create --image ubuntu:22.04

# List sandboxes
den ls

# Execute command
den exec <id> -- python3 -c "print('hello')"

# Delete sandbox
den rm <id>
```

The CLI connects to `http://localhost:8080` by default. Override with `--server`:

```bash
den --server http://remote:8080 ls
```

## Using the MCP Server

For AI tool integration (Claude Code, Cursor):

```bash
den mcp
```

See [MCP Integration](mcp.md) for setup instructions.

## Next Steps

- [API Reference](api-reference.md) — All endpoints documented
- [Configuration](configuration.md) — Tune resource limits, auth, networking
- [SDK Guide](sdk.md) — Use the Go, TypeScript, or Python client
