# Dogfood flow lane 1021t: per-hunk citation drafts and build interrupts

Date: 2026-10-21. Tickets: V1-2283 (with V1-2214 folded in), V1-1442, V1-1531.
Requirements `DCW-V0-150` and `DCW-V0-151` are accepted under the owner's standing auto-accept
(2026-10-11).

## Problem

- V1-2283 / V1-2214: lanes wrote one span for every hunk and hit `citation-plan-uniform`
  (`DCW-V0-048`), or met `empty-citation-plan` with no guidance on how to write rows. The uniform
  refusal named the rule but showed no example of a per-hunk plan.
- V1-1442: an interrupt while `script/dogfood-finish.sh` built its tree or base binary exited with
  no step line and no `NOT_PRODUCED` row, so the lane could not tell what was left undone.
- V1-1531: a repeated span (a Markdown table separator) is refused as not stable; the ticket asked
  for guidance naming a unique line and a test for that case.

## Decisions

- New read-only verb `corvint dogfood propose-citations BASE` (`DCW-V0-150`). It drafts one row
  per owed hunk from the prepared map: the nearest enclosing heading or declaration above the hunk
  that lies outside every hunk range of that path and occurs exactly once at both BASE and the
  working tree; otherwise the nearest such line. Governing instruction files and paths absent at
  BASE get `unknown` rows whose note starts `proposed:`. It never drafts a uniform plan. It does
  not write a plan file: a draft is a reviewer's starting point, not evidence that a line supports
  the hunk.
- The `empty-citation-plan` and `citation-plan-uniform` fix lines name the command; the uniform one
  shows a two-row spec-and-code example (V1-2214 AC1). V1-2214 AC2 (an unsupported verify command
  names the grammar) was already met on main by `internal/trace/record.go` `safeCommand`
  (fcc873e6e) and its existing fix hint.
- `dogfood-finish.sh` traps `INT`/`TERM` during the two builds only (`DCW-V0-151`) and clears them
  before `finish-lane`, which `DCW-V0-067` already covers.
- V1-1531 AC1 was already met by the `CEM-V1-322` refusal (74e28c356), which names the rule and
  the nearest stable base spans; this lane adds the table-separator test only.

## Evidence

- `TestProposeCitationsDCWV0150DraftsPerHunkPlan`, `...NeverDraftsUniformPlan`,
  `...RefusesWithoutMatchingMap`, `TestChangeDCWV0048RefusesUniformPlan`: PASS.
- Replay of #1457: a clone at cd2eb6049e with BASE 8def072e08 prepared a 26-hunk map;
  `propose-citations` drafted 26 rows (25 distinct cite rows, 1 unknown for the new build-log
  file). `dogfood change` with that draft as `DOGFOOD_CITATIONS` reported `cem-cite PRODUCED`, and
  the map held 25 supported hunks and 1 unknown `insufficient-evidence` hunk.
- `script/dogfood-cache_test.sh`: PASS; with the trap lines removed it fails (rc 1).
- `TestCEMV1322RepeatedTableSeparatorNamesUniqueSpansV11531`: PASS. It is a regression test of
  delivered behavior, so it has no failing pre-fix state.

## NOT_RUN

- `script/dogfood-change_test.sh` (gate-only shell suite; its empty-plan assertion was updated).
- Affected tier 2 packages, left to batch CI.

## Rollback

Revert this lane's commit: `propose-citations`, its hint text and the `dogfood-finish.sh` traps
disappear; no map, plan, receipt or sealed CEM format changes.

## Not delivered

- V1-2274: needs an owner decision on verifier independence (PR #1472).
- V1-2083: its inputs live in another lane's directory.
- V1-1638: design question (merged sealed CEM hunks shift ordinals); needs a decision.
- V1-2177, V1-2171: not started within the lane budget.
