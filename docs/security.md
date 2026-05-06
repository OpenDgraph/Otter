# Otter Security Notes

Short, truthful summary of what the shipped defaults do and do not protect
against, and how to move from dev defaults to something safer.

## Operating modes

Otter has two operating modes selected via `dev_mode` (YAML) or `DEV_MODE`
(env). `dev_mode` defaults to `true` if unset.

### `dev_mode: true`

- If `ws_token` is unset, Otter generates a random token at startup and
  logs it once to stdout with an explicit warning. Restarting rotates it.
- If `ws_allowed_origins` is empty, the WebSocket `CheckOrigin` accepts
  every origin. A warning is logged at startup.
- Suitable for local development, CI, and the shipped Docker example.

### `dev_mode: false`

- Otter refuses to start unless **both** are explicit:
  - `ws_token` (or `WS_TOKEN` env) is non-empty.
  - `ws_allowed_origins` (or `WS_ALLOWED_ORIGINS` env) has at least one
    entry.
- This is the contract that makes the dev-only defaults safe: they only
  apply when the operator has explicitly asked for dev mode.

Turn off dev mode like this:

```yaml
dev_mode: false
ws_token: "replace-me-with-a-real-secret"
ws_allowed_origins:
  - https://app.example.com
  - "*.internal.example.com"
```

Or via environment:

```bash
export DEV_MODE=false
export WS_TOKEN="$(openssl rand -hex 24)"
export WS_ALLOWED_ORIGINS="https://app.example.com,*.internal.example.com"
```

## HTTP server hardening

The HTTP proxy and the WebSocket upgrade endpoint both run on
`http.Server` instances with explicit timeouts:

| Field               | Value           | Notes                                  |
|---------------------|-----------------|----------------------------------------|
| `ReadHeaderTimeout` | 5s              | mitigates slow-loris                   |
| `ReadTimeout`       | 30s (HTTP only) | bounds the full request read           |
| `WriteTimeout`      | 60s (HTTP only) | long enough for batched upserts        |
| `IdleTimeout`       | 120s            | closes idle keep-alive connections     |

On SIGINT or SIGTERM, Otter calls `srv.Shutdown` on both servers with a
10-second grace period before forcing exit.

Request bodies on the HTTP proxy are capped with `http.MaxBytesReader`.
The cap defaults to 1 MiB. Oversized bodies are rejected with HTTP 413.
Override with:

```yaml
max_body_bytes: 4194304   # 4 MiB
```

or

```bash
export MAX_BODY_BYTES=4194304
```

WebSocket messages are capped separately, also defaulting to 1 MiB:

```yaml
ws_max_message_bytes: 4194304   # 4 MiB
```

or

```bash
export WS_MAX_MESSAGE_BYTES=4194304
```

## WebSocket auth

- Token comparison uses `crypto/subtle.ConstantTimeCompare` via
  `ConstantTimeTokenEqual` to avoid leaking the token length through
  timing.
- Origin matching supports exact URL strings and `*.example.com`
  subdomain wildcards. A literal `*` pattern is accepted only as a
  deliberate escape hatch in dev mode.
- `validate.go` already rejects auth messages without a token.

## GraphQL upstream

The `forwardGraphQL` path uses a dedicated `http.Client` with a 30s
total timeout, so a hanging Dgraph backend cannot pin a handler
goroutine indefinitely.

## Config redaction

The startup "Final Loaded Configuration" log dump redacts both
`dgraph_password` and `ws_token` before logging. Neither is ever
printed in plain text after `LoadConfig` returns.

## Residual risks (intentionally unfinished)

The following are called out explicitly so operators know what they
are picking up:

- There is no per-client rate limit on either the HTTP or WebSocket
  surface. A single abusive client can still saturate a backend.
- There is no mTLS or TLS termination in Otter itself; the shipped
  setup assumes a reverse proxy in front for that.
- The WebSocket token is a shared secret, not per-user auth. Rotation
  requires restarting Otter. A multi-user auth story is not in scope
  for Phase 3.
- `internal/proxy/proxy.go:95` still derives the HTTP port as
  `grpcPort - 1000`. It works for canonical Dgraph ports and will
  silently misbehave otherwise.
- Health checks in `docker-compose.yml` are replaced by the host-side
  `scripts/wait-for-otter.sh`; there is no liveness/readiness probe
  baked into Otter itself.

Each of these is a tracked follow-up in `docs/phase1_backlog.md`
under the "Next" and "Later" sections.
