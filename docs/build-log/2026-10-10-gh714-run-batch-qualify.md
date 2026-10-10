# GitHub #714: experimental `run-batch` and `qualify` (V1-1075)

Date: 2026-10-10

## Problem

A large browser-tested ticket needed an agent session to run each batch of obligations, read the
Playwright report and explain which obligations moved and why. A final run that passed only after a
retry, with a failing post-check or with a broken neighbouring spec, looked as green as a clean one.
The same obligation could fail the same way run after run, each run spending a lane.

## Decision

The agent-drafted requirements TOL-V0-028..033 in
`docs/specs/corvint-tasks-obligation-ledger-v0.md` are proposed, pending owner acceptance, and the
delivery is experimental. No decision file is written: 0485 is the next free number, but the
requirements are not accepted. The coordinator delegated the in-task forks; each was decided here.

- One closed local configuration file (`corvint-tasks-run-config/0`) holds every command as argv.
  Commands never run through a shell, and reuse the lease runner's `COMMAND` gate execution
  (declared environment, timeout, output cap, process-group kill) through
  `store.RunBatchCommand`, so no new process code exists.
- `run-batch` credits only through the unchanged `ticket obligations witness` path, called in
  process. Witness rules, roles, `--ids`, the post-check refusal and the empty qualified-version
  list all apply unchanged. Causes come from re-running the same classifier on the pre-write
  ledger; `NOT_REACHED`, `NOT_NAMED` and `NOT_OBSERVED` use the TOL-V0-025 source scanner and are
  heuristic.
- Claim, renewal and HANDOFF stay with the caller. The job only acquires and releases a member for
  the caller's claimed attempt with the existing `pool acquire` and `pool release` verbs. Capture
  runs before release, because release quarantines and resets the member.
- The repeat-failure refusal (TOL-V0-031) reads only earlier summaries under `--out`, not the store,
  and keys on a fixture digest of the Git object ids of the configured fixture paths and the
  `--spec` paths at the commit. It reuses `LOOP_DETECTED` with the detail prefix
  `OBLIGATION_REPEAT_FAILURE:`. No new result code was added.
- `qualify` runs static checks before any lane, never credits and never writes the store.
  `NOT_QUALIFIED` reuses `GATE_FAILED` with `QUALIFY_NOT_QUALIFIED:`. Neighbours are the other specs
  under directories changed between base and commit. A change at the repository root selects
  none, rather than every spec. Failing neighbours are re-run at the base in one temporary detached
  worktree (`store.WithDetachedWorktree`) to tell `PRE_EXISTING` from new; an unobserved base
  counts as new (fail closed).
- Both jobs require a clean checkout of the commit (`STALE_TREE`, `DIRTY_WORKTREE`) and re-check it
  after the run. An interrupt or a changed checkout credits nothing and fails qualification.
  Results directories are created exclusively, so a reused request id refuses
  `REQUEST_ID_CONFLICT`.

## Evidence

- Branch `claude/gh714-run-batch-qualify` merges #712/#713 final head 32545bab as merge commit
  54b962db, at the coordinator's instruction.
- Corvint use: `corvint context --task "corvint-tasks run-batch qualify ..." --subject
  internal/tasks/cli/preflight.go` routed to the preflight, witness and lease-runner code that is
  reused here.
- Failing before: in a detached worktree at 54b962db with only `internal/tasks/cli/run_batch_test.go`
  added, every new test failed with "unknown verb". After the change, they pass:
  `TestTOLV0028_RunConfigIsClosed`, `TestTOLV0029_RunBatchCreditsAndWritesCauses`,
  `TestTOLV0029_RunBatchPrepFailureIsNotReached`, `TestTOLV0030_RunBatchRefusesAStaleOrDirtyCheckout`,
  `TestTOLV0031_RepeatFailureRefusedBeforeLaneAcquire` and `TestTOLV0032_QualifyVerdicts`. The
  existing TOL-V0, preflight-help and pool tests also pass, as does the whole
  `internal/tasks/obligation` package.
- The tests use a fake `sh` test command that writes Playwright-shaped json from templates. Real
  Playwright is `NOT_RUN`.

## Codex round 1 fixes

Codex round 1 on 80b1f2c7 returned seven findings, fixed in one commit. Each new test was shown to
fail before its fix by reverting only that fix in a scratch detached worktree that held the fixed
tree, then running only that test:

1. An interrupt seen after the report's source evidence is read but before the witness mutation is
   submitted now credits nothing and refuses `GATE_FAILED` with `LANE_FAILED:`; a submitted
   witness is atomic (TOL-V0-033). `TestTOLV0033_InterruptBeforeWitnessSubmitCreditsNothing`
   (before: the interrupted job credited AC-1).
2. A neighbour result with retry above 0 is `RETRY` even when the base also fails, and top-level
   neighbour report errors disqualify whether or not a test failed (TOL-V0-032). Three new
   `TestTOLV0032_QualifyVerdicts` cases (before: the first two were `QUALIFIED`; the third failed
   on the reason text only, because the old code already disqualified that case).
3. A refused lane release fails the job: `run-batch` refuses with the release's code and
   witnesses nothing, and `qualify` is `NOT_QUALIFIED` with `LANE_FAILED` (TOL-V0-030).
   `TestTOLV0030_RefusedReleaseFailsTheJob`, with an injected release refusal (before: the job
   credited and qualify was `QUALIFIED`).
4. Capture and release run on a fresh context bounded by the capture timeout, so an interrupt
   during the tests keeps the capture output (TOL-V0-030, 033).
   `TestTOLV0033_CaptureRunsAfterAnInterruptDuringTests` (before: no `server.log`).
5. The fixture digest for `.` is the commit's root tree id (TOL-V0-031).
   `TestTOLV0031_RootFixturePathIsDigested` (before: a root-level change stayed `LOOP_DETECTED`).
6. A refused acquire's code is the job's code, not `GATE_FAILED` (TOL-V0-030).
   `TestTOLV0030_AcquireRefusalCodeIsTheJobs` (before: `GATE_FAILED`).
7. The manifest streams each file through sha256 and records the copied size (TOL-V0-033).
   `TestTOLV0033_ManifestStreamsAnOversizedArtifact` with a 512 MiB sparse report: the manifest
   lists it, the verdict is `CANDIDATE_FAILURE` with no admissible report, and qualify allocates
   less than 384 MiB (before: 702,844,944 bytes allocated).

## Limits and NOT_RUN

- Live Playwright under `run-batch` and `qualify` is `NOT_RUN`. In production the qualified-version
  list is empty, so a real report still refuses `UNSUPPORTED_VERSION` and `run-batch` credits
  nothing until a live fixture qualifies.
- Not implemented: test-slot accounting and releasing the claim with HANDOFF inside the job.
- The interrupt tests replace the job context; a real `SIGINT`/`SIGTERM` to a running job is
  `NOT_RUN`.
- A command that changes only untracked files is not detected by the checkout re-check.
- `CORVINT_POOL_MEMBER` is the only way a configured command learns its lane; health preparation is
  whatever `pool acquire` already does.
