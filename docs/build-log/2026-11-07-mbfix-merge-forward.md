# mbfix merge-forward round 1: origin/main 538e823f4 (PR #124)

Date: 2026-11-07. Branch `claude/mbfix`. The lane was bound, unsealed, at `957ad01d1` against base
`4052b8540b5fff818a819db37b55371ea5428c2c`. PR #124 then landed on origin/main as
`538e823f4a1d220a3fb5b12c0529248f18924823`, and the lane conflicted with it. The lane follows
`docs/DOGFOOD.md` "Rebinding after a conflicted merge-forward" (`DWC-V0-7601`, `DWC-V0-7602`). It
merges `538e823f4` with `git merge --no-ff` and no `-X` strategy option, as merge `b0e15c095`. It
then binds, checks and seals the whole range with `BASE=merge-forward`, which derives
`538e823f4`. The binding at the old base is replaced, not sealed.

Conflicted paths (`script/dogfood-merge-base.sh --paths` at `b0e15c095`): `docs/specs/REQUIREMENTS.tsv`, `docs/specs/compat-replay-runner-v0.md`, `docs/specs/daily-change-evidence-workflow-companion-v0.md`, `docs/specs/eol-policy-gate-v0.md`, `docs/specs/go-archive-gate-v0.md`, `docs/specs/release-artifact-integrity-v0.md`.

## Resolutions

- **Makefile line citations** (`compat-replay-runner-v0.md`, `eol-policy-gate-v0.md`,
  `go-archive-gate-v0.md`, `release-artifact-integrity-v0.md`). Both sides edited the `Makefile`,
  so both shifted the `Makefile:N@hash` line numbers. Neither side was picked. Each citation now
  points at the line in the merged `Makefile` that holds the same anchored content:
  `spec-requirements-check` is at 410, the `eol-policy` gate list at 95-96 and its phony targets
  at 521-527, `go-archive-gate` at 245, and `gate` at 154. `script/check-line-citations.sh` passes
  on the merged index.
- **`daily-change-evidence-workflow-companion-v0.md`.** Both sides are kept:
  - the lane's amended `DWC-V0-044` to `DWC-V0-047` and their acceptance rows;
  - main's `DWC-V0-7600` to `DWC-V0-7603` with their failure-mode and acceptance rows.

  The `Exists:` digest line is the union of both lists, and each DWC ID is defined exactly once.
  Main's two report-publication failure rows ("crosses a mount boundary" and "Any other
  publication failure") are also kept. They belong to main's `DTL-V0-016` publisher, which the
  lane keeps (`docs/build-log/2026-11-06-mbfix.md`, "Report publication"). That decision dropped
  only the lane publishers as superseded, so the earlier lane merge had over-deleted these two
  rows.
- **`REQUIREMENTS.tsv`.** It was not hand-merged. `make spec-requirements-resolve` regenerated it
  from the staged specs, together with `INDEX.json` and `README.md`. `README.md` merged cleanly.
  Its DWC row keeps main's `DWC-V0-001..7603` range, which covers the lane's IDs.

## Round 2: origin/main cb4037359 (PRs #122, #123, #129)

Date: 2026-11-07. The sealed range at `983de7dcd`/`0214614c8` was bound from `538e823f4`. Main then
landed PRs #122, #123 and #129 as `cb403735903b1e54a8469373489960c0b2b89d99`. The orchestrator
merged it with `git merge --no-ff` and no `-X` strategy option, as merge `0d9191b70`. The lane
rebinds the whole range from the merged mainline commit with `BASE=merge-forward`, which now
derives `cb4037359`. The seal at `0214614c8` stays in the ancestry and is not rewritten.

Conflicted path (`script/dogfood-merge-base.sh --paths` at `0d9191b70`): `cmd/corvint/dogfood_flow_portable_test.go`.

- **`cmd/corvint/dogfood_flow_portable_test.go`** (`TestDogfoodDailyPathCompletesWhenImpactRefusesTheRepositoryOrModuleRoot`,
  the first-pass `dogfood change` assertion). The lane side changed the expected exit code from 1 to
  3. Main's side wrapped `stderr` in `dropGuardingNote()` for the prefix and the
  `coordination-time-impact` checks. The resolution keeps both: `code != 3` and
  `dropGuardingNote(stderr)`. The failure message still prints the raw `stderr`.
