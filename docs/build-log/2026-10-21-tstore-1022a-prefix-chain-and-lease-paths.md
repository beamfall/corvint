# 2026-10-21 tstore-1022a: prefix receipt chain binding and complete-route lease paths

Lane tstore-1022a, base origin/main f29ddc0db09c01c370cd3c9bcc5022a296ae3876. Tickets V1-1663 and
V1-1568 delivered; the other fifteen assigned Tasks tickets are listed in the PR body with reasons.

## Problem

- V1-1663: when a checkpoint-resumed read or writer route bound an ON-V0-012/013 note pre-state, it
  read the prefix receipt and only checked that a previous digest was present (or absent for
  receipt 1). A prefix receipt whose previous digest named a different chain position still bound,
  while the complete audit, walking from receipt 1, decides JOURNAL_FORKED for the same store. The
  resumed and complete routes therefore disagreed.
- V1-1568: the lease paths that keep the complete audit route instead of the PSR-V0-050 resume base
  had no single stated reason each, so a reader could not tell whether a complete walk was a design
  choice or a missed optimisation.

## Decisions

- ON-V0-015 (accepted under the owner's standing auto-accept of 2026-10-11): a resumed route binds
  each prefix receipt's chain position by value. The checkpoint receipt itself binds by its digest
  (`chain.resumedSha`, taken from the read checkpoint or the writer checkpoint); a prefix receipt
  below the checkpoint binds its previous digest against the retained digest of the receipt before
  it (`Reader.priorLink`, through `snapshot.DecodeRetainedReceipt`, so an elided liveness receipt
  binds by its retained digest). A tail receipt above the checkpoint is already chain-checked by the
  tail walk. A mismatch is the existing `noteForked` refusal, so the read route falls back to the
  complete walk and the writer declines.
- ON-V0-015 amends the ON-V0-013 read bound: at most 4 prefix receipt reads per note event and 2 per
  tail ticket post (the receipt and its predecessor). TestONV0013 asserts the amended bound.
- PSR-V0-200 (accepted under the owner's standing auto-accept of 2026-10-11) records five lease paths
  that keep the complete route and why: a handoff-policy candidate, a model reading an elided path,
  a declined prefix walk, a store without change times, and a tail over 256 receipts or a changed
  token. V1-1568 is met by this rationale rather than by new resume code.

## Evidence

- `TestONV0014_UnboundNotePreStateDeclinesTheWriter/prefix-prev` and
  `TestONV0014_ReadAndWriterResumesBindPrefixState/prefix-receipt-prev` fail on the base and pass
  with the change; the store ON tests pass (internal/tasks/store, 49.8 s under host load).
- Affected tiers 0+1 (internal/tasks/{journal,store,service,ticket,authority,cli,intent,mutation,
  scale,transaction}) and the internal/tasks traps were run before push; results are in the PR body.

## NOT_RUN

- PSR-V0-200 paths 1 (handoff-policy candidate) and 2 (model reading an elided path) have no focused
  test of their own.
- Wall-time measurement of the extra predecessor read at 20k/47k receipts (V1-1662): the read count
  is deterministic, but the host load (13 to 24 on 12 cores) does not qualify a duration.
- Affected tier 2 (76 units) is left to batch CI.

## Rollback

Revert the lane's commits. Without them a resumed route accepts a prefix receipt whose previous
digest names the wrong chain position, as before; PSR-V0-200 is documentation only.
