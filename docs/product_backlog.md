# Otter Product Backlog

Opinionated and pragmatic. Items are scoped to what the current codebase
can actually absorb, not to what `why.md` and `internal/loadbalancer/idea.md`
describe as long-term ambition.

> **2026-05-01 reconciliation.** Items N2 (HTTP 413 mapping), N3 (port
> arithmetic), N5 (per-IP rate limiting), N7 (WebSocket message cap) and
> the new N8 (CORS allow-list) below are landed. The remaining `Now`
> items are N1 (slog), N4 (richer endpoint shape) and N6 (TODO sweep).

## Reading this document

Each item has:

- **Why** — the user-visible or operator-visible problem it solves.
- **Deps** — what must land first.
- **Effort** — S (< 1 day), M (1-3 days), L (1-2 weeks), XL (> 2 weeks).
- **Risk** — low / medium / high. "High" means either design uncertainty
  or a meaningful chance of regressing a working path.
- **Validate with** — the cheapest way to prove it works:
  `unit`, `docker-e2e`, `load`, or `docs`.

Category tags:

- **[GATE]** gateway and proxy reliability
- **[BAL]** balancing and cluster introspection
- **[QL]** query language and transpiler work
- **[FW]** framework / ontology / graph-model ideas

Horizons:

- **Now** — 0-2 weeks, should land in the current `feat/hardening-pass`
  stream.
- **Next** — the phase after. Concrete enough to spec, not scheduled yet.
- **Later** — kept in the backlog because it shapes the design, but does
  not belong on a near-term schedule.

---

## Now

Small, high-leverage items. Every one of them improves operability
*without* introducing new design surface. The goal of the Now horizon
is to make the gateway boring.

### N1. `[GATE]` Structured logging with `log/slog` and request IDs

- **Why.** Today every line is `log.Printf`. Correlating a failed
  upstream mutation with the originating HTTP request or WS connection
  is a grep exercise. The Phase 3 hardening added SIGINT/SIGTERM
  shutdown but retained ad-hoc prints. A single `slog.Logger` with a
  request-scoped middleware unlocks triage.
- **Deps.** None.
- **Effort.** M.
- **Risk.** low. The change is mechanical; the only subtle piece is
  keeping the WS handler's per-message logs readable.
- **Validate with.** `unit` for the middleware, visual inspection of
  the Docker-backed logs for field shape, `docs` note in `runbook.md`.

### N2. `[GATE]` Translate oversize-body rejections to HTTP 413

STATUS: Done. `WriteRequestBodyReadError` in `internal/helpers/helpers.go`
maps `*http.MaxBytesError` to 413 and surfaces the configured limit; both
HTTP handlers and the validate endpoints now go through it. Covered by
`TestWriteRequestBodyReadError_MaxBytesReaderMapsTo413`.

### N3. `[GATE]` Fix `port - 1000` assumption in `selectBackendHost`

STATUS: Done (minimal form). `internal/proxy/proxy.go` now consults
`Config.DgraphHTTPEndpoints` (yaml `dgraph_http_endpoints` /
`DGRAPH_HTTP_ENDPOINTS` env) before falling back to `grpcPort - 1000`.
The fallback path logs a one-shot warning per unmapped endpoint so
operators can spot silent misroutes. Covered by
`TestResolveHTTPEndpoint_*` in `internal/proxy/proxy_test.go`.

The richer config shape proposed in N4 (per-endpoint pair structure) is
not yet shipped — operators today set the explicit map alongside the
existing `dgraph_endpoints` list.

### N4. `[GATE]` Endpoint config shape: address-pair instead of offset math

- **Why.** Today an endpoint is a string like `"localhost:9081"` and
  the HTTP side is implied. The minimal fix in N3 added a sibling map
  (`dgraph_http_endpoints`) that pairs each gRPC endpoint with its HTTP
  twin, but the underlying list is still `[]string`. The richer shape
  ```yaml
  dgraph_endpoints:
    - grpc: localhost:9081
      http: localhost:8081
  ```
  would let the balancer carry both addresses on `EndpointInfo` and
  remove the lookup hop in the proxy. Backwards compatibility: accept
  the old string form and the parallel `dgraph_http_endpoints` map,
  logging a deprecation when the map is used.
- **Deps.** Builds on N3 (already done in minimal form). Unlocks Next
  #B1 cleanly.
- **Effort.** M.
- **Risk.** medium. Touches config parsing, balancer, and proxy.
- **Validate with.** `unit` + `docker-e2e`.

### N5. `[GATE]` Basic per-IP rate limiting on HTTP and WS upgrade

STATUS: Done. `internal/ratelimit` is a dependency-free token bucket
keyed by client IP (X-Forwarded-For, falling back to `RemoteAddr`).
Wired into `/query`, `/mutate`, `/graphql` (via `routing.SetupRoutes`)
and the `/ws` upgrade in `cmd/proxy/main.go`. Configured via
`rate_limit_rps` / `rate_limit_burst` (yaml) or `RATE_LIMIT_RPS` /
`RATE_LIMIT_BURST` (env). Both zero disables the limiter so the default
behaviour is unchanged for `make e2e`. Per-message limiting on an
already-upgraded WebSocket connection is tracked separately as X6.

### N6. `[GATE]` Sweep and delete stale TODOs / dead branches

- **Why.** `handler.go` has a `// ! TODO: Add tests` on `HandleGraphQL`
  (now partially covered), `balancer.go` has a `round-robin-healthy`
  case returning "not implemented yet", and several `fmt.Println`
  debugging lines remain in `pBalancer.go`. None of them are bugs, but
  they trip contributors.
- **Deps.** None.
- **Effort.** S.
- **Risk.** low.
- **Validate with.** `unit`; grep for `TODO`, `FIXME`, `fmt.Print` and
  decide case by case.

### N7. `[GATE]` WebSocket per-connection body cap

STATUS: Done. `applyReadLimit` in `internal/websocket/websocket.go`
calls `conn.SetReadLimit` with `ws_max_message_bytes` (default 1 MiB).
Configurable via the `WS_MAX_MESSAGE_BYTES` env override.

### N8. `[GATE]` CORS allow-list with credentials hardening

STATUS: Done. `applyCORS` in `internal/proxy/utils.go` honours
`cors_allowed_origins` (yaml) or `CORS_ALLOWED_ORIGINS` (env). Only
matched origins are reflected back, and `Access-Control-Allow-Credentials`
is set only for explicit matches; the legacy permissive behaviour is
gated behind `dev_mode: true` so a non-dev deployment fails closed by
default. The wildcard `"*"` pattern reflects `*` without credentials,
matching the browser specification.

---

## Next

Items that require meaningful design or depend on the observability
baseline from Now. These are the right targets for Phase 4.

### X1. `[GATE]` Prometheus `/metrics` endpoint on its own port

- **Why.** Phase 3 added timeouts and graceful shutdown; the missing
  piece for "production-operable gateway" is a metrics surface. Counter
  per route+status, histogram per route, balancer hit counter per
  endpoint, WS active-connections gauge. Host on a dedicated port so
  the HTTP proxy surface stays free of introspection endpoints.
- **Deps.** N1 (the slog work makes field naming consistent).
- **Effort.** L.
- **Risk.** medium. The work is straightforward but naming the
  metrics well matters because changing them later is a breaking
  change.
- **Validate with.** `unit` for the middleware, `docker-e2e` scraping
  `/metrics` during a `make e2e` run.

### X2. `[BAL]` `round-robin-healthy` balancer

- **Why.** The code stub in `balancer.go` reads:
  ```go
  case "round-robin-healthy":
      return nil, fmt.Errorf("round-robin-healthy balancer is not implemented yet")
  ```
  Operators want it. Implementation: a background health poller hits
  each alpha's `/health` at a configurable interval, marks endpoints
  as healthy/unhealthy, and the balancer skips unhealthy ones. If all
  endpoints are unhealthy, fall through to the full list (fail-open,
  log loudly) so the gateway does not black-hole when the poller is
  stale.
- **Deps.** N4 (HTTP port on the endpoint shape) is a strong prereq;
  otherwise the poller inherits the `-1000` assumption.
- **Effort.** L.
- **Risk.** medium. Health-check flapping can cause oscillation;
  implement a simple "N successes to recover" to dampen.
- **Validate with.** `unit` with a fake poller, `docker-e2e` by
  killing one alpha during `make e2e-up` and watching traffic shift.

### X3. `[BAL]` Cluster state inspector (cached `/state`)

- **Why.** `internal/loadbalancer/idea.md` already documents the
  shape of Dgraph's `/state` and `/health` responses. Exposing a
  cached read of them via an Otter route (`/otter/state`,
  `/otter/health`) gives the balancer and the operator a single
  source of truth. Do *not* feed this into the balancer yet
  (that is X5) — this is the substrate.
- **Deps.** None, but improves once N1 lands.
- **Effort.** M.
- **Risk.** low.
- **Validate with.** `unit` for the cache, `docker-e2e` for the
  end-to-end read.

### X4. `[GATE]` Docker `healthcheck` in `docker-compose.yml`

- **Why.** Today `scripts/wait-for-otter.sh` is a host-side stopgap.
  A proper `healthcheck` on the `otter` service (plus `depends_on:
  condition: service_healthy` on test runners) lets CI and local
  users rely on compose primitives instead. The script can stay as a
  convenience but should not be the mechanism.
- **Deps.** None.
- **Effort.** S.
- **Risk.** low.
- **Validate with.** `docker-e2e` (the existing `make e2e` flow).

### X5. `[BAL]` Feed `/state` into balancer decisions

- **Why.** Once X3 exists, the balancer can prefer non-leader nodes
  (for reads), route mutations to leaders, and skip tablets marked
  `remove: true`. This is what `idea.md` calls
  "state-based balancing" and it is the single biggest differentiator
  vs. a generic HTTP load balancer.
- **Deps.** X3. Strongly benefits from X2.
- **Effort.** L.
- **Risk.** high. Dgraph `/state` is semi-private and its schema has
  shifted between majors; the Dgraph v24 vs v25 discrepancy already
  noted in `README.md` is an example.
- **Validate with.** `unit` against fixture `/state` payloads,
  `docker-e2e` with a 3-alpha cluster to verify leader-avoidance,
  `docs` note in `security.md` about the new dependency on a private
  surface.

### X6. `[GATE]` WebSocket per-connection rate limiting + idle disconnect

- **Why.** Complements N5 and N7. A connection can stay open for
  120s of idle time (the configured `IdleTimeout`), but within a
  single connection nothing bounds message rate. Add a per-connection
  token bucket and a stricter idle-without-traffic close.
- **Deps.** N5 is the pattern to reuse.
- **Effort.** M.
- **Risk.** low.
- **Validate with.** `unit`, `docker-e2e`.

### X7. `[GATE]` Read/write split balancer (`round-robin-on-RW`)

- **Why.** A step-up from `purposeful`: the operator declares a
  read-only pool and a writable pool. Unlike `purposeful`, the
  pools are discovered dynamically from `/state` (leader vs
  non-leader). In practice this is X5 phrased as a first-class
  balancer mode.
- **Deps.** X5.
- **Effort.** L.
- **Risk.** medium. Mostly depends on getting X5 right.
- **Validate with.** `docker-e2e` with a 3-alpha cluster.

---

## Later

These items are kept on purpose because they anchor the long-term
design. None of them should be worked on until the Now + Next list
is at least 80 % done. They are written here so contributors know
where the project is heading, not so they can start tomorrow.

### `[QL]` Cypher → DQL transpiler

- **Where it stands today.** `internal/astneo/` parses a subset of
  Cypher (`MATCH`, `CREATE`, `WHERE`, `RETURN`) into a participle AST.
  `ParseQueryParts` even returns an **error** when it sees `CREATE`
  ("CREATE clause parsed: %+v") &mdash; this is stub code that signals
  "I parsed the thing" through a non-nil error. There is no emitter.
- **Why it is Later.** A real transpiler is weeks of work: the parser
  covers maybe 20 % of the Cypher grammar, node/relationship semantics
  do not map 1:1 onto DQL's predicate model, and the correctness bar
  for a query-language translator is brutal. Shipping a half-working
  transpiler hurts the project's reputation more than not shipping
  one.
- **Recommended staging.** First build a tiny translation harness
  behind a feature flag (`enable_cypher: false`) that covers one
  canonical pattern (`MATCH (n:Person) RETURN n.name`) end-to-end
  with golden tests. Do not advertise Cypher as a feature until the
  golden-test suite has at least 50 patterns.
- **Effort.** XL.
- **Risk.** high.

### `[BAL]` Leader-aware routing (`round-robin-avoid-leaders`, `round-robin-leaders-only`)

- **Where it stands today.** Conceptually covered by X5 / X7 in Next.
  Promoting this from Later to Next should happen only after X5 proves
  reliable.
- **Effort.** M on top of X5.
- **Risk.** medium.

### `[BAL]` Goroutine / CPU / jemalloc introspection

- **Where it stands today.** `idea.md` points at Dgraph's
  `/debug/jemalloc`. Attractive in theory, noisy in practice. Most
  balancers do fine with `/health` + `/state`.
- **Why it is Later.** High integration cost, marginal gain over
  `/state` + external Prometheus scraping.

### `[FW]` Type sharding, region-based indexing, UID reservation

- **Where it stands today.** Pure vision in `why.md`. None of
  `internal/` has scaffolding for predicate prefixing, UID reservation,
  or region decorators.
- **Why it is Later.** These are framework-level decisions that
  presume Otter is already the *mandatory* front door to Dgraph for
  every client. Until the gateway is demonstrably production-grade,
  adding ontology features is inverting the dependency tree.
- **Trigger to revisit.** When Otter has stable observability,
  leader-aware routing, and the Cypher transpiler handles at least
  one end-to-end flow. At that point "opinionated graph model"
  becomes a natural next layer instead of a premature abstraction.

### `[FW]` Ontological schema system (`schema for ... { type User is Person ... }`)

- Same reasoning as above. `internal/astgraphql/ast.go` already
  converts Dgraph GraphQL schemas into a `GraphNode` tree, which is
  a plausible *input* to an ontology layer, but the output side is
  unbuilt.
- **Trigger to revisit.** Same as type sharding.

### `[FW]` Query decorators (`Query @(graph: "g11", region: "Europe") { ... }`)

- Late-stage vision feature. Depends on the ontology layer existing.

### `[QL]` Additional transpilers (GraphQL-to-DQL beyond pass-through, SPARQL, etc.)

- Only meaningful once the Cypher work has proven the harness shape.

---

## Cross-cutting: testing discipline for each horizon

| Horizon | New unit tests required | `docker-e2e` required | Docs update required |
|---------|--------------------------|------------------------|-----------------------|
| Now     | yes, for every item      | for N3, N5, N7         | `runbook.md` deltas   |
| Next    | yes                      | for every item         | `security.md` for X5, new `metrics.md` for X1 |
| Later   | not yet                  | not yet                | keep docs untouched   |

The `e2e` build tag contract is non-negotiable: any test that needs a
live Otter must carry `//go:build e2e` so `go test ./...` stays
hermetic. That is currently the cheapest signal the project has and
must not regress.
