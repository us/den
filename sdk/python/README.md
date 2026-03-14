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
sandbox = client.create(image="ubuntu:22.04")

# Execute a command
result = sandbox.exec(["python3", "-c", "print('Hello from Den!')"])
print(result.stdout)  # Hello from Den!

# Read/write files
sandbox.write_file("/tmp/hello.py", "print('hello world')")
content = sandbox.read_file("/tmp/hello.py")

# Clean up
sandbox.destroy()
```

## Async Support

```python
import asyncio
from den import Den

async def main():
    client = Den("http://localhost:8080", api_key="your-key")

    sandbox = await client.acreate(image="ubuntu:22.04")
    result = await sandbox.aexec(["echo", "async works!"])
    print(result.stdout)
    await sandbox.adestroy()

asyncio.run(main())
```

## Storage

```python
from den import Den, SandboxConfig, StorageConfig, VolumeMount

client = Den("http://localhost:8080")

# Persistent volume
sandbox = client.create(
    image="ubuntu:22.04",
    storage=StorageConfig(
        volumes=[VolumeMount(name="my-data", mount_path="/data")]
    ),
)

# S3 import/export
sandbox.s3_import(
    bucket="my-bucket",
    key="data/input.csv",
    dest_path="/home/sandbox/input.csv",
)

sandbox.s3_export(
    source_path="/home/sandbox/output.csv",
    bucket="my-bucket",
    key="results/output.csv",
)
```

## Snapshots

```python
# Save state
snapshot = sandbox.snapshot(name="after-setup")

# Restore later
restored = client.restore_snapshot(snapshot.id)
```

## Features

- Sandbox lifecycle management (create, list, get, stop, destroy)
- Command execution with timeout and environment variables
- File operations (read, write, list, mkdir, delete, upload, download)
- Persistent volumes, shared volumes, tmpfs configuration
- S3 integration (hooks, on-demand, FUSE)
- Snapshot/restore
- Port forwarding
- Async support via `httpx`
- Type-safe with Pydantic models

## Requirements

- Python >= 3.10
- Den server running (see [Den repo](https://github.com/us/den))

## License

MIT
