# Application Flow Understanding V0

Owner: Russell Lewis  
Date: 2026-09-22  
Intent status: proposed  
Delivery status: experimental  
Repository baseline: `cd9fec9ae5ed8199ee3c844884ca4212d3e4031c`

## Agent digest
- Claim: Corvint maps declared web flows to source-bound assertion candidates and bounded browser observations, preserving coverage gaps and contradictions.
- Status: proposed/experimental
- Exists: pure Go flow reporting/recording, an optional source-test scanner and guided Chromium observer, and bounded fixture evaluations.
- Blocked on: profile acceptance and external-application qualification; no promotion or exhaustive discovery claim.
- Read next: Supported profile; Requirements; First end-to-end proof.

## Outcome

Corvint should explain how an application works, which tests check each behavior, which behaviors
lack adequate test evidence, and whether observed UI behavior agrees with that explanation.
It should learn additional flows from explicit observation and preserve disagreements with intent.

The owner's ambition is all application flows. Exhaustiveness is only measurable within a declared
boundary: build, routes, roles, data states, feature flags, browsers, integrations and exploration
budget. Unknown branches and unsupported surfaces remain visible. A high percentage over discovered
flows must never imply that undiscovered flows do not exist.

## Existing foundation and verified gap

Inspected original source and spec digests at the baseline; no runtime behavior was tested for this proposal.

| Existing mechanism | Reuse | Boundary |
| --- | --- | --- |
| [JS/TS provider](js-live-test-provider-v0.md), [receipt types](../../internal/jstestprovider/receipt.go) | Test outcomes, identities, build freshness, failure-artifact pointers and owned-process cleanup | Proposed/experimental; expressly no coverage observation or qualified network containment |
| [Playwright result parser](../../internal/jstestprovider/playwright.go) | Preserve failed, skipped, flaky, interrupted and infrastructure outcomes | Reporter results do not establish which flow postconditions were asserted |
| [OCM](ocm-v0-dogfood.md) | Requirement-to-test/source witness references | Structural links do not establish relevance, adequacy or correctness |
| [TCQ](test-claim-qualification-v0.md) | Exact test-anchor and caller-report matching | Proposed; caller-reported evidence does not establish adequacy or execution authenticity |
| [External test selection](external-test-selection-v1.md) | Conservative selection over explicitly declared relations | Walks only provider-declared relations; does not discover application behavior |

The missing integrated capability is a model of application behavior connected to assertion evidence
and observations. No inspected foundation supplies that complete outcome. Existing task-store tickets
`V1-0025` (evidence navigation) and `V1-0027` (provider authoring kit) are adjacent, not delivery claims
for this feature. This document neither changes their scope nor creates a competing execution queue.

## Model

A **flow** is a user or system goal with an actor, preconditions, ordered actions, state transitions,
observable outcomes and important alternatives. A route or call graph supplies clues; neither alone
describes a flow. Shared screens reached with different permissions or persisted state may represent
different states. Form validation, cancellation, loading, retries, errors, reload and back navigation
belong in the model where relevant.

Represent three connected kinds of evidence:

1. **Expected behavior:** owner-authored requirements and reviewed flow definitions. Code-derived or
   agent-derived expectations remain hypotheses until accepted.
2. **Test behavior:** stable test identity, the transition exercised, assertion source and the exact
   outcome checked. Visiting a page, running a function and asserting persistence are different relations.
3. **Observed behavior:** actions and resulting UI/API state from a particular run, with input/build
   identities, artifacts and an explicit result for each expected postcondition.

One flow can have an inferred branch, an asserted unit-level condition, a failed end-to-end run and
a stale screenshot simultaneously. Keep evidence basis, test relationship, run outcome and freshness
as separate axes; do not compress them into a single confidence score or green badge.

## Requirements

- `AFU-V0-001`: Declare the analyzed application/build, supported adapters, routes, actors, data/flag
  variants, integrations, excluded surfaces and exploration limits. Report observed inventory and
  unresolved exploration frontier separately; never report universal completeness.
- `AFU-V0-002`: Discover candidate flows from existing repository intent, route/navigation/form/event
  structure, test source and supplied observation traces. Cite immutable source or a bound observation
  for each state/transition and its inclusion reason. Unsupported syntax produces an explicit gap.
- `AFU-V0-003`: Represent each flow's actor, preconditions, actions, expected postconditions and branch
  conditions. Distinguish owner intent, inferred expectations and observed outcomes. Observed behavior
  may contradict intent but MUST NOT silently redefine it.
- `AFU-V0-004`: Map tests at the test-case and relevant assertion level. Distinguish declared links,
  static candidates, observed execution and observed postcondition assertions. File proximity, test
  names, line coverage or a passing suite alone cannot establish semantic coverage. Dynamic assertions
  or ambiguous mappings remain unresolved, with their candidate evidence available for review.
- `AFU-V0-005`: Report actionable gaps: no test found in the declared inventory, exercised without
  a mapped assertion, untested branch/role/data variant, stale evidence, failed/flaky run, and unknown
  mapping. State the missing behavior and suggested test level; never equate missing evidence with
  proof that no test exists. Incomplete test discovery blocks a definitive inventory absence claim.
- `AFU-V0-006`: In an explicitly started safe test session, discover available actions from the UI's
  accessible/DOM structure, use visual evidence where structure is insufficient, and observe resulting
  state transitions. Record candidates that were unavailable or not explored. Visual-only interactions
  unsupported by the first adapter remain visible gaps, not invented successful actions.
- `AFU-V0-007`: Before verification, freeze independently justified expected outcomes; compare each
  with observed UI state and necessary backend/persistence effects. A click completing or screenshot
  matching does not establish a business outcome. If no accepted oracle exists, report exploratory
  agreement with a hypothesis rather than confirmed intended correctness.
  Label UI-only and backend-confirmed observations separately; backend confirmation needs an
  independent state probe bound to the same backend build and resettable fixture instance.
- `AFU-V0-008`: Bind observations to repository/tree and served build, test/assertion and config
  identities, runner/browser, backend build, fixture/reset state, declared nonsecret flags and role.
  Unknown, changed or mismatched inputs prevent a current confirmed claim. Preserve failed attempts,
  retries, skips, timeouts and infrastructure failures independently.
- `AFU-V0-009`: Learning is an explicit local record operation over bounded, screened artifacts.
  Keep new flow hypotheses, contradictions and reviewed corrections with provenance; do not overwrite
  accepted intent, broaden execution permission, train from secrets or silently change ranking. Reads
  remain nonmutating. Any retrieval/learning mechanism change needs its existing frozen evaluation gate.
- `AFU-V0-010`: Use a separately invoked optional browser/test provider with declared action scope,
  origin/egress controls, isolated test identities and resettable data. Enforce configured limits and
  cleanup on success, failure and interruption. Deny actions outside the declared test boundary;
  inability to enforce it is a blocked observation, never successful verification.
- `AFU-V0-011`: Source, UI text and imported artifacts are untrusted data, never execution instructions.
  Bound and screen retained traces/screenshots/network evidence; exclude secrets and personal data,
  restrict artifact paths and preserve explicit capture/redaction gaps. Redacted evidence must not be
  presented as a full raw transcript. Do not record arbitrary response bodies or full environments.
- `AFU-V0-012`: Explain each flow and gap through source/assertion/run evidence, expose contradictions,
  and invalidate affected relations after changes. Offer a bounded next verification action. Exported
  observations retain their actual authority class; hashes or local reports do not confer attestation.

## First end-to-end proof

Experimental pilot assumption: use a disposable local web fixture before a user application.
The fixture has an editor and a viewer, a create-item form, required-field validation, persistent
storage, reload, a denied-write branch and a controllable save failure. This tests one complete
vertical path while avoiding a generalized crawler or a new UI before usefulness is established.

Corvint receives visible accepted requirements for persistence and viewer permissions alongside
normal source/tests/UI inputs. Those requirements justify known-branch gaps and correctness checks.
A frozen evaluator separately retains a labelled state/transition inventory, test-to-assertion
answers and seeded-defect labels. Supplied flow declarations count as intent ingestion, never as
independent discovery; score inferred additions and known-requirement coverage separately. Faulty
code remains visible in the normal inputs, but the fault labels and evaluator answers do not.

The pilot includes a resettable backend state probe independent of the UI and its client cache.
It reads the fixture's persisted item and ownership records after successful or denied writes;
reload is an additional UI check. Bind probe results to backend build, fixture instance, run and
action sequence. A denied viewer write must leave backend state unchanged. The probe is an
evaluation/observation facility, not a source of inferred flow labels. Its identity and result
bindings are new work: the existing JS provider's app-build digest does not supply them.

| Case | Required observation |
| --- | --- |
| Successful creation | Infer the candidate journey, show its provenance, run it, confirm persistence through the independent backend probe and reload, identify the exact persistence assertion |
| No-assertion test | A test clicks Save and passes; report execution without a persistence assertion |
| Missing denial test | Identify the viewer-write rejection as a known expected branch lacking a test in the fully enumerated fixture inventory |
| Seeded persistence defect | UI shows success but the backend probe finds no saved item; detect disagreement with the visible accepted persistence requirement even if a cache makes reload appear correct |
| Seeded authorization defect | Viewer changes persisted state; the independent probe detects a contradiction with accepted permissions even if current UI behavior is internally consistent |
| Generated characterization test | A generated test matches the broken UI; it must not override the accepted oracle or clear the contradiction |
| Hidden or unsupported branch | Preserve the unobserved/unsupported frontier and prevent an exhaustive-coverage claim |
| Drift and inconclusive runs | Changed build/fixtures, skipped test, flaky result, timeout or missing browser cannot produce current confirmation |
| Cancellation and hostile content | No owned server/browser descendants survive interruption; page text cannot widen permissions or trigger undeclared actions |

The developer fixtures and held-back evaluation cases must differ. Measure flow/branch discovery
precision and recall against the fixture oracle, assertion-mapping precision, false-confirmation rate,
missed seeded defects, gap usefulness, and complete-command wall time including review/correction.
Require zero false confirmations and detection of both seeded defects in this bounded proof.
Report exact counts and unresolved cases; fixture success does not establish general-app accuracy.

## Execution order and verification

1. Freeze the small pilot's scope, oracles, supported source/test subset and observation boundary.
   Verify that evaluator answers and fault labels remain hidden, and that visible intent is not
   counted as independent discovery.
2. Prove source + one real test + browser observation + a gap/contradiction report end to end.
   Reuse the provider's lifecycle/result machinery where its evidence contract fits. A manual flow
   manifest is a useful baseline, but passing one is not discovery acceptance.
3. Evaluate the bounded discovery and assertion mapper against withheld variants; compare complete
   time and correction burden with manual flow inventory plus ordinary E2E tests. Stop expansion
   if the integrated loop produces false confidence or no useful missing-test/defect signal.
4. Only after that proof, expand routes/roles/frameworks and add explicitly recorded observations,
   change invalidation and CLI/agent presentation. Native application adapters are subsequent slices.

Use one builder and one independent reviewer; no nested delegation. Before code changes, register
the owning spec in `docs/specs/README.md`, settle numbered requirements and add scoped work to the
existing Corvint Tasks store. Preserve the existing source/CEM/OCM, frozen evaluations, review and
canonical gate requirements. An experimental provider is not promoted by composing it here.

## Boundaries, rollout and rollback

The default local product remains one native Go binary without a permanent daemon, browser install,
account or outbound service. Browser execution requires a separate accepted optional execution
profile; the present JS/TS provider expressly does not supply that network boundary. Read-only core
queries can consume validated artifacts without launching a browser or changing learned state.

For the first experiment, propose one local application origin plus its declared local backend,
one browser, two roles, at most 50 states, 100 transitions, 200 actions, five minutes of exploration
and 5 MiB of sanitized retained evidence. Reaching any bound reports truncation. These are proposed
pilot budgets, not existing engine limits or a promise of application completeness.

No production crawling, external purchases/messages, automatic intent acceptance, automatic merging
of generated tests, native-platform qualification or exhaustive path proof belongs in V0. Existing
repository gates remain mandatory; test-gap advice cannot authorize skipping them.

Start with opt-in experimental artifacts in the provider's existing transport boundary; freeze a
new codec only when the complete proof identifies necessary data. Do not extend an existing wire
profile's semantics silently. Rollback disables the optional observer/consumer and invalidates its
derived claims while retaining original evidence and the ordinary test workflow.

## Traceability and promotion

The supported subset is **IMPLEMENTED / EXPERIMENTAL**. The complete requirements remain **PARTIAL**:
there is no general route parser, autonomous crawler, visual interaction, arbitrary test grammar,
test-run authenticity, OS-wide egress sandbox or measured external-application accuracy. The fixture
suite is a regression evaluation, not a blinded accuracy study. Renamed controls/routes are a second
variant; this does not establish precision/recall across independently authored applications.

| Requirement | Named executable evidence | Remaining boundary or explicit gap |
| --- | --- | --- |
| AFU-V0-001 | `TestAFUV0DecodeAndAdmission` (malformed, duplicate-key, unknown-field and oversized input and non-loopback origins refused); `TestFlowsCLIReadPurity` and `TestAFUV0EvidenceSeparation` (`complete:false`, explicit frontier); gate `development-fixture` asserts `report.complete === false` | Explicit manifest only. Budget truncation gaps (`exploration-budget`, `request-budget`, `evidence-budget`) are emitted by `observe.mjs` but no test reaches a budget |
| AFU-V0-002 | `TestAFUV0CaptureAndRecord` (dirty source and escaping symlink refused); scanner tests "AFU-V0-002: comments and string contents are not runnable tests" and "AFU-V0-004/005: unsupported syntax cannot certify missing assertions" (`inventoryComplete:false`); gate fixture cases assert two `test-inferred` candidates | No general route/navigation/event parser. Gap: no test asserts the emitted `test-syntax-unresolved:<path>` string, or a test file from another framework (no `@playwright/test` import) |
| AFU-V0-003 | `TestAFUV0EvidenceSeparation` (role stays `caller-declared-unverified`); `TestAFUV0ContradictionsAndDrift` (inferred basis yields `hypothesis-agreement`, never acceptance) | No inferred preconditions or branch conditions |
| AFU-V0-004 | `TestAFUV0EvidenceSeparation` (`assertion-candidate`, `action-candidate-without-assertion`, no cross-role mapping); `TestAFUV0InvalidEvidenceDoesNotBecomeCoverage`; scanner test "AFU-V0-004: source/test/assertion anchors and input digests" | Static assertion candidates only; no source-test run authenticity; independent backend test mapping unsupported |
| AFU-V0-005 | `TestAFUV0EvidenceSeparation` (`no-test-in-declared-inventory`; incomplete inventory yields `mapping-unknown`); scanner test "AFU-V0-005: no assertion remains an action candidate"; gate unchecked-save and missing-viewer coverage states | Failed/flaky-run and untested data-variant gaps are not inferred |
| AFU-V0-006 | `TestAFUV0EvidenceSeparation` (`unexplored-control:` frontier); gate `development-fixture` and `held-back-selectors` (8 controls, `#help` unexercised) | Guided actions only. Gap: the `unaddressable-or-visual-control` gap for visual-only controls has no test |
| AFU-V0-007 | `TestAFUV0ContradictionsAndDrift` (contradiction, hypothesis agreement, missing postcondition refused); `TestAFUV0InvalidEvidenceDoesNotBecomeCoverage` (forged backend layer refused); gate `persist-broken` and `auth-broken` | Oracles come from the caller manifest; no accepted-oracle authority |
| AFU-V0-008 | `TestAFUV0ContradictionsAndDrift` (changed tree yields `stale`); `TestAFUV0DecodeAndAdmission` subtest on conflicting route roles; gate `wrong-served-identity` and source-drift `stale` assertions | Retries and flakiness are not inferred from one observation |
| AFU-V0-009 | `TestAFUV0CaptureAndRecord` (exclusive private write, authority elevation refused); `TestFlowsCLIReadPurity` (read and failed read leave repository bytes unchanged); gate repeated `flows record` refused | Explicit caller-owned artifact only; no ranking or training |
| AFU-V0-010 | `TestAFUV0NoDescendantsOnInterruption`; `TestAFUV0ContradictionsAndDrift` (failed cleanup yields `inconclusive`); lifecycle tests "AFU-V0-010: repeated signals join delayed browser cleanup" and "AFU-V0-010: interruption waits for pending browser launch"; `TestAFUV0DecodeAndAdmission` (non-loopback origins refused); gate `SIGINT`, `SIGTERM` and `cross-origin-http-and-websocket` (zero sentinel requests, `request-blocked-or-unavailable` and `websocket-blocked` gaps) | Trusted local code, not OS-sandboxed. Gap: the standing `non-http-browser-transports-unqualified` and `escaped-daemon-descendants-unqualified` gaps are emitted by `observe.mjs` but no test asserts them |
| AFU-V0-011 | `TestAFUV0DecodeAndAdmission` (script action and nonscalar oracle refused); `TestAFUV0InvalidEvidenceDoesNotBecomeCoverage` (raw input value refused); scanner anchor test (input retained only as digest); gate asserts the raw viewer fill value never appears in evidence, and `cross-origin-http-and-websocket` injects page script that cannot widen scope | Sanitized structural artifacts only. Gap: secret-shaped input refusal (`internal/appflows/input.go` `Decode`) has no test |
| AFU-V0-012 | `TestAFUV0EvidenceSeparation`, `TestAFUV0ContradictionsAndDrift`, `TestFlowsCLIReadPurity`; gate `source-only` case | No attestation or automatic intent acceptance. Gap: the per-flow `next` action and the copy of evidence gaps into the report frontier have no test |

The early real-browser evaluation observed three declared scenarios, eight structural controls and
two inferred test journeys in each healthy fixture. Both seeded backend defects were detected with
zero false confirmations in the evaluated cases. Complete-command benefit, general accuracy and
external-application validation are **NOT_OBSERVED**. Final source/check binding belongs to the keyed
dogfood reports and CEM, not to this preliminary fixture result.

V1-0100 reran `script/web-flows-gate` on the public-history tree `a98d770` on 2026-09-23: 6/6
scanner and lifecycle tests passed, all 9 browser cases passed, both seeded backend defects were
detected and the evaluation reported `falseConfirmations:0`. That count is a literal in
`tools/web-flows/test/e2e.mjs`; the zero is enforced by the per-case assertions that the defective
flow is `contradicted` and that no flow is `matched` under a wrong served identity. The repository
`make gate` was not run for that change and remains a release-attestation item. Flow reports keep
basis, coverage, runtime and freshness as separate fields (`internal/appflows/types.go` `Flow` and
`Report`) and carry no combined score; no test asserts the absence of such a field.

The owner authorized implementation on 2026-09-22. This does not accept the proposed technical
contract or qualify its execution profile. The prototype remains experimental and cannot be
promoted or advertised as generally delivered. Native platforms, visual-only exploration and
arbitrary test helpers remain unsupported.

## Supported profile

The proposed `application-flow-intent/0` is caller-owned input, not governing project authority.
It declares exact loopback origin, tracked source/test files, backend source, immutable fixture seed,
an owned server argv, identity/reset endpoints and role-bearing scenarios. Actions are literal
fill/click/reload; checks are text/contains/count/visible or read-only same-origin JSON-pointer
comparisons. The optional prototype discovers source-test candidates and live DOM controls, then
executes only explicit actions. This is guided exploration; untouched controls remain open frontier.
Conflicting role labels for one exact route are refused; role identity remains explicitly
`caller-declared-unverified` because this subset has no authenticated role-setup adapter.

The proposed `application-flow-evidence/0` carries pinned Git/source/manifest identities, parsed
tests, discovered controls/states, per-check observations, runtime/fixture bindings, cleanup and
explicit gaps. Static matches are candidates; a scenario pass is never a passing source-test run.
Mapping requires the same supported test's literal role-bearing route, ordered actions and exact
assertion target/operator/value. Unsupported setup, helpers, branches and dynamic tests invalidate
negative mapping claims. Source-only scanning starts neither a browser nor a server.

The observer's owned server echoes a fresh launcher nonce, served frontend digest, backend source
digest and immutable fixture seed through its declared identity endpoint. Recheck before publish;
missing or changed identity prevents a current run. This cooperative contract is CALLER_REPORTED,
not execution attestation. Mutable persisted state is probed separately from immutable fixture seed.

The optional Node/Playwright experiment requires a trusted local server and installed pinned packages;
normal core reads install nothing. Exact-origin HTTP(S) routing, refused probe redirects, blocked
WebSockets and service workers bound supported requests. These controls do not sandbox hostile
server/native code or claim complete OS egress containment; other transports are unqualified.
The prototype refuses non-loopback origins. Successful runs require non-PARTIAL owned-process-group
cleanup, completed browser.close and observed server exit. procgroup deliberately refuses its broader
RequireDescendantCleanup profile; escaped daemon descendants remain unqualified. Interruption tests
check the actual trusted fixture descendants. Profile acceptance is still required for promotion.

Artifacts retain bounded identifiers and boolean/count comparisons, not raw DOM text, response
bodies, input values, screenshots or arbitrary environment. Secret-shaped input is refused.
`flows record` is an explicit screened, exclusive write of validated evidence to a caller-selected
file. Reads are pure; no automatic learning ledger or ranking change is introduced. Recorded
observations enrich later reports only while their source/manifest bindings remain current.
