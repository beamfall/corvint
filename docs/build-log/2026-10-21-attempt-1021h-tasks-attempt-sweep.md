# 2026-10-21 lane attempt-1021h: Tasks attempt and store sweep

Lane attempt-1021h took the six Tasks store and attempt tickets that lane tasks-store-1021g (PR
#1448) left not started: V1-1702, V1-1653, V1-0916, V1-0910, V1-0907 and V1-1692. Base
`8def072e08e9730a6102657c887655f62993b31d`. The new requirements, ATR-V0-193 and PSR-V0-169, are
accepted under the owner's standing auto-accept (2026-10-11).

## V1-1692: rollback below the `progress` member (ATR-V0-193)

Problem: the ATR-V0-080..084 rollback paragraph inferred, without a test, that a build before PR
#1085 refuses an attempt record whose `loopEvidence` carries `progress`.

Experiment (recorded, not maintained): at the lane base a scratch test in
`internal/tasks/snapshot` encoded the same attempt record twice through `Attempt.Encode`, with and
without a `HANDOFF_EVIDENCE` `progress` member. A detached worktree at `b896dd3b3^` (`d35b0ee33`,
the merge before #1085's first commit) decoded both with its own `DecodeAttempt`:

```
with: code=MALFORMED err=MALFORMED: /priorGenerations/1/loopEvidence: closed object violated (unknown keys: progress)
without: code= err=<nil>
```

That build's `transaction.loadAttempts` decodes every attempt record on each lease write and stops at
the first error, so one such record makes every lease write refuse. Attempt records and their journal
afterimages are never rewritten, so no store-side migration exists.

Decision: ATR-V0-193 states the boundary instead of inventing a migration. A store that never
recorded a claim under `progressSignal` stays readable after a rollback once the policy key is
removed. `TestATRV0193_RollbackReadableWithoutProgressSignal` pins this: without the signal the
recorded `loopEvidence` has exactly the pre-#1085 key set. After any claim recorded under the
signal, the only supported recovery is a build at or after merge `d9aa46b9e`. The test fails when
`LoopEvidenceWith` records `progress` unconditionally. The rollback paragraph now cites this entry.

## V1-1653: non-publishing refresh recurred on every write (PSR-V0-169)

Problem, confirmed from the code: `advanceWriterCheckpoint` scheduled the CAL-V0-117 complete audit
whenever the new head was 512 or more past `FullSeq`. A refresh that publishes nothing leaves
`FullSeq` where it was, so every later writer-route write scheduled another complete audit until
the 4,096-receipt bound. Causes include intent-only divergence, pending cleanup, `SNAPSHOT_MOVED`,
a lock failure and a superseded token.

Decision: the first refresh is still due at exactly 512. Retries are due only every further 64
receipts (`writerRefreshRetry`). No state is persisted and every refresh, invalidation and binding
rule is unchanged. The 4,096 bound still sends writes to the complete route. A missed refresh is
retried 64 receipts later. CAL-V0-117's Scheduling bullet points to the amendment.

Evidence: `TestPSRV0169_NonPublishingRefreshDoesNotRecurPerWrite` drives `advanceWriterCheckpoint`
over every distance from 1 to 4,095 with `FullSeq` fixed. 57 writes schedule a refresh, none of them
consecutive. Under the old rule the test fails at distances 512 and 513. The existing CAL-V0-117
refresh tests pass. Deviation: criterion 1 asks for a measurement on a store of at least 8k
receipts. The recurrence is a count property of the scheduling rule, which the test proves for
every distance, so that timing measurement is NOT_RUN.

## Not delivered

- V1-1702 (pool release on the writer route): skipped, because it needs an owner decision. On
  2026-10-16 the owner declined CAL-V0-264 (V1-1494, `a5827b719`), which kept widen, reap and the
  pool verbs other than confirm-safe on the complete route to preserve CAL-V0-116 rewritten-prefix
  detection. Adding `POOL_RELEASE` to `writerLeaseVerb` would reverse that decision.
- V1-0916 (refresh latency inside the triggering caller): skipped. JSG-V0-010 segmented stores
  replace the refresh with the seal. Any further unsegmented-path change needs the journal and
  segment code that a peer lane holds.
- V1-0910 (stat-keyed TreeDigest cache) and V1-0907 (incremental review fold): not started. Each
  needs persisted derived state with its own binding and staleness design, which is larger than this
  lane's budget.

## Rollback

Revert this change. PSR-V0-169 changes no stored byte. ATR-V0-193 adds a test and spec text only.
