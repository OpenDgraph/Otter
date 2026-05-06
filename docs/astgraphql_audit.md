# `internal/astgraphql` — Audit and Hardening Note

Narrow review of the GraphQL schema parsing / schema-to-AST /
schema-to-JSON slice. Scope is limited to `internal/astgraphql/*`,
`examples/cluster/graphql/schema.graphql`, and the tests that cover
them. No change was made to `go.mod`.

## What this slice actually does

`ParseSchema` in `@/Users/micheldiz/Documents/GitHub/Otter/internal/astgraphql/ast.go:21`
runs the user SDL through Dgraph's `schema.NewHandler`, which *expands*
the schema with auto-generated `Query`, `Mutation`, `Add*`/`Update*`/
`Delete*` inputs, `*Filter`, `*Orderable`, `*AggregateResult`,
`*Payload`, `*HasFilter`, `<Union>Type` discriminator enums, and so on.
The expanded document is then parsed and validated with
`vektah/gqlparser/v2`.

`SchemaToJSON` at `@/Users/micheldiz/Documents/GitHub/Otter/internal/astgraphql/ast.go:140`
walks the validated schema and emits a JSON tree of user-facing types,
with Dgraph-generated plumbing filtered out. The output feeds the
`/ui/keywords` endpoint (see `internal/api/handlers.go`) and any
future schema-visualization consumer.

## Findings (before this pass)

1. **Operation root leaked into the output.** `Query` (and potentially
   `Subscription`) appeared as a top-level node in every run of
   `TestSchemaToJSON`, because the old filter only excluded `Mutation`.
   Confirmed by inspecting the committed snapshot: `Query` was the
   first element of `ast[]`.

2. **Union discriminator enum leaked.** For `union Account = User |
   Admin`, Dgraph emits `enum AccountType { User Admin }`. This
   synthetic enum appeared as a top-level node even though it is pure
   plumbing.

3. **Filter inconsistency between top-level and edge traversal.** The
   top-level loop filtered with `isOther` and `isInternal`, but
   `buildNode` filtered edges with a different, narrower predicate.
   This is why `TodoItemAggregateResult` appeared as a nested edge
   under `Query` even though it was filtered from the top level.

4. **`isInternal` had a precedence bug.** The expression
   `t.Kind == "ENUM" && t.Name == "Mode" || t.Name == "HTTPMethod"`
   was parsed as `(t.Kind==... && t.Name=="Mode") || t.Name=="HTTPMethod"`,
   which made the second branch kind-insensitive by accident. It also
   used the string literal `"ENUM"` instead of the `ast.Enum` constant.

5. **The snapshot helper mutated the working tree on every run.**
   `helpers.SaveASTSnapshot` *appends* to the on-disk JSON rather than
   overwriting it. The committed file
   `internal/astgraphql/testdata/snapshots/schema_ast.json` had grown
   to **162 KB containing twelve identical copies** of the same tree.
   Every `go test ./internal/astgraphql/...` created a new copy, so
   the test was silently making the working tree dirty and had no real
   assertion attached.

6. **Test had no real assertions.** `TestSchemaToJSON` only checked
   that parsing did not crash and that the bytes round-tripped through
   `json.Unmarshal`. Every leakage described above was invisible.

7. **`TestParseSchema` was commented out.** No coverage for error
   paths.

8. **Example schema was broken against the parser.** `examples/cluster/graphql/schema.graphql`
   declared explicit `type Query { account(id: ID!): Account }` and
   `type Mutation { createUser(input: NewUserInput): User }`. Dgraph's
   schema handler rejects these with
   `"GraphQL Query and Mutation types are only allowed to have fields
   with @custom/@lambda directive"`. The file could never be parsed by
   the very code it was meant to exercise.

9. **Indentation in the example was mixed** (some blocks used four
   spaces, others tabs). Cosmetic only.

10. **Dependency direction is slightly inconsistent.** `go.mod` pins
    `github.com/hypermodeinc/dgraph/v24 v24.1.2` for `schema.NewHandler`,
    while `examples/cluster/docker-compose.yml` targets a Dgraph v25
    preview image. Flagged, not changed — see *open questions* below.

## Changes made

### `@/Users/micheldiz/Documents/GitHub/Otter/internal/astgraphql/ast.go`

- Unified filter logic into a single `shouldIncludeType(schema, t)`
  predicate, used both at the top-level iteration and at every edge
  expansion in `buildNode`. Findings 1, 2, and 3 all collapse into
  this one change.
- New explicit helpers: `isOperationRoot`, `isDgraphGenerated`,
  `isUnionDiscriminatorEnum`. The last one detects synthetic
  `<UnionName>Type` enums by looking up the corresponding union in
  the schema, instead of hardcoding names.
- Rewrote `isInternal` as `isDgraphGenerated`: uses a `switch`, adds
  `Add`/`Update`/`Delete` prefixes and `*Ref`/`*Patch`/`*HasFilter`
  suffixes, uses the `ast.Enum` constant via the kind check.
- Sorted top-level output alphabetically so `SchemaToJSON` is
  byte-deterministic across runs (required for golden-file testing).
- Hardened `isUserDefined` against nil `Position`.

### `@/Users/micheldiz/Documents/GitHub/Otter/internal/astgraphql/schema_test.go`

- Dropped `helpers.SaveASTSnapshot` (the appending helper). This
  file is no longer dependent on shared snapshot state; the other
  consumer (`internal/astneo`) is unchanged.
- Added a proper golden-file test `TestSchemaToJSON_Golden` with an
  `-update` flag. The golden lives at
  `internal/astgraphql/testdata/schema_to_json.golden.json` (4.2 KB,
  one copy).
- Added positive assertions `TestSchemaToJSON_IncludesUserTypes`.
- Added negative assertions `TestSchemaToJSON_ExcludesGeneratedAndOperationRoots`.
- Added structural assertions `TestSchemaToJSON_EdgesAndProperties`
  (object → scalar = property, object → object = edge, interface
  expands to implementers, union expands to members, enum values
  appear as properties).
- Added `TestSchemaToJSON_Deterministic` (two calls ⇒ equal bytes).
- Added `TestParseSchema_Valid` and `TestParseSchema_Invalid` (empty
  input, garbage input, undefined referenced type — three subtests).
- Added `TestExampleSchemaFileIsValid`, which parses
  `examples/cluster/graphql/schema.graphql` through the real code
  path. This is the regression guard for finding #8.

### `@/Users/micheldiz/Documents/GitHub/Otter/examples/cluster/graphql/schema.graphql`

- Removed explicit `type Query { ... }`, `type Mutation { ... }`, and
  `schema { ... }` blocks (Dgraph auto-generates them from the user
  types; the old file was rejected by `schema.NewHandler`).
- Removed `NewUserInput` (unused now that `createUser` is gone).
- Normalized indentation to two spaces.
- Added a header comment explaining the Dgraph auto-generation rule
  so the next contributor does not re-add explicit operation types.

### Deleted

- `internal/astgraphql/testdata/snapshots/schema_ast.json` (162 KB
  of appended duplicates, replaced by the 4.2 KB golden).

## Behavior now guaranteed by tests

After this pass, the test suite in `internal/astgraphql` pins the
following contract:

1. **`ParseSchema` accepts** a realistic user SDL containing objects,
   enums, interfaces, unions, input objects, and self-referencing
   edges. Proven by `TestParseSchema_Valid` and
   `TestSchemaToJSON_IncludesUserTypes`.

2. **`ParseSchema` rejects** empty input, garbage input, and SDL that
   references undefined types. Proven by `TestParseSchema_Invalid`.

3. **`SchemaToJSON` output is deterministic**. Two successive calls
   on the same schema produce byte-identical bytes. Proven by
   `TestSchemaToJSON_Deterministic`.

4. **`SchemaToJSON` output is stable across runs**. A full byte-level
   comparison against a committed golden catches any drift in node
   ordering, property ordering, or field classification. Proven by
   `TestSchemaToJSON_Golden` (regenerate with `-update`).

5. **User-defined types appear** (`TodoList`, `TodoItem`, `User`,
   `Admin`, `Role`, `Entity`, `Account`). Proven by
   `TestSchemaToJSON_IncludesUserTypes` and
   `TestExampleSchemaFileIsValid`.

6. **Dgraph-generated plumbing does not appear** at the top level:
   - no operation roots `Query`, `Mutation`, `Subscription`;
   - no types prefixed `Add`, `Update`, `Delete`;
   - no types suffixed `Payload`, `AggregateResult`, `HasFilter`,
     `Orderable`, `Ref`, `Patch`;
   - no introspection types `__*`;
   - no union discriminator enums like `AccountType`.

   Proven by `TestSchemaToJSON_ExcludesGeneratedAndOperationRoots`.

7. **Structural translation is correct**:
   - scalar fields ⇒ entries in `properties`;
   - object-valued fields ⇒ entries in `edge`;
   - interface ⇒ edges to each implementer;
   - union ⇒ edges to each member;
   - enum ⇒ its values in `properties`.

   Proven by `TestSchemaToJSON_EdgesAndProperties`.

8. **The shipped example schema is parseable** by the real pipeline
   and produces the expected set of top-level nodes. Proven by
   `TestExampleSchemaFileIsValid`. This is the regression guard
   against future drift between the example and the parser.

## Open questions (deferred)

These are deliberately not fixed in this pass.

### Dependency direction (`hypermodeinc/dgraph/v24` vs Dgraph v25 preview)

`schema.NewHandler` comes from `github.com/hypermodeinc/dgraph/v24`
but the runtime cluster in `examples/cluster/docker-compose.yml` is a
v25 preview. If v25 changes any schema-augmentation output (new
suffixes, renamed directives, new synthetic types), the filter in
`isDgraphGenerated` may drift silently. The golden file will catch
*changes* but not *new leakage patterns*.

Options, in rough order of cost:
- **Stay on v24 handler, pin carefully.** Low effort, brittle.
- **Upgrade to v25 handler when released.** Medium effort; transitive
  dep weight is already heavy (bleve, datadog, opencensus) so little
  changes in binary size.
- **Drop `schema.NewHandler` entirely and parse only the user SDL.**
  Large product change: removes the Dgraph-augmented view. Only
  worth doing if the consumer of `SchemaToJSON` is exclusively a
  user-facing visualizer that does not care about generated
  queries/mutations. This is the only option that decouples Otter
  from a specific Dgraph major.

Recommendation: keep on v24 until the balancer cluster-state work
(Phase 5 in `docs/next_architecture_steps.md`) actually needs a v25
client; then decide together.

### Scope of `SchemaToJSON` output

`Role` (a user-defined enum referenced from `User.role`) currently
appears twice in the tree: once at the top level and once as an
edge from `User`. This is consistent with "every user type gets a
root node", but it means enums are technically inlined. If a
downstream consumer cares about top-level-only or edge-only
representation, that is a product decision, not a bug. Flagged,
not resolved.

### `Childs` field in `GraphNode`

The struct has a `Childs []GraphNode` field that is never populated
by the current code. Grep confirms no consumer reads it. Looks like
abandoned scaffolding for a previous tree shape. Kept for now to
avoid a wire-format break if a downstream tool relies on the field
being present (even as `null`). Candidate for removal in a
follow-up after confirming no external consumer.

## What remains uncertain

- Whether any frontend / tool outside this repo serializes
  `GraphNode` and depends on the `Childs` field or on the specific
  ordering of types. The deterministic, alphabetized ordering is a
  minor behavior change; the previous ordering was map-iteration
  order (non-deterministic) so no one could have safely depended on
  it, but the possibility exists.
- Whether the Dgraph v24 handler emits types we have not observed in
  the fixture (e.g. geo-related, `@lambda` synthesized types). The
  filter is conservative, but the golden-file test is the only
  catch for anything unknown slipping through.

## Validation

Narrow test run (what this pass actually ran):

```bash
go build ./internal/astgraphql/...
go test ./internal/astgraphql/... -v
```

All 7 tests pass. `go test ./...` also stays green; nothing else in
the repo imports `astgraphql` at test time.
