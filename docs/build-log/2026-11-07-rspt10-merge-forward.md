# rspt10 merge-forward evidence

SPDX-License-Identifier: Apache-2.0

The owner requested a merge-forward of lane `claude/rspt10` from
`2cabb356d2c002d0845aff7fe685f6284d0e8772` onto mainline
`9761ea1f459c0d3b3c963953e24a76eaeda16370`, preserving existing commits.
The host supplied merge `57388f8e0e1e694b89996c3c7ca177831d13c022` and
regeneration `bc4f4ce292ed0d3cece2d7c7bb0e25e371469024` before this round.
The whole-range CEM is rebound against that exact merged mainline, following
`docs/DOGFOOD.md`'s conflicted merge-forward route. The lane was unsealed;
the owner requires a passing check without a seal or publication.

## Resolutions and retained scope

- `docs/specs/README.md`: the host retained both sides' spec entries and the
  mainline's expanded affected-plan and reader-precision descriptions. Git's
  remerge report identifies this as a resolved path.
- `docs/specs/REQUIREMENTS.tsv`: the host took mainline's generated projection
  for the merge, then regenerated in `bc4f4ce29`, restoring all fourteen rows
  from the combined specs. Git's remerge report identifies this resolved path.
- The host reports append-only spec sections were combined by merge drivers;
  `docs/specs/affected-plan-selection-v0.md` and
  `docs/specs/affected-zero-unknown-v0.md` retain mainline additions alongside
  the lane's inherited requirements. No requirement ID was renumbered.
- `docs/specs/INDEX.json` remains the combined projection; the regeneration
  commit changes only `REQUIREMENTS.tsv`. The initial
  `make -s spec-requirements-check` passed (rc 0), including its existing
  specindex and lrfrepo checks.
- No conflict-resolved or regenerated Go file exists in this round, so the
  owner's per-package merge-repair test selection is empty. Broader tests,
  doc-gates, affected selection and independent verification remain host-owned.

Historical evidence and its explicit unknown reasons remain in ancestry and
in the retained input files. Rebinding does not promote unknown evidence,
prior test outcomes, or absent independent review into fresh qualification.
The initial preparation reported missing citation, intent and outcome inputs;
its log is retained under the lane's private TMPDIR as `merge-prepare.log`.
