# 2026-10-06: V1-0867 nested-module frontier decided per module

## Intent

Ticket V1-0867 (P2, bug). The Go `affected` plugin raised `go:nested-module-frontier` whenever any
`go.mod` below the root was not a `go.work` use. That widened every Go plan to `UNKNOWN`, even
when no nested module could import a root package. The ticket asks the plugin to close the frontier
when every nested manifest is read and none requires or replaces the root module path. It must
keep the frontier open on any doubt, and must leave the reverse direction (a change inside a
nested module) as it is.

## Change

- Spec `docs/specs/affected-plan-v0.md` adds the proposed AFP-V0-028, which narrows the accepted
  AFP-V0-008 frontier, with non-goals, rollback, a traceability row and the intent-status mention.
  AFP-V0-008 gains only a cross-reference.
- `internal/liveverify/affected/golang/nested.go` is new. `nestedModules` produces one evidence
  record (`Manifest`, `Open` reason) per unlisted module's `go.mod`, read through the plugin's
  `affected.Source`. `observeModules` raises the frontier only when a record is open. Each module
  is judged on its own.
- The reader is a bounded (1 MiB) go.mod tokenizer. It knows `//` comments, interpreted and raw
  strings, one-line directives and parenthesized blocks, and the verbs module, go, toolchain,
  godebug, require, exclude, replace, retract, tool and ignore. Anything else fails, so the module
  stays open.
- `testdata/workspace/stray/go.mod` now requires a listed module, so the existing workspace tests
  still observe an open nested frontier.

## Decisions (fail closed where the spec was silent)

- A module stays open when:
  - it requires or replaces an observed module path, replaces a module with one, or names a tool
    under one (its own tools excepted);
  - a directory replacement is absolute, uses Windows path syntax, lies outside the repository, or
    resolves anywhere except its own directory or another unlisted module. A relative replacement
    that resolves to the root, or into an observed module, therefore counts as a replace;
  - a `go.work` sits in the module's directory or any ancestor below the root, because the go tool
    would use it;
  - any observed module path is unresolved, so no requirement can be ruled out.
- An unreadable, over-size or unparsable manifest is uncertainty: the module stays open. Under
  `BuildFS`, a read error stays the existing fatal build error, which is stricter than uncertainty.
- Evidence stays internal. `affected.Result` has no evidence member, and adding one would change
  the plan wire and the graph digest. The tests read it through `export_test.go`.
- Reverse direction: unchanged. A nested module's files are never units, so a change inside one is
  still `UNINDEXED_SOURCE_PATH` or `UNOWNED_DIRTY_PATH`, and a test asserts this.

## Finding: the ticket premise does not hold on this repository

This repository has four nested modules. Three of them close:
- `benchmarks/snapshot-reader`
- `conformance/interactive-alpha/fixture`
- `interop/cem01-go`

The fourth, `tools/local-authority/go.mod`, stays open with `requires github.com/Beamfall/corvint`.
It requires the root module and replaces it with `../..`, and it imports `internal/authoritystore`,
`internal/localauthority` and `internal/cem/...`. It has been present since the initial public
snapshot and is present at every survey merge base. Keeping the frontier for it is correct, so on
this repository the fix does not drop the unknown.

Replay: the five survey ranges were replayed in a scratch `--shared` clone, never in the survey
worktrees. Each replay applied the Go-only part of `pre..post` as one commit on `pre` and ran
`corvint affected --base pre`, with binaries built from `3dc863bd` and from this branch:

| PR | base | fix |
|---|---|---|
| 571 | `UNKNOWN`, 79 selected, `go:build-constraint-variants`, `go:nested-module-frontier` | identical |
| 614 | `UNKNOWN`, 51, same two | identical |
| 594 | `UNKNOWN`, 70, same two | identical |
| 557 | `UNKNOWN`, 52, same two | identical |
| 610 | `UNKNOWN`, 51, same two | identical |

Counterfactual: 571 was replayed again on a base commit that deletes `tools/local-authority`. Base
binary: `UNKNOWN`, 79, `go:build-constraint-variants` and `go:nested-module-frontier`. Fix binary:
`UNKNOWN`, 79, `go:build-constraint-variants` only. The fix drops the nested unknown once no root-requiring
module exists. The plan stays `UNKNOWN` for `go:build-constraint-variants`, which is out of scope.

Owner question (not implemented): attach the frontier as a unit-level `Frontier` on the root
packages an open nested module imports directly. If a target is unknown, absent or unreadable, the
frontier would stay at plugin level. Most survey plans would then lose the nested unknown. This is
a reachability design change, not part of this fail-closed fix.

## Limits and NOT_RUN

- Not covered: a nested module's tests that read observed data (AFP-V0-012), symlinked manifests
  the walk does not follow, and a `GOWORK` environment value.
- `make gate`: NOT_RUN, per the owner's standing preference for scoped work. The focused affected
  tests and the doc gates were run. See the commit and handoff for exact results.

## Evidence

- New tests in `internal/liveverify/affected/golang/nested_test.go`, all `_V1_0867`. With the old
  unconditional frontier forced back, `TestIndependentNestedModulesCloseTheFrontier_V1_0867` and
  `TestNestedFrontierReadsTheSuppliedSource_V1_0867` fail. With the fix, the package passes.
- Scratch run on `git archive 3dc863bd`: three modules closed and `tools/local-authority/go.mod`
  open, as listed above.
