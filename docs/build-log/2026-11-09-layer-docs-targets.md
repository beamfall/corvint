# Lane laydocs: shared and tests packages stop importing the docs and analyzer layers

Experimental, under the owner's standing lane auto-accept; base `0e99e1e6a` (the unmerged
decision 0594 branch), branch `claude/layer-docs-targets`. The import ratchet
(`docs/specs/import-ratchet-v0.md`, manifest of `docs/build-log/2026-11-09-layers-manifest.md`)
listed 39 wrong-way edges. This lane owned 17 of them, the ones whose target is a docs, analyzer
or release package or whose importer is `internal/mcp/toolgroup`; the parallel `laytests` lane
owns the edges whose target is a tests-layer package. Ten edges are removed and the baseline now
lists 29. No command, MCP tool list or file output changes; every MCP binary serves the tools it
served before.

## Edges removed (10)

| Edge | Fix |
|---|---|
| `internal/liveverify/rbmutate -> internal/analyzerruby` | New shared leaf `internal/rubysyntax`: the bounded Ruby lexer (`Mask`, `MaskSource`, `IdentifierByte`) moved out of `analyzerruby/lexer.go` unchanged. `analyzerruby` keeps `maskRuby` and `identifierByte` as one-line wrappers; `rbmutate` imports `rubysyntax`. |
| `internal/testrunner/registry -> internal/analyzerrust` | New shared leaf `internal/rustsyntax`: the Rust lexer (`Tokenize`, `Token`, `Kind`, `Keywords`, the four lexer bounds) and the executable-body extractor (`ExecutableBodies`, `Body`, `Range`, `Unsupported`) moved out of `analyzerrust`. `analyzerrust` keeps the bounds, body types and `ExecutableBodies` under its own names as aliases and wrappers, so its RSP-V0-005 tests stay where they are; the registry imports `rustsyntax`. |
| `cmd/corvint -> .github/cishards` | `cmd/corvint` only read the constant `MaxPatternBytes`. `sessiontest`, which already imports the planner, exports it as `sessiontest.MaxPatternBytes`; the three test-slice files read that. |
| `internal/flowcoverage/testfixture -> internal/doccorpus` | The two-field `Repository{ID, Revision}` moved into `internal/lineanchor` (the decision 0594 item 6 carve-out, which already holds `Anchor`); `doccorpus.Repository` is an alias. `flowcoverage.Denominator.Source` and the fixture use `lineanchor.Repository`. Twelve unkeyed `Repository{a, b}` literals in `doccorpus` became keyed, because `go vet` flags unkeyed literals of another package's type. |
| `internal/mcp/cli -> internal/doccorpus` | `LocalInputPathRule` (DCP-V1-054, a std-only path rule) moved into `internal/projectpath` as `LocalInputRule`; `doccorpus.LocalInputPathRule` wraps it and `mcp/cli` reads the shared one. |
| `internal/mcp/cli -> internal/mcp/docsbridge` | Re-key: `docsbridge` imports only `docsenvelope`, `mcp/bridge` and `mcp/rootbind`; by its own doc it reaches the docs tools "without linking a documentation package" by exec'ing the `corvint-docs` app (decision 0594 item 3). Its explicit `docs` prefix is dropped, so it falls under the `internal/mcp` shared prefix. |
| `internal/mcp/toolgroup -> internal/mcp/docsbridge` | Same re-key. The `toolgroup.Docs` adapter stays in `toolgroup`. |
| `internal/mcp/toolgroup -> internal/mcp/testvaliditybridge` | Inversion: the adapter is now `testvaliditybridge.Group`, beside its registry. `toolgroup` exports `Answer`, `Coded` and `FailureResult` for the bridges and keeps the `Group` interface, the error-profile constants, `Failure`, `Help` and `RootRelativePath`. `cmd/corvint-test-validity-mcp` and `mcp/cli` construct `testvaliditybridge.Group`. |
| `internal/mcp/toolgroup -> internal/mcp/corpusbridge` | Inversion: `toolgroup/corpus.go` became `corpusbridge/group.go`: `corpusbridge.Group` (was `toolgroup.Corpus`), `corpusbridge.Artifact` and `OpenArtifact` (were `CorpusArtifact`, `OpenCorpusArtifact`), `corpusbridge.Failure` (was `CorpusFailure`), with `CallDeadline`, `TimingMeta` and `SearchTool`. `mcp/cli` constructs them; `docs/specs/mcp-multi-root-v0.md` names the new identifiers. |
| `internal/mcp/toolgroup -> internal/doccorpus` | Went with the corpus group: `RefusalUnverifiedLimitation` and `StatCorpusFile` are now read by `corpusbridge`, which already imports `doccorpus`. |

The `toolgroup -> testevidencedoc` edge is the `laytests` lane's and is untouched; `toolgroup`
otherwise imports no product.

## Edges left in the baseline (7)

- `cmd/corvint -> internal/command/flows`. The `corvint flows` verbs and `corvint how` run in
  process (`cmd/corvint/flows.go`, help and the catalog); the docs shim forwards only the `docs`
  group and the `--corpus=` join. `command/flows` reads `doccorpus` for `flows docs` and for the
  `flows appmap infer --corpus` trust join, and `requestpath`, so re-keying it to `tests` would
  add `command/flows -> doccorpus` (its only consumer, `requestpath`, imports shared code and
  could move with it). Forwarding `flows` to `corvint-docs` would make the verbs depend on an
  installed app: a behaviour change. Stays until the owner decides where the flows verbs live.
- `internal/appflows -> internal/doccorpus` and `internal/appmap -> internal/doccorpus`. Both
  need `BuildBehaviorAdapter` (the 2,279-line adapter validator, which itself imports
  `testvaliditydoc`), canonical `Encode` (with its per-type limits and counters),
  `PublishMaintenancePair`, the stability thresholds (`stability.go` imports `jstestprovider`
  and `testvalidity`) and the behavior schema types. A shared carve would move most of
  `doccorpus`'s schema and validator, tangled with the `laytests` edges inside `doccorpus`; an
  inversion has no lower-layer wiring point, because `cmd/corvint` serves these verbs and is
  `tests`. Left as the manifest's named largest debt.
- `internal/flowcoverage -> internal/doccorpus`. Beyond `Anchor` and `Repository` (now
  `lineanchor`) it calls `doccorpus.Open` on a `corpus` denominator. `mcp/bridge` (shared) is a
  caller through `corvint.flows.coverage`, so an opener interface would have to be wired by
  `cmd/corvint-mcp` (`tests`), adding an edge instead of removing one.
- `internal/flowcoverage -> internal/flowdocs`. `flowdocs` imports `doccorpus`, so it cannot be
  re-keyed; the same `mcp/bridge` caller blocks an inversion.
- `internal/mcp/cli -> internal/mcp/corpusbridge`. `corvint-mcp --tool-profile corpus` serves
  the corpus group in process through `corpusbridge` and `doccorpus`. A registration model with
  `cmd/corvint-mcp` and `cmd/corvint` wiring the group would move this edge to two `tests`
  binaries (`cmd/corvint -> corpusbridge`, `cmd/corvint-mcp -> corpusbridge`): one line
  removed, two added. Dropping the group is the decision 0594 item 1 shape ("`corvint-mcp`
  serves impact, test validity, help and status") but is a behaviour change outside this lane.
- `internal/sessiontest -> .github/cishards`. `sessiontest` runs the planner (`PlanFrom`,
  `SplitPackages`, `ProfileDigest`, the bounds), so it cannot be served by a constant. The
  package lives under `.github`, which the lane may not edit; the sound fix is moving the
  planner to `internal/cishards` (keyed `tests`) with the `.github` consumers following, in the
  `.github` change. Re-keying `.github/cishards` to `tests` in `areas.json` would also clear it
  without touching `.github` (its production imports are std only) but contradicts the
  manifest's ".github is release" rule, so it is left for the owner.

## Manifest

`tools/import-ratchet/areas.json`: `internal/rubysyntax` and `internal/rustsyntax` added to
`shared`; `internal/mcp/docsbridge` removed from `docs` (falls under `internal/mcp`, shared).
`tools/import-ratchet/baseline.json`: the ten lines above deleted by hand; 29 edges remain. The
IRT-V0-005 control still passes against the new first line.

## Commands

- `GOTOOLCHAIN=local go build ./...`: PASS after each commit.
- `GOTOOLCHAIN=local go test -count=1 ./tools/import-ratchet/`: PASS after each commit (0.08 s).
- `GOTOOLCHAIN=local go vet` on every touched package (`rubysyntax`, `analyzerruby`,
  `liveverify/rbmutate`, `rustsyntax`, `analyzerrust`, `testrunner/registry`, `sessiontest`,
  `lineanchor`, `doccorpus`, `flowcoverage`, `flowcoverage/testfixture`, `projectpath`,
  `mcp/cli`, `cmd/corvint`, `mcp/toolgroup`, `mcp/corpusbridge`, `mcp/testvaliditybridge`,
  `mcp/docsbridge`, `cmd/corvint-test-validity-mcp`, `cmd/corvint-docs-mcp`,
  `cmd/corvint-mcp`): clean. `gofmt -l`: clean.
- `GOTOOLCHAIN=local go test -count=1` per package: `analyzerruby` 2.1 s, `liveverify/rbmutate`
  0.1 s, `analyzerrust` 0.02 s, `testrunner/registry` 1.4 s, `projectpath` 0.01 s, `sessiontest`
  3.4 s, `doccorpus` 57 s, `mcp/cli` 20.9 s, `mcp/corpusbridge` 29.9 s, `mcp/testvaliditybridge`
  0.02 s, `mcp/docsbridge` 5.0 s, `cmd/corvint-test-validity-mcp` 0.04 s, `cmd/corvint-docs-mcp`
  1.5 s: all PASS. `lineanchor`, `flowcoverage/testfixture`, `rubysyntax`, `rustsyntax` and
  `cmd/corvint-mcp` have no test files.
- `make -s unbounded-readers-check traceability-tests-check`: PASS.
- Not run (lane rule): `cmd/corvint`, `internal/liveverify/affected`, `internal/flowcoverage`,
  `internal/appflows`, `internal/appmap`, `internal/command/flows` tests, `make windows-vet`,
  `make changed-checks`, `make gate`.

`TestMCPV08300UnavailableLaunchHandshake` in `mcp/cli` fails on this host with the default
temp directory because `/tmp/.git` exists as an empty directory, which the launch-worktree probe
takes for an enclosing worktree; with `TMPDIR` set to a private directory the package passes.
The failure predates this lane and is not a code defect.

## Rollback

Revert the five code commits; the ten edges return to the baseline and the manifest prefixes to
their previous keys. No wire contract, store or CLI changes.
