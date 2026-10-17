# V1-1598, V1-1601, V1-1604: doctor review fixes after #1060

A Codex review of #1060 (merged at 323aa4d82) found seven defects in the doctor findings. All seven
are fixed with regression tests. Spec: `docs/specs/corvint-tasks-doctor-v0.md` amends TQD-V0-021,
026, 027, 030, 031 and 033 under the owner's standing auto-accept of lane requirements
(2026-10-11).

## Findings and decisions

1. A FIFO `state.json` blocked the doctor in `dispatch.LoadLedger`. The doctor now reads the ledger
   through the cli's no-follow, nonblocking, regular-file-only `readBounded`, then calls the new
   `dispatch.DecodeLedger`. A FIFO is UNREADABLE with a `scan` warning (TQD-V0-031). The
   dispatcher's own `LoadLedger` is unchanged, because the dispatcher writes that file itself.
2. On darwin, a known holder pid that is not a considered `corvint-tasks` process was replaced by
   the longest-running waiter. Now a known foreign holder is a warning only. The longest-running
   opener stands in only when the platform reports no holder pid (TQD-V0-021).
3. A Claude `result` line that the CAL-V0-075 reader refuses (a duplicated or case-variant member)
   was counted as a host error. It now counts for nothing (`supervisor.ClaudeResultAdmitted`,
   TQD-V0-030).
4. A Claude `user` line with several `tool_result` blocks counted once. `ClassifyHostLine` now
   returns one event per block, in block order (TQD-V0-030).
5. A `--refresh` without `notify` configured rewrote the cache without its `notify` member. That
   erased the notified set and any recorded failure, so the next configured refresh ran the hook
   again. The member is now carried unchanged (TQD-V0-026).
6. Which 256 worker logs were read depended on directory order. Entries are now sorted by name
   before the limit applies (TQD-V0-027).
7. The scan-equivalence check compared only counts. `TestTQDV0033_ReadModesEquivalent` runs by
   default with the observation clock frozen and requires byte-identical `findings` and `scan`
   values between per-read and one-session access. The opt-in measurement uses the same assertion
   (TQD-V0-033).

## Evidence

- `TestTQDV0021_QueueLockHeld` (foreign holder case) and `TestTQDV0026_NotifyOncePerChange`
  (unconfigured refresh case) were run with their fixes reverted. Both failed.
- The FIFO case was not run with fix 1 reverted, because without the fix it blocks rather than
  fails.

## Rollback

Revert the change. No journal, cache or ledger format changes. The cache `notify` member keeps
its existing shape.
