# Playwright Run Evidence V0

Owner: Russell Lewis
Date: 2026-10-11
Intent status: accepted (decision 0520; V1-1230, V1-1229)
Delivery status: experimental
Authoritative inputs: V1-1230 / GitHub #798 and V1-1229 / GitHub #797 owner requests; GitHub
#809 (repeat-each) and #810 (shards) for PWR-V0-011..023, agent-drafted and accepted under the
owner's standing auto-accept (2026-10-11) after independent review; V1-1578 / GitHub #1021 (result
reuse) and #1024 (evidence freshness) for PWR-V0-033..040, agent-drafted and accepted under the same
standing auto-accept; V1-1590 / GitHub #1033 (a revision-pinned failure baseline for the run gate)
for PWR-V0-041..045, agent-drafted and accepted on 2026-10-17 under the same standing auto-accept; AGENTS.md invariants 1, 2, 4, 7 and 8.

## Agent digest
- Claim: A read-only reader of Playwright JSON reports classifies per-test outcome stability across runs and behaviour parity between baseline and candidate runs.
- Status: accepted (decision 0520; V1-1230, V1-1229)/experimental
- Exists: `internal/playwrightruns` and the experimental `corvint runs stability` and `corvint runs compare` commands; each `--repeat-each` repeat is one observation, and shard reports merge into one logical run only as a complete set, else `SHARD_SET_INCOMPLETE`, `SHARD_DUPLICATE` or `SHARD_INPUTS_NOT_DECLARED` (PWR-V0-011..023); `corvint runs reusable` lists each selected test as `REUSABLE` or `MUST_RUN` with a reason from fingerprinted prior passes, `corvint runs redundancy` reports re-run test minutes by category, and `corvint runs fresh` reads evidence freshness over the repository-declared reach in `.corvint/evidence-reach.json` (PWR-V0-033..040); `corvint runs baseline record` and `diff` gate candidate runs on a revision-pinned failure baseline whose known entries were confirmed by an isolated re-run (PWR-V0-041..045).
- Blocked on: owner acceptance; fixture durations, cleanup and passing assertions are not in the JSON reporter output.
- Read next: Requirements; Boundaries and failure modes; Delivery.

## User and verified baseline

The adopter reruns a Playwright suite several times and needs to know which tests change outcome
when nothing that should matter has changed, and whether a candidate change (for example a page-object
refactor) keeps the suite's behaviour. At `8b659142492966161ca82d1f5ff8f503fec1c504` the CLI
has no Playwright run command. `internal/appflows` and `internal/jstestprovider` parse the same
reporter, but neither keeps project, describe title path, worker index and expected status together,
and neither is a run-set view. The simpler baseline is reading `flaky` markers in one HTML report,
which sees only retries inside one run, and diffing two reports by hand. Success is a repeatable local
verdict that abstains, with a named reason, whenever the runs cannot be shown to be comparable.

## Requirements

- `PWR-V0-001`: One shared reader MUST read an explicit Playwright json-reporter file read-only (a
  regular file opened non-blocking, at most 64 MiB, one JSON value with `config.version` and a
  `suites` array, at most 50 000 tests, 100 results per test and 16 errors per result) and refuse
  anything else with a named reason (`REPORT_UNREADABLE`, `REPORT_BOUND_EXCEEDED`,
  `REPORT_NOT_PLAYWRIGHT_JSON`); unknown test or result statuses and a test without `projectName`
  are refused, never mapped. A test's stable identity is (project, file, title path), where the
  title path is the describe titles plus the test title without the file suite. Two tests with one
  identity in one report are recorded as ambiguous. First-error lines are the first non-empty line
  after ANSI removal, scrubbed by the shared run-hygiene and secret screens, and limited to 240
  bytes. Paths are read in either slash or backslash syntax and emitted as cleaned slash paths; an
  error location under `config.rootDir` is made rootDir-relative. Every emitted identity part,
  location, version and declared input is secret-screened, while matching, pooling and assertion
  comparison use the unscreened values so screening never merges two tests or hides a difference. The reader writes nothing and uses no
  network.
- `PWR-V0-002`: Runs pool only at identical identity inputs: the Playwright version plus the
  `applicationRevision`, `configDigest` and `testFileBlobs[file]` values the run declares under
  `config.metadata.corvintRunIdentity`. Nothing is inferred from paths, timestamps or report
  content; a report without the declaration, or a test whose file has no declared blob, yields
  `IDENTITY_INPUT_MISSING`, as does a file whose declared spellings (for example `a.spec.ts` and
  `./a.spec.ts`) name different blobs; those conflicting declarations still take part in the
  `PWR-V0-008` within-side input comparison. A changed test file blob is a different pool.
- `PWR-V0-003`: `Stability(runs, minRuns)` MUST classify each pool: `INTERMITTENT` when outcomes
  differ or any run is Playwright-`flaky` (at any run count, listing the distinct first-error lines in
  run order); `DETERMINISTIC` only when at least `minRuns` (default 3, minimum 2, maximum 64) runs
  share one outcome; otherwise `INSUFFICIENT` with `BELOW_THRESHOLD`. Unpoolable observations are
  `INSUFFICIENT` with `IDENTITY_INPUT_MISSING`, `IDENTITY_AMBIGUOUS`, `RUN_INTERRUPTED` or `NOT_RUN`,
  and never contribute to any pool. A run's outcome is `flaky`; `notRun` (and `NOT_RUN`) when the test
  has no result or its last result is `skipped` while `expectedStatus` is not `skipped`, which is how
  Playwright records a test it never started after a setup or serial-suite failure; otherwise its last
  result status.
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

- `PWR-V0-006`: `Compare(baseline, candidate)` MUST match tests by the `PWR-V0-001` identity across
  the two report sets (1 to 64 reports per side). For each test and side it reports the report count
  N, the outcome histogram (a report without the test counts as `absent`), `k/N` stability (k is the
  most frequent outcome's count), the highest retry index and the distinct failed assertions
  (origin, matcher, file, line) over every result. A side without the test still reports N
  `absent`. A test whose identity is ambiguous in any report is `AMBIGUOUS`, with no side statistics
  and no other verdict. Otherwise a test is `MISSING` when no candidate report has it, `NEW` when no
  baseline report has it, `CHANGED` when the outcome histograms differ in outcomes
  or proportions, the highest retry differs, or the test-file or unknown-origin assertions differ, and
  otherwise `SAME`.
- `PWR-V0-007`: A failed assertion's origin MUST come only from its reported location: `TEST_FILE`
  when it is the test's own file, `SUPPORT` for any other file (a page object, helper or fixture), and
  `UNKNOWN` without a location. The matcher is the complete `expect(...)` matcher identifier in the error text up to the next space, dot or
  parenthesis (custom and Unicode matcher names included),
  including any `not`, `resolves` or `rejects` modifiers (for example `resolves.not.toHaveText`), or
  empty. A difference only in `SUPPORT` assertions keeps the test `SAME`, sets `supportDifference`,
  and is counted in `summary.supportDifferences` and the reason `SUPPORT_ASSERTIONS_DIFFER`.
- `PWR-V0-008`: The overall verdict MUST be `INCOMPARABLE` with every applicable reason, in the fixed
  order `REPORT_UNREADABLE` (with a problem row naming side, 1-based report and the refusal reason;
  no test rows), `PLAYWRIGHT_VERSION_DIFFERS`, `PROJECTS_DIFFER` (the test projects or the configured `config.projects` names differ),
  `CONFIG_DIFFERS` (every report
  declares a `configDigest` and they differ), `INPUTS_DIFFER_WITHIN_SIDE` (declared identity inputs
  differ among the declaring reports of one side, in any order), `REPORT_HAS_ERRORS` (top-level report errors), `NO_TESTS`,
  `IDENTITY_AMBIGUOUS` and `RUN_INTERRUPTED`; an unreadable report does not suppress the reasons the
  admitted reports show. Otherwise it is `DIVERGED` with `TESTS_CHANGED`,
  `TESTS_MISSING` or `TESTS_NEW`, or else `PARITY`. A report without the identity declaration adds
  the unknown `IDENTITY_INPUTS_NOT_DECLARED` instead of being treated as matching.
- `PWR-V0-009`: Each admitted report MUST be listed per side with its position, SHA-256, Playwright
  version, test projects, configured projects, whether identity inputs were declared, configured workers, the sorted distinct
  worker indexes and the summed result duration. Fixture durations and cleanup outcomes MUST be
  `NOT_OBSERVED`, because the json reporter does not serialize them.
- `PWR-V0-010`: `corvint [--root PATH] runs compare --baseline FILE[,FILE...] --candidate FILE[,FILE...]`
  (each option repeatable, at most 64 files per side) MUST emit one compact JSON document with profile
  `corvint-runs-compare/0` and exit 0, including when a report is unreadable (then `INCOMPARABLE`).
  Invalid arguments refuse with `runs-invalid-arguments` and an output failure with
  `runs-output-unavailable`, exiting 2 without echoing argument values. Topic help MUST document the
  verdicts, reasons and `NOT_OBSERVED` facts.
- `PWR-V0-011`: The reader MUST read `config.projects[].repeatEach` and, for a project whose
  config entries all declare one value above 1, treat each spec entry of a test as one repeat,
  numbered from 0 in report order (the JSON reporter carries no repeat index). Stability counts each
  repeat as one observation of its logical run, so one `--repeat-each 3` report with outcomes
  `{passed, failed, passed}` is `INTERMITTENT` and is never `BELOW_THRESHOLD` at `--min-runs 3`.
- `PWR-V0-012`: A test identity that occurs more than once in a logical run stays
  `IDENTITY_AMBIGUOUS` (compare `AMBIGUOUS`) unless every occurrence has one spec location and the
  count equals the project's declared `repeatEach`; a spec without a reported line has no location
  to share. A single occurrence is one plain observation. A `repeatEach` above 1024 refuses the
  report as `REPORT_BOUND_EXCEEDED`.
- `PWR-V0-013`: Stability output MUST add `repeatEach` to a run summary and `repeats[]` (the repeat
  index of each observation, parallel to `runs[]`) to an entry only when a repeat-each run
  contributes, and `shard` to a run summary only when the report declares one; all are omitted
  otherwise.
- `PWR-V0-014`: Compare MUST lay each test out over observation slots: per admitted logical run, as
  many slots as the run declares repeats for the test's project, an unfilled slot counting as
  `absent`. N repeats in one report then give the same `k/N` as N single-repeat reports. Report
  facts add `repeatEach` and `shard` only when declared.
- `PWR-V0-015`: Unsharded single-repeat inputs MUST produce stability and compare documents
  byte-identical to the output before `PWR-V0-011`, pinned by goldens taken at
  `645304a2fb71065f9061d8d8d866600d703e84c6`.
- `PWR-V0-016`: A report declaring `config.shard` MUST have `1 <= current <= total` (else
  `REPORT_NOT_PLAYWRIGHT_JSON`) and `total <= 1024` (else `REPORT_BOUND_EXCEEDED`). Shard reports
  group into one logical run by shard total, Playwright version, declared `applicationRevision`,
  `testRevision` (PWR-V0-033), `configDigest` and `testFileBlobs`, and `repeatEach` declaration; reports that differ in any of
  these are never merged. A logical run is indexed by its first report's
  position; a report without `config.shard` is its own logical run.
- `PWR-V0-017`: A shard set is admitted only when shards 1..total are each present exactly once at
  declared inputs. Otherwise it is refused, in this precedence, with `SHARD_INPUTS_NOT_DECLARED`,
  `SHARD_SET_INCOMPLETE` or `SHARD_DUPLICATE`, and `shardSets[]` lists every set with its reports,
  shard indexes, status, reason, `missing` and `duplicate` indexes.
- `PWR-V0-018`: An admitted set is read as one run at its shared inputs: tests concatenated in shard
  order, projects united, workers and report errors summed, and repeats resolved over the whole set, so the same test in two shards is ambiguous
  unless `repeatEach` explains it.
- `PWR-V0-019`: Stability MUST count an admitted set as one run, giving the same entries as one
  unsharded report of the same tests; every test of a refused set is `INSUFFICIENT` with the set's
  refusal reason and never pools.
- `PWR-V0-020`: Compare MUST treat an admitted set as one report of its side. A refused set adds its
  reason, in the fixed order after `REPORT_UNREADABLE`, and a problem naming the side and logical run;
  the verdict is `INCOMPARABLE` and test rows are withheld, never `PARITY`.
- `PWR-V0-021`: Sharding and repeat-each MUST compose: Playwright shards repeat instances, so a
  sharded `--repeat-each` run is resolved over its merged set, and shards that declare different
  `repeatEach` fall in different groups and are refused as incomplete.
- `PWR-V0-022`: `merge-reports` JSON output (no `config.shard`, projects listed once per merged shard)
  MUST read as one unsharded run with the same stability and compare entries as the unsharded report.
- `PWR-V0-023`: `corvint runs` topic help MUST document repeat observations, shard sets, the three
  shard refusals and the `shardSets[]` and `repeats[]` fields, and both commands MUST carry them end to
  end without writing.
- `PWR-V0-033`: A retained result's fingerprint MUST be (Playwright version, declared
  `applicationRevision`, declared `testRevision`, the declared `testFileBlobs` entry of the test's
  file, declared `configDigest`, project). `testRevision`, the optional repository-relative
  `testRootPath` of `config.rootDir`, the optional `corvintRunCategory` and `stats.startTime`
  (RFC 3339) are read from the report as declared, never inferred from Git; an unreadable or
  ill-typed member is absent. A qualified pass is an observation of an admitted logical run whose
  outcome is `passed` with status `expected`, that is not ambiguous, not interrupted, not in a report
  with top-level errors, and not in a refused shard set.
- `PWR-V0-034`: `Reusable` MUST give every distinct selected test exactly one row, `REUSABLE` or
  `MUST_RUN`, with one reason. `EXACT_PASS` (`REUSABLE`) needs at least one prior at the selected
  fingerprint and every such prior a qualified pass. Otherwise it fails closed: `IDENTITY_AMBIGUOUS`
  for an ambiguous selected identity, `IDENTITY_INPUT_MISSING` naming the undeclared selected inputs
  (or detail `prior` when only undeclared priors exist), `NO_PRIOR_RESULT`, `PRIOR_NOT_PASSED`
  (failed, flaky, skipped or unexpected) or `PRIOR_NOT_QUALIFIED` (interrupted, ambiguous, report
  errors, refused shard set) naming the prior runs and detail.
- `PWR-V0-035`: A prior whose fingerprint differs from the selection in any input MUST NOT make the
  test reusable; without a freshness condition the row is `FINGERPRINT_CHANGED` naming the differing
  fields of the nearest prior in the fixed order playwrightVersion, applicationRevision,
  testRevision, testFileBlob, configDigest. A different project is a different test.
- `PWR-V0-036`: With the freshness condition (`--fresh`), a prior that differs only in
  `applicationRevision` and/or `testRevision` is `FRESH_PASS` (`REUSABLE`) only when every differing
  revision pair is `FRESH` for the test file under PWR-V0-039 and every prior at that revision pair is
  a qualified pass. A `STALE` pair is `STALE` naming the fields and at most 16 deciding paths plus a
  truncated count; an unresolved pair is `FRESHNESS_UNRESOLVED` with the freshness reason, including
  `TEST_ROOT_UNRESOLVED` when the test file cannot be located through `testRootPath` or a `rootDir`
  inside the repository, or when its report file name is absolute, volume-qualified or escapes the
  root (checked before it is joined to the root). Freshness never overrides a blob, config, version or project change.
- `PWR-V0-037`: `Redundancy` MUST order logical runs by `stats.startTime`, exclude a run without one
  (`START_TIME_MISSING`) or a refused shard set (`SHARD_SET_REFUSED`), and with `--since` keep earlier
  runs as `HISTORY` whose passes still count. For each counted executed test it MUST tally total
  time, exact re-run time (a qualified pass at the full fingerprint strictly earlier), same-file-content
  re-run time (else a strictly earlier qualified pass at the same version, test-file blob, config
  digest and project) and unfingerprinted time, per `corvintRunCategory` (else `UNDECLARED`) and in
  total, with minutes rounded to 0.01 and shares of total time to 0.0001.
- `PWR-V0-038`: The shared-code reach MUST be declared by the repository in
  `.corvint/evidence-reach.json` (profile `corvint-evidence-reach/0`, a `shared` array of at most 256
  patterns, at most 64 KiB, unknown fields refused), read from the NEW commit's tree as a regular
  blob. A pattern is a clean repository-relative slash path whose segments are `path.Match` globs; a
  whole `**` segment spans any number of segments (at most 4), and a trailing `/` covers a directory.
  No shared-code path is hard-coded; the declaration file itself is always in reach.
- `PWR-V0-039`: `runs fresh` MUST read the paths changed between the OLD and NEW commits through
  bounded pinned Git (`diff-tree --no-renames`, at most 16 MiB) and return `FRESH` only when no
  changed path is cited, matches a shared pattern or is the declaration; `STALE` naming each deciding
  path and why (`cited`, `shared`, `reach-config`, sorted, at most 256 plus a truncated count); or
  `UNRESOLVED`, never `FRESH`, with `REVISION_UNRESOLVED`, `REACH_UNDECLARED`, `REACH_INVALID`,
  `REACH_UNREADABLE`, `DIFF_FAILED`, `DIFF_BOUND_EXCEEDED` or `CITED_PATH_INVALID`.
- `PWR-V0-040`: `corvint runs reusable --selection R --prior R...`, `runs redundancy --runs R...
  [--since T]` and `runs fresh --old REV --new REV [--cite PATH,...]` MUST be read-only, refuse
  malformed arguments as `runs-invalid-arguments` and unreadable reports as
  `runs-report-unavailable` naming the option and position, offer `--summary` (counts and verdict
  without per-test or per-run rows, such as `priors` and `shardSets`, or path lists), and be documented in `corvint runs` topic help.

### Revision-pinned failure baseline (GitHub #1033, V1-1590)

The run gate today fails on every failing test, so a branch that inherits a broken test from its
base is blocked by failures it did not cause. PWR-V0-041..045 let the gate block only on failures
that are not in a baseline recorded at a pinned revision, without ever excusing a flake or a change's
own tests.

- `PWR-V0-041`: `corvint runs baseline record --rev REV --full R... --isolated R...` MUST print a
  `corvint-run-baseline/0` document `{profile, revision, known, flaky}` for the caller to retain,
  with `revision` the full commit id REV resolves to. A test is `known` only when it failed
  unambiguously (Playwright `unexpected`) in a full-run report and in an isolated re-run report;
  each known entry binds both reports' paths and sha256. A test that failed, or was flaky, in the
  full run only is listed under `flaky` with its full-run evidence. Identities a report holds
  ambiguously are left out. A report that declares an `applicationRevision` other than REV refuses
  `runs-baseline-unavailable` (`APPLICATION_REVISION_DIFFERS`); more than 4096 entries refuse
  (`BASELINE_BOUND_EXCEEDED`); an unresolvable REV refuses `runs-revision-unresolved`.
- `PWR-V0-042`: `runs baseline diff --baseline B --runs R...` MUST read B strictly (at most 16 MiB,
  unknown members, another profile, a revision that is not a full object id, a known entry without
  isolated evidence or a duplicate identity refuse `runs-baseline-unavailable`). A candidate failure
  that is a known entry is excused (`KNOWN_VERIFIED`) only when both recorded reports re-read at diff
  time with their recorded sha256 and still fail the test unambiguously; otherwise it blocks with
  `EVIDENCE_MISSING`, `EVIDENCE_CHANGED` or `EVIDENCE_PASSING`.
- `PWR-V0-043`: A candidate failure in a `flaky` entry MUST block as `SUSPECTED_FLAKE` and is never
  excused. A known failure whose file is under an `--own` path (resolved through each candidate's
  `testRootPath`) MUST block as `OWN_FILE`. A failure not in the baseline blocks as `NOT_IN_BASELINE`,
  an ambiguous failure as `IDENTITY_AMBIGUOUS`, and a candidate report with top-level errors blocks
  the verdict with `REPORT_HAS_ERRORS`.
- `PWR-V0-044`: The diff MUST print a `corvint-runs-baseline-diff/0` receipt with `verdict`
  (`PASS` or `BLOCKED`) and `reasons`, bound to the baseline `revision` and the sha256 of its bytes
  and to each candidate report's path and sha256, listing the `new`, `known` (each `excused` with a
  reason), `flaky`, `fixed` (baseline entries the candidate passed) and `unobserved` sets, sorted,
  with `counts` of each plus `excused` and `blocking`. It exits 1 when `BLOCKED`, 0 on `PASS`.
- `PWR-V0-045`: With `--base REV`, the receipt MUST report `staleness` as `STALE` when the baseline
  revision is not an ancestor of the branch point (the merge base of `HEAD` and REV) or is more than
  `--max-age` commits (default 50) behind it, `FRESH` otherwise with the commit count, and
  `NOT_OBSERVED` with a reason (`BASE_NOT_GIVEN`, `REVISION_UNRESOLVED`, `BRANCH_POINT_UNRESOLVED`,
  `COUNT_UNAVAILABLE`) when it cannot be read. Staleness is reported, not a verdict input.

## Boundaries and failure modes

Non-goals: running Playwright, fetching reports, reading HTML/blob/JUnit reporters, directory or glob
input, receipts as identity input, inferring identity from Git, feeding obligation outcomes from
the expected-fail flag, telling page objects, helpers and fixtures apart, and counting passing
assertions. Those remain `NOT_RUN` until a later requirement.

Shard and repeat limits: two complete shard sets at the same inputs form one group with every shard
duplicated and are refused as `SHARD_DUPLICATE` (pass them as one run each through `--repeat-each`
or `merge-reports`); repeat order is report order; a set with undeclared inputs is refused rather
than grouped on Playwright version alone; `merge-reports` merges metadata shallowly, so its identity
inputs are whatever the shard reports shared.

Failure modes: a report that declares false identity inputs pools runs that differ; the verdict is
only as good as the declaration, and the reader cannot verify it. An interrupted, ambiguous or
undeclared observation abstains rather than joining a pool. Two runs below the threshold never become
`DETERMINISTIC`. Compare reads assertion origin from error locations only: an assertion that moves
lines in a refactored test file is `CHANGED`, and a matcher absent from the error text is empty.
Undeclared configs are compared only on version and projects and stay an explicit unknown. Secret
screening is heuristic. The JSON reporter omits fixture timings, cleanup and
passing `expect` calls, so they stay `NOT_OBSERVED`.

Reuse and freshness non-goals (PWR-V0-033..040): writing or caching results, a retained-result store
beyond the reports the caller names, computing fingerprints from the working tree, per-test import
graphs, rename tracking, `corvint test` (STR-V0) receipts, and skipping tests: the read only lists
decisions. Reuse limits: the decision trusts declared fingerprints exactly as stability does; a
`FRESH_PASS` is as good as the declared reach, which covers only cited and declared shared paths, so
an undeclared dependency outside them can still change behaviour. A report from another machine
needs `testRootPath` to locate its test files; without it the decision is `MUST_RUN` with
`FRESHNESS_UNRESOLVED`/`TEST_ROOT_UNRESOLVED`, never a guess from `config.rootDir` (V1-1611). Redundancy undercounts tests without a start time or
fingerprint, which it reports as excluded or unfingerprinted rather than guessing.

Baseline non-goals (PWR-V0-041..045): running the suite or the isolated re-run (the caller runs both
and names the reports), storing the baseline (the caller retains the printed document), a baseline
cache or index, excusing a flake after any number of observations, and blocking on staleness.
Baseline failure modes: a moved or rewritten evidence report un-excuses its known failure, which is
the intended fail-closed direction; a test whose presented identity was secret-screened never
matches its candidate key and so blocks as new; a candidate `flaky` result is neither failing nor
fixed and leaves its baseline entry unobserved.

## Implementation, rollout and rollback

Keep parsing in `internal/playwrightruns`, CLI parsing in `cmd/corvint/runs.go` and the dispatch and
help edits minimal. This additive experimental profile changes no Core profile or persisted format.
Rollback removes the dispatch line, the help entries and the package; there is no data migration.
Reuse and freshness (PWR-V0-033..040) roll back by removing the `reusable`, `redundancy` and `fresh` dispatch cases and help,
`cmd/corvint/runs_reuse.go`, `internal/playwrightruns/reuse.go` and `redundancy.go`, the declared-fact
fields, `internal/evidencereach` and `.corvint/evidence-reach.json`; nothing is persisted. They extend
`corvint runs` rather than adding a top-level verb so the root dispatch and its receipts stay unchanged.
The baseline (PWR-V0-041..045) rolls back by removing the `baseline` dispatch case and help,
`cmd/corvint/runs_baseline.go` and `internal/playwrightruns/baseline.go`; retained baseline documents
are caller files and become inert.
Promotion beyond experimental needs owner acceptance of these requirements.

## Acceptance evidence

Synthetic reports shaped like `@playwright/test` 1.63 json-reporter output: `{fail,pass,fail}` at
identical inputs is `INTERMITTENT` with both first errors; a changed test file blob is not pooled;
fewer runs than the threshold is `INSUFFICIENT`; an intermittent expected-fail carries the flag.
Identical runs compare as `PARITY`; a changed test-file assertion is `DIVERGED`; a different
Playwright version, config digest or project set, or a missing report, is `INCOMPARABLE`; a
support-only assertion difference stays `SAME` and is reported as a support difference.

Repeat-each and shard reports produced by `@playwright/test` 1.61.1 and sanitized
(`internal/playwrightruns/testdata/runs`, provenance in the 2026-10-11 runs build-log entry): one
`--repeat-each 3` report is `INTERMITTENT` with `repeats [0,1,2]`; three shards and `merge-reports`
output equal one unsharded run; two of three shards is `SHARD_SET_INCOMPLETE` naming shard 3; a
rerun shard is `SHARD_DUPLICATE`; a shard at another application revision is not merged; a test in
two shards is ambiguous; a sharded `--repeat-each 2` run is `INTERMITTENT` with `k/N` 1/2.

Reuse and freshness (synthetic reports and temporary Git repositories): a qualified prior at the same
fingerprint is `EXACT_PASS`; each changed fingerprint input is `FINGERPRINT_CHANGED` naming it; missing
inputs, no prior, a failed, flaky, interrupted, error-carrying or refused-shard prior are `MUST_RUN`;
`--fresh` turns a changed application revision into `FRESH_PASS` when only out-of-reach paths
changed, `STALE` naming `e2e/support/login.ts` when a shared path changed, and
`FRESHNESS_UNRESOLVED`/`TEST_ROOT_UNRESOLVED` for an unlocated test root. Redundancy splits exact from
same-file-content re-runs, ignores ties and later passes, and keeps `--since` history. `runs fresh`
returns `FRESH`, `STALE` for cited, shared and declaration changes, and `UNRESOLVED` for each
resolution error; the repository is byte-unchanged after the reads. Live qualification ran the three
reads on this repository's retained reports and commits (2026-10-17 V1-1578 build-log entry).

Baseline (synthetic reports and a temporary Git repository): a failure in both runs is known and one
in the full run only, or flaky there, is a suspected flake; a verified known failure passes the gate;
new and flaky failures block; missing, changed and passing evidence and an own file un-excuse a known
failure; report errors block; passing entries are fixed; a malformed baseline, a revision-declaring
report at another revision and an unresolved revision refuse; staleness is `FRESH` at 3 of 3 commits,
`STALE` past `--max-age 2` and `NOT_OBSERVED` without `--base`. Running real Playwright suites for
the full and isolated runs is `NOT_RUN`.

## Delivery

| Requirement | Implementation | Focused evidence | Status |
|---|---|---|---|
| PWR-V0-001 | `internal/playwrightruns` | `TestReaderIdentityAndHygiene`, `TestReadFileRefusesFIFO`, `TestReaderAndCompareEdgeCases` | experimental |
| PWR-V0-002 | `internal/playwrightruns` | `TestStabilityVerdicts`, `TestReaderAndCompareEdgeCases` | experimental |
| PWR-V0-003 | `internal/playwrightruns` | `TestStabilityVerdicts`, `TestRunsStabilityCLI`, `TestReaderAndCompareEdgeCases` | experimental |
| PWR-V0-004 | `internal/playwrightruns` | `TestStabilityVerdicts` | experimental |
| PWR-V0-005 | `cmd/corvint/runs.go` | `TestRunsStabilityCLI` | experimental |
| PWR-V0-006 | `internal/playwrightruns` | `TestCompareVerdicts`, `TestReaderAndCompareEdgeCases` | experimental |
| PWR-V0-007 | `internal/playwrightruns` | `TestCompareVerdicts`, `TestReaderAndCompareEdgeCases` | experimental |
| PWR-V0-008 | `internal/playwrightruns` | `TestCompareVerdicts`, `TestReaderAndCompareEdgeCases`, `TestRunsCompareCLI` | experimental |
| PWR-V0-009 | `internal/playwrightruns` | `TestCompareVerdicts`, `TestReaderAndCompareEdgeCases` | experimental |
| PWR-V0-010 | `cmd/corvint/runs.go` | `TestRunsCompareCLI` | experimental |
| PWR-V0-011 | `internal/playwrightruns` | `TestRepeatEachIsObservations`, `TestRunsRepeatAndShardCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-012 | `internal/playwrightruns` | `TestRepeatCountMismatchAmbiguous` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-013 | `internal/playwrightruns` | `TestRepeatEachIsObservations` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-014 | `internal/playwrightruns` | `TestCompareRepeatsMatchSingleReports`, `TestRunsRepeatAndShardCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-015 | `internal/playwrightruns` | `TestSingleRepeatOutputByteIdentical` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-016 | `internal/playwrightruns` | `TestShardDeclarationValidated`, `TestShardSetRefusals`, `TestCompleteShardSetMatchesWholeRun` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-017 | `internal/playwrightruns` | `TestShardSetRefusals`, `TestRunsRepeatAndShardCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-018 | `internal/playwrightruns` | `TestShardOverlapAmbiguous` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-019 | `internal/playwrightruns` | `TestCompleteShardSetMatchesWholeRun`, `TestShardSetRefusals`, `TestRunsRepeatAndShardCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-020 | `internal/playwrightruns` | `TestCompleteShardSetMatchesWholeRun`, `TestShardSetRefusals`, `TestRunsRepeatAndShardCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-021 | `internal/playwrightruns` | `TestShardedRepeatEachComposes` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-022 | `internal/playwrightruns` | `TestCompleteShardSetMatchesWholeRun` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-023 | `cmd/corvint/runs.go` | `TestRunsRepeatAndShardCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-033 | `internal/playwrightruns` | `TestRunFactsDeclaration`, `TestReusableDecisions` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-034 | `internal/playwrightruns` | `TestReusableDecisions`, `TestRunsReusableCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-035 | `internal/playwrightruns` | `TestReusableDecisions`, `TestRunsReusableCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-036 | `internal/playwrightruns`, `cmd/corvint/runs_reuse.go` | `TestReusableFreshness`, `TestRunsReusableCLI` (including a report from another machine: a foreign `config.rootDir` with `testRootPath` declared resolves `FRESH_PASS` or `STALE`, and without it, or with an escaping one, `FRESHNESS_UNRESOLVED`/`TEST_ROOT_UNRESOLVED`; V1-1611) | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-037 | `internal/playwrightruns` | `TestRedundancy`, `TestRunsRedundancyCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-038 | `internal/evidencereach` | `TestParseConfig`, `TestClassify` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-039 | `internal/evidencereach` | `TestClassify`, `TestReaderCheck`, `TestRunsReusableCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-040 | `cmd/corvint/runs_reuse.go` | `TestRunsReusableCLI`, `TestRunsRedundancyCLI`, `TestReusableSummaryForm`, `TestReaderCheck` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-041 | `internal/playwrightruns/baseline.go`, `cmd/corvint/runs_baseline.go` | `TestPWRV0041_RecordKnownNeedsBothRuns`, `TestPWRV0041_ReadBaselineShape`, `TestPWRV0041_RunsBaselineCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-042 | `internal/playwrightruns/baseline.go` | `TestPWRV0042_DiffExcusesOnlyVerifiedKnown`, `TestPWRV0042_EvidenceReReadUnexcuses`, `TestPWRV0041_RunsBaselineCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-043 | `internal/playwrightruns/baseline.go`, `cmd/corvint/runs_baseline.go` | `TestPWRV0042_DiffExcusesOnlyVerifiedKnown`, `TestPWRV0043_OwnFileAndReportErrorsBlock`, `TestPWRV0041_RunsBaselineCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-044 | `internal/playwrightruns/baseline.go`, `cmd/corvint/runs_baseline.go` | `TestPWRV0042_DiffExcusesOnlyVerifiedKnown`, `TestPWRV0041_RunsBaselineCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-045 | `cmd/corvint/runs_baseline.go` | `TestPWRV0041_RunsBaselineCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |

Repository-wide gate is `NOT_RUN` in this scoped lane.
