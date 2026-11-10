# Change proposal: RSpec and Minitest discovery, selection, run and result reporting through `corvint test`

Change name: `ruby-providers`
Ticket: none filed yet (task 1 files it in `.taskman/`, milestone `v1-0`)
Product requirement served: PROD-TEST-006, PROD-TEST-008 and PROD-TEST-011 in
`docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: `docs/specs/ruby-providers-v1.md`, a new spec with its own prefix `RBR-V1`;
selection rules stay in `docs/specs/ruby-affected-test-adapter-v0.md` (proposed/experimental)
Requirement IDs this change adds: RBR-V1-001 to RBR-V1-012
Owner answers this change implements: answer 3, G1, G2, G4, G10, G13, G23, G26
(`owner-answers-2026-11-09.md`), and owner decisions R7, R8, R10, R11, R13, R14 and R15
(2026-11-09)
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. No lane has been started for this change.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/ruby-providers/spec.md`).

## Why

RSpec and Minitest both block 1.0 (G1); they publish narrowing numbers with no bar (G4). Owner
decision R8 (2026-11-09) makes both line-exact runners. Today `corvint affected` can select Ruby
tests statically, an experimental RSpec-only provider binary exists outside `corvint test`, and the
compiled-in Ruby runner code has no caller. G23 asks for one compiled-in runner contract for all
seven runners and partly supersedes decision 0592 (RSP-V0-025/026).

### What exists today (verified in `integ` at 9ec3a29d0)

- Selection. `internal/liveverify/affected/ruby/` (ruby.go, autoload.go, frontier.go, nesting.go)
  recognises both frameworks: `frameworkMinitest` at `ruby.go:52`, the Minitest class pattern
  (`Minitest::Test`, `ActiveSupport::TestCase`, `ActionDispatch::IntegrationTest`,
  `ApplicationSystemTestCase` and others) at `ruby.go:69`, and `def test_` methods at `ruby.go:70`.
  `cmd/corvint/affected.go:1853` calls `advisoryRSpecCheck` (`:2113-2117`, AFP-V0-066), which prints
  a Bundler RSpec command. There is no advisory Minitest command (no match for "minitest" in the
  non-test files `cmd/corvint/affected*.go`).
- Provider binary. `cmd/corvint-ruby-test-provider/main.go:1-14` is "the experimental RSpec provider"
  (LPCV-V0-090..095, 131..138 in `live-proof-carrying-verification-v0.md`). Its only subcommand is
  `rspec --dir` (usage at `:35`). It runs RSpec once with its JSON formatter or the shipped
  `corvint-rspec-json/0` formatter, can run through `--exec-prefix` (for example
  `docker compose run --rm app`), prints a `corvint-ruby-test-provider/0` document, retains it under
  `.corvint/test-evidence` with `--retain`, and exits 2 on refusal and 1 on infrastructure or
  retention failure; a failed example exits 0. Its core is `internal/rubytestprovider`, whose
  non-test files never mention Minitest. **The audit claim that this binary covers Minitest is not
  supported by the code; it is RSpec only.** It is not wired into `corvint test`.
- Compiled-in runner with no caller. `internal/testrunner/registry/runner_plugin.go:83-91`
  (`SelectionPlugin`) returns a `RubyPlugin` for `rspec`, `minitest`, `test-unit` and `rails-test`;
  `registry/ruby_plugin.go:45` (`NewRubyPlugin`) builds it on `internal/rubylinecov`, which handles
  RSpec, Minitest and test-unit (`rubylinecov/collect.go:5`, `:288`; `select.go:13`).
  `internal/testrunner/dynamic/ruby_selection.go:19,57` handles the same four runner names. A
  repository-wide search finds no non-test caller of `SelectionPlugin` or `NewRubyPlugin`.
- Run. `corvint test` admits only `corvint-js-test-provider` with runners `playwright` and `vitest`
  (`internal/sessiontest/declared.go:40-55`).
- Mutation. `internal/liveverify/rbmutate` (FPK-V0-068..081, RSpec only) has no importers outside
  itself and is removed by G26.
- Supporting packages named by the audit and present: `internal/rubydialect`,
  `internal/rubylinecov`, `internal/rubyautoload`, `internal/rubyprofile`,
  `conformance/ruby-profile-v0`. Their completeness for Minitest is NOT_VERIFIED.

### What is missing

- `corvint test` cannot run RSpec or Minitest.
- No Minitest provider of any kind; no advisory Minitest command.
- No per-test outcome or duration in a `corvint test` receipt for either runner.
- No test-kind label (G13), no FIH-V1 adapter, no NMS-V1 measurement.
- Two output shapes (`corvint-ruby-test-provider/0` and the registry types) where G23 wants one.

## What Changes

- Supply a `static` Ruby lister to `corvint inventory refresh` (owner decision R10 (2026-11-09)).
- Wire the compiled-in Ruby runner into `corvint test` for RSpec and Minitest (including
  `rails test`), on the single G23 contract, at tier `file` until the Ruby line evidence passes
  FIH-V1 and at tier `line` after (owner decision R8 (2026-11-09)).
- Keep `corvint-ruby-test-provider/0` output byte-compatible while the binary exists (G24 keeps
  provider output formats), and retire the binary once the compiled-in path lands (owner decision
  R14 (2026-11-09); this answers the earlier open question).
- Add an advisory Minitest command next to the RSpec one.
- Minitest per-test results come from Corvint's compiled-in Ruby hook (`MinitestHook#run` in
  `internal/rubylinecov`, which at integ `9ec3a29d0` already maps each result to passed, failed or
  pending keyed by `Class#method` and source file), extended with `result.error?` so an exception is
  recorded as `error`, not `failed` (settled without the owner, D9). `minitest-reporters` and JUnit
  XML are not used.

## Capabilities

### New Capabilities

- `ruby-providers`: discovery, run and result reporting for RSpec and Minitest.

### Modified Capabilities

- `session-test-runner`: admits RSpec and Minitest through the compiled-in contract.
- `ruby-affected-test-adapter`: unchanged selection; adds the run link.

## Non-goals

- No test-unit support claimed for 1.0 beyond what the shared code already does.
- No change to Ruby static selection rules.
- No line-evidence narrowing of browser E2E (RSpec `type: :system` or `js: true` with a browser
  driver) (G2); Capybara `rack_test` is not e2e (G13).
- No revival of rbmutate (G26).

## Impact

- `cmd/corvint`, `internal/sessiontest`, `internal/testrunner/registry`, `internal/rubylinecov`,
  `tools/release-binary/binaries.json` (Ruby provider entry removed).
- Must not depend on the external protocol packages removed by G23.
