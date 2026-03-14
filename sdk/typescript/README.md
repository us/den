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

const client = new Den("http://localhost:8080", { apiKey: "your-key" });

// Create a sandbox
const sandbox = await client.create({ image: "ubuntu:22.04" });

// Execute a command
const result = await sandbox.exec(["python3", "-c", "print('Hello from Den!')"]);
console.log(result.stdout); // Hello from Den!

// Read/write files
await sandbox.writeFile("/tmp/hello.py", "print('hello world')");
const content = await sandbox.readFile("/tmp/hello.py");

// Clean up
await sandbox.destroy();
```

## Storage

```typescript
import { Den } from "@us4/den";

const client = new Den("http://localhost:8080");

// Persistent volume
const sandbox = await client.create({
  image: "ubuntu:22.04",
  storage: {
    volumes: [{ name: "my-data", mountPath: "/data" }],
  },
});

// S3 import/export
await sandbox.s3Import({
  bucket: "my-bucket",
  key: "data/input.csv",
  destPath: "/home/sandbox/input.csv",
});

await sandbox.s3Export({
  sourcePath: "/home/sandbox/output.csv",
  bucket: "my-bucket",
  key: "results/output.csv",
});
```

## Snapshots

```typescript
// Save state
const snapshot = await sandbox.snapshot("after-setup");

// Restore later
const restored = await client.restoreSnapshot(snapshot.id);
```

## WebSocket Streaming

```typescript
// Stream command output in real-time
const stream = await sandbox.execStream(["python3", "long_script.py"]);

for await (const message of stream) {
  if (message.type === "stdout") process.stdout.write(message.data);
  if (message.type === "stderr") process.stderr.write(message.data);
  if (message.type === "exit") console.log(`Exit code: ${message.data}`);
}
```

## Features

- Sandbox lifecycle management (create, list, get, stop, destroy)
- Command execution with timeout and environment variables
- WebSocket streaming for real-time output
- File operations (read, write, list, mkdir, delete, upload, download)
- Persistent volumes, shared volumes, tmpfs configuration
- S3 integration (hooks, on-demand, FUSE)
- Snapshot/restore
- Port forwarding
- Full TypeScript types

## Requirements

- Node.js >= 18 or Bun
- Den server running (see [Den repo](https://github.com/us/den))

## License

MIT
