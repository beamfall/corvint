# Tasks writer: one-pass `Mutate` and pinned journal reads

Native ticket V1-0645, CAL-V0-070/S20 in `docs/specs/corvint-tasks-agent-leases-v0.md`. This entry
follows `2026-10-04-tasks-writer-history-cost.md`, which measured the baseline and stopped at a
proposed design.

## Owner decision (2026-10-04)

Implement (A), merging the lookup and complete audits with their digests reused, and (B), opening
the store's directories once per scan, now. Keep both strictly inside the existing contract, so
audit results and refusals are byte-identical to before. Defer (C), the writer checkpoint, until A
and B have measured before/after numbers. The coordinator confirmed CAL-V0-070/S20; CAL-V0-069/S19
is issue 494's and CAL-V0-071..072/S21 is issue 354's. CAL-V0-070 now records A and B, and the
checkpoint stays in S20 as a proposed design, owner decision pending, deferred 2026-10-04.

## What changed

- (A) `journal.Reader.AuditForMutation` makes `RequestIndex.Lookup`'s audit once and also retains
  the selection `Mutate` derives from its inventory: the queue, the policy, and every ticket and
  release file. The two refusals only the second `Audit` used to raise are deferred: the selection
  budget and intent divergence (an `intentOnly` projection pass over the same observation).
  - A found request replays exactly as before.
  - Physical digests are published only when nothing is deferred, so an inventory refusal still
    comes first.
  - `MutationAudit.Canonical` answers only for exactly the retained selection under `Audit`'s own
    path checks; anything else falls back to a separate `Audit`.
  - `store.Mutate` uses it, and its inventory reuses the observed digests through the existing
    lease-path `scanWithReader` hook.
- (B) The journal's native reader opens each parent directory once per audit attempt
  (`safeopen.PinDir`, the same first step `InRoot` made) and opens each file with one no-follow
  `openat` beneath it (`safeopen.InDir`). Before, each file cost three opens and three closes.

Rejected:
- Retaining the merged observation past one locked mutation. It would be a cache. CAL-V0-061 keeps
  every mutation on the complete audit, and relaxing that is (C)'s pending owner decision.
- Reusing the observation for a selection other than the retained one. `Audit`'s path and
  budget checks would then need re-deriving, so `Canonical` declines instead.
- Pinning the inventory's fresh reads as well. They are reached only for files the audit did not
  observe, so the gain is small, and `intent.ReadFile`'s identity checks would need their own
  equivalence proof.

Race limit: one observation now stands for the lookup, the digests and the canonical records.
Byte-identity is claimed for a store that only lock-holding writers change. An outside edit to
`head.json` or the intent tree during a mutation is still refused `SNAPSHOT_MOVED` by the pre-apply
binding, where the old second pass could have refused it first with another code. An outside edit
to any other journal file is seen by the next audit.

## Equivalence evidence

- `TestCALV0070_MergedMutationAuditEquivalence` (`internal/tasks/journal`) checks 24 store states
  across four flows: separate with per-file opens, separate with pinned opens, and merged with
  each. The states include tampered, missing, stray, symlinked and FIFO receipts, requests and
  tickets; capture-time races; the selection budget; and a pending receipt.
- `TestCALV0070_PinnedDirOpensMatchInRoot` (`internal/tasks/safeopen`) compares `InDir` with
  `InRoot` for 12 names after the directory's path has been replaced.
- `TestCALV0070_MutateAuditSequenceEquivalence` (`internal/tasks/store`) compares `Mutate`'s old
  and new sequences on a `Mutate`-built store over seven states. On the clean store, 197 of 197
  inventory digests were reused.
- Mutation check, not maintained, with the source restored afterwards:
  - un-deferring the selection-budget refusal failed the four budget cases of the journal test;
  - swapping the order of the two deferred refusals failed `budget-before-ticket-edited`.
- The existing `journal`, `safeopen` and `store` package tests passed: store 380 s at load
  average 18–39.

No inventory-only refusal was found reachable on a settled store. A stray state directory is
refused by the lookup capture in both flows, before any deferred refusal.

## Measurement after A and B

Instrument. The benchmark now reports `cpu-ms/op`, and the profile records `CPUPhases` beside its
wall-time `Phases`. Both are the process's user plus system CPU time from `getrusage`, on Unix only;
other platforms report wall time alone. This was added because wall time for identical runs varied
up to fourfold with concurrent agents' load. Every pair below compares this change with its base
`d4a896bd`, in a scratch worktree that has the same measurement code applied and nothing else
changed. Commands, run with `TMPDIR` resolved (V1-0753):

```sh
CORVINT_TASKS_HISTORY_PROFILE=<report.json> CORVINT_TASKS_HISTORY_SIZES=500,2000,7000 \
  GOTOOLCHAIN=local go test -count=1 -timeout 30m \
  -run TestCALV0070_WriterHistoryProfile -v ./internal/tasks/store/
GOTOOLCHAIN=local go test -count=1 -timeout 30m -run '^$' \
  -bench BenchmarkCALV0070_MutateAt2000Receipts -benchtime 5x -benchmem ./internal/tasks/store/
```

Benchmark, three alternating rounds (after, then base), one-minute host load average falling from
34 to 13. Each cell gives wall time and CPU time per `Mutate`:

| Round | After A and B | Base |
|---|---|---|
| 1 | 1.61 s, 1,945 ms | 3.75 s, 4,421 ms |
| 2 | 1.46 s, 1,793 ms | 3.33 s, 4,065 ms |
| 3 | 1.34 s, 1,688 ms | 3.41 s, 4,178 ms |

Every after run allocated 0.861 GB in 5.218 million allocations per `Mutate`, and every base run
1.655 GB in 9.006 million.

Profile pair, after then base back to back, load 11–12. The S20 table gives wall time; CPU medians
in milliseconds, base → after:

| Receipts | Lookup audit | Complete audit | Merged audit | Fresh inventory | Observed inventory | `Mutate` | Lease observed audit |
|---|---|---|---|---|---|---|---|
| 500 | 401 → 386 | 405 → 383 | 399 | 215 → 223 | 8 | 1,287 → 648 | 438 → 397 |
| 2,000 | 1,637 → 1,332 | 1,449 → 1,269 | 1,238 | 927 → 838 | 20 | 4,298 → 1,678 | 1,369 → 1,266 |
| 7,000 | 4,668 → 4,101 | 4,418 → 4,229 | 4,162 | 3,183 → 3,294 | 59 | 13,883 → 5,209 | 4,385 → 4,122 |

Most of the gain is A. The merged audit costs one audit, and the observed inventory takes 8–59 ms
instead of 0.2–3.2 s. B alone appears in the after tree's separate lookup and complete audits,
which read through pinned directories: their CPU time is 4–19% lower. The fresh inventory, which B
does not cover, is unchanged within noise. Lease preparation gains only B, so 6–9%.

Runs made earlier, 19:18–19:37 at load 41–124, are kept as evidence of load sensitivity, not as the
comparison:
- wall-only benchmark pairs: after 7.43 s and 3.90 s, base 7.54 s and 15.20 s per `Mutate`, with
  the same allocation counts as above;
- wall-only profiles: after (load 41–98) gave `Mutate` 3,176, 6,778 and 8,118 ms at 500, 2,000 and
  7,000 receipts; base (load 38–121) gave 4,137, 4,483 and 14,104 ms.

The phase-1 baseline in `2026-10-04-tasks-writer-history-cost.md` was taken at load 25–33 and is
not compared with these runs; the base was measured again instead. Raw outputs stayed in a local scratch directory and are not retained.

## Limits

- A covers `store.Mutate` only. Release, policy, barrier, reconciliation, import and pool sweep
  keep their separate passes. Lease preparation already made one pass.
- B covers journal audit reads only. The inventory's fresh reads still resolve every path from `/`.
- Cost stays proportional to receipt history. Only (C), or something like it, would bound it.

Not run:
- the writer checkpoint;
- Linux;
- the live-store measurement and the issue 545 waves;
- the full affected-package plan (173 packages, scope UNKNOWN);
- `make gate` and the dogfood change evidence.

No receipt, journal, intent or archive format changed. No store was written outside temporary
fixtures.
