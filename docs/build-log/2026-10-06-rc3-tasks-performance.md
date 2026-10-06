# 2026-10-06: rc.3 Corvint Tasks performance

## Intent

Make `corvint-tasks` measurably faster for rc.3 without changing any output, receipt, journal byte,
refusal or authority decision. The known targets were V1-0645 (writers pay a complete-history audit
per mutation), V1-0850 (the lease commit keeps `retainCheckpoint` under the writer lock) and V1-0646
(a read reads the intent tree several times). Field context is issue 637: `LOCK_TIMEOUT` on
`.git/taskman.prepare.lock` with 6 to 10 writers at a journal head near 12,500. The work reuses
decisions from #494, #545, V1-0645, V1-0781 and V1-0854. It does not touch
`internal/tasks/authority/preparation_lock.go`, the `release` / `attempt heartbeat` flags or
`internal/tasks/dispatch/`, which other lanes are editing.

## Method

Two synthetic stores were built with the opt-in `TestRC3_SyntheticStore` generator
(`CORVINT_TASKS_SYNTH_ROOT`, `..._TICKETS`, `..._RECEIPTS`). Each has 880 tickets, the size of the
live store on this date. `s3k` has about 3,050 receipts and `s12k` about 12,000. A copy of a live
store refuses relocation, so every run restores a pristine byte copy of each store to the same path.

A profiling build replaced only `cmd/corvint-tasks/main.go` and `internal/tasks/cli/clock.go` through
`go build -overlay`. It writes CPU and allocation profiles and records the writer lock's hold time
through `authority.WithLockObserver`. The shipped binary has no profiling code. The driver runs five
of each mutation (`ticket refine`, `ticket complete-manual`, `claim`, `renew`, `release`) and five of
each read (`queue status`, `receipt audit`, `ticket show`, `ticket search`, `roadmap`). It reports the
median wall time and, for mutations, the median writer-lock hold.

## Ranked hotspots (base `855077fc`, `s3k`)

1. **Mutate under the writer lock, about 4.1 to 4.4 s held per `refine` or `complete-manual`.**
   `AuditForMutation` (a full journal walk: openat, `wire.Parse`, `ticket.Decode`) took about 1.9 s.
   `observeMutation` took about 1.3 s. Its watched inventory reopened every receipt, request and
   projection through `safeopen.Root` / `InRoot`, a no-follow descent from `/` per file.
2. **Lease verbs (`claim`, `renew`, `release`), about 2.4 to 2.6 s wall.** The lock hold was only
   140 to 170 ms. The rest was the prepare-side audit, which is outside the writer lock and under
   the preparation lock owned by the issue-637 lane.
3. **Reads, about 0.3 to 0.4 s.** Each read hashed the intent tree three times: probe, body
   `LoadExpecting` and probe. Every record was opened by a per-file traversal from the store root.
4. **`receipt audit`, 3.3 s (`s3k`) and 12.8 s (`s12k`).** It is a deliberate complete-history audit.

## Change

- **W1 / W1b, CAL-V0-070 watched inventory through pinned parents.**
  - `observeMutation` reads its watched inventory through `pinnedInventory`
    (`internal/tasks/store/guarded_inventory.go`). Each receipt and request is read through one
    pinned parent directory descriptor (`safeopen.PinDir` + `InDir`, via the new
    `intent.ReadFileFromDir`), which costs one no-follow `openat` per file.
  - The stat, validation, identity checks, byte bounds and per-file digests are those of
    `ReadFileFromRoot`. No observed digest is passed in, so `readUnchanged` still compares fresh
    reads with the merged audit's reads. The V1-0775 mapped-write check is kept.
  - Any pinned failure, including a failed close, is returned as an inventory error. Mutate then
    takes the existing `fresh` path: it closes the watch, reads an ordinary inventory and runs a full
    audit. That path still owns every refusal.
  - Other inventory callers keep the ordinary reader.
- **W2, TM-V0-008 probed-tree decode.** `withStore` keeps the tree the latest probe hashed. When its
  digest is the snapshot's `IntentTree`, the body decodes those bytes through the new
  `intent.LoadTree` instead of reading the tree a third time through `LoadExpecting`.
  - `LoadTree` applies `LoadExpecting`'s pin check and refusal text.
  - `decodeTree` still re-hashes every captured file.
  - The second probe still reads the tree fresh, so a change between the body and that probe is
    still a moved snapshot and a retry.
- **W3, TM-V0-008 pinned tree directories.** `TreeDigest` phase 2 reads `tickets/` and `releases/`
  records beneath one pinned descriptor per subdirectory (`safeopen.PinSubDir`, then `InDir`).
  - `PinSubDir` is exactly `InRoot`'s intermediate traversal step.
  - Top-level files keep `readInRoot`.
  - If pinning fails, every file in that subdirectory falls back to `readInRoot`, which reproduces
    the original error.
  - Open, validation and bound errors share `readOpened`, so the error text is unchanged.

There is no wire, receipt, journal or spec change.

## Decisions

- **Toggles exist only for parity tests.** `pinnedMutationInventory`, `reuseProbedTree` and
  `pinTreeDirs` default to on. Each is a package variable, never configuration.
- **Not implemented, reported with measured potential:**
  - **Move Mutate's audit and inventory before the writer lock, as lease prepare already does
    (V1-0645).** After W1 the lock hold at `s3k` is still about 2.8 s, of which `AuditForMutation` is
    about 1.9 s. Moving both out would bring the hold to about 0.15 s, the lease level. This needs an
    owner decision on the V1-0645 audit placement and on the V1-0775 mapped-write window.
  - **Checkpoint/prefix resume for `AuditForMutation` and the lease prepare audit.** This is the
    issue-637 driver: about 1.6 s per writer at `s3k` and about 5 to 6 s at `s12k`, held under the
    prepare lock. It needs a decision on trusting a retained verified prefix, and the prepare lock
    belongs to another lane.
  - **V1-0850, `retainCheckpoint` under the writer lock.** It measured about 10 ms of a 140 to
    150 ms lease lock hold, which is dominated by the journal write (about 111 to 130 ms) and fsync
    (20 to 29 ms). The hold was flat from `s3k` to `s12k`. The gain is too small for the risk, so
    this was not changed. Both stores hold 880 tickets, so this does not show flatness in ticket
    count.
  - **Share the journal capture's intent reads with the probe**, about 30 ms per read.
  - **The installed `corvint-tasks` (`0.0.0-tcp01-unverified+build.202`) predates the read
    checkpoint.** A current build runs `queue status` on the live store in about 0.3 s, against the
    3.8 s baseline. No code change is needed: upgrading the binary gives this.
- **Found, not fixed (pre-existing output nondeterminism).** `journal.Reader.projections`
  (`internal/tasks/journal/records.go`) iterates the `canonical` map and returns the first projection
  that differs.
  - When two or more projections differ, the refusal's path changes between runs on the same bytes.
    Its code can change too: `INTENT_DIVERGED` for an `intent/` path, `JOURNAL_FORKED` otherwise.
  - It was observed on base code: on a store with two tickets written outside the journal,
    `ticket blockers` and `receipt audit` named `A.json` on some runs and `B.json` on others.
  - Fixing it changes which refusal is reported, so it is outside this byte-identical change.
  - The parity fixtures here build their stores through the journal, so each refused case has a
    single divergent path.

## Evidence

All tests ran on Darwin with `GOMAXPROCS=2 GOTOOLCHAIN=local go test -p 1 -count=1 -timeout 30m`.

### Parity

- `TestCALV0070_PinnedInventoryMutateParity` replays seven Mutate requests on a 70-receipt store,
  first with the pinned inventory and then with the ordinary one, each time from the same byte copy
  at the same path. The requests are:
  - two creates;
  - a replay, and a `REQUEST_ID_CONFLICT`;
  - an `INTENT_DIVERGED` from a ticket edit outside the journal;
  - a refusal from a stray `receipts/nested` directory;
  - a final create.

  The stages, reports and errors are deep-equal, and so is every byte of the state directory, the
  intent tree and the retained checkpoint. The first create reuses the merged audit
  (`watched,audited,observed`).
- `TestCALV0070_PinnedInventoryFailureFallsBackFresh`: an injected parent-open or parent-close
  failure completes through `watched,audited,observed,fresh`.
- `TestTMV0008_ProbedTreeReadParity`: 13 read verbs produce byte-identical envelopes with the probed
  tree and with the third pass. The stores are clean, diverged-ticket, malformed-ticket,
  unexpected-entry, queue-mismatch and receipt-pending.
- `TestTMV0008_ProbedTreeMovedSnapshotParity`: a commit between the body and the second probe still
  causes exactly one retry, then `OK`, in both modes.
- `TestTMV0008_LoadTreeParity`: `LoadTree` equals `LoadExpecting`, gives identical pin-refusal text,
  and refuses a tampered capture as `SNAPSHOT_MOVED`.
- `TestTMV0008_PinnedTreeDigestParity`: the trees are equal, and so are the errors, for the valid,
  empty, release-record, unreadable-ticket (`EACCES`) and stray-entry cases.

### Microbenchmarks (`-benchtime=20x`, loaded host)

| Benchmark | Before | After |
|---|---|---|
| `BenchmarkCALV0070_WatchedInventory` (2,000 receipts) | 0.99 to 1.58 s/op, 360k allocs | 0.32 to 0.44 s/op, 158k allocs |
| `BenchmarkTMV0008_ReadIntentTree` (880 tickets, probe + body + probe) | 114 ms, 117 MB | 81 ms, 86 MB |
| `BenchmarkTMV0008_TreeDigest` (880 tickets, one probe) | 59 ms, 26.6k allocs | 34 ms, 16.1k allocs |

### Binary medians of five runs

These are medians of five runs, comparing base `855077fc` with this change. For each store the after binary ran first and the base binary immediately after, on the same host. The host was loaded by other work, with a load average of 18 to 44. "Lock" is the median writer-lock hold.

| Store | Command | Base wall | After wall | Change | Base lock | After lock |
|---|---|---|---|---|---|---|
| `s3k` | `ticket refine` | 4.199 s | 2.811 s | -33% | 4004 ms | 2577 ms |
| `s3k` | `ticket complete-manual` | 3.966 s | 2.642 s | -33% | 3777 ms | 2512 ms |
| `s3k` | `claim` | 2.363 s | 2.247 s | -5% | 131 ms | 131 ms |
| `s3k` | `renew` | 2.228 s | 2.181 s | -2% | 116 ms | 121 ms |
| `s3k` | `release` | 2.369 s | 2.344 s | -1% | 137 ms | 140 ms |
| `s3k` | `queue status` | 0.310 s | 0.210 s | -32% | - | - |
| `s3k` | `receipt audit` | 3.037 s | 2.937 s | -3% | - | - |
| `s3k` | `ticket show` | 0.342 s | 0.206 s | -40% | - | - |
| `s3k` | `ticket search` | 0.270 s | 0.151 s | -44% | - | - |
| `s3k` | `roadmap` | 0.278 s | 0.148 s | -47% | - | - |
| `s12k` | `ticket refine` | 15.219 s | 10.184 s | -33% | 14952 ms | 9984 ms |
| `s12k` | `ticket complete-manual` | 14.752 s | 9.928 s | -33% | 14550 ms | 9741 ms |
| `s12k` | `claim` | 8.469 s | 8.922 s | +5% | 153 ms | 153 ms |
| `s12k` | `renew` | 8.686 s | 8.812 s | +1% | 114 ms | 121 ms |
| `s12k` | `release` | 8.396 s | 9.184 s | +9% | 140 ms | 168 ms |
| `s12k` | `queue status` | 0.326 s | 0.247 s | -24% | - | - |
| `s12k` | `receipt audit` | 12.588 s | 13.154 s | +4% | - | - |
| `s12k` | `ticket show` | 0.316 s | 0.234 s | -26% | - | - |
| `s12k` | `ticket search` | 0.258 s | 0.157 s | -39% | - | - |
| `s12k` | `roadmap` | 0.272 s | 0.159 s | -42% | - | - |

- `refine` and `complete-manual` hold the writer lock about one third less, at both store sizes.
  Lock hold is the issue-637 contention measure.
- The four reads that go through `withStore` are 24 to 47 % faster.
- No change targets the lease verbs or `receipt audit`. Their differences, -5 % to +9 %, are within
  this host's noise. An earlier unpaired base run on `s12k` measured `claim` 8.33 s, `renew` 7.96 s,
  `release` 8.15 s and `receipt audit` 12.77 s. Their lock holds did not move.
- An earlier W1+W2-only build on `s3k`, paired the same way, measured `refine` 2.99 s against 4.45 s,
  with a lock hold of 2,799 ms against 4,233 ms.

## NOT_RUN

- `make gate`: not run, by the standing owner preference for scoped work.
- `internal/tasks/authority` tests: not touched by this change and not run.
- The other dependents in the affected plan's 64 units (`cmd/corvint` and others) only import
  the touched packages. They were not run.
- Linux and Windows runtime: not run. Vet passed for darwin, linux and windows. On Windows,
  `PinSubDir` is the `unsupported` stub, so `TreeDigest` falls back to `readInRoot` per file.
- The 6 to 10 concurrent-writer field load from issue 637 was not reproduced.

## Rollback

Revert the commit. Alternatively, set `pinnedMutationInventory`, `reuseProbedTree` or `pinTreeDirs`
to `false` to restore any one previous read path on its own. No stored state, format or wire
contract depends on these paths.
