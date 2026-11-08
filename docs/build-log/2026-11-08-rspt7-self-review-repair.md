# Runner policy self-review repair

V1-5109; RSP-V0-046 and RSP-V0-057. The review found that governing runners
outside selector inputs disappeared, normal sampling skips poisoned history,
and confirmation rejected valid native supersets. No public contract, golden,
existing assertion, production rollout or sampling rate changes.

The execution domain now includes every governing runner. Before admission its
plan is preserved; after admission a runner without selector coverage is FULL.
Only the selector's qualified selected form can narrow. Unknown runner kinds
with no selector input follow the same FULL default, without a runner-name list.

Detection certifies the three known no-work outcomes (not sampled, not on the
default branch, no exclusions) only after retained-plan and sampling admission.
Unknown policy states and failed captures remain incomplete. A nonselected draw
at an owner rate below one now preserves subsequent policy narrowing.

Confirmation admits only inventoried FULL captures or explicit inventoried sets
covering the requested tests. Reconciliation uses that actual plan and requires
one terminal result per captured test; unknown identities/outcomes, undeclared
results and missing requested or wider-plan results remain unproven. This closes
the plan/result mismatch class without permitting arbitrary result supersets.
Miss/flake classification still requires the existing base/head outcome rules.

Acceptance evidence: `TestRSPV0046GoverningRunnerCoverage`,
`TestRSPV0057NoWorkHistoryComplete`, `TestRSPV0057ConfirmationSupersets`, and
`TestRSPV0057ConfirmationUnknownPlans`, plus every existing runnerpolicy test,
passed via `script/heavy.sh GOTOOLCHAIN=local go test -count=1 -timeout 30m
./internal/runnerpolicy` (0.013s package time). Both FULL and explicit multi-test
confirmation fixtures cover deterministic misses and flakes. No assertion was
weakened. Raw output is outside the repository in the lane evidence directory.

Self-development route: affected used against the requested base, selected
`internal/runnerpolicy`; the installed binary reported stale relative to this
checkout, so that advisory's unknowns remain. Pre-change capture is inherited;
no repeated capture. CEM rebinding/check follows final edits. Independent review
and broader integration checks belong to the orchestrator; live default-branch
captures and protected persistence remain NOT_OBSERVED. Optional provider,
mutation, migration and UI routes do not apply to this bounded policy repair.

Rollback: revert this repair commit; the three findings then remain unresolved.
No qualification or delivery promotion is claimed by these local fixtures.
