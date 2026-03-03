---
title: MCP Integration
---

# MCP Integration

Den includes a built-in [Model Context Protocol](https://modelcontextprotocol.io/) (MCP) server. This lets AI tools like Claude Code, Cursor, and other MCP-compatible clients create sandboxes, run code, and manage files directly.

## Quick Start

```bash
# Start the Den API server (in one terminal)
den serve

# Start the MCP server (in another terminal, or configure in your AI tool)
den mcp
```

The MCP server communicates over **stdio** (stdin/stdout) using JSON-RPC 2.0.

## Setup with Claude Code

Add to your Claude Code MCP configuration (`~/.claude.json` or project `.mcp.json`):

```json
{
  "mcpServers": {
    "den": {
      "command": "den",
      "args": ["mcp"]
    }
  }
}
```

With a config file:

```json
{
  "mcpServers": {
    "den": {
      "command": "den",
      "args": ["mcp", "--config", "/path/to/den.yaml"]
    }
  }
}
```

## Setup with Cursor

Add to your Cursor MCP settings (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "den": {
      "command": "den",
      "args": ["mcp"]
    }
  }
}
```

## Available Tools

The MCP server exposes nine tools:

### `create_sandbox`

Create a new isolated sandbox environment.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `image` | string | No | Docker image (default: `ubuntu:22.04`) |
| `timeout` | string | No | Sandbox lifetime (default: `30m`) |
| `cpu` | number | No | CPU limit in NanoCPUs |
| `memory` | number | No | Memory limit in bytes |

**Returns:** Sandbox ID, status, and metadata.

### `exec`

Execute a command inside a sandbox.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `sandbox_id` | string | Yes | Target sandbox ID |
| `cmd` | string[] | Yes | Command and arguments |
| `env` | object | No | Environment variables |
| `workdir` | string | No | Working directory |
| `timeout` | number | No | Timeout in seconds (max 300) |

**Returns:** Exit code, stdout, and stderr.

### `read_file`

Read a file from a sandbox.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `sandbox_id` | string | Yes | Target sandbox ID |
| `path` | string | Yes | Absolute file path |

**Returns:** File content as text, or base64-encoded if binary.

### `write_file`

Write content to a file in a sandbox.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `sandbox_id` | string | Yes | Target sandbox ID |
| `path` | string | Yes | Absolute file path |
| `content` | string | Yes | File content |

**Returns:** Success confirmation.

### `list_files`

List files in a directory.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `sandbox_id` | string | Yes | Target sandbox ID |
| `path` | string | Yes | Directory path |

**Returns:** Array of files with name, size, mode, and type.

### `destroy_sandbox`

Stop and remove a sandbox.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `sandbox_id` | string | Yes | Target sandbox ID |

**Returns:** Success confirmation.

### `list_sandboxes`

List all active sandboxes.

**Parameters:** None

**Returns:** Array of sandbox objects with ID, image, status, and timestamps.

### `snapshot_create`

Save the current state of a sandbox.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `sandbox_id` | string | Yes | Target sandbox ID |
| `name` | string | No | Snapshot name |

**Returns:** Snapshot ID and metadata.

### `snapshot_restore`

Create a new sandbox from a snapshot.

**Parameters:**
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `snapshot_id` | string | Yes | Snapshot ID to restore |

**Returns:** New sandbox ID and metadata.

## Example Conversation

Once configured, you can ask Claude to use sandboxes naturally:

> **You:** Run this Python script in a sandbox and show me the output:
> ```python
> import sys
> print(f"Python {sys.version}")
> print(sum(range(1000)))
> ```

Claude will:
1. Call `create_sandbox` to create an environment
2. Call `write_file` to save the script
3. Call `exec` to run it
4. Return the output to you
5. Call `destroy_sandbox` to clean up

## MCP Protocol Details

The MCP server implements the **2024-11-05** protocol version with the following capabilities:

- **Transport:** stdio (JSON-RPC 2.0 over stdin/stdout)
- **Methods:** `initialize`, `tools/list`, `tools/call`, `ping`
- **Notifications:** `notifications/initialized`, `notifications/cancelled`
- **Logging:** All MCP server logs go to stderr (stdout is reserved for the protocol)

## Architecture

```
AI Tool (Claude Code / Cursor)
  ↓ stdio (JSON-RPC 2.0)
MCP Server (den mcp)
  ↓ in-process
Engine + Docker Runtime
  ↓ Docker API
Sandbox Containers
```

The MCP server creates its own Engine and Docker Runtime instances. It connects directly to Docker — it does not go through the HTTP API server.

## Configuration

The MCP server uses the same config file as the API server:

```bash
den mcp --config den.yaml
```

Relevant config options:
- `runtime.docker_host` — Docker socket path
- `sandbox.*` — Default sandbox settings
- `log.level` — MCP server log verbosity (logs to stderr)
