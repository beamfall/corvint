# Runner selection self-review repair

V1-5104 / rspt2 repairs four findings against `RSP-V0-009`, `RSP-V0-012`,
`RSP-V0-055` and `LPCV-V0-952..953`. Production enforcement remains disabled.

The soundness class is loss of column information when a line-only edit is
compared to coverage or body ranges. The closed allowlist certifies body-only
edits only on strict interior body lines. Coverage certifies a full line only
when it starts at column 1 (or after the block's starting line) and precedes the
block's ending line. Other forms, including arbitrary prefix/suffix constructs
and inter-block gaps, reach the enclosing executed body; body boundaries use
rule (c). No syntax-specific list of unsafe expressions is maintained. This
conservative choice can select extra tests on partial lines.

`TestLPCVV0952PartialCoverageLines` covers a one-line case expression before an
unexecuted return, unknown prefix/suffix forms, a gap between blocks and a
whole-line negative control. `TestRSPV0012BodyBoundaryReplacements` covers an
uncalled function signature, closing boundary and unknown boundary forms.

Exclusion additionally requires `Record.CollectionReady`. The in-process,
core-owned qualification view carries the explicit single-collection exception;
it is not a plugin or JSON contract. Missing qualification defaults to requiring
agreeing collections. Observed nondeterminism always retains its diagnostic
and selects the test, including with the exception. `TestRSPV0055CollectionReadiness`
covers the single-collection refusal and exception, unadmitted qualification,
agreement, disagreement and an unknown observation.

Modeled build attribution preserves trace and native components while assigning
modeled loaded edges. Modeling is not evidence that unknown observations are
empty. `TestRSPV0009ModeledBuildRetainsTraceAndUnion` checks every attributed test,
template reads, listings, lookups, executable dependencies, unknown states and
subsequent collection unions. The old assertion that required build reads to
disappear is replaced as expressly requested by the repair finding.

Whole-package `internal/runnerselection` and `internal/testevidence` tests passed
through `script/heavy.sh`, selected by the prescribed affected command. All
committed goldens remain unchanged. The first run of the new union regression
failed because it expected an absent component to remain absent in a union;
the established union contract normalizes unknown observations to incomplete.
The new regression now asserts that exact incomplete state and diagnostic.
No pre-existing assertion was weakened to accommodate output.

The installed affected binary reports `self-binary-stale`, and its plan is
`UNKNOWN`; its selection is supporting scope evidence, not equivalent to a
current-binary or repository-wide gate. Raw logs and the affected receipt remain
outside the repository under `/home/agent/lanes/rspt2-evidence/`.
Independent review, repository-wide gates, live qualification and native ticket
completion belong to the orchestrator. The repair changes no wire shape, error
code, exit code, stderr text or frozen output. Rollback reverts this repair;
shadow plans remain non-authoritative either way.

The prescribed `make -s spec-requirements-check line-citations-check` passed,
including its whole specindex package and selected lrfrepo checks, all Go tests
through the existing heavy wrapper. Citation repinning found no moved anchors.

Retained raw-log SHA-256 digests (test durations are Go package reports):

- `repair-runnerselection.log`: PASS, 0.014 s; `76a67e193d6d06b898b9c1e77ea687da9604918cd6389ba3f49ca8fcf8ab88c3`.
- `repair-testevidence-final.log`: PASS, 0.090 s; `aaba50e37a237272d0999cb70319f1e4878b466c5ac04d235caf5886d78e04f8`.
- `repair-spec-citations.log`: PASS; `cdb2c9ffa18fc9ebcbe76b41adcb60158aa34de2dcadb9a71b8f8e2bfdd80ce1`.
