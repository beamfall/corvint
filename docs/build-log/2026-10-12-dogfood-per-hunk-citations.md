# 2026-10-12 Dogfood per-hunk citations and a chore class for bind and seal maps (V1-1281)

Branch `claude/v1-1281-citations` from `origin/main` 1f35e3da7. Requirements `DCW-V0-048` and
`DCW-V0-049` in `docs/specs/daily-change-evidence-workflow-v0.md` and `CEM-V1-106` in
`docs/specs/cem-stable-v1.md`, each accepted under the owner's standing auto-accept (2026-10-11).
Input: the V1-1262 citation baseline (`2026-10-12-cem-citation-baseline-593-maps.md`).

## Intent

The V1-1262 baseline found 180 of 592 committed maps invariant, 171 of them citing exactly one
evidence item. Every one of those 171 is a change map, not a self-referential bind or seal map:
161 have more than one hunk and 10 have one, and they were bound in commits whose subjects say
`chore`; the bind loop accepted their plans, which cite one span for every hunk. The
producer should refuse that shape for a real change, and readers of the citation measure should
be able to keep genuine bind and seal maps apart once the stable producer can mark them.

## Requirements

1. `DCW-V0-048`: a change is a chore when `git diff --no-renames --name-only BASE HEAD` names at
   least one path and every path is exactly `.corvint/change.cem.json` or
   `.corvint/changes/NAME.cem.json` (NAME one nonempty segment; a quoted path never matches).
   After the `DCW-V0-019` binding, `dogfood change` refuses a uniform plan as `cem-cite
   NOT_PRODUCED citation-plan-uniform`, citing nothing, unless the change is a chore. Uniform: at
   least two map hunks, no unknown row, one `PATH` and `START:END` across every citation row
   (relation ignored), and every hunk named by ordinal or full ID. The `fix:` line names
   `hunks=N span=PATH:START:END`.
2. `DCW-V0-049`: when the prepared map declares `cem/1.0` and the change is a chore, the run
   writes `role` (`seal-chore` if a `.corvint/changes/` path is touched, else `bind-chore`)
   through the stable encoder before any cite; never on a `cem/0.x` map. `tools/cem-citation-baseline`
   reproduces the baseline rows and summary with a class per row and a summary per class.
3. `CEM-V1-106`: `cem status --citation` on a `cem/1.0` map adds the envelope member
   `citationClass` (`{"class": "change"|"chore", "role": ROLE}`) beside `stableVerification`,
   which stays identical to `cem verify-stable`.

## Decisions

- The refusal is exact-shape only: two spans, a kept-unknown hunk or a partial staged plan pass.
  Judging whether a span supports its hunk stays with the measured tiers (`CEM-V1-080..082`).
- The role write sits after `cem-prepare` in `run()` (`internal/dogfoodflow/chore.go`
  `markStableChoreMap`). Core `cem prepare` writes `cem/0.2` today and `cem-cite` still refuses a
  `cem/1.0` map (`CEM-V1-032`), so the write is dormant on the live path until V1-1276; it is
  tested directly with the stable fixture and with a mocked stable prepare.
- The class is an envelope member, not a result member, so `CEM-V1-095`'s "the result MUST NOT
  echo it" holds and the sealed result shape, `RESULT.schema.json` and the portable consumer are
  unchanged (`make cem-stable-regen-check` is not affected).
- The baseline tool runs `cem status` in process (`internal/cem/cli.Run`): the installed
  `corvint` predates `--citation` and refuses it as `invalid-arguments`. A `cem/0.x` map has no
  role, so the tool classes it by its `BASE..TARGET` paths with `dogfoodflow.ChoreRole`.
- Three existing tests used uniform two- and three-hunk plans and now cite distinct spans
  (`TestChangeDCWV0046...`, `TestChangeRefusesAPlanWhoseOrdinalsMoved`, the portable
  `testDogfoodFinishFromBinary`). `script/dogfood-change_test.sh` plans were checked by inspection
  only: none is uniform under the rule (extra unnamed hunks, a mismatch first, or partial plans).

## Failure modes

- A plan can still pass with two near-identical spans or one kept-unknown hunk; the reason and
  the tier measure make that visible, the refusal does not.
- Before V1-1276 no committed map carries a role, so every class comes from paths.
- A stable chore map that cannot be rewritten is treated as not prepared in that run.

## Acceptance evidence

- `TestChoreRoleDCWV0048ClassifiesOnlyCEMPaths`, `TestChangeDCWV0048RefusesUniformPlan`
  (`internal/dogfoodflow`); `TestDogfoodReasonAdmitsCitationPlanUniform`
  (`internal/observations`).
- `TestMarkChoreRoleDCWV0049WritesStableRoleOnly`, `TestChangeDCWV0049MarksPreparedStableChoreMap`
  (`internal/dogfoodflow`); `TestBaselineDCWV0049SplitsRowsByCitationClass`,
  `TestBaselineRowKeepsThePythonEncoding` (`tools/cem-citation-baseline`).
- `TestCEMV1106StatusCitationClassGroupsChoreMaps` (`internal/cem/cli`).
- Tool reproduction against the V1-1262 evidence (verification of the tool, not the criterion 3
  rerun): over the 604 maps now under `.corvint/changes/` the tool ran in 8m09s and exited 0. All
  593 baseline rows are identical to `evidence/v1-1262/citation-baseline-593-maps.jsonl` apart
  from the added `class`; removing the 11 newer maps from its `all` summary gives exactly
  `citation-baseline-summary.json`, histogram key order included; `f0fda542` again fails
  `git-budget-exceeded`. All 604 maps are class `change`: none of the committed maps is a chore
  by the `DCW-V0-048` rule, and 10 of the 11 newer maps are invariant.

## Retained limits and NOT_PRODUCED

- Criterion 3 (rerun of the baseline split by role class after the cutover): NOT_PRODUCED pending
  V1-1276. V1-1277 runs, from the repository root after the cutover lands,
  `GOTOOLCHAIN=local go run ./tools/cem-citation-baseline --root . --maps .corvint/changes --summary docs/build-log/evidence/v1-1277/citation-baseline-summary.json > docs/build-log/evidence/v1-1277/citation-baseline-maps.jsonl`.
  For `cem/1.0` maps the tool passes no `--artifacts`; if converted maps need their artifacts to
  reach a citation block, V1-1277 must extend the tool rather than read their rows as errors.
- The role write and `citationClass` are live only for `cem/1.0` maps; V1-1276 must cite this
  ticket (`DCW-V0-049`) before its cutover lands.
- The committed maps are not re-bound; the 171 single-evidence invariant maps remain in history.
- `make dogfood-change` was not run on this lane (caller instruction); the repository-wide gate
  is NOT_RUN under the owner's focused-test preference.

## Rollback

Revert the V1-1281 commits. A uniform plan is cited again, a stable chore map keeps no role,
`cem status --citation` loses `citationClass`, and the tool disappears. No committed map carries
a role, so nothing needs migrating; a later stable map with a role stays valid under `CEM-V1-095`.
