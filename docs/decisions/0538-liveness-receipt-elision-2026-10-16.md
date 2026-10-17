# Decision 0538 — Superseded liveness receipts may be elided in place

Date: 2026-10-16. Status: accepted. Authority: owner, 2026-10-16, in-session choice (relayed by the
orchestrator for ticket V1-1508).
Tickets: V1-1508 ([issue 982](https://github.com/beamfall/corvint/issues/982)). Amends the TCP-00
journal clause J1 ("retained contiguous history") as recorded in
`docs/specs/corvint-tasks-agent-leases-v0.md` (CAL-V0-300..274).

## Context

Issue 982 measured one real queue over 18 days. Heartbeat and renew receipts were 61 % of the
receipts and 67 % of receipt bytes. Each liveness receipt carries the whole attempt record, yet it
stops carrying information once a later receipt posts the same attempt. J1 required every receipt
to be retained byte for byte. A `PRUNE` receipt is refused, and deleting a receipt breaks the hash
chain (`JOURNAL_FORKED`), so the journal could only grow.

The lane offered three options. The owner chose option 2, "elide in place", opt-in.

## Decision

1. **J1 is relaxed.** It changes from "retained contiguous history" to "contiguous names and
   digests; content may be elided after supersession". Every receipt name `1..head` still exists,
   and every chain digest and head digest is computed over the original bytes. The only content
   that may be dropped is the attempt record of a superseded heartbeat or renew receipt.
2. **Scope.** Only a completed heartbeat or renew `TRANSITION` may be elided. It must post exactly
   its attempt record and its request, and differ from the previous record of that attempt only in
   `lastHeartbeatAt` and a lease with the same holder. A later receipt must post the same attempt.
   Ownership and outcome receipts are never elided: claim, release, handoff, reap, widen, submit,
   gate, complete, ticket and administrative receipts. The head receipt and the latest record of
   every attempt are never elided either, and audit and verify refuse a store where one is.
   Compaction also keeps one whole receipt at least every 1024 receipts of an attempt, so a
   bounded backward reader such as doctor can always restore a tombstone.
3. **Format.** An elided receipt is `taskman-receipt/1`. It is the original receipt with that
   attempt post's `record` set to null, plus `receiptSha256` (the digest of the original bytes)
   and `elided` (the path, `lastHeartbeatAt` and `lease`). Older binaries refuse it as
   `UNSUPPORTED_VERSION` (CAL-V0-131); they never misread it.
4. **Complete audit and archive verify gain an elision step.** Each walk from receipt 1 restores
   an elided receipt from the latest earlier record of its attempt and digest-checks the result
   against `pre`, `post` and `receiptSha256`. It then chains the original bytes. A tombstone that
   does not restore exactly is `JOURNAL_FORKED`.
5. **Opt-in only.** `corvint-tasks receipt compact [--dry-run]` is the only writer. Heartbeat and
   renew write paths, fencing, lease expiry and replay rules do not change. A store that is never
   compacted keeps exactly the bytes it had before.
6. **Rollback.** Run `archive export` before every compaction and keep each export. To roll back,
   restore each elided receipt from the export taken just before the run that elided it (overlay
   the exports' `receipts/` members oldest first, never replacing a `taskman-receipt/0` file with
   a tombstone), remove the two derived checkpoints, then run the older binary. The restored
   originals have the digests the chain already binds, so this also works after later writes.

## Consequences

- `taskman-receipt/1` joins `LiveFormats`. A binary built with this change therefore has a
  different live format set from earlier binaries. Under CAL-V0-130, moving between them needs one
  drain, as for any format addition.
- Request files, and the cost of a complete audit, are unchanged. Restoring a tombstone costs about
  as much as reading the original. Bounding audit time and request files stays with #967 and the
  later items of issue 982.
- The issue-shaped fixture has 36 receipts, 24 of them liveness. Compaction reduces receipt bytes
  from 115,742 to 84,079 (27 %). Records with long generation histories elide more bytes per
  receipt. The issue's figure (about 60 %) was not measured here.
