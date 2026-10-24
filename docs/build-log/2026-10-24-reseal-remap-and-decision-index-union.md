# Reseal after merging main, and union-merged decision index rows

Date: 2026-10-24. Lane: claude/dogfood-docs-1024 (V1-1638, V1-0332). Specs:
`docs/specs/daily-change-evidence-workflow-v0.md` (`DCW-V0-340`, `DCW-V0-341`) and
`docs/specs/decision-number-gate-v0.md` (`DNG-V0-009`). Test platform: linux/WSL2.

## Owner authority

Both designs follow the operator decisions recorded in
`docs/build-log/2026-10-24-owner-held-decisions.md` (PR #1721). The requirements are accepted
under that delegation.

## Decisions

- V1-1638. A reseal after merging origin/main reuses the prior ordinal plan. When the recorded
  citation-plan binding explains every moved ordinal row by inserted hunks on admitted sealed-CEM
  paths, `dogfood change` remaps the rows in memory and prints
  `NOTE citation-plan-remapped`. Any other shift still refuses with the `DCW-V0-066` fix line.
  The plan file and the recorded binding are never rewritten, so a rerun remaps the same way.
  Without a recorded binding (a fresh worktree) nothing is remapped.
- An unknown admitted sealed-CEM hunk that the plan does not name is cited against the
  `DCW-V0-042` bullet as BASE holds it, with relation `specification`, by one appended `cem cite`.
  The span is computed from BASE, not hard-coded. When the bullet is missing, the hunk stays owed.
  Auto-citing was chosen over leaving the hunk unknown: `dogfood check` recomputes the unknown
  allowance from the map, so an unknown seal hunk would fail check and seal.
- V1-0332. `.gitattributes` gives only `docs/decisions/README.md` the `merge=union` driver.
  `REQUIREMENTS.tsv` is generated and stays out (decision 0543). `check-decision-numbers.sh`
  remains the fail-closed check on any union result.

## Encoding

No map, plan, binding, report or sealed CEM format changes. Existing artifacts stay readable.

## Limits

- Whether GitHub's own merge honors `merge=union` is NOT_OBSERVED.
- `dogfood batch-check` replays merges with `git merge-tree` in a scratch repository that has no
  attributes, so a union-resolved index may show as an unbound merge resolution, as a manual
  resolution already does.
