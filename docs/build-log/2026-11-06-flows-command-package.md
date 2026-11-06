# Flows command package (V1-4756, V1-4747 slice)

Following the breakage move (V1-4751), the `flows` command handlers and their handler-level tests
move from `cmd/corvint` into `internal/command/flows` behind one narrow entry. No other command
moves and no generic command framework is introduced.

## Scope

- `internal/command/flows` owns every `flows` subcommand handler (record, export, import, map,
  gaps, impact, ingest, docs, stability, ingest-coverage, coverage, appmap, navigate,
  request-path), the `how` handler, their help texts and the `testdata/flows` goldens. The entry
  is `Run(ctx, root, args, out, diagnostic, env) int`, where `Env` injects the two root-owned
  behaviours the handlers need: `EmitError` (the root JSON diagnostic writer) and `BaseRefusal`
  (the root `unsupported-affected-revision` refusal shared with `affected`). `flows impact`
  returns a private unresolved-base sentinel that `runFlowSubcommand` converts through `BaseRefusal`, so the
  refusal bytes are unchanged.
- `cmd/corvint/flows.go` keeps `flowInvocation` (the `--root` preamble and `how` alias argument
  shaping), and `runFlows` is a one-line wrapper, so `main.go` dispatch is untouched. Root help
  reads the exported help constants (`HowHelp`, `IntentHelp`, `NavigateHelp`, `AppmapHelp`,
  `AppmapPlanHelp`, `RequestPathHelp`). The help catalog reads `Subcommands()`, which returns the
  table sorted; the catalog already sorted operations, so its bytes are unchanged.
- Handler-level tests moved with their names unchanged, and `TestMain` isolates Git as in the
  breakage package. These stay in root because they go through root dispatch, the `how` alias,
  root help or the compiled binary:
  - `TestFlowsHelpDoesNotRequireRepository`
  - `TestSKHV0810HowAlias`
  - `TestAFUV1130FlowsHelpSaysTestLinksOnly`
  - `TestAFUV1904Help`
  - `TestFlowDocMaintenanceCompiled`
  - `flows_appmap_ui_test.go`

  The root help assertion from `TestAMAPV0211BuildFromCLI` is now a separate root test,
  `TestAMAPV0211BuildFromHelp`.
- The root `affected`, `impact` and MCP-parity tests use the shop and appmap fixtures. Those tests
  keep a root copy of the fixture helpers in `cmd/corvint/flows_fixtures_test.go` (about 280
  lines of duplicated test code).
- `tools/capabilities-check` also reads subcommand tables from `internal/command/*`. The CAP-V0-006
  and CAP-V0-007 text now says so.
- `.corvint/test-read-scopes.json` declares the package's fixture reads.

Non-goals: no flag, help, diagnostic, exit-code or wire change; no other command moved, including
`docs flows`; no generic command framework. No speedup is claimed.

## Evidence

Base `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`, Go 1.27.3, Darwin.

- Byte parity: 55 fixed invocations gave identical stdout, stderr and exit code from the base and
  candidate binaries, covering:
  - help forms for every subcommand, `help flows`, `help how` and `help --json`
  - usage refusals for every subcommand
  - the `flows impact --base nonexistent` revision refusal
  - `--root` preamble errors
  - success paths `flows export --emit provider`, `flows map` and `flows gaps` over a committed
    copy of the appmap fixture
- `go list -deps ./internal/command/flows` contains no `cmd/` package.
- Size before and after the move:

  | Location | Base | After |
  | --- | --- | --- |
  | `cmd/corvint` tests | 1537 `func Test` | 1471 |
  | `internal/command/flows` tests | none | 68, including the new `TestMain` |
  | `cmd/corvint` test lines | 83,532 | 79,523 |
  | `cmd/corvint` production lines | 51,914 | 49,508 |
  | `internal/command/flows` lines | none | 2,475 production, 4,406 test |

  The one new root test is the split help assertion `TestAMAPV0211BuildFromHelp`.
- Focused tests via `script/heavy.sh`, all passing:
  - `./internal/command/flows` (whole package)
  - `./cmd/corvint` for the 94 remaining flows, affected-fixture, help, help-catalog and
    docs-flows tests
  - `./tools/capabilities-check`
  - `./internal/liveverify/affected -run TestBatchReplayDeclaredRuntimeReaders_V12941`
- Other checks, all passing: `make capabilities-check`, `make unbounded-readers-check`,
  `make test-read-scopes-check`, `make spec-requirements`, `go vet`, `gofmt -l` and
  `make windows-vet`.
- CI named test slices still list the moved names under `cmd/corvint`. An anchored slice simply
  matches fewer tests, and the new package runs in its own default slice, so there is no
  `.github` change.

## Rollback

Revert the commit: the handlers, tests and goldens return to `cmd/corvint/flows*.go`,
`cmd/corvint/flow_coverage*.go` and `cmd/corvint/testdata/flows`, and the capabilities-check scan
returns to `cmd/corvint` alone. There is no persistent state, schema or wire contract to unwind.
