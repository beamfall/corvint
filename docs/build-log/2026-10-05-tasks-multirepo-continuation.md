# Multi-repository integration and checkpointed continuation (issue 354)

Date: 2026-10-05
Owner intent: GitHub #354 (2026-10-05: finish the remaining code so the issue can close); native
V1-0475, suspected bugs V1-0772 and V1-0771.
Requirements: CAL-V0-086 (amendment), CAL-V0-087, CAL-V0-088 and CAL-V0-089 (S21) in
`docs/specs/corvint-tasks-agent-leases-v0.md`. Base `d524530fc9f53e62a14a3918a760b6df9d80f66e`.

## Already on main at the base

Checked by their requirement and traceability rows at `d524530f`, not re-implemented:

- the writer one-pass work (CAL-V0-070);
- the Claude Code and OpenCode supervisor hosts (V1-0755/V1-0756, CAL-V0-074..077);
- the per-stage effort allowlist and a stage wall of up to 240 minutes (S13, CAL-V0-062/063).

## V1-0772 and V1-0771: long work root and unproved stops (CAL-V0-086)

- **Root cause.** The attempt decoded `worktreePath` as a 128-byte Identifier. A stage worktree is
  `<work root>/<program>/<assignment>/<generation>/<stage>-<turn>`. Under a resolved `TMPDIR` longer
  than about 81 bytes, DISPATCH refused `LIMIT_EXCEEDED` after the worktree and program records
  already existed. This part is deterministic, not load-dependent.
- **Load-dependent part (V1-0771).** On Darwin, `kill(-group, 0)` answers `EPERM` for a group whose
  only member is an unreaped zombie, so the drain reported survivors. The role then journaled
  `FINISHED` over `BLOCKED_RECOVERY` and was refused `MALFORMED: program transition`. Separately,
  the stage watcher cancelled a running stage after one unlocked read failed
  `MALFORMED: staging/aNN: unassigned stage slot`.
- **Fix.**
  - `worktreePath` is now a PathText of at most 4096 bytes.
  - The stage refuses an uncarriable path, or `@name` sibling path, before any mutation.
  - An unproved drain ends the role with a non-retryable `SURVIVORS` error and journals nothing
    `FINISHED`.
  - The drain re-probes `EPERM` until it sees `ESRCH` or its deadline passes.
  - The watcher tolerates 30 seconds of continuous read failure; a successful read resets the window.
- **Evidence.**
  - At the base:
    - `TestCALV0072_MultiRepositoryGatesFailClosed` with a 113-byte resolved `TMPDIR` fails
      `VALIDATION_FAILED` (the V1-0772 symptom).
    - The new `TestCALV0086_*` tests fail, including
      `LIMIT_EXCEEDED: /worktreePath: Identifier longer than 128 bytes (225)`.
    - Before the role fix, an unproved stop returned `MALFORMED` instead of `SURVIVORS`.
  - With the fix:
    - All of these pass.
    - Acceptance: `go test -count=10 -run TestCALV0072_MultiRepositoryGatesFailClosed` passes under
      the 113-byte `TMPDIR` and under concurrent store load; a separate `-count=10` run also passes
      with the default `TMPDIR`.
    - `go test -count=20 -timeout 30m -run 'TestCALV0075_ClaudeCodeProgramFakeHost|TestCALV0077_OpenCodeProgramFakeHost'`
      passes (706.6s). An earlier attempt hit Go's default 10-minute timeout at 600.7s, not a
      test failure.

## Multi-repository gates, integration and context (CAL-V0-087..088)

- **Gates.** A required gate observes the composite of the primary worktree `HEAD` and every
  sibling's `HEAD` commit. It is clean only when every worktree is clean. A direct `gate run`
  observes the same composite.
- **Designation.** Integration needs a per-repository designation, `integrationBranch` in the config
  together with `ownIntegrationCheckout`. The designation and the checkout's directory identity
  become part of the immutable repository binding.
- **Grant scope.** The INTEGRATE grant scope names every repository and its branch. A
  single-repository grant cannot authorize a multi-repository landing.
- **Landing.** After `INTEGRATE_INTENT`, under the store lock, the supervisor fast-forwards each
  changed repository in name order, with hooks disabled, and the queue checkout last. A restarted
  integrator skips landed repositories. It stops `BLOCKED_RECOVERY` when the queue checkout is
  already at the candidate but a repository candidate is missing from its branch.
- **Undesignated repositories.** A changed repository without a designation still refuses
  `UNSUPPORTED`. An unchanged one is never moved.
- **Context.** Every stage obtains one Core context packet per repository, from its sibling
  worktree. Any failure refuses the stage before the host is launched; it never degrades to
  primary-only context.
- **Mutation check.** One run applied three mutations together:
  - drop the landed-repository skip;
  - drop the sibling cleanliness conjunct;
  - always use the single-repository grant formula.

  Each was killed by its own test: `TestCALV0087_InterruptedIntegrationLandsOnce`
  (`TARGET_ADVANCED`), `TestCALV0072_MultiRepositoryGatesFailClosed` (dirty sibling not refused)
  and `TestCALV0087_GrantMustNameEveryTarget` (partial grant accepted).

## Checkpointed continuation (CAL-V0-089)

Criterion 5 of V1-0475 asks for stages beyond one hour, or checkpointed continuation that preserves
the session and worktrees, with correct ownership across interruption, restart and expiration, and
without duplicate integration. The stage wall already reaches 240 minutes (CAL-V0-063). This slice
adds the checkpointed path.

Decisions:

- **Policy, not config.** Policy `supervision.continuations` is a canonical count from 1 to 16. The
  bound lives in the policy so that the owner, not the operator's local config, decides how much
  unattended wall time a stage may consume. 16 continuations of a 240-minute wall is 64 hours.
  Owner question: is 16 the right ceiling?
- **Continue only own-wall stops.** The supervisor records whether its own stage wall stopped the
  run (`DeadlineExceeded` from the stage context, caller context live, and the program wall not
  binding).
  - The program wall, a control, a heartbeat refusal, a failed watch read and caller cancellation
    all end the role as before.
  - The stage must also have checkpointed cleanly:
    - `WAITING` on the same stage, unanswered;
    - a recorded session and preserved worktree;
    - no live worker;
    - quiescence `PROVED`.
  - It re-reads program records before each continuation and fails closed on a read error. Owner
    question: should a transient read error be retried instead?
- **Ordinary transitions.** Each continuation is an ordinary ANSWER (`checkpointed continuation n
  of N`) and DISPATCH. Ticket and policy revision fencing, the lane turn cap and the shared program
  turn and wall caps therefore still apply. The supervisor also checks those caps itself, so a
  capped program stops with its question unanswered rather than with a refused ANSWER.
- **Same session, same worktree.**
  - Codex resumes with `exec resume <session>`. OpenCode resumes with `--session <session> --fork`
    (CAL-V0-077).
  - An implement stage stopped at its wall already commits a per-turn candidate ref, so partial
    work survives even if the host session is lost.
- **Integrate once.** An integrate continuation reuses the recorded grant and records no second
  one. The queue checkout moves only after `INTEGRATE_INTENT`, at the end of the last turn.
  - The operator `retry` path skips GRANT for an answered integrate wait with a recorded grant.
  - A different nonempty `--grant` is refused `APPROVAL_MISSING`.
- **Fail closed on hosts that cannot resume.**
  - Claude Code reports its session only in its final result, so a wall-stopped turn has no session
    to resume. A policy with `continuations` and host `claude-code` is refused `UNSUPPORTED`.
  - No supported host reports the token usage of an interrupted turn, and DISPATCH would refuse
    every continuation as `lane token usage unknown`. A nonzero lane or program token cap is
    therefore refused `UNSUPPORTED` as well.
  - Both checks run for a new program before any record or process exists, and again at every stage
    launch. Owner questions: should Claude Code continuation be built (it would need a fixed
    `--session-id` at launch or a streamed session event), and should token caps keep refusing?
- **No automatic continuation by `run`.** `run` never continues an unanswered checkpoint. Only the
  owning role invocation, or an explicit operator `retry`, does.

### Evidence

- **Tests**, all passing. They use fake hosts, a pinned Core, the test binary as lane leader, and
  real Git repositories.
  - `TestCALV0089_PolicyContinuationsBound` (intent): decoding and bounds.
  - `TestCALV0089_InterruptedSessionCapability` (supervisor): Codex and OpenCode report the
    capability; Claude Code does not.
  - `TestCALV0089_StageRechecksContinuations` (store, internal): the stage-launch recheck refuses
    Claude Code and both token caps.
  - In `internal/tasks/store`:
    - `TestCALV0089_CodexContinuationResumesPreservedSession`
    - `TestCALV0089_OpenCodeContinuationForksPreservedSession`
    - `TestCALV0089_ContinuationBoundThenOperatorRestart`
    - `TestCALV0089_TurnCapsBoundContinuation` (program and lane)
    - `TestCALV0089_DrainStopsContinuation` (while running, at the wall, and before admission)
    - `TestCALV0089_ProgramWallExpiryEndsContinuation`
    - `TestCALV0089_IntegrateCheckpointRestartKeepsGrant`: a stuck integrate turn checkpoints with
      the grant and no effect; after restart, `retry` resumes `integrate-session`, lands exactly one
      commit and completes.
    - `TestCALV0089_UnsupportedContinuationRefusedBeforeMutation`
- **Mutation check.** Each mutation was applied alone and run against its named test, with the
  files restored and verified by checksum afterwards.

  | Mutation | Result |
  |---|---|
  | M0 no continuation loop | killed |
  | M1 drop `!programBound` | survived |
  | M1b M1 plus drop the program wall check | killed |
  | M2 drop the policy bound | killed |
  | M3 drop the control check | killed |
  | M4 drop the `DeadlineExceeded` condition | survived |
  | M4b M4 plus drop the control check | killed |
  | M5 drop the lane turn check | killed |
  | M5b drop the program turn check | killed |
  | M6a drop the host capability check | killed |
  | M6b drop the token cap check | killed |
  | M6c drop the stage-launch recheck | killed |
  | M7 drop the integrate grant reuse | killed |

  - M1 survives because `continuable` also refuses an elapsed program wall.
  - M4 survives because a control-caused cancel is also caught by the pending-control check. The
    heartbeat-refusal and failed-watch-read cancel paths have no separate test.
  - M6c first survived. `TestCALV0089_StageRechecksContinuations` was added for it and kills it.
- **Package runs:**
  - `go test -count=1 -timeout 30m ./internal/tasks/... ./internal/specindex`
  - `go vet ./internal/tasks/...`
  - `gofmt`
  - the doc gates: `spec-requirements-check`, `requirement-definitions-check`,
    `traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
    `error-code-ownership-check`, `unbounded-readers-check`, `use-case-receipts-check` and
    `use-case-receipts-test`
- **Affected plan.** `corvint affected --base d524530f` (Corvint 1.0.0-rc.1 build 163) selected 181
  units with scope `UNKNOWN` and 24 `LANGUAGE_FRONTIER` unknowns. Most were reached through the
  documentation edits. Units outside `internal/tasks/...` and `internal/specindex` are `NOT_RUN`,
  per the owner's focused-test preference. `cmd/corvint/main.go` is unchanged, so no use-case
  receipt repin was needed.

## Review

**Codex round 1** (`codex exec -s read-only`, `gpt-6-astra`, diff `d524530f..92702b02`) found no P0
or P1 and two P2 findings. Both were verified and repaired.

1. **A pending drain or cancel could race automatic continuation.** A control recorded after
   `continuable` read the program did not stop the continuation's ANSWER, admission or DISPATCH.
   - Verified: without the repair, the new subtest's drain, recorded once the continuation answer
     is recorded, returned success at once because the program was already `FINISHED` and
     released. The continuation then launched, and the resumed host built to completion.
   - Repair: the continuation's admission, the `READY` its resumed stage writes first, goes through
     a fenced program transition. That transition refuses `FENCED` when the `programs.json` revision
     it replaces carries a control. It binds the same revision as its expected digest, so the check
     and the write are atomic against a concurrent control request. A control recorded after
     admission is the ordinary running-stage case, which the watcher sees within one poll; this is
     documented as a failure mode.
   - Test: `TestCALV0089_DrainStopsContinuation/before_admission`.
   - Mutations: forcing the fence flag off, and dropping the guard in `programTransition`. Both
     were killed.
2. **A failed execution masked an unproved drain.** A wall timeout together with an escaped
   `setsid` process reached the caller as `MALFORMED context deadline exceeded`.
   - Repair: `SURVIVORS` now takes precedence and keeps the stage error's text.
   - Test: `TestCALV0086_UnprovedStopIsNotFinished/after_the_stage_wall`.
   - Mutation: restoring the `runErr == nil` condition was killed.

**Codex round 2** (diff `d524530f..e9292377`) confirmed the continuation fence and found no P0 or
P1 and one P2, verified and repaired.

3. **An unproved stop over an existing candidate was checked as a read-only stage first.** A stage
   whose attempt already carries a candidate tree (review, or an implement repair) ran the
   read-only candidate check before `STOPPED`. When that stage both changed the worktree and left
   an escaped process, the role returned `read-only stage changed candidate` with the attempt left
   `STOPPING`, losing the `SURVIVORS` diagnosis. The same branch structure is on `d524530f`.
   - Repair: the candidate check runs only after a proved drain; an unproved stop goes straight to
     `STOPPED`, which classifies it `SURVIVORS` and ignores the tree.
   - Test: `TestCALV0086_UnprovedStopIsNotFinished/over_a_candidate_it_changed`, driven by a new
     fake-host `review-escape` switch that edits the candidate and leaves a `setsid` process.
   - Mutation: dropping the `out.Clean` condition was killed (`MALFORMED read-only stage changed
     candidate`).

**Codex round 3** (diff `d524530f..a23e098f`) reported no P0 to P3 findings. Its sandbox could not
run Go tests or Corvint impact checks, so that round is source review only.

## Live qualification (NOT_RUN)

No live hosted-agent run was made. Steps for the owner, on darwin/arm64, with the hosts installed
here: Codex CLI 0.153.2, Claude Code 2.1.267 and OpenCode 2.0.21.

1. In a disposable fixture store with one P1 ticket, set up the policy and config:
   - `host` `codex` in both the policy `supervision` object and the config;
   - config `wallSeconds` `60`;
   - policy `supervision.continuations` `"2"`;
   - lane and program token caps `"0"`.
2. Run `corvint-tasks run --program P --config C --role implementer --host codex`, using a ticket
   whose implement turn needs more than one minute.
3. Expect two `checkpointed continuation` answers on the attempt, the same Codex session ID in each
   `exec resume`, and per-turn refs under `refs/corvint/tasks/`.
4. Expect either `REVIEW` or a final `WAITING` with the question unanswered, followed by a
   successful `corvint-tasks retry --program P --config C --role implementer`.
5. Repeat with host `opencode` and expect `--session S --fork` resumes.
6. Repeat with host `claude-code` and expect `UNSUPPORTED` before any program record.
7. For multi-repository integration, add two designated checkouts and record the per-repository
   fast-forwards and one `INTEGRATED`.

## Out-of-scope findings (reported to the coordinator, not filed by this lane)

1. **Suspected reader defect.** An unlocked journal read can fail `MALFORMED: unassigned stage
   slot` while a concurrent writer stages (`internal/tasks/journal/stage_read.go:57`; the lease-proof
   reader retries only `SNAPSHOT_MOVED`). `RequestProgramControl` polls through it, so an operator
   `drain` can report failure although the control was recorded. The continuation drain test
   retries around it.
   - Version: Corvint 1.0.0-rc.1 build 163, Go 1.27.1, darwin/arm64.
   - Expected: a retry or a consistent snapshot. Actual: `MALFORMED`.
2. `CleanupWorktrees` has no CLI caller.
3. A reassign in `OpenWorkflow` leaves the previous stage worktrees behind.
4. The CAL-V0 spec status repeats its `S19 CAL-V0-069` clause, and the slice table repeats its S19
   row.
5. CLI `run --role implementer` reselects answered `WAITING` attempts of any stage.
6. `gate run` requires an unexpired lease after an idle gap.
7. Program transition refusals are plain errors, so their wire code is lost to `wire.CodeOf`.
8. The `retry` usage text says `--grant FILE`, but the value is a grant ID.
9. **Suspected, not reproduced.** The owner's phase writes do not fence on a control. `persist` and
   the run journal callback copy `Control` from an unlocked read, but `ProgramTransition` binds the
   revision it reads later. A control recorded between those two reads is overwritten with the
   stale value. `RequestProgramControl` then reports `control superseded`, which fails loudly but
   loses the control. The continuation admission fence above covers only the continuation's first
   write. The pattern predates this branch (`persist` at `d524530f`).
10. **Reproduced with a scratch probe (not retained).** A review stage that exits cleanly after
    changing the candidate worktree returns `read-only stage changed candidate` as a plain,
    retryable `MALFORMED` error. It leaves the attempt `STOPPING` with its worker flag set, and a
    rerun of `run --role reviewer` is refused `RESOURCE_COLLISION`. Expected: a non-retryable
    typed refusal and a recoverable or blocked attempt. No test covers this clean path. The branch
    structure predates this branch (`d524530f`).

## Owner questions

- **worktreePath.** Is widening `worktreePath` to a 4096-byte PathText acceptable as a one-way
  downgrade?
- **Watcher tolerance.** Are the 30-second watcher tolerance and the `EPERM` re-probe acceptable?
- **Per-repository context.** Should Core context stay required for every repository, or may a
  stage degrade to primary-only context?
- **Designation scope.** Are the designation shape (a label plus the recorded identity) and the
  grant scope right? `integrationBranch` is validated only as a label.
- **Downgrade.** Is the one-way downgrade from multi-repository records acceptable?
- **Partial landing.** Is keeping an earlier repository landed when a later one is
  `TARGET_ADVANCED` acceptable?
- **Continuations.** Is the ceiling of 16 continuations right?
- **Token caps.** Should continuation stay refused under token caps?
- **Claude Code.** Should Claude Code continuation be built?
- **Read errors.** Should `continuable` keep failing closed on a transient read error?
- **ID allocation.** This change takes CAL-V0-086..089. CAL-V0-089..094 were unused in every ref
  and lane checked. The coordinator should confirm the allocation.

## Rollback

- **CAL-V0-089.** Remove `continuations` from the policy; every wall stop then returns to the
  operator `retry` path. A binary without CAL-V0-089 refuses a policy that carries it
  (`unknown field`).
- **CAL-V0-087.** Remove `integrationBranch` from the config; a later program's changed
  repositories then refuse integration again.
- **CAL-V0-086.** Revert the decoder and the workflow together. A binary without CAL-V0-086 refuses
  an attempt whose recorded `worktreePath` is longer than 128 bytes.
