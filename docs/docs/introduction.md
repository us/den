# Den

Self-hosted, open-source sandbox runtime for AI agents. Run untrusted LLM-generated code in secure, isolated Docker containers with fine-grained resource limits.

> **100 sandboxes on E2B = ~$600/hour. 100 sandboxes on Den = one $5/month server.**

## Why den?

AI agents need to execute code, but running arbitrary LLM output on your machine is dangerous. Cloud sandbox services (E2B etc.) add latency, cost, and vendor lock-in. den gives you the same functionality as a single Go binary you self-host.

## Shared Resource Model

Traditional sandbox runtimes allocate fixed resources per container. If each sandbox gets a dedicated 512MB, a server with 8GB RAM can only run ~10 sandboxes before hitting the wall — even when most of them are idle.

Den takes a different approach inspired by Google Borg and AWS Firecracker: **shared memory with pressure-based throttling**.

| Approach | Model | 8GB Server Capacity |
|----------|-------|---------------------|
| **Traditional** (E2B, etc.) | Fixed 512MB per container | ~10 sandboxes |
| **Den** (shared resources) | Shared memory + pressure monitoring | **100+ sandboxes** |

### How it works

1. **Overcommit** — Den allows 10x memory overcommit by default. Most sandboxes use a fraction of their allocated memory at any given time.
2. **Pressure monitoring** — A background goroutine samples host memory every 5 seconds and classifies pressure into 5 levels:
   - **Normal** (< 80%) — No action
   - **Warning** (80-85%) — Logged, no action
   - **High** (85-90%) — Per-container `memory.high` limits applied via cgroup v2
   - **Critical** (90-95%) — Aggressive throttling, new sandbox creation blocked (HTTP 503)
   - **Emergency** (> 95%) — Maximum throttling, creation blocked
3. **Soft limits, not hard kills** — Den uses cgroup v2 `memory.high` (throttle) instead of `memory.max` (OOM kill). Containers slow down under pressure but keep running.
4. **Auto-recovery** — When pressure drops back to Normal/Warning, memory limits are automatically removed.

### Cost comparison

| Setup | Sandboxes | Cost | Per-sandbox cost |
|-------|-----------|------|------------------|
| **E2B** | 100 | $0.10/min × 100 = **$600/hr** | $6.00/hr |
| **Den** (Hetzner CX22) | 100 | **$5/month** | $0.05/month |
| **Den** (bare metal) | 100 | One-time hardware cost | Effectively free |

That's a **120x cost reduction** for the same workload.

- **Isolated Docker containers** with all capabilities dropped, read-only rootfs, PID limits
- **REST API + WebSocket** for sandbox lifecycle, command execution, file operations
- **SDKs** for Go, TypeScript, and Python
- **MCP server** for Claude Code, Cursor, and other AI tools
- **Snapshots** — checkpoint and restore sandbox state via `docker commit`
- **Storage** — persistent volumes, shared volumes, tmpfs, S3 sync (hooks / on-demand / FUSE)
- **Dashboard UI** for monitoring

## Quick Example

```bash
# Start the server
den serve

# Create a sandbox
den create --image ubuntu:22.04 --timeout 1800 --memory 536870912

# Execute a command
den exec <sandbox-id> -- python3 -c "print(2 + 2)"

# Destroy the sandbox
den rm <sandbox-id>
```

```go
package main

import (
    "context"
    "fmt"
    client "github.com/us/den/pkg/client"
)

func main() {
    c := client.New("http://localhost:8080",
        client.WithAPIKey("your-api-key"),
    )

    sb, _ := c.CreateSandbox(context.Background(), client.SandboxConfig{
        Image:   "ubuntu:22.04",
        Timeout: "30m",
    })

    result, _ := c.Exec(context.Background(), sb.ID, client.ExecOpts{
        Cmd: []string{"python3", "-c", "print('Hello from sandbox!')"},
    })

    fmt.Println(result.Stdout) // Hello from sandbox!
    c.DestroySandbox(context.Background(), sb.ID)
}
```

## Security Model

Every sandbox container runs with:

| Control | Setting |
|---------|---------|
| Capabilities | ALL dropped, only `NET_BIND_SERVICE` added |
| Rootfs | Read-only |
| Privileges | `no-new-privileges` |
| PID limit | 256 (default, configurable) |
| Memory | 512MB (default, configurable) |
| CPU | 1 core (default, configurable) |
| Network | Internal Docker network (`den-net`) |
| Port binding | `127.0.0.1` only |

## Performance

Benchmarked on M-series Apple Silicon:

| Operation | Latency |
|-----------|---------|
| Create sandbox | ~100-160ms (cold), ~5ms (warm pool) |
| Execute command | ~20-30ms |
| Read file | ~28-30ms |
| Write file | ~56-70ms |
| Throughput | ~66 req/s |

## Next Steps

- [Installation](#installation) — Install den
- [Quick Start](#quick-start) — Run your first sandbox
- [MCP Server](#mcp) — Connect to Claude Code / Cursor
