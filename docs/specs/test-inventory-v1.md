# Test inventory V1

Owner: Russell Lewis
Date: 2026-11-11
Intent status: accepted
Delivery status: partial
Authoritative inputs: change folder `docs/specs/openspec/changes/test-inventory/` (`proposal.md`,
`design.md`, `tasks.md` and `specs/test-inventory/spec.md`), `docs/specs/PRODUCT-PLAN-1.0.md`
(PROD-TEST-004, PROD-TEST-005, PROD-TEST-017) and
`docs/decisions/0596-accept-product-plan-1-0-2026-11-10.md` (owner decisions R10, R15 and R38).

## Agent digest
- Claim: One committed inventory lists every test of the seven blocking runners by canonical ID; a missing or doubtful list runs everything.
- Status: accepted / partial. Change `test-inventory` auto-accepted under G14 (decision 0596); delivered under V1-5406 except the GitHub Action refresh step.
- Exists: `internal/testinventory`; `corvint inventory refresh`; `inventory` in `affected-summary/1` and `corvint.affected`; `corvint-test: INVENTORY` in `corvint test`; `Inventory` in `corvint watch` plan events.
- Blocked on: the GitHub Action refresh step (task 7) waits on change `github-action`; selection from the inventory is `e2e-inventory-discovery` and `runner-contract`.
- Read next: Surfaces; Traceability.

## User and boundary

An agent, a developer or the GitHub Action plans which tests a change runs. Before this change only
Playwright had a listing step (`corvint affected discovery`, a receipt printed to stdout), `corvint
test` read declared full commands with no list of individual tests, and no command wrote a list of
tests to the repository. A selection had no known list to start from.

This spec is the canonical home of requirements TIV-V1-001 to TIV-V1-012, copied from the change's
`specs/test-inventory/spec.md`. Design decisions, failure modes and rollback stay in the change's
`design.md`, which owns them; this file adds only the delivery record (Surfaces, Traceability).
Selection from the inventory belongs to `e2e-inventory-discovery` and `runner-contract`.

## Requirements

- `TIV-V1-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) One committed inventory

Corvint MUST keep one committed inventory file for all seven blocking runners at
`.corvint/test-inventory.json` with profile `corvint-test-inventory/0`. Every entry MUST be keyed by
the canonical test ID (CTI-V1-001) and name the file the test lives in.

**Scenario: repository with Go and Vitest**
- GIVEN a repository with one Go module and one Vitest project
- WHEN `corvint inventory refresh` completes
- THEN `.corvint/test-inventory.json` holds one instance record for each, with canonical IDs

- `TIV-V1-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) Only the refresh verb writes the inventory

`corvint inventory refresh` MUST be the only command that writes the inventory. `corvint affected`,
`corvint test`, `corvint watch` and the MCP tools MUST only read it and MUST NOT run a lister.
`corvint watch` MUST NOT refresh the inventory. The GitHub Action MUST run
`corvint inventory refresh` before it plans. Delivery: the GitHub Action part is not yet built (task
7 waits on change `github-action`); every other part is delivered.

**Scenario: watch with a stale inventory**
- GIVEN a stale inventory
- WHEN `corvint watch` handles a save
- THEN it reports `inventory-stale`, runs the widened selection and leaves the file unchanged

- `TIV-V1-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) Runner instances keyed by runner, root and config

Each inventory record MUST describe one runner instance identified by runner, root directory and
config file. A repository MAY declare several instances of one runner. `CORVINT_SELECTION=full:<runner>`
MUST apply to every instance of that runner.

**Scenario: two Jest roots**
- GIVEN Jest instances at `apps/web` and `apps/admin`
- WHEN `CORVINT_SELECTION=full:jest` is set
- THEN both instances run their full commands

- `TIV-V1-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) One lister per runner, marked static or executing

Each runner MUST have one lister, and each instance record MUST state whether its lister was
`static` (reads source only) or `executing` (runs project code). Go, RSpec and Minitest listers are
static; Vitest, Jest, Playwright and pytest listers are executing. An executing lister MUST run only
inside `corvint inventory refresh`.

**Scenario: Playwright record**
- GIVEN a refreshed inventory for a Playwright project
- WHEN the record is read
- THEN it states lister kind `executing` and the lister command used

- `TIV-V1-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) Inventory bound to the tree

Each instance record MUST store the commit it was refreshed at, the config file digest, the lockfile
digest when one exists, the lister name and version, and a blob digest for every listed test file.

**Scenario: binding fields present**
- GIVEN a refreshed pytest instance
- WHEN the record is read
- THEN it carries the commit, config digest, lister version and one blob digest per test file

- `TIV-V1-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) Absent inventory runs the declared full command

When the inventory file is missing, or has no record for a declared runner instance, that instance
MUST run its declared full command and the plan MUST print reason `inventory-absent`.

**Scenario: no inventory committed**
- GIVEN no `.corvint/test-inventory.json`
- WHEN `corvint affected` plans any change
- THEN every declared runner instance is selected in full with reason `inventory-absent`

- `TIV-V1-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) Config, lockfile or lister change makes the instance stale

When the config file digest, the lockfile digest, the lister version or the inventory profile
differs from the record, every test of that instance MUST run and the plan MUST print reason
`inventory-stale`.

**Scenario: Vitest config edited**
- GIVEN `vitest.config.ts` changed since the last refresh
- WHEN `corvint affected` plans the change
- THEN every Vitest test of that instance is selected with reason `inventory-stale`

- `TIV-V1-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) A changed test file makes only its tests unassigned

When a listed test file's blob digest differs from the record, or a new test file appears under the
instance root, every test in that file MUST run and be counted `unassigned`. Other files keep their
entries.

**Scenario: new test added to one file**
- GIVEN a test added to `spec/user_spec.rb` after the refresh
- WHEN `corvint affected` plans the change
- THEN `spec/user_spec.rb` runs as `unassigned` and other RSpec files are planned from their entries

- `TIV-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) Invalid inventory runs every test

When the inventory file cannot be read, fails schema validation or has an unknown profile, every test
of every declared runner MUST run and the plan MUST print reason `inventory-invalid`.

**Scenario: truncated file**
- GIVEN a truncated `.corvint/test-inventory.json`
- WHEN `corvint affected` plans any change
- THEN every declared runner runs in full with reason `inventory-invalid`

- `TIV-V1-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) Refresh refuses drift

`corvint inventory refresh` MUST record HEAD and the source digest before and after each lister and
MUST refuse with `inventory-refresh-drift`, leaving the previous file unchanged, if either moved.

**Scenario: file edited during refresh**
- GIVEN a test file is edited while the Jest lister runs
- WHEN refresh finishes the lister
- THEN it refuses with `inventory-refresh-drift` and the old inventory is kept

- `TIV-V1-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) Refresh bounds output and records lister failures

Refresh MUST refuse with `inventory-too-large` when the file would exceed its size cap. When one
lister fails, refresh MUST write the other instances, omit the failed instance and record the
failure, so readers treat that instance as absent.

**Scenario: pytest collection error**
- GIVEN `pytest --collect-only` exits with a collection error
- WHEN refresh completes
- THEN the pytest instance is omitted with the error recorded and the next plan runs pytest in
  full with `inventory-absent`

- `TIV-V1-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-5406) Add-only schema

New fields MAY be added to `corvint-test-inventory/0`; existing fields MUST NOT change meaning. A
reader MUST ignore fields it does not know. A change of meaning requires a new profile, which old
readers treat as `inventory-invalid`.

**Scenario: newer writer adds a field**
- GIVEN an inventory with an extra field from a newer writer
- WHEN an older reader plans a change
- THEN it ignores the field and plans normally

## Surfaces

Delivered by ticket V1-5406 (build-log `docs/build-log/2026-11-11-test-inventory.md`):

- `internal/testinventory` (tests layer) holds the schema, the canonical writer and the reader
  (TIV-V1-001, TIV-V1-003, TIV-V1-012): every list sorted, two-space indented JSON, one trailing
  newline; a member it does not know is kept and written back, never read. An instance is declared
  by its config file (`go.mod`; `vitest.config.*`; `jest.config.*`; `playwright.config.*`;
  `pytest.toml`, `.pytest.toml`, `pytest.ini` or `.pytest.ini`, or a `pyproject.toml` `tool.pytest`
  or `tool.pytest.ini_options` table or a `tox.ini` or `setup.cfg` pytest section, first present in
  pytest's documented search order, in that order; `.rspec` or
  `spec/spec_helper.rb`; `test/test_helper.rb`), one per runner and root, outside `node_modules`,
  `vendor`, `testdata` and hidden directories; the lockfile is the nearest one of the runner's
  ecosystem at or above the root. Files are named by Git blob object ID from the commit tree or the
  working tree, read with lock-free, hermetic Git that never rewrites the index.
- Listers (TIV-V1-004, TIV-V1-005): Go `go-ast` (static: top-level `Test`, `Fuzz` and runnable
  `Example` functions per package directory, build constraints not evaluated; an unparseable file is
  its package ID with `test-id-unmapped`, and several unparseable files of one package share one such
  entry while every file stays bound); RSpec `rspec-static` (static, file level); Minitest
  `minitest-static` (static: `def test_` methods of a top-level class deriving directly from
  `Minitest::Test`, otherwise file level); Vitest `vitest list --json`, Jest `jest --listTests` (file
  level), Playwright `playwright test --list --reporter=json` converted through
  `playwright-discovery/0` (its source digest is recorded), pytest `--collect-only -q -p
  no:cacheprovider` (all executing, from the project's own `node_modules/.bin` or `.venv`). Each
  record binds the commit, config and lockfile blobs, the lister identity and the blob of every
  listed test file and, for a static or proven instance, every candidate file.
- Default-pattern proof (TIV-V1-005, TIV-V1-008): an executing instance keeps the runner's default
  test-file names only when the reader proves it; otherwise it is unproven and the verdict's
  `cause` names why (the config, then up to four causes). A JavaScript or TypeScript config is
  proven when it is read whole and its tokens (comments aside) hold no import but the runner's own
  package (`vitest`, `vitest/config`; `jest`, `@jest/types`; `@playwright/test`; a type-only import
  is not checked), no call but the identity helper `defineConfig`, no spread (except a Playwright
  `...devices['Name']`), function, class, constructor, tagged template, template expression,
  escape, computed key or computed member, and no pattern member named as an identifier or string:
  Vitest `include`, `includeSource`, `projects`, `workspace`, `typecheck`, `dir`, `root`; Jest
  `testMatch`, `testRegex`, `projects`, `preset`, `rootDir`, `roots`; Playwright `testMatch`. A
  `jest.config.json` is proven when it is an object naming none of the Jest members. Vitest also
  needs no `vitest.workspace.*` or `vitest.projects.*` file beside the config, and Jest no `jest`
  member in the `package.json` beside it. A `package.json` with a `jest` member, or a script that
  runs vitest, declares the runner's instance when its directory holds no config file of that
  runner (last in precedence); such an instance is always unproven, and the Vitest lister then runs
  without `--config`. A Jest or Vitest instance also needs every script of the `package.json` in its
  root and each directory above it that runs the runner to pass only output, reporting, coverage,
  watch, parallelism, timeout, snapshot and filter flags (long flags compared without case, dashes or
  a `no-` prefix, so `--test-match` is `--testMatch`): a script passing `--testMatch`, `--testRegex`,
  `--dir`, `--root`, `--rootDir`, `--roots`, `--config`, `-c`, Jest's `--runTestsByPath` (it runs the
  files it names whatever their names; `--findRelatedTests` stays allowed), any flag outside that
  list, a shell expansion, a word holding a backslash or a brace (the reader models neither escapes
  nor brace expansion, so `jest \-\-config=ci.js` and `jest {--config,ci.js}` may pass a flag;
  V1-5435), or Vitest's `bench` or `typecheck` command makes the instance unproven, its
  cause naming the manifest, the script and the flag (V1-5425, V1-5427). The scripts fail closed on
  indirection too (V1-5427): a package-manager script run (`npm run`, `npm test`, `yarn`, `pnpm` or
  `bun`, with or without `run`, a `concurrently` `npm:name` word, or `npm-run-all`, `run-s` or
  `run-p`) that forwards any `-`-prefixed word, or a word holding a shell expansion, backslash or
  brace, is unproven, its cause naming the flag and the manager, and so is a Vitest `bench` or
  `typecheck` positional it forwards (`npm test bench` beside `"test": "vitest"`; V1-5435), unless
  the script it names is in the same manifest and mentions neither the runner nor a
  package manager (`npm run build -- --watch` beside `"build": "tsc"` stays proven); npm forwards
  the positionals after the script name and only the flags after `--`, a flag before `--` leaves
  the target unresolved, and a run of
  the runner itself (`yarn jest --ci`, `npm exec jest -- --ci`) has the words after the runner judged
  by its flag list instead. A quoted
  word holding whitespace is split and scanned again, so `sh -c "jest --config ci.js"`, `bash -c`,
  `concurrently "jest --config a"` and `nodemon --exec "vitest --dir src"` are read as the runner's
  own commands, and any other word of such a wrapper that mentions the runner is unproven. When a
  word of such a wrapper before the runner mentions it or starts a package-manager run, each
  `-`-prefixed word after a standalone `--` and each `{@}`, `{*}` or `{N}` placeholder in such a
  word is unproven too (V1-5435), since `concurrently "jest {@}" -- --testMatch x` passes those words
  into the command it runs; a wrapper none of whose words reach the runner (`concurrently "tsc -w"
  -- --preserveWatchOutput`) is not judged. A word mentions the runner with any backslash removed,
  so `j\est --config ci.js` is an unmodelled command. A command
  the reader does not model that mentions the runner anywhere (`node -e "require('jest')..."`) is
  unproven; the cost is that an unrelated command naming the runner, such as `eslint jest.config.js`,
  also leaves the instance unproven. A pytest section is proven when it sets no
  `python_files` and no `testpaths`, its `addopts` hold only output, reporting, strictness, failure
  and import-mode options (a positional path, `-p`, `--doctest-modules` or `--doctest-glob` is none
  of them), and no `conftest.py` under the root defines a collection hook (`pytest_collect_file`,
  `pytest_pycollect_makemodule`, `pytest_collect_directory`, `pytest_ignore_collect`) or
  `pytest_plugins`. Every pytest config in the root that declares pytest is proven, not only the
  first, because which one pytest reads depends on its version. An INI file is read as pytest's
  `iniconfig` reads it (a header starts at column 0 and may carry a comment; an indented line
  continues the value above), and an indented line holding the pytest header is unproven. A TOML file
  is scanned for each key's full path, so an indented, quoted (basic or literal) or dotted key or
  header counts, CRLF line ends included; a table below the pytest table counts as setting the option
  it names, even with no key under it. A TOML file the scanner cannot read (one starting with a UTF-8
  byte order mark among them, V1-5427), an escaped key, or the pytest table set inline is unproven,
  and a `pyproject.toml` with an escaped key declares pytest, so `[tool."py\u0074est".ini_options]`
  is unproven rather than missed; a `pyproject.toml` the scanner cannot read declares pytest when
  it holds the text `pytest` or any backslash, since a key spells pytest only literally or through an
  escape (a byte order mark before an escaped table name included, V1-5435). An unreadable config is unproven.
- Unproven instances (TIV-V1-005, TIV-V1-008): refresh binds only the files the lister names, so the
  record stays small in a large root. The verdict judges every file under the root with one of the
  instance's candidate extensions that the record does not bind (its config and lockfile aside) as
  unassigned: JavaScript or TypeScript for the JavaScript runners; Python for pytest, plus `.txt`,
  `.rst` and `.md` when a doctest option appears or an `addopts` option or the config cannot be
  ruled out; any file when a `conftest.py` hook may collect it. No directory is skipped for an
  unproven instance but `.git` and the inventory's temporary files, nested roots of the same runner
  included. A proven executing instance skips only `.git` and `node_modules`; the static runners
  keep their skipped directories. The reader cannot tell an unbound file that existed at refresh
  from a new one, so an unproven instance reads every unbound file of its languages as unassigned
  on every plan: the cost is selection breadth, never a missed test file.
- `corvint inventory refresh [--runner RUNNER] [--root DIR]` (TIV-V1-002, TIV-V1-010,
  TIV-V1-011) is the only writer: it replaces the file through a temporary file in the same
  directory, refuses `inventory-refresh-drift` when the working tree's source digest or HEAD moved
  after any lister or at one final check just before the write (made also when the filters select
  no instance), refuses `inventory-too-large` over 8 MiB, and records a failed lister in `failures`
  with the instance left out. The source digest leaves out the inventory file and its temporary
  files (`.corvint/.test-inventory-*.tmp`). The working-tree reader refuses a `--root` that is not
  the Git top level, and refresh then fails `inventory-unavailable`.
- Readers (TIV-V1-002, TIV-V1-006 to TIV-V1-009) judge the file and never run a lister or write:
  `corvint affected --summary` (`affected-summary/1`, member `inventory` and `inventory` text
  lines) and the `corvint.affected` MCP tool, which returns that verdict, against HEAD for a based
  plan and the working tree otherwise; `corvint test` at HEAD (`corvint-test: INVENTORY` on stderr,
  an `inventory:` summary line, the dry-run JSON `inventory` member); `corvint watch` against the
  working tree on every save (plan event member `Inventory`). The frozen `affected-plan/0` and
  `affected-plan/1` documents are not extended. A verdict never narrows: an instance runs in full
  with its reason, or is planned from its record with every unassigned file added.
- `CORVINT_SELECTION=full:<runner>` (TIV-V1-003): `Verdict.ForceFull` marks every instance of the
  runner full with `selection-full`; parsing the variable belongs to change
  `narrowing-measurement`, which owns its grammar and refusal, and no command reads it yet.
- NOT_PRODUCED: the GitHub Action refresh step (task 7, change `github-action`).
- Known limits: the proof reads the tree, not the host: a pytest plugin loaded through an installed
  entry point, `PYTEST_ADDOPTS` and command-line patterns are not seen. Candidate extensions are
  the runner's languages, so a Jest pattern over another extension (`.vue`) is not a candidate. A
  pytest `[tool.pytest]` table alone (no `ini_options`) declares no instance. A test directory
  outside the instance root (Playwright `testDir: '../e2e'`) is not a candidate; its listing fails
  the lister, which leaves the instance absent. The static RSpec and Minitest listers keep the
  default names: an `.rspec` `--pattern` or a Rake `TestTask` pattern is not read.

## Traceability

| Requirement | Evidence |
|---|---|
| TIV-V1-001 | `TestTIVV1001RoundTripCanonicalBytes` (`internal/testinventory`); `TestTIVV1001InventoryRefreshCLI` (`cmd/corvint`, one Go module and one Vitest project) |
| TIV-V1-002 | `TestTIVV1002ReadersPrintReasonsAndWriteNothing` (affected, watch and test read only), `TestTIVV1002InventoryHelpAndRefusals` (`cmd/corvint`); the GitHub Action part is NOT_PRODUCED (task 7) |
| TIV-V1-003 | `TestTIVV1003SeveralInstancesOfOneRunner`, `TestTIVV1003DetectInstances`, `TestTIVV1003PytestConfigPrecedence`, `TestTIVV1003ForceFullCoversEveryInstance`, `TestTIVV1004JestListerTwoRoots`, `TestTIVV1004VitestDeclaredByPackageScript` (`internal/testinventory`) |
| TIV-V1-004 | `TestTIVV1004GoStaticLister`, `TestTIVV1004GoUnmappedFilesCollapse`, `TestTIVV1004RSpecStaticLister`, `TestTIVV1004MinitestStaticLister`, `TestTIVV1004VitestLister`, `TestTIVV1004JestListerTwoRoots`, `TestTIVV1004PlaywrightLister`, `TestTIVV1004PytestLister` (`internal/testinventory`; each refreshes twice to identical bytes) |
| TIV-V1-005 | `requireBinding` in every TIV-V1-004 lister test; `TestTIVV1005TreesNameFilesByBlobID`, `TestTIVV1005UnprovenRefreshBindsOnlyListedFiles`, `TestTIVV1005UnprovenRecordHoldsOnlyListedFiles` (a 150,000-file root stays under MaxBytes), `TestTIVV1005WorkTreeRefusesSubdirectoryRoot` (`internal/testinventory`) |
| TIV-V1-006 | `TestTIVV1006To009ReaderRules` rows `absent file`, `absent record`, `absent after lister failure`; `TestTIVV1002ReadersPrintReasonsAndWriteNothing` (`absent`) |
| TIV-V1-007 | `TestTIVV1006To009ReaderRules` rows `stale config`, `stale lockfile`, `stale lockfile appeared`, `stale lister version`, `stale config moved`; `TestTIVV1002ReadersPrintReasonsAndWriteNothing` (`stale`) |
| TIV-V1-008 | `TestTIVV1006To009ReaderRules` rows `unassigned changed file`, `unassigned new file`; `TestTIVV1008UnprovenConfigFailsClosed` (custom patterns, a Vitest workspace file, `mergeConfig` from a package, `includeSource`, a Jest preset, `package.json` jest and vitest declarations, pytest doctest options and a conftest hook, a skipped-directory include, a computed key, an escape; V1-5425: indented, quoted, dotted and native pytest TOML keys and headers, `pytest.toml`, `.pytest.toml` and `.pytest.ini` precedence, INI header comments and indented headers, and `package.json` script flags), `TestTIVV1008ScriptRunsFailClosed` (V1-5427: flags forwarded through npm, yarn, pnpm, concurrently `npm:` and `npm-run-all`, `sh -c`, `bash -c`, `concurrently`, `nodemon --exec`, an unmodelled wrapper, an escaped word, `--runTestsByPath`), `TestTIVV1008ScriptEscapesFailClosed` (V1-5435: concurrently `{@}`, `{*}` and `{1}` placeholders and flags after the outer `--`, `bash -c` flags after `--`, backslash and brace runner arguments, a backslash or expansion forwarded through npm, a backslash-escaped runner name, Vitest `bench` and `typecheck` forwarded through npm, yarn and pnpm, a byte order mark before an escaped pytest table name), `TestTIVV1008PytestConfigEdgesFailClosed` (V1-5427: byte order mark, CRLF, literal keys, comments in multi-line values, tables below `ini_options`, an escaped table name), `TestTIVV1008ProvenDefaultControl` (V1-5435 adds forwarded positionals and a wrapper that do not reach the runner), `TestTIVV1005UnprovenRefreshBindsOnlyListedFiles` (`internal/testinventory`); `TestTIVV1002ReadersPrintReasonsAndWriteNothing` (`unassigned`) |
| TIV-V1-009 | `TestTIVV1006To009ReaderRules` rows `invalid truncated`, `invalid profile`, `invalid oversize`; `TestTIVV1009DecodeRefusesInvalid`; `TestTIVV1002ReadersPrintReasonsAndWriteNothing` (`invalid`) |
| TIV-V1-010 | `TestTIVV1010RefreshRefusesDrift` (`internal/testinventory`, source and HEAD drift), `TestTIVV1010FinalDriftCheckBeforeWrite`, `TestTIVV1010TemporaryInventoryFileIsNotDrift`; `TestTIVV1010InventoryRefreshDriftCLI` (`cmd/corvint`) |
| TIV-V1-011 | `TestTIVV1011ListerFailureRecorded`, `TestTIVV1011EncodeRefusesOversize` (`internal/testinventory`); `TestTIVV1011InventoryRefreshTooLargeCLI` (`cmd/corvint`) |
| TIV-V1-012 | `TestTIVV1012UnknownAddedFieldPreserved` (`internal/testinventory`) |

## Non-goals

No selection logic (`e2e-inventory-discovery`, `runner-contract`); no running of tests by
`affected`, the planning step of `test`, `watch` or MCP to build the list (product invariant 4); no
cross-package analysis and no sharding outside the GitHub Action (owner decision R15); no runner
beyond the seven blocking runners; no line evidence (`line-evidence`) and no run results
(`run-evidence`).

## Rollback

Revert the change's commits and delete `.corvint/test-inventory.json`. Readers then see an absent
inventory and run each runner's full command with `inventory-absent`, the safe fallback. TIV-V1-001
to TIV-V1-012 stay listed as retired if the change is withdrawn and are never reused.
