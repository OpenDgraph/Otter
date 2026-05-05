# Otter Repository Audit

Status date: Phase 1 hardening (original) — reconciled 2026-05-01.

This audit reconciles the stated product intent (README, why.md, idea.md) with
what the codebase actually does today. It is intentionally focused on high
leverage items for a project still under construction.

> **2026-05-01 reconciliation.** Most of the high-severity findings below have
> been resolved since the original Phase 1 pass. Section 2 keeps the
> historical entries with a `STATUS:` line; do not treat the H/M/L tags as
> live unless the status line says `Open`. New issues found after the
> reconciliation belong in `docs/product_backlog.md` rather than here.
>
> Resolved:
> - **H1** secret leak — `redactedYAML` in `internal/config/config.go` masks
>   `dgraph_password` and `ws_token` in the startup dump.
> - **H2** server-enable flags — `enabled()` helper in `cmd/proxy/main.go`
>   plus the `select` over `errCh`/`sigCh` makes `enable_http: false` and
>   `enable_websocket: false` take effect, and the process stays alive when
>   only one server is enabled.
> - **H3** purposeful balancer validation — `ValidatePurposeful` in
>   `internal/loadbalancer/pBalancer.go` fails startup when `groups` is empty
>   or any purpose lacks a usable endpoint.
> - **H4** mutation panic on empty upsert — `CheckMutationBody` rejects empty
>   `upsert: []` and `upsert: {}` (`internal/helpers/helpers.go`), and
>   `HandleMutation` covers the multi-block case via `WriteJSONResponseList`.
> - **H5** WS upsert error payload — replaced with `fmt.Appendf` formatting
>   in `internal/websocket/websocket.go`, no literal `%v` left in flight.
> - **M1** `ratel-graphql` YAML key — `normalizeLegacyKeys` rewrites the
>   legacy hyphenated key to `ratel_graphql` before unmarshalling.
> - **M2** `isDQL` operator precedence — new line-by-line implementation in
>   `internal/proxy/utils.go` with explicit unit coverage.
> - **M3** WebSocket dev defaults — `dev_mode`, `ws_token`,
>   `ws_allowed_origins`, plus `validateSecurity` enforce fail-closed
>   behaviour in production. Documented in `docs/security.md` and the README.
> - **L1** gRPC-to-HTTP port arithmetic — `dgraph_http_endpoints` (and the
>   `DGRAPH_HTTP_ENDPOINTS` env override) lets operators pin the HTTP port
>   per gRPC endpoint; the legacy `-1000` formula is now a fallback that
>   logs a warning when used.
> - **Body cap** — 1 MiB default for HTTP and WS, with HTTP 413 on overflow
>   (`maxBytesMiddleware`, `WriteRequestBodyReadError`).
> - **CORS** — `applyCORS` in `internal/proxy/utils.go` honours
>   `cors_allowed_origins` and only enables `Allow-Credentials` for matched
>   origins; the legacy permissive behaviour is gated behind dev mode.
> - **Per-IP rate limiting** — `internal/ratelimit` token bucket wired into
>   `/query`, `/mutate`, `/graphql` and the `/ws` upgrade. Disabled when
>   `rate_limit_rps` is zero (back-compat default).
>
> Still open / not yet addressed:
> - **L2** chatty `fmt.Println` in `internal/dgraph/client.go` — minor.
> - **L3** schema-cleaning facet drop — flagged, not urgent.
> - Docs vs reality drift in section 3 has been partially fixed by README
>   updates; `internal/loadbalancer/idea.md` is still ahead of the README
>   roadmap.
> - Test coverage gaps in section 5: `internal/dgraph` and `internal/routing`
>   still lack tests.

Scope reviewed: `README.md`, `why.md`, `internal/loadbalancer/idea.md`,
`cmd/proxy/main.go`, `internal/config/`, `internal/proxy/`, `internal/helpers/`,
`internal/websocket/`, `internal/loadbalancer/`, `internal/routing/`,
`internal/dgraph/`, `internal/api/`, `manifest/*.yaml`, `examples/cluster/*`,
`e2e/*`, `makefile`, and the dirty working tree.

---

## 1. Product intent vs current implementation

- Stated intent (README / why.md):
  - Dgraph proxy/gateway with load balancing (round-robin and purposeful).
  - HTTP proxy for DQL (`/query`, `/mutate`, `/alter`, `/health`, `/state`,
    `/admin/schema`, `/ui/keywords`).
  - WebSocket server supporting `auth`, `ping`, `query`, `mutation`, `upsert`.
  - GraphQL pass-through to Dgraph's `/graphql`, with Ratel frontend proxying.
  - Aspiration: framework, transpilers, ontology schema, tablet-aware routing.
- Implemented today:
  - `round-robin` (default) and `defined` / `purposeful` balancers.
  - HTTP handlers for the endpoints above, plus `/validate/dql` and
    `/validate/schema` (schema AST helpers).
  - WebSocket handler with hardcoded token auth (`"banana"`) and permissive
    `CheckOrigin`.
  - GraphQL forwarded via `forwardGraphQL` when enabled and query is not DQL.
  - Ratel proxied via `HandleFrontend` when `ratel` host is set.
  - AST experiments under `internal/astgraphql`, `internal/astneo`,
    `internal/astdqlplus`, `internal/parsing` (early, partial).
- Not implemented (despite appearing in docs or roadmap):
  - Health checks (`round-robin-healthy` returns "not implemented").
  - Leader-aware / RW-separated / state-based balancing.
  - Cypher to DQL transpilation.
  - Framework features from why.md (type sharding, ontologies, UID
    reservations, region-based indexing).

Takeaway: the working product is a small Dgraph gateway with optional
WebSocket. Everything else should be treated as research, not near-term.

---

## 2. Correctness / safety issues found

Severity H = high, M = medium, L = low.

### H1. Secret leak via final config log dump

STATUS: Resolved (`redactedYAML` in `internal/config/config.go`).

File: `internal/config/config.go:267-272`.

After masking DGRAPH_PASSWORD everywhere else, `LoadConfig` ends with:

```go
if cfgYaml, err := yaml.Marshal(&cfg); err == nil {
    log.Println("--- Final Loaded Configuration ---")
    for _, line := range strings.Split(strings.TrimSpace(string(cfgYaml)), "\n") {
        log.Println("  " + line)
    }
}
```

The `dgraph_password` field is emitted in plaintext to logs. This is the main
secret-leak path and is trivial to fix by marshalling a redacted copy.

### H2. `enable_http` / `enable_websocket` flag semantics are broken

STATUS: Resolved (helper `enabled()` in `cmd/proxy/main.go` plus `select` over signal/error channels).

File: `cmd/proxy/main.go`.

Two bugs:

1. The checks use `if cfg.EnableHTTP != nil`. Since `LoadConfig` always sets
   these pointers (default `true` when unset), the condition is always true.
   Setting `enable_http: false` in YAML does not disable the HTTP server. Same
   for WebSocket.
2. The final guard:
   ```go
   if cfg.EnableHTTP != nil && cfg.EnableWebSocket != nil {
       log.Fatal("Both HTTP and WebSocket servers are disabled. Nothing to run.")
   }
   ```
   uses `!= nil` (should be disabled-semantics) and uses `AND` (should be `OR`
   of "both disabled"). It is also unreachable in practice because the
   WebSocket branch uses `log.Fatal(http.ListenAndServe(...))` which blocks.

Side effect: when WebSocket is truly disabled but HTTP is enabled, `main`
returns right after launching the HTTP goroutine, so the process exits
immediately and the HTTP server dies with it.

### H3. `defined` / `purposeful` balancer does not validate `Groups`

STATUS: Resolved (`ValidatePurposeful` in `internal/loadbalancer/pBalancer.go`).

File: `cmd/proxy/main.go:25-37`, `internal/loadbalancer/pBalancer.go`.

- If `balancer_type: defined` but `groups:` is empty, `NewPurposefulBalancer`
  returns a balancer with no entries. Every `Next(purpose)` call then fails at
  runtime with 503. Startup succeeds with no warning.
- The purposeful branch does not assign `err` from the constructor (it does
  not return one), so the generic `err != nil` check a few lines later is a
  no-op for that path.

### H4. `HandleMutation` panics on empty upsert response

STATUS: Resolved (`CheckMutationBody` rejects empty arrays/objects; multi-block path now uses `WriteJSONResponseList`).

File: `internal/proxy/handler.go:57-91`.

After running N upsert goroutines:

```go
if len(errs) > 0 { ... return }
helpers.WriteJSONResponse(w, http.StatusOK, responses[0])
```

- If `upserts` is `[]` (empty JSON array accepted by `CheckMutationBody`),
  `responses` is nil/empty and `responses[0]` panics.
- On success with N > 1, only the first response is returned; later responses
  and their uids/latency are silently dropped.
- `WriteJSONResponse` dereferences `resp` fields without nil checks.

### H5. WebSocket upsert error payload is malformed

STATUS: Resolved (replaced with `fmt.Appendf` in `internal/websocket/websocket.go`).

File: `internal/websocket/websocket.go:157`.

```go
conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"%v"}`))
```

The `%v` is never substituted and the `err` value is discarded. Clients get
the literal placeholder on upsert backend selection failures.

### M1. `ratel-graphql` YAML key mismatch

STATUS: Resolved (`normalizeLegacyKeys` rewrites the legacy hyphenated key before unmarshalling).

Files: `manifest/config.yaml:10`, `manifest/config_docker.yaml:9`.

The Go struct tag is `ratel_graphql`, but both example manifests use
`ratel-graphql`. The flag is silently ignored when loaded from YAML and falls
back to the environment default (`true`). Also, `RATEL_GRAPHQL` env with an
unparseable value downgrades to a simple `!= "false"` check, which can
accidentally enable the flag.

### M2. `isDQL` operator precedence bug

STATUS: Resolved (line-by-line implementation in `internal/proxy/utils.go` with explicit unit coverage).

File: `internal/proxy/utils.go:8-16`.

```go
if len(line) > 0 && !strings.HasPrefix(line, "#") && strings.Contains(line, "func:") || strings.Contains(line, "schema {}") {
```

Because `&&` binds tighter than `||`, `schema {}` matches even on comment
lines. Unlikely to cause user-visible incidents today, but wrong. Also, the
DQL/GraphQL detection rule is very loose — a GraphQL document that mentions
`func:` in a string argument would be misrouted.

### M3. WebSocket defaults are dev-only but not labelled as such

STATUS: Resolved (`dev_mode`, `ws_token`, `ws_allowed_origins`, `validateSecurity`; documented in README + `docs/security.md`).

Files: `internal/websocket/websocket.go:16-23`, `internal/websocket/papers.go:48`.

- `CheckOrigin` returns `true` always. Comment says "not safe for production".
- `IsValidToken` returns `token == "banana"`. Hardcoded.
- There is no config knob to swap either.
- Docs (README) present WebSocket as a normal feature without calling out
  that it is development-only as shipped.

### L1. gRPC to HTTP port derivation is hardcoded

STATUS: Resolved (`dgraph_http_endpoints` map / `DGRAPH_HTTP_ENDPOINTS` env override; legacy `-1000` formula remains as a logged fallback).

File: `internal/proxy/proxy.go:95-103`, `internal/loadbalancer/balancer.go:81`.

- Reverse proxy computes HTTP port as `grpcPort - 1000`.
- Round-robin balancer stores `offset = port - 9080`.
- Works for canonical Dgraph ports (`9080`/`8080`, `9081`/`8081`, ...). Breaks
  silently for custom ports.

### L2. Chatty client creation logs

File: `internal/dgraph/client.go:18-19`. `fmt.Println` for every endpoint
client created. Should be `log.Printf` or removed.

### L3. `schema {}` cleaning misses predicate-level `dgraph.*` in array form

File: `internal/proxy/proxy_dql.go:49-108`. Current filter drops top-level
predicates and types prefixed with `dgraph.`, which is fine, but removes
facet/tokeniser metadata that some callers rely on. Not urgent; just flagged.

---

## 3. Docs vs reality drift

- README says HTTP endpoint examples use port `8080`; manifests use `8084`.
- README lists balancer modes as `round-robin` and `defined`; code also
  accepts `purposeful` as an alias.
- README claims "GraphQL queries via Ratel" as a feature; the actual knob is
  `graphql: true`, which enables `forwardGraphQL` on `/query` and `/graphql`.
  `ratel-graphql` (misnamed key) is unrelated to that path and currently
  ignored when read from YAML.
- README WebSocket section does not mention the hardcoded token or that
  `CheckOrigin` accepts every origin.
- Roadmap in README conflates framework-level research (`why.md`,
  `idea.md`) with near-term work. There is no "Current Status" section.
- `internal/loadbalancer/idea.md` contains concrete ideas (health, state,
  tablet moves) that are not reflected in the README roadmap order.

---

## 4. Dependency / version coherence

- `go.mod` imports `github.com/hypermodeinc/dgraph/v24 v24.1.2`
  (used by `internal/astgraphql/ast.go` for `schema.NewHandler`).
- `examples/cluster/docker-compose.yml` pins
  `dgraph/dgraph:v25.0.0-preview1` and `dgraph/ratel:v25.0.0-preview1`.
- The gRPC wire protocol between dgo v240 and v25 preview is typically
  compatible, but the schema handler pulled from the v24 Go module may
  disagree with v25 server behaviour for GraphQL schemas.
- This mismatch is not documented anywhere.

Recommendation: either pin Docker to `v24.*` for now, or add a note
explaining the mixed-version posture and run a smoke test before shipping
GraphQL schema features against v25.

---

## 5. Test coverage snapshot

- `internal/helpers`: good coverage for `CheckMutationBody` paths.
- `internal/astgraphql`: schema-to-JSON test rewritten in the working tree
  to use the new Dgraph-aware parser.
- `internal/astneo`, `internal/parsing`: basic parse tests.
- `internal/api`: validation handler tests present.
- `internal/proxy`: `handler_test.go` is all `TODO` stubs (untracked).
- `internal/config`, `internal/loadbalancer`, `internal/websocket`,
  `internal/routing`, `internal/dgraph`: no tests.
- `e2e/*`: requires a running stack on `:8084` / `:8089`. Not gated.
  `go test ./e2e/...` will fail without services.

---

## 6. E2E / Docker posture

- `make rund` brings up 1 zero, 3 alphas, 1 ratel, 1 otter via
  `docker compose`.
- Only `dgraph-alpha` (group 1) exposes stable host ports (`8081`, `9081`).
  alpha2/alpha3 use ephemeral host ports, which is fine if everything goes
  through `otter`, but confusing in docs.
- `e2e/http/endpoints_test.go` dials `localhost:8084` unconditionally. The
  package has no build tag or env gate, so it will always fail locally
  without Docker. It is only skipped today because the tests live under
  `e2e/`, which is not part of `./internal/...`.
- `e2e/setup/setup.go` talks to `localhost:9080`, but the compose file
  exposes `9081`. Out of sync.

---

## 7. Conclusion

The base gateway works for local DQL use against a single Dgraph cluster.
The main risks are:

1. Secret leakage from the final config dump.
2. Server-enable flags silently ignored.
3. Defined balancer accepting empty config at startup.
4. Mutation upsert path crashing on an unusual input shape.
5. WebSocket dev defaults presented as production features.

These are the targets of the Phase 1 hardening pass. The rest (framework,
ontology, new transpilers, leader-aware balancing) stays on the roadmap and
should not be implemented until the base is trustworthy.
