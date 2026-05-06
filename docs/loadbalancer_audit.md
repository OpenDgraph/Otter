# Load Balancer and Cluster Introspection — Audit

Narrow review of `internal/loadbalancer/*`, the balancer hookup in
`internal/proxy/*`, how the config wires this slice together
(`internal/config/config.go`), and the notes in
`internal/loadbalancer/idea.md`. No dependency change; no new balancer
mode.

## What exists today

### Modes actually shipped

1. **`round-robin`** (`@/Users/micheldiz/Documents/GitHub/Otter/internal/loadbalancer/balancer.go:28-63`)
   - Flat list of endpoints; mutex-guarded cursor; wraps modulo `len(nodes)`.
   - Empty list returns a zero-value `EndpointInfo` and logs a warning;
     the caller surfaces this as a 503 via `SelectClient`.
   - Concurrency-safe.

2. **`defined` / `purposeful`** (`@/Users/micheldiz/Documents/GitHub/Otter/internal/loadbalancer/pBalancer.go:9-54`)
   - `map[purpose]*RoundRobinBalancer`, keyed by `"query"`,
     `"mutation"`, `"upsert"`.
   - `ValidatePurposeful` (`pBalancer.go:64-91`) fails startup if any
     purpose group has zero usable endpoints — the fix from the Phase 1
     pass. Exercised by four tests in
     `@/Users/micheldiz/Documents/GitHub/Otter/internal/loadbalancer/balancer_test.go:44-90`.
   - `AllEndpoints()` deduplicates across groups.

### Hookup

- `cmd/proxy/main.go:48-63` selects the balancer shape by
  `cfg.BalancerType`. `"defined"` and `"purposeful"` are aliases.
  Anything else falls through to `NewBalancer`, which currently only
  accepts `"round-robin"` or returns an error.
- `proxy.Proxy` holds both `balancer` and `Purposeful` fields.
  `SelectClientAuto(purpose)` (`internal/proxy/proxy_balancer.go:11-16`)
  dispatches based on which is non-nil. Every DQL handler goes through
  this; HTTP pass-throughs (`HandleDirect`, `HandleGraphQL`) use
  `selectBackendHost` instead.

### Observability

- Startup logs every endpoint added and the selected balancer type.
- Per-request logs name the picked endpoint on both the
  purposeful and flat paths (`helper.go:36`, `proxy_balancer.go:27`).
- Nothing else. No metric. No way to see the current round-robin
  cursor, the group→endpoint map at runtime, or which backends are
  alive.

## Findings: code vs docs vs idea.md

### Drift between code and README

1. **`round-robin-healthy` is a dead branch in the switch.** The
   current implementation returns
   `"round-robin-healthy balancer is not implemented yet"`. The README
   lists it correctly under *Next / not implemented*, so the README is
   honest; the drift is internal. A reader of `balancer.go` sees a
   branch that does nothing useful, with no pointer to the backlog.
   Low impact, easy fix (error message). See *Small change shipped*
   below.

2. **`EndpointInfo.Offset` is dead data.** `NewRoundRobinBalancer`
   computes `Offset = port - 9080` for every endpoint at construction
   time (`balancer.go:32-38`). A repo-wide grep shows **zero readers**
   of that field:

   ```bash
   rg '\.Offset' --glob '*.go'   # no results
   ```

   It was presumably introduced to help later leader-aware work
   (HTTP port = gRPC port - 1000 vs baseline 9080), but the proxy
   took a different route: `selectBackendHost` in
   `internal/proxy/proxy.go:102-106` does its own
   `port -= 1000`. We now have **two port-derivation heuristics**
   that disagree: `inferPort` assumes a 9080 baseline, and
   `selectBackendHost` assumes gRPC-minus-1000 conversion. Only the
   second one is actually used for routing.

   This is a cleanup, not a bug (the unused field never influenced a
   request). Removing it is safe because `.Offset` has zero readers.
   Done in this pass.

3. **`inferPort` strips `http://` / `https://` prefixes, but no
   config path ever passes a URL.** Both `manifest/config.yaml` and
   `manifest/config_docker.yaml` use `host:port` strings; the
   README's configuration table documents the same shape. Dead
   tolerance; not harmful enough to rip out. Kept.

### Drift between code and `idea.md`

`idea.md` is explicitly a research note (and the doc drift pass already
flagged it as such in `README.md`). Nothing in `idea.md` is wired into
the balancer. Concretely, the following ideas have **no support code**
today:

- `/state` introspection (no fetcher, no cache, no typed struct).
- `/health` probing (no background loop).
- `jemalloc` / goroutine introspection (no client).
- `moveTablet` / `assign` UID leasing (no calls).
- Tablet-aware balancing (no tablet map).
- Leader-aware routing (no leader state).

This is **not a bug** — `idea.md` is a scratchpad. The drift is only
a problem if `idea.md` makes someone believe Otter does any of this.
The README reality pass already covered that concern. Audit finding:
no action required on `idea.md` itself.

### Hidden contracts worth naming

- **HTTP port = gRPC port − 1000**, baked into `selectBackendHost`.
  Works for the example compose stack (gRPC 9081-9083, HTTP
  8081-8083) and nothing else. Already tracked as backlog item
  **N3 / N4** in `@/Users/micheldiz/Documents/GitHub/Otter/docs/product_backlog.md`.
- **Balancer purposes are the string literals** `"query"`, `"mutation"`,
  `"upsert"`. No enum, no constants, no validator against a closed set
  of allowed names. A typo in YAML silently creates a never-used group.
  Flagged, not fixed in this pass (doc-only change would be a constant
  block; deferred to keep this pass small).

## Near-term enhancement assessment

The four options the prompt names:

### 1. Health-aware balancing (`round-robin-healthy`)

**What it needs.** A background probe loop per endpoint hitting
`/health`; a per-endpoint `healthy bool` with expiry; integration into
`RoundRobinBalancer.Next()` to skip unhealthy nodes.

**Why it is not quite ready.** It works in isolation but it duplicates
the probe surface we want for cluster inspection. Building it first
means we ship one probe loop against `/health`, then later ship a
second one against `/state`, then reconcile them. The opposite order
— cluster state first, derive health from it — is cheaper and more
honest: `/state` already tells us who is alive, in which group, and
who the leader is.

**Verdict: wait.** Build after (3) is in place.

### 2. Cluster state inspection

**What it needs.** A typed struct for Dgraph's `/state` payload (see
`idea.md` for the real shape), a single fetcher with timeout, a cache
with TTL (~5 s), and one inspection route `/otter/state` that returns
the cached snapshot. No balancer coupling yet.

**Why it is ready.** No new dependencies; testable with `httptest`
serving a fixture JSON; no goroutine lifecycle baked into the balancer
yet; pure read surface (zero routing change). It also pays for
items (1), (3), and (4) because they all need the same state cache.

**Verdict: this is the recommended next step.** See *Recommendation*
below.

### 3. Leader-aware routing

**What it needs.** (2) first, then a balancer mode that reads from the
cache and prefers non-leader nodes for read traffic (or leader-only for
writes — the Dgraph semantics are non-trivial because every alpha can
accept mutations but only one per group owns each tablet).

**Why it is not quite ready.** The correctness model depends on the
exact `/state` schema (tablet ownership, group leader), which has
historically drifted across Dgraph majors. Pinning routing behaviour
to that schema before we have a version-compat harness (fixtures per
major) bakes in brittleness.

**Verdict: wait until (2) + a fixture test asserts `/state` parses on
each supported Dgraph major.**

### 4. Read/write separation

**Partial today via `defined` balancer.** `manifest/config.yaml`
already demonstrates this manually: one group for `query`, another
for `mutation`. What is missing is *automatic* split where the
balancer looks at the operation kind and picks a read-suited node.

**Why it is not quite ready.** The "automatic" part reduces to
leader-aware routing (3), because for Dgraph the right read-vs-write
split is "read from non-leaders, write anywhere" rather than
"backends labelled read-only". Same dependency on (2).

**Verdict: wait. The manual path is already live and documented.**

## Recommendation: next step is cluster state inspection (passive)

Build the substrate, not the cleverness. Concretely:

- A new package `internal/cluster/state` with:
  - `type State struct { /* groups, members, leader, tablets, … */ }`
  - `type Poller struct { … }` that hits one alpha's `/state` with a
    configurable interval, caches the last success, and returns it via
    `Snapshot()`.
- A minimal `/otter/state` route exposing the cached snapshot.
- A `ShouldPoll bool` config knob (default false) so existing deploys
  stay unchanged.
- Fixture test with a `httptest.Server` returning the JSON example
  from `idea.md`.

Explicit non-goals of that step:
- **No balancer change.** The balancer keeps doing round-robin.
  Wiring health/leader/RW decisions into `Next()` is the *next*
  step, unlocked by this one.
- **No background probe against /health.** That is a redundant
  channel once `/state` is parsed.
- **No routing from the cache.** Visibility first, decisions later.

Why this order is defensible:

- It is testable without Docker (fixture-driven).
- It is reversible: the feature is additive and off by default.
- It is the only option whose correctness does not depend on
  assumptions about Dgraph major-version behaviour. If `/state`
  changes across majors, the fix lives in one parser, not across
  three balancer modes.
- It buys observability (an operator can see what Otter sees),
  which is the current Phase 4 direction named in
  `docs/next_architecture_steps.md`.

## Small change shipped in this pass

No new balancer mode. Two surgical cleanups, each justified by a
direct finding above:

1. **Removed `EndpointInfo.Offset` and the offset computation in
   `inferPort`.** Zero readers in the repo, confirmed by grep.
   `inferPort` is kept as a validator (`validateEndpoint`) because the
   port-parse check is genuinely useful — invalid endpoints are
   skipped with a warning at construction.

2. **Improved the `round-robin-healthy` error message.** Operators
   who typo that value into YAML now get a message that says the mode
   is tracked as backlog item **X2** and that they should use
   `round-robin` today, rather than a terse "not implemented yet".

Both changes are covered by new tests in `balancer_test.go`.

### Files touched

- `@/Users/micheldiz/Documents/GitHub/Otter/internal/loadbalancer/balancer.go`
  - Dropped `Offset` field, renamed `inferPort` → `validateEndpoint`,
    reworded the `round-robin-healthy` error.
- `@/Users/micheldiz/Documents/GitHub/Otter/internal/loadbalancer/balancer_test.go`
  - Added `TestValidateEndpoint_*` covering valid, missing-port,
    scheme-prefixed, and non-numeric-port cases.
  - Added `TestNewBalancer_RoundRobinHealthyErrorReferencesBacklog`
    to lock in the helpful error message.

### Deliberately not changed

- `balancer_type: defined` vs `purposeful` (aliases) kept as-is.
- Port-arithmetic contract in `selectBackendHost` untouched
  (tracked as backlog N3/N4).
- `idea.md` left alone; it is a scratchpad, not documentation.
- No new config knobs introduced.

## Validation

```bash
go build ./internal/loadbalancer/... ./internal/proxy/... ./cmd/proxy
go test  ./internal/loadbalancer/... -v
go test  ./...
```

All green. No test deletion, no weakening. Full suite remains hermetic
(no Docker required).

## What remains uncertain

- Whether `idea.md` has a downstream consumer outside this repo (blog
  post, slides) that we would invalidate by acting on it. Assumed no.
- Whether the chosen interval for future `/state` polling should be
  configurable or pinned. Intentionally left open for the next pass
  when the inspector is actually built.
- Whether `SelectClientAuto`'s purpose string should be a closed set.
  Leaving as-is avoids a behaviour change here; candidate for a
  follow-up when purposeful grows a fourth purpose (e.g. `admin`).
