# 2026-10-16 V1-1508: superseded liveness receipts elided in place (issue 982)

V1-1508 ([issue 982](https://github.com/beamfall/corvint/issues/982)) reports that heartbeat and
renew receipts are kept forever. On one real queue they were 61 % of receipts and 67 % of receipt
bytes. The owner chose option 2, "elide in place", opt-in (owner, 2026-10-16, in-session choice).
That choice is recorded as decision 0538 and as the spec amendment CAL-V0-300..274 in
`docs/specs/corvint-tasks-agent-leases-v0.md`. TCP-00 J1 is relaxed to "contiguous names and
digests; content may be elided after supersession".

Base: `ff96980fd`. Host: a shared 12-core Darwin machine with load averages between 27 and 116
during this work, mostly 45 to 70.

## Design

- **Tombstone.** `taskman-receipt/1` is the original receipt with the attempt post's `record`
  set to null, plus `receiptSha256` and `elided {path, lastHeartbeatAt, lease}`. The header,
  `pre`, `post` digests and the request afterimage are kept. A strict `taskman-receipt/0` decode
  refuses it with `UNSUPPORTED_VERSION`, so an older binary cannot misread it.
- **Restoration.** `snapshot.ReceiptRestorer` walks from receipt 1 and keeps the latest inline
  record of each leased attempt. To restore a tombstone it replaces the two liveness members in
  that record, then digest-checks the result against `pre`, `post` and `receiptSha256`. Any
  mismatch is `JOURNAL_FORKED`. These readers use it:
  - the complete journal walk;
  - archive verify;
  - the receipt binding fold;
  - external review inputs;
  - doctor, which restores from the nearest earlier retained record.

  Chain links are always computed over the original bytes. A walk resumed from a checkpoint
  treats a tombstone as an unusable checkpoint and falls back to the complete walk. Request-ID
  replay reads the retained header.
- **Writer.** `receipt compact [--dry-run]` takes the writer lock and requires a complete settled
  audit. It drops both derived checkpoints. It elides a liveness receipt only after a later
  receipt posts the same attempt, so the head receipt and each attempt's latest record are never
  elided. It keeps a superseded receipt whole when the latest whole record of its attempt is 1024
  or more receipts back, so doctor's 4096-receipt backward search always finds one. Each tombstone
  is published by fsync and rename. A closing complete audit proves the result. Heartbeat and
  renew write paths, fencing and lease expiry are unchanged.
- **Unsuperseded tombstones.** The complete walk and archive verify refuse, as `JOURNAL_FORKED`,
  a store whose walk ends with a tombstone that is still the latest post of its attempt. The
  compactor never writes one; this check catches a hand-elided latest record.

## BASE vs fixed bytes

`TestCALV0304_CompactLivenessIssueShape` builds an issue-shaped store: four attempts, six rounds of
interleaved heartbeats and renews, then two releases.

| Store | Receipts | Liveness | Receipt bytes |
| --- | --- | --- | --- |
| BASE (no compaction, identical to `ff96980fd` behaviour) | 36 | 24 (67 %) | 115,742 |
| Fixed, after `receipt compact` | 36 | 22 elided | 84,079 (-27 %) |

Two liveness receipts stay whole because each is the latest record of an attempt that was never
released. Savings per receipt grow with the attempt record. The issue's 17 KB receipts carry long
generation histories, so they should save a larger share (inference, not measured). Request files
and complete-audit cost are unchanged; both are out of scope (#967, issue 982 item 4).

## Evidence

- New tests:
  - `TestCALV0302_CompactLivenessKeepsAuditReplayAndFencing`:
    - a dry run writes nothing;
    - receipt audit and archive export/verify pass before and after compaction;
    - replays of an elided heartbeat and an elided renew return the original attempt and
      generation and write nothing;
    - wrong-generation, expired-lease and reaped-generation writes are still `FENCED`;
    - a rerun continues;
    - rollback from the pre-compaction export passes a strict decode and the audit.
  - `TestCALV0300_ElisionScopeAndTamper`:
    - only heartbeat and renew receipts are elidable;
    - a holder change is not elidable;
    - tampered digest or liveness members, or a missing predecessor, are `JOURNAL_FORKED`.
  - `TestCALV0300_UnsupersededTombstoneRefused`: a hand-elided latest record of an attempt is
    refused by the complete audit, archive export and archive verify.
  - `TestCALV0302_CompactLivenessBoundsElidedRuns`: with a run bound of 4, two of eleven superseded
    heartbeats stay whole and no tombstone lies 4 or more receipts after a whole record.
  - `TestCALV0304_CompactLivenessIssueShape`.
  - `TestCALV0302_ReceiptCompactCLI`: usage refusal, dry run, compaction, receipt audit, a doctor
    scan with no stop, replay and rerun.
- Mutations, one per fix. Each one failed the named test and was then reverted:
  - publish without waiting for supersession: the head was elided;
  - complete walk without the restorer: the audit was refused;
  - archive verify chaining tombstone bytes: `prev does not link`;
  - strict decode in replay: `UNSUPPORTED_VERSION` on replay, in both the store and CLI tests;
  - `ElideLiveness` without the liveness-only check: the holder change was elided;
  - `LiveFormats` without `taskman-receipt/1`: `TestCALV0131_*`;
  - doctor without restoration: `scan stopped`;
  - complete walk without the unsuperseded check: the audit accepted the elided latest record;
  - archive verify without it: export and verify accepted it;
  - compaction without the run bound: 11 elided, want 9.
- Focused runs (`corvint affected --base ff96980fd` selected these as direct or dependency
  units):
  - `snapshot`, `journal`, `archive`, `transaction`, `criterionbinding`, `intent`, `mutation`,
    `scale`, `importer`, `ticket` and `service` passed in full.
  - `store` and `cli` ran in shards. The only failures were load-sensitive tests that also fail on
    a BASE build at the same load, or that pass on rerun: `TestPSR*` pool sweep and dispatch
    tests, `TestCALV0089_IntegrateCheckpointRestartKeepsGrant` and `TestTQDV0010*` plugin
    timeouts.
  - `authority` ran in four shards and passed. One shard hit the 580 s wall-clock limit at load
    70 and passed on rerun. `TestGH494PreparationAdmission` failed once with a 100 ms
    `LOCK_TIMEOUT` under load and passed on rerun.
- `go vet` passed, and so did the Windows cross-vet of the six changed packages.
- Upgrade note: `taskman-receipt/1` joins `LiveFormats`. Under CAL-V0-130, moving between this
  binary and an earlier one needs one drain.

## Independent review

One Codex round (read-only) found three P2 issues; all three are fixed in this change:
- audit and verify accepted a tombstone at the head or at an attempt's latest record (fixed by the
  unsuperseded check);
- a run of more than 4096 tombstones of one attempt would stop doctor's scan (fixed by the
  1024-receipt run bound);
- the documented rollback was wrong after repeated compactions, because a later export already
  holds earlier tombstones (fixed: keep every export and overlay them oldest first, never
  replacing an original with a tombstone, in the spec, decision 0538 and the command help).

NOT_RUN:
- `make gate` and the full `go test ./...`, by lane policy;
- `cmd/corvint` and the other PATH_LITERAL doc-reader units that the affected plan selected only
  because of documentation paths;
- live-store compaction of a real queue.
