# UID Reservation — Implementation Sketch

> **Status: DRAFT / DISCUSSION.** Companion to
> `docs/design/uid_reservation.md`. This doc proposes the package
> layout, lifecycle, and test harness for the first implementation
> PR. **No code lands until this doc is reviewed.** Nothing here
> overrides the round-1 / round-2 decisions in the parent doc; if a
> conflict exists, the parent wins and this doc is wrong.

## What this doc commits to

Only the *mechanical* shape of the first PR. Specifically:

- Where the new package lives and what files it contains.
- What the YAML config block looks like.
- The startup sequence for Mode A (unchanged) and Mode B (new).
- How `/admin/assign` is called and how `0x01` is written.
- How subsequent boots validate the recorded reservation.
- The fixture-backed test harness.

## Round-3 resolutions (2026-05-04)

The original *Open implementation questions* (Q1–Q5 at the bottom)
were five mechanical unknowns. Round 3 closes three of them outright
and reshapes the remaining two:

- **Q1 (Zero vs Alpha for `/admin/assign`):** **Zero.** The endpoint
  has historically lived on Zero and the author considers a change
  unlikely. The YAML knob is renamed `zero_admin_url` accordingly.
- **Q2 (Does Dgraph reserve any low UIDs internally?):** **No.**
  Dgraph reserves nothing; even the very first UID has to be
  requested explicitly via `/admin/assign` (with a count) or
  implicitly via a mutation that contains blank nodes / new RDF
  lines. The `0x01`-as-first-UID convention (**D2** in the parent
  doc) is therefore safe on a fresh cluster.
- **Q5 (admin auth in v25 OSS):** **Treat as optional but supported.**
  All Dgraph features that used to be enterprise (ACL,
  multi-tenancy, etc.) are now open-source. Auth may or may not be
  enabled depending on operator config; the impl must accept ACL
  credentials but does not require them.
- **Q3 (cluster-ID safety guard) and Q4 (transport split):** still
  open after a reframe. See the rewritten *Open implementation
  questions* section below.

### Naming convention is TBD

The author flagged in round 3 that `Otter.` (CamelCase, type-style)
vs `otter.` (lowercase, predicate-style) as the **prefix for system
predicates and types** is *not yet decided*. Throughout this doc
any identifier of the form `otter.*` or `Otter.*` is **placeholder**
and will be revisited before PR1 lands. Where the doc says
`otter.version`, read `<system-prefix>.version`; where it says
`Otter.NamedGraph`, read `<SystemType>.NamedGraph`. Nothing else in
the design depends on the choice.

## What this doc does **not** cover

- Cross-cluster edge predicate conventions (parent doc, *Still
  open #2*).
- Query-time scoping surface (parent doc, *Still open #3*; a
  separate Layer 3.5 doc).
- Layer 4 prefix sharding (parent doc, **D6**).
- LLM enrichment workflow (parent doc, **D11**).
- Multi-tenant deployment automation (creating namespaces,
  per-tenant config). The first PR runs in a single namespace;
  multi-tenant operators run one Otter per namespace until that
  story is designed.

## Position in the Otter startup sequence

`cmd/proxy/main.go` currently runs:

```
config.LoadConfig
  -> balancer construction
    -> proxy construction
      -> HTTP / WS server start
        -> signal wait
```

The reservation step inserts between config load and balancer
construction. If the operator is in Mode A (the default), it is a
no-op. If the operator is in Mode B, it does the lease + write +
validate dance and either succeeds or aborts startup.

Concretely:

```
config.LoadConfig
  -> reservation.Bootstrap (NEW; no-op in Mode A)
    -> balancer construction
      -> proxy construction
        -> HTTP / WS server start
          -> signal wait
```

`reservation.Bootstrap` is the only entry point this layer exposes
to `main.go`. Everything else is package-internal.

## Package layout proposal

New directory `internal/reservation/`. File split chosen so each
file has a single concern and tests can mock Dgraph at a small
interface boundary.

```
internal/reservation/
  config.go        // ReservationConfig struct + validation
  bootstrap.go     // Bootstrap(): the only exported entry point
  lease.go         // /admin/assign client wrapper
  write.go         // initial 0x01 + cluster-root write
  validate.go      // subsequent-boot consistency check
  metadata.go      // schema constants, predicate names
  errors.go        // sentinel errors + categorisation for main.go
  bootstrap_test.go
  lease_test.go
  write_test.go
  validate_test.go
  testdata/
    fake_alpha.go  // httptest server that mimics /admin/assign +
                   // /mutate enough to drive the suite
```

Naming notes:

- `internal/reservation/` rather than `internal/names/` because the
  package is not a name resolver; it owns the *reservation block*
  primitive. Naming and resolution belong in a higher layer.
- The package depends on `internal/dgraph` (existing client) plus
  `net/http` for the `/admin/assign` call. It must **not** depend
  on `internal/proxy`, `internal/loadbalancer`, `internal/routing`,
  or any other request-path package; reservation is a startup-only
  concern.

## Config surface

YAML block under the existing config root. Mode A is the absence of
the `reservation:` block.

```yaml
# manifest/config.yaml
reservation:
  enabled: true                    # Mode B gate; default false
  zero_admin_url: "http://localhost:6080"
                                   # where /admin/assign lives, on
                                   # the Dgraph Zero. Distinct from
                                   # the Alpha endpoints used by the
                                   # balancer.
  extra_uids: 100                  # D3, soft cap
  graphs:
    - name: ontology
    - name: users
    - name: products
    - name: friendship
```

Constraints enforced at config-load time (in `config.go` of the
new package, called from `Bootstrap`):

- `enabled: false` (or section omitted) -> Mode A; Bootstrap is a
  no-op.
- `enabled: true` requires non-empty `graphs:` and
  `zero_admin_url:`. Otherwise refuse to start.
- Each `graphs[].name` must match `^[a-z][a-z0-9_]{0,30}$`. This is
  not arbitrary: the name becomes part of `otter.graph.<name>`
  predicates and we want to forbid characters that would force
  escaping in DQL.
- `extra_uids` defaults to 100, hard floor 1, no hard ceiling
  (per **D3**).
- Names must be unique. Duplicates are rejected at load time.

The Go struct mirrors the YAML directly, with `yaml:` and `env:`
tags consistent with the rest of `internal/config`.

## Lifecycle: Mode A (no-op)

`Bootstrap` returns `nil` immediately. No HTTP calls, no log lines
beyond a single `INFO: reservation disabled (Mode A)` for
operator visibility. The rest of Otter is unchanged.

## Lifecycle: Mode B, first boot

Sequence (single goroutine, synchronous, blocks `main.go` until
done or failure):

1. **Probe `0x01`.** Issue a read-only DQL query against the
   configured alpha admin URL (or via the existing dgo client; see
   *Open implementation questions* below):

   ```
   {
     q(func: uid(0x01)) {
       uid
       expand(_all_)
     }
   }
   ```

   Three possible outcomes:
   - **Empty result** (no node at `0x01`): proceed to step 2.
   - **Result with `otter.*` predicates already set:** this is not
     a first boot; jump to the *subsequent boot* path below.
   - **Result with non-Otter predicates:** abort with a directed
     error. Per **D4**, no migration. The error names the
     conflicting predicates and tells the operator to use a fresh
     namespace, fresh cluster, or fall back to Mode A.

2. **Lease the block.** `POST <zero_admin_url>/admin/assign?what=uid&num=N`
   where `N = 1 + extra_uids` (one for `0x01`, the rest for cluster
   roots). The endpoint lives on the Zero (round-3 Q1). Expected
   response shape (verify against current Dgraph; see *Open
   implementation questions*):

   ```json
   { "startId": "0x1", "endId": "0x65" }
   ```

   If `startId != "0x1"` (i.e. the cluster has previously assigned
   UIDs and our reservation no longer starts at `0x01`), abort
   with the same directed error as in step 1's third outcome. The
   `0x01` convention (**D2**) is non-negotiable. Round-3 Q2
   confirms this is realistic on a fresh cluster: Dgraph reserves
   no internal UIDs.

3. **Write `0x01` + cluster roots.** A single mutation, atomic from
   the caller's perspective:

   ```
   {
     "set": [
       { "uid": "0x1", "otter.version":            "<binary version>" },
       { "uid": "0x1", "otter.reservation.start":  "0x1" },
       { "uid": "0x1", "otter.reservation.count":  "100" },
       { "uid": "0x1", "otter.cluster.cid":        "<observed cid>" },
       { "uid": "0x1", "otter.namespace":          "<observed ns>" },
       { "uid": "0x1", "otter.created_at":         "<RFC3339>" },
       { "uid": "0x1", "otter.settings":           { "uid": "0x2" } },
       { "uid": "0x2", "dgraph.type":              "Otter.Settings" },

       { "uid": "0x3", "otter.cluster.name":       "ontology" },
       { "uid": "0x3", "dgraph.type":              "Otter.NamedGraph" },
       { "uid": "0x4", "otter.cluster.name":       "users" },
       { "uid": "0x4", "dgraph.type":              "Otter.NamedGraph" },
       { "uid": "0x5", "otter.cluster.name":       "products" },
       { "uid": "0x5", "dgraph.type":              "Otter.NamedGraph" },
       ...
     ]
   }
   ```

   The mutation is illustrative. The committed schema firms up in
   the first PR; the parent doc's *Still open #1* (concrete
   contents of the critical tier) is finalised here.

4. **Re-read `0x01`** and confirm the write landed. Without this,
   a partially-applied mutation (alpha crash, network loss) could
   leave Otter believing the reservation is good when only some
   roots exist. Cheap insurance.

5. **Log a success line** with the leased range, cluster CID, and
   the list of named graphs. This is the only operator-visible
   confirmation that Mode B succeeded.

If any step fails, `Bootstrap` returns a non-nil error and `main.go`
calls `log.Fatalf`. There is no retry loop in this PR; an operator
re-running Otter is the supported recovery path.

## Lifecycle: Mode B, subsequent boots

Triggered when step 1 above sees `otter.*` predicates already set.

1. **Read the recorded reservation** from `0x01` (full set of
   `otter.*` predicates) plus all `Otter.NamedGraph` nodes.
2. **Compare to YAML intent**:
   - `otter.version` ahead of binary version -> abort. Future
     reservation written by a newer Otter; refuse to operate.
   - `otter.cluster.cid` mismatch -> abort. Operator pointed Otter
     at a different cluster than last boot.
   - `otter.namespace` mismatch -> abort. Same alarm, different
     direction.
   - YAML adds, removes, or renames a graph -> abort. Per **D4**,
     no silent migration. Error message names the divergence and
     leaves resolution to the operator.
   - Everything matches -> log a `reservation validated, N graphs`
     line and return.

`Bootstrap` returns `nil` only when the recorded state and the
YAML intent fully agree, *or* the YAML adds nothing the recorded
state lacks (forward-compatible read).

## Failure modes summary

| Condition                                       | Behaviour                       |
|-------------------------------------------------|---------------------------------|
| `reservation.enabled: false` or section omitted | Mode A, Bootstrap returns nil   |
| `enabled: true`, `graphs:` empty                | abort at config validation      |
| Cluster has non-Otter data at `0x01`            | abort, name conflicting preds   |
| `/admin/assign` returns non-`0x1` start         | abort, name observed start      |
| Mutation in step 3 fails                        | abort, surface Dgraph error     |
| Re-read in step 4 missing predicates            | abort, name missing preds       |
| Subsequent boot sees newer `otter.version`      | abort                           |
| Subsequent boot sees CID mismatch               | abort                           |
| Subsequent boot sees graph diff vs YAML         | abort, name diff                |

Every abort is `log.Fatalf` from `main.go`'s perspective. The
package itself returns errors of one of a small number of sentinel
types declared in `errors.go`, so future callers (a CLI, a
liveness probe) can categorise.

## Test harness

Two tiers, mirroring the existing Otter testing strategy.

### Unit tier — `httptest`-backed fake alpha

`testdata/fake_alpha.go` exposes a struct that an in-process
`httptest.Server` can drive. Behaviour:

- `POST /admin/assign?what=uid&num=N` -> returns a configurable
  start/end pair. Default behaviour mimics a fresh cluster:
  `startId=0x1, endId=0x1+N`.
- `POST /mutate` -> validates the inbound body against an
  expectation script and records every mutation it sees. Tests
  assert on the recorded mutations after `Bootstrap` returns.
- `POST /query` -> serves canned responses keyed on a regex over
  the query string. Default behaviour returns "node `0x01` does
  not exist" so the first-boot path is the natural default.

Test cases the suite must cover (one Go test each, all with build
tag-free unit visibility):

- Mode A: Bootstrap is a no-op even with fake alpha unreachable.
- Mode B first boot, happy path: assigns, writes, re-reads, returns
  nil. Asserts on the exact set of mutations sent.
- Mode B first boot, alpha returns `startId=0x42` (not `0x1`):
  Bootstrap returns the right sentinel error.
- Mode B first boot, mutation fails: Bootstrap surfaces the error.
- Mode B subsequent boot, all matches: Bootstrap returns nil, no
  mutations sent.
- Mode B subsequent boot, CID mismatch: sentinel error.
- Mode B subsequent boot, YAML adds a graph: sentinel error
  (no migration in PR1; D4).
- Mode B with non-Otter data at `0x01`: sentinel error,
  conflict reported.

### E2E tier — real Dgraph

A new file `e2e/reservation_test.go` (build tag `e2e`) that:

1. Boots the existing docker-compose stack.
2. Drops and recreates the namespace (or recreates the cluster,
   whichever is cheaper).
3. Runs Otter with a Mode B config.
4. Reads `0x01` directly via the Dgraph HTTP endpoint and asserts
   on the metadata.
5. Restarts Otter; asserts the second boot logs validation
   success and writes nothing.

The E2E run is the only place the assumption "fresh cluster
returns `startId=0x1`" is verified end-to-end. Until that test
exists and passes, the implementation is provisional.

## Open implementation questions

Three of the original five questions are resolved (see *Round-3
resolutions* at the top). Two reshape into:

### Q3-bis. Cluster-identity safety guard

The original Q3 assumed `dgraph.cluster.id` is a stable, queryable
field. The author flagged that he is not sure this exists in
current Dgraph at all, and asked whether the field is a Dgraph
concept or an Otter-side invention. Honest answer: I was
referring to a Dgraph-side concept whose presence in v25 OSS I
cannot confirm without checking. Three options for the safety
guard:

- **(a) Use Dgraph's cluster ID if it exists.** Save it on `0x01`
  at first boot; abort on subsequent boots if it changes. Detects
  repointing reliably.
- **(b) Synthesize an Otter-side UUID at first boot.** Save it on
  `0x01`. Does **not** detect repointing to a fresh-but-different
  cluster (the new cluster has no `0x01`, so Otter sees "first
  boot" again and just generates a new UUID). Detects only
  reverting between two already-Otter-bootstrapped clusters
  &mdash; a narrow case.
- **(c) Drop the cluster-id check entirely.** Rely on
  `otter.namespace` plus operator competence. Simplest. Loses a
  safety net for the case where the operator silently repoints
  Otter at a different (unbootstrapped) cluster.

**Lean:** **(c) by default.** If PR1 confirms (a) is implementable
in v25 OSS without enterprise dependencies, promote to (a).
Option (b) provides false security and is rejected.

### Q4-bis. Transport split for the bootstrap calls

The original Q4 asked "HTTP vs gRPC for the steps" without saying
which steps. Reframed: Bootstrap makes three kinds of call.

| Step                            | Where it lives        | Transport options          |
|---------------------------------|-----------------------|----------------------------|
| `/admin/assign`                 | Dgraph **Zero**       | HTTP only                  |
| Read `0x01` (probe + validate)  | Dgraph **Alpha**      | gRPC (dgo) **or** HTTP `/query`  |
| Write `0x01` + cluster roots    | Dgraph **Alpha**      | gRPC (dgo) **or** HTTP `/mutate` |

Two viable shapes:

- **Split (preferred).** A small new HTTP client in `lease.go`
  talks to Zero for `/admin/assign`. The existing
  `internal/dgraph/Client` (gRPC) handles the read and the write
  on Alpha. ACL credentials flow through the existing client
  unchanged. New code is bounded to one function.
- **Uniform HTTP.** A single new HTTP client handles all three.
  Slightly more new code (re-implements query/mutate over HTTP),
  but removes a transport boundary inside the package.

**Lean:** **split**. Says yes to the existing client investment
and keeps the new code minimal. If round-4 review prefers
uniform HTTP, the refactor is contained to `lease.go` and the
read/write helpers.

### Q6 (new). Naming convention for system predicates and types

Flagged by the author in round 3. Throughout this doc the
`otter.*` predicate names and `Otter.*` type names are
**placeholders**. Before PR1 ships, the parent doc has to record a
decision on which of:

- `otter.<area>.<field>` (lowercase + dots), mirroring
  `dgraph.type`, `dgraph.graphql.schema`, etc.
- `Otter.<Type>` types plus bare predicate names, more
  GraphQL-ish.
- `_otter_*` underscore-leading, less collision-prone but
  visually noisy.
- A scheme the author has in mind that has not been written down
  yet.

... is the convention. The choice is not implementation-blocking
for the *design* in this doc, but it is blocking for any code in
PR1 that names predicates. Author input requested.

## Definition of done for PR1

PR1 is mergeable when:

1. `internal/reservation/` exists with the file split above.
2. The unit tier is green and covers every row of *Failure modes
   summary*.
3. The E2E tier is green against `examples/cluster/docker-compose.yml`.
4. `cmd/proxy/main.go` calls `reservation.Bootstrap` between
   config load and balancer construction.
5. `manifest/config.yaml` and `manifest/config_docker.yaml` are
   unchanged (Mode A stays default).
6. A new `manifest/config_reservation.yaml` shows Mode B for
   docs / smoke testing.
7. The README's "Later / research" section moves the
   UID-reservation bullet to "Now" or "Next" (operator's call) and
   adds a one-line note that Mode B exists behind config.
8. The parent doc records the round-4 decisions for the remaining
   open questions: Q3-bis (cluster-id safety guard), Q4-bis
   (transport split), and Q6 (naming convention).
9. Every `otter.*` / `Otter.*` placeholder in this doc and the
   code is replaced with the real names from Q6's decision.

## Out of scope for PR1, recorded for the next round

- A CLI (`otter reservation status`, `otter reservation diff`).
  Useful but not on the critical path; subsequent PR.
- A `/reservation` HTTP endpoint that returns the recorded state.
  Same reasoning.
- Migration tooling for operators who ran an early Mode B and
  want to add a graph. Per **D4**, no migration is the contract;
  this would relax it and needs its own design round.
- Multi-tenant automation (one Otter per namespace orchestrated
  centrally). The first PR works for one namespace; tenancy
  story is a separate doc.

## Next step

Review this sketch. If the package layout, config surface, or
lifecycle is wrong, fixes here are cheap; the same fixes after
PR1 ships are not. Once this doc is signed off:

1. Resolve the *Open implementation questions* against a real
   Dgraph (15-30 minutes of probing).
2. Open PR1 with the file split and the unit tier.
3. Add the E2E tier in the same PR if it lands without
   surprises; otherwise split it into PR1.5.
