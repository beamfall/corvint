# Application Map V0 — revision-pinned screen graph and capped projections

Owner: Russell Lewis
Date: 2026-10-07
Intent status: accepted
Delivery status: experimental
Authoritative inputs: owner request [issue 657](https://github.com/beamfall/corvint/issues/657)
(native ticket V1-0956), `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/application-flow-understanding-v1.md` (AFU-V1 flow intents and navigation blocks),
`docs/specs/documentation-corpus-v1.md`, and the orchestrator notes of 2026-10-07 on
[issue 659](https://github.com/beamfall/corvint/issues/659) (V1-0958, TypeScript path aliases),
[issue 658](https://github.com/beamfall/corvint/issues/658) (V1-0957, run-verified steps) and
[issue 660](https://github.com/beamfall/corvint/issues/660) (V1-0959, scenario planner), and
[issue 669](https://github.com/beamfall/corvint/issues/669) part 2 (V1-0982, tests in a second
repository; AMAP-V0-016 proposed (V1-0981); AMAP-V0-017..020 proposed (V1-0982)), and
[issue 685](https://github.com/beamfall/corvint/issues/685) (V1-1027, state-name tables reached
through AngularJS dependency injection; AMAP-V0-021..023 accepted by decision 0460), and
[issue 705](https://github.com/beamfall/corvint/issues/705) (V1-1061, injected string enums,
one-level re-exports and the `di-constant` diagnostic; AMAP-V0-024..026 accepted by decision 0475), and
[issue 1200](https://github.com/beamfall/corvint/issues/1200) (V1-1941, request-path keys,
outbound calls and guard order; AMAP-V0-130..139), and
[issue 1215](https://github.com/beamfall/corvint/issues/1215) (V1-1967, cross-checking a
blocker's cited 404 against the route table; AMAP-V0-140..146).

## Agent digest
- Claim: A revision-pinned screen graph joins routes, flows and E2E tests through imports, served as byte-capped projections that read STALE or UNKNOWN.
- Status: accepted (decision 0446; V1-0956); experimental. AMAP-V0-001..015 are implemented in `internal/appmap` and `corvint flows appmap` over a committed fixture; no adopter-scale qualification. AMAP-V0-016 (state names and parents read through imported constant tables) is proposed (V1-0981; GitHub #669). AMAP-V0-017..020 (tests in a second, aliased repository) are proposed (V1-0982; GitHub #669) and implemented over synthetic repositories. AMAP-V0-021..023 (state-name tables injected as AngularJS `.constant(...)` registrations within a manifest-declared scope) are accepted (decision 0460; V1-1027; GitHub #685) and implemented over synthetic repositories. AMAP-V0-024..026 (an injected string enum read only when every member is a literal string, one level of `export ... from` behind the registering file's import, and a `di-constant` unknown naming why the one in-scope registration did not resolve) are accepted (decision 0475; V1-1061; GitHub #705) and implemented over synthetic repositories. AMAP-V0-040..048 are accepted (decision 0518; V1-1131); literal Rails/React routes and optional declared-glob coverage remain experimental. AMAP-V0-050..059 are accepted (decision 0518; V1-1136); static UI-router entry candidates only, runtime and PROVEN paths deferred. AMAP-V0-060..065 are accepted (decision 0518; V1-1138); a map-only qualification report with a resolved-percent gate. AMAP-V0-070..072 are accepted (decision 0518; V1-1144); star-export ambiguity paths and a named failed hop. AMAP-V0-075..077 are accepted (decision 0518; V1-1145); `tests.root: "."` for an aliased test repository with declared-directory dirty detection. AMAP-V0-078 is accepted (decision 0519; V1-1169); a same-file `const` React route array only. AMAP-V0-079 is accepted (decision 0520; V1-1220); an imported React route array read through one literal named-import hop. AMAP-V0-080..082 are accepted (decision 0520; V1-1168); entry rows read shortest PROVEN paths from a bound edge ledger, with NOT_OBSERVED for screens no proven path reaches. AMAP-V0-090..099 are accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025); `flows appmap refs`, a name-level check of test-referenced routes, selectors and mocked endpoints the declared source lacks, experimental. AMAP-V0-100..107 (`flows appmap actions`, the ROUTE-ACTION-MISSING check; V1-1890; GitHub #1174) and AMAP-V0-110..116 (`flows appmap branches`, reaching tests by boundary and setup writers per cited branch; V1-1891; GitHub #1175) are accepted under the owner's standing auto-accept and delivered experimental over inline fixtures. AMAP-V0-130..139 are accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200); `flows request-path`, the environment and configuration keys, outbound calls and guard order of one Express-style route, with a fail-closed pre-run profile check, experimental. AMAP-V0-140..146 are accepted under the owner's standing auto-accept (2026-10-11; V1-1967; GitHub #1215); `flows appmap blocker`, a route-table cross-check of a blocker note's cited 404 requests (BLOCKER-404-NO-ROUTE), experimental over inline fixtures.
- Exists: the `ui-router-states/0` router dialect, the import-graph test join over the existing contextindex web import relation, the four projections (`screen`, `flow`, `find`, `scaffold`) and the overlay seam (`internal/appmap/overlay.go`).
- Blocked on: owner acceptance; alias-imported specs stay UNKNOWN until V1-0958 lands; MCP tools and corpus records are follow-ups.
- Read next: Requirements; Overlay seam; Failure modes; Owner questions.

## User and measurable job

An agent writing or repairing an E2E test for a large single-page application needs to know, for
one screen or one flow: how to reach it, what guards it, which page-object methods already act on
it, which specs already prove it, and which of those facts are still true at the current revision.
Today an agent rebuilds this by reading router files and grepping tests by URL, which misses specs
that reach a screen by clicking, and returns either too little or a ~135 KB dump.

Measured jobs, each over the committed fixture in `internal/appmap/testdata/fixture`:

1. **Graph**: compiling the fixture yields one node per router state keyed by app and route
   template, with resolved hierarchy; every unresolvable or ambiguous route reads `UNKNOWN`.
2. **Join**: a spec that reaches a screen only by clicking through page objects is attributed to
   that screen through the import graph, not by URL.
3. **Freshness**: an element whose anchored lines changed after the map revision reads `STALE`.
4. **Bounded answers**: every projection stays within its byte cap, reports omitted counts and has
   an explicit full escape; no call can return an unbounded document.
5. **Scaffold**: a draft spec skeleton for a flow imports only modules that resolve in the real
   test tree and cites the closest asserting spec.

## Verified current state

At `8af2bf6`: AFU-V1 compiles flow intents with optional navigation blocks (state template and
locator per step) and `flows navigate` writes a navigation map, but nothing models the router
hierarchy, permissions or feature flags, joins specs to screens through page objects, or pins
page-object methods to line spans. The contextindex already records relative and `@/` web import
relations per source (`Index.Imports`); there was no exported helper to read import bindings or to
resolve one specifier for one importer. This slice adds that thin read-only wrapper
(`internal/contextindex/webimport_api.go`) and does not change JS/TS resolution.

## Definitions

- **Map**: the `application-map/0` artifact compiled at one revision.
- **Screen**: one router state; ID `screen:<app>:<state>`; key `<app>` plus route template.
- **Template**: the absolute route path with parameters written `{name}`; the collision key blanks
  parameter names (`/clubs/{}/members`).
- **Anchor**: `{path, start_line, end_line, blob, span_sha256}` read from Git at the map revision.
- **Element ID**: the stable identifier of a screen, flow (`flow:<flow_id>`), step
  (`step:<flow_id>/<step_id>`), file (`file:<path>`), method (`method:<path>#<name>`) or selector
  (`selector:<16 hex>` over kind, value and name, independent of line).
- **Join**: the attribution of a test file to the screens it reaches through its import closure.

## Requirements

AMAP-V0-001 to AMAP-V0-015 are (accepted by decision 0446; V1-0956); AMAP-V0-016 is proposed
(V1-0981; GitHub #669) and awaits owner acceptance; AMAP-V0-021 to AMAP-V0-023 are accepted
(decision 0460; V1-1027; GitHub #685); AMAP-V0-024 to AMAP-V0-026 are accepted (decision 0475;
V1-1061; GitHub #705).

Note (2026-10-14; V1-1365): the manifest admits an optional `registries` member, a list of
`string-key-registry/0` declarations owned by `NGI-V0-007` in `non-go-impact-v0.md`. The map
validates it with the same closed check and refuses an invalid one with `appmap-invalid-manifest`,
but does not read it; only non-Go syntax impact does.

- `AMAP-V0-001`: The map MUST be compiled from a closed `application-map-manifest/0` document
  (at most 256 KiB) read from Git at the evaluated revision: `app` matching
  `^[a-z0-9][a-z0-9._-]{0,63}$`, `hash_prefix` one of empty, `#` or `#!`, 1..64 `routers` each
  with a known `dialect`, optional `flows` intent directory, a `tests` layout whose `specs`,
  `page_objects`, `workflows` and `scenarios` directories lie under `tests.root`, and optional
  `page_object_screens` bindings. An unknown member, escaping or absent path, a declared test or
  flow directory that is not a directory at the revision, or an unknown dialect MUST refuse with
  `appmap-invalid-manifest`; an unresolvable revision with
  `appmap-invalid-revision`; an invalid flow intent with `appmap-invalid-flows`. (accepted by decision 0446; V1-0956)
- `AMAP-V0-002`: The `ui-router-states/0` dialect MUST read every `.state('name', {...})` and
  `.state({name, ...})` call with literal `name`, `parent`, `url`, `abstract` and
  `data.permissions` / `data.flags`, and resolve hierarchy from the dotted name or `parent`.
  A child URL is appended to its parent's template; `^` marks an absolute URL; `:p`, `{p}` and
  `{p:type}` become `{p}`; `?a&b` becomes the query list. A non-literal name, URL or value, a
  duplicate name, a missing, unresolved or cyclic parent MUST make that state and every descendant
  `UNKNOWN` with its reason, never guessed. A repeated key in one object literal makes that object
  non-literal, since the map cannot prove which value the program sees; a regular-expression
  literal is a non-literal value. A literal object counts only as the whole argument, so
  `.state('home', {url: '/x'} && config)` reads `non-literal-value` (and `.state({...} || x)`
  `non-literal-name`). A file that ends inside a state call (`app.state('home',`) reads that
  state's configuration as non-literal and never fails the build. (accepted by decision 0446; V1-0956)
- `AMAP-V0-003`: Screens MUST be keyed by app plus template. Two resolved screens with the same
  collision key MUST be reported `ambiguous-template`, and a lookup (by element ID, state name,
  template or URL) that matches several screens MUST return status `UNKNOWN` with the candidates,
  never pick one; a lookup that matches none returns `no-matching-screen`. URL lookup strips the
  manifest's hash prefix, removes the authority only after a leading scheme (`https://host`), so a
  `://` inside a query or fragment is data; the authority ends at the first `/`, `?` or `#`, so
  `https://host?next=/home` is the root, never `/home`; and it refuses a `${...}` substitution inside a segment
  (`partial-segment-substitution`). (accepted by decision 0446; V1-0956)
- `AMAP-V0-004`: Each AFU-V1 navigation step MUST become a map step on the unique screen whose
  template matches its state template, with its selector and strength and the page-object methods
  that already use the same selector (`reuse`). A step without a navigation entry reads
  `no-navigation-step`; one whose state matches zero or several screens reads `UNKNOWN` with that
  reason. Flow preconditions and precondition flows are attached to every screen a resolved step
  lands on.
  (accepted by decision 0446; V1-0956)
- `AMAP-V0-005`: Test files MUST be joined to screens through the contextindex import graph:
  spec or workflow → (workflows, scenarios, support)* → page object → screen, at most 8 hops. A page
  object is bound to a screen by a manifest `page_object_screens` entry or, failing that, by
  exactly one literal `goto`/`waitForURL`/`toHaveURL` target; zero or several targets read
  `page-object-unbound` / `page-object-ambiguous`; a page object with any non-literal target (`goto(target)`) stays unbound
  (`page-object-unresolved-target`), since that target may be the screen. A spec's own literal navigation attributes it
  directly (`basis: goto`). An import that is neither resolved nor an external package (including
  a TypeScript `paths` alias until V1-0958 lands) MUST leave that file's join `UNKNOWN`
  (`unresolved-import`); an import that resolves to a source file outside `tests.root` leaves it
  `UNKNOWN` (`import-outside-tests`), since the join does not read beyond the test tree, and every screen projection's `test_join` MUST read `UNKNOWN` while any
  test file's join is `UNKNOWN`, any test file was excluded by the index or could not be read
  (`excluded-by-index`, `unparsed-imports`), or any page object is `page-object-unbound`,
  `page-object-ambiguous` or `unknown-state`, since such a file may be the one that reaches a
  screen; attributions found through resolved imports are kept as a lower bound.
  The map MUST NOT implement alias resolution itself. (accepted by decision 0446; V1-0956)
- `AMAP-V0-006`: Each screen MUST aggregate permissions and flags (inherited from the nearest
  declaring ancestor, with `permissions_from` / `flags_from`), flow preconditions, page objects and
  their methods, workflows, scenarios, specs (with basis and import chain) and flow steps, each
  with its anchor. (accepted by decision 0446; V1-0956)
- `AMAP-V0-007`: Selectors MUST carry a kind and strength: `getByTestId` and a `data-testid`
  attribute locator are `strong` (`data-test` and `data-test-id` locators are CSS); `getByRole`, `getByLabel`, `getByPlaceholder`, `getByAltText`
  and `getByTitle` are `medium`; `getByText` and other CSS locators are `weak`; a non-literal
  argument is `unknown`. A literal counts only as a whole argument (followed by `)` or `,`), so
  `'save-' + id` is non-literal, as is such a `goto` URL (`non-literal-url`). String and template
  escapes decode to the program's value (`\n`, `\xHH`, `\uHHHH`, `\u{H}`, surrogate pairs, line
  continuations); a legacy octal escape, lone surrogate, or short or non-hex `\u` escape makes the
  literal non-literal, and a failed escape never consumes past the end of the source. A `/` after
  the `)` of an `if`, `while`, `for` or `with` condition, or after a keyword such as `else`,
  `return`, `throw` or `await`, starts a regular-expression literal, so text inside it is never
  read as a selector; after any other `)` it is a division. A
  `getByRole` name is read from the options object's top-level `name` member in any position, and
  the selector is `unknown` when the object has a spread, a computed or shorthand key, a repeated
  `name`, or a non-literal name, and when the options are passed by reference
  (`getByRole('button', opts)`) or the literal is only part of the argument (`{ name } && opts`),
  since any of these can set a name the map cannot read. Line
  continuations inside a string advance the line count of later anchors. A
  secret-shaped literal is dropped and reported `secret-shaped`.
  (accepted by decision 0446; V1-0956)
- `AMAP-V0-008`: Edges MUST come only from evidence: `flow-step` edges between adjacent
  resolved navigation steps (a step that is not placed on one screen breaks the chain, so no edge
  is inferred across it), and `test-sequence` edges between page objects of different screens
  constructed in order within one spec or workflow, each citing its source element and span. No
  edge is inferred from router hierarchy alone. (accepted by decision 0446; V1-0956)
- `AMAP-V0-009`: The map MUST be deterministic for one revision: sorted nodes, edges, files and
  unknowns, and a `digest` (SHA-256 over the encoding with an empty digest). Loading a map file
  MUST refuse a symlink, a file over 64 MiB, an unknown member, trailing data, a wrong schema or a
  digest mismatch with `appmap-invalid-map`. (accepted by decision 0446; V1-0956)
- `AMAP-V0-010`: Every element MUST be pinned to the anchor it was read from. A projection MUST
  evaluate the anchors it prints against `--revision` (default `HEAD`) with one tree read and
  bounded blob reads: the same blob, or the same span digest at the same lines, is `FRESH`; an
  absent path or a different span is `STALE`; an unresolvable revision or Git failure is
  `UNKNOWN`. A line shift without a content change therefore reads `STALE` (fail-closed). The
  check MUST run even when the evaluated revision is the map's own, because a map file's anchors
  are claims of the file, not proof. A screen's template, query, permissions and flags derive from
  its ancestors' states, so `screen` and `flow` also check every ancestor anchor and print the
  worst of them with the screen's own as `lineage_freshness` (`STALE` over `UNKNOWN` over `FRESH`).
  A spec attributed through imports derives from every file on its chain, so each spec item also
  prints `chain_freshness`, the worst of the spec's and every `via` file's anchor (`UNKNOWN` when a
  chain file is absent from the map), and a screen with a `STALE` chain is not cited as `FRESH`.
  A page object placed by a manifest `page_object_screens` entry rests on the manifest, so the
  manifest anchor joins its chain and the screen's citations.
  (accepted by decision 0446; V1-0956)
- `AMAP-V0-011`: `screen`, `flow`, `find` and `scaffold` projections MUST each return one JSON
  document no larger than its budget: defaults 4096, 6144, 2048 and 6144 bytes; `--budget` in
  256..65536; `--full` raises the ceiling to 1 MiB and is exclusive with `--budget`
  (`appmap-invalid-query`). Lists are trimmed from the tail, uniformly first and then greedily in
  section order, and an `omitted` object reports the dropped count of every list. A budget that
  cannot hold the document head refuses with `appmap-budget-too-small`. Each head carries
  `schema`, `app`, `map_revision`, `map_digest`, `evaluated_revision`, `budget` and `full`, so a
  later planner (V1-0959) can compose projections without re-reading the map. `flow` reports the
  unknowns of the flow, its steps and every screen it prints.
  (accepted by decision 0446; V1-0956)
- `AMAP-V0-012`: `find` MUST match a 2..128 byte query case-insensitively against element IDs and
  labels of screens, flows, steps, files, methods and selectors (every element's ID, not only its
  label), returning references without
  freshness evaluation (`evaluated_revision: NOT_EVALUATED`); other lengths refuse with
  `appmap-invalid-query`. (accepted by decision 0446; V1-0956)
- `AMAP-V0-013`: `scaffold` MUST draft a skeleton for one flow beside the closest asserting spec:
  a spec with a resolved join and at least one assertion that reaches a screen on the flow, ranked
  by flow screens reached, reused page-object files on its chains, fewer import hops, then path.
  It borrows that spec's imports only when each is external or resolves to a file in the map, else
  emits it commented `// UNRESOLVED` with an unknown; it imports and calls existing page-object
  methods for steps that reuse them, adding an import under the class's own name when the borrowed
  imports bind it only under an alias. Every name the scaffold binds (borrowed imports, generated
  imports, page-object variables, `page`, `test`, `expect`) is reserved for one file; a reused file
  whose class or variable name is already taken is emitted `// UNRESOLVED` with
  `binding-collision`, and its steps fall back to TODOs. It leaves locator and outcome TODOs
  otherwise. A selector of strength `unknown`, and a step with no selector (whatever a map file
  claims as its reuse), is never offered as reuse. A reused method whose anchor is `STALE` at the
  evaluated revision is not called: the step keeps a commented note and a TODO, its reuse item
  reads `freshness: STALE`, and the unknown `scaffold-reuse` / `stale-reuse` is reported. A reused
  method is called only when it is a public instance method of the page object's class, directly
  in the class body, whose header literally declares no parameters (`callable`: no getter, setter,
  static, private or protected member, no module function, no object-literal method before, after
  or inside the class); otherwise the step keeps a note and a TODO and reports `scaffold-reuse` /
  `reuse-not-callable`. Borrowed import statements are kept one per statement, each with exactly
  the names it binds. A statement the map could not read in full (over 8 lines or 512 bytes) is
  never copied in part nor guessed as a side-effect import: it is emitted as a commented
  `// UNRESOLVED import { names } from '...'` line, binds nothing, and reports `scaffold-import` /
  `unread-statement`. When no borrowed, resolvable import binds `test`, the scaffold emits
  `// UNRESOLVED import { test }` and reports `scaffold-import` / `test-unbound`. With no
  eligible spec, `closest` reads `UNKNOWN no-asserting-spec`. It never writes the file.
  (accepted by decision 0446; V1-0956)
- `AMAP-V0-014`: Learned facts MUST attach only through the overlay seam: an `Overlay` is asked once
  per projection for the element IDs that projection selects, its facts are kept only for those IDs
  and, after byte trimming, only while the element's ID is still printed elsewhere in the
  document (a dropped fact counts in `omitted.learned`),
  only when `text` is at most 1024 bytes and not secret-shaped, capped at 4096 facts
  (`overlay-bound-exceeded`), printed in a separate `learned` section with `authority: learned`
  and `freshness: STALE` when their element's anchor is stale. An overlay error is reported as
  `overlay-unavailable`. Overlay facts MUST NOT change any node, edge, strength, join or
  freshness. (accepted by decision 0446; V1-0956)
- `AMAP-V0-015`: Building and projecting MUST NOT write to the repository, the index, or any
  ledger; they read Git objects only, with no network, database or background process. Inputs are
  bounded (64 router files, 4 MiB per router, 20000 states and test files, 1 MiB per test file,
  128 MiB of test source, 64 MiB map) and a breach refuses with `appmap-bound-exceeded`.
  (accepted by decision 0446; V1-0956)
- `AMAP-V0-016`: A `.state()` name (positional or `name:`) or `parent` written as a member
  expression `X.Y` MUST resolve to the string literal `Y` holds when `X` is a constant table the map
  reads whole: a top-level `const X = {Y: '...'}` (optionally `as const` or wrapped in
  `Object.freeze(...)`) or `enum X {Y = '...'}`, declared in the router file itself or exported
  from the tracked file that a static ES `import` of `X` (named, aliased, or default via
  `export default {...}` / `export default X`) resolves to at the map revision through the
  contextindex web import resolver (relative paths and tsconfig/jsconfig `baseUrl`/`paths`). The
  screen MUST carry the declaring line, then the import statement for an imported table, as
  `name_from` / `parent_from` (AMAP-V0-010 anchors at the map revision, omitted for literals, so
  literal-only maps are byte-identical), and those anchors join the screen's lineage freshness. Anything the map cannot prove MUST stay `UNKNOWN`
  `non-literal-name` (a name) or `non-literal-value` (a parent), never guessed: a computed key,
  spread or repeated key in the table; a non-string or missing member; a `let`/`var`, typed or
  non-literal initializer; a duplicate declaration of `X` (including a merged enum); `X` both
  declared and imported; any use of `X` in the router or declaring file other than an unassigned
  member read, `typeof X`, `export default X` or `export { X }` (an assignment, `delete`, `++`, or
  passing `X` along could change it); a type-only, namespace, CommonJS or re-exported binding; and
  an import that resolves to no tracked, indexed and readable file. Each declaring file is lexed at
  most once per build and only when a router references it. Mutation from a third module is not
  read. Status: proposed (V1-0981; GitHub #669).
- `AMAP-V0-017`: A manifest MAY name the repository that holds its tests with `tests.repo`, a root
  alias matching `^[a-z][a-z0-9-]{0,31}$` (otherwise `appmap-invalid-manifest`). `build` binds
  aliases with repeatable `--repo ALIAS=ABSOLUTE_ROOT`, the MMR-V0-001/002 spelling and bounds
  (shared code `internal/rootalias`), and MAY read the manifest itself from an aliased root with
  `--manifest-repo ALIAS`. Routers and flows always come from `--root` at `--revision`; `tests.*`
  directories, the test import index and test files come from the aliased root at its `HEAD`
  commit. The join, unknowns and strengths are the same as when both trees share one repository.
  A single-root manifest compiles to the same bytes whether or not `--repo` is passed.
  (proposed (V1-0982; GitHub #669))
- `AMAP-V0-018`: A map built with an aliased root MUST pin it in `roots`
  (`{repo, revision, inputs}`, sorted by alias; `inputs` names `manifest` and/or `tests`) and MUST
  name the root on every anchor and unknown read from it with `repo`; an anchor or unknown without
  `repo` comes from `--root`. A single-root map carries neither member, so its bytes and digest are
  unchanged. (proposed (V1-0982; GitHub #669))
- `AMAP-V0-019`: A build MUST refuse rather than join part of a tree when an alias the manifest
  needs is not declared, or its root is not absolute and clean, does not exist, is not the top of a
  Git worktree, has no `HEAD` commit, is the `--root` repository, or its worktree status cannot be
  read (`appmap-root-unavailable`); or when it has staged, unstaged or untracked changes under the
  manifest or `tests.root` it is read for (`appmap-root-dirty`), because `HEAD` would then differ
  from the checkout an agent edits. Changes elsewhere in that root do not refuse. Status is read
  with `--no-optional-locks` and nothing is written. (proposed (V1-0982; GitHub #669))
- `AMAP-V0-020`: Projections, the scenario planner (AMSP-V0) and run verification (RVN-V0) read
  only `--root`. An anchor with `repo` MUST read freshness `UNKNOWN` and print its `repo`, never
  `FRESH` or `STALE` from a same-named path in `--root`; router, flow and manifest anchors in
  `--root` keep AMAP-V0-010. `find` items, planner method and spec references and anchor views
  print `repo` for evidence from an aliased root. The scaffold prints `proposed_repo` when its
  closest spec comes from an aliased root. RVN step anchors are router lineage and stay in `--root`.
  (proposed (V1-0982; GitHub #669))
- `AMAP-V0-021`: A manifest MAY declare `di_constants`, 1..16 unique repository-relative paths
  (directories or files) whose tracked JavaScript and TypeScript sources hold the app's AngularJS
  `.constant(...)` registrations; this scope is project-owned authority for which registrations
  belong to the app. An empty list, or a malformed, escaping or repeated path, MUST refuse with
  `appmap-invalid-manifest`, as MUST a path under which the revision's index holds no such source.
  More than 20000 sources in scope, or more than 128 MiB of their text read, refuses with
  `appmap-bound-exceeded`. The scope is read from the `--root` index at the map revision, only
  when a router reads a name through injection. Without the member no name resolves through
  injection, so an existing manifest compiles to the same bytes. Status: accepted (decision 0460; V1-1027;
  GitHub #685).
- `AMAP-V0-022`: A `.state()` name (positional or `name:`) or `parent` written `X.Y` in a router
  file that neither declares nor imports `X` MUST resolve through AngularJS injection when all of
  these hold: every binding of `X` in the router file is a plain parameter (an identifier,
  optionally `?` or typed `: T`) of an injectable function -- a top-level arrow or function, or the
  direct argument of `.config(...)`, alone or as the last element of an inline array annotation of
  string literals; the member read lies in the body of such a function; an inline annotation, or
  an `F.$inject = ['...', ...]` statement for the top-level function `F` in the router file, has
  one entry per parameter and names `'X'` at the parameter's position, and `F` is used nowhere in
  the router file but its declaration, that statement, `.config(F)` or an export; the
  `di_constants` scope holds exactly one registration of `'X'`, written `.constant('X', T)` with a
  literal name; and `T` is an object literal argument read whole, or an identifier that the
  AMAP-V0-016 rules read whole in the registering file (declared there, or imported by it), the
  registration being the one use of `T` there beyond member reads. The screen MUST carry, in
  `name_from` / `parent_from`, the table's declaring line, the import statement in the registering
  file for an imported `T`, the `.constant(...)` call and the router function's parameter list, in
  that order (AMAP-V0-010 anchors at the map revision; a declaring line equal to the call's span is
  carried once); they join the screen's lineage freshness.
  Status: accepted (decision 0460; V1-1027; GitHub #685).
- `AMAP-V0-023`: A name or parent the injection path cannot prove MUST stay `UNKNOWN`
  `non-literal-name` (a name) or `non-literal-value` (a parent), never guessed: no `di_constants`;
  no registration of `'X'`, or more than one, in scope; an object-map registration
  `.constant({...})` that names `X` (its value is not read) or has a spread or computed key; a
  `.constant(...)` call in scope that is unclosed, or whose first argument is not a string literal
  or object literal read whole, unless its receiver is `_` or `lodash` (whose calls are never
  registrations); a source in scope the index excluded, could not read as text, or holds over
  4 MiB (each of the last three makes every injected name in that build unprovable); `T`
  written as anything but an object literal or identifier, or an identifier that
  AMAP-V0-016 cannot prove (including a use of `T` other than the registration and member reads);
  `X` bound in the router file in any other way (a nested, destructured, defaulted or rest
  parameter, a local declaration, an import, an argument of a call); a member read outside an
  injectable function's body; a function whose return type is not a dotted type name; an
  annotation naming another injectable at that position or with a different length; a
  `$inject` in the router file other than one top-level `F.$inject = [string literals]` statement
  per function; an unnamed top-level function in a router file that has such a statement; and
  another use of `F`. Annotations and registrations outside the router file and the scope, a
  registration through an aliased function
  (`const c = m.constant; c(...)`), the AngularJS module a registration belongs to, and a
  registration added in a new file after the map revision are not read; a static map cannot see
  them. Status: accepted (decision 0460; V1-1027; GitHub #685).
- `AMAP-V0-024`: A TypeScript `enum` or `const enum` that AMAP-V0-022 reads as an injected
  registration's `T` (declared in the registering file, imported by it, or reached through
  AMAP-V0-025) MUST be a table only when every member has a literal string initializer (a quoted
  string or a template literal without substitutions). One member without an initializer, or with
  a numeric, computed, referencing or substituted initializer, or a computed key, makes it no table:
  every name read through it stays `UNKNOWN` (AMAP-V0-023) with the AMAP-V0-026 reason
  `non-literal-member`. A router file's own imported enum keeps the AMAP-V0-016 rule (such a member
  is absent; the others resolve). Status: accepted (decision 0475; V1-1061; GitHub #705).
- `AMAP-V0-025`: When the tracked file the registering file's import of `T` resolves to (the
  directly imported file, or the `index` file of an imported directory, under the existing web
  import resolution) does not declare the imported name, an injected registration MUST follow
  exactly one level of its `export ... from 'M'` statements: `export { T }`, `export { S as T }`,
  `export { default as T }` or `export * from 'M'`. A named re-export of the name shadows every
  `export *`, as in ECMAScript; so does a name the imported file exports itself in any other form
  (an exported `let`, `var`, `function`, `class` or unreadable `const`, a local `export { ... }`
  list, or any other export statement), which the reader does not follow. A candidate module that
  exports the name in any form counts as a candidate, whether or not the reader can read it as a
  table. An export statement the reader does not read name by name (a `declare`, `abstract` or
  `namespace` export, a destructuring, a declaration with several declarators) counts every
  identifier in it as a name it may export, and a file whose exports the reader cannot list may
  export any name: one whose brackets do not nest and match, with a backslash outside a string (an
  escaped identifier, inside a template substitution too), with a string export name, with an
  export statement the reader rejects or whose module name it cannot decode exactly, with a
  top-level `export` no reader consumed, with source the lexer cannot place as code, comment or
  literal (a `/` it cannot place as a regular expression or a division, as after `}`, `of`,
  `yield`, `await`, a TypeScript `x!`, `<` or a `>` not of `=>`; an unterminated string, regular
  expression, block comment or template; a `/` inside a template substitution; a `//` comment
  holding another line terminator; a non-ASCII identifier character that is not a letter, digit,
  mark or connector; an HTML-like comment), or a `.tsx`/`.jsx` file with any `<` (the lexer does
  not read JSX).
  Such a file never yields a declaration: as a candidate it counts as one that does not export the
  name as a readable table, and no table in it is read whole (`not-read-whole`), since an escaped
  identifier or a statement the reader skipped could write it. The table resolves only when
  exactly one candidate module, itself a tracked, indexed and readable file of the repository,
  declares and exports it (or holds the default export it names) under the AMAP-V0-016 and
  AMAP-V0-024 rules; the re-export statement joins the anchors between the declaring line and the
  registering file's import. Every file on that chain -- the registering file, the imported
  barrel and the declaring file, and the registering file when it declares the table itself --
  MUST pass the same write checks for every other binding of the table it holds: each import of
  the table's name (or `default`, when the table is a default export) under another local name,
  from any module, must be only read under the AMAP-V0-016 rule, and no namespace import, dynamic
  `import(...)`, `require(...)` or import item the reader cannot read may name a module that
  resolves to the declaring file, to a file that could re-export (any `export ... from`), to a file
  it cannot read, or that it cannot resolve (a non-literal or unresolved specifier); a package is
  never such a module. A binding it cannot prove only read leaves the name `UNKNOWN`
  (`not-read-whole`). Otherwise the name stays `UNKNOWN` with the AMAP-V0-026 reason: two
  candidates, a re-export item the reader cannot read, or an imported file whose exports the
  reader cannot list, `ambiguous-barrel`; a candidate module that does not declare the name but
  could re-export it again, `barrel-depth-exceeded`; a candidate module outside the repository
  index (a package, an unresolved or unreadable path), `out-of-scope`; no candidate, a type-only
  re-export, `export * as T`, the imported file's own unfollowed export of the name, or one
  candidate that does not export it as a readable table, `identifier-not-found`. Re-exports are
  not followed for a router file's own imports (AMAP-V0-016). Status: accepted (decision 0475;
  V1-1061; GitHub #705).
- `AMAP-V0-026`: When the `di_constants` scope holds exactly one registration of an injected name
  `X`, the scope is not poisoned, and a member read through it does not resolve, the map MUST carry
  one unknown `{kind: "di-constant", ref: X, reason, path, line}` naming the registering file and
  the `.constant(...)` call's first line, where `reason` is the first failure the reader met:
  `unreadable-registration` (`T` is neither an object literal nor an identifier, or the name comes
  from an object-map registration), `identifier-not-found`, `ambiguous-binding` (`T` both declared
  and imported, or bound twice), `not-read-whole` (another use of `T`, or of another binding of
  the table in a file on the AMAP-V0-025 chain, could change it, or the declaring file is unread),
  `out-of-scope`, `ambiguous-barrel`, `barrel-depth-exceeded`, `non-literal-member` (a spread,
  computed or repeated key, or an AMAP-V0-024 enum member), or `member-not-found`. Identical
  unknowns are carried once. The screen keeps its AMAP-V0-023 `non-literal-name` /
  `non-literal-value`; a build that resolves every injected read, has no scope, or finds no or more
  than one registration adds none, so its bytes are unchanged. The build still exits 0.
  Status: accepted (decision 0475; V1-1061; GitHub #705).

- `AMAP-V0-040`: The experimental `rails-routes/0` and `react-router/0` dialects MUST
  compile through the existing revision-pinned map builder alongside `ui-router-states/0`.
  Existing manifests retain their encoding; unknown dialects and more than 64 explicit routers
  still refuse. Status: accepted, decision 0518 (V1-1131).
- `AMAP-V0-041`: Rails literal `get`, `post`, `root` and `mount` declarations, `resources`
  and `resource`, and nested `namespace`, `scope`, `member` and `collection` blocks MUST
  produce route templates with named helpers as state names. Resource expansion represents
  unique helper/path pairs (not HTTP actions); nested and literal `shallow: true` resources
  preserve collection ancestry while shallow member routes drop resource ancestors. Only
  conservative regular singularization is supported; custom inflections remain UNKNOWN.
  Status: accepted, decision 0518 (V1-1131).
- `AMAP-V0-042`: Rails expressions, interpolation, constraints, defaults, lambdas, unsupported
  options and unrecognized statements MUST emit named UNKNOWN reasons; an unsupported block
  MUST NOT lend resolved routes to its descendants. A literal `draw :name` MUST read
  `routes/name.rb` beside the declaring top-level router at the same revision, once per draw
  site, at most one level; explicit routers and attempted draws share the 64-file bound. Missing, escaping, repeated or deeper draws
  remain UNKNOWN. Draw and enclosing-scope anchors MUST join derived screen freshness.
  Status: accepted, decision 0518 (V1-1131).
- `AMAP-V0-043`: React `createBrowserRouter` literal object arrays and
  `createRoutesFromElements` / `Routes` literal JSX `Route` trees MUST compile nested
  `children`, index routes and terminal splats. Literal `id` supplies the state name; otherwise
  a deterministic file/ordinal name identifies the declaration. Parent containers with children
  are abstract; pathless children inherit the parent path. Nested absolute paths must retain
  the parent prefix. Status: accepted, decision 0518 (V1-1131).
- `AMAP-V0-044`: React lazy routes, non-literal paths/IDs/index values, computed or repeated
  keys, spreads, non-literal children, conditional JSX and JSX quote escapes/entities not read exactly MUST emit named UNKNOWN reasons.
  Unresolved parents make descendants UNKNOWN; unsupported syntax never invents a screen.
  Element/Component expressions are opaque presentation references, never executed.
  Status: accepted, decision 0518 (V1-1131).
- `AMAP-V0-045`: Optional manifest `router_globs` MUST contain 1..64 unique safe repository-relative
  patterns (segment glob syntax plus whole-segment `**`). At the pinned revision the builder
  MUST compare regular tracked files matched by these patterns against explicit `routers`,
  report each absent declaration as `router-not-declared`, and emit `app_coverage` with status,
  patterns, matched, declared and undeclared paths. This is declared-scope coverage, never a
  claim of whole-application completeness. No globs means no new wire member.
  Status: accepted, decision 0518 (V1-1131).
- `AMAP-V0-046`: All dialects MUST use shared normalized parameter and template collision keys;
  terminal Rails `*name` and React `*` splats normalize to `{*name}` and share `{*}` keys.
  URL lookup MUST preserve ambiguity across dialects and match terminal splats across segments.
  Rails required splats carry `splat_nonempty: true`; React splats may match an empty tail.
  Unsupported optional/regex path syntax remains UNKNOWN rather than a guessed template.
  Status: accepted, decision 0518 (V1-1131).
- `AMAP-V0-047`: Coverage discovery MUST read only bounded Git tree metadata (at most 20000
  regular files), deduplicate overlapping globs and sort paths deterministically. Malformed
  patterns refuse as `appmap-invalid-manifest`; resource breaches refuse as
  `appmap-bound-exceeded`. Router parsing is bounded to 128 nesting levels and existing byte/state
  limits; unsupported or malformed files produce router UNKNOWN evidence.
  Status: accepted, decision 0518 (V1-1131).
- `AMAP-V0-048`: Focused committed fixtures MUST demonstrate mixed-dialect build, URL lookup,
  test attribution, round-trip map loading and deterministic output, including omitted router
  coverage. Tests MUST assert UNKNOWN without guessed resolved screens for dynamic constructs.
  Status: accepted, decision 0518 (V1-1131).

- `AMAP-V0-050`: `flows appmap entries --map FILE` MUST emit a deterministic static
  `application-map-entries/0` projection with one row per retained screen, its entry URL template,
  required path parameters, parent parameters, permission/flag guard, and last verified revision.
  Static analysis MUST leave `last_verified_revision` empty and identify its basis as `static-only`.
  Status: accepted, decision 0518 (V1-1136).
- `AMAP-V0-051`: Classification MUST be one of `deep-linkable`, `requires-parent-params`,
  `abstract`, `guarded(permission/flag)`, or `UNKNOWN`. Precedence is unresolved/ambiguous,
  abstract, unsupported entry constraints, guarded, parent parameters, then deep-linkable.
  Deep-linkable means a static candidate with its required parameters, never proof of loading.
  Status: accepted, decision 0518 (V1-1136).
- `AMAP-V0-052`: Required parameters MUST be the sorted unique union of path parameters on the
  screen and its ancestors, including parent parameters absent from an absolute child URL.
  Query parameters MUST be projected separately without claiming they are required. No fixture
  values, role or data state may be invented; arbitrary `params` configuration remains UNKNOWN.
  Status: accepted, decision 0518 (V1-1136).
- `AMAP-V0-053`: UI-router declarations MUST retain optional entry metadata distinguishing
  an explicit URL from a URL-less state and identifying nonempty/dynamic resolves, transition
  hooks, redirects and unsupported parameter configuration. Those constraints, dynamic guard
  metadata, missing/cyclic lineage and unresolved parents MUST yield UNKNOWN with reasons.
  Empty literal resolve/params objects add no constraint. Constraints inherit through ancestors.
  Status: accepted, decision 0518 (V1-1136).
- `AMAP-V0-054`: Entry results MUST carry the map revision/digest and evaluated revision.
  Freshness MUST check the manifest and complete blobs of the screen's lineage (including name
  and parent bindings), conservatively invalidating on any change to those files. Unknown or
  aliased evidence MUST NOT be upgraded to fresh. Reads inspect committed Git content only.
  Status: accepted, decision 0518 (V1-1136).
- `AMAP-V0-055`: A changed template, guard, ancestor or manifest MUST make the projected
  classification UNKNOWN and verification `UNVERIFIED_AT_HEAD`; unchanged evidence is
  `STATIC_ONLY`, unavailable evidence is `UNKNOWN`. No static check may populate a runtime
  last verified revision or emit VERIFIED. Rebuilding is the recovery path for stale entries.
  Status: accepted, decision 0518 (V1-1136).
- `AMAP-V0-056`: The projection MUST retain `proven_path` as an empty list with status UNKNOWN
  and reason `no-proven-edge-evidence` until a revision-bound edge-proof consumer is implemented.
  Existing `flow-step` and `test-sequence` edges MUST NOT be treated as PROVEN execution.
  Shortest PROVEN paths and edge-ledger receipt integration are explicitly deferred.
  Status: accepted, decision 0518 (V1-1136).
- `AMAP-V0-057`: Entries MUST use the existing bounded map loader and projection renderer,
  default 6144 bytes, `--budget` 256..65536 or exclusive `--full` capped at 1 MiB, with omitted
  entry counts. Output order MUST be screen ID order; a head that cannot fit MUST refuse.
  Status: accepted, decision 0518 (V1-1136).
- `AMAP-V0-058`: Old maps without entry metadata MUST remain readable and project UNKNOWN
  (`entry-metadata-unavailable`) for otherwise routable screens. Other dialects stay UNKNOWN
  until they supply equivalent entry constraints. Existing projections keep their contracts;
  the optional metadata participates in the map digest. No new error codes are introduced.
  Status: accepted, decision 0518 (V1-1136).
- `AMAP-V0-059`: This slice MUST remain static and read-only: it MUST NOT visit URLs or run
  application code. Runtime flags MUST refuse. A future opt-in trusted-local runtime proposal
  MUST require declared fixture/session and read-effect entries, refuse every other effect,
  and bind loads/redirect/error receipts to revision and entry identity before ledger use.
  Runtime execution and its receipt format remain deferred, not implemented by this slice.
  Status: accepted, decision 0518 (V1-1136).

- `AMAP-V0-060`: `flows appmap stats --map FILE` MUST emit a deterministic
  `application-map-stats/0` projection computed from the map file alone: screens total, resolved
  and unknown with the resolved percentage, unknown reasons, test-file join resolved/UNKNOWN
  counts, page objects and unbound page objects, edges by basis and the selector strength
  histogram. It MUST NOT read the repository or any URL.
  Status: accepted, decision 0518 (V1-1138).
- `AMAP-V0-061`: Every count MUST reconcile with the map: resolved plus unknown screens equals
  `screens`, the unknown-reason counts (all of them, including omitted) equal the map's `unknowns`
  entries (each unresolved screen already carries one), edge counts sum to `edges`, and join counts sum to `files`.
  Status: accepted, decision 0518 (V1-1138).
- `AMAP-V0-062`: Unknown reasons MUST be grouped by kind and reason (the `source` member is always
  `unknown`), ordered by count descending then kind and reason, limited to the top 5 with an
  `unknown_reasons_omitted` count, each with at most 3 example anchors ordered by path, line and
  reference.
  Status: accepted, decision 0518 (V1-1138).
- `AMAP-V0-063`: Output MUST be capped like other projections (default 6144 bytes, `--budget`
  256..65536 or exclusive `--full` up to 1 MiB) and identical for identical maps.
  Status: accepted, decision 0518 (V1-1138).
- `AMAP-V0-064`: `--min-resolved PCT` (0..100) MUST still write the report and then exit 1 when the
  resolved percentage is below PCT, with a message naming the dominant unknown reasons; at or above
  PCT, or when absent, build-style success (exit 0) holds. Out-of-range values refuse with exit 2.
  An empty map reads 0 percent.
  Status: accepted, decision 0518 (V1-1138).
- `AMAP-V0-065`: The report is a measure of the declared router and test scope only; it MUST NOT
  claim equivalence with a hand-built map, and adopter-scale qualification remains `NOT_RUN`.
  Status: accepted, decision 0518 (V1-1138).

- `AMAP-V0-070`: An injected `.constant(...)` table imported through a directory specifier that
  resolves to `index.{ts,js}`, through exactly one `export * from` module declaring the name, whose
  string enum or object has only literal members, MUST resolve every state name and parent through
  it, anchored as AMAP-V0-025 does. This pins behavior the reader already has for the reported
  shape; the reported failure was not reproduced on a minimal fixture (see V1-1144 build log).
  Status: accepted, decision 0518 (V1-1144).
- `AMAP-V0-071`: When several star exports of a barrel declare the imported name, the name MUST
  stay UNKNOWN and the `di-constant` unknown MUST carry `detail` `ambiguous-reexport:` plus the
  sorted candidate paths joined by commas. The reason stays `ambiguous-barrel`, so existing
  reasons and fixtures are unchanged.
  Status: accepted, decision 0518 (V1-1144).
- `AMAP-V0-072`: A `di-constant` unknown with reason `not-read-whole` MUST carry `detail` naming the
  failed hop: `registering-file-use`, `registering-file-bindings`, `registering-file-examined`,
  `declaring-file-unread`, `import-chain-bindings`, `import-chain-examined` or
  `registration-examined`. `detail` is an optional member of an unknown and absent otherwise, so a
  map without such unknowns compiles to the same bytes; a map with one gains the member and a new
  digest. This is the one deliberate byte change of this requirement.
  Status: accepted, decision 0518 (V1-1144).
- `AMAP-V0-075`: A manifest `tests.root` of exactly `"."` MUST be accepted and name the whole
  aliased test repository; the specs, page-object, workflow and scenario directories stay relative
  to it. Every other value is validated as before (empty, absolute, `..` and `./` forms refuse).
  Status: accepted, decision 0518 (V1-1145).
- `AMAP-V0-076`: With `tests.root` `"."` and an aliased test repository, dirty-root detection MUST
  cover only the declared directories plus the manifest file, so an untracked or modified file
  outside them does not refuse, and one inside them refuses with `appmap-root-dirty`.
  Status: accepted, decision 0518 (V1-1145).
- `AMAP-V0-077`: A manifest whose `tests.root` is not `"."` MUST compile to the same bytes and with
  the same dirty-root scope as before this requirement.
  Status: accepted, decision 0518 (V1-1145).
- `AMAP-V0-078`: `createBrowserRouter(NAME)` MUST compile the literal array bound by the one
  top-level `const NAME = [...]` (optionally typed `: Type` or `: Type[]`) in the same router file,
  exactly as the inline array would compile (AMAP-V0-043/044 apply to its contents), only when NAME
  appears nowhere else in the file except the router call and as a property after `.` (a `...` spread is another use). A NAME bound by an `import` MUST
  stay UNKNOWN as `imported-react-routes` (no other file is read); any other declaration, assignment,
  mutation, call argument, non-top-level or trailing-expression binding (anything after the literal other than `;` or a
  statement starting on a later line), or any file holding a `${}` template substitution or a `\` escape outside a literal, MUST stay UNKNOWN as
  `computed-react-routes`. Rails computed route arrays are not read; an imported React array is
  read only as AMAP-V0-079 states.
  Status: accepted (decision 0519; V1-1169); the imported-array clause is AMAP-V0-079 (decision 0520).
- `AMAP-V0-079`: When NAME in `createBrowserRouter(NAME)` is bound only by one named import
  `import { NAME } from 'SPEC'` or `import { ORIGINAL as NAME } from 'SPEC'` whose literal SPEC
  starts with `./` or `../`, the builder MUST read the one module SPEC names at the build revision
  (SPEC itself when it ends in `.ts`, `.tsx`, `.js` or `.jsx`, otherwise exactly one of SPEC plus
  those extensions or `SPEC/index` plus them) and compile the array its one top-level
  `export const ORIGINAL = [...]` binds under the AMAP-V0-078 rules, which apply to the module as
  they apply to the router file (any other use of ORIGINAL there stays `computed-react-routes`).
  Exactly one hop is read: an import of ORIGINAL in the module stays `imported-react-routes`, as do
  default, namespace, `type`, `default as` and package or aliased specifiers. No candidate reads
  `unresolved-react-routes-import` and more than one reads `ambiguous-react-routes-import`; a
  conditional, logical, loop or `.map` construct in the module makes its states
  `dynamic-route-context`. Derived states are anchored to the module, unnamed ones named
  `react:<module-path>:<ordinal>`, and carry the router file as a `name_from` anchor so a change to
  either file leaves them not FRESH. The read module counts toward the 64 router-file bound.
  Status: accepted (decision 0520; V1-1220).
- `AMAP-V0-080`: `flows appmap entries` MUST accept an optional `--ledger FILE` naming an
  `application-edge-ledger/0` document. The ledger MUST name the map's app and digest and the
  evaluated revision and carry no omitted edges, or the command refuses with
  `appmap-invalid-query` and no stdout. With a ledger the head MUST read
  `basis: "static-entries+edge-ledger"` and add `ledger_digest`; without one the output is
  byte-for-byte the AMAP-V0-050..059 projection. This is the revision-bound edge-proof consumer
  AMAP-V0-056 waits for.
  Status: accepted (decision 0520; V1-1168).
- `AMAP-V0-081`: With a ledger, each row's `proven_path` MUST be the shortest screen sequence from
  a `deep-linkable` entry to that screen that crosses only ledger edges with status PROVEN whose
  evidence is VERIFIED by a receipt with a valid test key, with `proven_by` listing one test key per edge. Ties go to the lower entry
  screen ID, then the lower next screen ID, then the lower test key. A deep-linkable screen reads
  `path_status: "ENTRY"` (`static-deep-link`) with itself as the path; a reached screen reads
  PROVEN (`ledger-edges`); any other screen reads `NOT_OBSERVED` (`no-proven-path-from-entry`) with
  an empty path. Unproven, contradicted, receipt-less, empty-key and self-loop ledger edges never
  contribute.
  Status: accepted (decision 0520; V1-1168).
- `AMAP-V0-082`: A path MUST NOT start at, end at or cross a screen whose entry verification is not
  `STATIC_ONLY` at the evaluated revision; such a row reads `path_status: "UNKNOWN"`
  (`screen-source-not-fresh`). Paths come only from retained ledger receipts: no URL is visited,
  no application code runs, and the entry itself stays a static candidate, so classification,
  `verification` and `last_verified_revision` are unchanged by the ledger (AMAP-V0-055/059).
  Status: accepted (decision 0520; V1-1168).
- `AMAP-V0-090`: `flows appmap refs --map FILE --source DIR [--source DIR]... [--budget N | --full]`
  MUST read only the map file and committed objects of `--root` at the map's revision: the map's
  test files and the regular files under 1..16 unique repository-relative `--source` directories
  that are trees at that revision. It writes `application-map-refs/0` to stdout and no file,
  index, ledger or trace. Zero or more than 16 sources, a duplicate, escaping or non-directory
  source, or an out-of-range budget (an explicit `--budget 0` included) or `--budget` with
  `--full` refuse with `appmap-invalid-query`; a map revision that is not
  a commit of `--root` refuses with `appmap-invalid-revision`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-091`: References MUST be read from dotted calls in the map's test files with the
  AMAP-V0-007 lexer: `goto`, `waitForURL` and `toHaveURL` give route references; `route` gives
  one endpoint reference per checkable path segment of its first argument; `getByTestId` gives a
  test-id name, `locator` gives the test id of an exact `[data-testid=...]` and otherwise the `#id`,
  `.class`, `data-*` attribute names and `data-*`/`id` attribute values of its CSS text, with
  whitespace allowed around the attribute name and operator. An `#id` or `.class` starting with a
  digit is not CSS and is skipped; an attribute value is checked whatever its first character
  (`[data-status="404"]` checks `404`). Text-level
  locators (`getByRole`, `getByLabel`, `getByPlaceholder`, `getByAltText`, `getByTitle`,
  `getByText`, `text=`, `role=`), XPath and CSS without such a name MUST be counted as unchecked
  with a reason, never as a miss.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-092`: A whole string literal argument MUST be an `exact` value. A template with a
  substitution, or a literal followed by `+`, MUST be a `prefix` value up to the first dynamic
  part (a leading substitution before `/` is an origin and skipped). A regular-expression
  literal MUST be a `prefix` of its leading literal text (`^` dropped, escaped punctuation read as
  the character, stopping at a metacharacter, class escape or optional character) labelled
  `regular-expression` (an escaped `\*` in it is a literal character, not a glob, and the label
  stays). A string URL glob is a prefix of the text before its first `*`. A value with no
  literal start, or any other argument, MUST be unchecked with the reasons `dynamic-value`,
  `regex-without-literal`, `glob-without-literal` or `non-literal-value`; a value over 256 bytes is
  unchecked as `value-too-long`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-093`: A route reference MUST be checked against the map's resolved, non-abstract
  screens through the AMAP-V0-003 URL rules (authority, hash prefix, query and fragment removed;
  `{}` and splat templates). An exact value is present when some template matches it; a prefix is
  present when some template can still match a URL that starts with it (complete segments match,
  the partial last segment starts the template segment's literal text, a splat matches once its
  literal segments do). A value that does not start with `/` after that removal is unchecked as
  `relative-url`. Screens that are not resolved are counted in `screens_unknown` and never match.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-094`: Selector and endpoint names MUST be checked at name level only: an exact name is
  present when it occurs as a maximal run of `[A-Za-z0-9_-]` in a source file, a prefix when such
  a run starts with it, and a name holding other characters when it occurs as a substring.
  Endpoint segments drop the scheme and authority (or a leading `host.tld`/`host:port` segment),
  the query and fragment, and empty, glob-only and digit-only segments; a segment holding a glob
  character is a prefix of its literal start. Every report MUST carry `check: "name-level"` and
  the fixed `limits` sentence saying rendering, reachability, attribute placement and
  runtime request matching are not checked.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-095`: Known false-positive classes MUST be labelled on the miss: `catch-all-route`
  when only a root-level splat screen matches the route, `regular-expression` for a value taken
  from a regular expression, and `app-templated-name` for an exact name a proper prefix of which,
  ending in `-` or `_`, occurs as a source name. Labelled misses are reported and counted in
  `misses_labelled`; they never fail the check.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-096`: Each miss MUST report its `kind` (`route`, `selector`, `endpoint`), `value`,
  `match` (`exact` or `prefix`), optional `class`, `occurrences` and up to three `path:line` test
  locations, grouped by kind, value, match and class, unlabelled misses first. Unchecked references
  are grouped by kind and reason with a count and one example location. The document MUST fit
  `--budget` (256..65536, default 6144) or `--full` (1 MiB) through the AMAP-V0-011 renderer with
  per-section `omitted` counts, and a budget that cannot hold the head refuses with
  `appmap-budget-too-small`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-097`: The source corpus MUST stay within the AMAP-V0-015 test bounds (20000 files,
  1 MiB per file, 128 MiB in total, else `appmap-bound-exceeded`); the total counts every source
  path, even when paths share one blob. A source file over 1 MiB or holding a NUL byte is skipped
  and counted in `source_files_skipped`, and no map test file of `--root`, read or unread, is part
  of the corpus. A map test file read from an aliased root, or whose blob at the revision
  differs from its anchor, is not read and is counted in `test_files_unread`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-098`: The command MUST exit 1 after writing the report when at least one unlabelled
  miss exists, 0 when none exists, and 2 with no stdout on any refusal.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-099`: A secret-shaped reference value (the `internal/secretscreen` writer matcher)
  MUST be counted as unchecked with reason `secret-shaped` and never printed or looked up.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025).
- `AMAP-V0-100`: `flows appmap actions --map FILE [--budget N | --full]` MUST read only the map
  file and committed objects of `--root` at the map's revision: the manifest at the map's
  manifest anchor, its `rails-routes/0` routers and their one-level `draw` files, and Ruby and
  view paths under each router's application root (the directory two levels above the router
  file). It writes `application-map-actions/0` to stdout and no file, index, ledger or trace. A
  map revision that is not a commit of `--root`, or a manifest read from an aliased root or whose
  blob differs from its anchor, refuses with `appmap-invalid-revision`; budget errors refuse as
  AMAP-V0-090 does. Routers of `ui-router-states/0` and `react-router/0` resolve client
  components, not server actions, and MUST be listed with reason `no-server-handler` rather than
  checked.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1890; GitHub #1174).
- `AMAP-V0-101`: Every route a supported Rails declaration compiles MUST resolve to a handler
  (controller constant and action). `resources :x` yields `index`, `create`, `new`, `edit`,
  `show`, `update` (one route on verb `PATCH|PUT`, matching either) and `destroy` on
  `XController`; `resource :x` yields the same without `index`
  on the plural controller (a name whose plural is not a bare `s` suffix, one ending `s`, `x`,
  `z`, `ch`, `sh`, `y`, `f`, `fe` or `o`, abstains `unsupported-rails-inflection` unless
  `controller:` names it). Symbol or symbol-array `only:` and `except:` (`%i[]` included)
  remove the routes they exclude, and those routes are never reported. `controller:` and
  `module:` options, `namespace` and `scope module:` set the controller path, and a controller
  written with a leading `/` (in `to:`, `controller:` or `root`) ignores the enclosing module, as
  Rails does; nested resources
  resolve to their own controller; `member`/`collection` blocks, `on:` options and bare verb
  routes inside a resource block resolve to that resource's controller. `get`, `post`, `patch`,
  `put`, `delete` and `match` with `to: "c#a"`, a `"path" => "c#a"` pair or `controller:` plus
  `action:`, and `root "c#a"` or `root to: "c#a"`, resolve to the named action; a `path:` option replaces the
  path segment, `match` takes its verbs from `via:` (else `ANY`), and options may be written
  `key: :sym`. `mount` targets and `to: redirect(...)` or `to: Constant` routes are counted as
  not applicable with reason `mounted-application`, `redirect` or `rack-endpoint`. A `draw` that
  is not a literal single name abstains `non-literal-draw`, a second draw of one file
  `repeated-draw`, an absent draw file `draw-unavailable`, and a draw inside a drawn file
  `draw-depth-exceeded`. Any other construct, non-literal value or
  route without a handler MUST abstain at its declaration line with the AMAP-V0-042 reason or
  `no-handler`; a block under an abstained declaration is not mined.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1890; GitHub #1174; review-round amendment accepted by the owner 2026-10-19, PR #1190).
- `AMAP-V0-102`: A handler MUST be read from `app/controllers/<controller path>_controller.rb`
  under the application root at the map revision and MUST define the expected class constant,
  either nested in modules or by a compound name; otherwise the route abstains with
  `handler-file-not-found` or `handler-class-not-found`. The class's actions are its public
  instance methods: `def name` (endless definitions included) at the class body's own level,
  with bare `private`, `protected` and `public` switching visibility and `private def name` or
  `private :a, :b` hiding the named methods. `private :x` or `public :x` also changes the
  visibility of an inherited `x`; the first definition or visibility change met in Ruby lookup
  order decides it. Bodies of a class reopened in one file join in file order. Singleton methods (`def self.x`, `class << self`)
  are never actions. A route whose action exists only as a non-public method and has no template
  (AMAP-V0-104) is reported missing with basis `non-public-method`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1890; GitHub #1174; review-round amendment accepted by the owner 2026-10-19, PR #1190).
- `AMAP-V0-103`: An action defined by an ancestor class or an included module MUST count as
  defined. The superclass resolves lexically (innermost module first, then top level) through the
  Zeitwerk file convention under `app/controllers`, and a reference written `::X` resolves at top
  level only; `ActionController::Base`,
  `ActionController::API` and `ActionController::Metal` end the chain with no actions. `include`d
  modules resolve under `app/controllers/concerns`, `app/controllers` and `app/models/concerns`;
  their public methods at module level and inside `included do` count, while `class_methods do`,
  `module ClassMethods` and singleton methods do not. Modules included by a module are followed;
  a module already in the chain is skipped, and `include A, B` places `A` before `B` while a later
  `include` statement precedes an earlier one, as Ruby does. More than 16 classes and modules in one
  chain, or a class cycle, abstains `ancestor-depth-exceeded`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1890; GitHub #1174; review-round amendment accepted by the owner 2026-10-19, PR #1190).
- `AMAP-V0-104`: A route whose action is not a public method MUST count as defined when a view
  template `app/views/<prefix>/<action>.*` exists at the revision for the controller path or the
  path of an ancestor controller class in its chain (`application` included), as Rails view
  prefixes do (implicit rendering), with basis `implicit-template`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1890; GitHub #1174; review-round amendment accepted by the owner 2026-10-19, PR #1190).
- `AMAP-V0-105`: When an action is not found, or is found only as a non-public method, a chain
  member that could define it at run time
  MUST make the route abstain rather than report: an unresolved superclass
  (`unresolved-ancestor`) or included module (`unresolved-module`) naming the constant, a
  non-constant `include` (`unresolved-module`), a chain member using `define_method`,
  `class_eval`, `instance_eval`, `module_eval`, `class_exec` or `instance_exec` anywhere,
  defining `method_missing`, `action_missing` or `respond_to_missing?`, or calling
  `alias_method`, `alias`, `send`, `public_send`, `attr`, `attr_*` or `delegate` at body level,
  or holding a `def` inside a conditional or block at body level (`dynamic-definition`, with path
  and line), and a
  handler file the AMAP-V0-007 Ruby masking leaves in an open or unterminated state
  (`unsupported-ruby-syntax`).
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1890; GitHub #1174; review-round amendment accepted by the owner 2026-10-19, PR #1190).
- `AMAP-V0-106`: Each finding MUST carry rule `ROUTE-ACTION-MISSING`, the declaration
  `path:line`, HTTP verb, route path, controller constant, handler file and the missing action;
  each abstention carries the declaration, controller, action (when known), reason and the
  `path:line` that caused it. The report head counts routes, defined, missing, abstained and not
  applicable (head member `not_applicable_total`), and the document fits `--budget` (256..65536, default 6144) or `--full` through the
  AMAP-V0-011 renderer, findings before abstentions. The command exits 1 after writing the report
  when a finding exists, 0 when none does, and 2 with no stdout on any refusal.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1890; GitHub #1174).
- `AMAP-V0-107`: Router reads MUST stay within the AMAP-V0-015 router bounds and controller,
  concern and view reads within the test bounds (20000 paths, 1 MiB per file, 128 MiB in total,
  else `appmap-bound-exceeded`), one budget shared by every application root a command loads
  and, for branches, by the map's test files; a Ruby file over 1 MiB is not read and a route needing it
  abstains `handler-file-unreadable`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1890; GitHub #1174; review-round amendment accepted by the owner 2026-10-19, PR #1190).
- `AMAP-V0-110`: `flows appmap branches --map FILE --cite PATH:START[-END] [--cite ...]
  [--budget N | --full]` MUST take 1..16 cited ranges of committed `.rb` files (an obligation is
  passed as its cited lines), each at most 400 lines and inside its file at the map revision,
  and read only the map file and committed objects of `--root` at that revision: the AMAP-V0-100
  routers, Ruby files under the `app`, `spec`, `test` and `factories` directories of the
  application roots its cites belong to (other roots are not read), YAML fixtures under `spec/fixtures` and `test/fixtures`, and the map's test files.
  It writes `application-map-branches/0` to stdout and no file, index, ledger or trace. An
  escaping, duplicate, non-Ruby, absent or out-of-range cite, a cite outside every application
  root, or the AMAP-V0-090 budget errors, refuse with `appmap-invalid-query`; the AMAP-V0-100 revision errors refuse the same way.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1891; GitHub #1175; review-round amendment accepted by the owner 2026-10-19, PR #1190).
- `AMAP-V0-111`: Branches MUST be read from the cited lines after AMAP-V0-105 masking: a
  statement-start `if`, `unless`, `elsif` or `when`, and a modifier `if` or `unless` after an
  expression. Each branch reports its `path:line`, kind, guard text (at most 200 bytes), the
  enclosing class or module constant and method. A cite with no branch reports `no-branch`.
  Ternaries, `&&`/`||` short circuits, `case` without `when`, loops and `rescue` are not
  branches in this slice.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1891; GitHub #1175).
- `AMAP-V0-112`: Each branch MUST list the tests that reach it grouped by boundary `unit`,
  `request` and `e2e`, each with status `reached`, `possible` or `none`, and a `none` boundary
  MUST carry the note `none at <boundary>`. Ruby tests are `spec/**/*_spec.rb` and
  `test/**/*_test.rb`; a path segment `requests`, `controllers`, `integration` or `api` makes a
  request test, `system`, `features`, `e2e` or `acceptance` an e2e test, anything else a unit
  test; the map's test files are e2e tests. A unit test reaches a branch when it names the
  enclosing constant and the method (`.m`, `#m` or `:m`). A request test reaches it when the
  enclosing method is a routed action, or a method of the same controller one routed action
  calls, or a callback the controller's own body registers (`before_action`, `after_action`,
  `around_action` and their `prepend_`/`append_` forms naming the method as a positional symbol),
  which the routed actions its registration applies to reach, and the test calls that route's verb
  with a literal path matching its template or with its `_path`/`_url` helper. Callback statements
  replay in file order across reopened bodies: a later registration replaces an earlier one,
  `only:`/`except:` (symbol, string or list) restrict it and `skip_*_action` removes it for its
  actions. A registration or skip with `if:`, `unless:`, another option or a value it cannot read,
  or continued past its line, leaves the affected actions' tests `possible` with basis
  `conditional-callback-literal-path`, `conditional-callback-path-helper` or
  `conditional-callback-visit`. A call inside a comment or string literal (AMAP-V0-007 masking of
  the test file) is not a call. An e2e test reaches a GET action whose route it visits (`visit`,
  `goto`, `waitForURL`, `toHaveURL`); one that only visits another GET route of the same
  controller is `possible` with basis `same-controller-screen`. Every listed test carries its
  basis; this is static naming evidence, not execution coverage.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1891; GitHub #1175; review-round amendment accepted by the owner 2026-10-19, PR #1190).
- `AMAP-V0-113`: Each guard's inputs MUST be resolved to model attributes or reported
  unresolved. `recv.attr`, followed by at most one of `present?`, `blank?`, `nil?`, `any?`,
  `empty?` or `zero?`, binds `recv` (`@x`, `x` or `current_x`) to the model `X` when
  `app/models/x.rb` exists; a bare attribute in a model file binds to that model. `params`,
  `session`, `cookies`, `request` and `ENV` inputs are unresolved as `request-input`; a receiver
  with no model as `unresolved-receiver`; calls with arguments, longer chains and other
  expressions as `unsupported-expression`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1891; GitHub #1175).
- `AMAP-V0-114`: Setup writers MUST be derived as one reusable model-attribute writer index:
  FactoryBot factories and traits under `spec/factories`, `test/factories` and `factories`
  (`factory :x [, class: X]`, with `class:` also read as a quoted name or a symbol, attribute
  lines; a nested factory inherits its ancestors' attributes, and a `class:` value it cannot read
  binds the factory to no model), YAML fixtures (file name as table, second-level
  keys as attributes), and setup endpoints, the routes whose first path segment is `test`,
  `__test__`, `e2e`, `cypress`, `__cypress__`, `testing`, `test_support` or `test_setup`. An
  endpoint's action and the same-class methods it calls write through `Model.create`, `create!`,
  `new`, `update`, `update!`, `find_or_create_by`, `insert`, `upsert`, FactoryBot
  `create(:f, ...)`, `record.update(...)` and `record.attr =`; a literal hash names the
  attributes written, and a non-literal argument writes every attribute with basis
  `attributes-from-request`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1891; GitHub #1175; review-round amendment accepted by the owner 2026-10-19, PR #1190).
- `AMAP-V0-115`: A branch with a boundary that is not `reached` MUST carry a `setup` entry: its
  guard, the writers of each resolved input (setup endpoints first, then factories and traits,
  then fixtures) and every unresolved input with its reason. Status is `unresolved-guard` when any
  input is unresolved, even when candidates exist, `candidates` when writers exist, `no-input`
  when the guard reads no input, and `none-found` only when every input resolved and nothing
  writes it; an unresolved guard MUST never read `none-found`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1891; GitHub #1175).
- `AMAP-V0-116`: The document MUST fit `--budget` (256..65536, default 6144) or `--full` through
  the AMAP-V0-011 renderer with an `omitted` count and the head member `branches_total`, carry the fixed `limits` sentence saying the
  reach is static naming evidence and setup is a write-name join, and exit 0 after writing it or
  2 with no stdout on any refusal. Reads stay within the AMAP-V0-107 bounds; a secret-shaped guard
  (the `internal/secretscreen` writer matcher) prints `guard_redacted: true` and no guard text.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1891; GitHub #1175).
- `AMAP-V0-130`: `flows request-path --file PATH (--route 'METHOD /path' | --handler NAME)
  [--revision REV] [--env-profile FILE [--warn]]` MUST write one `corvint-request-path/0` JSON
  document read from committed JavaScript/TypeScript blobs at `--revision` (default `HEAD`) through
  the shared application-map lexer. It MUST NOT read the working tree, run or evaluate code, open a
  network connection or write any file; `--env-profile` is the only non-Git input.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-131`: The request path MUST list, in run order, the `recv.use([prefix,] ...)`
  middleware registered earlier in `--file` on the route's receiver whose literal prefix covers the
  route path, then the route registration's arguments (arrays flattened), each a `GUARD` except the
  last, the `HANDLER`. For each step it MUST list the environment keys read as `process.env.KEY`,
  `process.env['KEY']`, `process.env[CONST]` through a top-level string constant, or destructuring
  of `process.env`, and the configuration keys read as `config.get|has('key')` through the `config`
  or `nconf` package, in the step's function (parameter defaults included), in functions it calls
  that resolve to a declaration in a read file, in factory-call arguments, and in top-level
  initializers (every declarator of a declaration) it references, a called client's included. A key
  read only with a fallback (`||`, `??`, or a destructuring default) at every read MUST read
  `defaulted: true`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-132`: A call through `axios`, `node-fetch`, `cross-fetch`, `got`, `undici`,
  `superagent`, `ky`, `needle`, `request` or `openid-client` (other than client construction such as
  `create`), through `http`/`https` `request` or `get`, through the global `fetch`, or through a
  top-level client a `pkg.create(...)` call derives (calling the client or a member of it), MUST be
  listed as an outbound call with its step, callee text, location and function chain, and its step
  MUST read `outbound: YES`. A step with no outbound call reads `UNKNOWN` when it has an
  unresolved call, middleware, source or bound, else `NONE_FOUND`. Calls on the step function's
  own parameters with at most one member (the Express `req`, `res` and `next` API) and on
  JavaScript built-in globals read as local; every other call is resolved, classified or unknown.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-133`: Each guard reading `outbound: YES` that runs before a later guard reading
  `NONE_FOUND` with no unknowns MUST produce one `OUTBOUND_GUARD_BEFORE_CHEAP_GUARD` finding naming
  the outbound guard, those later guards and the outbound callees a test of a later step must
  stand in for. A guard order with no such pair MUST produce none.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-134`: A key, call, middleware, receiver or source the reader cannot resolve statically
  MUST be listed under `unknowns` with kind `ENV_KEY`, `CONFIG_KEY`, `CALL`, `MIDDLEWARE`, `MOUNT`,
  `SOURCE` or `BOUND`, a reason, its text and location; it MUST NOT be omitted or reported as an
  absent key or call. A call through a package on neither the outbound list nor the local list
  reads `package-not-followed`; a call on a name that is not a top-level declaration or import
  (a parameter, local or undeclared global) reads `callee-not-resolved`; a file the lexer is unsure
  of adds a `SOURCE` unknown to each step that reads it.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-135`: `--env-profile` MUST read a dotenv-format profile (at most 1 MiB) and report each
  non-defaulted request-path environment key that is unset or empty by name only, the defaulted
  keys unset or empty, the configuration keys it does not check, and the count of every unknown by
  kind.
  No profile value, and no profile key the path does not read, may appear in any output; a
  malformed line refuses naming only its line number.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-136`: The pre-run verdict MUST be `BLOCKED` when a required key is unset or empty,
  else `INCOMPLETE` when any unknown other than `CONFIG_KEY` exists, else `READY`. By default
  (`FAIL_CLOSED`) a verdict other than `READY` MUST read `batch: DO_NOT_SCHEDULE` and exit 1 after
  writing the report; `--warn` reads `SCHEDULE_WITH_WARNINGS` and exits 0; `READY` reads `SCHEDULE`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-137`: Reads MUST stay within 64 files, 1 MiB per file, 512 function visits and a call
  depth of 24; exceeding the file, visit or depth bound MUST add a `BOUND` unknown. Relative modules
  resolve as the exact path, then `.ts .tsx .js .jsx .mjs .cjs .mts .cts` (a `.js` specifier also as
  its TypeScript sibling), then `index.*`; symlinks and submodules are never read, and a path
  outside the repository reads `module-outside-repository`. An imported module over 1 MiB or
  unreadable reads a `CALL` unknown (`bound-exceeded`, `read-failed`) on the step that needs it.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-138`: A refusal MUST exit 2 with one coded error and no stdout: no literal registration
  of the route (`request-path-route-not-found`), more than one (`request-path-route-ambiguous`), no
  such handler (`request-path-handler-not-found`), `--file` not a committed regular file
  (`request-path-file-not-found`), an unresolvable revision (`request-path-invalid-revision`), a Git
  read failure of `--file` or of the `--handler` module (`request-path-source-unavailable`), either
  over its bound (`request-path-bound-exceeded`) or an unreadable or malformed profile
  (`request-path-profile-invalid`).
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-139`: The same committed inputs and arguments MUST produce byte-identical reports:
  steps in run order, keys sorted by name, unknowns ordered by step, file and line.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200).
- `AMAP-V0-140`: `flows appmap blocker --map FILE --note TEXT [--budget N | --full]` MUST take
  the text of one blocked or fixture-gap classification or run failure note (1..16384 bytes, no
  NUL) and read only the map file and committed objects of `--root` at the map revision: the
  AMAP-V0-100 routers and the controller and concern Ruby files of the application roots whose
  routes a cited request matches. It writes `application-map-blocker/0` to stdout and no file,
  index, ledger, trace or classification. An empty or oversized note, more than 16 cited requests
  or the AMAP-V0-090 budget errors refuse with `appmap-invalid-query`; the AMAP-V0-100 revision
  errors refuse the same way.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1967; GitHub #1215).
- `AMAP-V0-141`: A cited request MUST be read only from a note line holding a standalone `404`
  token: each `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD` or `OPTIONS` followed by a path
  starting `/` (an `http(s)://host` prefix is dropped). The query and fragment, trailing
  `.,;:)]}` and a trailing `/` are removed, requests are deduplicated by method and path in note
  order, and each keeps its note line. A note citing none reads verdict `not-applicable`.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1967; GitHub #1215).
- `AMAP-V0-142`: A request MUST match a compiled `rails-routes/0` route (AMAP-V0-101) when the
  route verb serves the method (`HEAD` is served by `GET`; `PATCH|PUT` by either) and the paths
  match segment by segment: literal segments exactly, `:param` one non-empty segment without a dot,
  `*glob` the remaining one or more segments, after first trying the request with a trailing
  `.format` removed from its last segment as Rails' optional format does. The first matching route
  in compiled order decides. A request under a literal `mount` path (`=> "/p"` or `at: "/p"`), or
  matching a literal rack-endpoint or redirect route, MUST read `uncertain` with reason
  `mounted-application`, `rack-endpoint` or `redirect` and that declaration's `path:line`; a
  mount, rack endpoint or redirect whose path is not literal covers every request path.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1967; GitHub #1215).
- `AMAP-V0-143`: Each cited request MUST read exactly one verdict: `accepted` when a route
  matches and AMAP-V0-102..105 find its action defined; `flagged` with rule `ROUTE-ACTION-MISSING`
  and the handler file and basis when they find it missing; `uncertain` with the abstention
  reason and `path:line` when they abstain; `uncertain` with `non-literal-request-path` for a
  path holding `{`, `}`, `$`, `*` or a `:param` segment; when no route matches, `uncertain` with
  `route-table-incomplete` and the first route abstention (or `no-server-route-table` when the
  manifest has no Rails router) if the table is incomplete, else `flagged` with rule
  `BLOCKER-404-NO-ROUTE`. A path the table cannot resolve is never flagged.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1967; GitHub #1215).
- `AMAP-V0-144`: A `BLOCKER-404-NO-ROUTE` or `route-table-incomplete` request MUST name up to 3
  nearest compiled routes sharing a literal segment with it, each with verb, path, declaration
  `path:line`, controller and action, ranked by: serving the method, then more request segments
  matched in order (`:param` matches any), then more shared literal segments, then the smaller
  segment-count difference, then compiled order.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1967; GitHub #1215).
- `AMAP-V0-145`: The report head MUST carry the map identity, budget, rule
  `BLOCKER-404-NO-ROUTE`, routers checked, route count, `route_table_incomplete`, the cited count,
  the overall verdict (`flagged` over `uncertain` over `accepted` over `not-applicable`) and the
  per-verdict counts, then the `requests` rows in note order through the AMAP-V0-011 renderer
  (default 6144 bytes). The command exits 1 after writing the report when a request is flagged,
  0 otherwise, and 2 with no stdout on any refusal.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1967; GitHub #1215).
- `AMAP-V0-146`: Router and handler reads MUST stay within the AMAP-V0-107 bounds, and the same
  committed inputs and note MUST produce a byte-identical report.
  Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1967; GitHub #1215).

## Wire contract

`corvint flows appmap build --manifest FILE [--revision REV] [--repo ALIAS=ABSOLUTE_ROOT]...
[--manifest-repo ALIAS]` writes `application-map/0` to stdout. The manifest's optional
`di_constants` array (AMAP-V0-021) adds no map member: a name it resolves carries `name_from` /
`parent_from` as AMAP-V0-016 does. With an aliased tests root the map
adds `roots: [{repo, revision, inputs}]` after `revision`, and `repo` as the first member of each
anchor and unknown read from that root (AMAP-V0-017/017); projections print `repo` on
those anchors. Optional `router_globs` adds `app_coverage` with `status`, `patterns`, `matched`,
`declared` and `undeclared` arrays. For example, `"router_globs": ["config/**/routes*.rb",
"app/**/routes.*"]` discovers regular tracked files in that scope; it does not auto-admit them
as routers. Rails helpers have no `_path`/`_url` suffix. Unnamed React declarations use
`react:<router-path>:<ordinal>` state names. The new dialects use whole-file anchors so lexical
scope changes cannot leave derived screens FRESH; draw context anchors join `name_from`.
`corvint flows appmap screen|flow|find|scaffold --map FILE --screen|--flow|--text VALUE
[--budget N | --full] [--revision REV]` writes `application-map-screen/0`,
`application-map-flow/0`, `application-map-find/0` or `application-map-scaffold/0`. Refusals exit 2
with one coded JSON error on stderr and nothing on stdout.
`corvint flows appmap refs --map FILE --source DIR [--source DIR]... [--budget N | --full]` writes
`application-map-refs/0`: `schema`, `app`, `map_revision`, `map_digest`, `budget`, `full`,
`check`, `limits`, `sources`, `source_files`, `source_files_skipped`, `test_files_read`,
`test_files_unread`, `screens_unknown`, `references`, `checked`, `unchecked_total`,
`misses_unlabelled`, `misses_labelled`, then `misses` (`kind`, `value`, `match`, optional
`class`, `occurrences`, `tests`), `unchecked` (`kind`, `reason`, `count`, `example`) and
`omitted` (AMAP-V0-090..099).
`corvint flows appmap actions --map FILE [--budget N | --full]` writes `application-map-actions/0`:
`schema`, `app`, `map_revision`, `map_digest`, `budget`, `full`, `rule`, `routers_checked`,
`routers_not_checked`, `routes`, `defined`, `missing`, `abstained`, `not_applicable_total`, then
`findings` (`rule`, `declaration`, `verb`, `path`, `controller`, `handler_file`, `action`,
optional `basis`), `abstentions` (`declaration`, optional `controller` and `action`, `reason`,
`at`) and `not_applicable` (`declaration`, `reason`) (AMAP-V0-100..107).
`corvint flows appmap branches --map FILE --cite PATH:START[-END] [--cite ...] [--budget N |
--full]` writes `application-map-branches/0`: `schema`, `app`, `map_revision`, `map_digest`,
`budget`, `full`, `limits`, `cites`, `branches_total`, `test_files_read`, `test_files_unread`,
`routes`, `setup_endpoints`, `factories_and_traits`, `fixtures`, then `branches` (`at`, `kind`,
`guard` or `guard_redacted`, `constant`, `method`, `boundaries` of `boundary`, `status`,
optional `note`, `count` and `tests` of `test`, `basis` and optional `route`, and optional
`setup` of `status`, `inputs` of `model`, `attribute`, `expression`, `writers` (`kind`, `name`,
`at`, `model`, `attributes` or `all_attributes`, `basis`) and optional `writers_omitted`, and
`unresolved` of `expression` and `reason`) and `no_branch` (`cite`, `reason`)
(AMAP-V0-110..116).
`corvint flows request-path --file PATH (--route 'METHOD /path' | --handler NAME) [--revision REV]
[--env-profile FILE [--warn]]` writes `corvint-request-path/0`: `schema`, `revision`, `file`,
`route` or `handler`, `steps` (`order`, `role`, `name`, `registered`, `outbound`, `envKeys`,
`configKeys`, `unknowns`, `file`, `line`), `envKeys` and `configKeys` (`name`, `defaulted`,
`steps`, `readAt`), `outboundCalls` (`step`, `callee`, `via`, `file`, `line`), `unknowns` (`kind`,
`reason`, `text`, `step`, `file`, `line`), `guardOrder` (`finding`, `outboundGuard`,
`cheapGuards`, `standInFor`, `message`), `filesRead` and, with `--env-profile`, `preRun`
(`profile`, `mode`, `verdict`, `batch`, `unset`, `empty`, `defaultedUnsetOrEmpty`,
`configKeysNotChecked`, `unresolved`, `message`) (AMAP-V0-130..139).

### Owned error codes

| Code | Meaning |
| --- | --- |
| `appmap-invalid-manifest` | The manifest is absent, not closed, over its bound, or names an escaping path, unknown dialect, directory outside `tests.root`, or declared directory absent at the revision. |
| `appmap-invalid-revision` | The build revision does not resolve to a commit, or a `refs`, `actions` or `branches` map revision is not a commit of `--root` or its manifest does not match the map's manifest anchor. |
| `appmap-invalid-flows` | A flow intent under the manifest's `flows` directory does not decode. |
| `appmap-invalid-map` | A map file is not a regular file, is over 64 MiB, is not closed, has trailing data, a wrong schema or a digest mismatch. |
| `appmap-invalid-query` | `--budget` is out of range or combined with `--full`, a `find` query is not 2..128 bytes, or a `refs` `--source` list is empty, over 16, duplicated, escaping or not a directory at the map revision, or a `branches` `--cite` list is empty, over 16, duplicated, escaping, not Ruby, absent, out of range or outside every application root. |
| `appmap-budget-too-small` | The budget cannot hold the projection head. |
| `appmap-bound-exceeded` | An input bound of AMAP-V0-015 was exceeded. |
| `appmap-root-unavailable` | A manifest alias is not declared with `--repo`, or its root is not absolute and clean, missing, not the top of a Git worktree, without a `HEAD` commit, the `--root` repository, or its status is unreadable (AMAP-V0-019). |
| `appmap-root-dirty` | An aliased root has staged, unstaged or untracked changes under the manifest or `tests.root` read from it (AMAP-V0-019). |
| `request-path-route-not-found` | `--file` has no literal registration of the `--route` method and path, or the registration names no handler (AMAP-V0-138). |
| `request-path-route-ambiguous` | `--file` registers the `--route` method and path more than once. |
| `request-path-handler-not-found` | No function `--handler` is declared or imported in `--file`. |
| `request-path-file-not-found` | `--file` is not a committed regular file at the revision. |
| `request-path-invalid-revision` | `--revision` does not resolve to a commit. |
| `request-path-source-unavailable` | A Git read of `--file` or of the `--handler` module failed. |
| `request-path-bound-exceeded` | `--file` or the `--handler` module exceeds 1 MiB, or the 64-file bound of AMAP-V0-137 was reached before `--file` was read. |
| `request-path-profile-invalid` | `--env-profile` is unreadable, over 1 MiB, or has a line that is not `KEY=VALUE` (named by line number only). |

### Unknown reasons

`non-literal-name`, `non-literal-url`, `non-literal-value`, `duplicate-state`, `missing-parent`,
`unresolved-parent`, `parent-cycle`, `ambiguous-template`, `no-matching-screen`,
`no-matching-flow`, `partial-segment-substitution`, `no-navigation-step`, `unresolved-import`,
`import-depth-exceeded`, `unparsed-imports`, `excluded-by-index`, `page-object-unbound`,
`page-object-ambiguous`, `page-object-unresolved-target`, `import-outside-tests`, `unknown-state`,
`secret-shaped`, `no-asserting-spec`, `binding-collision`, `stale-reuse`, `reuse-not-callable`, `test-unbound`, `unread-statement`,
`overlay-unavailable` and `overlay-bound-exceeded`; for kind `di-constant` (AMAP-V0-026),
`unreadable-registration`, `identifier-not-found`, `ambiguous-binding`, `not-read-whole`,
`out-of-scope`, `ambiguous-barrel`, `barrel-depth-exceeded`, `non-literal-member` and
`member-not-found`. Unknown kinds are `state`, `screen`,
`screen-permissions`, `screen-flags`, `template`, `step`, `file`, `import`, `test-join`,
`page-object`, `selector`, `scaffold`, `scaffold-import`, `scaffold-reuse`, `overlay` and
`di-constant` and (V1-1131) `router`.

`flows request-path` (V1-1941, AMAP-V0-134) uses its own kinds and reasons, listed in the
request-path section at the end of this spec.

V1-1131 adds `router-not-declared`, `unsupported-rails-syntax`, `unsupported-rails-statement`,
`rails-constraints`, `rails-defaults`, `non-literal-rails-scope`, `non-literal-rails-resource`,
`unsupported-rails-inflection`, `unsupported-rails-scope`, `non-literal-rails-route`,
`non-literal-draw`, `repeated-draw`, `draw-unavailable`, `draw-depth-exceeded`,
`unsupported-react-syntax`, `react-routes-not-found`, `non-literal-react-routes`,
`non-literal-react-route`, `non-literal-react-children`, `lazy-route`, `non-literal-path`,
`non-literal-index`, `invalid-index-route`, `invalid-absolute-child`, `unsupported-react-option`,
`dynamic-route-context`, `router-depth-exceeded` and `unsupported-route-pattern`; V1-1169 adds
`imported-react-routes` and `computed-react-routes` (AMAP-V0-078); V1-1220 adds
`unresolved-react-routes-import` and `ambiguous-react-routes-import` (AMAP-V0-079). Unknown helper
names also reuse `non-literal-name`; blocked descendants reuse `unresolved-parent`. Unsupported
Ruby block structure invalidates the file's extracted states. React root conditionals invalidate
its states, while JSX quote entities/escapes and unrepresentable optional/partial parameter
patterns remain UNKNOWN. This does not evaluate Ruby custom inflections or JavaScript control flow.

V1-1890 (`actions`) abstention reasons: the V1-1131 Rails reasons above, `unresolved-parent`, `no-handler`,
`handler-file-not-found`, `handler-class-not-found`, `handler-file-unreadable`,
`unresolved-ancestor`, `unresolved-module`, `dynamic-definition`, `ancestor-depth-exceeded` and
`unsupported-ruby-syntax`; not-applicable reasons `no-server-handler`, `mounted-application`,
`redirect` and `rack-endpoint`. V1-1891 (`branches`) adds `no-branch` and `unsupported-ruby-syntax`
for a cite, and input reasons `request-input`, `unresolved-receiver`, `unsupported-expression`
and `redacted`.

## Overlay seam

`internal/appmap/overlay.go` defines `Fact{element_id, source, kind, revision, text, authority}`,
`AuthorityLearned = "learned"` and `Overlay.Facts(ctx, elementIDs) ([]Fact, error)`; callers pass
overlays in `Options.Overlays`. Element IDs are the stable IDs of Definitions, so a fact recorded
against `step:book-tee-time/select-slot` or `method:e2e/pages/teesheet.page.ts#selectSlot` finds
its element in any later map where that element still exists. Issue 658 (V1-0957, run-verified
steps) is specified separately in `docs/specs/run-verified-navigation-v0.md` (RVN-V0): it adds an
optional `tests` array to map steps (declared AFU-V1 `test` links) and supplies step status as a
`run-verification` overlay through this seam.

## Non-goals and simpler baseline

- The accepted baseline is `ui-router-states/0`; Rails and React support below is accepted
  (decision 0518) experimental V1-1131 scope. Vue Router and Angular Router remain follow-ups. No Cypress `cy.visit` or Playwright fixture-injected page objects.
- No TypeScript `paths`/`baseUrl` alias resolution (V1-0958 owns it).
- No MCP tools, documentation-corpus records or persisted index; the map is an explicit file.
- No revision other than `HEAD` for an aliased root, and no projection, planner or run-verification
  read of an aliased root (its anchors read `UNKNOWN`, AMAP-V0-020); routers and flows never come
  from an aliased root.
- No AngularJS evaluation or module graph: an injected state-name table is read only from the
  router file and the manifest's `di_constants` scope (AMAP-V0-021..023).
- No ranking or learning; overlays are advisory. Run verification is RVN-V0, opt-in per call.
- No browser execution and no write of the scaffold.
- Simpler baseline: grep specs by URL. It misses click-through specs and is unbounded; the
  fixture's `teesheet-click.spec.ts` is the counter-example.

## Trust boundary, limits and failure modes

- Router files, tests and intents are untrusted repository data: values are read literally, never
  evaluated; anything non-literal is `UNKNOWN`.
- A map file is untrusted input: closed decode plus digest check; it cannot change what Git says
  about freshness.
- Failure modes: a dynamic route or parent (`UNKNOWN`), colliding templates (`UNKNOWN` with
  candidates), an alias or missing import (join `UNKNOWN`), a page object with no or several URLs
  (unbound), Git unavailable at projection time (`UNKNOWN` freshness), an oversized repository
  (`appmap-bound-exceeded`), an overlay failure (`overlay-unavailable`, map facts unaffected), a
  test file the index excluded or could not read (join `UNKNOWN`), a partial literal argument or
  inexact escape (`unknown` strength or `non-literal-url`), a role name a spread could override
  (`unknown`), an ancestor state changed under an unchanged screen (`lineage_freshness: STALE`),
  two reused page objects with one class name (`binding-collision`), and a hand-edited map whose
  anchors disagree with Git (`STALE`, even at the map's own revision) or that claims reuse for a
  step with no selector (TODO, no call), a repeated object key (non-literal), a role name passed by
  reference (`unknown`), a page object with a non-literal target (unbound), an import outside
  `tests.root` (join `UNKNOWN`), a changed workflow under an unchanged spec (`chain_freshness:
  STALE`), a reused method changed since the map was built (`stale-reuse`, no call), a short
  `\u` escape at the end of a file (inexact, no read past the end), a URL in a query parameter
  (data, not an authority), a literal router object that is only part of its argument
  (`non-literal-value`), a manifest rebinding a page object (`chain_freshness: STALE`), and a
  reused member that is not a public method callable without arguments (`reuse-not-callable`, no
  call), a `data-test` attribute locator (CSS, never a test ID), a role options literal that is
  only part of its argument (`unknown`), two import statements from one module (kept apart), and
  a closest spec that calls `test` under another name (`test-unbound`), an object-literal method
  outside the class body (not callable), an import statement over 8 lines (`unread-statement`, no
  partial copy), a scheme URL whose query names a path (the root), a regular expression after a
  control-statement condition (no selector), and a router file truncated inside a state call
  (non-literal, no panic), and a state name or parent read through a constant table the map cannot
  prove whole (AMAP-V0-016; `non-literal-name` / `non-literal-value`), or one another module
  mutates at run time (not read; a static map cannot see it), and a state name or parent read
  through an injected `.constant(...)` table the map cannot prove unique and whole within the
  `di_constants` scope (AMAP-V0-023; `non-literal-name` / `non-literal-value`), or re-annotated
  outside the router file (not read); an injected enum with any non-literal member (AMAP-V0-024),
  or a table behind two candidate, two-level or out-of-repository re-exports (AMAP-V0-025), stays
  `UNKNOWN` with a `di-constant` unknown naming why (AMAP-V0-026).
- Second repository (AMAP-V0-017..019): an undeclared alias or an unusable root refuses with
  `appmap-root-unavailable`; uncommitted changes under what the map reads from it refuse with
  `appmap-root-dirty`; a same-named path in `--root` never makes an aliased anchor `FRESH`.
- Limits: per-method and per-file anchors, not per-statement; flow steps cite their intent file;
  `test_join` is global, not per screen.
- Accepted limit, out of scope (V1-1061; GitHub #705): the injected-table reader (AMAP-V0-021..026)
  assumes non-adversarial application source. It fails closed on the forms ordinary code uses to
  replace or shadow `angular.module`, but code that deliberately rebuilds the `angular` global's
  name, or reaches it through other host objects or string construction, can still leave a stale
  literal state name. Reviews treat that class as out of scope; repository data is still never
  evaluated.

## Deterministic acceptance and traceability

| Requirement | Evidence |
| --- | --- |
| AMAP-V0-001 | `TestAMAPV0015ReadOnlyAndRefusals`, `TestAMAPV0001DeclaredDirectoriesExist` |
| AMAP-V0-002 | `TestAMAPV0002HierarchyResolution`, `TestAMAPV0002WildcardExhaustedSubject`, `TestAMAPV0002RepeatedKeys`, `TestAMAPV0002ConfigMustBeWholeArgument`, `TestAMAPV0002TruncatedStateCall` |
| AMAP-V0-003 | `TestAMAPV0002HierarchyResolution`, `TestAMAPV0003QueryURLIsNotAuthority`, `TestAMAPV0003AuthorityEndsAtQueryOrFragment` |
| AMAP-V0-004 | `TestAMAPV0004FlowSteps` |
| AMAP-V0-005 | `TestAMAPV0005ImportGraphJoin`, `TestAMAPV0005UnreadSpecKeepsJoinUnknown`, `TestAMAPV0005UnboundPageObjectKeepsJoinUnknown`, `TestAMAPV0005UnresolvedTargetBlocksBinding`, `TestAMAPV0005ImportOutsideTests`, `TestAMAPV0005SeparateImportsFromOneModule`, `TestAMAPV0007PartialLiterals` |
| AMAP-V0-006 | `TestAMAPV0002HierarchyResolution`, `TestAMAPV0004FlowSteps` |
| AMAP-V0-007 | `TestAMAPV0007SelectorStrength`, `TestAMAPV0007PartialLiterals`, `TestAMAPV0007EscapesAndSpreads`, `TestAMAPV0007RegexReferencedOptionsAndContinuations`, `TestAMAPV0007ShortUnicodeEscape`, `TestAMAPV0007TestIDAttributeIsExact`, `TestAMAPV0007RoleOptionsWholeArgument`, `TestAMAPV0007RegexAfterControlCondition` |
| AMAP-V0-008 | `TestAMAPV0005ImportGraphJoin`, `TestAMAPV0004FlowSteps` |
| AMAP-V0-009 | `TestAMAPV0009DeterministicArtifact`, `TestAMAPV0FlowsAppmapCLI` |
| AMAP-V0-010 | `TestAMAPV0010StaleAnchors`, `TestAMAPV0010SameRevisionVerified`, `TestAMAPV0010AncestorLineageStale`, `TestAMAPV0010ChainFreshness`, `TestAMAPV0010ManifestBindingFreshness` |
| AMAP-V0-011 | `TestAMAPV0011ProjectionBudgets`, `TestAMAPV0011EveryBudgetFits`, `TestAMAPV0011FlowReportsScreenUnknowns`, `TestAMAPV0FlowsAppmapCLI` |
| AMAP-V0-012 | `TestAMAPV0012Find`, `TestAMAPV0012FindByID` |
| AMAP-V0-013 | `TestAMAPV0013Scaffold`, `TestAMAPV0013AliasedImportRebound`, `TestAMAPV0013UnknownSelectorNotReused`, `TestAMAPV0013ReuseWithoutSelector`, `TestAMAPV0013GeneratedBindingCollision`, `TestAMAPV0013StaleReuseNotCalled`, `TestAMAPV0013MethodWithArgumentsNotCalled`, `TestAMAPV0013TestUnbound`, `TestAMAPV0013MethodOutsideClassNotCallable`, `TestAMAPV0013UnreadImportStatement` |
| AMAP-V0-014 | `TestAMAPV0014OverlaySeam`, `TestAMAPV0014FactsFollowTrimmedElements` |
| AMAP-V0-015 | `TestAMAPV0015ReadOnlyAndRefusals`, `TestAMAPV0FlowsAppmapCLI` |
| AMAP-V0-016 | `TestAMAPV0016ConstantStateNames`, `TestAMAPV0016UnprovableConstantsStayUnknown` |
| AMAP-V0-017 | `TestAMAPV0017TwoRootJoin`, `TestAMAPV0017RootUnavailable`, `TestAMAPV0017RootPathKeepsTrailingSpace` |
| AMAP-V0-018 | `TestAMAPV0017TwoRootJoin`, `TestAMAPV0018SingleRootUnchanged` |
| AMAP-V0-019 | `TestAMAPV0017RootUnavailable`, `TestAMAPV0019DirtySecondRoot` |
| AMAP-V0-020 | `TestAMAPV0020ProjectionsReadAliasedAnchorsUnknown` |
| AMAP-V0-021 | `TestAMAPV0021DIConstantsManifest`, `TestAMAPV0022InjectedStateNames` |
| AMAP-V0-022 | `TestAMAPV0022InjectedStateNames`, `TestAMAPV0022InjectionAnnotations` |
| AMAP-V0-023 | `TestAMAPV0023UnprovableInjectionStaysUnknown`, `TestAMAPV0023ScopePoisoned` |
| AMAP-V0-024 | `TestAMAPV0024InjectedStringEnum`, `TestAMAPV0025InjectedTableThroughBarrel` |
| AMAP-V0-025 | `TestAMAPV0025InjectedTableThroughBarrel` |
| AMAP-V0-026 | `TestAMAPV0026DIConstantDiagnostics`, `TestAMAPV0024InjectedStringEnum`, `TestAMAPV0025InjectedTableThroughBarrel` |
| AMAP-V0-040 | `TestAMAPV0040MixedDialects` |
| AMAP-V0-041 | `TestAMAPV0041RailsRoutes` |
| AMAP-V0-042 | `TestAMAPV0042RailsUnknownAndDraw` |
| AMAP-V0-043 | `TestAMAPV0043ReactRoutes` |
| AMAP-V0-044 | `TestAMAPV0044ReactUnknown` |
| AMAP-V0-045 | `TestAMAPV0045RouterCoverage` |
| AMAP-V0-046 | `TestAMAPV0046SharedTemplates` |
| AMAP-V0-047 | `TestAMAPV0047Bounds` |
| AMAP-V0-048 | `TestAMAPV0048DialectCLI` |
| AMAP-V0-078 | `TestAMAPV0078ReactSameFileRouteArray` |
| AMAP-V0-079 | `TestAMAPV0079ReactImportedRouteArray` |

Implementation: `internal/appmap`, `internal/contextindex/webimport_api.go`,
`cmd/corvint/flows_appmap.go`, `internal/rootalias` (AMAP-V0-017). Build logs:
`docs/build-log/2026-10-07-application-map.md`, `docs/build-log/2026-10-07-appmap-second-root.md`,
`docs/build-log/2026-10-08-appmap-injected-state-names.md` (AMAP-V0-021..023,
`internal/appmap/stateinject.go`), `docs/build-log/2026-10-09-v1-1061-appmap-enum-barrel.md`
(AMAP-V0-024..026).

## Rollout, rollback and compatibility

The slice is additive and experimental: a new package, one exported read-only contextindex
wrapper and one `flows` subcommand. Rollback removes `internal/appmap`,
`cmd/corvint/flows_appmap.go`, `internal/contextindex/webimport_api.go`, the `flowsAppmapHelp`
reference in `cmd/corvint/help.go` and this spec's index rows; no stored state, schema or ledger
needs migration. Map files are explicit outputs and may be discarded.
AMAP-V0-016 adds two optional screen members (`name_from`, `parent_from`) that literal-only maps
omit; rolling it back removes `internal/appmap/stateconst.go`, the `member` value kind in
`internal/appmap/jslex.go` and the lookup argument of `parseRouter`, after which those names read
`non-literal-name` again and a map that carries the members still decodes in a build with them.
The second-root requirements AMAP-V0-017..020 are additive: rolling them back
 removes the `repo`/`roots` members, `BuildRoots`, the two refusal codes
and the `--repo`/`--manifest-repo` flags; `internal/rootalias` may stay as the MMR-V0 helper.
The injection requirements AMAP-V0-021..023 are additive and opt-in: rolling them back removes
`internal/appmap/stateinject.go`, the `di_constants` manifest member and the token position the
`member` value carries; a manifest that declares the member is then refused as not closed, and
names it resolved read `non-literal-name` again.
The enum, re-export and diagnostic requirements AMAP-V0-024..026 are additive within the opt-in
scope: rolling them back removes the `di` mode of
`lookup`, `reexported`, `readReexport` and the enum `partial` flag in
`internal/appmap/stateconst.go` and the `di-constant` unknowns; injected enums with a non-literal
member then resolve their literal members again, tables behind a re-export read
`non-literal-name`, and no map member or schema changes.

## Follow-ups (proposed tickets)

1. Corpus record family and MCP tools `map_screen`, `map_flow`, `map_find`, `map_scaffold`.
2. Further router dialects (React Router, Vue Router, Angular Router).
3. Cypress `cy.visit` and Playwright fixture-injected page objects in the join.
4. Default-import bindings and re-exports in scaffold borrowing.
5. Per-step line anchors in flow intents.
6. Adopter-scale qualification: compile time, map size and projection latency on a large app.
7. A precomputed find index for maps beyond fixture size.
8. Re-test alias-imported specs once V1-0958 lands.
9. Fall back to the next reuse candidate when the first page object's binding collides.
10. AMAP-V0-016 extensions: CommonJS `require` bindings, re-exports and namespace imports
    (`C.X.Y`). (A table reached through AngularJS dependency injection is AMAP-V0-021..023.)
11. Let projections, `flows plan` and the corpus MCP map tools accept `--repo` so aliased anchors
    read `FRESH`/`STALE` instead of `UNKNOWN` (AMAP-V0-020).
12. A per-root revision (`--repo-revision ALIAS=REV`) instead of `HEAD` only.
13. Carry `repo` on scaffold and planner unknowns and exploration refs that cite aliased files.
14. AMAP-V0-022 extensions: annotations in another file (`module.config(['$stateProvider', 'X',
    Routes])`), `.run(...)` and provider functions, and per-module registration scoping.

## Owner questions

Decision 0446 keeps every V0 default below; each question stays open for a later owner answer.

1. Should a static segment outrank a parameter (`/clubs/new` over `/clubs/{id}`) instead of the
   current fail-closed `ambiguous-template`?
2. Is a new AMAP-V0 spec right, or should this extend AFU-V1 or DCP-V1?
3. Which MCP surface and corpus record shape should expose the projections?
4. Is the exported contextindex wrapper (`WebImportBindings`, `ResolveWebImport`) the accepted API,
   or should the map consume a different import relation?
5. Should `test_join` be per screen instead of `UNKNOWN` globally while any spec is unresolved?
6. Are the default budgets (4/6/2/6 KiB) right?
7. Should a pure line shift read `FRESH` by content search instead of the fail-closed `STALE`?
8. Should the join follow imports beyond `tests.root` (shared source helpers) instead of the
   fail-closed `import-outside-tests`?
9. Should a page object with one literal and one non-literal target bind to the literal one instead
   of the fail-closed `page-object-unresolved-target`?
10. Should a repeated object key take the last value (JavaScript semantics) instead of making the
    router object non-literal?
11. Should the scaffold still call a `STALE` reused method with a warning instead of leaving a TODO?
12. Should a reused method that takes parameters be called with argument TODOs (invalid until
    edited) instead of the fail-closed TODO, and should the manifest anchor be narrowed from the
    whole file to the `page_object_screens` entry?
13. Should the scaffold generate `import { test } from '@playwright/test'` (or call the borrowed
    alias) instead of the fail-closed `test-unbound`, and should a Playwright `testIdAttribute`
    configuration make another attribute the test-ID attribute?
14. The map reads JavaScript and TypeScript with a token- and line-level reader, not a full
    parser; review rounds 1 to 6 each found further edge cases, each repaired fail-closed. Should
    an adopter-scale qualification on real test repositories (follow-up) decide whether a full
    parser is needed before promotion?
15. For a second repository (V1-0982): is `HEAD` plus a dirty refusal scoped to the read inputs the
    right policy, or should an aliased root take an explicit revision, or allow a dirty tree and
    read `UNKNOWN`?
16. For injected state-name tables (V1-1027): is an explicit manifest scope (`di_constants`) the
    right way to say which `.constant(...)` registrations belong to the app, or should the map
    infer the scope from AngularJS module names or the router's import closure, and should a
    poisoned scope report its own unknown instead of leaving each name `non-literal-name`?

## V1-1131 experimental scope and rollback

The owner request is issue 773. Decision 0518 accepts these additions (AMAP-V0-040..048).
Implementation owns new router dialect/coverage files and the minimal manifest/build dispatch and
shared-template seams; edge coverage, flow extraction, test grouping and change impact stay with
the other lane. Start with parser fixtures, then committed build/CLI witnesses and focused gates.
Rollback removes the new dialect declarations and router_globs from manifests and rebuilds maps;
older closed-schema consumers must rebuild without the new optional coverage member. Full Ruby/JS
evaluation, Rails custom inflections/engines, runtime route mutation, imported route arrays and
adopter qualification are excluded. Dynamic route sets remain UNKNOWN. No network or interpreter
is invoked by the builder. Model routing: current builder for bounded parser work, independent
review for fail-closed and evidence-boundary mistakes; spend telemetry is NOT_OBSERVED.

## Static entries slice (V1-1136)

Owner input: issue #778 as supplied for this build. Implementation is experimental; numbered
requirements above are accepted (decision 0518). The smallest baseline reads the existing map without launching
the adopter. `entries --map FILE [--revision REV] [--budget N | --full]` adds a separate projection;
UI-router screens carry optional `entry` declaration metadata. Existing maps remain loadable by the
new reader; older closed readers require rebuilding with their matching producer. No migration
or runtime receipt is supplied. Rollback removes this projection and optional metadata, then
rebuilds maps. No edge-ledger, test-plan or change-impact behavior is changed.

| Requirement | Implementation | Focused evidence |
| --- | --- | --- |
| AMAP-V0-050, AMAP-V0-051, AMAP-V0-052, AMAP-V0-053 | `internal/appmap/entries.go`, `internal/appmap/router.go` | `TestEntriesClassification` |
| AMAP-V0-054, AMAP-V0-055 | `internal/appmap/entries.go` | `TestEntriesRevisionBinding` |
| AMAP-V0-056, AMAP-V0-058 | `internal/appmap/entries.go` | `TestEntriesLegacyAndUnprovenEdges` |
| AMAP-V0-057, AMAP-V0-059 | `cmd/corvint/flows_appmap_entries.go` | `TestEntriesCLI`, `TestEntriesBudget` |
| AMAP-V0-080, AMAP-V0-081, AMAP-V0-082 | `internal/appmap/entries.go`, `cmd/corvint/flows_appmap_entries.go` | `TestAMAPV0080EntriesLedgerPaths`, `TestEntriesCLI` |

V1-1168 (issue #778) adds `--ledger` (AMAP-V0-080..082): "runtime" paths are read from retained
observed navigation evidence, the edge ledger's receipt-backed PROVEN edges, never by launching
the application. Rollback removes the flag; the static projection is unchanged.

Promotion needs owner acceptance; runtime verification, non-UI-router entry
metadata, dynamic resolves/params interpretation and adopter-scale qualification remain deferred.

## Test reference check slice (V1-1581)

Owner input: [issue 1025](https://github.com/beamfall/corvint/issues/1025) (V1-1581). The
requirements AMAP-V0-090..099 are accepted under the owner's standing auto-accept of 2026-10-11
and delivered experimental. Job: before a Playwright suite runs, list the routes, selectors and
mocked endpoint segments the tests name that the application source does not, so a stale test
reference is found statically. The check reuses the map's test files and URL matching
(AMAP-V0-003, AMAP-V0-007) and the V1-1134 call scan; it adds no map member, manifest member or
error code.

| Requirement | Implementation | Focused evidence |
| --- | --- | --- |
| AMAP-V0-090 | `internal/appmap/refs.go`, `cmd/corvint/flows_appmap_refs.go` | `TestAMAPV0090RefsReadOnlyContract`, `TestAMAPV0098RefsCLI` |
| AMAP-V0-091 | `internal/appmap/refs.go` | `TestAMAPV0091ScanRefs`, `TestAMAPV0095LabelledClasses` |
| AMAP-V0-092 | `internal/appmap/refs.go`, `internal/appmap/jslex.go` | `TestAMAPV0092ValueModes` |
| AMAP-V0-093 | `internal/appmap/refs.go`, `internal/appmap/router.go` | `TestAMAPV0093RouteCheck` |
| AMAP-V0-094 | `internal/appmap/refs.go` | `TestAMAPV0094NameLevelCheck`, `TestAMAPV0090RefsReadOnlyContract` |
| AMAP-V0-095 | `internal/appmap/refs.go` | `TestAMAPV0095LabelledClasses` |
| AMAP-V0-096 | `internal/appmap/refs.go` | `TestAMAPV0096BoundedOutput`, `TestAMAPV0098RefsCLI` |
| AMAP-V0-097 | `internal/appmap/refs.go` | `TestAMAPV0097CorpusBounds` |
| AMAP-V0-098 | `cmd/corvint/flows_appmap_refs.go` | `TestAMAPV0098RefsCLI`, `TestAMAPV0098UnlabelledCount` |
| AMAP-V0-099 | `internal/appmap/refs.go` | `TestAMAPV0099SecretShapedValues` |

Fixture: `internal/appmap/testdata/refs` (a `react-router/0` app with Playwright specs and a page
object). Live qualification runs the command over the committed AMAP-V0 fixture and over this one
and retains the bounded output in the build log.

Non-goals: no usage, reachability, rendering or attribute-placement analysis (a name that occurs
anywhere in the source is present); no runtime request matching of `route` globs or regular
expressions; no Cypress, WebdriverIO or framework-specific `testIdAttribute` configuration; no
text-level selector check; no reading of an aliased root, generated files or build output; no map
rebuild. Simpler baseline: grep each test literal across the source. It has no route semantics,
no prefix handling for templated values and no false-positive labels.

Failure modes: a name present only in a comment or unrelated string reads present (name-level
false negative); a name the application builds at run time without a literal prefix reads as an
unlabelled miss (false positive); a route served by a router the manifest does not declare, or by
a screen that stays UNKNOWN, reads as a miss and `screens_unknown` says how many screens could not
match; an unanchored regular expression is treated as a path prefix and labelled. Each outcome is
named in the report rather than resolved by guessing.

Rollback: remove `internal/appmap/refs.go`, `cmd/corvint/flows_appmap_refs.go`, the `refs` verb
in `runFlowsAppmap` and its help text, the `regex` token member in `internal/appmap/jslex.go`
(nothing else reads it) and this section's requirements and fixture; `urlPath` in
`internal/appmap/router.go` is a behavior-preserving extraction that may stay. No stored state,
map schema or ledger needs migration; the report is an explicit stdout document.

## Route action and branch query slices (V1-1890, V1-1891)

Owner input: [issue 1174](https://github.com/beamfall/corvint/issues/1174) (V1-1890) and
[issue 1175](https://github.com/beamfall/corvint/issues/1175) (V1-1891). The requirements of
this slice, AMAP-V0-100..107 and AMAP-V0-110..116, are accepted under the owner's standing auto-accept of
2026-10-11 and delivered experimental. Jobs: before a Rails change ships, report each compiled
route whose controller does not define its action (`ROUTE-ACTION-MISSING`) with the declaration
line, handler and action; and, for each branch a reviewer cites, list the tests that reach it by
boundary and, when a boundary is unreached, the setup endpoints, factories, traits and fixtures
that write the guard's inputs. Both verbs reuse the map's `rails-routes/0` routers (AMAP-V0-041)
and test files; they add no map member, manifest member or error code.

| Requirement | Implementation | Focused evidence |
| --- | --- | --- |
| AMAP-V0-100 | `internal/appmap/route_handlers.go`, `cmd/corvint/flows_appmap_actions.go` | `TestAMAPV0100ActionsReadOnlyContract`, `TestAMAPV0100ClientRoutersNotChecked`, `TestAMAPV0106ActionsCLI` |
| AMAP-V0-101 | `internal/appmap/route_actions.go` | `TestAMAPV0101ActionVerdicts`, `TestAMAPV0101RouteCompilation`, `TestAMAPV0105ChainDoubtAndDraws`, `TestAMAPV0101AbsoluteControllersAndUpdateVerbs` |
| AMAP-V0-102 | `internal/appmap/ruby_structure.go`, `internal/appmap/route_handlers.go` | `TestAMAPV0101ActionVerdicts`, `TestAMAPV0100ActionsReadOnlyContract`, `TestAMAPV0103LookupOrderCases` |
| AMAP-V0-103 | `internal/appmap/route_handlers.go` | `TestAMAPV0101ActionVerdicts`, `TestAMAPV0105ChainDoubtAndDraws`, `TestAMAPV0103LookupOrderCases` |
| AMAP-V0-104 | `internal/appmap/route_handlers.go` | `TestAMAPV0101ActionVerdicts`, `TestAMAPV0103LookupOrderCases` |
| AMAP-V0-105 | `internal/appmap/ruby_structure.go`, `internal/appmap/route_handlers.go` | `TestAMAPV0101ActionVerdicts`, `TestAMAPV0105ChainDoubtAndDraws`, `TestAMAPV0103LookupOrderCases` |
| AMAP-V0-106 | `internal/appmap/route_handlers.go`, `cmd/corvint/flows_appmap_actions.go` | `TestAMAPV0106ActionsBudget`, `TestAMAPV0106ActionsCLI` |
| AMAP-V0-107 | `internal/appmap/route_handlers.go` | `TestAMAPV0107OversizedHandler`, `TestAMAPV0116SharedReadBudget` |
| AMAP-V0-110 | `internal/appmap/branch_tests.go`, `cmd/corvint/flows_appmap_branches.go` | `TestAMAPV0110BranchesReadOnlyContract`, `TestAMAPV0116BranchesCLI`, `TestAMAPV0116SharedReadBudget` |
| AMAP-V0-111 | `internal/appmap/branch_tests.go` | `TestAMAPV0111BranchReachAndSetup`, `TestAMAPV0113UnresolvedGuards` |
| AMAP-V0-112 | `internal/appmap/branch_tests.go` | `TestAMAPV0111BranchReachAndSetup`, `TestAMAPV0112CallbackAndHelperReach`, `TestAMAPV0112CallbackRestrictions`, `TestAMAPV0112CommentedCallsIgnored` |
| AMAP-V0-113 | `internal/appmap/branch_tests.go` | `TestAMAPV0113UnresolvedGuards`, `TestAMAPV0111BranchReachAndSetup` |
| AMAP-V0-114 | `internal/appmap/setup_writers.go` | `TestAMAPV0111BranchReachAndSetup`, `TestAMAPV0114FactoryInheritanceAndClass` |
| AMAP-V0-115 | `internal/appmap/branch_tests.go` | `TestAMAPV0111BranchReachAndSetup`, `TestAMAPV0113UnresolvedGuards` |
| AMAP-V0-116 | `internal/appmap/branch_tests.go`, `cmd/corvint/flows_appmap_branches.go` | `TestAMAPV0116BudgetAndRedaction`, `TestAMAPV0116BranchesCLI`, `TestAMAPV0116SharedReadBudget` |

Fixture: the inline Rails application of `internal/appmap/route_actions_test.go`
(`railsFixtureFiles`: routes, controllers with an ancestor, a concern, a template-only action,
a dynamic definition, an unresolved ancestor, a missing file and class, models, a factory with a
trait, a YAML fixture, a setup endpoint, unit and request specs and a Playwright spec) and the
CLI fixture of `cmd/corvint/flows_appmap_actions_test.go`. The setup-writer index
(`newSetupWriterIndex`, `writersOf` in `internal/appmap/setup_writers.go`) is the reuse point
for V1-1900.

Non-goals: no general data-setup model, data-flow analysis or evaluation of Ruby; no execution
coverage (reach is static naming evidence); no Rails custom inflections, routing constraints,
engines' own routes or `respond_to`-style dynamic dispatch; no Sinatra, Grape or non-Rails
handler; no mined branch kinds beyond AMAP-V0-111; no map rebuild. Simpler baseline: grep the
controller for `def <action>` and the specs for the method name. It has no route compilation,
`only:`/`except:` handling, ancestors, concerns or templates, and no named abstentions.

Failure modes: an action defined by code the reader does not follow (a gem's concern, an
engine, a `method_missing` outside the chain) reads missing only when no chain member shows a
dynamic definition, so a finding is a statement about the read chain, not about Ruby at run
time; a test that names a branch's method without executing the branch reads `reached` (static
naming false positive); a writer named by a factory or endpoint may not produce the value the
guard needs. Each outcome carries its basis or reason rather than a guess.

Rollback: remove `internal/appmap/route_actions.go`, `route_handlers.go`, `ruby_structure.go`,
`branch_tests.go`, `setup_writers.go`, `cmd/corvint/flows_appmap_actions.go`,
`cmd/corvint/flows_appmap_branches.go`, the `actions` and `branches` verbs in `runFlowsAppmap`
with their help text, and this section's requirements. No stored state, map schema or ledger
needs migration; the reports are explicit stdout documents.

## Request-path environment, outbound calls and guard order (V1-1941)

Owner input: [issue 1200](https://github.com/beamfall/corvint/issues/1200) (V1-1941). The
requirements AMAP-V0-130..139 are accepted under the owner's standing auto-accept of 2026-10-11
and delivered experimental. Job: before a test batch runs against a target environment, know which
environment and configuration keys one route's request path reads (guards included), which of its
guards call out to the network, and whether such a guard runs before a cheaper validation guard
(so a test of the later step needs a stand-in), and refuse to schedule the batch when a required
key is unset or empty, without ever printing a value. The reader is the new package
`internal/requestpath`; it reuses `appmap.LexJS` and adds no map, manifest or index member.

| Requirement | Implementation | Focused evidence |
| --- | --- | --- |
| AMAP-V0-130 | `internal/requestpath/requestpath.go`, `internal/requestpath/git.go`, `cmd/corvint/flows_request_path.go` | `TestAMAPV0138RequestPathCLI` |
| AMAP-V0-131 | `internal/requestpath/requestpath.go`, `internal/requestpath/scan.go`, `internal/requestpath/parse.go` | `TestAMAPV0131TokenExchangeKeysAndOutbound`, `TestAMAPV0132CommonJSRouterChain`, `TestAMAPV0134ReviewReproductions` |
| AMAP-V0-132 | `internal/requestpath/scan.go` | `TestAMAPV0131TokenExchangeKeysAndOutbound`, `TestAMAPV0132CommonJSRouterChain`, `TestAMAPV0134ReviewReproductions` |
| AMAP-V0-133 | `internal/requestpath/requestpath.go` | `TestAMAPV0133GuardOrderFlagged`, `TestAMAPV0133GuardOrderNeedsProvenCheapGuard`, `TestAMAPV0138RequestPathCLI` |
| AMAP-V0-134 | `internal/requestpath/scan.go`, `internal/requestpath/requestpath.go` | `TestAMAPV0134UnresolvableIsUnknown`, `TestAMAPV0134DynamicConfigKey`, `TestAMAPV0134UnresolvedMiddlewareAndReceiver`, `TestAMAPV0134ReviewReproductions` |
| AMAP-V0-135 | `internal/requestpath/profile.go` | `TestAMAPV0135PreRunBlocksByNameOnly`, `TestAMAPV0135ProfileRefusesByLineOnly`, `TestAMAPV0135QuotedEmptyWithComment`, `TestAMAPV0134DynamicConfigKey`, `TestAMAPV0138RequestPathCLI` |
| AMAP-V0-136 | `internal/requestpath/profile.go`, `cmd/corvint/flows_request_path.go` | `TestAMAPV0135PreRunBlocksByNameOnly`, `TestAMAPV0134UnresolvableIsUnknown`, `TestAMAPV0138RequestPathCLI` |
| AMAP-V0-137 | `internal/requestpath/requestpath.go`, `internal/requestpath/git.go` | `TestAMAPV0137Bounds` |
| AMAP-V0-138 | `cmd/corvint/flows_request_path.go`, `internal/requestpath/requestpath.go` | `TestAMAPV0138Refusals`, `TestAMAPV0138ImportedHandlerReadBound`, `TestAMAPV0138RequestPathCLI` |
| AMAP-V0-139 | `internal/requestpath/requestpath.go` | `TestAMAPV0139Deterministic` |

Acceptance (issue 1200): (a) the token-exchange guard lists `OIDC_ISSUER` and
`IDP_TOKEN_ENDPOINT` and marks `axios.post` outbound; (b) a profile with both keys empty reads
`BLOCKED`, names them only and does not schedule; (c) the outbound guard before the validation
guard is flagged and the reordered route is not; (d) a dynamic key, an escaping `process.env`, an
unresolved import, an unfollowed package and a call on an undeclared global read as unknowns. The
fixtures are inline in the tests; 30 source mutations of these rules are each killed by them
(build log).

Unknown reasons: `dynamic-key`, `env-object-escapes` (`ENV_KEY`, `CONFIG_KEY`);
`module-not-found`, `module-outside-repository`, `read-failed`, `export-not-found`,
`member-not-found`, `receiver-not-resolved`, `request-object-call`, `package-not-followed`,
`callee-not-resolved` (`CALL`); `middleware-not-resolved`, `middleware-expression-not-resolved`, `package-not-followed`
(`MIDDLEWARE`); `parent-router-not-read`, `receiver-origin-not-read` (`MOUNT`); `lexer-unsure`
(`SOURCE`); `bound-exceeded` (`BOUND`, `CALL`).

Non-goals: no Koa, Fastify, NestJS, Next.js or decorator routing; no middleware registered in
another file, on a parent router or by `app.use(router)` mounting (each reads a `MOUNT` unknown);
no data flow through parameters, locals or returned clients, so a call on such a value reads
`callee-not-resolved` rather than being followed; no module top-level code that is not referenced
from the path; no secret-store, Vault or cloud parameter lookups; no runtime
tracing; no check of configuration values against a profile. The batch decision is advisory
output for the scheduler and exit code; the command schedules nothing. Simpler baseline: grep
`process.env` in the route file. It misses keys read in imported guards and initializers, cannot
tell guards from handlers or outbound from local calls, and silently drops dynamic reads.

Failure modes: a local variable that shadows a top-level name is scanned as that name
(over-reports keys); an object initializer referenced on the path contributes every key it reads,
used or not (over-reports); a call on a call's result (`a().b()`) is not classified apart from
`a()`, and a step-parameter call such as `res.json()` is taken as the Express API even when a
guard was handed some other object; a package missing from both lists reads
`package-not-followed` rather than being guessed; a lexer-unsure file is still read and carries a
`SOURCE` unknown on each step that reads it. Real handlers that call methods on locals read
`UNKNOWN` often, which keeps the guard-order finding conservative. Each
over- or under-report is visible in the report rather than resolved by guessing.

Rollback: remove `internal/requestpath`, `cmd/corvint/flows_request_path.go` and its test, the
`flowsRequestPathHelp` term in `cmd/corvint/help.go`, the eight `request-path-*` error codes above
and this section's requirements. No stored state, schema, ledger or other command changes; the
report is an explicit stdout document.

## Blocker 404 route cross-check (V1-1967)

Owner input: [issue 1215](https://github.com/beamfall/corvint/issues/1215) (V1-1967), a
follow-up to issue 1174. The requirements AMAP-V0-140..146 are accepted under the owner's
standing auto-accept of 2026-10-11 and delivered experimental. Job: an obligation recorded as
blocked or as a fixture gap because a request got HTTP 404 is checked against the route table
before the classification is trusted, so a test that requests a path no route serves is named
as the test's fault, with the routes it most likely meant. The verb reuses the AMAP-V0-100
route compilation and AMAP-V0-102..105 action resolution and adds no map member, manifest member
or error code. Corvint holds no blocker store: the caller passes the classification or failure
note text and acts on the verdict and exit code.

| Requirement | Implementation | Focused evidence |
| --- | --- | --- |
| AMAP-V0-140 | `internal/appmap/route_blocker.go`, `cmd/corvint/flows_appmap_blocker.go` | `TestAMAPV0141BlockerNoteBounds`, `TestAMAPV0145BlockerCLI` |
| AMAP-V0-141 | `internal/appmap/route_blocker.go` | `TestAMAPV0143BlockerVerdicts`, `TestAMAPV0141BlockerNoteBounds` |
| AMAP-V0-142 | `internal/appmap/route_blocker.go`, `internal/appmap/route_actions.go` | `TestAMAPV0142RouteMatching`, `TestAMAPV0142BlockerUncertain`, `TestAMAPV0143BlockerVerdicts` |
| AMAP-V0-143 | `internal/appmap/route_blocker.go` | `TestAMAPV0143BlockerVerdicts`, `TestAMAPV0142BlockerUncertain` |
| AMAP-V0-144 | `internal/appmap/route_blocker.go` | `TestAMAPV0143BlockerVerdicts`, `TestAMAPV0142BlockerUncertain` |
| AMAP-V0-145 | `internal/appmap/route_blocker.go`, `cmd/corvint/flows_appmap_blocker.go` | `TestAMAPV0145BlockerCLI` |
| AMAP-V0-146 | `internal/appmap/route_blocker.go`, `internal/appmap/route_handlers.go` | `TestAMAPV0146BlockerDeterministic`, `TestAMAPV0107OversizedHandler` |

Acceptance (issue 1215): (a) a note citing `404 POST /api/v2/widgets/42/archive` where only
`POST /api/widgets/:id/archive` exists is flagged `BLOCKER-404-NO-ROUTE` naming that route;
(b) a 404 on a path the table matches with a defined action is accepted unchanged, and one whose
action is missing reads `ROUTE-ACTION-MISSING`; (c) a mounted, rack-endpoint or non-literal path,
and an unmatched path while the table holds an uncompiled route, read `uncertain` with a reason.
The fixtures are inline in `internal/appmap/route_blocker_test.go` and
`cmd/corvint/flows_appmap_blocker_test.go`.

Non-goals: no blocker or obligation store, no rewrite or refusal of a recorded classification
(the verdict and exit code are advisory to the caller); no Express, React or other non-Rails
server router; no routing constraints, engines' own routes, custom inflections or route order
beyond compiled order; no reading of HTTP logs or traces beyond the note text; no fuzzy matching
of methods. Simpler baseline: grep `config/routes.rb` for the last path segment. It cannot expand
resources, scopes or members, ignores the method, and cannot tell an unresolvable table from a
missing route.

Failure modes: a `constraints` block or a later-declared route that Rails would prefer can make
the first compiled match differ from Rails' match (the row names the matched route); a note
citing a path on a line without `404` is not read; a mount written other than `=> "/p"` or
`at: "/p"` covers every path, so every unmatched request reads `uncertain` (conservative, never
a false flag); nearest routes are a ranking hint, not a claim of intent.

Rollback: remove `internal/appmap/route_blocker.go`, `cmd/corvint/flows_appmap_blocker.go`, their
tests, the `opaque` route record in `route_actions.go` and `route_handlers.go`, the `blocker` verb
and help text in `runFlowsAppmap`, and this section's requirements. No stored state, map schema
or ledger needs migration.
