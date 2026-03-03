---
title: CLI Reference
---

# CLI Reference

Den is a single binary that works as both a server and a CLI client.

## Global Flags

```
--config string    Config file path (default: den.yaml)
--server string    API server URL for client commands (default: http://localhost:8080)
```

The server URL can also be set via the `DEN_URL` environment variable.

---

## Server

### `den serve`

Start the HTTP API server.

```bash
den serve [flags]
```

**Flags:**
```
--config string   Path to config file (default: den.yaml)
```

**Examples:**
```bash
# Start with defaults (port 8080, no auth)
den serve

# Start with config file
den serve --config production.yaml
```

The server:
1. Connects to Docker daemon
2. Creates the `den-net` network (if needed)
3. Restores any persisted sandboxes from the BoltDB store
4. Starts the HTTP API and dashboard on the configured port

Press `Ctrl+C` for graceful shutdown (destroys all running sandboxes).

---

## Sandbox Management

### `den create`

Create a new sandbox.

```bash
den create [flags]
```

**Flags:**
```
--image string     Docker image (default: ubuntu:22.04)
--timeout string   Sandbox lifetime (default: 30m)
--cpu int          CPU limit in NanoCPUs
--memory int       Memory limit in bytes
```

**Examples:**
```bash
# Default sandbox
den create

# Custom image with 1-hour timeout
den create --image python:3.12 --timeout 1h

# Resource-limited sandbox
den create --image node:20 --memory 268435456 --cpu 500000000
```

**Output:**
```
d6jcj6a9qf76oti2r2sg
```

### `den ls`

List all sandboxes.

```bash
den ls
```

**Output:**
```
ID                     IMAGE           STATUS    CREATED              EXPIRES
d6jcj6a9qf76oti2r2sg  ubuntu:22.04    running   2026-03-03 11:44:25  2026-03-03 12:14:25
e7kdk7b0rg87puj3s3th  python:3.12     running   2026-03-03 11:45:00  2026-03-03 12:45:00
```

### `den exec`

Execute a command inside a sandbox.

```bash
den exec <sandbox-id> -- <command> [args...]
```

**Examples:**
```bash
# Simple command
den exec d6jcj6a9qf76oti2r2sg -- echo "Hello!"

# Run Python
den exec d6jcj6a9qf76oti2r2sg -- python3 -c "print(2+2)"

# Interactive-style (not truly interactive, but multi-word)
den exec d6jcj6a9qf76oti2r2sg -- bash -c "ls -la /tmp && echo done"
```

**Output:**
```
Hello!
```

The command's stdout is printed to your terminal. Non-zero exit codes are reflected in the CLI's exit code.

### `den rm`

Destroy a sandbox (stop and remove).

```bash
den rm <sandbox-id>
```

**Example:**
```bash
den rm d6jcj6a9qf76oti2r2sg
```

---

## Snapshots

### `den snapshot create`

Create a snapshot of a running sandbox.

```bash
den snapshot create <sandbox-id> [flags]
```

**Flags:**
```
--name string   Snapshot name (optional)
```

**Examples:**
```bash
den snapshot create d6jcj6a9qf76oti2r2sg
den snapshot create d6jcj6a9qf76oti2r2sg --name "after-setup"
```

### `den snapshot ls`

List snapshots.

```bash
# List all snapshots
den snapshot ls

# List snapshots for a specific sandbox
den snapshot ls <sandbox-id>
```

### `den snapshot restore`

Restore a snapshot to a new sandbox.

```bash
den snapshot restore <snapshot-id>
```

Creates a new running sandbox from the snapshot image and prints the new sandbox ID.

---

## Statistics

### `den stats`

Show system or sandbox statistics.

```bash
# System-wide stats
den stats

# Specific sandbox stats
den stats <sandbox-id>
```

**System output:**
```
Total sandboxes:   5
Running:           3
Stopped:           2
Snapshots:         2
```

**Sandbox output:**
```
CPU:      2.5%
Memory:   15 MB / 512 MB
PIDs:     3
Net RX:   1.0 KB
Net TX:   0.5 KB
```

---

## MCP Server

### `den mcp`

Start the MCP (Model Context Protocol) server in stdio mode.

```bash
den mcp [flags]
```

**Flags:**
```
--config string   Path to config file
```

This command starts a JSON-RPC 2.0 server over stdin/stdout, designed to be launched by AI tools like Claude Code or Cursor.

See [MCP Integration](mcp.md) for setup instructions.

---

## Version

### `den version`

Print version information.

```bash
den version
```

**Output:**
```
den v0.0.2 (abc1234) built 2026-03-03
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid usage / bad arguments |

For `den exec`, the exit code matches the command's exit code inside the sandbox.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `DEN_URL` | API server URL (default: `http://localhost:8080`) |
| `DEN_API_KEY` | API key for authentication |
| `DEN_SERVER__PORT` | Override server port |
| `DEN_LOG__LEVEL` | Log level (`debug`, `info`, `warn`, `error`) |

See [Configuration](configuration.md) for all environment variable options.
