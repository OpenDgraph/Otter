# UID Reservation and Named Graphs — Discussion Doc

> **Status: DRAFT / DISCUSSION.** This document does not define shipped
> behaviour. It frames the design space for what the README calls the
> "UID-reservation / named-graph convention" so we can iterate on it
> before any code lands. Nothing here is binding.

## Where this sits in the Otter layering

The README separates four concerns. This doc is about Layer 3 only.

1. **Layer 1 &mdash; Balancing writes across alphas.** Already shipped
   in its manual form via `balancer_type: defined`. Unaffected by
   this doc.
2. **Layer 2 &mdash; Additional query languages.** Out of scope here.
   `internal/astneo/` Cypher parser exists but is not wired.
3. **Layer 3 &mdash; UID reservation as an architectural primitive.**
   *This doc.* Reserve a small block of UIDs; give each reserved UID
   the semantics of a stable, named entry point into a subgraph; let
   every other Otter feature (schema scoping, prefix sharding, query
   decorators) build on top of that primitive.
4. **Layer 4 &mdash; Predicate-prefix sharding.** Consumes Layer 3 to
   decide *which prefix* to apply. Out of scope here except where
   Layer 3 has to leave the door open for it.

Layers 1 and 4 are about physical distribution. Layers 2 and 3 are
about semantic abstraction. The open question for Otter is whether
Layer 3 can be made useful *without* forcing Layer 4 on anyone.

## Decisions recorded (round 1, 2026-04-23)

Short form of what the author locked in during the first review
round; each decision is expanded in the relevant section below.

- **D1. `0x01` is hybrid.** Critical configs sit as direct predicates
  on UID `0x01`. Less critical settings expand into a small settings
  subgraph hanging off `0x01`.
- **D2. Start UID is fixed.** Always `0x01` for Otter system state.
  Not operator-configurable.
- **D3. Reservation size is soft.** ~100 additional UIDs is the
  current intuition for the "other uses" block; not a hard cap.
  Reserving more is cheap and we may grow it over time.
- **D4. No migration path from existing data.** Otter's
  named-graph/reservation mode requires a cluster that does not
  already have user data at `0x01`. Operators who want to attach
  Otter to a populated cluster can only use **balancer-only mode**;
  the reservation feature is opt-in per deployment.
- **D5. Align with Dgraph multi-tenancy.** Dgraph's multi-tenant
  namespace mechanism went open-source; Otter piggybacks on it
  instead of inventing its own scoping. See the new
  *Multi-tenancy / namespaces* section below. This answers the
  "dedicated namespace or default" question: **use Dgraph
  namespaces.**
- **D6. Predicate-prefix "sharding" is out of scope for this doc.**
  The author clarified that Layer 4 prefixes (used by Dgraph's own
  internal GraphQL to separate the GraphQL world from DQL) are a
  distinct conversation to be opened later. Every previous
  open question that leaned on prefix semantics is deferred until
  that separate doc exists.

What is **still open** after round 1, and why, is collected at the
bottom in *Still open*.

## Decisions recorded (round 2, 2026-04-23)

Round 2 resolved the largest architectural question the first draft
left open, and replaced the abstract "root UID" framing with a
sharper mental model the author named in review. The label
`Option X` was used in the reply but the *semantic content* matched
what this doc previously tagged as `Option Y`; the tension is
resolved either way and is not worth relitigating by name.

- **D7. One Dgraph database holds every named graph.** Named graphs
  are **not** separated by Dgraph namespaces. All cluster roots
  live in the same namespace so that cross-graph edges are
  first-class. The classic example the author gave: *users*,
  *products*, *friendship*, *recommendation*, *preferences* all
  have to be able to point at each other; separating them into
  tenants defeats the point of a graph database.
- **D8. The "pergola / grape cluster" metaphor is canonical.**
  - The **pergola** (*pergolato*) is the reservation block &mdash;
    the bus of Otter-owned UIDs (`0x01` plus the reserved slots).
  - Each **grape cluster** (*cacho*) is one named graph: a whole
    subgraph with its own purpose (e.g. an ontology; a user graph;
    a products graph).
  - Each **grape** (*uva*) is a single node inside a cluster.
  - The pergola is the *entry point* of the model: navigation
    starts at `0x01` and fans out to cluster roots.
- **D9. Nodes do not carry cluster membership.** A node has no
  predicate saying "I belong to cluster N". Cluster identity is
  defined purely by the root UID you enter from; a node can be
  referenced from any number of cluster traversals. This
  retrospectively resolves the "membership convention" question
  that round 1 deferred &mdash; **there is no membership convention
  by design.**
- **D10. Cross-cluster edges are a first-class feature.** A node
  in cluster A can hold an edge into cluster B. Traversing that
  edge from B's side naturally surfaces every cluster that
  references the target. This is the main reason D7 holds: if
  clusters were in different namespaces, these edges could not
  exist.
- **D11. LLM-driven enrichment is a planned use case of Layer 3,
  not a requirement of it.** An LLM can inspect a node and
  emit new cross-cluster edges (example given: "this node looks
  like a document, so anchor it into the ontology cluster at the
  `Document` concept"). Layer 3 only has to make cluster roots
  stable and addressable; enrichment lives at a higher layer and
  is out of scope for this doc.

D7–D10 together collapse the *Option X / Option Y* question in
*Multi-tenancy / namespaces* and kill *Still open #2* from round 1
(there is nothing to pick because there is no membership concept).
The section on namespaces is rewritten accordingly below.

## What the primitive is, stated plainly

> Otter reserves a small, fixed block of UIDs at cluster bootstrap.
> The first reserved UID (`0x01`) holds Otter's own system metadata.
> The remaining reserved UIDs are named entry points into user-owned
> subgraphs. Everything else is user graph space, unchanged.

This is not a new Dgraph feature. Dgraph already assigns UIDs freely
and `0x01` is just the first UID it hands out. The novelty is the
**convention** that says "Otter owns this range; here is what each
slot means".

Two framing points the author emphasised in review:

1. **Otter reserves *everything* up front.** The reservation is not a
   per-feature lease; it is a one-time allocation at Otter bootstrap.
   Reservations can be small (the number often fits in ~100 UIDs);
   the goal is not to pre-allocate user data, it is to claim a
   stable, low address range for the gateway itself.

2. **Type sharding is already prefix-based in Dgraph.** Dgraph's own
   internal GraphQL layer writes predicates under a prefix that
   separates the GraphQL world from the DQL world. Any additional
   prefix convention Otter adds on top of that is *more* sharding in
   the same spirit, not a new mechanism. This doc treats prefixes as
   a consequence of Layer 4, not an invention of Layer 3.

## What goes at `0x01`

The author's constraint: UID `0x01` stores Otter's system state.
The *shape* of that state is hybrid (**D1**):

- The **critical, always-read** configs sit as direct predicates on
  UID `0x01`, so any introspection query is a single-hop read.
- **Less critical** settings expand into a small subgraph rooted at
  `0x01`, reached via a dedicated edge (working name:
  `otter.settings`). That subgraph can grow without bloating the
  root node.

The split between "critical" and "expanded" is empirical and will
firm up when concrete settings exist. The shape itself is decided.

Initial candidates for the **direct predicates on `0x01`** (critical
tier):

- `otter.version`: which version of Otter wrote the reservation.
  Lets future Otter binaries refuse to run against a reservation
  written by an incompatible version.
- `otter.reservation.start`: first UID owned by Otter. By **D2**
  this is always `0x01`; recorded explicitly so readers do not
  have to guess.
- `otter.reservation.count`: size of the reserved block beyond
  `0x01`. Per **D3** this is a soft number and may change over time.
- `otter.cluster.cid`: the Dgraph cluster ID observed at reservation
  time, so an accidental repoint is visible.
- `otter.namespace`: the Dgraph namespace this reservation lives in
  (see *Multi-tenancy / namespaces*).

Initial candidates for the **expanded settings subgraph** (extra
tier, hanging off `otter.settings`):

- Index of named graphs (one child per declared name), so iteration
  over names is a natural traversal rather than a wide fan-out on
  the root.
- Diagnostic fields (`otter.created_at`, `otter.last_seen_at`,
  boot counters).
- Anything discovered later that does not deserve a predicate on
  the root node.

**Naming convention for system predicates.** The doc uses
`otter.<area>.<field>` as a working convention (`otter.version`,
`otter.reservation.start`, etc.). This is **not** the Layer 4
"prefix sharding" discussed in `why.md` &mdash; it is just dot-
separated naming that matches how Dgraph itself names internal
predicates (`dgraph.type`, `dgraph.graphql.schema`, ...). Whether
Otter eventually also uses Layer 4-style prefixes on *user* predicates
is a separate conversation (**D6**) and not settled by this doc.

## Shape of a named graph (the pergola / grape model)

The author's round-2 metaphor (**D8**) is the canonical mental
model. Mapping it to concrete Otter terms:

| Metaphor            | Technical term                | What it is                                                          |
|---------------------|-------------------------------|---------------------------------------------------------------------|
| Pergola (*pergolato*) | Reservation block            | The UIDs Otter owns: `0x01` plus the reserved ∼100 (**D3**) slots. |
| Grape cluster (*cacho*) | Named graph                | A whole subgraph with a purpose and a stable root UID.              |
| Grape (*uva*)       | Node                         | A single DQL node anywhere reachable inside (or across) clusters.   |
| Vine / jump-edge    | Cross-cluster edge (**D10**) | A predicate on a node in cluster A that points at a node in cluster B. |

Three things follow from the metaphor that Layer 3 commits to:

1. **Cluster roots are stable addresses.** Reserved UIDs beyond
   `0x01` are cluster roots. Each one is the pre-agreed entry point
   for one whole graph type ("the ontology", "the user graph",
   "the products graph", ...).
2. **Nodes do not self-identify** (**D9**). A node has no
   `otter.graph` / `otter.cluster` / back-edge predicate asserting
   which cluster it belongs to. Cluster identity is a function of
   *how you reached the node*, not of any data on the node.
3. **Cross-cluster edges are expected and encouraged** (**D10**).
   The whole reason the clusters share a namespace (**D7**) is that
   a user-graph node can declare an edge into an ontology cluster,
   and entering the ontology from that target surfaces every other
   cluster that references it.

### Example reservation layout

Illustrative, not a proposed config format:

```
0x01  otter.system                           # pergola root
0x02  named graph "ontology"   (cluster root)
0x03  named graph "users"      (cluster root)
0x04  named graph "products"   (cluster root)
0x05  named graph "friendship" (cluster root)
...
0x65  last slot of the reservation block    (D3; soft ∼100)
0x66  first user data UID
```

A concrete traversal in this layout, showing D9 and D10 at work:

- Enter the *users* cluster at `0x03`; walk to a node representing
  `alice`.
- `alice` has a `profession` edge pointing into the *ontology*
  cluster at a node representing the concept `Doctor`.
- Entering the *ontology* cluster at `0x02` and walking to
  `Doctor`, traversing incoming edges, surfaces `alice` again
  plus every other user-graph (or products-graph, or friendship-
  graph) node that also anchored itself to `Doctor`.
- No node ever says "I am in the users cluster"; that is purely a
  function of entering via `0x03`.

### Forward-looking use case: LLM enrichment (informational)

**D11** records the author's intent to use LLMs to populate
cross-cluster edges automatically. Example sketch: an LLM inspects
a freshly-written node in the user graph, classifies it ("this
looks like a document"), and writes an edge from the node to the
*ontology* cluster at the `Document` concept. This is **not** part
of Layer 3 and the doc does not scope it further; it is recorded
here so future sections do not accidentally design Layer 3 in a way
that prevents it (e.g. by making cluster roots hard to discover
from Dgraph, or by requiring membership predicates that would
force the LLM to lie about where a node "belongs").

### What Layer 3 explicitly does **not** commit to

- **Predicate-level schema for cross-cluster edges.** Whether
  Otter ships a standard predicate name pattern (e.g. `link.to`,
  `anchor`, etc.) or lets users pick their own is a schema-
  convention question, not a reservation-primitive question. Out
  of scope here.
- **Query-time scoping syntax.** How a caller addresses "run this
  query rooted at the *users* cluster" belongs to a later
  Layer 3.5 doc about the query surface. Out of scope here.
- **Storage-level prefixing.** Any `g.<name>.<predicate>`-style
  rewrite of user predicates is Layer 4 (**D6**). Out of scope here.

## Bootstrap model: two explicit modes

The author clarified (**D4**) that Otter does **not** attempt to
attach its reservation layer to a cluster that already has user
data. The feature is opt-in per deployment and has two explicit
modes:

### Mode A &mdash; balancer only

This is the *existing* Otter behaviour and the default for round 1.
The operator points Otter at any Dgraph cluster, populated or not,
and uses only Layer 1 (purposeful balancing). No `0x01` write, no
reservation, no namespace coupling.

Nothing in this mode changes. It exists so users who want the
write-balancing gap filled can get it *without* buying into the
named-graph convention.

### Mode B &mdash; reservation enabled (named graphs)

This is the new mode this doc scopes. It requires:

1. **A cluster that does not already own UID `0x01` for non-Otter
    purposes.** Typically a fresh cluster (or a fresh namespace
    inside a multi-tenant cluster &mdash; see below). Dgraph allows
    using any UID, but only after it is assigned; the precondition
    is operational, not a Dgraph limitation.
2. **A YAML block declaring intent.** Illustrative shape:

   ```yaml
   # illustrative; not a committed config shape
   otter:
     reservation:
       enabled: true       # Mode B gate
       extra_uids: 100     # D3; soft, may grow
       graphs:
         - name: users
         - name: products
         - name: orders
   ```

3. **A startup sequence** that resolves the intent:
   - If no reservation is recorded under UID `0x01`, Otter calls
     Dgraph's `/admin/assign` to lease the block, writes `0x01`
     metadata and one root-UID per declared name, and is done.
   - If a reservation is already recorded and matches, Otter
     validates it and continues. This is the steady state for every
     subsequent boot.
   - If a reservation is recorded but diverges from YAML, Otter
     fails startup with a directed error. No silent migration.

4. **Nothing is ever un-reserved by Otter.** Removing a declared
   graph from YAML is a no-op; the UID keeps existing in the
   cluster. Garbage collection of abandoned named graphs is out of
   scope for this primitive.

### Starting UID and size

Per **D2**, the start UID is fixed at `0x01`. Not operator-
configurable. The implicit "Otter's system lives at `0x01`"
convention is part of the contract, not a knob.

Per **D3**, the reservation size beyond `0x01` is soft. ~100 is the
current intuition; operators can reserve more if they expect many
named graphs. Reserving more costs essentially nothing in Dgraph
because UIDs are cheap.

### What if `0x01` is already taken?

Per **D4**, Otter fails startup with a directed error telling the
operator either to (i) use a fresh namespace (see below), (ii) use
a fresh cluster, or (iii) fall back to Mode A. It does **not**
attempt migration and **not** log-and-proceed. The reservation is
a precondition, not a best-effort.

## Multi-tenancy / namespaces

Round 2 **supersedes** the round-1 position. Dgraph multi-tenancy
is still a feature Otter may eventually sit on top of, but it is
**not** what separates named graphs. Named graphs all live in the
same namespace so that cross-cluster edges (**D10**) are possible
at the storage layer.

### Why namespaces are not the named-graph mechanism

Dgraph namespaces are an isolation boundary. Each namespace has
its own UID space and reads cannot reach across. For Otter's
*tenant* story that is exactly the right semantics: one customer
should not see another customer's data. For Otter's *named-graph*
story that is exactly the wrong semantics: the whole point of
having a cluster called *ontology* next to a cluster called *users*
is that a `users` node can anchor itself at an `ontology` concept
and queries can traverse the link.

So the layering is:

| Concern                     | Mechanism                                    | Scope                   |
|-----------------------------|----------------------------------------------|-------------------------|
| Isolation between customers | Dgraph namespace (tenant)                    | Optional; above Layer 3 |
| Separation between named graphs | Otter's reservation + cluster-root UIDs  | Layer 3                 |
| Predicate-level sharding    | Layer 4 prefixes (to be designed separately) | Layer 4                 |

Concretely for Layer 3:

- **One Otter reservation = one Dgraph namespace.** Inside that
  namespace, all named graphs coexist and can cross-reference.
- **Multi-tenant deployments** get one reservation per tenant
  namespace. Each tenant has its own pergola at UID `0x01` in
  their namespace. Cross-tenant traversal is impossible by
  Dgraph's own rules, which is the correct behaviour for tenancy.
- **Single-tenant deployments** run in namespace 0 (default).
  Nothing about Layer 3 requires multi-tenancy to be enabled.
- **`otter.namespace` is still recorded on `0x01`** as a safety
  guard: an Otter instance pointed at the wrong namespace should
  notice and refuse to start.
- **Layer 4 interaction remains deferred.** Namespaces are a
  UID-prefix mechanism below the user-facing predicate layer. If
  Layer 4 later adds predicate-prefix sharding, the two mechanisms
  stack cleanly (namespace filters first, then predicate prefix).
  Nothing here needs to change for that to be possible.

The round-1 *Option X / Option Y* tension is closed: X (one
namespace per named graph) would have broken cross-cluster edges
and is therefore rejected. Y (one namespace holds many named
graphs) is the committed shape, now framed as the pergola model.

## Explicit non-goals

These are deliberately out of scope for Layer 3:

- Predicate prefixing rules (`g.11.User.name`). That is Layer 4.
- Region decorators (`@By(region)`). Layer 4 and beyond.
- Query decorators (`Query @(graph: "g11", region: "Europe")`).
  Depends on Layer 3 + Layer 4.
- Inheritance (`type X is Y`, `belongs`, `extends`). Depends on a
  schema transformer that does not exist.
- UID reservation for *user data* (e.g. "UID 0x342F32 = San
  Francisco"). The author's framing is that user data lives *outside*
  the reserved block. Predefined geographic UIDs are a separate
  feature that, if built, would use a *different* reservation.
- Cypher or any other language transpilation.
- Cross-cluster UID stability. A reservation is bound to a cluster
  CID; pointing Otter at a new Dgraph cluster means redoing the
  reservation.

## What has to be true before any code lands

After round 2, every implementation-blocking decision is made.
The remaining work is mechanical:

1. **Done in round 1:** `0x01` shape (**D1**), start UID fixed
   (**D2**), reservation size is soft (**D3**), no-migration
   contract (**D4**), namespace role recorded (**D5**; superseded
   by D7 as the named-graph separator but still valid as a tenancy
   mechanism), Layer 4 prefix discussion deferred (**D6**).
2. **Done in round 2:** one database holds every named graph
   (**D7**), pergola / grape cluster model is canonical (**D8**),
   no node-level membership (**D9**), cross-cluster edges are
   first-class (**D10**), LLM enrichment is a use case not a
   requirement (**D11**).
3. **Required regardless of decisions:** a fixture-backed test
   strategy. Every reservation path needs to be exercisable with
   `httptest` against a fake alpha so CI does not require a real
   cluster.
4. **Deferred on purpose, not implementation-blocking:** the items
   in *Still open* below.

## Relationship to existing code

Almost none. What exists today:

- `internal/parsing/dql.go:11-27` has a toy `RenderQuery(q, prefix, _)`
  that re-renders a DQL query with a prefix prepended to every
  predicate. It is not wired to any route and has one smoke test
  without assertions (`parseDQL_test.go:8-23`). If Layer 4 ever
  ships, this function is the *shape* of the transformation that
  will eventually be needed, but it would have to be rebuilt: it
  only covers `Func.Attr` and `child.Attr`, ignoring filters,
  facets, variable blocks, aggregates, and mutations.
- `internal/astgraphql/ast.go` reads Dgraph's GraphQL schema into a
  `GraphNode` tree. If the named-graph layer ever needs to reason
  about user types, this is a plausible input format &mdash; but
  only for the subset of users who already maintain a Dgraph
  GraphQL schema.
- `internal/loadbalancer/idea.md` mentions `/admin/assign` as the
  leasing call we would use. No code consumes it yet.

No package in `internal/` owns UID leasing, named-graph resolution,
or reservation metadata. A future implementation would live under a
new path, probably `internal/reservation/` or `internal/names/`;
naming to be decided with the first PR that actually builds it.

## Still open

After round 2 the open list is short and mostly informational. None
of these block the first implementation PR.

1. **Concrete contents of the critical tier on `0x01`.** The list
   in *What goes at `0x01`* is a starting set. It will firm up as
   the first PR surfaces real needs.
2. **Cross-cluster edge predicate conventions.** Whether Otter
   ships an opinionated predicate name pattern for
   cross-cluster edges (`link.to`, `anchor`, `refers_to`, ...) or
   leaves the choice entirely to users. Not a reservation-primitive
   concern; probably belongs in a separate doc about schema
   conventions on top of Layer 3.
3. **Query-time scoping surface (Layer 3.5).** How a caller asks
   "run this query rooted at the *users* cluster". Separate doc
   when it becomes relevant.
4. **Layer 4 prefix semantics.** The author wants to open this in
   its own thread (referencing `why.md`'s predicate-prefix
   sketches). Nothing in Layer 3 presupposes a specific answer.
5. **LLM enrichment workflow details.** Recorded as **D11**;
   design lives in its own doc when we get there. Layer 3 just
   has to not paint itself into a corner that would block this.

## Open status

- Layer 1 work remains the active path. Nothing in this doc gates
  on Layer 3; Layer 3 gates on Layer 1 hardening and the
  cluster-state inspector landing first (those give Otter the
  visibility it needs before it starts *writing* system state to a
  cluster).
- Round 2 answered the last implementation-blocking question.
  Next artefact for this layer is a short companion doc
  `docs/design/uid_reservation_impl.md` sketching: the package
  layout (probably `internal/reservation/`), the `/admin/assign`
  call sequence, the `0x01` write sequence, how Mode A / Mode B
  is surfaced in config, and the fixture test harness. Still no
  code until that second doc is reviewed.
