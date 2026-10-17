# V1-1618: journal retention (receipt retire)

GitHub #1055; owner priority "get 1055 done" (2026-10-17). Requirements JRT-V0-001..014 in the new
`docs/specs/corvint-tasks-journal-retention-v0.md`, agent-drafted and accepted under the owner's
standing auto-accept of lane requirements (2026-10-11). The agent leases contract is cited, not
extended.

## Choice: one retention story

Option (a), chosen: an in-place seal at anchor S, the head receipt at retire time. Receipts
1..S-1 move into a verified `archive export` the operator keeps; the live store keeps receipt S,
a `RETIRED_PREFIX` seal binding S's writer checkpoint, receipt S-1's digest (S's `prev`), the
archive digest and size, and S's afterimages. Every later audit is rooted at S.

Option (b), rejected: re-genesis into a fresh store by import. Genesis admits only INIT posts and
import writes shadow records, so every ticket and attempt would change journal identity; later
receipts could not bind anything from before the cut, and rollback would need a store swap.

Reasons for (a): it keeps every identity, keeps the receipt chain unchanged after S, makes rollback
an exact byte restore from the archive the seal names, and needs no new receipt kind.
Conservative defaults: operator-invoked only; refuses while any barrier, live attempt, held
reservation, pool occupancy, review or escalation reference, pending redo or staging slot exists;
nothing is dropped until the export verifies and its digest is in the seal; one-shot (export,
compaction and a second retire refuse on a sealed store); a marker makes an interrupted run
recoverable only to the old or the new store.

Considered and not taken: retiring at an older checkpoint instead of the head (needs a settled
state at a past sequence the folds cannot prove cheaply); dropping `evidence/`, `pinned/` or
`requests/` blobs (later receipts may name them); charging the seal to capacity.

## Measurement (JRT-V0-013)

`TestJRTV0013_RetireBoundsAuditCost`, darwin, Go 1.27.2, host load averages 59-72, live store in
a test temp dir. One attempt renewed N times, then released; complete audit timed once before and
once after retire.

| Cycles | Receipts before -> after | Live receipt bytes before -> after | Seal bytes | Archive bytes | Complete audit before -> after |
| --- | --- | --- | --- | --- | --- |
| 40 (default) | 46 -> 1 | 143,368 -> 3,461 | 9,863 | 265,728 | 652 ms -> 281 ms |
| 300 (`JRT_MEASURE_CYCLES=300`) | 306 -> 1 | 957,817 -> 3,466 | 26,508 | 1,678,336 | 573 ms -> 158 ms |

Live receipt bytes after retire are the anchor's alone; the seal (anchor afterimages) grows with
the anchor state, not the history. Under this load the complete audit time is dominated by fixed
cost and does not resolve the per-receipt slope (the 306-receipt audit was not slower than the
46-receipt one), so no audit-time extrapolation is claimed.

Extrapolation (bytes only, linear at the measured ~3.1 KB per renewal receipt): a 40,000-receipt
store of this shape holds about 125 MB of live receipts before and one anchor (about 3.5 KB) plus
the seal after. The 40,000-receipt store itself is NOT_RUN: building it through real lease verbs
costs about 0.9 s per receipt on this host (about 10 hours). Audit time at 40,000 receipts is
NOT_RUN.

## Residuals

- A handoff policy, review submission, artifact link or claim admission from before S is not
  seeded; a later receipt that names one refuses.
- No checkpoint route on a sealed store: every writer audit is the sealed walk from S, and
  `ReviewFold` refolds from S each tick. Both are bounded by the post-anchor journal.
- One-shot: a second retire needs unretire first; export, compaction and import refuse on a sealed
  store; CLI `archive restore` stays unimplemented (TCP-02).
- A consistently forged seal is detectable only against the archive. The archive-path check does
  not resolve symbolic links.
- Non-terminal tickets without review or escalation references are carried by value in the seal;
  the owner may prefer a stricter all-terminal rule.
- The REDO_PENDING refusal has no direct test.
