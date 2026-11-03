# Ruby per-test line-coverage collector (V1-4374)

Base: `c82e85099bd0a13b54234eb6fb20e63c234e1a64`, stacked on `claude/v1-4405-language-plugin-hooks`.
Requirements: `LPCV-V0-950`, `952`, `953`, and partly `954`, `955` and `962` in
`docs/specs/live-proof-carrying-verification-v0.md` (`962` partly: an incomplete run publishes no
record, but the collector runs tests in order only, with no isolated pass or `isolation-divergent` check);
registration under `LGP-V0-013` and `LGP-V0-020`.
Status: implemented, unqualified.

## Decisions

- One process per suite. `internal/rubylinecov/linecov.rb` is loaded via `RUBYOPT`. It starts `Coverage` (lines and
  methods), hooks the first framework base class that opens (RSpec reporter, `Minitest::Test`,
  `Test::Unit::TestCase`; Rails tests are Minitest), and takes `Coverage.result(stop: false, clear: true)`
  around each test, which gives per-test counters without a process per test (`LPCV-V0-955`).
- Coverage outside every test (file loads, `before(:all)`) goes to the pseudo-test `(load)`. A test whose window
  required a repository file is merged into it. Any selection that reaches `(load)` falls back, because
  load-time code runs for every test.
- Subprocesses and forks inherit the running test's sequence number and dump under it. A process that
  `exec`s dumps first. A child whose dump is missing `done`, or that ran another Ruby, makes its test's record
  incomplete; an incomplete test's file is listed in `Index.Unordered`, so it is never excluded (`LPCV-V0-954`).
- Records are stored in the existing `linecov-index/2` format. A line block is `Start=End=L`. A method block keeps
  its columns (shifted by one), so `StartCol>0` marks a method.
- Mapping by line covers only an edit strictly inside one method body (between its `def` and `end` lines).
  Any other edit is a fallback, because Ruby runs class and module bodies at load time: a non-Ruby or added
  file, a test helper, a wrong blob, or a change outside a method (`LPCV-V0-953`).
  An edit inside a method selects every test of the innermost method containing it, not the tests of the
  changed line: Ruby counts a `when X then` line 0 even when its condition ran, so per-line ownership
  would miss tests.
- Every recorded test file is listed in `Index.Unordered`, so every plan runs whole. The collector has only
  the in-order run, and a test can depend on state an earlier test built (a memoized value, a reused
  subprocess) without running that code. The isolated pass of `LPCV-V0-962` (V1-4415) is what can lift this.
- The JSON is hand-encoded so the collector never activates a `json` gem version different from the bundle's.

## Evidence

- Mac Ruby 2.6.10 (minitest 5.11.3, test-unit 3.2.9): `TestLPCVV0955LiveMinitestPerTest` and
  `TestLPCVV0955LiveTestUnitPerTest` pass.
- Docker `corvint-rspec-qual:2026-10-20` (Ruby 3.4.11, rspec 3.13.6): `TestLPCVV0955LiveRSpecThroughDocker`
  passes.
- `TestLPCVV0954DumpJoinAndIncompleteness` and `TestLPCVV0953RubyFallbacks` pass.
- `make plugin-test PLUGIN=ruby` passed (rc=0). It ran on the remote heavy host, where the live tests skip
  when Ruby or Docker is missing.

## Not done

Fault-injection qualification (`LPCV-V0-963`) and the real-repository shadow were not run (`NOT_RUN`).
Wiring the index into a selection command and the isolated-run divergence check of `LPCV-V0-962` are also not done. Children that clear their environment, and
non-Ruby children, are not covered; they are recorded as incomplete only when they leave a partial dump.
These are filed as follow-up tickets.

The independent review's remaining exclusion risks are recorded on V1-4415, and none is reachable while every
plan runs whole: a syntax or structural edit in a method no test called, `Plan` taking test names from the
index rather than the head, a fork that skips `at_exit`, repository files run through `load` or `eval`, and
data-driven test-unit tests that share a name. Rails `parallelize` workers are not recorded either.
