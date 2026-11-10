# Change proposal: one compiled-in runner contract for the seven blocking runners

Change name: `runner-contract`
Ticket: none filed yet. G23 asks for a ticket for the Jest end-to-end check; it is to be filed in
`.taskman/` before dispatch (not filed by this proposal).
Product requirement served: PROD-TEST-006 and PROD-TEST-011 in `docs/specs/PRODUCT-PLAN-1.0.md`
Owning capability spec: a new spec `docs/specs/runner-contract-v1.md` (prefix `RNC-V1`). It does not
exist yet; the lane creates it in its first commit.
Requirement IDs this change adds: RNC-V1-001 to RNC-V1-013, plus RNC-V1-014 to RNC-V1-017 (added
2026-11-09 after the review round) and RNC-V1-018 (shell provider status; final decision round
2026-11-09 under R42, M11). RNC-V1-012 is the canonical test ID approved by owner decision
R11 (2026-11-09); its rules are CTI-V1-001 to CTI-V1-012 in change `canonical-test-id`, and
RNC-V1-012 adds no rule of its own. `RNC-` has no entry in `docs/specs/REQUIREMENTS.tsv` (checked
2026-11-09 on `claude/layers-integ` at `9ec3a29d0`).
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/runner-contract/spec.md`).

## Why

G1 makes seven runners blocking for 1.0: Go, Vitest, Jest, Playwright, pytest, RSpec and Minitest
(confirmed by owner decision R7 (2026-11-09); Cypress is dropped from 1.0 by owner decision R6
(2026-11-09)). G23 asks for one compiled-in contract that all of them implement, and removes the
external runner protocol. Today the code has three separate paths and only three of the seven
runners are selected and run by `corvint test`.

### Today's state, from the code at `9ec3a29d0`

- `corvint test` runs units of three adapters: `go`, `declared` and `provider`
  (`internal/sessiontest/manifest.go:60-64`).
- Go: one `go test -json` invocation over the selected packages, plus one per test slice
  (`internal/sessiontest/run.go:133-163`). Go is selected and run.
- Declared languages come from `.github/cishards/test-commands.json`, profile
  `corvint-test-commands/0` (`internal/sessiontest/declared.go:26-29`). Each language maps to one
  full-suite argv or to none (`declared.go:343-373`). A unit routed to a language runs that whole
  command (`internal/sessiontest/plan.go:539-559`), not a narrowed selection.
- The only test provider is `corvint-js-test-provider` (`declared.go:40-43`). It knows two runners:
  `playwright` as kind `e2e` and `vitest` as kind `unit` (`declared.go:48-55`). Any other runner is
  refused (`declared.go:270-275`), and so are watch flags and a bare `--` (`declared.go:283-291`).
  - Provider unit IDs are `typescript:<runner>:<path>` (`internal/sessiontest/provider.go:19-21`,
    `:32-39`); other units stay with the declared full command (`provider.go:41-60`).
  - Vitest runs the whole `--dir` as a superset (`provider.go:96-97`).
  - Playwright filters by a file regex (`provider.go:132-137`); a selected file with no test in the
    receipt is `NOT_RUN` (`provider.go:285-290`).
  - Provider unavailable or receipt unreadable: `NOT_RUN` and the run is `INCOMPLETE`
    (`provider.go:143-158`, `:202-214`; `run.go:31`, `:801`).
- `cmd/corvint-js-test-provider/main.go:229-250` has subcommands `unit` (Vitest), `bun`, `e2e`
  (Playwright), `negate`, `qualify-keep-reporters` and `external-origins`. There is no Jest
  subcommand. Its Vitest JSON reporter output is Jest-compatible (`internal/jstestprovider/vitest.go:9`).
- `cmd/corvint-ruby-test-provider/main.go:1-14`, `:87`: RSpec only, experimental, profile
  `corvint-ruby-test-provider/0`. `internal/sessiontest` does not call it. There is no Minitest
  provider.
- The release binary list `tools/release-binary/binaries.json` (lines 22-25, integ 9ec3a29d0)
  ships `corvint-test-runner`, `corvint-go-test-provider`, `corvint-js-test-provider` and
  `corvint-ruby-test-provider` with role `product`. This conflicts with RNC-V1-001 (review T-M1).
- The task named `internal/registry`; that path does not exist. The registry is
  `internal/testrunner/registry`. `runner_plugin.go:19-25` defines `RunnerPlugin` with `Pieces`,
  `Inventory`, `Command`, `Parse` and `Evidence` (RSP-V0-025). `CommandRequest` carries a key, a
  purpose (`inventory`, `run`, `collect`), the tests (nil means full) and the run directory;
  `CommandResponse` carries argv, cwd, env, artifacts and a `Superset` flag. The file states that no
  current runner produces evidence. `SelectionPlugin` returns the Ruby plugin for `rspec`,
  `minitest`, `test-unit` and `rails-test`. Plugins exist for Python, Ruby and JavaScript, including
  Jest. This registry is used by the separate binary `cmd/corvint-test-runner`, not by `corvint test`.
- `internal/testrunner/dynamic/build.go:19` lists Jest, pytest, RSpec and Minitest; Jest is run with
  `--json --outputFile=... --runInBand` (`build.go:73-74`).
- The affected adapters detect Jest (`internal/liveverify/affected/typescript/typescript.go:60`,
  `:2110-2147`), name Python tests `python:<module>` (`.../python/python.go:96`) and have a Ruby
  adapter.
- `internal/runnerprotocol`, `internal/runnerpolicy`, `internal/runnerqualification`,
  `tools/runner-plugin`, `tools/runner-qualification` and `tools/runner-reference` exist.
- `corvint test` already accepts `--shard I/N` and has `test shards` and `test verify-shards`
  subcommands (`cmd/corvint/test.go`, usage lines 34-40, integ 9ec3a29d0); no requirement covers
  sharding the selections of runners other than Go (review T-M9).

### Does Jest run end to end through `corvint test`?

No, by code reading. `corvint test` can plan a Jest test file but can only run it through the
declared TypeScript full-suite command (unnarrowed), or mark the run `INCOMPLETE` when no command is
declared. The same holds for pytest, RSpec and Minitest. No end-to-end run was performed for this
proposal: NOT_RUN.

## What Changes

- One Go interface, compiled into `corvint`, that every runner implements: discovery (listing for
  the inventory), selection, run, result parsing and `NOT_RUN` reporting. It starts from
  `registry.RunnerPlugin` (G23).
- `corvint test` drives all seven runners through it. A runner without a working implementation
  runs its declared full command with a printed reason (owner answer 2, G1).
- Each runner supplies a lister to the one committed inventory under `.corvint/`, written only by
  `corvint inventory refresh` (owner decision R10 (2026-11-09); change `test-inventory`). Each
  lister is marked `static` or `executing`. Ruby uses static classification (review T-B5).
- Every test is keyed by the canonical test ID, RNC-V1-012 (owner decision R11 (2026-11-09);
  change `canonical-test-id`), in plan, receipt, links, evidence and watch.
- The declared full command moves from Corvint's internal CI file `.github/cishards/test-commands.json`
  to the adopter file `.corvint/test-commands.json`, with a built-in default full command for each
  runner when the file or the runner's entry is absent (RNC-V1-013). Corvint's own old path is read
  for one release.
- Results are per test where the runner's machine report allows it, with per-test durations
  (owner decision R19 (2026-11-09); change `per-test-durations`); a selected test missing from the
  results is `NOT_RUN` and the run is `INCOMPLETE`. Results feed the append-only run result record
  PROD-TEST-020 (owner decision R7 (2026-11-09); change `run-evidence`).
- Each selection carries a tier from `line | file | flow | full` and the `exact` flag (owner
  decision R16 (2026-11-09)); this replaces the earlier `exact | superset | full` wording.
- Several instances per runner, keyed by root directory and config file (owner decision R15
  (2026-11-09); RNC-V1-014). Sharding of every runner's selection is provided for the Action only
  (RNC-V1-015). Argument length is bounded (RNC-V1-016).
- The kill switch follows the grammar `full | full:<runner>[,<runner>]` defined by NMS-V1-008 and
  applies on every surface; an unknown value exits 2 in the CLI and CI and runs all tests in agent
  hooks and MCP (owner decision R13 (2026-11-09)).
- The provider binaries are retired as each runner's built-in contract lands and are not shipped as
  separate 1.0 artifacts (owner decision R14 (2026-11-09); RNC-V1-017).
- A change no runner owns, or a removed or unavailable runner, runs all tests with a named reason
  (owner decision R32 (2026-11-09)).
- The external protocol packages and tools are removed (G23).
- Each runner's tier and every fallback are printed in the run and recorded in the release manifest
  (G1).

## Per-runner contract

Runner flags below come from each runner's public documentation and were not run here
(NOT_VERIFIED). Each implementation records the flags it uses in the spec with a fixture test.
The discovery column is the runner's lister for `corvint inventory refresh` (change
`test-inventory`); `corvint affected` reads the committed inventory and runs no lister.

| Runner | Discovery (lister kind) | Selection | Result source | Today |
| --- | --- | --- | --- | --- |
| Go | `go list` and `go test -list` (executing) | package plus `-run` regex | `go test -json` | selected and run (`run.go:144-163`) |
| Vitest | `vitest list --json` (executing) | file paths; title filter | JSON reporter | provider, whole `--dir` superset (`provider.go:96-97`) |
| Jest | `jest --listTests`; titles from the JSON report (executing) | `--runTestsByPath` plus `-t` | `--json --outputFile` | full declared command only |
| Playwright | `playwright test --list --reporter=json` (executing; change `e2e-inventory-discovery`) | file regex, then `--grep` by title | JSON reporter | provider, file regex (`provider.go:132-137`) |
| pytest | `pytest --collect-only -q` node IDs (executing) | node IDs; file level in 1.0 (owner decision R8 (2026-11-09)) | JUnit XML (`--junitxml`) | full declared command only |
| RSpec | static classification of spec files (static; review T-B5, matches RBR-V1-001) | `path:line` or example IDs | `--format json` | experimental provider, not called |
| Minitest | static classification of test files (static); no built-in listing | file plus `-n /regex/` | Corvint's compiled-in Ruby hook (`MinitestHook#run` in `internal/rubylinecov`), extended with `result.error?` (settled D9) | none |

The earlier RSpec row named `rspec --dry-run --format json` for discovery. It is replaced by static
classification so that refresh runs no project code where it can avoid it (review T-B5); the
settled removal of the `--dry-run` alias (S2) is unchanged.

## Capabilities

### New Capabilities

- `runner-contract`: the compiled-in runner interface and its seven implementations.

### Modified Capabilities

- `session-test-run` (STR-V0): the declared-provider path becomes one implementation of the
  contract.
- `runner-selection-plugins` (RSP-V0): `RunnerPlugin` becomes the contract.

## Non-goals

- Cypress. Dropped from 1.0 and listed as post-1.0 by owner decision R6 (2026-11-09); the earlier
  "conditional under G1" wording is withdrawn.
- A plugin protocol for third-party runners.
- Narrowing quality targets; those are G4 and measured elsewhere.
- New cross-package analysis for monorepos (owner decision R15 (2026-11-09)).
- Defining the canonical test ID rules (change `canonical-test-id`), the inventory format (change
  `test-inventory`), the run result record (change `run-evidence`) or per-test duration storage
  (change `per-test-durations`).
- The Jest selection details beyond RNC-V1-010 (change `jest-provider`).

## Impact

- Code: `internal/testrunner/registry`, `internal/sessiontest`, `cmd/corvint/test.go`,
  `cmd/corvint-js-test-provider`, `cmd/corvint-ruby-test-provider`, `cmd/corvint-go-test-provider`,
  `tools/release-binary/binaries.json`.
- Removed (G23): `internal/runnerprotocol`, `internal/runnerpolicy`, `internal/runnerqualification`,
  `tools/runner-plugin`, `tools/runner-qualification`, `tools/runner-reference`. Kept:
  `registry.RunnerPlugin`, `internal/runnerselection`, `internal/testrunner`.
- Retired after their runner's built-in contract lands (owner decision R14 (2026-11-09)):
  `corvint-go-test-provider`, `corvint-js-test-provider`, `corvint-ruby-test-provider`.
- Whether `cmd/corvint-test-runner` survives as a separate binary is still open (see `design.md`).
- Measured by changes `fault-injection-harness` (G3) and `narrowing-measurement` (G4).
