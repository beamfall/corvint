# Spec delta: ruby-providers

The lane copies each statement below into `ruby-providers-v1.md`, inside its `## Requirements`
section, as a numbered definition line that begins `(proposed 2026-11-09; delivery not-started)`.

## ADDED Requirements

### Requirement: RBR-V1-001 Discovery for both frameworks

The provider MUST supply a `static` lister to `corvint inventory refresh` (change `test-inventory`,
owner decision R10 (2026-11-09)) that inventories RSpec examples and Minitest test methods with
their files, using the framework classification in `internal/liveverify/affected/ruby`, keyed by
the canonical test ID (RNC-V1-012). Each Rails engine or separate Ruby root is its own runner
instance (RNC-V1-014, owner decision R15 (2026-11-09)). (G1, G23)

#### Scenario: Rails application

- **GIVEN** a Rails application with `test/` Minitest files and no `spec/`
- **WHEN** inventory runs
- **THEN** every `test_` method and `test "..."` block is listed under runner `minitest`

#### Scenario: Rails engine

- **GIVEN** a Rails application with an engine under `engines/billing/` that has its own `test/`
- **WHEN** inventory runs
- **THEN** the engine's tests are listed under a separate Minitest instance rooted at
  `engines/billing/`

### Requirement: RBR-V1-002 Selection granularity

RSpec and Minitest are line-exact runners (owner decision R8 (2026-11-09)). The provider MUST
narrow to the selected examples and test methods (tier `line`) once the Ruby line evidence passes
the FIH-V1 proof (change `line-evidence`); until then it MUST run the files the plan selected with
tier `file`. Each selection MUST carry a reason. (answer 3, G2)

#### Scenario: File-level before the proof

- **GIVEN** no passed FIH-V1 verdict for Ruby line evidence
- **WHEN** a plan selects one example
- **THEN** its whole spec file runs with tier `file`

#### Scenario: Line-level after the proof

- **GIVEN** a passed FIH-V1 verdict for RSpec line evidence
- **WHEN** a plan selects one example
- **THEN** only that example runs with tier `line`

### Requirement: RBR-V1-003 Run RSpec through `corvint test`

`corvint test` MUST run RSpec selections through the compiled-in contract via Bundler, with an
optional execution prefix, without an external provider executable. (G23)

#### Scenario: Docker prefix

- **GIVEN** a declared execution prefix `docker compose run --rm app`
- **WHEN** `corvint test` runs an RSpec selection
- **THEN** RSpec runs inside the prefix and the receipt records the prefix

### Requirement: RBR-V1-004 Run Minitest through `corvint test`

`corvint test` MUST run Minitest selections, using `bin/rails test FILES` when the repository is a
Rails application and the Minitest runner otherwise. (G1, G23; the exact invocation for non-Rails
repositories is NOT_VERIFIED and is recorded in the spec before code)

#### Scenario: Rails application

- **GIVEN** a Rails application with `bin/rails` and a selection in `test/models/`
- **WHEN** `corvint test` runs it
- **THEN** it runs `bin/rails test` with the selected files and the receipt records each test

#### Scenario: activesupport (non-Rails path)

- **GIVEN** the rails/rails pin and a selection in `activesupport/test/`, which has no `bin/rails`
- **WHEN** `corvint test` runs it
- **THEN** the non-Rails Minitest invocation runs only the selected files and the receipt records
  each test

### Requirement: RBR-V1-005 Per-test results

The receipt MUST record per test, keyed by canonical test ID, the outcome (passed, failed, error,
skipped, pending) and its duration as additive members of `corvint-test-run/0`; outcomes feed the
run result record PROD-TEST-020 (change `run-evidence`, owner decision R7 (2026-11-09)) and
durations follow change `per-test-durations`. (G4, G23) For Minitest the outcome MUST come from
Corvint's compiled-in Ruby hook, with `error` taken from `result.error?` (D9).

#### Scenario: Minitest exception

- **GIVEN** a Minitest test that raises an exception
- **WHEN** the run ends
- **THEN** the hook records it as `error`, not `failed`

#### Scenario: Pending example

- **GIVEN** a pending RSpec example
- **WHEN** the run ends
- **THEN** it is recorded as pending and does not make the run FAIL

### Requirement: RBR-V1-006 Kind label

Each test MUST carry `kind` and `kindSource`: e2e for RSpec `type: :system` or `type: :feature`
specs run with `js: true` or another browser driver, and for system tests driven by a browser; not
e2e for Capybara `rack_test` (G13); integration when owner-declared; unit otherwise; UNKNOWN on
conflict. It MUST NOT change selection. (G13, review T-m11)

#### Scenario: rack_test system spec

- **GIVEN** a `type: :system` spec using `driven_by :rack_test`
- **WHEN** inventory runs
- **THEN** its kind is not e2e

#### Scenario: Feature spec with a browser driver

- **GIVEN** a `type: :feature, js: true` spec using a browser driver
- **WHEN** inventory runs
- **THEN** its kind is e2e

### Requirement: RBR-V1-007 Fail closed on runner trouble

Missing tools, zero executed selections, invalid documents or a dead execution prefix MUST produce
a non-PASS result with a typed reason. Selected tests absent from the results are `NOT_RUN` and the
run is `INCOMPLETE` (RNC-V1-006, review T-m2). (G1, G10)

#### Scenario: Missing gem

- **GIVEN** a Gemfile without rspec
- **WHEN** an RSpec selection runs
- **THEN** the result is `runner-unavailable`, exit 2

#### Scenario: Zero examples executed

- **GIVEN** a selected spec file that no longer holds examples
- **WHEN** `corvint test` runs it
- **THEN** the selected examples are `NOT_RUN` and the run is `INCOMPLETE`

### Requirement: RBR-V1-008 Existing provider output unchanged until retired

Until `cmd/corvint-ruby-test-provider` is retired under RNC-V1-017 (owner decision R14
(2026-11-09)), its `corvint-ruby-test-provider/0` output, flags and exit codes MUST stay
byte-compatible. (G24)

#### Scenario: Retained document

- **GIVEN** a document retained before this change
- **WHEN** read after it
- **THEN** it parses with the same meaning

### Requirement: RBR-V1-009 Advisory Minitest command

`corvint affected` MUST print an advisory Minitest command for Minitest selections, and it MUST
name exactly the tests `corvint test` runs; the same MUST hold for the RSpec command. (G23)

#### Scenario: Parity

- **GIVEN** one plan with RSpec and Minitest selections
- **WHEN** advice and run are compared
- **THEN** both lists match per runner

### Requirement: RBR-V1-010 Environment, kill switch and fallbacks

The receipt MUST record Ruby, Bundler and framework versions and the `Gemfile.lock` hash; the
provider MUST honour `CORVINT_SELECTION` (grammar owned by NMS-V1-008, owner decision R13
(2026-11-09)) on every instance of the named runner and print every fallback. (G1, G10)

#### Scenario: Per-runner switch

- **GIVEN** `CORVINT_SELECTION=full:minitest`
- **WHEN** a narrowed plan runs
- **THEN** Minitest runs in full with reason `kill-switch`

### Requirement: RBR-V1-011 Harness adapter

Both runners MUST expose the FIH-V1-008 call: run named tests at a commit and return per-test
outcomes. (G3, G23)

#### Scenario: Minitest harness call

- **GIVEN** a fault case naming two Minitest tests
- **WHEN** the harness calls the adapter
- **THEN** it receives two outcomes

### Requirement: RBR-V1-012 Ruby provider binary retired

Source: owner decision R14 (2026-11-09). When RBR-V1-003 and RBR-V1-004 land,
`cmd/corvint-ruby-test-provider` MUST be removed from `tools/release-binary/binaries.json` under
RNC-V1-017, and its RSpec behaviour MUST be available through `corvint test`.

#### Scenario: Release after both runners land

- **WHEN** a release is built after RBR-V1-003 and RBR-V1-004 land
- **THEN** it ships no `corvint-ruby-test-provider` binary and `corvint test` runs RSpec and
  Minitest
