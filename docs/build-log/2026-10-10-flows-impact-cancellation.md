# V1-0279: cancellation reaches the impact walk

## Intent and scope

AFU-V1-059 and MCPV0-035 are accepted, decision 0512.
The user requests cancellation of an active MCP impact graph walk, including all language
adapters and callers. No wire schema changes, new error codes, decision files, publication,
or task-store writes. origin/main and HEAD were identical at
`29e1f653486fca13216606683d3a5c2b05cacd36`; GitHub main compare was identical.
All eight open PRs (747, 749–755) were checked for changed spec paths; none uses these
owning specs or reserves AFU-V1-059/MCPV0-035.

## Implementation and limits

`affected.Build` and `Language.Units` now take context. The existing SourceLanguage reader
seam carries a context-bound disk source to all eight adapters. One source shares walks only
inside that Build; cancellation cannot contaminate another build's cache. Each traversal
callback, including included-directory walks, each cached-file filtering step and each
source operation checks context. Graph composition checks between languages and units and
before returning a graph. An active filesystem syscall or bounded per-file parse may finish
before the next check; this does not promise a wall-clock deadline for blocked filesystem I/O.

The deterministic MCP regression first proves a healthy initialized fixture, gates the tenth
actual walk callback using a test-only context wrapper, then cancels the real request context.
It requires `cancelled`, no ready result, no further traversal callback, and a healthy next call.
Adapter regressions prove mid-walk cancellation for every language and no partial Build graph.
The independent reviewer identified an absolute-root admission regression; the context walk
now preserves the previous guard and a regression covers every adapter.

Rollback: revert the source/adapter signatures and callers together, restore the old walk cache,
and remove the proposed clauses and tests. No persisted format or data migration is involved.

## Evidence and unresolved workflow

Corvint query retained at `/private/tmp/v1-0279-context.json`: one authority result with
four omitted ranked results and retained uncertainty. Affected advice retained at
`/private/tmp/v1-0279-affected.json`: language frontiers remain open; the user's explicit
focused-test scope overrides its static full-gate suggestions. Initial index refresh failed
`repository-probe-failed` (Git metadata changed during observation). Initial dogfood-change
failed because its Git-metadata output directory is outside the sandbox. Final CEM/OCM binding,
checking and sealing are NOT_PRODUCED: the user prohibits committing.

Installed Core: 1.0.0-rc.3 (build 407). Tasks: 0.0.0-tcp01-unverified+build.407.
`corvint-update check --allow-network` refused its version smoke with
`process-after-start-failed: descendant snapshot unavailable`; available-version freshness
remains UNKNOWN. No update activated. Account credit/usage telemetry NOT_OBSERVED: no
account telemetry tool is exposed. No tooling tickets filed because the user prohibits task-store writes.

Normal Git staging is blocked by read-only linked-worktree metadata. Required staged checks
use `/private/tmp/v1-0279-git/index` and its private object directory with the original object
store as a read-only alternate. The repository's actual index remains unchanged. macOS mktemp
uses an unwritable default directory even with TMPDIR set; a task-local PATH shim supplies
an explicit template inside the requested TMPDIR for no-template invocations.

Verification results are recorded below after the final runs. No `make gate` or whole-module
`./...` test was run. Initial focused run failed compilation from a duplicate mechanical
context argument; corrected before the passing cancellation run. A second compile error in
the new MCP fixture's byte-to-string conversion was also corrected.

## Observed verification

- PASS: all eight adapter package suites with `go test -race -count=1 -timeout 30m`.
  The initial combined command also contained compile failures in graph/MCP tests; those
  failures remain recorded and are superseded only by the repaired focused runs.
- PASS: focused graph cancellation, absolute-root admission, included/direct walks, source
  operations, shared-walk compatibility and cross-language selection regressions under `-race`.
- PASS: `TestAFUV1059FlowsImpactCancelledMidWalk` under `-race` (107.580 seconds for fixture,
  both healthy calls and the cancellation call together; cancel-to-return assertion <=5 seconds).
  The earlier ten-second preparation wait timed out before injection and is not cancellation evidence.
- PASS: independent read-only review after the absolute-root repair and the final guidance caller
  context propagation. No blocking findings remain; no independent test execution claimed.
- PASS: `sh script/repin-use-case-receipts.sh --check`, all 18 receipts current after repinning
  UC-CHANGE-CONSEQUENCE's affected CLI subject and its ledger digest. `cmd/corvint/main.go` unchanged.
- The standalone Tasks release check has the same version-smoke refusal as Core, freshness UNKNOWN.

The final CLI and required-check outcomes follow after completion; checks use the requested
GOTOOLCHAIN=local, TMPDIR=/private/tmp/claude-501/gotmp and GOMAXPROCS=3.

- PASS: focused CLI `TestAffectedSelectionArguments`,
  `TestAffectedPlaywrightProfileEmitsProjectDistinctUnits` and
  `TestAFUV1FlowsCLIReverseLookups` under `-race` (276.982 seconds).
- PASS: the requested combined make command (spec-requirements-check,
  requirement-definitions-check, traceability-tests-check, decision-numbers-check,
  line-citations-check, error-code-ownership-check, unbounded-readers-check,
  use-case-receipts-check, use-case-receipts-test). Traceability reports three existing
  planned witnesses. Its index-based citation pass noted the final unstaged MCP table
  rearrangement; a final staged documentation pass closes that delta below.
- Retained failure: `TestRepositoryGuidanceReviewBranches` failed `git-timeout` at
  the guidanceRead call in `cmd/corvint/repository_guidance_test.go` during the existing review path.
  No timeout budgets or unrelated product behavior changed. One reduced-concurrency
  retry is pending; its outcome is retained below.

- PASS: reduced-concurrency guidance review retry under `-race` (110.313 seconds).
- PASS: flow-query goldens and dirty-worktree regression in the expanded CLI run. That command
  failed its clean-tree test with `git-cancelled` during worktree status collection; the clean-tree
  test subsequently passed alone under `-race` (13.166 seconds). The timeouts remain recorded;
  reduced-load success is observed, but host load as their cause is an inference.
- The redundant initial broad package run was interrupted after the adapter suites completed;
  its full appflows suite is NOT_RUN to completion. Focused CLI flow tests and the actual MCP
  impact regression supply the requested impact-path evidence instead.
- Final documentation pass initially failed because the private staged index was inherited by a
  Git fixture in the preceding required checks and reinitialized. The missing legacy-citation
  file and zero planned witnesses exposed that private-index contamination. Restoring a fresh
  copy of the original index and restaging the final task files closes this harness failure.
  No actual repository index or ticket store was modified. No tooling ticket write authorized.

- PASS: final restaged spec-requirements-check, requirement-definitions-check,
  traceability-tests-check, decision-numbers-check and line-citations-check; specindex passed,
  three pre-existing planned witnesses retained, and the complete original index preserved.
- PASS: final diff whitespace/readback check. Actual HEAD remains the original base; the real
  index, task store and forbidden plugin manifest remain untouched. No commit or push performed.

Local requested patch verification is complete. Owner acceptance is decision 0512;
post-commit dogfood binding/sealing and native ticket completion remain outside this authorized
uncommitted, no-store-write task. Official tool freshness and account telemetry remain unknown.
