# @us4/den

TypeScript SDK for [Den](https://github.com/us/den) — the self-hosted sandbox runtime for AI agents.

> **100 sandboxes on E2B = ~$600/hour. 100 sandboxes on Den = one $5/month server.**

## Installation

```bash
bun add @us4/den
# or
npm install @us4/den
```

## Quick Start

```typescript
import { Den } from "@us4/den";

const client = new Den({ url: "http://localhost:8080", apiKey: "your-key" });

// Create a sandbox
const sandbox = await client.sandbox.create({ image: "ubuntu:22.04" });

// Execute a command
const result = await sandbox.exec(["python3", "-c", "print('Hello from Den!')"]);
console.log(result.stdout); // Hello from Den!

// Read/write files
await sandbox.writeFile("/tmp/hello.py", "print('hello world')");
const content = await sandbox.readFile("/tmp/hello.py");

// List files
const files = await sandbox.listFiles("/tmp");

// Clean up
await sandbox.destroy();
```

## Storage

```typescript
import { Den } from "@us4/den";

const client = new Den({ url: "http://localhost:8080" });

// Persistent volume
const sandbox = await client.sandbox.create({
  image: "ubuntu:22.04",
  storage: {
    volumes: [{ name: "my-data", mountPath: "/data" }],
  },
});
```

## Snapshots

```typescript
// Save state
const snapshot = await sandbox.snapshot("after-setup");

// Restore from snapshot
const restored = await client.sandbox.restoreSnapshot(snapshot.id);
```

## Sandbox Management

```typescript
// List all sandboxes
const sandboxes = await client.sandbox.list();

// Get a specific sandbox
const sb = await client.sandbox.get("sandbox-id");

// Stop a sandbox
await sandbox.stop();

// Get sandbox stats
const stats = await sandbox.stats();

// Delete a snapshot
await client.sandbox.deleteSnapshot(snapshot.id);
```

## Features

- Sandbox lifecycle management (create, list, get, stop, destroy)
- Command execution with timeout and environment variables
- File operations (read, write, list, mkdir, delete)
- Persistent volumes, shared volumes, tmpfs configuration
- Snapshot/restore
- Port forwarding
- Full TypeScript types

## Requirements

- Node.js >= 18 or Bun
- Den server running (see [Den repo](https://github.com/us/den))

## License

MIT
