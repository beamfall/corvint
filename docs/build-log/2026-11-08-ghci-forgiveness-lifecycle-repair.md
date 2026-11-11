<!-- SPDX-License-Identifier: Apache-2.0 -->
# Forgiveness requires complete invocations and understood output

Owner intent: ghci self-review repair, V1-5098 / CIN-V0-8720. Base:
`9761ea1f459c0d3b3c963953e24a76eaeda16370`.

Finding class: a listed failure was treated as sufficient evidence for an entire package,
although a later test could time out without a terminal test event, or a later invocation
could end without a package outcome. A successful diagnostic rerun cannot fill those gaps.

Decision: certify a closed lifecycle grammar. Each package starts once, tests run and may
pause/resume before passing/failing/skipping, every started test terminates, and the package
terminates before its next invocation. Only a failure in that invocation authorizes its
failed outcome. EOF with any active invocation refuses. Interleaved packages remain supported.
Benchmarks, unknown actions/fields/output types and illegal transitions refuse.

Output is also closed: named testing control/result frames, four-space-indented testing log
lines while that test is active, and package PASS/FAIL/name-reset/timed-summary/no-test frames.
All other stdout remains unproven. This deliberately refuses arbitrary direct stdout and
future formats, rather than maintaining a list of fatal diagnostic spellings. A panic that
test2json closes with a test failure still refuses. No diagnostic report, status code, stderr,
golden, policy exception or existing assertion was changed.

Evidence: all nine `known-flakes_test.py` tests and all four `go-product_test.py` tests passed.
New regressions cover the reported timeout and incomplete later invocation, missing terminal
events without diagnostic output, paused unfinished tests, unknown output/actions/metadata,
fatal output even with terminal events, and complete interleaved/parallel/repeated invocations.
The actual workflow block preserves FLAKY but refuses status clearing for incomplete and unknown
streams despite a passing rerun. Go's original timeout and panic JSON fixtures also refused:
timeout SHA-256 `08bc0f12d42a03911d14b9f135125c0cafaafd0915b5b4a193c304c71b381aa8`;
panic SHA-256 `83e43a61393c291f093a3545e96d33b9385dbc6aa6870d580476a0e13e2c6e22`.
`make -s spec-requirements-check line-citations-check` passed, including the existing whole
`internal/specindex` tests and the target's selected `internal/lrfrepo` tests, through heavy.sh.
The staged citation repin found no unchanged content that moved. Go version is go1.27.3.

Retained failure: the whole `tools/ci-forward-row` package ran through `script/heavy.sh` and
failed `TestAFPV0591CoverageGuard` (62.070 seconds). Its harness provides no integrity result,
so the earlier CIN-V0-8721 aggregate check refuses before the coverage assertions. This repair
is limited to CIN-V0-8720; it does not alter that harness or weaken the integrity requirement.
Affected scope is UNKNOWN (76 unknowns: 75 LANGUAGE_FRONTIER, one NO_SELECTABLE_TEST).
Hosted qualification and independent review remain with the orchestrator. Native ticket writes
and publication are prohibited. Rollback: revert this repair; doing so restores the known gap
and must not be represented as a sound authorization policy.
