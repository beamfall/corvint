# Reap survey reporting distinguishes child transactions

Issue 372 identified that a successful no-argument `reap` retained the survey's `NoChange` kind
and empty top-level receipt, then inherited the generic “changed nothing” warning even after one or
more per-attempt reap transactions committed.

The report now preserves those survey facts while carrying the actual receipt entry name returned
by each fresh completed child transaction, aligned with its reaped attempt and generation. The CLI
emits `reapReceipts` only when those child receipts exist and describes the receiptless survey and
completed child transaction count separately. An empty survey keeps the existing no-change warning.
Replays and failures do not manufacture completed children, a parent receipt, aggregate atomicity,
or mutation counts.

This implements the reporting clause added to `CAL-V0-011`. Focused store tests cover one and
multiple expired attempts plus an empty retry and validate every named receipt's attempt,
generation, kind, and outcome. A disposable CLI integration test covers zero, one, and multiple
expired attempts, exact receipt alignment, and the empty retry. Rollback removes the additive report field and restores the
generic warning branch; it does not require queue-state migration because lease and receipt policy
are unchanged.
