# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 0.0.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in Den, please report it responsibly.

**Do NOT file a public GitHub issue for security vulnerabilities.**

Instead, please email security concerns to: **security@den.dev**

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 1 week
- **Fix timeline**: Depends on severity, typically within 2 weeks for critical issues

## Security Model

Den executes untrusted code inside Docker containers with the following hardening:

- **Dropped capabilities**: `ALL` capabilities dropped, minimal set added back
- **Read-only root filesystem**: Only tmpfs mounts and explicit volumes are writable
- **PID limits**: Default 256 processes per container
- **No new privileges**: `no-new-privileges` security option
- **Network isolation**: Containers on internal Docker network
- **Port binding**: Forwarded ports bind to `127.0.0.1` only
- **Path validation**: Null byte and traversal protection on all file operations
- **Constant-time auth**: API key comparison resistant to timing attacks
- **SSRF protection**: S3 endpoints validated against internal/private IP ranges

## Known Limitations

- Container isolation relies on Docker; consider gVisor or Kata for higher-risk workloads
- S3 FUSE mount requires `SYS_ADMIN` capability — disabled by default
- Authentication is disabled by default for local development convenience
