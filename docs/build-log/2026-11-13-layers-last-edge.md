# Layer ratchet: the last edge, and what the restructuring tickets still lack

Experimental, under the owner's standing lane auto-accept; lane `struct1113`, base
`366408098f741126468695245f461cb4fa9dbf8b`, tickets V1-4807, V1-4809, V1-4814 and V1-5249.
Contract: `docs/specs/import-ratchet-v0.md` (IRT-V0-001, IRT-V0-005). Each ticket was found
PARTIAL on 2026-11-13; this entry records the part this lane built and the evidence for the
criteria that were already met, so they need not be re-derived.

## The last baseline edge: `internal/sessiontest -> .github/cishards`

`docs/build-log/2026-11-11-layers-remaining-edges.md` left this edge because moving the shard
planner out of `.github/cishards` is a `.github` change, which `github-split-check` keeps out of
a code range, and `docs/build-log/2026-11-09-layer-docs-targets.md` named re-keying as the other
fix but left it for the owner as contrary to ".github is release".

Decision 0594 item 8 defines the release layer as tooling "which serves the others and is not
shipped". `.github/cishards` is shipped: `cmd/corvint` and `cmd/corvint-test-runner` link it
through `internal/sessiontest` (`PlanFrom`, `SplitPackages`, `ProfileDigest` and the bounds that
`corvint test` and `affected` use), and its production imports are the standard library only.
The manifest already keys every package by role rather than directory, so it now keys
`.github/cishards` `tests`, the layer of its only product consumer; `release` packages
(`tools/ci-shard-costs`, `tools/ci-test-slices`, `tools/corvint-pr-tests`,
`tools/live-test-run`) may import it as before, and the rest of `.github` stays `release`. Its CI
command `.github/cishards/cmd` sits in a hidden directory that no scanned package imports, so it
is outside the graph (IRT-V0-003) and needs no prefix; a `release` prefix for it failed the
ratchet with `STALE_AREA_PREFIX`. No Go file, CLI or MCP output changes. The baseline is now
empty. Moving the planner's files out of `.github` remains the sound structural follow-up, in its
own `.github` change; the re-key does not depend on it.

With the baseline empty, the IRT-V0-005 self-check used to skip both controls. It now runs the
added-edge control as before, and the removal control baselines that control edge over the
unchanged graph, which must yield exactly one `STALE_BASELINE_EDGE` naming it. The requirement
text says so.

If the owner rejects the re-key, revert `4e50b0f39` and `d701c1243`: the edge returns to the
baseline and nothing else changes.

## V1-4809: the five upward imports

| Edge in the 2026-11-06 baseline | Disposition | Removed from the baseline in |
|---|---|---|
| `internal/runidentity -> internal/liveverify/affected/typescript` | package deleted by the decision 0594 cut | `7933d88de` |
| `internal/lspstdio -> internal/mcp/bridge` | package deleted with the LSP server (`55142ca92`) | `bed004663` |
| `internal/gokernel -> internal/observations` | both keyed `shared` by the five-layer re-key (V1-5233) | `4978a7791` |
| `internal/testsupport -> internal/testplatform` | both keyed `shared` by the same re-key | `4978a7791` |
| `internal/sessiontest -> .github/cishards` | `.github/cishards` keyed `tests` (above) | `4e50b0f39` |

None of the five is in `tools/import-ratchet/baseline.json`; `TestImportRatchet` passes over 318
packages with 0 baselined edges.

## V1-5249: the remaining criteria

1. No baseline edge remains, so none targets `docs`, `analyzers` or `release` from a lower or
   sibling layer; `TestImportRatchet` passes.
2. Behaviour unchanged: the 2026-11-11 entry recorded the focused tests of nine of the thirteen
   packages its splits touched. The other four (`jstestreceipt`, `rubytestreceipt`,
   `testdiscovery`, `testvaliditymodel`) have no test files; their code runs under the tests of
   the packages they were split from. At this lane's head these all pass (`script/heavy.sh`,
   `-count=1`, one package per command): `internal/jstestprovider`, `internal/rubytestprovider`,
   `internal/testvaliditydoc`, `internal/testsource`, `internal/flowcoverage`,
   `internal/appflows`, `internal/appmap`, `internal/doccorpus`, `cmd/corvint-docs`, and the
   corpus serving paths `internal/mcp/cli`, `internal/mcp/corpusbridge` and
   `cmd/corvint-docs-mcp`; `cmd/corvint` with `-run '^(TestCoreVerbsEmitTheFrozenProfiles|TestCLIKit)$'`.
   The whole `cmd/corvint` package was not run (lane rule: `-run` only). The re-key changes no Go
   file, so no CLI or MCP output can change.
3. The 2026-11-11 entry lists the sixteen edges it removed; this entry records the last one.

## V1-4807: what is met and what is not

- Criterion 1 (the step-1 baseline shrinks by the edges these packages caused): the 78-edge
  baseline of `a8a2dddf3` held four, `internal/cem/gitauth -> internal/cem/cemcode`,
  `-> internal/cem/patch`, `-> internal/cem/wire` and `internal/cem/gitrun -> internal/cem/cemcode`.
  `c7788871c` removed the two `cemcode` edges and `4978a7791` the two `gitauth` edges; none has
  returned since.
- Criterion 2: `tools/import-ratchet/direct_git_allowlist.txt` records every scanned direct Git
  spawn outside `internal/gitrun` with a reason, enforced by IRT-V0-020 (59 spawns at the base,
  down from the panel's 79 through deletions; none routed, because each listed site's result
  would change under `gitrun`, as IRT-V0-020 states).
- Criterion 3: `internal/gitrun`, `internal/cemcode` and `internal/cem/gitauth` tests pass at this
  head, and `go vet ./...` exits 0 (Commands).
- Not done: moving `internal/cem/gitauth`. `49a7d03df` kept it because it imports `cem/wire`
  and `cem/patch` (25 uses of `wire.IsGitOid`, plus `ValidatePath`, `ExcludedCEMPath` and
  `patch.MaxPatchBytes`), so a move would not take it out of CEM's dependencies, and six of its
  importers are under `internal/liveverify/affected/golang/`, which another lane owns this round.
  Whether to move it, and with what leaf for those names, is an owner call.

## V1-4814: the remaining criteria

- The step-4 traceability gate (V1-4803) stays green: `make traceability-tests-check` exits 0 at
  this head.
- Not done: `.github/cishards/test-slices.json` still names the deleted
  `TestCAPV0042EveryTreeNodeAnswersHelp` and `TestHFlagAliasesHelpFlag`. That is a `.github` edit,
  outside this lane.

## Commands

- `script/heavy.sh go test -count=1 -v ./tools/import-ratchet/`: PASS (318 packages, 0 edges,
  both IRT-V0-005 controls run).
- The focused package tests above, `internal/sessiontest` and
  `internal/liveverify/affected/liverepo` (selected by `corvint affected`): PASS.
- `corvint affected --base 366408098…`: scope UNKNOWN, 244 selected, 3 `LANGUAGE_FRONTIER`; tier 0
  is `tools/import-ratchet`, `cmd/corvint` and `liverepo`.
- `make -s spec-requirements-check requirement-definitions-check traceability-tests-check
  line-citations-check test-read-scopes-check`: exit 0.
- `GOTOOLCHAIN=local go vet ./...` (once, at the lane head `f0783e4c4`, under `script/heavy.sh`):
  exit 0, which meets the `go vet ./...` criterion of V1-4807 and V1-4809. `make windows-vet`:
  exit 0. `make changed-checks BASE=366408098…`: ok.

## Rollback

Revert `4e50b0f39` and `d701c1243`; the baseline regains its one edge.
