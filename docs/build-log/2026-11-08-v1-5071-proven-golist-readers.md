# V1-5071: proven `go list` import-graph readers

V1-5071 moves this repository's `go list` import-graph guards into one test-only package,
`internal/closureguard`, that reads nothing but the listings. The AFP-V0-9010 rule then classifies
every test there as a proven reader: an import-set change selects it, a body-only edit does not.
AFP-V0-9600 records the contract. The classifier itself is unchanged.

## What moved

Fifteen guards keep their assertions and move from their owning packages:

- `cmd/corvint/core_network_boundary_test.go` (both network allowlists) to `network_test.go`.
- Candidate and parent guards to `candidates_test.go`. These come from `cmd/corvint-analyzer-rust`,
  `cmd/corvint-analyzer-shader/core_parent_test.go`, `cmd/corvint-analyzer-structured-data`,
  `internal/analyzerstructured`, `internal/analyzerpython/modularity_test.go` and `internal/console`.
- Companion guards from `cmd/corvint-test` to `companion_test.go`: CTF-V0-007, SKH-V0-163 and
  CCE-V0-015. The source-text halves of CTF-V0-007 and CCE-V0-015 stay in `cmd/corvint-test`.
- `internal/contextindex` (LTA-V0-009) and `internal/swiftimpact` (NGI-V0-988) to `ledgers_test.go`.
- `internal/proctrace` (RSP-V0-020) to `proctrace_test.go`.

To be provable, each guard has to follow five rules:

- Derive its own root from its own `runtime.Caller(0)` name, using nested `filepath.Dir`. A `..`
  literal names the repository root to rule (d) and makes the package unbounded.
- Feed that root to a single listing.
- Give every listing its own output and error names.
- Set a literal environment.
- Capture stderr in a `bytes.Buffer`, not from an `*exec.ExitError`, because the type assertion
  observes the error value.

Behavior notes:

- The network guards no longer set `GOFLAGS=`. Mentioning GOFLAGS makes a unit unproven.
  `GOWORK=off` and the other literals stay.
- `TestRSPV0020ImportDirectionRejects` is removed. It only self-tested the test-local `checkImports`
  helper. That helper is now inlined into `TestRSPV0020ImportDirection`, because passing the listing
  to a helper is an escape. The forbidden-edge assertions themselves are unchanged.

Spec citations were updated in the MCP test-validity, Playwright run-evidence, corvint-test fold,
change-compiled E2E, runner-selection and affected-plan-selection specs. Two `.corvint/unbounded-readers.json`
reasons were updated to name the new home: `cmd/corvint` and `internal/analyzerpython`.

## Measurement

The binary was built with `GOTOOLCHAIN=local go build ./cmd/corvint`. The comparison runs
`affected --base <pre-scratch> --format json` on a scratch commit that adds one comment line inside
`TestAFPV0099NarrowedRows` in `tools/ci-forward-row/narrowing_test.go`.

| | IMPORT_GRAPH_READER units | IGR test files | selected units | selected test files |
|---|---|---|---|---|
| before (origin/main b9b2b878) | 16 | 820 | 240 | 973 |
| after (873b023a) | 6 | 568 | 233 | 943 |

The plan drops seven units outright, 30 test files in total:

- `cmd/corvint-analyzer-rust` (1)
- `cmd/corvint-analyzer-shader` (3)
- `cmd/corvint-analyzer-structured-data` (2)
- `cmd/corvint-test` (11)
- `internal/analyzerstructured` (6)
- `internal/proctrace` (6)
- `internal/swiftimpact` (1)

Three units leave the IGR witness but stay selected as UNBOUNDED_READER (87 to 90) for their other
reads: `internal/contextindex` (205), `internal/console` and `internal/analyzerpython`.
`internal/closureguard` is not selected.

Soundness check: a scratch commit adding `_ "hash/crc64"` to `cmd/corvint/adopt_setup.go` selects
`internal/closureguard`, all five test files, with witness IMPORT_GRAPH_READER via
`cmd/corvint` and `internal/closureguard`. Both scratch commits were dropped.

## Retained readers

- `cmd/corvint` `TestCEMSeamsDependOnlyOnStdlibAndGit` and `golist_test.go`. The CEM guard lists
  `./internal/cem/...`, and a `/...` literal is rule (d). `cmd/corvint` is unbounded for its other
  reads anyway (`.corvint/unbounded-readers.json`).
- `tools/test-read-scopes-check/main.go` and `golang/typedreach/config.go`. These are production
  callers, not guards.
- `.github/cishards`. It is the universe cost table, and it mentions GOFLAGS.
- The `cmd/corvint` TempDir and EmbedFiles fixture listings, plus the classifier ground-truth fixtures
  in `internal/liveverify/affected` and `golang`. Those packages are unbounded for other reasons.
- `internal/liveverify/provider/interlock`. It was not an IMPORT_GRAPH_READER unit in the baseline
  plan, so it is out of scope.

The declared read scopes of `cmd/corvint-analyzer-rust`, `cmd/corvint-analyzer-shader` and
`internal/analyzerstructured` still include `internal/`. Narrowing them is an owner declaration,
not part of this change.

## Validation

- `internal/closureguard` was run as a whole package through `script/heavy.sh`: ok, exit 0.
- `-run 'TestClosureGuard|TestCommandDirectory|ImportGraph' ./internal/liveverify/affected/golang/`:
  ok, exit 0.
- Earlier focused runs passed: the 15 closureguard tests, the two classifier tests, and
  `cmd/corvint-test` `TestCTFV0007FoldedGroupBoundary|TestCCEV0015CoreBoundary`.
- `GOTOOLCHAIN=local go vet` on every changed package: clean.
- `make windows-vet`: exit 0.
- These checks passed:
  - `make -s test-read-scopes-check line-citations-check`
  - `unbounded-readers-check`
  - `spec-requirements-check`
  - `spec-capacity-check`
  - `spec-status-check`
  - `use-case-receipts-check`
- `make gate`, dogfood binding and independent review: NOT_RUN, by the lane charter.

Rollback: revert the commit. The guards return to their packages with the same assertions, and
those homes only add selections.
