# selbatch merge-forward round 2: origin/main 59b31f12e (2026-11-07)

`origin/main` gained PR #123 (ci-speed, `AFP-V0-2700` to `AFP-V0-2702`), so `5cc925bf0` merges
`59b31f12e` into `claude/selbatch` on top of the sealed tip `445dcdd46`. The lane binds from that
commit with `BASE=merge-forward` (`DWC-V0-7601`, `DWC-V0-7602`). `script/dogfood-merge-base.sh
--paths` lists three conflicted paths, each resolved as follows:

- `docs/specs/affected-plan-selection-v0.md`: both sides' new requirement blocks kept, the lane's
  `AFP-V0-2240` to `AFP-V0-2247` rounds first, then main's "Live test tier (V1-4815)" section with
  `AFP-V0-2700` to `AFP-V0-2702`. No ID collides.
- `docs/specs/README.md`: main's affected-plan-selection row, with its merged range
  `AFP-V0-548..2702` and the V1-4815 claim, but the lane's description wording kept (timeouts,
  deletions, runtime-rooted Go readers, CI match); the union range is therefore `548..2702`.
- `docs/specs/REQUIREMENTS.tsv`: regenerated with `make spec-requirements-resolve`, not merged by
  hand. It holds the lane's `AFP-V0-2240` to `AFP-V0-2247` and main's `AFP-V0-2700` to
  `AFP-V0-2702`. `docs/specs/INDEX.json` regenerated alongside; it did not conflict.

No other path differs from Git's own merge. To roll back, revert `5cc925bf0` with `-m 1`.

## Interaction fix (not a conflict)

`make doc-gates` failed `test-read-scopes-check` on the merged tree only. The lane declares
`.github/cishards` as reading nothing (`[]`, `a06dac2f6`), and main's new `live_test.go`
(`AFP-V0-2701`) walks the repository from `../..` and reads `../workflows/ci.yml`. Neither side
fails alone: `origin/main` has no entry for the package, and the lane's `[]` fits the old tests.
`.corvint/test-read-scopes.json` now declares `.github/cishards` as reading
`.github/workflows/ci.yml`, `internal/companionrelease` and `internal/liveverify/affected`, the
three names the gate reported, so a change to a live-tier test file selects the package's tests.
