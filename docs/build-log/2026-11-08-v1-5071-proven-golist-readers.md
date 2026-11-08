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

- The network guards keep their `GOFLAGS=` reset, restored in repair round 1 (see below), along
  with `GOWORK=off` and the other literals.
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

## Repair round 1 (independent review P1)

Finding: the first commit dropped the `GOFLAGS=` reset from the two network guards to keep them
proven. Without it, an inherited `-overlay` can substitute stale dependency source, so a new
`net/http` importer escapes detection, and an inherited `-e` suppresses loader failures.

Fix (c3b9e176):

- All six `.Env` lines in `internal/closureguard/network_test.go` again append `"GOFLAGS="`, in
  the original helper's order.
- The classifier in `internal/liveverify/affected/golang/importgraph.go` now treats one shape as
  safe: the exact empty `"GOFLAGS="` string as a direct entry of an `x.Env = append(...)` or
  `x.Env = []string{...}` assignment. `namesGoEnvironment` masks only those entries before the
  per-package GOFLAGS/GOENV scan, and `literalEnvironment` accepts the same entry. An empty value only
  removes ambient flags, so it cannot widen what `go list` reads. Any non-empty or concatenated
  value, any `GOENV` value, and any other mention (including comments and other literals) still
  makes the unit unproven.
- New shape subtests in `TestClosureGuardShapeIsProven_AFP_V0_9600`: an empty reset is proven; a
  non-empty literal, a dynamic value and an empty reset outside an `.Env` assignment are unproven.
- The AFP-V0-9600 text and traceability row state the rule.

Measurement, with the binary rebuilt at c3b9e176 and the same body-only scratch edit: 6
IMPORT_GRAPH_READER units / 568 test files, 233 selected units / 943 test files, unchanged from
the first commit (baseline 16 / 820 and 240 / 973). `internal/closureguard` is not selected.
Soundness: the `_ "hash/crc64"` scratch selects `internal/closureguard`, five test files, with
witness IMPORT_GRAPH_READER via `cmd/corvint`. Both scratch commits were dropped.

Validation: `internal/closureguard` whole package via `script/heavy.sh`: ok. The golang package with
`-run` covering the closure-guard, import-graph (AFP-V0-2200 to 2202), symbol, reader and
environment tests: ok, including `TestClosureGuardIsProvenImportGraphReader_AFPV09600` on the
real package. gofmt -l and go vet on both packages: clean. Doc checks
(`test-read-scopes-check line-citations-check long-lines-check spec-requirements-check`): ok.

## Repair round 2 (rescope)

Finding (round-2 re-review, P1): the round-1 masking hid every `"GOFLAGS="` entry independently,
so `command.Env = append(os.Environ(), "GOFLAGS=")` followed by
`command.Env = append(os.Environ(), "GOWORK=off")` stayed proven. The second assignment restores
ambient flags (for example `-f={{.Doc}}`), so a doc-only edit could be skipped.

Decision: under the lane convergence cap (at most two review-fix rounds), the coordinator rescoped
the exception to a fail-closed package rule rather than patching another shape.

Fix (ca39b487):

- `goEnvironmentUnproven` in `internal/liveverify/affected/golang/importgraph.go` replaces
  `namesGoEnvironment`. It is evaluated once per package, at its first `go list` reader file.
- Any identifier named `Setenv`, `Unsetenv` or `Clearenv` in the package makes it unproven,
  called or not and on any receiver, because `os.Environ()` carries what they set.
- If the package names `GOFLAGS` or `GOENV` anywhere, every `Env` assignment in every file must be
  a `literalEnvironment` that carries the exact `"GOFLAGS="` entry directly. That covers any
  receiver, an index of the field, and an `Env:` composite key. Those entries must also be the only
  mentions of either name. Anything else makes the package unproven.
- A package that never names either name keeps the earlier AFP-V0-2200/9010 rule. Its literal `Env`
  assignments need no reset, so existing proven fixtures (for example `"GOTOOLCHAIN=local"`) are
  unchanged. Stated limit: a listing with no `Env` assignment, or one in such a package, still
  inherits ambient GOFLAGS. That is the classifier's existing assumption, not something this round
  changes.
- `internal/closureguard/candidates_test.go`: its five `Env` lines now also append `"GOFLAGS="`.
  The package names GOFLAGS (in the network guards), so without the reset in these lines the strict
  rule would make it unproven. The added reset only strengthens these guards.
- New shape subtests: a later reassignment that drops the reset, a second guard without it, `Env`
  from a variable, `t.Setenv("GOFLAGS", "")`, and `os.Setenv` of a computed name are all unproven. A
  reassignment that carries the reset in both is proven. All of the earlier subtests are kept.
- The AFP-V0-9600 text and traceability row state the rule.

Measurement, with the binary rebuilt at ca39b487 and the same body-only scratch edit: 6
IMPORT_GRAPH_READER units / 568 test files, 233 selected units / 943 test files, unchanged.
`internal/closureguard` is not selected. Soundness: the `_ "hash/crc64"` scratch selects
`internal/closureguard`, five test files, with witness IMPORT_GRAPH_READER via `cmd/corvint`. Both
scratch commits were dropped.

Validation via `script/heavy.sh` with `-run`:

- `TestClosureGuard` on the golang package: ok, 15 shape subtests plus the real-package
  `TestClosureGuardIsProvenImportGraphReader_AFPV09600`.
- The wider golang classifier set (import-graph, import-set, symbol, reader, environment,
  identity, command-directory): ok.
- `AFP_V0_90|AFPV0237|AFP_V0_22|AFP_V0_96|GoEnvironment` across `affected`, `golang/...` and
  `typedreach`: ok.
- `internal/closureguard` with `-run 'Test'`: ok.

gofmt -l and go vet are clean, the doc checks pass, and `make windows-vet` reports no diagnostics.
