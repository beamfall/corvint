# 2026-10-21 design-1021: declared documentation paths and scope-bounded literal readers

Lane design-1021 (pre-1.0 ticket sweep). Tickets V1-0923, V1-0922, V1-1766 and, at the coordinator's
request, V1-2069. Base `c9ca8beff43d8f548b054d3b3d40e3054cecf816`. AFP-V0-160 and AFP-V0-161 and the
AFP-V0-023 rule (c) amendment are accepted under the owner's standing auto-accept (2026-10-11).

## V1-0923: DELIVERED as AFP-V0-160

Problem: a diff that only adds a `docs/build-log/` entry (decision 0423) raised `UNOWNED_DIRTY_PATH`
and made the affected plan `UNKNOWN`, widening every documentation-only lane to the full gate.

Decision: a repository declaration, `.corvint/documentation-paths.json`
(`corvint-documentation-paths/0`), modelled on the AFP-V0-088 task-store class. Declared paths that
would raise `UNOWNED_DIRTY_PATH` move to a `DECLARED_DOCUMENTATION` `nonSource` group; reader rules are
unchanged, so every literal, read-scope or unbounded reader is still selected. This repository
declares only `docs/build-log/`. The product hard-codes no layout; the declaration cannot name
`.corvint/`. `internal/gateselect` and `tools/gate-affected-select` are unchanged (wider, safe).

Evidence: six `_AFPV0160` tests in `internal/liveverify/affected/documentation_test.go`; with the
`Select` call removed the classification tests fail. At base, `corvint affected` over a probe
build-log file reported `UNKNOWN` with `UNOWNED_DIRTY_PATH`; the lane binary reports `BOUNDED` with the
path in the new group.

## V1-0922: BLOCKED, existing behaviour kept (no code change)

The charter said to stop if indexing nested Go modules proved unbounded or broke parity tests. A
prototype that listed nested modules: (1) emitted `interop/cem01-go` and `tools/local-authority`
patterns in `provider.go.packages` that do not resolve from the root module offline (with network,
`go list -mod=mod` fetched a stale published copy and rewrote `go.mod`); (2) failed 7 existing golang
plugin tests (AFP-V0-031/V1-0867 nested-module frontier, AFP-V0-050 read-scope ownership, `go.work`
workspace semantics). Indexing needs a per-module run root on the provider wire, a contract change.
Residual: a nested-module-specific unknown (the ticket's acceptance allows it) or a per-module run
root profile; to be filed.

## V1-1766: basename part ALREADY DELIVERED; scoped narrowing added as AFP-V0-161

The `.taskman/policy.json` basename over-selection is already fixed by AFP-V0-072
(`namesFileOnlyWithin`, ea0230ed4, V1-1806): those readers no longer resolve on the original path.
Remaining over-selection comes from whole-segment anchored (`/x`), climbing (`../x`) and file-name
literals joined to a root supplied at run time; no literal index can rule those out without dropping
real readers, so literal matching is not narrowed.

AFP-V0-161 uses project authority instead: a package with an AFP-V0-023 read-scope declaration is not
a rule (c) reader of a path outside its scope, because the declaration states it and full CI confines
the tests with Landlock. Every in-scope path keeps its relation
(`TestDeclaredReadScopeKeepsEveryInScopeLiteralReader_AFPV0161`). Measured on a probe
`testdata/x/policy.json`: resolving namers 52 -> 22; tasks/intent, journal and archive dropped.
Still selected: `cmd/corvint` and `internal/tasks/cli` (undeclared; whole-segment literals).

## V1-2069: mechanism delivered, acceptance not met

A read-scope declaration now bounds rule (c), so decision 0548's route works for any declared
package. Ticket-path namers 12 -> 10 at this tree. What acceptance still needs: a declaration for
`internal/tasks/transaction` with Landlock CI evidence, and the remaining full-path literal holders
(`cmd/corvint`, `internal/contextindex`, `internal/gateselect`, `internal/liveverify/affected`,
`internal/taskprojection`, `internal/cemchain`, `internal/opencodequalification`,
`internal/tasks/cli`) plus two shell scripts, which cannot be declared and genuinely name the path.

## NOT_RUN

`make gate` (charter: focused only); full CI Landlock confinement of the new rule (CI runs it on
the PR); the `tools/gate-affected-select` mirror is intentionally unchanged.

## Rollback

Revert this lane's commit: delete `documentation.go`, its test, `.corvint/documentation-paths.json`,
the graph `documentation` field/digest member and `Select` call (AFP-V0-160); delete the read-scope
guard at the top of `resolves` and `readscope_literal_test.go`, and restore the AFP-V0-023 rule (c)
sentence (AFP-V0-161). No persisted state or wire profile outside the plan's `nonSource` changes.
