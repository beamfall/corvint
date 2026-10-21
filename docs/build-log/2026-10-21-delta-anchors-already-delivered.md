# V1-0583: DLT requirement anchors are already delivered on main

Spike ticket V1-0583. Decisions are delegated to the orchestrator (decision 0548).

## Finding

The ticket was delivered by commit `019ffffe9`, "Anchor delta requirements and record owner docs-only
deferral" (2026-10-01). That commit is an ancestor of base `90ab9f33e`.

**Exact-ID anchors exist for DLT-V0-001 through DLT-V0-010.**

- `internal/delta` and `cmd/corvint/delta_test.go` carry `t.Run("DLT-V0-00N …")` anchors for 001, 002
  and 004 through 010.
- DLT-V0-003 is anchored where its evidence lives, in `internal/cem/gitauth`. Examples are
  `bounded_blob_test.go:18`, `:48`, `:52` and the cancellation test.
- The evidence table in [immutable-delta-v0](../specs/immutable-delta-v0.md), around line 143, maps
  each ID to named tests.

**The docs-only acceptance stays PARTIAL.** The owner's 2026-10-01 decision, "keep docs-only as
tests-needed for now", is recorded as an accepted interim limit, with V1-0579 still open. That meets
acceptance criterion 2 without claiming a pass.

## Proposed recommendation

Close V1-0583 as delivered. Cite `019ffffe9` and the per-ID anchors as evidence.

The independent re-review of the regenerated OCM report set (acceptance criterion 3) belongs to the
next CEM that touches `internal/delta`. Doing it now would mean rerunning a historical enrollment,
which a spike has no reason to do.
