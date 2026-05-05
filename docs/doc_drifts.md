# Doc Drifts Corrected (Prompt 4 pass)

Focused reality-alignment pass on `README.md`. No code changes in this
pass, except implicit via the earlier hardening branches. Scope kept
deliberately surgical.

## Biggest drifts fixed

1. **WebSocket safety claim was stale.**
   The previous "Current Status" section still said auth was "a single
   hardcoded token (`banana`)" and the origin policy permissive, with a
   forward reference to `docs/phase1_backlog.md` for the fix. Phase 3
   actually shipped `dev_mode`, `ws_token`, `ws_allowed_origins`, the
   fail-closed contract, and redaction. The README now describes what
   the code does today and points readers at `docs/security.md` for
   the contract.

2. **Roadmap's "Short-term" items were already done.**
   The list still had:
   - Configurable WebSocket auth token and origin allowlist
   - HTTP server timeouts and graceful shutdown
   - Build-tag gating for E2E tests
   All three are implemented on `feat/hardening-pass`. Moved them into
   a new "Implemented today" block at the top of the roadmap, and
   replaced the short-term list with the real nearest-term follow-ups
   (`slog`, 413 status, rate limiting).

3. **HTTP endpoints table was incomplete.**
   Previously only `/query` and `/mutate` were listed, even though the
   Features bullet right above claimed `/alter`, `/graphql`, `/health`,
   `/state`, `/admin/schema`, `/ui/keywords`. Cross-checked against
   `internal/routing/router.go` and expanded the table to match
   reality (added the missing rows plus `/validate/dql` and
   `/validate/schema`).

4. **Intro over-promised Cypher translation.**
   The lede claimed "Cypher-to-DQL translation (in progress)". The
   `internal/astneo` package is a stub; nothing end-to-end is wired.
   Reframed the intro so the gateway reality comes first and the
   multi-language/semantic enrichment story is flagged explicitly as a
   research direction.

5. **No real configuration reference.**
   The old "Configuration" subsection was three lines that told the
   reader to grep `internal/config/config.go`. Replaced with a table
   covering every knob the code reads today, including the new
   security and hardening options.

6. **Testing Strategy lived only in the runbook.**
   `docs/runbook.md` had the full table, but the README did not even
   mention that `go test ./...` is safe without Docker. Added a
   dedicated "Testing Strategy" section that names the tiers, the
   build tag, and links to the runbook for depth.

7. **Structural / layout drift.**
   - Two consecutive `---` separators before "WebSocket Usage".
   - "Run Locally" was sandwiched between the orphaned "Example
     WebSocket Payload" and the HTTP endpoints table, far from the
     Docker run-book it belonged next to.
   - "Example WebSocket Payload" duplicated the example inside
     "WebSocket Usage".
   - Heading levels mixed: `# Otter Design` was H1 while the rest of
     the document lived at H2, with its children at H3.
   All of the above got cleaned up: Docker + Local are now one
   "Quick Start" block, the orphan payload is gone, the duplicate
   rules are gone, and every content section is at H2.

8. **WebSocket example token was misleading.**
   The example payload hard-coded `"token": "banana"`. That value is
   a dev-only pin in `manifest/config_docker.yaml`; it is not the
   default elsewhere (where dev-mode generates an ephemeral token).
   Replaced with `"<your ws_token>"` plus explicit guidance to use the
   token printed at startup or the configured value.

## Intentionally **not** touched

- **`why.md`** — flagged in the README intro and in "Current Status"
  as a vision document. Its own opening line already says
  "Nothing below is written in stone." No edits; no claims there
  that currently contradict reality of the shipped binary.
- **`internal/loadbalancer/idea.md`** — same treatment. Research
  notes, explicitly labelled as exploratory.
- **`docs/runbook.md`**, **`docs/security.md`**, **`docs/repo_audit.md`**,
  **`docs/phase1_backlog.md`** — already aligned with the code state
  after the previous three phases.
- **Branding emojis** (`🦦` title, `🚧` status) — preserved as author
  choice. No new emojis introduced.
- **`manifest/*.yaml`** — only Phase 3 already added `dev_mode`
  and pinned `ws_token`. This pass does not touch them.

## How to verify

```bash
# Structure is self-consistent (all H2 headings after the title):
awk '/^##+ /' README.md

# No more "banana" hardcode claim, no more stale short-term items:
grep -n "banana" README.md
grep -n "- \[ \]" README.md

# Endpoints table matches the router:
grep 'HandleFunc' internal/routing/router.go
```
