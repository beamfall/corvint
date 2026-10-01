# Tasks read cost independent of receipt history

Owner-requested issue [446](https://github.com/beamfall/corvint/issues/446) amends the accepted
agent-lease contract with S12, CAL-V0-059..061. Native ticket is V1-0644; follow-ups are V1-0645
(writers still pay the complete audit, twice in `store.Mutate`), V1-0646 (the intent tree is read
several times per read) and V1-0647 (is `queue status --summary` still wanted). Base is public
c2c7fc12988d3e957d51fba66c7033b9eb53376b.

## Decisions

1. One audit per read. `queue status` and `plan preview` replayed the receipt chain several times;
   the read context now memoizes one proof and every projection reuses its intent digests.
2. Writer-retained checkpoint. After its complete settled audit a writer retains
   `<state directory>.checkpoint.json` (`taskman-audit-checkpoint/0`) best-effort by temporary file
   and rename. A read rebinds it to the named receipt, replays only the receipts after it with the
   unchanged per-receipt validators, verifies every non-request projection and the intent tree, and
   falls back to the complete audit on any error. Reads never write it (invariant 4), which is why
   it names the head before the writer's own receipt and a read always replays at least one receipt.
3. The verdict is labelled. `journalAudit` and structural consistency say `CHECKPOINT_PLUS_TAIL`,
   never `CONSISTENT`, for a resumed read. Mutations, barrier removal, reconciliation, request
   lookups and `receipt audit` keep the complete audit.
4. No `--summary` flag. The default read no longer scales with history, so a second weaker read
   mode was not added; V1-0647 keeps the question open for the owner.

Rejected: a checkpoint written by reads (violates invariant 4); a checkpoint inside the state
directory (state scans and archive export would see it, older runtimes would refuse it as a stray
file); trusting the checkpoint without rebinding to the receipt digest and the projections.

## Limits

A resumed read does not detect an altered receipt before the checkpoint sequence, a stray receipt
beyond head+1, altered or stray `requests/` and `evidence/` files the tail does not post, duplicate
request IDs against the prefix, stray files in directories it does not list, or a projection
rewritten together with its checkpoint entry.
`TestCALV0061_CheckpointLimitsStayWithFullAudit` pins each limit and shows the complete audit
refusing the same store. The inventory digest differs between the two modes.

Writers are not accelerated, so the writer-contention half of the issue is reduced only by readers
holding files for a fraction of the time. Checkpoint emission was measured only on this macOS host.

## Independent review

One independent source review found four defects, all repaired before binding: a read verb
(`pending`, `program show`) reached the checkpoint writer through the shared lease audit; lease
writers emitted outside the writer lock; the resumed read did not check the head version digest;
and it did not bind the checkpoint generation to the named receipt. Emission moved into the locked
commit path with one fixed temporary name. The review also showed that a checkpoint entry forged
together with its projection passes a resumed read. That is now a stated limit rather than a
repair, because re-deriving every entry from prefix receipts restores the cost this slice removes;
the complete audit still refuses it.

## Evidence

Live store, 1,829 receipts, 862 checkpoint entries (119 KB), macOS, Go 1.27.1, warm cache, read
through an overlay build that pointed at a scratch checkpoint so the live `.git` was not written:

| Command | origin/main | one complete audit | resumed from checkpoint |
| --- | --- | --- | --- |
| `queue status` | 4.5–4.9 s | 1.08–1.43 s | 0.24–0.25 s (journal audit about 83 ms) |
| `plan preview` | NOT_OBSERVED | 1.18–1.20 s | 0.25 s |

The remaining 0.25 s is `snapshot.Reader.Read` over the intent tree, independent of history.
The issue's 9–22 s figure under concurrent writers was not reproduced: NOT_OBSERVED.

Tests: `TestCALV0059_CheckpointCodecAndDerivation`, `TestCALV0061_CheckpointTailEqualsFullAudit`
(a counting source proves no prefix receipt is read), `TestCALV0061_CheckpointFallsBackToFullAudit`,
`TestCALV0061_CheckpointScopeAndMovement`, `TestCALV0061_CheckpointLimitsStayWithFullAudit`
(`internal/tasks/journal`) and `TestCALV0060_WritersRetainACheckpointReadsResumeFromIt`
(`internal/tasks/cli`). Repository-wide gate: NOT_RUN (scoped issue work).

Tooling at change start: `corvint 1.0.0-rc.1 (build 163)` equals tag `v1.0.0-rc.1`;
`corvint-tasks 0.0.0-tcp01-unverified+build.202` against tag `tasks-dev-20260929.2`;
`corvint-update check` reported `RELEASE_OBSERVED` with publisher identity `NOT_VERIFIED`, no update applied.

Rollback: delete the checkpoint file, or revert the reader option; no journal, intent or archive
format changed.
