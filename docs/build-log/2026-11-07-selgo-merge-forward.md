# Selection: selgo merge-forward onto PR #124

Date: 2026-11-07
Lane: selgo (branch `claude/sel-go`), merge-forward round 1. Merge commit `4ba63b957` merges
`origin/main` at `538e823f4` (PR #124) into the sealed lane head `7be1ab31a`.
Route: `docs/DOGFOOD.md`, "Rebinding after a conflicted merge-forward (V1-4721, V1-0527)".

## Conflicted paths

`script/dogfood-merge-base.sh --paths` lists one path:

- `docs/specs/README.md`: the three adjacent registry rows for `affected-plan-selection`,
  `affected-zero-unknown` and `daily-change-evidence-workflow-companion`. Resolved by taking
  main's rows (`AZU-V0-001..9950`, `DWC-V0-001..7603`) and keeping this lane's range bound
  `AFP-V0-548..2748`. `make spec-requirements` reported no changes.

The merge used no `-X ours` or `-X theirs` strategy option. Every other path merged cleanly.
