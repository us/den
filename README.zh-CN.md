<p align="center">
  <h1 align="center">Den</h1>
  <p align="center">面向 AI 智能体的自托管沙箱运行时</p>
  <p align="center">
    <a href="docs/docs/quick-start.md">快速开始</a> &bull;
    <a href="docs/api-reference.md">API 参考</a> &bull;
    <a href="docs/docs/sdks.md">SDK</a> &bull;
    <a href="docs/docs/mcp.md">MCP 集成</a> &bull;
    <a href="docs/docs/configuration.md">配置</a>
  </p>
  <p align="center">
    <a href="README.md">English</a> | <b>中文</b>
  </p>
</p>

---

Den 为 AI 智能体提供安全、隔离的沙箱环境来执行代码。它是 E2B 等云沙箱服务的开源、可自托管替代方案。

**单一可执行文件。零配置。兼容任何 AI 框架。**

> **E2B 上 100 个沙箱 = 每小时约 $600。Den 上 100 个沙箱 = 每月 $5 的服务器。**

```
curl -sSL https://get.den.dev | sh
den serve
```

## 最新更新

### 共享资源管理 (v0.0.6)

- **内存压力监控** — 实时 5 级压力系统（正常 → 警告 → 高 → 严重 → 紧急），具有滞后防抖
- **动态内存节流** — 基于主机压力自动调整每个容器的 cgroup v2 `memory.high`
- **压力感知调度** — 在严重/紧急级别时拒绝创建新沙箱（HTTP 503）
- **资源状态 API** — `GET /api/v1/resources` 查询主机内存、压力级别和沙箱指标
- **平台支持** — Linux（直接 cgroup v2, `/proc/meminfo`）和 macOS（Docker API 回退）
- **自动恢复** — 压力降低时自动移除内存限制

### 存储层 (v0.0.5)

- **持久化和共享卷** — Docker 命名卷，跨沙箱挂载（读写/只读）
- **S3 集成** — 钩子同步、按需导入/导出、FUSE 挂载
- **Go、TypeScript (`@us4/den`)、Python (`den-sdk`) SDK** — 完整存储类型支持

详见 [CHANGELOG.md](CHANGELOG.md) 了解完整版本历史。

## 为什么选择 Den？

AI 智能体需要运行代码，但在您的机器上运行不受信任的代码是危险的。Den 通过以下方式解决这个问题：

- **隔离容器** — 每个沙箱运行在独立的 Docker 容器中，具有最小权限、只读根文件系统、PID 限制和资源约束
- **共享资源模型** — 容器智能共享主机内存，而非固定分配。动态压力监控 + 自动节流（Google Borg / AWS Firecracker 方式）。10 倍超分配 = 每美元多 10 倍沙箱
- **简洁的 REST API** — 通过 HTTP 创建沙箱、执行命令、读写文件、管理快照
- **WebSocket 流式传输** — 实时命令输出，适用于交互式场景
- **MCP 服务器** — 原生支持 Model Context Protocol，兼容 Claude、Cursor 等 AI 工具
- **快照/恢复** — 保存沙箱状态并随时恢复，实现可复现的环境
- **存储** — 持久化卷、共享卷、可配置 tmpfs、S3 集成
- **Go + TypeScript + Python SDK** — 一流的客户端库

## 安装

```bash
# Go
go get github.com/us/den@latest

# TypeScript
bun add @us4/den
# 或: npm install @us4/den

# Python
pip install den-sdk
# 或: uv add den-sdk
```

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

    client "github.com/us/den/pkg/client"
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
| **持久化卷** | Docker 命名卷，沙箱销毁后数据保留 |
| **共享卷** | 多个沙箱挂载同一卷（读写或只读） |
| **可配置 Tmpfs** | 每个沙箱的 tmpfs 大小和选项覆盖 |
| **S3 同步** | 通过钩子、按需 API 或 FUSE 挂载导入/导出文件 |
| **端口转发** | 将沙箱端口暴露到主机（绑定到 127.0.0.1） |
| **资源限制** | 每个沙箱的 CPU、内存、PID 限制 |
| **压力监控** | 主机内存压力检测 + 动态节流 |
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
- **动态内存节流** — 基于 cgroup v2 `memory.high` 的节流，而非硬杀；5 级压力系统 + 自动恢复
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
                    │   引擎    │  生命周期、回收、压力监控
                    └──┬────┬──┘
                       │    │
          ┌────────────┘    └────────────┐
  ┌───────┴───────┐           ┌──────────┴─────────┐
  │ Docker 运行时 │           │     存储层         │
  │  Docker SDK   │           │ 卷、S3、Tmpfs      │
  └───────┬───────┘           └──────────┬─────────┘
          │                              │
  ┌───────┴───────┐           ┌──────────┴─────────┐
  │  Docker 容器  │           │  S3 / MinIO        │
  │  （沙箱）     │           │  Docker 卷         │
  └───────────────┘           └────────────────────┘
```

## 性能

在 Apple Silicon（M 系列）上的基准测试：

| 操作 | 延迟 | 说明 |
|------|------|------|
| API 健康检查 | < 1ms | 近零开销 |
| 创建沙箱 | ~100ms | 冷启动；预热池可降至 ~5ms |
| 执行命令 | ~20-30ms | 包含 Docker exec 往返 |
| 读取文件 | ~28-30ms | 基于 exec 的文件 I/O |
| 写入文件 | ~56-70ms | 基于 exec + 自动创建目录 |
| 销毁沙箱 | ~1s | SIGTERM + 清理 |
| 并行创建 (5x) | ~42ms/个 | 并发容器创建 |
| 并行执行 (10x) | ~7ms/个 | 并发命令执行 |

### 与竞品对比

| | **Den** | E2B | Daytona | Modal |
|---|---|---|---|---|
| 沙箱创建 | **~100ms** | ~150ms | ~90ms | 2-5s |
| 价格 | **免费** | $0.10/分钟+ | 免费（复杂） | $0.10/分钟+ |
| 每服务器最大沙箱 | **100+（共享资源）** | ~10（固定分配） | ~10（K8s pods） | 不适用（云） |
| 安装 | **`curl \| sh`** | SDK + API key | Docker + K8s | SDK + API key |
| 自托管 | **简单（单二进制）** | 困难（Firecracker+Nomad） | 繁重（K8s） | 否 |
| 离线运行 | **是** | 否 | 部分 | 否 |
| 许可证 | AGPL-3.0 | Apache-2.0 | Apache-2.0 | 专有 |

## 文档

- [快速开始](docs/docs/quick-start.md) — 安装、首个沙箱、基本用法
- [API 参考](docs/api-reference.md) — 完整的 REST API 文档
- [配置指南](docs/docs/configuration.md) — 所有配置选项说明
- [SDK 指南](docs/docs/sdks.md) — Go、TypeScript 和 Python 客户端库
- [MCP 集成](docs/docs/mcp.md) — 与 AI 工具配合使用
- [架构设计](docs/docs/architecture.md) — 内部设计和安全模型
- [CLI 参考](docs/cli.md) — 命令行工具

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
  allow_volumes: true
  allow_s3: true
  max_volumes_per_sandbox: 5

s3:
  endpoint: "http://localhost:9000"  # MinIO or S3-compatible
  region: "us-east-1"
  access_key: "minioadmin"
  secret_key: "minioadmin"

auth:
  enabled: true
  api_keys:
    - "your-secret-key"

resource:
  overcommit_ratio: 10.0
  monitor_interval: "5s"
  enable_auto_throttle: true
```

查看[配置指南](docs/docs/configuration.md)了解所有选项。

## 贡献

```bash
# 克隆和构建
git clone https://github.com/us/den
cd den
go build ./cmd/den

# 运行测试
go test ./internal/... -race

# 使用竞态检测器运行
go test ./internal/... -count=1 -race -v
```

## 许可证

AGPL-3.0 — 详见 [LICENSE](LICENSE)。
