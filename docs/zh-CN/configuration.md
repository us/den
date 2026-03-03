---
title: 配置指南
---

# 配置指南

[English](../configuration.md) | **中文**

Den 通过 YAML 文件、环境变量或 CLI 参数进行配置。值按以下顺序合并（后者覆盖前者）：

1. 内置默认值
2. YAML 配置文件
3. 环境变量

## 配置文件

通过 `--config` 参数传递配置文件：

```bash
den serve --config den.yaml
```

## 完整配置参考

```yaml
# den.yaml — 所有选项及默认值

server:
  host: "0.0.0.0"            # 监听地址
  port: 8080                  # 监听端口
  allowed_origins:            # CORS 和 WebSocket 允许的源
    - "http://localhost:8080"
    - "http://127.0.0.1:8080"
  rate_limit_rps: 10          # 每秒每密钥/IP 的请求数
  rate_limit_burst: 20        # 突发允许量
  tls:
    cert_file: ""             # TLS 证书路径
    key_file: ""              # TLS 私钥路径

runtime:
  backend: "docker"           # 运行时后端（仅支持 "docker"）
  docker_host: ""             # Docker 套接字（空 = 默认）
  network_id: ""              # 自定义 Docker 网络（空 = 自动创建）

sandbox:
  default_image: "ubuntu:22.04"  # 默认容器镜像
  default_timeout: "30m"         # 默认沙箱生存时间
  max_sandboxes: 50              # 最大并发沙箱数
  default_cpu: 1000000000        # CPU 限制，NanoCPU（1 核）
  default_memory: 536870912      # 内存限制，字节（512MB）
  default_pid_limit: 256         # 每沙箱最大进程数
  warm_pool_size: 0              # 预创建的待命沙箱数

store:
  path: "den.db"       # BoltDB 数据库文件路径

auth:
  enabled: false              # 启用 API 密钥认证
  api_keys:                   # 有效 API 密钥列表
    - "your-secret-key-here"

log:
  level: "info"               # 日志级别：debug, info, warn, error
  format: "text"              # 日志格式：text, json
```

## 环境变量

每个配置选项都可通过环境变量设置。使用前缀 `DEN_`，嵌套分隔符为 `__`（双下划线）：

| 配置路径 | 环境变量 |
|----------|----------|
| `server.host` | `DEN_SERVER__HOST` |
| `server.port` | `DEN_SERVER__PORT` |
| `server.rate_limit_rps` | `DEN_SERVER__RATE_LIMIT_RPS` |
| `sandbox.default_image` | `DEN_SANDBOX__DEFAULT_IMAGE` |
| `sandbox.default_timeout` | `DEN_SANDBOX__DEFAULT_TIMEOUT` |
| `sandbox.max_sandboxes` | `DEN_SANDBOX__MAX_SANDBOXES` |
| `sandbox.default_memory` | `DEN_SANDBOX__DEFAULT_MEMORY` |
| `auth.enabled` | `DEN_AUTH__ENABLED` |
| `store.path` | `DEN_STORE__PATH` |
| `log.level` | `DEN_LOG__LEVEL` |

示例：

```bash
DEN_SERVER__PORT=9090 \
DEN_SANDBOX__MAX_SANDBOXES=100 \
DEN_AUTH__ENABLED=true \
  den serve
```

## 选项详解

### 服务器

| 选项 | 描述 |
|------|------|
| `server.host` | 绑定的 IP 地址。`0.0.0.0` 监听所有接口，`127.0.0.1` 仅本地 |
| `server.port` | HTTP API 和控制面板的 TCP 端口，默认 `8080` |
| `server.rate_limit_rps` | 每秒最大持续请求数。设为 `0` 禁用 |
| `server.rate_limit_burst` | 超出持续速率的最大突发请求数 |

### 沙箱

| 选项 | 描述 |
|------|------|
| `sandbox.default_image` | 未指定镜像时使用的默认 Docker 镜像 |
| `sandbox.default_timeout` | 沙箱存活时间。Go 时间格式：`30m`、`1h`、`24h` |
| `sandbox.max_sandboxes` | 并发沙箱硬限制。超出时返回 `503` |
| `sandbox.default_cpu` | NanoCPU 单位。`1000000000` = 1核，`500000000` = 0.5核 |
| `sandbox.default_memory` | 字节单位。`536870912` = 512MB，`1073741824` = 1GB |
| `sandbox.default_pid_limit` | 容器内最大进程数，防止 fork 炸弹 |

### 认证

启用后，所有 API 请求必须包含有效的 `X-API-Key` 头。密钥使用常量时间比较防止时序攻击。

生成安全密钥：
```bash
openssl rand -hex 32
```

## 示例配置

### 开发环境（最简）

```yaml
log:
  level: debug
```

### 生产环境

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  rate_limit_rps: 20
  rate_limit_burst: 40
  tls:
    cert_file: /etc/ssl/certs/den.pem
    key_file: /etc/ssl/private/den.key

sandbox:
  default_timeout: "1h"
  max_sandboxes: 100
  default_memory: 1073741824

auth:
  enabled: true
  api_keys:
    - "prod-key-abc123..."

log:
  level: info
  format: json

store:
  path: /var/lib/den/den.db
```

### 资源受限环境

```yaml
sandbox:
  max_sandboxes: 10
  default_memory: 268435456   # 256MB
  default_cpu: 500000000      # 0.5 核
  default_pid_limit: 128
  default_timeout: "10m"
```
