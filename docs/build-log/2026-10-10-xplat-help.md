# V1-0393 platform refusals and V1-0629 CEM citation help

Date: 2026-10-10. Base: `2145af619f68698dbb1c097369e29d589e623f7d`.
Scope: two separable repairs requested by the owner; no new or amended requirements,
wire vocabulary, platform qualification, decisions, commits, publication or ticket-store writes.
Existing authority: `PRS-V1-004` and the supported basis relation table in
`docs/CHANGE-EVIDENCE-MAP.md#supported`. The isolated analyzers remain deferred/experimental.

## V1-0393

FreeBSD/amd64 baseline vet reproduced undefined `openBoundedInput`, `readDescriptor` and
`ReadNoFollow` in the three named packages. Added complement-tagged refusing implementations
and focused refusal tests in `internal/cem/publish/read_unsupported{,_test}.go`,
`internal/analyzerswift/descriptor_other{,_test}.go` and
`experimental/analyzers/sqlnative/read_other{,_test}.go`. Darwin/Linux/Windows source and
build tags, including SQL's OS-specific identity helpers, are unchanged. The SQL fallback
preserves cancellation and nil-context rejection before unsupported acquisition.

Compile-only vet PASS for all three packages on freebsd/amd64, openbsd/amd64, netbsd/amd64,
solaris/amd64, darwin/arm64, linux/amd64 and windows/amd64. Host package tests PASS before and
after the change on darwin/arm64. All three fallback tests also PASS on the host using explicit
Go file lists, which select the refusing implementations independently of build tags.
Cross vet compiled the fallback tests; native BSD/Solaris execution is NOT_RUN. These platforms
are not qualified by this repair (`PRS-V1-004`); Windows remains UNSUPPORTED.

Separately commit the six fallback/test files and the acceptance-evidence addition in
`docs/specs/corvint-1.0-product-and-release-v1.md` for this ticket.

## V1-0629

`cmd/corvint/help.go` now lists call-site, decision, dependency, implementation, incident,
specification and test-claim, and points to their normative supported basis definitions.
`TestCEMCiteHelpListsRelations` exercises `cem cite --help` without repository or stdin reads,
asserting all seven values and the reference. The wire/CLI accepted vocabulary is unchanged.
`docs/CHANGE-EVIDENCE-MAP.md` records this discoverability repair; the existing anchored
help citation in `docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md` moved from line 952 to 955
with the same content hash after the help insertion.

Focused help tests PASS: `TestCEMCiteHelpListsRelations`,
`TestHelpInvocationsAreDeterministicAndDoNotInspectRootOrStdin`, and
`TestHelpFlagAnywhereFollowsArgparse`. Receipt repin reports all 18 receipts current; no receipt
or ledger changes were needed. Receipt-script tests PASS (five cases).

Separately commit the two help files and the two documentation files above for this ticket.
This single requested build-log entry records both repairs and may accompany either commit.

## Verification and retained limits

Go tests, vet and make checks used `GOTOOLCHAIN=local`, `TMPDIR=/private/tmp/claude-501/gotmp`,
and `GOMAXPROCS=3`; local Go is 1.27.1. No `make gate` or repository-wide Go tests ran.
Independent native reviewer found no blocking correctness issues in the implementation,
test, normative-reference and spec diff; no duplicate test runs were requested from it.
Documentation gates PASS: `spec-requirements-check`, `requirement-definitions-check`,
`traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
`error-code-ownership-check`, `unbounded-readers-check`, `use-case-receipts-check` and
`use-case-receipts-test`. The line-citation gate first failed on the shifted help citation;
after reading the identical old/new content and correcting its line reference it passed.
The final `sh script/repin-use-case-receipts.sh --check` and `git diff --check` also PASS.
Command outputs are retained in `/private/tmp/xplat-{baseline-tests,tests,help-tests,
vet-matrix,doc-gates,doc-gates-final,receipt-test}.log`.

Corvint query and affected routes were used at this worktree/base; receipts are retained in
`/private/tmp/xplat-context.json` and `/private/tmp/xplat-affected.json`. Query limit omissions
remain explicit; affected selected the four touched Go units but retained UNKNOWN scope,
build-constraint variants, nested-module and other language-frontier unknowns. Its generic
full-gate advice was overridden by the owner's explicit focused-only instruction.
Index refresh and `make dogfood-change` failed because the shared Git metadata directory is
read-only. Post-commit CEM/OCM binding, dogfood checking/sealing and native ticket completion
are NOT_PRODUCED: commits and ticket writes are expressly outside this task's authorization.

Specs and changed source were staged only in an isolated temporary index/object directory
under `/private/tmp/xplat-staging`; the owner's actual index remains untouched. Regeneration
with `make -s spec-requirements` produced byte-identical `REQUIREMENTS.tsv`. A temporary
`mktemp` wrapper directs no-argument scratch creation to the requested TMPDIR because the
system command otherwise chose an unwritable system cache directory. Repository scripts
are unchanged. Receipt fixture tests ran without the isolated Git environment.

Installed tools: Core 1.0.0-rc.3 build 407; Tasks 0.0.0-tcp01-unverified+build.407.
Official update checks for both components failed their version smoke with
`process-after-start-failed: descendant snapshot unavailable`; available versions/freshness
remain UNKNOWN and no update was installed. Account usage telemetry is NOT_OBSERVED because
no account-usage reader is available. No tool-friction ticket was filed under the owner's
explicit no-ticket-store-writes instruction.

Rollback: remove the complement-tagged fallback/test files and revert the help/documentation
edits. No store migration, wire change, qualification promotion or executable activation occurred.
