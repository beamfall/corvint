# Runner contract V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: partial
Health: KEEP
Authoritative inputs: change folder `docs/specs/openspec/changes/runner-contract/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/runner-contract/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-006, PROD-TEST-011) and `docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md`
(owner decisions R6, R7, R10, R11, R13 to R16, R19, R32 and R38).

## Agent digest
- Claim: One compiled-in contract drives every blocking runner by canonical test ID and runner instance; every fallback runs a full command with a reason.
- Status: accepted / partial. Change `runner-contract` auto-accepted under G14 (decision 0596); tasks 1 to 4, 6 and 9 delivered under V1-5416; task 5 (but Minitest) and part of task 7 under V1-5443.
- Exists: `internal/testrunner/contract` (Go, Vitest, Playwright, Jest, pytest and RSpec select and parse; Minitest runs full); the `corvint test` `contract` adapter running narrowed Vitest, Jest, Playwright, pytest and RSpec selections with per-test receipt results (RNC-V1-019); `internal/testrunner/killswitch` (NMS-V1-008 grammar) on `corvint test`, `affected` and `watch`; `.corvint/test-commands.json` with built-in defaults; `unowned-path` and `runner-removed` FULL plans.
- Blocked on: Minitest selection and parser (task 5); the kill switch in agent hooks, MCP, the Action and a widened `affected` plan (task 7); tasks 8, 10, 11, 12; deleting Corvint's old declaration edits `.github/`.
- Read next: Requirements; Surfaces; Traceability.

## User and boundary

An agent, a developer or the GitHub Action runs the tests a change selects. Before this change
`corvint test` ran Go through its own adapter, Vitest and Playwright through the separate
`corvint-js-test-provider` binary, and every other runner only through a language-wide command
declared in Corvint's internal CI file `.github/cishards/test-commands.json`; an adopter without
that file got an `INCOMPLETE` FULL run. The compiled-in `registry.RunnerPlugin` was used only by
`corvint-test-runner`.

This spec is the canonical home of requirements RNC-V1-001 to RNC-V1-019, copied from the change's
`specs/runner-contract/spec.md`. Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds only the delivery record (Surfaces, Traceability).

## Requirements

- `RNC-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) One compiled-in contract

Source: G23, owner decision R14 (2026-11-09). Every blocking runner MUST be driven through one Go
interface compiled into `corvint`, derived from `registry.RunnerPlugin`, and MUST NOT need an
external runner protocol or a separate executable to be selected and run. Once a runner's
implementation lands, its provider binary is retired (RNC-V1-017).
Delivery: partial. The interface (`internal/testrunner/contract`) and its seven implementations are
compiled in, `corvint test` resolves every runner instance's full command through it, and its
`contract` adapter runs narrowed Vitest, Jest, Playwright, pytest and RSpec selections through it
(RNC-V1-019). Not yet built: Go still runs through the STR-V0 `go` adapter, and a repository that
declares a `providers` entry still runs Vitest and Playwright through `corvint-js-test-provider`
(task 11).

**Scenario: Release binary alone**
- WHEN `corvint test` runs a selection containing Go, Jest and pytest tests
- THEN all three are driven through the same interface inside the `corvint` binary

- `RNC-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Discovery is explicit and pinned

Source: owner answer 4, G2, owner decision R10 (2026-11-09), review T-B5. Each runner MUST supply a
lister, marked `static` or `executing`, to `corvint inventory refresh`, which writes the one
committed inventory under `.corvint/` (change `test-inventory`). The inventory MUST hold a pinned
list of canonical test IDs bound to the tree. `corvint affected` and `corvint watch` MUST NOT run a
lister or any project code; they read the committed inventory. RSpec and Minitest use static
classification. When the inventory is absent, the runner's declared full command runs with reason
`inventory-absent`; when it is stale, all tests run with reason `inventory-stale`.
Delivery: partial. Each runner names the inventory's own lister and kind, and `corvint affected`
starts no runner or lister process. An instance whose committed inventory record is absent, stale or
unreadable runs its full command, with the verdict's reason, instead of a contract selection
(RNC-V1-019). Not yet built: selection from the inventory's listing (the plan's own test files are
what a contract unit runs), and running every runner's tests, Go included, on `inventory-stale`
(only the stale instance runs in full).

**Scenario: Plan read**
- WHEN `corvint affected` plans a change in a pytest project
- THEN it starts no child process

**Scenario: Inventory absent**
- WHEN the repository has no committed inventory
- THEN each runner runs its declared full command and the receipt names `inventory-absent`

- `RNC-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Selection is exact or labelled not exact

Source: G2, owner decision R16 (2026-11-09). Each runner MUST turn a selection into argv that runs
exactly the selected tests, recorded `exact: true`, or a larger set recorded `exact: false` with a
one-line reason. The selection's tier is one of `line | file | flow | full`.
Delivery: partial. The contract records tier, `exact` and reason for Go, Vitest and Playwright
selections, and for Jest, pytest and RSpec; `corvint test` prints and records them for every
contract run (RNC-V1-019). Not yet built: the release manifest's per-runner tier (task 12).

**Scenario: Vitest directory run**
- WHEN the Vitest implementation can only run a whole directory
- THEN the run records `exact: false` and the reason

- `RNC-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) All seven runners run through `corvint test`

Source: G1, owner answer 2, owner decisions R7 and R32 (2026-11-09). `corvint test` MUST select and
run Go, Vitest, Jest, Playwright, pytest, RSpec and Minitest tests through the contract, and a
runner without a working implementation MUST run its declared full command with a printed reason.
A changed path that no runner instance owns, or a runner that has been removed or is unavailable,
MUST make the plan run all tests with a named reason (`unowned-path` for an unowned path,
`runner-removed` for a removed or unavailable runner). A path a Go selection rule attributes (an
enclosing package or embed, a package whose files name it, a declared read scope or declared reader
that bears on it, a selection the plan makes through a path-literal, path-glob, declared-read-scope,
enclosing-package or declared-consumer witness naming the path, or a nested module whose own suite
runs it) is owned by that Go instance and MUST NOT force a full run under this rule; a selection
through any other witness, such as an unbounded reader witnessed by whichever dirty path sorts first,
attributes no path;
`UNOWNED_DIRTY_PATH` MUST NOT be an allowlisted pass for any other path (orchestrator ruling
2026-11-11 on the R32 scope).
Delivery: partial. A runner with no implementation runs its full command with
`runner-not-implemented`; a language with no declared command runs each runner instance's full
command; an unowned path no Go rule attributes plans FULL with `unowned-path`, an attributed one
narrows to its packages, and a removed runner plans FULL with `runner-removed`. `corvint test`
runs the selected Vitest, Jest, Playwright, pytest and RSpec files through the contract
(RNC-V1-019); Minitest runs its full command with the reason. Not yet built: Minitest selection
(task 5), and widening every runner for one whose binary is unavailable (its units are `NOT_RUN`
and the run `INCOMPLETE`).

**Scenario: Minitest without implementation**
- WHEN the Minitest implementation is not available
- THEN the declared Ruby command runs in full and the output names the reason

**Scenario: Runner removed**
- WHEN a commit deletes the Jest configuration that the inventory names
- THEN every runner runs all tests and the receipt names `runner-removed`

- `RNC-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Per-test results from machine reports

Source: G23, owner answer 8 (review T-m1 replaces the earlier G24 citation), owner decisions R7 and
R19 (2026-11-09). Each runner MUST record an outcome and a duration per selected test, keyed by
canonical test ID, from the runner's machine report, and per file only where the report has no
finer unit, marked with its granularity. Outcomes feed the run result record PROD-TEST-020 (change
`run-evidence`); durations follow change `per-test-durations`.
Delivery: partial. The contract reads the Go, Vitest, Playwright, Jest and pytest machine reports
into one outcome and duration per canonical test ID, and RSpec's per file (its stock report names
no example group chain). A file that failed without a failing test is a file-level fail, and a
run-level failure in the report (Jest `success: false`, Playwright `errors`, an RSpec error outside
examples, a pytest internal error) is returned with the results. The `corvint test` receipt
carries them per contract invocation (RNC-V1-019). Not yet built: the Minitest parser (task 5).

**Scenario: Jest JSON report**
- WHEN a Jest run writes its JSON report
- THEN each selected test title has its own outcome and duration in the `corvint test` receipt

- `RNC-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Missing results are `NOT_RUN`

Source: invariant 2. A selected test absent from the runner's results MUST be `NOT_RUN`, and the run
MUST be `INCOMPLETE`.
Delivery: partial. `contract.Reconcile` marks every selected test the report omits `NOT_RUN` and the
run incomplete, and also marks the run incomplete when a package or file did not finish (a Go
package that panicked, timed out, ended with a started test or never ended) or the report was read
only in part; it keeps the most severe of duplicate results and marks the run not exact. The
`corvint test` contract adapter records its `NOT_RUN` results and makes the run `INCOMPLETE`
(RNC-V1-019); it reconciles each selected file and every test the committed inventory lists in it
while the file is unchanged (the manifest's `listed`), so a test the runner omits is `NOT_RUN` even
when a sibling in its file passed.

**Scenario: Test filtered away**
- WHEN a selected RSpec example does not appear in the JSON report
- THEN it is `NOT_RUN` and the run is `INCOMPLETE`

- `RNC-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Runners supply kind signals

Source: G13. Each runner MUST report the kind it knows (`unit`, `integration`, `e2e`) as a
`kindSource: runner` signal for the plan; kind is reporting only.
Delivery: partial. `contract.Kind` reports `e2e` for Playwright and no kind for the other runners,
and each Playwright contract unit of the `corvint test` manifest carries `kind: e2e` with
`kindSource: runner`. Not yet built: the signal in the frozen `corvint affected` plan.

**Scenario: Playwright**
- WHEN the Playwright implementation reports its tests
- THEN each carries kind `e2e` with `kindSource: runner`

- `RNC-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Kill switch per runner on every surface

Source: G10, owner decision R13 (2026-11-09), review T-M6. `CORVINT_SELECTION=full`, set globally
or as `full:<runner>[,<runner>]`, MUST make each named runner run its full command on every instance
of that runner, and the run MUST say so. The grammar, the same in the Action, is defined once by
NMS-V1-008 in change `narrowing-measurement`; the contract reads the parsed value and defines no
grammar of its own. The rule applies on five surfaces, `corvint affected`, `corvint test`,
`corvint watch`, the GitHub Action and agent hooks, and on the MCP tools. An unknown value MUST exit
2 with a clear message on the CLI surfaces and in the Action, and MUST run all tests in agent hooks
and MCP.
Delivery: partial. `internal/testrunner/killswitch` reads the NMS-V1-008 grammar (an empty value is
off). `corvint test` applies it: `full` plans FULL with reason `kill-switch`, `full:<runner>` runs
every instance of each named runner (Go: its whole universe and each nested module) in full and keeps the others narrowed,
the manifest and receipt record `killSwitch`, and stderr prints `corvint-test: KILL-SWITCH`. An
unknown value exits 2 with `kill-switch-value-unknown` and runs nothing on `corvint test`,
`corvint affected` and `corvint watch`. `corvint affected` names an active switch on stderr without
widening its frozen plan, and `corvint watch`, which answers only narrowed Go selections, refuses a
switch that sends Go to its full command with `kill-switch-full-unsupported`. Not yet built: the
FULL kill-switch state in the `corvint affected` plan (frozen output, CCF-V1-006), agent hooks, the
MCP tools and the GitHub Action.

**Scenario: Jest only**
- WHEN the kill switch is set for Jest only
- THEN Jest runs in full on every Jest instance and the other runners stay narrowed

**Scenario: Unknown value on the CLI**
- WHEN `CORVINT_SELECTION=fulll` and the user runs `corvint test`
- THEN it exits 2 with a message naming the accepted forms and runs nothing

**Scenario: Unknown value in an agent hook**
- WHEN an agent hook asks for the affected tests with `CORVINT_SELECTION=fast`
- THEN the answer selects all tests and names the unknown value

**Scenario: MCP tool**
- WHEN the `corvint.affected` MCP tool is called with `CORVINT_SELECTION=full:pytest`
- THEN every pytest test is selected and the other runners stay narrowed

- `RNC-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) External protocol removed

Source: G23. `internal/runnerprotocol`, `internal/runnerpolicy`, `internal/runnerqualification` and
`tools/runner-*` MUST be removed, and no remaining package may import them.
Delivery: not yet built (task 10).

**Scenario: Build after removal**
- WHEN the repository builds after the change
- THEN none of the six paths exists and `go vet ./...` passes

- `RNC-V1-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Jest proven end to end

Source: G23. A fixture repository with Jest tests MUST show `corvint affected` selecting a Jest test
and `corvint test` running exactly that test with a per-test outcome.
Delivery: delivered on a fixture Jest. In a fixture repository with two Jest test files and a
committed inventory, `corvint affected` selects the one file a change reaches and `corvint test`
runs only it through `npx jest --runTestsByPath` and records its test's outcome and duration; the
Jest executable is a fixture that writes Jest's documented `--json` report, since no live Jest is
installed on the lane host (NOT_RUN on Linux and macOS).

**Scenario: One Jest test selected**
- WHEN a change touches code reached by one Jest test
- THEN only that test runs and the receipt records its outcome

- `RNC-V1-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Tier and fallback printed and recorded

Source: G1, owner decision R16 (2026-11-09). Each run MUST print each runner's tier (`line`,
`file`, `flow` or `full`), its `exact` flag and every fallback reason, and the release manifest
MUST record each runner's tier and qualification limits. The earlier tier names `exact`,
`superset` and `full` are replaced.
Delivery: not yet built (task 12).

**Scenario: Release manifest**
- WHEN a release is built
- THEN the manifest lists all seven runners with their tier and any fallback

- `RNC-V1-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Canonical test ID

Source: owner decision R11 (2026-11-09), review T-B4. Every test of every runner MUST be identified
by the canonical test ID defined by CTI-V1-001 to CTI-V1-012 in change `canonical-test-id`, and the
same ID MUST be used in the plan, the receipt, flow links, run and line evidence, and watch.
RNC-V1-012 adds no rule of its own.
Delivery: partial. Contract commands take, and its results return, canonical test IDs (CTI-V1-001),
and the `corvint test` manifest units and receipt outcomes and results of a contract run carry them.
Not yet built: the run result record (change `run-evidence`).

**Scenario: Same ID end to end**
- WHEN a pytest test is selected, run and reported
- THEN the plan, the receipt and the run result record carry the same canonical test ID

- `RNC-V1-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Full command declared by the adopter, with a built-in default

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
Delivery: delivered. Corvint's own commands are at `.corvint/test-commands.json`, kept
byte-identical to the old `.github/cishards/test-commands.json` by `tools/test-commands-check` until
the readers move and the old file is deleted, which edits `.github/` (build log).

**Scenario: Adopter with no declaration**
- GIVEN a pytest repository with no `.corvint/test-commands.json` and no `.github/cishards/`
- WHEN the plan for a change is FULL
- THEN `corvint test` runs `python -m pytest` and prints that the built-in default was used

**Scenario: Declared command wins**
- GIVEN `.corvint/test-commands.json` declares `uv run pytest -p no:cacheprovider` for pytest
- WHEN the plan is FULL
- THEN that declared argv runs, not the built-in default

**Scenario: Old CI path ignored for adopters**
- GIVEN an adopter repository that has only `.github/cishards/test-commands.json`
- WHEN the plan is FULL
- THEN the built-in default runs and the old file is not read

Failure mode: the file is malformed or names an unknown profile: exit 2 with a typed error, no
test runs (never a silent default).

- `RNC-V1-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Runner instances keyed by root

Source: owner decision R15 (2026-11-09), review T-M8. A runner instance MUST be identified by
runner name, root directory (repository-relative) and config file. A repository MAY have several
instances of one runner (for example pnpm workspaces with several Vitest or Jest configs, several
Playwright projects, `go.work` modules, Rails engines, several pytest rootdirs). Receipts, the
inventory and watch answers MUST name the instance. The kill switch stays keyed by runner name and
applies to every instance of that runner. No new cross-package analysis is added.
Delivery: partial. Contract commands are keyed by instance and run from its root;
`.corvint/test-commands.json` takes instance entries; the manifest runs one
`declared:<runner>@<root>` unit per instance; a narrowed contract selection runs only the instances
that own a selected file, each in its own invocation from its root, and the receipt names the
instance (RNC-V1-019).

**Scenario: Two pytest rootdirs**
- GIVEN pytest rootdirs `services/a` and `services/b`
- WHEN a change reaches tests in `services/a` only
- THEN only the `services/a` instance runs, and the receipt names that instance

- `RNC-V1-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Deterministic sharding for the Action

Source: owner decision R15 (2026-11-09), review T-M9. For every runner, the contract MUST provide a
deterministic partition of a selection into N shards, so that the same plan and N give the same
shards on every machine. Shards of non-Go runners are used by the GitHub Action only (change
`github-action`); this change adds no other sharding mode.
Delivery: not yet built (task 8).

**Scenario: Same shards twice**
- WHEN the same plan is split into 4 shards twice
- THEN both splits assign every canonical test ID to the same shard

- `RNC-V1-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Argument length bounded

Source: review T-M9. When a runner's argv would exceed the operating system's argument limit, the
implementation MUST pass the selection through an argument file if the runner accepts one, and
otherwise MUST widen the selection to file level (`tier: file`, `exact: false`) with a reason; if
still over the limit it MUST run the runner's full command with a reason. It MUST NOT drop tests.
Delivery: partial (V1-5455, 2026-11-13). `contract.ArgvFits` bounds one argument below Linux's
`MAX_ARG_STRLEN` and the whole argv at 256 KiB. Jest takes no argument file, so an over-long `-t`
pattern widens the selection to its files (`tier: file`, `exact: false`, reason `argv-limit`) and
over-long file paths run the full command with that reason. Not yet built for the other runners
(task 8).

**Scenario: Thousands of RSpec examples**
- WHEN a selection holds 20,000 RSpec `path:line` arguments
- THEN the run uses an argument file or widens to files, and the receipt names the reason

- `RNC-V1-017`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Provider binaries retired

Source: owner decision R14 (2026-11-09), review T-M1. After a runner's implementation of this
contract lands, its provider binary (`corvint-go-test-provider`, `corvint-js-test-provider` or
`corvint-ruby-test-provider`) MUST be removed from `tools/release-binary/binaries.json` and folded
into `corvint`; none of them ships as a separate 1.0 artifact.
Delivery: not yet built (task 11).

**Scenario: Ruby contract landed**
- WHEN the RSpec and Minitest implementations land
- THEN `binaries.json` has no `corvint-ruby-test-provider` entry and `corvint test` runs RSpec
  without it

- `RNC-V1-018`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5416) Shell provider status

Source: final decision round 2026-11-09 under R42, M11; `source-removal` RMV-V1-015 keeps the
shell provider. The shell provider MUST be kept as a built-in provider outside the qualified
runner set. The support table MUST list it as `kept, not qualified`. Its results MUST NOT count
toward a 1.0 no-miss or narrowing claim. Any change it cannot map with certainty MUST select
every shell test, with tier `full`, `exact: false` and a one-line reason. This requirement does
not change the list of seven blocking runners. Whether `kept, not qualified` matches the label
vocabulary of the support table is NOT_VERIFIED; the lane checks it when it writes the table.
Delivery: not yet built (task 12).

**Scenario: Shell change that cannot be mapped**
- GIVEN a change to a shell script that the shell provider cannot map to tests with certainty
- WHEN `corvint affected` plans the change
- THEN every shell test is selected with tier `full` and a reason

**Scenario: Shell results in a release claim**
- GIVEN a run whose shell tests passed
- WHEN the release manifest states the 1.0 no-miss and narrowing claims
- THEN the shell provider is listed as `kept, not qualified` and its results are not counted

- `RNC-V1-019`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5443) `corvint test` runs narrowed contract selections

Source: RNC-V1-001, RNC-V1-004, RNC-V1-005 and RNC-V1-010 (G23, owner decisions R7, R14, R19), task
5. On the local profile, `corvint test` MUST run each selected Vitest, Jest, Playwright, pytest or
RSpec test file of a language `.corvint/test-commands.json` declares no command for through the
contract: one manifest unit (adapter `contract`) per file and owning instance, whose `unit` is the
file-level canonical test ID and whose `slice` is the instance key, and one invocation per instance,
run from its root, whose argv is the contract's narrowing of the instance's full command
(RNC-V1-013) with the machine report written to a fresh private file outside the repository. A
`python:<module>` unit MUST run the test files the plan lists for it. The receipt MUST record per
contract invocation the instance, tier, `exact` flag and reason, each unit's outcome under its
canonical test ID, and one result per test read from the report (outcome, duration, granularity),
with every selected test the report does not answer `NOT_RUN` and the run `INCOMPLETE`. A missing,
unreadable or oversized report, a contract refusal or an undiscoverable runner MUST leave every unit
`NOT_RUN`; a failing test, a run-level failure no test carries or a nonzero exit MUST NOT pass. A file
no instance owns, a module the plan lists no test file for, Minitest, the ci profile and a language
with a declared command keep the full command; an instance whose inventory record is absent, stale or
unreadable MUST run its full command with that reason; in FULL every instance runs its full command
and no contract unit is added. Each run MUST print `corvint-test: CONTRACT <instance> tier=<tier>
exact=<bool>`.
Delivery: delivered for Vitest, Jest, Playwright, pytest and RSpec with fixture runners; live runs
of each runner are NOT_RUN (none is installed on the lane host).

**Scenario: One Jest file selected**
- GIVEN a Jest repository with a committed inventory and two test files
- WHEN a change reaches one of them and `corvint test` runs
- THEN only that file runs through `npx jest --runTestsByPath` and the receipt records its tests'
  outcomes and durations under their canonical test IDs

**Scenario: Report missing**
- WHEN the runner exits 0 without writing its machine report
- THEN every selected file is `NOT_RUN` and the run is `INCOMPLETE`

**Scenario: Inventory absent**
- WHEN the repository commits no inventory record for the instance
- THEN the instance runs its full command with reason `inventory-absent`

## Surfaces

Delivered by ticket V1-5416 (build-log `docs/build-log/2026-11-11-runner-contract.md`):

- `internal/testrunner/contract` (tests layer) is the compiled-in interface derived from
  `registry.RunnerPlugin` (RNC-V1-001, RNC-V1-003, RNC-V1-005, RNC-V1-006, RNC-V1-012,
  RNC-V1-014). `Runner` has `Name`, `Lister`, `Default`, `Command` and `Parse`; `Instance` is
  runner, root and config under the inventory's `<runner>@<root>` key; `Reconcile` joins a
  selection with a report, and `ReconcileParsed` keeps the results a `*ReportError` (unreadable
  lines, run-level failures) came with. Go selects by package directory and one `-run` pattern,
  keeping every flag with its value, and reads `go test -json`; `-args`, `--` or a flag it cannot
  model runs the full command with `exact: false`. Vitest appends the selected files and
  `--reporter=json --outputFile=REPORT`; Playwright appends escaped file filters and
  `--reporter=json`, writing through `PLAYWRIGHT_JSON_OUTPUT_NAME`. A file filter drops a leading
  `-` and is `exact` only when a non-empty inventory listing holds no other file of the instance
  whose path contains it; a test-level ID runs its file. Jest runs files by literal path
  (`--runTestsByPath`, `--json --outputFile=REPORT`) and selected tests through one anchored `-t`
  pattern, and an unsupported setup runs its full command (JST-V1-003, JST-V1-008); pytest runs exact node IDs and reads pytest's
  own JUnit XML report (`--junitxml=REPORT`, xunit1, since PYR-V1-004; the pytest-json-report
  reader stays); RSpec runs spec files and
  writes `--format json --out REPORT` beside the progress formatter; a path starting with `-` is
  passed as `./path`. Minitest runs its full command with `runner-not-implemented` and has no
  parser. `Kind` is `e2e` for Playwright (RNC-V1-007). Runner flags come from public documentation
  and were not run against the real tools (NOT_VERIFIED). The package starts no process and reads
  no file.
- Listers (RNC-V1-002): each runner's `Lister` is the inventory's own identity
  (`testinventory.CurrentLister`), static for Go, RSpec and Minitest and executing for the others;
  the listers stay in `internal/testinventory`.
- `.corvint/test-commands.json` (RNC-V1-013, RNC-V1-014), profile `corvint-test-commands/0`, adds
  `runners` (runner name to argv) and `instances` (`<runner>@<root>` to argv) and makes `go`
  optional. `corvint test` reads it first; a malformed file exits 2 with `test-commands-invalid`
  and runs nothing. Corvint's own repository (module `github.com/Beamfall/corvint`) still reads
  `.github/cishards/test-commands.json` while the new file is absent and prints `corvint-test:
  NOTICE`; no other repository reads that path. Corvint now carries both files, and
  `tools/test-commands-check` fails unless they are byte-identical. A language with no declared command runs one
  `declared:<runner>@<root>` unit per instance the tree declares, from the instance root: its
  instance entry, else its runner entry, else the built-in default, printed as `corvint-test:
  DEFAULT`. The built-in Go default (no declared environment or flags) runs only where a Go
  instance exists. `corvint adopt --setup` reports the new file and its entries.
- Unowned path and removed runner (RNC-V1-004): `gateselect.AllowedUnknown` still admits
  `UNOWNED_DIRTY_PATH`, and both Go selectors (the CI driver's and the qualified one `corvint test`
  reads) then fall back with `unowned-path: ...: <path>` for an unowned path that no rule
  attributes: rules (b) and (c), a read-scope declaration that bears on it, the plan's
  documentation or evidence readers, its selections through a path-literal, path-glob,
  declared-read-scope, enclosing-package or declared-consumer witness, or an unobserved nested
  module. Rule (d), the packages whose reads no literal bounds, attributes no path, and neither does
  a selection through any other witness kind (the planner witnesses an unbounded reader with the
  first dirty path). The qualified selector's Go-only proof keeps only the unowned entries the
  planner itself named, so a non-Go path a declared runner owns does not force FULL there. With no Go universe `corvint test`
  plans FULL with the same reason itself. A go.sum or go.mod change keeps its separate root-module
  fallback. A runner instance the committed inventory or `.corvint/test-commands.json` names but the tree no
  longer declares plans FULL with `runner-removed`.
- NOT_PRODUCED by V1-5416: tasks 5, 7, 8, 10, 11 and 12, and the deletion of Corvint's old
  declaration (build log).

Delivered by ticket V1-5443 (build-log `docs/build-log/2026-11-13-runner-contract-run-path.md`):

- `internal/sessiontest` adapter `contract` (RNC-V1-019), registered under
  `corvint-test-adapter/0` beside `go`, `declared` and `provider`. `Build` turns each selected
  `typescript:<runner>:<path>`, `ruby:rspec:<stem>` and `python:<module>` unit (the latter through
  `Input.SelectedTests`, the plan's `Selection.Tests`) into contract units in every owning instance
  of `Input.Instances`; `Input.FullInstances` (the inventory verdict's full instances) sends an
  instance to `declared:<runner>@<root>` with the verdict's reason. `Invocations` derives one argv
  per instance with `ContractReportPlaceholder`; the launch derives it again with a private
  `corvint-test-report-*` file, reads at most `testid.MaxReportBytes`, parses it with the
  contract (the repository root with symbolic links resolved, then as given), reconciles it with
  `contract.ReconcileParsed` and removes the directory. `Invocation` gains `instance`, `tier`,
  `exact`, `exactReason` and `tests`, and `Unit` gains `kind` and `kindSource`, all omitted
  elsewhere, so every other manifest and receipt keeps its bytes.
- `internal/testrunner/killswitch` reads `CORVINT_SELECTION` (RNC-V1-008, the NMS-V1-008
  grammar; the refusal codes `kill-switch-value-unknown` and `kill-switch-full-unsupported`).
  `Input.KillSwitch` and `testinventory.Verdict.ForceFull` apply it in `corvint test`; the
  manifest and receipt gain `killSwitch`.
- NOT_PRODUCED by V1-5443: the Minitest selection and parser (the Ruby hook is the line-coverage
  collector; build log), the kill switch's widened `affected` plan, hooks, MCP and Action surfaces,
  and tasks 8, 10, 11 and 12 (build log).

## Traceability

| Requirement | Evidence |
|---|---|
| RNC-V1-001 | `TestRNCV1001EveryRunnerImplementsTheContract` (`internal/testrunner/contract`); `TestRNCV1013FullRunsInstanceDefaults`, `TestRNCV1019ContractUnitsPlanTheSelectedFiles` (`internal/sessiontest`) |
| RNC-V1-002 | `TestRNCV1002EveryRunnerSuppliesItsLister` (`internal/testrunner/contract`); `TestRNCV1002AffectedRunsNoListerInAPytestProject`, `TestRNCV1002JestWithoutInventoryRunsItsFullCommand` (`cmd/corvint`); `TestRNCV1019WhatTheContractDoesNotNarrow` (`internal/sessiontest`) |
| RNC-V1-003 | `TestRNCV1003GoSelection`, `TestRNCV1003VitestDirectoryRunIsNotExact`, `TestRNCV1003PlaywrightSelection`, `TestRNCV1003FileFilterIsNeverAFlag`, `TestRNCV1003GoFlagValuesSurvive`, `TestRNCV1003JestSelection`, `TestRNCV1003PytestSelection` (`internal/testrunner/contract`); `TestRNCV1019ContractRunRecordsPerTestResults` (`internal/sessiontest`) |
| RNC-V1-004 | `TestRNCV1004UnimplementedRunnerRunsFull` (`internal/testrunner/contract`); `TestRNCV1004RemovedRunners`, `TestSTRV0003BuildVerdicts` rows `allowlisted unknowns stay narrowed`, `an unattributed unowned path runs every test` and `a removed runner runs every test`, `TestDriverAndShadowShareOneUnknownAllowlist_AFPV0982` (`internal/sessiontest`); `TestUnknownPlanNarrowsOnlyThroughLanguageFrontiers_CINV0860` (`internal/gateselect`); `TestQualifiedUnownedPathIsThePlannersVerdict_RNCV1004` (`internal/gateselect`); `TestUnownedPathRunsAllOnlyWhenNoRuleAttributesIt_RNCV1004`, `TestUnownedPathWithoutGoUniverseRunsAll_RNCV1004`, `TestUnboundedReaderDoesNotAttributeAnUnownedPath_RNCV1004` (`internal/liveverify/affected`); `TestRNCV1004RemovedRunnerAndUnownedPathRunEverything` (`cmd/corvint`) |
| RNC-V1-005 | `TestRNCV1005GoReport`, `TestRNCV1005VitestReport`, `TestRNCV1005PlaywrightReport`, `TestRNCV1005GoTruncatedLineIsReported`, `TestRNCV1005FileLevelFailuresFailTheRun`, `TestRNCV1005JestReport`, `TestRNCV1005PytestReport`, `TestRNCV1005RSpecSelectionAndReport` (`internal/testrunner/contract`); `TestRNCV1019ContractRunRecordsPerTestResults` (`internal/sessiontest`); `TestRNCV1010JestSelectionRunsEndToEnd` (`cmd/corvint`) |
| RNC-V1-006 | `TestRNCV1006MissingResultsAreNotRun`, `TestRNCV1006GoPackageThatEndsAbnormallyIsIncomplete`, `TestRNCV1006DuplicateResultsKeepTheMostSevere`, `TestRNCV1006RunRecordsFailureIncompletenessAndDuplicates` (`internal/testrunner/contract`); `TestRNCV1019ContractRunFailsClosed`, `TestRNCV1006OmittedInventoriedTestIsNotRun`, `TestRNCV1006ContractUnitsCarryListedTests` (`internal/sessiontest`); `TestRNCV1006ListedTestsOfUnchangedFiles` (`internal/testinventory`) |
| RNC-V1-007 | `TestRNCV1007RunnerKinds` (`internal/testrunner/contract`); `TestRNCV1019ContractUnitsPlanTheSelectedFiles` (`internal/sessiontest`); the frozen `affected` plan signal is NOT_PRODUCED |
| RNC-V1-008 | `TestRNCV1008KillSwitchGrammar`, `TestRNCV1008KillSwitchScope` (`internal/testrunner/killswitch`); `TestRNCV1008KillSwitchPlans`, `TestRNCV1008GoKillSwitchRunsNestedModules` (`internal/sessiontest`); `TestRNCV1008KillSwitchOnCorvintTest`, `TestRNCV1008KillSwitchOnAffectedAndWatch` (`cmd/corvint`); hooks, MCP and the Action NOT_PRODUCED |
| RNC-V1-009 | NOT_PRODUCED (task 10) |
| RNC-V1-010 | `TestRNCV1010JestSelectionRunsEndToEnd`, `TestJSTV1009TwoRootFixtureThroughTheConsumers` (`cmd/corvint`) |
| RNC-V1-011 | `TestRNCV1019ContractUnitsPlanTheSelectedFiles` (the printed tier and exact flag come from the planned selection); the release manifest is NOT_PRODUCED (task 12) |
| RNC-V1-012 | `TestRNCV1003GoSelection`, `TestRNCV1005GoReport` (`internal/testrunner/contract`) |
| RNC-V1-013 | `TestRNCV1013BuiltInDefaults` (`internal/testrunner/contract`); `TestRNCV1013DeclarationMirror` (`tools/test-commands-check`); `TestRNCV1013ReadDeclaredResolution`, `TestRNCV1013InstanceCommand`, `TestRNCV1013FullRunsInstanceDefaults` (`internal/sessiontest`); `TestRNCV1013CLIFullCommandScenarios`, `TestRNCV1013CLIMalformedCommandsRefused` (`cmd/corvint`) |
| RNC-V1-014 | `TestRNCV1014InstanceOwnsItsTests`, `TestRNCV1003GoSelection` (nested instance) (`internal/testrunner/contract`); `TestRNCV1013InstanceCommand`, `TestRNCV1019WhatTheContractDoesNotNarrow` (two instances own one file) (`internal/sessiontest`) |
| RNC-V1-015 | NOT_PRODUCED (task 11) |
| RNC-V1-016 | `TestRNCV1016JestArgvLimitWidens` (`internal/testrunner/contract`, Jest only); the other runners NOT_PRODUCED (task 8) |
| RNC-V1-017, RNC-V1-018 | NOT_PRODUCED (tasks 11 and 12) |
| RNC-V1-019 | `TestRNCV1019ContractUnitsPlanTheSelectedFiles`, `TestRNCV1019WhatTheContractDoesNotNarrow`, `TestRNCV1019ContractRunRecordsPerTestResults`, `TestRNCV1019ContractRunFailsClosed`, `TestRNCV1019ContractRunnerUnavailable`, `TestRNCV1019PlaywrightReportPathReachesTheRunner`, `TestRNCV1019InventoryFallbackRunsUnselectedInstances`, `TestRNCV1019FIFOReportIsIncomplete`, `TestSTRV0004AdapterCapabilityManifest`, `TestPYRV1006RunnerTroubleFailsClosed` (`internal/sessiontest`); `TestRNCV1010JestSelectionRunsEndToEnd` (`cmd/corvint`) |

## Non-goals

Cypress (post-1.0, owner decision R6); a plugin protocol for third-party runners; narrowing quality
targets (G4); new cross-package analysis for monorepos (owner decision R15); the canonical test ID
rules (`canonical-test-id`), the inventory format (`test-inventory`), the run result record
(`run-evidence`) and per-test duration storage (`per-test-durations`); Jest selection details beyond
RNC-V1-010 (`jest-provider`).

## Rollback

Revert the change's commits. The kill switch (RNC-V1-008) falls back to each runner's full command
(RNC-V1-013) without a revert. RNC-V1-001 to RNC-V1-019 stay listed as retired if the change is
withdrawn and are never reused.
