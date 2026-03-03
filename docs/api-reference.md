---
title: API Reference
---

# API Reference

All endpoints are served under `/api/v1/`. Responses are JSON unless otherwise noted.

## Authentication

When auth is enabled, include the API key header:

```
X-API-Key: your-secret-key
```

Unauthenticated requests return `401 Unauthorized`.

## Health & Version

### Health Check

```
GET /api/v1/health
```

Response `200 OK`:
```json
{"status": "ok"}
```

### Version

```
GET /api/v1/version
```

Response `200 OK`:
```json
{
  "version": "0.1.0",
  "commit": "abc1234",
  "build_date": "2026-03-03T00:00:00Z"
}
```

---

## Sandboxes

### Create Sandbox

```
POST /api/v1/sandboxes
Content-Type: application/json
```

Request body:
```json
{
  "image": "ubuntu:22.04",
  "env": {"MY_VAR": "value"},
  "workdir": "/home/sandbox",
  "timeout": 1800,
  "cpu": 1000000000,
  "memory": 536870912,
  "ports": [
    {"sandbox_port": 3000, "host_port": 0, "protocol": "tcp"}
  ]
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | `ubuntu:22.04` | Docker image to use |
| `env` | object | `{}` | Environment variables |
| `workdir` | string | `""` | Working directory |
| `timeout` | int | `1800` | Auto-expiry in seconds (default 30 min) |
| `cpu` | int | `1000000000` | CPU limit in NanoCPUs (1 core = 1e9) |
| `memory` | int | `536870912` | Memory limit in bytes (default 512MB) |
| `ports` | array | `[]` | Port mappings (set `host_port: 0` for auto-assign) |

Response `201 Created`:
```json
{
  "id": "d6jcj6a9qf76oti2r2sg",
  "image": "ubuntu:22.04",
  "status": "running",
  "created_at": "2026-03-03T11:44:25.809Z",
  "expires_at": "2026-03-03T12:14:25.809Z"
}
```

Error responses:
- `400` — Invalid request body
- `429` — Rate limit exceeded
- `503` — Maximum sandbox limit reached

### List Sandboxes

```
GET /api/v1/sandboxes
```

Response `200 OK`:
```json
[
  {
    "id": "d6jcj6a9qf76oti2r2sg",
    "image": "ubuntu:22.04",
    "status": "running",
    "created_at": "2026-03-03T11:44:25.809Z",
    "expires_at": "2026-03-03T12:14:25.809Z"
  }
]
```

### Get Sandbox

```
GET /api/v1/sandboxes/{id}
```

Response `200 OK`:
```json
{
  "id": "d6jcj6a9qf76oti2r2sg",
  "image": "ubuntu:22.04",
  "status": "running",
  "created_at": "2026-03-03T11:44:25.809Z",
  "expires_at": "2026-03-03T12:14:25.809Z"
}
```

Error: `404` — Sandbox not found

### Stop Sandbox

```
POST /api/v1/sandboxes/{id}/stop
```

Stops the container without removing it. The sandbox can be inspected but not used for exec or file operations.

Response `200 OK`:
```json
{"status": "stopped"}
```

Error: `404` — Sandbox not found

### Destroy Sandbox

```
DELETE /api/v1/sandboxes/{id}
```

Stops and removes the container and all associated state.

Response `204 No Content`

Error: `404` — Sandbox not found

---

## Command Execution

### Execute Command (Sync)

```
POST /api/v1/sandboxes/{id}/exec
Content-Type: application/json
```

Request body:
```json
{
  "cmd": ["python3", "-c", "print('hello')"],
  "env": {"KEY": "value"},
  "workdir": "/tmp",
  "timeout": 30
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cmd` | string[] | **required** | Command and arguments |
| `env` | object | `{}` | Additional environment variables |
| `workdir` | string | `""` | Working directory inside sandbox |
| `timeout` | int | `30` | Timeout in seconds (max 300) |

Response `200 OK`:
```json
{
  "exit_code": 0,
  "stdout": "hello\n",
  "stderr": ""
}
```

Error responses:
- `400` — Invalid request (empty cmd)
- `404` — Sandbox not found
- `409` — Sandbox is not running

### Execute Command (WebSocket Streaming)

```
GET /api/v1/sandboxes/{id}/exec/stream
Upgrade: websocket
```

Connect via WebSocket, then send a JSON message:

```json
{
  "cmd": ["python3", "script.py"],
  "env": {},
  "workdir": "/tmp",
  "timeout": 60
}
```

Server streams messages:

```json
{"type": "stdout", "data": "output line\n"}
{"type": "stderr", "data": "error line\n"}
{"type": "exit", "data": "0"}
```

On error:
```json
{"type": "error", "data": "execution failed"}
```

The connection closes after the `exit` message.

---

## File Operations

All file paths are specified via the `path` query parameter. Paths must be absolute.

Writable locations (with default security config):
- `/tmp`
- `/home/sandbox`
- `/run`
- `/var/tmp`

### Read File

```
GET /api/v1/sandboxes/{id}/files?path=/tmp/hello.py
```

Response `200 OK`: Raw file content (Content-Type based on extension)

Errors:
- `400` — Missing path parameter
- `404` — Sandbox or file not found
- `409` — Sandbox is not running

### Write File

```
PUT /api/v1/sandboxes/{id}/files?path=/tmp/hello.py
Content-Type: application/octet-stream

print("Hello World!")
```

Request body is the raw file content. Parent directories are created automatically.

Response `200 OK`:
```json
{"success": true}
```

Errors:
- `400` — Missing path parameter
- `404` — Sandbox not found
- `409` — Sandbox is not running

### List Directory

```
GET /api/v1/sandboxes/{id}/files/list?path=/tmp
```

Response `200 OK`:
```json
[
  {
    "name": "hello.py",
    "path": "/tmp/hello.py",
    "size": 21,
    "mode": "-rw-r--r--",
    "mod_time": "2026-03-03T12:00:00Z",
    "is_dir": false
  },
  {
    "name": "data",
    "path": "/tmp/data",
    "size": 4096,
    "mode": "drwxr-xr-x",
    "mod_time": "2026-03-03T11:55:00Z",
    "is_dir": true
  }
]
```

### Create Directory

```
POST /api/v1/sandboxes/{id}/files/mkdir?path=/tmp/mydir
```

Creates the directory and all parent directories.

Response `204 No Content`

### Delete File or Directory

```
DELETE /api/v1/sandboxes/{id}/files?path=/tmp/hello.py
```

Recursively removes the file or directory.

Response `204 No Content`

### Upload File (Multipart)

```
POST /api/v1/sandboxes/{id}/files/upload?path=/tmp/uploaded.bin
Content-Type: multipart/form-data

--boundary
Content-Disposition: form-data; name="file"; filename="data.bin"
Content-Type: application/octet-stream

<binary data>
--boundary--
```

Max upload size: 100MB

Response `204 No Content`

### Download File

```
GET /api/v1/sandboxes/{id}/files/download?path=/tmp/hello.py
```

Returns the file as a download with `Content-Disposition: attachment` header.

Response `200 OK`: Raw file content

---

## Snapshots

Snapshots save the current state of a sandbox using `docker commit`. You can restore a snapshot to create a new sandbox in the same state.

> **Note:** Files stored in tmpfs mounts (`/tmp`, `/home/sandbox`, `/run`, `/var/tmp`) are **not preserved** in snapshots. Docker commit captures the container's writable layer, but tmpfs is stored in memory and not part of the layer. To persist files across snapshots, write them to a non-tmpfs path inside the container.

### Create Snapshot

```
POST /api/v1/sandboxes/{id}/snapshots
Content-Type: application/json
```

Request body:
```json
{
  "name": "after-setup"
}
```

Response `201 Created`:
```json
{
  "id": "snap_abc123",
  "sandbox_id": "d6jcj6a9qf76oti2r2sg",
  "name": "after-setup",
  "image_id": "sha256:...",
  "created_at": "2026-03-03T12:00:00Z",
  "size": 0
}
```

### List Snapshots

```
GET /api/v1/sandboxes/{id}/snapshots
```

Response `200 OK`:
```json
[
  {
    "id": "snap_abc123",
    "sandbox_id": "d6jcj6a9qf76oti2r2sg",
    "name": "after-setup",
    "image_id": "sha256:...",
    "created_at": "2026-03-03T12:00:00Z",
    "size": 0
  }
]
```

### Restore Snapshot

```
POST /api/v1/snapshots/{snapshotId}/restore
```

Creates a new sandbox from the snapshot image.

Response `201 Created`:
```json
{
  "id": "new_sandbox_id",
  "image": "sha256:...",
  "status": "running",
  "created_at": "2026-03-03T12:05:00Z",
  "expires_at": "2026-03-03T12:35:00Z"
}
```

### Delete Snapshot

```
DELETE /api/v1/snapshots/{snapshotId}
```

Removes the snapshot image from Docker.

Response `204 No Content`

---

## Port Forwarding

### List Ports

```
GET /api/v1/sandboxes/{id}/ports
```

Response `200 OK`:
```json
[
  {
    "sandbox_port": 3000,
    "host_port": 49152,
    "protocol": "tcp"
  }
]
```

Ports are configured at sandbox creation time via the `ports` field in the create request. Forwarded ports bind to `127.0.0.1` only.

---

## Statistics

### Sandbox Stats

```
GET /api/v1/sandboxes/{id}/stats
```

Response `200 OK`:
```json
{
  "cpu_percent": 2.5,
  "memory_usage": 15728640,
  "memory_limit": 536870912,
  "memory_percent": 2.93,
  "network_rx_bytes": 1024,
  "network_tx_bytes": 512,
  "disk_read_bytes": 4096,
  "disk_write_bytes": 2048,
  "pid_count": 3,
  "timestamp": "2026-03-03T12:00:00Z"
}
```

### System Stats

```
GET /api/v1/stats
```

Response `200 OK`:
```json
{
  "total_sandboxes": 5,
  "running_sandboxes": 3,
  "stopped_sandboxes": 2,
  "total_snapshots": 2
}
```

---

## Error Responses

All errors follow a consistent format:

```json
{
  "error": "description of what went wrong"
}
```

| Status | Meaning |
|--------|---------|
| `400` | Bad request (invalid JSON, missing required fields) |
| `401` | Unauthorized (missing or invalid API key) |
| `404` | Resource not found (sandbox, snapshot, file) |
| `408` | Request timeout (exec exceeded timeout) |
| `409` | Conflict (sandbox is not running) |
| `413` | Payload too large (body > 1MB for JSON, > 100MB for uploads) |
| `429` | Too many requests (rate limit exceeded) |
| `500` | Internal server error |
| `503` | Service unavailable (sandbox limit reached) |

## Rate Limiting

When rate limiting is enabled, the API enforces per-key limits. Requests exceeding the limit receive `429 Too Many Requests`.

Rate limit is tracked by API key (when auth is enabled) or by client IP (when auth is disabled).

Default: 10 requests/second with burst of 20.
