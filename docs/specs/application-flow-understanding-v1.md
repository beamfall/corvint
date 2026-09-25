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
- Status: accepted (decision 0385)/planned; S1-S3 implement AFU-V1-001..005, 007..011, 015..018 and 036..038 (unqualified), 012..014 partially, and AFU-V1-006 as a validated `/2` wire profile no producer emits yet; S4 implements AFU-V1-019..024 and the AFU-V1-040 frozen corpus (unsafe-narrowing rate 0 on both bases), with the two live changes `NOT_RUN`; S5 implements AFU-V1-025..028 and the AFU-V1-029 observer gate, which no observer calls yet (partial); S6 implements AFU-V1-030..033 (unqualified); nothing else is implemented or qualified.
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
  `import` subcommands (`cmd/corvint/flows.go:40-41@97eb5689`), routes `record` through the
  root-confined exclusive writer (`internal/appflows/report.go:317@18a159fa`,
  `internal/appflows/input.go:90-103@06fa25b5`), and checks Lstat before open
  (`internal/appflows/input.go:64-71@8ab13087`).
- The `/1` behavior provider pins exactly three repositories, `app`, `golf_e2e` and `docs_corpus`
  (`internal/doccorpus/behavior.go:41-45@2b4b5d34`). S3 adds the `/2` `repositories` list beside it.
- ETS selection returns `narrow-selection-allowed` once no obligation is uncovered
  (`internal/extevidence/selection.go:823-836@bae209bb`). It proves nothing about the tests it did
  not select, and says so (`internal/extevidence/selection.go:38@a0a072cd`).
- The Playwright reader derives `flaky` from every attempt's status
  (`internal/jstestprovider/playwright.go:193-200@f01509c4`). Since S2 it keeps every attempt's
  duration, failure message, anchor and attachments in memory
  (`internal/jstestprovider/playwright.go:131-133@9ab61026`). The unprofiled receipt carries them
  as the additive `attemptDetails` member (`internal/jstestprovider/receipt.go:65-68@1afda4df`),
  scrubbed per attempt by the shared run-evidence hygiene and the product secret screen
  (`internal/jstestprovider/playwright.go:134-158@17847d1b`). The `corvint-playwright-external`
  profiles refuse the member on encode and on reporter decode
  (`internal/jstestprovider/projection.go:63-65@df31a2ae`,
  `internal/jstestprovider/external.go:281-283@d54a4a23`), because a profiled wire field needs
  another profile revision; their reporter still emits the last attempt's detail only.
- The issue-53 adapter reconciles flows, variations, tests and assertions in both directions, and
  never has narrowing authority (`docs/specs/documentation-corpus-v1.md:191-196@cea38e20`). S4
  leaves it so: only `internal/appflows/selection.go` produces an `e2e-safe` omission.

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

### Run-evidence wire contract

This subsection fixes the S2 wire shape. It adds no requirement and no root verb.

- Record: the closed members are `schema` (`test-run-evidence/0`), `authority`, `run_id`, `runner`
  (`name`, `version`), `source` (`commit`, `tree`, `clean`), `build_artifact_digest`, `environment`
  and `fixture` (`id`, `digest`), `test_key`, the optional `project`, `attempts`, `cleanup` and
  `negative_controls` (`test_key`, `expected`, `observed`). An attempt has `ordinal` (1..n in run
  order), `outcome`, `duration_ms`, `assertion_anchors` (`path`, `line`), the optional `failure` and
  `attachments` (`name`, `path`; a path pointer, never content). The encoding is compact JSON with a
  trailing newline, and decode accepts only that canonical encoding. A `STATIC` record carries only
  the test key: no run header, attempt or negative control, and cleanup `not-declared`.
- Classification: a test's result is its last attempt's outcome, except that a passed last attempt
  after a failed or timed-out one is `flaky` through the shared TCQ-V0-049 rule; a test with no
  attempt is `not-run`. A negative control's `observed` is that classification of the control test in
  the same report. A record is verified only when it is not `STATIC`, its result is `passed`, cleanup
  is `done` and every negative control was observed failing (`failed` or `timedOut`) as its expected
  outcome; a control that passed never supports verification. A Playwright `passed` result is recorded
  as a `failed` attempt, with the reason as its failure, when the test's `expectedStatus` is not
  `passed` (`test.fail()`) or, for its last result, when Playwright marked the test `unexpected`; so
  such a test is never `passed` or verified. A `test.fail()` test that fails as expected keeps its
  `failed` attempt and is classified `failed`, never verified, although Playwright counts it green.
- Ingest formats are `playwright-json`, `junit-xml` and `go-test-json`; the caller supplies the run
  header the report does not carry. Playwright keeps one attempt per `results[]` entry. JUnit keeps
  each `flakyFailure` or `flakyError` as an earlier failed attempt before a final pass, and each
  `rerunFailure` or `rerunError` as a later failed attempt after the first `failure` or `error`;
  `[[ATTACHMENT|path]]` markers in `system-out` are the attachments. `go test -json` keeps each run of
  a test as an attempt, with each `file_test.go:line:` report as an assertion anchor; a test with no
  terminal event is `timedOut` when its package hit the test timeout and `interrupted` otherwise.
- The byte bound, and for JSON reports the depth pre-scan, run on the raw report before decode. The
  record, attempt and link counts, and the JUnit depth, run during the walk before an item is kept;
  a Playwright report is counted after it decodes, which the byte bound caps. Any exceeded bound,
  including an encoded record over the byte bound, makes the whole ingest incomplete, with no
  record, under one code: `run-evidence-byte-bound`, `run-evidence-record-bound` (8192 tests),
  `run-evidence-attempt-bound` (32 attempts), `run-evidence-link-bound` (256 anchors and attachments
  per attempt) or `run-evidence-traversal-bound` (depth 64).
- Hygiene runs before encoding and fails safe. A failure line is replaced by a drop marker when it
  contains, anywhere and in any case, `cookie`, `authorization`, `bearer`, `token`, `secret`,
  `password` or `passwd`, `api-key`, `csrf`, `session`, `credential` or a `basic` credential. A
  request or response body marker (`request:`, `response body:` and the like) anywhere in a line cuts
  the detail there: the text before it is kept, screened the same way, then the drop marker, and
  nothing after it. Attachments without a path, named for cookies, authorization, tokens, secrets,
  passwords, sessions, credentials, requests, responses, bodies or storage state, or pointing at a
  HAR file are dropped. Encoding then refuses secret-shaped content and a record over the byte bound,
  so no ingest returns a record that could not be written.

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

### Query wire contract

This subsection fixes the S3 wire and argv shapes. It adds no requirement and no root verb. Every
query reads the intents committed in `DIR` at `HEAD` through Git, as `export` does, and writes one
compact JSON document to stdout.

- `corvint [--root PATH] flows map --flows DIR [--evidence FILE]...` writes `application-flow-map/1`:
  `schema`, `revision`, `review` (the `Summarize` denominator: `revision`, `reviewed`,
  `reviewed_denominator`, `stale`, `inferred`, `review_attestation` `self` and the `limitation` text
  stating that review identity is not verified) and `flows`. Each flow has `flow_id`, `proposed`,
  `status` (`complete` or `incomplete`, as `gaps` derives it), `steps` and `outcomes` (`id`, `links`)
  and `variations` (`id`, `steps`, `outcomes`, `projects`, `links`, `evidence`, `verified`). A link is
  the evaluated link: `flow`, `from`, `basis`, `review_state`, `target`, `blob`, `revision`,
  `reviewed_at` and `review_attestation` on reviewed links. An `evidence` row has `test_key`, the
  optional `project`, `state` and `authority` (the highest of `STATIC`, `INGESTED` and
  `LOCALLY_OBSERVED` among its matched records, or `none`).
- `flows map --flows DIR --path P` or `--test-key K` writes `application-flow-lookup/1` (`schema`,
  `revision`, `path` or `test_key`, and `flows`, each hit `flow`, `from`, `basis`, `review_state`),
  derived from the forward links at `HEAD`. The two selectors exclude each other and `--evidence`.
  A `--path` that is not a canonical repository-relative path (a leading `./` or `/`, an empty or
  `..` segment) exits 2 as `invalid-arguments`, never an empty answer.
- Evidence files are `test-run-evidence/0` JSONL as `ingest` writes it, read through the same
  regular-file, byte, canonical-decode and combined 8192-record bounds; any refused file refuses the
  query. Evidence pairs are each declared (non-`inferred`) test key of a variation times each of its
  `projects`, or once when it lists none. A record matches a pair by `test_key`, and by `project` when
  the pair has one. The pair's `state` is the first that holds: `missing` (no non-`STATIC` record),
  `stale` (none ran the `HEAD` commit and tree from a clean worktree; only those count below),
  `flaky` (any is `flaky`, or their classifications diverge under the shared TCQ-V0-049 rule, such
  as one `passed` and one `failed`), `failed` (none `passed`), `negative-control-missing` (no passed
  record observed every control failing as expected and ran every `adapter.negative_controls` key),
  `cleanup-unverified` (any passed record has cleanup other than `done`), else `verified`. Across
  records this is the per-pair rule only; DCP-V1-023/024 aggregation stays a declared partial. A
  variation is `verified` only when it has at least one pair and every pair is `verified`. S3 does
  not carry evidence forward
from an earlier commit as the Verified definition allows; such evidence is `stale`, which can
under-report and never over-report.
- `corvint [--root PATH] flows gaps --flows DIR [--evidence FILE]...` writes `application-flow-gaps/1`:
  `schema`, `revision` and `flows`, each `flow_id`, `status` and `gaps` (`code`, the optional
  `member`, `test_key` and `project`, and `detail`). A flow with no link has only `unmapped-flow`. A
  flow whose every link is `inferred` has `inferred-only`. Each `stale` link is `stale-link`; any other
  link neither `reviewed` nor `inferred` is `unreviewed`. A flow that declares no variation has one
  flow-level `no-test`, so it is never `complete`. Per variation: no declared test link is
  `no-test`; declared tests with no declared assertion link from the variation or its outcomes is
  `test-without-assertion`; each of its outcomes with no declared assertion link is
  `assertion-unlinked`; and each evidence pair maps `missing` and `failed` to `evidence-missing`,
  `stale`, `flaky`, `negative-control-missing` and `cleanup-unverified` to the like-named code.
- `corvint [--root PATH] flows impact --flows DIR --base SHA` writes `application-flow-impact/1`:
  `schema`, `base`, `revision`, `changed_paths` (the `base..HEAD` tree diff), `graph` (`digest`,
  `scope` and `unknown`, each `REASON: detail` from the affected plan; an `UNKNOWN` scope means the
  hit list may be short) and `flows`, only those with a hit, each `flow_id`, `variations`,
  `test_keys` and `hits` (`from`, `basis`, `review_state`, `target`, `via`). A link is hit when its
  target path changed (`via` is that path) or when the Corvint impact graph reaches the target's
  owning unit from a changed unit through imports (`via` is the changed path, each unit from the
  changed one to the owner, then the target path; the shortest such path in sorted breadth-first
  order). A hit reaches its own variation or every variation listing its step or outcome, and those
  variations' test keys of any basis. An unresolvable `--base` is `unsupported-affected-revision`.
  The graph is walked from the working tree while intents and changed paths come from `HEAD`, so
  when `git status` reports any tracked or untracked difference from `HEAD` before or after the
  graph build, the scope is `UNKNOWN` with `DIRTY_WORKTREE: detail` and the result never claims a
  complete `HEAD` answer; a failed status read is `unsupported-affected-status`.
- `corvint [--root PATH] flows ingest --format playwright-json|junit-xml|go-test-json --from FILE`
  takes the run header as `--run-id`, `--runner-version`, `--source-commit`, `--source-tree`,
  `--source-clean`, `--build-artifact-digest`, `--environment-id`, `--environment-digest`,
  `--fixture-id`, `--fixture-digest`, `--cleanup` (default `not-declared`) and repeatable
  `--control "SUBJECT<TAB>CONTROL<TAB>EXPECTED"`. It writes the `test-run-evidence/0` JSONL to stdout
  only after every record encodes. An exceeded bound, including a report file over the byte bound,
  exits 2 with the bound's code and writes nothing.
- The `/2` behavior provider (`corvint-corpus-behavior-provider/2`) replaces the `/1` `revisions`
  member with `repositories`, a non-empty list of `root_commit` and `revision` sorted by unique root
  commit, one of which is the provider source; the contract digest covers the declarations as in
  `/1`. The corpus compiler refuses a `/2` registry.

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

### E2E-safe wire contract

This subsection fixes the S4 wire shape. It adds no requirement and no root verb.

- Provider: `--provider FILE` names one repository-relative, closed
  `application-flow-selection-provider/1` document with `flows` (the intent directory), `runner_config`,
  the optional `discovery` (a `playwright-discovery/0` record; `--playwright-discovery` overrides it
  and needs no `--playwright-config`), `global_paths` (directory or file prefixes, for example named
  fixtures and seeds), `inventory` (`test_key`, `project`, `path`), the optional `flow_tiers` (flow ID
  to the tiers its coverage must complete) and the optional `coverage` path. `--repository` is
  refused. Inventory and each path list are bounded (1,024 tests, 4,096 paths); exceeding a bound is
  `e2e-bound-exceeded`.
- Coverage: the named file is a closed `application-flow-coverage/0` record list (`test_key`,
  `commit`, `tiers` of `tier`, `complete`, `paths`), an ingested runner artifact outside the provider
  so that adding a record does not itself change a global path.
- Global paths: the ETS built-in names, `Dockerfile*`, `.env.*`, the provider, the runner config,
  everything under `flows`, and every `global_paths` prefix.
- Result: `advice.test_selection` is the `e2e-safe-selection/0` object with `profile`, `state`,
  `state_reason`, `base`, `mandatory`, `discovery` (`path`, `state`), `selected` (`reasons` from
  `test-file-changed`, `static-reach`, `linked-to-closure`, `observed-coverage`), `omitted_tests`
  (`basis`, `proof` with `links` or `coverage`, `disjoint_from` and, for `reviewed-links`, the
  attestation), `omissions_per_basis`, `fallback` (`code`, `subject`), `note` (the ETS note plus the
  per-basis note) and `untrusted_text_fields`. The ETS `omitted` member is not reused, because it
  counts list cuts. An unreadable or malformed provider or coverage file is `blocked`
  (`provider-unavailable`).
- The coverage basis is tried first. A changed path that neither the impact graph owns, nor a
  reviewed link targets, nor any coverage record names is `e2e-unmapped-change` for that basis. The
  TypeScript `e2e-runtime-dependency` and `executable-config-unresolved` frontiers do not unbound the
  closure under this profile, as under the Playwright profile: reviewed links and coverage prove the
  runtime dependency, and the runner config is a global path reconciled through discovery.

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

### Navigation wire contract

This subsection fixes the S5 wire shape. It adds no requirement and no root verb.

- Intent: an `application-flow-intent/1` document MAY carry the optional closed `navigation` member,
  with `precondition_flows` (flow IDs, run first, never the flow itself) and `steps`. Each step entry
  names one intent step and carries `locator`, `expect` (outcome IDs), and the optional `ready`,
  `input_fixture` (a fixture ID, never a value), `effect` and `recovery` (a later step that no
  variation lists). A `ui` entry has a `state` route template starting with `/` and a locator that
  is exactly `role` and `name`, or a `test_id`; `ready` has the same form. An `api` entry has no `state`: its locator is a `method`
  (`GET`, `HEAD`, `OPTIONS`, `POST`, `PUT`, `PATCH` or `DELETE`) and a `path` template, which are its
  state, and a declared `read` on a method other than `GET`, `HEAD` or `OPTIONS` is refused.
- Traffic: `--traffic FILE` (repeatable) names `application-flow-traffic/0` JSONL records (`flow_id`,
  `step_id`, `method`, `form_submit`) read with the flow-input discipline; with no locator or text
  field, a record can only raise a class. A `method` other than `GET`, or `form_submit` true, raises a
  declared `read` to `write-irreversible` (`effect_basis` `observed-traffic`); an absent `effect` is
  `write-irreversible` (`undeclared`); other declared classes stand (`declared`).
- Map: `application-navigation-map/0` has `revision`, `states` (`state_id` `ui:<route>` or
  `api:<METHOD> <path>`, `kind` `route` or `api-operation`, `template`) and per flow `status` (as
  `flows gaps`), `precondition_flows` and `transitions`, one per intent step. A transition carries
  `action`, `state`, `locator`, the flow's `preconditions`, `ready`, `input_fixture`, `expect` (the
  outcome records), `effect_class`, `effect_basis`, `recovery` and `verification`. A step is
  `verified` only when a variation that lists it has every required evidence pair verified; a step
  with no navigation entry has no state or locator. A precondition flow that is not declared or that
  closes a cycle refuses the map.
- Packet: `--goal` writes `application-navigation-packet/0` with `goal`, `max_effect`, the referenced
  `states`, `flows` (each with its `status`) and `steps`: the precondition flows depth first, each
  once, then the goal, in intent step order. Steps some other step names as its `recovery` move to
  `recovery`. Each step carries `grant` `granted`, or `requires-grant` with `grant_needed` when its
  class is above `max_effect`. More than 256 steps, or more than the 8,192 traffic records, is
  `navigation-bound-exceeded`. `--max-effect` without `--goal` is refused.
- Observer gate: `origins.json` in the flows directory is a closed `application-flow-origins/0`
  document with `disposable`, unique canonical `http` or `https` origins (`scheme://host[:port]`,
  lower case, no user, path, query or fragment), read at the evaluated commit, never the working
  tree; an absent file lists none. A `read` or `write-reversible` transition is admitted anywhere;
  any other class, unknown included, only against a listed origin, else
  `observer-origin-not-disposable`.

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

### Proven-documentation wire contract

This subsection fixes the S6 wire and argv shapes. It adds no requirement and no root verb. Intents,
the committed page and sidecar, the waivers file and the hand-written Markdown are all read at `HEAD`
through Git; `--page`, `--claims`, `--docs-root` and `--waivers` are canonical repository-relative
paths, and `--page` and `--claims` differ.

- `corvint [--root PATH] flows docs --flows DIR --page FILE --claims FILE [--docs-root DIR]
  [--evidence FILE]...` renders the fixed template and writes the page and then the sidecar, each
  through an exclusively created temporary file in the target's directory and a rename; an existing
  non-regular target or a non-directory parent is refused. It prints the two written paths. An
  `invalid-anchor` or `unknown-anchor` under `--docs-root` refuses the render.
- There is one claim per outcome listed by each variation. A page claim ID is
  `FLOW/VARIATION/OUTCOME`; an anchored claim ID is `PATH:FLOW/VARIATION/OUTCOME` with the document
  path as `source`. The page carries each claim ID in a code span, prefixes every claim that is not
  `PROVEN` with its state in bold, and backslash-escapes all ASCII punctuation in intent text. It
  embeds no commit, so regeneration from equal intents and claim states is byte-identical.
- A claim is `CONTRADICTED` when any evidence pair of its variation is `failed` or `flaky`; else
  `STALE` when a link from the variation, one of its steps or the outcome is `stale`, or a pair is
  `stale`; else `PROVEN` only when every pair is `verified`, the outcome has a declared assertion
  link, and every declared link from those members is `reviewed`; else `UNPROVEN`.
- `flow-doc-claims/0`: `schema`, `page` and `claims` sorted by `id`, each with `id`, `source`,
  `flow`, `variation`, `outcome`, `evidence` (the required `test_key` and optional `project` pairs,
  never run IDs) and `state`.
- `flow-doc-waivers/0`: `schema` and `waivers`, each with a unique `claim` ID, `reason`, `reviewer`
  and `expires` (`YYYY-MM-DD`). A waiver is unexpired while the current UTC date is before
  `expires`; on that date it has expired. The file is closed-schema and a malformed file refuses the
  check.
- `flows docs --check ... [--waivers FILE]` writes nothing to the repository and prints
  `flow-doc-check/0`: `schema`, `revision`, `status` (`pass` or `fail`), `failures` (`code`,
  optional `claim`, `path` and `line`, and `detail`), `waived` (claim IDs) and, with `--docs-root`,
  `coverage`: one `unanchored-documents` row with `value` (the unanchored count), `denominator`
  (Markdown documents under the docs root, the page excluded), `revision`, `rule`, `limitation` and
  `paths`. A `.md` entry that is not a regular file, such as a symlink or a submodule, is not read and
  counts as unanchored. The failure codes are `claim-lost-proven`, `rendered-bytes-differ` (for an absent file
  too), `unknown-anchor` and `invalid-anchor`. A committed claim whose state changed, or which is no
  longer rendered, is waived by an unexpired entry naming its ID, and its committed form is then
  used for the byte comparison. A failing check exits 2 with `flow-docs-check-failed`.

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
| AFU-V1-006 | `TestAFUV1BehaviorProviderV1BytesUnchanged`, `TestAFUV1BehaviorProviderV2MultiRepository`; partial: `/2` is a validated wire profile (`ValidateBehaviorProviderV2`), but no producer emits it and the external-evidence provider registry accepts only `/1` |
| AFU-V1-007 | `TestAFUV1ReviewAnchorValidAndStale` |
| AFU-V1-008 | `TestAFUV1ReviewAnchorValidAndStale`, `TestAFUV1ReviewAnchorNotAncestor`, `TestAFUV1ReviewAnchorMustChangeIntent`, `TestAFUV1InferredExcludedFromReviewed`, `TestAFUV1ReviewLinkMustExistAtAnchor`, `TestAFUV1ReviewEvidenceTargetUnavailable`, `TestAFUV1ReviewContentIdentityRestoredTarget`, `TestAFUV1FlowsQueryGoldens` (the `review` denominator and limitation in `map`) |
| AFU-V1-009 | `TestAFUV1InferredExcludedFromReviewed`, `TestAFUV1FlowsQueryGoldens` (inferred links outside the denominator, `inferred-only`, no evidence pair from an inferred test) |
| AFU-V1-010 | `TestAFUV1ReverseLookupsDerived`, `TestAFUV1ExportLeavesRepositoryByteIdentical`, `TestAFUV1FlowsCLIExportIsReadOnly`, `TestAFUV1FlowsCLIReverseLookups` (a non-canonical `--path` refused), `TestAFUV1FlowsQueriesAreReadOnly` |
| AFU-V1-011 | `TestAFUV1RunEvidenceClosedSchema`, `TestAFUV1NegativeControlFailed`, `TestAFUV1FlowsCLIPassingControlNeverVerifies` (a control that passed never verifies) |
| AFU-V1-012 | `TestAFUV1PlaywrightAdapterKeepsEveryAttempt`, `TestAFUV1JUnitAdapterKeepsEveryAttempt`, `TestAFUV1GoTestAdapterKeepsEveryAttempt`, `TestAFUV1PlaywrightProviderKeepsEveryAttempt` (the unprofiled receipt wire carries `attemptDetails`, and the external profiles refuse it); partial: the `corvint-playwright-external` reporter and profiles still carry only the last attempt's detail, which needs a `/3` profile revision and its live reporter qualification |
| AFU-V1-013 | `TestAFUV1FailedAttemptThenPassIsFlaky`, `TestAFUV1PlaywrightUnexpectedNeverPassed`, the retry-passed case of each adapter test; partial: per-test classification only, aggregation of repeated runs under the DCP-V1-023 counters and DCP-V1-024 policy needs the `internal/doccorpus` stability counting exposed for `test-run-evidence/0` records |
| AFU-V1-014 | `TestAFUV1StaticNeverVerified`, `TestAFUV1PlaywrightUnexpectedNeverPassed`; partial: ingest emits `INGESTED` and the schema accepts `LOCALLY_OBSERVED`, but the AFU-V0-010 observer does not yet emit run-evidence records |
| AFU-V1-015 | `TestAFUV1FlowsQueryGoldens` (`map.golden.json`), `TestAFUV1EvidenceStateOrder` (including mixed pass and fail across current records, and a passed record without cleanup `done`), `TestAFUV1ReadRunEvidenceDiscipline` |
| AFU-V1-016 | `TestAFUV1FlowsQueryGoldens` (`gaps.golden.json`, every gap code reached), `TestAFUV1EvidenceStateOrder`, `TestAFUV1ReadRunEvidenceDiscipline`, `TestAFUV1ZeroVariationFlowIncomplete` |
| AFU-V1-017 | `TestAFUV1FlowsQueryGoldens` (`impact.golden.json`: a direct hit and a hit through the impact graph), `TestAFUV1FlowsCLIReverseLookups` (unresolvable base), `TestAFUV1FlowsImpactDirtyWorktreeUnknown` (an uncommitted edit makes the scope `UNKNOWN`, never a confident no-hit) |
| AFU-V1-018 | `TestAFUV1FlowsQueriesAreReadOnly` (`map`, lookup, `gaps`, `impact`, `ingest` and `export` leave the repository, `.git` included, byte-identical) |
| AFU-V1-019 | `TestAFUV1019UndiscoveredTestForbidsNarrowing`, `TestAFUV1040SelectionCorpusReport` (the `undiscovered-test`, `missing-discovery`, `stale-discovery` and `test-file` cases) |
| AFU-V1-020 | `TestAFUV1020ExclusionProofsPerBasis` (a reviewed-links proof, a declared link without a review anchor, an inferred-only link), `TestAFUV1040SelectionCorpusReport` |
| AFU-V1-021 | `TestAFUV1020ExclusionProofsPerBasis` (a coverage proof, stale coverage evidence, an incomplete tier), `TestAFUV1040SelectionCorpusReport` |
| AFU-V1-022 | `TestAFUV1022EveryFallbackCodeYieldsFullSuite` (one subtest per closed code), `TestAFUV1040SelectionCorpusReport` (global-path cases) |
| AFU-V1-023 | `TestAFUV1020ExclusionProofsPerBasis`, `TestAFUV1040SelectionCorpusReport` (per-basis counts, both notes) |
| AFU-V1-024 | `TestAFUV1024StrictAndCoverageBytesUnchanged` (goldens captured before S4), `TestAFUV1024E2ESafeRefusesMalformedInput`, `TestAffectedSelectionArguments` |
| AFU-V1-025 | `TestAFUV1025NavigationMapGolden` (`navigate.golden.json`: states, readiness, expected outcomes, recovery, verified and unverified steps, credentials by fixture ID, read-only), `TestAFUV1025PreconditionFlowsResolve` |
| AFU-V1-026 | `TestAFUV1026LocatorsOnlyFromIntent` (no locator for an unmapped step; traffic with page text refused), `TestAFUV1026NavigationLocatorShapes` |
| AFU-V1-027 | `TestAFUV1027EffectClasses` (raised by observed `POST`, never lowered, undeclared), `TestAFUV1027EffectRaisedNeverLowered` |
| AFU-V1-028 | `TestAFUV1028PacketRequiresGrant` (`navigate-checkout.golden.json`), `TestAFUV1028ScriptedAgentCompletesGoals` (a scripted agent completes each mapped fixture goal from the packet alone, including a recovery, performs no write under the default `read`, and cannot run an unmapped goal), `TestAFUV1028PacketBound` |
| AFU-V1-029 | `TestAFUV1029ObserverRefusesNonDisposableOrigin` (listed, unlisted, non-canonical and unknown-class cases; absent, uncommitted and malformed `origins.json`); partial: `AdmitTransition` is the gate, but no Corvint observer executes navigation transitions yet, and the AFU-V0 manifest observer carries no effect class |
| AFU-V1-030 | `TestAFUV1030DocsRenderGoldenAndByteStable` (`docs.golden.md`, `docs.claims.golden.json`, regeneration at a later commit byte-identical, argument refusals) |
| AFU-V1-031 | `TestAFUV1031ClaimStateOrder`, `TestAFUV1030DocsRenderGoldenAndByteStable` (every state marked on the page, `PROVEN` unmarked) |
| AFU-V1-032 | `TestAFUV1032DocsCheckDrift` (a lost `PROVEN` and a hand-edited page fail; the check is read-only), `TestAFUV1032WaiverExpiry` (unexpired, expired and malformed waivers), `TestAFUV1032WaiverExpiryBoundary` (expired on its expiry date), `TestAFUV1032WaiverKeepsUnrenderedClaim` (a waived claim that is no longer rendered keeps its committed form) |
| AFU-V1-033 | `TestAFUV1033AnchoredMarkdown` (a changed unanchored document passes, an unknown anchor fails the check and refuses the render), `TestAFUV1033AnchorParsing`, `TestAFUV1032DocsCheckDrift` (the coverage row and the anchored claim's lost `PROVEN`), `TestAFUV1033NonRegularMarkdownSkipped` (a symlinked `.md` is unanchored, not a failure) |
| AFU-V1-034..035 | MCP conformance with and without the selector |
| AFU-V1-036 | `TestAFUV1036DocsReplaceConfined`, `TestAFUV1InputRegularBeforeOpen`, `TestAFUV1InputSwapAfterLstatRefused`, `TestAFUV1ManifestRegularBeforeOpen`, `TestAFUV1ImportRefusesCaseVariantName`, `TestAFUV1RecordConfinedToRoot`, `TestAFUV1ImportNeverOverwrites`, `TestAFUV1IntentClosedSchema` (symlinked `--flows`) |
| AFU-V1-037 | `TestAFUV1IntentBoundsRefused`, `TestAFUV1IntentCountBoundedBeforeRead`, `TestAFUV1IntentCountBoundedWithoutRetired`, `TestAFUV1ImportCombinedFlowBound`, `TestAFUV1ImportScreensAndBoundsSource`, `TestAFUV1RunEvidenceBoundsIncomplete`, `TestAFUV1FlowsCLIIngest`, `TestAFUV1ReadRunEvidenceDiscipline` |
| AFU-V1-038 | `TestAFUV1IntentSecretScreened`, `TestAFUV1ImportScreensAndBoundsSource`, `TestAFUV1RunEvidenceSecretsDropped`, `TestAFUV1PlaywrightProviderScrubsEveryAttempt` (the provider receipt scrubs every attempt and the last-attempt fields); the screen runs in `EncodeRunEvidence`, the only run-evidence encoding, and `flows ingest` writes only what it encodes (`TestAFUV1FlowsCLIIngest`) |
| AFU-V1-039 | the committed acceptance fixture |
| AFU-V1-040 | `TestAFUV1040SelectionCorpusReport`: the frozen corpus `cmd/corvint/testdata/e2e-safe-corpus.json` (20 labelled, fault-injected cases over five tests) and its report `cmd/corvint/testdata/e2e-safe-corpus.report.json`; `coverage` omits 15 with 0 unsafe (reduction 0.15), `reviewed-links` omits 3 with 0 unsafe (reduction 0.03), no basis withdrawn |

Live qualification: the companion surfaces are qualified on Beamfall with one UI flow and one API
flow. The Core profile is qualified by the corpus report (AFU-V1-040) plus one real change against
the Corvint Playwright fixture suite (`conformance/interactive-alpha/fixture`) and one on Beamfall.
Both stay `NOT_RUN` until retained.

## Rollout, rollback, and compatibility

Slices, each its own change with tests:

1. S1 is the intent model, export and import, review anchors, and the two input and output hardening
   fixes (AFU-V1-001..010, 036).
2. S2 is run evidence and the three adapters (AFU-V1-011..014, 037, 038). It delivered the record,
   the adapters, the bounds and the hygiene, and the unprofiled provider receipt now carries and
   scrubs every attempt; the AFU-V1-012..014 remainders named in the matrix stay open.
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
| AFU-V1-008..010 | implemented: `internal/appflows/review.go` (`EvaluateLinks`, `Summarize`, `FlowsForPath`, `FlowsForTestKey`), surfaced by `flows map` in `internal/appflows/query.go` |
| AFU-V1-015..018 | implemented: `internal/appflows/query.go`, `impact.go`, `runingest.go` (`IngestRunFile`), `cmd/corvint/flows.go` |
| AFU-V1-025..029 | implemented, 029 partial (see the matrix): `internal/appflows/navigate.go`, `origins.go`, the `navigation` member in `intent.go`, `cmd/corvint/flows_navigate.go` |
| AFU-V1-030..033 | implemented: `internal/appflows/docs.go` (`RenderDocs`, `CheckDocs`, `ReplaceConfined`), `cmd/corvint/flows_docs.go` |
| AFU-V1-006 | partial (see the matrix): `internal/doccorpus/behavior.go` |
| AFU-V1-011..014, 038 | implemented, 012..014 partial (see the matrix): `internal/appflows/runevidence.go`, `runingest.go`, `internal/runhygiene/runhygiene.go`, `internal/jstestprovider/playwright.go`, `receipt.go`, `projection.go`, `external.go` |
| AFU-V1-019..024, 040 | implemented: `internal/appflows/selection.go` (`SelectE2E`), `cmd/corvint/affected.go`, `internal/liveverify/affected/typescript/playwright_discovery.go` (`VerifyPlaywrightDiscovery`), `internal/extevidence/selection.go` (`SelectionNote`); corpus `cmd/corvint/testdata/e2e-safe-corpus.json` |
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
