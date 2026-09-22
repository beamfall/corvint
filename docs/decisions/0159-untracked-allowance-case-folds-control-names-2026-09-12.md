# Decision 0159 — Untracked allowance case-folds control names

Date: 2026-09-12. Status: accepted. Authority: repository owner instruction, 2026-09-12,
delegated coordinator call. Base: `5b4961197a36e063f0ff392c6984a97d73b05622`.

`internal/untrackedallowance/allowance.go` (`OverlapsGoBuild`) matched the Go/Git control
basename set and the `vendor` path segment with plain string equality. On a case-insensitive
filesystem (macOS APFS, the default here) an untracked path that differs from a control name
only in case is the same directory entry the Go toolchain and Git actually resolve, so the
exact-match check let a build-changing untracked path through as `UNTRACKED-ALLOWED`.

Measured on this darwin host (scratch module, `git status --porcelain=v1 -z
--untracked-files=all`): a committed `internal/x/x.go` makes `go list ./...` report
`m/internal/x`. Adding an untracked, lowercase `internal/go.mod` (a plain control basename)
changes that to `go: warning: "./..." matched no packages`; adding an untracked
`internal/GO.MOD` instead, with no other change, produces byte-identical output. Before this
decision, `OverlapsGoBuild("internal/GO.MOD", …)` returned `false` (exact match on
`goControlNames["go.mod"]` fails), so range impact would have bound it as allowed while it
demonstrably changes the Go build view. The `.gitignore` case variant in the same bug report was
verified rather than inferred: an untracked `.GITIGNORE` in a repo with `core.ignorecase true`
is read by `git check-ignore` as `.gitignore:1:...` and successfully hides a matching untracked
path from `git status`, so it has the same live effect as the lowercase control name.

The owner call: fail closed on every host. `OverlapsGoBuild` case-folds both the control-basename
lookup and the `vendor` segment comparison unconditionally, without branching on the host's actual
filesystem case sensitivity. Folding on a case-sensitive host only loses an allowance (a path is
classified as overlapping when it need not be), which is the safe direction; folding on a
case-insensitive host is required for GPK-V0-060's disjointness proof to hold. This amends
GPK-V0-060's closed list, restated as case-insensitive, and requires no new requirement ID: the
overlap conditions are unchanged in substance, only the comparison semantics of two of them.

Rule bump: `Rule` moves from `corvint-untracked-allowance/0` to `corvint-untracked-allowance/1` since
the classification a receipt's `range.untrackedAllowance` attests to has changed (a path
previously excluded from the allowed set under some case-insensitive collision is now refused
outright). No other production package reads the literal rule string; every caller compares
against the exported `untrackedallowance.Rule` symbol.

Not applied to the `.go`-suffix directory-entry check or to the package-directory containment
check: `go build`'s own source-file scan requires an exact lowercase `.go` suffix regardless of
host filesystem, so an untracked `FOO.GO` is not compiled as a Go source file by that rule; a
same-directory collision with a tracked `foo.go` is already caught by the package-directory
containment branch, which is directory-keyed and unaffected by this change.

Rollback: revert `strings.ToLower`/`strings.EqualFold` in `OverlapsGoBuild` to plain equality and
restore `Rule = "corvint-untracked-allowance/0"`. A receipt already bound under `.../1` then names a
rule whose exact classification predicate no longer exists.
