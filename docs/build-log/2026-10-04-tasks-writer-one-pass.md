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
    lease-path `scanWithReader` hook. The second review fix withdrew this digest reuse (below).
  - A change watch guards the reuse (added after review, below), and since the second review fix,
    a content check against a fresh inventory.
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

## Second review fix: content check

Codex re-reviewed 78fb8383..4ba43576 and returned CHANGES_REQUIRED with two findings. Both were
verified before fixing.

P1: the watch does not see a write through a shared writable mapping, so a reused inventory digest
could be stale. `Mutate` could then publish where the separate passes refuse `JOURNAL_FORKED`.
- A scratch probe, not kept, changed one byte of a watched file through a shared mapping and then
  read the file and called the watch's `Check`. Reads returned the new byte at once on both
  platforms.
- On this host's APFS (kqueue), the watch reported the write after `msync` (`MS_ASYNC` or
  `MS_SYNC`), but not without `msync` and not on `munmap`.
- In a Linux arm64 container (`golang:1.27.1` under colima, tmpfs, inotify), it reported none of
  the four variants. A plain `write` was reported, as `SNAPSHOT_MOVED`.
- Through the real `Mutate`, with the reuse accepted regardless of content, a request projection
  changed through the mapping after the merged audit was committed on both platforms.

Fix. Codex asked for reuse to be disabled on Linux until content validation covers such writes,
and kept on macOS only if the watch can be shown to see them. It cannot be, because macOS reports
nothing until `msync`, so reuse is restricted on both platforms by content validation:
- `scan.go` returns to its base `d4a896bd`, and the inventory is read fresh again;
- `readUnchanged` requires every file the merged audit read to be listed in that inventory with
  the same digest and size;
- only then, and only if `Canonical` answers and the watch's `Check` is clean, do the merged
  audit's records stand in for the second `Audit`. Otherwise the watch is closed and the inventory
  and the second `Audit` run again, as before.

The digests vouch for content and the watch for names, types and modes. The saving left is the
second `Audit`: the inventory's digests are no longer reused, so the earlier 8–59 ms observed
inventory is again a full read (measured below).

Residual window. A mapped write made after the inventory read a file is seen by neither check. It
is not a new acceptance, for this reason. The merged audit read the file before the inventory did,
and both returned the same bytes, so every read `Mutate` made of it predates the write. The outcome
is therefore the one the separate passes give when the same write follows their second pass, which
the pre-apply binding and the model handle as before. A write between the two reads is refused,
with the separate passes' code. The content check does not depend on which writes a filesystem
reports. `Mutate` qualifies the Git common dir against the §5.1 allowlist first. Only APFS and
tmpfs were probed; that bears on the watch's own reports, not on the content check.

Rejected:
- Disabling reuse on Linux alone. The macOS probe shows the same gap without `msync`.
- A stat-identity check. POSIX lets a mapped write update `st_mtime` at any time before the next
  `msync`, so it has the watch's gap.

P2: no test distinguished the descriptor-exhaustion retry. The previous entry recorded this as not
injected. `TestCALV0070_MutateRetriesAuditWithoutWatch` now runs `Mutate` in a child test process
under a controlled `RLIMIT_NOFILE`. Every gap below the highest open descriptor is filled with
`/dev/null`, so exactly the stated number remain free.
- Clean store, 70 receipts. A search finds the fewest free descriptors with which the merged audit
  completes: 137, on every run on both platforms. That need does not depend on read order, because
  the audit keeps every parent it read pinned until it ends.
- At the test-only `watched` stage, the limit leaves 137 free descriptors less the watch's, or none
  if the watch holds more. That is none on macOS, where the watch holds 276 (one per watched path),
  and 136 on Linux, where it holds 1. Closing the watch alone gives the audit enough.
- The merged audit must fail for want of descriptors. The new `retry: <error>` stage must find
  exactly the watch's descriptors released, and `Mutate` must complete through the fresh passes.
- Corrupt request projection. The refusing audit's need varied from 79 to 90 between runs, with
  map read order, so it gets no free descriptor while the watch is held and no limit after the
  retry. `Mutate` must refuse with the separate passes' `lookup: JOURNAL_FORKED` text, and must not
  project the request.
- No descriptor may stay open afterwards.

`TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit` maps a request projection and a ticket
before `Mutate` starts. It first confirms that the watch does not report a write through the
mapping that reads see. It then flips one byte through the mapping, without `msync`, at the
`audited` stage. `Mutate` must refuse with the separate passes' code (`JOURNAL_FORKED`,
`INTENT_DIVERGED`) after the stages `watched`, `audited`, `observed` and `fresh`. It must publish
nothing and project nothing. Both tests are in `mutation_audit_unix_test.go`.

Mutation checks, not maintained, with the source restored afterwards. Both were run on macOS and in
the Linux container:
- with `readUnchanged` forced true, the mapped request projection committed and the test failed;
- with the retry removed, both descriptor cases failed.

Tests run:
- In the Linux arm64 container, on the final source, `go vet` of `store` and `authority` passed.
  Every `CALV0070` test passed in `store`, `journal` and `safeopen`, and the probe ran in
  `authority`. The clean descriptor case logged the watch holding 1 descriptor and the audit
  needing 137.
- On macOS, the `store`, `journal`, `safeopen` and `authority` packages ran one at a time and all
  passed. `store` took 620 s at load 37–68.
- `go vet ./internal/tasks/...` passed for darwin and linux, as did `GOOS=windows go build ./...`
  and `gofmt -l`.

Descriptor observation, not changed here: B keeps one pinned directory per parent read, in
addition to the `os.Root` each parent already retains. A clean merged audit therefore needs 137 free
descriptors at 70 receipts.

Suspected, not reproduced, and outside this change's scope: lease preparation also takes its
inventory digests from its audit (`guardedLeaseInventory` with `observation.Files`, `lease_write.go`
line 156). Under the lock, it relies on the watch's `Check` and the head digest (lines 397 and
436). A mapped write in that window would be seen by neither. It is filed as V1-0775, not fixed:
lease code belongs to concurrent issue 494 work.

### Measurement after the second review fix

The pair was run again on macOS, 21:59–22:14, against the same base and with the same commands. The
one-minute load average was 34–52, falling to 15 during the last base profile. CPU per `Mutate` for
each benchmark round, after then base:

| Round | After the second fix | Base |
|---|---|---|
| 1 | 2,857 ms | 4,081 ms |
| 2 | 3,151 ms | 5,644 ms |
| 3 | 3,725 ms | 5,494 ms |

The medians are 3,151 and 5,494 ms, a 42.6% cut; paired, the cuts were 30%, 44% and 32%. Every after
run allocated 1.035 GB in 5.618 million allocations, against 0.868 GB and 5.277 million with the
digests reused. The base allocated 1.655 GB in 9.006 million.

Profile pair, CPU medians in milliseconds:

| Receipts | `Mutate`, base → after | Merged audit | Fresh inventory and content check | Base: lookup, complete audit, inventory |
|---|---|---|---|---|
| 500 | 1,796 → 1,337 | 537 | 466 | 564, 563, 391 |
| 2,000 | 4,895 → 3,751 | 1,562 | 1,488 | 1,812, 1,763, 1,186 |
| 7,000 | 14,032 → 10,733 | 4,528 | 4,624 | 4,809, 4,883, 3,731 |

A `Mutate` now saves one complete audit. The inventory is again a full read, as in the base, and
the content check adds no read. The profile's cut, 23–26%, is likely understated: the base profile
ran as load fell, and its inventory phase came out 75, 302 and 893 ms cheaper than the after tree's
same read. Inferred, not measured: at low load the saving should be about the base's
complete-audit share, roughly a third, less the watch's 5–10%.

Linux, 22:14–22:17: the same benchmark in an arm64 container (`golang:1.27.1` under colima, 6
virtual CPUs, tmpfs), with both trees streamed in. Host load was 12–25, and container load under
1.3. CPU, then wall, per `Mutate`:

| Round | After the second fix | Base |
|---|---|---|
| 1 | 2,159 ms, 1.50 s | 3,855 ms, 2.72 s |
| 2 | 3,105 ms, 2.52 s | 4,938 ms, 3.93 s |
| 3 | 2,547 ms, 1.78 s | 4,019 ms, 2.80 s |

The medians are 2,547 and 4,019 ms CPU, a 36.6% cut; paired, the cuts were 44%, 37% and 37%. After
allocated 1.000 GB in 5.679 million allocations, and the base 1.621 GB in 9.070 million. The
Linux profile was not run, nor any Linux measurement of the digest-reusing design.

## Third review

Codex approved `4ba43576..e9001205` with no P1 or P2 finding. Its one P3: the failure-mode row for
an edit before the watch's check claimed any such edit is detected. A write through a shared mapping
after the inventory read the file escapes both checks, so the row now covers only an edit the watch
or the content check detects, and points to the shared-mapping rows. Codex's medians, 42.6% on
macOS and 36.6% on Linux, replace the rounded figures.

Running CI's full doc-gates list then failed `unbounded-readers-check`, which the earlier rounds
had not run. `TestCALV0070_PinnedDirOpensMatchInRoot` (from A+B) passes the literal `".."` as a
name that `InRoot` and `InDir` must refuse, and AFP-V0-012 rule (d) reads a test literal made only
of `..` components as reaching the repository root. The package's tests read only `t.TempDir`
trees, so `internal/tasks/safeopen` is declared with an empty read scope (AFP-V0-023), as
`internal/dogfoodflow` was. CI's Landlock confinement checks that declaration.

## Limits

- A covers `store.Mutate` only. Release, policy, barrier, reconciliation, import and pool sweep
  keep their separate passes. Lease preparation already made one pass.
- B covers journal audit reads only. The inventory's fresh reads still resolve every path from `/`.
  Since the second review fix every `Mutate` takes that full inventory, so pinning its reads is
  now worth measuring (V1-0776); the earlier rejection assumed it read only files the audit had
  not.
- Cost stays proportional to receipt history. Only (C), or something like it, would bound it.

Not run:
- the writer checkpoint;
- a low-load measurement after either review fix;
- the Linux profile, and filesystems other than APFS and tmpfs (Linux ran the focused tests and
  the benchmark only);
- the live-store measurement and the issue 545 waves;
- the full affected-package plan (173 packages, scope UNKNOWN);
- `make gate` and the dogfood change evidence.

No receipt, journal, intent or archive format changed. No store was written outside temporary
fixtures.
