# Otter 🦦

> Under construction 🚧

> Built for performance. Designed for graphs.

Otter is a lightweight, purpose-driven proxy and query transpiler for [Dgraph](https://dgraph.io).  
It intelligently balances traffic between Dgraph clusters and adds support for advanced query workflows — including Cypher-to-DQL translation (in progress).


Otter aims to serve as the foundation for future support of multiple graph languages, offering modular extensions, semantic enrichment, and introspection tools.

# Why this Software?

Read [Why](why.md) this software was created.

---


# Otter Design

Current design.

![Otter Design](assets/design.png)

### Features

-  Round-robin and purposeful load balancing across Dgraph alphas
-  HTTP proxy for Dgraph `/query`, `/mutate`, `/alter`, `/health`, `/state`,
   `/admin/schema`, `/ui/keywords`
-  WebSocket server with support for `auth`, `ping`, `query`, `mutation`, `upsert`
-  GraphQL pass-through to Dgraph's `/graphql` endpoint when enabled
-  Configurable via YAML and environment variables

### Current Status

- Working today: the items listed above, against a local Dgraph cluster.
- Experimental / dev-only: WebSocket auth is a single hardcoded token
  (`"banana"`) and accepts every `Origin`. See `docs/repo_audit.md` for
  details and `docs/phase1_backlog.md` for the planned hardening.
- Not implemented yet despite appearing in the roadmap: health-aware
  balancing, leader-aware routing, Cypher transpilation, and the framework
  features from `why.md` and `internal/loadbalancer/idea.md`. Those remain
  research directions.
- Dgraph version posture: the Go module depends on `hypermodeinc/dgraph/v24`
  for GraphQL schema handling, while `examples/cluster/docker-compose.yml`
  pins `dgraph/dgraph:v25.0.0-preview1`. gRPC traffic is compatible in
  practice but schema behaviour is not guaranteed across majors; smoke-test
  before relying on it.

---

## Development Workflow

Full step-by-step commands live in `docs/runbook.md`. Short version:

- `make test` / `make test-unit` &mdash; fast local tests, no Docker.
- `make e2e-up` + `make e2e-wait` &mdash; boot the Docker stack and wait
  until Otter answers on `/health` and `/query`.
- `make test-e2e` &mdash; run the Docker-backed suite (build tag `e2e`).
- `make e2e` &mdash; one-shot: up, wait, seed, test, tear down.
- `make e2e-down` &mdash; stop the stack and drop its named volumes.

`go test ./...` only runs unit tests; the E2E suite is gated by the
`e2e` build tag so a clean checkout stays green without Docker.

## Run Otter with Docker

Requirements

* Clone the repository

* Docker

* Docker Compose

* (optional) make installed

#### Run with make

```bash
make rund          # foreground, logs streamed
# or
make e2e-up        # background; pair with make e2e-wait
```

 Manual Docker Compose
If you don't have make:

```bash
cd examples/cluster
docker compose up --build
```

#### Configuration
By default, Otter will load config from:

```ini
CONFIG_FILE=/app/manifest/config_docker.yaml
```

If you want to change the config:

```
manifest/config_docker.yaml
```

Or override with environment variables (see internal/config/config.go for supported vars)

---

### Example WebSocket Payload

```json
{
  "type": "upsert",
  "query": "query { u as var(func: eq(email, \"test@example.com\")) }",
  "mutation": "uid(u) <name> \"Test\" .",
  "cond": "@if(eq(len(u), 1))",
  "commitNow": true
}
```

---

### Run Locally

```bash
git clone https://github.com/OpenDgraph/Otter.git
cd Otter
```

```bash
export CONFIG_FILE=./manifest/config.yaml
go run cmd/proxy/main.go
```

Set your balancer strategy inside `config.yaml`:

```yaml
balancer_type: purposeful # or round-robin
```

---

###  HTTP Proxy Endpoints

| Endpoint   | Method | Description         |
|------------|--------|---------------------|
| `/query`   | POST   | Executes a DQL query |
| `/mutate`  | POST   | Executes a mutation  |

Supported Content-Types:

- `application/json`
- `application/dql`

Example request (default proxy port is `8084` in the shipped manifests):
```bash
curl -X POST http://localhost:8084/query \
  -H "Content-Type: application/json" \
  -d '{"query": "{ data(func: has(email)) { uid name email } }"}'
```

---

---

### WebSocket Usage

**URL**: `ws://localhost:8089/ws`

> Otter ships with `dev_mode: true` by default, which means the WebSocket
> handler auto-generates a random auth token on startup (logged once) and
> accepts every `Origin`. The Docker example pins `ws_token: "banana"` for
> reproducibility &mdash; change it before exposing port 8089.
>
> Set `dev_mode: false` and provide `ws_token` and `ws_allowed_origins`
> (or the `DEV_MODE`, `WS_TOKEN`, `WS_ALLOWED_ORIGINS` env vars) to run
> with the fail-closed behaviour. See `docs/security.md` for details.

#### Supported message types:

- `auth` -> authenticate
- `ping` -> keep connection alive
- `query` / `mutation` / `upsert` → require authentication

#### Example (after auth):

```json
{
  "type": "query",
  "query": "{ data(func: has(email)) { uid name email } }",
  "token": "banana",
  "verbose": true
}
```

###  Load Balancing Modes

Available types:

- `round-robin` *(default)*
- `defined` or `purposeful` *(per-purpose: query/mutation/upsert)*

To use `defined`, provide a YAML like this:

```yaml
balancer_type: defined
groups:
  query:
    - localhost:9080
  mutation:
    - localhost:9081
  upsert:
    - localhost:9082
```

Otter now validates `groups` at startup and fails fast when the map is
empty or any purpose has no usable endpoint.

---

###  Roadmap

Short-term (see `docs/phase1_backlog.md`):
- [ ] Configurable WebSocket auth token and origin allowlist
- [ ] HTTP server timeouts and graceful shutdown
- [ ] Build-tag gating for E2E tests

Mid-term:
- [ ] `round-robin-healthy` support
- [ ] `round-robin-on-RW` separate readonly and write only
- [ ] Cluster state inspection via `/state`

Research (see `why.md` and `internal/loadbalancer/idea.md`):
- [ ] Leader-aware routing (`round-robin-avoid-leaders`, `round-robin-leaders-only`)
- [ ] State-based balancing using resource introspection
- [ ] Graph model abstraction and ontology schemas
- [ ] Cypher / other transpilers
- [ ] Become a framework

Implemented today:
- [x] `round-robin` basic round-robin
- [x] `round-robin-purposeful` / `defined`

---