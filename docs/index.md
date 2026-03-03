---
title: Den Documentation
description: Self-hosted sandbox runtime for AI agents
---

# Den Documentation

**English** | [中文](zh-CN/index.md)

Den provides secure, isolated sandbox environments for AI agents to execute code. It's the open-source, self-hosted alternative to cloud sandbox services.

## Overview

Den runs as a single binary that manages Docker containers as sandboxes. AI agents interact with it via REST API, WebSocket, MCP protocol, or client SDKs.

```
Agent → Den API → Docker → Isolated Container
```

Each sandbox is a Docker container with:
- Dropped capabilities and read-only root filesystem
- Resource limits (CPU, memory, PIDs)
- Automatic expiry after configurable timeout
- Full file I/O and command execution

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](getting-started.md) | Install, configure, create your first sandbox |
| [API Reference](api-reference.md) | Complete REST API with examples |
| [Configuration](configuration.md) | All config options with defaults |
| [SDK Guide](sdk.md) | Go, TypeScript, and Python clients |
| [MCP Integration](mcp.md) | Use with Claude Code, Cursor, and other AI tools |
| [Architecture](architecture.md) | Internal design, security model, data flow |
| [CLI Reference](cli.md) | All CLI commands and flags |

## Quick Example

```bash
# Start the server
den serve

# Create a sandbox
curl -X POST http://localhost:8080/api/v1/sandboxes \
  -H 'Content-Type: application/json' \
  -d '{"image": "ubuntu:22.04"}'

# Run code
curl -X POST http://localhost:8080/api/v1/sandboxes/{id}/exec \
  -H 'Content-Type: application/json' \
  -d '{"cmd": ["python3", "-c", "print(42)"]}'
```
