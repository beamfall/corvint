# Change proposal: a usable UI map for a react-router web app

Change name: `appmap-usable-ui-map`
Tickets: V1-5467 (React routes made by `.map` over a same-file array), V1-5468 (coverage misses
gotos in literal loops and same-file helpers), V1-5471 (`flows appmap <sub> --help`) and V1-5472
(`flows appmap build --worktree`).
Product requirement served: PROD-DOCS-006 (accepted, decision 0596): Corvint maps an application's
screens and routes to flows and E2E tests.
Owning capability spec: `docs/specs/application-map-v0.md` (prefix `AMAP-V0`).
Requirement IDs this change adds: AMAP-V0-970 to AMAP-V0-977. No `AMAP-V0-97x` ID exists in `docs`
on `origin/main` at `072760041` or on the 40 most recently updated remote branches (checked
2026-11-12).
Intent status: auto-accepted under G14 (decision 0596): it implements the accepted PROD-DOCS-006
and adds no product requirement.
Delivery status: in progress.
Written before dispatch: yes, committed before any code in the lane.
Layout: this folder follows G14 (`proposal.md`, `design.md`, `tasks.md`,
`specs/application-map/spec.md`).

## Why

Building a full UI map of a React web app (react-router, base path `/web`) with
`corvint-docs flows appmap` stopped at four places:

- A console page declares its sections in one array and renders
  `{sections.map(s => <Route path={s.path} .../>)}`. The build reads that child as
  `non-literal-react-children` and the whole console collapses into one `/console/{*splat}`
  screen.
- Specs that visit every page in `for (const r of PAGES) { ... page.goto(r) }`, or through a
  same-file helper `gotoManage(page, 'users')`, leave those screens unreached in
  `flows appmap coverage`. On the reproduction fixture coverage reported 1 of 5 screens reached;
  `flows appmap infer` resolved the helper calls but not the loop, so the two verbs disagreed.
- `flows appmap <sub> --help` printed the generic `flows` help whenever a flag came before
  `--help`, the `--root` preamble was present, or the request was `help flows appmap <sub>`.
- `flows appmap build` refuses a manifest that exists only in the working tree, with no way to map
  an uncommitted checkout and no hint in the refusal.

## What Changes

- A `.map` child over a same-file literal array of objects expands into one route per element
  (AMAP-V0-970); anything the reader cannot prove literal keeps `non-literal-react-children`
  (AMAP-V0-971). The builder version moves to `appmap-builder/4`.
- One goto resolver serves the build and infer: a `for (const X of ARR)` loop over literal strings
  gives one goto per value (AMAP-V0-972); a spec's call to a navigation helper with a literal
  argument attributes the screen (AMAP-V0-973); coverage and infer agree and an unresolved goto is
  never counted as reached (AMAP-V0-974).
- `flows appmap <sub>` prints that subcommand's own help for `--help` or `-h` anywhere before `--`
  and for `help flows appmap <sub>` (AMAP-V0-975).
- `flows appmap build --worktree` builds from the uncommitted working tree into a map labelled as
  unpinned working-tree evidence (AMAP-V0-976); the default refusal names the opt-in and the
  opt-in refuses with flags it cannot honour (AMAP-V0-977).

## Impact

- Code: `internal/appmap` (`router_react.go`, `router_literal.go`, `tests.go`, `compile.go`,
  `site_knowledge_ts.go`, new `gotoresolve.go` and `worktree.go`, `types.go`),
  `internal/command/flows/flows_appmap.go`, `cmd/corvint-docs/help.go`, and the regenerated
  command-tree snapshot.
- Wire: the optional map member `working_tree`, the attribution basis `helper-goto`, and
  `builder` `appmap-builder/4`; every existing map rebuilds once under `--from`.
- Spec: AMAP-V0-970 to 977 in `docs/specs/application-map-v0.md`; AMAP-V0-215 names
  `appmap-builder/4`.

## Non-goals

- Computed arrays (`.filter`, spreads, imports, function results), mutation analysis beyond the
  named mutators, and arrays whose elements are not object literals.
- Loops other than `for (const X of ...)` with a braced body: `forEach`, `for (let ...)`,
  index loops, destructured loop variables, and nested template expressions.
- Helper navigation through more than one hop or through a loop variable argument.
- Help for verbs outside `flows appmap`.
- Working-tree maps for aliased roots, `--from` incremental builds of an unpinned map, and
  consumers that re-read source at the map revision.

## Failure modes

- A shadowing binding the conservative scan misses could give a route or goto the wrong value; the
  scan refuses on any second declaration, parameter or arrow binding of the name it cannot place.
- A helper that edits its argument before navigating is excluded already by AMAP-V0-342 (the
  parameter must reach the goto unchanged).
- A `--worktree` map names a synthetic commit that the real repository does not hold; verbs that
  re-read source at the map revision refuse with `appmap-invalid-revision` rather than read the
  wrong content.

## Acceptance evidence

Fixture tests named `TestAMAPV097x...` in `internal/appmap` and `cmd/corvint-docs`, the per-package
test runs, and a demonstration of each fix with the lane binary on the reproduction fixture.

## Rollback

Revert the lane's commits. Maps written by `appmap-builder/4` rebuild once under `--from` with an
older binary (`builder-version-differs`); a `--worktree` map is refused by an older binary as a map
member it does not decode (AMAP-V0-960) and must be rebuilt.
