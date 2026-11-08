# Go line selection: merge of main and enablement for 1.0

Tickets: V1-4364, V1-4365, V1-4366. Lane base `6f12f4704235dc8f9bea19fc12995bfb68c746c3`;
sealed round-f head `2d53be618a7aea9cd59ccdd983bd62563439592f`; merged main commit
`b75053a5b23cb0a4737a593d1fc1912aa3b79436` (683 commits ahead of the lane base; `origin/main` had
moved on to `4052b8540b5fff818a819db37b55371ea5428c2c` by fetch time, so the commit the round named
is the one merged). Observed 2026-11-06 on the WSL lane host (go1.27.3 linux/amd64).

## Merge: conflicted paths and how each was resolved

Main had meanwhile landed its own `internal/linecov` work (LPCV-V0-7600..7607: in-order test
windows, the child-process audit overlay, language collectors, deadlines, `linecov watch`) while
this lane carried the GLS-V0 collector parts, process-group isolation, the `/4` compact format and
the selector. The merge conflicted in 17 paths:

| Path | Resolution |
|---|---|
| `internal/linecov/collect.go` | Both: the lane's per-package `runPackage` (at most four binaries, process groups, `incomplete-process-tree`, `-tests`/`-inorder` parts, profile compaction) now builds under main's window and collector overlays with the pinned `PWD`, runs the in-order suite with `CORVINT_LINECOV_WINDOWS`, audits children through `CORVINT_LINECOV_AUDIT` (`uninstrumented-child`, `missing-child-counters`) and attributes windows per package. Overlay and window scratch is removed with the package, so scratch stays bounded. |
| `internal/linecov/index.go` | Both: `Merge` compares the lane's in-order part lists and main's deadline, window-fallback and language identity fields. |
| `internal/linecov/select.go` | Lane: the conflict was a mis-aligned hunk of main's `FileByPath` refactor against the lane's two-sided `AnalyzePaths`; main's lookups live in `lookup.go` and are used by `selectLines`. |
| `tools/linecov/main.go` | Main's `help` on a usage error; main's copy of the old single-file `qualify` dropped, since the lane's `qualify.go` supersedes it. |
| `cmd/corvint/affected.go`, `test.go`, `help.go` | Both flag sets (`--line-index`, `--line-test-timeout` with main's `--format json`, `--full`, `--fail-fast-order`, `--ownership-audit`). |
| `docs/AUTOMATION.md`, `docs/CAPABILITIES.md`, `docs/SPEC-TOOLCHAIN-INTEGRATION.md`, `docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md`, `docs/specs/core-compatibility-freeze-v1.md` | Main's side, then `script/check-line-citations.sh --repin` (12 moved citations). |
| `docs/specs/INDEX.json` | Main's side, then `make spec-requirements` re-added the GLS row. |
| `conformance/use-cases-v0/ledger.json`, `receipts/UC-CHANGE-CONSEQUENCE/implementation.json` | Main's side, then `script/repin-use-case-receipts.sh`. |
| `internal/capcatalog/catalog.json`, `trees/corvint.json` | Main's side, then `make capability-catalog` (also regenerates `tools/linecov/command-tree.json`). |

Repairs the merge needed beyond conflict text, each with the test that showed it:

- The `/4` header extension now carries main's index fields (test and suite deadlines, window
  fallbacks, language, command, executable, runner family, config); the gob extension is
  field-tolerant, so `/4` indexes written before this read unchanged. Without it the language
  collectors' round trip lost `Language` (`TestLanguageCommands`).
- `WorkingDiff` (watch) reads the working copy as the new side of a Go file: the lane's analysis
  requires both sides (`GLS-V0-012`) and reported `missing new-side evidence`
  (`TestWatchWorkingTree`). The same test's hand-built index gets an in-order record, since a
  package with records and no in-order record is unordered (`GLS-V0-008`).
- `Record.Union`: the Go collector marks every in-order record as owning every block it executed,
  so in-order reach keeps the package whole as `GLS-V0-008` and `GLS-V0-013` require; a language
  collector's in-order record keeps main's residual rule (it owns its blocks only when it executed
  one no test record did). Decision: main's windows attribute in-order reach to the test that ran it
  and its predecessors, but a test that reaches a block alone and in order can still depend in
  order on state an earlier test left, which no window records; the owner's rule is never to trade
  a miss for precision, so the windows widen test records and never narrow the in-order record.
  `TestInOrderWindows` now asserts the attribution (owners of the changed line) and the whole
  package.
- The window template checks `WriteCounters` before opening a window: a binary whose instrumented
  code no test references links no counters, and the template panicked `program not built with
  -cover` in the in-order run, which made that package unordered for good
  (`TestQualifyRefusesFailingBaseline` fixture, reproduced on a one-function module).
- `linecov qualify` runs suites under the deadline the index's runner identity names unless
  `-test-timeout` is given, and refuses `missing-recorded-test-deadline` for none (main's
  `LPCV-V0-7603`, `TestQualificationDeadline`); the round-f note about the 2m default is closed.
- `tools/linecov/help.go` names `qualified` and `miss` and the lane's flags, so the command tree
  snapshot covers every dispatched subcommand.

Focused packages after the merge, through `script/heavy.sh`, one per command: `internal/linecov`,
`tools/linecov`, `internal/jslinecov`, `internal/rubylinecov`, `internal/sessiontest`,
`tools/ci-reuse-plan` and the `cmd/corvint` line-selection, use-case and help subset all pass;
`make capability-catalog-check` and `make line-citations-check` pass.

## Enablement: what the blocker was and how it is solved

The 2026-11-04 cap turned every NARROWED selection into SHADOW `line-narrowing-disabled` because the
runner identity did not bind the inherited process environment: an index collected with `CI=true`
and a local run with `CI` unset shared one identity, so a test that returns early under `CI` was
recorded as never reaching `F()` and could have been excluded from a change to `F()` where it fails.
Round g binds the environment per record instead of deferring (spec commits `6e2488ff0`,
`f1014d1f0`; code `b64b7d47a`, `6fd8f8e8e`, `824c57c62`, `4955100da`):

- `GLS-V0-017`: the collector overlay rewrites `os.Getenv`, `os.LookupEnv` and `os.Environ` in the
  test binary and every instrumented child it starts; each inherited read is reported once per
  process (`read NAME VALUE`, `unset NAME`, `all`) to `CORVINT_LINECOV_ENVAUDIT`. A read of a value
  the program set itself, or that its parent set for it (`CORVINT_LINECOV_ENVSELF`, computed by
  the exec audit), is not inherited and is not reported. The record binds each variable to the
  SHA-256 of its value or to `unset`, and a whole-environment read to the SHA-256 of every
  non-neutral `NAME=VALUE`; neutral are GOFLAGS, GOCOVERDIR, `CORVINT_LINECOV_*` and PWD. The `/4`
  header extension carries the bindings table and each test's bindings; `EnvBound` is set only when
  every record was observed, so an older or hand-built index binds no environment.
- `GLS-V0-018`: `Plan` and `Narrow` compare the bindings of every test they would leave out, and of
  each package's in-order record, against the run environment (`corvint affected`: this process;
  `corvint test`: the profile's; CI: the shard's; `linecov qualify`: its own). What does not hold
  is kept as `environment`; an unbound in-order record keeps the package whole as `environment`;
  an index without bindings is the `environment-unbound` FALLBACK (whole in `linecov select`, `ci`
  and `qualify`). Because the in-order run reads the union of what every test reads, the effective
  rule is package-level: a package narrows only under an environment that holds every variable any
  of its tests read.
- `GLS-V0-009`: the identity names the runtime settings the Go runtime reads itself (GOMAXPROCS,
  GOGC, GOMEMLIMIT, GORACE) as `runtime=`, and the `go test -exec` wrapper by content hash as
  `exec=`. CI shards run every test under `.github/testconfine` with `-timeout 50m`; the index
  workflow now collects under the same wrapper, root and deadline, and the shard plans with
  `--line-test-timeout 50m --line-exec`. The wrapper built here from the same sources has SHA-256
  `11a8f7e48078d7f7a4090d80bec3309d25953aaaca05690fbd64e38b53c25107`; CI's own build decides
  equality, and a different build is a `runner-mismatch`, never a narrowing.
- `GLS-V0-013`: static-read units are now the units the static plan's reader rules select for the
  analysed Go paths (`affected.Graph.Readers`: literal, glob, unbounded, declared scope, existence,
  import-graph, declared owner, embedding package). The 2026-11-04 rule kept every unit with any
  path token whole, and the graph records plain string literals (`"neg"`, `"other"`) as tokens, so
  the CLI fixture's `user` package, and in practice almost every unit, was `static-read` whole;
  that, not the environment, is why the 2026-11-03 twenty-PR measurement showed zero reduction.
- Fault injection (`TestFaultInjection`, `env` fixture): collection runs with
  `LINECOV_FIXTURE_SKIP`, `LINECOV_FIXTURE_ALL` and `LINECOV_FIXTURE_TOOL` set, so a guarded body,
  a whole-environment reader's body and a body only the `cmd/tool` child reaches under another
  environment are unexecuted. A mutant in each fails nothing and is excluded under the collection
  environment (three negative controls); with the variable unset the mutant fails and the bound
  package is kept whole (`environment`). A test's own `os.Setenv` binds nothing. `TestUnboundIndexDecidesNothing`,
  `TestEnvAuditParse`, `TestEnvUnbound`, `TestRunnerIDBindsRuntimeSettings`,
  `TestRunnerIDBindsExecWrapper` and `TestCollectUnderExecWrapper` cover the rest.
- Not solved, stated: FC is still read through `go env -changed`, which does not report it on
  go1.27.3; this module has no Fortran, a test that reads FC binds it, and it stays an open item.
  Host properties no variable names (CPU count, time, network, files outside the module) stay
  unmodelled; the main full-run miss detector is the backstop.

## In-order records, children and the real tree

Collecting the Corvint tree showed two interactions the fixtures had not:

1. Main's child audit (`LPCV-V0-7601`) made the in-order run of nearly every package divergent,
   since some test of almost every package starts git, a shell or node, so the package was unordered
   and ran whole for every change (7 of the 9 qualification packages). The in-order record supports
   no exclusion by itself, and the tests that start such children are divergent alone and always
   kept, so it now keeps its parent-side union over an `uninstrumented-child` audit; any other audit
   failure still yields no in-order record (`GLS-V0-008` amendment, `TestUninstrumentedChild`).
2. A test that starts hundreds of instrumented children (`TestBuiltCLIFreshUniquePermutations`)
   failed `covdata textfmt` with an argument list too long; the collector converts child counter
   directories in batches of 64.

Two harness repairs: `go test -json` frames results from output lines, so the tools' own tests,
which collect fixture modules and print another run's `--- FAIL: TestFail`, failed the baseline as
`tools/linecov.TestFail`; the collector now indents a divergent run's output and the harness counts
a failure only for a test its package declares (`GLS-V0-031`). Negative controls are drawn only from
packages whose declared tests, and whose suite importers' tests, all have records; an unrecorded
test could execute any body, so the harness names the packages it skips (`GLS-V0-032`).

## Qualification on the Corvint tree (V1-4365, GLS-V0-030..034)

Run on the WSL lane host under the CI identity, so the record can admit CI narrowing: `GOFLAGS=-race`,
`CGO_ENABLED=1`, `-test-timeout 50m`, `-exec` the `test-confine` wrapper built from
`.github/testconfine` as `ci.yml` builds it (SHA-256 `11a8f7e4…c25107`, Landlock ABI 7), with
`CORVINT_CONFINE_ROOT` the disposable checkout. The host carried a load average of 30 to 55 from other
lanes throughout, and every command had to fit a ten-minute window, which shaped the sample.

- Index: `linecov collect` at `0f73fbae6` over 20 packages (the mutated packages, their static
  suites and the control packages), 229 + 192 records merged (`linecov merge`), 72 + 52 divergent:
  every divergent record is a test that starts git, a shell or node (`uninstrumented-child`), or a
  `tools/linecov` test that reads the runner identity under collection's own `-cover` GOFLAGS, and
  is therefore always kept. Only `tools/linecov`'s in-order run stays divergent.
- Mutated packages (touched by the last 40 merges of main and outside `cmd/corvint`'s dependency
  closure, so a suite fits the window): `internal/jslinecov`, `internal/rubylinecov`,
  `internal/nativelinecov`, `tools/ci-reuse-plan`, `internal/funclink`, `internal/linecov/v8`,
  `internal/betarung`, `tools/conflict-markers-check`, `tools/capabilities-check`. Suite: those plus
  `tools/linecov`, `internal/browsercov`, `internal/browsercov/cmd/browsercov` (their static reach),
  224 declared tests in 12 packages, run whole per mutant with `go test -race -exec test-confine`.
- Seed 20261106, sites 0..8 of the seeded order in five invocations (`-start/-count`), folded by
  `linecov qualify -summarize`: **9 mutants, 4 killed, 5 survived, 0 invalid, 0 misses**; kinds
  case 2, closure 2, else-if 2, loop 2, statement 1; operators constant 4, negate 4, delete 1.
  Every test a mutant failed was in the narrowed set. Negative controls: two statement deletions in
  `tools/conflict-markers-check/main.go` (`main`, unexecuted by its tests) kept no test of their
  package and none by coverage and failed none (`control-passed`); the other packages yield no
  control because an unrecorded test of theirs or of a suite importer could execute any body.
- Narrowed versus static: the static plan runs all 224 suite tests per mutant; the narrowed set kept
  55 to 101, mean 74.1 (33 %), of which the unrecorded tests (always kept) are the floor of about
  50. Selection latency per mutant: 4.6 to 12.8 ms (`selectMillisMax`).
- Record: `.github/linecov/qualification.json` (`linecov-qualified/1`, report digest
  `sha256:5109bcff…ae99c`, index commit `0f73fbae6`, 21 source blobs at `1c0611628`); `linecov
  qualified -root . -record … -rev HEAD -exec test-confine` reports 21 sources matching.
  Reports, logs and the index: `~/lanes/lineidx-evidence/round-g/qual/`.
- What this does not show: `cmd/corvint` and every package in its dependency closure were not
  mutated here, because one suite run of `cmd/corvint` alone exceeds the window on this host; a
  mutant there is reached only through `cmd/corvint`'s own tests, which the record admits by the
  same rules. The owner decides whether this sample is enough to turn
  `CORVINT_LINECOV_NARROWING` on; the kill switch and miss detector stay the backstop.

## Twenty merged pull requests (GLS-V0-050, V1-4366)

One build (`corvint` at `1c0611628`) and one index at `28a3ff86a` (the base of the oldest of the
twenty first-parent merges ending at `b75053a5b`), collected on the lane host with the local
identity (GOFLAGS empty, no wrapper, 10m) over 288 of the 293 Go units the twenty static plans
select, in thirteen parts merged by `linecov merge`; the `internal/tasks/*` family (15 packages)
was not collected because its suites did not fit the host's window, and `cmd/corvint` was
collected in three isolated-test parts with its in-order run skipped, so both run whole. Static
`plan`, `provider` and `advice` are byte-identical with and without `--line-index` for every range.
Counts are declared top-level test names of the static plan's Go units, not executions.

| PR | Static Go packages | Static names | Line-selected names | State | load / select / git ms | Fallback reasons (kept-by-fallback category counts) |
|---|---:|---:|---:|---|---|---|
| #115 | 150 | 9563 | 9563 | FALLBACK | 2.9 / 0.0 / 0 | static-scope-unknown |
| #120 | 43 | 5224 | 5224 | SHADOW | 3.6 / 175.4 / 470 | non-go 151, static-read 5073 (change paths: added 1, non-go 10) |
| #112 | 33 | 5009 | 5009 | FALLBACK | 8.9 / 0.0 / 0 | static-scope-unknown |
| #118 | 62 | 5701 | 5701 | SHADOW | 4.7 / 216.3 / 459 | non-go 153, static-read 5548 (change paths: added 3, non-go 11, test-helper 1) |
| #117 | 58 | 5467 | 5467 | FALLBACK | 3.2 / 0.0 / 0 | static-scope-unknown |
| #116 | 265 | 13198 | 13198 | FALLBACK | 3.0 / 0.0 / 0 | static-scope-unknown |
| #114 | 260 | 12610 | 12610 | FALLBACK | 4.0 / 0.0 / 0 | static-scope-unknown |
| #113 | 255 | 12411 | 12411 | FALLBACK | 4.7 / 167.8 / 209 | fallback-scope-unknown |
| #111 | 48 | 5359 | 5359 | FALLBACK | 12.0 / 290.3 / 307 | fallback-scope-unknown |
| #109 | 250 | 12381 | 12381 | SHADOW | 3.4 / 211.9 / 573 | declaration 60, non-go 5484, static-read 6837 (change paths: added 3, declaration 1, non-go 6, test-helper 1) |
| #110 | 88 | 6796 | 6796 | SHADOW | 4.3 / 172.6 / 364 | added 371, declaration 192, non-go 547, static-read 5686 (change paths: added 3, declaration 2, non-go 5) |
| #108 | 253 | 12473 | 12473 | SHADOW | 4.0 / 137.2 / 491 | added 2600, non-go 3080, static-read 6793 (change paths: added 1, non-go 4, test-helper 5) |
| #107 | 47 | 5378 | 5378 | SHADOW | 3.0 / 128.2 / 308 | non-go 607, static-read 4771 (change paths: declaration 1, non-go 14, test-helper 2) |
| #105 | 139 | 9485 | 9485 | FALLBACK | 4.3 / 0.0 / 0 | static-scope-unknown |
| #106 | 55 | 5544 | 5544 | SHADOW | 3.0 / 46.1 / 204 | non-go 446, static-read 5098 (change paths: non-go 2, test-helper 1) |
| #103 | 45 | 5326 | 5326 | FALLBACK | 3.0 / 0.0 / 0 | static-scope-unknown |
| #104 | 51 | 5413 | 5413 | SHADOW | 3.0 / 44.5 / 230 | non-go 682, static-read 4731 (change paths: non-go 7) |
| #102 | 131 | 8551 | 8551 | FALLBACK | 4.3 / 0.0 / 0 | static-scope-unknown |
| #101 | 37 | 4717 | 4717 | SHADOW | 4.5 / 10.8 / 137 | non-go 74, static-read 4643 (change paths: non-go 1, test-function 2) |
| #100 | 59 | 5733 | 5733 | SHADOW | 4.4 / 4.5 / 153 | non-go 598, static-read 5135 (change paths: non-go 4, test-function 3, test-helper 1) |

Result: **156,339 static versus 156,339 line-selected names, zero reduction**, for the same reasons
as the 2026-11-03 sample and one new one, now visible because the static-read rule is precise:

- Eight ranges are `static-scope-unknown` (the static plan carries `LANGUAGE_FRONTIER` unknowns
  outside the AFP-V0-982 allowlist) and two `fallback-scope-unknown`: nothing to narrow soundly.
- The ten analysed ranges are SHADOW only because no qualification record exists at their bases;
  their proposed decisions keep every unit whole, and the categories say why: every one of these
  pull requests changes non-Go paths (workflows, docs, scripts, fixtures), whose static reach is
  reader-based (`PATH_LITERAL_READER`, `UNBOUNDED_READER`, `DECLARED_READ_SCOPE`,
  `IMPORT_GRAPH_READER` witnesses dominate their plans), and a reader of a changed path stays whole
  as `static-read`. The units reached through imports are kept whole by the same non-Go paths'
  fallback reach. No range is a pure Go body change.
- Where a change is a Go body edit, narrowing is real: the nine qualification mutants above kept
  33 % of their suites. The gain on real pull requests therefore depends on changes being Go body
  edits, and on the static plan bounding its reads, not on the line selection itself.
- Latency: load 2.9 to 12.0 ms; select 4.5 to 290 ms and git 137 to 573 ms on ranges whose drift
  from the index commit holds up to 700 changed paths, on a host at load 10 to 40. The
  `TestLazyPlanBudget` 50-file workload (synthetic index of 1,250 tests) passes its 100 ms p95
  budget when the host is quiet and misses it (p95 139 to 166 ms) while other lanes load it; the
  budget is a quiet-host contract (`GLS-V0-003`). Retained:
  `~/lanes/lineidx-evidence/round-g/measure/` (`measurement.json`, per-range receipts, parts).

## Round g review repairs (2026-11-08)

The round g review (Codex, `CHANGES_REQUIRED` at `4acb38a6a`) found two P1 and two P2 defects.
Each repair below fails closed. The design choices are orchestrator-made under delegation.

- **Collector-modified variables (P1, GLS-V0-017).** Collection sets GOCOVERDIR and adds the
  coverage and overlay flags to GOFLAGS, and the old audit treated both as neutral. So a test that
  skipped its assertion path whenever GOCOVERDIR was set recorded no coverage there and could be
  excluded. Now an inherited read of GOFLAGS, GOCOVERDIR or `CORVINT_LINECOV_*` is reported as
  `modified NAME` and binds `collector-modified`, which no environment holds: the test is always
  kept, and its package runs whole when the in-order record carries the binding. A
  whole-environment read sees the same variables, so it binds `collector-modified` too, instead of
  an environment hash. Reads whose first caller outside `os` is the coverage runtime or the exec
  audit are the collector's own and are not reported, and the counter windows read their variable
  through `syscall`; without this every record would be unbound. The fault-injection fixture gains
  `cov` (bodies reached only without GOCOVERDIR or without `-cover` in GOFLAGS) and `envall`. It
  asserts those packages run whole under every change and that no other record binds a collector
  name.
  - Decision: a read of a collector-modified variable is unbound, not reproduced. Collection cannot
    observe the value an ordinary run would see.
  - Cost: precision. Such tests, and whole-environment readers, are never excluded.
- **TestMain-only packages (P1, GLS-V0-008, GLS-V0-013).** Collection now records the in-order
  suite for every test binary, including one that declares no named test. `HeadTestNames` and
  `HeadTests` give a directory that has `_test.go` files but no named test an empty, non-nil list.
  A unit like that runs whole as `suite-unrecorded` when it has no complete suite record, and
  whole when the change or drift reaches its suite record. A suite-only run that starts an
  uninstrumented child diverges, so the package stays unordered. Tests:
  `TestSuiteOnlyPackageKeptWithoutSuiteRecord` (`Plan` and `Narrow`, with and without the record),
  `TestCollectSuiteOnlyPackage` and `TestHeadTestNames`.
  - Decision: `suite-unrecorded` is a whole-unit category. It is not an exclusion with a witness.
- **CI `qualified` wrapper (P2, GLS-V0-034).** The ci.yml `qualified` call now passes
  `-exec "$RUNNER_TEMP/test-confine"`. `TestWorkflowQualifiedInvocationMatchesWrapperRecord` reads
  the invocation from ci.yml and runs it against a record bound to a wrapper. The record admits;
  the old invocation, without `-exec`, refuses it.
- **Miss confirmation conditions (P2, GLS-V0-046).** Each suspect now carries `runner`, the
  identity of the index the narrowed selection read (`Suspect.Runner`). `linecov-miss` builds
  `test-confine` with the same commands as the CI shards, a test pins the two steps equal, and
  derives the expected runner prefix from `go env`. That prefix covers the race build, the
  architecture variables (pinned to `linecov.ArchFeatureVars`), GOEXPERIMENT, the empty runtime
  environment and the wrapper hash. Confirmation reruns at base and merge under that wrapper,
  `CORVINT_CONFINE_ROOT`, `GOFLAGS=-race` and the recorded deadline.
  `TestLinecovMissConfirmationReproducesRecordedRunner` runs the real step. A test that needs 30
  minutes is confirmed under a recorded 50m deadline, and not confirmed under a 20m one. A runner
  without the wrapper, with another wrapper, without a deadline, without the race build or with
  no runner at all is not reproduced.
  - Decision: a suspect whose runner is absent, not reproducible here, or cannot finish three runs
    under its deadline within the budget (`BUDGET_SECONDS` 17400, step 300 min, job 360 min) stays
    suspected and is never confirmed. A suspect from before this change, which has no `runner`,
    is therefore never confirmed.
  - Decision: a confirmed record retains the full suspect, including its runner.

### Remaining limits

- The shell compares only the identity fields named above. Variables the runner identity refuses
  outright (`UncarriedEnvVars`) are not re-checked in the workflow.
- Child `go build` launchers in tests still see the coverage GOFLAGS. Their reads are bound
  (kept), not reproduced.
- The index format is unchanged. An index collected before this round still holds the old
  bindings: TestMain-only packages have no suite record (now `suite-unrecorded`, whole), and
  collector-variable reads are unrecorded neutral names. Only a fresh collection binds them.
- The committed qualification record must itself be produced under the wrapper for the CI check
  to admit it.

Focused tests, WSL, go1.27.3, `GOTOOLCHAIN=local`: `./internal/linecov/...`, `./tools/linecov`,
`./tools/ci-reuse-plan` and the four language linecov packages plus `./internal/sessiontest` pass.
`TestLazyPlanBudget` missed its quiet-host budget once under parallel package load and passed when
run alone. In `./cmd/corvint` (`-run "Linecov|Affected"`),
`TestAffectedRejectsNonRepositoryAndExtraArguments` fails only because `/tmp` on the lane host is
inside a Git work tree.
