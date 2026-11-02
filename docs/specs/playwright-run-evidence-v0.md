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
for PWR-V0-041..045, agent-drafted and accepted on 2026-10-17 under the same standing auto-accept;
V1-2729 / GitHub #1835 (a producer for the identity declaration and one per-run statement for an
undeclared report) for PWR-V0-046..051, agent-drafted and accepted on 2026-10-25 under the same
standing auto-accept; GitHub #1866 (files under a project `testDir` get no blob) for
PWR-V0-052..053, agent-drafted and accepted on 2026-10-26 under the same standing auto-accept;
V1-2774 / GitHub #1873 (a run against a stale build pools as the declared revision) for
PWR-V0-054..057, agent-drafted and accepted on 2026-10-26 under the same standing auto-accept;
V1-2799 / GitHub #1896 (interrupted runs, argument reasons, an incomplete baseline, staleness detail
and a last-N window) for PWR-V0-058..063, agent-drafted and experimental under the standing lane
auto-accept (2026-10-27); V1-2812 / GitHub #1896 asks 4 and 5 (closure-scoped identity and identity
sidecars) for PWR-V0-064..071, agent-drafted and experimental under the standing lane auto-accept
(2026-10-27); V1-2812 (computed per-test closures) for PWR-V0-072..074, agent-drafted and
experimental under the standing lane auto-accept (2026-10-27); AGENTS.md invariants 1, 2, 4, 7 and 8.

## Agent digest
- Claim: A read-only reader of Playwright JSON reports classifies per-test outcome stability across runs and behaviour parity between baseline and candidate runs.
- Status: accepted (decision 0520; V1-1230, V1-1229)/experimental
- Exists: `internal/playwrightruns` and the experimental `corvint runs stability` and `corvint runs compare` commands; each `--repeat-each` repeat is one observation, and shard reports merge into one logical run only as a complete set, else `SHARD_SET_INCOMPLETE`, `SHARD_DUPLICATE` or `SHARD_INPUTS_NOT_DECLARED` (PWR-V0-011..023); `corvint runs reusable` lists each selected test as `REUSABLE` or `MUST_RUN` with a reason from fingerprinted prior passes, `corvint runs redundancy` reports re-run test minutes by category, and `corvint runs fresh` reads evidence freshness over the repository-declared reach in `.corvint/evidence-reach.json` (PWR-V0-033..040); `corvint runs baseline record` and `diff` gate candidate runs on a revision-pinned failure baseline whose known entries were confirmed by an isolated re-run (PWR-V0-041..045); `corvint runs identity --config FILE` prints the `corvintRunIdentity` declaration computed read-only from the test (and optional `--app-root` application) checkout, and `runs stability` names an undeclared report once per run with that remedy instead of one abstention per test (PWR-V0-046..051); a repeatable `--project-test-dir DIR[:REGEXP]` declares blobs for files under a project's own `testDir`, keyed `../` as the report spells them, and an `IDENTITY_INPUT_MISSING` stability entry names its `missingInputs` (PWR-V0-052..053); `--artifact-digest VALUE` records the operator-supplied `servedArtifactDigest`, runs pool, shard and reuse only at one served digest, `runs compare` reports `UNDEPLOYED` when the candidate served the baseline's build (or another build at the same revision), and a run without the digest states `servedDigest` `UNKNOWN` (PWR-V0-054..057); an interrupted observation folds into the test's pooled row as `excluded[]`, `runs-invalid-arguments` names `reason` and `option`, `baseline record` refuses an incomplete full run (`FULL_RUN_INCOMPLETE`) unless `--allow-unobserved` lists the tests as unobserved, a not-ancestor baseline names its merge base and side counts, and `runs stability --last N` windows by start time (PWR-V0-058..063); `--identity REPORT=FILE` and `--identity-sidecars` attach a hash-bound identity sidecar that never overrides an embedded declaration, and `runs stability --pool-by closure` pools on caller-declared per-test closure digests (`runs identity --closure-digests FILE`), refusing a contradicted closure (PWR-V0-064..071); `runs identity --compute-closures` computes per-test closure digests under rule `corvint-test-closure/0` from the static TypeScript graph, failing closed to no digest (PWR-V0-072..074); fetching the digest from a URL is not delivered.
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
  `config.metadata.corvintRunIdentity`, plus `servedArtifactDigest` when declared (PWR-V0-055). Nothing is inferred from paths, timestamps or report
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
  admitted reports show. Otherwise it is `UNDEPLOYED` when PWR-V0-056 applies, else `DIVERGED` with `TESTS_CHANGED`,
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
  `645304a2fb71065f9061d8d8d866600d703e84c6`, apart from the served-digest statements PWR-V0-055..056
  add (`servedDigest` per run and per report fact, `summary.servedDigestUnknownRuns` and the
  `SERVED_DIGEST_UNKNOWN` compare unknown); the goldens were regenerated once for that amendment.
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
  each known entry binds the paths and sha256 of the first full-run and isolated reports that show
  that failure, whatever the report order. Report paths are recorded as given, relative to the
  caller's directory, and read against `--root` at diff time. A test that failed, or was flaky, in the
  full run only is listed under `flaky` with its full-run evidence. Identities a report holds
  ambiguously are left out. A report that declares an `applicationRevision` other than REV refuses
  `runs-baseline-unavailable` (`APPLICATION_REVISION_DIFFERS`); more than 4096 entries refuse
  (`BASELINE_BOUND_EXCEEDED`); an unresolvable REV refuses `runs-revision-unresolved`.
- `PWR-V0-042`: `runs baseline diff --baseline B --runs R...` MUST read B strictly (at most 16 MiB,
  opened without blocking so a FIFO refuses as unreadable,
  unknown members, another profile, a revision that is not a full object id, a known entry without
  isolated evidence or a duplicate identity refuse `runs-baseline-unavailable`). A candidate failure
  that is a known entry is excused (`KNOWN_VERIFIED`) only when both recorded reports re-read at diff
  time with their recorded sha256 and still fail the test unambiguously; otherwise it blocks with
  `EVIDENCE_MISSING`, `EVIDENCE_CHANGED` or `EVIDENCE_PASSING`.
- `PWR-V0-043`: A candidate failure in a `flaky` entry MUST block as `SUSPECTED_FLAKE` and is never
  excused. A known failure whose file is under an `--own` path (resolved through each candidate's
  `testRootPath`) MUST block as `OWN_FILE`; with `--own` given, a known failure in a candidate whose
  test root cannot be located blocks as `TEST_ROOT_UNRESOLVED`. A failure not in the baseline blocks
  as `NOT_IN_BASELINE`, and an identity any candidate report holds ambiguously blocks as
  `IDENTITY_AMBIGUOUS` even when another report fails it unambiguously; and a candidate report with top-level errors blocks
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

### Identity declaration producer (GitHub #1835, V1-2729)

The reader still never infers identity (PWR-V0-002); this separate producer computes the declaration
the adopter embeds in `config.metadata`, so the rules every adopter would otherwise reinvent live in
one place.

- `PWR-V0-046`: `corvint runs identity --config FILE [--test-dir DIR] [--app-root DIR]
  [--test-match REGEXP]` MUST print one `corvint-run-identity/0` document whose `corvintRunIdentity`
  member is exactly the object to embed under `config.metadata.corvintRunIdentity`
  (`applicationRevision`, `configDigest`, `testFileBlobs`, `testRevision`, `testRootPath`), plus the
  repository-relative `config` and `testDir`, the `testMatch` used, `counts` (`testFiles`,
  `digestFiles`, `untrackedOutsideDigest`, `modifiedDigestFiles`, `modifiedTestFiles`) and sorted
  `limits`. Relative paths resolve against `--root`; `--test-dir` is the config's top-level `testDir`
  (which is `config.rootDir`) and defaults to the config's directory, as Playwright's does. A
  project's own `testDir` below it is not passed: its files are found under the top-level directory
  and keyed relative to it, as the reporter keys them; a project `testDir` outside it is passed with
  `--project-test-dir` (PWR-V0-052). Playwright's config is not evaluated, so a non-default top-level
  `testDir` or `testMatch` MUST be passed explicitly.
- `PWR-V0-047`: Test files MUST be the tracked and untracked-not-ignored files under the test
  directory whose test-directory-relative path matches `--test-match` (default
  `(^|/)[^/]*\.(spec|test)\.[cm]?[jt]sx?$`, Playwright's default `testMatch`), excluding any path with a
  `node_modules` segment and any tracked file deleted from the working tree. `testFileBlobs` MUST be
  keyed by that test-directory-relative slash path, which is the JSON reporter's `file` because
  `config.rootDir` is `testDir`, and each value MUST be the Git blob id of the working-tree content
  (the index blob when unmodified; otherwise hashed without writing an object).
- `PWR-V0-048`: `configDigest` MUST be `sha256:` over the line `config\t<config path>\n` (so two
  configs over one tree never share a digest) followed by the sorted lines `mode blob\tpath\n` of every
  tracked file of the test checkout that is not a test file (support code, page objects, fixtures,
  lockfiles and the config itself, plus the config when it is untracked), using working-tree blob ids
  for modified files. A change to any of them changes the digest; a test-file change does not, since
  it is pooled through `testFileBlobs`. Untracked non-test files are outside the digest and MUST be
  counted and named with the limit `UNTRACKED_FILES_OUTSIDE_DIGEST`; ignored files are excluded.
- `PWR-V0-049`: `testRevision` MUST be the test checkout's `HEAD` commit and `testRootPath` the
  test directory's repository-relative path (`.` at the root, PWR-V0-033). `applicationRevision` MUST
  be the `HEAD` commit of `--app-root` (default the test checkout); when that checkout's tracked
  working-tree diff against `HEAD` is non-empty it MUST be suffixed `+worktree.sha256:` plus the
  sha256 of `git diff --binary --full-index HEAD` bytes and carry the limit
  `APPLICATION_WORKTREE_MODIFIED`, so a dirty application never pools with its clean commit.
- `PWR-V0-050`: The producer MUST refuse with `runs-identity-unavailable` and one reason —
  `CONFIG_UNREADABLE`, `NOT_A_REPOSITORY`, `TEST_DIR_OUTSIDE_REPOSITORY`, `TEST_MATCH_INVALID`,
  `TEST_REVISION_UNRESOLVED`, `APPLICATION_REVISION_UNRESOLVED`, `UNMERGED_PATHS`,
  `SUBMODULE_MODIFIED`, `INDEX_FLAGS_SET`, `FILTER_CONFIGURED`, `PATH_UNREPRESENTABLE`,
  `NO_TEST_FILES` or `GIT_READ_FAILED` — rather than print a partial declaration. Both the test
  checkout and a distinct `--app-root` checkout MUST be validated before any content is hashed: an
  unmerged entry, an assume-unchanged or skip-worktree entry (whose working-tree change Git would not
  report, `INDEX_FLAGS_SET`), a submodule with any modification including untracked or dirty inner
  content, and a repository-configured `filter.*.clean` or `filter.*.process` command
  (`FILTER_CONFIGURED`) refuse. It MUST only read: Git runs with a scrubbed environment (no global or
  system config), `GIT_OPTIONAL_LOCKS=0` and `core.fsmonitor=false`, blob ids are computed without
  `-w`, and the index and working tree are byte-unchanged afterwards.
- `PWR-V0-051`: `runs stability` MUST state a report that declares no identity inputs once, on its
  `runs[]` entry, with `unpooledTests` (its test count) and a `remedy` naming `runs identity`, and count
  such reports in `summary.identityNotDeclaredRuns`, instead of emitting one
  `IDENTITY_INPUT_MISSING` entry per test. Those observations still never join a pool, and a test seen
  only in undeclared reports has no `tests[]` entry. Shard reports keep `SHARD_INPUTS_NOT_DECLARED`
  (PWR-V0-014), and a declared report missing one file's blob still abstains per test (PWR-V0-002).
- `PWR-V0-052`: `runs identity` MUST accept a repeatable `--project-test-dir DIR[:REGEXP]` naming a
  project's own `testDir` (relative paths resolve against `--root`; the first colon after an optional
  Windows drive prefix separates `REGEXP`, which defaults to the effective `--test-match`; an empty
  `DIR`, or an empty `REGEXP` after a colon, is `runs-invalid-arguments`). The files under `DIR` whose
  `DIR`-relative slash path matches `REGEXP` (no `node_modules` segment, deleted tracked files
  excluded, untracked-not-ignored included) MUST be test files by the PWR-V0-047 rules: their
  `testFileBlobs` key is their path relative to the top-level test directory, `../` included
  (`../setup/global.setup.ts`), which is how the JSON reporter spells them because
  `config.rootDir` is the top-level `testDir`; they leave `configDigest` (PWR-V0-048) and count in
  `testFiles`. The document MUST list each named directory in `projectTestDirs[]` (`dir`
  repository-relative, `testMatch`, `status` `RESOLVED`, `testFiles`). Because the config is not
  evaluated, a project `testDir` is never discovered or guessed; without the flag its tests keep
  abstaining with the PWR-V0-053 statement.
- `PWR-V0-053`: A `--project-test-dir` that resolves (after symlinks) outside the test checkout MUST
  refuse with `runs-identity-unavailable` / `PROJECT_TEST_DIR_OUTSIDE_REPOSITORY` before anything
  under it is read, and an invalid `REGEXP` with `TEST_MATCH_INVALID`. One that is not an existing
  directory MUST be listed `UNRESOLVED` with reason and limit `PROJECT_TEST_DIR_UNRESOLVED`, and one
  matching no file `UNRESOLVED` with `PROJECT_TEST_DIR_NO_TEST_FILES`; either contributes no blob
  and the rest of the declaration is still printed. A `runs stability` entry with reason
  `IDENTITY_INPUT_MISSING` MUST name `missingInputs`, the sorted union over its observations of
  `applicationRevision`, `configDigest` and `testFileBlobs[<file>]` (with `corvintRunIdentity` for an
  undeclared shard report), so a report file that has no blob, such as an unnamed project
  `testDir`'s `../` file, is visible per test.

### Served artifact digest (GitHub #1873, V1-2774)

A declared `applicationRevision` says which source the run was meant to exercise, not which build the
target actually served. These requirements let the operator state the served build and keep runs of
different builds apart. Corvint fetches nothing: the digest is a value the operator supplies.

- `PWR-V0-054`: `runs identity` MUST accept one `--artifact-digest VALUE` (1 to 256 characters of
  `A-Za-z0-9._:+/=@-`) and record it as `corvintRunIdentity.servedArtifactDigest`; the document
  MUST state `servedDigest` `DECLARED` with it and `UNKNOWN` without it. An empty, invalid or repeated
  value MUST refuse with `runs-invalid-arguments` before anything is read.
- `PWR-V0-055`: `runs stability` MUST pool only runs whose declared `servedArtifactDigest` is equal
  (absent is its own value) and show it in the pool's `inputs`. Each run summary MUST state
  `servedDigest`: `UNKNOWN` without a declaration or digest, `DISAGREES` when another run in the set
  declares a different digest at the same `applicationRevision`, else `DECLARED`; the summary MUST count
  `servedDigestUnknownRuns` and `servedDigestDisagreeingRuns` (omitted when zero).
- `PWR-V0-056`: `runs compare` MUST report verdict `UNDEPLOYED`, after the `INCOMPARABLE` reasons and
  before `DIVERGED`, when every report declares a digest and each side has one (revision, digest)
  pair, with reason `SERVED_DIGEST_UNCHANGED` (revisions differ, digests equal) or
  `SERVED_DIGEST_DIFFERS_AT_REVISION` (revision equal, digests differ), followed by any `TESTS_*`
  reasons. A report without a digest MUST add unknown `SERVED_DIGEST_UNKNOWN` and never yields
  `UNDEPLOYED` or a pass on that basis; each report fact MUST state `servedDigest` `DECLARED` or
  `UNKNOWN`. Differing digests within one side are `INPUTS_DIFFER_WITHIN_SIDE`.
- `PWR-V0-057`: The `runs reusable` fingerprint MUST include `servedArtifactDigest`, so a prior pass at
  another (or no) served build is `MUST_RUN` / `FINGERPRINT_CHANGED` naming that field, and shard
  reports that served different builds MUST NOT group into one logical run. Each `runs reusable`
  selection and prior run summary MUST state `servedDigest` `DECLARED` or `UNKNOWN`, never empty.

### Interrupted runs, argument reasons and baseline completeness (GitHub #1896, V1-2799)

Status: experimental under the standing lane auto-accept (2026-10-27). An interrupted run must not
make a test look like two tests, a refusal must say what was wrong, and a baseline must never treat a
test its full run did not finish as passing.

- `PWR-V0-058`: In `runs stability`, an observation that is `RUN_INTERRUPTED` or `NOT_RUN`, from a run
  that is not a refused shard set and that declares every pool input for the test, MUST be listed in
  `excluded[]` (`run`, `repeat` when repeated, `reason`) of the entry pooled for that test at the same
  inputs, and MUST NOT be counted toward its outcomes or threshold. Without such an entry it MUST keep
  its own `INSUFFICIENT` row with that reason. Each `runs[]` entry MUST state `interrupted: true` when
  any of its results has status `interrupted`; the summary MUST count `interruptedRuns` and
  `excludedObservations` (omitted when zero).
- `PWR-V0-059`: Every `runs-invalid-arguments` refusal MUST carry `reason`, one of
  `SUBCOMMAND_MISSING`, `SUBCOMMAND_UNKNOWN`, `OPTION_UNKNOWN`, `VALUE_MISSING`, `VALUE_INVALID`,
  `ARGUMENT_UNEXPECTED`, `OPTION_REQUIRED`, and `option` when the option is known: a defined option,
  or an undefined one only when it is a plain name of at most 40 characters of `A-Za-z0-9-` that the
  secret screen does not match. A value MUST never be echoed.
- `PWR-V0-060`: `runs baseline record` MUST NOT treat an interrupted occurrence as failure evidence,
  and MUST list as unobserved every unambiguous full-run test that no full-run report observed
  executing without an interrupted result and that is not known or flaky, each with `reason`
  (`RUN_INTERRUPTED`, or `NOT_RUN` when it never executed) and the first full report that lists it.
  With any unobserved test it MUST refuse `runs-baseline-unavailable` with `option` `full`, `reason`
  `FULL_RUN_INCOMPLETE` and the `unobserved` count, unless `--allow-unobserved` is given, when the
  baseline carries them under `unobserved[]`. An unobserved test is never recorded as passing.
  `ReadBaseline` MUST refuse an `unobserved` entry without file, title path, valid evidence or one of
  those reasons, or duplicating another entry; a binary without this member refuses the document.
- `PWR-V0-061`: `runs baseline diff` MUST list the baseline's unobserved tests in
  `unobservedAtBaseline` with `counts.unobservedAtBaseline`, and a failing candidate test that the
  baseline lists as unobserved MUST be new with reason `UNOBSERVED_AT_BASELINE` (blocking).
- `PWR-V0-062`: A `STALE` staleness with `BASELINE_NOT_ANCESTOR` MUST add `mergeBase`,
  `baselineSideCommits` (merge base to baseline revision) and `branchSideCommits` (merge base to branch
  point) when Git names a merge base; each count is absent when Git cannot count it.
- `PWR-V0-063`: `runs stability --last N` (1 to 64, once) MUST pool only the last `N` logical runs
  ordered by `stats.startTime`, ties by argument position, state `last`, and mark every report of the
  other logical runs `outsideWindow: true`. A report without a start time MUST refuse
  `runs-report-unavailable` with `option` `last`, `reason` `START_TIME_MISSING` and its `run`.

### Closure-scoped identity and identity sidecars (GitHub #1896, V1-2812)

Status: experimental under the standing lane auto-accept (2026-10-27). A report produced before its
identity was declared can be given one without editing it, and a suite whose configuration or
unrelated files change often can pool on what each test can actually load, when the caller vouches
for it.

- `PWR-V0-064`: An identity sidecar is one JSON object, read only from a regular file without blocking
  and at most 8 MiB, with exactly the members `profile` (`corvint-run-identity-sidecar/0`),
  `reportSha256` (64 lowercase hex digits) and `corvintRunIdentity` (an object read by the rules of an
  embedded declaration, PWR-V0-002, PWR-V0-033, PWR-V0-055 and PWR-V0-068). It binds to exactly the
  report whose raw bytes have that SHA-256. A sidecar that cannot be read refuses
  `IDENTITY_SIDECAR_UNREADABLE`, one over the bound `IDENTITY_SIDECAR_BOUND_EXCEEDED`, a malformed one
  (another profile, an unknown or missing member, trailing data) `IDENTITY_SIDECAR_INVALID`, and one
  bound to other bytes `IDENTITY_SIDECAR_MISMATCH`.
- `PWR-V0-065`: `runs stability`, `compare`, `reusable`, `redundancy`, `baseline record` and
  `baseline diff` MUST accept a repeatable `--identity REPORT=FILE`, where `REPORT` is resolved as the
  command's report options resolve it and MUST name one of the command's reports exactly once, else
  `runs-invalid-arguments` `VALUE_INVALID` with `option` `identity`; and `--identity-sidecars`, which
  reads `REPORT.identity.json` beside every report without an explicit `--identity`. Under
  `--identity-sidecars` an absent file leaves that report as it is; any other read failure refuses. A
  sidecar refusal is `runs-report-unavailable` with the report's `option`, its 1-based `run` and the
  PWR-V0-064 or PWR-V0-066 `reason`. `baseline diff` does not consult sidecars when it re-reads
  recorded evidence, whose identity it does not use.
- `PWR-V0-066`: A sidecar MUST NOT override an embedded declaration. When the report's
  `config.metadata` carries a `corvintRunIdentity` member, a sidecar whose declaration differs from the
  embedded one in any read input (the PWR-V0-002 inputs with their conflicting spellings, the
  PWR-V0-068 closure, `testRevision` or `testRootPath`) refuses `IDENTITY_SIDECAR_CONFLICT`; an equal
  one keeps the report `EMBEDDED`. Otherwise the sidecar's declaration is the report's, and its
  `corvintRunCategory` stays the report's own.
- `PWR-V0-067`: When `--identity` or `--identity-sidecars` is given, each run statement MUST carry
  `identitySource` (`EMBEDDED` when the report carries the member, `SIDECAR`, or `NOT_DECLARED`):
  `runs[]` of stability and redundancy, `reports` of compare, `selection` and `priors` of reusable,
  the recorded evidence of `baseline record` and `runs[]` of `baseline diff`. A shard set with any
  `SIDECAR` member is `SIDECAR`. Without either option every output is unchanged. `ReadBaseline` MUST
  refuse an evidence `identitySource` outside those three values.
- `PWR-V0-068`: `corvintRunIdentity` MAY declare `testClosureRule` and `testClosureDigests`
  (`file` to `sha256:` and 64 lowercase hex digits), keyed like `testFileBlobs`. The admitted rules
  are `DECLARED` and the computed `corvint-test-closure/0` (PWR-V0-072). Under `DECLARED` the caller asserts that each digest changes whenever any file its test can load or
  execute changes, including the test file itself; Corvint does not verify it. Another rule, a value of
  another form and a file whose spellings declare different digests are dropped, so the tests abstain;
  a malformed closure member MUST NOT hide the other inputs. Closure members join the shard-set key.
- `PWR-V0-069`: `runs stability --pool-by inputs|closure` (once; `inputs` is the default and its
  output is unchanged) with `closure` MUST pool on `playwrightVersion`, `applicationRevision`, the
  declared `servedArtifactDigest`, `testClosureRule` and `testClosureDigest` instead of `configDigest`
  and `testFileBlob`, which are then absent from `inputs`. A missing one abstains as
  `IDENTITY_INPUT_MISSING` naming `applicationRevision`, `testClosureRule` or
  `testClosureDigests[<file>]` in `missingInputs`; there is no fallback to the PWR-V0-002 key. The
  report states `poolBy` `closure`, each `runs[]` entry `closureSource` (`DECLARED`, `COMPUTED` or
  `NOT_DECLARED`, PWR-V0-074), and the summary `closureNotDeclaredRuns` when non-zero.
- `PWR-V0-070`: With `--pool-by closure`, two reports that declare the same rule and closure digest
  for one file but different non-empty `testFileBlobs` for it MUST refuse `runs-report-unavailable`
  with `option` `pool-by`, `reason` `CLOSURE_DIGEST_CONTRADICTED`, the later `run` and `conflictsWith`
  the earlier: a closure contains its test file, so one claim is false.
- `PWR-V0-071`: `runs identity --closure-digests FILE` MUST read a regular JSON object of at most
  8 MiB mapping declared test keys to `sha256:` digests and print them as `testClosureDigests` with
  `testClosureRule` `DECLARED`. A key that is not a `testFileBlobs` key of the declaration, a duplicate
  key, a value of another form, or an unreadable, oversized or malformed file refuses
  `runs-identity-unavailable` with `reason` `CLOSURE_DECLARATION_INVALID`. Tests the file does not
  cover keep no closure digest.
- `PWR-V0-072`: (experimental under the standing lane auto-accept (2026-10-27); V1-2812)
  `runs identity --compute-closures` MUST set `testClosureRule` `corvint-test-closure/0` and, for each
  declared test the closure of `TJAA-V0-140` covers, a `testClosureDigests` entry: SHA-256 over
  `rule\tcorvint-test-closure/0\nconfig\t<config path>\n` and the sorted `configDigest` and
  test-file lines of the same checkout, less the test's `Excludable` files. A file the checkout
  modified relative to `HEAD` is never left out. The document states `closure` `{"source":"COMPUTED"}`.
  With `--closure-digests` the declared digests win, the rule is `DECLARED` and `closure.source` is
  `DECLARED`. Without the option the output is unchanged and has no `closure` member.
- `PWR-V0-073`: (same acceptance) A test whose closure is not computed MUST get no digest, so it
  abstains under `--pool-by closure`, and is named in `closure.unknown` with the reason and the limit
  `TEST_CLOSURE_UNKNOWN`: the config is not statically resolved, no project admits the test, the
  closure is `TJAA-V0-141` unknown (for example a dynamic import), or a closure file has no line in
  the digest (untracked or ignored). When a frontier outside the suite widens a test's closure
  (`TJAA-V0-141`), its digest leaves no file out, `closure.widened` names the cause and the limit
  `TEST_CLOSURE_EXCLUSION_WIDENED` is stated. Every failure keeps or widens the digest input; none
  narrows it.
- `PWR-V0-074`: (same acceptance) The reader MUST admit rule `corvint-test-closure/0`, from an
  embedded declaration or a sidecar, and `runs stability --pool-by closure` MUST state such a run's
  `closureSource` `COMPUTED`. The rule is part of the pool key, so a `COMPUTED` and a `DECLARED`
  digest never pool together. Stability never computes a closure at read time; the digest comes
  from the producer. Any other rule is still dropped.
- `PWR-V0-174`: (experimental under the standing lane auto-accept (2026-10-28); V1-2912; GitHub
  #1918) Amends `PWR-V0-058` for `--pool-by closure`. An interrupted or not-run observation of a test
  that has no declared inputs at all (no closure digest) MUST be listed in `excluded[]` of that
  test's `IDENTITY_INPUT_MISSING` row when it has one, so every pool mode keeps one row per test
  identity and the summary counts distinct tests. Without such a row it keeps its own `INSUFFICIENT`
  row.
- `PWR-V0-175`: (same acceptance) `runs stability --last N` MUST state the window rule applied in
  `window`: `all` (default, the `PWR-V0-063` window, which counts an interrupted logical run toward N)
  or `completed` (`--window completed`, which requires `--last` and skips interrupted logical runs,
  marking them `outsideWindow`). Under `all`, when the window truncated, `windowInterruptedRuns`
  lists the reports of interrupted runs that took a slot.
- `PWR-V0-176`: (same acceptance) Amends `PWR-V0-059`. An unknown argument that starts with a defined
  option name followed by whitespace MUST refuse `OPTION_UNKNOWN` with `option` set to that name and
  `hint` `OPTION_VALUE_JOINED` (option and value passed as one argument). The value is never echoed.
- `PWR-V0-177`: (same acceptance; V1-2914; GitHub #1920) Amends `PWR-V0-073`. A Playwright config
  unknown of reason `playwright:browser-identity-unresolved` (a dynamic `use.launchOptions`,
  `browserName` or device) decides which browser runs, not which source files a test loads, so it
  MUST NOT leave every computed test closure unknown. Every other config unknown still does, except
  the widening unknowns of `PWR-V0-184` and `PWR-V0-185`. The escaped regexp string inside the
  config's own identity call is reproduced and resolved by `PWR-V0-183`: the escape refused the
  `process` root, so a `process.env` read in a non-identity `use` value (`baseURL`) became a
  `playwright:use-value-unverified` unknown of every project that left every closure unknown.
- `PWR-V0-178`: (same acceptance) `closure` MUST list in `sharedCauses` each reason that leaves two or
  more tests without a digest, once, with its `tests` count and the reason's own location, in
  addition to the per-test `unknown` map.
  Evidence: `TestStabilityClosureInterruptedAndCompletedWindow`, `TestPWRV0059_RunsInvalidArgumentsReason`,
  `TestPWRV0176_RunsJoinedOptionValue`, `TestPlaywrightTestClosuresNonIdentityUse_PWRV0177`,
  `TestSharedClosureCauses_PWRV0178`. Rollback: revert; the three #1918 gaps and the whole-suite
  unknown return.
- `PWR-V0-183`: (experimental under the standing lane auto-accept (2026-10-28); V1-2914; GitHub
  #1920) Amends `PWR-V0-073` with `TJAA-V0-147`. A `\` escape inside a quoted string that is one
  whole element of an array literal opening with the plain literals `'runs', 'identity'` (the argv
  of the config's own `corvint runs identity` call, such as a `--project-test-dir` regexp) MUST NOT
  refuse the `process` root, so the config's `process.env` reads stay side-effect free and no test
  closure is unknown or widened for that escape. An escape anywhere else keeps refusing the root.
- `PWR-V0-184`: (same acceptance) Amends `PWR-V0-073` and `TJAA-V0-141`. A `playwright:use-value-unverified`
  unknown (a `use` value the static selection cannot prove side-effect free) runs config code but
  cannot change which test files a project admits or depends on, so it MUST NOT leave every closure
  unknown. The tests of the project that holds it, and of every project that reaches it through
  `dependencies` or `teardown`, MUST keep a known closure with an empty `Excludable` and `Widened`
  naming the unknown's detail and location (the identity output's `closure.widened`, and every
  such test keeps a digest over the whole suite); the tests of every other project keep their
  excludable files.
- `PWR-V0-185`: (same acceptance) As `PWR-V0-184` for a project whose `metadata` is not a static
  literal (`<project> metadata is dynamic`). Every other `playwright:config-syntax-unresolved`
  unknown still leaves every closure unknown.
- `PWR-V0-186`: (same acceptance) The reporter's configuration (a dynamic `use.launchOptions`,
  `process.env` reads in the global `use`, an escaped regexp string in its own identity call and a
  setup project selected through `--project-test-dir`) MUST digest every test with no `closure.unknown`,
  `closure.sharedCauses` or `closure.widened`. When tests widen for different reasons,
  `closure.widened` MUST be the first reason in byte order, independent of iteration order.
  Evidence: `TestPlaywrightTestClosuresIdentityArgvEscape_PWRV0183`,
  `TestPlaywrightTestClosuresUnverifiedUseWidensProject_PWRV0184`,
  `TestPlaywrightTestClosuresDynamicProjectMetadataWidens_PWRV0185` in
  `internal/liveverify/affected/typescript/playwright_test_closure_gh1920_test.go`;
  `TestPWRV0186_IssueConfigDigestsEveryTest`, `TestPWRV0186_WidenedReasonDeterministic` in
  `internal/runidentity/computed_closure_test.go`. Rollback: revert; the whole suite is unknown
  again on one unverified `use` value, dynamic project metadata or an escaped identity argv string.

## Boundaries and failure modes

Non-goals: running Playwright, fetching reports, reading HTML/blob/JUnit reporters, directory or glob
input, receipts as identity input, inferring identity from Git inside the readers (the separate
`runs identity` producer computes a declaration the adopter chooses to embed), feeding obligation outcomes from
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
beyond the reports the caller names, computing fingerprints from the working tree inside the reuse
reads (only `runs identity` does, PWR-V0-046..050), per-test import
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

Identity producer non-goals (PWR-V0-046..051): evaluating `playwright.config.*`, writing the
declaration into the config, per-test import graphs (every non-test tracked file is in one digest,
which over-splits pools rather than merging different code), hashing untracked application files or
nested submodule content, and verifying that a report was produced from the declared checkout.
Failure modes: an adopter that passes the wrong `--test-dir` or `--test-match` gets keys that do not
match the report's `file` and every test abstains with `IDENTITY_INPUT_MISSING`, the fail-closed
direction; a project `testDir` the adopter does not name, or names with a narrower `REGEXP` than the
project's `testMatch`, leaves those files unpooled with `missingInputs` naming each file
(PWR-V0-052..053), and reading project `testDir`s from a resolved config or a `--list` report is a
non-goal of this slice; untracked support files change behaviour without changing the digest, which the limit
names; a modified submodule refuses rather than hashing its pointer.
Served digest (PWR-V0-054..057): a `--served-digest-url` fetch is a non-goal of this slice and remains
open, because the core binary's network boundary (invariant 7, guarded by
`cmd/corvint/core_network_boundary_test.go`, V1-1670) admits no HTTP client reachable from a core
command; the operator obtains the digest and passes `--artifact-digest`. The digest is not verified
against the target, so a wrong value is taken as declared; a missing digest is uncertainty
(`UNKNOWN` / `SERVED_DIGEST_UNKNOWN`), never a pass. `runs baseline diff` does not yet compare served
digests.
Interrupted runs and baseline completeness (PWR-V0-058..063) non-goals: detecting a full run made
incomplete by top-level report errors (for example a missing spec file) is not covered by
`FULL_RUN_INCOMPLETE`, which reads only listed tests. Failure modes: an interrupted observation at
inputs where the test never otherwise pooled stays its own `INSUFFICIENT` row; `--allow-unobserved`
makes a baseline that excuses nothing for the unobserved tests, so their candidate failures block;
`--last` trusts each report's declared start time.
Closure identity and sidecars (PWR-V0-064..071) non-goals: computing closures (PWR-V0-072), verifying a declared
closure, signing sidecars, discovering sidecars other than `REPORT.identity.json`, and pooling
`reusable`, `redundancy` or `compare` by closure. Trust: a `DECLARED` closure is the caller's claim,
exactly as the PWR-V0-002 inputs are; Corvint refuses only what it can disprove from the reports
(`CLOSURE_DIGEST_CONTRADICTED`), and a closure that omits a loaded file merges runs that differ, the
named limit. A sidecar is bound to the report's bytes, so a rewritten report refuses
`IDENTITY_SIDECAR_MISMATCH`, but nothing proves the sidecar describes the checkout that ran.
Computed closures (PWR-V0-072..074) start from the `configDigest` file set (PWR-V0-047) and leave
out only TypeScript files the per-test walk of `TJAA-V0-140` proves the test cannot load: files that
only other tests' imports reach. They do not use plain `affected` selection, which marks every
TS-owned path `UNKNOWN` in a Playwright repository. Named limits: runtime loading (`fs` reads,
fixtures fetched by URL), module state one worker shares across tests, and code reached only through
an unmodelled path are outside the graph; a computed closure is Corvint's static claim, not an
observation of what the run loaded. Non-goals: computing a closure in `runs stability`, verifying a
`DECLARED` closure against the computed one, and closures for configs outside the static subset.

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
The identity producer (PWR-V0-046..051) rolls back by removing the `identity` dispatch case and help,
`internal/runidentity`, and the `unpooledTests`, `remedy` and `identityNotDeclaredRuns` fields with
their branch in `Stability`; declarations already embedded in configs stay valid input to the readers.
Project test directories (PWR-V0-052..053) roll back by removing the `--project-test-dir` flag and
`splitProjectTestDir`, `ProjectTestDirs` and `projectScopes`/`reportKey` in `internal/runidentity`, and
the `missingInputs` field; declarations with `../` keys stay valid reader input.
The served digest (PWR-V0-054..057) rolls back by removing the `--artifact-digest` flag, the
`servedArtifactDigest` declaration and input fields, the `servedDigest` statements and counters, the
`UNDEPLOYED` verdict branch and the fingerprint and shard-key field, and restoring the PWR-V0-015
goldens; declarations that carry the field are then ignored as unknown metadata.
PWR-V0-058..063 roll back by removing the `excluded`, `interrupted`, `outsideWindow` and `last` fields
with `StabilityLast`, the refusal `reason`/`option` fields, `--allow-unobserved` with the `unobserved`
baseline member and `UNOBSERVED_AT_BASELINE`, and the three staleness fields; a retained baseline with
`unobserved` then refuses as malformed, the fail-closed direction.
PWR-V0-064..071 roll back by removing `--identity`, `--identity-sidecars`, `--pool-by` and
`--closure-digests` with their help, `sidecar.go`, the closure reader fields and the `identitySource`,
`closureSource`, `poolBy` and `closureNotDeclaredRuns` fields; a retained baseline with evidence
`identitySource` then refuses as malformed, and declarations with closure members ignore them.
PWR-V0-072..074 roll back by removing `--compute-closures` with its help, the `closure` document
member, the two limits and `ClosureRuleComputed` from the reader; declarations carrying
`corvint-test-closure/0` are then dropped again, so their tests abstain.
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
`STALE` past `--max-age 2` and `NOT_OBSERVED` without `--base`. Live (lane cmd-w26, 2026-11-01,
V1-1624): real Playwright 1.63.0 json-reporter full and isolated runs of a three-test suite
recorded one known and one suspected-flake entry; candidate runs then gave `PASS` with the known
failure `KNOWN_VERIFIED` and the flake `fixed`, and exit 1 `BLOCKED` for a returning flake
(`SUSPECTED_FLAKE`), a new failure (`NOT_IN_BASELINE`, staleness `FRESH` at 1 commit), `--own` on
the known file (`OWN_FILE`) and a re-run isolated report (`EVIDENCE_CHANGED`); the commands and
digests are in `docs/build-log/2026-11-01-cmd-w26.md`. A real suite larger than three tests stays
`NOT_RUN`. For PWR-V0-058..063 (synthetic reports and temporary Git
repositories): an interrupted observation folds into the pooled row as excluded; an interrupted full
run refuses `FULL_RUN_INCOMPLETE`, records the test as unobserved with `--allow-unobserved`, and a
candidate failure of it blocks as `UNOBSERVED_AT_BASELINE`; a side-branch baseline names the fork
commit and 2/1 side commits; `--last 2` of three timed runs drops the earliest; each argument refusal
names its reason and option without echoing values.

Identity producer (temporary Git repositories): keys and blobs equal `git ls-files -s` for clean specs
under a nested `testDir` and the config-directory default; a modified spec and an untracked spec are
hashed from the working tree without moving `configDigest`; a deleted spec is dropped; a change to a
page object, lockfile, config or helper moves the digest and stat-only changes do not; an untracked
support file is a named limit; a separate `--app-root` gives its own `HEAD` and a dirty one the
`+worktree.sha256:` suffix that changes with the diff; each refusal is named; the index bytes,
mtime and status are unchanged after a read. End to end, the printed declaration embedded in three
passing reports pools to `DETERMINISTIC`, and two undeclared reports in a stability set are named once
each with `unpooledTests` and the remedy. Running real Playwright with an embedded declaration is
`NOT_RUN`.

Closure identity and sidecars (synthetic reports and temporary Git repositories): a sidecar bound to a
report without a declaration makes it `SIDECAR` and pools it with an embedded equal; a sidecar bound
to other bytes, of another profile, with an unknown member, trailing data, a non-object declaration or
over 8 MiB refuses with its reason naming the report option and position; an equal sidecar keeps
`EMBEDDED`, and a differing one (revision, config, blob, closure, `testRevision`, or a malformed
embedded member) refuses `IDENTITY_SIDECAR_CONFLICT` without replacing the embedded inputs; an
`--identity` naming no command report, without `=`, or twice for one report is `VALUE_INVALID`;
compare, redundancy, reusable and baseline record and diff state `identitySource`, a shard set with a
sidecar member is `SIDECAR`, a sidecar's `applicationRevision` is checked against `baseline record
--rev`, and `ReadBaseline` refuses an unknown `identitySource`. Without the options the stability
output is byte-identical (the PWR-V0-011 goldens and an explicit comparison). Two reports at different
`configDigest` but one declared closure pool `DETERMINISTIC` under `--pool-by closure` and split under
`inputs`; a report without a closure is `closureSource` `NOT_DECLARED` and its test abstains naming
`testClosureDigests[cart.spec.ts]` and `testClosureRule`; a closure digest over two test-file blobs
refuses `CLOSURE_DIGEST_CONTRADICTED`; a reserved or non-string rule, a malformed digest, a non-object
member and conflicting spellings are dropped while the other inputs remain. `runs identity
--closure-digests` prints the declared closure without changing the other inputs and refuses an
undeclared key, a duplicate key, a malformed value, an unreadable path and a directory
(`CLOSURE_DECLARATION_INVALID`). `runs identity --compute-closures` over a committed fixture gives
two different digests; a committed edit to a helper only the other test imports leaves the first
test's digest unchanged, while the same edit uncommitted, an edit to its own helper, to a helper
application code shares, or to a non-TypeScript file changes it; a declared closure wins as
`DECLARED`. A dynamic import, an untracked closure file and an unresolved config leave the affected
tests without a digest under `TEST_CLOSURE_UNKNOWN`, and a frontier outside the suite widens under
`TEST_CLOSURE_EXCLUSION_WIDENED`. Stability pools two computed runs as `COMPUTED` and keeps a computed
and a declared run apart. A real Playwright run with a sidecar or a computed closure is `NOT_RUN`.

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
| PWR-V0-041 | `internal/playwrightruns/baseline.go`, `cmd/corvint/runs_baseline.go` | `TestPWRV0041_RecordKnownNeedsBothRuns`, `TestPWRV0041_ReadBaselineShape`, `TestPWRV0041_KnownBindsFailingFullReport`, `TestPWRV0041_RunsBaselineCLI`, `TestPWRV0042_RunsBaselineRelativeRoot` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-042 | `internal/playwrightruns/baseline.go` | `TestPWRV0042_DiffExcusesOnlyVerifiedKnown`, `TestPWRV0042_EvidenceReReadUnexcuses`, `TestPWRV0041_ReadBaselineRefusesFIFO`, `TestPWRV0041_RunsBaselineCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-043 | `internal/playwrightruns/baseline.go`, `cmd/corvint/runs_baseline.go` | `TestPWRV0042_DiffExcusesOnlyVerifiedKnown`, `TestPWRV0043_OwnFileAndReportErrorsBlock`, `TestPWRV0043_AmbiguousFailureNeverExcused`, `TestPWRV0043_RunsBaselineOwnFailsClosed`, `TestPWRV0041_RunsBaselineCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-044 | `internal/playwrightruns/baseline.go`, `cmd/corvint/runs_baseline.go` | `TestPWRV0042_DiffExcusesOnlyVerifiedKnown`, `TestPWRV0041_RunsBaselineCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-045 | `cmd/corvint/runs_baseline.go` | `TestPWRV0041_RunsBaselineCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-046 | `internal/runidentity`, `cmd/corvint/runs.go` | `TestPWRV0047_TestFileBlobsKeyedAsReportFile`, `TestRunsIdentityCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-047 | `internal/runidentity` | `TestPWRV0047_TestFileBlobsKeyedAsReportFile`, `TestPWRV0047_WorkingTreeBlobsAndUntrackedSpecs` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-048 | `internal/runidentity` | `TestPWRV0048_ConfigDigestCoversTrackedSupportCode`, `TestPWRV0048_ConfigBoundIntoDigest` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-049 | `internal/runidentity` | `TestPWRV0049_ApplicationRevisionFromNamedCheckout`, `TestPWRV0047_TestFileBlobsKeyedAsReportFile` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-050 | `internal/runidentity`, `cmd/corvint/runs.go` | `TestPWRV0050_RefusalsAndReadOnly`, `TestPWRV0050_CheckoutValidation`, `TestRunsIdentityCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-051 | `internal/playwrightruns/stability.go` | `TestStabilityVerdicts`, `TestRunsIdentityCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-052 | `internal/runidentity`, `cmd/corvint/runs.go` | `TestPWRV0052_ProjectTestDirKeyedAsReportFile`, `TestRunsIdentitySetupProjectPools`, `TestSplitProjectTestDir` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-053 | `internal/runidentity`, `internal/playwrightruns/stability.go` | `TestPWRV0053_ProjectTestDirUnresolvedOrOutside`, `TestRunsIdentitySetupProjectPools` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |

Repository-wide gate is `NOT_RUN` in this scoped lane.
| PWR-V0-054 | `internal/runidentity`, `cmd/corvint/runs.go` | `TestRunsIdentityServedDigest` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-055 | `internal/playwrightruns/stability.go`, `reader.go` | `TestServedDigestStability`, `TestSingleRepeatOutputByteIdentical` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-056 | `internal/playwrightruns/compare.go` | `TestServedDigestCompare`, `TestRunsIdentityServedDigest`, `TestCompareVerdicts` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-057 | `internal/playwrightruns/reuse.go`, `shards.go` | `TestServedDigestReuseAndShards`, `TestRunsReusableCLI` | accepted under the owner's standing auto-accept (2026-10-11)/experimental |
| PWR-V0-058 | `internal/playwrightruns/stability.go` | `TestStabilityInterruptedAndWindow` | experimental under the standing lane auto-accept |
| PWR-V0-059 | `cmd/corvint/runs.go`, `runs_baseline.go`, `runs_reuse.go`, `runs_response_wait.go`, `runs_server_errors.go` | `TestPWRV0059_RunsInvalidArgumentsReason`, `TestRunsStabilityCLI`, `TestRunsCompareCLI` | experimental under the standing lane auto-accept |
| PWR-V0-060 | `internal/playwrightruns/baseline.go`, `cmd/corvint/runs_baseline.go` | `TestPWRV0060_RecordUnobserved`, `TestPWRV0060_RunsBaselineUnobserved` | experimental under the standing lane auto-accept |
| PWR-V0-061 | `internal/playwrightruns/baseline.go` | `TestPWRV0060_RecordUnobserved`, `TestPWRV0060_RunsBaselineUnobserved` | experimental under the standing lane auto-accept |
| PWR-V0-062 | `cmd/corvint/runs_baseline.go` | `TestPWRV0062_RunsBaselineNotAncestorSides` | experimental under the standing lane auto-accept |
| PWR-V0-063 | `internal/playwrightruns/stability.go`, `cmd/corvint/runs.go` | `TestStabilityInterruptedAndWindow`, `TestPWRV0063_RunsStabilityLast` | experimental under the standing lane auto-accept |
| PWR-V0-064 | `internal/playwrightruns/sidecar.go` | `TestPWRV0064_SidecarBindsOneReport`, `TestPWRV0065_RunsIdentitySidecars` | experimental under the standing lane auto-accept |
| PWR-V0-065 | `cmd/corvint/runs_identity_sidecar.go`, `runs.go`, `runs_reuse.go`, `runs_baseline.go` | `TestPWRV0065_RunsIdentitySidecars`, `TestPWRV0067_RunsBaselineIdentitySource` | experimental under the standing lane auto-accept |
| PWR-V0-066 | `internal/playwrightruns/sidecar.go`, `reader.go` | `TestPWRV0066_SidecarNeverOverridesEmbedded`, `TestPWRV0065_RunsIdentitySidecars` | experimental under the standing lane auto-accept |
| PWR-V0-067 | `internal/playwrightruns/sidecar.go`, `stability.go`, `compare.go`, `redundancy.go`, `reuse.go`, `baseline.go`, `shards.go` | `TestPWRV0067_IdentitySource`, `TestPWRV0067_RunsBaselineIdentitySource`, `TestSingleRepeatOutputByteIdentical` | experimental under the standing lane auto-accept |
| PWR-V0-068 | `internal/playwrightruns/reader.go`, `shards.go` | `TestPWRV0068_ClosureDeclaration` | experimental under the standing lane auto-accept |
| PWR-V0-069 | `internal/playwrightruns/stability.go`, `cmd/corvint/runs.go` | `TestPWRV0069_PoolByClosure`, `TestPWRV0069_RunsStabilityPoolByClosure` | experimental under the standing lane auto-accept |
| PWR-V0-070 | `internal/playwrightruns/stability.go`, `cmd/corvint/runs.go` | `TestPWRV0070_ClosureContradiction`, `TestPWRV0069_RunsStabilityPoolByClosure` | experimental under the standing lane auto-accept |
| PWR-V0-071 | `internal/runidentity/identity.go`, `cmd/corvint/runs.go` | `TestPWRV0071_ClosureDigests`, `TestPWRV0071_RunsIdentityClosureDigests` | experimental under the standing lane auto-accept |
| PWR-V0-072 | `internal/runidentity/identity.go`, `cmd/corvint/runs.go`, `internal/liveverify/affected/typescript/playwright_test_closure.go` | `TestPWRV0072_ComputedClosureDigests`, `TestPWRV0072_RunsIdentityComputeClosures` | experimental under the standing lane auto-accept; real Playwright run `NOT_RUN` |
| PWR-V0-073 | `internal/runidentity/identity.go` | `TestPWRV0073_ComputedClosureFailsClosed` | experimental under the standing lane auto-accept |
| PWR-V0-074 | `internal/playwrightruns/reader.go`, `stability.go` | `TestPWRV0074_PoolByComputedClosure`, `TestPWRV0068_ClosureDeclaration` | experimental under the standing lane auto-accept |
