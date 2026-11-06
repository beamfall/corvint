# Advisory Darwin coverage outside required CI (V1-4732)

Date: 2026-11-05. Requirement: CIN-V0-4625; CIN-V0-4626 remains deferred.
Intent amendment is recorded under the owner's standing auto-accept (2026-10-11),
with the V1-4732 lane charter as the task authority. No new requirement IDs.

The owner-supplied before measurements are CI run 42138143815: 9,526 seconds for
176 sequential Darwin packages, including `cmd/corvint` 1,328 s,
`internal/tasks/store` 1,150 s, `internal/doccorpus` 463 s,
`internal/tasks/cli` 438 s and `internal/appmap` 358 s. None of the last twelve
CI runs completed go-macos successfully. On PR #102 the required jobs passed at
03:42Z, but the PR remained BLOCKED for more than 25 minutes because CI completion
and its control-plane status waited for the advisory macOS job. These are charter
observations, not measurements independently reproduced in this Linux lane.

Decision: move Darwin coverage into the pull-request workflow `Go macOS`, preserving
the `go-macos` advisory summary check. Recompute protected-base DOCS verification
and the existing trusted MET verdict there. Four independent macOS matrix legs
recompute the native inventory and fail-closed narrowing, keeping Linux package
artifacts and REUSE out of the Darwin selection. CI contains no Darwin job or
needs edge, so its completion no longer waits for this workflow. Its control-plane
trigger and Linux REUSE admission stay unchanged. The deferred V1-4636 integration
now refers to the independent check, never a nonexistent CI-local dependency.

Placement uses the existing `.github/cishards/package-costs.json` Linux history
and the whole-package longest-cost-first rule, with median cost for an unknown
package and complete round-robin placement if the table cannot be read. It never
splits tests or changes the selected set. Each shard runs one invocation per module
with `go test -p 3 -parallel 2`, retaining subsequent module results after a failure.
Four macOS runners is the hard maximum; each has a 40-minute diagnostic timeout.
The native planner/build cost is repeated on each runner to avoid a fifth macOS
planning job or a serial macOS planning dependency. The existing permission
exception table gains only the copied trusted verdict job's read-only grant;
this registration is required by CIN-V0-7501, not a broader permission change.

Shard zero alone writes the existing `macos-selection` artifact with the unchanged
selection schema and original inventory/narrowing paths. Every shard also retains
its package list, cost estimates, module logs and actual outcome in its own
`macos-selection-shard-N` artifact. The summary checks all four outcomes, including
missing/failed artifacts and failures before test execution. Continue-on-error
keeps all these checks advisory. Darwin failures themselves are V1-4701's scope.

Expected after: remove all advisory Darwin time from required CI completion, and
target under 30 minutes for a FULL Darwin selection. With twelve package execution
slots, 9,526 s / 12 is about 794 s; the observed longest indivisible package is
1,328 s (22.1 min). This suggests room for provisioning/build overhead below 30
minutes, but is an optimistic scheduling estimate, not a measured speedup. Linux
costs, package CPU contention, repeated native tool builds and per-module boundaries
can invalidate it. Hosted FULL runtime and Darwin outcomes are NOT_OBSERVED;
publication is explicitly outside this lane. Retained per-module JSON logs support
later native cost and wall-time qualification without changing the wire record.

Verification: Python selector and shard tests cover complete deterministic
placement, unknown/malformed costs, nested module grouping, whole-package batching,
three package workers, and continuation after a module failure. The whole
`tools/ci-reuse-plan` package includes the new required-needs-graph test and an
admission test rejecting the separate Darwin workflow as Linux REUSE proof.
The existing `tools/ci-forward-row` package exercises trusted narrowing guards.
Actual commands/results, digests and any failures are retained in the lane handback
and external evidence directory. YAML and embedded shell parsing are local checks;
a hosted Actions run is NOT_RUN, not implied by those checks. Independent review
and repository-wide gates belong to the host orchestrator under the lane charter.

Dogfood routes used: pre-change context, query and impact receipts, affected planning,
and final CEM/report/check against base 9db7a94169821d9b28701eaa88866aee2fec1083.
The first impact request used a directory unsupported by the installed binary;
a second pre-change request named the original Go file and retained all three
receipts. The installed affected binary reported stale-self degradation; its
selection is advisory and the explicitly required touched package still runs whole.
Retrieval/learning evaluations and providers are not applicable to workflow-only
behavior. No Tasks writes, publication, model-backed review or CEM seal are authorized.

Rollback: revert this workflow extraction, sharder and matching tests/spec amendment
together, restoring the former advisory CI job and its known completion delay.
No branch-protection settings, JSON fields, error/exit contracts, committed goldens
or frozen core outputs change.
