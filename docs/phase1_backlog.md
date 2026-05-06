# Otter Phase 1 Backlog

Ranked by expected value divided by effort. Items in `Now` are the minimum
work to make the base gateway trustworthy. `Next` is what should follow once
Phase 1 lands. `Later` is anything that depends on the base being stable.

Status tags:
- `[done]` implemented in the Phase 1 hardening pass.
- `[open]` not yet implemented.

---

## Now (Phase 1 hardening)

1. `[done]` Redact secrets in final config log dump.
   - Why: `dgraph_password` is currently written to stdout logs.
   - Change: marshal a copy with the sensitive fields masked before logging.
   - Validation: unit test asserting redaction.

2. `[done]` Fix `enable_http` / `enable_websocket` semantics in startup.
   - Why: `enable_http: false` and `enable_websocket: false` are silently
     ignored today. The "both disabled" guard is unreachable and inverted.
   - Change: check the dereferenced pointer, run servers concurrently, wait
     on at least one, and fail fast if both are disabled.
   - Validation: unit test (cannot start real sockets), plus manual smoke.

3. `[done]` Validate `groups` for the `defined` / `purposeful` balancer.
   - Why: starting Otter with `balancer_type: defined` and no `groups` or
     no valid endpoints is silent until the first request.
   - Change: error out at startup if the purposeful balancer cannot serve
     at least one purpose with at least one valid endpoint.
   - Validation: unit test on `NewPurposefulBalancer` / startup path.

4. `[done]` Harden `HandleMutation` upsert path.
   - Why: empty `upsert: []` arrays and partial failures could panic or
     silently drop responses.
   - Change: treat empty upsert arrays as a 400, aggregate all responses in
     a stable shape, and tolerate nil fields in `WriteJSONResponse`.
   - Validation: unit tests for empty array, single and multi upsert.

5. `[done]` Fix WebSocket upsert error payload formatting.
   - Why: `{"error":"%v"}` literal is returned when backend selection fails.
   - Change: use `fmt.Appendf` / `fmt.Sprintf` with the real error.

6. `[done]` Translate oversize-body rejections to HTTP 413.
   - Why: `http.MaxBytesReader` was wired, but handlers still surfaced a
     generic 400 on read failure.
   - Change: detect `*http.MaxBytesError`, map it to
     `413 Payload Too Large`, and include the configured byte limit in
     the error payload.
   - Validation: helper + API + proxy handler tests.

7. `[done]` Cap incoming WebSocket messages.
   - Why: the HTTP body cap did not protect the WS path.
   - Change: add `ws_max_message_bytes` / `WS_MAX_MESSAGE_BYTES`
     (default 1 MiB) and apply it via `conn.SetReadLimit`.
   - Validation: config tests and WS read-limit tests.

8. `[done]` Reconcile `ratel-graphql` YAML key.
   - Why: YAML struct tag is `ratel_graphql`; both shipped manifests use
     `ratel-graphql`, so the flag is silently ignored.
   - Change: accept both keys on load (alias), or fix the manifests.
     Chose to accept both to stay backward compatible.

9. `[done]` Fix `isDQL` operator precedence bug + flag the detection as
   best-effort.
   - Why: comment-only lines containing `schema {}` were incorrectly treated
     as DQL.
   - Change: add explicit parentheses, add a targeted unit test, keep the
     heuristic but document it.

10. `[done]` README reality alignment (ports, balancer names, WS caveats,
   version mismatch note, current status section).

9. `[open]` Add a `dev-mode: true` default with an explicit WebSocket token
   and origin-allowlist config.
   - Why: currently `"banana"` token and `CheckOrigin = true` are production
     footguns. At minimum make the token configurable.
   - Deferred here because it crosses config + websocket + docs surfaces
     and deserves its own pass. Tracked in `Next`.

---

## Next (Phase 2)

1. `[done]` WebSocket auth hardening.
   - `ws_token` / `WS_TOKEN` configurable, random ephemeral token in dev
     mode (logged once), hard requirement in non-dev mode.
   - `ws_allowed_origins` / `WS_ALLOWED_ORIGINS` with exact and
     `*.subdomain` matching.
   - Constant-time token comparison via `crypto/subtle`.
   - See `docs/security.md`.

2. `[done]` HTTP server hardening.
   - `http.Server` with explicit `ReadHeaderTimeout`, `ReadTimeout`,
     `WriteTimeout`, `IdleTimeout` for the HTTP proxy; handshake-only
     timeouts for the WebSocket upgrader.
   - `http.MaxBytesReader` middleware driven by `max_body_bytes` /
     `MAX_BODY_BYTES` (default 1 MiB).
   - Graceful shutdown on SIGINT/SIGTERM with a 10s grace period.
   - GraphQL upstream client has a 30s total timeout.

3. Gateway observability.
   - Structured logs (slog), request IDs, basic metrics (Prometheus-compatible
     handler on a separate port).
   - Health endpoint that actually checks backend reachability.

4. `[done]` E2E test discipline.
   - `//go:build e2e` tag on every file under `e2e/*`.
   - `e2e/setup/setup.go` now honours `DGRAPH_GRPC` and defaults to the
     port that `examples/cluster/docker-compose.yml` actually exposes.
   - `make e2e` boots the stack, waits on `scripts/wait-for-otter.sh`,
     seeds sample data, runs the tagged suite, and tears down. Split
     targets (`test-unit`, `test-e2e`, `e2e-up`, `e2e-down`, `e2e-wait`,
     `e2e-seed`) are documented in `docs/runbook.md`.

5. Port assumption cleanup.
   - Stop hardcoding `grpcPort - 1000` for HTTP; store HTTP port alongside
     gRPC port in `EndpointInfo` or derive it from config.

6. Docker healthchecks.
   - `make e2e-wait` is a stopgap; once verified on the shipped Dgraph
     image, add proper container healthchecks and switch `depends_on` to
     `condition: service_healthy`.

---

## Later

- Health-aware round robin (`round-robin-healthy`).
- Read/write split balancer (`round-robin-on-RW`).
- Leader-aware routing (`round-robin-avoid-leaders`, `round-robin-leaders-only`).
- State-based balancer using `/state` and resource introspection from
  `internal/loadbalancer/idea.md`.
- Cypher to DQL transpiler (`internal/astneo`).
- Framework features from `why.md`: type sharding, ontology schemas, UID
  reservations, region-based indexing, query decorators.

These depend on the base gateway being trustworthy, observable, and
covered by a working E2E harness. Do not start them in Phase 1.

---

## Recommended next 3 items after Phase 1

1. WebSocket auth hardening (Next #1).
2. HTTP server timeouts + graceful shutdown (Next #2).
3. E2E gating + runbook (Next #4).
