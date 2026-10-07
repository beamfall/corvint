# Application Map V0 — revision-pinned screen graph and capped projections

Owner: Russell Lewis
Date: 2026-10-07
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner request [issue 657](https://github.com/beamfall/corvint/issues/657)
(native ticket V1-0956), `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/application-flow-understanding-v1.md` (AFU-V1 flow intents and navigation blocks),
`docs/specs/documentation-corpus-v1.md`, and the orchestrator notes of 2026-10-07 on
[issue 659](https://github.com/beamfall/corvint/issues/659) (V1-0958, TypeScript path aliases),
[issue 658](https://github.com/beamfall/corvint/issues/658) (V1-0957, run-verified steps) and
[issue 660](https://github.com/beamfall/corvint/issues/660) (V1-0959, scenario planner).

## Agent digest
- Claim: A revision-pinned screen graph joins routes, flows and E2E tests through imports, served as byte-capped projections that read STALE or UNKNOWN.
- Status: proposed (pending owner acceptance; V1-0956); experimental. AMAP-V0-001..015 are implemented in `internal/appmap` and `corvint flows appmap` over a committed fixture; no adopter-scale qualification.
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

Every requirement below is (proposed, pending owner acceptance; V1-0956).

- `AMAP-V0-001`: The map MUST be compiled from a closed `application-map-manifest/0` document
  (at most 256 KiB) read from Git at the evaluated revision: `app` matching
  `^[a-z0-9][a-z0-9._-]{0,63}$`, `hash_prefix` one of empty, `#` or `#!`, 1..64 `routers` each
  with a known `dialect`, optional `flows` intent directory, a `tests` layout whose `specs`,
  `page_objects`, `workflows` and `scenarios` directories lie under `tests.root`, and optional
  `page_object_screens` bindings. An unknown member, escaping or absent path, a declared test or
  flow directory that is not a directory at the revision, or an unknown dialect MUST refuse with
  `appmap-invalid-manifest`; an unresolvable revision with
  `appmap-invalid-revision`; an invalid flow intent with `appmap-invalid-flows`. (proposed, pending owner acceptance; V1-0956)
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
  `non-literal-name`). (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-003`: Screens MUST be keyed by app plus template. Two resolved screens with the same
  collision key MUST be reported `ambiguous-template`, and a lookup (by element ID, state name,
  template or URL) that matches several screens MUST return status `UNKNOWN` with the candidates,
  never pick one; a lookup that matches none returns `no-matching-screen`. URL lookup strips the
  manifest's hash prefix, removes the authority only after a leading scheme (`https://host`), so a
  `://` inside a query or fragment is data, and refuses a `${...}` substitution inside a segment
  (`partial-segment-substitution`). (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-004`: Each AFU-V1 navigation step MUST become a map step on the unique screen whose
  template matches its state template, with its selector and strength and the page-object methods
  that already use the same selector (`reuse`). A step without a navigation entry reads
  `no-navigation-step`; one whose state matches zero or several screens reads `UNKNOWN` with that
  reason. Flow preconditions and precondition flows are attached to every screen a resolved step
  lands on.
  (proposed, pending owner acceptance; V1-0956)
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
  The map MUST NOT implement alias resolution itself. (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-006`: Each screen MUST aggregate permissions and flags (inherited from the nearest
  declaring ancestor, with `permissions_from` / `flags_from`), flow preconditions, page objects and
  their methods, workflows, scenarios, specs (with basis and import chain) and flow steps, each
  with its anchor. (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-007`: Selectors MUST carry a kind and strength: `getByTestId` and a `data-testid`
  attribute locator are `strong` (`data-test` and `data-test-id` locators are CSS); `getByRole`, `getByLabel`, `getByPlaceholder`, `getByAltText`
  and `getByTitle` are `medium`; `getByText` and other CSS locators are `weak`; a non-literal
  argument is `unknown`. A literal counts only as a whole argument (followed by `)` or `,`), so
  `'save-' + id` is non-literal, as is such a `goto` URL (`non-literal-url`). String and template
  escapes decode to the program's value (`\n`, `\xHH`, `\uHHHH`, `\u{H}`, surrogate pairs, line
  continuations); a legacy octal escape, lone surrogate, or short or non-hex `\u` escape makes the
  literal non-literal, and a failed escape never consumes past the end of the source. A
  `getByRole` name is read from the options object's top-level `name` member in any position, and
  the selector is `unknown` when the object has a spread, a computed or shorthand key, a repeated
  `name`, or a non-literal name, and when the options are passed by reference
  (`getByRole('button', opts)`) or the literal is only part of the argument (`{ name } && opts`),
  since any of these can set a name the map cannot read. Line
  continuations inside a string advance the line count of later anchors. A
  secret-shaped literal is dropped and reported `secret-shaped`.
  (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-008`: Edges MUST come only from evidence: `flow-step` edges between adjacent
  resolved navigation steps (a step that is not placed on one screen breaks the chain, so no edge
  is inferred across it), and `test-sequence` edges between page objects of different screens
  constructed in order within one spec or workflow, each citing its source element and span. No
  edge is inferred from router hierarchy alone. (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-009`: The map MUST be deterministic for one revision: sorted nodes, edges, files and
  unknowns, and a `digest` (SHA-256 over the encoding with an empty digest). Loading a map file
  MUST refuse a symlink, a file over 64 MiB, an unknown member, trailing data, a wrong schema or a
  digest mismatch with `appmap-invalid-map`. (proposed, pending owner acceptance; V1-0956)
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
  (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-011`: `screen`, `flow`, `find` and `scaffold` projections MUST each return one JSON
  document no larger than its budget: defaults 4096, 6144, 2048 and 6144 bytes; `--budget` in
  256..65536; `--full` raises the ceiling to 1 MiB and is exclusive with `--budget`
  (`appmap-invalid-query`). Lists are trimmed from the tail, uniformly first and then greedily in
  section order, and an `omitted` object reports the dropped count of every list. A budget that
  cannot hold the document head refuses with `appmap-budget-too-small`. Each head carries
  `schema`, `app`, `map_revision`, `map_digest`, `evaluated_revision`, `budget` and `full`, so a
  later planner (V1-0959) can compose projections without re-reading the map. `flow` reports the
  unknowns of the flow, its steps and every screen it prints.
  (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-012`: `find` MUST match a 2..128 byte query case-insensitively against element IDs and
  labels of screens, flows, steps, files, methods and selectors (every element's ID, not only its
  label), returning references without
  freshness evaluation (`evaluated_revision: NOT_EVALUATED`); other lengths refuse with
  `appmap-invalid-query`. (proposed, pending owner acceptance; V1-0956)
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
  method is called only when it is a public instance method whose header literally declares no
  parameters (`callable`: no getter, setter, static, private or protected member, no module
  function); otherwise the step keeps a note and a TODO and reports `scaffold-reuse` /
  `reuse-not-callable`. Borrowed import statements are kept one per statement, each with exactly
  the names it binds. When no borrowed, resolvable import binds `test`, the scaffold emits
  `// UNRESOLVED import { test }` and reports `scaffold-import` / `test-unbound`. With no
  eligible spec, `closest` reads `UNKNOWN no-asserting-spec`. It never writes the file.
  (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-014`: Learned facts MUST attach only through the overlay seam: an `Overlay` is asked once
  per projection for the element IDs that projection selects, its facts are kept only for those IDs
  and, after byte trimming, only while the element's ID is still printed elsewhere in the
  document (a dropped fact counts in `omitted.learned`),
  only when `text` is at most 1024 bytes and not secret-shaped, capped at 4096 facts
  (`overlay-bound-exceeded`), printed in a separate `learned` section with `authority: learned`
  and `freshness: STALE` when their element's anchor is stale. An overlay error is reported as
  `overlay-unavailable`. Overlay facts MUST NOT change any node, edge, strength, join or
  freshness. (proposed, pending owner acceptance; V1-0956)
- `AMAP-V0-015`: Building and projecting MUST NOT write to the repository, the index, or any
  ledger; they read Git objects only, with no network, database or background process. Inputs are
  bounded (64 router files, 4 MiB per router, 20000 states and test files, 1 MiB per test file,
  128 MiB of test source, 64 MiB map) and a breach refuses with `appmap-bound-exceeded`.
  (proposed, pending owner acceptance; V1-0956)

## Wire contract

`corvint flows appmap build --manifest FILE [--revision REV]` writes `application-map/0` to stdout.
`corvint flows appmap screen|flow|find|scaffold --map FILE --screen|--flow|--text VALUE
[--budget N | --full] [--revision REV]` writes `application-map-screen/0`,
`application-map-flow/0`, `application-map-find/0` or `application-map-scaffold/0`. Refusals exit 2
with one coded JSON error on stderr and nothing on stdout.

### Owned error codes

| Code | Meaning |
| --- | --- |
| `appmap-invalid-manifest` | The manifest is absent, not closed, over its bound, or names an escaping path, unknown dialect, directory outside `tests.root`, or declared directory absent at the revision. |
| `appmap-invalid-revision` | The build revision does not resolve to a commit. |
| `appmap-invalid-flows` | A flow intent under the manifest's `flows` directory does not decode. |
| `appmap-invalid-map` | A map file is not a regular file, is over 64 MiB, is not closed, has trailing data, a wrong schema or a digest mismatch. |
| `appmap-invalid-query` | `--budget` is out of range or combined with `--full`, or a `find` query is not 2..128 bytes. |
| `appmap-budget-too-small` | The budget cannot hold the projection head. |
| `appmap-bound-exceeded` | An input bound of AMAP-V0-015 was exceeded. |

### Unknown reasons

`non-literal-name`, `non-literal-url`, `non-literal-value`, `duplicate-state`, `missing-parent`,
`unresolved-parent`, `parent-cycle`, `ambiguous-template`, `no-matching-screen`,
`no-matching-flow`, `partial-segment-substitution`, `no-navigation-step`, `unresolved-import`,
`import-depth-exceeded`, `unparsed-imports`, `excluded-by-index`, `page-object-unbound`,
`page-object-ambiguous`, `page-object-unresolved-target`, `import-outside-tests`, `unknown-state`,
`secret-shaped`, `no-asserting-spec`, `binding-collision`, `stale-reuse`, `reuse-not-callable`, `test-unbound`,
`overlay-unavailable` and `overlay-bound-exceeded`. Unknown kinds are `state`, `screen`,
`screen-permissions`, `screen-flags`, `template`, `step`, `file`, `import`, `test-join`,
`page-object`, `selector`, `scaffold`, `scaffold-import`, `scaffold-reuse` and `overlay`.

## Overlay seam

`internal/appmap/overlay.go` defines `Fact{element_id, source, kind, revision, text, authority}`,
`AuthorityLearned = "learned"` and `Overlay.Facts(ctx, elementIDs) ([]Fact, error)`; callers pass
overlays in `Options.Overlays`. Element IDs are the stable IDs of Definitions, so a fact recorded
against `step:book-tee-time/select-slot` or `method:e2e/pages/teesheet.page.ts#selectSlot` finds
its element in any later map where that element still exists. Issue 658 (V1-0957, run-verified
steps) is specified separately in `docs/specs/run-verified-navigation-v0.md` (RVN-V0): it adds an
optional `tests` array to map steps (declared AFU-V1 `test` links) and a typed per-step
`verification` field rather than an overlay fact.

## Non-goals and simpler baseline

- No router dialect other than `ui-router-states/0`; React Router, Vue Router and Angular Router are
  follow-ups. No Cypress `cy.visit` or Playwright fixture-injected page objects.
- No TypeScript `paths`/`baseUrl` alias resolution (V1-0958 owns it).
- No MCP tools, documentation-corpus records or persisted index; the map is an explicit file.
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
  a closest spec that calls `test` under another name (`test-unbound`).
- Limits: per-method and per-file anchors, not per-statement; flow steps cite their intent file;
  `test_join` is global, not per screen.

## Deterministic acceptance and traceability

| Requirement | Evidence |
| --- | --- |
| AMAP-V0-001 | `TestAMAPV0015ReadOnlyAndRefusals`, `TestAMAPV0001DeclaredDirectoriesExist` |
| AMAP-V0-002 | `TestAMAPV0002HierarchyResolution`, `TestAMAPV0002WildcardExhaustedSubject`, `TestAMAPV0002RepeatedKeys`, `TestAMAPV0002ConfigMustBeWholeArgument` |
| AMAP-V0-003 | `TestAMAPV0002HierarchyResolution`, `TestAMAPV0003QueryURLIsNotAuthority` |
| AMAP-V0-004 | `TestAMAPV0004FlowSteps` |
| AMAP-V0-005 | `TestAMAPV0005ImportGraphJoin`, `TestAMAPV0005UnreadSpecKeepsJoinUnknown`, `TestAMAPV0005UnboundPageObjectKeepsJoinUnknown`, `TestAMAPV0005UnresolvedTargetBlocksBinding`, `TestAMAPV0005ImportOutsideTests`, `TestAMAPV0005SeparateImportsFromOneModule`, `TestAMAPV0007PartialLiterals` |
| AMAP-V0-006 | `TestAMAPV0002HierarchyResolution`, `TestAMAPV0004FlowSteps` |
| AMAP-V0-007 | `TestAMAPV0007SelectorStrength`, `TestAMAPV0007PartialLiterals`, `TestAMAPV0007EscapesAndSpreads`, `TestAMAPV0007RegexReferencedOptionsAndContinuations`, `TestAMAPV0007ShortUnicodeEscape`, `TestAMAPV0007TestIDAttributeIsExact`, `TestAMAPV0007RoleOptionsWholeArgument` |
| AMAP-V0-008 | `TestAMAPV0005ImportGraphJoin`, `TestAMAPV0004FlowSteps` |
| AMAP-V0-009 | `TestAMAPV0009DeterministicArtifact`, `TestAMAPV0FlowsAppmapCLI` |
| AMAP-V0-010 | `TestAMAPV0010StaleAnchors`, `TestAMAPV0010SameRevisionVerified`, `TestAMAPV0010AncestorLineageStale`, `TestAMAPV0010ChainFreshness`, `TestAMAPV0010ManifestBindingFreshness` |
| AMAP-V0-011 | `TestAMAPV0011ProjectionBudgets`, `TestAMAPV0011EveryBudgetFits`, `TestAMAPV0011FlowReportsScreenUnknowns`, `TestAMAPV0FlowsAppmapCLI` |
| AMAP-V0-012 | `TestAMAPV0012Find`, `TestAMAPV0012FindByID` |
| AMAP-V0-013 | `TestAMAPV0013Scaffold`, `TestAMAPV0013AliasedImportRebound`, `TestAMAPV0013UnknownSelectorNotReused`, `TestAMAPV0013ReuseWithoutSelector`, `TestAMAPV0013GeneratedBindingCollision`, `TestAMAPV0013StaleReuseNotCalled`, `TestAMAPV0013MethodWithArgumentsNotCalled`, `TestAMAPV0013TestUnbound` |
| AMAP-V0-014 | `TestAMAPV0014OverlaySeam`, `TestAMAPV0014FactsFollowTrimmedElements` |
| AMAP-V0-015 | `TestAMAPV0015ReadOnlyAndRefusals`, `TestAMAPV0FlowsAppmapCLI` |

Implementation: `internal/appmap`, `internal/contextindex/webimport_api.go`,
`cmd/corvint/flows_appmap.go`. Build log: `docs/build-log/2026-10-07-application-map.md`.

## Rollout, rollback and compatibility

The slice is additive and experimental: a new package, one exported read-only contextindex
wrapper and one `flows` subcommand. Rollback removes `internal/appmap`,
`cmd/corvint/flows_appmap.go`, `internal/contextindex/webimport_api.go`, the `flowsAppmapHelp`
reference in `cmd/corvint/help.go` and this spec's index rows; no stored state, schema or ledger
needs migration. Map files are explicit outputs and may be discarded.

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

## Owner questions

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
