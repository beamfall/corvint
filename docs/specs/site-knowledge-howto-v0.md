# Site Knowledge How-To V0 — "show me how to do X" as a live-proven E2E test mined from tests

Owner: Russell Lewis
Date: 2026-10-19
Intent status: accepted
Delivery status: not-started
Authoritative inputs: the owner's intent of 2026-10-19, quoted verbatim below (native ticket
V1-1897); the stage tickets V1-1898..V1-1905; `AGENTS.md` (product invariants 1..8);
`docs/SPEC-DRIVEN-DEVELOPMENT.md`; and the contracts named in "Existing contracts and extension
points", read through `docs/specs/INDEX.json` and their Agent digests.

## Agent digest
- Claim: Page objects, steps, setup and edges mined from tests answer "how do I X" with a live-proven E2E test, or abstain naming the missing knowledge.
- Status: accepted (owner intent 2026-10-19; V1-1897); not-started. SKH-V0-001..084 for stages 1..8 (V1-1898..V1-1905) are accepted under the owner's standing auto-accept (2026-10-11); nothing is implemented.
- Exists: only the contracts this extends: AMAP-V0 map and selectors, ANL-V0 edges, AFU-V1 intents, DCP-V1 inventories and test knowledge, AMSP-V0 planner draft, CCE-V0 prover, NEA-V0 controls, RVN-V0 binding, PWP-V0 receipts and TSA-V0 validity.
- Blocked on: owner confirmation of the assumed first adopter and of N for the preregistered measure; then stage 1 (V1-1898).
- Read next: Owner intent; Definition of "confidently"; Requirements; Preregistered measure; Owner questions.

## Owner intent

Recorded verbatim, including its spelling, as the owner wrote it on 2026-10-19:

> I'd like to get to a point where we have the knowledge to be able to generate a full list of
> steps to do anything in the site. we should know all the page objects, links, data setup, and
> steps to do everything and this should be gathered from testss. then I should be able to say
> "show me how to do x" and the system should be be able to confidently be able to build a e2e
> test that perfectly demostrates it

This spec turns that intent into eight staged slices. The intent is owner-written and accepted.
The numbered requirements below are written by a lane and accepted under the owner's standing
auto-accept of 2026-10-11. They become owner-accepted only when the owner reads them back
(`SRG-V0-009`, `SRG-V0-010`).

## User and measurable job

The user is a developer or agent working on a web application that already has an E2E suite. They
ask, in plain language, "show me how to do X". Today they must find the right page objects, work
out which data the scenario needs, and copy steps from neighbouring tests by hand. Corvint maps
screens, flows and edges (AMAP-V0, ANL-V0, AFU-V1), but it cannot yet list the steps that perform
an arbitrary task or produce a test that demonstrates one.

Measured jobs, each against the confirmed first adopter at a pinned revision:

1. **Knowledge**: every page object, step, setup record and edge in the model cites the test or
   source line it was mined from, and every construct the extractors do not support is a named
   UNKNOWN.
2. **Answer**: for an owner-written request "how do I X", `corvint how` returns a task plan whose
   every element cites evidence, or a closed abstention code naming the missing knowledge.
3. **Demonstrate**: the generated test passes the stage 8 proof pipeline. That pipeline is the
   definition of "confidently" below.
4. **Measure**: the preregistered measure reports answered, abstained and failed counts, with
   0 unanchored statements and byte-identical reruns.

## Verified current state

At origin/main `6ceee260` (2026-10-18):

- AMAP-V0 compiles a revision-pinned screen graph. It resolves the `ui-router-states/0` dialect
  (`AMAP-V0-002`) and the literal `rails-routes/0` and `react-router/0` dialects (`AMAP-V0-040`..
  `042`). Selectors carry a kind and strength (`AMAP-V0-007`), and edges come only from evidence
  (`AMAP-V0-008`). The `scaffold` projection drafts a skeleton beside the closest asserting spec
  (`AMAP-V0-013`), and learned facts attach only through the overlay seam (`AMAP-V0-014`).
- AMSP-V0 resolves plain-language request steps to mapped flows (`AMSP-V0-002`). It composes
  navigation over mapped steps (`AMSP-V0-006`) and drafts a guarded Playwright skeleton
  (`AMSP-V0-008`), served by `corvint flows appmap plan` and the corpus MCP tool `corvint.map_plan`
  (`AMSP-V0-010`).
- ANL-V0 classifies static page-object step effects (`ANL-V0-035`, `ANL-V0-036`). It has a
  config-installed Playwright navigation-events producer (`ANL-V0-037`..`046`).
- RVN-V0 binds Playwright receipts to map steps only. Selectors and shown methods stay
  `unverified` (AMSP-V0 owner question 9).
- AFU-V1 flow intents are authored, and `flows import` writes proposed intents only
  (`AFU-V1-004`).
- DCP-V1 statically inventories legacy Jasmine and Mocha/Cypress cases and their assertion criteria
  (`DCP-V1-060`..`069`). Reviewed test-knowledge records joined by trust level (`DCP-V1-086`..`092`)
  are in open PR #1165 (V1-1868) and are not on origin/main.
- TSA-V0 selects setup projects and flags static test-validity defects, including the JS/TS
  example-hygiene rules `JS-NO-ASSERTION`, `JS-CONSTANT-EXPECTATION` and `JS-SUBJECT-MOCKED`
  (`TSA-V0-073`..`081`, V1-1858).
- CCE-V0 compilation is killed. The V1-1677 spike on `actualbudget/actual` found 0 of 10 changes
  useful against a 40% threshold (`docs/build-log/2026-10-18-cce-spike-v1-1677.md`). V1-1733
  pivots CCE to a discrimination prover that runs existing hosted tests and reports the verdict
  ladder of `CCE-V0-009` without compiling anything.
- NEA-V0 repeats new Playwright tests and runs approved negative controls (`NEA-V0-002` and
  then `NEA-V0-003`). PWP-V0 binds external-server Playwright receipts, and its `/2` profile redacts
  sensitive input evidence. The JS/TS live-test provider (`js-live-test-provider-v0.md`) is a
  prose-only experiment profile with no requirement IDs (decision 0179).
- Nothing extracts page-object classes with ordered actions, normalises steps into one IR, models
  data setup, mines template links, clusters flows from tests, answers a task query or generates
  a test from mined evidence.

Known gaps this spec depends on:

- **V1-1712**: AMAP's `react-router/0` reads 0 screens from JSX `<Routes>` trees inside components
  (0 screens on actualbudget). A React adopter makes V1-1712 a prerequisite of stage 1.
- **V1-1236**: test-id and raw-selector clicks, property-bound locators, inherited page-object
  methods and spec-local helpers are not followed. Stage 1 completes it in its owning spec
  (`SKH-V0-012`).
- **V1-1191**: there is no bounded, local, link-only discovery crawl that reports unmapped URLs.
  Stage 4 consumes the crawl when it lands (`SKH-V0-042`).

## Assumed first adopter

**ASSUMPTION — the owner must confirm before stage 1 starts (Owner question 1).** The first
adopter is a Rails application with an AngularJS ui-router front end. Its tests are a Playwright
E2E suite, RSpec (including Capybara feature or system specs) and a legacy Jasmine suite.

Why this choice: AMAP already resolves both of its router dialects. They are `ui-router-states/0`
(`AMAP-V0-002`, with the constant-table and DI extensions `AMAP-V0-016` and `AMAP-V0-021`..`026`)
and `rails-routes/0` (`AMAP-V0-040`..`042`). Stage 1 therefore needs no new router dialect. The
legacy Jasmine suite enters through the existing DCP-V1 inventory (`DCP-V1-060`..`062`). If the
owner chooses a React adopter instead, V1-1712 becomes a prerequisite ticket ahead of stage 1.

## Definitions

- **Page object**: a class or module whose methods wrap locators and actions on one screen or
  component, for example a Playwright or Cypress TS class or a Capybara/SitePrism page.
- **Step IR**: the closed, normalised step record of stage 2 (`SKH-V0-020`).
- **Setup record**: a typed description of how a precondition is produced and cleaned up
  (`SKH-V0-030`).
- **Edge provenance**: one of `static` (mined from templates or components), `test-observed`
  (`AMAP-V0-008`), `ledger-proven` (ANL-V0) or `run-verified` (RVN-V0). No provenance is promoted
  into another.
- **Candidate flow**: a cluster of step sequences proposed by stage 5. It is never accepted intent.
- **Task plan**: the stage 6 answer: goal, preconditions, step path and oracle.
- **Anchored statement**: a generated line that traces to a learned evidence record, meaning a
  mined statement, page-object call, setup call or assertion copied from a real test. An
  **unanchored statement** is any other generated line, including one marked TODO.
- **Abstention**: an answer that names a closed code and the missing knowledge instead of a plan
  or a test.

## Definition of "confidently"

A generated test demonstrates X **confidently** only when all of the following hold:

1. it ran live against the target application at a pinned application revision and passed, with
   repeats as in `NEA-V0-002`;
2. it passed TSA-V0 validity with no defect finding, including the JS/TS hygiene rules of
   TSA-V0 (`TSA-V0-073`..`081`);
3. it discriminates: the approved negative controls (`NEA-V0-003`, with the BBF-V0 `wrong-locator`
   and `wrong-expected-value` controls) fail it, and the seeded-mutant discrimination verdict
   (`CCE-V0-009`), run through the V1-1733 prover, does not read non-discriminating;
4. every line traces to learned evidence (0 unanchored statements), and its steps, selectors and
   methods are bound to run evidence (RVN-V0, extended in stage 8).

Otherwise the system abstains and names what is missing. It never presents a plan, a draft or a
passing but non-discriminating test as a demonstration.

## Existing contracts and extension points

Each stage extends the named point. None of these contracts is restated here.

| Contract | What it owns | Extension point this spec uses | Stage |
|---|---|---|---|
| AMAP-V0 | revision-pinned screen graph, selectors, dialects, `scaffold` | import-graph join (`AMAP-V0-005`), selector kind/strength (`AMAP-V0-007`), edge evidence (`AMAP-V0-008`), overlay seam (`AMAP-V0-014`), dialects (`AMAP-V0-040`..`042`) | 1, 4, 7 |
| ANL-V0 | revision-evaluated edge ledger, navigation-events producer | static page-object step effects (`ANL-V0-035`), producer (`ANL-V0-037`..`046`) | 1, 4 |
| AFU-V1 | flow intents, links, run evidence | `flows import` of proposed intents (`AFU-V1-004`), inferred links stay separate (`AFU-V1-009`) | 5 |
| DCP-V1 | documentation corpus | legacy inventory (`DCP-V1-060`..`069`), test-knowledge trust joins (`DCP-V1-086`..`092`, PR #1165) | 2, 5 |
| AMSP-V0 | multi-step plan over maps | step resolution (`AMSP-V0-002`), freshness (`AMSP-V0-003`), navigation (`AMSP-V0-006`), draft (`AMSP-V0-008`), budget (`AMSP-V0-009`), surfaces (`AMSP-V0-010`) | 6, 7 |
| CCE-V0 | discrimination verdict (compilation killed by V1-1677) | `CCE-V0-009` ladder through the V1-1733 prover | 8 |
| NEA-V0 | repeats and negative controls for new Playwright tests | `NEA-V0-002`, `NEA-V0-003` | 8 |
| RVN-V0 | receipt-bound step verification | `RVN-V0-002`..`005`, widened from steps to selectors and methods | 8 |
| PWP-V0 | external-server Playwright receipts | receipt binding and `/2` sensitive-input redaction | 3, 8 |
| TSA-V0 | static test analysis | setup-select, validity rules `TSA-V0-073`..`081` | 3, 8 |
| js-live-test-provider | JS/TS live run receipts (prose-only profile) | Playwright adapter as the live runner | 8 |

## Requirements

### General (V1-1897)

- `SKH-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897) Every
  learned record MUST be pinned to one Git revision and carry at least one anchor naming its path,
  Git blob, line span and span digest, plus its provenance class. This covers page objects,
  methods, steps, setup records, edges, candidate flows, task plans and generated lines. A record
  without an anchor MUST NOT be emitted (invariant 1).
- `SKH-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897) Missing,
  ambiguous or unsupported evidence MUST produce an explicit UNKNOWN or abstention that names the
  file, the construct and the kind of knowledge missing. No stage may invent a step, locator,
  setup, edge or oracle (invariant 2).
- `SKH-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897) Learned
  steps, flows and task plans MUST stay proposals with authority `learned` until a human accepts
  them through the existing AFU-V1 path. They MUST NOT outrank project-owned authority or become
  accepted intent by themselves (invariants 3 and 8).
- `SKH-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897) Matching
  plain-language text to learned records MUST be lexical and deterministic. It uses normalised
  tokens, identifiers, titles and anchored literals only, with no embeddings, vector models or
  network calls (invariant 7).
- `SKH-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897)
  Extraction, model building and the task query MUST be static read commands in core `corvint`.
  They read Git at a pinned revision, run no browser, Node, Ruby or package manager, and write
  nothing (invariant 4). Running a generated test MUST happen only in an existing companion
  (`SKH-V0-083`), because browser and Node stay out of the core binary (invariant 7).
- `SKH-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897)
  `storageState` contents, cookies, tokens and typed literal values (fill steps and setup
  parameters) MUST pass through `internal/secretscreen` and the PWP-V0 `/2` redaction rules before
  any of them is stored, printed or generated. A screened value is replaced by a named parameter
  slot, and the record says that it was screened. A generated test MUST NOT embed a screened value
  (invariant 5).
- `SKH-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897) For one
  revision and one set of inputs, every output MUST be byte-identical across reruns, ordered by
  stable IDs, and byte-capped with an explicit truncation marker.
- `SKH-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897) Each stage
  MUST extend the extension point named for it in "Existing contracts and extension points"
  instead of adding a parallel model, command or MCP surface. A stage that cannot do so MUST
  record why in its build-log entry before adding one.
- `SKH-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897) The owner
  MUST confirm the first adopter before stage 1 starts. No stage may leave `experimental` before
  the stage it reads from has fixture evidence plus one real-adopter sample.

### Stage 1: page objects (V1-1898)

- `SKH-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1898) A
  revision-pinned page-object model MUST list each page-object class with its methods,
  parameters, locators (kind and strength as in `AMAP-V0-007`) and the ordered actions each method
  performs, each with a source span. This covers Playwright and Cypress TypeScript and at least
  one Ruby form (Capybara/SitePrism). Any other form MUST abstain, naming the file and the form.
- `SKH-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1898) Page
  objects MUST join to AMAP-V0 screens through the existing import-graph join (`AMAP-V0-005`). A
  method whose effect (navigation or form submit) is statically known MUST record it with the
  classification of `ANL-V0-035`. Unknown effects stay `unknown`.
- `SKH-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1898) Inherited
  methods, included modules and spec-local helpers MUST be followed one bounded, anchored hop at a
  time. An unresolved hop is an UNKNOWN at its own line. This completes V1-1236 in its owning spec
  (AMAP-V0 or ANL-V0) and does not duplicate it.
- `SKH-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1898) The model
  MUST be served through the existing `corvint flows appmap` command family and the corpus MCP as
  byte-capped projections, not through a new top-level command.
- `SKH-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1898) Stage 1
  evidence MUST include committed fixtures for each supported form plus one real-adopter sample.
  The sample reports counts of classes, methods, locators and abstentions.

### Stage 2: step IR (V1-1899)

- `SKH-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1899) Spec bodies
  and page-object methods MUST normalise into one closed step IR with the kinds `visit`, `click`,
  `fill`, `select`, `check`, `upload`, `wait-for` and `assert` (with matcher and expected value).
  Each step carries a locator reference, parameter slots and a source span, for Playwright,
  Cypress and Capybara at least. Legacy Jasmine cases enter through the DCP-V1 inventory criteria
  (`DCP-V1-060`..`062`), not through a second Jasmine parser.
- `SKH-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1899) Page-object
  calls MUST inline to their IR with arguments bound to parameter slots. An unparseable step MUST
  become an explicit `opaque` step with its span, never dropped. A sequence that holds an opaque
  step is marked incomplete.
- `SKH-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1899) Identical
  step sequences across tests MUST deduplicate into reusable step records with usage counts, each
  citing every source test.
- `SKH-V0-023`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1899) The IR MUST
  be a closed, versioned record. An unknown step kind or field is refused, not coerced.

### Stage 3: data setup (V1-1900)

- `SKH-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900) The
  following MUST be extracted into typed setup records: FactoryBot factories and traits, fixture
  files, seeds, test-setup HTTP endpoints and their handlers' model writes, `cy.task`, Playwright
  setup projects, `test.extend` fixtures, `beforeEach` and `beforeAll` hooks, `storageState`,
  `page.route` mocks and API seeding calls. Each record names its parameters, the entities and
  attributes it produces, its cleanup and its anchors.
- `SKH-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900)
  Preconditions used by existing tests MUST link to the setup records that produced them. The
  query "what can produce condition C" MUST return candidates with evidence, or `unresolved`.
- `SKH-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900) Setup
  extraction MUST reuse TSA-V0 setup-select for setup projects, V1-1891 for setup endpoints per
  branch and V1-1883 for seed normalisation, without duplicating them.
- `SKH-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900)
  `storageState` contents and typed literal values MUST be screened under `SKH-V0-006` before
  storage. A setup record keeps the `storageState` path and digest and only the screened contents.
- `SKH-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900) A setup
  record backed by a `page.route` mock MUST be labelled `mocked`. No plan or proof may treat a
  mocked response as evidence of real backend behaviour.

### Stage 4: navigation edges (V1-1901)

- `SKH-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) Static edges
  from templates and components MUST be added to the AMAP-V0 screen graph with provenance
  `static`. Sources include `a href`, `ui-sref`, `routerLink`, `link_to` and `button_to`, form
  actions, and programmatic navigation such as `$state.go`, `navigate` and `redirect_to`. Static
  edges stay distinct from `test-observed` (`AMAP-V0-008`) and `ledger-proven` (ANL-V0) edges.
- `SKH-V0-041`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) Router
  dialect coverage MUST be widened only for a measured adopter need, and unsupported dialects
  abstain (the `AMAP-V0-042` rule). For a React adopter, V1-1712 is a prerequisite.
- `SKH-V0-042`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) Observed
  navigation MUST come from the ANL-V0 navigation-events producer (`ANL-V0-037`..`046`) and, once
  delivered, the V1-1191 link-only crawl. There is no second capture path.
- `SKH-V0-043`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) A dynamic
  target, such as an interpolated URL or a computed state name, MUST be an UNKNOWN edge at its
  line. It is never resolved by guessing.

### Stage 5: flow inference (V1-1902)

- `SKH-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Step-IR
  sequences across tests MUST cluster deterministically into candidate flows (start screen, steps,
  end screen and outcome) and variations, by shared step-record IDs and screens. Each candidate
  cites its source tests.
- `SKH-V0-051`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Candidate
  flows MUST enter only as proposed AFU-V1 intents through `flows import` (`AFU-V1-004`). They are
  never accepted intent (invariant 8).
- `SKH-V0-052`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Candidates
  MUST join DCP-V1 test-knowledge trust levels (`DCP-V1-086`..`092`, PR #1165) and rank
  `run+reviewed` sources above `run-unreviewed` and unreviewed ones. Trust is labelled, never
  promoted (`DCP-V1-090`).
- `SKH-V0-053`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Each
  candidate MUST report its support (distinct source tests) and its conflicts (tests that diverge
  at a step). A candidate with one source test is labelled `single-source`.

### Stage 6: the `corvint how` task query (V1-1903)

- `SKH-V0-060`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1903)
  `corvint how "<goal>"` and one MCP tool MUST resolve a plain-language goal to a task plan. The
  plan holds the goal screen and outcome, the required preconditions with their setup records, a
  step path through the screen graph built from learned steps and page-object methods, and an
  assertion oracle copied from an existing test. Every element cites evidence.
- `SKH-V0-061`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1903) Resolution
  MUST be lexical (`SKH-V0-004`) over flow titles, test titles, step records and screen names. It
  reuses AMSP-V0 step resolution (`AMSP-V0-002`) and navigation composition (`AMSP-V0-006`) rather
  than a second planner.
- `SKH-V0-062`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1903) An ambiguous
  goal MUST return ranked candidates with ties named, never broken by an arbitrary choice. An
  unsupported goal MUST return an abstention with one closed code: `no-route`,
  `no-setup-for-precondition`, `no-oracle`, `no-page-object`, `ambiguous-goal` or
  `stale-evidence`.
- `SKH-V0-063`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1903) A plan with
  any element stale at the evaluated revision (`AMSP-V0-003` freshness) MUST read `STALE`, never
  `COMPLETE`.
- `SKH-V0-064`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1903) The plan
  MUST fit the AMSP-V0 byte budget rules (`AMSP-V0-009`).

### Stage 7: generator (V1-1904)

- `SKH-V0-070`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1904) From a
  `COMPLETE` task plan, the generator MUST emit a runnable test in the target suite's framework and
  conventions. It reuses existing page objects, setup records and fixtures, and places the test
  beside the closest existing spec (the `AMAP-V0-013` placement rule).
- `SKH-V0-071`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1904) Every
  generated line MUST trace to a learned evidence record, and the generator MUST emit that
  line-to-evidence map with the test.
- `SKH-V0-072`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1904) A line
  without evidence MUST be a marked TODO, and the result reads `INCOMPLETE`, never done. Because
  CCE-V0 compilation was killed (V1-1677: 0 of 10 useful; V1-1733 pivot), the generator MUST NOT
  synthesise locators, matchers, values or control flow. Statements and assertions are copied from
  real tests with only parameter-slot substitution.
- `SKH-V0-073`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1904) The
  generator MUST extend `flows appmap plan --request --draft` (`AMSP-V0-008`, the `test.fixme`
  skeleton) and the `corvint.map_plan` MCP tool rather than a parallel surface. The draft guard
  stays until stage 8 proves the test.
- `SKH-V0-074`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1904) Generation
  MUST be byte-identical across reruns for the same plan and revision, and it writes only to an
  output path the caller names explicitly.

### Stage 8: proof pipeline (V1-1905)

- `SKH-V0-080`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) One
  companion command MUST take a generated test through every condition of "confidently". It runs
  the test live through the js-live-test-provider Playwright adapter and PWP-V0 at a pinned
  application revision, and applies TSA-V0 validity (`TSA-V0-073`..`081`). It binds steps,
  selectors and methods to run evidence, with RVN-V0 widened from steps to selectors and methods.
  It runs the NEA-V0 and BBF-V0 negative controls and the `CCE-V0-009` discrimination verdict
  through the V1-1733 prover. The command emits one acceptance receipt.
- `SKH-V0-081`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) The receipt
  MUST read `PROVEN` only when every condition of "confidently" holds. Otherwise it names the first
  failing stage with its evidence, using one closed code: `generate-incomplete`, `run-failed`,
  `validity-defect`, `binding-unverified`, `control-survived`, `not-discriminating` or
  `infrastructure`.
- `SKH-V0-082`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) The live run
  MUST repeat as in `NEA-V0-002`, with retries at zero. A flaky result is never `PROVEN`.
- `SKH-V0-083`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) The pipeline
  MUST run in existing companions (`corvint-test`, `corvint-tests-accept`,
  `corvint-js-test-provider`). Core `corvint` reads only the receipt.
- `SKH-V0-084`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) Stage 8 MUST
  be qualified on the confirmed real adopter with the preregistered request set, reporting proven,
  abstained and failed counts, unanchored statements and rerun byte identity.

## Preregistered measure

The measure is fixed before stage 6 runs on any request, and it is closable by the single owner.

- **Request set**: N owner-written "how do I X" requests for the confirmed adopter. N is proposed
  as 20, and the owner confirms it (Owner question 2). The set and its SHA-256 are committed in a
  build-log entry before the first run and are not edited afterwards. A request added later is
  reported separately.
- **Answered**: the stage 8 receipt reads `PROVEN`, which means a passing, discriminating test, and
  the owner confirms that the test demonstrates X.
- **Abstained**: a closed abstention code from `SKH-V0-062` or `SKH-V0-081`. An abstention is an
  acceptable outcome, never a failure of invariant 2.
- **Wrong**: a `PROVEN` test that the owner judges does not demonstrate X. Any wrong answer blocks
  promotion.
- **Kill threshold**: if fewer than 40% of the N requests are answered, the generation approach
  (stages 7 and 8) is killed. The same threshold killed CCE-V0 compilation.
- **Hard gates**: 0 unanchored statements across all `PROVEN` tests, and byte-identical reruns of
  the model, the plans and the generated tests at the same revision. Either gate failing fails the
  run, whatever the answered rate.
- **Report**: answered, abstained (by code), wrong and failed counts, unanchored statements, rerun
  identity and wall minutes per request.

## Non-goals and simpler baseline

- No synthesised test code. Statements, locators and oracles are mined or copied, never generated
  by a model (`SKH-V0-072`).
- No embeddings, semantic search, hosted service, daemon or network dependency (invariant 7).
- No browser, Node or Ruby execution in core `corvint` (`SKH-V0-005`).
- No new router dialect except for a measured adopter need (`SKH-V0-041`).
- No replacement of AMAP-V0, ANL-V0, AFU-V1, AMSP-V0 or the CCE-V0 prover. This spec only extends
  them.
- No automatic acceptance of mined flows or steps as product intent.
- No claim of exhaustive coverage. "Everything in the site" means everything the tests evidence;
  the rest is a named gap.

Simpler baseline: an agent reads the existing tests and copies the closest one by hand. The
preregistered measure compares against it only qualitatively. If stages 1..6 alone (knowledge and
plans without generation) give the owner most of the value, that result is recorded and stages 7
and 8 may stop there.

## Trust boundary, constraints and limits

- **Inputs**: Git blobs at a pinned revision of the application and test repositories, declared
  in the AMAP manifest. Run receipts come only from the stage 8 companions.
- **Authority**: everything mined is `learned`. Owner-accepted intent (AFU-V1 accepted flows) and
  project authority outrank it (invariant 3).
- **Secrets**: `SKH-V0-006` and `SKH-V0-033`. A screening failure fails closed: the value is not
  stored.
- **Limits**: each stage inherits the byte caps of the projection it extends (AMAP-V0 projections
  and `AMSP-V0-009`). The record count and file size bounds of each extractor are set in its stage's
  build-log entry before the extractor is built.
- **Fail-closed choices**: unknown constructs, dynamic targets, stale evidence, unscreenable values
  and missing oracles all abstain. Nothing fails open.

## Failure modes

| Failure | Required behaviour |
|---|---|
| Unsupported test or page-object form | abstain, naming the file and form (`SKH-V0-010`) |
| Inherited or helper method not resolvable | UNKNOWN at that hop's line (`SKH-V0-012`) |
| Step cannot be parsed | `opaque` step; the sequence is incomplete (`SKH-V0-021`) |
| Precondition with no setup record | `unresolved`; the plan abstains with `no-setup-for-precondition` |
| Mocked endpoint mistaken for real behaviour | prevented by the `mocked` label (`SKH-V0-034`) |
| Dynamic navigation target | UNKNOWN edge (`SKH-V0-043`) |
| Router dialect unsupported (for example JSX `<Routes>`, V1-1712) | 0 screens with a named abstention, never guessed screens |
| Goal matches several flows equally | ranked candidates with ties named (`SKH-V0-062`) |
| Evidence changed since it was mined | `STALE` (`SKH-V0-063`) |
| Generated line has no evidence | TODO and `INCOMPLETE` (`SKH-V0-072`) |
| Test passes but does not discriminate | `not-discriminating`, never `PROVEN` (`SKH-V0-081`) |
| Flaky live run | never `PROVEN` (`SKH-V0-082`) |
| Secret in `storageState` or a typed value | screened and replaced by a parameter slot (`SKH-V0-006`) |
| Browser, server or provider failure | `infrastructure`, kept separate from test failure |

## Acceptance evidence and testing matrix

| Stage | Fixture evidence | Real-adopter evidence |
|---|---|---|
| 1 | one fixture per supported page-object form, including inherited and helper hops | class, method, locator and abstention counts on the adopter |
| 2 | IR fixtures for Playwright, Cypress and Capybara, with opaque steps and dedup | step-record and opaque counts |
| 3 | setup fixtures for each listed source, plus secret-screening tests | setup records and unresolved preconditions |
| 4 | template-link fixtures per syntax, with dynamic targets as UNKNOWN | static and observed edge counts, kept separate |
| 5 | clustering fixtures with conflicts and single-source candidates | candidate flows imported as proposed intents |
| 6 | `how` fixtures for answered, ambiguous and every abstention code | plans for the preregistered requests |
| 7 | generation fixtures with evidence maps and TODO lines, plus rerun identity | generated tests for the planned requests |
| 8 | receipt fixtures for `PROVEN` and every failing-stage code | the preregistered measure report |

Each stage's tests cite its requirement IDs in their names or docstrings. The real-adopter
qualification is recorded in a build-log entry with the adopter revision, tool versions and the
measure report. It does not cite private product names.

## Rollout and rollback

- Every stage ships `experimental`, behind the existing `flows appmap` family, `corvint how` and
  the companion commands. No surface is advertised before its stage evidence exists.
- All learned records are derived state. Deleting them and regenerating at the same revision gives
  the same bytes (`SKH-V0-007`), so a rollback never loses authority.
- A stage is rolled back by reverting its commits. Earlier stages keep working, because each stage
  only reads the one before it.
- If the preregistered measure meets the kill threshold, stages 7 and 8 become `deferred` with a
  `Disposition:` header naming the return condition. Stages 1..6 are kept only if their own
  evidence shows use, and the decision is recorded in a build-log entry.
- This spec adds no wire field to an existing contract until the stage that needs it amends that
  contract's spec in the same change.

## Traceability

| Requirements | Ticket | Implementation | Evidence |
|---|---|---|---|
| `SKH-V0-001`..`009` | V1-1897 | this spec | NOT_STARTED |
| `SKH-V0-010`..`014` | V1-1898 | none | NOT_STARTED |
| `SKH-V0-020`..`023` | V1-1899 | none | NOT_STARTED |
| `SKH-V0-030`..`034` | V1-1900 | none | NOT_STARTED |
| `SKH-V0-040`..`043` | V1-1901 | none | NOT_STARTED |
| `SKH-V0-050`..`053` | V1-1902 | none | NOT_STARTED |
| `SKH-V0-060`..`064` | V1-1903 | none | NOT_STARTED |
| `SKH-V0-070`..`074` | V1-1904 | none | NOT_STARTED |
| `SKH-V0-080`..`084` | V1-1905 | none | NOT_STARTED |

## Promotion and kill criteria

- **Promote** to `implemented` when every stage has its fixture evidence, and to `validated` when
  the preregistered measure answers at least 40% of the requests with 0 wrong answers, 0
  unanchored statements and byte-identical reruns. Promotion also needs the owner to read back the
  `standing` requirements (`SRG-V0-010`).
- **Kill** stages 7 and 8 if fewer than 40% of the requests are answered. Kill any stage whose
  real-adopter sample shows it cannot anchor its records.

## Owner questions

1. Confirm the assumed first adopter (Rails + AngularJS ui-router, Playwright E2E, RSpec, legacy
   Jasmine), and approve cloning it and running its suite locally for stage 8. If the adopter is a
   React application instead, V1-1712 becomes a prerequisite of stage 1.
2. Confirm N for the preregistered measure (proposed 20) and the 40% kill threshold.
3. Which Ruby page-object form does the adopter use, Capybara page classes or SitePrism? Stage 1
   needs at least one.
4. Should the stage 7 generator target only Playwright in V0, leaving RSpec system specs for later?
5. Should stage 5 wait for `DCP-V1-086`..`092` (PR #1165) to merge, or start with unreviewed trust
   only?
