# Layer ratchet: the remaining wrong-way edges

Experimental, under the owner's standing lane auto-accept; base `b55635262`, branch
`claude/layers`, lane `layers`, tickets V1-5248 and V1-5249. Decision 0594 item 8 layers the
engine as `shared` (0), `analyzers`, `tests`, `docs` (1) and `release` (2);
`tools/import-ratchet/baseline.json` listed 17 wrong-way edges after decision 0595 and
`docs/build-log/2026-11-09-layer-tests-targets.md`. This change removes 16 and leaves one,
`internal/sessiontest -> .github/cishards`, because removing it needs a `.github` change.
No requirement, rule or wire contract changes: `areas.json` gains and moves prefixes, the
baseline only shrinks, and no CLI, MCP or file output differs.

## The constraint that decided the placements

A package reached by the binaries of both products cannot sit in either product's layer,
because no binary can wire in a package from a sibling layer: `cmd/corvint-docs`,
`cmd/corvint-docs-mcp` and `cmd/corvint-web-flows` are keyed `docs`, and `cmd/corvint`,
`cmd/corvint-mcp`, `cmd/corvint-test-validity-mcp` and the providers are keyed `tests`. The
`go list -deps` closure of every `tests` and `docs` binary at the base showed which packages
both products reach:

- `docs`-keyed packages that `tests` binaries reach: `doccorpus`, `flowdocs`,
  `mcp/corpusbridge`. `corvint` and `corvint-mcp` serve the corpus tools through
  `mcp/cli -> mcp/corpusbridge`, and the flows MCP profile (`toolProfileFlows` in `mcp/cli`)
  serves `corvint.flows.coverage`, whose `flowcoverage.Compile` reopens a flow-doc generation
  with `flowdocs.Open`, a full `flowdocs.Generate` rederivation over `doccorpus`.
- `tests`-keyed packages that `docs` binaries reach: `liveverify/affected` and its 13 language
  packages, `langplugin`, `jstestprovider`, `rubytestprovider`, `testvaliditydoc`,
  `stepnegation`, `testevidence`, `testsource`. `corvint-docs flows impact` and
  `flows status --base` run `appflows.FlowImpactAt`, which builds the affected impact graph
  (`affected.Build`, `languages.SelectAt`) and reports its unknowns; `doccorpus` decodes provider
  receipts and test-validity documents; `corvint-docs corpus discover-tests` ran
  `testsource.BuildTestDiscovery`.

After this change both lists are empty. With behaviour fixed and no edge accepted as debt, each
such package either moved to `shared` whole, when everything in it is needed by both, or was
split, when one product's half is cleanly separable (a provider runner from its receipt model,
lint checks from the source readers discovery uses).

## Edges removed

| Edge | How | Commit |
|---|---|---|
| `appflows -> liveverify/affected`, `-> affected/languages`, `-> affected/typescript`; `doccorpus -> affected/typescript` | Re-keyed `internal/liveverify/affected` (the planner and its language packages) and `internal/langplugin` to `shared`; both import only shared packages. The three now-redundant `langplugin/*` shared prefixes are dropped. | `2379de2a3` |
| `appflows`, `appmap`, `doccorpus`, `flowcoverage -> jstestprovider` | New shared `internal/jstestreceipt`: the type-checked closure of what those four, the test-validity model, the RSpec model and test discovery read (receipt types, `DecodeAttemptReceipt`, qualification, screening, sensitive-input validation, run-record parsers, projections; 236 declarations). `jstestprovider` keeps the adapters and runners and every moved exported name as an alias or one-line wrapper. | `c20cf5f97` |
| (enabler) | New shared `internal/rubytestreceipt`: the RSpec document, `Validate`, `ProjectExample`, `ProjectRun`; reads `jstestreceipt`. `rubytestprovider` keeps the runner and aliases. | `ec9a3baab` |
| `appmap`, `doccorpus -> testvaliditydoc` | New shared `internal/testvaliditymodel`: `Decode`, `Project`, `ProjectPinned`, `Document`, `Test`, locations and the rejection, route-stub, unstubbed-call and vacuous-equality evidence. `testvaliditydoc` keeps discovery of retained evidence and the step-negation join (its only readers of `stepnegation` and `testevidence`) and aliases. `doccorpus` reads the bounded readers from `pinnedread` directly. | `2c7518c90` |
| `appflows`, `appmap`, `flowcoverage -> doccorpus`; `flowcoverage -> flowdocs`; `mcp/cli -> mcp/corpusbridge` | Re-keyed `internal/doccorpus`, `internal/flowdocs` and `internal/mcp/corpusbridge` (now under the `internal/mcp` shared prefix) to `shared`; with the receipt models split out they import only shared packages. | `02e222ba5` |
| `cmd/corvint-docs -> testsource` | New shared `internal/testdiscovery`: `BuildTestDiscovery` with the Git blob reader (`Repo`), the JS/TS token model (`source`) and the RSpec line reader it is built on. Go keeps methods with their type, so the 212 lint, validity and parity methods of `source`, `jsReader`, `rspecReader`, `Repo`, `rubyText` and `rspecSupport` became package functions in `testsource` taking the reader first (a name a local declaration shadows at a call site is prefixed with its type, for example `RepoSeedRules`). The reader fields and methods `testsource` uses are exported; every moved name keeps its old name in `testsource`, and the `runGit` test hook is assigned through `testdiscovery.RunGit`. | `7510b754f` |

The four new packages are keyed `shared` in `areas.json`; the ratchet showed no new wrong-way
edge after each step. Baseline: 17 edges before, 1 after.

The splits were made with a type-checked mover (`golang.org/x/tools/go/packages`): it computes
the closure of the named roots (a moved type takes all its methods, a const or var group moves
whole so `iota` values hold), moves the declarations byte for byte with their comments, exports an
unexported name only when code left behind uses it, and leaves an alias or wrapper for every
moved name still referenced. Exported for the remaining code: `jstestreceipt` 96 names and 10
fields, `rubytestreceipt` 40 names and the selector fields, `testvaliditymodel` 13 names and the
three `Input` fields (`JS`, `GoSession`, `Ruby`), `testdiscovery` 109 names, 133 fields and 28
methods. None of the exported fields belongs to a JSON-encoded type except `Input`, which is
never encoded.

## Placements that differ from the orchestrator's delegation

The orchestrator placed the work under the owner's delegation (OWNER-delegated). Four points
could not be honoured without a behaviour change or a sibling-layer edge, so the reachability
rule above decided them:

1. "Flow impact analysis and E2E test selection belong in the tests layer." `FlowImpactAt` and
   `ReadInventoryProvider` are called by `command/flows` (`corvint-docs`) and `mcp/bridge`
   (`corvint-mcp`, `corvint-docs-mcp`), so they stay in `shared` `appflows` (decision 0595 keeps
   the flow readers shared) and the planner moved down instead. `SelectE2E`, `JoinFlowPlan`,
   `ReconcileProvider` and `BindE2EFlows` are called only from `cmd/corvint` and could move to a
   tests package, but once the planner is shared no edge needs it, so they were left in place.
2. "Flow doc rendering (`flowdocs`, `doccorpus` text) stays in docs." `flowdocs` is the
   generator `flowcoverage` rederives through, and `doccorpus` is served to `tests` binaries, so
   both are `shared`. `doccorpus.Render` and `Maintain` (`render.go`, 189 lines) are used only by
   `cmd/corvint-docs` and could move to a docs package, but `doccorpus`'s in-package tests
   (`provider_test.go`, `review_regression_test.go`) call `Render`, which would form an import
   cycle; not carved.
3. "Invert lower-layer docs reads through a registry the binary wires." No remaining read had a
   binary on one side only, so no registry was added.
4. "`testvaliditydoc`: rendering stays docs." `testvaliditydoc` is keyed `tests` and has no
   rendering; its model went to `shared` and the rest stays `tests`.

The `corpusbridge` re-key is the option the delegation named for a package that is really
shared. `testsource` stays `tests`, as `TestCMSV0915FlowReaderIsSharedAndFlowCommandsAreDocs`
pins.

## Edge left in the baseline

`internal/sessiontest -> .github/cishards` (tests to release): `sessiontest` plans shards with
`cishards.PlanFrom`, `SplitPackages`, `ProfileDigest`, `MaxShards`, `MaxSlices` and
`MaxPatternBytes`. Removing the edge means moving the partitioner out of `.github/cishards` and
having `.github/cishards` import it, which is a `.github` change; `github-split-check`
(`ASD-V0-706`) refuses a range that mixes `.github` with code, so it needs its own change.
Copying the partitioner instead would make two sources of truth for CI shard plans.

## Commands

- `GOTOOLCHAIN=local go build ./...`, `go vet` of every touched package, `gofmt -l`: clean
  after each step.
- `script/heavy.sh GOTOOLCHAIN=local go test -count=1 ./tools/import-ratchet/`: PASS after each
  step (no `NEW_WRONG_WAY_EDGE`, no `STALE_BASELINE_EDGE`).
- `script/heavy.sh GOTOOLCHAIN=local go test -count=1 <pkg>`: PASS for `jstestprovider`,
  `rubytestprovider`, `testvaliditydoc`, `testsource` (also after the method conversion alone),
  `flowcoverage`, `appflows`, `appmap`, `doccorpus`, `cmd/corvint-docs`,
  `cmd/corvint-js-test-provider`, `cmd/corvint-ruby-test-provider`, `mcp/testvaliditybridge`.
- `corvint affected --base b55635262`: scope UNKNOWN, 312 units selected (the split packages
  are imported widely), 50 `LANGUAGE_FRONTIER`, `NO_SELECTABLE_TEST` for the four new packages
  (no test files of their own; their code runs under the unchanged tests of the packages they
  came from) and two `UNSELECTED_BINARY_CONSUMER` of `corvint-js-test-provider`.
- NOT_RUN: the 303 selected units that are not directly changed (including `cmd/corvint`), and
  the two declared binary consumers; `make gate` (not requested).

## Rollback

Revert the branch: the moved declarations return to their packages, the four new packages and
their `areas.json` prefixes disappear, `liveverify/affected`, `langplugin`, `doccorpus`,
`flowdocs` and `mcp/corpusbridge` return to their previous keys, and the 16 baseline lines
return.
