# MCP Server

den includes a built-in Model Context Protocol server for AI tool integration.

## Setup

```bash
den mcp [--config den.yaml]
```

Communicates via JSON-RPC 2.0 over stdin/stdout. Logs go to stderr.

### Claude Code

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

With config:

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

### Cursor

Add to `.cursor/mcp.json`:

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

The MCP server exposes 9 tools:

| Tool | Parameters | Description |
|------|-----------|-------------|
| `create_sandbox` | `image?`, `timeout?`, `cpu?`, `memory?` | Create a new sandbox |
| `exec` | `sandbox_id`, `cmd`, `env?`, `workdir?`, `timeout?` | Execute a command |
| `read_file` | `sandbox_id`, `path` | Read file (auto base64 for binary) |
| `write_file` | `sandbox_id`, `path`, `content` | Write a file |
| `list_files` | `sandbox_id`, `path` | List directory contents |
| `destroy_sandbox` | `sandbox_id` | Destroy a sandbox |
| `list_sandboxes` | — | List all sandboxes |
| `snapshot_create` | `sandbox_id`, `name?` | Create a snapshot |
| `snapshot_restore` | `snapshot_id` | Restore from snapshot |

## Architecture

The MCP server creates its own Engine + Docker Runtime instance — it does not use the HTTP API server.

```
Claude Code / Cursor
  ↓ stdio (JSON-RPC 2.0)
den mcp
  ↓ in-process
Engine + Docker Runtime
  ↓ Docker API
Sandbox Containers
```

Protocol version: `2024-11-05`
