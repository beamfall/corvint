# 2026-11-16: line-selection qualification ledger (V1-4365)

> Ruby row superseded by `2026-11-16-r1ruby-review-repair.md` (decision 0601; it replaces the state in `2026-11-16-r1ruby-ruby-line-admission.md`): Minitest qualified, RSpec not. The rest of this ledger stands.

Requirements: LPCV-V0-950 through LPCV-V0-955, LPCV-V0-963; GLS-V0-030 through
GLS-V0-034. Lane source: `df315e35056d15c7933b5ff97c605cedf5ef4919`.

## Admission decision

This ledger distinguishes a retained historical measurement from qualification of the
current collector/selector/runner tuple. No current production exclusion is enabled
by this documentation. A family marked **UNQUALIFIED** retains its whole static plan;
zero is never substituted for an unobserved miss count. Swift and Kotlin/JVM remain
supported 1.0 providers (decision 0600), with this same qualification boundary.

## Beamfall corpus pins

`git ls-remote https://github.com/beamfall/REPOSITORY HEAD` printed these pins in this
lane. These are frozen inputs for a future fault run, not execution evidence. Every
row has mutant count, kills, selected-set sizes and misses **NOT_MEASURED**, and
negative-control state **NOT_OBSERVED**. No suite or fault run was performed at these
pins; the owner's criterion permits an explicitly unqualified static family.

| Family | Repository | Full commit | State / effective plan |
|---|---|---|---|
| Go core | `https://github.com/beamfall/core` | `b6b3428df171a8f6181175076fcbbb4972936995` | UNQUALIFIED / static |
| TypeScript / JavaScript web | `https://github.com/beamfall/web` | `43e6bb70b445e5e6233088ca4e5f950b9135e80b` | UNQUALIFIED / static |
| TypeScript / JavaScript docs | `https://github.com/beamfall/docs` | `2f5c89d930fc1e4ee4e767fe93628d99ec6369b9` | UNQUALIFIED / static |
| Rust | `https://github.com/beamfall/linux` | `7394ca245bd23e669270639cf763aa0af504ee07` | UNQUALIFIED / static |
| C# | `https://github.com/beamfall/windows` | `4277d16576f9dccc41162d1fc3a5cc7027c66d49` | UNQUALIFIED / static |
| Kotlin / JVM | `https://github.com/beamfall/android` | `b84a7ae875a6a498c67abd0d4143e0c5de959881` | UNQUALIFIED / static |
| Swift | `https://github.com/beamfall/apple` | `a01a07388014a3db1650f57c87d4d1f221d69090` | UNQUALIFIED / static |
| Python | `https://github.com/beamfall/workspace` | `eae39c5e11ecd0da0d87cad8184124fbd33f9123` | UNQUALIFIED / static |
| Go plugin repository | `https://github.com/beamfall/plugin-sdk` | `66cb3ff308037fab30d53753b1272547876a39c1` | UNQUALIFIED / static |

The chosen plugin repository is `plugin-sdk`; its pinned `go.mod` was read through
the repository API. The initial `plugin-go` lookup returned repository-not-found and
is not a corpus pin. The pin ledger is retained outside the repository as
`family-pins.json`, SHA-256 `1f8cc669fc656ac8fec6491cb35194fa4c90609c9c0c461dcb9a68ad719fb5b3`.

## Corvint Go evidence and binding limit

The existing `.github/linecov/qualification.json`, read in this lane, retains a Go
run at `993c60eba4d4cfeba7cdb3f7b3f21130596e4247`: seed 20261109, 20 sites,
20 mutants, 13 killed, 0 invalid, 2 controls and 0 misses. These are historical
record fields, not a fresh run. The original scope, selected sizes and exclusions
are in `docs/build-log/2026-11-09-linequal.md`. The harness and seeded-miss tests
remain in `tools/linecov/qualify_test.go`; cross-package negative detection is in
`qualify_crosspkg_test.go`. They exercise statement deletion, condition negation,
constant mutation, case expressions, else-if conditions, loop headers, closures and
the uncovered-body fallback. No assertion or mutant family is removed.

A fresh source-identity comparison of the record's 29 source blobs against this
lane base found **8 mismatches**: admission.go, collect.go, compact.go, index.go,
pgroup.go and runner.go under internal/linecov; internal/liveverify/affected/readers.go;
and tools/linecov/qualify.go. The historical record therefore does **not** qualify
this lane's current implementation. It must not be used to re-enable CI narrowing.
A fresh tuple run and replacement CI qualification record are outside this docs
record; `.github/` changes are prohibited in this lane.

Source comparison: `go-qualification-source-audit.json`, SHA-256 `1664b2308b3edced4be924140057ee7b038a6dc399ce57b90787854fc5363532`.

## Public Ruby corpus

Pin: `https://github.com/bblimke/webmock`, tag v3.25.1 resolved to
`f90fd503000279076c02269299067e08bf1ca1ee`. License: MIT, retained upstream
`LICENSE` (Copyright 2009–2010 Bartosz Blimke), SHA-256 `b3c3b74c1eb3e9556b22551b36ec556c03ce5c189d825edfb986d1df9d37e837`.
The unmodified upstream Rakefile defines the complete `spec` RSpec suite and the
`minitest` suite; the gemspec declares both runners. The clone is outside the
repository at `/home/agent/lanes/r1docs-evidence/webmock`. No corpus source is vendored.

The earlier dry-equalizer record in `docs/experiments/linecov-2026-11-04-ruby-shadow.json`
is a four-fault RSpec shadow at a different commit; it cannot stand in for Minitest
or qualify the current tuple. The WebMock run and its limitations follow below.

### Runtime and baseline

This host initially had no Ruby. Ruby 3.3.8 was built outside the repository, with
Zlib 3.1.1, OpenSSL 3.0.13 and Psych 5.1.2 reported by the resulting runtime. The
upstream dependency ranges initially resolved Minitest 6.1 (which rejects the
upstream `--pride` option), Curb 1.5 (unsupported by this WebMock pin), and an
Async HTTP tuple lacking the expected reason constant. The first complete RSpec
attempt did not finish: `timeout 600 bundle exec rake spec` sent SIGTERM while
Async's pool waited to drain; its remaining task-owned processes were killed.
That attempt has no complete-suite count and is **NOT_COMPLETE**.

A separate, retained `Webmock.Gemfile` pins Minitest below 6, Curb below 1.1,
Async HTTP 0.88.0 and protocol-http1 0.30.x, without modifying corpus source or
assertions. With `BUNDLE_GEMFILE` naming that file and the external `ruby-env`
wrapper supplying the built runtime and installed gems:

- `timeout 600 bundle exec rake minitest`: **PASS**, 23 runs, 50 assertions,
  0 failures, 0 errors, 0 skips; 0.058600 seconds (suite seed 33773).
- `timeout 600 bundle exec rake spec`: **FAIL**, 5,190 examples, 467 failures;
  51.05 seconds. Observed failures include real-network Google HTTP expectations
  receiving 301 instead of 200 and TLS connection resets. This is not a finding
  that every failure has the same cause. A passing complete RSpec baseline was
  **NOT_PRODUCED**; a full RSpec mutation qualification was consequently **NOT_RUN**.

### Measured Minitest fault sample

The current `tools/linecov` binary collected all upstream Minitest tests with
`ruby-collect`, using the command below, a two-minute suite timeout and a
30-second test timeout. Seed **20261116** orders the three fixed mutation sites
and the complete suite. The external `qualify-minitest.py` then commits each
one-line mutant separately, calls `ruby-select` against the pinned base, runs all
23 tests with an external result reporter, compares every failing test with the
selected set, and restores the pinned base. No test assertion was weakened.

```sh
bundle exec ruby -Ilib -Iminitest -e 'Dir["minitest/**/*.rb"].sort.each { |f| require File.expand_path(f) }' -- --seed 20261116
```

All sites are in `lib/webmock/request_execution_verifier.rb`. Every mutant ran
23 tests. The collection's two static test files are `minitest/webmock_spec.rb`
and `test/shared_test.rb`.

| Mutation | Line | Mutant commit | Selected | Failing tests | Misses |
|---|---:|---|---:|---:|---:|
| Condition negate (`==` to `!=`) | 24 | `cc645bec69ec67cba5243a421cc38f937b97f523` | 23 | 14 | 0 |
| Constant change (`times(1)` to `times(2)`) | 70 | `4302e2c6bedaa062745781194ce37b4eb170228c` | 23 | 0 | 0 |
| Statement delete (assignment to `nil`) | 10 | `c03720f3108db870b2e917d42fd262fb1ff8c6b4` | 23 | 1 | 0 |
| Unexecuted-line control (argument to `999`) | 66 | `4b7e38f7f0f48f5346db153009967c176a33d9af` | 23 | 0 | 0 |

The three fault mutants killed **2 of 3**; the additional control killed no test.
Every miss list is empty. However, every plan reports CoverageSelected **0**,
StaticFallback **23**, Excluded **0**, and `coverage-collector-unqualified`;
`Scope` is `user` and `GateEligible` is false. Zero observed misses here comes from retaining
the full static suite, not from successful narrowing. The separate complete-suite
Coverage probe measured zero hits on control line 66 (lines 10, 24 and 70 had
24, 18 and 3 hits). The control **fails the zero-selected requirement**: it selected
23 tests. The family therefore remains **UNQUALIFIED / static**, despite the
passing Minitest baseline. The sample does not qualify RSpec, alternate mutation
classes, or the complete Ruby family.

### Retained evidence and disposition

All following artifacts are under `/home/agent/lanes/r1docs-evidence/`. The raw
RSpec failure log is 1,360,885 bytes and remains outside the repository. The JSON
sample record retains every selected and failing test name, every miss list,
mutant commit, and digests of each plan and test-result file.

| Artifact | SHA-256 |
|---|---|
| `Webmock.Gemfile` | `e4be3c19c363b7ea5686842e005606101c992d7366624a02f754f383fafd0d62` |
| `Webmock.Gemfile.lock` | `a050c71c73cedcea2fb82cac9e7828c7179602565b48fd8f92032c979da71c92` |
| `webmock-minitest-supported.log` | `3306c99e1b20aaa3338a2789cc78aa0242e7518164cb522dccebfcefc01b7a11` |
| `webmock-rspec-supported.log` | `3910301e88c990e78b7ba5af7770ebe0f10dcddc3dd9ba8277f290db471eddfb` |
| `webmock-rspec-baseline.log` | `767e26b935302cfe371dbec1e3f3a07d17fe5816688a3d6c5415b7821a498a91` |
| `webmock-minitest.idx` | `7c0e197b51597a373b398cd1f3a362a32c0cdbd23f2804314e068fd6216e1530` |
| `qualify-minitest.py` | `18beec6032e900141fc7d30ea3c756b3b859d84848cc388daca45f14b20dc553` |
| `minitest-report.rb` | `2198444d88a52c39ca8c49a8534b45f71845a9d2e551f9a1c7a709e360bd7da8` |
| `minitest-qualification.json` | `651eccd148dad5cd175add47a3dae886d30ef46bb6965b64afdc46ea8be2cd32` |
| `minitest-line-hits.json` | `c725b5f2a37cab2c2da0ccdea63a1e2edc7674ee3bd05b71b2c2c93516706c41` |

**V1-4365 remains PARTIAL.** The pinned public corpus was run, and all Beamfall
families now have explicit, pinned unqualified records. Ruby qualification still
needs a passing full RSpec baseline in a compatible external corpus environment,
a collector qualification that passes the zero-selection control, and full fault
runs for both runners. Current Corvint Go enablement also needs a fresh bound run
and its CI qualification artifact; editing `.github/` is prohibited in this lane.
The historical Go result is retained, not silently promoted across changed source
identities. No frozen wire contract, core-freeze output, test assertion, or CI
configuration changed. These limits prevent a delivered ticket claim.

## Self-review repair: delivery marker

The `delivered` trailer in `0c897512c888154fceb1b0680d078c9b525fd36c`
was incorrect. The attempted `Corvint-Ticket-State: partial` repair was invalid:
native `worktrail.ValidState` accepts only `started`, `delivered`, and `blocked`.
`latestByTicket` skips malformed transitions, so that attempted repair did not
supersede delivery. The independent-review repair appends a valid `blocked`
transition with a reason; only that valid transition cancels the earlier delivery.
V1-4365 must stay open: the 467 RSpec baseline failures, unrun full RSpec mutation
qualification, and 23-test static fallback for the unexecuted-line control leave
the acceptance criteria unmet (`GateEligible: false`). No new qualification is
claimed under LPCV-V0-950 through LPCV-V0-955, LPCV-V0-963 or GLS-V0-030 through
GLS-V0-034.

History is retained under the no-amend rule. A trailer-presence checker is not
native ingestion evidence. The final V1-4365 transition must remain `blocked`:
criterion 4 requires fresh Go qualification and a `.github/` change reserved to
the orchestrator. The native ingestion check is read-only (`close-merged
--dry-run`); this lane does not write the task store.

## Independent review: external baseline repair

The earlier 467-failure RSpec run remains a failed measurement. Its failures were
partitioned by the observed exception or response and retained by original failure
number in `rspec-failure-counts.json`; complete example excerpts remain in
`rspec-failure-groups.json`. These groups total exactly 467; no failed example was
removed from the corpus or relabelled passing.

| Observed cause | Failures |
|---|---:|
| external connection reset | 7 |
| external HTTP redirect (301 versus expected 200) | 30 |
| Async protocol headers / EOF incompatibility | 18 |
| external empty response | 42 |
| external TLS failure | 34 |
| external server closed connection | 7 |
| HTTP.rb 6 API incompatibility | 320 |
| Patron 1.1 file API incompatibility | 2 |
| external Typhoeus response mismatch | 7 |

The pinned upstream CI (`.github/workflows/CI.yml` at the corpus commit) runs a
Ruby matrix of 2.6 to 3.4 plus head (3.3 included), installs `libcurl4-openssl-dev`,
and runs `bundle exec rake`, whose default task runs `spec`,
`spec_http_without_webmock`, `test` and `minitest` online. It does not
pin a dependency lockfile, so this lane cannot reconstruct the exact historical
CI gem resolution. Ruby 3.3.8 and the previously built native libraries were kept.
The external `Webmock-review.Gemfile` additionally pins HTTP.rb 5.2.x,
protocol-http 0.49.x, Rack 2.2.x, and Patron 0.13.3.x. The lockfile retains every
resolved version; these are a measured compatible tuple, not a claimed historical
CI lockfile. No upstream test, tag, assertion or source file was edited.

Environment repair was attempted first: the full online suite then ran 5,190
examples with 139 failures (56.74 s). The corpus's own `NO_CONNECTION=1` switch,
defined in `spec/spec_helper.rb` by `filter_run_excluding net_connect: true`, was
then used. It removes 344 examples, yielding the complete 4,846-example offline
suite. The first offline run still had two Patron file-API failures; after pinning
Patron 0.13.3, the same complete offline suite passed: **4,846 examples, 0 failures,
0 pending, 0 outside-example errors**, 8.60994335 s. The latest complete Minitest
baseline also passed: **23 tests, 50 assertions, 0 failures/errors/skips**. The
original public corpus, commit and licence binding are unchanged.

Commands, with the external `ruby-env` wrapper and `BUNDLE_GEMFILE` naming the
retained review Gemfile:

```sh
bundle install
timeout 600 bundle exec rake spec
NO_CONNECTION=1 timeout 600 bundle exec rspec --pattern 'spec/**/*_spec.rb' \
  --require ./spec/spec_helper.rb --format json --out EXTERNAL_REPORT
timeout 600 bundle exec rake minitest
```

The initial offline report and the final one are both retained. The apparent real
HTTP fetch in Patron's `get_file` test was itself a Patron adapter incompatibility:
that test stubs its request, and passes unchanged with Patron 0.13.3. It was not
filtered out. No replacement public corpus was needed. The separate upstream
`spec_http_without_webmock` task (`bundle exec rake spec_http_without_webmock`,
online, its own `real_net_http_spec.rb` helper) also passed in this round: **38
examples, 0 failures**.

### Full-suite fault runs

`qualify-ruby-review.py` freezes seed **20261116**, seven fault sites covering the
three mutation operators plus case expressions, else-if, a loop header and a
closure, and one negative control. Each fault runs all **23 Minitest tests** and
all **4,846 offline RSpec examples**. Both seeded baselines pass before the first
fault. Each fault is a separate commit descending directly from the pinned corpus;
the disposable fault clone is distinct from the collector's clean clone.

| Fault / source site | Minitest failing tests | RSpec failing examples |
|---|---:|---:|
| statement-delete: `lib/webmock/request_execution_verifier.rb` line 10 | 1 | 0 |
| loop-header: `lib/webmock/util/query_mapper.rb` line 212 | 2 | 142 |
| case-expression: `lib/webmock/util/query_mapper.rb` line 249 | 2 | 116 |
| constant-change: `lib/webmock/request_execution_verifier.rb` line 70 | 0 | 224 |
| else-if: `lib/webmock/request_execution_verifier.rb` line 21 | 14 | 1175 |
| closure: `lib/webmock/util/query_mapper.rb` line 254 | 0 | 7 |
| condition-negate: `lib/webmock/request_execution_verifier.rb` line 24 | 14 | 1105 |
| negative-control: `lib/webmock/util/query_mapper.rb` line 107 | 0 | 0 |

Kill counts: **5/7 Minitest**, **6/7 RSpec**. Both complete suites kill **0** tests
for the control at `lib/webmock/util/query_mapper.rb` line 107. Separate complete
baseline Coverage probes measure **0 hits** on that line in both runners; the
control changes `array_value = false` to `array_value = true` inside the unused
method. Surviving mutants remain in the record. The original three-fault sample
and its failed control remain historical evidence; they have not been discarded.

Fresh Minitest collection (the external `rubyprobe` calls `rubylinecov.Collect`
and `rubylinecov.Index`, the functions `linecov ruby-collect` calls) records all
**23/23** passing tests with successful isolation and no unordered file. The real
`linecov ruby-select` then plans each fault head against the pinned base:

| Fault | Candidate selected | Effective selected | Misses | Candidate fallback (`rubylinecov.Analyze`) |
|---|---:|---:|---:|---|
| statement-delete, line 10 | 23 | 23 | 0 | the change reaches load-time code |
| loop-header, line 212 | 23 | 23 | 0 | the change reaches load-time code |
| case-expression, line 249 | 23 | 23 | 0 | the change reaches load-time code |
| constant-change, line 70 | 23 | 23 | 0 | the change reaches load-time code |
| else-if, line 21 | 23 | 23 | 0 | the change reaches load-time code |
| closure, line 254 | 23 | 23 | 0 | the change reaches load-time code |
| condition-negate, line 24 | 23 | 23 | 0 | the change reaches load-time code |
| negative-control, line 107 | 23 | 23 | 0 | edit outside a method body `lib/webmock/util/query_mapper.rb:107` |

Every observed Minitest failure is selected (**0 misses**), but only because no
site narrows: each plan is the whole static suite, `GateEligible` false,
`coverage-collector-unqualified`. The zero-selection control **FAILS** (23
selected). RSpec has no index (below), so its selected-set sizes and misses are
**NOT_PRODUCED**.

The harness initially stopped before mutation on a nullable Minitest `skipped`
value, then stopped after five completed faults on a checked closure coordinate
(255 named the closing bracket; the intended `val` is line 254). Both attempts are
retained. The coordinate guard prevented mutation of the wrong statement, and the
remaining intended faults resumed without rerunning or dropping completed rows.
RSpec identities use the example ID's rerun path, matching the collector's
`metadata[:rerun_file_path]`, rather than a shared example's definition file.

### Criteria 3 and 6: where this lane stops

The owner asked for criteria 3 and 6 to be finished in this round unless something
cannot be done from this host. Three things stop them; each is stated with its
command and output. The fault runs and baselines above stand; nothing is promoted.

1. **No RSpec index (criterion 6).** The collector runs each test alone, serially,
   after the ordinary run (`internal/rubylinecov/isolate.go`, `isolateArtifacts`),
   and cannot resume. One isolated run of one example with the collector loaded,
   exactly as `collectOnce` launches it (`RUBYOPT=-r corvint_linecov.rb`,
   `CORVINT_LINECOV_ONLY` and `CORVINT_LINECOV_PACKAGE` naming
   `./spec/acceptance/async_http_client/async_http_client_spec.rb[1:2]`,
   `NO_CONNECTION=1 bundle exec rspec --pattern 'spec/**/*_spec.rb' --require ./spec/spec_helper.rb`),
   printed `1 example, 0 failures` (process exit 1) and took **5,862 ms** and
   **5,966 ms** (two runs; the suite load alone, `--dry-run`, took 2,780 ms). The 4,846 isolated
   runs therefore need about 7.9 hours, serially. This lane may not start
   background jobs and a foreground command ends after 10 minutes. Missing: a host
   run of `linecov ruby-collect -root WEBMOCK -out rspec.idx -command
   '["bundle","exec","rspec","--pattern","spec/**/*_spec.rb","--require","./spec/spec_helper.rb"]'`
   with `NO_CONNECTION=1` and a multi-hour budget, or an accepted parallel or
   resumable collector.
2. **The accepted Ruby selector cannot select zero (criterion 3).** `Analyze`
   maps an edit only inside a method body some recorded test called; an edit in a
   method no test called is a fallback. That is accepted requirement
   `LPCV-V0-7605` ("uncalled method edits and changed test files use static
   fallback"), asserted by `TestUncalledMethodFallback` and by `checkCalc`'s
   line-18 case in `internal/rubylinecov`. An edit in a called method selects at
   least that method's tests. The control above therefore selects the full suite
   by design. Selecting zero needs an owner amendment of `LPCV-V0-7605` and changes
   to those assertions; this round forbids weakening an assertion, so no selector
   code changed. The seven faults also fall back, because the Minitest load bucket
   (`rubylinecov.Load`) reaches every faulted method.
3. **No Ruby admission path exists.** `languageDecide` and `languageWorkingPlan`
   (`tools/linecov/languages.go`) set every effective file whole with
   `coverage-collector-unqualified` and never read a qualification.
   `linecov.CheckQualification` (`internal/linecov/admission.go`) accepts only a
   `family: "go"` record read from `.github/linecov/qualification.json`. "Qualifying
   through the real admission path" would first need a specified Ruby record and
   check, which is new capability, and a record location outside `.github/`
   or a `.github/` change, which this lane may not make.

Ruby stays **UNQUALIFIED / static**. Criterion 3: NOT_DONE (control selects 23
Minitest tests; RSpec control selection NOT_PRODUCED; kills are 0 in both
suites). Criterion 6: NOT_DONE (both complete baselines pass; all seven faults and
the control ran against both complete suites; RSpec selection NOT_PRODUCED).

### Baseline evidence bindings

All paths below are under `/home/agent/lanes/r1docs-evidence/`; SHA-256:

- `rspec-failure-counts.json`: `c7d7bc1045d4b076d1a3447efb46ef9969ac0840f9450a747822739e54786b86`.
- `rspec-failure-groups.json`: `ee1693cf31d0fff43057835ae34e38d9282f77a9f4658009234118b31f442e88`.
- `Webmock-review.Gemfile`: `1c03a224074a3e7cfdd72317a7d10b77dde07750bcc4a6216d3a992b60933c52`.
- `Webmock-review.Gemfile.lock`: `c7b28a31fa528d0eb7fcd5c1a3d68d274681219875dc807789a0c6b4f8a57ff2`.
- `webmock-review-online.log`: `98e57043691622e62a8cd3b31fe868b87aa245e5c566f75d0ef2e0520d3df6c3`.
- `webmock-review-offline.json`: `d90ebb2cad45f3ca6d7b0333e92cc3dc7be44910f60dac9ab8051cbcb0c9dbe5`.
- `webmock-review-offline-final.json`: `bb68fb9c43ccff71c974b350a9d983e3a2523a56f80b296e682fe1231b89f011`.
- `webmock-review-minitest-baseline.log`: `1431498240d57f78fbc01b1130f9a091c30a39afbce18e1254fef54e21ad0db1`.
- `minitest-review-collector.outcomes.json`: `f5b7651f917cead18e2000e3cce65b716c66f74690641049473c2854f7071587`.
- `minitest-review-collector.idx`: `847a713142661fb684f90f67611c4945ebd33aed41229e15bf320020bb1a085c`.
- `minitest-review-selection.log`: `065ac7fd2673c7d9754db6ddebb0161ef0c6bb3c9ec5c924e825f4485bf8c6a0`.
- `minitest-review-hits.json`: `f71c63017a03507c4f600f0faa9f355c57d00b73d0d9d87adfeb1d056a338a22`.
- `rspec-review-hits.json`: `e288c1a12e4e21e421e8d0c6b05d59a121e4a71e7b5bd27f837ef6b36e4f1a47`.
- `ruby-review-faults.json` (seed, sites, fault heads, both suites' failing tests, Minitest plans): `50adb028fc6e34b256e62bfff23ed0587867920fc9d32ecdc168d7019563adbb`.
- `qualify-ruby-review.py`: `bd9bf59adacbcfdd5afa718f6f3a8cce664ca632befbc8a8bae5a4bac1ec1d47`.
- `select-ruby-review.py`: `c20e878b3ee6e65022af2991a54147c3959984339fd82f87dcd61eca0007ddff`.
- `minitest-review-analyze-all.log` (candidate fallback per fault): `f4ff836540fdfde05bb17cc39619f0b193be9b420b0ef75313934e0ae97d4815`.
- `webmock-review-without-webmock.log`: `991abee493927641d427ee71c759177ecf5c7ae4802ff36af4e1a766c1ab999e`.
- `rspec-isolated-probe/run3.log`: `ad8ee8497c3feb4766c098aba4217eabbe1ed426146168c1c16b526ebc1f5778`; `run4.log`: `943cae4461ae93247a07c2b7b983abf429da636271c1affd1a24f8a1dc090f46`; `suite.jsonl` (run 4 dump, one test event): `f939948c87aca91e6b182d6f6954e13a711324f0d74ccec7f64f7188bda44321`. Runs 1 and 2 passed `./`-prefixed package paths, selected 0 examples and are not timing evidence.
