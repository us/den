<p align="center">
  <h1 align="center">Den</h1>
  <p align="center">面向 AI 智能体的自托管沙箱运行时</p>
  <p align="center">
    <a href="docs/zh-CN/getting-started.md">快速开始</a> &bull;
    <a href="docs/zh-CN/api-reference.md">API 参考</a> &bull;
    <a href="docs/zh-CN/sdk.md">SDK</a> &bull;
    <a href="docs/zh-CN/mcp.md">MCP 集成</a> &bull;
    <a href="docs/zh-CN/configuration.md">配置</a>
  </p>
  <p align="center">
    <a href="README.md">English</a> | <b>中文</b>
  </p>
</p>

---

Den 为 AI 智能体提供安全、隔离的沙箱环境来执行代码。它是 E2B 等云沙箱服务的开源、可自托管替代方案。

**单一可执行文件。零配置。兼容任何 AI 框架。**

```
curl -sSL https://get.den.dev | sh
den serve
```

## 为什么选择 Den？

AI 智能体需要运行代码，但在您的机器上运行不受信任的代码是危险的。Den 通过以下方式解决这个问题：

- **隔离容器** — 每个沙箱运行在独立的 Docker 容器中，具有最小权限、只读根文件系统、PID 限制和资源约束
- **简洁的 REST API** — 通过 HTTP 创建沙箱、执行命令、读写文件、管理快照
- **WebSocket 流式传输** — 实时命令输出，适用于交互式场景
- **MCP 服务器** — 原生支持 Model Context Protocol，兼容 Claude、Cursor 等 AI 工具
- **快照/恢复** — 保存沙箱状态并随时恢复，实现可复现的环境
- **Go + TypeScript + Python SDK** — 一流的客户端库

## 快速开始

### 前置要求

- Docker 已安装并运行
- Go 1.21+（从源码构建时需要）

### 运行服务器

```bash
# 构建并运行
go build -o den ./cmd/den
./den serve

# 或使用自定义配置
./den serve --config den.yaml
```

### 创建沙箱并运行代码

```bash
# 创建沙箱
curl -X POST http://localhost:8080/api/v1/sandboxes \
  -H 'Content-Type: application/json' \
  -d '{"image": "ubuntu:22.04"}'
# → {"id":"abc123","status":"running",...}

# 执行命令
curl -X POST http://localhost:8080/api/v1/sandboxes/abc123/exec \
  -H 'Content-Type: application/json' \
  -d '{"cmd": ["python3", "-c", "print(2+2)"]}'
# → {"exit_code":0,"stdout":"4\n","stderr":""}

# 写入文件
curl -X PUT 'http://localhost:8080/api/v1/sandboxes/abc123/files?path=/tmp/hello.py' \
  -d 'print("Hello from sandbox!")'

# 读取文件
curl 'http://localhost:8080/api/v1/sandboxes/abc123/files?path=/tmp/hello.py'

# 销毁沙箱
curl -X DELETE http://localhost:8080/api/v1/sandboxes/abc123
```

### 使用 Go SDK

```go
package main

import (
    "context"
    "fmt"

    client "github.com/den/den/pkg/client"
)

func main() {
    c := client.New("http://localhost:8080", client.WithAPIKey("your-key"))
    ctx := context.Background()

    // 创建沙箱
    sb, _ := c.CreateSandbox(ctx, client.SandboxConfig{
        Image: "ubuntu:22.04",
    })

    // 运行代码
    result, _ := c.Exec(ctx, sb.ID, client.ExecOpts{
        Cmd: []string{"echo", "Hello from Go SDK!"},
    })
    fmt.Println(result.Stdout)

    // 清理
    c.DestroySandbox(ctx, sb.ID)
}
```

### 使用 MCP（Claude Code、Cursor）

```bash
# 启动 MCP 服务器（stdio 模式）
den mcp
```

添加到 Claude Code 配置文件（`~/.claude/claude_desktop_config.json`）：

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

现在 Claude 可以直接创建沙箱、运行代码和管理文件了。

## 功能特性

| 功能 | 描述 |
|------|------|
| **沙箱 CRUD** | 创建、列出、获取、停止、销毁容器 |
| **命令执行** | 同步执行，返回退出码、stdout、stderr |
| **流式执行** | 基于 WebSocket 的实时输出 |
| **文件操作** | 在沙箱内读取、写入、列出、创建目录、删除文件 |
| **文件上传/下载** | 多部分上传和直接下载 |
| **快照** | 通过 `docker commit` 保存和恢复沙箱状态 |
| **端口转发** | 将沙箱端口暴露到主机（绑定到 127.0.0.1） |
| **资源限制** | 每个沙箱的 CPU、内存、PID 限制 |
| **自动过期** | 沙箱在可配置的超时后自动销毁 |
| **速率限制** | 所有 API 端点的每密钥速率限制 |
| **API 密钥认证** | 基于 Header 的认证，使用常量时间比较 |
| **MCP 服务器** | 基于 stdio 的 Model Context Protocol，用于 AI 工具集成 |
| **控制面板** | 内嵌的 Web UI，用于监控和管理 |

## 安全性

Den 非常重视安全性。每个沙箱运行时具有：

- **最小权限** — 丢弃 `ALL` 能力，仅添加最小必要集
- **只读根文件系统** — 仅 tmpfs 挂载点（`/tmp`、`/home/sandbox`）可写
- **PID 限制** — 每个容器默认最多 256 个进程
- **禁止提权** — `no-new-privileges` 安全选项
- **网络隔离** — 容器运行在内部 Docker 网络中
- **端口绑定** — 转发端口仅绑定到 `127.0.0.1`
- **路径验证** — 对所有文件操作进行空字节和路径遍历防护
- **常量时间认证** — API 密钥比较抗时序攻击
- **无错误泄露** — 内部错误记录日志，向客户端返回通用消息

## 架构

```
┌──────────────────────────────────────────────────────┐
│                     客户端                            │
│  CLI  │  Go SDK  │  TS SDK  │  Python SDK  │  MCP   │
└───────┴──────────┴──────────┴──────────────┴────────┘
                          │
                    ┌─────┴─────┐
                    │  HTTP API  │  chi 路由 + 中间件
                    │  WebSocket │  gorilla/websocket
                    └─────┬─────┘
                          │
                    ┌─────┴─────┐
                    │   引擎    │  生命周期、回收、限制
                    └─────┬─────┘
                          │
                ┌─────────┴─────────┐
                │  Docker 运行时    │  Docker SDK
                └─────────┬─────────┘
                          │
              ┌───────────┴───────────┐
              │    Docker 容器        │
              │   （隔离的沙箱）       │
              └───────────────────────┘
```

## 性能

在 Apple Silicon（M 系列）上的基准测试：

| 操作 | 延迟 |
|------|------|
| API 健康检查 | < 1ms |
| 创建沙箱 | ~100-160ms |
| 执行命令 | ~20-30ms |
| 读取文件 | ~28-30ms |
| 写入文件 | ~56-70ms |
| 并行吞吐 | ~66 req/s |

## 文档

- [快速开始](docs/zh-CN/getting-started.md) — 安装、首个沙箱、基本用法
- [API 参考](docs/zh-CN/api-reference.md) — 完整的 REST API 文档
- [配置指南](docs/zh-CN/configuration.md) — 所有配置选项说明
- [SDK 指南](docs/zh-CN/sdk.md) — Go、TypeScript 和 Python 客户端库
- [MCP 集成](docs/zh-CN/mcp.md) — 与 AI 工具配合使用
- [架构设计](docs/zh-CN/architecture.md) — 内部设计和安全模型
- [CLI 参考](docs/zh-CN/cli.md) — 命令行工具

## CLI

```
den serve                         # 启动 API 服务器
den create --image ubuntu:22.04   # 创建沙箱
den ls                            # 列出沙箱
den exec <id> -- echo hello       # 执行命令
den rm <id>                       # 销毁沙箱
den snapshot create <id>          # 创建快照
den snapshot restore <snap-id>    # 恢复快照
den stats                         # 系统统计
den mcp                           # 启动 MCP 服务器
den version                       # 版本信息
```

## 配置

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  rate_limit_rps: 10
  rate_limit_burst: 20

sandbox:
  default_image: "ubuntu:22.04"
  default_timeout: "30m"
  max_sandboxes: 50
  default_memory: 536870912  # 512MB

auth:
  enabled: true
  api_keys:
    - "your-secret-key"
```

查看[配置指南](docs/zh-CN/configuration.md)了解所有选项。

## 贡献

```bash
# 克隆和构建
git clone https://github.com/den/den
cd den
go build ./cmd/den

# 运行测试
go test ./internal/... -race

# 使用竞态检测器运行
go test ./internal/... -count=1 -race -v
```

## 许可证

AGPL-3.0 — 详见 [LICENSE](LICENSE)。
