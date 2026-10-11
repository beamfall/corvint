# Batch tools validation — V1-1110, V1-1123, V1-0257

## Scope and authority

Base: `2145af619f68698dbb1c097369e29d589e623f7d` (`origin/main` at handoff).
One existing builder worktree; no commits, pushes, decision files or Tasks-store writes.
AFP-V0-043 is accepted, decision 0505.
DCW-V0-034 is accepted, decision 0505; accepted DCW-V0-014
is referenced unchanged. V1-0257 adds evidence for the existing MCPV0-028 enum,
with no new requirement. The builder retained its existing model/effort for these
bounded fixes; an independent native reviewer used the same model/effort.

## Changes and falsifiers

- Refresh and plain check now use the same per-log completed-stream reader as
  advisory check. Malformed input, a truncated final event, an unfinished package
  and an empty shard refuse before table writes. The legacy non-strict observer
  remains available unchanged. Valid raw streams are compared byte-for-byte
  against tables derived through that legacy observer.
- Local-outcome verification commands are validated before recorder invocation,
  including the first pass with an uncommitted sidecar. The trace recorder's
  existing normalization, secret screening, syntax and unique-command/length
  limits are shared. Unsupported syntax retains its existing remediation and
  error text, appending the offending command. Clean verify files still reach
  the recorder. The portable CLI tests cover both binding passes; the existing
  shell verify-file block also asserts that syntax refusal does not call the recorder.
- One enum cross-check parses typed gitstatus constants, compares the conformance
  Go inventory and cases JSON, extracts the closed MCPV0-028 spec clause, and
  checks the MCP command's fallback literal against gitstatus.

Rollback: revert this batch's strict-reader selection, early validation and
cross-check tests/spec proposals. Recorder admission and the legacy observer
retain their original validation behavior.

## Verification

The complete requested doc-gate command passed: spec-requirements-check,
requirement-definitions-check, traceability-tests-check, decision-numbers-check,
line-citations-check, error-code-ownership-check, unbounded-readers-check,
use-case-receipts-check and use-case-receipts-test (five receipt cases).
The separate `sh script/repin-use-case-receipts.sh --check` also reports all
18 receipts current. `cmd/corvint/main.go` was unchanged.

The enum cross-check passed, failed as expected with a temporary single-source
`git-filter-MUTATION` in the conformance Go inventory, then passed again after
byte-for-byte restoration. Mutation evidence is in
`/private/tmp/batch-tools-enum-mutation.log`; baseline and restored logs are
`/private/tmp/batch-tools-enum-baseline.log` and
`/private/tmp/batch-tools-enum-restored.log`.

The focused unit packages (ci-shard-costs, dogfoodflow, trace, gitstatus) passed.
The complete `TestDogfoodChange*` CLI rerun passed, including both new DCW-V0-034
tests; `go test -count=1 -timeout 30m ./cmd/corvint -run TestDogfoodChange`.
The complete `bash script/dogfood-change_test.sh` harness passed, including the
new first-pass verification-file refusal. The complete requested focused command
was run with `GOTOOLCHAIN=local`, `TMPDIR=/private/tmp/claude-501/gotmp`,
`GOMAXPROCS=3`, `-count=1` and `-timeout 30m`:

```sh
go test -count=1 -timeout 30m ./tools/ci-shard-costs ./internal/dogfoodflow ./internal/trace ./internal/gitstatus ./conformance/...
```

It exited 1 because six existing conformance suites failed. The four changed/relevant
unit packages passed. Retained failures:
- `conformance/frontier-v0`: fixture setup reports `cem prepare: invalid-arguments:
  revision must be bounded printable text`.
- `conformance/go-live-test-v0`: production-provider child execution failed.
- `conformance/mcp-2026-07-28`: five existing CEM/context/flows/planted-Git tests
  timed out waiting for responses. The new enum cross-check separately passed.
- `conformance/ocm-v0`: real-verifier fixtures and frozen-vector checks failed.
- `conformance/release-artifact-v0`: archive qualification refused the uncommitted worktree.
- `conformance/work-queue-v0`: existing stream-matrix controls observed zero
  output bytes for fast-overflow, timeout and inherited-pipe cases.

The initial run also observed frontier, OCM, release-artifact and work-queue
failures; final cli-parity and pulse suites passed despite initial failures.
MCP timeout and live-provider failure root causes are NOT_ESTABLISHED.
The release-archive failure specifically reports `worktree is not clean`,
consistent with this builder's explicit no-commit scope. OCM fixtures report
the same bounded-printable-revision refusal as frontier.
These failures are retained outside the three-ticket implementation scope,
not asserted fixed or universally attributable to the sandbox. The focused
`go test -count=1 -timeout 30m ./conformance/mcp-2026-07-28 -run
'^TestReasonClass'` passed (5.094s), covering the existing reason-class cases
and the new source cross-check. All owned test runners completed.
Complete command logs are retained
outside tracked source under `/private/tmp/batch-tools-*`. No full repository
suite, race run or `make gate` was requested or run.

An independent read-only review found no source correctness issues. It identified
the already-covered shell verify-file block, which was extended and re-reviewed
without findings. Final index-based traceability reports zero planned/missing tests.
Initial verification caught and repaired a Go regex escape,
an error-substring compatibility regression, inconsistent spec status mirrors,
index-only traceability needing staged test sources, and a fixture assertion
that needed `git status --untracked-files=all`.

## Corvint evidence and environment limits

The installed Core reports `1.0.0-rc.3 (build 407)`; Tasks reports
`0.0.0-tcp01-unverified+build.407` with qualification `NOT_RUN`.
Official component checks through `corvint-update check --allow-network`
failed their version smoke with `process-after-start-failed: descendant snapshot
unavailable`; available release freshness stays UNKNOWN. No update verification
was bypassed. Account-usage telemetry was not available through session tools
(`NOT_OBSERVED`).

Corvint index refresh and the original bounded query were retained, then
`corvint affected --base` was run before tests. The affected receipt keeps
scope UNKNOWN, its language frontiers and unowned documentation paths; its
full-suite suggestions do not supersede the owner's explicit focused-test scope.
Original outputs: `/private/tmp/batch-tools-affected-before.json`,
`/private/tmp/batch-tools-affected.json`,
`/private/tmp/batch-tools-dogfood-start.log`.

The initial `make dogfood-change` could not create its evidence directory in
the shared Git worktree metadata outside writable roots. Final post-commit
CEM binding/check/seal is NOT_RUN under the no-commit instruction.

The actual Git index is outside writable roots, so `git add` there refused.
Specs and changed tests were staged in a scratch index/object directory to run
the repository's index-based requirements generator and doc gates:
`/private/tmp/batch-tools-git-index`, `/private/tmp/batch-tools-git-objects`.
The real index remains untouched; the coordinator must stage the delivered files.
A scratch PATH wrapper supplies an explicit writable directory to macOS
`mktemp` for recipes that omit a template. No project tooling was changed
for these sandbox accommodations.

No newly observed tooling blocker was filed because this builder is explicitly
forbidden to write the Tasks store. Native ticket completion, integration,
owner acceptance and final evidence binding belong to the coordinator.
