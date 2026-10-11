# Gate ledger partial package passes (V1-0245)

Intent: GL-V0-002 and GL-V0-009 amendments are accepted, decision 0511
(V1-0245). A resolved batch failure must retain individually proven passes keyed on
the existing content and bound proofs, while failed packages remain runnable.

Scope: `tools/gate-ledger/`, `docs/specs/gate-ledger-v0.md`, generated
`docs/specs/REQUIREMENTS.tsv`, and this entry. No decision files, commits, publication,
or Tasks-store writes are authorized for this concurrent worktree task.

Implementation: resolved batches capture newline-delimited Go JSON, forward output,
and retain the command's exit status. Records are written only after every requested
package has a complete outcome. Package passes survive other package failures;
failed packages, skipped packages and unproven bounds never record. Missing,
truncated, malformed, contradictory or duplicate-field events refuse the batch's
records. A single event is limited to 1 MiB and each package to 100000 active tests.
Existing per-key locks, package bounds, keys and unresolved whole-tree records remain
the reuse boundary. Rollback: disable the ledger with `CORVINT_GATE_LEDGER=off`, or
revert this scoped patch; changed tooling content invalidates prior keys.

Verification: pre-change focused gate-ledger tests passed (73.096s). Parser refusal/output tests passed (1.619s). Real Go failure and build-failure
same-tree retry tests passed (446.016s). Final focused suite passed (85.808s), with `GOTOOLCHAIN=local`,
`TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3`, `-count=1 -timeout 30m`,
and package `./tools/gate-ledger`. Git index/object overrides were explicitly
unset for test fixture isolation. The earlier suite also passed (520.448s). All requested documentation gates passed:
`spec-requirements-check`, `requirement-definitions-check`,
`traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
`error-code-ownership-check`, `unbounded-readers-check`, `use-case-receipts-check`,
and `use-case-receipts-test`. Documentation checks used the private staged index;
traceability passed after staging the owned tests there. The receipts check reports
all 18 receipts current. No repository-wide test or `make gate` was run.

Independent review: one native read-only reviewer identified unfinished-test,
post-pass build-failure and duplicate-field refusal gaps. Each has a regression
case and a parser repair; repair review passed with no unresolved findings. The actual Go fixture covers
test failure and build failure, same-tree retry, failed-package absence and
unresolved execution. Separate malformed/partial fixtures check the record boundary.

Verification limits: the real shared index is outside writable roots, so `git add`
was refused there. Documentation checks use a private copy of that index and a
scratch object directory; only owned files are staged there. A scratch `mktemp`
wrapper supplies the allowed temp directory for default templates and preserves
explicit paths. The receipt-test fixture inherits Git overrides and rewrites
its caller's private index: final readback detected this, so the scratch index was
rebuilt and fixture tests rerun with the overrides unset. Source/spec read gates
retain the staged snapshot; fixture tests use their own Git state. The real index
and unrelated concurrent edits are untouched.
Initial focused tests failed because the fixture selector had no tests and emitted
skip rather than pass; a fixture test now establishes the required pass. A 90-second
diagnostic timed out waiting for child Go execution; the final run passed under the governing 30-minute
hang detector. All owned test and gate command sessions completed. Earlier digest-format and unstaged-test
traceability failures were repaired, with original logs retained under
`/private/tmp/v1-0245-*`.

Corvint self-use: the runtime-requested `index --if-stale` refresh was attempted
and refused because the shared Git snapshot directory is outside writable roots
(`/private/tmp/v1-0245-index.log`). `query` was unavailable during a concurrent repository metadata
change. Tracked-path `impact` was used and retained its ranked omissions and excluded
paths. `affected --base 29e1f653486fca13216606683d3a5c2b05cacd36` ran before
tests; output is retained at `/private/tmp/v1-0245-affected.json`. Broad suggested
gates are explicitly excluded by the owner's scoped instruction. Post-commit CEM,
OCM and sealing are NOT_PRODUCED because committing and edits outside this task's
files are forbidden. Retrieval/learning evaluations are not applicable to this
ledger change. Installed Core is 1.0.0-rc.3 build 407; Tasks is
0.0.0-tcp01-unverified+build.407. `corvint-update check --allow-network` failed its
process version smoke (descendant snapshot unavailable); release freshness remains
UNKNOWN. No update was installed and no tooling ticket was filed because Tasks
writes are forbidden. Account usage and billed-token telemetry are NOT_OBSERVED.
