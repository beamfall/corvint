# 2026-10-06: V1-0867 nested-module frontier decided per module

## Intent

Ticket V1-0867 (P2, bug). The Go `affected` plugin raised `go:nested-module-frontier` whenever any
`go.mod` below the root was not a `go.work` use. That widened every Go plan to `UNKNOWN`, even
when no nested module could import a root package. The ticket asks the plugin to close the frontier
when every nested manifest is read and none requires or replaces the root module path. It must
keep the frontier open on any doubt, and must leave the reverse direction (a change inside a
nested module) as it is.

## Change

- Spec `docs/specs/affected-plan-v0.md` adds the proposed AFP-V0-031, which narrows the accepted
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
  - an observed `go.mod` or the root `go.work` requires it, replaces it, names a tool under it, or
    replaces a module with its directory (added after review, below);
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

## Review repairs (Codex round 1)

Codex reported three findings. Each was checked against the go 1.27.1 `x/mod/modfile` lexer and
rules, and each was repaired with regression tests that fail without the repair:

- P1, confirmed: a nested manifest alone cannot establish independence. If the root requires and
  replaces a nested module, the root build compiles the nested module's packages, and those
  packages resolve imports of root packages against the main module without any `require` line
  in the nested `go.mod`. `observedReach` now parses every observed `go.mod` and the root
  `go.work`. A nested module stays open when the observed side requires it, replaces it, names a
  tool under it, or replaces a module with its directory. An observed directory replacement that
  is absolute or leaves the repository opens every nested module.
- P1, confirmed: `readModuleSourcePath` only trims quotes, so `module "example.test/ro\x6ft"` was
  read as a literal escape. Observed manifests are now parsed with the strict reader (which uses
  `strconv.Unquote`). Any disagreement with the path the plugin read opens every nested module.
  `readModuleSourcePath` itself is unchanged, because unit naming is outside this ticket's scope.
- P2, confirmed: known verbs with malformed arguments closed the frontier. The reader now checks
  each directive the way `modfile` does:
  - `go` and `toolchain` use the toolchain's version patterns and may appear only once;
  - `godebug` needs `key=value`;
  - `require` and `exclude` take two arguments, the second a `v`-prefixed version;
  - `retract` takes a version or a `[v, v]` interval;
  - `tool`, `ignore` and `use` take one argument;
  - a replacement version must be `v`-prefixed;
  - the lexer splits on `()[]{},` as the go tool does, and rejects non-printable runes, invalid
    UTF-8 and `/*` comments.

## Review repairs (Codex round 2)

Codex reported two further findings. Both were confirmed against x/mod v0.39.0 `rule.go`, and
both were repaired with regression tests. With the round-1 `nested.go` restored, all five new
subtests fail; with the repair, the package passes.

- P1, confirmed: an observed directory replacement was matched against nested directories by
  exact name only. On a case-insensitive filesystem, `replace example.test/alias => ./NESTED`
  names `nested/`, and the frontier closed. A linked path has the same effect. The new
  `unknownDirectory` opens every nested module unless each observed directory replacement is
  exactly an observed module directory, or lies at or below an unlisted module's directory. The
  nested side already accepted only exact matches, so it was not affected.
- P2, confirmed: `go`, `toolchain` and `godebug` are validated against the raw token in
  `modfile`, so a quoted argument is a parse error there. The reader now rejects these
  arguments when they are quoted.

## Review repairs (Codex round 3)

Codex reported three P2 findings, all forms that `modfile` refuses but the reader still
accepted. All three were confirmed against x/mod v0.39.0 and repaired. With the round-2
`nested.go` restored, all six new subtests fail; with the repair, the package passes.

- Raw strings: `parseString` rejects any token that is not `"`-quoted but contains a quote
  character. The lexer now refuses a raw string outright, and a quote inside an unquoted
  argument is a parse error.
- Blocks: only the verbs `modfile` admits as blocks are allowed (go.mod: godebug, require,
  exclude, replace, retract, tool, ignore; go.work: godebug, use, replace). `go (` and
  `toolchain (` now fail. A `module` block stays refused, which is stricter than `modfile`.
- Versions: a version must now be the canonical form (`vMAJOR.MINOR.PATCH`, an optional
  prerelease, an optional `+incompatible`). The go tool canonicalizes some shorthand forms, so
  refusing them is stricter than the go tool and only keeps a module open.

The binary was rebuilt and the replays repeated with the final code:
- the 571 counterfactual is `UNKNOWN`, 79, with `go:build-constraint-variants` only;
- real 571 and real 610 still keep `go:nested-module-frontier`.

## Review repairs (Codex round 4)

Codex reported two P2 findings. Both were confirmed against x/mod v0.39.0 (`parseReplace` and
`module.CheckPathMajor`) and repaired. With the round-3 `nested.go` restored, all eight new
subtests fail. One of them, a module replacement without a version, was already open there,
but for a directory reason.

- Path/major compatibility: `require`, `exclude` and a versioned replacement source must pass
  the same check as `SplitPathVersion` and `CheckPathMajor`, including the gopkg.in forms and
  the `.v1` `v0.0.0-` pseudo-version allowance. An invalid path suffix fails too.
- Replacement form: a target without a version must be a directory path, and a directory target
  must not carry a version.

Rationale, recorded for the owner: a manifest the go tool rejects cannot be built at all, so a
laxer read could not hide a dependency that actually builds. Still, AFP-V0-031 says
"unparsable stays open", so the reader may be stricter than `modfile` but never laxer.

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
After the review repairs, the binary was rebuilt and the run repeated. Real 571 was still
`UNKNOWN`, 79, with both unknowns. The 571 counterfactual was still `UNKNOWN`, 79, with
`go:build-constraint-variants` only.

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
