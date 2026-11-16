# 2026-11-16: line-selection qualification ledger (V1-4365)

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
was incorrect and is superseded by this repair's `Corvint-Ticket-State: partial`.
V1-4365 must stay open: the 467 RSpec baseline failures, unrun full RSpec mutation
qualification, and 23-test static fallback for the unexecuted-line control leave
the acceptance criteria unmet (`GateEligible: false`). No new qualification is
claimed under LPCV-V0-950 through LPCV-V0-955, LPCV-V0-963 or GLS-V0-030 through
GLS-V0-034.

History is retained under the no-amend rule. The external lane trailer checker
counts any historical delivered block and does not cancel it on a later partial
block. Its pass cannot authorize closing this ticket. Integration must exclude
V1-4365 from automatic completion until qualification succeeds; task-store writes
and changes to the host integration tooling are outside this repair's authority.
