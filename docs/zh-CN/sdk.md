---
title: SDK 指南
---

# SDK 指南

[English](../sdk.md) | **中文**

Den 提供 Go、TypeScript 和 Python 客户端 SDK。所有 SDK 封装 REST API，为每种语言提供惯用接口。

## Go SDK

### 安装

```bash
go get github.com/den/den
```

### 使用

```go
package main

import (
    "context"
    "fmt"
    "log"

    client "github.com/den/den/pkg/client"
)

func main() {
    // 创建客户端
    c := client.New("http://localhost:8080",
        client.WithAPIKey("your-api-key"),
    )
    ctx := context.Background()

    // 创建沙箱
    sb, err := c.CreateSandbox(ctx, client.SandboxConfig{
        Image:   "ubuntu:22.04",
        Timeout: "30m",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("沙箱已创建: %s\n", sb.ID)

    // 执行命令
    result, err := c.Exec(ctx, sb.ID, client.ExecOpts{
        Cmd:     []string{"python3", "-c", "print(2 + 2)"},
        Timeout: 30,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("输出: %s", result.Stdout)

    // 写入文件
    c.WriteFile(ctx, sb.ID, "/tmp/hello.py", []byte(`print("Hello!")`))

    // 读取文件
    content, _ := c.ReadFile(ctx, sb.ID, "/tmp/hello.py")
    fmt.Printf("文件内容: %s\n", string(content))

    // 创建快照
    snap, _ := c.CreateSnapshot(ctx, sb.ID, "安装后")
    fmt.Printf("快照: %s\n", snap.ID)

    // 销毁沙箱
    c.DestroySandbox(ctx, sb.ID)

    // 从快照恢复
    restored, _ := c.RestoreSnapshot(ctx, snap.ID)
    fmt.Printf("已恢复沙箱: %s\n", restored.ID)
    c.DestroySandbox(ctx, restored.ID)
}
```

---

## TypeScript SDK

### 安装

```bash
bun add @den/sdk
# 或
npm install @den/sdk
```

### 使用

```typescript
import { Den } from '@den/sdk';

const ah = new Den({
  url: 'http://localhost:8080',
  apiKey: 'your-api-key',
});

// 创建沙箱
const sandbox = await ah.sandbox.create({
  image: 'ubuntu:22.04',
  timeout: '30m',
});

// 执行命令
const result = await sandbox.exec(['python3', '-c', 'print("hello")']);
console.log(result.stdout); // "hello\n"

// 写入和读取文件
await sandbox.writeFile('/tmp/test.py', 'print("world")');
const content = await sandbox.readFile('/tmp/test.py');

// 快照和恢复
const snapshot = await sandbox.snapshot('检查点-1');
const restored = await ah.snapshot.restore(snapshot.id);

// 流式执行（WebSocket）
const stream = await sandbox.execStream(['python3', 'long_task.py']);
for await (const msg of stream) {
  if (msg.type === 'stdout') process.stdout.write(msg.data);
  if (msg.type === 'exit') console.log(`退出码: ${msg.data}`);
}

// 清理
await sandbox.destroy();
```

---

## Python SDK

### 安装

```bash
uv add den
# 或
pip install den
```

### 使用

```python
from den import Den

ah = Den(
    url="http://localhost:8080",
    api_key="your-api-key",
)

# 创建沙箱
sandbox = ah.sandbox.create(image="ubuntu:22.04", timeout="30m")

# 执行命令
result = sandbox.exec(["python3", "-c", "print('hello')"])
print(result.stdout)     # "hello\n"
print(result.exit_code)  # 0

# 写入和读取文件
sandbox.write_file("/tmp/test.py", "print('world')")
content = sandbox.read_file("/tmp/test.py")

# 快照和恢复
snapshot = sandbox.snapshot("检查点-1")
restored = ah.snapshot.restore(snapshot.id)

# 清理
sandbox.destroy()
```

### 异步使用

```python
import asyncio
from den import AsyncDen

async def main():
    ah = AsyncDen(url="http://localhost:8080", api_key="your-api-key")
    sandbox = await ah.sandbox.create(image="ubuntu:22.04")
    result = await sandbox.exec(["echo", "异步!"])
    print(result.stdout)
    await sandbox.destroy()

asyncio.run(main())
```

---

## 常见模式

### 运行多个命令

```python
commands = [
    ["apt-get", "update"],
    ["apt-get", "install", "-y", "curl"],
    ["curl", "https://example.com"],
]

for cmd in commands:
    result = sandbox.exec(cmd, timeout=60)
    if result.exit_code != 0:
        print(f"失败: {result.stderr}")
        break
```

### 并行沙箱

```go
var wg sync.WaitGroup
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
