## 2026-09-27 Corvint Tasks agent leases S2: cutover is one AUTHORITY_SWITCH receipt, not a record rewrite

S2 of `docs/specs/corvint-tasks-agent-leases-v0.md` (CAL-V0-004 to 006) was accepted as a batched
rewrite of every `IMPORT` record into a `NATIVE` record, under a `CUTOVER` barrier held across the
batches. Building it showed that TCP-00 §5.4 step A5 already defines cutover differently: one
`AUTHORITY_SWITCH` receipt posts `queue.json` with `canonicalWriter` `NATIVE` and is "the single
publication boundary for every shadow record". The in-tree eligibility rule already follows it:
`CUTOVER_MISSING` holds only while the writer is not `NATIVE`, so imported records become eligible
the moment the switch commits, with no record written.

Decision. The requirements were amended to the A5 design before merge; the owner is asked to accept
the amendment with the S2 pull request.

- `corvint-tasks cutover --decision <ref>` commits one `AUTHORITY_SWITCH` stage under an `OWNER`
  binding. The decision reference is the receipt's `requestId`, so the same reference replays and a
  different one is refused `BLOCKED` once the queue is `NATIVE`. The post sets a `CUTOVER` write
  barrier on the old source, which only the old writer reads; native writes are unaffected.
- A stand-alone switch under any barrier is refused `PAUSED`, as A5 requires. The cutover-driven
  A1 to A4 sequence (pause, quiescence, archive, import under the barrier) is not built: the
  operator imports first, then switches. Quiescence holds trivially because the writer still
  requires an empty reservation set.
- The stage descriptor limit for the new operation is 5 artifacts and 1474 bytes, measured on the
  widest descriptor. The writer flow shared with `policy update` was extracted into one helper
  rather than copied.

What the rewrite design would have cost and why it was set aside: every record rewritten through
the per-batch audit is the same quadratic cost measured for the first import (about 30 minutes for
2,894 records under load), and the records would lose their `IMPORT` provenance to a revision
chain. The switch is O(1), keeps every record byte-identical, and `import` after the switch was
already refused, which is what CAL-V0-005 needed.

Evidence: `TestCALV0004_*` and `TestCALV0005_*` in `internal/tasks/store` and `internal/tasks/cli`,
and the stage codec maxima in `internal/tasks/snapshot`. Rollback: remove the `cutover` verb,
`internal/tasks/store/cutover.go` and the `AUTHORITY_SWITCH` stage operation; a queue already
switched stays `NATIVE`, and the receipt kind predates this change, so its receipts stay decodable.
