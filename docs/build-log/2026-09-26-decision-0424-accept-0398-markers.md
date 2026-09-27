## 2026-09-26 decision 0424, V1-0385, V1-0339, V1-0342, V1-0230, V1-0341: accept the verified 0398 amendments

The owner delegated acceptance of each verified "(proposed, decision 0398)" marker. Decision 0424
records the result. Two verifier reports were treated as hypotheses, and each wording fix was
checked against the code before it was made. No code contradicted a report.

- Accepted as written: IDX-SNAP-V0-001, 003 and 007; CEM-PILOT-018; PRS Core item 2 and the V1-0021
  criterion amendment; the AFP-V0-021 sidecar narrowing; UCV0-014. The IDX-SNAP-V0-007 base clause
  said one hour, but `snapshotTemporaryStaleAfter` is ten minutes, so it now says ten minutes.
- Accepted with the wording corrected to match the code:
  - GPK-V0-027. Each path keeps its own refusal code, as `mapStandaloneQueryBuildError` and
    `queryAdmissionError` show.
  - AFP-V0-009. The comment rule is stated as `stripShellComment` applies it, and the no-gate unknown
    is added only when the declaration sources were not truncated.
  - AFP-V0-021 rule (d). There are four corrections, following `escapesPackage`, the Go plugin's
    lex step and `unboundedReaders`: only a lex failure after the imports marks a unit; a non-test
    literal must climb to a named component; a test literal made only of `..` counts; and test users
    are reached only from `locatesRoot` units and their dependents.
- The runbook's step 8 marker was stale, because decision 0422 had already accepted it. Its
  direction was also reversed: `replayCoreModeN1` requires every N-1 member to still be emitted
  here, not the converse.
- `coreN1Skips` also skips `impact path non-utf8 source`. The v0.8.1 binary refuses that fixture
  with exit 2 ("Git tree output is malformed"), and 53c02369 (IDX-SNAP-V0-024) is not an ancestor of
  v0.8.1. CCF-V1-007 now names this skip.
- Still proposed: AFP-V0-002, 004 and 005; TJAA-V0-005; TCQ-V0-052; CEM-SM-006; TCP-V0-015;
  UCV0-015. Wording was fixed for AFP-V0-004 (every importer of an absent in-module package at the
  module root) and TCP-V0-015 (the conditional action applies to test rows only).
- V1-0385. The DCG-V0-014 rationale, the `check-line-citations.sh` message and its test case 11 no
  longer describe the build log as prepended. The pin rule is unchanged, and the two DCG-V0-014
  trace pins were repinned after the changed lines were read.

To fit the GPK-V0-027 rewording in its original five lines, those lines are longer than the usual
wrap width. Moving them would shift the pinned citation in decision 0093.

Evidence: the doc checks, `use-case-receipts-check`/`-test`, `conformance/use-cases-v0`, the
specindex tests and `go vet ./cmd/corvint/` pass. The exhaustive gate is `NOT_RUN` (documentation
and comments only). Rollback: revert this change.
