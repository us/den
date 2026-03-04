# SDKs

den provides official SDKs for Go, TypeScript, and Python.

| SDK | Package | Install |
|-----|---------|---------|
| Go | [`github.com/us/den`](https://pkg.go.dev/github.com/us/den) | `go get github.com/us/den@latest` |
| TypeScript | [`@den/sdk`](https://www.npmjs.com/package/@den/sdk) | `bun add @den/sdk` |
| Python | [`den`](https://pypi.org/project/den/) | `uv add den` / `pip install den` |

## Go SDK

```go
import client "github.com/us/den/pkg/client"

c := client.New("http://localhost:8080",
    client.WithAPIKey("your-api-key"),
)

// Create sandbox (timeout in seconds)
sb, _ := c.CreateSandbox(ctx, client.SandboxConfig{
    Image:   "ubuntu:22.04",
    Timeout: 1800, // 30 minutes
})

// Execute command
result, _ := c.Exec(ctx, sb.ID, client.ExecOpts{
    Cmd: []string{"python3", "-c", "print('hello')"},
})
fmt.Println(result.Stdout)

// File operations
c.WriteFile(ctx, sb.ID, "/tmp/test.py", []byte(`print("world")`))
content, _ := c.ReadFile(ctx, sb.ID, "/tmp/test.py")

// Snapshots
snap, _ := c.CreateSnapshot(ctx, sb.ID, "checkpoint")
restored, _ := c.RestoreSnapshot(ctx, snap.ID)

// Cleanup
c.DestroySandbox(ctx, sb.ID)
```

### Methods

| Method | Description |
|--------|-------------|
| `CreateSandbox(ctx, config)` | Create a new sandbox |
| `GetSandbox(ctx, id)` | Get sandbox details |
| `ListSandboxes(ctx)` | List all sandboxes |
| `StopSandbox(ctx, id)` | Stop a sandbox |
| `DestroySandbox(ctx, id)` | Destroy a sandbox |
| `Exec(ctx, id, opts)` | Execute a command |
| `ReadFile(ctx, id, path)` | Read file contents |
| `WriteFile(ctx, id, path, content)` | Write a file |
| `CreateSnapshot(ctx, id, name)` | Snapshot a sandbox |
| `RestoreSnapshot(ctx, snapshotID)` | Restore from snapshot |
| `Health(ctx)` | Health check |

## TypeScript SDK

```typescript
import { Den } from '@den/sdk';

const den = new Den({
  url: 'http://localhost:8080',
  apiKey: 'your-api-key',
});

// Create sandbox (timeout in seconds)
const sandbox = await den.sandbox.create({
  image: 'ubuntu:22.04',
  timeout: 1800, // 30 minutes
});

const result = await sandbox.exec(['python3', '-c', 'print("hello")']);
console.log(result.stdout);

await sandbox.writeFile('/tmp/test.py', 'print("world")');
const content = await sandbox.readFile('/tmp/test.py');

// Snapshots
const snapshot = await sandbox.snapshot('checkpoint');
const snapshots = await sandbox.listSnapshots();

await sandbox.destroy();
```

## Python SDK

```python
from den import Den

# Sync usage
client = Den("http://localhost:8080", api_key="your-api-key")

sandbox = client.sandbox.create(image="ubuntu:22.04")
result = sandbox.exec(["echo", "hello"])
print(result.stdout)

sandbox.destroy()
client.close()
```

### Async Usage

```python
import asyncio
from den import Den

async def main():
    client = Den("http://localhost:8080", api_key="your-api-key")

    sandbox = await client.sandbox.acreate(image="ubuntu:22.04")
    result = await sandbox.aexec(["echo", "hello"])
    print(result.stdout)

    await sandbox.adestroy()
    await client.aclose()

asyncio.run(main())
```
