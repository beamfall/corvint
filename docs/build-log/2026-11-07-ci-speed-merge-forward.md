# ci-speed merge-forward onto origin/main (2026-11-07)

Merged origin/main `538e823f4a1d220a3fb5b12c0529248f18924823` into `claude/ci-speed` with Git's default
merge (no `-X` option). `script/dogfood-merge-base.sh --paths` named one conflicted path:

- `docs/specs/README.md`: three adjacent index rows. Resolution keeps this lane's
  `AFP-V0-548..2702` row with its V1-4815 live-tier note, and main's `AZU-V0-001..9950` and
  `DWC-V0-001..7603` rows. `make spec-requirements` reported no changes.

The pre-merge `dogfood-closeout-precheck` failures were triaged before this merge:
`make lane-scripts-test` (6 lane-screen tests, V1-4982) and `make host-adapter-node-test`
(PPI-V0-8100/8101 need the pi-protected node_modules, V1-4723) fail identically on origin/main;
the batch-drift failure was the `TestContextExpandSequenceBoundExpires` flake (V1-4981), which
passed on retry; `github-split` refuses this PR by design, because #123 is the CI-speed PR
whose purpose is its `.github` change and it is already published.
