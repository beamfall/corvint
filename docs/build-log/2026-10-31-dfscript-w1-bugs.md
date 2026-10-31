# 2026-10-31 dfscript-w1 bug lane

Lane `dfscript-w1`, base `780f9e97099aa3ccd52744a0c239a744c4a30adf`: 15 bug tickets in
`internal/dogfoodflow`, the `Makefile`, `script/*` and the CI forward-row tooling.

## Decisions

- **V1-2557 and V1-2312 share one root cause.** Both tests call a shell function with prefix
  assignments (`TEST_GOVERSION=go1.27.3 expect_builds ...`; `PATH=$shim LANE_TEST_FAIL_GIT=... clean`).
  When those assignments outlive the call, the later case runs with go1.27.3, or with the
  failing-git shim. The two lanes saw these failures on the Mac (`/bin/bash` 3.2). Neither
  reproduces on WSL bash 5.3, at the current base or at the reported bases. Simulating the leak
  (an `export` in place of the prefix) reproduces each reported message exactly: `rg -q '^#
  go1.27.2$'` fails, and `lane-clean dry run: KEEP active ...` shows `merged` kept as
  `has-worktrees`. The fix runs each case in a subshell, and a guard fails if the variable
  survives its case. The orchestrator was asked to confirm the bash 3.2 behaviour on the Mac.
  A scan of `script/*.sh` finds more prefix assignments to functions outside `$(...)`, in
  `batch-precheck_test.sh`, `core-n1-source_test.sh`, `lane-scripts_test.sh` (the `pv`, `cemv`
  and `spv` cases) and `select-previous-release_test.sh`. They were left alone: their cases pass,
  and none is a reported failure.
- **V1-3247 (ASD-V0-940).** Reproduced: `heavy.sh false` exits 1 and prints only the sha and slot
  lines. The slot holder now ends a non-zero exit with `heavy.sh: exit rc=N cause=command|signal-SIG|timeout cmd=NAME`.
  The calling shell adds `cause=offload host=H`, `refused` or `wrapper`. The remote half adds no
  second line of its own; its holder line is relayed. The trailing code stays on the single
  `main` line, because the remote bash reads the payload from the same stdin.
- **V1-3037 (ASD-V0-941).** The new `BATCH_PRECHECK_GO_WRAPPER` is one quoted executable path,
  like `BATCH_PRECHECK_HEAVY`, and is empty by default, so behaviour is unchanged. It wraps every
  `make` and `go run` step. Drift steps keep `BATCH_PRECHECK_HEAVY`. A default of `script/heavy.sh`
  was rejected: the ticket asks for an unchanged default, and a per-step slot serialises the
  precheck.
- **V1-2393 (ASD-V0-942).** At this base, 9 script tests had no Makefile target (the ticket counted
  11). All 9 now have one, and six are gated (`GATE_STEPS` and `shell-test`, so CI's shell command
  runs them). Three stay outside the gate. `public-release-check-test` fails its 8-second
  TERM-join bound on a loaded host (it failed once here at load 36 and passed twice when quiet).
  `release-env-compat-test` already runs inside `companion-release-gate-test`.
  `seed-planning-store-test` needs `atm` and `/private/tmp`. Two scripts gained the executable bit.
  `TestASDV0942EveryScriptTestHasAMakeTarget` locks the invariant.
- **V1-3346 (TSP-V0-940).** gateselect reads the declaration itself, with the planner's validity
  rules, rather than importing the planner's unexported reader. It clears `locatesRoot` only when
  every declared package matches; anything else declares nothing, which is the wider selection.
  Both declaration frontiers fall back as module-level frontiers. gate-ledger's fixture copy list
  and read scope gained `gatedlocators.go`.
- **V1-2445 (DCW-V0-940).** The lane's real inputs were not available, so prepare could not be
  reached in a plain clone at head `7c1a864ac` (the run stopped on missing citation and intent
  inputs, with no OCM refusal). The AC's second branch was delivered instead. `mapOCMCEMError` and
  `mapOCMRepositoryError` had collapsed the underlying Git refusal into a bare `repository object
  unavailable`; they now keep it, and the OCM sites name the target, path, revision or blob. A
  `repository-object-unavailable` row of any dogfood step now prints a `fix:` line that quotes the
  step's stderr and gives the fetch or unshallow remedy.
- **V1-3036 (DCW-V0-941).** Reproduced end to end. The committed deferral record is a hunk of the
  map at the bind commit, so the rebind re-prepares and cites it into an uncommitted sidecar, and
  OCM prepare then refuses `excluded-artifact-mismatch` and `intent-scope-drift`. finish-lane now
  runs one bind-scope settle pass after the bind commit when that commit carries a record. If the
  sidecar is the only changed path, it amends the bind commit with it. The amend records no
  trace: the settle and bind passes run no recorder, and a rebind on the old commit would have
  seen a dirty sidecar.

## ALREADY-DELIVERED evidence

- V1-2424: the root `go.mod` now pins `go 1.27.3`. `script/heavy.sh go version` in
  `interop/cem01-go` prints go1.27.3, the pin. The nested-module case is
  `script/lane-scripts_test.sh` ASD-V0-014 (V1-2384), where the nested `go 1.98.0` line does not
  override the root pin.
- V1-3225: commit 91e0d0b38 (V1-3234) names `file:line holds "LITERAL", which locates the root` plus
  the rule, and `TestRootResolvedRefusalNamesLocatingLiteralAndRule_V1_3234` locks it. Observed
  live in this lane.
- V1-2391: the `cem-cite:citation-plan-uniform` fix names the per-hunk form, from be0895013;
  `TestDogfoodFinishLaneDCWV0061StopsOnUniformPlanAndResumes` passes at base.
- V1-2269: `TestAFPV0041ObserveSkipsTheShadowSummaryLine` (tools/ci-test-slices) passes at base.
- V1-2270: `script/ci-least-privilege-exceptions.tsv` carries the `V1-2270` rows for
  `ci-forward-bar.yml` and `ci.yml` `job:narrowing-verdict`, and `make ci-least-privilege-check`
  passes at base.

## BLOCKED

- V1-3332: both the row-absence alarm and the AFP-V0-030 kill switch it feeds sit in
  `.github/forward-rows/verdict.sh` and `bar.jq`, which another lane owns. Proposed shape for that
  owner: verdict.sh already lists the epoch's eligible completed runs. Count the runs in the
  window that have no row and no epoch-1 marker, and make the verdict NOT ESTABLISHED, with a
  `rowAbsence` count, when that count reaches N.
- V1-2901: the acceptance needs an owner routing decision (PR shards on GitHub-hosted runners),
  reserved capacity or a separate host, and 7 days of per-runner remeasurement.

## Host findings

- WSL `/usr/bin/mkdir` is uutils coreutils 0.8.0. Racing `mkdir` calls can both succeed: in 2 of
  8 trials, two of 100 racing calls succeeded. A `mkdir` lock is therefore not mutual exclusion
  on this host; the ASD-V0-941 test locks with perl `flock`.
- `script/batch-precheck_test.sh` section 9 fails on this host's tmpfs `/tmp` (3.1 GiB free,
  under its 4 GiB floor) and passes with TMPDIR on disk.
