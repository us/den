---
title: CLI 参考
---

# CLI 参考

[English](../cli.md) | **中文**

Den 是一个单一可执行文件，同时作为服务器和 CLI 客户端使用。

## 全局参数

```
--config string    配置文件路径（默认：den.yaml）
--server string    客户端命令的 API 服务器地址（默认：http://localhost:8080）
```

服务器地址也可通过 `DEN_URL` 环境变量设置。

---

## 服务器

### `den serve`

启动 HTTP API 服务器。

```bash
# 使用默认配置
den serve

# 使用配置文件
den serve --config production.yaml
```

按 `Ctrl+C` 优雅关闭（销毁所有运行中的沙箱）。

---

## 沙箱管理

### `den create`

创建新沙箱。

```bash
# 默认沙箱
den create

# 自定义镜像和超时
den create --image python:3.12 --timeout 1h

# 资源限制
den create --image node:20 --memory 268435456 --cpu 500000000
```

参数：`--image`、`--timeout`、`--cpu`、`--memory`

### `den ls`

列出所有沙箱。

```
ID                     IMAGE           STATUS    CREATED              EXPIRES
d6jcj6a9qf76oti2r2sg  ubuntu:22.04    running   2026-03-03 11:44:25  2026-03-03 12:14:25
```

### `den exec`

在沙箱中执行命令。

```bash
den exec d6jcj6a9qf76oti2r2sg -- echo "你好！"
den exec d6jcj6a9qf76oti2r2sg -- python3 -c "print(2+2)"
den exec d6jcj6a9qf76oti2r2sg -- bash -c "ls -la /tmp && echo 完成"
```

命令的 stdout 打印到终端。非零退出码反映在 CLI 的退出码中。

### `den rm`

销毁沙箱（停止并移除）。

```bash
den rm d6jcj6a9qf76oti2r2sg
```

---

## 快照

### `den snapshot create`

```bash
den snapshot create <sandbox-id>
den snapshot create <sandbox-id> --name "安装后"
```

### `den snapshot ls`

```bash
# 列出所有快照
den snapshot ls

# 列出特定沙箱的快照
den snapshot ls <sandbox-id>
```

### `den snapshot restore`

```bash
den snapshot restore <snapshot-id>
```

从快照创建新的运行沙箱。

---

## 统计

### `den stats`

```bash
# 系统统计
den stats

# 特定沙箱统计
den stats <sandbox-id>
```

---

## MCP 服务器

### `den mcp`

启动 MCP 服务器（stdio 模式），供 AI 工具使用。

```bash
den mcp
den mcp --config den.yaml
```

参见 [MCP 集成](mcp.md)。

---

## 版本

### `den version`

```bash
den version
# → den v0.1.0 (abc1234) built 2026-03-03
```

---

## 退出码

| 代码 | 含义 |
|------|------|
| 0 | 成功 |
| 1 | 一般错误 |
| 2 | 无效用法 / 参数错误 |

`den exec` 的退出码与沙箱内命令的退出码一致。

## 环境变量

| 变量 | 描述 |
|------|------|
| `DEN_URL` | API 服务器地址（默认 `http://localhost:8080`） |
| `DEN_API_KEY` | 认证 API 密钥 |
| `DEN_SERVER__PORT` | 覆盖服务器端口 |
| `DEN_LOG__LEVEL` | 日志级别 |

查看[配置指南](configuration.md)了解所有环境变量选项。
