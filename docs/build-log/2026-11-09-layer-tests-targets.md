# Layer ratchet: the shared and docs importers of tests-layer packages

Experimental, under the owner's standing lane auto-accept; base `0e99e1e6a` (the unmerged
decision 0594 cut), branch `claude/layer-tests-targets`, lane `laytests`. Decision 0594 item 8
layers the engine as `shared` (0), `analyzers`, `tests`, `docs` (1) and `release` (2), and the
import ratchet (`docs/specs/import-ratchet-v0.md`, `tools/import-ratchet/baseline.json`) lists
the wrong-way edges still to remove. This lane took the 22 edges whose target is keyed `tests`
and whose importer is keyed `shared` or `docs`; it removed 11 and leaves 11 with the reason each
needs a planned split rather than a local fix. Behaviour is unchanged: no CLI, MCP or file
output differs, every moved name keeps its old value under its old name, and every removed
edge is deleted from the baseline by hand.

## Edges removed

| Edge | Fix |
|---|---|
| `appflows -> extevidence` | Re-keyed `internal/extevidence` to `shared`: the external evidence provider wire records (repositories, selections, endpoints) import only shared packages and are needed by `appflows` (shared) and `breakagemap` (tests). |
| `appflows -> tcq` | Re-keyed `internal/tcq` to `shared`: library-and-wire-conformance only, imports only `cem/wire`, consumed by `appflows`, `testvalidity`, and both test providers. |
| `appflows -> testvalidity`, `doccorpus -> testvalidity` | `testvalidity` is the one shared projection vocabulary (its own doc says so); its only tests-layer import was `liveverify/mutate` for the two verdict strings and the witness shape. It now mirrors them (`StrengthKilled`, `StrengthSurvived`, `MutationWitness`); `mutate`'s new test holds the two equal; the uncalled `FromMutationReport` adapter is gone. Re-keyed to `shared`. |
| `doccorpus -> liveverify/affected` | New shared leaf `internal/repopath`: the canonical repository-relative path rule (`Valid`), `Normalize`, `DecodeNameList`, the walk and per-file bounds and the bounded no-follow `ReadSource` (the two platform readers moved with it). `affected` keeps `ValidRelativePath`, `NormalizePaths`, `DecodeNameList`, `ReadSource`, `ErrInvalidUnit`, `ErrWalkLimit`, `ErrStatusMalformed`, `MaxWalkEntries` and `MaxSourceBytes` as one-line aliases of the same values, so `errors.Is` and the error texts are unchanged. `doccorpus` reads the leaf. |
| `mcp/bridge -> plansnapshot` | `plansnapshot` used `affected` only for `ValidRelativePath` and `DecodeNameList`; it reads `repopath` now and is re-keyed to `shared` (its other imports, `cem/gitrun`, `gitstatus`, `gokernel`, already are). |
| `appflows -> testvaliditydoc` | New shared leaf `internal/pinnedread`: the no-follow, directory-pinning, bounded file reader (`ReadFile`, `ReadFileBounded`, `ReadAbsoluteFileBounded`, `StreamFileBounded`, `StatFile`, the three pinned opens, `ReadRegular`, `ErrInputTooLarge`, the platform refusal) moved out of `testvaliditydoc` with its five tests; `testvaliditydoc` keeps every name as an alias of the same value and `MaxInputBytes` as `pinnedread.MaxBytes`. `appflows` reads the leaf for the flows maintenance receipts. |
| `command/flows -> testevidencedoc` | Inverted: `flows.Env` gains `TestEvidence TestEvidenceReader`, a function the binary supplies that parses the `corvint test-evidence` arguments and returns the query with its `Read`; `flows test-evidence` only joins the returned document (encoded verbatim as `evidence`) to the flow lookup. `flows.go` gains `flowEnvSubcommands` for subcommands built from `Env`; `Subcommands()` lists both maps. `cmd/corvint` wires `testevidencedoc` in `flows_test_evidence.go`; the flows package test supplies the same adapter (a test-only import). |
| `mcp/toolgroup -> testevidencedoc` | Inverted: `toolgroup.TestEvidence` gains `Parse TestEvidenceParse`; `internal/mcp/cli` (tests) supplies `testevidencedoc.Parse` and `Read`. A nil `Parse` is an internal error; both binaries set it. The refusal order (invalid params, binding, cancellation, read) is unchanged. |
| `cmd/corvint-docs-mcp -> swiftimpact`, `cmd/corvint-web-flows -> swiftimpact` | The blank import registers the Swift SwiftPM reader for `contextindex` path impact. Path impact is reached only through `contextindex.Impact`, `ImpactSyntax`, `EvalImpact`, `EvalImpactSyntax` and `PathImpactSyntax`; `go list -deps` of both binaries shows no package of theirs calling any of them except `mcp/bridge.Registry`, which `corvint-docs-mcp` never constructs (it serves `docsbridge` tools and help only). The registration was unreachable, so the two imports are removed. `corvint`, `corvint-mcp` and `corvint-test-validity-mcp` keep theirs. |

New shared prefixes in `areas.json`: `internal/repopath`, `internal/pinnedread`; moved to shared:
`internal/extevidence`, `internal/tcq`, `internal/testvalidity`, `internal/plansnapshot`. The
ratchet shows no new wrong-way edge after each move. Baseline: 39 edges before, 28 after.

## Edges left in the baseline

- `appflows -> jstestprovider`, `appmap -> jstestprovider`, `doccorpus -> jstestprovider`,
  `flowcoverage -> jstestprovider`: the consumers use the Playwright receipt types,
  `DecodeAttemptReceipt`, `QualifiedReceiptBindingReady`, `QualifiedApplicationRevision` and
  the projections. A scratch build of `receipt.go`, `attempt_details.go`, `qualification.go`
  and `projection.go` as their own package needs 30 more symbols from `external.go`,
  `freshness.go`, `preflight.go`, `bun.go`, `sensitive_input*.go`, `qualified_screen.go`,
  `project_reporters.go` and `keep_reporters_qualification.go`: the qualified-document decoder
  recomputes the canonical envelope, so it is the receipt-validation half of the provider, not a
  leaf. Splitting the provider into receipt and runner packages is a planned package split.
- `appmap -> testvaliditydoc`, `doccorpus -> testvaliditydoc`: `Decode`, `Project`, `Test` and
  `Document` project receipts through `jstestprovider` and `rubytestprovider`; they follow the
  provider split above.
- `appflows -> liveverify/affected`, `appflows -> liveverify/affected/languages`,
  `appflows -> liveverify/affected/typescript`: `FlowImpactAt` and the E2E selector embed the
  planner (`Build`, `Graph`, `Plan`, `Unit`, `languages.SelectAt`, `VerifyPlaywrightDiscovery`,
  `PlaywrightDiscoveryUnit`) and are called from `mcp/bridge` (shared) and `command/flows`
  (docs), so a function-value inversion would have to thread the planner through both. The
  layering decision has to place the flows impact and E2E selection in `tests` (as
  `appflows/e2erunner` already is) or split the planner's data model; neither is a local fix.
- `doccorpus -> liveverify/affected/typescript`: `PlaywrightListedTests` needs the Playwright
  discovery model (`playwright_discovery_list.go`, 25 more symbols in a scratch build).
- `cmd/corvint-docs -> testsource`: `BuildTestDiscovery` is built on `testsource.Repo`, whose
  methods span eight files; moving it is the `testsource` split, not a leaf extraction.

## Commands

- `GOTOOLCHAIN=local go build ./...`: PASS after every step.
- `GOTOOLCHAIN=local go test -count=1 ./tools/import-ratchet/`: PASS after every step (no
  `NEW_WRONG_WAY_EDGE`, no `STALE_BASELINE_EDGE`).
- `GOTOOLCHAIN=local go vet`, also with `GOOS=windows`: `repopath`, `pinnedread`,
  `liveverify/affected`, `testvalidity`, `liveverify/mutate`, `testvaliditydoc`, `appflows`,
  `doccorpus`, `plansnapshot`, `mcp/bridge`, `command/flows`, `mcp/toolgroup`, `mcp/cli`,
  `cmd/corvint`, `cmd/corvint-docs-mcp`, `cmd/corvint-web-flows`: clean.
- `GOTOOLCHAIN=local go test -count=1 <pkg>`: PASS for `testvalidity`, `liveverify/mutate`
  (the new parity test), `repopath`, `pinnedread`, `testvaliditydoc`, `plansnapshot`,
  `appflows`, `mcp/bridge`, `doccorpus`, `command/flows`, `cmd/corvint-web-flows`;
  `mcp/toolgroup` has no tests.
- `internal/mcp/cli`: `TestMCPV08300UnavailableLaunchHandshake` FAILS (`corvint.status` answers
  `repository-unavailable` instead of `invalid-root`) and fails the same way with this lane's
  `mcp/cli` edits reverted: pre-existing on this host. Every other test of the package passes.
- `cmd/corvint-docs-mcp`: `TestPRBV0010ServerHonoursProfileKnobs` FAILS because the host has a
  stray `/tmp/.git`, so the profile path under `t.TempDir()` is refused as inside a worktree:
  environmental, unrelated to the removed import. Every other test of the package passes.
- `env GOTOOLCHAIN=local make -s spec-requirements`: no changes.
  `make -s unbounded-readers-check traceability-tests-check`: PASS.
- NOT_RUN: `go test` of `cmd/corvint` and `internal/liveverify/affected` (lane bound; both were
  vetted, including their tests, for linux and windows), `make gate`, `make windows-vet` as a
  whole (replaced by per-package `GOOS=windows go vet`).

## Rollback

Revert the branch: the six packages return to their previous keys, the two leaves disappear and
their callers import `affected` and `testvaliditydoc` again, the eleven baseline lines return.
