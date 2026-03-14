# den-sdk

Python SDK for [Den](https://github.com/us/den) — the self-hosted sandbox runtime for AI agents.

> **100 sandboxes on E2B = ~$600/hour. 100 sandboxes on Den = one $5/month server.**

## Installation

```bash
pip install den-sdk
# or
uv add den-sdk
```

## Quick Start

```python
from den import Den

client = Den("http://localhost:8080", api_key="your-key")

# Create a sandbox
sandbox = client.sandbox.create(image="ubuntu:22.04")

# Execute a command
result = sandbox.exec(["python3", "-c", "print('Hello from Den!')"])
print(result.stdout)  # Hello from Den!

# Read/write files
sandbox.write_file("/tmp/hello.py", "print('hello world')")
content = sandbox.read_file("/tmp/hello.py")

# List files
files = sandbox.list_files("/tmp")

# Clean up
sandbox.destroy()
```

## Async Support

```python
import asyncio
from den import Den

async def main():
    client = Den("http://localhost:8080", api_key="your-key")

    sandbox = await client.sandbox.acreate(image="ubuntu:22.04")
    result = await sandbox.aexec(["echo", "async works!"])
    print(result.stdout)
    await sandbox.adestroy()

asyncio.run(main())
```

## Storage

```python
from den import Den, StorageConfig, VolumeMount

client = Den("http://localhost:8080")

# Persistent volume
sandbox = client.sandbox.create(
    image="ubuntu:22.04",
    storage=StorageConfig(
        volumes=[VolumeMount(name="my-data", mount_path="/data")]
    ),
)
```

## Snapshots

```python
# Save state
snapshot = sandbox.snapshot(name="after-setup")

# Restore from snapshot
restored = client.sandbox.restore_snapshot(snapshot.id)
```

## Sandbox Management

```python
# List all sandboxes
sandboxes = client.sandbox.list()

# Get a specific sandbox
sb = client.sandbox.get("sandbox-id")

# Stop a sandbox
sandbox.stop()

# Get sandbox stats
stats = sandbox.stats()
```

## Features

- Sandbox lifecycle management (create, list, get, stop, destroy)
- Command execution with timeout and environment variables
- File operations (read, write, list, mkdir, delete)
- Persistent volumes, shared volumes, tmpfs configuration
- Snapshot/restore
- Port forwarding
- Async support via `httpx`
- Type-safe with Pydantic models

## Requirements

- Python >= 3.10
- Den server running (see [Den repo](https://github.com/us/den))

## License

MIT
