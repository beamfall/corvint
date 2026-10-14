# 2026-10-14 V1-0668: Linux zombie-only group retirement, hosted amd64 and arm64 qualification

1.0 scope: `PRS-V1-018` (1.0 scope per the V1-0020 dependency). Base: origin/main
4c1ca0d0f94ec36e2ac141c5515742a1fb23a758. Governing contract: `docs/specs/process-group-owner-v0.md`
(PGO-V0-001, -006, -009). New requirement: `PGO-V0-011`, accepted under the owner's standing
auto-accept (2026-10-11); V1-0668. V1-0632 stays OPEN.

## Intent and decision

The Linux /proc pre-reap quiet proof (`PGO-V0-006`, `internal/groupreap/owner_proc.go`) already
existed. What was left was host evidence, plus five real-process Owner tests in
`internal/groupreap/owner_unix_test.go` that skipped on Linux behind `requireObservedOwnerPlatform`
("observed on Darwin only").

This change removes that gate after the tests passed on Linux:

- `TestOQ12TwoStepAbsenceOnRetiredGroup` keeps the platform quiet proof on Linux
  (`QuietProof: real.QuietProof`). Its recorder then sees only the post-reap signal-0 probes. Its
  Darwin-only check on the pre-reap probe is kept for Darwin.
- It adds `TestOwnerRetiresHostileGroups` for three hostile shapes:
  - a leader and member that ignore SIGTERM (checked by test-owned per-PID SIGTERM);
  - a forked grandchild orphaned inside the group while its leader has exited;
  - the same grandchild while its leader is still running.

Every case asserts that exactly one `kill-group` happens before `reap` and that no group signal
follows the reap. No product code changed.

Test-owned signals to a member now go through the member's pinned table identity (`pinMember`,
then `signalPinned`: a pidfd on Linux, a start-time recheck on Darwin) instead of a bare PID.
`startScriptWithMember` also registers a cleanup that retires a group left unfinished by a failed
test, using the owner's own pre-reap `Stop`/`Finish`.

`TestWaitResidueDarwinDeniedLiveMemberIsResidue` stays Darwin-only. It tests Darwin's signal-0 EPERM
ambiguity, which Linux does not have, and its Linux counterpart is
`TestLinuxOwnerHoldsWhileLiveMemberRemains`.

## Inventory: Linux skips at BASE and after

| Test | BASE on Linux | After |
|---|---|---|
| groupreap TestOwnerRetiresLingeringMemberAfterNormalExit, TestOwnerStopRetiresRunningGroup, TestOQ12LiveMemberAfterReapIsObservedLive, TestOQ12TwoStepAbsenceOnRetiredGroup, TestOwnerHoldsOnInjectedSignalFailureWithoutReaping | skip (Darwin-only gate) | run, pass |
| groupreap TestOwnerRetiresHostileGroups (3 subtests) | absent | run, pass |
| groupreap TestWaitResidueDarwinDeniedLiveMemberIsResidue | skip | skip (Darwin semantics; counterpart above) |
| groupreap TestLinux* (10), cem/gitrun TestStableSuccessReleasesOwnedGroup | run, pass | unchanged |
| interop/cem01-go TestStableLinux* (5), TestStableLifecycle* | run (a `!stableOwnerSupported()` skip cannot fire silently: TestStableLinuxOwnerSupported fails instead) | unchanged |
| cem/verify TestStableS0EPublicCases go1.24.13 (2), TestStableLiteralRepository | skip (tuple not installed; external fixture root) | unchanged, out of scope |

## Hosted Linux amd64 evidence (BASE)

The source is merge-queue run 38645188769 (merge_group, head = BASE, runner image ubuntu-24.04
20261012.334.1, go1.27.2, `-race -json`). All its jobs succeeded.

- Job 116130374760, go-product-shard (3), internal/groupreap: 128 pass, 0 fail, 6 skip.
  - All 10 `TestLinux*` pass, for example `"Action":"pass",...,"Test":"TestLinuxOwnerRetiresZombieOnlyGroup","Elapsed":0.01` and `TestLinuxSignalZeroCannotProveZombieOnlyQuiet` (0.3s).
  - The 6 skips are the five gated tests above plus the Darwin residue test.
- Job 116130374623, internal/cem/gitrun: 67 pass, 0 skip. `TestStableSuccessReleasesOwnedGroup` passes in 0.01s.
- Job 116130374657, internal/cem/verify: 204 pass, 0 fail, 3 skip (listed above).
- Job 116130374728, internal/cem/gitauth: passes.
- Job 116130374743, internal/cem/cli: 38 pass.
- Job 116130173992, go-interop: `ok github.com/Beamfall/corvint/interop/cem01-go 48.678s`. This job
  is non-verbose, so per-test lines are NOT_OBSERVED.

The main push run 38643190285 at d3be74b06 also succeeded. Fresh amd64 per-test lines for the five
un-gated tests and the hostile cases come from this change's PR CI (go-product-shard `-json`). They
were not observed before publication, and the lane does not poll CI.

## AC1: original failures and refusal codes

The original source is PR476 head d5fdbe733b5bfbc0ff3c5a2aa2d1cc195bb8e0d2, run 36963786786. The
ticket's retained paths (`/private/tmp/corvint-orchestrator-pr476-ci-shard1.log`,
`/private/tmp/pr476-linux-gitrun-timeout-events.json`) no longer existed, so both job logs were
downloaded again from GitHub.

- **Job 110703115457, go-product-shard (1): cancelled.**
  - internal/cem/gitrun `TestStableSuccessReleasesOwnedGroup` hung until `panic: test timed out
    after 50m0s`.
  - This is a hang, so there is no refusal code.
  - arm64 at d5fdbe733 reproduces it: `panic: test timed out after 1m30s` at `lifecycle_test.go:83`
    with `-timeout 90s`.
  - It passes at BASE on hosted amd64 and on arm64.
- **Job 110703115422, go-product-shard (3): failure.**
  - internal/cem/verify `TestStableRepositoryRejectsCreateOverAuthenticatedEmptyBaseTree` (20.03s).
    The hosted log prints only `Outcome:UNSUPPORTED Stage:repository Code:0xc00033e040`, a pointer.
  - internal/cemcandidate, each about 20s per refusal:
    - `TestStableAssemblySeparateSnapshotIdentityAndFullEnvelope/{sha1,sha256}`
    - `TestStableAssemblyPreservesFull03Witnesses/{observed,not-run}`
    - `TestStableAssemblyRetainsNativeFailureAndRetryBytes`
  - internal/cem/gitauth `TestCanonicalDiffWithCreateDestinations{ProvesBaseEntries,WithholdsProofOnProvenanceFailure}`
    failed with `Author identity unknown` (exit 128). That is a fixture identity failure, not a
    refusal, so there is no code.
- **arm64 reproduction at d5fdbe733.** Setup: golang:1.27.1, uid 1000, `--network none`,
  Linux 6.8 aarch64, git 2.47.3, with an uncommitted diagnostic `t.Logf` of the scalar code.
  - All six reproduce.
  - Each repository refusal prints
    `{"code":"unsupported-process-containment","issueCodes":[],"outcome":"UNSUPPORTED","stage":"repository"}`,
    and the verify case exits 2. Timings are 20.03 to 20.07s, the same as hosted.
  - The scalar code on hosted amd64 itself remains NOT_OBSERVED, because the original log retained
    only a pointer and replaying historical source on hosted CI is outside this lane.
- **At BASE.** The verify and gitauth cases pass on hosted (jobs 116130374657, 116130374728) and on
  arm64. The three cemcandidate tests were deleted by 744e8fc44 (V1-1278, all consumers moved to
  cem/1.0), so they cannot be rerun.

## arm64 evidence (this change)

Setup: colima VM (aarch64, already running and not started by this lane), golang:1.27.2 image
sha256:5bc7f572…, uid 1000, `--network none`, `GOTOOLCHAIN=local GOPROXY=off`. The source was piped
in with tar. Logs are in the lane scratch `evidence/`.

- **internal/groupreap with `-race -count=1 -v`:** passes both with PID 1 = `sh` and with
  `--init` (PID 1 = docker-init).
  - Every newly un-gated test and all three hostile subtests pass, for example
    `--- PASS: TestOwnerRetiresHostileGroups/sigterm-ignoring (0.20s)`.
  - Skips: `TestWaitResidueDarwinDeniedLiveMemberIsResidue` (by design) and
    `TestRetireEscapedIdentityBoundary`, which skips because the container's low test PID collides
    with its synthetic table. That skip is environmental and does not happen on hosted CI.
  - On the first PID-1-`sh` run, the untouched `TestLinuxOwnerHoldsWhenMemberRemainsVisibleAfterReap`
    failed once. The Owner returned HOLD, as required, but with `PostReapObserved=false`, because
    the 2s post-reap cap fired between the probe and the deadline check. This is a timing race in
    the assertion, not an ownership defect. It passed alone, and the next full run passed.
- **internal/cem/gitrun, internal/cem/cli, internal/cem/gitauth with `-race`:** pass. gitauth's
  `TestRelativePathsLinkedWorktree` skips because git 2.47.3 lacks `--relative-paths`.
- **internal/cem/verify with `-race`:** 204 pass, with the same 3 skips as hosted.
- **interop/cem01-go with `-race -v`:** 372 PASS lines.
  - All `TestStableLinux*` and `TestStableLifecycle*` pass, including
    `TestStableLinuxLiveMemberBehindZombieLeaderHolds` (a SIGTERM-ignoring member) and
    `TestStableLifecycleHostileDescendantsRetired`.
  - Two `TestStableCanonicalEnvelopeCells/relative-{sha1,sha256}` fail. The cause is the image's
    git 2.47.3 rejecting `worktree add --relative-paths`, which is environmental: hosted ubuntu
    passes the package, and the cases are not lifecycle tests.
  - The full21 go1.24.13 rows skip, as at BASE.

Darwin arm64 (macOS 25.6) `go test -race -count=1 ./internal/groupreap` passes with no skips. go vet
passes for darwin, linux and windows. No Darwin product path changed, and interop and the frozen
full21 results are untouched.

## Independent review (Codex, one read-only round)

Codex reported four findings. All four were confirmed and fixed; the arm64 groupreap runs above
were repeated after the fixes (both PID 1 modes pass).

- **P1:** E1b cleanup sent SIGKILL to a bare member PID after the leader reap, so a reused PID
  could be hit. Fixed with the pinned identity.
- **P1:** a hostile-test failure before `Finish` could leave a `sleep 300` member running. Fixed
  with the owner cleanup in `startScriptWithMember` plus a pinned member kill.
- **P2:** the SIGTERM survival check used signal 0, which a zombie also answers. It now reads the
  process table and requires the same identity in a non-zombie state, then checks that the leader
  has not exited.
- **P2:** `PGO-V0-011` said "is qualified" before the hosted per-test lines existed. It now states
  that the missing architecture stays unqualified until both kinds of evidence are retained.

## Failure modes and limits

- **Non-reaping PID 1.** On a host whose PID 1 or subreaper does not reap orphans, a killed orphan
  stays a zombie member. Post-reap signal 0 sees it as live and the Owner HOLDs (fail-closed, see
  Non-goals). That host is not qualified. The PID-1-`sh` container reaped promptly.
- **Hosted amd64 lines.** Hosted amd64 per-test lines for the newly run cases are pending this PR's
  CI. Interop hosted per-test lines stay NOT_OBSERVED until that job runs verbose, which is outside
  this lane's scope.
- **Out of scope:** S0E go1.24.13 cases are NOT_RUN (tuple not installed), and
  `TestStableLiteralRepository` needs the external `CEM_STABLE_FIXTURE_ROOT` harness.

## Rollback

Revert the change commit. That restores the Darwin-only gate and removes `PGO-V0-011` and the
hostile test. No product code, store or wire format changes.
