# CLI Commands

## Global Flags

```
--config string    Path to config file (default: den.yaml)
--server string    API server URL (default: http://localhost:8080)
```

## den serve

Start the HTTP API server.

```bash
den serve [--config den.yaml]
```

Creates `den-net` Docker network, restores persisted sandboxes from BoltDB, starts API + dashboard. Graceful shutdown with Ctrl+C (destroys all running sandboxes).

## den create

Create a new sandbox.

```bash
den create [--image ubuntu:22.04] [--timeout 1800] [--cpu 1000000000] [--memory 536870912]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--image` | `ubuntu:22.04` | Docker image |
| `--timeout` | `1800` | Lifetime in seconds |
| `--cpu` | `1e9` | CPU in NanoCPUs (1e9 = 1 core) |
| `--memory` | `536870912` | Memory in bytes (512MB) |

Outputs the sandbox ID.

## den ls

List all sandboxes.

```bash
den ls
# ID        IMAGE           STATUS    AGE
# cq4hsj3k  ubuntu:22.04    running   5m
```

## den exec

Execute a command inside a sandbox.

```bash
den exec <sandbox-id> -- <command> [args...]
den exec cq4hsj3k -- python3 -c "print(2+2)"
```

## den rm

Destroy a sandbox.

```bash
den rm <sandbox-id>
```

## den stats

Show resource usage.

```bash
den stats              # System-wide
den stats <sandbox-id> # Per-sandbox (CPU, memory, PIDs, network)
```

## den snapshot

```bash
den snapshot create <sandbox-id> [--name "checkpoint"]
den snapshot ls [sandbox-id]
den snapshot restore <snapshot-id>
```

## den mcp

Start MCP server in stdio mode.

```bash
den mcp [--config den.yaml]
```

## den version

```bash
den version
# den v0.1.0 (commit: abc1234, built: 2026-03-03)
```

## Environment Variables

- `DEN_URL` — API server URL (default: `http://localhost:8080`)
- `DEN_API_KEY` — API key for authentication
