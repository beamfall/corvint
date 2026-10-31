# Tasks core lane tasks-core (wave 2) — 15 tickets

Base: origin/main `6c967b21caa2ffb0324a681f873a891996d82e34`. Branch
`claude/lane-taskscore-w2-1031`, built on the Mac. Tests ran through `script/heavy.sh`, one
package per command, while other lanes loaded the host (1-minute load from 7 to above 200 on
12 CPUs). Timings below carry the host load measured beside them.

Dispositions: ten delivered (V1-1212, V1-1007, V1-3185, V1-3186, V1-3191,
AGENT-TASKS-ADMIN-RESERVATION-1, V1-0232, V1-2479, V1-0673, V1-0757), three ALREADY-DELIVERED
(V1-1473, V1-2204, V1-2383) and two BLOCKED on external inputs (V1-0464, V1-0436).

## Delivered

- **V1-1212** (`81bca83f4`, TQD-V0-010): test repair only. The doctor plugin-findings test runs
  at the production timeout, and the timeout path is witnessed by a separate test.
- **V1-1007** (`f19026665` spec, `e74ad9d7a` code; KHN-V0-800): a policy change limited to the
  top-level `knowHow` key is handoff-compatible, so it no longer fences live handoffs. A store
  witness covers a WORKER know-how add whose lease expires between the pre-lock sample and the
  in-lock commit; it fails with the in-lock resample disabled.
- **V1-3185** (`09097d374`, `ca9c3a92e`; ATR-V0-800..801): a claim refused `RESOURCE_COLLISION`
  now names the next action: a directory key reserves every path beneath it, so tickets writing
  distinct files declare exact file touchPaths with `ticket set-effects`; and when the claim took
  the declared scope although `--scope` was given, the detail says so (ATR-V0-215 unchanged).
  Exact-file reservations under one directory are admitted concurrently.
- **V1-3186** (`ce22b3f23`, `65d87080c`; LGL-V0-800): `receipt audit` memoizes decoded attempt
  posts the way `TicketPosts` already did for ticket posts. Interleaved A/B, 5 runs per binary,
  host load 9-14: wall time 25-29% lower at 3k, 13k and 40k synthetic receipts and 29% lower
  (6.05 s → 4.3 s) on a 15,147-receipt copy of the live store, output byte-identical. Binding-fold
  CPU fell from 1.93 s to 1.05 s and allocation from 7.95 GB to 5.42 GB. Two rejected
  hypotheses: GC tuning (no change) and a parallel receipt read-ahead (no gain; opens were not on
  the critical path).
- **V1-3191** (`b78f1b7b3`, `bffeb9de0`; TRM-V0-800..802): `claim` and `widen` take
  `--admit-under-barrier`, recorded in the lease request, so an operator can admit one scoped
  attempt under the admission barrier. The default CLI role is OPERATOR, so the explicit flag,
  not the role, is the gate; that is named as a failure mode in the spec.
- **AGENT-TASKS-ADMIN-RESERVATION-1** (`29af744ae`): `docs/TASKS-EXTERNAL-AGENTS.md` states that
  store administration reserves only each attempt's declared scope.
  `TestCALV0023_AdministrationReservesOnlyExecutionEffects` proves it, and also that an effects
  change is refused `ATTEMPT_LIVE` without writing while an attempt is live.
- **V1-0232** (`91ce6a78d`, `dbf5540d1`; TRM-V0-803..804): an external release attestation for a
  COMMAND gate must carry the policy argv, and readiness refuses one that does not. The live
  release-checklist gate therefore now needs `argv` in record-gate payloads.
- **V1-2479** (`fcee79923`, `1b380b3db`; CTS-V0-800): import-map amendment refuses renaming the
  source item of an imported ticket. The test fails without the fix (the rename commits).
- **V1-0673** (`255b70dbb`, `84f094e9a`; CAL-V0-060 clarification, no new ID): the
  administrative writer (policy, import plan, pause, cutover, relocate) neither retains nor
  removes the read or writer checkpoint. Decision: this is the intended complete-audit fallback,
  not a defect. A retained checkpoint stays a valid resume point; with none, reads are complete
  until the next ordinary writer retains one. Correctness loss NOT_OBSERVED; the next-read cost
  was not measured. Related: V1-0644.
- **V1-0757** (`252e099d7`, `bd9ebcb4f`; LGL-V0-801): the B2 lease-gate load harness takes
  `CORVINT_TASKS_LEASE_GATE_LOAD_MUTATORS=N`, non-lease writers that cycle create, refine,
  complete-manual and note beside the lease writers, and the report now carries a post-run
  complete audit. Run: 7,000 receipts, 100 tickets, 6 lease writers, 3 mutators, 30 s, 1 s think,
  darwin/arm64, 12 CPUs, go1.27.3, load 7.09, QUALIFIED. 248 calls (102 non-lease), no codes,
  `LOCK_TIMEOUT`=0, audit `CONSISTENT AGREES`. Wall p50/p99 (ms): create 133/705, refine
  108/220, complete-manual 103/241, note 106/240, heartbeat 120/1089, renew 117/780, claim
  520/670, widen 2180/4431, reap 2162/5343, release 102/146.
  Mixed binary: a build of pre-1.0 `32aa5a3f` (2026-10-03) pointed at a store created by the
  current build refuses `queue status` and `ticket create` with `MALFORMED: policy.json: closed
  object violated (unknown keys: reservations)` and writes nothing; head stays at 2 and the audit
  stays `CONSISTENT AGREES`. The probe ran the binaries in turn, not concurrently; since the
  refusal precedes any write, concurrency should not change it (inference).

## ALREADY-DELIVERED

- **V1-1473**: the named test passed 10/10 (`-count=10`) at host load 105-201. No change.
- **V1-2204**: the B2 harness already covers 15 writers at 40k receipts (JSG-V0-021) and B3 is
  JSG-V0-020. Smoke run of B3: 2,000 receipts × 3 iterations, UNQUALIFIED at load 28.42 as
  designed, heartbeat p50 about 44 ms, reap about 739 ms. The macOS qualification run stays
  NOT_RUN.
- **V1-2383**: B3 already measures widen and reap.

## BLOCKED (external)

- **V1-0464**: needs a production activation C6 evidence write in `core-history`, outside this
  repository and lane.
- **V1-0436**: the incremental import sequence is on main; only the Beamfall re-measurement is
  open. Its pinned 2,894-item input lived under `/tmp` and is gone, and regenerating it needs the
  Beamfall exporter, outside the lane. Re-measurement NOT_RUN.

## Cross-lane edits

Kept minimal and only where a ticket's fix lives:

- `internal/tasks/cli` (lane tasks-cli): `cli.go`, `command_help.go`, `lease.go`, `release.go`
  (V1-3191, V1-0232 flags and help); tests `doctor_test.go`, `nonfixture_release_test.go`,
  `release_test.go`, `widen_resume_test.go` (fixture argv, new flag coverage).
- `internal/tasks/store` (lane tasks-store): `external_review.go`, `release.go`, and the tests
  `admin_checkpoint_test.go`, `admin_reservation_test.go`, `importmap_amend_test.go`,
  `know_how_worker_expiry_test.go`, `lease_exact_paths_test.go`, `lease_gate_load_test.go`.

## Friction

- The hook blocks `for`+`sleep` loops, and heavy.sh waits on host load (it reached 80); load
  from other lanes skews timings, so A/B runs had to be interleaved.
- Whole-package runs need `HEAVY_WHOLE_REASON`; one test run went to the background after
  180 s under load.
- A store under `/private/tmp` is refused `UNSUPPORTED_FILESYSTEM`.
- The leases spec is at its OCM obligation limit, so its contracts get amended through companion
  specs.
- `receipt audit` has no `--through`.
- A WORKER role cannot run CLI claims (`UNAUTHORIZED`), and the default CLI role is OPERATOR.
- No spec governed release attestations before TRM-V0-803.
- A zsh glob breaks `grep --include`.
- Evidence inputs pinned under `/tmp` disappear (V1-0436).
- `writerStore` in store tests has a hidden minimum receipt count: 20 fails with "head 51".
- The import-map canonical-sort requirement is easy to trip in tests.
- An acceptance paragraph starting with a backticked requirement ID counts as a second
  definition in `make spec-requirements`.
- `ticket create --template bug --skeleton` and `--template=bug` are both rejected as unknown
  flags.
- `init` on a fresh repository fails `MALFORMED` with only an lstat warning when
  `.taskman/queue.json` is missing.
- Product evidence: a pre-1.0 binary refuses the current store's policy (`reservations`) as
  `MALFORMED` instead of naming a version skew.
