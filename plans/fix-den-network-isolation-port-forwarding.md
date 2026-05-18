# Den — Network Isolation & Port-Forwarding Fix — Implementation Plan (v9 — FINAL, operator-decided)

> **STATUS: plan-loop STOPPED by operator at iteration 8 (cap). 2/6 [CONSENSUS]
> (API/SDK, Pragmatist — both fully source-verified). The operator made the
> dominant security decision explicitly (see below) and three concrete
> mechanical fixes are folded into v9. Remaining residuals are DOCUMENTED &
> ACCEPTED, not silently dropped. Implementation proceeds from v9.**
>
> **Operator decision (2026-05-18), resolving the 4-iteration-recurring dominant
> finding (Codex CRITICAL ★ + Security 🟡 ★):** the native-Linux + loopback +
> auth-off branch is **REFUSE-by-default — `platform_override` attestation
> (or `auth.enabled=true`, or `none`) is MANDATORY**. The v8 "permit-with-WARN"
> presumption path is **removed**. This is a deliberate, operator-accepted
> **breaking operational change** (every existing default-config deployment adds
> one explicit attestation flag on upgrade). This makes Den genuinely
> secure-by-default and **resolves Codex-iter8-CRITICAL + Security-iter8-🟡**
> (both reviewers explicitly stated require-attestation resolves the finding).
>
> **v9 mechanical fixes folded in (no further review loop — operator chose
> stop+implement):**
> - **Docker-correctness iter8 🔴 (moby-source + live-verified):** a `--network
>   none` container's `NetworkSettings.Networks` is **NEVER empty** — it always
>   carries exactly the `{"none":{…}}` key. The v8 "`Networks` non-empty ⇒
>   breach" predicate is inverted and would force-remove every correct `none`
>   sandbox / deadlock CI. v9 predicate = **"any network key other than `none`,
>   OR any endpoint with a non-empty `EndpointID`/`NetworkID`/`IPAddress`"**.
> - **Testing iter8 🟡 + Codex iter8 WARN:** ONE CI topology is chosen
>   (GitHub Actions `services: docker:dind` + `DOCKER_HOST=tcp://docker:2375`);
>   the **mandatory CI gate is API-level only** (`Inspect().NetworkSettings.
>   Ports` sentinel + a one-shot `docker run --rm` exit-code probe, both over
>   the TCP daemon API, **needing no container-name resolution**). In-container
>   `docker exec`/`ip addr` legs (row 5 IPv6, marquee curl) are **explicitly
>   LOCAL-ONLY**; CI egress/DNS coverage is the API exit-code probe + the
>   local `scripts/e2e-network.sh`. The unspecified `docker ps --filter`
>   Actions-label discovery is **dropped** (not portable).
> - **Codex iter8 WARN:** the reconcile mismatch predicate is **ANY-deviation
>   (OR), not AND** — covers the operator `internal→bridge` migration case.
>
> **Documented & ACCEPTED residuals (operator chose stop, not extend):**
> - Rows 2/3/5's *in-container* egress/DNS/IPv6 assertions remain LOCAL-ONLY
>   proofs; CI proves the same behaviors via the API exit-code probe + Inspect
>   sentinel only. Accepted (Testing iter8 🟡 — narrowed, not eliminated).
> - With REFUSE-by-default the v8 "habituated-WARN-on-default" Security iter8 🟡
>   is **moot** (no permit-with-WARN path remains); the locally-proxied-`unix://`
>   residual is now an **explicit operator attestation** (`platform_override`),
>   not a silent presumption — the strongest closure short of daemon-locality
>   probing (Open Q5, deferred).

---

## (historical) v8 title

## Context

Den hardcodes its Docker network as `Internal: true` (`internal/runtime/docker/docker.go:90-93`).
An internal bridge has **no gateway route, no NAT/masquerade, no docker-proxy**, so **both**
advertised port-forwarding and outbound egress/DNS are dead. Reproduced live:
`HostConfig.PortBindings={"8000/tcp":[{"HostIp":"127.0.0.1","HostPort":"9123"}]}` but
`NetworkSettings.Ports={"8000/tcp":[]}`; same `-p` on the default bridge → HTTP 200; on `den-net`
→ unreachable; in-sandbox DNS → `Temporary failure in name resolution`. `README.md:188`
("Port Forwarding — Expose sandbox ports to host") and `README.md:205` ("Network isolation —
Containers on internal Docker network") are mutually contradictory in shipped docs.

### Verified facts (research, cited; reconfirmed across 7 review iterations)

- `Internal: true` is the **single root cause** of dead ingress *and* egress (moby#36174,
  moby#26724; docs.docker.com). Flipping to `Internal:false` with the **same** `PortBindings`
  restores publishing. *(Docker-internals; Docker-correctness reviewer)*
- **`Internal: true` is NOT full isolation.** The container still gets `eth0`+IP **and reaches
  the bridge gateway IP and any host service bound to `0.0.0.0`** (docs.docker.com; moby#21990).
  *(Security; Docker-correctness)*
- **Den's control plane is reachable from inside sandboxes in every connected mode incl.
  `internal`.** `config.go:119` `Server.Host:"0.0.0.0"`; `config.go:157` `Auth.Enabled:false`;
  `main.go:124` only warns. A sandbox reaches `<den-net-gateway>:8080` → the **unauthenticated**
  Den API → host-bind/escalation. *(Security iter2 C1/C2 — the dominant finding)*
- **The bind-guard safety argument rests on a co-residency precondition: the Den process,
  the Docker daemon, and the bridge gateway are the same native-Linux host.** Iter-6 (Codex
  C1 ★) showed a daemon-facts-only allowlist fails open for a **macOS/Windows** host running
  Colima/Lima/OrbStack/Rancher Desktop (a `unix://` socket to a Linux-VM daemon whose
  `docker info` reports `OSType=linux`) — closed in v7 by the mandatory `clientGOOS=="linux"`
  clause. **Iter-7 (Codex CRITICAL + Security 🟡#1 — the now-dominant residual): even with
  `clientGOOS=="linux"` AND a local `unix://` endpoint, co-residency is NOT machine-provable.**
  A native-Linux Den host whose local `unix://` socket is itself a proxy to a remote/VM daemon
  — `socat UNIX-LISTEN:/var/run/docker.sock TCP:remote:2375`, `ssh -L .../docker.sock`, a
  bind-mounted rootless/sibling-container socket, or a docker-context-proxied socket — passes
  every allowlist clause while co-residency is **false**. This class is **realistic and
  not-rare on Linux hosts (CI bastions, remote-Docker workflows), not an irreducible edge**.
  **v9 operator decision (resolving Codex iter6 C1★/iter7 CRITICAL★/iter8 CRITICAL★ and
  Security iter5 C2/iter6/iter7 🟡#1★/iter8 🟡):** co-residency is **never presumed silently
  and never permit-with-WARN**. On the native-Linux loopback auth-off branch Den **REFUSES to
  start** unless the operator **explicitly attests** co-residency via `runtime.platform_override
  = "linux-native-docker-co-resident"` (or removes the precondition entirely via
  `auth.enabled=true` or effective `none`). The undetectable local-`unix://`-proxy residual is
  thereby converted from a *silent presumption* into an **explicit, logged operator act** — the
  strongest closure achievable without daemon-locality probing (Open Q5, deferred).*
- **`runtime.docker_host` (`config.go:53`, koanf `docker_host`) is defined but DEAD: grep
  confirms it is referenced nowhere outside the struct tag.** The Docker client is built solely
  via `client.NewClientWithOpts(client.FromEnv, …)` (`docker.go:54`) — only the `DOCKER_HOST`
  **env var** reaches the client; `runtime.docker_host` cannot currently affect the endpoint.
  The classifier reasons about the **effective** endpoint via `client.DaemonHost()` —
  moby-source-verified `func (cli *Client) DaemonHost() string { return cli.host }`
  (`client/client.go:400-402`), defaulting to `unix:///var/run/docker.sock`
  (`client_unix.go:7`) when env unset (`WithHostFromEnv` leaves the default in place), and
  reflecting a programmatic `WithHost`. It is NEVER `""` from a successfully constructed client
  (`ParseHostURL` rejects empty) — the "empty ⇒ local" branch is **defensive-only/dead-but-
  harmless**, kept for total-function shape. v1 keeps `runtime.docker_host` inert and adds a
  test asserting (a) `runtime.docker_host` set + `DOCKER_HOST` unset ⇒ `DaemonHost()` is the
  default unchanged, AND (b) `DOCKER_HOST=tcp://x:2375` ⇒ `DaemonHost()` **does** change —
  proving the distinction is real, not vacuous (Codex iter7 W). Wiring `runtime.docker_host`
  is explicitly out of scope. *(Codex iter6 C2/iter7 W; Pragmatist iter6 W2; Docker-correctness
  iter7 source-verified)*
- **`/api/v1/version`, `/api/v1/health`, the dashboard `/*` are unauthenticated by design,
  even when `auth.enabled=true`** (`server.go:110-112` before the auth group; `common.go:60-66`;
  dashboard `embed.FS`). `Version` is read-only → residual when auth on = fingerprinting.
  Guard contract: *"no unauthenticated access to **state-changing** `/api/v1/*`"*.
  *(Security iter4 C2)*
- **`common.go` `/health` returns two shapes — `{"status":"ok"}` (200) or
  `{"status":"unhealthy","reason":…}` (503) — both `map[string]string`** (`:43-56`). Adding
  `features` is **purely additive** (no key renamed/removed); forces a typed struct /
  `map[string]any` on `/version`+`/health`. **No `DisallowUnknownFields` (Go), no Pydantic
  `extra="forbid"` (sdk/python default `extra="ignore"`), no TS strict shape** anywhere in
  `sdk/`/`pkg/` (grep-verified, API/SDK iter6/iter7) → backward-safe for all three SDKs and
  any `status==ok`/substring probe; only an **exact-body or strict-JSON-schema** external probe
  breaks. *(Security iter4 W2; API/SDK iter6 W1/iter7 D2, verified)*
- **The server build version is `"dev"` for every non-goreleaser build** (`common.go:13`;
  `Makefile:2`). A semver `>=0.1.0` capability floor is unimplementable. *(API/UX iter4 C1 ★;
  Codex iter4 W ★)*
- **`enable_icc`** = bridge driver option `com.docker.network.bridge.enable_icc`; readable only
  via `NetworkInspect().Options` (defaulted options not echoed; absent ⇒ default `true`) → a
  Den-written label is the only reliable truth source. Blocks container↔container but **NOT
  container→gateway→host**; not an egress control. Daemon floor for Den is **API ≥1.42**.
  *(Docker-correctness W2/iter4 W2; iter6-confirmed at moby `bridge_linux.go:340,592`)*
- **IPv6 is the typed `network.CreateOptions.EnableIPv6 *bool`** (`json:",omitempty"`, moby
  `v28.3.2+incompatible`, source-verified `api/types/network/network.go:37`). A non-nil `false`
  pointer is **always serialized**. The protective control is **NOT** the pointer/omitempty —
  it is the **startup API-floor assert ≥1.42 AND the post-create `Inspect().EnableIPv6==false`
  re-verify** (`Inspect.EnableIPv6` is a plain `bool`, `api/types/network/network.go:81`,
  sourced from `nw.IPv6Enabled()` at `daemon/network.go:639` — also a reconcile mismatch
  trigger). Pre-1.42 daemons do not process the typed field (used a driver label). *(Docker-
  correctness iter3 C2/iter4 W1/iter6 W1/iter7, moby-source-verified at exact lines)*
- **`none` (moby-source-verified iter5/6/7 at v28.3.2):** **NO create-time cross-check** rejects
  a non-empty `EndpointsConfig` + `NetworkMode("none")`. `daemon.validateNetworkingConfig`
  (`daemon/create.go:357-382`) does no none↔EndpointsConfig check; for a single `den-net`
  endpoint `updateNetworkSettings` (`daemon/container_operations.go:194-195`) hits
  `// Avoid duplicate config` / `return nil` and the container is **silently attached to
  `den-net`** — a **silent isolation breach, not an HTTP 400**. Empty/nil `EndpointsConfig`
  *without* `NetworkMode("none")` → default bridge (`container_operations.go:406-411`).
  Therefore:
  - **PRIMARY, load-bearing control = empty/nil `EndpointsConfig`**, via short-circuiting the
    `netID==""→r.networkID` default (`docker.go:181-183`).
  - `HostConfig.NetworkMode("none")` is **defense-in-depth, NOT daemon-backstopped**.
  - **TWO authoritative guards**: (a) the `buildContainerCreateSpec` **unit test**; (b) a
    **production post-create runtime assertion** — after create (and **strictly before any
    `ContainerStart`** — see CD/§3), nil-guard `inspect.NetworkSettings` (it is a
    `*container.NetworkSettings`, `api/types/container/container.go:185` — Docker-correctness
    iter7 🔵). **v9 predicate (Docker-correctness iter8 🔴, moby-source + live-verified): a
    `--network none` container's `NetworkSettings.Networks` is NEVER empty — it ALWAYS carries
    exactly the single key `"none"` mapping to a zeroed `EndpointSettings`** (`daemon/
    container_operations.go:405-410`, surfaced verbatim by `daemon/inspect.go:39-45`). The
    breach signal is therefore **NOT map non-emptiness** but: `inspect.NetworkSettings.Networks`
    contains **any key ≠ `"none"`**, OR any endpoint whose `EndpointID`/`NetworkID`/`IPAddress`
    is non-empty. If breach when effective mode is `none`, **force-remove the container and
    fail-loud**. *(Docker-correctness iter5 C1/iter7-verified/**iter8 🔴 predicate-corrected**;
    Security iter6 🟡#1/iter7; Codex iter7 W)*
- **Effective mode must be resolved BEFORE the Docker boundary.** `runtime.Runtime.Create(ctx,
  id, cfg)` has **no mode parameter**. The engine resolves the effective mode (per-sandbox
  `network_mode` override vs global `default_network_mode`; per-sandbox may only *increase*
  isolation) and writes it onto `cfg.NetworkMode`; `docker.Create` reads `cfg.NetworkMode` and
  passes it explicitly into the pure `buildContainerCreateSpec(cfg, networkID, mode)`.
  *(Codex iter6 W; Pragmatist verified `runtime.go` Create signature)*
- `network.go` `PortForwarder` is **dead code** (only self + `port.go:36` comment; `port.go`
  already 501 both verbs). *(Pragmatist, verified)*
- **The absent-`ports` API bug is a SINGLE one-line defect, NOT a response-struct gap (API/SDK
  iter7 C1, source-verified):** the response plumbing is **already correct** —
  `handlers/sandbox.go:43` `sandboxResponse` **already has** `Ports json:"ports,omitempty"`;
  `:54` `toSandboxResponse` **already copies** `s.Ports`; `engine/sandbox.go:18` `Sandbox`
  **already has** `Ports json:"ports,omitempty"` and `:61` `toResponse` already serializes it.
  The **sole** defect is `engine.CreateSandbox`'s `Sandbox{…}` literal (`engine.go:258-264`)
  **not assigning `Ports: cfg.Ports`**, so `s.Ports` is nil at the source. **The fix is that
  one assignment; the response/handler structs are already correct and MUST NOT be modified**
  (prevents a wrong-layer "fix"). This resolves the Pragmatist-vs-API/SDK framing conflict:
  both confirm the fix location is `engine.CreateSandbox`; v8 corrects the *diagnosis* wording.
  *(Pragmatist iter4 C1/iter6/iter7 source-verified; API/UX iter4 W1; API/SDK iter7 C1 ★)*
- **`engine.NewEngine` has 5 call sites** (re-grep-verified iter6/7, defined `engine.go:48`):
  `cmd/den/main.go:136`, `cmd/den/mcp.go:59`, `tests/integration/storage_test.go:77`
  (`//go:build integration`), `internal/api/handlers/sandbox_test.go:40`,
  `internal/engine/engine_test.go:36`. **2 production sites (`main.go`, `mcp.go`) wire the
  guard substantively; 3 test sites pass a zero policy** (Pragmatist iter7 🔵). *(exact)*
- **`mcp.NewServer(eng, logger)` (`server.go:73`) receives no config/validator**; MCP create
  (`tools.go:81-107`) calls `s.engine.CreateSandbox` directly; MCP is **stdio — starts NO HTTP
  listener** (`mcp/server.go:61`). `engine.CreateSandbox` wraps errors `fmt.Errorf("creating
  sandbox: %w", err)` (`engine.go:271-273`) — but the validator returns **before**
  `runtime.Create`, so a directly-returned `*netpolicy.ValidationError` reaches the handler
  **un-wrapped**. *(Pragmatist iter4 C2/iter6/iter7, verified)*
- **`Protocol` is an API/SDK contract field.** Go `client.go:56` (`omitempty`); TS
  `types.ts:186` (optional); Python `types.py:16` `protocol: str = "tcp"` (default in
  accept-set → Python unaffected even sending the implicit default; only a consumer *explicitly*
  sending `"udp"` breaks). Accept-set **case-insensitive `{"", "tcp"}`; the validator
  normalizes to canonical lowercase `"tcp"` before nat construction.** `nat.NewPort` is
  **case-preserving** (go-connections `v0.6.0` `nat/nat.go:30-43` — `fmt.Sprintf("%d/%s")`, no
  ToLower; source-verified iter7), so without normalization `"TCP"` yields a `8000/TCP` key
  that silently breaks publishing — normalization is **mechanically load-bearing**. SDK
  doc-comments still listing `"udp"` (`types.ts:185`, Python) are scrubbed in Phase 4.
  *(API/UX iter2 C2/C3; iter6; Docker-correctness iter7 verified)*
- **`docker.go:108-114` merges `cfg.Labels` LAST, overwriting Den-set `den.id`** (verified) —
  caller can spoof `den.id`. **Containers carry only `den.id` (`labelID`) + `den.created`
  (`labelCreated`); there is NO container `den.managed` label — `den.managed` is network-only**
  (Pragmatist iter6 W1, verified). *(Security iter3 W1)*
- **Reconcile is on the concrete `*docker.DockerRuntime`, NOT `runtime.Runtime`**
  (`runtime.go:166-200` has neither `EnsureNetwork` nor `Reconcile`; `EnsureNetwork` is
  concrete `docker.go:77`). **`store.Store` is a 10-method interface** (`store.go:33-49`:
  `SaveSandbox`/`GetSandbox`/`ListSandboxes`/`DeleteSandbox`/4 snapshot methods/`Close` —
  NOT the 2 the v7 "narrow surface" wording implied; Testing iter7 🟡). The failing test stub
  **embeds a nil/no-op `store.Store` and overrides only `ListSandboxes`/`GetSandbox`** (Go
  embedded-interface pattern) — no `StoreReader` type, no 10-method hand-roll. `docker.go`
  does not import `internal/store` today and `internal/store` imports neither `runtime` nor
  `engine` → `Reconcile(ctx, store.Store)` introduces **no import cycle** (Testing iter7
  source-verified). *(Testing iter5 C2/iter6/iter7, verified)*
- **`rootCmd.Execute()` prints error **and usage** on every `RunE` failure** unless
  `SilenceUsage` is set; cobra `v1.10.2` keeps the `Error:` prefix — a guard test must assert
  `strings.Contains(stderr, <const>)` (substring) + a negative assertion on the allow path.
  *(Testing iter4 C4/iter5 C3/iter6/iter7, verified)*
- koanf env: `DEN_` prefix, lowercase, `__`→`.`; file+env only, no flag layer; `(*Config).
  Validate()` (`config.go:175`) no logger, uniform `if cond {return fmt.Errorf}` pattern;
  **no `Warnings()` method exists** (net-new); both `main.go:76`+`mcp.go:34` already call
  `Validate()` post-`Load()` with a `logger` in scope → emission is ~4 lines at 2 sites
  (Pragmatist iter7 verified). *(verified)*
- Integration harness broken: `Makefile:16`/`ci.yml:70` lack `-tags integration` & `./tests/...`;
  funcs are `TestIntegration_*`; dind sets no `DOCKER_HOST`, needs `DOCKER_TLS_CERTDIR=""` on
  the dind **service** block; both `integration-test`+`sdk-test` `if:` must change together.
  **The dind topology forces `DOCKER_HOST=tcp://docker:2375` → `classifyPlatform=unknown` → a
  default `0.0.0.0`/auth-off start is REFUSED by design.** So every integration row that needs
  Den's HTTP API up explicitly sets its own startup config (Phase 5 per-row "Den cfg").
  **v9 SINGLE chosen CI topology (Testing iter8 🟡 + Codex iter8 WARN — "choose one, don't add
  a may-fail pre-step"): GitHub Actions `services: docker:dind` + `DOCKER_HOST=
  tcp://docker:2375` (the daemon API IS reachable over TCP from a direct runner job; service
  ports publish to the runner). The MANDATORY CI gate is API-LEVEL ONLY and needs NO
  container-name resolution: (i) `Inspect().NetworkSettings.Ports["x/tcp"]` non-empty
  (ingress sentinel) and (ii) a one-shot `docker run --rm <img> sh -c '<probe>'` whose EXIT
  CODE encodes the DNS/egress result (rows 2/3) — both issued purely through the TCP Docker
  API.** The unspecified `docker ps --filter`-by-Actions-label discovery and the
  step-launched-`--name` variant are **DROPPED** (not portable). In-container `docker exec`/
  `ip addr`/host-`curl` legs (row-5 IPv6, the marquee `curl==200`) are **explicitly
  LOCAL-ONLY**; CI proves egress/DNS via the exit-code probe + Inspect sentinel, and full
  in-container behavior via the local `scripts/e2e-network.sh`. *(Testing iter2-8; Codex
  iter6 C3 ★/iter7 W/iter8 WARN — single topology fixed)*
- release-please: single package `"."`, `release-type: go`, `bump-minor-pre-major: true`
  (`bump-patch-for-minor-pre-major:true` also present — affects only non-breaking pre-1.0
  bumps, NOT the `feat!:`→`0.1.0` outcome; API/SDK iter7 S3 verified), manifest `0.0.6`.
  `feat!:` pre-1.0 → `0.1.0`. SDK `package.json`/`pyproject.toml` at `0.0.6`, **not** in the
  manifest → same-PR manual `0.1.0` edit safe; Go module reaches `0.1.0` only when the Release
  PR merges and `v0.1.0` is tagged. Harmless transient: in-repo SDK manifests lead the Go tag
  by one Release-PR cycle (unpublished pre-tag → no consumer observes skew). *(API/UX iter4
  C3/iter7 S3, file-verified)*
- Existing SSRF guard (`s3.go:118`) is Den's S3 paths only; zero in-sandbox effect. *(Security)*

## Approach

### Central decision 1 — default stays `internal` (reconfirmed 7×)

Identical to today. Egress + working port-forwarding are opt-in `bridge`; full network-off is
`none`. Non-breaking default. Rejecting connectivity-on-default is **recorded, not dropped**.

### Central decision 2 (v9 — operator-decided) — control-plane isolation via a PLATFORM-AWARE, POSITIVE-ALLOWLIST, CLIENT-OS-GATED bind guard that is FAIL-CLOSED AGAINST EVERY MACHINE-DETECTABLE UNSAFE TOPOLOGY and REFUSE-BY-DEFAULT (mandatory `platform_override` co-residency attestation) on the undetectable local-`unix://` residual; host-firewall egress filtering DESCOPED

Corrects v4's "OS-independent" claim, v5's blacklist, v6's daemon-facts-only allowlist (Codex
iter6 C1 ★, closed by `clientGOOS`), v7's absolute "proven-safe" branding, **and v8's
permit-with-mandatory-WARN presumption path, which Codex iter8 (CRITICAL ★) + Security iter8
(🟡 ★) showed is disclosure, NOT a security control — a WARN that fires on the *recommended
default* config on every normal boot is structurally trained-to-ignore, leaving the
unauthenticated control-plane escape open under the default config on any locally-proxied
`unix://` host.** **v9 operator decision (2026-05-18):** the local-`unix://` co-residency
residual is **NOT presumed and NOT permit-with-WARN — Den REFUSES to start** on the
native-Linux loopback auth-off branch unless the operator **explicitly attests** via
`runtime.platform_override="linux-native-docker-co-resident"` (or removes the precondition via
`auth.enabled=true` / effective `none`). Rejected-and-recorded: (i) keep "proven-safe"
(dishonest); (ii) v8 permit-with-WARN (habituated log on the default config = security
theater — Codex/Security iter8). The operator **accepted the breaking operational cost**
(every existing default-config deployment sets one attestation flag on upgrade) in exchange
for genuine secure-by-default. `RequiresCoResidencyPresumptionDisclosure` + the mandatory-WARN
machinery from v8 are **REMOVED** (the attestation flag + its existing ERROR-level per-start
log *is* the disclosure, and it is now an explicit operator act, not a silent default).

> **v1 closes the control-plane escape with a zero-privilege, iptables-free, positive-allowlist,
> client-OS-gated guard. It is FAIL-CLOSED against every *machine-detectable* unsafe topology.
> On the one *undetectable* residual (a local `unix://` socket itself proxied to a remote/VM
> daemon) it is PRESUMED-CO-RESIDENT and emits a MANDATORY per-start disclosure. Egress is
> honest by opt-in:**
>
> 1. **Fatal bind guard, default-ON (`allow_unsafe_bind` opt-OUT).** Den **refuses to start**
>    (non-zero exit, committed message) unless **machine-detectably safe OR
>    operator-attested-safe**. Pure decision function `netpolicy.BindGuardDecision(auth,
>    hostClass, mode, runtimePlatform, platformOverrideAttested, allowUnsafeBind)`. **Safe**
>    iff: `allowUnsafeBind=true` **OR** `auth.enabled=true` **OR** effective
>    `default_network_mode==none` **OR** (`hostClass==loopback` **AND**
>    `runtimePlatform==linux-native-docker` **AND `platformOverrideAttested==true`**).
>    **v9 (operator decision):** the loopback branch is **REFUSE-by-default** — a genuinely
>    native-Linux loopback auth-off host with `platform_override` UNSET ⇒ `platformOverride
>    Attested==false` ⇒ **REFUSE** (the v8 permit-with-WARN path is gone).
>    `platformOverrideAttested := (cfg.runtime.platform_override ==
>    "linux-native-docker-co-resident")`. Setting that literal **both** forces
>    `runtimePlatform=linux-native-docker` for the bind-guard input (v8 behavior, retained)
>    **and** satisfies the mandatory attestation clause — it is the single explicit operator
>    act that says "I attest this `unix://` socket is local & co-resident." Refusal-message
>    remediation list (safest-first): `auth.enabled=true` · effective `none` ·
>    `platform_override="linux-native-docker-co-resident"` (attest co-residency) ·
>    `allow_unsafe_bind=true` (last-resort/dangerous).
>    - `hostClass = netpolicy.classifyHost(server.host)`: `loopback` **iff**
>      `net.ParseIP(host).IsLoopback()` for a literal in `127.0.0.0/8`/`::1`. Empty, `0.0.0.0`,
>      `::`, any hostname (incl. `localhost`), any non-loopback IP, unparseable ⇒ `non-loopback`
>      (fail-closed).
>    - `runtimePlatform = netpolicy.classifyPlatform(info, effectiveDockerHost, clientGOOS)` —
>      a **positive allowlist**. `linux-native-docker` is returned **iff ALL** hold:
>      (a) the `docker info` call **succeeded**; (b) **`clientGOOS == "linux"`** — the Den
>      process's own `runtime.GOOS` (closes the macOS/Windows-host-via-`unix://`-socket-to-
>      Linux-VM hole — Codex iter6 C1 ★); (c) `info.OSType == "linux"`; (d)
>      `info.OperatingSystem` does **not** contain `"Docker Desktop"`; (e) `info.KernelVersion`
>      does **not** contain `"linuxkit"` or `"microsoft-standard-WSL2"`; (f)
>      `DecodeSecurityOptions(info.SecurityOptions)` **succeeds** and yields no `name=rootless`
>      — **a present-but-empty `SecurityOptions` slice satisfies (f)** (`DecodeSecurityOptions`
>      source-verified iter7: empty→`([]SecurityOpt{},nil)`; rootless reliably self-advertises
>      `name=rootless`; malformed→error→unknown; polarity pinned, truth-table row — Security
>      iter6 🟡#2/iter7 💚); (g) `effectiveDockerHost` is **local** — empty (defensive-only,
>      never produced by a real client — Docker-correctness iter7 🔵) or `unix://`
>      (`tcp://`/`ssh://`/`npipe://` ⇒ remote ⇒ NOT co-resident). **`effectiveDockerHost`
>      is `client.DaemonHost()`** (negotiated, post-resolution; also catches programmatic
>      `WithHost`), **never** raw `os.Getenv("DOCKER_HOST")` or the dead `runtime.docker_host`
>      (Codex iter6 C2). Scheme matched as a scheme prefix, not substring. **If ANY clause is
>      false / missing / unparseable, or `docker info` errors ⇒ `runtimePlatform = unknown`**
>      (fail-closed). Structurally `for each clause { if !clause { return unknown } }; return
>      linux-native-docker` — never detect-the-bad-and-pass-the-rest.
>    - **Co-residency is a NAMED precondition, ATTESTED not presumed (v9).** Clauses (b)+(g)
>      eliminate the *detectable* non-co-resident cases (non-Linux client; remote
>      `tcp://`/`ssh://` endpoint). The **irreducible undetectable residual** — a *local*
>      `unix://` socket itself proxied to a remote/VM daemon (`socat UNIX-LISTEN…TCP:remote`,
>      `ssh -L …docker.sock`, bind-mounted rootless/sibling-container socket, docker-context-
>      proxied socket; **a realistic, not-rare class on Linux CI/bastion hosts, NOT an edge**
>      — Security iter7 🟡#1 ★/iter8 🟡) — **cannot be detected from `docker info`+
>      `DaemonHost()`+`clientGOOS`.** **v9 operator decision: NOT presumed, NOT
>      permit-with-WARN — REFUSE unless explicitly attested.** On the loopback branch the
>      operator MUST set `platform_override="linux-native-docker-co-resident"` (the attestation
>      "I confirm this socket is local & co-resident"); absent it, Den refuses. The
>      v8 `RequiresCoResidencyPresumptionDisclosure` decision fn and the mandatory-per-start
>      WARN are **REMOVED** — there is no longer a permitted-but-presumed path to disclose; the
>      attestation flag's existing **ERROR-level per-start log** (committed string, safest-first
>      remediation, risk-equivalence) *is* the disclosure, and it now fires only on an
>      **explicit operator act**, never on the bare default. (Codex iter8 CRITICAL ★ + Security
>      iter8 🟡: a WARN on the recommended-default config is security theater — closed by
>      REFUSE, the option both reviewers named as the genuine resolution.)
>    - `runtime.allow_unsafe_bind=true` is the explicit opt-in-to-unsafe escape: Den logs an
>      **ERROR-level message every start** naming the exact escape; the refusal-message
>      remediation list is **safest-first**; the flag is labeled last-resort/dangerous.
>    - Guard **scoped to the single `server.host`/`server.port` HTTP listener** (the only one —
>      MCP is stdio, no pprof/metrics). A **structural test** AST-parses `cmd/den/mcp.go`
>      (`go/parser`) and asserts **zero references** to `net.Listen`/`http.Server`/
>      `http.ListenAndServe`/`(*http.Server).Serve` in that file; **scope-stated: a single-
>      file static assertion, NOT a transitive-helper guarantee — paired with a code-comment
>      tripwire** (Testing iter7 🟡 — mechanism pinned, claim downscoped).
>    - **In MCP-only mode the bind guard is a no-op** (no HTTP listener); **the bridge-refusal
>      guard (CD2.2) STILL runs in MCP mode** (Security iter5 W).
>    - Both guards **abort BEFORE `EnsureNetwork`/reconcile**, in both `main.go` and `mcp.go`.
>    - **Rationale (netfilter-accurate):** conservative/fail-safe. For `0.0.0.0`/`::` the
>      den-net gateway IP is accepted-on; for a specific non-gateway host IP in `internal` the
>      sandbox has no default route (`ENETUNREACH`) but the guard still refuses (deliberately
>      conservative).
> 2. **`bridge` is refusal-by-default everywhere** unless `runtime.allow_unsafe_bridge=true`.
>    Connected, NAT'd, unfiltered → reaches metadata/RFC1918/host. **Runs in HTTP AND MCP mode.**
> 3. **Egress filtering is a tracked follow-up** (`netpolicy` host-firewall — v3 design
>    preserved in Open Q3). **Explicitly also covers `internal` mode.** Not in v1.
> 4. **`runtime.platform_override` — v9 dual role: MANDATORY co-residency attestation +
>    tight bind-guard-scoped contract** (Codex iter6 C4/iter7 W; **v9 operator decision**):
>    accepted values are **exactly `""` (default) OR the single literal
>    `"linux-native-docker-co-resident"`**; any other value ⇒ config `Validate()` error. When
>    set to the literal it (a) **forces `runtimePlatform = linux-native-docker` for the
>    `BindGuardDecision` input ONLY** — scoped strictly to the bind-guard decision; MUST NOT be
>    consumed as a true platform fact by reconcile, IPv6, or any future path (code comment + a
>    test asserts override-derived `runtimePlatform` is not read outside `BindGuardDecision`;
>    Codex iter7 W) — **and (b) sets `platformOverrideAttested=true`, the v9 MANDATORY clause
>    that unlocks the loopback branch** (without it, native-Linux loopback auth-off REFUSES).
>    It is **no longer merely a bypass — on the loopback branch it is the REQUIRED operator
>    attestation of socket-locality/co-residency.** Den logs the **SAME ERROR-level per-start
>    contract as `allow_unsafe_bind`** (committed string naming the attested precondition +
>    the socat/SSH-forward/docker-context void condition, safest-first remediation,
>    risk-equivalent-to-bypassing-the-classifier). It is NOT a quieter bypass than the flag it
>    backstops.
>
> **Honest scope of "secure-by-default" (v9):** *exactly* "no unauthenticated access to Den's
> state-changing control plane, with the undetectable local-socket co-residency residual
> **REFUSED-by-default and only permitted under an explicit operator `platform_override`
> attestation**, on every supported platform, fail-closed against every detectable unsafe
> topology." **NOT** tenant containment: in `internal` the sandbox
> still reaches the bridge gateway, **the embedded-DNS resolver + any other host `0.0.0.0`
> service** (Security iter7 🟡#3), co-tenants — **only `none` is a tenant/egress boundary in
> v1**. SECURITY.md states this verbatim; the operator release-note bucket carries the
> top-line ("`internal` does NOT contain a sandbox; only `none` is a tenant boundary").

1. **One config vocabulary.** `type NetworkMode string` + `NetworkModeInternal/Bridge/None`.
   Global `runtime.default_network_mode` (default `"internal"`); per-sandbox `network_mode`.
2. **Per-sandbox `network_mode` ∈ exactly `{"", "none"}`** (case-sensitive; JSON
   `null`/absent/`""` = inherit). Every other value, **including one equal to the global**, →
   **HTTP 400** with a typed body: `per-sandbox network_mode may only be omitted or "none"
   (it can only increase isolation)`. The "requesting the current global mode is a 400" footgun
   is in the API-contract release bucket and SDK docs.
3. **Mode semantics:**
   - `internal`: `den-net`, `Internal:true`, `enable_icc=false`, `EnableIPv6:ptr(false)`.
     Today's behavior; bind guard prevents the control-plane escape; **nothing else contained**.
     Port mappings accepted with a create/startup **warning** they are inert.
   - `bridge`: `den-net`, `Internal:false`, `enable_icc=false`, `EnableIPv6:ptr(false)`.
     Egress + `127.0.0.1` publishing work. **Refuses unless `allow_unsafe_bridge=true`**; no
     egress filter in v1.
   - `none`: **empty/nil `EndpointsConfig` (PRIMARY, via short-circuiting the
     `netID==""→r.networkID` default) + `HostConfig.NetworkMode="none"` (defense-in-depth, NOT
     daemon-backstopped)**; clear `portBindings`+`exposedPorts` (asserted as **two separate**
     sub-assertions). `len(cfg.Ports)>0` → **HTTP 400**. **Production post-create assertion,
     ordering pinned (Security iter7 🟡#2 TOCTOU / Codex iter7 W):** the `Inspect()` runs
     **strictly before any `ContainerStart`** (a created-but-not-started container has no
     process and cannot egress → the TOCTOU window is closed *by construction*; this
     create→assert→start ordering is a **tested invariant** — row-12 asserts the container was
     never started before the check). nil-guard `inspect.NetworkSettings`. **v9 breach
     predicate (Docker-correctness iter8 🔴): a correct `none` container ALWAYS has
     `NetworkSettings.Networks == {"none":{zeroed}}` (non-empty by design — moby
     `container_operations.go:405-410`/`inspect.go:39-45`). Breach iff `Networks` contains any
     key ≠ `"none"` OR any endpoint with non-empty `EndpointID`/`NetworkID`/`IPAddress`** (NOT
     "Networks non-empty" — that would force-remove every correct `none` sandbox). On breach:
     **log the full `Inspect().NetworkSettings` JSON at ERROR (forensics — Security iter7 🔵),
     force-remove the container (`RemoveOptions{Force:true}`), and return the committed error
     EVEN IF cleanup partially fails, wrapping the cleanup error as context** (Codex iter7 W —
     never leave a running network-attached sandbox after reporting failure). `EnsureNetwork`
     **no-op** when global mode `none`.
4. **`EnsureNetwork` fatal, idempotent-safe, `none`-aware.** `main.go`/`mcp.go` **abort** on
   error. Label managed network `den.managed=true`, `den.network.mode`, `den.network.icc`.
5. **Reconcile only on operator-initiated mode change, spoof-resistant, store-fail-closed:**
   - **Den-set labels authoritative at the Docker boundary:** reorder `docker.go:108-114` so
     `den.id`/`den.created` are applied **after** the `cfg.Labels` loop (skip `den.*` in the
     loop); validator strips/rejects `den.*` from caller labels.
   - **Destructive ownership requires ALL THREE: network labeled `den.managed=true` (label, NOT
     name) AND every attached container name-prefixed `den-<id>` AND labeled `den.id` AND
     present in Den's store.** A **name-only** match (configured `network_id`, no Den label) is
     **inspect-only / fail-loud — NEVER sufficient for any mutation** (a configured name can
     collide with an operator-owned network — Codex iter6 W). **Store read failure /
     corruption / unavailable ⇒ fail-closed BEFORE any Docker mutation.** Any
     mismatch/ambiguity → fail-loud, never touch.
   - **Mismatch predicate (v9 — Codex iter8 WARN: ANY-deviation / OR, NOT AND):** the network
     is stale iff **ANY** of: `Inspect().Internal != (desiredMode==internal)` (covers the
     operator `internal→bridge` migration — the v8 AND-joined predicate missed this and left
     port-forwarding broken on the existing network), **OR** `Inspect().EnableIPv6 != false`,
     **OR** the `den.network.icc` label/value ≠ desired (Options string only for
     legacy/unlabeled). Each compared against the *desired* config, not a fixed constant.
   - Destroy only under `DEN_RUNTIME__RECONCILE_NETWORK=true` (env/yaml). Sequence: stop →
     `NetworkDisconnect(ctx, net, ctr, force=true)` → `NetworkRemove`. **`NetworkRemove` 403
     `active endpoints` ⇒ fail-loud, never force-loop** (`ActiveEndpointsError`→403,
     source-verified iter7; `NetworkRemove` has no force option). **`404` on
     `NetworkDisconnect` ⇒ continue (idempotent); `404` on `NetworkRemove` ⇒ success.**
     Recreate; **`409 NetworkNameError` (name OR ID collision, both `libnetwork` paths,
     source-verified iter7) ⇒ re-inspect, re-run the FULL mismatch predicate, succeed only if
     it now matches, else fail-loud, never re-destroy.** No auto-restart. No-op happy path is
     a tested invariant.
   - **`Reconcile` is a concrete `*docker.DockerRuntime` method, signature pinned
     `Reconcile(ctx context.Context, st store.Store) error`** — store as **method parameter**.
     `store.Store` is a 10-method interface; the failing test stub **embeds a nil/no-op
     `store.Store` and overrides only `ListSandboxes`/`GetSandbox`** (Go embedded-interface
     pattern; no `StoreReader` type, no 10-method hand-roll — Testing iter7 🟡). Engine-layer
     tests construct the concrete `*docker.DockerRuntime` and call `rt.Reconcile(ctx,
     failingStoreStub)`. **Intra-/cross-phase ordering (Codex iter7 W):** Phase 1's label-
     hardening + `den.network.*` managed labels land **before** Phase 2; Phase 2 reconcile
     tests run **only against post-hardening labels**, never pre-hardening.
6. **Delete dead `PortForwarder`; DooD unsupported**; API tests assert **both** `POST` and
   `DELETE /ports` → 501.
7. **Fix the absent-`ports` API bug in Phase 1 — a SINGLE one-line assignment.**
   `engine.CreateSandbox`'s `Sandbox{…}` literal (`engine.go:258-264`) gains `Ports:
   cfg.Ports`. **The response/handler structs (`sandboxResponse`, `engine.Sandbox`,
   `toSandboxResponse`, `toResponse`) are ALREADY correct and MUST NOT be modified** (API/SDK
   iter7 C1 ★). **Post-fix contract is "`ports` present iff non-empty"**, NOT "always
   populated": all three response paths use `omitempty` (Go) / `?` (TS) / `| None` (Python),
   so a zero-port sandbox still omits the key **by design** — the fix changes "absent for ALL
   sandboxes" → "present for sandboxes that HAVE ports" (API/SDK iter7 C2 ★). v8 explicitly
   adopts option (a) keep-`omitempty`-document-precisely; rejects option (b) drop-`omitempty`
   (a larger shape change, and existing SDK consumers already tolerate the optional key).
   `GET /sandboxes/{id}` **and** `GET /sandboxes` both go absent→present-iff-non-empty.
8. **Validation at a single chokepoint INSIDE `engine.CreateSandbox`**: new
   `internal/runtime/netpolicy` package (importable by `engine`/`handlers`/`mcp`, no cycle —
   `runtime`/`config` are stdlib-leaf, Pragmatist iter7 source-verified). `engine.NewEngine`
   gains the policy param (**5 call sites: 2 production wire it, 3 test pass zero**).
   `CreateSandbox` (a) **resolves the effective mode** and writes `cfg.NetworkMode`; (b) calls
   the validator; on failure returns a **typed `netpolicy.ValidationError`** (un-wrapped).
   Handlers add `errors.As(&netpolicy.ValidationError{})` → **stable 400 body**.
   `mcp.NewServer` **unchanged**. Validator enforces: per-sandbox ceiling `{"",none}`;
   `none`+ports; `Protocol` `{"","tcp"}` case-insensitive **normalized to lowercase**; **both
   `SandboxPort` AND `HostPort` in `1..65535`** (`HostPort=0` rejected — Open Q1); `den.*`
   label strip. `createSandboxRequest` and MCP `createSandboxArgs`+`InputSchema` gain
   `network_mode`. **JSON-tag invariant scoped (API/SDK iter7 W1):** `network_mode,omitempty`
   pinned identically across the **3 encode/round-trip structs** (runtime config,
   `createSandboxRequest`, Go SDK `SandboxConfig`); MCP `createSandboxArgs` is **decode-only**
   so its tag value is non-load-bearing — `network_mode` is added there matching that struct's
   local **no-`omitempty`** convention, and the invariant **intentionally does not extend to
   it** (stated, not an oversight).
9. **SDK capability negotiation via an explicit feature token:**
   - Server: `features: []string` on `/api/v1/version` **and** `/api/v1/health` (typed struct;
     **additive only**; `/health` external-probe risk bounded to exact-body/strict-schema).
     `"network_mode"` set **unconditionally in any binary with the feature code**, independent
     of build `version`.
   - **Capability hint, NOT authentication** (plaintext, forgeable). Fail-closed: if `features`
     lacks `"network_mode"` when the caller set one, the SDK **errors before create**.
     SECURITY.md states this.
   - SDK check **lazy + scoped**: only when the create call's network-mode argument is a
     **non-empty string** (`None`/`undefined`/`""`/unset-kwarg all ⇒ inherit ⇒ **no
     `/version` call, no error**). Single endpoint **`GET /api/v1/version`**
     (Python: `f"{_base_url}/version"` = `/api/v1/version`, matches TS — API/SDK iter7 S1).
     Entrypoints: Go `Client.CreateSandbox` (`client.go:131`), TS `SandboxManager.create`
     (`sandbox.ts:324`, required config object — mode via `config.network_mode`), Python
     `SandboxManager.create` (`sandbox.py:454`, **dual path: `network_mode=` kwarg OR
     `config.network_mode`**). **Phase 5 asserts, per SDK, the trigger AND the no-call path;
     Python additionally covers `network_mode=None` kwarg, `SandboxConfig(network_mode=None)`,
     AND unset-kwarg ⇒ all no-call** (API/SDK iter6 W3/iter7 S2).
   - SDK `0.1.0` published to npm/PyPI **only after the `v0.1.0` tag**; in-repo
     `package.json`/`pyproject.toml` `0.1.0` edit lands in the feature PR (safe, outside the
     manifest; harmless one-Release-PR-cycle in-repo lead — API/SDK iter7 S3).
   - **Go SDK *response* struct `Sandbox` gains `Ports []PortMapping \`json:"ports,
     omitempty"\``** (`client.go:107-113`; `PortMapping` already in-package with
     `sandbox_port`/`host_port`/`protocol,omitempty` — TS `types.ts:101-102` / Python
     `types.py:78` already model response `ports`; zero cross-SDK divergence — API/SDK iter7
     D3). **TS `version()` return type gains `features?: string[]` (optional — non-optional
     would itself break against a `"dev"`/old server); Python `version()`/`aversion()` are
     untyped `dict` pass-throughs (`client.py:99-117`) → a docstring line stating `features`
     is the COMPLETE contract, NO typed model added** (avoids a needless Pydantic boundary —
     API/SDK iter7 S1; Phase 4 states this chosen option explicitly).

### Trade-offs accepted

- Default `internal` → port-forwarding needs one explicit opt-in. **"Secure-by-default" =
  no-unauth-state-changing-control-plane with the local-socket co-residency residual
  REFUSED-by-default (mandatory `platform_override` attestation), NOT tenant containment**
  (scoped, not unqualified).
- **`bridge` refusal-by-default (`allow_unsafe_bridge`) until egress filtering;** runs in HTTP
  + MCP.
- **Bind guard is platform-aware, positive-allowlist, CLIENT-OS-GATED, fail-closed against
  every machine-detectable unsafe topology, and REFUSE-BY-DEFAULT on the undetectable
  local-`unix://`-proxy residual (mandatory `platform_override` co-residency attestation),
  default-ON (`allow_unsafe_bind` opt-out)** (v9 operator decision). macOS/Windows host,
  Docker Desktop, rootless, remote-daemon, unknown ⇒ refuse. **Native-Linux loopback/auth-off
  with `platform_override` UNSET ⇒ REFUSE** (the v8 permit-with-WARN path is removed —
  Codex/Security iter8 deemed a habituated-WARN-on-default security theater). **Every
  existing default-config deployment (incl. the common native-Linux loopback auth-off one)
  sets the `platform_override` attestation flag (or `auth.enabled`/`none`) on upgrade —
  operator-accepted breaking operational change** in exchange for genuine secure-by-default.
- **`runtime.platform_override` is the v9 MANDATORY loopback-branch co-residency attestation
  AND risk-equivalent to bypassing the classifier, bind-guard-scoped** (single literal, same
  loud per-start ERROR as `allow_unsafe_bind`, MUST NOT be read as a true platform fact
  elsewhere).
- **The bind guard closes ONLY the control-plane escape, only under the presumed co-residency.**
  `internal` is not tenant-contained until the deferred egress PR. Only `none` is a v1 boundary.
- **Shared host kernel + opt-in `bridge` blast radius:** the SECURITY.md matrix carries an
  explicit consequence column "untrusted-code-with-kernel-CVE can pivot to RFC1918/metadata"
  for the `bridge`+unfiltered cell (Security iter6 🟡#5).
- **The feature token is a capability hint, not authentication.**
- **SDK create errors when a non-empty network mode is set against a feature-less server**
  (lazy/scoped; zero-network-mode users unaffected).
- `Protocol` non-`tcp` → 400: documented **breaking** input change; the Phase-1 validator
  `feat!:` commit body **enumerates ALL breaking validations**; release outcome (`0.1.0`)
  unaffected.
- Per-sandbox override only increases isolation; requesting the global mode → 400.
- **`ports` present iff non-empty** (not unconditionally present) — documented, all 3 SDKs.
- DooD port access unsupported. Host-firewall egress filtering deferred (Open Q3).
- Adding `features` changes `/version`+`/health` response shape (additive; exact-body probes).

### What we are NOT doing (scope guard)

- No host iptables/`DEN-EGRESS`/IPv6-ip6tables in v1 (follow-up; covers `internal` too).
- No dynamic port add/remove (`port.go` 501; both verbs tested).
- No egress allowlist/proxy/domain policy in v1.
- No change to container hardening (`CapDrop: ALL`, `no-new-privileges`, `ReadonlyRootfs`,
  seccomp, PID limits); `enable_icc=false`'s L2 mitigation sound **only because `NET_RAW`
  dropped** — comment + tripwire test.
- No UDP (non-`tcp` → 400).
- **No wiring of `runtime.docker_host`** — it stays inert; a test asserts (a) it cannot affect
  the client AND (b) `DOCKER_HOST` env still moves `DaemonHost()` (Codex iter7 W); the
  classifier uses `client.DaemonHost()` only.
- **No machine proof of socket locality** — the local-`unix://`-proxy residual is
  **REFUSED-by-default and only permitted under an explicit `platform_override` operator
  attestation**, NOT detected and NOT presumed-with-WARN (v9; Open Q5).
- No reliance on the daemon to reject a misconstructed `none` spec; correctness enforced by
  the `buildContainerCreateSpec` unit test **AND** the production create→assert→(remove|start)
  post-create `Inspect()` assertion.
- No modification of the already-correct `ports` response structs — only the one
  `engine.CreateSandbox` assignment (API/SDK iter7 C1).
- No automatic platform "fix" — on unsafe/unknown the guard refuses, never silently rebinds.
- No per-sandbox connected-mode override; no multi-network management.
- No change to Den's default bind/auth values (guard + docs only).
- No CLI flags (no flag layer; env/yaml only).

## Phases

### Phase 0 — Fix the integration test harness (gating prerequisite)

**Files:** `Makefile:12-16`, `.github/workflows/ci.yml:56-75`.

1. `test-integration`: `go test -tags integration ./internal/... ./tests/... -run TestIntegration -v`.
2. CI: mirror tag/path/run; **`env: { DOCKER_TLS_CERTDIR: "" }` on the dind *service* block**
   + `DOCKER_HOST=tcp://docker:2375` + a `docker info` pre-step that **FAILS the job** if
   Docker is unreachable. **v9: NO container-discovery step (dropped — not portable). The
   single CI topology is `services: docker:dind` + `DOCKER_HOST=tcp://docker:2375`; Phase-5
   CI assertions are API-level only (Inspect-Ports sentinel + one-shot `docker run --rm`
   exit-code probe), needing no container-name resolution** (Testing iter8 🟡 / Codex iter8
   WARN). Trigger += `pull_request`; **update BOTH `integration-test` and `sdk-test` `if:`**.
   **Min-test-count gate**. State the bootstrap paradox (Phase-0 PR runs under old `push` →
   validated by a follow-up push + local).
3. **Naming + build-tag meta-check** (fail if any `tests/integration/*_test.go` lacks
   `//go:build integration` or has an exported `TestX` not `^TestIntegration_`); also flag the
   dropped `EnsureNetwork` errors at `mcp.go:57` and `storage_test.go`.
4. Prove `storage_test.go` compiles+runs; fix its dropped `EnsureNetwork` → `require.NoError`.

**Effort:** ~0.5d. **Risk:** Low.

### Phase 1 — `netpolicy`, modes, validator, allowlist bind guard + presumption disclosure, bridge refusal

1. `type NetworkMode string`+constants (`runtime.go`); `NetworkMode`
   (`json:"network_mode,omitempty"`) on `SandboxConfig`.
2. `config.go`: `RuntimeConfig` += `DefaultNetworkMode`, `ReconcileNetwork`,
   `AllowUnsafeBridge`, `AllowUnsafeBind`, `PlatformOverride` (koanf tags). Defaults:
   `DefaultNetworkMode:"internal"`, others `false`/`""`. `Validate()` enum-only **incl.
   `PlatformOverride ∈ {"", "linux-native-docker-co-resident"}` case-sensitive** (any other →
   start refusal — Codex iter6 C4). New `Warnings()` method + ~4-line 2-site emission.
3. `internal/runtime/netpolicy`: (a) validator (ceiling, none+ports, protocol normalize-to-
   lowercase, **both-side** port range, `den.*` strip) with typed `ValidationError` +
   **one exported committed-string const block**; (b) pure `classifyHost(host)`,
   **`classifyPlatform(info system.Info, effectiveDockerHost string, clientGOOS string)
   runtimePlatform`** (positive allowlist incl. `clientGOOS=="linux"` + empty-`SecurityOptions`
   -passes polarity + scheme-prefix host match), **`BindGuardDecision(auth, hostClass, mode,
   runtimePlatform, platformOverrideAttested, allowUnsafeBind) bool`** (v9: loopback branch
   requires `platformOverrideAttested==true`), `bridgeRefusalDecision(...)`,
   `platformOverrideDecision(...)`. **v9: `RequiresCoResidencyPresumptionDisclosure` is
   REMOVED** (no permit-with-WARN path remains — the loopback branch now REFUSES unless
   attested; the attestation's existing ERROR log is the disclosure). All unit-tested
   standalone (no socket, no live Docker — injected `system.Info`+strings).
4. Extract pure `buildContainerCreateSpec(cfg, networkID, mode) (containerCfg, hostCfg,
   networkCfg, error)` — explicit `networkID`+`mode`; `none` ⇒ empty `EndpointsConfig`,
   `r.networkID` NOT substituted, `NetworkMode("none")`, `PortBindings` empty, `ExposedPorts`
   empty (unit test asserts the last two **separately**).
5. `docker.go`: struct += `networkMode/reconcileNetwork/allowUnsafeBridge` + opts; **add
   distinctly-named system accessors `DaemonHost() string` (no ctx, no error — source-verified)
   and `SystemInfo(ctx) (system.Info, error)` — NOT colliding with the container-inspect
   `Info(ctx,id)`** (Pragmatist iter6 W2); `EnsureNetwork` (`none`→no-op; else create
   `Internal:(mode==internal)`, `enable_icc=false`, `EnableIPv6:ptr(false)`, labels; **API≥1.42
   floor assert AND post-create `Inspect().EnableIPv6==false` re-verify**); reorder label
   merge; `Create` reads `cfg.NetworkMode`, uses `buildContainerCreateSpec`, **adds the
   production post-create assertion ordered create→`Inspect()`(nil-guard NetworkSettings)→
   (breach? ERROR-log NetworkSettings JSON + force-remove + wrap-cleanup-err + committed
   error | proceed)→start** for `none`, where **v9 breach = `Networks` has any key ≠ `"none"`
   OR any endpoint with non-empty `EndpointID`/`NetworkID`/`IPAddress`** (NOT "Networks
   non-empty" — Docker-correctness iter8 🔴) (Security iter6 🟡#1/iter7 🟡#2; Codex iter7 W);
   reject non-`tcp` (defense-in-depth). The `engine.CreateSandbox` one-line `Ports: cfg.Ports`
   assignment (Approach §7) lands here too; response structs untouched.
6. `engine`: `NewEngine` += netpolicy param (**5 call sites; 2 prod wire, 3 test zero**);
   `CreateSandbox` **resolves effective mode → `cfg.NetworkMode`**, calls validator (typed
   `ValidationError`), sets `sandbox.Ports = cfg.Ports`.
7. `sandbox.go` `createSandboxRequest` += `network_mode` (tag `network_mode,omitempty`);
   handler adds `errors.As(&netpolicy.ValidationError{})` → stable 400. MCP `createSandboxArgs`
   **+ `InputSchema`** += `network_mode` (decode-only, no-`omitempty`, invariant-exempt);
   MCP handler same `errors.As`.
8. `cmd/den`: **`rootCmd.SilenceUsage = true`**. `main.go`/`mcp.go`: `SystemInfo`+`DaemonHost`
   + `runtime.GOOS` → `classifyPlatform`; **guards (bind+bridge-refusal+platform-override)
   run and abort FIRST**; **bind guard no-op without HTTP listener; bridge-refusal ALWAYS
   runs**; `EnsureNetwork` fatal; **v9: native-Linux loopback auth-off with `platform_override`
   UNSET ⇒ `BindGuardDecision` returns unsafe ⇒ REFUSE (non-zero exit, committed message with
   safest-first remediation incl. the attestation flag) — NO permit-with-WARN path**;
   `allow_unsafe_bind=true`/`platform_override` set ⇒ **ERROR-level log every start** (naming
   the attested precondition + the socat/SSH-forward/docker-context void condition); committed
   strings to stderr; emit `Warnings()`.
9. `den.example.yaml`/`configuration.md`: modes, default, ceiling, env names, the **platform
   safety matrix** (incl. `clientGOOS`, remote-daemon, macOS-VM-via-unix-socket, **and the
   local-`unix://`-proxy presumed-residual** rows), co-residency *presumed* precondition,
   `platform_override` contract, "bridge unsafe; `runtime.docker_host` inert".

**Effort:** ~5.5d (v9: REMOVES the v8 presumption-disclosure fn & WARN wiring — net simpler;
the `platformOverrideAttested` clause is one extra bool input to `BindGuardDecision`; keeps
platform_override bind-guard-scoping test, none ordered-cleanup w/ v9 predicate, ports
one-liner). **Risk:** Med.

### Phase 2 — Safe reconciliation (operator-initiated mode change only)

Per Approach §5: triple-ownership with **label-only (never name-only) destructive gate**,
store-fail-closed before any Docker mutation, `Inspect().EnableIPv6` predicate,
`NetworkDisconnect(force=true)`, 404-on-disconnect=continue, 403-active-endpoints=fail-loud,
404-on-remove=success, 409-NetworkNameError(name|ID)=concurrent-actor→full-predicate-re-verify
(never re-destroy), env/yaml opt-in destroy, no auto-restart, no-op invariant, `none`→no-op.
**Signature pinned `func (r *DockerRuntime) Reconcile(ctx context.Context, st store.Store)
error`**; failing stub embeds a no-op `store.Store`, overrides `ListSandboxes`/`GetSandbox`
(Testing iter7 🟡). **Lands AFTER Phase-1 label-hardening; tests only against post-hardening
labels** (Codex iter7 W).

**Effort:** ~1.25d. **Risk:** Med.

### Phase 3 — Delete `PortForwarder`; DooD scope

Delete `PortForwarder`+helpers; `port.go` 501 explicit message; **API tests assert both `POST`
and `DELETE /ports` → 501**; fix `architecture.md:74`; add "DooD port access unsupported" to
`architecture.md`+`SECURITY.md`.

**Effort:** ~0.25d. **Risk:** Low.

### Phase 4 — Docs / SECURITY.md / SDK reconciliation (committed wording)

- `README.md:188,205-206`, `README.zh-CN.md:205`: three modes + `internal` default; explicit
  188-vs-205 reconciliation note; **operator top-line: "`internal` does NOT contain a sandbox;
  only `none` is a tenant boundary."**
- `SECURITY.md` — committed: (1) the **PLATFORM SAFETY MATRIX** as primary artifact, axes
  {clientGOOS linux/non-linux} × {linux-native / Docker-Desktop / rootless / remote-daemon /
  macOS-VM-via-unix-socket / **local-unix-socket-proxied-to-remote (presumed-residual)**} ×
  {internal/bridge/none} × {auth on/off} × {loopback/non-loopback}, consequence columns incl.
  escape-open? other-egress-open? kernel-CVE→RFC1918/metadata pivot?; co-residency named as a
  **presumed (machine-unverifiable) precondition with a mandatory per-start WARN**; loopback
  insufficient on Desktop/rootless/remote/non-linux-client. (2) `/version`,`/health`,dashboard
  unauth even with auth on. (3) shared kernel not a boundary. (4) SSRF guard = Den S3 only;
  `bridge` unfiltered/opt-in-unsafe; egress follow-up covers `internal`. (5)
  `enable_icc=false` sound only because `NET_RAW` dropped; not container→gateway→host; daemon
  floor API≥1.42. **(6) explicit operator-facing sentence: in `internal` the embedded-DNS
  resolver (127.0.0.11) + bridge gateway path to OTHER host `0.0.0.0` services is UNCONTAINED
  in v1; DNS-rebind applies; only `none` contains it; deferred egress (Open Q3) covers
  `internal`** (Security iter7 🟡#3 — no longer a bare label). (7) rejected
  connectivity-on-default + deferred egress, recorded. (8) Den-as-iptables-root out of v1.
  (9) feature token = capability hint, NOT auth. **(10) the local-`unix://`-socket-proxied-
  to-remote residual enumerated as a REALISTIC, NOT-RARE class (socat UNIX-LISTEN…TCP:remote,
  ssh -L …docker.sock, docker-context-proxied, bind-mounted rootless/sibling socket): on such
  a host the guard permits the start under the presumed precondition AND emits the mandatory
  per-start WARN; `platform_override` is the explicit acknowledgement; `auth.enabled=true` or
  `none` is the real mitigation** (Security iter7 🟡#1 ★ / Codex iter7). (11)
  `platform_override` risk-equivalence + bind-guard-scoped (not a platform fact elsewhere).
- `docs/.../architecture.md:74,147,148`, `rest-api.md:239,249`, `api-reference.md:590,609`,
  `configuration.md:23`: align; `POST/DELETE /ports`→501; document the request-the-global-mode
  →400 footgun + the *presumed* co-residency precondition + the per-start WARN.
- **SDKs:** add `network_mode` (tag `network_mode,omitempty` — the 3 encode structs; MCP
  decode-only exempt) to Go `client.go` (~:94-104), TS `types.ts`, Python `types.py`; **add
  `Ports []PortMapping \`json:"ports,omitempty"\`` to the Go `Sandbox` *response* struct
  (`client.go:107-113`); response structs elsewhere already correct — DO NOT touch** (API/SDK
  iter7 C1). **TS `version()` return gains `features?: string[]`; Python `version()`/
  `aversion()` documented (docstring) to include `features` — NO typed model** (API/SDK iter7
  S1; chosen option stated). Scrub `"udp"` from `PortMapping.protocol` SDK doc-comments
  (`types.ts:185`, Python). Add the lazy/scoped feature-token check at the named per-SDK
  create entrypoints (Python dual kwarg/config path covered). State the post-fix **"`ports`
  present iff non-empty"** contract in SDK docs. Manual `0.1.0` edit in
  `package.json`/`pyproject.toml` same-PR; **publish after the `v0.1.0` tag** (note the
  harmless one-Release-PR-cycle in-repo lead). **Deferred (Open Q4):** SDKs as release-please
  packages.

**Effort:** ~1.0d. **Risk:** Low. **Gate:** SECURITY.md (1)-(11) + the platform matrix re-reviewed.

### Phase 5 — Tests + machine-checkable e2e (dind-runnable + documented local-only legs)

**Per-row Den startup config (resolves the CI bind-guard deadlock — Codex iter6 C3 ★):** dind
is `DOCKER_HOST=tcp://docker:2375` ⇒ `classifyPlatform=unknown` ⇒ default `0.0.0.0`/auth-off
start refused. Each row that needs Den's HTTP API **explicitly sets** its startup config (the
**Den cfg** column). Rows 1-5, 9-12 = **`auth.enabled=true`+API key** (safe via the auth
clause regardless of `unknown`; also exercises the auth-on branch); row 7 =
`allow_unsafe_bind=true`; rows 6, 8 deliberately unset (they test refusal).

**dind daemon addressing (Testing iter7 🔴 / Codex iter7 W):** rows 1/2/3/5 reach the dind
daemon/containers via the **Phase-0 discovery-step-resolved container name** (`docker ps
--filter` by the Actions label) on the runner's LOCAL socket (no `DOCKER_HOST`), OR a
step-launched `--name den-dind`. If neither is portable in the target CI, **row-1's host-curl
leg is a documented LOCAL-ONLY leg** and the CI-portable assertion is `Inspect().
NetworkSettings.Ports["x/tcp"]` non-empty (the sentinel) — the marquee `curl==200` is then a
local-only proof, explicitly labeled (parallels the positive-path local-only leg).

**Shared committed-string constants** in **one exported `netpolicy` const block** imported by
production AND tests — never hand-copied. **Per-suite executed-test floor (Testing iter7 🟒 —
`testing.M.Run()` returns no count, so the v7 "TestMain records the count" is unworkable):**
implemented as a **package-level counter incremented by a one-line `netpolicytest.Mark(t)`
(`t.Helper()`) at the top of each suite test**, asserted ≥ a per-package floor in `TestMain`
after `m.Run()`. Mechanism named and workable.

Unit: validator (ceiling incl. `"None"`/whitespace/`null`/value-equal-to-global, none+ports,
protocol `{"","tcp"}` incl. `tcp`/`TCP`→`8000/tcp` & `udp`→400, **both** SandboxPort+HostPort
range, `den.*` strip — HTTP-handler **AND** MCP-path); `classifyHost`
(`""`,`0.0.0.0`,`::`,`::1`,`127.0.0.1`,`localhost`,LAN-IP,unparseable); **`classifyPlatform`
truth table — POSITIVE allowlist with `clientGOOS`:** native-linux+linux-client+unix ⇒
linux-native; **darwin-client+linux-daemon+`unix://` ⇒ unknown (Colima/OrbStack, Codex iter6
C1 ★); windows-client ⇒ unknown**; "Docker Desktop"/linuxkit/WSL2 ⇒ unknown; `name=rootless`
⇒ unknown; **empty/sparse `SecurityOptions` parses ⇒ (f) passes**; malformed `SecurityOptions`
⇒ unknown; `tcp://`/`ssh://`/`npipe://` ⇒ unknown; **empty (defensive-only)/`unix://` default
⇒ local; custom `unix:///path` ⇒ local**; `docker info` error ⇒ unknown; OSType≠linux ⇒
unknown; **`BindGuardDecision` truth table over auth × hostClass × mode × platform ×
platformOverrideAttested × allowUnsafeBind — v9 KEY ROWS: native-Linux loopback auth-off
`platformOverrideAttested=false` ⇒ UNSAFE/REFUSE; same with `platformOverrideAttested=true`
⇒ SAFE; auth-on or `none` ⇒ SAFE regardless of attestation** (asserts the v9 REFUSE-unless-
attested decision; `RequiresCoResidencyPresumptionDisclosure` and its truth table are
REMOVED — no permit-with-WARN path exists);
`platformOverrideDecision` (`""`→no-force; literal→force+ERROR-contract; other→config error);
**platform_override-derived `runtimePlatform` is NOT read outside `BindGuardDecision`** (Codex
iter7 W); **guard-ordering test via an injected `func(context.Context) error` startup seam
(default `rt.EnsureNetwork`) + ordered recorder; the test substitutes the func and asserts the
guard recorder entry precedes the EnsureNetwork entry — no live daemon, no interface over
EnsureNetwork** (Testing iter7 🟡 — injection point pinned); **MCP structural test: `go/parser`
AST-scan of `cmd/den/mcp.go` asserts zero `net.Listen`/`http.Server`/`http.ListenAndServe`/
`Serve` refs — single-file, NOT transitive; + comment tripwire** (Testing iter7 🟡);
**bridge-refusal-runs-in-MCP test**; `bridgeRefusalDecision`; `buildContainerCreateSpec`
(`none` ⇒ empty `EndpointsConfig` AND `r.networkID` NOT substituted AND `NetworkMode=="none"`
AND **`PortBindings` empty AND `ExposedPorts` empty as separate asserts**; per-port
`HostIP:"127.0.0.1"`; non-nil `false` `EnableIPv6`; caller `cfg.Labels["den.id"]="spoofed"`
does NOT override Den-set `den.id`); config enum + `PlatformOverride` validation +
`DEN_RUNTIME__DEFAULT_NETWORK_MODE` override; **`runtime.docker_host` set + `DOCKER_HOST`
unset ⇒ `DaemonHost()` default unchanged, AND `DOCKER_HOST=tcp://x` ⇒ `DaemonHost()` changes**
(Codex iter6 C2/iter7 W — both legs); **`GET /sandboxes/{id}` AND `GET /sandboxes` →
ports-present-iff-non-empty (a zero-port sandbox omits the key — the post-fix contract);
non-zero-port sandbox populates it; Go SDK `Sandbox` round-trips a `ports` array**; **the
response/handler structs are unchanged (a test importing `handlers`/`engine` asserts the
pre-existing `ports` tags still present — guards against a wrong-layer edit)**; **`EnsureNetwork`
error ⇒ non-zero exit**; **per-SDK (Go/TS/Python): non-empty network mode + feature-less
`/version` ⇒ loud error; `""`/`None`/`undefined`/unset ⇒ no `/version` call even vs a `"dev"`
server — Python additionally `network_mode=None` kwarg & `SandboxConfig(network_mode=None)` &
unset-kwarg** (API/SDK iter6 W3/iter7 S2); **TS test infra is built from zero in this phase
(no existing `*.test.ts`, no mock lib): a bun test file using `mock`/`spyOn` on global
`fetch`** (Testing iter7 🟡 — mechanism named, scaffolding budgeted).

Integration (`//go:build integration`, `TestIntegration_*`, unique per-test network names,
force-removed in `t.Cleanup()` even on failure with loud cleanup-failure log):

| # | Mode | Den cfg | Ports | Expectation |
|---|---|---|---|---|
| 1 | bridge(`allow_unsafe_bridge`) | auth-on+key | mapped | `Inspect` `NetworkSettings.Ports["x/tcp"]` non-empty (CI-portable sentinel) **AND the marquee e2e via the Phase-0-discovered dind container name on the runner's LOCAL socket (no `DOCKER_HOST`)** `curl http://127.0.0.1:<hp>`→200+body grep; if dind addressing non-portable in target CI the curl leg is documented LOCAL-ONLY, sentinel stays the CI gate (Testing iter7 🔴/Codex iter7 W) |
| 2 | bridge | auth-on+key | none | in-container `getent hosts <h>`+HTTPS GET via the discovered daemon. **Baseline on default bridge first; 4 DNS×TCP quadrants:** ok+ok→real test (Fatal on den-net fail); fail+fail→Skip; ok+fail→Skip; **fail+ok→Skip**. Baseline in a dedicated default-bridge container; den-net assertion in a separate den-net container |
| 3 | internal | auth-on+key | mapped | host/in-dind GET→refused/timeout; external egress fails — **run ONLY if the cell-2 default-bridge baseline proved egress works, else Skip** |
| 4 | none | auth-on+key | mapped-req | create → **HTTP 400** |
| 5 | none | auth-on+key | none | **CI (API-level): `Inspect` `HostConfig.NetworkMode=="none"` AND `NetworkSettings.Networks` has ONLY the `"none"` key (no key ≠ `none`, no populated EndpointID/NetworkID/IPAddress — v9 predicate)**; one-shot `docker run --rm` exit-code probe ⇒ external GET fails. **LOCAL-ONLY:** container has exactly `{lo}`, no `inet6 … scope global` — `ip addr` output parsing (NOT `ip addr show eth0`), AFTER container-running readiness |
| 6 | bind-guard | auth-off + `0.0.0.0` + internal, `allow_unsafe_bind` unset | — | Den exits non-zero **AND `strings.Contains(stderr, <const>)` AND negative: the row-7 allow-path start does NOT contain it** |
| 7 | bind-guard | same + `allow_unsafe_bind=true` | — | Den starts **AND an ERROR-level log line with the committed escape wording** |
| 8 | bridge-refusal | `default_network_mode:bridge`, `allow_unsafe_bridge` unset | — | Den exits non-zero **AND committed stderr substring**; with flag → starts; **also asserted in MCP-only mode** |
| 9 | reconcile | auth-on+key | — | concrete `*docker.DockerRuntime`; `den-net` `Internal:true`+Den labels; `rt.Reconcile(ctx, realStore)` → match, no mutation, nil |
| 10 | reconcile | auth-on+key | — | concrete type; **name-only (no `den.managed` label)** → `rt.Reconcile()` returns **typed actionable error, network unmodified** (name-only never destructive) |
| 11 | reconcile | auth-on+key | — | concrete type; `RECONCILE_NETWORK=true` + full ownership → clean recreate, no orphaned/auto-restarted sandbox; **store-read-failure: `rt.Reconcile(ctx, embeddedFailingStoreStub)` ⇒ fail-closed, no Docker mutation** |
| 12 | none (negative regression + ordering witness) | auth-on+key | none | global `internal`/`bridge`, **per-sandbox `none`**; create → `Inspect` `HostConfig.NetworkMode=="none"` **AND `NetworkSettings.Networks` has ONLY the `"none"` key (v9 predicate — NOT "empty"; a correct `none` container always carries `{"none":{zeroed}}`) AND the container was NEVER started before the post-create assertion** (the create→assert→start TOCTOU-closed invariant — Security iter7 🟡#2 / Docker-correctness iter8 🔴; doubles as the production-assertion + ordering witness) |

- Readiness: bounded poll (N attempts, fixed interval, hard timeout); **no `sleep`**;
  container-running readiness separated from port/DNS/IPv6 assertions.

**Manual e2e — machine-checkable** `scripts/e2e-network.sh` (non-zero exit per *named*
assertion; `tee`s transcript; greps the shared committed substrings): bridge serve→publish→
`curl`==200+body; same sandbox DNS+HTTPS ok; recreate `internal` (control: paired bridge
sandbox still serves) → publish refused + external egress fails; **bind-guard:**
auth-off+`0.0.0.0`+internal without `allow_unsafe_bind` → non-zero + exact committed message.
**Documented LOCAL-ONLY positive-path leg** (CI structurally cannot prove the allow path): on
a native-Linux host with a local `unix://` socket, `server.host=127.0.0.1`, auth-off,
`internal`: **(neg) `platform_override` UNSET ⇒ Den REFUSES (non-zero exit + committed
attestation-remediation message)**; **(pos) `platform_override="linux-native-docker-co-
resident"` set ⇒ Den STARTS, a sandbox CANNOT reach the control plane via the gateway, AND
the ERROR-level attestation log line (committed wording naming the void condition) is
present** (the v9 operator-decision proof — replaces the v8 WARN leg). "Identical
Linux/macOS" claimed ONLY for the iptables-free guard legs.

**Effort:** ~2.75d (v9: NO dind-discovery harness — API-level CI gate is simpler; NO
RequiresCoResidency truth table — removed; + the v9 BindGuard attestation rows, the
REFUSE/start-with-attestation local leg, TS-test-infra-from-zero, response-structs-unchanged
guard test, the one-shot exit-code probe, the `Mark(t)` count mechanism). **Risk:** Med.

## Verification

```
go build ./... && go vet ./...
make test                 # validator, classifyHost, classifyPlatform(clientGOOS/effective-host), BindGuard(+platformOverrideAttested v9 rows), platformOverride(+scope+attestation), guard-ordering(injected seam), MCP-AST-structural, bridge-refusal-in-MCP, spec(v9 none predicate), config, docker_host-inert(both legs), ports-present-iff-non-empty, response-structs-unchanged, Go-SDK ports, per-SDK token ×3(+Python kwarg cases)
make test-integration     # -tags integration, dind TLS service env, DOCKER_HOST=tcp://docker:2375, API-level CI gate (Inspect-Ports sentinel + one-shot exit-code probe), per-row Den cfg, 12-row matrix, min-count>0
./scripts/e2e-network.sh  # machine-checkable; guard legs identical Linux/macOS; +local-only leg (REFUSE w/o attestation; starts + ERROR attestation log WITH it)
```

**Committed error/message strings** in one exported `netpolicy` const block (Phase 1):
unsafe-bind refusal (v9: incl. the "set `platform_override` to attest co-residency"
remediation), unsafe-bridge refusal, `none`+ports 400, invalid per-sandbox mode 400,
invalid protocol 400, invalid port-range 400, `allow_unsafe_bind` per-start ERROR,
`platform_override` per-start ERROR (v9: names the attested precondition + socat/SSH-forward/
docker-context void condition — this IS the disclosure; the v8 per-start WARN is REMOVED),
`none` post-create-assertion failure (incl. the offending-network identity).

Commits via `/commit`: `feat:` (network_mode surface, allowlist bind guard + presumption
disclosure, bridge refusal, feature-token endpoint, `platform_override`), `fix:` (port-
forwarding in bridge; absent-`ports` API one-liner; Go-SDK `Ports` response field), **one
`feat!:` Phase-1 validator commit whose body ENUMERATES ALL breaking validations** (Protocol
non-`tcp`→400; `none`+ports→400; per-sandbox ceiling→400); `docs:`. **Release-note buckets:**
*Operator-visible* (EnsureNetwork fatal; allowlist+clientGOOS bind guard default-ON;
**v9 BREAKING: native-Linux loopback auth-off REFUSES unless `platform_override=
"linux-native-docker-co-resident"` attests co-residency (or `auth.enabled=true`/`none`) —
every existing default-config deployment sets one flag on upgrade**; bridge refusal-by-default;
new keys incl. `platform_override`; loopback insufficient on Desktop/rootless/remote/
non-linux-client; `internal`≠tenant-containment top-line) · *API-contract breaking* (Protocol non-`tcp`→400; `none`+ports→400; request-the-
global-mode→400; SDK errors when network mode set vs feature-less server) · *API response-
shape* (`GET /sandboxes/{id}` **and** `GET /sandboxes` `ports` **present iff non-empty**,
was-absent-for-all→present-when-non-empty; Go SDK `Sandbox` gains `Ports`; `/version`+
`/health` gain `features` — additive). Expected: **`0.0.6 → 0.1.0`**. SDK npm/PyPI publish
**after** the `v0.1.0` tag.

## Open questions

1. `HostPort=0` ephemeral publishing rejected in v1.
2. `EnsureNetwork`/`Reconcile` interface vs concrete: **keep concrete**; `Reconcile` pinned
   `Reconcile(ctx context.Context, st store.Store) error` (store as method param; stub embeds
   no-op `store.Store`).
3. **Egress-filter follow-up PR** (descoped v3 `DEN-EGRESS`): `INPUT`+`DOCKER-USER` dual hook,
   atomic `iptables-restore`, re-assert watcher, typed `EnableIPv6:false`+`scope global`
   post-start assertion, IP blocklist, embedded-DNS model, rootless/macOS handling, **also
   applied to `internal`**, host-iptables CI topology. Issue-tracked; ~4–5d.
4. Automate SDK versioning via release-please `extra-files`/added packages. Deferred.
5. **Co-residency residual — v9: REFUSED-by-default, mandatory operator attestation (CLOSED
   for default config; further automation deferred):** the irreducible undetectable hole — a
   local `unix://` socket itself proxied to a remote/VM daemon (socat/SSH-forward/docker-
   context — realistic, not-rare on Linux CI/bastion) — is undetectable from `docker info`+
   `DaemonHost()`+`clientGOOS`. **v9 operator decision (resolving Codex iter6 C1★/iter7/iter8
   CRITICAL★ + Security iter5/iter6/iter7/iter8 🟡): Den REFUSES the loopback branch unless
   the operator explicitly attests via `platform_override="linux-native-docker-co-resident"`
   (or removes the precondition via `auth.enabled=true`/`none`).** The v8 permit-with-WARN
   presumption is removed (Codex/Security iter8: a WARN on the recommended default is security
   theater). The residual is now an explicit, ERROR-logged operator act, not a silent
   presumption. A future hardening could *machine-verify* daemon locality (e.g. a daemon-side
   liveness sentinel comparing namespaces) to drop even the attestation requirement on
   genuinely co-resident hosts. Deferred.

## Order of execution & risk table

| Phase | Files | Effort | Risk | Order |
|---|---|---|---|---|
| 0 Fix harness (+dind TLS, single-topology API-level CI gate, min-count, both `if:`, meta-check) | Makefile, ci.yml | 0.5d | Low | 1 |
| 1 netpolicy + modes + validator + allowlist(clientGOOS) bind guard + **v9 REFUSE-unless-platformOverrideAttested** + bridge refusal + platform_override(bind-scoped attestation) + feature token + ports one-liner + none ordered-cleanup (v9 predicate) | runtime.go, config.go, docker.go, netpolicy, engine (5 sites), sandbox.go, mcp tools, common.go, cmd/*, example | 5.5d | Med | 2 |
| 2 Safe reconciliation (label-only-destructive, store-fail-closed, pinned concrete `Reconcile(ctx,store.Store)`, **post-Phase-1-labels only**) | docker.go, engine | 1.25d | Med | 3 |
| 3 Delete PortForwarder + DooD scope | network.go, port.go(+tests), architecture.md, SECURITY.md | 0.25d | Low | 4 |
| 4 Docs/SECURITY/SDK (+matrix w/ presumed-residual row, Go-SDK Ports, TS/Py version() features no-model, feature-token, ports-iff-non-empty contract, 0.1.0 edit) | README*, SECURITY.md, docs/*, pkg/client, sdk/*, package.json, pyproject.toml | 1.0d | Low | 5 |
| 5 Tests + e2e (12-row API-level CI gate, classifyPlatform/BindGuard(+attested)/guard-ordering/MCP-AST truth tables, per-SDK ×3+Py-kwarg, TS-infra-from-zero, local-only REFUSE/start-with-attestation leg, v9 none predicate) | tests/integration/network_test.go, unit tests, scripts/e2e-network.sh | 2.75d | Med | 6 |

Total ≈ **11.25 days nominal; realistic range 11–15d** (v9 net-simpler than v8: removed the
presumption-disclosure fn/WARN wiring and the dind-discovery harness; the spread is
test-harness + CI debugging + 3-language SDK gating, not design complexity). Phase 0 gates 5;
1→2 sequential (Phase 1 labels before Phase 2); 3,4 independent; 5 validates 1–2.
**Follow-up egress-filter PR** (Open Q3): separately ~4–5d, not in scope.

## Iteration log
- **Iteration 0** (2026-05-17): initial plan from 5-agent parallel research.
- **Iteration 1** (2026-05-17): 9 criticals: default→`internal`; single enum; `none` wiring;
  Phase 0; `EnsureNetwork` fatal; v1 DOCKER-USER blocklist; SDK fan-out; PortForwarder/DooD
  scope; committed SECURITY wording.
- **Iteration 2** (2026-05-17): 13 criticals incl. the sandbox→Den-API escape, `none`
  inversion, `Protocol` breaking change, DOCKER-USER under-spec, CI dind `DOCKER_HOST`,
  validation bypass, reconcile migration hole, IPv6.
- **Iteration 3** (2026-05-17): INPUT-chain confirmed; host-firewall descoped; OS-independent
  fatal bind guard + bridge-refusal; typed `EnableIPv6`; spoof-resistant reconcile; Phase-1
  empty-ports; dind TLS env; meta-check; `buildContainerCreateSpec`; `none` substitution
  invariant; `allow_unsafe_*` polarity; `0.0.6→0.1.0`.
- **Iteration 4** (2026-05-17): two ★consensus criticals — bind guard NOT OS-independent,
  semver floor unimplementable. CD2 platform-aware/fail-closed; SDK semver→`features` token;
  validation→single chokepoint w/ typed `ValidationError`; `none` NetworkMode-primary;
  +~22 warnings; 6.25→9.25d.
- **Iteration 5** (2026-05-17): Pragmatist [CONSENSUS]. 7🔴/13🟡. v6: blacklist→positive-
  allowlist `classifyPlatform`; co-residency named (★consensus w/ Codex); false moby none-400
  removed; row-1 `docker exec` into dind; Reconcile concrete-only + store stub;
  `SilenceUsage`+negative; Go-SDK `Ports`; +13 warnings; 9.25→10.0d.
- **Iteration 6** (2026-05-17): [CONSENSUS] Pragmatist, Docker-correctness, API/SDK. NOT:
  Security, Testing, Codex (4 CRIT). v7: Codex C1★ `clientGOOS=="linux"` clause; Codex C2
  effective `DaemonHost()`+inert-test; Codex C3★ per-row Den-cfg; Codex C4 platform_override
  contract; Testing C2 `Reconcile(ctx,store.Store)`; Security 🟡#1 production `none` post-
  create assertion; +~20 folded warnings; 10.0→11.0d / 11–14d.
- **Iteration 7** (2026-05-17): 5 fresh agents + Codex on v7. **[CONSENSUS]: Docker-
  correctness (full moby-source re-verification — every v7 mechanical claim confirmed at
  v28.3.2/go-connections v0.6.0), Pragmatist (5 sites exact, no cycle, accessors clean, effort
  defensible).** NOT: Security (3🟡), Testing (1🔴/6🟡), API/SDK (2🔴/1🟡), Codex (1 CRIT/6
  WARN). Addressed in v8, none silently dropped: **(Codex CRIT ★ + Security 🟡#1 ★ — the
  dominant)** v7's absolute "FAIL-CLOSED/proven-safe" branding conflicted with the
  undetectable local-`unix://`-proxy residual (socat/SSH-forward/docker-context — realistic,
  not-rare on Linux); v7 would say safe & log nothing. v8 reframes CD2 to "fail-closed against
  every machine-DETECTABLE unsafe topology; PRESUMED-co-resident with a **MANDATORY per-start
  WARN disclosure** on the loopback-only-safe branch" + `RequiresCoResidencyPresumptionDisclo
  sure` decision fn + SECURITY.md(10) enumerating the residual as a realistic class; rejected
  (i) keep-proven-wording (ii) refuse-all-native-loopback, recorded. **(Testing 🔴 + Codex W)**
  GitHub Actions `services:` dind not addressable as `docker` on the runner socket → Phase-0
  dind-discovery step + step-launched fallback + documented local-only marquee fallback with
  the CI-portable sentinel. **(API/SDK C1 ★)** absent-ports diagnosis corrected — response
  structs ALREADY correct, sole fix is the one `engine.CreateSandbox` assignment, structs
  MUST NOT be touched + a response-structs-unchanged guard test. **(API/SDK C2 ★)** post-fix
  contract reworded to "`ports` present iff non-empty" (option (a) keep-omitempty; (b)
  rejected). **(API/SDK W1)** json-tag invariant scoped to the 3 encode structs; MCP
  decode-only intentionally exempt. **(Security 🟡#2 + Codex W)** `none` create→assert→start
  ordering pinned as a tested TOCTOU-closed invariant (row-12) + force-remove + wrap-cleanup-
  err + ERROR-log offending NetworkSettings. **(Security 🟡#3)** SECURITY.md(6) upgraded from
  a bare label to an explicit `internal`-embedded-DNS/gateway-uncontained operator sentence.
  **(Codex W)** docker_host-inert test gains the positive `DOCKER_HOST` companion leg;
  platform_override scoped strictly to the bind-guard decision (+ not-read-elsewhere test);
  Phase-1-labels-before-Phase-2 ordering pinned. **(Testing 🟡×4)** store.Store = 10-method
  embed-stub (not "narrow surface"); guard-ordering injected-`func` seam pinned; MCP
  structural test = `go/parser` single-file AST scan (claim downscoped); TS test infra built
  from zero w/ bun `mock`/`spyOn`; `TestMain` count mechanism replaced with a workable
  `Mark(t)` package counter (`testing.M.Run()` returns no count). **(Docker-correctness 🔵)**
  nil-guard `inspect.NetworkSettings`; `DaemonHost()` empty-branch noted defensive-only.
  **(API/SDK 🔵)** Python untyped-dict no-model decision; Python kwarg/config token sub-cases;
  release-please harmless-transient note. Effort rebaselined **11.0→12.0d nominal / 12–16d**.
  Rejected: none silently.
- **Iteration 8 — FINAL** (2026-05-18): 5 fresh agents + Codex on v8. **[CONSENSUS]: API/SDK
  (C1/C2/W1 all source-verified resolved — absent-ports = the single `engine.CreateSandbox`
  literal, structs already correct; cross-SDK decisions coherent), Pragmatist (all 6
  load-bearing claims re-verified exact; presumption machinery judged proportionate not
  gold-plating).** NOT: Security (1🟡 — habituated-WARN-on-default), Docker-correctness (**1🔴
  NEW** — `none` `Networks` is never empty, always `{"none":{}}`; v8 predicate inverted),
  Testing (2🟡 — dind topology unspecified / rows 2/3/5 no CI-portable fallback), Codex
  (**1 CRITICAL** — WARN is disclosure not control; require attestation/auth/none — recurring
  4th iteration + 2 WARN: reconcile AND-predicate misses internal→bridge; dind topology not
  chosen). **plan-loop hit cap=8 with criticals; per skill rule the operator was asked
  (extend vs stop) + the dominant security product decision.** **Operator decided
  (2026-05-18): STOP loop + implement; security posture = REFUSE (mandatory `platform_override`
  co-residency attestation on the native-Linux loopback auth-off branch — NOT permit-with-
  WARN).** v9 folds in, none dropped: **(Codex CRIT★ + Security🟡★ — resolved by operator's
  REFUSE choice, the exact option both reviewers named)** v8 permit-with-WARN + `RequiresCo
  ResidencyPresumptionDisclosure` REMOVED; `BindGuardDecision` gains `platformOverride
  Attested`; loopback branch REFUSES unless attested; CD2/trade-offs/scope/Open-Q5/release-
  buckets reframed; net-simpler. **(Docker-correctness 🔴)** `none` breach predicate corrected
  to "any key ≠ `none` OR populated EndpointID/NetworkID/IPAddress" in Context/§3/Phase-1.5/
  rows 5+12. **(Testing 🟡 + Codex WARN)** single CI topology fixed (`services: docker:dind` +
  `DOCKER_HOST=tcp://docker:2375`, API-level-only mandatory gate: Inspect-Ports sentinel +
  one-shot exit-code probe, no container-name resolution); discovery step dropped; in-container
  legs explicitly LOCAL-ONLY (accepted residual). **(Codex WARN)** reconcile mismatch predicate
  → ANY-deviation/OR (covers internal→bridge). Effort rebaselined **12.0→11.25d / 11–15d**
  (v9 net-simpler). Accepted residuals documented (rows 2/3/5 CI-coverage narrowed; Open Q5
  attestation not yet machine-verified). Rejected: none silently. **plan-loop CLOSED.**
