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
  - A change watch guards the reuse (added after review, below).
- (B) The journal's native reader opens each parent directory once per audit attempt
  (`safeopen.PinDir`, the same first step `InRoot` made) and opens each file with one no-follow
  `openat` beneath it (`safeopen.InDir`). Before, each file cost three opens and three closes.
  The pinned handle stays open beside each `os.Root` for the attempt, so an audit attempt now holds
  roughly 520 more descriptors at once on a store with 256 request shards. They close with the
  attempt.

## Review fix: change watch

Codex reviewed the first commit (78fb8383) and returned CHANGES_REQUIRED; a Claude review approved
with nits and confirmed the same window. Both found that the merged audit's digests and records
were reused without noticing an outside edit made after that audit. The old second pass had
refused such an edit:
- An overwritten request projection, or one replaced by a symlink, was refused `JOURNAL_FORKED` or
  `UNSUPPORTED_FILESYSTEM`. With the reuse, the stale digest was accepted and the mutation
  committed.
- An edited ticket was refused `INTENT_DIVERGED`. With the reuse, the pre-apply binding refused it
  `SNAPSHOT_MOVED`.
- An edited `reservations.json` or a stray `barrier.json` was refused `JOURNAL_FORKED`. With the
  reuse, `journalBytes` re-read them and the model refused them as malformed.

Fix. Before the merged audit reads anything, `Mutate` now registers `authority.WatchChanges`,
lease preparation's existing kqueue/inotify watch over the state directory and the intent tree.
`observeMutation` takes the inventory from the observed digests and the canonical Result from
`Canonical`, then calls the watch's `Check`. It keeps them only if both succeeded and nothing
changed. Otherwise, and whenever the watch cannot be registered, it closes the watch, takes a fresh
inventory and runs the second `Audit`, which is exactly the old sequence after the lookup. On
success the watch closes after the writer lock is released, as lease preparation's does.

On macOS the watch holds one descriptor per watched path: every file and directory in the state
directory and the intent tree, so thousands on a store with thousands of receipts. With B's pinned
parents (about 520 descriptors per audit attempt with 256 request shards) the merged audit could
meet the process limit where the separate passes, which held no watch, would not. Refusals are
therefore never taken under the watch. A refusal of the merged audit is taken again with the watch
closed, and a refusal of the reuse comes from the fresh passes. This costs a second audit, but only
on a refusal.

Rejected for the fix:
- A stat-identity check (inode, size, mtime, ctime) after the inventory. It is cheaper, but it
  depends on timestamp granularity and needs a re-read rule for racy files. The watch is the
  mechanism the lease path already relies on, and it has no timestamp assumption.
- Refusing `SNAPSHOT_MOVED` on any change. That would change refusal codes; repeating the passes
  keeps them.
- Checking again just before publication. The old sequence also had a window after its second
  pass, covered only by the pre-apply binding and the model's own validation. A later check would
  add refusals the old code never raised.

The claim is narrowed to match. Results, outcomes and replays stay byte-identical, but refusals
carry the same code, not the same text: with two or more diverged files, `projections` walks a Go
map, so the path a refusal names was already random before this change.

Rejected:
- Retaining the merged observation past one locked mutation. It would be a cache. CAL-V0-061 keeps
  every mutation on the complete audit, and relaxing that is (C)'s pending owner decision.
- Reusing the observation for a selection other than the retained one. `Audit`'s path and
  budget checks would then need re-deriving, so `Canonical` declines instead.
- Pinning the inventory's fresh reads as well. They are reached only for files the audit did not
  observe, so the gain is small, and `intent.ReadFile`'s identity checks would need their own
  equivalence proof.

Race limit: an outside edit after the merged audit and before the watch's check makes the
inventory and the second `Audit` run fresh, so it gets the old refusal code. An edit after the
check meets what an edit after the old second pass met:
- the pre-apply binding refuses a changed `head.json` or intent tree `SNAPSHOT_MOVED`;
- the model validates `barrier.json` and `reservations.json`, which are re-read;
- an edit to any other journal file is seen by the next audit.

## Equivalence evidence

- `TestCALV0070_MergedMutationAuditEquivalence` (`internal/tasks/journal`) checks 24 store states
  across four flows: separate with per-file opens, separate with pinned opens, and merged with
  each. The states include tampered, missing, stray, symlinked and FIFO receipts, requests and
  tickets; capture-time races; the selection budget; and a pending receipt.
- `TestCALV0070_PinnedDirOpensMatchInRoot` (`internal/tasks/safeopen`) compares `InDir` with
  `InRoot` for 12 names after the directory's path has been replaced.
- `TestCALV0070_MutateAuditSequenceEquivalence` (`internal/tasks/store`) compares `Mutate`'s old
  and new sequences on a `Mutate`-built store over eight states, including an edited
  `reservations.json`. The new sequence runs `Mutate`'s own `observeMutation` under a watch and
  fails if the unchanged store is observed twice. On the clean store, 197 of 197 inventory digests
  were reused.
- `TestCALV0070_MutateRefusesChangesAfterMergedAudit` (`internal/tasks/store`) injects seven outside
  changes through the real `Mutate`, using a test-only stage callback carried in its context. The
  changes are made after the merged audit or after the inventory and `Canonical`, before the check.
  - Each refusal matches the separate passes with the same change made after their lookup:
    `JOURNAL_FORKED` for an overwritten projection, `reservations.json` and a stray
    `barrier.json`; `UNSUPPORTED_FILESYSTEM` for a symlinked projection; `INTENT_DIVERGED` for an
    edited ticket.
  - Head, receipts and staging are unchanged, and the request is not projected.
  - An overwritten projection and an edited ticket made before `Mutate` starts get the separate
    passes' code; the ticket refusal comes from the fresh passes.
  - An unchanged store commits without a fresh pass.
- Mutation check, not maintained, with the source restored afterwards:
  - un-deferring the selection-budget refusal failed the four budget cases of the journal test;
  - swapping the order of the two deferred refusals failed `budget-before-ticket-edited`;
  - ignoring the watch's `Check` failed all seven injected changes: the overwritten and symlinked
    projections committed, the edited tickets were refused `SNAPSHOT_MOVED`, and the
    `reservations.json` and `barrier.json` changes failed validation;
  - returning a refusal from the reuse instead of the fresh passes failed the stage checks of
    `ticket-edited` and `ticket-edited-before`.
- The existing `journal`, `safeopen` and `store` package tests passed: store 380 s at load
  average 18–39.
- After the review fix, the `store`, `journal`, `safeopen` and `authority` packages passed at load
  22–31 (store 512 s, authority 307 s). The fix's last change, refusals taken without the watch,
  then passed the focused tests. A second `store` run at load 27–57 failed only
  `TestPSRSweepOwnerBarrier`, the host-load sensitivity already tracked for the pool-sweep tests.
  It waits two seconds for a shell step to start. In five runs each at load about 55, it failed
  once on this change and once on the base `d4a896bd`.

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

## Measurement after the review fix

The same pair was run again with the change watch in place, 20:26–20:46, at load 27–59 from
concurrent agents. No quieter window came. Wall times swung up to threefold between rounds, so only
CPU time and allocation are compared.

Benchmark, three alternating rounds; CPU per `Mutate`, then allocation:

| Round | After the fix | Base |
|---|---|---|
| 1 | 2,059 ms | 5,381 ms |
| 2 | 2,325 ms | 5,323 ms |
| 3 | 2,299 ms | 5,688 ms |

Every after run allocated 0.868 GB in 5.277 million allocations, against 0.861 GB in 5.218
million before the fix. The base allocated 1.655 GB in 9.006 million. The median cut in CPU time,
57%, is the same as before the fix.

Profile pair, CPU medians in milliseconds, base → after:

| Receipts | `Mutate` | Merged audit | Observed inventory | Watch setup and close |
|---|---|---|---|---|
| 500 | 1,888 → 960 | 594 | 14 | 69 → 73 |
| 2,000 | 5,836 → 2,503 | 1,575 | 30 | 214 → 231 |
| 7,000 | 16,610 → 6,769 | 4,979 | 84 | 703 → 714 |

The watch is the same code in both trees, so its row shows how far load inflated CPU time: up to
about twice the low-load figures above (34, 112 and 509 ms on the after tree). Measured at low load,
it costs 5–10% of the after-change `Mutate`. At this load it costs 8–11%. `Check` is a
non-blocking poll and is not timed separately.

## Limits

- A covers `store.Mutate` only. Release, policy, barrier, reconciliation, import and pool sweep
  keep their separate passes. Lease preparation already made one pass.
- B covers journal audit reads only. The inventory's fresh reads still resolve every path from `/`.
- Cost stays proportional to receipt history. Only (C), or something like it, would bound it.

Not run:
- the writer checkpoint;
- a low-load measurement after the review fix;
- descriptor exhaustion injected between the watch and the merged audit (the retry without the
  watch is covered only by the refusals it repeats);
- Linux;
- the live-store measurement and the issue 545 waves;
- the full affected-package plan (173 packages, scope UNKNOWN);
- `make gate` and the dogfood change evidence.

No receipt, journal, intent or archive format changed. No store was written outside temporary
fixtures.
