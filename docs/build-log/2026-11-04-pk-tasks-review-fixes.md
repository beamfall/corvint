# Tasks round-1 review corrections

Owner-requested fixes over round-0 head `2514fee0ca701b2e4dfd30a6ec3aa115d7d1247c`, retaining
base `6c33df33bc5670131482f5263b410cb980580657`. The independent review requested these three
corrections; no unrelated implementation work is added.

## Decisions and findings

1. P2 / V1-4260 / CTW-V0-7100: FIXED by taking conservative option (a). Remove the
   `mutationBatchKey` / `applyMutation` dispatch and the activation of `WithBatchWrites` in
   `MutateBatch`. Both mutation publishers are byte-identical to base; the batch driver differs
   only by retained stage instrumentation. Restore CTW-V0-013's per-entry audit-cost non-goal
   verbatim and LGL-V0-013/016's lease-only exclusions. No batch publisher is claimed proved.
   CTW-V0-7100 is now proposed, experimental measurement only. Retain the opt-in native profile
   and all round-0 samples: 61.013 s base, 79.520 s capture-only, 69.462 s overlap under different
   contention. These do not establish improvement. V1-4260 stays PARTIAL: profile retained, no
   speedup shipped; Mac <=20 s CLI qualification remains unavailable.
2. P3 / V1-3173 / TCL-V0-7102: FIXED. An explicit projection's INTENT_DIVERGED repair names
   `receipt audit --worktree` and the selected projection, asks for an agreeing projection, and
   omits the unrelated CTW-V0-002 linked-intent commit/carry advice. The existing audit test now
   asserts the repair text, path and absence of that advice.
3. P3 / V1-3173 / TCL-V0-7102: FIXED. Resolve ancestors separately, then check the unresolved
   DIR leaf with `intent.CheckNoSymlink` before leaf `EvalSymlinks`. A symlinked DIR refuses
   UNSUPPORTED_FILESYSTEM. An ancestor alias remains valid, preserving macOS /tmp and /var
   compatibility. The new test covers both paths and verifies that the owning journal is unchanged.

The old overlap-asserting test is retargeted to base serial publication and base ordering edges.
`TestCTWV07100_BatchPublicationFailureCleansStages` passes on that path, including retry, exact
replay and consistent audit. Existing CAL-V0-106 batch and TCX-V0-977 binding tests also pass
with the race detector. No new shared model, group commit, audit bypass or sync overlap is enabled.
Rollback of the CLI corrections is source-only; no stored or wire format changes. Restoring the
rejected batch optimizations requires new authority and a qualifying measurement, not rollback.

## Corrected round-0 dispositions

- V1-4164: already satisfied at base; test pins the existing limit (TCL-V0-7101, 128 bytes).
- V1-2203: marginal pass based on fixes that already existed. The retained medians are 49.00
  and 49.05 ms against <=50 ms, a margin under 2%; no latency headroom is established.
- V1-4306: the ERG-V0-017 half is refuted (20/20 at base and head), not a reproduced fix.
  The LGL-V0-7100 compaction timestamp test correction remains as delivered in round 0.

## Verification and retained limits

The [round-1 evidence directory](evidence/2026-11-04-pk-tasks-review-fixes/results.json)
retains command/result pairs and exact compressed logs. GOTOOLCHAIN=local reports go1.27.3.
The required commands pass, one package per heavy.sh invocation, with the shared lock directory
and TMPDIR=/home/russell/.cache/corvint-tmp:

- `go test -count=1 -race -run 'TestCTWV07100|TestCALV0106|TestTCXV0977' ./internal/tasks/store/`
  PASS (28.183 s package time).
- `go test -count=1 -run 'TestTCLV0710|TestTEAV07100|TestATRV0190' ./internal/tasks/cli/`
  PASS (2.130 s package time).
- `make spec-requirements BASE=6c33df33bc5670131482f5263b410cb980580657`, after staging owning
  specs: PASS, including the heavy.sh obligation-bound test. CTW-V0-7100 no longer has accepted
  status in REQUIREMENTS.tsv. Existing full-spec capacity warnings do not add obligations here.
- `make traceability-tests-check line-citations-check`: PASS; the citation checker reports 180
  existing skipped non-root basenames. No claim is made about those unchecked citations.
- Base-path byte comparison and verbatim non-goal check: PASS, retained in
  `base-path-equality.txt`. The native profile is intentionally not activated in this fix round;
  retained samples remain failed experiments, not new performance evidence.

`make dogfood-change BASE=...` ran at start, reporting missing final citation/intent/outcome
inputs for the existing round-0 CEM. `dogfood-prechange` refuses new receipts after the first
change commit (`prechange-after-change`); the original base-bound pre-change receipts remain
operative. A fresh bounded `corvint context` result and `corvint affected --base BASE` selection
are retained in `/tmp/pk-tasks-r1/`. Context omits lexical rows at limit 5; it is discovery only.
The installed binary warns `self-binary-stale`; the Makefile's current-tree dogfood build is used
for binding and checking. The affected plan retains UNKNOWN scope, build-constraint variants,
foreign shell invocations and package-init fallback; broader closure is NOT_RUN under the owner's
explicit focused-test instruction, not represented as covered.

Independent follow-up review remains orchestrator-owned; model-backed reviewers are NOT_RUN.
Full gate, Windows vet, new performance qualifications, native ticket completion, publication
and seal are NOT_RUN/NOT_PRODUCED under the lane restrictions or retained external blockers.
Ranking/learning/frozen CEM-wire partitions are unchanged and excluded. Token/cache/cost telemetry
is NOT_OBSERVED. Final rebind and dogfood-check results are recorded in the external lane handback
and `/tmp/pk-tasks-r1/`; V1-4488's known report-publication EXDEV, if repeated, remains a failed
check for host rerun and is never treated as a pass.
