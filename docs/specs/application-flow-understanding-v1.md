# Application Flow Understanding V1 — proven flows, safe E2E selection, navigation and documentation

Owner: Russell Lewis
Date: 2026-09-24
Intent status: accepted (decision 0385)
Delivery status: planned
Authoritative inputs: owner request [issue 175](https://github.com/beamfall/corvint/issues/175) and
the owner's additions of 2026-09-24 (mapping E2E tests to code changes, an agent that can navigate
and use a website, documentation proven accurate by the same evidence), `AGENTS.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/specs/application-flow-understanding-v0.md`,
`docs/specs/documentation-corpus-v1.md` (issue 53, `DCP-V1-027..032`),
`docs/specs/external-test-selection-v1.md`, `docs/specs/corvint-1.0-product-and-release-v1.md`,
decisions 0374 and 0385.

## Agent digest
- Claim: Reviewed flows link to source, tests and run evidence; Corvint selects E2E tests with exclusion proofs, maps navigation and proves documentation claims.
- Status: accepted (decision 0385)/planned; S1 implements AFU-V1-001..005, 007 and 036 (unqualified) and AFU-V1-008..010 as library evaluation with only the provider record on the CLI (partial, CLI surface in S3); nothing else is implemented or qualified.
- Exists: the AFU-V0 experimental `corvint flows` report and `record`, the issue-53 behavior adapter, ETS-V1 selection and the Playwright provider this spec extends.
- Blocked on: implementation slices S1-S8 (Rollout) and the acceptance evidence below.
- Read next: Requirements; Trust boundary, limits, and failure modes; Deterministic acceptance.

## User and measurable job

A team with a web application and its E2E suite needs four answers that today come from memory:
which user-visible behaviors exist, which tests prove each one, which of those tests a given change
can skip, and how an agent should operate the application. The same evidence should make the
product documentation checkable. Measured jobs:

1. **Selection**: over a frozen labelled corpus whose ground truth comes from injected faults, the
   `e2e-safe` profile never omits a test that fails on the faulted change (unsafe-narrowing rate 0)
   and reports its reduction ratio. Any case it cannot prove falls back to the full relevant suite.
2. **Explanation**: for one UI flow and one API flow, `flows map` names every declared step's
   source, test, assertion and latest run evidence, and `flows gaps` reports an unmapped flow and a
   stale link as incomplete, never as covered.
3. **Navigation**: an agent given only a navigation packet completes the fixture application's
   declared tasks, stopping before every action whose effect class it was not granted.
4. **Documentation**: every rendered claim is `PROVEN`, `UNPROVEN`, `CONTRADICTED` or `STALE`, and
   the drift gate fails when a claim that was `PROVEN` stops being so.

## Verified current state

At `978b37b`:

- `corvint flows` dispatched only `record` beside its report, created its output without root or
  symlink confinement, and opened an input before the regular-file check. S1 adds the `export` and
  `import` subcommands (`cmd/corvint/flows.go:35-36@97eb5689`), routes `record` through the
  root-confined exclusive writer (`internal/appflows/report.go:317@18a159fa`,
  `internal/appflows/input.go:87-100@06fa25b5`), and checks Lstat before open
  (`internal/appflows/input.go:61-68@8ab13087`).
- The behavior provider pins exactly three repositories, `app`, `golf_e2e` and `docs_corpus`
  (`internal/doccorpus/behavior.go:35-39@2b4b5d34`).
- ETS selection returns `narrow-selection-allowed` once no obligation is uncovered
  (`internal/extevidence/selection.go:820-833@bae209bb`). It proves nothing about the tests it did
  not select, and says so (`internal/extevidence/selection.go:874@5368f5ce`).
- The Playwright reader derives `flaky` from every attempt's status
  (`internal/jstestprovider/playwright.go:151-158@f01509c4`), but keeps the duration, failure
  message, anchor and attachments of the last attempt only, so earlier attempts' detail is lost
  (`internal/jstestprovider/playwright.go:113-130@9632fff8`).
- The issue-53 adapter reconciles flows, variations, tests and assertions in both directions, and
  never has narrowing authority (`docs/specs/documentation-corpus-v1.md:191-196@cea38e20`).

## Definitions

- **Flow**: one user-visible behavior with a caller-assigned stable `flow_id`, kind `ui` or `api`,
  actor, preconditions, ordered steps, expected outcomes and `variation_id`s (the DCP-V1-027 unit).
- **Flow intent**: one `application-flow-intent/1` JSON file per flow in a caller-named tracked
  directory. It is repository-owned data, not a new specification language.
- **Link**: a typed edge from a flow step or outcome to a source span, a test key, an assertion
  anchor, or a run-evidence record. Its **basis** is `declared` (written in the flow intent),
  `reviewed` (declared, with a valid review anchor), `observed` (per-test code coverage recorded in
  run evidence at `INGESTED` or higher) or `inferred` (scanner, import or observer output, or a title
  match).
- **Review anchor**: `reviewed_at` naming a commit that is an ancestor of the evaluated revision,
  that changed the flow intent file, and after which the link's target content is unchanged.
- **Test key**: a stable test identity the caller declares (a Playwright annotation or tag of type
  `corvint-test-key`, or a declared mapping from a runner's stable ID), qualified by the runner
  project or browser when the runner has more than one. A key derived from a title is `inferred`.
- **Run evidence**: one `test-run-evidence/0` record of one test key's execution with every attempt.
- **Authority**: the ladder `STATIC` < `INGESTED` < `LOCALLY_OBSERVED`. `EXTERNALLY_ATTESTED` is
  reserved and is not accepted in 1.0.
- **Verified**: a flow variation whose linked assertions passed in run evidence at `INGESTED` or
  higher, stable under the stability policy, with cleanup `done` and every declared negative control
  observed failing. Evidence from commit C holds at a later revision R only when no link target of
  the variation, no path the impact graph reaches from those targets, no path in its tests' static
  reach, no path in the evidence's coverage record, and no global path changed between C and R;
  otherwise it is `evidence-stale`. Behavior outside the declared links, their reach and any
  recorded coverage is not tracked, and the report says so.
- **Static reach**: a test file plus every path the Corvint impact graph reaches from it (imports,
  helpers, page objects and fixtures).
- **Obligation closure**: the changed paths plus every path the Corvint impact graph names as
  depending on them at the head revision.
- **Global paths**: the built-in set plus the provider's declared `global_paths`, which can add paths
  and never remove built-in ones. The built-in set is the flows directory, the provider file, the
  runner configuration, every lockfile and package manifest, every build or container file
  (`Makefile`, `Dockerfile`, compose files), and every fixture, seed-data and environment file that a
  flow intent, the provider or the runner configuration names.
- **Exclusion proof**: the reason a test was left out of an `e2e-safe` selection. It names its basis
  (`coverage` or `reviewed-links`), the evidence or links it rests on, and the changed paths it is
  disjoint from.
- **Effect class**: `read` < `write-reversible` < `write-irreversible` < `external-side-effect`.

## Requirements

### Model, import and export

- `AFU-V1-001`: A flow intent MUST be a closed `application-flow-intent/1` document with `flow_id`,
  `revision` (a positive integer the author increases on every content change), `kind`, `actor`,
  `preconditions`, `steps`, `outcomes`, `variations` and `links`. `corvint flows` reads intents
  only from the directory named by `--flows DIR` inside the repository root, one file per flow,
  named `<flow_id>.json`.
- `AFU-V1-002`: A `flow_id` or `variation_id` MUST NOT be reused for a different behavior or
  regenerated. A renamed flow keeps its ID. A deleted ID stays reserved in a
  `retired.json` list in the same directory.
- `AFU-V1-003`: `flows export` MUST compile the intent set to one
  `corvint-behavior-adapter-request/1` document (DCP-V1-027) and to one external-evidence provider
  record (EEP-V1). There is one reconciler: DCP-V1-031 checks the exported request. AFU-V1 adds no
  second reconciliation.
- `AFU-V1-004`: `flows import` MUST accept a `corvint-behavior-adapter-request/1` document, an OpenAPI
  3.0 or 3.1 document, and a Playwright JSON test list. Each imported flow is written as a new intent
  marked `proposed`, whose links all have basis `inferred` until a reviewed commit removes the mark.
  Import never overwrites an existing intent file.
- `AFU-V1-005`: Importing an exported behavior-adapter request and exporting the result MUST
  reproduce the request's canonical bytes. The `proposed` mark lives only in the intent files.
- `AFU-V1-006`: Flows MAY span repositories. The behavior provider moves to a `/2` profile whose
  `repositories` list names each participating repository by root commit and revision, in place of
  the three fixed members. `/1` readers keep reading `/1`.

### Links and review

- `AFU-V1-007`: Every link MUST carry its basis, its target identity (path and blob span, test key,
  assertion anchor, or evidence digest) and the revision it was evaluated at.
- `AFU-V1-008`: A link is `reviewed` only when its review anchor is valid at the evaluated revision.
  A changed target turns it into `stale`, which counts as unreviewed. A review anchor attests only
  that a commit which changed the flow intent is an ancestor of the evaluated revision, with the
  target unchanged since. Corvint MUST NOT read or compare Git author, committer or signature fields
  as review authority. Every `reviewed` link and reviewed denominator carries
  `review_attestation: "self"`, and the text report states that review identity is not verified.
- `AFU-V1-009`: `inferred` links MUST be reported separately and MUST NOT count toward any reviewed
  denominator, a verified state or an exclusion proof. An `observed` link supports only the
  `coverage` basis (AFU-V1-021) and never counts as reviewed.
- `AFU-V1-010`: Reverse lookups (source path to flows, test key to flows) MUST be derived at query
  time from the forward links at the evaluated revision. They are never stored as a second source of
  truth.

### Wire and command contract

This subsection fixes the S1 wire and argv shapes. It adds no requirement and no root verb.

- Intent: the closed members are `schema`, `flow_id`, `revision`, `proposed` (present only on
  imported intents), `kind`, `actor`, `preconditions`, `steps` (`step_id`, `action`), `outcomes`
  (`outcome_id`, `behavior`, `matcher`, `locator`, `value`), `variations` (`variation_id`,
  `preconditions`, `steps`, `observable_facts`, `outcomes`, `projects`), `links` and the optional
  `adapter` (`derivation`, the DCP-V1 `evidence` anchor, `required_pages`, `negative_controls`,
  `ordered_events`, `missing_e2e_review`). A link has `from` (a step, outcome or variation ID),
  `basis` (`declared` or `inferred`), `target` (`type` `source`, `test`, `assertion` or `evidence`,
  with `path`, `start_line`, `end_line`, `test_key`, `assertion` and `digest`) and `reviewed_at`,
  which `inferred` links refuse. `retired.json` is `application-flow-retired/1` with `flow_ids` and
  `variation_ids`.
- `corvint [--root PATH] flows export --flows DIR --emit inventory|provider|request [--envelope FILE]`
  writes one document to stdout and nothing to the repository. `inventory` is
  `application-flow-inventory/1`, the flow and variation records a behavior-adapter request maps.
  Every emit reads the intents and `retired.json` committed in `DIR` at `HEAD` through Git, with the
  working-tree count, byte, decode and secret bounds, so a dirty or untracked intent file never
  reaches an export. `provider` is an EEP-V1 `corvint-application-flows` record evaluated at `HEAD`:
  one entity per flow and variation, one relation per link whose rule carries the review state, and
  `review_attestation=self` on reviewed links. `request` requires `--envelope`, a
  `corvint-behavior-adapter-request/1` whose `application-flows` input anchor names, by full commit
  ID, path and blob, a committed blob whose bytes are exactly the inventory; Git verifies that before
  export sets that input's document and its flow and variation mappings, and export then refuses any
  result the DCP-V1 adapter refuses. Every exported intent needs an `adapter` member.
- Review evaluation: a review anchor covers a link only when the intent file as committed at the
  anchor already declares the same `from` and `target` with basis `declared` (`link-not-at-anchor`
  otherwise, including a link that was `inferred` at the anchor). Target comparison is by content
  identity, so a target changed and then restored (A to B to A) is unchanged. An `evidence` target is `evidence-unavailable`, never `reviewed`, until S2 supplies a
  run-evidence store that shows it exists at the evaluated revision.
- `corvint [--root PATH] flows import --flows DIR --from FILE --format behavior-adapter-request|openapi|playwright-list`
  is all-or-nothing. It refuses the whole set before writing when any ID exists (a `.json` name in any
  letter case counts), is retired or repeats, or when existing plus imported flows exceed the flow
  bound. It creates each file with `O_EXCL`; when a write fails it removes the files it created,
  names them, and reports that no intent from the import remains written. Rollback assumes the import
  is the only writer in the flows directory: a file another process puts at a created path before
  rollback can be removed. It prints the written paths. OpenAPI input is JSON only.

### Run evidence

- `AFU-V1-011`: A `test-run-evidence/0` record MUST carry the run ID, the runner name and version, the
  source commit and tree with a clean-worktree flag, the build artifact digest, the environment ID and
  digest, the fixture ID and digest, the test key, and every attempt in order. Each attempt carries its
  ordinal, its outcome (`passed`, `failed`, `timedOut`, `skipped` or `interrupted`), its duration and
  the assertion anchors it observed. The record also carries cleanup (`done`, `failed` or
  `not-declared`) and each negative control with its expected and observed outcome.
- `AFU-V1-012`: The ingest adapters for Playwright JSON, JUnit XML (including `flakyFailure` and
  `rerunFailure` elements) and `go test -json` MUST keep every attempt's outcome, duration, failure
  detail and attachments. The Playwright provider MUST stop keeping only the last attempt's detail.
- `AFU-V1-013`: A test with a failed attempt and a later passed attempt MUST be `flaky`, never
  `passed`. Repeated runs aggregate under the DCP-V1-023 counters and the DCP-V1-024 stability policy.
- `AFU-V1-014`: Ingested evidence has authority `INGESTED`. Evidence that Corvint's own observer
  produced (AFU-V0-010) has `LOCALLY_OBSERVED`. A source mapping alone is `STATIC`, and a `STATIC`
  row MUST NOT be shown as verified.

### Queries

- `AFU-V1-015`: `corvint flows map` MUST report each flow, variation, step and outcome with its links,
  basis, authority and verified state at the evaluated revision.
- `AFU-V1-016`: `corvint flows gaps` MUST report each gap with one closed code: `unmapped-flow`,
  `no-test`, `test-without-assertion`, `assertion-unlinked`, `stale-link`, `inferred-only`,
  `evidence-missing`, `evidence-stale`, `evidence-flaky`, `negative-control-missing`,
  `cleanup-unverified` or `unreviewed`. A flow with any gap is `incomplete`.
- `AFU-V1-017`: `corvint flows impact --base SHA` MUST name the flows, variations and test keys that
  the changed paths reach through links and through the Corvint impact graph, with the path of each.
- `AFU-V1-018`: All of these commands are read commands under product invariant 4. Only `import`,
  `docs` rendering and `record` write, and they write only to the paths their arguments name.

### E2E-safe selection (Core)

- `AFU-V1-019`: `corvint affected --provider FILE --selection-profile e2e-safe` MUST select every test
  linked to the change's obligation closure and every test whose file changed. It MAY narrow only when
  every test in the E2E inventory is either selected or has an exclusion proof. The inventory is
  reconciled against runner discovery at the head revision (`--playwright-discovery`, or the
  provider's declared discovery record); a discovered test missing from the provider's inventory, or
  no discovery record, forbids narrowing.
- `AFU-V1-020`: An exclusion proof with basis `reviewed-links` MUST require that every link of the
  test is `reviewed` and not stale at the base revision (a `declared` link without a review anchor,
  and an `inferred` link, cannot support one), that those link targets and the test's static reach
  are disjoint from the obligation closure, and that every changed path outside the global paths is
  the target of at least one reviewed link of some flow. The proof records that link completeness is an
  author attestation.
- `AFU-V1-021`: An `observed` coverage link MAY add tests to the selection. An exclusion proof with
  basis `coverage` MUST require run evidence whose coverage record declares completeness for every
  tier the flow declares (for example, client and server), which holds at the base revision under the
  Verified carry-forward rule (so no covered path changed between the evidence commit and the base),
  and whose covered paths are disjoint from the changed paths.
- `AFU-V1-022`: No test may be omitted when any changed path is a global path. When narrowing is not
  proven, the result MUST be the full relevant suite, meaning every test in the discovered inventory,
  with one or more closed codes: `e2e-unmapped-change`, `e2e-inventory-incomplete`,
  `e2e-exclusion-unproven`, `e2e-global-path-changed`, `e2e-map-stale`, `e2e-inferred-link-only` or
  `e2e-bound-exceeded`. Any such code keeps the ETS state at or above `full`: the profile never
  reports `narrow-selection-allowed` while a code is present.
- `AFU-V1-023`: Every omitted test MUST appear with its exclusion proof and basis in the result, and
  the result counts omissions per basis. A learned or predictive ranking MAY reorder selected tests,
  but MUST NOT remove any. The ETS note is kept, and the `e2e-safe` output adds that each omission is
  proven only on its named basis.
- `AFU-V1-024`: The profile is an additive value of the frozen `--selection-profile` option. The
  `strict` and `coverage` outputs stay byte-identical. The DCP-V1-031 adapter result keeps no
  narrowing authority; only this profile's exclusion proofs narrow.

### Navigation map

- `AFU-V1-025`: `corvint flows navigate` MUST derive an `application-navigation-map/0` from flow
  intents and run evidence. It contains states (a route template or API operation, with a stable state
  ID), transitions (action, locator, preconditions, a readiness condition to wait for, expected
  observations, effect class, and any declared recovery transition), and each transition's verified
  state. Authentication and session setup are precondition flows that name credentials only by
  fixture ID.
- `AFU-V1-026`: Locators MUST come from the flow intent or test source: an accessible role and name, a
  test ID, or an API method and path template. Text scraped from a live page is never served.
- `AFU-V1-027`: Every transition MUST carry an effect class. A declared class MAY be raised by
  observed non-GET traffic or by an undeclared form submit, and is never lowered. An undeclared class
  is `write-irreversible`.
- `AFU-V1-028`: `flows navigate --goal FLOW_ID` MUST return a bounded packet: the ordered steps, the
  locators, the expected observation after each step, the effect class, and the verified state per
  step. An unverified step is labelled unverified, never as working. `--max-effect CLASS` (default
  `read`) names the highest effect class the caller grants; each step above it is marked
  `requires-grant` with the class it needs.
- `AFU-V1-029`: The Corvint observer MUST NOT perform a `write-irreversible` or
  `external-side-effect` transition unless the target origin is listed as disposable in the
  repository-owned `origins.json` in the flows directory.

### Proven documentation

- `AFU-V1-030`: `corvint flows docs` MUST render user documentation from flow intents with fixed
  templates, and a `flow-doc-claims/0` sidecar that maps each rendered claim to its flow, variation,
  outcome, evidence IDs and state. It generates no free prose in 1.0.
- `AFU-V1-031`: A claim is `PROVEN` only when its variation is verified at the evaluated revision.
  Otherwise it is `CONTRADICTED` (the evidence failed), `STALE` (the evidence or link is stale) or
  `UNPROVEN`. The rendered page marks every claim that is not `PROVEN`.
- `AFU-V1-032`: `corvint flows docs --check` MUST fail when a committed claim that was `PROVEN` is no
  longer `PROVEN`, or when the committed rendered bytes differ from regeneration. The only exception
  is an unexpired entry in a `flow-doc-waivers/0` file that names the claim ID, the reason, the
  reviewer and an expiry date.
- `AFU-V1-033`: Hand-written Markdown under the declared docs root MAY opt in with a claim anchor
  comment naming a flow, variation and outcome. Anchored claims are checked like rendered ones under
  AFU-V1-031 and AFU-V1-032, and an anchor naming an unknown flow, variation or outcome fails the
  check. An unanchored document never fails `flows docs --check`. The check lists unanchored
  documents by path in one coverage row with its value, denominator (Markdown documents under the
  docs root), revision and limitation (an anchor proves the named outcome, not the surrounding
  prose). Unanchored text is `UNPROVEN`, never `PROVEN`.

### MCP companion surface

- `AFU-V1-034`: `corvint-mcp --tool-profile flows` MUST advertise read-only `corvint.flows.map`,
  `corvint.flows.gaps`, `corvint.flows.impact` and `corvint.flows.navigate`, following the
  closed-selector rule of `MCPV0-026` (amended when this is delivered). Without the selector,
  `corvint-mcp` output is unchanged.
- `AFU-V1-035`: Repository-authored text in any flows response is returned inside the untrusted-data
  envelope, and is never an instruction to the calling agent.

### Security and bounds

- `AFU-V1-036`: Every write MUST resolve under the repository root without following a symlink.
  `record` and `import` create their files with `O_EXCL`; `docs` rendering replaces a page through a
  confined temporary file in the same directory and a rename. Every input MUST be rejected unless `Lstat` shows a regular file
  before it is opened.
- `AFU-V1-037`: Byte, record, attempt, link and traversal bounds apply before decode and walk. When a
  bound is exceeded, the result is incomplete with a named code, never a truncated success.
- `AFU-V1-038`: Recorded evidence MUST drop cookies, authorization headers, tokens and request or
  response bodies, and pass the product secret screen before any write. Flow intents name credentials
  only by fixture ID.

### Acceptance and evaluation

- `AFU-V1-039`: A committed fixture application MUST carry one UI flow and one API flow that reach
  `verified`, one declared flow with no test (`unmapped-flow`), and one link whose target changed
  after review (`stale-link`). The last two stay `incomplete` in `map`, `gaps`, `navigate` and `docs`.
- `AFU-V1-040`: A frozen, labelled, fault-injected selection corpus MUST report unsafe-narrowing
  rate and reduction ratio per basis, and the count of each fallback code. The unsafe-narrowing rate
  MUST be 0 for a basis to ship.

## Non-goals and simpler baseline

The simpler baseline is today's behavior: run the full relevant E2E suite, read the docs, and let
the agent explore. Every AFU-V1 result falls back to that baseline when it cannot prove more.

Not in 1.0:

- LLM-written documentation prose;
- an export of `application-navigation-map/0` in an external agent format (WebMCP, Arazzo, llms.txt
  or a browser-agent plan): the packet is served only as native JSON by `flows navigate` and
  `corvint.flows.navigate`;
- reviewer identity or separation-of-duties enforcement;
- mandatory claim anchors in hand-written documentation;
- autonomous crawling beyond the AFU-V0 guided observer;
- an API traffic observer (API evidence is ingested);
- `EXTERNALLY_ATTESTED` evidence;
- narrowing from inferred links alone;
- a new root verb (all commands sit under `corvint flows`, `corvint affected` or `corvint-mcp`);
- a new specification language;
- any hosted or networked service.

## Trust boundary, limits, and failure modes

Source code, UI text, imported documents and runner reports are untrusted data. Authority comes
only from repository-owned flow intents with valid review anchors, and from evidence that holds at the
evaluated revision. Review is self-attested: an anchor proves a committed change, not who approved it. The failure modes this spec prevents:

- A title rename silently re-matches another test: title keys are `inferred`, and a changed key
  makes the link `stale`.
- A retry-passed test reads as passed: every attempt is kept, and the test is `flaky`.
- A shared fixture changes and E2E tests are skipped: the fixture is a global path, so no test is
  excluded.
- A new test missing from the provider inventory is skipped: the inventory is reconciled against
  runner discovery, and a gap forbids narrowing.
- A change touches code no flow claims: `e2e-unmapped-change`, and the full suite runs.
- An author under-declares a flow's links: the `reviewed-links` basis carries that risk openly; the
  corpus measures it per basis, and a basis with any unsafe omission is withdrawn.
- A page tells the agent to click something: scraped text is never served.
- An agent performs an irreversible action it was not granted: the packet marks it `requires-grant`.
- A proven documentation claim silently degrades: the drift gate fails.
- A report writes through a symlink or a reader blocks on a FIFO: both are refused.

## Deterministic acceptance and testing matrix

| Requirements | Evidence |
| --- | --- |
| AFU-V1-001 | `TestAFUV1IntentClosedSchema`, `TestAFUV1ExportReadsCommittedIntents`, `TestAFUV1FlowsCLIExportUsesCommittedIntents` |
| AFU-V1-002 | `TestAFUV1RetiredAndDuplicateIDsRefused`, `TestAFUV1ImportRefusesRetiredIDs` |
| AFU-V1-003 | `TestAFUV1ExportCompilesRequestAndProvider`, `TestAFUV1ExportRefusesUnanchoredInventory`, `TestAFUV1ExportRequestRefusesForgedAnchor`, `TestAFUV1ExportReadsCommittedIntents`, `TestAFUV1FlowsCLIExportUsesCommittedIntents`, `TestAFUV1ExportLeavesRepositoryByteIdentical`, `TestAFUV1FlowsCLIExportIsReadOnly` |
| AFU-V1-004 | `TestAFUV1ImportOpenAPIAndPlaywright`, `TestAFUV1ImportNeverOverwrites`, `TestAFUV1FlowsCLIImportNeverOverwrites`, `TestAFUV1ImportRollsBackOnFailedWrite`, `TestAFUV1ImportRefusesCaseVariantName`, `TestAFUV1FlowsCLIImportReportsNothingWritten` |
| AFU-V1-005 | `TestAFUV1RoundTripByteExact` |
| AFU-V1-006 | `/2` multi-repository fixture (S3) |
| AFU-V1-007 | `TestAFUV1ReviewAnchorValidAndStale` |
| AFU-V1-008 | `TestAFUV1ReviewAnchorValidAndStale`, `TestAFUV1ReviewAnchorNotAncestor`, `TestAFUV1ReviewAnchorMustChangeIntent`, `TestAFUV1InferredExcludedFromReviewed`, `TestAFUV1ReviewLinkMustExistAtAnchor`, `TestAFUV1ReviewEvidenceTargetUnavailable`, `TestAFUV1ReviewContentIdentityRestoredTarget`; partial: library evaluation plus the provider record, text report and denominator CLI surface in S3 |
| AFU-V1-009 | `TestAFUV1InferredExcludedFromReviewed`; partial: `Summarize` is library-only, CLI surface in S3 |
| AFU-V1-010 | `TestAFUV1ReverseLookupsDerived`, `TestAFUV1ExportLeavesRepositoryByteIdentical`, `TestAFUV1FlowsCLIExportIsReadOnly`; partial: reverse lookups are library-only, CLI surface in S3 |
| AFU-V1-011..014 | adapter fixtures with a retry-passed test, a timed-out attempt and a failed negative control |
| AFU-V1-015..018 | map, gaps and impact goldens on the fixture application, and the read-only mutation check |
| AFU-V1-019..024 | the fault-injected corpus reported per basis, one case per fallback code, an undiscovered-test case, the byte identity of `strict` and `coverage` |
| AFU-V1-025..029 | navigation goldens, effect raising from observed traffic, `requires-grant` marking, the observer refusal, and a deterministic scripted agent that completes each fixture goal from the packet alone |
| AFU-V1-030..033 | docs goldens, drift failure on a lost `PROVEN`, waiver expiry, the anchored Markdown case |
| AFU-V1-034..035 | MCP conformance with and without the selector |
| AFU-V1-036 | `TestAFUV1InputRegularBeforeOpen`, `TestAFUV1InputSwapAfterLstatRefused`, `TestAFUV1ManifestRegularBeforeOpen`, `TestAFUV1ImportRefusesCaseVariantName`, `TestAFUV1RecordConfinedToRoot`, `TestAFUV1ImportNeverOverwrites`, `TestAFUV1IntentClosedSchema` (symlinked `--flows`) |
| AFU-V1-037 | `TestAFUV1IntentBoundsRefused`, `TestAFUV1IntentCountBoundedBeforeRead`, `TestAFUV1IntentCountBoundedWithoutRetired`, `TestAFUV1ImportCombinedFlowBound`, `TestAFUV1ImportScreensAndBoundsSource`; run-evidence attempt bounds in S2 |
| AFU-V1-038 | `TestAFUV1IntentSecretScreened`, `TestAFUV1ImportScreensAndBoundsSource`; recorded run evidence in S2 |
| AFU-V1-039..040 | the committed acceptance fixture and the frozen corpus report |

Live qualification: the companion surfaces are qualified on Beamfall with one UI flow and one API
flow. The Core profile is qualified by the corpus report (AFU-V1-040) plus one real change against
the Corvint Playwright fixture suite (`conformance/interactive-alpha/fixture`) and one on Beamfall.
Both stay `NOT_RUN` until retained.

## Rollout, rollback, and compatibility

Slices, each its own change with tests:

1. S1 is the intent model, export and import, review anchors, and the two input and output hardening
   fixes (AFU-V1-001..010, 036).
2. S2 is run evidence and the three adapters (AFU-V1-011..014, 037, 038).
3. S3 is `map`, `gaps` and `impact`, and the `/2` behavior provider (AFU-V1-006, 015..018).
4. S4 is the Core `e2e-safe` profile and its corpus (AFU-V1-019..024, 040).
5. S5 is the navigation map (AFU-V1-025..029).
6. S6 is proven documentation (AFU-V1-030..033).
7. S7 is the MCP tool profile (AFU-V1-034, 035).
8. S8 is the acceptance fixture and live qualification (AFU-V1-039).

Rollback removes the `e2e-safe` value, the new `flows` subcommands and the MCP profile. Flow intents,
waivers and claim sidecars are repository data and need no migration. `/1` provider readers are
unchanged. S4 must not ship unless AFU-V1-040 passes. The parts of S1-S3 that `e2e-safe` reads (the
intent model, links, review anchors and run-evidence ingest) are Core-owned; the companion commands
are not. If S4 is not delivered and qualified when the Core candidate freezes, the candidate ships
without the `e2e-safe` value, so neither S4 nor a companion slice blocks it (decision 0385).

## Traceability

| Requirement range | Implementation (planned) |
| --- | --- |
| AFU-V1-001..005, 007 | implemented: `internal/appflows/intent.go`, `tree.go`, `review.go`, `export.go`, `import.go`, `cmd/corvint/flows.go` |
| AFU-V1-008..010 | partial, CLI surface in S3: `internal/appflows/review.go` (`EvaluateLinks`, `Summarize`, `FlowsForPath`, `FlowsForTestKey`); the CLI exposes only the provider record |
| AFU-V1-015..018, 025..033 | `internal/appflows`, `cmd/corvint/flows.go` |
| AFU-V1-006 | `internal/doccorpus/behavior.go` |
| AFU-V1-011..014 | `internal/appflows` evidence, `internal/jstestprovider/playwright.go` |
| AFU-V1-019..024 | `internal/extevidence/selection.go`, `cmd/corvint/affected.go` |
| AFU-V1-034..035 | `internal/mcp`, `cmd/corvint-mcp` |

## Unresolved decisions and promotion or kill criteria

Owner questions, answered by expert review (decision 0385):

1. External agent format for the navigation packet: deferred (Non-goals). Revisit when a format
   reaches a stable release that carries the effect class, verified state and untrusted marking
   without loss, and a named consumer shows a task that fails through `corvint.flows.navigate` and
   succeeds through that format.
2. Distinct reviewer: no; review is self-attested (AFU-V1-008). Revisit when a repository with two or
   more human committers asks for enforced review, or when `EXTERNALLY_ATTESTED` is accepted; the
   form then is a repository-owned reviewer list plus `git verify-commit` against an
   `allowed_signers` file, off by default.
3. Mandatory anchors in hand-written docs: no; opt-in and reported (AFU-V1-033). Revisit when a
   repository asks to protect a docs path, or a qualification finds hand-written drift that an anchor
   would have caught; the form then is a repository-owned mandatory-path list with a ratchet ceiling.

Promotion: Core `e2e-safe` is promoted by an unsafe-narrowing rate of 0 on the frozen corpus and on
both live changes, reported per basis. A basis with a nonzero rate is withdrawn on its own; the
profile keeps the other. The companion surfaces are promoted by the Beamfall qualification. Kill: if the
corpus cannot reach an unsafe-narrowing rate of 0 with a nonzero reduction, `e2e-safe` is withdrawn
and selection stays at `strict`. The companion surfaces ship without it.
