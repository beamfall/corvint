# Decision 0049 — Index Snapshot V0 is accepted; the session-start hook refreshes the snapshot

Date: 2026-09-04. Status: accepted. Authority: repository owner, verbatim instruction "I want you
to make the calls for me and keep the codex work going" (2026-09-04). Decision 0048 item 1 left the
automatic index refresh as the single blocker on the packet-5 row; this record decides it.

## What is decided

1. `docs/specs/index-snapshot-v0.md` moves from proposed to accepted intent. Its delivery stays
   experimental: the invariant 7 benchmark/format gate for the encoding is still unmet and the gob
   file is not the transport-neutral encoding.
2. `IDX-SNAP-V0-011`: `corvint index --if-stale` probes the snapshot header for the current tree
   and engine and writes nothing when it matches, emitting a `mutates:false` receipt; otherwise it
   is `index`.
3. `IDX-SNAP-V0-012`: the Claude Code plugin's `session-start` hook starts `index --if-stale` as a
   detached child after writing its own output and never waits for it. The harness event itself
   stays a read verb (invariant 4, `IDX-SNAP-V0-005`); the write happens in the one verb allowed to
   write, started by the adapter, not by Corvint. No other event starts a refresh, so a session
   pays at most one build per tree it starts on.

Why the hook and not the event: the session-start hook has a 2 s timeout and an index build on a
3,000-file repository costs about 1 s cold, so a refresh inside the event would race the timeout on
larger repositories and would put a write on a read path. A detached child costs the hook nothing
and leaves the next `file-change`, `user-prompt`, and compact `session-start` on the snapshot path
(`IDX-SNAP-V0-008`, `IDX-SNAP-V0-010`).

## What follows

Corvint's own Claude Code plugin runs from `integrations/claude-code`, so this repository dogfoods
the refresh as soon as it lands. Once that is observed in the self-observation ledger, the
snapshot-present perf corpus binds for `GPK-V0-017(a)` and `(b)` (decisions 0037 item 1 and 0048
item 1) and the packet-5 retry can be preregistered. That retry is a follow-up, not part of this
record.

## Rollback

Restore `Intent status: proposed` in the spec header, digest, `docs/specs/README.md`, and
`docs/specs/INDEX.json`; remove `IDX-SNAP-V0-011` and `IDX-SNAP-V0-012` and regenerate
`docs/specs/REQUIREMENTS.tsv`; remove this row from `docs/decisions/README.md`. Code that lands
against the two clauses is reverted with them.


## 2026-09-08 — Partial supersession of item 3 only

The owner's cross-host lifecycle repair and standing every-child cleanup requirement supersede
only item 3's detached refresh policy with amended `IDX-SNAP-V0-012`. Independent Gate A accepted
no automatic persistent refresh from hooks, bounded existing synchronous reads, and explicit owned
`index --if-stale` preparation. Items 1 and 2 remain accepted; all original dated text above is
historical evidence. See `docs/BUILD-LOG.md` for the conflict, repair and verification record.

Explicit warmup does not satisfy the original GPK-V0-017(a) automatic-refresh qualification; that
condition remains unresolved and no packet-5 promotion is claimed. A rollback of this lifecycle
repair must retain owned process lifetimes, not silently restore the superseded detached child.
