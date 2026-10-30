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
- Status: accepted (decision 0446; V1-0956); experimental. AMAP-V0-001..015 are implemented in `internal/appmap` and `corvint flows appmap` over a committed fixture; no adopter-scale qualification. AMAP-V0-016 (state names and parents read through imported constant tables) is proposed (V1-0981; GitHub #669). AMAP-V0-017..020 (tests in a second, aliased repository) are proposed (V1-0982; GitHub #669) and implemented over synthetic repositories. AMAP-V0-021..023 (state-name tables injected as AngularJS `.constant(...)` registrations within a manifest-declared scope) are accepted (decision 0460; V1-1027; GitHub #685) and implemented over synthetic repositories. AMAP-V0-024..026 (an injected string enum read only when every member is a literal string, one level of `export ... from` behind the registering file's import, and a `di-constant` unknown naming why the one in-scope registration did not resolve) are accepted (decision 0475; V1-1061; GitHub #705) and implemented over synthetic repositories. AMAP-V0-040..048 are accepted (decision 0518; V1-1131); literal Rails/React routes and optional declared-glob coverage remain experimental. AMAP-V0-050..059 are accepted (decision 0518; V1-1136); static UI-router entry candidates only, runtime and PROVEN paths deferred. AMAP-V0-060..065 are accepted (decision 0518; V1-1138); a map-only qualification report with a resolved-percent gate. AMAP-V0-070..072 are accepted (decision 0518; V1-1144); star-export ambiguity paths and a named failed hop. AMAP-V0-075..077 are accepted (decision 0518; V1-1145); `tests.root: "."` for an aliased test repository with declared-directory dirty detection. AMAP-V0-078 is accepted (decision 0519; V1-1169); a same-file `const` React route array only. AMAP-V0-079 is accepted (decision 0520; V1-1220); an imported React route array read through one literal named-import hop. AMAP-V0-080..082 are accepted (decision 0520; V1-1168); entry rows read shortest PROVEN paths from a bound edge ledger, with NOT_OBSERVED for screens no proven path reaches. AMAP-V0-090..099 are accepted under the owner's standing auto-accept (2026-10-11; V1-1581; GitHub #1025); `flows appmap refs`, a name-level check of test-referenced routes, selectors and mocked endpoints the declared source lacks, experimental. AMAP-V0-100..107 (`flows appmap actions`, the ROUTE-ACTION-MISSING check; V1-1890; GitHub #1174) and AMAP-V0-110..116 (`flows appmap branches`, reaching tests by boundary and setup writers per cited branch; V1-1891; GitHub #1175) are accepted under the owner's standing auto-accept and delivered experimental over inline fixtures. AMAP-V0-130..139 are accepted under the owner's standing auto-accept (2026-10-11; V1-1941; GitHub #1200); `flows request-path`, the environment and configuration keys, outbound calls and guard order of one Express-style route, with a fail-closed pre-run profile check, experimental. AMAP-V0-203..205 are accepted under the owner's standing auto-accept (V1-2699; GitHub #1851); injected-name reads past non-reaching imports, typed/shadowed/asserted router forms, and a detail on every `non-literal-name`, experimental over synthetic repositories. AMAP-V0-206..208 are accepted under the owner's standing lane auto-accept (2026-10-11; GitHub #1869); timer callbacks are not loaders and loader/closure refusals name their line, inline array annotations with a trailing comma bind by position, and a router's own table import follows `export *` barrels to a bounded depth, experimental over synthetic repositories. AMAP-V0-209 is experimental under the standing lane auto-accept (V1-2803; GitHub #1899); a static member read through `.constructor` is not a module loader. AMAP-V0-210 is experimental under the standing lane auto-accept (V1-2849; GitHub #1908); map and impact unknowns name a declaration remedy or state their uncertainty. AMAP-V0-211 is experimental under the standing lane auto-accept (V1-2848; GitHub #1913); `flows appmap build --from MAP [--verify]` carries a previous map forward when no build input changed and otherwise rebuilds in full; per-entry re-derivation is delivered for flow edits (AMAP-V0-240) and source edits outside the test files (AMAP-V0-245). AMAP-V0-215..216, 240 and 245 are experimental under the standing lane auto-accept (V1-2875; GitHub #1913); a `--from` build whose only read change is a flow edit re-derives just the flows (AMAP-V0-240); one whose read changes are content edits of `--root` sources other than the test files, `.json` files and build metadata re-derives the routers, test joins and flows and keeps each test file's entry, failing closed to a full build when the context index's source set or bounds may change (AMAP-V0-245); a map names its builder version, which `--from` requires to match, and maps with aliased roots carry forward root by root; the builder identity also binds the binary's VCS revision and a modified-tree binary never carries a map. AMAP-V0-212..214 are experimental under the standing lane auto-accept (V1-2850; GitHub #1907); `flows appmap endpoints` joins Rails and JSON OpenAPI endpoints to $http, axios and fetch call sites and the screens that call them, with every unread call or route unknown. AMAP-V0-217..221 are experimental under the standing lane auto-accept (V1-2876; GitHub #1907): a substitution spans one or more segments, request interceptors and generated clients are unknown, block YAML OpenAPI is read, `flows appmap impact` places a changed controller or declaration through the join (and `corvint-test e2e obligations` with it, CCE-V0-152), and a `$ref` path item is unknown. AMAP-V0-222..223 are experimental under the standing lane auto-accept (V1-2876; GitHub #1907): the join, impact and obligations read `ALIAS:PATH` inputs from the map's aliased roots, a server in an aliased root links to `--root` callers, and a caller in an aliased root links to its endpoints but stays the unknown `aliased-root-caller-not-placed`. AMAP-V0-340..341 are experimental under the standing lane auto-accept (V1-2926; GitHub #1932): asset imports do not block the injected-name reach walk, and a named router function in an inline `.config([...names, F])` annotation binds by position. AMAP-V0-342..343 are experimental under the standing lane auto-accept (V1-2928; GitHub #1934): learned one-hop helper navigation as offered-only page-object remedy candidates, and `flows appmap stats --remedies`. AMAP-V0-350 is experimental under the standing lane auto-accept (GitHub #1940): `flows appmap refs` refuses when a map test file's aliased root is not bound. AMAP-V0-351..358 are experimental under the standing lane auto-accept (GitHub #1938/#1939, V1-2932): the Rails route compiler skips non-route statements and regions, reads regex/hash constraints and an optional `(:name)` scope, follows nested draws with their scope under the application root (the nearest directory holding `config/routes.rb` or `config/application.rb`), impact compares Rails endpoints at base and head, and the client scan lexes JSX and reads AngularJS `$resource` factories and literal-baseURL `axios.create` instances. AMAP-V0-365..368 are experimental under the standing lane auto-accept (V1-2965..V1-2968; GitHub #1951..#1954): class field types are not loaders and loader refusals name the construct, the Rails screen reader skips non-route statements, template and URL-hash navigation helpers, and `--from` re-derives for an added path no input reads. AMAP-V0-140..146 are accepted under the owner's standing auto-accept (2026-10-11; V1-1967; GitHub #1215); `flows appmap blocker`, a route-table cross-check of a blocker note's cited 404 requests (BLOCKER-404-NO-ROUTE), experimental over inline fixtures.
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
  `page_object_screens` bindings, `base_path` (AMAP-V0-191) and per-router `mount`
  (AMAP-V0-192). An unknown member, escaping or absent path, a declared test or
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
  directly (`basis: goto`). An import that is neither resolved (a TypeScript `baseUrl`/`paths`
  alias resolves through the index's resolver, AMAP-V0-195) nor an external package MUST leave that file's join `UNKNOWN`
  (`unresolved-import`); an import that resolves to a source file outside `tests.root` leaves it
  `UNKNOWN` (`import-outside-tests`), since the join does not read beyond the test tree, and every screen projection's `test_join` MUST read `UNKNOWN` while any
  test file's join is `UNKNOWN`, any test file was excluded by the index or could not be read
  (`excluded-by-index`, `unparsed-imports`), or any page object is `page-object-unbound`,
  `page-object-ambiguous`, `unknown-state` or `unresolved-screen` (AMAP-V0-359), since such a file may be the one that reaches a
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
  imports bind it only under an alias. A page-object file whose class is declared
  `export default class` records `class_export: "default"` in the map (omitted for any other
  class; a loaded map with any other value is refused as `appmap-invalid-map`); when the borrowed imports do not bind such a class under its own name, the scaffold
  emits `// UNRESOLVED import { Class }`, reports `scaffold-import` / `default-export`, and calls
  none of its methods (V1-0974). Every name the scaffold binds (borrowed imports, generated
  imports, page-object variables, `page`, `test`, `expect`) is reserved for one file; a reused file
  whose class or variable name is already taken is emitted `// UNRESOLVED` with
  `binding-collision`, and its steps fall back to TODOs. It leaves locator and outcome TODOs
  otherwise. A selector of strength `unknown`, and a step with no selector (whatever a map file
  claims as its reuse), is never offered as reuse; a loaded map that lists reuse beside an
  `unknown` selector gets no call, a note and a TODO, and reports `scaffold-reuse` /
  `unknown-selector-reuse` (V1-0968). A reused method whose anchor is `STALE` at the
  evaluated revision is not called: the step keeps a commented note and a TODO, its reuse item
  reads `freshness: STALE`, and the unknown `scaffold-reuse` / `stale-reuse` is reported. A reused
  method is called only when it is a public instance method of the page object's class, directly
  in the class body, whose header literally declares no parameters (`callable`: no getter, setter,
  static, private or protected member, no module function, no object-literal method before, after
  or inside the class, including one in an object literal assigned to a class field); otherwise the step keeps a note and a TODO and reports `scaffold-reuse` /
  `reuse-not-callable`. Borrowed import statements are kept one per statement, each with exactly
  the names it binds, also when two statements share a line; statement boundaries are read
  lexically, so an `import`, `;` or specifier inside a string or comment never bounds or matches
  a statement, and a statement with a comment inside is treated as unread (V1-0968). A statement the map could not read in full (over 8 lines or 512 bytes) is
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
- `AMAP-V0-200`: (proposed, standing lane auto-accept 2026-10-11; V1-2696, V1-2701, GitHub #1802,
  #1808) `build` MAY take repeatable `--repo-revision ALIAS=REV` for an alias bound by `--repo`.
  That root is then read at REV from committed objects instead of `HEAD`: its pin in `roots`
  records the resolved commit and `"stated": true`, the AMAP-V0-019 worktree-status check is not
  read for it (no checkout state is an input), and a later commit or uncommitted change in that
  checkout leaves the map bytes unchanged. A REV that is not a commit of the root, or an alias the
  manifest reads nothing from, refuses `appmap-root-unavailable`; `--repo-revision` for an alias
  without `--repo` is a usage error. Without `--repo-revision` builds are unchanged. The same three
  flags are bound for `corvint-test e2e` (CCE-V0-134). Non-goal: an explicit revision for `--root`
  beyond the existing `--revision`. Rollback: remove the flag and the `stated` member; no stored
  state migrates. Acceptance: `TestAMAPV0200StatedAliasRevision`. A `corvint-workspace/0` declaration
  (`--workspace FILE` or discovered `.corvint/workspace.json`) binds the same roots and revisions;
  see `docs/specs/workspace-declaration-v0.md` (WSD-V0).
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
- `AMAP-V0-190`: A JSX `<Routes>` tree MUST be read even when the file elsewhere holds hooks,
  conditionals, logical operators or loops, provided the tree is statically placed: at every
  enclosing level the statement holding it carries no `?`, `&&`, `||`, `if`/`else`/`switch`/`case`
  or loop keyword, it is not returned after a condition, and every enclosing bracket is a
  `return (`, `= (`, arrow-body or JSX-child group or a function block body. A call argument,
  array element, attribute or object value, class or control-flow block, or any unrecognised
  enclosure keeps the file-wide `dynamic-route-context` rule. So does any enclosing block that
  could exit before the tree: a `throw`, a `return` inside a nested control-flow block, an earlier
  `return` at the block's own level, with or without a semicolon; exits inside nested function
  bodies do not count. A JSX expression child `{...}`
  inside the tree MUST record `non-literal-react-children` at its own line and is never harvested,
  while its literal siblings are still read; a comment-only `{}` child is ignored.
  `createBrowserRouter` and `createRoutesFromElements` keep the file-wide rule. Status: accepted
  under the owner's standing lane auto-accept (V1-1712).
- `AMAP-V0-191`: Optional manifest `base_path` MUST be the application's literal router basename:
  one or more `/`-led segments of unreserved characters (`^(/[A-Za-z0-9._~-]+)+$`), with no
  trailing slash, empty segment or parameter; anything else refuses with
  `appmap-invalid-manifest`. A test URL MUST match a screen only after its path is stripped of
  the base path at a segment boundary; a path outside it reads `outside-base-path`, never a match
  against the bare routes. The map records `base_path`, and every URL the map projects for a
  screen carries it before the hash prefix. A static router-relative link target (a `to` or
  `routerLink` attribute, a router or history call, a bare `navigate()`) is written without the
  base path and MUST resolve against the bare routes; an `href`, form action and
  `location.assign` target is a browser URL and carries it. Status: accepted under the owner's standing lane
  auto-accept (V1-2642).
- `AMAP-V0-192`: An optional `mount` on a `react-router/0` router (`{"router": PATH, "path":
  "/area/*"}`) MUST place that router's routes under the named router's terminal-splat route, as
  react-router matches descendant `<Routes>`: each mounted path is relative to the mount path
  without its `/*`, a top-level mounted state takes the splat route as its parent, the splat route
  becomes abstract, and each moved state carries the mounting router's anchor so a change to
  either file leaves it not FRESH. The mount MUST name another declared `react-router/0` router
  and a literal absolute path ending in `/*`, and mounts MUST form no cycle; the named router
  MUST declare that path exactly once other than as a redirect, as that router declares it; a
  router mounted into a mounted router is placed under its host route's placed path. Any other mount refuses with
  `appmap-invalid-manifest`. Status: accepted under the owner's standing lane auto-accept
  (V1-2642).
- `AMAP-V0-193`: An enclosing function body MAY hold guard statements before the `return` of a
  statically placed `<Routes>` tree (AMAP-V0-190): an `if (COND) STATEMENT;` or `if (COND) {...}`
  with no `else`, at the block's own level, whose body holds a `return` or `throw` and no
  `<Routes>` or `<Route>` element. Such a tree MUST be read, and each of its screens MUST carry
  the entry constraint `render-guarded`, since the guard decides whether the tree renders, never
  which routes it declares. An `else`, a guard holding a route element, or any other exit keeps
  `dynamic-route-context`. Status: accepted under the owner's standing lane auto-accept
  (V1-2642).
- `AMAP-V0-194`: A childless `<Route>` whose element is only a `<Navigate ... />` element MUST be
  read as a redirect: it is abstract, declares no screen and matches no URL. A route whose element
  holds a `Navigate` inside another element is not a redirect. Status: accepted under the owner's
  standing lane auto-accept (V1-2642).
- `AMAP-V0-195`: The test import index MUST resolve a bare specifier through the tsconfig/jsconfig
  governing the importing test file (`compilerOptions.baseUrl`, `paths`, `extends`) by reusing the
  contextindex web import resolver (V1-0958), not a second resolver: a specifier the resolver
  reads as one repository file is resolved to it, a package reading keeps the existing package
  rule, and an unresolved reading of a non-built-in bare specifier stays `unresolved-import` even
  when some package.json declares its package (an unindexed `extends`, an ambiguous pattern or a
  manifest outside the importer's chain leaves it unknown, never external). An `unresolved-import`
  unknown MUST carry `detail` `specifier <specifier>`, suffixed ` (config resolution unknown)` for
  that declared-package case. A computed `import(...)`/`require(...)` argument (not one exact
  literal) MUST read `unresolved-import` with `detail` `computed specifier` and leave the join
  `UNKNOWN`. Failure mode: specs importing a page object through a `paths` alias
  (`import X from 'lib/pages/sheet'`) read `unresolved-import`, so their joins and ANL-V0-049's
  importing specs were lost. Non-goal: reading a package's own sources. Rollback: remove
  `bareConfigImport` and the detail; aliased imports read `unresolved-import` again.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; V1-2700, issue #1806).
- `AMAP-V0-196`: When a binding check (`registering-file-bindings`, `import-chain-bindings`, or the
  binding part of `registering-file-examined` / `import-chain-examined`) refuses a `di-constant`,
  the AMAP-V0-072 `detail` MUST append `:` and the binding that failed as `path:line:name`: the
  import's local name at its statement's first line, or the module of a namespace import, dynamic
  import, `require` or unread import item (`*` for a module that is not an exact literal). Imports
  are checked in name order, so the binding named is stable. An `export * from` barrel one level
  behind the import (AMAP-V0-025, AMAP-V0-070) stays read when the target declares the table once
  and no binding on the chain may hold it. Failure mode: a refusal named only
  `import-chain-bindings`, so an adopter could not tell which import blocked the read. Non-goal:
  narrowing which bindings may hold the table. Rollback: remove `bind` and `spaceLines` from
  `internal/appmap/stateconst.go`; the detail is the hop alone again.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; V1-2699, issue #1805).
- `AMAP-V0-201`: The #1805 / #1851 shape MUST resolve: a registering file importing the table
  through a directory specifier (`./routes` to `routes/index.ts`) whose barrel holds one
  `export * from` statement, a `di_constants` entry naming that directory, and the manifest in an
  aliased root (AMAP-V0-017), when the table is declared exactly once and never rebound. Every
  other `di-constant` refusal on that chain MUST carry `detail` naming the hop that refused, in
  addition to the AMAP-V0-072 / AMAP-V0-196 hops: `import-<state>:path:line:module` (state
  `unresolved` or `package`) or `import-unreadable:path:line:module` for an import or re-export
  that does not reach a read repository file; `barrel-unlisted:path`, `barrel-local-export:path:name`,
  `barrel-named-reexports:path:line,line`, `barrel-no-reexport:path:name`,
  `barrel-namespace-reexport:path:line`, `barrel-depth:path:line:target` (the re-export at line
  reaches another barrel), `barrel-candidate-unread:path:name` and `barrel-no-candidate:path:name`
  for the barrel; and `binding:path:line:name`, `unbound:path:name`, `not-exported:path:name` or
  `not-a-table:path:name` (exported, but not as a literal table) for the binding. Reasons are
  unchanged; `member-not-found` and `non-literal-member` name no hop. Failure mode: an adopter saw
  `non-literal-name` on every state with no detail and could not tell which hop refused. Non-goal:
  following a second barrel level or a rebound table. Rollback: remove the hop assignments in
  `lookup`, `target` and `reexported`; `detail` is again only the AMAP-V0-072 / AMAP-V0-071 hops.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; V1-2699, issues
  #1805/#1851).
- `AMAP-V0-202`: A `state` unknown `non-literal-name`, and a `screen` unknown `non-literal-value`
  whose state's own `parent` was an unresolved injected read, MUST carry `detail` naming why the
  injected read refused: `di-constant:X:reason[:hop]` when an AMAP-V0-026 unknown is carried, or
  else `router-file-unread:path`, `router-template-use:path:X`, `router-annotation:path:line`,
  `router-binding:path:line:X` (a use of the parameter other than a member read),
  `not-injected:path:X`, `outside-injectable:path:line:X`, `scope-unreadable:path[:line]` (the
  first source that poisoned the scope), `no-registration:X` or
  `ambiguous-registration:X:path:line,...`; with a scope declared, a member read `X.Y` whose `X`
  no parameter binds reads `not-injected:path:X`. A build without a `di_constants` scope, and a
  parent that is neither a string nor a member read, carries no detail, so those maps keep
  their bytes; a map with a scope and an unresolved member read gains the member and a new digest,
  the one deliberate byte change of this requirement (a state name of any other form names its
  form under AMAP-V0-205). Each
  `flows appmap stats` example anchor (AMAP-V0-062) MUST carry its unknown's `detail` when it has
  one, a final ordering key. Failure mode: these refusals left no `di-constant` unknown and a bare
  `non-literal-name`, and `stats --full` dropped every detail. Non-goal: detail on
  descendants (`unresolved-parent`). Rollback: drop the detail return from `constLookup` and
  `injected` and the stats example member; the unknowns lose the member.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; V1-2699, issues
  #1805/#1851).
- `AMAP-V0-203`: Another import or module reference of a file on the injection chain MUST block
  the table read (`import-chain-bindings`, `registering-file-bindings`) only when the module it
  names may hold the table: the declaring file itself, an unresolved, unread or unreadable module,
  a non-literal module name, a package the repository declares locally (AMAP-V0-025), or a
  repository file that re-exports or relays an import AND whose module graph (imports,
  side-effect imports, re-exports, namespace, dynamic and `require` references, transitively over repository files,
  at most 4096 files) may reach the declaring file. An import binding that is never read is
  skipped. The binding named by the AMAP-V0-196 hop MUST end with why it may hold the table:
  `table-name`, `module-not-literal`, `module-unresolved`, `local-package`,
  `module-unreadable:path`, `module-unread:path`, `declaring-module`, or a reach reason
  `reaches-table:path:module`, `reach-unresolved:path:module`, `reach-local-package:path:module`,
  `reach-module-not-literal:path`, `reach-unreadable:path`, `reach-unread:path` or
  `reach-limit:path`. Failure mode (#1851): a registering module that also registers a service
  imported through a `tsconfig` `paths` alias to another package's `export *` barrel refused every
  injected state as `import-chain-bindings` with no reason, although that barrel never loads the
  table. Non-goal: proving which export of a reaching module is the table; reaching is enough to
  refuse. Rollback: restore the boolean `mayHold` (no `reaches`) and the binding without its why
  suffix in `internal/appmap/stateconst.go`.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; V1-2699, issue #1851).
- `AMAP-V0-204`: The router-side read of an injected parameter MUST resolve, all other AMAP-V0-022
  checks holding, when: the parameters carry TypeScript annotations made of names, literals,
  `typeof`/`keyof`, member access, unions, intersections, generic arguments and array suffixes;
  a nested function declares its own parameter of the same name (a shadow: its body reads another
  binding, and an inline `['name', ..., fn]` annotation whose names match the parameters counts
  as an injection of that name only when it names it exactly once); the name appears in a
  template substitution that is a plain member read while no substitution in the file is opaque
  and no template with a substitution is tagged; or the member read is followed by a
  `!` non-null or `as`/`satisfies` assertion. An assignment through an asserted member
  (`(X.Y as T) = v`) MUST still refuse as a write. A state read inside a shadowing function MUST
  refuse with `shadowed:path:line:X` (line of the shadowing function). Failure mode (#1851):
  typed parameters, a controller re-injecting the constant and template uses refused as
  `router-binding` / `router-template-use` with the value statically known. Non-goal: data flow
  through locals, spread or computed members. Rollback: revert `paramFunc`, `skipParamType`,
  `inlineAnnotation`, the shadow branch of `injection`/`injected` and `assertedEnd` /
  `skipAssertedType`.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; V1-2699, issue #1851).
- `AMAP-V0-205`: Every `state` unknown `non-literal-name` MUST carry `detail`. A name the reader
  does not follow names its form: `name-missing` (no name argument or `name` property) or
  `name-expression:<form>` with form `template`, `call`, `optional-chain`, `member-chain`,
  `member-unread`, `member-<token>` (a member read followed by an operator such as `+`),
  `non-null`, `identifier` or `expression`. A member read of a constant the router file declares
  or imports itself that does not resolve MUST carry `const:X:<reason>[:hop]`. The name is never
  guessed: each stays UNKNOWN. This deliberately changes the bytes of a map with such an unknown
  (it gains the `detail` member). Failure mode (#1851): 15 of 153 unknowns carried no detail.
  Non-goal: evaluating concatenations or calls. Rollback: drop `nameForm` and the router-own
  `const:` detail in `internal/appmap/router.go` and `stateconst.go`.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; V1-2699, issue #1851).
- `AMAP-V0-206`: A `setTimeout` or `setInterval` call whose first argument is a function literal
  (`function`, `async function`, `x =>` or `(...) =>` without a return type) MUST NOT count as a
  module loader: the timer calls the function and evaluates no string. A `function` expression
  counts only when its whole body is followed by `,` or `)`; one invoked in place hands the timer
  its result and stays a loader. The bare name, a call with
  any other first argument, a string key and a template substitution naming it stay loaders. The
  `reach-module-not-literal` reason of AMAP-V0-203 MUST name the reference's line
  (`reach-module-not-literal:path:line`), and a refusal of the examined closure of AMAP-V0-023 /
  AMAP-V0-025 (`registering-file-examined`, `import-chain-examined`, `registration-examined`) MUST
  append the closure hop that failed: `closure-module-not-literal:path:line`,
  `closure-<state>:path:module` (an unresolved or local-package module),
  `closure-unknown-extension:path`, `closure-unreadable:path`, `closure-limit:N` or
  `closure-not-quiet:path` (with `:line:construct` since AMAP-V0-373). Every module the registering file, the chain or the router loads stays
  examined: an unrelated import whose graph holds a loader the reader cannot name still refuses,
  now naming the file and line. Failure mode (#1869): 56 states refused as
  `import-chain-bindings:...:<ServiceB>:reach-module-not-literal:<file>` with no line, past a
  service whose graph only scheduled callbacks. Non-goal: pruning the examined closure or the
  reach walk to the chain that reaches the table (a module loaded only for its effects can still
  inject or write the table; AMAP-V0-369 narrows only the binding check and the limit), and
  reading `Function` in a type position. Rollback: remove the
  timer case of `moduleLoader` and `timerCallback`, and the line and closure hops in `walk`,
  `closure` and `examined` in `internal/appmap`.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; issue #1869).
- `AMAP-V0-207`: An inline array annotation (`.config([...names, fn])` or another AMAP-V0-022
  inline form) MUST bind each string name to the parameter at its position, and a trailing comma
  after the function (`fn, ]`, as formatters write it) MUST NOT stop the read. When the names
  annotate the function but their count differs from the parameters, or the name at the
  parameter's position differs, a read of that parameter MUST stay unknown with `detail`
  `annotation-arity:path:line:X` or `annotation-position:path:line:X` (line is the function's first
  line), never a guess. Failure mode (#1869): 53 states in prettier-formatted
  `config(['$stateProvider', ..., ($stateProvider, ...) => ...,\n])` routers read
  `not-injected`. Non-goal: binding a parameter whose name differs from the string at its
  position. Rollback: restore `next(toks, after, "]")` in `injectable` and `inlineAnnotation` and
  drop `arrayEnd` and `diFunc.mismatch` in `internal/appmap/stateinject.go`.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; issue #1869).
- `AMAP-V0-208`: A member read of a table the router file imports itself (AMAP-V0-016) whose
  imported module does not declare the name but re-exports MUST follow its `export ... from`
  statements, named re-exports shadowing stars as in AMAP-V0-025, through at most 8 barrel levels
  (`maxBarrelDepth`). A (barrel, export name) pair reached again on the same path is a cycle and
  exports nothing through it; the same barrel reached under another export name is followed. One
  declaration reached by two paths is one binding. Duplicate edges to one (module, name) are
  followed once, each (barrel, name, depth) answer that cut no cycle is reused, and a walk that
  reads more than 256 barrels (`maxBarrelVisits`) reads `barrel-depth-exceeded` with
  `barrel-visit-limit:path:256`, naming the barrel the walk started from. Two distinct declarations read
  `ambiguous-barrel` with `ambiguous-reexport:paths` (AMAP-V0-071); a chain deeper than the bound
  reads `barrel-depth-exceeded` with `barrel-depth:path:line:target`; every other AMAP-V0-201
  barrel hop that may export the name refuses with its hop; a barrel that provably exports no such
  name keeps `identifier-not-found:not-exported:path:X`. The resolved state is anchored at the
  declaration, each re-export statement from the deepest out, and the router's import. Failure
  mode (#1869): 12 states read `const:RoutesConstants:identifier-not-found:not-exported:
  <dir>/routes/index.ts` through `export * from './routes.constants'`. Non-goal: deeper barrels on
  the injected (AMAP-V0-025) chain, which still follows one level. Rollback: drop the router-own
  re-export case in `lookup` and the depth, cycle, memo and visit handling of the barrel walk
  (`starFollow`, `follow`, `followBarrel`), restoring the
  one-level `reexported` in `internal/appmap/stateconst.go`.
  Status: accepted under the owner's standing lane auto-accept (2026-10-11; issue #1869).
- `AMAP-V0-209`: A `.constructor` property followed by a plain dotted member `.Y`, where `Y` is
  not `constructor`, `call`, `apply` or `bind` (`x.constructor.$delete(...)`, a static member of
  the object's class), MUST NOT count as a module loader of AMAP-V0-203: it names no module and
  evaluates no string. Every other `.constructor` use stays a loader and keeps the
  `reach-module-not-literal:path:line` refusal of AMAP-V0-206: called (`x.constructor(...)`),
  bare (`const C = x.constructor`), reaching the constructor itself (`.constructor.constructor`,
  `.call`, `.apply`, `.bind`), read through `?.` or a computed member (`x.constructor[k]`), and
  each later `.constructor` hop checked on its own (`x.constructor.prototype.constructor(...)`).
  The rule a reached module taints the constant under is unchanged: a module the reader cannot
  name still refuses. Failure mode (#1899): an injected literal constant read
  `reach-module-not-literal` because a reached service called `x.constructor.$delete(...)`.
  Non-goal: proving a reached module cannot write the table (the reach walk stays
  conservative), naming the expression kind in the detail (the wire string is unchanged), and
  relaxing `constructor` inside template substitutions. Rollback: restore
  `return property(toks, i)` for `constructor` in `moduleLoader` and drop `staticMember` and
  `constructorReach` in `internal/appmap/stateconst.go`.
  Status: experimental under the standing lane auto-accept (2026-10-11; V1-2803, issue #1899).
- `AMAP-V0-210`: Each map unknown and each `application-map-impact/0` unknown MUST be able to
  name its remedy, `{file, key, candidates, basis, uncertainty}` (`appmap.UnknownRemedy`; the
  optional `remedy` member of an impact unknown): the declaration file and member whose edit would
  resolve it, the values the map infers for that member, and what cannot be inferred. A
  `page-object-ambiguous`, `page-object-unresolved-target` or `page-object-unbound` page object
  names `page_object_screens` with one `{"path","state"}` candidate per state its resolved
  `page.goto` lines reach (sorted, offered and never chosen) and the lines as basis; a target that
  resolves nowhere, more than one reached state, a page object that navigates nowhere the map
  resolves, a declared entry whose file is missing, and an `unknown-state` entry (a state the map
  does not hold; correct the entry's state) each state their uncertainty; an `unresolved-screen`
  entry names no file or key and states that screen's own blocking unknown (AMAP-V0-359). `router-not-declared` names `routers` with the file as candidate and states that
  its dialect is not inferred. An impact `unmapped-path`, `unresolved-reference` or
  `reference-graph-bound` takes the keyed remedy of a map unknown at the same path when there is
  one. Every other unknown (`manifest-changed`, `non-ui-path`, `test-support-unreached`,
  `test-import-graph-incomplete`, any other map reason) names no file, key or candidate and states
  why no declaration change is inferred. Failure mode (#1908): an unknown said what was missing
  but not which declaration would bound it, and an inferred value was either absent or would have
  been guessed. Non-goals: choosing between candidates, writing the manifest, inferring a router
  dialect or a page object's state from anything but its resolved navigation, and a remedy for
  the reference-hop bounds. Compatibility: the impact `remedy` member is optional and additive;
  `LoadImpactReport` decodes reports without it, and the digest of a report with unknowns changes.
  A remedy enlarges each unknown, so a very small `--budget` that held a report's header may now
  refuse it with `impact report header exceeds --budget`.
  Rollback: delete `internal/appmap/remedy.go` and the `Remedy` field of `ImpactUnknown`.
  Status: experimental under the standing lane auto-accept (2026-10-11; V1-2849; GitHub #1908).
- `AMAP-V0-211`: `flows appmap build --from MAP` MUST write the map a full build at `--revision`
  gives, byte for byte. `appmap.BuildFrom` carries MAP to the revision (only `revision` and the
  digest change) when MAP is a single-root map of the same `--manifest`, its revision resolves in
  the root, and every path `git diff-tree -r --no-renames` reports between the two revisions is a
  status-`M` content edit of a regular file whose mode and seven-column `ls-tree -l` size width
  are unchanged and that no build input reads: not a context-index source
  (`contextindex.ImpactPathAdmitted`, `Makefile`, `go.mod`), not a `.git*` file or under
  `.corvint`, not under the manifest, tests root, specs, page objects, workflows, scenarios,
  `di_constants`, flows or router paths, and not a `router_globs` match. Anything else rebuilds in
  full and names why (`aliased-roots`, `manifest-path-differs`, `from-revision-unavailable`,
  `changed-input`) in `appmap.Incremental`: the context index admits by suffix, probes every
  tracked path to resolve imports and bounds the whole tree listing, so an added, removed or
  retyped path of any kind is a changed input. `--verify` (only with `--from`) also builds in full
  when MAP was carried and refuses `appmap-incremental-mismatch` unless the digests agree. Failure
  mode (#1913): every build re-derived the whole map even when no input changed. Non-goals:
  re-deriving only the entries a changed input reaches beyond flow edits (AMAP-V0-240) and
  source edits outside the test files (AMAP-V0-245); the map records no per-entry dependency set
  and the context index is whole-repository. Reuse across aliased roots is AMAP-V0-216 and
  builder-version detection AMAP-V0-215. Measured on the committed fixture with an asset edit (WSL2, go1.27.3, mean of 20):
  full build 45.7 ms, `--from` carried 16.6 ms, `--from` that rebuilt 58.6 ms (the reuse check
  adds about 13 ms when it fails). Compatibility: additive flags; the map bytes are unchanged.
  Rollback: delete `internal/appmap/incremental.go` and the `--from`/`--verify` flags.
  Status: experimental under the standing lane auto-accept (2026-10-11; V1-2848; GitHub #1913).
- `AMAP-V0-212`: `flows appmap endpoints --map FILE --source DIR... [--rails-routes FILE]...
  [--openapi FILE]...` MUST read server endpoints at the map revision from `--root` only: each
  `--rails-routes` file (and its nested draws, AMAP-V0-353) through the rails-routes/0 route compiler, with
  verb, path, `handler` (`controller#action`) and the conventional controller file
  (`app/controllers/<controller>_controller.rb` under the router's application root); and each
  `--openapi` JSON OpenAPI 3 or Swagger 2 document's `paths` and methods, with `operationId`,
  prefixed by the path of its `servers[].url` (or `basePath`). Parameters (`:id`, `{id}`)
  normalize to `{}` and a terminal Rails splat to `{*}`; an endpoint's `id` is `VERB /path` in that
  form, with `#n` on a duplicate. A route the compiler abstains on, skips or cannot bound, a
  route pattern it cannot normalize, and a document that is neither JSON nor the AMAP-V0-219 YAML subset (`openapi-yaml-unsupported`), not
  OpenAPI (`openapi-unreadable`), has servers with a templated (`openapi-server-url-not-literal`)
  or differing (`openapi-servers-differ`) path, a path that is not a template
  (`openapi-path-not-template`), or a `$ref` path item (`openapi-path-ref-not-read`, AMAP-V0-221) MUST be an unknown with its reason; a document with such a reason
  contributes no endpoint. Zero or more than 16 declarations, zero or more than 16 sources, an
  aliased or unsafe path, or a repeated path refuse `appmap-invalid-query`.
  Status: experimental under the standing lane auto-accept (2026-10-11; V1-2850; GitHub #1907).
- `AMAP-V0-213`: Client calls MUST be read from the `.js`, `.jsx`, `.ts`, `.tsx`, `.mjs` and `.cjs`
  files (not `.d.ts`) under the `--source` directories through the shared JavaScript lexer:
  `$http.get|post|put|patch|delete|head|jsonp(url, ...)`, `$http({url, method})`,
  `axios.get|post|put|patch|delete|head|options|postForm|putForm|patchForm(url, ...)`,
  `axios(url, config)`, `axios(config)`, `axios.request(config)`, and `fetch(url, init)` called
  bare or on `window`, `globalThis` or `self`. The URL is read from string and template literals
  and plain names or member chains joined by `+`; a substitution stands for one or more path
  segments (AMAP-V0-217), the query and fragment are dropped, and a literal `method` sets the verb (a
  non-literal one, or a non-literal fetch/axios init, reads `*`, any verb). A call whose URL is
  not read (`non-literal-url`), starts with a substitution (`url-base-not-literal`), is absolute
  or protocol-relative (`absolute-url`) or relative (`relative-url`), whose config sets `baseURL`
  (`config-base-url`) or is not a plain object (`non-literal-config`); an `axios.create` instance
  AMAP-V0-358 does not read (`client-instance-not-modelled`), another client method (`client-method-not-modelled`), an
  `axios.defaults` access (`client-defaults-not-read`), any other use of a client name that is not
  an import or declaration binding, a parameter or an object key (`client-reference-not-a-call`),
  a source over 1 MiB (`source-over-bound`) and a file the lexer cannot bound
  (`source-not-lexed`) MUST be unknowns with their reason, never dropped.
  Status: experimental under the standing lane auto-accept (2026-10-11; V1-2850; GitHub #1907).
- `AMAP-V0-214`: A call MUST list every endpoint it may reach: the verb matches (`*` and a Rails
  route without a verb match any; a Rails `GET` also takes `HEAD`) and each segment matches, a
  literal equal to a literal or filling a `{}` (not containing `.`), a substituted segment matching
  a `{}` or any literal whose text around the substitution fits, and `{*}` taking one or more
  remaining segments; a Rails endpoint also takes a trailing `.format` on the last literal
  segment. A call reaching none is the unknown `no-declared-endpoint`. Its screens are the screens
  anchored at its file, else the screens the reference hop impact uses (ANL-V0-069..072) reaches
  from it; a file the hop cannot place is an unknown with the hop's reason and detail. An
  endpoint's screens are the union over its calls. The report is `application-map-endpoints/0`
  (head: schema, app, map revision and digest, budget, limits, the declarations and sources read,
  source file, endpoint, call, unmatched-call and unknown counts, and `complete`, true only with no
  unknown), with `unknown`, `endpoints` and `calls` sections within `--budget` (default 6144) or
  `--full`. Failure mode (#1907): a changed controller action or API operation could not reach the
  screens whose code calls it. Non-goals: request interceptors and `axios.defaults` values,
  client instances, `$resource`, Angular `HttpClient`, reading generated API clients (they are
  unknown, AMAP-V0-218). A substitution spanning several segments is AMAP-V0-217, YAML OpenAPI
  AMAP-V0-219, impact through the join AMAP-V0-220, and aliased roots AMAP-V0-222..223.
  Compatibility: a new verb and schema; no existing wire changes. Rollback: delete
  `internal/appmap/endpoints.go`, `cmd/corvint/flows_appmap_endpoints.go` and the verb's dispatch.
  Status: experimental under the standing lane auto-accept (2026-10-11; V1-2850; GitHub #1907).
- `AMAP-V0-215`: A map MUST carry `builder`, the compiler's `appmap.BuilderVersion`
  (`appmap-builder/1`), which is bumped in the same change as any change to what a build derives
  from the same inputs. The map digest is computed with `builder` and `digest` both empty, so the
  member is provenance, not content, and a map written before it keeps its digest. `BuildFrom`
  rebuilds in full with the reason `builder-version-differs` when MAP's `builder` is absent or
  differs, and `flows appmap build --from` writes `flows appmap build: --from MAP rebuilt in full:
  <reason>` to stderr whenever it rebuilt, for any reason; a carried build is silent. The help
  documents `--from` (there is no `--previous` flag), every reason, and that a changed input other
  than an AMAP-V0-240 flow edit or an AMAP-V0-245 source edit still re-derives the whole map. Failure mode (V1-2875): a MAP written by an older compiler was carried
  forward as current. Non-goals: comparing builders by anything but equality. Compatibility: the
  map gains an optional member; an older binary, which decodes maps strictly, refuses a new map
  until upgraded, and every pre-existing map rebuilds once under `--from`. Rollback: delete the
  `Builder` field, `BuilderVersion` and the reason. The written `builder` is `BuilderVersion`
  followed by `+` and the first 12 hex digits of the binary's recorded `vcs.revision`, and `-dirty`
  when the binary was built from a modified tree (Go build info); a binary with no VCS stamp (a
  test binary) writes `BuilderVersion` alone. `BuildFrom` carries MAP only when its `builder`
  equals the running identity and that identity is not `-dirty`, so a Corvint upgrade that changes
  route parsing rebuilds every map once even if `BuilderVersion` was not bumped (Codex batch review
  item 7). Status: experimental under the standing lane
  auto-accept (V1-2875; GitHub #1913).
- `AMAP-V0-216`: `BuildFrom` MUST also carry a map with aliased roots (AMAP-V0-017..020, 200)
  when `--manifest-repo` names MAP's manifest root, every `--repo-revision` alias (outside a
  workspace) is one MAP pins, and each pinned alias opens as a full build would open it, at a HEAD
  pin for a HEAD pin and a stated revision for a stated one. The AMAP-V0-211 rule applies to the
  change in `--root` and, separately, to each aliased root from its pinned revision to the
  revision read now, each judged against every input the manifest names (a superset of what the
  build reads from that root); a HEAD-read root must also be clean under the paths it supplies
  (AMAP-V0-019) or the build rebuilds with `aliased-root-dirty`, which the full build then refuses
  as `appmap-root-dirty`. A carried map moves each pin's `revision` to the root's new commit and
  changes nothing else. A root that does not open rebuilds with `aliased-roots`, and the full
  build refuses as it would. Failure mode (V1-2875): every build with an aliased root re-derived
  the whole map. Non-goals: re-deriving only the entries a changed input reaches beyond
  AMAP-V0-240 flow edits and AMAP-V0-245 source edits in `--root` (the map records no per-entry dependency set and each root's context index is whole-repository). Compatibility:
  no wire change. Rollback: restore the `aliased-roots` refusal for any pinned root. Status:
  experimental under the standing lane auto-accept (V1-2875; GitHub #1913).
- `AMAP-V0-217`: A substitution segment of a call URL MAY span one or more template segments: the
  literal text before the substitution MUST fit the start of the first covered segment (or that
  segment is `{}`), the text after it the end of the last covered segment (or `{}`), and a `{*}`
  takes the rest; several substitutions in a path are matched by a bounded memoized search.
  Failure mode (V1-2876): `'/api/' + path` reached no endpoint and read `no-declared-endpoint`
  where the call may reach any endpoint under `/api`. Compatibility: a call may list more
  endpoints than before; never fewer. Rollback: restore one-segment matching in `segmentsReach`.
  Status: experimental under the standing lane auto-accept (V1-2876; GitHub #1907).
- `AMAP-V0-218`: A source whose first 2048 bytes carry a generator marker (`openapi-generator`,
  `OpenAPI Generator`, `swagger-codegen`, `Swagger Codegen`, `NSwag`, `Generated by orval`,
  `@generated`) and that names `fetch`, `axios`, `$http`, `XMLHttpRequest` or `HttpClient` MUST be
  the unknown `generated-client-not-modelled` with the marker as detail, and its calls are not
  read; an `axios.interceptors` or `$httpProvider.interceptors` reference MUST be the unknown
  `request-interceptor-not-modelled` (an interceptor may rewrite any URL). Failure mode (V1-2876):
  a generated client's URLs or an interceptor's rewrite were read as certain. Non-goals: reading
  generated clients or interceptor bodies. Rollback: drop `generatedClientMarker` and the
  interceptor case. Status: experimental under the standing lane auto-accept (V1-2876; GitHub
  #1907).
- `AMAP-V0-219`: An `--openapi` document that is not JSON MUST be read as YAML when it uses only
  block mappings, block sequences (compact items included), comments and plain or quoted one-line
  scalars, with one optional leading `---`; every scalar reads as a string. Any other form (flow
  collections, anchors, aliases, tags, block scalars, merge keys, multi-line scalars, several
  documents or a directive, tab indentation, duplicate keys, no top-level mapping, an unterminated
  quote) MUST refuse the document as `openapi-yaml-unsupported` with `line N: why` as detail, and
  the document contributes no endpoint. Failure mode (V1-2876): every YAML OpenAPI document was an
  unknown. Non-goals: a general YAML parser. Rollback: delete `internal/appmap/openapiyaml.go`.
  Status: experimental under the standing lane auto-accept (V1-2876; GitHub #1907).
- `AMAP-V0-220`: `flows appmap impact` with `--endpoint-source DIR...` and `--rails-routes` or
  `--openapi` (`appmap.Options.Endpoints`) MUST join endpoints at the map revision and place a
  changed path that is an endpoint's controller file or declaration file on the screens of that
  endpoint's calls, basis `endpoint:<ID>`. Each such path also keeps, as impact unknowns,
  `endpoint-not-called` (an endpoint it serves that no read call reaches; detail the endpoint ID),
  `endpoint-declaration-changed` (a changed OpenAPI declaration file, detail the endpoint count at
  the map revision; a changed Rails declaration is compared at base and head, AMAP-V0-355) and `endpoint-callers-incomplete` (the join had any unknown other than
  `no-declared-endpoint`; detail the count and the first), each with a stated-uncertainty remedy,
  so the verdict stays `full-suite-required`. Without the inputs impact is unchanged. Failure mode
  (#1907): a changed controller was `non-ui-path` though the join knew its calling screens.
  Rollback: drop `Options.Endpoints` and `impactEndpoints`. Status: experimental under the
  standing lane auto-accept (V1-2876; GitHub #1907).
- `AMAP-V0-221`: An OpenAPI path item that holds `$ref` MUST refuse the document as
  `openapi-path-ref-not-read` with the path key as detail; the document contributes no endpoint,
  so the join is never `complete` while omitting the referenced operations (invariant 2).
  Failure mode (Codex batch review item 8): a `$ref` path item contributed neither an endpoint nor
  an unknown. Non-goals: resolving local or external path-item references. Rollback: drop the
  check in `openAPIEndpoints`. Status: experimental under the standing lane auto-accept (V1-2876;
  GitHub #1907).
- `AMAP-V0-222`: Each `flows appmap endpoints` input (`--source`, `--rails-routes`, `--openapi`;
  `appmap.EndpointsOptions`) MAY be `ALIAS:PATH`, read at the map's pin of the aliased root ALIAS
  bound by `--repo ALIAS=ABSOLUTE_ROOT` (`appmap.Options.Repos`); a bare path is read from
  `--root`. Endpoints, calls and unknowns read from an aliased root MUST carry `repo` (the alias),
  and an endpoint's `calls` entry from an aliased root MUST be `ALIAS:PATH:LINE`. A server in an
  aliased root MUST join to `--root` calls and their screens as declared. A call in an aliased
  root MUST link to the endpoints it reaches but MUST NOT be placed on a screen (screens are
  anchored in `--root` and the reference hop does not cross roots): its placement and an unknown
  carry `aliased-root-caller-not-placed`, so the join is not `complete` (invariant 2). One path
  MAY be read from two roots: calls and placement are keyed by root and path, so an aliased file
  is never placed through a `--root` file of the same path. The router-file draw allowance
  (AMAP-V0-042) is shared across every root's `--rails-routes`. An alias
  the map did not read or that is not bound refuses `appmap-root-unavailable`, and an unsafe
  relative part refuses `appmap-invalid-query`. A `--root`-only join MUST be byte-identical to
  AMAP-V0-212..221 output (no `repo` member). Failure mode (#1907): an end-to-end suite whose
  client or server lives in a second repository could not be joined at all. Non-goals: placing an
  aliased-root caller on a screen through that root's own references; reading an aliased root's
  worktree rather than its pin. Rollback: drop `Repo` from the join records, the per-root reads in
  `Endpoints`, and `--repo` on the verb. Status: experimental under the standing lane auto-accept
  (V1-2876; GitHub #1907).
- `AMAP-V0-223`: `flows appmap impact` MUST accept the AMAP-V0-222 inputs and `--repo`, and
  `corvint-test e2e obligations` MUST pass its bound aliased roots to the join (CCE-V0-152). Only
  endpoints declared in `--root` MUST be matched to `--root`'s changed paths, so a changed path
  equal to an aliased root's controller or declaration path is never placed through the join; the
  `endpoint-callers-incomplete` detail MUST name its first unknown root-qualified
  (`ALIAS:PATH REASON`). Failure mode (#1907): obligations could not open an aliased endpoint
  input, and a same-named `--root` path could borrow an aliased endpoint's screens. Non-goals:
  changed paths in an aliased root (impact reads only `--root`'s diff). Rollback: drop the
  `e.Repo` filter in `impactEndpoints` and `Repos` in the obligations join options. Status:
  experimental under the standing lane auto-accept (V1-2876; GitHub #1907).
- `AMAP-V0-240`: `BuildFrom` MUST re-derive only the flows, and keep every other entry of MAP,
  when every read change in `--root` that AMAP-V0-211 would otherwise call `changed-input` is a
  status-`M` content edit (mode and size width unchanged) of a `.json` file directly in the
  manifest's `flows` directory that `appflows.LoadIntentsAt` loads (`appflows.IntentCandidate`, so
  never `origins.json`), that lies under no other manifest input except the tests root and
  matches no `router_globs` pattern, and no other entry of MAP (with its flows, steps,
  preconditions, `flow-step` edges and `step` unknowns removed) names that path. It keeps MAP's
  screens, files, other edges and other unknowns, re-reads every flow at `--revision`, and computes
  the digest as a full build does, so the map equals a full build byte for byte;
  `appmap.Incremental` gains `rederived: ["flows"]` with `reused` false and no reason,
  `flows appmap build --from` writes `flows appmap build: --from MAP re-derived only: flows` to
  stderr, and `--verify` also checks a re-derived map. An added or removed flow, an edit in an
  aliased root, a flow that a spec or other input names, or any other read change rebuilds in full
  with `changed-input`. Failure mode (V1-2875; #1913): editing one flow re-derived the whole map.
  Failure mode (amended 2026-10-27, batch review): the first rule admitted `flows/origins.json`,
  which the flows stage never loads; a tests-root tsconfig extending it changed the full build's
  import resolution while `BuildFrom` kept the stale map. Non-goals: per-entry re-derivation for spec, page-object or config inputs (router and
  application source edits are AMAP-V0-245; the map records no per-entry dependency set, the context index is whole-repository, and the screen/test join and
  import graph span files). Compatibility: `Incremental` gains an optional member; map bytes are
  unchanged. Rollback: delete `rederiveFlows`, `flowScope` and the `Rederived` field. Status:
  experimental under the standing lane auto-accept (V1-2875; GitHub #1913).
- `AMAP-V0-245`: `BuildFrom` MUST re-derive the routers, screens, test joins and flows, and keep
  each test file's own entry, when every read change in `--root` that AMAP-V0-211 would otherwise
  call `changed-input` is a status-`M` content edit (mode and size width unchanged), at least one
  is not an AMAP-V0-240 flow edit, and none is the manifest, a `.git*` file, a path under
  `.corvint`, `Makefile`, `go.mod`, any `.json` file, or, when the tests are read from `--root`, a
  web-suffix file under the tests root (a router, a `di_constants` file or an application module,
  say). The tests stage reads only the test files, `package.json` and JSON configs, and resolves
  imports by path presence, so its output is unchanged: each test file keeps its anchor, facts,
  imports and the tests stage's `file`, `import` and `selector` unknowns, with the screen binding
  reset (and a joined file's `join` reset) before the join runs again over the new screens; its
  navigations are placed again from the pinned blob. It MUST rebuild in full instead, with a stated
  reason, when the edit may change the context index beyond the edited content:
  `index-admission-changed` when an edited path's blob crosses the per-source size bound, gains or
  loses a generated or LFS header (`contextindex.SourceAdmitted`), or grows while the revision
  fails `contextindex.CheckRevisionAdmission` (the aggregate bound); `config-shaped-source` when
  an admitted non-`.json` source parses as JSON or JSONC on either side, since a config `extends`
  may read any indexed path; and `changed-input` when an edited blob exceeds the 64 MiB read
  bound. The same admission check applies to an AMAP-V0-240 flow edit. An edit in an aliased root
  still rebuilds in full. The context index is still built over the whole repository when the
  routers read it (`di_constants`, or a router of the default `ui-router-states/0` dialect);
  `rails-routes/0` and `react-router/0` routers re-derive without it. `appmap.Incremental` gains
  `rederived: ["routers", "test-joins"]`, plus `"flows"` when the manifest declares flows, and
  `flows appmap build --from` names them on stderr; `--verify` also checks the re-derived map,
  which equals a full build byte for byte. Failure mode (V1-2875; #1913): any router or
  application source edit re-derived the whole map, including every test file. Non-goals:
  re-deriving only the screens or joins an edit reaches (the screen/test join and flows span
  files); per-entry re-derivation for test, page-object, config or aliased-root edits; avoiding
  the whole-repository index for the default dialect. Not measured: wall time against a full
  build. Compatibility: no wire change; `Incremental` reasons gain `index-admission-changed` and
  `config-shaped-source`. Rollback: delete `rederiveSources`, `keepTests`, `sourceScope`,
  `admissionBlocked` and `internal/contextindex/admission_api.go`, and restore `changed-input` for
  every read source edit. Status: experimental under the standing lane auto-accept (V1-2875;
  GitHub #1913).
- `AMAP-V0-340`: The injected-name reach walk (AMAP-V0-203) and the may-hold check MUST pass over a
  repository module whose extension is a template, stylesheet, data, image or font asset (the
  closure's asset set: `.html`, `.htm`, `.css`, `.scss`, `.sass`, `.less`, `.styl`, `.json`,
  `.svg`, images, fonts, `.txt`, `.md`), whatever the import form: such a module runs no code, so it
  can neither load nor export another module's table. A script module the reader cannot read still
  refuses with `reach-unread`. Failure mode (#1932): `import tpl from './x.html'` in a reached
  service made 120 states `non-literal-name ... reach-unread: x.html`. Non-goals: loaders that
  compile an asset into code (a bundler plugin that runs a template as script); reading an asset's
  contents. Rollback: drop the `assetSuffix` checks in `walk` and `mayHold`. Status: experimental
  under the standing lane auto-accept (V1-2926; GitHub #1932).
- `AMAP-V0-341`: A top-level named router function (`function F(...)` or `const F = (...) =>`)
  passed as the last element of an inline `.config(['a', ..., F])` annotation, a trailing comma
  allowed, MUST bind each annotation name to its parameter position, as `F.$inject` does
  (AMAP-V0-207): every annotation of F, `$inject` and inline alike, must name the parameter at its
  position with F's arity, else the read stays unknown with `annotation-arity` or
  `annotation-position`; any other use of F still refuses. Failure mode (#1932): 18 states read
  `not-injected` for `.config([...names, Routes])` while the `$inject` form resolved. Non-goals: a
  function imported from another module, an annotation array held in a variable. Rollback: drop
  `configArrayNames` from `injectable` and `onlyInjected`. Status: experimental under the
  standing lane auto-accept (V1-2926; GitHub #1932).
- `AMAP-V0-342`: For a page object that navigates nowhere itself and is `page-object-unbound`, the
  map MUST record as `helper_gotos` each plain call that passes an exact literal URL to a
  navigation helper: a top-level function of the page object's own file, or one a test-tree file
  exports and the page object imports by name, whose body passes one unassigned parameter into
  `.goto(...)` or `.waitForURL(...)` alone or after one `X +` prefix that is not itself a
  parameter (a return type, generic or not, may precede the body). Each entry carries the call
  line, the literal, the screen the URL resolves to when one does, status `unknown` and reason
  `helper-candidate:<helper file>:<name>:<position>`. The page object stays unbound: the
  `page_object_screens` remedy (AMAP-V0-210) offers the resolved states as candidates with a
  `learned:` basis and states that the map binds no screen from them. Failure mode (#1934): 84
  page objects that navigate through `openRoute(this.page, '/area/items')` each read "navigates
  nowhere the map resolves". Non-goals: more than one hop, template-literal URLs, binding a page
  object from a learned candidate. Compatibility: an optional, additive member. Rollback: drop
  `HelperGotos`, `internal/appmap/helpergoto.go` and the remedy's learned branch. Status:
  experimental under the standing lane auto-accept (V1-2928; GitHub #1934).
- `AMAP-V0-343`: `flows appmap stats --remedies` MUST add `unknowns_without_remedy_key` and a
  `remedies` section: every `UnknownRemedy` of the map's unknowns that names a key and at least one
  candidate, grouped by file, key and candidates, with the count of unknowns each names, most first
  and then by id; an unknown whose remedy has no key or no candidate counts as without a key. Without the flag
  the report is unchanged. Failure mode (#1934): remedies surfaced only in obligations for a
  changed span, so no verb listed them for the whole map. Non-goals: applying or ranking remedies
  by merges (CCE-V0-151, CCE-V0-297). Rollback: drop the flag and `ProjectStatsWith`. Status:
  experimental under the standing lane auto-accept (V1-2928; GitHub #1934).
- `AMAP-V0-350`: `flows appmap refs` MUST refuse with `appmap-root-unavailable` when a map test
  file lives in an aliased root that neither `--repo` nor `--workspace` binds, naming the first such
  alias, its count of map test files and the `--repo ALIAS=DIR` or `--workspace` remedy; a bound
  root is read at the map's pin of it (SKH-V0-189). A test file whose blob differs from the map's
  anchor still counts as `test_files_unread`. Failure mode (#1940): the unbound root's files were
  counted unread and the check exited 0 with no references read, so omitting the alias turned a
  failing check into a passing one. The other source verbs read only the roots `--source` names
  and already refuse an unbound one; `flows appmap branches` reads `--root` only and reports its
  aliased test files as unread, which it does not turn into a pass. Non-goals: reading an aliased
  root without its binding. Rollback: drop `unboundTestRoot` to restore the unread count. Status:
  experimental under the standing lane auto-accept (GitHub #1940).
- `AMAP-V0-351`: The rails-routes/0 route compiler (`flows appmap actions`, `endpoints` and
  `impact`) MUST skip, as the unknown `non-route-statement` at its line, a statement that names no
  route word and opens no `do` block and is a literal non-relative `require` (other than of a
  routes file), a `puts`, `warn`, `raise`, `p` or `pp` call, a `rescue`, `else`, `elsif`, `ensure`,
  `when` or `then` clause, a call on a constant receiver other than `Rails.application`, or a
  local assignment, where the arguments, the clause's expression (a `rescue`'s trailing `=> name`
  aside), the call or the assigned value is an inert expression: only literals without
  interpolation, punctuation, constants, `true`, `false`, `nil`, `and`, `or`, `not`, symbols and
  method names after `.`, `:` or `::`, never `self`, `Rails.application` or a bare lower-case name
  (a local or a helper call, so `ignored = install_routes` is not skipped and keeps
  `unsupported-rails-statement`); a `begin`, `if`, `unless`, `case`, `while` or `until` region
  whose opening condition is such an expression and that holds only such statements MUST be
  skipped the same way, and any other region MUST be the unknown `rails-region-not-read` at its
  opening line with none of its routes read. The rest of the file
  is read. A `non-route-statement` stays an endpoint-join unknown but does not make impact's
  callers incomplete (AMAP-V0-220). Failure mode (#1938): a library `require` or a logged
  `begin`/`rescue` refused the whole routes file as `unsupported-rails-syntax`, so every endpoint
  it declared was unknown. Non-goals: evaluating a condition or loop, `require_relative` (still
  `unsupported-rails-statement`), and the screen-map Rails reader (AMAP-V0-040..048), which is
  unchanged. Rollback: drop `railsInert` and `railsRegion` in `internal/appmap/route_actions.go`.
  Status: experimental under the standing lane auto-accept (GitHub #1938/#1939, V1-2932).
- `AMAP-V0-352`: `constraints:` and `defaults:` options given as hash literals (keys names, strings
  or symbols; values literal route values, nested hashes and Ruby regexp literals) MUST be accepted
  on `namespace`/`scope`, `resources`/`resource` and verb routes, and are read for shape only: a
  constrained route is reachable by every call its path matches, which over-approximates the join.
  A Ruby regexp in value position (after `,`, `(`, `[`, `{`, `:` or `=>`, closed on its line) lexes
  as one opaque value. A `scope "(:name)"` or `scope "(/:name)"` block MUST declare each route under
  it both with the `:name` segment and without it (the copy unnamed); a second optional scope
  nested inside one is the unknown `rails-optional-segments` and its routes keep only the
  segment-present path. Any other non-literal option value keeps its AMAP-V0-101 reason. Failure
  mode (#1938): a regex constraint hash and a locale scope made the routes file or block
  unsupported. Non-goals: evaluating constraints, optional groups inside a route path
  (`get "a(/:b)"`). Rollback: drop `railsHashOptions`, `railsHashLiteral`, the `opaque-regex` lex
  and `routeActionParser.add`'s optional copy. Status: experimental under the standing lane auto-accept (GitHub #1938/#1939, V1-2932).
- `AMAP-V0-353`: A literal `draw :name` or `draw "a/b"` (each `/` segment a literal name) in a
  compiled routes file or a drawn file MUST read `config/routes/<name>.rb` under the application
  root (AMAP-V0-354) with the drawing statement's scope (path prefix, module, controller and
  optional segment), following draws inside drawn files to 4 levels; a deeper draw is
  `draw-depth-exceeded`, a second draw of one file `repeated-draw`, an absent file
  `draw-unavailable`, each at the drawing line. A `--rails-routes` file that another listed file
  draws, and that does not draw it back, is read only through that draw, so its endpoints are not
  declared twice. Failure mode (#1938): a draw inside a drawn file abstained, so endpoints declared
  two files down were missing. Non-goals: draws outside `config/routes/`, the screen-map Rails
  reader's one-level draws. Rollback: set `maxRailsDrawDepth` to 1 and drop the peer-drawn skip in
  `readRailsRouters`. Status: experimental under the standing lane auto-accept (GitHub #1938/#1939, V1-2932).
- `AMAP-V0-354`: The application root of a compiled routes file MUST be the nearest directory at or
  above it (up to the repository root) holding `config/routes.rb` or `config/application.rb` as a
  blob at the read revision; controller files (`app/controllers/<controller>_controller.rb`) and
  draws resolve under it. When none holds either, `flows appmap endpoints` MUST report
  `rails-app-root-unresolved` at the file's line 1, detail "no config/routes.rb or
  config/application.rb at or above DIR", and leave each endpoint's controller empty; `actions`
  keeps its AMAP-V0-102 derivation. Failure mode (#1939): a drawn routes file listed alone resolved
  its controllers under `config/app/controllers`, so impact never matched a changed controller to
  its endpoints. Non-goals: reading `config.paths` overrides. Rollback: drop `railsAppRoot` and use
  the router's grandparent directory. Status: experimental under the standing lane auto-accept (GitHub #1938/#1939, V1-2932).
- `AMAP-V0-355`: `flows appmap impact` with Rails endpoint inputs MUST compare, for a changed path
  that is a listed `--root` Rails routes file, a Rails endpoint's declaration file or a `.rb` file
  under a listed `routes.rb`'s `routes/` directory, the endpoints (verb, path and handler) the
  listed routers compile at base and at head (a router absent at one side declares none there),
  and place only the differences: an added endpoint on the screens of the read calls that reach it
  (basis `endpoint:<ID>`), a removed one on those screens (basis `endpoint-removed:<ID>`), one
  present at both sides whose enclosing or own `constraints`/`defaults` (blocks and options, read
  as source tokens) differ (`changed`, basis `endpoint:<ID>`), each of two endpoints present at
  both sides that one request may match (verbs meet, `GET` meeting `HEAD`, and segments agree, a
  parameter with any segment and a splat with any remainder) whose declaration order flips
  (`reordered`, basis `endpoint:<ID>`), and any with no reaching call as `endpoint-not-called`
  (detail `added <ID>`, `removed <ID>`, `changed <ID>` or `reordered <ID>`). A listed `--root`
  router the change adds or deletes, so absent at the map revision, is read there as declaring no
  endpoints; one absent at the map revision and unchanged still refuses.
  Each difference is charged to its declaring file when that file changed, else to every changed
  Rails path. A route unknown at either side other than `non-route-statement` MUST keep
  `endpoint-declaration-changed` on each such path, detail "the base and head endpoint comparison
  is partial, first AT REASON"; `endpoint-callers-incomplete` is kept only when a difference was
  placed. A comment-only edit places nothing. Failure mode (#1938): any edit to a routes file
  stood for every endpoint it declared, so impact always asked for the full suite; review: a
  changed constraint, a reordered overlapping pair and an added or deleted router were missed or
  refused. Non-goals: evaluating constraints,
  OpenAPI documents and controllers, which keep AMAP-V0-220. Rollback: drop
  `impactJoin.compareRails`, `EndpointsOptions.absent`, the route `guard` and the `railsDiff`
  branch of `place`. Status: experimental under the standing lane auto-accept (GitHub #1938/#1939, V1-2932).
- `AMAP-V0-356`: The client scan (AMAP-V0-213) MUST lex every source but a `.ts`, `.mts` or `.cts`
  file with JSX: an element where an expression surely starts (a `<` followed by a name or `>`,
  not a TypeScript `<T,>`, `<T extends U>` or `<T = U>` type parameter list) becomes the tokens of its `{}`
  expression containers (attributes, spread and children, nested to 64 levels), so its tag names,
  attribute strings and text never reach the token stream; an element it cannot bound is lexer
  doubt and the file is `source-not-lexed`. Failure mode (V1-2932): JSX text such as `Don't`
  opened a string, so a `.tsx` file's calls were unread or misread. Non-goals: JSX in `.ts` files
  (a `<T>x` assertion), reading the screen map's components through this lexer. Rollback: use
  `lexJS` for every file in `scanCalls`. Status: experimental under the standing lane auto-accept (GitHub #1938/#1939, V1-2932).
- `AMAP-V0-357`: `$resource(URL, paramDefaults, actions)` with a literal `URL` MUST yield, at the
  `$resource` line with client `$resource`, one call per distinct verb of the default actions
  (`GET`, `POST`, `DELETE`) and of each declared action whose value is an object with a literal
  `method` and an optional literal `url`. A whole-segment `:name` or `{name}` is a substitution;
  a trailing one may be omitted, so the URL without it is a call too, and any other is the
  unknown `resource-param-may-be-omitted` (detail the url template). A non-literal URL or a
  segment holding `:` or braces otherwise is `non-literal-url`; an actions argument that is not an
  object literal, or an action not read, is `non-literal-resource-action` (detail `actions` or the
  action name). Failure mode (V1-2932): AngularJS resource factories were unread, so their
  endpoints looked uncalled. Non-goals: resource factories built without `$resource`, instance
  methods' `$`-prefixed calls (they share the class actions). Rollback: drop `resourceCalls`.
  Status: experimental under the standing lane auto-accept (GitHub #1938/#1939, V1-2932).
- `AMAP-V0-358`: `const|let|var NAME = axios.create(CONFIG)`, the initializer ending at the create
  call, with no argument or an object literal whose `baseURL` is absent or a literal and whose
  other keys are only `headers`, `timeout`, `withCredentials`, `responseType`, `responseEncoding`,
  `xsrfCookieName`, `xsrfHeaderName`, `params`, `paramsSerializer`, `maxContentLength`,
  `maxBodyLength`, `maxRedirects`, `validateStatus`, `decompress`, `auth`, `proxy`,
  `transformResponse` or `timeoutErrorMessage` MUST make NAME's calls in that file read as axios calls
  (client `axios-instance`) whose relative URL is joined to the base (one `/` between them, as
  axios does); a URL led by a substitution or absolute is not joined. A non-literal or non-object
  configuration, any other key (such as `method`, `url` or `adapter`) and an initializer chained
  past the create (`axios.create(...).get(...)`) are `client-instance-not-modelled` (detail NAME)
  at the create, and the instance's
  calls are not read; a create bound to no declared name keeps `client-instance-not-modelled`
  (detail `axios.create`); NAME declared again, reassigned or rebound as a parameter or key is
  `client-instance-shadowed` with its calls unread; an exported instance is read and stated as
  `client-instance-escapes`, since calls through it in other files are not read. A per-call
  `baseURL` keeps `config-base-url`; `NAME.create` is `client-instance-not-modelled`. Failure mode
  (V1-2932): every call through an instance was unknown, so the join missed its endpoints.
  Non-goals: instances imported from another file, `NAME.defaults` writes after creation.
  Rollback: drop `axiosInstances` and `endpointCall.base`. Status: experimental under the standing lane auto-accept (GitHub #1938/#1939, V1-2932).
- `AMAP-V0-359`: A `page_object_screens` entry whose state names a screen the map holds with
  status `UNKNOWN` MUST read `unresolved-screen` (kind `page-object`, the entry's path), with
  detail `SCREEN_ID UNKNOWN: REASON` naming that screen's own blocking unknown; its remedy names
  no file or key and says the entry's state needs no change. `unknown-state` MUST be kept for a
  state the map does not hold, and its remedy says so and asks to correct the entry's state. Both
  keep the page object unbound and `test_join` `UNKNOWN` (AMAP-V0-005). Failure mode (GitHub
  #1955, V1-2969): a correctly spelled state whose screen was unresolved read `unknown-state` with
  advice to respell it. Non-goals: resolving the screen's own unknown. Rollback: fold the
  `unresolved-screen` branch of `join` back into `unknown-state`. Status: experimental under the
  standing lane auto-accept (GitHub #1955, V1-2969).
- `AMAP-V0-365`: The injected-name reach reader MUST NOT count a token of a class field's type
  annotation as a module loader: a member directly in a class body whose heading is only names,
  `.`, `,` and `<...>`, written `name: Type`, `name?: Type` or `name!: Type` after any of
  `public`, `private`, `protected`, `readonly`, `static`, `declare`, `override`, `abstract` or
  `accessor`, at the start of a member (after the body's `{`, a `;`, a member body's `}`, or on a
  new line after a name other than a keyword an expression continues past (`typeof`, `new`, `in`,
  `await`, ...), a literal, `)` or `]`); the type runs to the first `;`, `=`, line break or
  closing bracket at its own depth. An initializer, a method's parameter or return types, an
  object literal property and any other form stay code, so `Function` there still refuses. A
  `reach-module-not-literal` or `closure-module-not-literal` reason MUST append the construct it
  read when it has one (`:Function`, `:setTimeout`, `:require(`, `:import(`, `:import=`, a loader
  inside a template substitution), after the line of AMAP-V0-206. Failure mode (#1951): every
  DI-constant state read `reach-module-not-literal:<models>/provider.model.ts:11` for
  `onSuccess: Function`. Non-goals: method signatures, index signatures, decorators before a
  field, a loader the module graph really holds (it still refuses every state the graph reaches).
  Rollback: drop `classFieldTypes` and `spaceWhat`. Status: experimental under the standing lane
  auto-accept (V1-2965; GitHub #1951).
- `AMAP-V0-366`: The Rails screen reader (`routers` of kind `rails`) MUST skip a statement that
  provably declares no route and consume a begin/if/unless/case/while/until region to its `end`
  as the endpoint reader does (AMAP-V0-351), each an unknown at its line (`non-route-statement`,
  `rails-region-not-read`, `unresolved-parent` under a blocked scope) instead of refusing the
  file. A region is skipped only when each keyword `end` in it is a statement by itself, each
  keyword `do` ends its statement and no `case`, `begin` or `x = if` opens an expression; a region
  holding `next`, `break`, `return`, `redo`, `retry` or `throw` refuses the file in both readers,
  since it may decide whether later routes exist; `:do` opens no block. A `namespace` or `scope` block with a top-level `constraints:` or `defaults:` option MUST
  be the unknown `rails-constraints` / `rails-defaults` at its line with its children
  `unresolved-parent`, as the block form is, since a format constraint may exclude HTML screens;
  the rest of the file is read. The endpoint reader compiles the same routes file, the issue's
  `POST /api/items/{}/confirm` included. Failure mode (#1954): a routes file starting with
  `require "x/web"` and a logging begin/rescue region was refused whole at line 1. Non-goals:
  screens under a constrained scope. Rollback: drop the region and inert branches of
  `railsParser.parse` and `railsGuardOption`. Status: experimental under the standing lane
  auto-accept (V1-2968; GitHub #1954).
- `AMAP-V0-367`: A navigation helper (AMAP-V0-342) MUST also be a function that passes its one
  parameter to `.goto(...)` or `.waitForURL(...)` as the only, trailing substitution of a template
  literal (`` `/app#${route}` ``; the literal before it is the helper's prefix), or as the hash of
  a URL object: `const v = new URL(X.url())` (or `location.href`, `window.location.href`) and one `v.hash = p` statement in the body before the
  call, `v` used nowhere else, passed as `v`, `v.href` or `v.toString()`. A call's candidate URL
  is the prefix and the literal, or `#` and the literal (one leading `#` dropped) for the hash
  form; a URL whose hash comes from the helper is placed from its hash on (the page before it
  serves the hash router's states) only when the map's `hash_prefix` is set and starts the hash,
  and otherwise names no screen. A function that navigates only by other forms is not a helper;
  each plain call of one by an unbound page object MUST be named in that `page-object-unbound`
  unknown's detail, sorted and comma-joined, as
  `helper-unfollowed:<helper file>:<line>:<name>:<reason>` with reason `template-form`,
  `url-object` or `expression` and the line of the helper's first such navigation. Failure mode
  (#1953): 0 of 84 page objects that navigate through `` goto(`/app#${route}`) `` or a URL-hash
  helper offered a candidate. Non-goals: binding a page object from a candidate, more than one
  hop, two substitutions, `let` URL objects. Rollback: drop `helperForm`, `navArg`, `urlHash` and
  the gap detail. Status: experimental under the standing lane auto-accept (V1-2967; GitHub #1953).
- `AMAP-V0-368`: `flows appmap build --from` MUST answer a regular file added in `--root` like a
  source edit (AMAP-V0-245, re-deriving routers, test joins and flows) when it lies outside every
  manifest input, router glob and the test root, is not the manifest, Git metadata, Corvint
  configuration, a Makefile, go.mod or `.json` file, has no suffix a web import resolves by
  probing (`.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.cjs`), and no kept test file's import names its
  base name as the specifier's last segment, cleaned or with a web suffix stripped (`theme.css.js`
  probes `theme.css`); while any bare test import is unresolved or resolved
  to a repository file (a `paths` mapping may point anywhere) an added path still rebuilds. An
  added path the index admits passes the same config-shaped and aggregate-bound checks as an edit,
  and any added path re-checks the revision's admission bound. The map is the full build's.
  Failure mode (#1952): 8 of 20 commits adding only `spec/**/*.rb` or `app/services/**` files read
  `rebuilt in full: changed-input` with byte-identical output. Non-goals: added paths in an
  aliased root, removed or renamed paths. Rollback: drop `sourceScope.add` and `addedNamed`.
  Status: experimental under the standing lane auto-accept (V1-2966; GitHub #1952).
- `AMAP-V0-369`: The injected-name reader MUST NOT count a `.constructor` property ending a plain
  dotted chain of names (`x.constructor`, `this.constructor`) as a module loader when it is only
  tested -- the left operand of `&&`, or negated by `!` with nothing after it but `)`, `;`, `,`,
  `}`, `&` or `|` -- or constructed with no arguments, `new x.constructor()` or
  `new (x.constructor)()` with an optional `as T` cast inside the parentheses: the first yields
  the value only when it is falsy and `new Function()` evaluates no string. Called, passed on,
  constructed with arguments, the right operand of `&&`, an operand of `||` and a computed or
  optional chain stay loaders, named `:constructor`. The examined closure of a table read through
  an import chain (AMAP-V0-025, `import-chain-examined`) MUST still examine every file it loads
  for injection (`closure-not-quiet`), now up to the reach bound of 4096 files, but MUST apply the
  binding check (`bindingsRead`) and the closure limit only to its files whose followed import
  graph may reach the declaring file: that file, any file the reader does not read item by item,
  and every file that loads one of them. A file whose every import the closure follows and none
  reaches the declaring file cannot bind its table. Decision on #1963's proposal to prune the
  whole closure: rejected, since an AngularJS module loaded only for its effects can inject and
  write the constant by name through DI without importing it (AMAP-V0-206's non-goal stands). The
  limit defaults to 256; the manifest's optional `di_closure_limit` (256 to 4096, only with
  `di_constants`) raises it, and the remedy of an unknown whose detail ends in `closure-limit:N`
  below 4096 MUST name the manifest key `di_closure_limit`. Failure mode (#1963): 138 of 138
  states read `reach-module-not-literal:...:constructor` for `model.constructor &&
  model.constructor.ns`, then for `new (this.constructor as any)()`, then
  `import-chain-examined:closure-limit:256` from the registering module's unrelated services.
  Non-goals: declaring a constant module read-only, `.constructor` reached through a computed
  member, a closure over 4096 files. Rollback: drop `constructorInert`, `reaching`, `limit`,
  `DIClosureLimit` and the remedy's closure-limit branch. Status: experimental under the standing
  lane auto-accept (V1-3056; GitHub #1963).
- `AMAP-V0-370`: A navigation helper (AMAP-V0-342, AMAP-V0-367) MUST also be a function whose
  other parameters carry a default that is one literal (a string, a template without
  substitutions, a number such as `60_000`, `true`, `false`, `null` or `undefined`), typed or
  not (`timeout: number = 60_000`). A function with any other default whose body navigates MUST
  be the gap `default-parameter` at the function's line. A candidate URL placed from its hash on
  MUST be looked up under the manifest's `base_path` (`base_path + "/" + hash`), and places no
  screen when the URL names an explicit page before the hash that lies outside `base_path`
  (AMAP-V0-191). An unbound page
  object's call `this.m(...)` MUST follow one hop more when m is a method of its own (first) class
  or, when that class declares no method m, of the class its `extends` clause names and imports by name from a test file, and
  m either navigates through a parameter itself (AMAP-V0-342/367 forms) or passes one parameter,
  alone, to exactly one navigation helper of m's file at that helper's URL position; a method
  declared twice in the class is read from neither declaration; the candidate
  reason is `helper-candidate:<helper file>:this.m:<position>`, and a gap of that helper is named
  as `helper-unfollowed:<helper file>:<line>:this.m:<reason>`. A call argument that is a template
  literal with substitutions is the candidate URL with each substitution `${}`; it places a
  screen only when each substitution is a whole segment matching a parameter segment
  (`/clubs/${id}/settings` places `/clubs/{}/settings`). Candidates are sorted by line. The
  `page-object-unbound` detail MUST never be empty: without a gap it is
  `helper-candidates:<count>` or `no-navigation`. Decision on #1964 item 4: helper candidates
  stay remedy candidates and never become bindings; the `page_object_screens` remedy
  (AMAP-V0-210) already lists each resolved candidate as a ready-to-paste
  `{"path": ..., "state": ...}` entry, so a hand binding can be replaced by pasting it. Failure
  mode (#1964): `openRoute(page, route, timeout: number = 60_000)` hid the helper with an empty
  detail; `/app#/...` candidates never placed under `base_path: "/app"`; inherited
  `this.openRoute(...)` calls and template arguments were not followed. Non-goals: more than one
  hop through methods, getters, `super.m(...)`, a method passing its parameter through an
  expression, a second class in one file, a class heading holding braces, binding a page object from a candidate. Rollback: drop `helperParamList`,
  `classNav`, `thisCalls`, `baseMethods`, the base path in `helperGoto` and the non-empty detail.
  Status: experimental under the standing lane auto-accept (V1-3057; GitHub #1964).
- `AMAP-V0-371`: Both Rails readers MUST consume a statement that ends in a block with block
  parameters (`%w[a b].each do |ns|`, `direct :x do |model|`, parameters being names, `,`, `*`,
  `&` and parentheses) to its matching `end` as a region (AMAP-V0-366: each nested `end` a
  statement by itself, no control transfer), unread, the unknown `rails-block-parameters` at its
  line (`unresolved-parent` under a blocked scope), and read the rest of the file; a region that
  holds such a block is consumed the same way. A body the reader cannot place or one that may
  transfer control still refuses the file, and the endpoint reader's refusal
  `unsupported-rails-syntax` MUST carry the line of the statement that refused it (1 only when a
  block is left open at the end; the screen reader keeps line 1). A
  `scope` or `namespace` option whose key names a dynamic segment of the scope's path (the
  positional path or `path:`, `(:locale)` included) and whose value is a regexp is a segment
  constraint: the endpoint reader records it in the scope's guard as for `constraints:` and reads
  the block, so `scope "(:locale)", locale: /en|fr/ do draw :api end` reads the drawn file under
  the optional segment (both paths, AMAP-V0-354). Failure mode (#1965): a namespace file with
  `%w[a b].each do |ns| ... end` and a main file with `direct :public_blob do |model| ... end`
  were refused whole at line 1 with 0 endpoints, and the locale scope blocked every drawn file.
  Non-goals: expanding a literal list loop (each expanded route would share one declaration line),
  brace blocks, reading a `direct` helper's route. Rollback: drop `railsBlockParams`, its branches
  in both parsers, `railsSegmentConstraints` and the refusal line. Status: experimental under the
  standing lane auto-accept (V1-3058; GitHub #1965).
- `AMAP-V0-372`: When the manifest reads its tests from an aliased root (`tests.repo`), `flows
  appmap build --from` MUST judge a path added in `--root` (AMAP-V0-368) only against the inputs
  read from `--root` -- the manifest path, routers, `di_constants` and flows -- and MUST NOT
  rebuild for a kept test import, since the tests resolve through the aliased root's own index,
  which a `--root` addition cannot change; any change to the aliased root is its own span and
  still rebuilds when it touches a read input. The map is the full build's. Failure mode
  (#1966): with `tests.repo: "e2e"` and `tests.root: "."`, 8 of 20 commits adding only server
  files rebuilt in full, because `"."` matched every added path and bare test imports resolved
  in the test repository. Non-goals: added paths in the aliased root, removed or renamed paths.
  Rollback: drop `sourceScope.rootInputs` and the aliased branch before `addedNamed`. Status:
  experimental under the standing lane auto-accept (V1-3059; GitHub #1966).
- `AMAP-V0-373`: The examined-closure injection check (diQuiet, AMAP-V0-025) MUST NOT read as an
  inline annotation an array that is the value of a property `K: [...]` of the object literal
  that is the whole first argument of a class decorator (`@D({...})`, `@A.B({...})`, followed past
  any further decorators by `export`, `default`, `abstract` or `class`): decorator metadata is read
  by its framework, never by the AngularJS injector; an array nested in it, and any other array,
  is judged as before. A template substitution that is not one name, member chain or literal MUST
  NOT by itself make a file not quiet: its code is lexed and passes the same checks as the file's
  tokens -- no injector or decorator name (diReach), no identifier, string or template naming the
  constant or its implicit `_name_`, no inline annotation the reader cannot read -- and nested
  templates are read the same way up to 8 levels; a substitution whose end the scan cannot place
  (a `/`, which may open a comment or regular expression, included), or that the lexer doubts,
  still fails. `closure-not-quiet` MUST append `:line:construct` naming the failure (AMAP-V0-422
  extends this to every distinct failure in the closure):
  `template-substitution`, `computed-annotation`, `$inject`, `string`, the injector or
  decorator name, `_name_`, or the constant's name for a use not provably only read; a file the
  reader does not read item by item appends `:unread`. Failure mode (#1968): every state read
  `closure-not-quiet:<file>` for `@NgModule({ imports: [Lib.forFeature(KEY, reducer)],
  providers: [Svc] })` and `` `.json?${new Date().getTime()}` ``. Non-goals: method or property
  decorators (their arrays stay annotations), a name the substitution builds at run time other
  than by a visible string or array. Rollback: drop `decoratorMetadata`, `substQuiet`,
  `substEnd` and the `line:construct` suffix. Status: experimental under the standing lane
  auto-accept (V1-3065; GitHub #1968).
- `AMAP-V0-384`: The examined-closure injection check (diQuiet, AMAP-V0-025) MUST NOT read as an
  inline annotation an array literal whose last element is an exact literal, number, `true`,
  `false`, `null`, an object or an array, unless the array lies directly in parentheses (a call's
  argument, which the callee may complete) or is assigned (`= [...]`) to a binding or member, which
  a later `.pop()` and `.push(fn)` may complete (V1-3089; such an array is still exempt as a
  `const` under AMAP-V0-385 only when its every use is admitted there): the injector invokes only an annotation whose last
  element is a function, so its other elements, dotted members included, name nothing it reads.
  Failure mode (#1979): every state read `closure-not-quiet:<file>:<line>:computed-annotation` for
  a route data object `{ only: [Enum.MEMBER, 'literal'] }`. Non-goals: an array mutated through
  a property path after its literal is evaluated (as for the all-literal arrays already
  accepted). Rollback: drop the
  `plainElement` branch before the element scan of `computedAnnotationIn`. Status: experimental
  under the standing lane auto-accept (V1-3082; GitHub #1979).
- `AMAP-V0-385`: The same check MUST NOT read as an inline annotation an array literal that is the
  whole initializer of an unexported `const NAME = [...]` (ended by `;`, `}`, the end of the file,
  or a name or decorator on a later line) whose last element is no function or arrow literal and
  whose every use of NAME is the whole value of a class-decorator metadata property
  (AMAP-V0-373) or a whole spread element `...NAME` of an array literal, which is judged by
  itself: such an array reaches no call and so no injector. Any other use -- a call argument, a
  spread into a call, an export, a shorthand property, a member call such as `.concat`, another
  binding of NAME or any template code naming it -- and `let` or `var` keep the array judged as an
  annotation. Failure mode (#1979): every state read
  `closure-not-quiet:<file>:<line>:computed-annotation` for a module-scope
  `const LIST = [ModA, ModB, ModC]` read by `@NgModule({ imports: LIST })`. Non-goals: an exported
  const read by another module (its uses there are not followed), a const read by an ordinary
  call. Rollback: drop `constHeld` and `functionLiteral`. Status: experimental under the standing
  lane auto-accept (V1-3082; GitHub #1979). AMAP-V0-420 adds closure-bound proofs for exported
  lists and scalar consumers; other escaping uses retain this refusal.
- `AMAP-V0-420`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3097)
  The examined-closure injection check MUST distinguish an array's value from the function
  returning it, and MUST exempt a computed array only with a non-injector use proof. In addition
  to AMAP-V0-373/384/385, admitted proofs are a literal used solely for a read of `.length` or
  `.join()` with no argument or one exact separator; a whole `const` data-list initializer whose
  references are only those scalar uses or class-decorator metadata; a named module-local top-level function
  with no executable reference in the examined closure; and a function parameter copied by
  `xs = [...xs]` whose other body references are only scalar array uses. Named imports of an
  exported list or dormant function MUST pass the same use check in every examined consumer.
  Namespace/dynamic consumers, re-exports, rebinding, hidden references, aliases, unknown calls,
  returned or passed copies, and opaque consumers MUST retain uncertainty and the prior refusal.
  Direct array arguments remain checked even in a dormant function. A synchronous arrow whose
  whole body is the array and which is the sole argument of an unshadowed named `mergeMap`
  import from the external `rxjs` or `rxjs/operators` package MAY be exempt: that operator
  flattens the returned array into elements, without passing the array identity to consumers.
  Local package substitutions, async callbacks, other operators, unknown callbacks and nested
  array elements have no such exemption. Proof caches MUST include the consumer closure; a proof
  for one router MUST NOT hide a later consumer reaching an injector. Real inline annotations
  passed to module registration, injector invoke/instantiate or `$inject` assignments MUST stay
  checked. Failure mode (#1994): ordinary action arrays, member receivers, exported pipe lists
  and helper spreads made every state unknown. Non-goals: arbitrary interprocedural evaluation,
  unknown callback semantics, prototype monkey-patching, or proving consumers outside the examined
  closure. Rollback: remove only these additive reach exemptions. Delivery: experimental; the
  RxJS consumer model is checked against its 7.8.2 source, not a general library API evaluator.
- `AMAP-V0-421`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3097)
  An exact `$injector.get('other')` or `injector.get('other')` naming a service different from the
  constant being read MUST leave the file quiet when it is a whole return, a discarded expression,
  or a nonexported const initializer whose references are provably only reads. A plain parameter
  declaration of that injector name MAY be exempt only when every body reference is an admitted
  get and no hidden reference exists. Dynamic or same-name gets, aliases, writes, chained uses,
  extra arguments and other injector operations MUST still block. Failure mode (#1994): an
  ng-upgrade factory returning a different service blocked unrelated state-name constants.
  Non-goal: arbitrary result escape analysis or exemptions inside template substitutions.
  Rollback: remove the exact-get exemption; no registration or injection binding rule changes.
- `AMAP-V0-422`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3097)
  After following the examined closure, its refusal detail MUST retain every distinct blocking
  file/line/construct, including multiple unreadable `$inject` annotations and hidden injector
  occurrences, rather than stopping at the first file or construct. Each quietness item keeps the
  existing `closure-not-quiet:path:line:construct` form (or `:unread`), joined with `;` in stable
  traversal/check order. Other binding refusals found in that traversal are retained too. The
  outer diagnostic code and reason and the single-item spelling stay unchanged. A closure that
  cannot be followed still refuses at that boundary; it MUST NOT claim to enumerate unread files.
  Failure mode (#1994): removing one blocker merely moved the diagnostic to the next file.
  Rollback: restore first-failure detail without changing the fail-closed decision.
- `AMAP-V0-423`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3110)
  The examined-closure injection check MUST read a `static $inject = [...]` field, optionally
  `readonly` and typed (`: T`), that is a member of the body of a named top-level class
  declaration (alone, `export`, `export default` or `abstract`) as that class's annotation, under
  the same rules as a top-level `Name.$inject = [...]` statement: the value is an exact list of
  string literals and ends its member or statement (`;`, `}`, the end of the file, or a later line
  starting with a name, string, number or `@`); a class or function is annotated once in the file;
  and a list naming the constant still blocks as a `string` use. A file whose only `$inject` uses
  are such lists not naming the constant MUST stay quiet. A computed, spread or substituted
  element, a second annotation of the same class (a second field or a later `A.$inject = [...]`),
  a later write such as `A.$inject.push(...)` or `this.constructor.$inject`, an instance field, a
  class expression, a nested or anonymous class, and a value continued past its line
  (`['a'].concat(x)`, or `.concat`/`(` on the next line) MUST keep the file blocking with
  `:$inject`. The same statement-end check now also applies to `Name.$inject = [...]`, which
  previously accepted a list continued on the next line by `.` or `(`; that form now blocks.
  Failure mode (#2004): a class carrying `static $inject = ['Action', 'Store', 'Model']` made
  every state in its closure `closure-not-quiet:<file>:<line>:$inject`. Non-goals: binding an
  injected parameter of a class constructor (AMAP-V0-022 still reads only router-file functions),
  static blocks, `Object.defineProperty`, decorators that write `$inject`. Rollback: drop
  `staticInject`, `classDeclaration` and `typeEnd`, and restore the previous end check in
  `annotations`. Delivery: experimental.
- `AMAP-V0-424`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3111)
  In addition to AMAP-V0-420, the check MUST exempt an array literal only by one of these bounded
  non-injector proofs, each evaluated against the whole examined closure: (a) its receiver call is
  `includes`, `indexOf` or `lastIndexOf` with arguments, `shift()` or `pop()` with none whose
  element result is an element sink or the whole initializer of a `const`/`let` binding used only
  as element sinks, or `forEach`, `some`, `every`, `find`, `findIndex`, `findLast` or
  `findLastIndex` with an inline arrow of at most two parameters whose element parameter is used
  only as element sinks (the element `find`/`findLast` returns too); an element sink is an operand
  of `==`, `!=`, `===` or `!==`, the operand of `typeof`, a discarded expression statement or the
  whole argument of `includes`/`indexOf`/`lastIndexOf` on an array literal, never a value that is
  stored, pushed, passed, returned, spread, called, indexed or placed in a container; (b) a
  parameter copy `xs = [...xs]` (AMAP-V0-420) whose other references are those element uses, the
  iterable of a `for (const|let V of xs)` head whose plain V is used only as element sinks to the
  end of the enclosing block, or a whole spread element `...xs` of a non-pattern array literal; (c) a `const NAME: T = [...]` initializer, exported or
  not, whose references pass AMAP-V0-385/420 (a top-level arrow element still counts as a
  function; an arrow nested in a call does not); (d) the whole argument of `NAME.push(...)` where
  NAME is a single `const`/`let NAME = []` whose initializer is exactly `[]` up to the statement
  end, unexported and unhidden, whose every other reference
  is `push`, `includes`/`indexOf`/`lastIndexOf` or a scalar use (AMAP-V0-420); (e) a whole
  argument of `this.m(...)` inside a method of a class body when m is not an injector-facing name
  (`run`, `config`, `controller`, `service`, `factory`, `provider`, `directive`, `component`,
  `filter`, `constant`, `value`, `decorator`, `animation`, `invoke`, `instantiate`, `annotate`,
  `get`, `register`, `has`), no argument before the array is a spread, every `this` in the class
  body is a plain `this.name` access (no alias and no `this[`), the class body declares m exactly
  once with simple parameters, m's body has no `arguments` and every reference to that parameter
  passes (b)'s uses, in every examined file m appears only as that declaration or a `.m(` call,
  with no string, template, unlisted, opaque or hidden occurrence, and no examined file has a
  computed class member or object key, an assignment (including `||=`, `&&=`, `??=`) to a
  computed member or through an assignment pattern, `Function(...)`, or the names
  `defineProperty`, `defineProperties`, `setPrototypeOf`, `__proto__`, `__defineGetter__`,
  `Reflect`, `Proxy`, `eval`, `assign`, `fromEntries` or `with`. Exported lists spread whole into
  class-decorator metadata in a consumer are admitted through AMAP-V0-420's consumer check.
  Any other use MUST keep `computed-annotation`, including: a pushed local that is returned,
  iterated or indexed, or whose initializer continues past `[]`; an element pushed, stored,
  passed, returned or rebuilt into an array from a `for ... of` body, a callback or a
  `find`/`shift`/`pop` result; a destructuring `for ... of` binding; `map`, `filter`, `slice` and
  `concat` results; a named or three-parameter callback; a copy passed, spread into a call or returned; a typed const passed
  or exported to a sink; a method that escapes, reads `arguments`, is overridden in a subclass,
  is mentioned elsewhere in the closure, has an injector-facing name, is called from a `function`
  (rebinding `this`) or from a class-field arrow, is not declared, is called on another object or
  after a spread argument, or whose class aliases or returns `this`, or whose closure can define a
  member under a computed or reflective name;
  `let`/`var` typed initializers. Missing proof keeps the blocker (product invariant 2). Failure
  mode (#2005): element-wise copies, local accumulators, literal `includes` receivers, typed
  const lists and method arguments made every state in the closure unknown. Non-goals: general
  escape or alias analysis, callback semantics beyond the listed methods, inheritance outside the
  examined closure. Element sinks and the closure scan over-approximate: a shadowing inner binding
  of the element's name, `!v` and other unlisted scalar uses still block. Rollback: drop
  `copyUse`, `elementUse`, `pushedLocal`, `methodArgument` and their helpers (including
  `elementSink`, `elementBinding`, `boundElement` and `dynamicMembers`), and restore `scalarArrayUse` in `localArrayCopy`/`dataNameUses` and
  `functionLiteral` in `heldArray`. Delivery: experimental.
- `AMAP-V0-425`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3161)
  The check MUST NOT read as an inline annotation an array binding pattern that is a whole
  parameter of an arrow function or a `function`: the outermost `[` follows the parameter list's
  `(` or a `,` in it, its `]` is followed by `,` or `)`, the pattern holds only names, holes, rest
  dots and nested `[...]`, and the list's `)` is followed by a glued `=>` or its `(` follows
  `function` or `function NAME` (not a property named `function`). Such a pattern binds elements of
  an argument and creates no array value, as the `const [a, b] = v` pattern already exempt does.
  A parameter pattern with a default or a type annotation, an object pattern holding an array
  pattern, a rest parameter `...[a]`, a method's parameter list and a parenthesized call argument
  (`register(([dep, fn]))`) MUST keep `computed-annotation`; a default value is an array value.
  Failure mode (#2020): `Object.entries(X).forEach(([, v]) => ...)` blocked every state.
  Non-goals: typed parameter patterns, method and constructor parameter lists. Rollback: drop
  `parameterPattern`. Delivery: experimental.
- `AMAP-V0-426`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3161)
  A reference to a `const NAME` (typed or not) whose initializer passes AMAP-V0-385/420/424 (c)
  MAY also be a whole argument of `this.m(...)` that AMAP-V0-424 (e) proves for an array literal at
  that position (same class, method, closure-wide name, `this`, dynamic-member and parameter
  conditions): the array then reaches only m's element-wise parameter. The closure-wide reference
  check of NAME is unchanged, so a parameter or other binding of the same name in the file still
  refuses. The dynamic-member scan MUST NOT read an empty `[]` or `{}` followed by `=` as a
  computed write: it is a type (`const ns: Kind[] = ...`) or an empty pattern that assigns nothing;
  `E[k] = v` and `E[k] ??= v` still refuse. A typed const passed to a method whose parameter
  escapes, to an injector-facing name or to a method on another receiver MUST keep
  `computed-annotation`. For this path and for an array literal argument alike, m's parameter MUST
  NOT be used as a spread element `...p`: the copy can sit in a literal exempt by AMAP-V0-384 (a
  plain last element) that m returns, and a caller that pops the plain element registers the rebuilt
  annotation. Failure mode (#2020): `const ns: Kind[] = [Kind.A, ...t.rules.map(r =>
  r.name)]; return this.update(t, ns)` blocked every state, and any typed `T[]` const made a file
  read as defining members dynamically. Non-goals: a `let` or reassigned binding, arguments of
  calls other than `this.m(...)`. Rollback: drop the `methodArgument` alternative in
  `dataNameUses` and the empty-bracket guard in `dynamicMembers`. Delivery: experimental.
- `AMAP-V0-427`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3161)
  These #2020 forms MUST keep `closure-not-quiet:<file>:<line>:computed-annotation`, each for the
  named reason: (1) `parameter-copy-product`, a parameter copy whose elements are spread, mapped
  and pushed into an accumulator that is returned (`combine([[dep], [fn]])` returns
  `[[dep, fn]]`, an annotation built from its input); (2) `returned-copy`, a returned
  `[...list].sort(cmp)` or `.map(cb)` copy, which reaches its callers' unexamined uses
  (`run(svc.sorted(deps))`); (3) `returned-rebuild`, a returned or reassigned spread rebuild
  `[...a, ...b]` or `acc = [...acc, item]`, which is the annotation builder itself;
  (4) `escaping-parameter`, an argument of `this.m(...)` whose parameter flows into `new C({ p })`,
  an object literal or a return, whose consumer may hand it to an injector; (5) a typed const
  passed to such a method (AMAP-V0-426 refuses it); (6) `unknown-callee`, an argument of a
  member call on a held dependency (`this.dep.all([...])`), whose implementation may be a service
  outside the closure calling `$injector.invoke`; (7) `element-escape`, a literal mapped by a
  callback that passes each element to an unknown call (`[m.from, m.to].map(v => fmt(v))`), which
  may push the elements in order and rebuild the literal's sequence. Injector-facing forms MUST
  stay blocked too: a provider object's `deps: [A, B]` (alone or nested in class-decorator
  metadata), a computed `$inject` value, and an array argument of a method or property decorator.
  The eighth reported form, an arrow parameter pattern, is no array (AMAP-V0-425). Missing proof
  keeps the blocker (product invariant 2). Non-goals: caller-flow (returned value) analysis,
  callee analysis outside `this.m(...)`, element-flow analysis through unknown calls. Rollback:
  none; this requirement records refusals that hold without new code.
- `AMAP-V0-650`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3164)
  Amending AMAP-V0-384: an array literal with a plain last element MUST NOT be exempt by that
  element when one of its elements spreads a name (`...p`, `...p.deps`) that an enclosing parameter
  list binds: the list of a `function`, method or arrow whose body holds the literal, read as every
  `(...)` before a `{` and every list or single name before an arrow, directly or through a
  return type (`): T[] {`, `): T =>`), at any nesting depth. Such a
  literal copies its caller's elements, so a caller that pops the plain element and pushes a
  function registers an annotation rebuilt from its argument, as a method's spread parameter does
  (AMAP-V0-426). The literal is then judged like any other array and keeps
  `closure-not-quiet:<file>:<line>:computed-annotation` unless AMAP-V0-385 or AMAP-V0-424 proves it.
  A spread of a module const or of `this` keeps the AMAP-V0-384 exemption, and any other
  parenthesized list before a `{`, such as an `if` condition, is read as a parameter list, which
  can only keep a literal judged. Failure mode (V1-3164): `function go(i) { return [...i, 0] }`,
  whose popped result a caller registered, stayed quiet. Non-goals: a spread whose operand is no
  name (a call or parenthesized expression), caller-flow analysis. Rollback: drop
  `parameterSpread` from `computedAnnotations`. Delivery: experimental.
- `AMAP-V0-374`: A `this.m(...)` call of an unbound page object whose own class does not declare m
  (AMAP-V0-370) MUST walk the test-root `extends` chain, each hop a class its file imports by name
  from a test file whose class is that name, at most 8 hops, to the nearest class declaring m, and
  read m there as AMAP-V0-370 reads a direct base. When the walk cannot finish -- an `extends` the
  reader cannot resolve (a package class included), a cycle, or more than 8 hops -- a call passing
  a literal or template argument MUST be the gap
  `helper-unfollowed:<page object file>:<call line>:this.m:inherited-depth` in the
  `page-object-unbound` detail instead of `no-navigation`; a call with no literal argument, and a
  chain that ends without declaring m, add nothing. Failure mode (#1969): 11 of 19 helper calls
  through `Leaf extends Mid extends Base` read `no-navigation`. Non-goals: mixins, `super.m(...)`,
  a class expression as the base. Rollback: drop `inheritedMethods`, `baseClass` and the
  `inherited-depth` gap. Status: experimental under the standing lane auto-accept (V1-3066;
  GitHub #1969).
- `AMAP-V0-394`: A hop of the AMAP-V0-374 walk MUST also follow a base class its file binds by the
  default clause of a static import (`import Mid from '<specifier>'`, alone or before a named
  clause) when the resolved test file's class is its default export (`export default class`),
  through a relative or a bare tsconfig-resolved specifier alike; a default import of a file whose
  class is not its default export, or a name two statements bind by default, is still a hop the
  reader cannot resolve. The default-clause reading is the application map's own and is used only
  by this walk: import `names`, sequence edges and the index's import reading are unchanged. A page object whose direct gotos read `page-object-unresolved-target` MUST carry,
  as that unknown's `detail`, the comma-joined sorted `helper-unfollowed:...` gaps of its helper
  calls (an `inherited-depth` gap included), following the direct-site details required by
  AMAP-V0-450. Failure mode
  (#1980): `Leaf extends Mid`, `import Mid from '../Mid'`, `export default class Mid extends Base`
  read `inherited-depth`, and a leaf of the same shape with an unplaceable direct goto read only
  `page-object-unresolved-target`. Non-goals: default-exported helper functions, `export default
  Mid` after the declaration, re-exports. Rollback: drop `defaultImports`, its use in
  `baseClass` and the unresolved-target detail. Status: experimental under the standing
  lane auto-accept (V1-3083; GitHub #1980).
- `AMAP-V0-450`: (accepted under the owner's standing auto-accept (2026-10-11); V1-3098) A page object with an unplaceable direct goto MUST retain its followed `helper_gotos` as
  remedy candidates, never as a screen binding. Its `page-object-unresolved-target` detail
  MUST name every unplaceable direct site as `<file>:<line>:<reason>`, in direct-site order,
  followed by the sorted helper gaps required by AMAP-V0-394, comma-joined; the detail MUST
  never be empty. A resolved direct site contributes no blocking detail. Failure mode (#1995):
  `Leaf extends Mid extends Base`, with default-exported Mid, a followed `this.openRoute`
  helper and a dynamic direct goto, dropped the helper and emitted an empty detail. Non-goals:
  new helper forms, inferred bindings or changes to injector reach checks. Rollback: revert the
  unresolved-branch retention and direct-site detail. Status: experimental over inline fixtures.
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
  `registration-examined` (other reasons name their hop under AMAP-V0-201). `detail` is an optional
  member of an unknown and absent otherwise, so a
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
  `repeated-draw`, an absent draw file `draw-unavailable`, and a draw nested deeper than the
  AMAP-V0-353 bound `draw-depth-exceeded`. Any other construct, non-literal value or
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
`page-object-ambiguous`, `page-object-unresolved-target`, `import-outside-tests`, `unknown-state`, `unresolved-screen`,
`secret-shaped`, `outside-base-path`, `no-asserting-spec`, `binding-collision`, `stale-reuse`, `reuse-not-callable`, `test-unbound`, `unread-statement`,
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
its states (except a statically placed JSX `Routes` tree, AMAP-V0-190), while JSX quote entities/escapes and unrepresentable optional/partial parameter
patterns remain UNKNOWN. This does not evaluate Ruby custom inflections or JavaScript control flow.

V1-1890 (`actions`) abstention reasons: the V1-1131 Rails reasons above, `unresolved-parent`, `no-handler`,
`handler-file-not-found`, `handler-class-not-found`, `handler-file-unreadable`,
`unresolved-ancestor`, `unresolved-module`, `dynamic-definition`, `ancestor-depth-exceeded` and
`unsupported-ruby-syntax`; not-applicable reasons `no-server-handler`, `mounted-application`,
`redirect` and `rack-endpoint`. AMAP-V0-351..354 add `non-route-statement`, `rails-region-not-read`,
`rails-optional-segments` and `rails-app-root-unresolved`; AMAP-V0-357..358 add the client
reasons `non-literal-resource-action`, `resource-param-may-be-omitted`, `client-instance-shadowed`
and `client-instance-escapes`. V1-1891 (`branches`) adds `no-branch` and `unsupported-ruby-syntax`
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
| AMAP-V0-013 | `TestAMAPV0013Scaffold`, `TestAMAPV0013AliasedImportRebound`, `TestAMAPV0013UnknownSelectorNotReused`, `TestAMAPV0013ReuseWithoutSelector`, `TestAMAPV0013GeneratedBindingCollision`, `TestAMAPV0013StaleReuseNotCalled`, `TestAMAPV0013MethodWithArgumentsNotCalled`, `TestAMAPV0013TestUnbound`, `TestAMAPV0013MethodOutsideClassNotCallable`, `TestAMAPV0013UnreadImportStatement`, `TestAMAPV0013ScaffoldDefaultExportNotNamedImport`, `TestAMAPV0013TwoImportsOnOneLine`, `TestAMAPV0013UnknownSelectorReuseDeclinedInScaffold`, `TestAMAPV0013ParseMapRefusesUnknownClassExport`, `TestAMAPV0013ImportStatementBoundariesAreLexical` |
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
| AMAP-V0-060 | `TestAMAPV0060StatsReconcile`, `TestStatsCLI` |
| AMAP-V0-061 | `TestAMAPV0060StatsReconcile` |
| AMAP-V0-062 | `TestAMAPV0060StatsReconcile` |
| AMAP-V0-063 | `TestAMAPV0060StatsReconcile` |
| AMAP-V0-064 | `TestAMAPV0064StatsSummary`, `TestStatsCLI` |
| AMAP-V0-065 | boundary statement with no executable evidence: the stats report carries no equivalence claim, and adopter-scale qualification is `NOT_RUN` |
| AMAP-V0-070 | `TestAMAPV0070DirectoryStarEnum` |
| AMAP-V0-071 | `TestAMAPV0070DirectoryStarEnum` |
| AMAP-V0-072 | `TestAMAPV0070DirectoryStarEnum`, `TestAMAPV0072FailedHopDoesNotLeakBetweenLookups` |
| AMAP-V0-075 | `TestAMAPV0075WholeRepositoryRoot` |
| AMAP-V0-076 | `TestAMAPV0075WholeRepositoryRoot` |
| AMAP-V0-077 | `TestAMAPV0018SingleRootUnchanged`, `TestAMAPV0019DirtySecondRoot`, `TestAMAPV0075WholeRepositoryRoot` (a `"."` join equals the narrower-root join) |
| AMAP-V0-078 | `TestAMAPV0078ReactSameFileRouteArray` |
| AMAP-V0-079 | `TestAMAPV0079ReactImportedRouteArray` |
| AMAP-V0-190 | `TestAMAPV0190ReactRoutesInHooksComponent` |
| AMAP-V0-191 | `TestAMAPV0191BasePath`, `TestSKHV0_113_BasePathRouterRelativeLinks` |
| AMAP-V0-192 | `TestAMAPV0192RouterMount`, `TestAMAPV0192NestedMount` |
| AMAP-V0-193 | `TestAMAPV0193GuardedReturn` |
| AMAP-V0-194 | `TestAMAPV0194NavigateRedirect` |
| AMAP-V0-195 | `TestAMAPV0195TsconfigPathsResolveTestImports`, `TestAMAPV0195ResolverRefusalAndComputedLoadStayUnknown`, `TestAMAPV0195AliasedWholeRootTsconfig` (aliased repository, `tests.root: "."`, conflicting application tsconfig, import-to-screen join) |
| AMAP-V0-196 | `TestAMAPV0196StarBarrelBindingDetail`, `TestAMAPV0070DirectoryStarEnum`, `TestAMAPV0196StarBarrelReportedScopes` (200 literal members, registering-file and registering-file-plus-directory scopes, declaration/barrel anchors) |
| AMAP-V0-201 | `TestAMAPV0201DirectoryBarrelTwoRepo`, `TestAMAPV0202InjectionRefusalDetail` (import, barrel and binding hops) |
| AMAP-V0-202 | `TestAMAPV0202InjectionRefusalDetail` (router binding, unreadable scope, zero or two registrations, outside the injectable, injected parent, stats example detail) |
| AMAP-V0-203 | `TestAMAPV0203RegisteringImportReach` (paths-alias barrel service, unused import and declared package resolve; a relaying barrel and a side-effect import name `reaches-table`), `TestAMAPV0196StarBarrelBindingDetail`, `TestAMAPV0202InjectionRefusalDetail` (why suffixes) |
| AMAP-V0-204 | `TestAMAPV0204RouterInjectionForms` (typed and generic parameters, shadows, template reads, `as` and `!` resolve; opaque template, asserted write and shadowed read refuse with detail; a tagged template and a duplicate-name annotation refuse) |
| AMAP-V0-205 | `TestAMAPV0205NameFormDetail` (each name form and the router-own `const:` detail) |
| AMAP-V0-206 | `TestAMAPV0206ChainLoaderDetail` (timer callbacks in a registered service's graph resolve; a timer given a value names `reach-module-not-literal:path:line`; a side-effect import's loader names `closure-module-not-literal:path:line`); `TestAMAPV0206TimerCallbackIIFE` (named and generator callbacks resolve; an invoked function expression or one with a return type stays a loader) |
| AMAP-V0-207 | `TestAMAPV0207ConfigArrayAnnotation` (expression, block and function bodies with a trailing comma resolve; arity and position mismatches name their detail) |
| AMAP-V0-208 | `TestAMAPV0208RouterBarrelStar` (one and two levels, a cycle and a diamond resolve with re-export anchors; two declarations, a chain past the bound and a barrel without the name stay unknown); `TestAMAPV0208BarrelCycleIdentity` (a chain back to a barrel under another name is followed and reads ambiguous); `TestAMAPV0208BarrelVisitBudget` (16 duplicate stars per level resolve quickly; 257 distinct barrels read `barrel-visit-limit`) |
| AMAP-V0-209 | `TestAMAPV0209ConstructorStaticMember` (the issue repro and other static members resolve; `.constructor.constructor`, `.call`, `.apply`, `.bind`, a call, a bare value, `?.`, a computed member and a later `.constructor` hop name `reach-module-not-literal:path:line`) |
| AMAP-V0-210 | `TestAMAPV0210UnknownRemedy` (ambiguous candidates sorted with basis lines; an unresolved target, a page object navigating nowhere and a declared missing file state uncertainty; `router-not-declared` names `routers`; a non-literal URL names no key); `TestAMAPV0210ImpactRemedy` (an unmapped path takes the router remedy; every other reason states uncertainty with no key) |
| AMAP-V0-211 | `TestAMAPV0211IncrementalDigestEqualsFullBuild` (over three seeded fixture histories, a map built from every other commit's map has the full build's digest; both carried and rebuilt maps occur); `TestAMAPV0211ReuseDecision` (asset and lockfile edits carry; admitted source, markdown, a test-root asset, a router, an added path and a mode change rebuild; an empty change carries; manifest path, unknown revision and aliased roots rebuild; a bad revision refuses); `TestAMAPV0211BuildFromCLI` (`--from` and `--verify` write the full build's map; `--verify` needs `--from`; a missing MAP refuses) |
| AMAP-V0-215 | `TestAMAPV0215BuilderBindsBinaryRevision` (the identity appends the 12-digit VCS revision and `-dirty`; another revision, a bare version and a dirty identity never carry; a MAP from another revision rebuilds with `builder-version-differs`); `TestAMAPV0215BuilderVersionForcesRebuild` (a full build carries the builder; a MAP with no or another builder parses with its digest and rebuilds with `builder-version-differs`; the same builder carries); `TestAMAPV0211BuildFromCLI` (a rebuilt `--from` names its reason on stderr, a carried one is silent, a MAP without `builder` rebuilds) |
| AMAP-V0-216 | `TestAMAPV0216AliasedRootsIncremental` (unread edits in both roots carry with the pin moved; a read edit in the aliased root rebuilds; a dirty aliased root refuses `appmap-root-dirty`; a missing root refuses; a stated revision against a HEAD pin rebuilds and a stated pin carries) |
| AMAP-V0-217 | `TestAMAPV0217MultiSegmentSubstitution` (a substitution covers one or several segments with its surrounding text fitting; a mismatched prefix or suffix and a surplus segment do not reach) |
| AMAP-V0-218 | `TestAMAPV0218InterceptorsAndGeneratedClients` (axios and `$httpProvider` interceptors and each generator marker are unknown; a marker past 2048 bytes or in a file naming no request API is ignored) |
| AMAP-V0-219 | `TestAMAPV0219OpenAPIYAML` (a block YAML document reads with servers, quoting and comments; each unsupported form refuses with its line) |
| AMAP-V0-220 | `TestAMAPV0220ImpactThroughEndpoints` (a changed controller is placed with basis `endpoint:ID`; an uncalled endpoint, a changed declaration and an incomplete join stay unknown; without inputs the path is `non-ui-path`) |
| AMAP-V0-221 | `TestAMAPV0221OpenAPIPathRefUnknown` (a `$ref` path item refuses the JSON and the YAML document with the path as detail) |
| AMAP-V0-222 | `TestAMAPV0222AliasedRootJoin` (an aliased server joins `--root` calls and screens with `repo`; an aliased caller links to its endpoint as `ALIAS:PATH:LINE`, is `aliased-root-caller-not-placed` and leaves the join incomplete; an aliased unmatched call keeps `repo`; one path read from both roots keeps both calls and places only the `--root` one; a `--root`-only join has no `repo`; unbound, unread and unsafe aliases refuse) |
| AMAP-V0-223 | `TestAMAPV0222ImpactAliasedRoots` (a same-named `--root` path stays `non-ui-path`; a controller with an aliased caller is `endpoint-callers-incomplete` naming `e2e:client/members.js`); `TestAMAPV0223ObligationsAliasedEndpointSource` (obligations reach the `--root` caller's screen and keep a root-qualified map-unknown span for the aliased caller) |
| AMAP-V0-240 | `TestAMAPV0240FlowsOnlyRederive` (a flow edit and a flow plus asset edit re-derive only flows with the full build's digest and the edited outcome; a flow plus spec edit, an added flow and a flow a spec imports rebuild with `changed-input`); `TestAMAPV0240OnlyLoadedIntentsAreFlowEdits` (an `origins.json` that the tests-root tsconfig extends changes its `paths`: the build rebuilds with `changed-input` and equals the full build; fails while `flowScope.edit` admits it); `TestAMAPV0211IncrementalDigestEqualsFullBuild` (seeded histories with carried, re-derived and rebuilt maps all equal the full build) |
| AMAP-V0-245 | `TestAMAPV0245SourceEditRederives` (a router edit with the tests in an aliased root re-derives routers, test joins and flows, equals the full build and drops the home page object's binding; a `react-router/0` route edit re-derives routers and test joins without the index and equals the full build); `TestAMAPV0245SourceEditFailsClosed` (a module gaining a generated header and a source crossing the size bound rebuild with `index-admission-changed`, a JSON-shaped source with `config-shaped-source`, a test source, `package.json` and the manifest with `changed-input`, each equal to the full build); `TestAMAPV0211ReuseDecision` (router, router screen, router state, application module, markdown and non-UI source edits re-derive); `TestAMAPV0211IncrementalDigestEqualsFullBuild` (seeded histories with these edits equal the full build); `TestAMAPV0245AdmissionRulesMatchIndex` (the exported admission rules agree with the built index); `TestAMAPV0211BuildFromCLI` (a CLI source edit names `re-derived only: routers, test-joins` with and without `--verify`; the step help documents AMAP-V0-245 and its reasons) |
| AMAP-V0-340 | `TestAMAPV0340AssetImportReach` (html, css, svg and json imports in default, named and side-effect forms leave the constant literal; an unread script still refuses) |
| AMAP-V0-341 | `TestAMAPV0341ConfigArrayIdentifier` (an arrow and a function named in `.config([...names, F])`, with a trailing comma, resolve; arity, position, a conflicting `$inject` and another use stay unknown with their detail) |
| AMAP-V0-342 | `TestAMAPV0342HelperGotoCandidates` (a page object navigating through an imported helper stays unbound and its remedy offers the learned state; an assigned parameter, a twice-declared name and a two-parameter helper are not helpers; call literals by position) |
| AMAP-V0-343 | `TestAMAPV0343StatsRemedies` (remedies group by id with unknown counts and a keyless count; the report without the flag has no remedies) |
| AMAP-V0-350 | `TestAMAPV0350RefsUnboundTestRoot` (a two-root map's refs over `--root` sources refuses naming the unbound test alias and flag, and reads every test file once bound); `TestAMAPV0097CorpusBounds` (a blob mismatch stays unread; an unbound alias refuses) |
| AMAP-V0-351 | `TestAMAPV0351RailsDrawsAndRoot` (a library `require` and a logged `begin`/`rescue` are per-statement `non-route-statement` unknowns while the routes are read); `TestAMAPV0352RailsRegionsAndOptionals` (`require_relative` stays unsupported; an `if` region declaring a route is `rails-region-not-read`; an assignment, argument or condition calling a bare helper or passing `self` is not skipped); `TestAMAPV0220ImpactThroughEndpoints` (a route region at head makes the comparison partial) |
| AMAP-V0-352 | `TestAMAPV0351RailsDrawsAndRoot` (a namespace with a regex constraint hash compiles; a `(:locale)` scope declares both paths); `TestAMAPV0352RailsRegionsAndOptionals` (nested optional scopes are `rails-optional-segments` with the segment-present path) |
| AMAP-V0-353 | `TestAMAPV0351RailsDrawsAndRoot` (a nested `draw "api/items"` keeps the namespace; listed drawn files are read once); `TestAMAPV0105ChainDoubtAndDraws` (nested draws are followed) |
| AMAP-V0-354 | `TestAMAPV0351RailsDrawsAndRoot` (controllers resolve under the root holding `config/application.rb`, also for a drawn file listed alone) |
| AMAP-V0-355 | `TestAMAPV0220ImpactThroughEndpoints` (a comment-only routes edit places nothing; a swapped route places the removed endpoint's screens and states the uncalled added one; a partial comparison keeps `endpoint-declaration-changed`; a changed constraint and a swap of overlapping endpoints place their screens, a swap of disjoint ones places nothing; an added and a deleted router are compared, an absent unchanged one refuses) |
| AMAP-V0-356 | `TestAMAPV0356ClientsJSX` (`.tsx`/`.jsx` text with an apostrophe, a call in an `onClick` container and `<T,>` and `<T = unknown>` arrows; a `.ts` assertion; an unbounded element is `source-not-lexed`) |
| AMAP-V0-357 | `TestAMAPV0357Resource` (default and declared actions with and without the trailing parameter; a non-literal action, a mid-path parameter and a non-literal URL are unknown) |
| AMAP-V0-358 | `TestAMAPV0358AxiosInstances` (a literal and an absent baseURL prefix calls; non-literal, reassigned, exported and unbound creates, a `method` or `url` config key, a chained initializer and a per-call baseURL are stated) |
| AMAP-V0-359 | `TestAMAPV0359UnresolvedScreenIsNotUnknownState` (a declared state present with status UNKNOWN reads `unresolved-screen` with the screen's reason and a no-change remedy; an absent state keeps `unknown-state` with the correct-the-state remedy) |
| AMAP-V0-365 | `TestAMAPV0365ClassFieldTypes` (the issue's model class and typed, optional and modified fields stay literal; an object literal, an initializer and a method parameter still refuse naming `Function`) |
| AMAP-V0-366 | `TestAMAPV0366RailsScreensSkipStatements`, `TestAMAPV0366RailsEndpointsIssueFile`, `TestAMAPV0366RailsRegionsFailClosed` (a control transfer or an unplaceable region end never yields a certain route; the issue's routes file yields screens and endpoints with per-statement notes; a constrained namespace is unknown at its line) |
| AMAP-V0-367 | `TestAMAPV0367HelperForms`, `TestAMAPV0367HelperFormsInMap` (template and URL-hash helpers and their candidates; a hash the prefix does not start places no screen; unfollowed forms are named in the unbound detail) |
| AMAP-V0-368 | `TestAMAPV0368AddedPathRederives` (added Ruby and asset files re-derive with the full build's digest; test-root, web, JSON, named and config-shaped additions and a bare unresolved import rebuild) |
| AMAP-V0-369 | `TestAMAPV0369ConstructorTests`, `TestAMAPV0369ClosureReach`, `TestAMAPV0369ClosureLimitManifest` (tested and argument-free constructed `.constructor` reads resolve, other uses refuse naming it; a 300-module graph that cannot reach the table resolves, one that can hits the limit with a `di_closure_limit` remedy and resolves once raised; out-of-range values refuse) |
| AMAP-V0-370 | `TestAMAPV0370HelperDefaults`, `TestAMAPV0370HelperHopsInMap` (literal defaults keep a helper, other defaults are a named gap; under `base_path` a template and a URL-hash candidate place their screens; an inherited `this.openRoute` with a template argument places a parameterised state; unbound details are never empty); `TestAMAPV0370HelperReviewHoles` (a page outside `base_path` and a twice-declared method place nothing) |
| AMAP-V0-371 | `TestAMAPV0371RailsBlockParameters`, `TestAMAPV0371RailsBlockFailClosed` (block-parameter loops and `direct` blocks are unread regions in both readers; a locale-constrained optional scope reads its drawn file; a control transfer, a one-line block and a stray `end` refuse at their line; a regexp naming no segment stays non-literal) |
| AMAP-V0-373 | `TestAMAPV0373QuietDecoratorsAndSubstitutions` (the #1968 decorator metadata and call-bearing substitution read the constant; a call's array, an array after the decorated class, a member decorator's array, and substitutions naming the constant or the injector refuse with line and construct); `TestAMAPV0025ChainBindingsFailClosed` (a substitution building the name by concatenation still refuses) |
| AMAP-V0-384 | `TestAMAPV0384QuietConstListsAndDataArrays` (the #1979 route data array `[Role.ADMIN, 'owner']` reads the constant; a plain-ended array passed to a call or held by a binding and mutated refuses) |
| AMAP-V0-385 | `TestAMAPV0384QuietConstListsAndDataArrays` (the #1979 const lists read by decorator metadata and spreads read the constant; a const ending in a function or arrow, passed or spread to a call, extended by `.concat`, named in a template, or bound by `let` refuses; unconsumed exports are now admitted by AMAP-V0-420) |
| AMAP-V0-420 | `TestAMAPV0420ArrayInjectorReach`, `TestAMAPV0420ArrayReachRefusals`, `TestAMAPV0420ReachMemoTracksClosure`, `TestAMAPV0025ChainBindingsFailClosed` (five reported shapes, minimal repro, scalar helper calls, canonical RxJS projection, closure-specific proof memoization, and real/escaped/unknown injector paths) |
| AMAP-V0-421 | `TestAMAPV0420ArrayInjectorReach`, `TestAMAPV0420ArrayReachRefusals` (different exact service and unwritten const; same/dynamic service, written result and escaped injector refuse) |
| AMAP-V0-422 | `TestAMAPV0422AllClosureBlockers`, `TestAMAPV0373QuietDecoratorsAndSubstitutions` (all files, arrays, annotations, injector tokens and repeated template occurrences retained) |
| AMAP-V0-423 | `TestAMAPV0423StaticInjectField`, `TestAMAPV0423StaticInjectRefusals` (reported class field, typed/readonly/default/two-class/mixed forms quiet; constant-naming, computed, spread, substitution, duplicate, later-written, pushed, instance, expression, nested and continued lists block) |
| AMAP-V0-424 | `TestAMAPV0424ElementWiseArrayUses`, `TestAMAPV0424ElementWiseRefusals` (minimal repro, the five reported forms and scalar element uses quiet; injector calls, escaping copies, elements pushed/stored/passed/returned from loops, callbacks and `find`/`shift` results, `map` rebuilds, derived arrays, escaping or non-`[]` accumulators, typed sinks, spread-prefixed, `this`-aliased, computed or reflective method definitions and other unprovable method arguments block) |
| AMAP-V0-425 | `TestAMAPV0425ParameterPatternsAndMethodConsts`, `TestAMAPV0427ReportedFormsStayRefused` (entries `([, v])`, nested, `function` and `async` parameter patterns quiet; default, typed, rest and parenthesized-argument patterns block) |
| AMAP-V0-426 | `TestAMAPV0425ParameterPatternsAndMethodConsts`, `TestAMAPV0427ReportedFormsStayRefused`, `TestAMAPV0424ElementWiseRefusals` (typed `Kind[]` const passed to a proved method quiet; escaping parameter, injector-facing name, other receiver and a parameter spread into a returned plain-ended literal block; computed and reflective member writes still block) |
| AMAP-V0-427 | `TestAMAPV0427ReportedFormsStayRefused` (the seven unproved #2020 forms, provider `deps`, computed `$inject` and method-decorator arrays keep `computed-annotation`) |
| AMAP-V0-650 | `TestAMAPV0650PlainFunctionSpreadRebuildBlocks` (function declaration, unexported, arrow body, arrow block, function expression, typed return, typed arrow return, later parameter, nested arrow and parameter-member spreads ending in a plain element and popped by a caller block; a spread-free literal and a module-const spread stay quiet) |
| AMAP-V0-374 | `TestAMAPV0374InheritedChain` (Leaf -> Mid -> Base places the helper's state; a cycle, an unresolved package base and a chain one hop past the bound are `inherited-depth` gaps) |
| AMAP-V0-394 | `TestAMAPV0394DefaultImportedMiddleClass`, `TestAMAPV0394DefaultImportBindings` (a default-exported, default-imported middle class places the helper's state through relative, bare and mixed-clause imports; a default import of a named-only class is `inherited-depth`, also in the `page-object-unresolved-target` detail; the default-clause reading skips named, namespace, type-only, dynamic and twice-bound imports) |
| AMAP-V0-450 | `TestAMAPV0450UnresolvedTargetKeepsHelperGotos` (Base/Mid/Leaf retains its inherited helper candidate without binding; every unplaceable direct site has a nonempty file/line/reason detail, excluding resolved sites); `TestAMAPV0394DefaultImportedMiddleClass` (direct-site details retain an unfollowed helper gap) |
| AMAP-V0-372 | `TestAMAPV0372AliasedTestsAddedPath` (with `tests.repo` and `tests.root "."`, added server files re-derive with the full build's digest despite bare test imports; an added web module and a test-repository addition rebuild) |
| AMAP-V0-212 | `TestAMAPV0212EndpointJoin` (Rails and OpenAPI endpoints with handler, controller file and operationId; a YAML document and an opaque route are unknown; bad queries refuse); `TestAMAPV0214Reaches` (OpenAPI basePath, differing and templated servers, unreadable and non-template documents); `TestAMAPV0212EndpointsCLI` (the verb writes the schema; a missing --map and no declaration refuse) |
| AMAP-V0-213 | `TestAMAPV0213CallSites` (each recognised $http, axios and fetch spelling with its verb and URL, bindings ignored, and every unread call or reference with its reason); `TestAMAPV0212EndpointJoin` (absolute, non-literal and unmatched calls in the fixture) |
| AMAP-V0-214 | `TestAMAPV0214Reaches` (verb, parameter, substitution, splat, HEAD and `.format` matching); `TestAMAPV0212EndpointJoin` (endpoints placed on the screens their calls reach through a router anchor and the Angular DI reference hop; an unreferenced file is a placement unknown) |
| AMAP-V0-200 | `TestAMAPV0200StatedAliasRevision` (stated pin; HEAD pin unstated; a later commit and a dirty file leave the bytes identical; a non-commit REV and an unread alias refuse `appmap-root-unavailable`) |

Implementation: `internal/appmap`, `internal/contextindex/webimport_api.go`,
`cmd/corvint/flows_appmap.go`, `internal/rootalias` (AMAP-V0-017). Build logs:
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-07-application-map.md`, `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-07-appmap-second-root.md`,
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-08-appmap-injected-state-names.md` (AMAP-V0-021..023,
`internal/appmap/stateinject.go`), `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-09-v1-1061-appmap-enum-barrel.md`
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
