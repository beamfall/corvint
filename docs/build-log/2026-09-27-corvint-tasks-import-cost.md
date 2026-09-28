## 2026-09-27 CAL-V0-018: first import carries one audit across its batches (S6)

`corvint-tasks import` no longer re-audits the store before each `IMPORT_APPLY` batch. The audit
taken before the first write is carried across batches under the held lock and session:
- The whole observation is bound once, before the first batch commits.
- Each later batch checks that `head.json` is still the head the previous batch wrote, and refuses
  `SNAPSHOT_MOVED` otherwise.
- `apply` checks each posted file against its receipt, so a ticket file changed between batches
  refuses `INTENT_DIVERGED`.
- The request-replay lookup walks the journal only when the carried inventory holds the batch's
  request file.
- After each commit the audit advances to the plan's final inventory (now returned as
  `transaction.Result.Final` from the capacity check), its head, and the posted ticket records.
`TestCALV0018_LaterBatchesCheckTheirHeadAndPosts` spoils the head, and separately one posted ticket,
before the second batch. Each case commits exactly one batch and refuses with the named code.

Profiling the first import also found `archive.SortFiles` re-encoding both elements on every
comparison. It now encodes each element once.

Measurements, first import into a fresh `ROADMAP` fixture store (`init`, then `import --file`),
on a 12-CPU host shared with other sessions:

| Export | Before | After | Load during the run |
|---|---|---|---|
| 300 items | 34 s | 30 s (carried audit only) | not recorded |
| 900 items, dependencies stripped | 148 s | 39 s | 38 to 43 before; not recorded after |
| 2,894 items, the full Beamfall export | not run | 271 s, with the CPU profiler on | 37 to 60 |

After the full import, `receipt audit` reported `CONSISTENT` at head 415 (414 batches). An
immediate reimport wrote nothing (2,894 unchanged) in 4 s.

The 271 s is under the 5-minute bound, but the load stayed well above the CPU count. The load
condition of CAL-V0-018 is therefore NOT_MET, and a run on a quiet host is still owed.

What is left is not proportional to the records written. In the full-run profile, of 208 s of CPU
samples:
- `transaction.Model` took 112 s. Its `validateInput` (74 s) decodes, canonicalizes and
  digest-checks every stored ticket on every batch, and `CheckCapacity` (36 s) re-measures the
  whole inventory.
- `apply` took 46 s, a fixed cost of about 110 ms per batch.
Both Model terms grow with the store, so the import as a whole is quadratic in the export size.
Removing them needs an incremental model input (a carried, already-validated ticket inventory and
capacity total), which widens the pure model's trust boundary. That is left as a follow-up rather
than folded into this slice.

Rollback: revert the change. Import then re-audits before each batch again, which is correct but
about 4 times slower at 900 items.
