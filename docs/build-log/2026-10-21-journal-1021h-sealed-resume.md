## 2026-10-21 V1-2202: prefix walk and resume on segmented stores (JSG-V0-011, slice S6a)

Lane journal-1021h, base `8def072e0`.

**Problem.** On a sealed (segmented) store, widen and reap ended `SNAPSHOT_MOVED` under sustained
appends (S5: widen 6/7, reap 3/4). Two refusals caused it. `walkPrefix` refused a segmented head, so
the `PrefixFallback` prefix walk always declined, and `Result.ResumeBase` refused a segmented head.
Every round therefore ran the complete audit, the next append moved it, and 16 rounds ended moved.

**Decisions.**
- `walkPrefix` walks a segmented journal through the segment view, as the complete walk does. It
  refuses the capture's segment error. `prefixHeld` requires the bound manifest and every segment it
  names to be the files the capture observed, and checks only the active receipts by listing.
- A resume base on a segmented store records the manifest and every bound segment with the digests
  the head and manifest bind, and treats the sealed receipts and the requests they posted as covered by
  them. The resumed audit (`held` with rehash) re-reads every segment and the manifest and compares
  size and SHA-256, so a sealed byte rewritten under restored stamps declines by bytes. The gated
  `RecheckResume` checks their stamps and change time only. A seal after the base declines, and the
  next round's audit is a new base.
- Not delivered (S6b): resuming across processes from the persisted sealed base without a complete
  preparation audit. Every command still runs one complete audit first.

**Evidence.**
- `TestJSGV0011_SealedStoreWidenAndReapResume` (1,100 receipts, 1..512 sealed, a dispatcher write
  after every audit). Before: widen 10.1 s, reap 30.5 s, each 16 audits, `SNAPSHOT_MOVED`, at host
  load 18/41/80 (1, 5 and 15 min, 12 cores). After: widen 0.98 s, reap 1.19 s, each one audit and one
  resumed write, at load 52/44/78.
- `TestJSGV0011_ResumeDeclinesRewrittenSegment`: a segment rewritten in place after the base audit
  or at `fast.lease.observe` is never committed over. The test fails before the fix, because no
  resume reaches the stage.
- `TestCALV0263_LeaseGateLoad` at 10,000 receipts, 6 writers, 60 s, 3 s think. The code was reverted
  for the before run.

| run | widen | reap | heartbeat p50 | load before/after |
| --- | --- | --- | --- | --- |
| before | 8/9 `SNAPSHOT_MOVED`, p50 12.4 s | 4/5 moved, p50 12.3 s | 111 ms | 12.8 / 10.6 |
| after | 21/21 ok, p50 4.0 s | 12/12 ok, p50 3.9 s | 134 ms | 16.4 / 15.0 |

- B2 shape (40,000 receipts, 15 writers, 90 s), after only: 138 calls with no error code. Widen 8/8,
  p50 15.4 s. Reap 5/5, p50 11.7 s. Heartbeat p50 89 ms. Load was 75 before and 41 after. Every load
  above exceeds the 12 cores, so no run qualifies for T2 or T3. B3 (the single-writer harness) does
  not exist yet: NOT_RUN.
- Regressions: `./internal/tasks/journal` passed. The resume set passed: PSR-V0-052..054, 056,
  112..116 and 150..153. So did JSG-V0-006..011, the CAL-V0-116 prefix and parity tests, and
  ON-V0-012/014. JSG-V0-001 and JSG-V0-013 (seal, migration and unseal) passed as well.
- Importers passed in full: `internal/tasks/{service,intent,mutation,authority,scale,transaction}`,
  plus the `TestTMV0008_AS07_ReadsLeaveStoreByteIdentical`, `TestCALV0131` and `TestImportDirection`
  traps. The remaining `internal/tasks/store` tests (outside the resume and segment sets), the other
  path-literal readers that `corvint affected` selected only for this build-log path, and
  `./cmd/corvint` are NOT_RUN. Two wider store runs passed 570 s and were stopped under host
  contention.

**Rollback.** Revert the commit. The change is internal, with no format change, and stores are
untouched.
