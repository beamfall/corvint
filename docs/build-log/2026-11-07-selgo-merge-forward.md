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

# Selection: selgo merge-forward onto PR #123 (round 2)

Date: 2026-11-07
Merge commit `6fc7a03e1` merges `origin/main` at `59b31f12e` (PR #123, ci-speed) into the sealed lane
head `5063a2807`. The BASE is `script/dogfood-merge-base.sh`: `59b31f12ed984802627f3ef44fa918f3022045cd`.
Route: `docs/DOGFOOD.md`, "Rebinding after a conflicted merge-forward (V1-4721, V1-0527)".

## Conflicted paths (round 2)

`script/dogfood-merge-base.sh --paths` lists three paths. The merge used no `-X ours` or `-X theirs`
strategy option; every other path merged cleanly.

- `docs/specs/affected-plan-selection-v0.md`: two conflict blocks, both resolved by keeping both
  sides' new blocks, this lane's first and main's after it. The lane-attribution paragraph keeps the
  selaudit/selgo sentence and main's ci-speed sentence (AFP-V0-2700..2702). The requirement section
  keeps this lane's AFP-V0-2610 and AFP-V0-2740..2748 blocks and main's AFP-V0-2700..2702 block.
- `docs/specs/README.md`: the `affected-plan-selection` row. Resolved by taking main's row, which
  carries the ci-speed claim sentence, with the union range bound `AFP-V0-548..2748`.
- `docs/specs/REQUIREMENTS.tsv`: generated; regenerated with `make spec-requirements-resolve`
  (`docs/specs/INDEX.json` was regenerated the same way).
