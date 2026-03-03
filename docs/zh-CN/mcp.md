---
title: MCP 集成
---

# MCP 集成

[English](../mcp.md) | **中文**

Den 内置了 [Model Context Protocol](https://modelcontextprotocol.io/)（MCP）服务器。AI 工具如 Claude Code、Cursor 等可以直接创建沙箱、运行代码和管理文件。

## 快速开始

```bash
# 启动 Den API 服务器（一个终端）
den serve

# 启动 MCP 服务器（另一个终端，或在 AI 工具中配置）
den mcp
```

MCP 服务器通过 **stdio**（stdin/stdout）使用 JSON-RPC 2.0 通信。

## Claude Code 配置

添加到 Claude Code MCP 配置文件（`~/.claude.json` 或项目 `.mcp.json`）：

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

使用配置文件：

```json
{
  "mcpServers": {
    "den": {
      "command": "den",
      "args": ["mcp", "--config", "/path/to/den.yaml"]
    }
  }
}
```

## Cursor 配置

添加到 Cursor MCP 设置（`.cursor/mcp.json`）：

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

## 可用工具

MCP 服务器提供九个工具：

| 工具 | 描述 | 必需参数 |
|------|------|----------|
| `create_sandbox` | 创建新的隔离沙箱 | 无（image 可选） |
| `exec` | 在沙箱中执行命令 | `sandbox_id`, `cmd` |
| `read_file` | 从沙箱读取文件 | `sandbox_id`, `path` |
| `write_file` | 写入文件到沙箱 | `sandbox_id`, `path`, `content` |
| `list_files` | 列出目录内容 | `sandbox_id`, `path` |
| `destroy_sandbox` | 停止并移除沙箱 | `sandbox_id` |
| `list_sandboxes` | 列出所有沙箱 | 无 |
| `snapshot_create` | 创建沙箱快照 | `sandbox_id` |
| `snapshot_restore` | 从快照恢复 | `snapshot_id` |

## 使用示例

配置完成后，您可以自然地要求 Claude 使用沙箱：

> **您：** 在沙箱中运行这个 Python 脚本并显示输出：
> ```python
> import sys
> print(f"Python {sys.version}")
> print(sum(range(1000)))
> ```

Claude 将会：
1. 调用 `create_sandbox` 创建环境
2. 调用 `write_file` 保存脚本
3. 调用 `exec` 运行脚本
4. 返回输出给您
5. 调用 `destroy_sandbox` 清理

## 架构

```
AI 工具（Claude Code / Cursor）
  ↓ stdio（JSON-RPC 2.0）
MCP 服务器（den mcp）
  ↓ 进程内调用
引擎 + Docker 运行时
  ↓ Docker API
沙箱容器
```

MCP 服务器创建自己的 Engine 和 Docker Runtime 实例，直接连接 Docker，不经过 HTTP API 服务器。

## 配置

MCP 服务器使用与 API 服务器相同的配置文件：

```bash
den mcp --config den.yaml
```

相关配置选项：
- `runtime.docker_host` — Docker 套接字路径
- `sandbox.*` — 默认沙箱设置
- `log.level` — MCP 服务器日志级别（日志输出到 stderr）
