# Playwright Run Evidence V0

Owner: Russell Lewis
Date: 2026-10-11
Intent status: proposed, pending owner acceptance (V1-1230)
Delivery status: experimental
Authoritative inputs: V1-1230 / GitHub #798 owner request; AGENTS.md invariants 1, 2, 4, 7 and 8.

## Agent digest
- Claim: A read-only reader of Playwright JSON reports classifies each test's outcome stability across runs at identical declared identity inputs.
- Status: proposed, pending owner acceptance (V1-1230)/experimental
- Exists: `internal/playwrightruns` and the experimental `corvint runs stability` command.
- Blocked on: owner acceptance; fixture durations, cleanup and passing assertions are not in the JSON reporter output.
- Read next: Requirements; Boundaries and failure modes; Delivery.

## User and verified baseline

The adopter reruns a Playwright suite several times and needs to know which tests change outcome
when nothing that should matter has changed. At `8b659142492966161ca82d1f5ff8f503fec1c504` the CLI
has no Playwright run command. `internal/appflows` and `internal/jstestprovider` parse the same
reporter, but neither keeps project, describe title path, worker index and expected status together,
and neither is a run-set view. The simpler baseline is reading `flaky` markers in one HTML report,
which sees only retries inside one run. Success is a repeatable local verdict per test that abstains,
with a named reason, whenever the runs cannot be shown to share their identity inputs.

## Requirements

- `PWR-V0-001`: One shared reader MUST read an explicit Playwright json-reporter file read-only (a
  regular file opened non-blocking, at most 64 MiB, one JSON value with `config.version` and a
  `suites` array, at most 50 000 tests, 100 results per test and 16 errors per result) and refuse
  anything else with a named reason (`REPORT_UNREADABLE`, `REPORT_BOUND_EXCEEDED`,
  `REPORT_NOT_PLAYWRIGHT_JSON`); unknown test or result statuses are refused, never mapped. A test's
  stable identity is (project, file, title path), where the title path is the describe titles plus
  the test title without the file suite. Two tests with one identity in one report are recorded as
  ambiguous. First-error lines are the first non-empty line after ANSI removal, scrubbed by the
  shared run-hygiene and secret screens, and limited to 240 bytes. The reader writes nothing and
  uses no network.
- `PWR-V0-002`: Runs pool only at identical identity inputs: the Playwright version plus the
  `applicationRevision`, `configDigest` and `testFileBlobs[file]` values the run declares under
  `config.metadata.corvintRunIdentity`. Nothing is inferred from paths, timestamps or report
  content; a report without the declaration, or a test whose file has no declared blob, yields
  `IDENTITY_INPUT_MISSING`. A changed test file blob is a different pool.
- `PWR-V0-003`: `Stability(runs, minRuns)` MUST classify each pool: `INTERMITTENT` when outcomes
  differ or any run is Playwright-`flaky` (at any run count, listing the distinct first-error lines in
  run order); `DETERMINISTIC` only when at least `minRuns` (default 3, minimum 2, maximum 64) runs
  share one outcome; otherwise `INSUFFICIENT` with `BELOW_THRESHOLD`. Unpoolable observations are
  `INSUFFICIENT` with `IDENTITY_INPUT_MISSING`, `IDENTITY_AMBIGUOUS`, `RUN_INTERRUPTED` or `NOT_RUN`,
  and never contribute to any pool. A run's outcome is `flaky` or its last result status.
- `PWR-V0-004`: An `INTERMITTENT` pool whose test is expected to fail (`expectedStatus` `failed` or a
  `fail` annotation) MUST carry the flag `EXPECTED_FAIL_NOT_DETERMINISTIC` and two suggestions:
  replace `test.fail()` with `test.fixme()` or `test.skip()` naming the defect id, or fix the varying
  setup. A deterministic expected-fail is not flagged.
- `PWR-V0-005`: `corvint [--root PATH] runs stability --runs FILE[,FILE...] [--runs ...] [--min-runs N]`
  MUST read at most 64 explicit report files (relative to `--root` when given), emit one compact JSON
  document with profile `corvint-runs-stability/0`, the admitted runs (position, SHA-256, version,
  whether inputs were declared), sorted entries and verdict counts, and exit 0. Invalid arguments
  refuse with `runs-invalid-arguments`, an unreadable or refused report with `runs-report-unavailable`
  (naming its reason and 1-based position), and an output failure with `runs-output-unavailable`; a
  refusal exits 2 with an empty stdout and never echoes argument values. Root and topic help MUST
  mark the command experimental under owner prefix PWR-V0.

## Boundaries and failure modes

Non-goals: running Playwright, fetching reports, reading HTML/blob/JUnit reporters, directory or glob
input, receipts as identity input, inferring identity from Git, and feeding obligation outcomes from
the expected-fail flag. Those remain `NOT_RUN` until a later requirement.

Failure modes: a report that declares false identity inputs pools runs that differ; the verdict is
only as good as the declaration, and the reader cannot verify it. An interrupted, ambiguous or
undeclared observation abstains rather than joining a pool. Two runs below the threshold never become
`DETERMINISTIC`. Secret screening is heuristic. The JSON reporter omits fixture timings, cleanup and
passing `expect` calls, so they stay `NOT_OBSERVED`.

## Implementation, rollout and rollback

Keep parsing in `internal/playwrightruns`, CLI parsing in `cmd/corvint/runs.go` and the dispatch and
help edits minimal. This additive experimental profile changes no Core profile or persisted format.
Rollback removes the dispatch line, the help entries and the package; there is no data migration.
Promotion beyond experimental needs owner acceptance of these requirements.

## Acceptance evidence

Synthetic reports shaped like `@playwright/test` 1.63 json-reporter output: `{fail,pass,fail}` at
identical inputs is `INTERMITTENT` with both first errors; a changed test file blob is not pooled;
fewer runs than the threshold is `INSUFFICIENT`; an intermittent expected-fail carries the flag.

## Delivery

| Requirement | Implementation | Focused evidence | Status |
|---|---|---|---|
| PWR-V0-001 | `internal/playwrightruns` | `TestReaderIdentityAndHygiene`, `TestReadFileRefusesFIFO` | experimental |
| PWR-V0-002 | `internal/playwrightruns` | `TestStabilityVerdicts` | experimental |
| PWR-V0-003 | `internal/playwrightruns` | `TestStabilityVerdicts`, `TestRunsStabilityCLI` | experimental |
| PWR-V0-004 | `internal/playwrightruns` | `TestStabilityVerdicts` | experimental |
| PWR-V0-005 | `cmd/corvint/runs.go` | `TestRunsStabilityCLI` | experimental |

Repository-wide gate is `NOT_RUN` in this scoped lane.
