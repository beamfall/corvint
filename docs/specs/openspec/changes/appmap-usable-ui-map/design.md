# Design: appmap-usable-ui-map

## Decisions

### React `.map` routes (AMAP-V0-970, AMAP-V0-971)

`reactJSX` reads a `{...}` JSX child as an opaque `jsx-expression`. The new reader accepts exactly
`ARR.map(P => <Route .../>)`, with `(P)` and a parenthesised element allowed, and nothing else in
the braces. `ARR` resolves through `constArray`, shared with the goto resolver:

- exactly one `const ARR = [...]` declaration in the file, at any depth, not exported, optionally
  typed; or `const ARR = useMemo(() => [...], deps)` (also `React.useMemo` and `=> ([...])`);
- every other use of `ARR` is `ARR.map(`, `ARR.forEach(`, `ARR.length` or the iterable of a
  `for (... of ARR)` header, so the array never escapes to code that could change it; a mutator
  call (`push`, `pop`, `shift`, `unshift`, `splice`, `sort`, `reverse`, `fill`, `copyWithin`) or
  index assignment is such a use;
- no template substitution in the file mentions `ARR` and no identifier is escaped.

Each element must be an object literal. In the element's tokens, `P.member` is replaced by the
member's literal string when the member is a literal string, and a template literal whose every
substitution is such a `P.member` becomes the joined string. The route attributes (`path`, `index`,
`id`) must then be literals; `P` may appear in any form inside `element` (its tokens are kept for
the AMAP-V0-882 comparison). Any element that fails these rules leaves the whole child as
`jsx-expression`, so `non-literal-react-children` stays. A `key` attribute names React
reconciliation, not a route option, and is ignored everywhere. A `.map` child that expands under
these rules is no longer a dynamic construct for the file's `dynamic-route-context` check; any other
`.map` still is.

`lexRoutes` now keeps a template literal's raw source in `code`, as `lexJS` does, so the template
rule can see its substitutions. Because these routes change what a build derives from the same
inputs, `BuilderVersion` becomes `appmap-builder/4` (AMAP-V0-215).

### One goto resolver (AMAP-V0-972 to AMAP-V0-974)

`gotoresolve.go` holds `loopValues(toks, at, name)`: walking out from the goto, each enclosing
`{` preceded by `for (const X of ITER)` with `X == name` binds it. `ITER` is an inline array of
exact strings or a `constArray` of exact strings. A second binding of `X` between the loop and the
use (a declaration, function or arrow parameter, or `catch` binding) refuses.

- Build: the goto argument is an exact literal (unchanged), the loop variable, or a template whose
  every substitution is the loop variable; one `rawGoto` per value, all on the goto's line.
- Infer: a `test(...)` call inside such a loop becomes one unit per value with the loop variable
  bound as a literal, and its title filled from the template, when the filled titles are distinct.
  Otherwise one dynamic unit stays, as today.
- Helpers: a spec or workflow now gets the AMAP-V0-342 helper candidates its calls make; each one
  that places a screen attributes it with basis `helper-goto` and the call line, and is kept in the
  file's `helper_gotos`.

Coverage counts a screen reached only from the attributions in `specs`, so an unresolved goto keeps
its `non-literal-url` or `no-matching-screen` reason and never adds a screen.

### Subcommand help (AMAP-V0-975)

`parseHelpInvocation` strips the `--root` preamble. When the leading words are exactly a
`flows appmap <sub>` node of the command tree and `--help` or `-h` appears before `--`, or the
request is `help flows appmap <sub>`, the topic is `node:flows appmap <sub>` and `helpText` prints
`clitree.Render` for that node: usage, flags, required inputs and one example.

### Working-tree build (AMAP-V0-976, AMAP-V0-977)

`BuildWorktree(ctx, root, manifest)` writes nothing in the real repository:

1. `git init` a temporary directory `T` whose `objects/info/alternates` names the real object
   directory, and copy the real `info/exclude`.
2. With `--work-tree=root`, `read-tree HEAD` (when HEAD exists) and `add -A` into `T`'s index, so
   tracked files, staged and unstaged edits, deletions and untracked files that `.gitignore` does
   not exclude are the snapshot. New blobs go to `T`'s objects only.
3. `write-tree`, then `commit-tree` with a fixed identity and date, so the same working tree gives
   the same commit.
4. `BuildRoots(T, manifest, commit)`, then set `working_tree` to
   `{"base_revision": HEAD or "", "evidence": "unpinned"}` and remove `T`.

The CLI refuses `--worktree` with `--revision`, `--repo`, `--manifest-repo`, `--repo-revision`,
`--workspace`, `--from` or an implicit workspace file. The default refusal for a manifest absent
at the revision but present as a working-tree file names `--worktree`.

## Failure modes

- The user's global excludes file is not read (the Git environment carries no global
  configuration), so untracked files only it ignores enter a `--worktree` snapshot.
- A `--worktree` map names a snapshot commit the real repository does not hold; verbs that re-read
  source at the map revision refuse it as an unknown revision.
- A shadowing binding the conservative scans miss could give a route or a goto the wrong value.
  Both scans refuse on any second declaration or parameter binding of the name.

## Acceptance evidence

`TestAMAPV0970*` to `TestAMAPV0977*` in `internal/appmap` and `cmd/corvint-docs`, the
per-package test runs, and the lane binary on the reproduction fixture (coverage before and after,
infer start screens, `--help` output, a `--worktree` build of an uncommitted fixture).

## Rollback

Revert the lane's commits; maps written by `appmap-builder/4` rebuild once under `--from` with
the older binary, and a `--worktree` map must be rebuilt.
