# Dogfood flow friction: verify accepted form, cancel reason, staged begin route, merged-forward base

Date: 2026-10-21 (lane run 2026-10-22). Lane: claude/dogf-1022b (base f29ddc0db). Tickets delivered:
V1-2344, V1-2304, V1-2177, V1-0953, V1-2171. Requirements `DCW-V0-170`, `DCW-V0-171`
(`docs/specs/daily-change-evidence-workflow-v0.md`) and `LCP-V0-038`
(`docs/specs/local-completion-policy-v0.md`) are accepted under the owner's standing auto-accept
(2026-10-11).

## Problem

- V1-2344: `finish-lane` refused a verify file with a quoted `-run 'TestA|TestB'` as
  `unsupported-verify-syntax` without saying which form is accepted, so lanes rediscovered it.
- V1-2304: `script/dogfood-cache_test.sh` computed a relative path with `python3`, which the
  GOC-V0-008 no-Python-runtime check forbids (`make no-python-runtime-dependency-test` failed).
- V1-2177: `corvint dogfood cancel` closed an active, unsatisfied enrollment with no reason, which
  lanes used to make room for a new base, discarding selected checks and observations.
- V1-0953: a stage-2 `dogfood begin` after a satisfied stage-1 refused `prior-completion-stale`
  with no next action, and DOGFOOD.md did not describe the staged route across enrollments.
- V1-2171 (V1-1322 AC3): a lane that merged origin/main past its unmoved BASE ran the expensive
  passes before failing on a resource bound; nothing named the base to use.

## Decisions

- `requireVerifyInput` prints an `accepted form:` line for `unsupported-verify-syntax` only; the
  grammar is unchanged (`DCW-V0-170`).
- The test computes `../$(basename "$fake_dir")` in the shell and checks it names the same
  directory.
- `Cancel` takes a reason. An `active` enrollment without `--reason` refuses
  `cancel-reason-required` with the check and observation counts; the reason is echoed as
  `cancelReason` and never persisted, so the closed state member set and older readers are
  unchanged (`LCP-V0-038`). Satisfied or cancelled enrollments cancel without a reason.
- `prior-completion-stale` keeps its code and state behaviour; its CLI remedy names the staged
  route (a fresh worktree or clone per stage). This is AC1 option (b), refusal with a documented
  next action; no lifecycle change.
- DOGFOOD.md "Produce the CEM" decides V1-1322 AC1: a merged-forward lane binds against
  `git merge-base origin/main HEAD`. Because the accepted contract selects no implicit anchor,
  the unanchored case is an early `NOTE merged-forward` naming the merged parent; with
  `corvint.dogfood.anchor` it is the early `base-not-anchored` refusal, which now names the
  anchored base (`DCW-V0-171`). A merged seal commit is excluded (sealed lane or stacked
  predecessor routes).
- Hardening: `script/lane-scripts_test.sh` unsets `git rev-parse --local-env-vars`, so a leaked
  `GIT_DIR`/`GIT_INDEX_FILE` cannot redirect its fixture commits into the caller's repository.
  V1-2312 itself did not reproduce at f29ddc0 and stays open.

## Evidence

- New tests fail with the fix reverted and pass with it: `TestFinishLaneDCWV0092RefusesVerifySyntaxBeforePrepare`,
  `TestCancelActiveEnrollmentRequiresReason`, `TestDogfoodCancelActiveEnrollmentRequiresReason`,
  `TestDogfoodPriorCompletionStaleNamesStagedRoute`, `TestMergedForwardDCWV0171NamesTheMergedMainline`,
  `TestChangeDCWV0171NotesOrRefusesBeforeAnyStep`. `TestStagedBeginAfterSatisfiedStageRefusesAndKeepsIt`
  pins existing behaviour (passes before and after).
- `./internal/localcompletion` full package passed; `script/dogfood-change_test.sh`,
  `script/dogfood-cache_test.sh`, `script/lane-scripts_test.sh` and
  `make no-python-runtime-dependency-test` pass.

## NOT_RUN

- `./cmd/corvint` full package (narrow `-run` subsets only), tier 2 of `corvint affected`, and
  `make gate`; left to batch CI.

## Rollback

Revert the lane's commits. No state, schema or wire format changes: the cancel reason is not
persisted, and the notes and fix lines are stderr text only.
