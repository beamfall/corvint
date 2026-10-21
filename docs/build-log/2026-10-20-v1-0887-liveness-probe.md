# V1-0887 liveness next-name probe (issue 642)

Date: 2026-10-20. Ticket: V1-0887 (P1, GitHub issue 642). Base:
`2b0cfce94ae2cea248c3ccab4b301af99af572d6`. Spec: `docs/specs/corvint-tasks-agent-leases-v0.md`
(`CAL-V0-224` amended in place, `CAL-V0-232` text updated; no new requirement ID, because the
leases spec is at its obligation bound). Authority: decision 0547 row V1-0887 (owner-delegated):
amend the `CAL-V0-224` non-goal to allow a next-name probe plus checkpoint-decode reuse; target
50 ms or less, flat from 3,000 to 13,000 receipts; if missed, record the shortfall and the
sharded-layout escalation instead of claiming it; lease, generation fencing and refusal semantics
unchanged.

Outcome: PARTIAL. Receipt work in a heartbeat or renew is now flat by construction (no listing of
`receipts/`, k + 1 receipt reads at any history size), and wall time is flat within host-load noise
from 3,000 to 13,000 receipts, but it is about 170 ms, not 50 ms.

## Change

- `journal.Reader.AuditForLiveness` runs the `CAL-V0-116` writer audit with `probeReceipts` set
  (only where the platform has a change time). Each capture stats `receipts/` through the native
  no-follow handle instead of listing it; the second capture moves the attempt unless the
  directory's mode, size, modification time and change time are unchanged; the walk reads the
  checkpoint's receipt and the tail by name and declines when the name after head exists.
- `store.observeWriter` takes a `liveness` flag; `leaseWriter` passes
  `transaction.LivenessLease(request)`, `mutateWriter` passes false. Resume audits are unchanged.
- Checkpoint-decode reuse, also allowed by decision 0547, is not implemented: `readCheckpoint`
  measured 1.1 ms at both sizes below.

Failure mode accepted with the probe (recorded in `CAL-V0-224`): a missing history name or a stray,
extra or temporary name that the walk does not read is not observed by a liveness write, which can
commit where the complete route refuses `MALFORMED`. The next non-liveness write lists and declines
to the refusing complete route; the `CAL-V0-117` scheduled complete audit (every 512 receipts)
refuses and removes the checkpoint, so the next write refuses.

## Tests and mutations

- `TestCALV0224_LivenessProbeNeverListsReceipts` (`internal/tasks/journal`, history 4 and 48,
  tail 3). Mutations, each run then restored: listing in the probe branch (fails: 2 listings);
  dropping the next-name probe (fails: pending receipt accepted); dropping the second capture's
  stamp comparison (fails: a same-size rename under a restored modification time keeps the first
  attempt). A first version of the last case (adding a name) survived that mutation because the
  inventory digest already binds the directory size; the rename changes only the change time.
- `TestCALV0224_LivenessWritesProbeReceipts` (`internal/tasks/store`): heartbeat and renew commit
  fast past a temporary name below head; widen leaves the writer route. Mutation: threading `false`
  instead of `transaction.LivenessLease` fails with `fast.declined ... journal is not plainly
  settled` and a `MALFORMED` refusal.

## Measurement

Command (in-process, synthetic store, 880 tickets, 5 reps each, run through
`/private/tmp/cem10-build/heavy.sh`): the store test binary with
`CORVINT_TASKS_WRITER_HOLD_PROFILE=<out> CORVINT_TASKS_HISTORY_SIZES=<n>
CORVINT_TASKS_HISTORY_TICKETS=880 CORVINT_TASKS_WRITER_HOLD_REPS=5`, `-test.run
'TestCALV0118_WriterHoldProfile$'`. Darwin, 12 CPUs, Go 1.27.3. Medians of 5, wall ms; observe is
the `mutate.fast.observe` phase. Load is the one-minute average before and after.

| Build | Receipts | Heartbeat | Renew | Observe (hb/renew) | Load |
|---|---|---|---|---|---|
| base | 3,000 | 164.3 | 168.8 | 61.0 / 58.2 | 40.8 to 47.7 |
| base | 13,000 | 166.9 | 170.4 | 67.4 / 63.8 | 86.8 to 69.2 |
| probe | 3,000 | 200.8 | 239.3 | 65.8 / 63.3 | 86.4 to 76.5 |
| probe | 3,000 (repeat) | 195.9 | 188.3 | 61.7 / 59.9 | 46.1 to 46.8 |
| probe | 13,000 | 169.7 | 170.5 | 59.4 / 52.9 | 54.0 to 48.8 |

At 13,000 receipts the observe phase fell by about 8 to 11 ms (the listing the probe removes). The
3,000-receipt probe rows are higher in the model and commit phases (validation 41 to 47 ms against
32 ms), which the probe does not touch; that is host noise at load 45 to 90, not a regression the
change can cause. Base CPU profile (3,000 receipts): about half in `openat`, 31% in capture and
intent reads, 20% in the `CAL-V0-189` intent `TreeDigest`, about 6.6% in the receipts listing.

## Shortfall and escalation

The 50 ms target is missed by about 120 ms. What remains is proportional to tickets, not receipts:
the intent tree read in the first capture and digested again before effects (`CAL-V0-189`),
capacity measurement, the transaction model, and about 30 ms of commit and fsync. A sharded
`receipts/` layout, the escalation decision 0547 names, removes only receipt-proportional work,
which the probe already removed for liveness writes, so it would not reach 50 ms either. Reaching
it needs the ticket-proportional reads reduced (V1-0915) or liveness records outside the audited
journal, which V1-0887 excludes without a further decision.
