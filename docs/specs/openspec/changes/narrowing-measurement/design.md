# Design: narrowing-measurement

Status: proposed (2026-11-09). Requirement text lives in `specs/narrowing-measurement/spec.md`.

## Decisions

1. A repository tool, proposed as `tools/narrowing-measure`, not a product verb, replays a frozen PR
   list per runner and writes one report per runner.
2. Narrowed means tier `line` or `file`; `flow` does not count (owner decision R16 (2026-11-09)).
3. Durations come from change `per-test-durations`; the excluded-test share is at most 5 percent
   (owner decision R19 (2026-11-09)).
4. Replay source: merged PRs in the 12 months before the manifest SHA, at least 100 per runner. The
   200-PR Go replay of Corvint is a feasibility check only (owner decision R19 (2026-11-09), review
   release-tasks 13).
5. The narrowing-rate bar and the cost-ratio bar are linked, and the report states the link
   (review release-tasks 13).
6. Wilson interval for every rate (settled release-tasks S6).
7. Selective means zero-miss FIH and not DEMOTED (settled release-tasks S5, review release-tasks
   22).
8. Verdicts are judged on the milestone M5a candidate SHA (owner decision R2 (2026-11-09)) and
   confirmed on the RC SHA after the freeze (review release-tasks blocker 2).
9. Kill-switch grammar `full` and `full:<runner>[,<runner>]`, owned here; unknown values exit 2 on
   CLI and Action and select all tests in agent hooks and MCP (owner decision R13 (2026-11-09)).
10. The agreement figure of owner decision R20 (2026-11-09) is reported (NMS-V1-011).
11. pytest is measured at tier `file`; its file-level selection is a recorded release exception
    (owner decisions R8 and R9 (2026-11-09)).
12. Cypress is dropped from 1.0 and listed as post-1.0 (owner decision R6 (2026-11-09)).

## Failure modes

- Per-test durations are missing for a runner: the cost ratio is `NOT_MEASURED`, the runner cannot
  PASS, and the report names the gap. It is never estimated from file counts.
- The baseline full run fails or is flaky beyond the G3 quarantine cap: the PR is excluded with a
  named reason; if excluded tests exceed 5 percent of the suite (owner decision R19 (2026-11-09)),
  the runner's result is `INVALID`.
- Fewer than 100 merged PRs in the replay window: `INVALID` with `replay-source-too-small`.
- The bar file changes after the freeze: the measurement refuses and names the hash mismatch.
- The kill switch has an unknown value or names an unknown runner: exit 2 with a typed code on CLI
  surfaces and in the Action; all tests selected in agent hooks and MCP (owner decision R13
  (2026-11-09)).
- The miss proof for a runner is absent or failed: the runner's status is FAIL whatever its
  narrowing numbers.
- A post-freeze RC verdict contradicts the M5a verdict: the runner is DEMOTED.

## Acceptance evidence

- The 200-PR Go feasibility replay report on Corvint, with the frozen bar file and its hash
  committed.
- One report per runner (seven) on its pinned repository at the M5a candidate SHA, confirmed at the
  RC SHA.
- Go tests for each requirement on a fixture corpus: below-bar, flow-not-narrowed, missing
  durations, publish-only, miss-first, edited bar, too few PRs, excluded share over cap,
  three-of-seven slip, kill switch global, per-runner, CLI typo and hook typo, agreement figure.
- RC qualification transcript showing both kill-switch forms forcing FULL.

## Rollback

The kill switch is the first rollback for a bad selection in the field (G10 order: kill switch,
then a 1.0.1 forward fix, then withdraw; tags are never deleted). To withdraw this change, revert
its commits; the measurement tool and report schema go, the IDs are retired, not reused.

## Open questions for the owner

- Decided, R42 (panel pick) A10: the 5 percent excluded-test cap (owner decision R19) and the G3
  2 percent quarantine cap both apply to the same baseline, each on its own set (NMS-V1-006).
