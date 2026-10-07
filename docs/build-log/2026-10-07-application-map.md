# 2026-10-07: application map — revision-pinned screen graph and capped projections (V1-0956)

## Intent

[Issue 657](https://github.com/beamfall/corvint/issues/657) (native V1-0956) asks for an application
map: a screen graph compiled from router definitions, joined to the E2E suite through the import
graph rather than by URL, pinned to the revision it was read at, and served through capped
projections plus a spec scaffold, with an overlay seam for learned facts. This entry records the
smallest vertical slice: the proposed contract `docs/specs/application-map-v0.md` (AMAP-V0-001..015,
each pending owner acceptance), the compiler and projections in `internal/appmap`, a committed
fixture application, and `corvint flows appmap build|screen|flow|find|scaffold`.

Governing spec: a new AMAP-V0 spec, because no existing spec models router hierarchy, page-object
binding or projection byte caps. AFU-V1 owns flow intents and navigation blocks, which the map
consumes unchanged; DCP-V1 owns corpus records, which are a follow-up. Whether to fold AMAP-V0 into
either is an owner question.

## Decisions

- **Fail closed everywhere a route is not literal.** Non-literal names, URLs and data, duplicate or
  orphaned states, and parent cycles make the state and its descendants `UNKNOWN`. Two screens with
  the same template collision key read `ambiguous-template`, and lookups return candidates rather
  than choosing; static-segment precedence is left as an owner question.
- **Import-graph join, no second resolver.** The map consumes the contextindex web import relation
  through a thin exported wrapper (`internal/contextindex/webimport_api.go`) that delegates to the
  index's own resolution. Alias resolution is not implemented here: per the orchestrator note,
  [issue 659](https://github.com/beamfall/corvint/issues/659) (V1-0958) owns TypeScript
  `baseUrl`/`paths`. Until it lands, an alias import that the index cannot resolve leaves that spec's
  join `UNKNOWN` (fixture `e2e/specs/alias.spec.ts`) and every screen projection's `test_join` reads
  `UNKNOWN`. Because the wrapper delegates, V1-0958's resolution reaches the map without a change
  here; re-testing is a follow-up.
- **Page-object binding** prefers the manifest's `page_object_screens`, then exactly one literal
  navigation target; zero or several targets are reported, never picked.
- **Edges only from evidence.** `flow-step` edges join adjacent resolved navigation steps; an
  unplaced step breaks the chain (found while writing the spec: the first draft linked across an
  unresolved middle step; `TestAMAPV0004FlowSteps` now fails without the fix). `test-sequence` edges
  follow page-object constructions in one spec or workflow.
- **STALE is fail-closed.** Freshness compares blob IDs, then the span digest at the same lines; a
  line shift without a content change reads `STALE`. One tree read plus bounded blob reads per
  projection; no background work.
- **Byte caps are enforced by construction.** `render` sizes the document exactly, binary-searches a
  uniform keep count, extends greedily in section order, and reports per-section omitted counts.
  The CLI test exposed an off-by-one in the per-section size (each `"key":[...],` costs key+6, not
  key+5), which let a projection overrun its budget by a few bytes and hit the internal guard;
  `TestAMAPV0011EveryBudgetFits` sweeps budgets from 256 to 8192 and fails without the fix.
- **Composability for issue 660 (V1-0959).** Each projection is a pure function of (map, query,
  options) whose head carries `map_digest` and `evaluated_revision`, so a planner can call several
  and check they came from the same map.
- **Overlay seam for issue 658 (V1-0957).** `internal/appmap/overlay.go` defines `Fact`, `Overlay`
  and `AuthorityLearned`; facts are keyed by stable element IDs, filtered to printed elements,
  bounded (1024 bytes, 4096 facts), secret-screened, forced to `authority: learned`, labelled
  `STALE` with their element, and never change nodes, edges, strength, join or freshness. No
  verification is implemented.

## Limits

- One router dialect (`ui-router-states/0`) and Playwright-style page objects with named imports;
  Cypress, fixture-injected page objects and default-import borrowing are follow-ups.
- Anchors are per method or per file, not per statement; flow steps cite their intent file.
- `test_join` is global, not per screen.
- Fixture-scale only: no adopter-scale timing, size or latency qualification.
- No MCP tools or corpus records; the map is an explicit file.

## Evidence

- `GOTOOLCHAIN=local go test -count=1 ./internal/appmap/` passes (AMAP-V0 tests listed in the
  spec's traceability table).
- `TestAMAPV0FlowsAppmapCLI` in `cmd/corvint` passes; `internal/specindex` passes;
  `internal/contextindex` passes after the analyzer schema bump.
- gofmt and go vet are clean on the changed packages.
- Remaining focused-test, gate and review results are recorded in the lane handoff.

## Dogfood context used

- Lane start: `corvint affected --base 8af2bf622eaae539f3b16205ea7a2d5f45a391ed` (Corvint
  1.0.0-rc.2 build 360) returned `PLAN_ONLY`, an empty Go selection for the empty range, and
  static language-frontier unknowns; it was re-run on the committed change for test selection.
- `corvint context --task` for the issue text returned 20 of 29 candidates with answerability
  `no-specific-terms`. The useful hits were `docs/specs/application-flow-understanding-v1.md`,
  `docs/specs/documentation-corpus-v1.md` and `docs/specs/INDEX.json`; most of the rest were
  unrelated Python and extension test files. Retained as friction for the handoff.
- No CEM was bound or sealed in this lane (orchestrator instruction): `NOT_PRODUCED`.

## NOT_RUN

- `make gate` and the exhaustive `go test ./...` (owner standing preference for scoped work).
- Adopter-scale qualification and any live browser run.

## Codex review

Round 1 (`codex exec -m gpt-6-astra -s read-only`, diff `8af2bf62..c5609cfb`) reported nine P2
findings. Each was confirmed against the source and repaired with a regression test that fails
without the fix (checked by reverting each fix in turn):

1. `wildcardMatch` sliced an exhausted subject and panicked (`/foo{a}-{b}` against `/foo`) —
   `TestAMAPV0002WildcardExhaustedSubject`.
2. A literal that only starts an argument (`'save-' + id`, `'/home' + suffix`) was read as the
   literal — `TestAMAPV0007PartialLiterals`.
3. A spec the index excluded or could not read had no `TestFile`, so `test_join` read `RESOLVED`
   — `TestAMAPV0005UnreadSpecKeepsJoinUnknown`. `test_join` now also reads `UNKNOWN` for any
   non-resolved page object or workflow join.
4. A declared test or flow directory absent at the revision was accepted and silently contributed
   nothing — `TestAMAPV0001DeclaredDirectoriesExist` (one `git cat-file --batch-check`).
5. At the map's own revision every anchor read `FRESH` unchecked, so a hand-edited map could claim
   any blob — `TestAMAPV0010SameRevisionVerified`. Writing it exposed a second gap: freshness kept
   one pinned blob per path, so an anchor disagreeing with its siblings read `UNKNOWN`; every
   pinned blob is now compared.
6. The scaffold called `new HomePage` when the closest spec imported `{ HomePage as Home }` —
   `TestAMAPV0013AliasedImportRebound` (an own-name import is added; a name clash reads
   `binding-collision`).
7. A `getByRole` name was read only as the first option, and unknown-strength selectors (all with
   an empty value) shared one ID and could be offered as reuse —
   `TestAMAPV0007PartialLiterals`, `TestAMAPV0013UnknownSelectorNotReused`.
8. `find` did not match method, file or selector IDs — `TestAMAPV0012FindByID`.
9. A learned fact survived byte trimming of its element — `TestAMAPV0014FactsFollowTrimmedElements`
   (render drops an annotation whose element ID is no longer printed and counts it as omitted).

AMAP-V0-001, 005, 007, 010, 012, 013 and 014 were amended in the same change.

Round 2 (diff `8af2bf62..b1246f16`) confirmed the round-1 repairs and reported seven P2 findings,
each confirmed and repaired with a regression test that fails without its fix:

1. A spec reaching a screen only through an unbound or ambiguous page object kept `test_join`
   `RESOLVED` — `TestAMAPV0005UnboundPageObjectKeepsJoinUnknown`.
2. A screen's template, permissions and flags derive from ancestor states whose anchors were not
   checked — `TestAMAPV0010AncestorLineageStale` (`lineage_freshness` in `screen` and `flow`).
3. `flow` dropped the unknowns of the screens it prints (for example `screen-flags`) —
   `TestAMAPV0011FlowReportsScreenUnknowns`.
4. A spread or repeated key after a `getByRole` name (`{ name: 'Book', ...options }`) could
   override it — `TestAMAPV0007EscapesAndSpreads` (the selector reads `unknown`).
5. Two reused page objects exporting one class name produced two imports of the same binding —
   `TestAMAPV0013GeneratedBindingCollision` (every bound name is reserved for one file).
6. Escapes were decoded by dropping the backslash (`'\u002d'` read as `u002d`) —
   `TestAMAPV0007EscapesAndSpreads` (exact decoding; legacy octal and lone surrogates are
   non-literal).
7. A map file claiming reuse for a step with no selector crashed `scaffold` —
   `TestAMAPV0013ReuseWithoutSelector`.

AMAP-V0-005, 007, 010, 011 and 013 were amended in the same change.

Round 3 (diff `8af2bf62..43c65094`) reported eight findings, each confirmed and repaired with a
regression test that a mutation of its fix makes fail:

1. `scaffold` called a reused page-object method whose anchor was `STALE` at the evaluated
   revision — `TestAMAPV0013StaleReuseNotCalled` (no call; TODO, `freshness: STALE`, `stale-reuse`).
2. `getByRole('button', opts)` read as a role with no name although `opts` may carry one —
   `TestAMAPV0007RegexReferencedOptionsAndContinuations` (the selector reads `unknown`).
3. A regular-expression literal was a literal string value —
   `TestAMAPV0007RegexReferencedOptionsAndContinuations` (now non-literal).
4. A page object with one literal and one non-literal target bound to the literal one —
   `TestAMAPV0005UnresolvedTargetBlocksBinding` (`page-object-unresolved-target`, unbound).
5. An import resolving outside `tests.root` was treated as a resolved dead end —
   `TestAMAPV0005ImportOutsideTests` (`import-outside-tests`, join `UNKNOWN`).
6. A spec attributed through a workflow read `FRESH` after the workflow changed —
   `TestAMAPV0010ChainFreshness` (`chain_freshness` over the spec and its `via` files).
7. A repeated object key read the first value where JavaScript keeps the last —
   `TestAMAPV0002RepeatedKeys` (last value for `get`; a router object with a repeated key is
   non-literal).
8. Line continuations inside a quoted string did not advance the line count of later anchors —
   `TestAMAPV0007RegexReferencedOptionsAndContinuations`.

Findings 4, 5, 7 and 1 resolve design forks fail-closed; they are owner questions 8 to 11 in the
spec. AMAP-V0-002, 005, 007, 010 and 013 and the unknown reasons were amended in the same change.

Round 4 (diff `8af2bf62..f263921f`) reported five P2 findings, each confirmed and repaired with a
regression test that a mutation of its fix makes fail:

1. A short `\u` escape at the end of a template (``String.raw`\u` ``) consumed past the source and
   panicked the lexer — `TestAMAPV0007ShortUnicodeEscape`.
2. `://` anywhere in a URL started an authority, so `/login?returnTo=https://h/#!/x` resolved to
   `/x` — `TestAMAPV0003QueryURLIsNotAuthority` (only a leading scheme).
3. A literal object that was only a prefix of the `.state` argument (`{...} && config`) was read
   as the configuration — `TestAMAPV0002ConfigMustBeWholeArgument`.
4. A manifest rebinding a page object left chains through it `FRESH` —
   `TestAMAPV0010ManifestBindingFreshness` (the whole-file manifest anchor joins a declared chain).
5. `scaffold` called a reused method that takes parameters with none —
   `TestAMAPV0013MethodWithArgumentsNotCalled` (`no_args`; `reuse-takes-arguments`).

Finding 5 and the whole-file manifest anchor resolve forks fail-closed (owner question 12).
Later rounds are recorded in the lane handoff.

## Analyzer schema bump

`internal/contextindex/webimport_api.go` is a consumer-only wrapper, but `TestAnalyzerSchemaInputs`
(IDX-SNAP-V0-017) deliberately pins every production source of the package, so `analyzerSchemaID`
moves to `corvint-analyzer/108` with the new audit digest (precedent: 64ccc2ac bumped it for an audited source change).
Extraction and encoding are unchanged; existing packs are re-derived once.
