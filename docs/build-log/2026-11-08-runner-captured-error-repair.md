# Captured results survive execution errors (V1-5105)

Self-review identified an RSP-V0-003 violation in the compiled-in selection
adapter: an execution error discarded captured results before reconciliation.
A missing identity then triggered FULL, whose passing report could hide an
already reported failure.

The adapter now parses every captured report regardless of execution status,
returns the parsed results, and joins execution and parse errors. Core
reconciliation retains the results and its existing sticky failed-result state.
No wire shapes, error codes, stderr contracts, goldens or existing assertions
change. This repairs the accepted requirement; it adds no capability.

The class is loss of partial evidence at an error-bearing producer boundary.
Admission still uses the core's closed terminal-outcome allowlist: passed,
failed, and skipped with a nonempty reason. Every other outcome is nonterminal,
keeps that identity missing, widens to FULL, and requires requalification.
There is no error-string allowlist: all execution errors preserve captured output.

`TestRSPV0003SessionExecutionErrorRetainsResults` covers the reported failed-a,
missing-b, execution-error case followed by a passing FULL retry; simultaneous
parse and execution errors with a retained failed result; and an unknown future
outcome that remains missing and forces FULL with sticky requalification.
The whole registry package, including existing tests, passed through
`script/heavy.sh go test -count=1 -timeout 30m ./internal/testrunner/registry/`
(0.007 package seconds). Raw evidence stays outside the repository:
`/home/agent/lanes/rspt3-evidence/test-registry-repair.log`.
SHA-256: `14dfada3f09c883a54874be4b3003ecf5d5da06cf98b06eee0e250f81280c1bd`.

The requested affected plan selected registry by DIRECT_SOURCE_CHANGE and
reported UNKNOWN, 451 selected, 480 excluded and 71 LANGUAGE_FRONTIER unknowns.
The installed binary emitted self-binary-stale; these are its observations, not
fresh-build qualification. Prior containment blockers remain recorded separately.
Independent review and broader checks belong to the host verifier. Rollback is
reverting this repair; doing so restores the known RSP-V0-003 violation.
