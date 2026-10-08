# 2026-10-08: role selection of answered waits by stage (V1-0827)

## Intent

Native ticket V1-0827 (observed on Corvint 1.0.0-rc.1 build 163 by the issue 354 lane, not retained
as a regression) reports that `corvint-tasks run --role implementer` selects answered `WAITING`
attempts whatever their stage, so an implementer run can resume a review or integrate stage. The
expected behavior is that the role filter applies to the stage of the waiting attempt. The change
adds proposed requirement CAL-V0-197 to `docs/specs/corvint-tasks-agent-leases-v0.md`, pending owner
acceptance. CAL-V0-191..196 are held by other in-flight work; CAL-V0-197 was free on `origin/main`
(`388fb832`).

## Defect confirmed

`TestCALV0197_RunRoleSelectsAnsweredWaitOfItsStage` (`internal/tasks/store`) builds an integrate
stage stopped at its wall under a continuation policy, answers it with an operator `retry`, and
then drives `run --program program --config ... --role implementer` through `cli.Run`. At the base
the run selected the integrate wait and failed `ERROR` `MALFORMED` with the native refusal
`transaction: stage dispatch phase`: the implementer run had opened the integrate attempt's
workflow and tried an implement-stage `DISPATCH` on it. The selection in `internal/tasks/cli/program.go`
tested `a.Phase == "WAITING" && a.Supervision.Answer != ""` without the stage.

The reviewer and integrator branches selected no answered wait, so an answered review or integrate
wait was reachable only through `resume` or `retry`, although CAL-V0-089 item 5 already resumes an
answered integrate wait under its recorded grant inside `RunRole("integrator")`.

## Decisions

- **One predicate.** `store.RoleSelects(stage, attempt)` is now the selection for `run` and `admit`
  in the CLI and for `reselected`, which classifies a `STOPPED` as reselectable for CAL-V0-078
  retryability and previously mirrored the CLI by hand.
- **Stage-matched waits for every role.** An answered wait is selected only when its recorded stage
  is the role's stage, for all three roles. Restricting only the implementer would have left an
  answered review or integrate wait unselected by any `run`; with the change `run --role integrator`
  resumes it, as the test proves (the attempt reaches `COMPLETED` with exactly one integration
  commit).
- **Nil-safe.** The CLI branch dereferenced `a.Supervision` unguarded; the shared predicate checks
  it, as `reselected` already did.
- Every other phase selection is unchanged; `resume`, `retry`, `answer`, `drain` and `cancel` are
  unchanged.
- **A new question is unanswered.** The independent review (Codex, P2) found that `DISPATCH` keeps
  the answer of the wait it resumes, and `STOPPED` entering `WAITING` with `host result
  unavailable` (implement) or `repair bound reached` (review) did not clear it, so a role run would
  reselect that new question on the old answer until a cap refused. The implement case already
  looped this way at the base; selecting review waits would have extended it to review. The shared
  new-question branch in `internal/tasks/transaction/supervisor.go`, which already regenerates
  `questionId`, now also clears the answer. The other new-question paths already cleared it.

## Evidence

- `TestCALV0197_RunRoleSelectsAnsweredWaitOfItsStage`: failing at the base as above; passing after,
  where the implementer run returns `OK` with no items and leaves the attempt (phase, stage,
  generation, answer, turns), the program record and the host session untouched, and the
  integrator run then resumes and integrates once.
- `TestCALV0197_RoleSelectsByStage`: the predicate over every role and stage for answered,
  unanswered and unsupervised waits, and the unchanged per-role phases.
- `TestCALV0197_NewQuestionClearsResumedAnswer`: at the base the resumed stop into `host result
  unavailable` kept `operator answer`; after the change it is unanswered and not selected.
- Focused `internal/tasks/cli`, `internal/tasks/store` and `internal/tasks/transaction` packages
  pass.

## Limits

Live supervised-host qualification is NOT_RUN; the evidence uses the fake Codex host fixture. The
full repository gate is NOT_RUN per the owner's scoped-issue preference.
