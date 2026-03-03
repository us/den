# den

Self-hosted, open-source sandbox runtime for AI agents. Run untrusted LLM-generated code in secure, isolated Docker containers with fine-grained resource limits.

## Why den?

AI agents need to execute code, but running arbitrary LLM output on your machine is dangerous. Cloud sandbox services (E2B etc.) add latency, cost, and vendor lock-in. den gives you the same functionality as a single Go binary you self-host.

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
