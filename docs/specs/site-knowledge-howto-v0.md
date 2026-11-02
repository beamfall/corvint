# Site Knowledge How-To V0 — "show me how to do X" as a live-proven E2E test mined from tests

Owner: Russell Lewis
Date: 2026-10-19
Intent status: accepted
Delivery status: experimental
Authoritative inputs: the owner's intent of 2026-10-19, quoted verbatim below (native ticket
V1-1897); the stage tickets V1-1898..V1-1905; `AGENTS.md` (product invariants 1..8);
`docs/SPEC-DRIVEN-DEVELOPMENT.md`; and the contracts named in "Existing contracts and extension
points", read through `docs/specs/INDEX.json` and their Agent digests.

## Agent digest
- Claim: Page objects, steps, setup and edges mined from tests answer "how do I X" with a live-proven E2E test, or abstain naming the missing knowledge.
- Status: accepted (owner intent 2026-10-19; V1-1897); experimental. SKH-V0-001..089 for stages 1..8 (V1-1898..V1-1905) are accepted under the owner's standing auto-accept (2026-10-11). Stages 1 and 2 are implemented as `flows appmap pageobjects` and `flows appmap steps` and the corpus MCP tool `corvint.map_knowledge` (V1-1898, V1-1899). Stage 3 (V1-1900, SKH-V0-100..105) is implemented as `flows appmap setup`: Playwright and Cypress fixtures, hooks, `storageState`, route mocks, API seeding, `cy.task` and setup projects, Rails factories, traits, fixture files, seeds and setup endpoints through the AMAP-V0-114 writer index, Ruby test links and the `--condition` query; the V1-1883 normaliser and TSA-V0 setup-select reuse in `SKH-V0-032` stay partial. Stage 4 (V1-1901, SKH-V0-110..115) is implemented as `flows appmap links`: static edges from templates and components with provenance `static` beside the map's test-observed edges; no router dialect is widened. Stage 5 (V1-1902, SKH-V0-120..126) is implemented as `flows appmap infer`: candidate flows and variations from test step sequences, ranked and blocked by DCP-V1 test knowledge, emitted as a proposed behavior-adapter request for `flows import`. Stage 6 (V1-1903, SKH-V0-143..152) is implemented as `flows appmap how` and the `how` view of `corvint.map_knowledge`; the top-level `corvint how` alias is a follow-up. Stage 7 (V1-1904, SKH-V0-153..162) is implemented as the guarded copy-only draft of `flows appmap plan --request GOAL --draft --source DIR` and `corvint.map_plan` with `sources`. Stage 8 (V1-1905, SKH-V0-163..176) is implemented offline as `corvint-test e2e how-accept` over recorded evidence; `corvint-test e2e how-evidence` (SKH-V0-180..184, V1-2595) records one live run on a synthetic fixture, which reads `acceptance-blocked`, not `infrastructure`; selectors and methods stay `binding-unverified`, the core-history qualification (`SKH-V0-084`) reads 0 proven, 9 abstained and 1 failed of 10 lane-written goals, condition 3 has a named source for hygiene (SKH-V0-185..187, V1-2641..2643) and blocks on association, freshness and cleanup, and nothing reads `PROVEN` until the V1-1733 prover contract is accepted.
- Exists: `flows appmap pageobjects`, `flows appmap steps`, `flows appmap setup` and `corvint.map_knowledge` (stages 1 and 2, partial stage 3, experimental), plus the contracts this extends: AMAP-V0 map and selectors, ANL-V0 edges, AFU-V1 intents, DCP-V1 inventories and test knowledge, AMSP-V0 planner draft, CCE-V0 prover, NEA-V0 controls, RVN-V0 binding, PWP-V0 receipts and TSA-V0 validity.
- Blocked on: nothing for stage 4 (V1-1901) or stage 5 (V1-1902); a measured adopter need for any new router dialect (`SKH-V0-041`). First adopter core-history (owner 2026-10-22); N = 10.
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
  (`DCP-V1-060`..`069`). Reviewed test-knowledge records joined by trust level (`DCP-V1-086`..`092`,
  V1-1868) landed at origin/main `bda19b02` (PR #1165).
- TSA-V0 selects setup projects and flags static test-validity defects, including the JS/TS
  example-hygiene rules `JS-NO-ASSERTION`, `JS-CONSTANT-EXPECTATION` and `JS-SUBJECT-MOCKED`
  (`TSA-V0-073`..`081`, V1-1858).
- CCE-V0 compilation is killed. The V1-1677 spike on `actualbudget/actual` found 0 of 10 changes
  useful against a 40% threshold (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-cce-spike-v1-1677.md`). V1-1733
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

## First adopter

**Owner decision, 2026-10-22: the first adopter for `SKH-V0-009` is core-history**
(`/Users/russelllewis/projects/core-history`). It is a React 19 + react-router single-page
application with a Playwright TypeScript E2E suite under `internal/web/app/e2e`, read through
`react-router/0` (`AMAP-V0-040`; V1-1712 is complete). The suite has no page-object classes and no
Ruby: its reusable helpers are exported module functions, so the qualified page-object form is
`playwright-ts-module`. The Playwright and Cypress class forms and the Capybara and SitePrism forms
have fixture evidence only and stay `experimental` (`SKH-V0-009`).

The adopter is read-only. Corvint never writes to it, never runs its application, and never runs
its tests against a network. Samples read its committed test sources at a pinned commit (its HEAD
from `git rev-parse HEAD`) in a disposable clone. Only counts, hashes and paths from a sample enter
the Corvint repository, in the build log; adopter source is never copied beyond minimal synthetic
fixtures.

**Real-adopter sample size.** One sample is the whole declared E2E directory of the adopter at the
pinned commit (26 source files and 62 tests at `5f5d44d3`), not a subset. The directory is inside
the extractor bounds, so a whole-suite read leaves no sampling choice to bias, and a rerun at the
same commit must be byte-identical (`SKH-V0-007`). A second adopter is not required for V0.

**N for the preregistered measure: 10** (orchestrator decision 2026-10-22, under the owner's
delegation of in-task forks of 2026-10-05; the owner may revise it). Reasons: the adopter suite has
62 tests, of which 30 read as complete step sequences today, so 20 distinct owner-written requests
with accepted oracles would exceed the demonstrated surface; one owner writes and judges every
request; and 10 keeps the 40% kill threshold meaningful (4 answered) at 10-point granularity. The
40% threshold is unchanged.

**Superseded assumption (2026-10-19).** The spec first assumed a Rails application with an AngularJS ui-router front end. Its tests are a Playwright
E2E suite, RSpec (including Capybara feature or system specs) and a legacy Jasmine suite.

Why this choice: AMAP already resolves both of its router dialects. They are `ui-router-states/0`
(`AMAP-V0-002`, with the constant-table and DI extensions `AMAP-V0-016` and `AMAP-V0-021`..`026`)
and `rails-routes/0` (`AMAP-V0-040`..`042`). Stage 1 therefore needs no new router dialect. The
legacy Jasmine suite enters through the existing DCP-V1 inventory (`DCP-V1-060`..`062`). The owner
chose a React adopter instead (above).

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

A generated test demonstrates X **confidently** only when all six of the following hold. Each
condition is checked on the exact candidate bytes of `SKH-V0-088`.

1. **Ran and passed**: it ran live against the target application at a pinned application
   revision and passed every repeat as in `NEA-V0-002`, with retries at zero (`SKH-V0-082`).
2. **Valid source**: TSA-V0 located exactly one example for it in the candidate's blob, with
   hygiene explicitly `ELIGIBLE` and no defect finding. An abstention, an `uncertain` file or a
   missing example is not validity (`SKH-V0-086`).
3. **Accepted by the provider**: the complete applicable NEA-V0 assessment holds, with every
   required axis observed and none blocked, freshness and cleanup included (`SKH-V0-082`).
4. **Discriminates**: every approved negative control reads `killed` (the NEA-V0-003 controls and
   the BBF-V0 `wrong-locator` and `wrong-expected-value` controls), and the discrimination verdict
   of the V1-1733 prover is on the allowlist of `SKH-V0-085`. Any other verdict, or none, blocks.
5. **Anchored and witnessed**: every line traces to learned evidence (0 unanchored statements),
   and every step, selector and method it uses has a behavioural witness from the live run
   (`SKH-V0-089`).
6. **Demonstrates the request**: its outcome assertion is the outcome oracle accepted for the
   request before the run (`SKH-V0-087`).

Conditions 1 to 5 without 6 read `EXECUTION_VERIFIED`: the test runs and discriminates, but
nothing shows that it demonstrates X. Only all six read `PROVEN`. Otherwise the system abstains
and names what is missing. It never presents a plan, a draft, an execution-verified test or a
passing but non-discriminating test as a demonstration.

## Existing contracts and extension points

Each stage extends the named point. None of these contracts is restated here.

| Contract | What it owns | Extension point this spec uses | Stage |
|---|---|---|---|
| AMAP-V0 | revision-pinned screen graph, selectors, dialects, `scaffold` | import-graph join (`AMAP-V0-005`), selector kind/strength (`AMAP-V0-007`), edge evidence (`AMAP-V0-008`), overlay seam (`AMAP-V0-014`), dialects (`AMAP-V0-040`..`042`) | 1, 4, 7 |
| ANL-V0 | revision-evaluated edge ledger, navigation-events producer | static page-object step effects (`ANL-V0-035`), producer (`ANL-V0-037`..`046`) | 1, 4 |
| AFU-V1 | flow intents, links, run evidence | `flows import` of proposed intents (`AFU-V1-004`), inferred links stay separate (`AFU-V1-009`) | 5 |
| DCP-V1 | documentation corpus | legacy inventory (`DCP-V1-060`..`069`), test-knowledge trust joins (`DCP-V1-086`..`092`) | 2, 5 |
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
  stable IDs, and byte-capped with an explicit truncation marker. Every output record MUST carry
  a closed, versioned profile string, and a reader MUST refuse an unknown profile or a newer
  version by name, never coerce or partly read it.
- `SKH-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897) Each stage
  MUST extend the extension point named for it in "Existing contracts and extension points"
  instead of adding a parallel model, command or MCP surface. A stage that cannot do so MUST
  record why in its build-log entry before adding one.
- `SKH-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1897) The owner
  MUST confirm the first adopter before stage 1 starts (confirmed 2026-10-22: core-history; see
  "First adopter"). No stage may leave `experimental` before
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
- `SKH-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2407) A page
  object the import-graph join leaves unbound MUST join through the visit targets of its methods'
  inlined steps, as the AMAP-V0 `page-object-url` basis does: exactly one matching screen joins
  with basis `page-object-url`. Otherwise the page object stays `UNKNOWN` with a closed
  `join_reason` (`page-object-unbound` when no method visits, `page-object-ambiguous` when targets
  match two or more screens, `page-object-unresolved-target` when a target matches none). An
  unresolved target also names its `join_detail`: the AMAP-V0 screen-index reason
  (`no-matching-screen`, `ambiguous-template`, `non-literal-url` or
  `partial-segment-substitution`) or `dynamic-target` for a visit whose URL is not a literal or
  bound parameter. The `pageobjects` report counts `join_reasons`.
- `SKH-V0-016`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2407) A method's
  effect MUST apply the `ANL-V0-035` classes in their precedence to its inlined steps and the setup
  its call graph reaches: `state-effect` when any step fills, selects, checks, uploads or presses or
  any reached unit issues a request; otherwise `unknown` when any step is unclassifiable;
  otherwise `navigation-only`. An `unknown` effect MUST name one closed `effect_reason`: the first
  unclassifiable step's opaque reason, `unclassified-click` for a click not on
  `getByRole('link')`, `focus-or-hover`, `arranges-run-only` for a method that only arranges the
  run (route mocks, init scripts, headers), or `no-ui-actions`. The `pageobjects` report counts
  `effect_reasons`.

### Stage 2: step IR (V1-1899)

- `SKH-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1899) Spec bodies
  and page-object methods MUST normalise into one closed step IR with the kinds `visit`, `click`,
  `fill`, `select`, `check`, `upload`, `wait-for`, `assert` (with matcher and expected value) and
  `opaque`, plus the kinds `SKH-V0-024` adds. An `opaque` step carries a closed `reason` (`parse-unsupported`, `dynamic-value` or
  `unresolved-call`) and the source span it could not read. Each step carries a locator
  reference, parameter slots and a source span, for Playwright, Cypress and Capybara at least.
  Legacy Jasmine cases enter through the DCP-V1 inventory criteria (`DCP-V1-060`..`062`), not
  through a second Jasmine parser.
- `SKH-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1899) Page-object
  calls MUST inline to their IR with arguments bound to parameter slots. An unparseable step MUST
  become an explicit `opaque` step with its span, never dropped. A sequence that holds an opaque
  step is marked incomplete, and a task plan that uses it reads `INCOMPLETE` and never reaches the
  generator. Stage 2 fixtures round-trip an `opaque` step and show that propagation.
- `SKH-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1899) Identical
  step sequences across tests MUST deduplicate into reusable step records with usage counts, each
  citing every source test.
- `SKH-V0-023`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1899) The IR MUST
  be a closed, versioned record. An unknown step kind or field is refused, not coerced.
- `SKH-V0-024`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2406) The closed
  step IR MUST also type `press` (a key sent to a located element or, through
  `page.keyboard.press`, to the `keyboard` target), `focus` and `hover`. `page.keyboard.type` and
  `insertText` are a `fill` on the `keyboard` target; Capybara `hover` and `send_keys` map to
  `hover` and `press`. Every other keyboard or mouse call stays `opaque`. A `press` is a state
  effect and a `focus` or `hover` an unknown effect under `SKH-V0-016`.

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

- `SKH-V0-100`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900) Rails
  setup sources MUST come from the AMAP-V0-114 setup-writer index that the branch query (V1-1891)
  builds over the application roots of the map's `rails-routes/0` routers, not from a second
  reader. Each factory becomes a `factory` record, each trait a `trait` record, each YAML fixture
  file a `fixture-file` record and each test-setup endpoint a `setup-endpoint` record. The
  endpoint record is anchored at its route declaration and produces its handler's model writes.
  `db/seeds.rb` and `db/seeds/**/*.rb` become one `seed` record per file, read with the same
  per-line write scan as endpoints. `produces` lists `entity`, `entity.attribute`, and
  `entity.*` when the request supplies the attributes. Entities use the TSA-V0-102 lower snake
  case. Each record keeps the index's write `basis`. A factory whose `class:` option is unread is
  `UNKNOWN` with reason `unread-factory-class`. A `FactoryBot.create` call is a factory use and is
  never read as a write to a `FactoryBot` model.
- `SKH-V0-101`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900) The report MUST list each SKH-V0-030 source it did not read in `not_read`,
  with the source and a closed reason. A map without a `rails-routes/0` router names
  `factorybot`, `fixture-files`, `seeds` and `setup-endpoints` with reason `no-rails-router`.
- `SKH-V0-102`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900) Each Ruby test in the declared sources MUST link to these records: the
  `factory` and `trait` records that `create`, `build`, `create_list` or `build_list` name in its
  body, and the `fixture-file` records that its file's `fixtures :t, ...` declarations name. A
  name with no record is listed under `unresolved` as `factory:NAME`, `trait:FACTORY:TRAIT` or
  `fixtures:TABLE`. It is never guessed. Ruby `before`, `let` and `let!` bodies are not followed.
- `SKH-V0-103`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900) `flows appmap setup` MUST accept a repeatable `--condition TEXT`: at most
  16, each 1 to 256 bytes on one line and not secret-shaped. Any other value is refused with
  `appmap-invalid-query`. Each condition is answered in `conditions`, matched lexically
  (`SKH-V0-004`) in this order:
  - a record whose literal `target` equals the text, except a `storageState` read (basis
    `target`);
  - for each `Entity.attribute` word, a record producing `entity.attribute` or `entity.*`
    (basis `produces-attribute`);
  - for each other word, singularised, a record producing that entity or fixture name (basis
    `produces-entity`).

  The answer is `candidates`, each with its setup ID, kind, basis and `mocked` label, or
  `unresolved` with reason `no-producing-record`. Every precondition of every flow in the map is
  answered the same way under `preconditions`, naming its flow. `preconditions_total` and
  `preconditions_linked` count them.
- `SKH-V0-104`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900) Playwright setup projects MUST be read from a literal `projects` list in a
  `playwright.config.*` file in the declared sources. A project that another project names in
  `dependencies` or `teardown` is a `setup-project` record. The record keeps its `testMatch` as
  written, the projects it `arranges`, and cleanup `teardown:NAME`. A regular-expression
  `testMatch` with no flag or the `i` flag, or a glob without `**`, binds the record's `requires`
  to the records of the declared files it selects under the config's directory. These cases are `UNKNOWN` instead:
  `dynamic-value` for a non-literal `testMatch`, `no-test-match` when there is none,
  `unsupported-test-match` for any other pattern, and `no-matching-setup-file` when no declared
  file matches. A test under the config's directory and outside the selected files is arranged
  by the setup project when a dependent project does not narrow its tests (no `testMatch`, `testDir`, `testIgnore`, `grep` or
  `grepInvert`).
- `SKH-V0-105`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1900) The setup report profile MUST be `site-knowledge-setup/1`. It adds the Rails
  and setup-project kinds, `basis`, `arranges`, the reasoned `not_read` list and the `conditions`
  and `preconditions` sections. No command reads `/0` or `/1` back, so no `/0` reader is kept. A
  later reader MUST refuse any other version by name (`SKH-V0-007`).
- `SKH-V0-142`: (experimental; #1705 review fix, V1-1900) A setup project MUST select its files
  only under its test directory: its own literal `testDir`, else the config's top-level literal
  `testDir`, else the config's directory, each joined to the config's directory. A `testIgnore`
  on the project or at the top level, a non-literal `testDir`, or a test directory outside the
  repository leaves the record `UNKNOWN` with reason `unsupported-test-filter`; the reader never
  selects files a filter it cannot evaluate might exclude (`SKH-V0-104`).

Implementation status (V1-1900): `flows appmap setup` and the `setup` view of
`corvint.map_knowledge` print `site-knowledge-setup/1`. For Playwright and Cypress TypeScript the
records are:

- `fixture`: each `X.extend({...})` property, with its `test` object and `base`, destructured
  parameters, `scope`, `option`, `auto` and `after-use` cleanup. Its dependencies on other
  fixtures resolve along the extend chain into `requires`. The rest go to `unresolved`, unless
  they are Playwright built-ins.
- `hook`: `beforeEach`, `beforeAll`, `afterEach`, `afterAll`, `before` and `after`.
- `storage-state`: `mode` `reads` for an object-literal `storageState` property, and `writes` for
  `.storageState({ path })`. A literal path keeps its committed blob when the declared sources
  hold it. Contents are never read.
- `route-mock`: `route`, `routeFromHAR` and `cy.intercept`, always `mocked`.
- `api-seed`: `request.<verb>` and `cy.request`.
- `task`: `cy.task`.
- `setup-project`: see `SKH-V0-104`.

A non-literal target is `UNKNOWN` with reason `dynamic-value`. Each record anchors to its span and
names the fixture, hook or helper it is `within`. Each test lists the records that arrange it:

- the fixtures it destructures, and their `requires`;
- the auto fixtures along its test object's chain;
- the hooks and `storageState` of its enclosing blocks;
- the records in its test body or in the helpers it reaches;
- the setup projects of an unrestricted dependent project.

Rails factories, traits, fixture files, seeds and setup endpoints follow `SKH-V0-100`..`102`. The
condition query follows `SKH-V0-103`. Two parts of `SKH-V0-032` remain open:

- V1-1883 seed normalisation is reused only through its TSA-V0-102 name rule. Its reader lives in
  `internal/testsource`, which imports `internal/appmap`.
- TSA-V0 setup-select needs a context index that this report does not build. Setup projects are
  read from the config instead.

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

- `SKH-V0-110`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) `flows appmap links` and the `links` view of `corvint.map_knowledge` MUST print
  `site-knowledge-edges/0`. It lists every map edge with provenance `test-observed` (basis
  `test-sequence`) or `flow-declared` (basis `flow-step`), and every static edge with provenance
  `static`, from this closed set. Static edges are never merged into the map's edges. A static
  edge whose resolved screen pair is also a test-observed edge is marked `observed`, and both
  records are kept. `provenance` counts each class. `ledger` names `flows appmap edges` as the
  only source of `PROVEN` edges (`SKH-V0-042`). Nothing is captured.
- `SKH-V0-111`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) Static edges MUST be read lexically from the committed `.html`, `.htm`, `.erb`,
  `.vue`, `.rb` and JavaScript or TypeScript files in the declared sources. Each link becomes an
  edge of one closed kind:
  - `a-href`: an `href` attribute, except on a `link` or `base` element;
  - `link-to`: a `to` attribute on `Link`, `NavLink`, `router-link` or `RouterLink`;
  - `ui-sref`: a `ui-sref` attribute, with its parameters dropped;
  - `router-link`: a literal `routerLink`, or a bound literal array of absolute segments;
  - `form-action`: the `action` of a `form` element;
  - `link_to` and `button_to`, taking the first argument in the block form and the second
    otherwise;
  - `redirect_to`;
  - `form_with`: `form_with url:` and `form_tag`;
  - `state-go`: `$state.go` and `$state.transitionTo`;
  - `navigate`: `router` or `history` `push`, `replace`, `navigate` or `navigateByUrl`,
    `location.assign` and a bare `navigate(...)` call.

  An attribute is read only when its element opens on the same line, so a script assignment
  such as `const href = '/x'` is not a link. In Ruby, calls are found in masked code and their
  arguments are read from the same line unmasked, up to an unquoted `#`. A script line that
  starts with `//` or `*` is skipped. A link that leaves the application or
  stays on the page is skipped: a URL with any scheme, with or without `//` (`https:`, `mailto:`,
  `tel:`, `javascript:`, `data:`, `vbscript:`, `blob:`, `file:`, in any letter case), or one starting with `//` or `#`. The target is first
  normalised as a URL parser would: leading and trailing C0 controls and spaces are stripped, every
  tab and newline is removed, and a backslash counts as a slash, so ` javascript:`, `java<TAB>script:`
  and `/\host` are skipped too. A template attribute value is also checked after its HTML character
  references decode, so `&#x6A;avascript:` and `javascript&colon;` are skipped (V1-3199).
- `SKH-V0-112`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) The source screen of a static edge MUST come only from Rails conventions,
  through the compiled `GET` routes of the map's `rails-routes/0` routers. A view
  `app/views/CTRL/ACTION.*` that is not a partial binds to the screen of `CTRL#ACTION`. A line
  inside `def ACTION` of `app/controllers/CTRL_controller.rb` binds the same way. In every other
  case `from_status` is `UNKNOWN`, with `from_reason` `source-not-bound`,
  `no-enclosing-action`, `no-screen-for-action` or `ambiguous-action-screen`.
- `SKH-V0-113`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) A literal URL target MUST resolve through the AMAP-V0 screen index, with the same
  reasons when it does not (`no-matching-screen`, `ambiguous-template` and the others). A
  `ui-sref`, `$state.go` or Rails `*_path` or `*_url` helper target resolves by screen state
  name. A relative state (`.child`, `^`) is `UNKNOWN` with reason `relative-state`, and a URL
  that does not start with `/` is `UNKNOWN` with reason `relative-target`. No router dialect is
  added: a target resolves only to screens the map already holds (`SKH-V0-041`).
- `SKH-V0-114`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) These targets MUST be `UNKNOWN` with reason `dynamic-target` at their line
  (`SKH-V0-043`): an interpolated value (`{{`, `${`, `<%`, `#{`), a bound attribute (`:to`,
  `v-bind:`, or `[...]` other than a literal routerLink array), a JSX expression, a non-literal
  call argument, and a Ruby argument that is neither a literal nor a route helper. Files in an
  unsupported template language (`.haml`, `.slim`, `.pug`, `.jade`, `.hbs`, `.handlebars`,
  `.mustache`, `.twig`, `.njk`, `.liquid`, `.svelte`) are named unknowns with reason
  `unsupported-template-language`. They are never read. A secret-shaped literal target
  (`SKH-V0-006`) is `UNKNOWN` with reason `screened-target`, and its text is never printed.
- `SKH-V0-115`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1901) The links report MUST use the site-knowledge file and byte bounds. It refuses
  more than 8192 static links with `appmap-bound-exceeded`, is byte-capped like the other reports,
  reruns byte-identically and writes nothing.
- `SKH-V0-141`: (experimental; #1705 review fix, V1-1901) A Ruby `*_path` or `*_url` helper MUST
  resolve a link target only when the helper name, with at most one balanced argument list, is
  the whole argument. Any trailing expression (`order_path(o) + '/edit'`, a method call on the
  result, a second argument list) or an unbalanced list makes the target dynamic (`SKH-V0-110`).

Implementation status (V1-1901): `flows appmap links` and the `links` view implement
`SKH-V0-040`, `042`, `043` and `110`..`115` over inline committed fixtures (Rails views and
controllers, TSX, an Angular or ui-router HTML template, and a HAML file). For `SKH-V0-041`, no
router dialect is widened: the cloud lane had no adopter repository to measure a need, so that
clause is NOT MET (NEEDS-HOST). The existing dialects keep the `AMAP-V0-042` abstention. The
V1-1191 link-only crawl is not delivered, so observed edges still come only from the map and the
ANL-V0 producer.

### Stage 5: flow inference (V1-1902)

- `SKH-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Step-IR
  sequences across tests MUST cluster deterministically into candidate flows (start screen, steps,
  end screen and outcome) and variations, by shared step-record IDs and screens. Each candidate
  cites its source tests.
- `SKH-V0-051`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Flow
  inference MUST only emit a proposed `corvint-behavior-adapter-request/1` import document on
  standard output and write nothing (`SKH-V0-005`). Candidate flows become intent files only when
  the operator separately runs `flows import` on that document, which writes them marked
  `proposed` (`AFU-V1-004`). They are never accepted intent (invariant 8).
- `SKH-V0-052`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Candidates
  MUST join DCP-V1 test-knowledge records (`DCP-V1-086`..`092`) and honour each join's verdict
  and selection (`DCP-V1-090`). A joined `defect` (selection `blocks`, at any trust level) blocks
  every candidate flow and oracle that would use that test, and the block is reported. A joined
  `unrelated` verdict removes the test as a step or oracle source. The remaining sources rank by
  trust: `candidate` selections (`run+reviewed` with verdict `bound` or `partial`) first, then
  `run-unreviewed`, then unreviewed tests. A `partial` verdict's gaps are carried to the flow
  and to stage 6 oracle selection as named gaps. High trust never overrides a defect, and trust
  is labelled, never promoted.
- `SKH-V0-053`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Each
  candidate MUST report its support (distinct source tests) and its conflicts (tests that diverge
  at a step). A candidate with one source test is labelled `single-source`.
- `SKH-V0-054`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Stage 5
  evidence MUST include a read-path test showing that inference and `corvint how` leave the
  worktree, index, refs and `.corvint/` unchanged apart from the invariant 4 ledgers, and an
  import round-trip fixture whose emitted document, imported by `flows import`, yields `proposed`
  intents equal to the candidates. Fixtures MUST show a `run+reviewed` test with a `defect` verdict
  blocking its flow instead of ranking first, and an `unrelated` test never used as a source.

- `SKH-V0-120`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) `flows appmap infer` and the `flow_candidates` view of
  `corvint.map_knowledge` MUST print `site-knowledge-flows/0`. The report lists `candidates` and
  `unclustered` tests, counts candidates by status, single-source candidates, conflicts and
  unclustered reasons, and writes nothing. Each candidate has learned authority and the anchor
  of its primary source test.
- `SKH-V0-121`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Each stage 2 test MUST be read into action segments. A run of consecutive
  steps inlined from one page-object method or helper is one `call` segment, and cites the
  stage 2 record when the run equals one. Every other step that is not an assertion or a wait is
  its own segment. The start screen is the first `visit` target's screen; the end screen is the
  last `visit` target's screen. Each resolves through the AMAP-V0 screen index. Tests with the
  same start and end screen form one candidate. Within it, tests with the same segment sequence
  form one variation. The candidate's steps are the primary variation's steps. A test that
  cannot be placed is listed as `unclustered` with reason `no-start-screen`,
  `start-screen-unresolved` (and the index reason) or `end-screen-unresolved`.
- `SKH-V0-122`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) A variation's outcomes MUST be the assertions after the last action that every
  source test of the variation makes. A candidate's outcomes are those every variation shares.
  A candidate with none has `outcome_status` `UNKNOWN`. A candidate with an opaque step in any
  source is `incomplete` (`SKH-V0-021`).
- `SKH-V0-123`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Each source test MUST join every test-knowledge record whose test anchor
  shares its path and overlaps its span, and every joined record applies. The rules are:
  - Any joined `unrelated` verdict removes the test, whatever else is joined, which is listed as `unclustered` with reason
    `unrelated-verdict` and the record ID.
  - Any joined selection `blocks` makes the candidate `blocked`, with the record IDs in
    `blocked_by` and the source labelled `blocked`, whatever its trust level.
  - Sources rank `candidate` (selection `candidate`), then `run-unreviewed`, then `unreviewed`.
  - The primary variation is the one with the best-ranked source, then the most support, then the
    lowest ID.
  - A `partial` verdict's gap is carried in `gaps` as `ID: gap`.
  - Each source keeps its trust level, verdict and selection as labels. They are never promoted.
- `SKH-V0-124`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) Each candidate MUST report `support`, the distinct source tests, and mark
  `single_source` when there is one. For every non-primary variation it reports a conflict: the
  step index where that variation diverges from the primary, the diverging action (or
  `end-of-sequence`), and its tests.
- `SKH-V0-125`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) `--emit request` MUST print one `corvint-behavior-adapter-request/1` holding
  every candidate whose `emit` is `emitted`. A candidate is emitted only when it is `proposed`
  and every variation has an outcome with a value: the literal expected value (not one that
  interpolates an unbound parameter slot, which is marked `slotted`), or the `DCP-V1-084`
  canonical value of a valueless state matcher. Otherwise `emit` is the candidate
  status or `no-emittable-outcome`. Each emitted candidate becomes one proposed intent:
  - flow ID `sk-` plus 12 hex digits of the candidate ID;
  - derivation `source-derived`, with the primary test's anchor as evidence;
  - required pages: the start and end states;
  - per variation, its actions (a repeat is numbered `#N`), outcomes, the outcome text as
    observable facts, project `unknown`, and inferred test links.

  The envelope's migration, discovery and empty candidate and test inputs carry inline anchors
  with the reason `site-knowledge flow inference output; generated, not committed, nothing ran`.
  The discovery input lists no executions. The request is checked by the DCP-V1 adapter before it
  is printed, and is refused with `appmap-invalid-query` if the adapter rejects it. It is bounded
  by the corpus byte limit. `flows import --format behavior-adapter-request` on it writes
  `proposed` intents only (`SKH-V0-051`).
- `SKH-V0-126`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1902) `--corpus ARTIFACT` MUST open a built corpus artifact with the corpus reader,
  which re-derives it against `--root`. An artifact built at another revision than the map is
  refused with `appmap-invalid-query`. Only `joined` records whose test anchor names the corpus
  repository are read, and at most `MaxTestKnowledge` of them. The MCP `flow_candidates` view
  reads no corpus. A missing or unreadable artifact, an `--emit` other than `report` or
  `request`, and every other flag error exit 2.
- `SKH-V0-140`: (experimental; #1705 review fix, V1-1902) An outcome's locator MUST keep the full
  selector identity: `kind=value`, plus ` name=NAME` when the selector has an accessible name.
  Two assertions that differ only by name are distinct outcomes, so neither a shared outcome nor
  an exported assertion is weakened to the bare role (`SKH-V0-122`, `SKH-V0-125`).

Implementation status (V1-1902): `flows appmap infer`, its `--emit request`, and the
`flow_candidates` MCP view implement `SKH-V0-050`..`054` and `120`..`126` over inline committed
fixtures. The corpus join is tested at its conversion boundary (`trustFromJoins`), with trust
supplied directly to the inference. No fixture builds a full corpus artifact with test knowledge.

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
- `SKH-V0-143`: (experimental; V1-1903) `corvint flows appmap how --map FILE --source DIR...
  [--condition TEXT]... [--ledger FILE] [--corpus ARTIFACT] [--budget N | --full] GOAL` and the
  `how` view of the `corvint.map_knowledge` MCP tool (`goal`, optional `conditions`) MUST print one
  `site-knowledge-how/0` answer: the best candidate flow from the stage 5 inference, its goal
  screen (the flow's end screen with its router anchor), its start screen, the setup and
  conditions, the steps, the path and the oracle. The MCP view reads no corpus and no ledger.
- `SKH-V0-144`: (experimental; V1-1903) The answer MUST carry one of `COMPLETE`, `INCOMPLETE`,
  `STALE` or `ABSTAINED` and a list of gaps sorted by code, each with one closed code from
  `SKH-V0-062`, a detail and an anchor when one exists. `COMPLETE` means no gap. No candidate
  covering any goal term is `ABSTAINED` with `no-route` and no step.
- `SKH-V0-145`: (experimental; V1-1903) The steps MUST be the primary variation's first source
  test's actions up to its last action, each with its kind, action, via, the screen it runs on
  and its source anchor. Each visit that changes screen MUST add a hop from the previous screen
  with basis `edge` when an admitted edge joins them and `visit` otherwise. A visit that resolves
  to no screen is a `no-route` gap, and an opaque step is a gap: `no-page-object` for an
  unresolved call and `no-route` otherwise.
- `SKH-V0-146`: (experimental; V1-1903) A hop's admitted edges MUST be the stage 4 static edges,
  the map's test-observed and flow-declared edges and, with `--ledger`, the `PROVEN` rows of an
  application-edge-ledger/0 built from this map, each with its provenance (`static`,
  `test-observed`, `flow-declared` or `ledger`), anchor or ledger digest and receipt test. A ledger
  for another app or map digest, a trimmed ledger or one whose digest does not match its content is
  refused with `appmap-invalid-query`.
- `SKH-V0-147`: (experimental; V1-1903) The setup MUST list the stage 3 records that arrange the
  primary test with their anchors and labels (mocked kept). An unresolved setup record arranging it
  and each `--condition` with no producing record (`conditionQuery`) are
  `no-setup-for-precondition` gaps; each condition is answered with its candidates.
- `SKH-V0-148`: (experimental; V1-1903) The oracle MUST be the trailing assertions of the primary
  test's outcome that every source of the variation shares (`SKH-V0-122`), each with matcher,
  locator, expected value and anchor; no shared outcome, or a candidate blocked by test knowledge,
  is a `no-oracle` gap. The oracle is never synthesised.
- `SKH-V0-149`: (experimental; V1-1903) A tie for the highest coverage MUST be `ABSTAINED` with
  `ambiguous-goal`, listing every candidate in rank order with its coverage, and no steps.
- `SKH-V0-150`: (experimental; V1-1903) The answer MUST be labelled a proposal with learned
  authority, never accepted intent, MUST be byte-identical across reruns for the same inputs and
  revision, and the read MUST change no repository file.
- `SKH-V0-151`: (experimental; V1-1903) Freshness MUST be evaluated over every cited anchor at the
  evaluated revision. A stale anchor makes the answer `STALE` with a `stale-evidence` gap; freshness
  that cannot be established is a `stale-evidence` gap with detail `freshness-unknown`.
- `SKH-V0-152`: (experimental; V1-1903) A goal MUST be 1 to 256 bytes of one line with no
  secret-shaped text, and conditions keep the stage 3 bounds; otherwise `appmap-invalid-query`.
  The answer fits the byte budget by trimming its sections (setup, conditions, steps, path,
  candidates) with omitted counts while the head stays whole, or refuses with
  `appmap-budget-too-small`.

Implementation status (V1-1903): `flows appmap how` and the `how` MCP view implement
`SKH-V0-060`..`064` and `143`..`152` over inline committed fixtures. The top-level `corvint how`
alias named in `SKH-V0-060` is not delivered: it needs the command registry, help and capability
catalog changes for a new top-level command, so it stays a follow-up. Resolution reuses the stage 5
candidate flows and the AMSP-V0 lexical resolver (`resolve`) rather than a second planner;
navigation composition (`AMSP-V0-006`) is not reused, because the path is the primary test's own
visits with admitted edges, not a composed route.

### Stage 7: generator (V1-1904)

- `SKH-V0-070`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1904) From a
  `COMPLETE` task plan, the V0 generator MUST emit a Playwright TypeScript test only, in the target
  suite's conventions. It reuses existing Playwright page objects, setup records and fixtures, and
  places the test beside the closest existing Playwright spec (the `AMAP-V0-013` placement rule).
  A plan with any step, setup or oracle mined only from another framework (Cypress, Capybara or
  RSpec, Jasmine) MUST abstain with `source-framework-unsupported`, naming the element, because
  copying across frameworks would be synthesis (`SKH-V0-072`). The RSpec provider's `preview`,
  `promotable: false` result never substitutes for stage 8 proof. This scope is an orchestrator
  decision under the owner's delegation of in-task forks (2026-10-05); the owner may revise it.
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
  skeleton) and the `corvint.map_plan` MCP tool rather than a parallel surface. Core always emits
  the guarded draft; only the stage 8 companion removes the guard, under `SKH-V0-088`.
- `SKH-V0-074`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1904) Generation
  MUST be byte-identical across reruns for the same plan and revision, and it writes only to an
  output path the caller names explicitly.

- `SKH-V0-153`: (experimental; V1-1904) With `--draft`, the how answer MUST carry a `draft` head
  field whose lines copy the primary test's file at the map revision line for line, in order,
  without the lines of the file's other mined tests, so imports, `test.use`, `describe` wrappers
  and the test body keep the suite's own conventions. Parameter-slot substitution is the identity
  in V0: no value is rewritten, so nothing is synthesised (`SKH-V0-072`). A file whose blob differs
  from the mined test's anchor between reads is refused.
- `SKH-V0-154`: (experimental; V1-1904) The draft MUST insert exactly one `test.fixme(true, ...)`
  guard line directly after the primary test's declaration line and record its line number, so
  the draft never runs as an executed test until the stage 8 companion removes that one line
  (`AMSP-V0-008`, `SKH-V0-088`).
- `SKH-V0-155`: (experimental; V1-1904) The draft's proposed path MUST sit in the primary test's
  directory, named `how-<first 12 hex of SHA-256 of the goal>` with the test file's spec suffix
  (`.spec.ts`, `.test.ts`, `.e2e.ts` or its extension), so the suite's test match selects it.
- `SKH-V0-156`: (experimental; V1-1904) Each answer gap MUST become a marked
  `// TODO <gap code>: <detail>` line: an opaque step's TODO precedes the copied call it names,
  other answer gaps follow the guard. A draft with any TODO line or gap reads `INCOMPLETE`; only a
  `COMPLETE` answer with none reads `COMPLETE`.
- `SKH-V0-157`: (experimental; V1-1904) A source line that is secret-shaped (`secretscreen`) or
  lies in a screened step MUST NOT be copied; it becomes a `secret-screened` TODO naming only its
  path and line, and the draft reads `INCOMPLETE`.
- `SKH-V0-158`: (experimental; V1-1904) A condition with setup candidates MUST become an
  `arrange` TODO naming the candidate setup IDs, with gap `no-setup-for-precondition`
  (`condition-not-arranged`); the generator never writes setup code. Setup the primary test
  already uses is present only as the copied lines that use it.
- `SKH-V0-159`: (experimental; V1-1904) An `ABSTAINED` answer MUST draft no lines and carry the
  answer's gaps. A primary test that is not Playwright in a JavaScript or TypeScript file, or a
  setup it uses that is anchored in a non-JavaScript file, MUST abstain with
  `source-framework-unsupported`, naming the test or setup and its anchor (`SKH-V0-070`).
- `SKH-V0-160`: (experimental; V1-1904) Each draft line MUST carry one evidence row: `copied` with
  path, source line and blob; `guard`; or `todo` with its gap code (`SKH-V0-071`). The draft's
  digest is the SHA-256 of its lines joined by newlines with a final newline. The answer and draft
  are byte-identical across reruns, and the read writes nothing: the caller or the stage 8
  companion writes the bytes to a path it names (`SKH-V0-074`).
- `SKH-V0-161`: (experimental; V1-1904) `corvint flows appmap plan --map FILE --request GOAL
  --draft --source DIR...` with optional `--condition`, `--ledger`, `--corpus` and `--budget N` or
  `--full` MUST print the how answer with its draft (`SKH-V0-073`). `--source` without `--draft`
  or `--request`, with `--step`, `--receipt`, `--bind`, `--revision` or more than one `--map`, and
  `--condition`, `--ledger` or `--corpus` without `--source` exit 2.
- `SKH-V0-162`: (experimental; V1-1904) The `corvint.map_plan` MCP tool MUST accept `sources`,
  optional `conditions` and `app` with `request` (one goal of at most 256 bytes) and `draft: true`,
  and return the same bytes as the library draft, framed as untrusted data. `sources` without
  `draft`, with `steps` or `revision`, and `conditions` or `app` without `sources` are invalid
  arguments.

Implementation status (V1-1904): `SKH-V0-070`..`074` and `153`..`162` are implemented in
`internal/appmap/site_knowledge_draft.go` over inline committed fixtures. The corpus join and edge
ledger stay CLI-only over MCP, as for the how view. Placement uses the primary test's directory,
which is the closest existing spec by construction; reuse of page objects, setup and fixtures is
the copied lines' own imports and calls.

### Stage 8: proof pipeline (V1-1905)

- `SKH-V0-080`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) One
  companion command MUST take a generated test through every condition of "confidently". It runs
  the test live through the js-live-test-provider Playwright adapter and PWP-V0 at a pinned
  application revision, and applies TSA-V0 validity (`TSA-V0-073`..`081`). It binds steps,
  selectors and methods to run evidence, with RVN-V0 widened from steps to selectors and methods.
  It runs the NEA-V0 and BBF-V0 negative controls and the `CCE-V0-009` discrimination verdict
  through the V1-1733 prover. The command emits one acceptance receipt.
- `SKH-V0-081`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) The receipt
  MUST read `PROVEN` only when all six conditions of "confidently" hold, and `EXECUTION_VERIFIED`
  when conditions 1 to 5 hold and 6 does not. Otherwise it names the first failing condition with
  its evidence, using one closed code: `generate-incomplete`, `run-failed`,
  `validity-unestablished`, `validity-defect`, `acceptance-blocked`, `binding-unverified`,
  `control-survived`, `not-discriminating`, `discrimination-inconclusive` or `infrastructure`. An
  `EXECUTION_VERIFIED` receipt names `oracle-unaccepted` or `oracle-mismatch`.
- `SKH-V0-082`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) The live run
  MUST repeat as in `NEA-V0-002`, with retries at zero; a flaky result is never `PROVEN`. The
  receipt MUST carry the complete applicable NEA-V0 assessment (`NEA-V0-004`, `NEA-V0-006`):
  association, hygiene, freshness and execution as the provider observed them, strength derived
  from matched controls, and cleanup facts. Any unknown required axis blocks with
  `acceptance-blocked`: request `/0` with freshness `UNKNOWN` is never accepted, request `/1` needs
  its observed PTF closure, and unknown cleanup blocks. Every blocked axis and its reason stays in
  the receipt.
- `SKH-V0-083`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) The pipeline
  MUST run in existing companions (`corvint-test`, `corvint-tests-accept`,
  `corvint-js-test-provider`). Core `corvint` reads only the receipt.
- `SKH-V0-084`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) Stage 8 MUST
  be qualified on the confirmed real adopter with the preregistered request set, reporting proven,
  abstained and failed counts, unanchored statements and rerun byte identity.
- `SKH-V0-085`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) The
  discrimination verdict MUST come from the V1-1733 prover applying the `CCE-V0-009` ladder, with
  the pinned application revision as head and a seeded mutant of it as base. Only `DISCRIMINATES`
  and `NEW_REACH` qualify, and only when the receipt names every mutant and control identity with
  its observed result. `PRESERVES` reads `not-discriminating`, `CONTROL_SURVIVED` reads
  `control-survived`, `REGRESSION_DETECTED` and `SCENARIO_BROKEN` read `run-failed`, and
  `INCONCLUSIVE`, `UNSTABLE`, a missing verdict or a prover that did not run read
  `discrimination-inconclusive`. Until the V1-1733 prover contract is accepted, nothing reads
  `PROVEN`. Stage 8 fixtures cover every ladder verdict.
- `SKH-V0-086`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) Validity
  MUST be established positively, never inferred from an empty finding list. `corvint tests
  validity` (`TSA-V0-081`) MUST return exactly one located example for the generated test, at its
  callee line in the candidate's exact blob, with hygiene `ELIGIBLE` after `ApplySourceFindings`
  and no defect finding. An `uncertain` file, an `ABSTAINED` example, a missing example, more
  than one matching example or a read of other bytes MUST block with `validity-unestablished`; a
  defect finding reads `validity-defect`. Stage 8 fixtures cover each blocking case.
- `SKH-V0-087`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) Before a
  request runs, it MUST be bound to an accepted outcome oracle naming the goal screen, locator
  reference, matcher and expected value. The oracle is either recorded by the owner in the frozen
  request set or taken from an AFU-V1 flow intent accepted by a reviewed commit; a learned or
  proposed oracle never qualifies. The receipt reads `PROVEN` only when the generated test's
  outcome assertion equals that oracle on all four fields. Otherwise it reads at most
  `EXECUTION_VERIFIED`, with `oracle-unaccepted` when no accepted oracle exists and
  `oracle-mismatch` when the assertion differs. A passing, discriminating test for a different
  goal is a required negative fixture and MUST NOT read `PROVEN`.
- `SKH-V0-088`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) Only the
  stage 8 companion MAY turn the guarded draft into an executable candidate, by one recorded
  transform that removes the `AMSP-V0-008` `test.fixme` guard and changes nothing else. The
  receipt records the draft digest, the transform and the candidate's SHA-256. Validity, every
  repeat, every control and the prover MUST run on those exact candidate bytes, and the final
  artifact is those same bytes, written only to a path the caller names; any byte difference voids
  the receipt. The guarded draft never counts as an executed test: a fixme or skipped run of it
  reads `run-failed`, which a fixture shows.
- `SKH-V0-089`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1905) A selector
  or method MUST read verified only with a behavioural witness from a passing repeat of the live
  run: a recorded action whose resolved locator is that selector's locator expression, or a
  recorded call whose call site lies in that method's anchored span, joined to the element by ID,
  revision and receipt under RVN-V0. Presence in a passing test's source is not a witness, and a
  step's `VERIFIED` fact never verifies its selectors or methods (`AMSP-V0-007`). The stage amends
  PWP-V0, RVN-V0 and AMSP-V0 to carry these observations in the same change; until then each such
  element stays `unverified` and the receipt reads `binding-unverified`. Fixtures cover a method
  never called, a branch not taken and a locator never exercised; each stays `unverified`.
- `SKH-V0-163`: (experimental; V1-1905) The stage 8 pipeline MUST live in
  `internal/skhproof`, linked into the `corvint-test` companion; neither core `corvint` nor
  `corvint-corpus-mcp` links it, so core can only read its receipt (`SKH-V0-083`).
- `SKH-V0-164`: (experimental; V1-1905) The one recorded transform MUST remove exactly the draft's
  guard line, which must equal the `SKH-V0-154` guard, and change nothing else. The transform
  records its kind `remove-guard-line`, the removed line number and SHA-256, the draft digest, and
  the candidate's SHA-256 and Git blob ID, which is the identity TSA-V0 validity reads
  (`SKH-V0-088`).
- `SKH-V0-165`: (experimental; V1-1905) A draft that is not `COMPLETE` has no candidate and reads
  `generate-incomplete`. A draft whose digest does not match its lines, whose guard line is not the
  guard, or which holds a second `test.fixme`, is refused with `skh-answer-invalid`.
- `SKH-V0-166`: (experimental; V1-1905) V0 reads recorded live-run evidence
  (`site-knowledge-proof-evidence/0`) and starts no browser. With no evidence the receipt reads
  `infrastructure` with `live-run-not-recorded` and evidence `none`. Evidence naming other bytes
  than the candidate's SHA-256, or a recorded infrastructure failure, reads `infrastructure`: a
  byte difference voids the receipt (`SKH-V0-088`).
- `SKH-V0-167`: (experimental; V1-1905) Condition 1 MUST hold only for 2 to 100 repeats
  (`NEA-V0-002`) that all read `passed` with retries zero, at a pinned application revision. A
  `fixme` or `skipped` repeat, a retried or failed repeat, or no revision reads `run-failed`.
- `SKH-V0-168`: (experimental; V1-1905) Condition 2 MUST hold only for exactly one `LOCATED`
  example with a callee line, in the candidate's exact Git blob, with hygiene `ELIGIBLE`, in a
  file that is not uncertain and has no defect finding. A defect reads `validity-defect`; any other
  shortfall reads `validity-unestablished` (`SKH-V0-086`).
- `SKH-V0-169`: (experimental; V1-1905) Condition 3 MUST hold only when the association, hygiene,
  freshness, execution and cleanup axes each read `OBSERVED`. Otherwise the receipt reads
  `acceptance-blocked` and lists each blocked axis with its value, a missing axis as `UNKNOWN`
  (`SKH-V0-082`).
- `SKH-V0-170`: (experimental; V1-1905) Until the `SKH-V0-089` PWP-V0, RVN-V0 and AMSP-V0
  amendments land, every recorded selector or method element MUST read unverified whatever witness
  is recorded, as MUST a step with no witness, an element of another kind, and an empty binding
  list. Any unverified element reads `binding-unverified` and is listed in the receipt.
- `SKH-V0-171`: (experimental; V1-1905) The receipt MUST read `control-survived` unless at least
  one approved negative control is recorded and every control reads `killed`; each control's
  identity and result stay in the receipt.
- `SKH-V0-172`: (experimental; V1-1905) The prover verdict MUST map by the `SKH-V0-085`
  allowlist; `DISCRIMINATES` or `NEW_REACH` naming no mutant reads `discrimination-inconclusive`.
  While the V1-1733 prover contract is unaccepted, every qualifying verdict also reads
  `discrimination-inconclusive` with `prover-contract-unaccepted (V1-1733)`, and the receipt
  carries `prover_accepted: false`.
- `SKH-V0-173`: (experimental; V1-1905) Condition 6 MUST compare the candidate's recorded outcome
  assertion with an accepted oracle (`site-knowledge-accepted-oracle/0`, origin
  `owner-request-set` or `afu-v1-flow-intent`, with its accepting commit) on screen, locator,
  matcher and expected value. No oracle, another origin or no commit reads `EXECUTION_VERIFIED`
  with `oracle-unaccepted`; any field difference or a missing assertion reads
  `EXECUTION_VERIFIED` with `oracle-mismatch` (`SKH-V0-087`).
- `SKH-V0-174`: (experimental; V1-1905) `corvint-test e2e how-accept --answer FILE [--write PATH]
  [--evidence FILE] [--oracle FILE]` MUST print the receipt. It writes the candidate only with
  `--write`, only by exclusive creation of that caller-named path; an existing file or a symlink is
  refused with `e2e-out-unsafe`, and `--write` for a draft that is not `COMPLETE` is refused with
  `generate-incomplete`. It runs no test and writes nothing else.
- `SKH-V0-175`: (experimental; V1-1905) Recorded evidence and oracles MUST decode strictly: the
  exact schema, no unknown field, at most 1 MiB, and at most 256 entries in each recorded list;
  anything else is refused with `skh-evidence-invalid` or `skh-oracle-invalid`.
- `SKH-V0-176`: (experimental; V1-1905) The receipt (`site-knowledge-acceptance/0`) MUST be
  canonical bytes with a final newline, the same for the same inputs, and MUST carry the evidence
  origin (`recorded` or `none`), the transform, the application revision, each assessed condition
  with its code, blocked axes, unverified elements, the verdict, mutants and controls.

Live-run recording (V1-2595, V1-1905):

- `SKH-V0-180`: (experimental; V1-2595) `skhproof.Record` MUST map the live run's own outputs for
  the answer's candidate into `site-knowledge-proof-evidence/0`: the hosted `e2e-prove/0` scenario
  whose placement is the draft's proposed path (`CCE-V0-034`..`036`), the
  `corvint-test-source-validity/0` report for that path, and the candidate's committed bytes at the
  head revision. It copies observed facts only: the head outcomes and the report's retries as runs,
  the scenario's controls and verdict, the qualified base revision as the one mutant, and the
  committed bytes' SHA-256 as the candidate digest. It records `execution` as `OBSERVED` only for a
  `DETERMINISTIC` head whose every outcome passed with no retry, and records no other axis. A
  missing or ambiguous hosted scenario, an unqualified head receipt, a head whose application and
  test revisions differ from the report's head, or a candidate it cannot read back is recorded as
  `infrastructure` with its reason; a draft that is not `COMPLETE` is refused with
  `generate-incomplete` and a report of another schema with `skh-evidence-invalid`.
- `SKH-V0-181`: (experimental; V1-2595) The recorded validity MUST be the candidate file's reading:
  its uncertainty, each `invalid`-severity finding's rule as a defect, and each example at that
  path with its line, hygiene state and the Git blob of the bytes at the report's revision. An
  example is `LOCATED` only when the report carries its `TSA-V0-081` strength axis; any other example
  records its hygiene state, so `SKH-V0-168` reads it as unestablished.
- `SKH-V0-182`: (experimental; V1-2595) The recorded bindings MUST list every draft step and every
  page-object method a step calls, each with no witness: the prove report carries no per-element
  behavioural witness, so `SKH-V0-170` reads them `binding-unverified` until `SKH-V0-089` lands.
- `SKH-V0-183`: (experimental; V1-2595) The recorded outcome assertion MUST be the answer's goal
  screen with its one oracle; with no goal screen or not exactly one oracle there is none, so
  `SKH-V0-173` reads `oracle-mismatch`.
- `SKH-V0-184`: (experimental; V1-2595) `corvint-test e2e how-evidence --answer FILE --prove FILE
  --validity FILE [--root DIR]` MUST print the recorded evidence, reading committed bytes only
  through `git cat-file` at `--root` (at most 1 MiB), running no test and writing no file. An
  unreadable answer is refused with `skh-answer-invalid` and an unreadable report with
  `skh-evidence-invalid`.
- `SKH-V0-185`: (experimental; V1-2641) A step whose innermost `via` is a method of a page object
  joined to a screen (`page-object:PATH#Class.method` or `page-object:PATH.func`, a resolved join
  with a screen) MUST run on that screen. In stage 5 such an action step (any kind but an
  assertion or a wait, which `how` does not place either), after the first visit and at or before
  the last action, moves the end screen to it (`SKH-V0-121`), so a test that navigates
  through page objects ends where its last such step runs, not at its last `visit`. In `how` such
  an action step records that screen, also before the first visit, and, when it changes a known
  screen, adds a hop with basis `edge` when an
  admitted edge joins the screens and `page-object` otherwise (`SKH-V0-145`); an opaque step keeps
  its gap. A step whose innermost `via` is an unjoined page object keeps the previous screen and
  never takes an outer page object's screen.
- `SKH-V0-186`: (experimental; V1-2643) `skhproof.Record` MUST read the hygiene axis of
  condition 3 (`SKH-V0-169`) from the TSA validity reading, and only when that report holds
  exactly one example at the candidate path, in a file that is not uncertain, whose Git blob is
  the bytes the run executed at head: hygiene is `OBSERVED` when that example's hygiene state is
  `ELIGIBLE`, and otherwise stays unrecorded and reads `UNKNOWN`.
- `SKH-V0-187`: (experimental; V1-2643) Association, freshness and cleanup MUST stay unrecorded,
  and so `UNKNOWN`, until a source observes them. The `e2e-prove/0` report's head outcomes carry
  a state per repeat and a location only on failure, so no passing outcome names the file, line
  or title the runner executed; the hosted obligation and the outcome count are the prover's
  request and a count, not an observed test identity, and are never read as association. The
  report carries the prover's staged test-tree digest and revisions, which are pins, not a
  runner-observed per-test source join, and `NEA-V0-004` with the PTF profile says a pin never
  promotes freshness. It carries no owned-group or descendant cleanup fact (`NEA-V0-006`). The
  named sources that would close them are an NEA request `/1` assessment of the candidate
  (runner-observed test identity, `corvint-playwright-freshness/0` closure and per-worker
  cleanup facts) or an `e2e-prove` report amendment that records all three per outcome; until one
  lands, condition 3 blocks with `association=UNKNOWN, freshness=UNKNOWN, cleanup=UNKNOWN`.
- `SKH-V0-188`: (experimental under the standing lane auto-accept; V1-2650) A `how` goal resolved
  by terms (`SKH-V0-061`, not an explicit `flow:` id) MUST be checked against each goal term the
  chosen candidate does not carry. When another candidate carries such a term and its weight is at
  least the weight of the lightest goal term the choice carries, the answer MUST be `ABSTAINED`
  with one `no-route` gap whose detail is `unmatched-goal-term: ` and those terms, listing the
  ranked candidates and no steps, even though the coverage exceeds `minCoverage`. When no
  candidate carries such a term, the choice stands but each such term MUST add a `no-route` gap
  with detail `unmatched-goal-term: TERM`, so the answer is never `COMPLETE`. A lacked term that
  another candidate carries but that weighs less than every carried term changes nothing. The
  draft (`SKH-V0-153`) inherits the answer. This amends `SKH-V0-144` and `SKH-V0-061`.
  - Non-goals: no change to the AMSP-V0 `plan --step` resolver, to `minCoverage`, to the term
    weights or stemming, to which text describes a candidate (the router file path in a screen
    state still yields terms such as `area`), or to the closed gap codes; no synonym or semantic
    matching.
  - Failure modes: a goal with a common filler word another candidate carries, heavier than the
    choice's lightest term, abstains where it once answered; an unknown filler word makes a
    correct answer `INCOMPLETE`. Both fail closed (invariant 2). A tie in weight refuses.
  - Rollback: revert the `outweighed` check in `internal/appmap/site_knowledge_how.go` and the
    candidates now kept on a chosen `resolve` result in `internal/appmap/plan.go`.
- `SKH-V0-189`: (experimental under the standing lane auto-accept; V1-2847, #1905) The source
  verbs `refs`, `pageobjects`, `steps`, `setup`, `links`, `infer` and `how` MUST read a
  `--source ALIAS:DIR` from the root the map pinned under `ALIAS` (`AMAP-V0-017`), at the revision
  the map recorded for it, from committed objects only, with that root's checkout named by a
  repeatable `--repo ALIAS=ABSOLUTE_ROOT`; a `--source DIR` without a valid alias prefix stays a
  directory of `--root` at the map revision. Answers over the aliased root MUST equal the answers
  over the single-root map of the same files apart from the map pin and the echoed sources, and a
  `how` draft MUST name the root of the copied test in `proposed_repo`. `refs` MUST read the map's
  test files of every bound alias. Typed refusals: an alias the map did not read, or whose root is
  not bound, or whose root lacks the pinned commit, is `appmap-root-unavailable` naming the alias; a
  `DIR` that is not a tree at the pinned revision is `appmap-invalid-query` naming the root; a
  `--source DIR` that is missing under `--root` but names an alias the map read adds the remedy
  `pass --source ALIAS:DIR with --repo ALIAS=ABSOLUTE_ROOT`; one path read from two roots, or one
  map test path joined in two roots, is `appmap-invalid-query`; `infer --emit request` over an
  aliased root is `appmap-invalid-query`, because its request carries no repository identity.
  - Non-goals: no `--source ALIAS:DIR` for `flows appmap plan --draft` or `corvint.map_plan`
    (an alias there refuses as an unbound root); no merging of two roots' files at one path; Rails
    setup and view screens stay read from `--root`; a `--root` directory whose name has a colon and
    a valid alias before it cannot be named (it parses as an alias); freshness of an aliased anchor
    stays unknown (`AMAP-V0-020`), so a `how` answer citing one is at most `INCOMPLETE`.
  - Failure modes: an operator who binds a different checkout of the aliased repository gets the
    pinned commit's content or a refusal, never its working tree; a map whose aliased root moved
    on refuses until the pinned commit is fetched there. Both fail closed (invariant 2).
  - Rollback: revert `internal/appmap/sourceroots.go`, the `Repos` option and the `--repo` flag in
    `cmd/corvint/flows_appmap_knowledge.go`; single-root `--source DIR` behavior is unchanged.
- `SKH-V0-329`: (experimental under the standing lane auto-accept; V1-2927, #1933) A map whose
  manifest lives in an aliased root (`--manifest-repo ALIAS`, `manifest.repo`) MUST be read by
  `links`, `setup` and `how` from that root at the revision the map pinned for it, from committed
  objects only, through the source verb's `--repo ALIAS=ROOT` or workspace binding (`WSD-V0-145`),
  and its blob MUST equal the map's manifest anchor or the verb refuses as
  `appmap-invalid-revision` naming the root. An unbound alias refuses as `appmap-root-unavailable`
  naming the alias and `pass --repo ALIAS=ABSOLUTE_ROOT`. `actions`, `blocker` and `branches`
  bind no aliased root, so they refuse such a map as `appmap-root-unavailable` naming the root that
  holds the manifest. Routers and Rails sources stay read from `--root`.
- `SKH-V0-330`: (experimental under the standing lane auto-accept; V1-2927, #1933) The source verbs
  MUST parse their flags in any order, `how`'s also after its GOAL, and a refusal of the arguments
  MUST name what failed before the usage text: the flag that did not parse (with its value error),
  an unexpected positional argument, a missing `--map` or a missing GOAL.

Implementation status (V1-1905): `SKH-V0-163`..`176` are implemented offline in
`internal/skhproof` and `corvint-test e2e how-accept` over inline test fixtures, so
`SKH-V0-080`, `081`, `083`, `085`..`088` hold over recorded evidence. The live run of `SKH-V0-080`
and `082` is NEEDS-HOST: neither the lane host nor its WSL2 runner carries the pinned tuple
(Playwright 1.64.0, Node v24.11.1, headless shell 1248), so no evidence file is recorded and every
shipped receipt reads `infrastructure`. The `SKH-V0-089` witness amendments are not made, so
selectors and methods read `binding-unverified`; the V1-1733 prover contract is unaccepted, so
nothing reads `PROVEN`; and the `SKH-V0-084` adopter qualification is NOT_RUN.

Implementation status (V1-2595, 2026-10-25): `SKH-V0-180`..`184` are implemented in
`internal/skhproof/record.go` and `corvint-test e2e how-evidence`. One live run under the pinned
tuple (darwin arm64; Node v24.11.1, Playwright 1.64.0, headless shell 1248) on a synthetic
ui-router fixture recorded evidence for one generated draft, and `how-accept` read it past
`infrastructure`: conditions 1 and 2 hold and condition 3 blocks with `acceptance-blocked`
(association, hygiene, freshness and cleanup `UNKNOWN`), because no recorder for those axes exists
(build log 2026-10-25-skh-live-acceptance). The fixture is synthetic, not an adopter. On the
core-history adopter a static sample of ten lane-written goals (not an owner request set) gave 0
proven, 10 abstained (`no-route`: stage 5 clusters none of the 62 tests) and 0 failed, so
`SKH-V0-084` stays open.

Implementation status (V1-2641, V1-2642, V1-2643, 2026-10-25): `SKH-V0-185`..`187` are
implemented in `internal/appmap` and `internal/skhproof/record.go`. Re-recording the retained
V1-2595 live outputs (prove, validity and draft byte-identical) through the new recorder reads
hygiene and execution `OBSERVED`; condition 3 still blocks with `acceptance-blocked` on
`association=UNKNOWN, freshness=UNKNOWN, cleanup=UNKNOWN` (`SKH-V0-187`). On core-history, with the
`AMAP-V0-191`..`194` router reading, stage 5 clusters 22 candidates (23 tests unclustered, down
from 62), and the same ten lane-written goals give 0 proven, 9 abstained and 1 failed: "open the
manage area" answers `COMPLETE` with an off-goal viewer test matched on "open" and "area" only.
The three generated drafts read `INELIGIBLE` (`JS-SKIPPED`) with their guard and `ELIGIBLE` with
no finding without it; `CCE-V0-009` mutants and RVN run receipts are not run, because the adopter
forbids running its application or tests (build log 2026-10-25-skh-v1905-requalify).

Implementation status (V1-2650, 2026-10-26): `SKH-V0-188` is implemented in
`internal/appmap/site_knowledge_how.go`. On the retained core-history clone at revision
`5b30b91b6`, with a map rebuilt there by the installed corvint (its digest differs from the
recorded qualification's, while goal 5 reproduced exactly: viewer candidate, coverage 0.765 on
"open" and "area"), the frozen ten goals (sha256 `1a10621d…`) give 0 proven, 10 abstained and 0
failed: goal 5 abstains with `unmatched-goal-term: manage`, because 13 manage-area candidates
carry "manage" (weight 0.99) and the choice's lightest term "area" weighs 0.74. Goals 6 and 9 stay
`INCOMPLETE`, each with one added `unmatched-goal-term` gap ("console", "change": no candidate
carries them) (build log 2026-10-26-skh-v2650-outweighed-goal-term).

Codex review fixes (V1-1900, V1-1903, V1-1904):

- `SKH-V0-177`: (experimental; Codex review fix, V1-1903) `how` MUST answer with the flow
  variation whose own source titles and step actions carry the most goal-term weight, the
  representative variation winning a tie, so a goal naming a term only one variation carries
  ("ship with a note") never answers with another variation's test and steps (`SKH-V0-143`).
- `SKH-V0-178`: (experimental; Codex review fix, V1-1904) The draft guard MUST go directly after
  the first line in the test's span that ends by opening its callback body (`=> {` or `) {`,
  optionally followed by a line comment), so a multi-line declaration keeps the guard out of its
  argument list. With no such line the draft abstains with `source-framework-unsupported`
  (`SKH-V0-154`).
- `SKH-V0-179`: (experimental; Codex review fix, V1-1900) A setup project whose file selection is
  `UNKNOWN` (for example an unsupported `testMatch` glob) MUST still arrange each test of an
  unnarrowed dependent project under its directory, so the dependency is reported as a
  `no-setup-for-precondition` gap in `how` rather than silently dropped (`SKH-V0-104`).
- `SKH-V0-810`: (accepted under the owner's standing auto-accept, 2026-11-01; V1-2594) `corvint how
  [FLAGS] GOAL` MUST be a top-level alias of `flows appmap how`: it takes the same flags and
  positional goal, writes byte-identical stdout, and refuses with the same codes. Root help MUST
  list it, so an agent finds the how-to answer without knowing the `flows appmap` nesting.

Static links commented out in HTML (`<!-- -->`) or ERB (`<%# %>`), and programmatic navigation
inside a string literal or after an inline comment, are not edges: this is a defect fix under
`SKH-V0-111`, which already reads links lexically from markup and code.

## Preregistered measure

The measure is fixed before stage 6 runs on any request, and it is closable by the single owner.

- **Request set**: N owner-written "how do I X" requests for the confirmed adopter. N is 10
  (decided 2026-10-22; see "First adopter"). The set and its SHA-256 are committed in a
  build-log entry before the first run and are not edited afterwards. Each request carries its
  accepted outcome oracle (`SKH-V0-087`), frozen with the set. A request added later is reported
  separately.
- **Answered**: the stage 8 receipt reads `PROVEN`, which means a passing, discriminating test
  whose outcome assertion is the request's accepted oracle, and the owner confirms afterwards that
  the test demonstrates X. An `EXECUTION_VERIFIED` receipt is not answered and is counted apart.
- **Abstained**: a closed abstention code from `SKH-V0-062` or `SKH-V0-081`. An abstention is an
  acceptable outcome, never a failure of invariant 2.
- **Wrong**: a `PROVEN` test that the owner judges does not demonstrate X. Any wrong answer blocks
  promotion.
- **Kill threshold**: if fewer than 40% of the N requests are answered, the generation approach
  (stages 7 and 8) is killed. The same threshold killed CCE-V0 compilation.
- **Hard gates**: 0 unanchored statements across all `PROVEN` tests, and byte-identical reruns of
  the model, the plans and the generated tests at the same revision. Either gate failing fails the
  run, whatever the answered rate.
- **Report**: answered, execution-verified, abstained (by code), wrong and failed counts,
  unanchored statements, rerun identity and wall minutes per request.

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
| Unsupported test or page-object form | abstain, naming the file and form (`SKH-V0-010`): an UNKNOWN with reason `form-unsupported` for an unsupported file type, `parse-unsupported` for a Ruby file the reader cannot read, and `second-class-in-file` for a second page-object class in one file |
| Inherited or helper method not resolvable | UNKNOWN at that hop's line (`SKH-V0-012`), with the closed hop reason `class-not-read`, `import-not-read`, `not-defined` or `not-exported` |
| Step cannot be parsed | `opaque` step; the sequence is incomplete (`SKH-V0-021`) |
| Precondition with no setup record | `unresolved`; the plan abstains with `no-setup-for-precondition` |
| Mocked endpoint mistaken for real behaviour | prevented by the `mocked` label (`SKH-V0-034`) |
| Dynamic navigation target | UNKNOWN edge (`SKH-V0-043`) |
| Router dialect unsupported (for example JSX `<Routes>`, V1-1712) | 0 screens with a named abstention, never guessed screens |
| Goal matches several flows equally | ranked candidates with ties named (`SKH-V0-062`) |
| Evidence changed since it was mined | `STALE` (`SKH-V0-063`) |
| Generated line has no evidence | TODO and `INCOMPLETE` (`SKH-V0-072`) |
| Test passes but does not discriminate | `not-discriminating`, never `PROVEN` (`SKH-V0-081`) |
| Discrimination verdict inconclusive, unstable or not run | `discrimination-inconclusive`, never `PROVEN` (`SKH-V0-085`) |
| Validity reader abstains, file uncertain or example missing | `validity-unestablished` (`SKH-V0-086`) |
| A required NEA-V0 axis is unknown | `acceptance-blocked`, with the axis kept in the receipt (`SKH-V0-082`) |
| Passing, discriminating test for a different goal | `EXECUTION_VERIFIED` with `oracle-mismatch`, never `PROVEN` (`SKH-V0-087`) |
| Guarded draft run, or candidate bytes changed after proof | `run-failed`; a byte change voids the receipt (`SKH-V0-088`) |
| Live run not recorded on the pinned Playwright host | `infrastructure` with `live-run-not-recorded`; never `PROVEN` (`SKH-V0-166`) |
| Draft edited after generation, or a second guard added | refused with `skh-answer-invalid` before any transform (`SKH-V0-165`) |
| Candidate path already exists or is a symlink | refused with `e2e-out-unsafe`; nothing written (`SKH-V0-174`) |
| Method or locator in the source but not exercised | `unverified`; `binding-unverified` (`SKH-V0-089`) |
| Reviewed source test with a `defect` verdict | blocks its flow and oracle (`SKH-V0-052`) |
| Plan element mined only from a non-Playwright test | `source-framework-unsupported` (`SKH-V0-070`) |
| Source line of the primary test is secret-shaped | not copied; `secret-screened` TODO and `INCOMPLETE` (`SKH-V0-157`) |
| Callback body opener of the primary test not located | abstain with `source-framework-unsupported`; no guard inside an argument list (`SKH-V0-178`) |
| Setup project with an unsupported file selection | `UNKNOWN` record still arranges the dependent tests; `how` reports `no-setup-for-precondition` (`SKH-V0-179`) |
| Link inside an HTML or ERB comment, or navigation inside a string | not an edge (`SKH-V0-111`) |
| Flaky live run | never `PROVEN` (`SKH-V0-082`) |
| Secret in `storageState` or a typed value | screened and replaced by a parameter slot (`SKH-V0-006`) |
| Browser, server or provider failure | `infrastructure`, kept separate from test failure |

## Acceptance evidence and testing matrix

| Stage | Fixture evidence | Real-adopter evidence |
|---|---|---|
| 1 | one fixture per supported page-object form, including inherited and helper hops | class, method, locator and abstention counts on the adopter |
| 2 | IR fixtures for Playwright, Cypress and Capybara, with dedup, `opaque` round trip and propagation to an incomplete plan | step-record and opaque counts |
| 3 | setup fixtures for each listed source, plus secret-screening tests | setup records and unresolved preconditions |
| 4 | template-link fixtures per syntax, with dynamic targets as UNKNOWN | static and observed edge counts, kept separate |
| 5 | clustering fixtures with conflicts and single-source candidates, a no-mutation read test, an import round trip, and defect and unrelated sources | candidate flows imported as proposed intents |
| 6 | `how` fixtures for answered, ambiguous and every abstention code | plans for the preregistered requests |
| 7 | generation fixtures with evidence maps and TODO lines, rerun identity, and a non-Playwright source abstention | generated tests for the planned requests |
| 8 | receipt fixtures for `PROVEN`, `EXECUTION_VERIFIED` and every code; every `CCE-V0-009` verdict; each validity block; blocked NEA axes; a wrong-goal test; a guarded-draft run; uncalled method, untaken branch and unexercised locator | the preregistered measure report |

Each stage's tests cite its requirement IDs in their names or docstrings. The real-adopter
qualification is recorded in a build-log entry with the adopter revision, tool versions and the
measure report. It does not cite private product names.

## Rollout and rollback

- Every stage ships `experimental`, behind the existing `flows appmap` family, `corvint how` and
  the companion commands. No surface is advertised before its stage evidence exists.
- All learned records are derived state. Deleting them and regenerating at the same revision gives
  the same bytes (`SKH-V0-007`), so a rollback never loses authority.
- A stage is rolled back by reverting its commits, dependents first. Before a stage is reverted,
  every stage that reads it (table below) MUST be reverted or disabled. A stage's amendment to a
  shared contract (AMAP-V0, AMSP-V0, RVN-V0 or PWP-V0) is reverted with it, after the consumers
  of that amendment.
- An older binary MUST refuse a newer derived record by its versioned profile (`SKH-V0-007`),
  naming it, and never read it partly. The record is then regenerated at the same revision.
- Rollback never edits or deletes intent files. Intents written by an explicit `flows import` and
  owner-accepted AFU-V1 intents stay as committed project files.
- If the preregistered measure meets the kill threshold, stages 7 and 8 become `deferred` with a
  `Disposition:` header naming the return condition. Stages 1..6 are kept only if their own
  evidence shows use, and the decision is recorded in a build-log entry.
- This spec adds no wire field to an existing contract until the stage that needs it amends that
  contract's spec in the same change.

Stage dependencies (each stage also reads the contracts named for it above):

| Stage | Reads stages |
|---|---|
| 1 page objects | none |
| 2 step IR | 1 |
| 3 data setup | 2 |
| 4 navigation edges | none |
| 5 flow inference | 2, 4 |
| 6 `corvint how` | 1, 2, 3, 4, 5 |
| 7 generator | 1, 3, 6 |
| 8 proof pipeline | 1, 2, 7 |

## Traceability

| Requirements | Ticket | Implementation | Evidence |
|---|---|---|---|
| `SKH-V0-001`..`009` | V1-1897 | this spec; `internal/appmap/site_knowledge_project.go` | 002, 005, 007 and 009: `TestSKHV0_002_UnsupportedImportAndDeterminism`, `TestSKHV0_013_PageObjectsAndStepsCLI` and the core-history sample (build log 2026-10-22-sk-1022l); 001, 003, 004, 006 and 008: by construction in stages 1..2, NOT_STARTED beyond them |
| `SKH-V0-010`..`014` | V1-1898 | `internal/appmap/site_knowledge*.go`, `cmd/corvint/flows_appmap_knowledge.go`, `cmd/corvint-corpus-mcp/mapknowledge.go` | experimental: `TestSKHV0_010_PlaywrightClassPageObject`, `TestSKHV0_010_ModulePageObjectAndSetupHelpers`, `TestSKHV0_010_CypressClass`, `TestSKHV0_010_CapybaraAndSitePrism`, `TestSKHV0_012_SpecLocalHelpersTemplatesAndValueAsserts`, `TestSKHV0_013_PageObjectsAndStepsCLI`, `TestSKHV0013CorpusMCPMapKnowledge`; core-history sample qualifies `playwright-ts-module` only (build logs 2026-10-22-sk-1022l and 2026-10-22-site-knowledge-setup-stage3) |
| `SKH-V0-015`, `016` | V1-2407 | `internal/appmap/site_knowledge.go`, `internal/appmap/site_knowledge_project.go` | experimental: `TestSKHV0_015_016_URLJoinAndNamedEffects`; core-history sample: 0 of 10 joined, every unjoined page object and unknown effect named |
| `SKH-V0-020`..`024` | V1-1899, V1-2406 | `internal/appmap/site_knowledge*.go` | experimental: `TestSKHV0_020_OpaqueStepsAreKept`, `TestSKHV0_021_InlinesCallsWithBoundArguments`, `TestSKHV0_022_DeduplicatesSequencesWithUsage`, `TestSKHV0_023_StrictStepDecoderRefusesOutsideTheClosedIR`, `TestSKHV0_024_PressFocusAndHoverAreTyped`; core-history sample for Playwright; Cypress and Capybara fixture-only |
| `SKH-V0-030`..`034`, `SKH-V0-100`..`105` | V1-1900, V1-2408 | `internal/appmap/site_knowledge_setup.go`, `internal/appmap/site_knowledge_setup_rails.go`, `internal/appmap/setup_writers.go`, `internal/appmap/site_knowledge_project.go`, `cmd/corvint/flows_appmap_knowledge.go`, `cmd/corvint-corpus-mcp/mapknowledge.go` | experimental: 030, 031, 033, 034 and 100..105 by `TestSKHV0_030_031_SetupRecordsAndTestLinks`, `TestSKHV0_100_105_RailsSetupProjectsAndConditions`, `TestSKHV0_103_SetupConditionCLI`, `TestSKHV0_013_PageObjectsAndStepsCLI` and `TestSKHV0013CorpusMCPMapKnowledge` over inline committed fixtures; 032 partial (V1-1891 reused; V1-1883 name rule only; TSA-V0 setup-select not joined); no real-adopter sample (NOT_RUN: cloud) |
| `SKH-V0-040`..`043`, `SKH-V0-110`..`115` | V1-1901 | `internal/appmap/site_knowledge_edges.go`, `cmd/corvint/flows_appmap_knowledge.go`, `cmd/corvint-corpus-mcp/mapknowledge.go` | experimental: 040, 042, 043 and 110..115 by `TestSKHV0_110_115_StaticNavigationEdges`, `TestSKHV0_114_StaticLinkRegressions`, `TestSKHV0_013_PageObjectsAndStepsCLI` and `TestSKHV0013CorpusMCPMapKnowledge` over inline committed fixtures; 041 dialect widening NOT MET (NEEDS-HOST: no adopter in the cloud lane); no real-adopter sample (NOT_RUN: cloud) |
| `SKH-V0-050`..`054`, `SKH-V0-120`..`126` | V1-1902 | `internal/appmap/site_knowledge_flows.go`, `cmd/corvint/flows_appmap_infer.go`, `cmd/corvint-corpus-mcp/mapknowledge.go` | experimental: 050..054 and 120..126 by `TestSKHV0_050_054_FlowInferenceTrustAndImport`, `TestSKHV0_121_125_FlowInferenceRegressions`, `TestSKHV0_126_InferCLIAndImport`, `TestSKHV0_126_TrustFromJoins` and `TestSKHV0013CorpusMCPMapKnowledge` over inline committed fixtures; the corpus join is fixture-tested at its conversion boundary; no real-adopter sample (NOT_RUN: cloud) |
| `SKH-V0-140`..`142` | V1-1900..V1-1902 (#1705 review fixes) | `internal/appmap/site_knowledge_flows.go`, `internal/appmap/site_knowledge_edges.go`, `internal/appmap/site_knowledge_setup.go` | experimental: `TestSKHV0_140_OutcomeKeepsSelectorName`, `TestSKHV0_141_RubyHelperConsumesWholeArgument`, `TestSKHV0_142_SetupProjectHonorsTestDir` (each failing first) over inline committed fixtures |
| `SKH-V0-060`..`064`, `SKH-V0-143`..`152` | V1-1903 | `internal/appmap/site_knowledge_how.go`, `cmd/corvint/flows_appmap_how.go`, `cmd/corvint-corpus-mcp/mapknowledge.go` | experimental: `TestSKHV0_143_HowAnswersGoal`, `TestSKHV0_144_HowAbstains`, `TestSKHV0_146_HowAdmitsLedgerEdges`, `TestSKHV0_147_HowSetupAndGaps`, `TestSKHV0_151_HowStale`, `TestSKHV0_152_HowBoundsAndBudget`, `TestSKHV0_143_HowCLI`, `TestSKHV0_143_MapKnowledgeHowView`; the top-level `corvint how` alias is PARTIAL |
| `SKH-V0-070`..`074`, `SKH-V0-153`..`162` | V1-1904 | `internal/appmap/site_knowledge_draft.go`, `cmd/corvint/flows_appmap_plan.go`, `cmd/corvint-corpus-mcp/mapplan.go` | experimental: `TestSKHV0_153_HowDraftCopiesPrimaryTest`, `TestSKHV0_156_HowDraftMarksTodos`, `TestSKHV0_157_HowDraftScreensSecrets`, `TestSKHV0_159_HowDraftAbstains`, `TestSKHV0_161_PlanDraftCLI`, `TestSKHV0_162_MapPlanHowDraft` |
| `SKH-V0-080`..`089`, `SKH-V0-163`..`176` | V1-1905 | `internal/skhproof/skhproof.go`, `cmd/corvint-test/howaccept.go` | experimental, offline over recorded evidence: `TestSKHV0_163_OnlyTheCompanionImportsThePipeline`, `TestSKHV0_164_TransformRemovesOnlyTheGuard`, `TestSKHV0_165_DraftMustVerify`, `TestSKHV0_166_EvidenceBindsTheCandidateBytes`, `TestSKHV0_167_RunsMustPassEveryRepeat`, `TestSKHV0_168_ValidityOfTheCandidateBlob`, `TestSKHV0_169_AcceptanceNeedsEveryAxis`, `TestSKHV0_170_SelectorsAndMethodsStayUnverified`, `TestSKHV0_171_EveryControlKilled`, `TestSKHV0_172_VerdictLadder`, `TestSKHV0_173_OracleSeparatesProvenFromExecution`, `TestSKHV0_174_HowAcceptCLI`, `TestSKHV0_175_StrictRecordedInputs`, `TestSKHV0_176_ReceiptIsStable`; the 080/082 live run is NEEDS-HOST (pinned Playwright tuple absent), 084 adopter qualification NOT_RUN, 089 witness amendments NOT_STARTED |
| `SKH-V0-180`..`184` | V1-2595, V1-1905 | `internal/skhproof/record.go`, `cmd/corvint-test/howevidence.go` | experimental: `TestSKHV0_180_RecordCopiesObservedFactsOnly`, `TestSKHV0_180_UnqualifiedRunIsInfrastructure`, `TestSKHV0_181_RecordValidity`, `TestSKHV0_182_BindingsUnwitnessedAndAssertion`, `TestSKHV0_184_HowEvidenceCLI` over inline fixtures; one live run on a synthetic fixture (build log 2026-10-25-skh-live-acceptance) |
| `SKH-V0-185` | V1-2641 | `internal/appmap/site_knowledge.go`, `site_knowledge_flows.go`, `site_knowledge_how.go` | experimental: `TestSKHV0_185_PageObjectStepScreens` |
| `SKH-V0-186`, `SKH-V0-187` | V1-2643, V1-1905 | `internal/skhproof/record.go` | experimental: `TestSKHV0_186_HygieneSourceOnly`, `TestSKHV0_180_RecordCopiesObservedFactsOnly`; retained V1-2595 live outputs re-recorded (build log 2026-10-25-skh-v1905-requalify) |
| `SKH-V0-188` | V1-2650 | `internal/appmap/site_knowledge_how.go`, `internal/appmap/plan.go` | experimental: `TestSKHV0_188_HowRefusesAnOutweighedMatch`; core-history goal 5 rerun (build log 2026-10-26-skh-v2650-outweighed-goal-term) |
| `SKH-V0-189` | V1-2847 (#1905) | `internal/appmap/sourceroots.go`, `internal/appmap/site_knowledge_project.go`, `internal/appmap/refs.go`, `cmd/corvint/flows_appmap_knowledge.go` | experimental: `TestSKHV0189AliasedSources`, `TestSKHV0189AliasedHowDraft`; reproduces both #1905 refusals |
| `SKH-V0-329`, `SKH-V0-330` | V1-2927 (#1933) | `internal/appmap/route_handlers.go` `readManifestAt`, `site_knowledge_edges.go`, `site_knowledge_setup_rails.go`; `cmd/corvint/flows_appmap_knowledge.go` `parseSourceVerb` | experimental: `TestSKHV0329AliasedManifestUnbound`, `TestWSDV0145SourceVerbsWorkspace` (links and setup over a manifest in the aliased root, trailing `--full`), `TestSKHV0330SourceVerbFlagOrder`; reproduces the #1933 links/setup refusal |
| `SKH-V0-177`..`179`, `SKH-V0-111` (comment fix) | V1-1903, V1-1904, V1-1900 (Codex review fixes) | `internal/appmap/site_knowledge_how.go`, `internal/appmap/site_knowledge_draft.go`, `internal/appmap/site_knowledge_setup.go`, `internal/appmap/site_knowledge_edges.go` | experimental: `TestSKHV0_177_HowPicksGoalVariation`, `TestSKHV0_178_DraftGuardAfterBodyOpener`, `TestSKHV0_179_UnknownSetupProjectStaysAGap`, `TestSKHV0_111_CommentedLinksAreNotEdges` (each failing first) over inline committed fixtures |
| `SKH-V0-810` | V1-2594 | `cmd/corvint/flows.go` `flowInvocation`, `cmd/corvint/help.go`, `cmd/corvint/flows_appmap_how.go` | experimental: `TestSKHV0810HowAlias` |

## Promotion and kill criteria

- **Promote** to `implemented` when every stage has its fixture evidence, and to `validated` when
  the preregistered measure answers at least 40% of the requests with 0 wrong answers, 0
  unanchored statements and byte-identical reruns. Promotion also needs the owner to read back the
  `standing` requirements (`SRG-V0-010`).
- **Kill** stages 7 and 8 if fewer than 40% of the requests are answered. Kill any stage whose
  real-adopter sample shows it cannot anchor its records.

## Owner questions

1. Answered 2026-10-22: the first adopter is core-history. Still open: approval to run its suite
   locally, offline, for stage 8; until then it is read-only and never run.
2. Answered 2026-10-22: N = 10, kill threshold 40% (see "First adopter"); the owner may revise N
   before the request set is frozen.
3. Answered by the adopter choice: core-history has no Ruby, so the Capybara and SitePrism forms
   stay fixture-only and `experimental`.

Decided without the owner: V0 generation is Playwright-only (`SKH-V0-070`). This is an orchestrator
decision under the owner's delegation of in-task forks (2026-10-05), made after review on
2026-10-19; the owner may revise it.
