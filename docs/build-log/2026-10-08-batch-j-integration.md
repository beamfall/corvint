# 2026-10-08: Batch J integration

## Intent

Batch J lands seven verified task lanes in one pull request on base `f33ea8ef`:

- V1-1022: Corvint Tasks obligation ledger (TOL), accepted by decision 0456. Delivery stays
  experimental.
- V1-1023: test consolidation planner (TCN-V0-001..012), with the interpretations accepted by
  decision 0466.
- V1-1024: step-level negative controls by fault injection (LPCV-V0-057..069). Experimental.
- V1-0782: host package version check in CI for the OpenCode integration.
- V1-0998: linked-worktree context revision. Refuted; the lane retains a regression test only.
- V1-0555: OCM link refusals and guidance explain requirement anchor token boundaries.
- V1-0520: OCM inline `go run` boundary.

The batch also adds decision 0469, which accepts OCM-V0-017 (V1-0555) and OCM-V0-018 (V1-0520) on
the owner's instruction.

## Integration decisions

- V1-0520 and V1-0555 both edited `docs/specs/ocm-v0-dogfood.md`. Both requirement additions were
  kept, and `REQUIREMENTS.tsv` was regenerated.
- In the V1-1022 merge, the TOL status keeps intent `accepted (decision 0456; V1-1022)` and
  delivery `experimental`. The digest sentence "nothing is implemented" was replaced with one
  stating that acceptance settles intent only and nothing may be promoted until acceptance
  evidence exists.
- Decision 0469 landed in the merge commit for V1-0520, so its rollback restores the requirement
  markers by hand.
- The batch then merged `origin/main` `b5616037` (batch H, PR #692). TOL obligations and the
  CAL-V0-195 milestone policy both extend the Tasks policy, so `intent/policy.go` keeps both
  optional keys (`obligations`, `milestones`) and both types. `queue status` compact output picks
  both `openWithoutMilestone` and `obligations`. `store/know_how.go` keeps both `FilesAtCommit`
  (TOL-V0-012) and the KHN-V0-024 `--repo` helpers. The know-how usage lines take batch H's
  `--repo ALIAS=ROOT` form, alongside the obligations usage lines.

## Evidence

Each lane retains its own build-log entry, focused tests and independent review. On the merged
tree, these all passed: the doc gates, `internal/specindex`, `go build ./...`, the focused
`internal/tasks` and `internal/taskman` packages, and the focused checks in the batch dogfood plan.
After the main merge, the focused `internal/tasks` packages (including `intent`) passed again. The
batch CEM is bound against base `b5616037000a06e4719c759a555f7e393390e94f` and sealed.

## Rollback

Revert the batch merge commit on `main`. Each lane can also be reverted on its own through its
merge commit in the batch branch.

## Second main merge (38fcf067, batch I and V1-1028)

Merging origin/main 38fcf067 brought V1-1028's keep-reporters qualification into
`internal/jstestprovider` beside this batch's negation run (LPCV-V0-058). Both sides are kept:
the e2e config line takes V1-1028's `kept` reporter form, wrapped by the negation prelude and
override; the provider usage lists `negate` and `qualify-keep-reporters`; `E2EConfig` carries
both `negation` and `KeepReportersQualification`. The two sides each declared a package-level
`observation` (this batch's type in `negate.go`, V1-1028's function); the function is renamed
`outcomeObservation`, with no behavior change. The UC-EVIDENCE-CARRYING-COMPLETION receipt
carries both sides' subject repins, so the ledger pins the merged receipt bytes. One legacy
unanchored citation in `js-live-test-provider-v0.md` was repinned with an anchor. The earlier
seal is dropped and the batch is rebound against the new base. Focused checks:
`internal/jstestprovider` and `cmd/corvint-js-test-provider` pass; the doc gates pass.
