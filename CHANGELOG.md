# Changelog

All notable changes to Den are documented in this file.

## [0.1.0](https://github.com/us/den/compare/v0.0.6...v0.1.0) (2026-06-18)


### ⚠ BREAKING CHANGES

* **s3:** a self-hosted S3/MinIO endpoint on loopback/RFC1918 is now refused by default; set storage.s3.allow_internal_endpoint=true to restore it.
* on the native-Linux + loopback-bind + auth-disabled configuration den now REFUSES to start unless runtime.platform_override="linux-native-docker-co-resident" is set (or auth is enabled, or the effective mode is "none"). This closes an unauthenticated control-plane host-escape reachable from inside a sandbox and is a deliberate, operator-accepted posture change. Per-sandbox network_mode other than "" or "none" is now a 400.

### Features

* **api:** add start, files/stat, and sandbox IP for mariete compat ([1b3154b](https://github.com/us/den/commit/1b3154b59acdbe4b1ed014762065e21a93380adf))
* connectable network modes and host port publishing ([9ad8988](https://github.com/us/den/commit/9ad8988016c5604ddf5624e7df0b6218645afaf2))
* **runtime:** add writable_rootfs opt-out for heavyweight images ([b4d049f](https://github.com/us/den/commit/b4d049f6b331fbbb73143e65dfc78a9b359dbabd))
* **s3:** SSRF-pin configured endpoint, opt-in internal exemption ([658823e](https://github.com/us/den/commit/658823e8fbb56d04a67983360ed6284ad8a273a4))


### Bug Fixes

* repair make test-integration floor misfire and TS SDK tsc build ([42c83aa](https://github.com/us/den/commit/42c83aa470d8cbdb4f74bc2226df886d3e272796))

## [v0.0.6] — 2026-03-14

### Added
- **Memory pressure monitoring** — Real-time host memory tracking with 5-level pressure system (Normal → Warning → High → Critical → Emergency)
- **Dynamic memory throttling** — Automatic per-container `memory.high` (cgroup v2) adjustment based on host pressure level
- **Pressure-aware sandbox creation** — Rejects new sandboxes at Critical/Emergency pressure (HTTP 503)
- **Hysteresis / debounce** — Pressure level changes require 2 consecutive readings to prevent flapping
- **Resource status API** — `GET /api/v1/resources` endpoint returning host memory, sandbox count, and pressure info
- **Platform abstraction** — `MemoryBackend` interface with Linux (`/proc/meminfo` + cgroup) and macOS (`sysctl` + Docker API) implementations
- **Direct cgroup v2 writes** — Linux containers get `memory.high` set via direct cgroup file write (sub-ms), Docker API fallback
- **Pressure drop recovery** — Memory limits automatically removed when pressure returns to Normal/Warning
- **OOM score management** — Dynamic OOM score adjustment for container processes (Linux only)
- **PID ≤ 1 protection** — Refuses to modify OOM score for init/host processes
- **Double-start protection** — `sync.Once` guard on `PressureMonitor.Start()`
- **Panic recovery** — All pressure monitor goroutine operations wrapped in `safeSample()` with recover
- **ResourceConfig** — Top-level `resource:` config section with configurable thresholds, intervals, and overcommit ratios

### Changed
- **Shutdown sequence** — `PressureMonitor.Stop()` now blocks until goroutine finishes via `doneCh` (prevents send-on-closed-channel panic)
- **Container limit updates** — Single `sync.Map.Range()` pass instead of two (fixes TOCTOU race)
- **Memory limit strategy** — Direct cgroup write first, Docker API fallback (was reversed)
- **Threshold configuration** — Resource handler uses engine config thresholds instead of hardcoded values

### Fixed
- **Hysteresis bypass** — `CurrentEvent().Level` now returns confirmed (debounced) level, not raw measurement
- **`memoryHigh=0` handling** — Writing `"max"` to cgroup v2 correctly removes limits (was no-op)

## [v0.0.5] — 2026-03-14

### Changed
- Renamed npm package to `@us4/den`
- Bumped SDK versions to 0.0.5

## [v0.0.4] — 2026-03-14

### Fixed
- npm scoped package public access
- Renamed PyPI package to `den-sdk`

## [v0.0.3] — 2026-03-14

### Added
- SEO meta tags, sitemap, robots.txt, and 404 page for docs site
- Docker registry switched to GHCR
- SDK publish made conditional (only when secrets are available)

### Fixed
- SSRF protection on internal network ranges
- Error leaking — internal errors no longer exposed to API clients
- Rate limiter hardening
- SDK import path and package name fixes
- GitHub/import references updated to `us/den`

## [v0.0.2] — 2026-03-14

### Added
- **Storage layer** — Persistent volumes, shared volumes, configurable tmpfs
- **S3 integration** — Hooks-based sync, on-demand import/export, FUSE mount
- **Go, TypeScript, Python SDKs** — Full storage type support

## [v0.0.1] — 2026-03-14

### Added
- Initial release
- Sandbox CRUD, exec, file operations, snapshots
- WebSocket streaming exec
- MCP server (stdio mode)
- Port forwarding, resource limits, auto-expiry
- API key authentication, rate limiting
- Embedded web dashboard
- CLI with all operations
