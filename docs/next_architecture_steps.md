# Otter — Next Architecture Steps

Companion to `docs/product_backlog.md`. The backlog ranks *items*; this
document ranks *intent*. It is deliberately opinionated.

## Product direction statement

> **For the next 1-2 phases, Otter should optimize for being the
> observable, purpose-aware Dgraph gateway.** Every hour of
> engineering should go toward making the gateway more operable,
> more honest about cluster state, and more trustworthy under load.
> The transpiler work and the framework / ontology ideas stay on
> the roadmap but off the schedule.

Restated as a single test: if a contributor joins today, can they
deploy Otter in front of a three-alpha Dgraph cluster, watch a dashboard,
catch a failure, and roll back safely? Today the answer is "mostly, if
they read the code". The next two phases should make the answer
"yes, the docs are enough".

## What Otter should optimize for

### 1. Operability over feature breadth

Phase 3 already added timeouts, graceful shutdown, body caps, and
auth modes. The remaining operability gaps are:

- No structured logs, so correlation is manual.
- No metrics surface, so dashboards and alerting have nothing to
  scrape.
- No body-size 413 response, so operators see 400s and think it is
  the client's fault.
- No rate limiting, so one misbehaving client can saturate a
  backend.

These four fixes (backlog items **N1, N2, N5** and Phase-4 **X1**)
buy more real-world reliability than any new feature would. They
also unlock the visibility needed to tell when the more ambitious
items are ready to land.

### 2. Honest cluster state before cleverer balancing

The balancer code is in a strange spot: `round-robin-healthy` is
an advertised mode that returns an error, and `purposeful` works
but has no awareness of whether its backends are alive. The
right sequence is:

1. Build the substrate: a cached read of `/state` and `/health`
   (backlog item **X3**).
2. Make decisions *visible* before making them *smart*: expose the
   cached state via an Otter route and log which backend got picked
   and why.
3. Only then teach the balancer to skip unhealthy nodes (**X2**) or
   prefer non-leaders (**X5**, **X7**).

This sequence avoids the trap of shipping "smart" balancing that is
actually just "opaque" balancing. It also means every balancer
feature ships with an inspection endpoint, which is what operators
will reach for first when something goes wrong.

### 3. A config shape that survives a heterogeneous cluster

The hidden contract in `internal/proxy/proxy.go` &mdash; HTTP port is
gRPC port minus 1000 &mdash; is fine for the shipped compose file
and nothing else. A shop with Dgraph on custom ports has no way to
tell Otter the truth. Backlog items **N3** and **N4** replace the
arithmetic with an explicit per-endpoint address pair. This is the
kind of fix that looks small but quietly enables every deployment
that is not a copy of the example cluster.

### 4. Tests that stay hermetic

The `e2e` build-tag contract has already paid for itself: `go test
./...` runs in two seconds and never touches Docker. Every new
feature in Now and Next must keep that property. Any test that
requires a live Otter carries `//go:build e2e`. No exceptions.

## What should explicitly wait

These decisions are not "maybe later"; they are
"explicitly not the next two phases":

### Cypher → DQL transpilation

`internal/astneo/` shows real effort &mdash; a participle-based parser
covering `MATCH`, `WHERE`, `RETURN`, and (as a stub) `CREATE`. What
is *not* there is the translation layer from the Cypher AST to DQL.
The parser recognises a small fragment of Cypher, and
`ParseQueryParts` signals "I parsed CREATE" by returning a non-nil
*error*. That is stub code.

Shipping an 80 % transpiler for a query language is actively harmful:
users file bugs that cannot be closed, and the project becomes known
for a half-feature. The right trigger for resuming this work is
*after* operability is solid and *after* one canonical end-to-end
flow is green with golden tests. Until then, `internal/astneo`
should be treated as a research sandbox, not a feature that needs
maintaining.

Concretely: do not reference Cypher in README claims, marketing, or
issue templates for the next two phases.

### Framework / ontology / type sharding

`why.md` documents a compelling long-term thesis: Otter becomes an
opinionated framework with ontological schemas, region-based
indexing, UID reservation, and predicate prefixing. None of this is
scaffolded in `internal/`. `internal/astgraphql/` is the closest
thing and it only reads Dgraph's own GraphQL schema into a node
tree for visualisation purposes.

Adding framework primitives now inverts the dependency: the gateway
is not yet the mandatory front door, so any opinionated schema work
is a feature in search of a user. Revisit this only after
leader-aware routing (X5) and the Cypher harness have both shipped.
At that point "opinionated graph model" stops being a leap and
becomes the next obvious layer.

### jemalloc and goroutine introspection

`internal/loadbalancer/idea.md` flirts with using
`/debug/jemalloc` and CPU / goroutine introspection for balancing
decisions. In practice, `/state` and `/health` cover the 80 %
case and a standard Prometheus scrape covers the rest. The 20 %
that jemalloc unlocks is not worth the coupling to Dgraph's
private debug surface.

### Multi-language transpilers (SPARQL, etc.)

Tracked in `why.md`. Strictly "after Cypher works end to end". If
Cypher is not ready, additional languages are not a conversation.

## Concrete two-phase plan

### Phase 4 &mdash; observability and rate control

Goal: an operator can run Otter in front of a real cluster, scrape
metrics, alert on failure, and bound misbehaving clients.

Scope (from the backlog):

- **N1** structured logging + request IDs
- **N2** 413 for oversize bodies
- **N5** per-IP rate limiting on HTTP + WS
- **N6** TODO / dead-code sweep
- **N7** WS per-connection body cap
- **X1** Prometheus `/metrics` on a dedicated port
- **X4** Docker `healthcheck` in `docker-compose.yml`

Exit criteria:

1. `make e2e` passes and `/metrics` returns non-empty output.
2. A burst test (backlog validation for **N5**) rejects excess
   traffic with 429.
3. A request that exceeds `max_body_bytes` is rejected with **413**
   and a response body that names the offending limit.
4. `docs/runbook.md` shows how to scrape `/metrics` and how to
   configure the rate limiter.
5. `go test ./...` runs in under ten seconds with no Docker.

### Phase 5 &mdash; cluster awareness

Goal: Otter reads `/state`, makes balancing decisions from it, and
exposes what it sees.

Scope (from the backlog):

- **N3** + **N4** endpoint address-pair config
- **X2** `round-robin-healthy` real implementation
- **X3** cached cluster state inspector
- **X5** feed `/state` into balancer decisions
- **X6** WS per-connection rate limiting + idle disconnect
- **X7** read/write split balancer (`round-robin-on-RW`)

Exit criteria:

1. Kill an alpha mid-`make e2e` and traffic drains off it within the
   configured health interval.
2. `/otter/state` returns a cached, truthful snapshot of the cluster.
3. The read/write split balancer correctly routes a known-write
   upsert to a known-writable backend in a 3-alpha integration test.
4. Any schema change in Dgraph's `/state` payload across majors is
   detected by a dedicated fixture test (not just by production
   breakage).

At the end of Phase 5, Otter is an *operable* gateway. That is the
right moment to ask whether the Cypher and framework work should
resume.

## Decision log

Short record of the decisions that frame this plan, so they are not
re-litigated every phase:

- **No production claims in README until Phase 5 exit.** The README
  reality-alignment pass (`docs/doc_drifts.md`) dialled the
  language back; keep it dialled back.
- **Dev-mode stays permissive by default.** The fail-closed prod
  mode from Phase 3 is the contract; do not inherit strictness into
  dev defaults, because that is the path to people disabling
  security checks to get `make e2e` to run.
- **`e2e` build tag is load-bearing.** Anything that tries to
  "simplify" the test layout by dropping the tag gets rejected.
- **`//go:build e2e` tests may assume Otter on `localhost:8084`
  and `localhost:8089`.** They may *not* assume a particular
  balancer mode; tests that care pin `BALANCER_TYPE`.
- **Config shape is part of the public API.** Any breaking change
  in YAML keys needs a migration note, even pre-1.0.
- **Dgraph major-version skew is a first-class risk**, not a
  footnote. `README.md` has the v24 vs v25 note; `docs/security.md`
  will grow a sibling note for X5 when `/state` schema dependency
  becomes real.

## What this plan is *not*

This plan is not a promise of dates, a pitch for funding, or a
map of every contributor's interests. It is a statement of what
the codebase is ready to absorb next, ordered so that each step
increases Otter's trustworthiness instead of its surface area.

If the priorities shift &mdash; for example, if a paying user lands
who *needs* Cypher &mdash; the test for whether to reorder is simple:
*can we ship that feature without regressing the operability work
in Phase 4?* If yes, reorder freely. If no, ship Phase 4 first.
