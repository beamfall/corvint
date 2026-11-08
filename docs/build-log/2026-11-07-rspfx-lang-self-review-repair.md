# Language runner self-review repair

Authority: owner self-review repair request; RSP-V0-002/003/005. Scope is the three
reported findings against `777b42c865c7551dbfa7a74bede56e06f972c211`.

The Rust omission class is an unsupported target universe behind a library-only
command. The closed allowlist is exactly one local `lib` target plus
`custom-build` targets, each with exactly one kind. Empty, multiple-kind, binary,
example, benchmark, unknown, zero-library and multiple-library universes refuse
with the existing `rust-library-target-universe-incomplete` reason. Integration
targets retain their existing specific refusal. Nextest is unchanged. The
library-plus-binary fixture includes a native binary unit test; a future kind
proves the default refuses rather than extending a denylist.

The JVM failure-loss class is an invalid suffix after independently complete
suites. The envelope accepts only the existing `testsuites`/`testsuite` grammar,
with its existing whitespace, comment and XML declaration rules. Every complete
suite is validated by the existing native parser; a malformed or unknown suffix
still returns the existing envelope error alongside validated results. A passing
FULL retry cannot erase an earlier failure. Tests cover a missing wrapper close,
truncated start, unfinished suite and unknown child for Gradle and Maven.

For MTP, choose the review's explicitly allowed complete-binding alternative.
Every accepted identity must have a hash-validated captured `.cs` source binding;
a native location can corroborate it but cannot substitute for that map. This
keeps the plugin immutable and makes the existing missing-collection loop cover
every accepted identity without adding wire fields or stateful discovery. Tests
cover absent bindings, an unknown source form, and a bound missing test producing
an incomplete record for each MTP runner. Existing reasons and schemas remain.

Witnesses: `TestRSPV0002RustTargetAllowlist`,
`TestRSPV0003JVMDamagedSuffixRetainsFailureAcrossRetry`, and
`TestRSPV0005DotnetMTPLocationOnlyMissingCollection`. The whole registry package
run failed in the unchanged `TestRSPV0040ShellTraceRefusals` at its existing
"invalid execution accepted" assertion. No test assertion or golden was weakened.
The external handback retains command results and closeout limits. Independent
review and repository-wide verification belong to the orchestrator. Revert this
repair commit to roll back; that restores the reported gaps, not qualified behavior.
