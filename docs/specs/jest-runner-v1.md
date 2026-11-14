# Jest runner V1

Owner: Russell Lewis
Date: 2026-11-12
Intent status: accepted
Delivery status: partial
Health: KEEP
Authoritative inputs: change folder `docs/specs/openspec/changes/jest-provider/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/jest-runner/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-001, PROD-TEST-004, PROD-TEST-020) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R7, R10, R11, R13,
R14, R16 and R19).

## Agent digest
- Claim: Jest runs inside the compiled-in runner contract: named discovery, anchored name selection with an honest `exact`, per-test results, full runs for odd setups.
- Status: accepted / partial. Change `jest-provider` auto-accepted under G14 (decision 0596); tasks 1 to 6 delivered under V1-5455; tasks 7 and 8 open on the watch run.
- Exists: Jest discovery by canonical ID; anchored, lossless-only `-t` selection, `exact`, report reading, `jest-unsupported-config`; the narrowed `corvint test` run, receipt and `full:jest`.
- Blocked on: `corvint watch` for non-Go indexes (change `watch-seven-runners`); no live Jest run.
- Read next: Surfaces; Traceability.

## User and boundary

A developer or agent working in a JavaScript or TypeScript repository that tests with Jest runs
`corvint test`, `corvint affected`, `corvint watch` or the MCP affected tool. Before this change
Jest was detected by the TypeScript affected analysis and the runner contract could run Jest files
by path, but `corvint test` ran a Jest project only through its declared full command, the
inventory named Jest tests only by file, and no check showed a Jest project end to end. The provider
binary `corvint-js-test-provider` had no Jest path, and owner decision R14 retires provider binaries.

This spec is the canonical home of requirements JST-V1-001 to JST-V1-010, copied from the change's
`specs/jest-runner/spec.md`, and JST-V1-020, added by V1-5455 after its independent review. Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds only the delivery record (Surfaces, Traceability).

## Requirements

- `JST-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Jest is a built-in runner

Source: owner decisions R7 and R14 (2026-11-09); G23 (owner answer). `corvint` MUST run Jest under
the compiled-in runner contract (RNC-V1-001). Jest support MUST NOT need a provider executable or a
`providers.jest` configuration entry.
Delivery: delivered. Jest is one of the contract's compiled-in runners (`contract.For("jest")`),
`providers.jest` is refused and a Jest instance's full command resolves to the built-in default.
`corvint test` runs a narrowed Jest selection through the contract adapter (RNC-V1-019) and its
receipt records one outcome and duration per canonical test ID, Jest's exit status kept (V1-5455,
2026-11-13). A live Jest run is NOT_RUN: this host has no Jest install.

**Scenario: Jest project with no provider**
- GIVEN a repository with `jest.config.js` and no provider configuration
- WHEN `corvint test` runs a selection that includes Jest tests
- THEN Corvint launches Jest itself and records per-test outcomes

- `JST-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Jest discovery into the inventory

Source: owner decisions R10 and R11 (2026-11-09). `corvint inventory refresh` MUST list Jest test
files with `jest --listTests` and test names with Jest's own collection, record the lister as
executing (TIV-V1-004) and name each test by canonical test ID (CTI-V1-004).
Delivery: delivered. The Jest lister runs `jest --listTests --config CONFIG`, then Jest's collection
run `jest --config CONFIG --ci --coverage=false --passWithNoTests --json --testNamePattern=^(?!)`,
whose pattern matches no name, and names each test from the report's `ancestorTitles` and `title`; a
listed file with no named test stays at file level with `test-id-unmapped`. Either run failing
leaves the instance absent (TIV-V1-011). The lister version is 2, so version-1 records are stale.

**Scenario: nested describe blocks**
- GIVEN `src/a.test.ts` with `describe("cart")` containing `it("adds item")`
- WHEN the inventory is refreshed
- THEN it lists `jest:src/a.test.ts::cart::adds item`

- `JST-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Selection arguments and exact

Source: owner decision R16 (2026-11-09). For selected Jest tests, Corvint MUST pass the selected
files with `--runTestsByPath` and one `-t` pattern, anchored at both ends, matching the selected
full names with every regular expression character escaped. `exact` MUST be true only when no
unselected test in those files has a selected full name.
Delivery: delivered in the contract. The pattern is `^(?:NAME|...)$`, and `exact` also compares
names case-insensitively, as Jest compiles `-t` with the `i` flag. A file listed only by file or a
missing listing is not exact, and a selection that mixes whole files and tests passes no pattern.
The pattern is passed only when it is proven lossless (JST-V1-020); otherwise the selected tests
run their whole files. `corvint test` selects Jest at file level today (RNC-V1-019), so it passes
no pattern.

**Scenario: name with regular expression characters**
- GIVEN a selected test named `handles (a+b)`
- WHEN `corvint test` builds the Jest command
- THEN the `-t` pattern matches that name literally and no other test

- `JST-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Results from Jest's JSON report

Source: owner decisions R7 and R19 (2026-11-09). Corvint MUST write Jest's JSON report to a private
path and MUST map each assertion result to its canonical test ID from `ancestorTitles` and `title`,
with its outcome and its `duration` (PTD-V1-002).
Delivery: delivered. The contract reads every assertion result into its canonical ID, outcome and
duration rounded to milliseconds; the report path is the caller's `Request.Report`, a private file
`corvint test` creates. The receipt's contract invocation lists them under `tests`, and each file
outcome carries their summed duration (PTD-V1-002).

**Scenario: passing test with duration**
- GIVEN a report where `cart > adds item` passed in 14 ms
- WHEN the receipt is written
- THEN the outcome for `jest:src/a.test.ts::cart::adds item` is passed with `durationMs` 14

- `JST-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Missing results are NOT_RUN

Source: every unknown runs with a named reason; PROD-TEST-020. A selected Jest test absent from the
report MUST be recorded NOT_RUN and the run MUST be INCOMPLETE. A missing or unparsable report MUST
make the run INCOMPLETE and MUST keep Jest's exit code.
Delivery: delivered. `contract.ReconcileParsed` marks a selected test the report omits NOT_RUN and an
empty or unparsable report makes every selected test NOT_RUN, the run incomplete. The receipt keeps
Jest's exit status in the invocation's `exit`, and a nonzero exit with every test passing is never a
pass (RNC-V1-019).

**Scenario: report missing**
- GIVEN Jest exits before writing its report
- WHEN Corvint reads results
- THEN the run is INCOMPLETE and the exit code is Jest's

- `JST-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Default command

Source: G23 (owner answer). With no declared command, the Jest command MUST be `npx jest` run from
the instance root (RNC-V1-013).
Delivery: delivered. `npx jest` is the Jest runner's built-in default and `Declared.InstanceCommand`
runs it from the instance root.

**Scenario: no declared command**
- GIVEN a Jest instance at `apps/web` with no declared command
- WHEN it runs
- THEN Corvint runs `npx jest` with `apps/web` as the working directory

- `JST-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Instances and full:jest

Source: owner decisions R10 and R13 (2026-11-09). Each Jest root and config MUST be a separate
runner instance (TIV-V1-003). `CORVINT_SELECTION=full:jest` MUST run every Jest instance in full.
Delivery: delivered. Each Jest root and config is its own instance (TIV-V1-003), and
`CORVINT_SELECTION=full:jest`, read by `corvint test` through the one kill-switch parser, makes
every Jest instance run its full command from its own root with reason `kill-switch` (RNC-V1-008).

**Scenario: two roots**
- GIVEN Jest instances at `apps/a` and `apps/b`
- WHEN `CORVINT_SELECTION=full:jest` is set
- THEN both instances run in full

- `JST-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Unsupported setups run in full

Source: every unknown runs with a named reason. A Jest version below the documented minimum, a
config Corvint cannot read, or a configured runner or reporter that prevents the JSON report MUST
run the instance's full command with reason `jest-unsupported-config`.
Delivery: delivered. `corvint test` reads each Jest instance's setup before it narrows: the config
its full command names with `--config` or `-c`, else the first of Jest's own config files in the
instance root, else Jest's defaults. `JestSetupFrom` reads the nearest `node_modules/jest/package.json` and the
config through the caller's reader. `JestSetup.Unsupported` refuses a nil setup, an unreadable
version, a major version below `MinJestMajor` (27, design Open 1 settled by V1-5455), an unreadable
config, `testResultsProcessor` (the configured reporter that rewrites the `--json` report;
`reporters` do not change it), a `testRunner` other than `jest-circus` or `jest-jasmine2`, a
`runner` other than `jest-runner`, a project named by path, and a JavaScript or TypeScript config
that loads or spreads another module (`require(`, `import`, `...`; a type-only import is allowed).
Such a config is read as text, so a value set through a preset is not seen, and a report that then
lacks a selected test leaves it NOT_RUN (JST-V1-005).

**Scenario: custom test runner**
- GIVEN a Jest config that sets a custom `testRunner` without JSON results
- WHEN a plan selects tests in that instance
- THEN the instance runs in full with `jest-unsupported-config`

- `JST-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) End-to-end check

Source: G23 (owner answer, open item on the Jest path through `corvint test`); RNC-V1-010. A Jest
fixture with two roots MUST be planned and run through `corvint affected`, `corvint test`, `corvint
watch` and the MCP affected tool, and every selected test in each receipt MUST match the inventory.
This result is the evidence for RNC-V1-010 (Jest proven end to end).
Delivery: partial. A two-root fixture repository with a committed inventory runs through `corvint
affected`, the MCP affected tool (`corvint.affected`, the same verdict as `corvint affected
--summary`) and `corvint test`, and every test the receipt records is one the inventory lists; the
in-process fixture also checks listing, selection and reconciliation. NOT_PRODUCED: the `corvint
watch` run, since watch answers only Go selections until change `watch-seven-runners`. The Jest
used is a stub in Jest's documented `--json` shape; a live Jest run is NOT_RUN.

**Scenario: one-line change**
- GIVEN the fixture and a change to one source file used by one Jest test
- WHEN the four consumers plan and run it
- THEN each selects the same tests, and the receipt lists them with outcomes and durations

- `JST-V1-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) No Jest provider binary ships

Source: owner decision R14 (2026-11-09). The 1.0 release binary list MUST contain no Jest provider
binary.
Delivery: delivered. The release binary catalog has no entry whose name or package names Jest.

**Scenario: release list**
- GIVEN the release binary list
- WHEN the release check runs
- THEN no entry names a Jest provider

- `JST-V1-020`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5455) Name filter only when lossless

Source: the independent review of V1-5455 (P1, 2026-11-12); JST-V1-003, JST-V1-005, CTI-V1-010.
Jest drops an ancestor titled `ROOT_DESCRIBE_BLOCK` from its `--json` report but keeps it in the full
name `-t` matches, so a pattern built from a canonical ID can skip a selected test, which the report
then shows as pending, neither failed nor missing. Corvint MUST pass a `-t` pattern only when the
mapping from the selected IDs to Jest's full names is proven lossless: the source of every selected
file is read and names no `ROOT_DESCRIBE_BLOCK`, and no selected test belongs to a duplicate group.
Otherwise the selected tests MUST run their whole files (`tier: file`, `exact: false`) with the
reason.
Delivery: delivered in the contract (`jestFilterLossy`); `corvint test` supplies the bounded source
reader through `JestSetup.Source`.

**Scenario: root-named describe block**
- GIVEN `describe('ROOT_DESCRIBE_BLOCK', () => test('fails', ...))` and the selected test
  `jest:f.test.js::fails`
- WHEN the contract builds the command
- THEN it passes `--runTestsByPath f.test.js` and no `-t`, and the report answers the test as failed

## Surfaces

Delivered by ticket V1-5455 (build-log `docs/build-log/2026-11-12-jest-runner.md`):

- `internal/testrunner/contract` (JST-V1-003 to JST-V1-008): `Request` gains `Listed` (the
  inventory's canonical IDs of the instance) and `Jest` (a `JestSetup`). The Jest `Command` adds
  `--runTestsByPath FILES` and, for selected tests, `-t ^(?:NAME|...)$`. A setup `Unsupported`
  refuses runs the full command with `jest-unsupported-config: CAUSE`. `regexpLiteral` escapes
  both Playwright file filters and Jest names. Runner flags come from Jest's public CLI
  documentation and were not run against a real Jest (NOT_VERIFIED).
- `internal/testinventory` (JST-V1-002): the Jest lister adds the collection run and names tests
  at test level. Lister `jest-list-tests` is now version 2. The recorded lister command stays the
  `--listTests` command.
- `internal/sessiontest` (JST-V1-001, JST-V1-004, JST-V1-005, JST-V1-008): the contract adapter
  reads each Jest instance's setup (`jestSetup`) into `Request.Jest`, including a bounded reader
  for the selected files' source (`JestSetup.Source`), and records each file outcome's summed
  duration. Tests pin that `providers.jest` is refused and the built-in default resolves from the
  instance root.
- `cmd/corvint` (JST-V1-007, JST-V1-009): end-to-end tests over a two-root fixture repository.
- `tools/release-binary` (JST-V1-010): a test over the embedded catalog.
- Added by V1-5455 on 2026-11-13: the lossless-filter rule (JST-V1-020) and the argument-limit
  fallback (RNC-V1-016): Jest takes no argument file, so an over-long `-t` pattern widens the
  selection to its files and over-long file paths run the full command, each with reason
  `argv-limit`.
- NOT_PRODUCED: the `corvint watch` part of JST-V1-009 (change `watch-seven-runners`). The live Jest
  run is NOT_RUN because this host has no Jest install. Jest's G4 measurement belongs to the
  narrowing measurement work.

## Traceability

| Requirement | Evidence |
|---|---|
| JST-V1-001 | `TestJSTV1001JestNeedsNoProvider`, `TestRNCV1019ContractRunRecordsPerTestResults` (`internal/sessiontest`); `TestJSTV1006DefaultCommandFromTheInstanceRoot` (`internal/testrunner/contract`); `TestJSTV1009TwoRootFixtureThroughTheConsumers` (`cmd/corvint`) |
| JST-V1-002 | `TestTIVV1004JestListerTwoRoots`, `TestJSTV1002FailedCollectionLeavesTheInstanceAbsent` (`internal/testinventory`) |
| JST-V1-003 | `TestJSTV1003NamePatternIsAnchoredAndEscaped`, `TestJSTV1003DuplicateNamesAreNotExact`, `TestJSTV1003RootDescribeBlockWidensToTheFile`, `TestRNCV1003JestSelection`, `TestRNCV1016JestArgvLimitWidens` (`internal/testrunner/contract`) |
| JST-V1-004 | `TestJSTV1004ReportMapsNestedDescribesWithDurations` (`testdata/jest-nested.json`, hand-written in Jest's `--json` shape) (`internal/testrunner/contract`); `TestRNCV1019ContractRunRecordsPerTestResults` (`internal/sessiontest`); `TestJSTV1009TwoRootFixtureThroughTheConsumers` (`cmd/corvint`) |
| JST-V1-005 | `TestJSTV1005MissingResultsAndReportsAreIncomplete` (`internal/testrunner/contract`); `TestRNCV1019ContractRunFailsClosed` (`internal/sessiontest`); `TestJSTV1009TwoRootFixtureThroughTheConsumers` (`cmd/corvint`, the exit status kept) |
| JST-V1-006 | `TestJSTV1006DefaultCommandFromTheInstanceRoot` (`internal/testrunner/contract`); `TestJSTV1001JestNeedsNoProvider` (`internal/sessiontest`) |
| JST-V1-007 | `TestJSTV1007TwoRootsRunInFull` (`internal/testrunner/contract`); `TestTIVV1004JestListerTwoRoots`, `TestTIVV1003ForceFullCoversEveryInstance` (`internal/testinventory`); `TestJSTV1007FullJestRunsEveryInstance` (`cmd/corvint`) |
| JST-V1-008 | `TestJSTV1008UnsupportedSetupsRunInFull`, `TestJSTV1008SetupFromReader` (`internal/testrunner/contract`); `TestRNCV1019ContractUnitsPlanTheSelectedFiles` (`internal/sessiontest`, the setup read before narrowing) |
| JST-V1-009 | `TestJSTV1009TwoRootFixtureChain` (`internal/testrunner/contract`, in-process part); `TestJSTV1009TwoRootFixtureThroughTheConsumers` (`cmd/corvint`: affected, the MCP affected tool and test); the watch run is NOT_PRODUCED |
| JST-V1-010 | `TestJSTV1010NoJestProviderBinary` (`tools/release-binary`) |
| JST-V1-020 | `TestJSTV1003RootDescribeBlockWidensToTheFile`, `TestJSTV1003DuplicateNamesAreNotExact` (`internal/testrunner/contract`) |

## Non-goals

No `corvint-jest-test-provider` binary and no Jest subcommand in `corvint-js-test-provider` (owner
decision R14); no line evidence (change `line-evidence`); no Jest-specific flow links (G13); no
support for a Jest project run through a wrapper other than the declared command, which runs the
declared full command.

## Rollback

Revert the change's commits. Jest projects then run their declared full command through `corvint
test`, and the inventory names Jest tests by file again. Receipts written with Jest canonical IDs
stay readable as data. JST-V1-001 to JST-V1-010 stay listed as retired if the change is withdrawn
and are never reused.
