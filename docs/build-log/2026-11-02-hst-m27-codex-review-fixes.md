# 2026-11-02 hst-m27 Codex review fixes for AHI-632 plan-affected checks

One Codex review round of the hst-m27 lane change (base `55f5a9fd74a4`) returned one P1 and four
P2 findings against `AHI-632` (V1-0878). Each finding was verified against the code first. All five
were accurate and all five are fixed. The spec text, traceability row and evidence for `AHI-632` in
`docs/specs/agent-harness-integration-v0.md` are amended in the same change.

## Dispositions

1. P1, the worktree qualification was lost: fixed. `planAffectedObject` ran a `--worktree` plan but
   kept only the summary and units. A `PRODUCED` object now also carries the receipt's observed HEAD
   `revision`, its `ACX-V0-851` `warning` and its `degradations` (`worktree-path-plan`), so the host
   is told that the checks are not pinned evidence.
2. P2, appending could displace the context packet: fixed. `runClaudePreToolRead` holds back
   `planAffectedContextReserve(paths)` through a context value, and `runClaudeAdapter` subtracts it
   from the packet budget. The reserve is the encoded compact `NOT_PRODUCED` object for the paths
   plus a 1,500-byte result allowance. The object is appended only when the combined answer passes
   `withinAdapterOutputLimit`. Otherwise the compact `plan-affected-over-budget` object is tried,
   and then the object is dropped, so the packet is never replaced by `corvint-output-too-large`.
   The degraded and reduced-packet paths are not budgeted by the reserve, but the fit check covers
   them.
3. P2, a late result could be accepted: fixed. The cutoff (host deadline less 150 ms) is fixed when
   the plan launches. It bounds the plan's own context through `context.WithDeadline`, and each
   result carries its completion time. A result finished at or after the cutoff becomes
   `plan-affected-deadline`, even when it is already waiting in the channel when the context compile
   returns.
4. P2, an ancestor symlink could escape the project: fixed. `planAffectedPaths` resolves the root.
   It admits a token only when `EvalSymlinks` of the joined candidate equals the candidate, so no
   component of the path is a symbolic link.
5. P2, the test did not prove success: fixed. `TestAHI632PlanAffectedOutcomes` uses the affected
   fixture with a dirty `core/core.go`. It asserts these outcomes:
   - The object is `PRODUCED`, selects `go:example.com/fixture/core`, has an omitted count of 0 and
     a `dirty=1` summary.
   - The object carries the HEAD revision, the not-pinned warning and `worktree-path-plan`.
   - A non-repository root gives `plan-affected-refused`.
   - A deadline inside the reserve gives `plan-affected-deadline`.
   - The reserve covers the fallback object.
   - A symlinked `external/` ancestor is not admitted. The old `Lstat` check admitted it.

## Tests

`corvint affected --base 55f5a9fd74a4` selected widely (scope `UNKNOWN`: 160 units, 80 unknowns,
and the installed binary was `self-binary-stale`). Only focused `cmd/corvint` runs were made,
through `script/heavy.sh`:
`-run 'TestAHI|TestAdapter|TestClaude|TestHostAdapter|TestACXV0851|PreTool|Plan'` passed in
69.7 s. `go vet ./cmd/corvint/` and `gofmt -l` are clean.
