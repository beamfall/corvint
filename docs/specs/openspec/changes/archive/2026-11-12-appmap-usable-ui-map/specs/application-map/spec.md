# Spec delta: application-map (appmap-usable-ui-map)

## ADDED Requirements

### Requirement: AMAP-V0-970 React routes mapped over a same-file literal array

In a `react-router/0` router file, a JSX child `{ARR.map(P => <Route .../>)}` MUST expand into one
route per element of ARR when ARR is bound by exactly one non-exported `const` in the same file (at
any depth) to an array literal, or to `useMemo(() => [...], deps)` returning one, every element is
an object literal, and every route attribute that names `P` reads a member holding a literal string
(`path={P.path}` or a template such as `` path={`${P.path}`} `` whose substitutions are only such
members). A `key` attribute MUST be ignored. Such routes change what a build derives, so
`appmap.BuilderVersion` MUST be `appmap-builder/4`.

#### Scenario: Console sections

- **GIVEN** `const sections = [{path: 'users'}, {path: 'roles'}]` and
  `<Routes>{sections.map(s => <Route key={s.path} path={s.path} element={<Page s={s}/>}/>)}</Routes>`
- **WHEN** `flows appmap build` reads the file
- **THEN** it emits the screens `/users` and `/roles` and no `non-literal-react-children` unknown

#### Scenario: useMemo array and template path

- **GIVEN** `const sections = useMemo(() => [{path: 'a'}], [])` and
  `` {sections.map(s => <Route path={`${s.path}`} .../>)} ``
- **WHEN** the build reads the file
- **THEN** it emits the screen `/a`

### Requirement: AMAP-V0-971 Unproven arrays keep the uncertainty

A `.map` child MUST stay the unknown `non-literal-react-children`, and expand nothing, when ARR has
a second declaration, is exported, imported or a parameter, escapes into any use other than
`.map(`, `.forEach(`, `.length` or a `for ... of` iterable (a mutator call or an index assignment
is such a use), a template substitution or escaped identifier could name it, any element is not an
object literal, or a route attribute reads `P` as anything but a member holding a literal string.

#### Scenario: Mutated array

- **GIVEN** the AMAP-V0-970 file plus `sections.push({path: 'x'})`
- **WHEN** the build reads it
- **THEN** it reports `non-literal-react-children` and no route from the array

#### Scenario: Non-literal element

- **GIVEN** an element `{path: computePath()}`
- **WHEN** the build reads the file
- **THEN** it reports `non-literal-react-children` and no route from the array

### Requirement: AMAP-V0-972 Gotos in a literal for-of loop

A `page.goto`, `waitForURL` or `toHaveURL` argument that is the variable X of an enclosing braced
`for (const X of ITER)` loop, or a template whose every substitution is X, MUST resolve to one goto
per value when ITER is an inline array of exact strings or a same-file array bound under the
AMAP-V0-971 rules whose elements are exact strings, and no other binding of X lies between the
loop and the use. `flows appmap build` records one goto per value on the goto's line; `flows appmap
infer` reads a `test(...)` call inside the loop as one test per value, with X bound to the value
and the title filled, when the filled titles are distinct. Any other argument keeps
`non-literal-url`.

#### Scenario: Visit every page

- **GIVEN** `const PAGES = ['/web/a', '/web/b']` and
  `for (const r of PAGES) { test(`visits ${r}`, async ({page}) => { await page.goto(r) }) }`
- **WHEN** the map is built and coverage and infer run
- **THEN** both screens are reached, attributed with basis `goto`, and infer starts one test at each

#### Scenario: Shadowed variable

- **GIVEN** the same loop whose callback declares `const r = other()`
- **WHEN** the map is built
- **THEN** the goto keeps `non-literal-url`

### Requirement: AMAP-V0-973 Navigation helpers called from specs

A spec or workflow file's call to an AMAP-V0-342 navigation helper (same file or imported from a
test file) with a literal argument MUST attribute the screen that helper call places to the spec,
with basis `helper-goto` and the call line as its `via`, and record the candidate in the file's
`helper_gotos`. A call whose URL places no screen attributes nothing.

#### Scenario: Same-file helper

- **GIVEN** `async function gotoManage(page, sub) { await page.goto(`/web/manage/${sub}`) }` and
  `await gotoManage(page, 'users')` in one spec
- **WHEN** the map is built and coverage runs
- **THEN** `/manage/users` is reached through the spec with basis `helper-goto`

### Requirement: AMAP-V0-974 Coverage and infer agree; unresolved gotos stay unknown

The build and infer MUST resolve loop gotos through one shared resolver. On a fixture whose specs
navigate by literal, loop and helper gotos, the screens coverage reports reached MUST equal the
start screens infer reports. A goto that does not resolve MUST keep its reason and MUST NOT make a
screen reached.

#### Scenario: Dynamic array

- **GIVEN** `for (const r of loadPages()) { ... page.goto(r) }`
- **WHEN** the map is built and coverage runs
- **THEN** the goto keeps `non-literal-url` and no screen is reached by it

### Requirement: AMAP-V0-975 Subcommand help

`corvint-docs [--root PATH] flows appmap SUB ...` MUST print SUB's own help (usage, flags, required
inputs and one example) when `--help` or `-h` appears anywhere before `--`, and so MUST
`corvint-docs help flows appmap SUB`. A SUB that is not a command keeps the existing behavior.

#### Scenario: Flags before help

- **GIVEN** `corvint-docs --root R flows appmap coverage --map M --full --help`
- **WHEN** it runs
- **THEN** it prints the `flows appmap coverage` usage, flags and example, not the `flows` help

### Requirement: AMAP-V0-976 Working-tree build opt-in

`flows appmap build --worktree` MUST build from the working tree of `--root` (tracked files with
their uncommitted edits and deletions, and untracked files the repository's ignore rules do not
exclude) without writing to the repository, and the map MUST carry
`working_tree: {"base_revision": <HEAD or "">, "evidence": "unpinned"}`. The same working tree
MUST give the same map. Readers MUST decode the new member (AMAP-V0-960).

#### Scenario: Uncommitted manifest

- **GIVEN** a repository whose manifest, router and specs are untracked
- **WHEN** `flows appmap build --manifest M --worktree` runs
- **THEN** it writes a map with `working_tree.evidence` `unpinned`, and `git status` and the
  object store of the repository are unchanged

### Requirement: AMAP-V0-977 Default refusal names the opt-in

Without `--worktree`, a build MUST keep refusing a manifest absent at the revision, and when that
manifest exists as a working-tree file the message MUST name `--worktree`. `--worktree` MUST refuse
with `--revision`, `--repo`, `--manifest-repo`, `--repo-revision`, `--workspace`, `--from`, `--verify`
or an implicit workspace file.

#### Scenario: Uncommitted manifest without the opt-in

- **GIVEN** an untracked manifest
- **WHEN** `flows appmap build --manifest M` runs
- **THEN** it refuses `appmap-invalid-manifest` and the message names `--worktree`

#### Scenario: Pinned flags

- **GIVEN** any repository
- **WHEN** `flows appmap build --manifest M --worktree --revision HEAD` runs
- **THEN** it refuses with the usage error and builds nothing
