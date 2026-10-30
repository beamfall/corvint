# Decision 0131 — gate-affected attributes dirty paths from a static index of imports and path literals

Date: 2026-09-12. Status: accepted, delegated call. Authority: repository owner delegation to make
owner calls and record them (orchestration thread, 2026-09-12). Governs `AFP-V0-012` and the
amended `AFP-V0-011` in `docs/specs/affected-plan-v0.md`.

## Context

Commit `48f089a8` made `make gate-affected` sound by falling back to the full `./...` run whenever a
dirty path was a document, a testdata fixture, or unowned, because any package's test may read such
a path. Replayed on the 50 first-parent commits ending at `09e4bc2f`, that rule fell back on 48.
Three defects were also open. A deleted `.go` file selected only its own package, not the packages
importing it. A Go file read as data by a package that does not import it (the pattern
`tools/local-authority` shows) was never selected. A dirty path under a newline-bearing directory
could be in neither `provider.go.packages` nor `plan.unknown`.

## Call

The selector (`tools/gate-affected-select`) indexes the repository under test itself: package
directories, imports, and the string literals each file carries, lexed with `go/scanner` and split
into path tokens. It selects, for every dirty path:

- the path's package and its dependents for Go source;
- the enclosing packages for any other path;
- every package whose literals name the path;
- every package whose reads no literal bounds, once any path is dirty.

A package that cannot be attributed is selected rather than forcing a global fallback. Global
fallback stays only when the index itself fails: a bound, an unreadable file, imports that do not
parse. A control character in any dirty path also falls back, which closes the newline-directory
defect at the consumer. Dependents are keyed by directory, so a deleted file or package still
reaches its importers.

"Unresolved" means one of these in the package's own files:

- `runtime.Caller`, `os.Getwd` or `--show-toplevel`;
- in tests, `..`-only tokens, a token resolving to the root or above, or a `./...` pattern;
- in non-test code, a `..` climb to a named component.

A package that depends on non-test code meeting any of those conditions is also unresolved.

## Alternatives weighed

1. **Declared reader mappings.** A hand-maintained table of which package reads which path was
   rejected. It goes stale silently, and a stale entry is an unsound narrowing.
2. **Propagating a dependency's literals to its dependents.** Every package importing a holder of
   `"docs/agent-"` would read `docs/agent-memory/fixes.md`. This was measured first. The docs-only
   commit `09e4bc2f` selected 128 packages, 74 of them as readers through `internal/contextindex`.
   It was rejected for the rule below.
3. **Chosen: dependency literals select only their holder.** A dependency's path literal resolves
   against a root or directory its caller supplies, and the caller's own literal or root-locating
   signal already selects the caller. A dependency that locates the root itself marks all its
   dependents unresolved.
4. **`go list -deps -test` per package.** Rejected for this slice. It needs the toolchain and a
   build-list resolution for each run, and it answers imports, not file reads.

## Evidence

- The replay (AFP-V0-011) falls back on 0 of 50 commits, down from 48. The mean selection is 112.9
  of 184 packages (minimum 99, median 109, maximum 138).
- The floor is 95 unresolved packages at `09e4bc2f`.
- `b5c04a72`, which the pre-`48f089a8` rule narrowed past its broken `internal/specindex`, now
  selects that package as a reader of `docs/decisions/README.md`.
- `TestSelectPackagesAttributesEveryDirtyPath` and `script/gate-affected_test.sh` cover the three
  defects and each attribution rule.

## Residual gaps

These are accepted because the fast tier is not the push gate. `make gate` stays mandatory, and
AFP-V0-011's 200-commit shadow run still gates any promotion.

- **Roots the index cannot see.** A test takes its root from an environment variable, a flag, or a
  subprocess other than `git rev-parse --show-toplevel`, with no other signal.
- **Fragmented paths.** A path is assembled only from fragments shorter than one whole component.
- **Production `..` literals.** A bare `..` or `../` in production code is assumed to reject a path,
  not to climb from the working directory.
- **Nested modules.** A nested module's tests, such as the `internal/wp3codec/codec.go` read in
  `tools/local-authority`, are outside `./...` in both the narrowed and the full run.
- **The floor.** 95 packages are selected on every change. Lowering it needs data flow from a
  root-locating call to a read, which this slice does not attempt.

## Rollback

Restore `tools/gate-affected-select/main.go` from `48f089a8`, delete `readers.go`, and revert
AFP-V0-011 and AFP-V0-012 in the spec. The data-path fallback returns, and `make gate` is
unaffected.
