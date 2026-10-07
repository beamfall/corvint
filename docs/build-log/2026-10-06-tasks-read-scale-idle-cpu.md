# 2026-10-06: corvint-tasks read scale and idle CPU (V1-0893, V1-0894)

## Intent

Native tickets V1-0893 (reads, claim selection and receipt audit stay fast at 10,000 tickets and
100,000 receipts) and V1-0894 (idle CPU of long-running processes), both citing GitHub
[beamfall/corvint#641](https://github.com/beamfall/corvint/issues/641); neither has its own issue.
Owner goals of 2026-10-06: large projects without slowdowns, no CPU spent unless essential. The
writer path, writer audit and journal checkpoint belong to V1-0645 and are not changed here.

## Change

- Spec `docs/specs/corvint-tasks-agent-leases-v0.md` adds CAL-V0-135..142, each marked proposed,
  in a V1-0893/V1-0894 subsection with non-goals, failure modes, acceptance evidence and rollback,
  plus a slices row, traceability rows, the input and the delivery status (mirrored in
  `docs/specs/README.md` and `INDEX.json`).
- CAL-V0-135: `store.foldReceipts` (receipt audit, redo binding, `FoldExternalReviews`) reads each
  receipt beneath one pinned `receipts/` descriptor (`store/receipt_files.go`, reusing
  `pinnedDirReader`) instead of a root-to-leaf `safeopen` descent per receipt, then re-binds the
  named directory (SNAPSHOT_MOVED on replacement).
- CAL-V0-136: the Darwin arm64/amd64 detached-host escape scan reads `kern.proc.all` through
  `sysctl` (`supervisor/detached_rows_darwin.go`) instead of forking `/bin/ps` every 200 ms; other
  platforms keep the `ps` reader (`detached_rows_ps.go`).
- CAL-V0-137: the supervisor's host exit capsule poll backs off from 10 ms to the 200 ms escape
  scan interval (`nextHostExitPoll`).
- CAL-V0-138: `dispatch` and `service run` carry the review binding fold between ticks
  (`store.ReviewFold`) and fold only appended receipts, falling back to the whole-history fold on
  any digest, chain or read mismatch.

## Decisions

- The read-side sandwich (`snapshot.Reader.Read` probe, body, probe) and the journal audit's
  double capture are left as they are: removing a pass changes the snapshot consistency contract.
  They are the dominant per-read cost at 10,000 tickets and are recorded below as the next design.
- No persisted fold or second checkpoint: one-shot reads that need the review fold still fold the
  whole history. Making them O(tail) needs the V1-0645 checkpoint to carry the fold state.
- The carried fold is stricter than the whole fold (it also checks the receipt chain); a mismatch
  only causes a refold, so answers are identical.

## Evidence

Synthetic stores (`historyStoreAt`-shaped, real leases plus padded renew and body receipts), host
with 12 CPUs at load 11 to 28 from other lanes, medians of 5 (3 for audit and archive).
Stores: A 1,000 tickets / 13,000 receipts; B 1,000 / 50,000; D 10,000 / 13,000 (checkpoint at
head). Wall s / CPU s.

| Read | A before | D before | B before |
|---|---|---|---|
| queue status | 0.36 / 0.35 | 2.38 / 2.46 | 0.28 / 0.29 |
| ticket show | 0.34 / 0.33 | 3.44 / 2.98 | 0.31 / 0.31 |
| plan preview --selected-only (claim selection) | 0.35 / 0.34 | 2.46 / 2.51 | 0.26 / 0.27 |
| plan preview (full) | 0.37 / 0.37 | LIMIT_EXCEEDED | 0.30 / 0.33 |

Reads stay flat in receipts (checkpoint plus tail) and grow about 7x from 1,000 to 10,000
tickets. A CPU profile of `ticket show` on D puts 41% in the `snapshot.Reader` probes
(`intent.TreeDigest`) and 33% in the journal audit's two captures, mostly `openat`/`lstat`.

| `receipt audit` (CPU s) | before | after |
|---|---|---|
| A (13,000 receipts) | 13.81 | 10.07 |
| D (13,000 receipts, 10,000 tickets) | 14.65 | 11.43 |
| B (50,000 receipts) | 53.53 | 37.64 |

In-process: `BenchmarkCALV0135_FoldReceiptBindings` (2,000 padded receipts) 1.74 s/op before,
1.21 s/op after. `BenchmarkCALV0138_DispatcherTickFold` at an unchanged 2,000-receipt head:
0.45 ms/op, against the 1.2 s whole fold each dispatcher tick paid when any ticket had a review.

Idle CPU over 60 s (CPU s per minute, supervisor plus reaped children, Darwin):

| Supervisor host wait | before | after |
|---|---|---|
| normal host | 0.41 | 0.02 |
| detached (OpenCode) host | 8.22 | 0.45 |

`kern.proc.all` costs 0.76 ms per scan in process against one `/bin/ps` fork per scan.
Idle `dispatch` (5 s tick, no matching role) is unchanged by this work and is the largest
remaining idle cost: 0.10 CPU s/min on an empty store, 5.71 on A and 18.84 on D (31% of a core),
because each tick runs a full store read.

Focused tests: `go test -run 'CALV0135|CALV0138|ERG|ESC|Receipt|Redo|Audit|Review|Escalat' ./store`,
`go test ./supervisor`, and `go test -run 'CALV0138|ERG|ESC|Dispatch|Service|Receipt|Review' ./cli`
all pass.

## Remaining scalers

- Every read digests the intent tree four to five times (two probes per snapshot read plus two
  journal audit captures). Removing passes needs a snapshot consistency design, for example a
  directory-identity probe in place of a full tree digest.
- One-shot reads that need the review fold (`ticket show` and `plan preview` when a gate is
  declared, the gate views) fold every receipt. They need the fold state in the V1-0645
  checkpoint, bound to its sequence and receipt digest.
- An idle dispatcher repeats the full observation and a fsynced ledger save every tick (fixed in
  the follow-up below by CAL-V0-139).
- Full `plan preview` at 10,000 tickets refuses LIMIT_EXCEEDED (more than 250,000 decoded nodes;
  fixed in the follow-up below).
- A writer renew over a 100,000-receipt tail with no checkpoint near head failed ENFILE on Darwin
  twice (the vnode table was saturated); a 20,000 descriptor process limit did not change it.

## Not run

Live fleet qualification; Linux idle measurement; the 100,000-receipt stores could not be finished
with a checkpoint at head (the final real renew failed ENFILE, above), so their reads were not
measured.

## Follow-up: idle dispatch, shared captures, full plan preview, Linux `/proc`

Coordinator follow-up of 2026-10-06 on the same branch. The coordinator assigned CAL-V0-140..142
to the three further requirements, all proposed.

- CAL-V0-139 (idle dispatch tick, `dispatch/idle.go`): `Tick` wraps the old tick. After a full
  tick that left `state.json` byte-identical (so no event and no launch), the dispatcher arms a
  gate keyed on the queue's store witness (`cli/dispatch_witness.go`: head, barrier and
  `VERSION` bytes, the state and receipt directories and every top-level intent entry's
  name/mode/size/mtime) and the identity of its ledger and configuration. While armed, a tick
  costs one witness and a `requests/` directory read. It reads in full on a witness change or
  error, a pending request, the 60 s safety net, the earliest observed lease expiry, recorded
  cooldown or pool sweep, or a clock behind the armed tick. Dispatchers with live workers,
  recoveries, uncertain launches, a work-state reader or pressure signal never arm. The ledger
  save now skips the write (and its fsyncs) when the encoded bytes are unchanged.
- CAL-V0-142 (service pools): `service run` built its dispatcher queue without
  `c.TicketPools()` (9114dc99, `cli/service.go`), so the service dispatcher ran without the
  configured ticket pools. Both verbs now share `newDispatchQueue`.
- CAL-V0-140 (shared captures): the journal audit's before and after captures reuse the bytes
  `snapshot.Reader`'s first probe hashed (`journal.Reader.IntentTree`, passed from `withStore` only
  when that tree is the one the snapshot pinned and probed-tree reuse is on). Every intent file is
  still `lstat`ed; a file whose size changed is read fresh; a same-size rewrite is caught by the
  second probe, which re-hashes every file after the body, so the read moves and retries. This
  removes the two audit content passes; the two probe passes remain.
- CAL-V0-141 (full plan preview): the result self-validation now allows 250,000 + 64 nodes per
  plan entry (`cli/plan.go`, `wire.Result.MaxNodes`); a typical entry is 27 nodes, so 10,000
  tickets (about 270,000 nodes) preview instead of refusing LIMIT_EXCEEDED, and an entry with 64
  blockers still refuses. Output bytes are unchanged.
- CAL-V0-136 amendment: Linux reads `/proc/<pid>/stat` (`supervisor/detached_rows_proc.go`)
  instead of forking `ps`; tested against a fake `/proc` root on Darwin. Other Darwin
  architectures keep `ps`.

Not built: a stat-keyed digest cache for the two probes. At D a profile of `queue status` after
the shared captures puts most CPU in the two `intent.TreeDigest` passes (a `safeopen` `openat`
plus `lstat` and a SHA-256 per file, 10,000 files each). A cache keyed on device, inode, size,
mtime and ctime would leave one `lstat` walk per probe and drop the two content passes, an
expected saving of roughly half the remaining read CPU at 10,000 tickets. It needs its own
consistency argument (mtime granularity, same-size rewrites within one tick) and is left for the
owner.

Follow-up evidence: 9114dc99 (before) against 9565d5fe (after) on the same stores, interleaved,
host load 8.6 to 11.4, medians of 5. Wall s / CPU s.

| Read | A before | A after | D before | D after |
|---|---|---|---|---|
| queue status | 0.29 / 0.32 | 0.19 / 0.21 | 1.79 / 2.15 | 1.38 / 1.70 |
| ticket show | 0.20 / 0.21 | 0.15 / 0.17 | 1.72 / 1.96 | 1.23 / 1.43 |
| plan preview (full) | 0.22 / 0.25 | 0.18 / 0.21 | LIMIT_EXCEEDED | 1.32 / 1.71 |
| plan preview --selected-only | 0.22 / 0.25 | 0.16 / 0.17 | 1.69 / 1.93 | 1.14 / 1.36 |

Idle `dispatch` (5 s tick, no matching role, 120 s window, CPU s per minute):

| Store | before | after |
|---|---|---|
| A (1,000 tickets) | 2.98 | 0.23 |
| D (10,000 tickets) | 17.77 | 1.44 |

The remaining idle cost at D is the 60 s safety-net full read (about 1.5 CPU s each); the stat-keyed
cache above would roughly halve it. Focused tests pass: `go test ./supervisor ./journal ./wire
./store ./dispatch ./cli` (`internal/tasks`, one package at a time, `GOMAXPROCS=3`).

## Rollback

Revert the code and the spec subsection. No stored state, request, receipt or wire shape changes;
the dispatcher ledger encoding is unchanged.
