# Otter Runbook

Copy-pastable commands for the three supported workflows. Anything not
listed here is intentionally out of scope for Phase 1.

## Prerequisites

- Go 1.24+ (matches `go.mod`).
- Docker with Compose v2. OrbStack works.
- `curl` (used by the wait helper).

Nothing else is required for the fast local path.

---

## 1. Fast local checks (no Docker)

Use this when iterating on code. It compiles the binary and runs unit +
internal integration tests. No network, no Docker.

```bash
make build         # compiles cmd/proxy into build/Otter
make test          # alias for test-unit
make test-unit     # go test ./cmd/... ./internal/...
go vet ./cmd/proxy ./internal/...
```

`go test ./...` is also safe: the E2E suite is gated behind the `e2e`
build tag, so a plain invocation only runs unit tests.

---

## 2. Run Otter against a Docker-backed Dgraph cluster

Boot the full stack (zero, three alphas, ratel, otter):

```bash
make e2e-up        # docker compose up -d --build
make e2e-wait      # polls /health and /query until ready
```

Hitting the stack from the host:

```bash
curl -s http://localhost:8084/health
curl -s -X POST http://localhost:8084/query \
  -H 'Content-Type: application/dql' \
  -d '{ q(func: has(name), first: 5) { uid name } }'
```

WebSocket (dev-only defaults: token `banana`, any origin accepted):

```bash
websocat ws://localhost:8089/ws <<'EOF'
{"type":"auth","token":"banana"}
{"type":"ping"}
EOF
```

Tear down:

```bash
make e2e-down      # docker compose down -v (removes named volumes)
```

### Seeding

The seeder is a one-off program under `e2e/setup`:

```bash
# Uses DGRAPH_GRPC env; defaults to localhost:9081 (compose alpha1 port).
make e2e-seed
```

If you run Dgraph locally without Docker on the standard port, override
the endpoint:

```bash
DGRAPH_GRPC=localhost:9080 make e2e-seed
```

---

## 3. Docker-backed E2E tests

One-shot, leaves no state behind:

```bash
make e2e
```

This boots the compose stack, waits for readiness, seeds sample data,
runs `go test -tags=e2e ./e2e/...`, and tears the stack down. The exit
code is the test exit code.

To run the suite against an already-running stack (faster iterate loop):

```bash
make e2e-up
make e2e-wait
make e2e-seed      # optional
make test-e2e      # go test -tags=e2e -count=1 ./e2e/...
# when done
make e2e-down
```

---

## Testing strategy at a glance

| Category                     | Command                       | Needs Docker | Build tag |
|------------------------------|-------------------------------|--------------|-----------|
| Unit / internal integration  | `make test-unit`              | no           | none      |
| Default `go test ./...`      | `go test ./...`               | no           | none      |
| Docker-backed E2E            | `make test-e2e`               | yes          | `e2e`     |
| One-shot E2E (up + test + down) | `make e2e`                | yes          | `e2e`     |

The `e2e` build tag is the single source of truth: if a test requires
a live Otter on `localhost:8084` / `localhost:8089`, it must carry the
`//go:build e2e` header so that a plain `go test ./...` stays green on
a machine without Docker.

---

## Ports and config reference

From `manifest/config_docker.yaml` and `examples/cluster/docker-compose.yml`:

| Service               | Host port   | Purpose                  |
|-----------------------|-------------|--------------------------|
| otter (HTTP)          | 8084        | DQL + GraphQL proxy      |
| otter (WebSocket)     | 8089        | WS gateway (dev auth)    |
| dgraph-zero           | 5080, 6080  | zero gRPC and admin HTTP |
| dgraph-alpha (alpha1) | 8081, 9081  | alpha HTTP + gRPC        |
| dgraph-alpha2         | ephemeral   | alpha HTTP + gRPC        |
| dgraph-alpha3         | ephemeral   | alpha HTTP + gRPC        |
| dgraph-ratel          | ephemeral   | Ratel UI                 |

alpha2 and alpha3 deliberately do not expose stable host ports. Cluster
traffic goes through Otter; only alpha1 is directly reachable from the
host for ad-hoc debugging.

---

## Known limitations (Phase 1)

- No Docker healthchecks on the Dgraph containers; `make e2e-wait`
  compensates by polling Otter.
- `examples/cluster/docker-compose.yml` pins
  `dgraph/dgraph:v25.0.0-preview1` while the Go module pulls
  `github.com/hypermodeinc/dgraph/v24` for schema parsing. gRPC is
  compatible in practice but GraphQL schema behaviour is not guaranteed
  across majors.
- WebSocket ships with a hardcoded token and permissive `CheckOrigin`.
  Never expose port 8089 outside a trusted development environment.
  See `docs/phase1_backlog.md` for the planned hardening.
