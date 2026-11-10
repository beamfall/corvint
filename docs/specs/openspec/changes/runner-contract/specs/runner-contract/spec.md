# Spec delta: runner-contract

The lane copies each statement below into `runner-contract-v1.md`, inside its `## Requirements`
section, as a numbered definition line that begins `(proposed 2026-11-09; delivery not-started)`.

## ADDED Requirements

### Requirement: RNC-V1-001 One compiled-in contract

Source: G23, owner decision R14 (2026-11-09). Every blocking runner MUST be driven through one Go
interface compiled into `corvint`, derived from `registry.RunnerPlugin`, and MUST NOT need an
external runner protocol or a separate executable to be selected and run. Once a runner's
implementation lands, its provider binary is retired (RNC-V1-017).

#### Scenario: Release binary alone

- **WHEN** `corvint test` runs a selection containing Go, Jest and pytest tests
- **THEN** all three are driven through the same interface inside the `corvint` binary

### Requirement: RNC-V1-002 Discovery is explicit and pinned

Source: owner answer 4, G2, owner decision R10 (2026-11-09), review T-B5. Each runner MUST supply a
lister, marked `static` or `executing`, to `corvint inventory refresh`, which writes the one
committed inventory under `.corvint/` (change `test-inventory`). The inventory MUST hold a pinned
list of canonical test IDs bound to the tree. `corvint affected` and `corvint watch` MUST NOT run a
lister or any project code; they read the committed inventory. RSpec and Minitest use static
classification. When the inventory is absent, the runner's declared full command runs with reason
`inventory-absent`; when it is stale, all tests run with reason `inventory-stale`.

#### Scenario: Plan read

- **WHEN** `corvint affected` plans a change in a pytest project
- **THEN** it starts no child process

#### Scenario: Inventory absent

- **WHEN** the repository has no committed inventory
- **THEN** each runner runs its declared full command and the receipt names `inventory-absent`

### Requirement: RNC-V1-003 Selection is exact or labelled not exact

Source: G2, owner decision R16 (2026-11-09). Each runner MUST turn a selection into argv that runs
exactly the selected tests, recorded `exact: true`, or a larger set recorded `exact: false` with a
one-line reason. The selection's tier is one of `line | file | flow | full`.

#### Scenario: Vitest directory run

- **WHEN** the Vitest implementation can only run a whole directory
- **THEN** the run records `exact: false` and the reason

### Requirement: RNC-V1-004 All seven runners run through `corvint test`

Source: G1, owner answer 2, owner decisions R7 and R32 (2026-11-09). `corvint test` MUST select and
run Go, Vitest, Jest, Playwright, pytest, RSpec and Minitest tests through the contract, and a
runner without a working implementation MUST run its declared full command with a printed reason.
A changed path that no runner instance owns, or a runner that has been removed or is unavailable,
MUST make the plan run all tests with a named reason (`runner-removed` for a removed or unavailable
runner); `UNOWNED_DIRTY_PATH` MUST NOT be an allowlisted pass.

#### Scenario: Minitest without implementation

- **WHEN** the Minitest implementation is not available
- **THEN** the declared Ruby command runs in full and the output names the reason

#### Scenario: Runner removed

- **WHEN** a commit deletes the Jest configuration that the inventory names
- **THEN** every runner runs all tests and the receipt names `runner-removed`

### Requirement: RNC-V1-005 Per-test results from machine reports

Source: G23, owner answer 8 (review T-m1 replaces the earlier G24 citation), owner decisions R7 and
R19 (2026-11-09). Each runner MUST record an outcome and a duration per selected test, keyed by
canonical test ID, from the runner's machine report, and per file only where the report has no
finer unit, marked with its granularity. Outcomes feed the run result record PROD-TEST-020 (change
`run-evidence`); durations follow change `per-test-durations`.

#### Scenario: Jest JSON report

- **WHEN** a Jest run writes its JSON report
- **THEN** each selected test title has its own outcome and duration in the `corvint test` receipt

### Requirement: RNC-V1-006 Missing results are `NOT_RUN`

Source: invariant 2. A selected test absent from the runner's results MUST be `NOT_RUN`, and the run
MUST be `INCOMPLETE`.

#### Scenario: Test filtered away

- **WHEN** a selected RSpec example does not appear in the JSON report
- **THEN** it is `NOT_RUN` and the run is `INCOMPLETE`

### Requirement: RNC-V1-007 Runners supply kind signals

Source: G13. Each runner MUST report the kind it knows (`unit`, `integration`, `e2e`) as a
`kindSource: runner` signal for the plan; kind is reporting only.

#### Scenario: Playwright

- **WHEN** the Playwright implementation reports its tests
- **THEN** each carries kind `e2e` with `kindSource: runner`

### Requirement: RNC-V1-008 Kill switch per runner on every surface

Source: G10, owner decision R13 (2026-11-09), review T-M6. `CORVINT_SELECTION=full`, set globally
or as `full:<runner>[,<runner>]`, MUST make each named runner run its full command on every instance
of that runner, and the run MUST say so. The grammar, the same in the Action, is defined once by
NMS-V1-008 in change `narrowing-measurement`; the contract reads the parsed value and defines no
grammar of its own. The rule applies on five surfaces, `corvint affected`, `corvint test`,
`corvint watch`, the GitHub Action and agent hooks, and on the MCP tools. An unknown value MUST exit
2 with a clear message on the CLI surfaces and in the Action, and MUST run all tests in agent hooks
and MCP.

#### Scenario: Jest only

- **WHEN** the kill switch is set for Jest only
- **THEN** Jest runs in full on every Jest instance and the other runners stay narrowed

#### Scenario: Unknown value on the CLI

- **WHEN** `CORVINT_SELECTION=fulll` and the user runs `corvint test`
- **THEN** it exits 2 with a message naming the accepted forms and runs nothing

#### Scenario: Unknown value in an agent hook

- **WHEN** an agent hook asks for the affected tests with `CORVINT_SELECTION=fast`
- **THEN** the answer selects all tests and names the unknown value

#### Scenario: MCP tool

- **WHEN** the `corvint.affected` MCP tool is called with `CORVINT_SELECTION=full:pytest`
- **THEN** every pytest test is selected and the other runners stay narrowed

### Requirement: RNC-V1-009 External protocol removed

Source: G23. `internal/runnerprotocol`, `internal/runnerpolicy`, `internal/runnerqualification` and
`tools/runner-*` MUST be removed, and no remaining package may import them.

#### Scenario: Build after removal

- **WHEN** the repository builds after the change
- **THEN** none of the six paths exists and `go vet ./...` passes

### Requirement: RNC-V1-010 Jest proven end to end

Source: G23. A fixture repository with Jest tests MUST show `corvint affected` selecting a Jest test
and `corvint test` running exactly that test with a per-test outcome.

#### Scenario: One Jest test selected

- **WHEN** a change touches code reached by one Jest test
- **THEN** only that test runs and the receipt records its outcome

### Requirement: RNC-V1-011 Tier and fallback printed and recorded

Source: G1, owner decision R16 (2026-11-09). Each run MUST print each runner's tier (`line`,
`file`, `flow` or `full`), its `exact` flag and every fallback reason, and the release manifest
MUST record each runner's tier and qualification limits. The earlier tier names `exact`,
`superset` and `full` are replaced.

#### Scenario: Release manifest

- **WHEN** a release is built
- **THEN** the manifest lists all seven runners with their tier and any fallback

### Requirement: RNC-V1-012 Canonical test ID

Source: owner decision R11 (2026-11-09), review T-B4. Every test of every runner MUST be identified
by the canonical test ID defined by CTI-V1-001 to CTI-V1-012 in change `canonical-test-id`, and the
same ID MUST be used in the plan, the receipt, flow links, run and line evidence, and watch.
RNC-V1-012 adds no rule of its own.

#### Scenario: Same ID end to end

- **WHEN** a pytest test is selected, run and reported
- **THEN** the plan, the receipt and the run result record carry the same canonical test ID

### Requirement: RNC-V1-013 Full command declared by the adopter, with a built-in default

Source: owner answer 2, invariant 2 (testing M7), owner decision R15 (2026-11-09). The full command
of each runner, used for every FULL fallback, MUST be read from `.corvint/test-commands.json` when
that file has an entry for the runner, and otherwise MUST be the built-in default for that runner.
`corvint test` MUST NOT read the full command from `.github/cishards/test-commands.json` in an
adopter repository. For one release, Corvint's own repository MAY still be read from the old path,
and only when `.corvint/test-commands.json` is absent; a notice MUST be printed when the old path is
used. The built-in defaults (from public runner documentation; NOT_VERIFIED, each pinned by a
fixture test) are: Go `go test ./...`; Vitest `npx vitest run`; Jest `npx jest`; Playwright
`npx playwright test`; pytest `python -m pytest`; RSpec `bundle exec rspec`; Minitest
`bin/rails test` when `bin/rails` exists, else `bundle exec rake test`. The file is keyed by runner
name; runner instances keyed by root (RNC-V1-014) extend the file with instance entries without
changing the runner-name entries, and an instance with no entry uses its runner's entry run from the
instance root.

#### Scenario: Adopter with no declaration

- **GIVEN** a pytest repository with no `.corvint/test-commands.json` and no `.github/cishards/`
- **WHEN** the plan for a change is FULL
- **THEN** `corvint test` runs `python -m pytest` and prints that the built-in default was used

#### Scenario: Declared command wins

- **GIVEN** `.corvint/test-commands.json` declares `uv run pytest -p no:cacheprovider` for pytest
- **WHEN** the plan is FULL
- **THEN** that declared argv runs, not the built-in default

#### Scenario: Old CI path ignored for adopters

- **GIVEN** an adopter repository that has only `.github/cishards/test-commands.json`
- **WHEN** the plan is FULL
- **THEN** the built-in default runs and the old file is not read

Failure mode: the file is malformed or names an unknown profile: exit 2 with a typed error, no
test runs (never a silent default).

### Requirement: RNC-V1-014 Runner instances keyed by root

Source: owner decision R15 (2026-11-09), review T-M8. A runner instance MUST be identified by
runner name, root directory (repository-relative) and config file. A repository MAY have several
instances of one runner (for example pnpm workspaces with several Vitest or Jest configs, several
Playwright projects, `go.work` modules, Rails engines, several pytest rootdirs). Receipts, the
inventory and watch answers MUST name the instance. The kill switch stays keyed by runner name and
applies to every instance of that runner. No new cross-package analysis is added.

#### Scenario: Two pytest rootdirs

- **GIVEN** pytest rootdirs `services/a` and `services/b`
- **WHEN** a change reaches tests in `services/a` only
- **THEN** only the `services/a` instance runs, and the receipt names that instance

### Requirement: RNC-V1-015 Deterministic sharding for the Action

Source: owner decision R15 (2026-11-09), review T-M9. For every runner, the contract MUST provide a
deterministic partition of a selection into N shards, so that the same plan and N give the same
shards on every machine. Shards of non-Go runners are used by the GitHub Action only (change
`github-action`); this change adds no other sharding mode.

#### Scenario: Same shards twice

- **WHEN** the same plan is split into 4 shards twice
- **THEN** both splits assign every canonical test ID to the same shard

### Requirement: RNC-V1-016 Argument length bounded

Source: review T-M9. When a runner's argv would exceed the operating system's argument limit, the
implementation MUST pass the selection through an argument file if the runner accepts one, and
otherwise MUST widen the selection to file level (`tier: file`, `exact: false`) with a reason; if
still over the limit it MUST run the runner's full command with a reason. It MUST NOT drop tests.

#### Scenario: Thousands of RSpec examples

- **WHEN** a selection holds 20,000 RSpec `path:line` arguments
- **THEN** the run uses an argument file or widens to files, and the receipt names the reason

### Requirement: RNC-V1-017 Provider binaries retired

Source: owner decision R14 (2026-11-09), review T-M1. After a runner's implementation of this
contract lands, its provider binary (`corvint-go-test-provider`, `corvint-js-test-provider` or
`corvint-ruby-test-provider`) MUST be removed from `tools/release-binary/binaries.json` and folded
into `corvint`; none of them ships as a separate 1.0 artifact.

#### Scenario: Ruby contract landed

- **WHEN** the RSpec and Minitest implementations land
- **THEN** `binaries.json` has no `corvint-ruby-test-provider` entry and `corvint test` runs RSpec
  without it

### Requirement: RNC-V1-018 Shell provider status

Source: final decision round 2026-11-09 under R42, M11; `source-removal` RMV-V1-015 keeps the
shell provider. The shell provider MUST be kept as a built-in provider outside the qualified
runner set. The support table MUST list it as `kept, not qualified`. Its results MUST NOT count
toward a 1.0 no-miss or narrowing claim. Any change it cannot map with certainty MUST select
every shell test, with tier `full`, `exact: false` and a one-line reason. This requirement does
not change the list of seven blocking runners. Whether `kept, not qualified` matches the label
vocabulary of the support table is NOT_VERIFIED; the lane checks it when it writes the table.

#### Scenario: Shell change that cannot be mapped

- **GIVEN** a change to a shell script that the shell provider cannot map to tests with certainty
- **WHEN** `corvint affected` plans the change
- **THEN** every shell test is selected with tier `full` and a reason

#### Scenario: Shell results in a release claim

- **GIVEN** a run whose shell tests passed
- **WHEN** the release manifest states the 1.0 no-miss and narrowing claims
- **THEN** the shell provider is listed as `kept, not qualified` and its results are not counted
