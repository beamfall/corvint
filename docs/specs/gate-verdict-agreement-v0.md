# Gate Verdict Agreement V0

Owner: Russell Lewis
Date: 2026-10-27
Intent status: experimental under the standing lane auto-accept (2026-10-27; V1-2841)
Delivery status: experimental
Authoritative inputs: V1-2841 / GitHub #1903 owner request, agent-drafted;
`test-source-analysis-v0.md` (`tests lint`, `tests duplicates`); `migration-evidence-ratchet-v0.md`
(`migration-ratchet`); AGENTS.md invariants 2, 3, 4 and 8.

## Agent digest
- Claim: Gate-shaped commands run advisory, compare findings with caller verdicts, log one agreement record per run and summarise agreement per rule.
- Status: experimental under the standing lane auto-accept (2026-10-27; V1-2841)/experimental
- Exists: `internal/gateagreement`, `cmd/corvint/gate_agreement.go`, `--advisory`, `--compare` and `--log` on `corvint tests lint`, `corvint tests duplicates`, `corvint tests validity` and `corvint migration-ratchet`, and `corvint tests agreement`.
- Blocked on: other gate-shaped commands (`tests citations`, `cem gate`) are not wired; real adopter runs are NOT_RUN.
- Read next: Requirements; Failure modes; Traceability.

## User and baseline

An adopter trialling a Corvint gate next to an existing check (a human review, another linter, a
previous CI script) wants to run it without blocking merges and to learn, per rule, how often the
two agree before letting the gate block. The simpler baseline is running the command with
`|| true`, saving its JSON and diffing by hand; that loses the build identity, the exit status and
the distinction between a disagreement and a scope Corvint did not decide. Success is one uniform
option set whose records are append-only, self-describing and summarisable.

## Requirements

- `GVA-V0-001`: (standing lane auto-accept, 2026-10-27; V1-2841) `corvint tests lint`, `corvint tests duplicates` and `corvint migration-ratchet`
  MUST accept `--advisory`, `--compare FILE` and `--log FILE`, each at most once, anywhere among
  the command's options. `--advisory` returns 0 where the command would return 1 and leaves every
  other status, including a refusal's 2, unchanged. `--compare` without `--log` refuses as invalid
  arguments. The command's stdout is unchanged by these options.
- `GVA-V0-002`: (standing lane auto-accept, 2026-10-27; V1-2841) The verdict file (schema `corvint-gate-verdicts/0`, at most 4 MiB and 100 000
  verdicts, strictly decoded) MUST hold `verdicts` of `{rule, path, line, test, other}`, each with a
  rule and a path or test. A line of 0 or none matches any line of the path; `other` names the
  second test of an unordered pair. A malformed, oversized or unreadable file refuses with
  `agreement-verdicts-invalid`. The record names the file as given and its sha256 digest.
- `GVA-V0-003`: (standing lane auto-accept, 2026-10-27; V1-2841) Each command MUST map its outcome to keys: a lint finding is `{rule, path, line}`;
  an unresolved duplicate pair is rule `duplicate-pair` with `test` and `other`; a ratchet policy
  failure `CODE:ID` is rule `CODE` on test `ID` (path `.` when there is no ID). A finding a caller
  verdict matches is an agreement, else Corvint-only; a verdict no finding matches is an
  abstention when it falls where the command did not decide, else caller-only. Lint abstains on an
  UNKNOWN finding (`finding-unknown`), an applied suppression (`suppressed`), an uncertain file
  (`file-uncertain`) and, in changed-lines scope, on every unmatched verdict
  (`outside-changed-lines`); duplicates abstains on an uncertain file (`file-uncertain`) and, when
  the pair list is truncated, on every unmatched pair verdict (`output-truncated`). A verdict for a rule the command
  does not decide (lint: its closed rule set; duplicates: `duplicate-pair`) is listed under
  `ignoredVerdicts` and counted nowhere.
- `GVA-V0-004`: (standing lane auto-accept, 2026-10-27; V1-2841) With `--log`, each run that produces a result MUST append one
  `corvint-gate-agreement/0` record: `at` (UTC), `command`, `arguments` (without the agreement
  options), `build` (the binary's version, build and VCS commit, each stated or `unknown`),
  `revision`, `exitCode`, `advisory`, `advisoryExitCode`, `comparison` (`COMPARED`, or
  `NOT_PRODUCED` with reason `no-verdict-file` when there is no `--compare`), `verdicts`,
  `findings`, the `agreements`, `corvintOnly`, `callerOnly`, `abstentions` (each with its reason)
  and `ignoredVerdicts` lists, and per-rule counts.
- `GVA-V0-005`: (standing lane auto-accept, 2026-10-27; V1-2841) The record MUST be written as one line with one write on an append-only
  descriptor created with mode 0600, before the command's output. A log path inside the analysed
  repository (after resolving its parent's symlinks) refuses with
  `agreement-log-inside-repository`, so a read command never writes the repository (for
  `migration-ratchet` the analysed repository is the source root a profile with record sources
  re-reads; a sourceless profile reads none); a log that is
  not a regular file, cannot be opened or written, or a record over 8 MiB refuses with
  `agreement-log-unavailable` and exit 2.
- `GVA-V0-006`: (standing lane auto-accept, 2026-10-27; V1-2841) A run the command refuses (exit 2 before a result) MUST append nothing.
- `GVA-V0-007`: (standing lane auto-accept, 2026-10-27; V1-2841) `corvint tests agreement --log FILE [--log FILE ...] [--command NAME]` MUST read
  at most 64 logs of at most 256 MiB each, needs no repository, and print one
  `corvint-gate-agreement-summary/0` report: the runs and compared runs, runs per command, and per
  rule the compared runs in which it had an outcome, its agreements, Corvint-only, caller-only and
  abstention counts and `agreementRate`, agreements over agreements plus Corvint-only plus
  caller-only, rounded to four decimals and `null` when that sum is zero. Abstentions never enter
  the rate; a `NOT_PRODUCED` run is counted as a run only.
- `GVA-V0-008`: (standing lane auto-accept, 2026-10-27; V1-2841) A log line that is not a `corvint-gate-agreement/0` record MUST be listed under
  `ignoredLines` with its log and line number and never counted; `--command` keeps only that
  command's records. An unreadable log or a line over 8 MiB refuses with
  `agreement-log-unavailable`.
- `GVA-V0-088`: (standing lane auto-accept, 2026-10-28; V1-2916) `migration-ratchet` MUST also key the receipt's identity deltas as findings: `removal:KIND` on the baseline ID, `state-regression:KIND` on the candidate ID of each state change whose direction is `regression`, and `broken-reverse-link:KIND` on the candidate (else baseline) ID, beside the `CODE:ID` policy failures. A delta carrying `excepted_by` is an abstention with reason `excepted`, not a finding.
- `GVA-V0-089`: (standing lane auto-accept, 2026-10-28; V1-2916) The record MUST carry `baselineRevision` when the command compares against a baseline (`migration-ratchet`), and the optional free-form `--label NAME` (given once) as `label`, so logs of several callers can be split.
- `GVA-V0-094`: (standing lane auto-accept, 2026-10-28; V1-2915) `corvint tests validity` MUST accept the `GVA-V0-001` options and `--label` with the same exit, logging and refusal rules (exit 1 when a test is refused as proof). Each finding is keyed as its rule at the path and line of the test it names (`test` `path:line`), including advisory findings; an `abstained` finding (`finding-abstained`) and a withheld cross-file reading (`cross-file`) abstain at that key, and an uncertain file abstains on its path (`file-uncertain`). The command has no closed rule set, so no verdict is ignored: a caller comparing several commands keeps one verdict file per command.
- `GVA-V0-095`: (standing lane auto-accept, 2026-10-28; review of GVA-V0-094) With `--legacy`, each `seedParity` and `assertionParity` finding of `tests validity` MUST be keyed like a file finding, as its rule at the path and line of the link's new test, so a caller verdict on it agrees rather than reading as `callerOnly`. A link whose seed parity is `not-analysed` or `uncertain` abstains (`parity-not-analysed`) on `SEED-PARITY-DIVERGES` and `SEED-PARITY-UNCERTAIN` there, and one whose assertion parity is `not-analysed` abstains on the unmatched, new-without-origin and weakened rules.
- `GVA-V0-096`: (standing lane auto-accept, 2026-10-28; V1-2960, GitHub #1946) `corvint migration-ratchet --help` MUST name every compare key form `GVA-V0-003` and `GVA-V0-088` emit (a policy failure `CODE:ID` as rule `CODE` on test `ID`, `removal:KIND` on the baseline ID, `state-regression:KIND` on the candidate ID, `broken-reverse-link:KIND` on the candidate, else baseline, ID, and an excepted delta as an abstention) and `--label NAME` (`GVA-V0-089`). The usage lines of `migration-ratchet`, `tests validity`, `tests lint` and `tests duplicates` MUST list `[--label NAME]` with the other agreement options, so the capability tree's flags for each include `--label`.

## Non-goals

Changing any finding, verdict or exit status beyond `--advisory`; deciding which side is right;
storing logs in the repository or any service; rotating or locking logs; an MCP tool; and wiring
gate-shaped commands other than the four named.

## Failure modes

Matching is exact on rule, path, test and pair, so a caller who names a line Corvint reports one
line off disagrees. In lint's changed-lines scope every unmatched verdict abstains, so caller-only
is never produced there; run with `--all-lines` for a full comparison. Concurrent appends from
several processes rely on `O_APPEND` single writes; a record over the platform's atomic write size
may interleave on some file systems. The `at` timestamp and build identity make records
non-deterministic by design. The caller's verdicts are believed as given.

## Acceptance evidence

Unit tests over synthetic findings and verdicts sort every outcome, honour line-free verdicts and
unordered pairs, refuse malformed verdict files, append and summarise logs with per-rule rates, a
command filter and a malformed line. CLI tests run advisory lint and duplicates with a verdict
file and a log, refuse a log inside the repository, `--compare` without `--log` and a repeated
option, append nothing for a refused run, summarise the log, and run the migration ratchet advisory
on a failing receipt and compare its policy failures. `TestGVAV0094ValidityAdvisoryCompareCLI` runs advisory
`tests validity` with a verdict file, a label and a log. No real adopter comparison was run (NOT_RUN).

## Rollout and rollback

Additive and opt-in: without the options every command behaves as before. Rollback removes
`internal/gateagreement`, `cmd/corvint/gate_agreement.go`, the option splitting in
`cmd/corvint/tests.go` and `cmd/corvint/migration_ratchet.go`, the `tests agreement` verb, their
tests, help and capability rows, and this spec. Existing logs are caller files and are left alone.

## Traceability

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| GVA-V0-001 | `cmd/corvint/gate_agreement.go`, `cmd/corvint/tests.go`, `cmd/corvint/migration_ratchet.go` | `TestGVAV0001AdvisoryCompareCLI`, `TestGVAV0003MigrationRatchetAdvisory` | experimental |
| GVA-V0-002 | `internal/gateagreement/agreement.go` | `TestGVAV0002VerdictFileRefusals`, `TestGVAV0003CompareSortsOutcomes` | experimental |
| GVA-V0-003 | `internal/gateagreement/agreement.go`, `cmd/corvint/gate_agreement.go` | `TestGVAV0003CompareSortsOutcomes`, `TestGVAV0001AdvisoryCompareCLI`, `TestGVAV0003MigrationRatchetAdvisory` | experimental |
| GVA-V0-004 | `internal/gateagreement/agreement.go` | `TestGVAV0003CompareSortsOutcomes`, `TestGVAV0001AdvisoryCompareCLI` | experimental |
| GVA-V0-005 | `internal/gateagreement/agreement.go`, `cmd/corvint/gate_agreement.go`, `cmd/corvint/migration_ratchet.go` | `TestGVAV0007SummaryRates`, `TestGVAV0001AdvisoryCompareCLI`, `TestMigrationRatchetCLIVerifiesRecordSourceSpans` | experimental |
| GVA-V0-006 | `cmd/corvint/tests.go` | `TestGVAV0001AdvisoryCompareCLI` | experimental |
| GVA-V0-007 | `internal/gateagreement/agreement.go`, `cmd/corvint/tests.go` | `TestGVAV0007SummaryRates`, `TestGVAV0001AdvisoryCompareCLI` | experimental |
| GVA-V0-008 | `internal/gateagreement/agreement.go` | `TestGVAV0007SummaryRates` | experimental |
| GVA-V0-088 | `cmd/corvint/gate_agreement.go` | `TestGVAV0088RatchetIdentityDeltaKeys` | experimental |
| GVA-V0-089 | `internal/gateagreement/agreement.go`, `cmd/corvint/gate_agreement.go` | `TestGVAV0088RatchetIdentityDeltaKeys` | experimental |
| GVA-V0-094 | `cmd/corvint/tests.go`, `cmd/corvint/gate_agreement.go` | `TestGVAV0094ValidityAdvisoryCompareCLI` | experimental |
| GVA-V0-095 | `cmd/corvint/gate_agreement.go` | `TestGVAV0095ValidityCompareKeepsParityFindings` | experimental |
| GVA-V0-096 | `cmd/corvint/help.go`, `cmd/corvint/tests.go` | `TestGVAV0096HelpDocumentsCompareKeysAndLabel` | experimental |
