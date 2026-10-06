# 2026-10-06: corvint-tasks read scale and idle CPU (V1-0893, V1-0894)

## Intent

Native tickets V1-0893 (reads, claim selection and receipt audit stay fast at 10,000 tickets and
100,000 receipts) and V1-0894 (idle CPU of long-running processes), both citing GitHub
[beamfall/corvint#641](https://github.com/beamfall/corvint/issues/641); neither has its own issue.
Owner goals of 2026-10-06: large projects without slowdowns, no CPU spent unless essential. The
writer path, writer audit and journal checkpoint belong to V1-0645 and are not changed here.

## Change

- Spec `docs/specs/corvint-tasks-agent-leases-v0.md` adds CAL-V0-135..138, each marked proposed,
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
- An idle dispatcher repeats the full observation and a fsynced ledger save every tick.
- Full `plan preview` at 10,000 tickets refuses LIMIT_EXCEEDED (more than 250,000 decoded nodes).
- A writer renew over a 100,000-receipt tail with no checkpoint near head failed ENFILE on Darwin
  twice (the vnode table was saturated); a 20,000 descriptor process limit did not change it.

## Not run

Live fleet qualification; Linux idle measurement; the 100,000-receipt stores could not be finished
with a checkpoint at head (the final real renew failed ENFILE, above), so their reads were not
measured.

## Rollback

Revert the code and the spec subsection. No stored state, request, receipt or wire shape changes.
