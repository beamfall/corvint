# Remove Greptile from the merge path (decision 0598)

- Date: 2026-11-12
- Trigger: the Greptile organization trial ended. Three review requests on beamfall/corvint#198
  (20:22:51Z, 20:26:24Z and one earlier) were each answered within seconds by a "trial has ended"
  review summary, never a score, so the required `greptile-gate` stayed pending. The local Greptile
  CLI had refused earlier the same day with "insufficient credits".
- Owner direction: remove Greptile; use the Codex CLI for reviews.
- Ruleset 26175601 (`main`): required contexts changed from `go-product`, `ci-control-plane`,
  `doc-gates`, `greptile-gate` to `go-product`, `ci-control-plane`, `doc-gates` (GitHub API PUT,
  2026-11-12). The prior ruleset JSON is retained in the orchestrator's session record for rollback.
- This change: retires GRG-V0, deletes `tools/ci-reuse-plan/greptile_gate_workflow_test.go`, and
  amends REL-V1-012 and CIN-V0-5019. The workflow file is deleted in a separate `.github` pull
  request, merged after this one.
- `tools/ci-forward-row/github_guard_test.go` keeps the literal name `greptile-gate.yml` only as an
  arbitrary unrelated workflow in its fixtures; it does not read the real file.
