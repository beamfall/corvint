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

AMAP-V0-001, 005, 007, 010, 012, 013 and 014 were amended in the same change. Later rounds are
recorded in the lane handoff.

## Analyzer schema bump

`internal/contextindex/webimport_api.go` is a consumer-only wrapper, but `TestAnalyzerSchemaInputs`
(IDX-SNAP-V0-017) deliberately pins every production source of the package, so `analyzerSchemaID`
moves to `corvint-analyzer/108` with the new audit digest (precedent: 64ccc2ac bumped it for an audited source change).
Extraction and encoding are unchanged; existing packs are re-derived once.
