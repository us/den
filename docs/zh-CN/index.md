---
title: Den 文档
description: 面向 AI 智能体的自托管沙箱运行时
---

# Den 文档

[English](../index.md) | **中文**

Den 为 AI 智能体提供安全、隔离的沙箱环境来执行代码。它是云沙箱服务的开源、可自托管替代方案。

## 概览

Den 作为单一可执行文件运行，通过 Docker API 管理容器作为沙箱。AI 智能体可通过 REST API、WebSocket、MCP 协议或客户端 SDK 与其交互。

```
智能体 → Den API → Docker → 隔离容器
```

每个沙箱是一个 Docker 容器，具有：
- 最小权限和只读根文件系统
- 资源限制（CPU、内存、PID）
- 可配置超时后自动过期
- 完整的文件 I/O 和命令执行

## 文档

| 指南 | 描述 |
|------|------|
| [快速开始](getting-started.md) | 安装、配置、创建首个沙箱 |
| [API 参考](api-reference.md) | 完整的 REST API 及示例 |
| [配置指南](configuration.md) | 所有配置选项及默认值 |
| [SDK 指南](sdk.md) | Go、TypeScript 和 Python 客户端 |
| [MCP 集成](mcp.md) | 与 Claude Code、Cursor 等 AI 工具配合使用 |
| [架构设计](architecture.md) | 内部设计、安全模型、数据流 |
| [CLI 参考](cli.md) | 所有 CLI 命令和参数 |

## 快速示例

```bash
# 启动服务器
den serve

# 创建沙箱
curl -X POST http://localhost:8080/api/v1/sandboxes \
  -H 'Content-Type: application/json' \
  -d '{"image": "ubuntu:22.04"}'

# 运行代码
curl -X POST http://localhost:8080/api/v1/sandboxes/{id}/exec \
  -H 'Content-Type: application/json' \
  -d '{"cmd": ["python3", "-c", "print(42)"]}'
```
