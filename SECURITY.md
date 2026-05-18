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
- **Network posture**: `network_mode` ∈ `internal` (default) / `bridge` / `none`. **Only `none` is a tenant/egress boundary**; `internal` still reaches the bridge gateway, embedded DNS and host `0.0.0.0` services (see Known Limitations)
- **Port binding**: published only in `bridge` mode, bound to `127.0.0.1`; fixed at creation (no runtime add/remove — `501`)
- **Path validation**: Null byte and traversal protection on all file operations
- **Constant-time auth**: API key comparison resistant to timing attacks
- **SSRF protection**: S3 endpoints validated against internal/private IP ranges

## Network Security Model & Platform Safety Matrix

This is the **primary security artifact** for the network feature. Read it before
running Den with authentication disabled, a non-loopback bind, or `bridge` mode.

### (1) Platform safety matrix

The bind guard's safety depends on **where the Docker daemon actually is** relative
to the Den process, because `127.0.0.1` host port bindings and the "is the control
plane reachable from a sandbox" question are both decided by daemon topology, not by
Den's config. `clientGOOS` is the OS Den runs on; the topology is how its Docker
client reaches the daemon.

| clientGOOS | Daemon topology | `none` | `internal` (default) | `bridge` | Auth off + loopback bind | Auth off + non-loopback bind |
|---|---|---|---|---|---|---|
| linux | **linux-native** (local `/var/run/docker.sock`, same kernel) | contained | escape-open¹, egress-closed | escape-open¹, egress-open | bind-guard **REFUSES** unless `platform_override` attested² | bind-guard **REFUSES** |
| linux | rootless dockerd (same host) | contained | escape-open¹ | escape-open¹, egress-open | REFUSES (override still required²) | REFUSES |
| linux | remote daemon (`tcp://`/`ssh://`) | contained | escape-open on **daemon** host | escape-open + egress on **daemon** host; published ports land on **daemon**, not Den host | REFUSES (override is **void**³ — class is not co-resident) | REFUSES |
| linux | local unix socket **proxied** to a remote daemon (`socat`/`ssh -L`/bind-mounted sibling socket) | contained | **presumed-residual**⁴: looks local, behaves remote — escape lands on daemon host | same, egress-open | REFUSES; override is **void**³ here too | REFUSES |
| darwin/win | Docker Desktop (LinuxKit VM, unix socket) | contained | escape-open inside the **VM**, not the macOS/Windows host | egress-open from VM; ports forwarded by Desktop to host loopback | REFUSES (override is **void**³ — not native co-resident) | REFUSES |
| darwin/win | macOS/Windows VM via unix socket (Colima/Lima/Rancher) | contained | escape-open inside the **VM** | egress-open from VM | REFUSES; override **void**³ | REFUSES |

¹ "escape-open" means: a sandbox can reach the bridge gateway, the embedded DNS
resolver (`127.0.0.11`), and any host service bound to `0.0.0.0` — including Den's
own unauthenticated control plane if auth is off. It is **not** a kernel escape; it
is L3 reachability of the host control plane. Kernel-CVE pivot (²below) is a
separate, always-present risk of a shared kernel.

² **`platform_override` co-residency attestation.** The only configuration in which
the bind guard permits an auth-off loopback bind is native, co-resident Docker on
Linux explicitly attested via `runtime.platform_override="linux-native-docker-co-resident"`.
This is an operator promise that the daemon shares the Den host's kernel and loopback.
It is **scoped to the bind decision only** — it does not relax any other control.

³ **The override is void on any proxied/remote/VM topology.** If the daemon is not
literally the Den host's daemon, the attestation is false by construction and the
guard must still refuse; we do not trust the override to paper over a remote daemon.

⁴ **local-unix-socket-proxied-to-remote is a realistic residual class, not
hypothetical.** `socat UNIX-LISTEN:… TCP:remote`, `ssh -L …/docker.sock`,
docker-context, or a bind-mounted sibling socket all present a local-looking socket
backed by a remote daemon. Den cannot reliably distinguish this from native-local at
runtime, so the guard treats "looks local" as **insufficient** and still requires the
explicit override; operators on this topology must not set the override.

### (2) The control plane is unauthenticated by default

`/api/v1/version`, `/api/v1/health`, and the embedded dashboard are reachable
**without an API key** even when auth is enabled (they are intentionally unauthenticated
liveness/UX surfaces). With auth disabled, the *entire* control plane — sandbox
create/exec/file I/O — is unauthenticated. The bind guard exists specifically so this
surface is not silently exposed to a `bridge`/`internal` sandbox or a LAN.

### (3) A shared kernel is not a tenant boundary

Den containers share the host kernel. Capability drop, `no-new-privileges`,
read-only rootfs, seccomp and PID limits raise the bar but do not make this a
hard multi-tenant boundary. For hostile multi-tenant workloads use gVisor or Kata.
A kernel-CVE pivot is possible from any mode including `none`.

### (4) SSRF protection scope

The SSRF allow/deny logic protects **Den's own S3 client** (endpoint validated
against internal/private ranges). It is not a general egress firewall for sandbox
traffic; in `bridge` mode a sandbox has unrestricted outbound network access.

### (5) Why `enable_icc=false` is sound here

Inter-container communication is disabled on the managed network. This is a real
control **only because** `NET_RAW` is dropped (no ARP/raw-socket sidestep) and the
Docker API floor is enforced at ≥ 1.42 (below it the typed `EnableIPv6 *bool` and
ICC options are silently ignored). Both conditions are checked at startup; if either
fails the network is not considered hardened.

### (6) `internal` does not contain a sandbox

In `internal` mode the sandbox still reaches the bridge gateway, the embedded DNS
resolver (`127.0.0.11`), and any host service bound to `0.0.0.0`. `internal` removes
NAT/egress to the *internet*, not reachability of the *host*. **Only
`network_mode=none` is a tenant/egress boundary.**

### (7) Rejected: connectivity-on-by-default

We deliberately did **not** make `bridge` (full egress) the default. The default is
`internal`. This trades out-of-the-box internet for a smaller default attack surface;
operators who need sandbox egress must opt in per-sandbox or via config. This decision
is recorded here so it is not silently reversed.

### (8) Out of scope for v1

Den does not program host iptables/nftables and does not run as an egress-filtering
root. Per-`internal` egress filtering is a tracked follow-up, not in v1. Do not assume
`internal` will gain egress filtering without an explicit release note.

### (9) The feature token is a capability hint, not auth

`/api/v1/version` and `/api/v1/health` advertise a `features` list (e.g.
`network_mode`). SDKs use it lazily to fail fast on unsupported servers. It is a
**capability hint only** — it is not an authentication or authorization signal and
must never be treated as one.

### (10) The local-`unix://`-socket-proxied-to-remote residual is realistic, not rare

This is called out as its own point because it is the residual class operators most
often miss. A daemon reached through a **local-looking** unix socket that is actually
proxied to a remote/other-host daemon — `socat UNIX-LISTEN:/var/run/docker.sock
TCP:remote:2375`, `ssh -L .../docker.sock`, a docker-context-proxied socket, or a
bind-mounted rootless/sibling socket — is **common in CI, dev containers, and
remote-Docker setups**, not a corner case. On such a host:

- `127.0.0.1` host port bindings land on the **daemon** host, not the Den host —
  published ports do not appear where the operator expects.
- The `platform_override` co-residency attestation is **false by construction** and
  is therefore **void**: the guard still refuses an auth-off/non-loopback bind.
- Den cannot reliably distinguish this topology from native-local at runtime, so
  "looks local" is treated as **insufficient**. The only real mitigations are
  `auth.enabled=true` or an effective `network_mode=none`; operators on this topology
  must **not** set `platform_override`.

### (11) `platform_override` risk equivalence

Setting `platform_override` is risk-equivalent to `allow_unsafe_bind`: both tell the
guard to permit a bind it would otherwise refuse. The difference is intent
documentation, not a stronger guarantee. It is **bind-guard-scoped** — it is not a
platform fact consulted anywhere else, and it relaxes no other control.

## Known Limitations

- Container isolation relies on Docker; consider gVisor or Kata for higher-risk workloads
- S3 FUSE mount requires `SYS_ADMIN` capability — disabled by default
- Authentication is disabled by default for local development convenience
- **`internal` is NOT a tenant boundary**: a sandbox still reaches the bridge gateway, the embedded DNS resolver (`127.0.0.11`) and any host service bound to `0.0.0.0`. Only `network_mode=none` contains a sandbox. Egress filtering for `internal` is a tracked follow-up, not in v1
- **Docker-out-of-Docker (DooD) port access is unsupported**: when den's Docker client targets a remote or socket-proxied daemon (`tcp://`, `ssh://`, `socat`/`ssh -L`/docker-context/bind-mounted sibling socket), `127.0.0.1` port bindings land on the *daemon* host, not the den host. The same proxied-socket topology also **voids** the `platform_override` co-residency attestation and re-exposes the unauthenticated control plane
- Dynamic port forwarding is not supported: `POST`/`DELETE /api/v1/sandboxes/{id}/ports` permanently return `501`; port mappings are fixed at sandbox creation
