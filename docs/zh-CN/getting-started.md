---
title: 快速开始
---

# 快速开始

[English](../getting-started.md) | **中文**

## 前置要求

- **Docker** — 已安装并运行（Den 通过 Docker API 管理容器）
- **Go 1.21+** — 从源码构建时需要

验证 Docker 是否运行：

```bash
docker info
```

## 安装

### 从源码构建

```bash
git clone https://github.com/den/den
cd den
go build -o den ./cmd/den
```

### 二进制发行版

```bash
# macOS / Linux
curl -sSL https://get.den.dev | sh

# 或从 GitHub Releases 下载
```

## 启动服务器

```bash
# 使用默认配置（端口 8080，无认证）
./den serve

# 使用配置文件
./den serve --config den.yaml
```

服务器将会：
1. 连接到 Docker
2. 创建 `den-net` 网络（如果需要）
3. 从 BoltDB 存储中恢复已持久化的沙箱
4. 在端口 8080 启动 HTTP API

## 创建您的第一个沙箱

### 1. 创建沙箱

```bash
curl -s -X POST http://localhost:8080/api/v1/sandboxes \
  -H 'Content-Type: application/json' \
  -d '{"image": "ubuntu:22.04"}' | jq
```

响应：
```json
{
  "id": "d6jcj6a9qf76oti2r2sg",
  "image": "ubuntu:22.04",
  "status": "running",
  "created_at": "2026-03-03T11:44:25.809Z",
  "expires_at": "2026-03-03T12:14:25.809Z"
}
```

保存 `id` 供后续步骤使用：
```bash
export SB_ID="d6jcj6a9qf76oti2r2sg"
```

### 2. 执行命令

```bash
curl -s -X POST "http://localhost:8080/api/v1/sandboxes/$SB_ID/exec" \
  -H 'Content-Type: application/json' \
  -d '{"cmd": ["echo", "Hello from sandbox!"]}' | jq
```

响应：
```json
{
  "exit_code": 0,
  "stdout": "Hello from sandbox!\n",
  "stderr": ""
}
```

### 3. 写入文件

```bash
curl -s -X PUT "http://localhost:8080/api/v1/sandboxes/$SB_ID/files?path=/tmp/hello.py" \
  -d 'print("Hello World!")'
```

### 4. 读取文件

```bash
curl -s "http://localhost:8080/api/v1/sandboxes/$SB_ID/files?path=/tmp/hello.py"
# → print("Hello World!")
```

### 5. 运行脚本

> **注意：** 默认的 `ubuntu:22.04` 镜像不包含 Python。请使用 `python:3.12` 镜像或先运行 `apt-get install -y python3` 安装。

```bash
curl -s -X POST "http://localhost:8080/api/v1/sandboxes/$SB_ID/exec" \
  -H 'Content-Type: application/json' \
  -d '{"cmd": ["bash", "-c", "cat /tmp/hello.py"]}' | jq .stdout
# → "print(\"Hello World!\")\n"
```

### 6. 清理

```bash
curl -s -X DELETE "http://localhost:8080/api/v1/sandboxes/$SB_ID"
# → 204 No Content
```

## 启用认证

在生产环境中，启用 API 密钥认证：

```yaml
# den.yaml
auth:
  enabled: true
  api_keys:
    - "your-secret-key-here"
```

然后在请求中传递密钥：

```bash
curl -H 'X-API-Key: your-secret-key-here' \
  http://localhost:8080/api/v1/sandboxes
```

## 使用 CLI

Den 内置了 CLI 客户端：

```bash
# 创建沙箱
den create --image ubuntu:22.04

# 列出沙箱
den ls

# 执行命令
den exec <id> -- python3 -c "print('hello')"

# 删除沙箱
den rm <id>
```

CLI 默认连接到 `http://localhost:8080`。使用 `--server` 覆盖：

```bash
den --server http://remote:8080 ls
```

## 使用 MCP 服务器

用于 AI 工具集成（Claude Code、Cursor）：

```bash
den mcp
```

查看 [MCP 集成](mcp.md) 获取设置说明。

## 下一步

- [API 参考](api-reference.md) — 所有端点文档
- [配置指南](configuration.md) — 调整资源限制、认证、网络
- [SDK 指南](sdk.md) — 使用 Go、TypeScript 或 Python 客户端
