# 2026-10-08: Test consolidation planner (V1-1023)

## Intent

Ticket V1-1023 (GitHub #681) asks for a read-only planner. It groups declared flow variations that
share a fixture state, screen and user into fewer focused tests, and the result is a
fail-closed, checkable table with reasons. The owner accepted `docs/specs/test-consolidation-planner-v0.md`
(TCN-V0-001..012) as written. That acceptance is recorded separately, so this change edits only
the spec's delivery status, its traceability table and its implementation notes.

## Delivered

- `internal/testplan`: decoder, abstention, reuse, duplicates, isolation, precedence order with
  cycle re-planning, `--max-steps` cut, plan JSON, table, and `check`.
- `internal/appmap/lineage.go` adds `LineageFreshness`, which evaluates one screen's lineage at a
  revision.
- `internal/testvaliditydoc` adds `ProjectBound` and `BoundPaths`. These let TCN-V0-004 recompute
  freshness against a Git revision through the shared builder rather than the worktree. The
  existing worktree discovery calls the same `bindFreshness` with its old comparison.
- `corvint test-plan consolidate|check`, with root help and command help.
- `corvint-corpus-mcp --consolidation` lists `corvint.consolidate_tests`. `corvint-mcp` is
  unchanged.

## Decisions

The decisions the requirements left open are listed under "Implementation notes" in the spec. The
material ones:

- **Bound freshness.** Bound paths are compared with the blob at the evaluated revision. A path
  with an uncommitted or untracked change reads `UNKNOWN` (`retained-bound-path-uncommitted`);
  `STALE` wins over `UNKNOWN`.
- **MCP inputs.** The MCP tool takes `input` and `plan` as JSON strings so that strict decoding
  sees the caller's exact bytes. The bound refusal leaves 4 KiB of the 1 MiB message for the
  JSON-RPC envelope.
- **`check` output.** A passing `check` prints the recomputed table. A failing one prints nothing
  and exits 1; invalid use exits 2.

## Finding

`REUSED` cannot be reached end to end from real provider documents:

- A qualified external JavaScript profile always reads freshness `UNKNOWN`
  (`retained-external-app-lifecycle-unverifiable`) and needs a full Playwright tuple.
- Go tests carry no `id`.
- An unprofiled receipt with an `id` is refused.

Reuse is therefore proven only through synthetic projections. This is retained for a follow-up
ticket.

## Evidence

Focused tests passed under `GOMAXPROCS=3 go test -p 1 -count=1`:

- `internal/testplan`, `internal/testvaliditydoc`, `internal/appmap`;
- `cmd/corvint` (the `TestTestPlan*` tests, help and core-freeze tests, then the whole package);
- `cmd/corvint-corpus-mcp`.

Every input is synthetic or the committed AMAP-V0 fixture. The requirement-to-test rows are in the
spec.

`NOT_RUN`:

- The owner-run qualification on a real labelled variation set.
- A dynamic no-network test (the package opens no socket by construction).
- `make gate` (not required for scoped issue work).
- Corvint dogfood was degraded at session start (`corvint-event-rejected:dogfood-event-deadline`).

## Rollback

Revert the change. No stored state or other wire changes. The use-case receipts repinned for
`cmd/corvint/main.go` return to their previous digests.
