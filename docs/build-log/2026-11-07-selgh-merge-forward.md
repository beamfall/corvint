# selgh merge-forward onto claude/ci-speed (2026-11-07)

Merge `e1e4775da093d037116e28cbd80d9c4118268504` brings `claude/ci-speed` at its new seal commit
`eeab3c0a0a89d61b1b828e0b13877cb6c0cd2d14` into `claude/sel-github` (PR #129, stacked on PR #123).
The pre-merge lane head was the seal `38b0d7ef96f0b2c09e9cee0f8efeb1bde52f0e0c`.
`script/dogfood-merge-base.sh --paths` names one conflicted path:

- `docs/specs/README.md`: three adjacent index rows. Each side changed a different row: this lane
  raised the Affected Plan Selection Precision range to `AFP-V0-548..2726` (AFP-V0-2720 to 2726,
  V1-4930 and V1-4934), and ci-speed's merge of origin/main raised Affected Zero Unknown to
  `AZU-V0-001..9950` and the Daily Change-Evidence Workflow Companion to `DWC-V0-001..7603`. The
  resolution takes the union: `AFP-V0-548..2726`, `AZU-V0-001..9950` and `DWC-V0-001..7603`, with
  every other cell unchanged. `docs/specs/REQUIREMENTS.tsv` was regenerated with
  `make spec-requirements` and has no hand edits.

The range is bound with `BASE=merge-forward` and `DOGFOOD_MAINLINE_REF=origin/claude/ci-speed`.
The default mainline, `origin/main`, refuses `merged-parent-not-mainline` because this lane's merged
parent is its stacked predecessor. With the predecessor as the mainline, the helper derives
`eeab3c0a0`, the predecessor seal commit that "Stacking on a sealed predecessor" in
`docs/DOGFOOD.md` names as this lane's base. The range from that commit holds the lane's own
commits, its round-1 sealed CEM and the resolved hunk.
