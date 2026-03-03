---
title: SDK Guide
---

# SDK Guide

Den provides client SDKs for Go, TypeScript, and Python. All SDKs wrap the REST API and provide idiomatic interfaces for each language.

## Go SDK

The Go client lives in `pkg/client` and can be imported directly from the module.

### Install

```bash
go get github.com/den/den
```

### Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    client "github.com/den/den/pkg/client"
)

func main() {
    // Create client
    c := client.New("http://localhost:8080",
        client.WithAPIKey("your-api-key"),
    )

    ctx := context.Background()

    // Create a sandbox
    sb, err := c.CreateSandbox(ctx, client.SandboxConfig{
        Image:   "ubuntu:22.04",
        Timeout: "30m",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Sandbox created: %s\n", sb.ID)

    // Execute a command
    result, err := c.Exec(ctx, sb.ID, client.ExecOpts{
        Cmd:     []string{"python3", "-c", "print(2 + 2)"},
        Timeout: 30,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Output: %s", result.Stdout) // "4\n"
    fmt.Printf("Exit code: %d\n", result.ExitCode) // 0

    // Write a file
    err = c.WriteFile(ctx, sb.ID, "/tmp/hello.py", []byte(`print("Hello!")`))
    if err != nil {
        log.Fatal(err)
    }

    // Read a file
    content, err := c.ReadFile(ctx, sb.ID, "/tmp/hello.py")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("File content: %s\n", string(content))

    // Run the script
    result, err = c.Exec(ctx, sb.ID, client.ExecOpts{
        Cmd: []string{"python3", "/tmp/hello.py"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Script output: %s", result.Stdout) // "Hello!\n"

    // Create a snapshot
    snap, err := c.CreateSnapshot(ctx, sb.ID, "after-setup")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Snapshot: %s\n", snap.ID)

    // Destroy sandbox
    err = c.DestroySandbox(ctx, sb.ID)
    if err != nil {
        log.Fatal(err)
    }

    // Restore from snapshot
    restored, err := c.RestoreSnapshot(ctx, snap.ID)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Restored sandbox: %s\n", restored.ID)

    // Clean up
    c.DestroySandbox(ctx, restored.ID)
}
```

### Client Options

```go
c := client.New("http://localhost:8080",
    client.WithAPIKey("key"),          // API key for authentication
    client.WithHTTPClient(customHTTP), // Custom http.Client
)
```

### Error Handling

```go
sb, err := c.CreateSandbox(ctx, config)
if err != nil {
    // err contains the HTTP status and error message
    fmt.Printf("Failed: %v\n", err)
}
```

---

## TypeScript SDK

### Install

```bash
bun add @den/sdk
# or
npm install @den/sdk
```

### Usage

```typescript
import { Den } from '@den/sdk';

const ah = new Den({
  url: 'http://localhost:8080',
  apiKey: 'your-api-key',
});

// Create a sandbox
const sandbox = await ah.sandbox.create({
  image: 'ubuntu:22.04',
  timeout: '30m',
});
console.log(`Sandbox: ${sandbox.id}`);

// Execute a command
const result = await sandbox.exec(['python3', '-c', 'print("hello")']);
console.log(result.stdout); // "hello\n"
console.log(result.exitCode); // 0

// Write and read files
await sandbox.writeFile('/tmp/test.py', 'print("world")');
const content = await sandbox.readFile('/tmp/test.py');
console.log(content); // 'print("world")'

// List directory
const files = await sandbox.listDir('/tmp');
files.forEach(f => console.log(`${f.name} (${f.size} bytes)`));

// Snapshot and restore
const snapshot = await sandbox.snapshot('checkpoint-1');
const restored = await ah.snapshot.restore(snapshot.id);

// Streaming exec (WebSocket)
const stream = await sandbox.execStream(['python3', 'long_task.py']);
for await (const msg of stream) {
  if (msg.type === 'stdout') process.stdout.write(msg.data);
  if (msg.type === 'stderr') process.stderr.write(msg.data);
  if (msg.type === 'exit') console.log(`Exit: ${msg.data}`);
}

// Clean up
await sandbox.destroy();
```

### Configuration

```typescript
const ah = new Den({
  url: 'http://localhost:8080',  // Server URL
  apiKey: 'your-key',            // Optional API key
  timeout: 30000,                // Default request timeout (ms)
});
```

---

## Python SDK

### Install

```bash
uv add den
# or
pip install den
```

### Usage

```python
from den import Den

ah = Den(
    url="http://localhost:8080",
    api_key="your-api-key",
)

# Create a sandbox
sandbox = ah.sandbox.create(image="ubuntu:22.04", timeout="30m")
print(f"Sandbox: {sandbox.id}")

# Execute a command
result = sandbox.exec(["python3", "-c", "print('hello')"])
print(result.stdout)     # "hello\n"
print(result.exit_code)  # 0

# Write and read files
sandbox.write_file("/tmp/test.py", "print('world')")
content = sandbox.read_file("/tmp/test.py")
print(content)  # "print('world')"

# List directory
files = sandbox.list_dir("/tmp")
for f in files:
    print(f"{f.name} ({f.size} bytes)")

# Snapshot and restore
snapshot = sandbox.snapshot("checkpoint-1")
restored = ah.snapshot.restore(snapshot.id)

# Clean up
sandbox.destroy()
```

### Async Usage

```python
import asyncio
from den import AsyncDen

async def main():
    ah = AsyncDen(
        url="http://localhost:8080",
        api_key="your-api-key",
    )

    sandbox = await ah.sandbox.create(image="ubuntu:22.04")
    result = await sandbox.exec(["echo", "async!"])
    print(result.stdout)

    await sandbox.destroy()

asyncio.run(main())
```

---

## Common Patterns

### Run Multiple Commands

```python
# Python
commands = [
    ["apt-get", "update"],
    ["apt-get", "install", "-y", "curl"],
    ["curl", "https://example.com"],
]

for cmd in commands:
    result = sandbox.exec(cmd, timeout=60)
    if result.exit_code != 0:
        print(f"Failed: {result.stderr}")
        break
```

### Project Setup Workflow

```typescript
// TypeScript
const sb = await ah.sandbox.create({ image: 'ubuntu:22.04' });

// Upload project files
await sb.writeFile('/home/sandbox/app.py', appCode);
await sb.writeFile('/home/sandbox/requirements.txt', requirements);

// Install dependencies
await sb.exec(['pip3', 'install', '-r', '/home/sandbox/requirements.txt']);

// Save state for later
const snap = await sb.snapshot('deps-installed');

// Run tests
const result = await sb.exec(['python3', '-m', 'pytest', '/home/sandbox/']);
console.log(result.exitCode === 0 ? 'Tests passed' : 'Tests failed');
```

### Parallel Sandboxes

```go
// Go — run tests in parallel
var wg sync.WaitGroup
results := make([]client.ExecResult, len(testFiles))

for i, file := range testFiles {
    wg.Add(1)
    go func(idx int, f string) {
        defer wg.Done()
        sb, _ := c.CreateSandbox(ctx, client.SandboxConfig{Image: "python:3.12"})
        defer c.DestroySandbox(ctx, sb.ID)

        c.WriteFile(ctx, sb.ID, "/tmp/test.py", []byte(f))
        results[idx], _ = c.Exec(ctx, sb.ID, client.ExecOpts{
            Cmd: []string{"python3", "/tmp/test.py"},
        })
    }(i, file)
}

wg.Wait()
```
