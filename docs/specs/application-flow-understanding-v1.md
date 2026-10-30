# Application Flow Understanding V1 — proven flows, safe E2E selection, navigation and documentation

Owner: Russell Lewis
Date: 2026-09-24
Intent status: accepted (decision 0385)
Delivery status: partially delivered; original issue-175 flows qualified locally
Authoritative inputs: owner request [issue 175](https://github.com/beamfall/corvint/issues/175) and
the owner's additions of 2026-09-24 (mapping E2E tests to code changes, an agent that can navigate
and use a website, documentation proven accurate by the same evidence), `AGENTS.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/specs/application-flow-understanding-v0.md`,
`docs/specs/documentation-corpus-v1.md` (issue 53, `DCP-V1-027..032`),
`docs/specs/external-test-selection-v1.md`, `docs/specs/corvint-1.0-product-and-release-v1.md`,
decisions 0374, 0385 and 0416; source-generation extension [issue 338](https://github.com/beamfall/corvint/issues/338).

## Agent digest
- Claim: Reviewed flows link to source, tests and run evidence; Corvint selects E2E tests with exclusion proofs, maps navigation and proves documentation claims.
- Status: accepted (decision 0385); partially delivered; original issue-175 flows qualified locally. S1-S4, S6-S8 delivered with local issue-175 evidence in `evidence/issues-167-175/`; S5 delivered with the 2026-10-08 navigation execution qualification. AFU-V1-012 now retains and consumes the qualified `/3` external attempt inventory. The Beamfall UI/API lifecycle and both semantic-change qualifications pass; the latter use conservative full-suite fallback, not reduced selections. Compiled MCP conformance covers both protocols and the pinned official schema. AFU-V1-006 has a bounded `/2` producer and declaration-only corpus ingestion; external runtime qualification remains unobserved; S5's AFU-V1-029 observer integration is delivered by the experimental navigation execution companion (decision 0455, `application-flow-navigation-execution-v0.md`), locally qualified on 2026-10-08; AFU-V1-014 retains its accepted observer gap.
- Exists: AFU-V1-049..053 adds fail-closed variation coverage and safe write-back; final source-bound qualification remains separately recorded. AFU-V1-043..048 defines the separately generated non-Go documentation profile, and AFU-V1-101..106 its optional declared flow inventory; it never upgrades the existing accepted-outcome documentation truth table. the AFU-V0 experimental `corvint flows` report and `record`, the issue-53 behavior adapter, ETS-V1 selection and the Playwright provider this spec extends.
- Blocked on: AFU-V1-006 external runtime qualification; production backend and hosted-CI qualification are not observed.
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
  root-confined exclusive writer (`internal/appflows/report.go:324@18a159fa`,
  `internal/appflows/input.go:161-174@06fa25b5`), and checks Lstat before open
  (`internal/appflows/input.go:135-142@8ab13087`).
- The `/1` behavior provider pins exactly three repositories, `app`, the legacy /1 member and `docs_corpus`
  (`internal/doccorpus/behavior.go:47-51@2b4b5d34`). S3 adds the `/2` `repositories` list beside it.
- ETS selection returns `narrow-selection-allowed` once no obligation is uncovered
  (`internal/extevidence/selection.go:823-836@bae209bb`). It proves nothing about the tests it did
  not select, and says so (`internal/extevidence/selection.go:38@a0a072cd`).
- The Playwright reader derives `flaky` from every attempt's status
  (`internal/jstestprovider/playwright.go:237-244@f01509c4`). Since S2 it keeps every attempt's
  duration, failure message, anchor and attachments in memory
  (`internal/jstestprovider/playwright.go:173-175@9ab61026`). The unprofiled receipt carries them
  as `attemptDetails` (`internal/jstestprovider/receipt.go`). The shared hygiene scrubs each
  attempt. The external reporter emits the same complete inventory only under the separate `/3`
  profile (`internal/jstestprovider/attempt_details.go`); earlier profiles continue to refuse it.
- The issue-53 adapter reconciles flows, variations, tests and assertions in both directions, and
  never has narrowing authority (`docs/specs/documentation-corpus-v1.md:198-203@cea38e20`). S4
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
  Import never overwrites an existing intent file. A Playwright list import places, keys and
  reports its tests as AFU-V1-128..130 require.
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
- `corvint [--root PATH] flows import --flows DIR --from FILE --format behavior-adapter-request|extractor-inventory|openapi|playwright-list`
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
- `AFU-V1-012`: The ingest adapters for Playwright JSON, the qualified `/3` Playwright receipt, JUnit XML (including `flakyFailure` and
  `rerunFailure` elements) and `go test -json` MUST keep every attempt's outcome, duration, failure
  detail and attachments. The Playwright provider MUST stop keeping only the last attempt's detail.
- `AFU-V1-013`: A test with a failed attempt and a later passed attempt MUST be `flaky`, never
  `passed`. Repeated runs aggregate under the DCP-V1-023 counters and the DCP-V1-024 stability policy.
- `AFU-V1-014`: Ingested evidence has authority `INGESTED`. Evidence that Corvint's own observer
  produced (AFU-V0-010) has `LOCALLY_OBSERVED`. A source mapping alone is `STATIC`, and a `STATIC`
  row MUST NOT be shown as verified. Accepted gap (decision 0417): the AFU-V0-010 observer is
  flow-scoped and has no test key, so it emits no per-test `LOCALLY_OBSERVED` record; the schema
  accepts the authority, and no Corvint producer emits it.
- `AFU-V1-041`: The planned repetition ordinals, the planned or manual run kind, the infrastructure
  failure class and the DCP-V1-024 policy of repeated runs MUST come from a separate
  repository-owned run registry read from the commit at `HEAD`, never from a `test-run-evidence/0`
  record, whose wire shape stays unchanged (decision 0417, PR #250). Each registry contribution MUST
  join exactly one record by run ID, test key and project.
- `AFU-V1-042`: `corvint flows stability` MUST report, per registry aggregate, the selected scope,
  the policy ID, the registry digest, the DCP-V1-023 raw counts, the applied threshold and a verdict,
  with every contributing run and all its attempts. A contribution that breaks DCP-V1-022, cannot
  join its record or contradicts it MUST refuse the command with one named code and no verdict. The
  command writes only to stdout.

### Run-evidence wire contract

This subsection fixes the S2 wire shape. It adds no requirement and no root verb.

- Record: the closed members are `schema` (`test-run-evidence/0`), `authority`, `run_id`, `runner`
  (`name`, `version`), `source` (`commit`, `tree`, `clean`), `build_artifact_digest`, `environment`
  and `fixture` (`id`, `digest`), `test_key`, the optional `project`, `attempts`, `cleanup` and
  `negative_controls` (`test_key`, `expected`, `observed`), and the optional `coverage_artifacts`
  (`id`, `digest`: at most 64, unique IDs, each digest a SHA-256 of a coverage file the run wrote;
  none on a `STATIC` record, AFU-V1-120). An attempt has `ordinal` (1..n in run
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
  `[[ATTACHMENT|path]]` markers in `system-out` are the attachments. One of those six failure
  elements is a `timedOut` attempt, not `failed`, only when its `message` attribute contains the
  case-sensitive phrase `timed out after ` (JUnit 4 `TestTimedOutException`, JUnit Jupiter
  `TimeoutException`); the same phrase in body text or in any other attribute leaves it `failed`
  (decision 0417). `go test -json` keeps each run of
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

### Run-registry wire contract

- Registry `flows-run-registry/0`, a committed regular file read at `HEAD` with the flow-input
  discipline (byte bound, strict decode, secret screen): `schema`, `policy` (`id` and `thresholds`,
  exactly one per scope `one-spec`, `feature-batch` and `suite`, each with the DCP-V1-024 members
  `required_repetitions`, `minimum_passed` and the `maximum_*` limits of `internal/doccorpus`) and
  `aggregates`, 1..8192, each with a unique `id`, `scope`, `test_key`, the optional `project`,
  `planned` (equal to the scope's `required_repetitions`) and `contributions`. A contribution has
  `run_id`, `run_kind` (`planned-repetition` or `manual-rerun`), `repetition` (1..`planned` for a
  planned run, 0 for a manual rerun) and `infrastructure_attempts`, the ordinals the owner classes as
  infrastructure failures.
- Report `flows-run-stability/0`: `schema`, `revision` (the `HEAD` commit), `registry` (its path),
  `registry_sha256` (of the committed bytes, which bind the policy), `policy_id`, `aggregates` and
  `limitations`. An aggregate has `id`, `scope`, `test_key`, `project`, the shared `runner` and
  `source`, `threshold`, `counts`, `verdict` and `contributions`, each the registry contribution with
  the joined record's `classification`, `cleanup` and `attempts`.
- Counting: each planned repetition adds one to `started`; to `completed` when its classification is
  `passed`, `failed`, `flaky` or `skipped`; to `passed` or `flaky` by classification; to `failed`,
  `timed_out`, `interrupted` and `skipped` when any attempt had that outcome; to
  `infrastructure_failed` when it names an infrastructure attempt; to `cleanup_failed` when cleanup is
  not `done`; and `len(attempts)-1` to `retry_consumed`. A manual rerun adds one to `manual_reruns`
  and, when its cleanup is not `done`, to `cleanup_failed`, and nothing else. The verdict is `clean`
  exactly when the `internal/doccorpus` DCP-V1-024 threshold test passes, else `not-stable`.
- Refusals, each naming the aggregate: `run-registry-invalid` (shape, policy or aggregate identity),
  `run-registry-invalid-repetition` (an ordinal out of range or repeated, or a manual rerun with an
  ordinal), `run-registry-planned-incomplete` (a planned ordinal missing), `run-registry-duplicate-run`
  (one run ID twice in an aggregate), `run-registry-missing-record` and
  `run-registry-ambiguous-record` (zero or several records join), `run-registry-unusable-record` (a
  `STATIC` record or a dirty source), `run-registry-contradictory-infrastructure` (an infrastructure
  ordinal that is absent, repeated, or on a `passed` or `skipped` attempt) and
  `run-registry-cross-identity` (runner, source, build artifact, environment, fixture or project
  differ from the aggregate's first contribution).

### Queries

- `AFU-V1-015`: `corvint flows map` MUST report each flow, variation, step and outcome with its links,
  basis, authority and verified state at the evaluated revision.
- `AFU-V1-016`: `corvint flows gaps` MUST report each gap with one closed code: `unmapped-flow`,
  `no-test`, `test-without-assertion`, `assertion-unlinked`, `stale-link`, `inferred-only`,
  `evidence-missing`, `evidence-stale`, `evidence-flaky`, `negative-control-missing`,
  `cleanup-unverified`, `stability-missing`, `stability-invalid`, `stability-incomplete`,
  `stability-stale`, `stability-failed` or `unreviewed`. A flow with any gap is `incomplete`.
- `AFU-V1-017`: `corvint flows impact --base SHA` MUST name the flows, variations and test keys that
  the changed paths reach through links and through the Corvint impact graph, with the path of each.
- `AFU-V1-018`: All of these commands are read commands under product invariant 4. Only `import`,
  `docs` rendering and `record` write, and they write only to the paths their arguments name.

### Query wire contract

This subsection fixes the S3 wire and argv shapes. It adds no requirement and no root verb. Every
query reads the intents committed in `DIR` at `HEAD` through Git, as `export` does, and writes one
compact JSON document to stdout.

- `corvint [--root PATH] flows map --flows DIR [--evidence FILE]... [--registry FILE]` writes `application-flow-map/1`:
  `schema`, `revision`, `review` (the `Summarize` denominator: `revision`, `reviewed`,
  `reviewed_denominator`, `stale`, `inferred`, `review_attestation` `self` and the `limitation` text
  stating that review identity is not verified) and `flows`. Each flow has `flow_id`, `proposed`,
  `status` (`complete` or `incomplete`, as `gaps` derives it), `steps` and `outcomes` (`id`, `links`)
  and `variations` (`id`, `steps`, `outcomes`, `projects`, `links`, `evidence`, `verified`). A link is
  the evaluated link: `flow`, `from`, `basis`, `review_state`, `target`, `blob`, `revision`,
  `reviewed_at` and `review_attestation` on reviewed links. An `evidence` row has `test_key`, the
  optional `project`, `state`, optional `stability_detail` (the blocking aggregate ID or registry refusal),
  and `authority` (the highest of `STATIC`, `INGESTED` and
  `LOCALLY_OBSERVED` among its matched records, or `none`).
- `flows map --flows DIR --path P` or `--test-key K` writes `application-flow-lookup/1` (`schema`,
  `revision`, `path` or `test_key`, and `flows`, each hit `flow`, `from`, `basis`, `review_state`),
  derived from the forward links at `HEAD`. The two selectors exclude each other, `--evidence` and `--registry`.
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
  `cleanup-unverified` (any passed record has cleanup other than `done`). Passing these run checks
  alone does not verify a pair. `--registry FILE` names the committed `flows-run-registry/0` at the
  same captured revision as the intents. Working-tree policy edits are ignored. The shared
  AFU-V1-041/042 validator and threshold evaluator apply; every aggregate naming the exact test key
  and actual project must qualify, so a caller cannot choose a weaker scope while another fails.
  A variation without declared projects requires qualification for each actual current project.
  No registry, absent committed file or missing matching aggregate yields `stability-missing`;
  malformed policy or invalid contributions yield `stability-invalid`; missing planned ordinals or
  missing records yield `stability-incomplete`; an aggregate bound to another source commit/tree
  yields `stability-stale`; a non-clean threshold verdict yields `stability-failed`.
  Every joined contribution, including manual reruns, must also pass the existing run checks with
  cleanup done and the variation's applicable controls. A permissive threshold never overrides
  flakiness, missing controls or cleanup failure. Only then is the pair `verified`. A variation is
  `verified` only when it has at least one pair and every pair is `verified`.
  `flows navigate` and `flows docs` accept the same optional `--registry` and use this same rule;
  no per-run pass alone verifies navigation or proves a documentation claim. S3 does not carry evidence forward
from an earlier commit as the Verified definition allows; such evidence is `stale`, which can
under-report and never over-report.
- `corvint [--root PATH] flows gaps --flows DIR [--evidence FILE]... [--registry FILE]` writes `application-flow-gaps/1`:
  `schema`, `revision` and `flows`, each `flow_id`, `status` and `gaps` (`code`, the optional
  `member`, `test_key` and `project`, and `detail`). A flow with no link has only `unmapped-flow`. A
  flow whose every link is `inferred` has `inferred-only`. Each `stale` link is `stale-link`; any other
  link neither `reviewed` nor `inferred` is `unreviewed`. A flow that declares no variation has one
  flow-level `no-test`, so it is never `complete`. Per variation: no declared test link is
  `no-test` (its detail names inferred-only test links, AFU-V1-252); declared tests with no declared assertion link from the variation or its outcomes is
  `test-without-assertion`; each of its outcomes with no declared assertion link is
  `assertion-unlinked`; and each evidence pair maps `missing` and `failed` to `evidence-missing`,
  `stale`, `flaky`, `negative-control-missing` and `cleanup-unverified` to the like-named code;
  stability states map to their identically named gap code, with the registry refusal or aggregate
  ID in `detail` when available.
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
- `corvint [--root PATH] flows stability --registry FILE [--evidence FILE]...` reads the registry
  `FILE`, a repository-relative path, from `HEAD` and writes `flows-run-stability/0` (Run-registry
  wire contract). A refusal exits 2 with its code and writes nothing.
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
  `/1`. `docs corpus behavior-provider --input REQUEST.json` emits `/2` from a closed
  `corvint-behavior-provider-request/2` document. Corpus ingestion retains this profile and checks
  local immutable anchors, comparing `repositories` against manifest `behavior_repositories`.
  Unavailable external bytes, missing expectations/members, stale revisions and unresolved links
  remain gaps. Discovery, migration and runtime qualification remain unknown, with empty verified
  sets and unconditional full-relevant-suite fallback; `/1` reconciliation is never reused for `/2`.
  The bounded producer request and refusal rules are documented in the documentation-corpus spec.

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
  whole Verified carry-forward rule, and whose covered paths are disjoint from the changed paths.
  Between the evidence commit and the base, no covered path, no global path, no path in the test's
  static reach (its file and imports), no link target of a flow that links the test, and no path the
  impact graph reaches from such a target may have changed. The impact graph is the head revision's,
  for that diff as for the obligation closure (decision 0416).
- `AFU-V1-022`: No test may be omitted when any changed path is a global path. When narrowing is not
  proven, the result MUST be the full relevant suite, meaning every test in the discovered inventory,
  with one or more closed codes: `e2e-unmapped-change`, `e2e-inventory-incomplete`,
  `e2e-exclusion-unproven`, `e2e-global-path-changed`, `e2e-map-stale`, `e2e-inferred-link-only`,
  `e2e-bound-exceeded` or `e2e-repository-unbound` (AFU-V1-069). Any such code keeps the ETS state
  at or above `full`: the profile never reports `narrow-selection-allowed` while a code is present.
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
  to the tiers its coverage must complete) and the optional `coverage` path. Every inventory
  `project` must be an explicitly present string. The empty string is Playwright's unnamed/default
  project identity and is valid when the complete canonical discovery carries that same empty
  identity. Missing/null inventory or discovery project fields are refused; project names are
  never filled in or normalized. Revision, config-byte and source-digest binding and exact
  project/test reconciliation remain mandatory; missing or inconsistent discovery retains the
  existing full-suite fallback. The `strict` and `coverage` outputs are unchanged. `--repository` binds
  only an application repository the provider declares, with `--repository-base` (AFU-V1-067..072).
  Inventory and each path list are bounded under the AFU-V1-065 ceilings (16,384 tests,
  65,536 paths per list including each coverage tier, 131,072 coverage paths in total, 131,072
  changed paths across the coverage evidence commits read); exceeding a ceiling is
  `e2e-bound-exceeded`.
- Coverage: the named file is a closed `application-flow-coverage/0` record list (`test_key`,
  `commit`, `tiers` of `tier`, `complete`, `paths`), an ingested runner artifact outside the provider
  so that adding a record does not itself change a global path.
- Global paths: the ETS built-in names, `Dockerfile*`, `.env.*`, the provider, the runner config,
  everything under `flows`, and every `global_paths` prefix.
- Result: `advice.test_selection` is the `e2e-safe-selection/1` object (AFU-V1-066) with `profile`,
  `state`, `state_reason`, `base`, `mandatory`, `discovery` (`path`, `state`), `selected` (`reasons`
  from `test-file-changed`, `static-reach`, `linked-to-closure`, `observed-coverage`), `omitted_tests`
  (`basis` and `proof`, the digest of one entry in `proofs`), `proofs` (each distinct proof once:
  `digest`, then `links` or `coverage` and, for `reviewed-links`, the attestation), `disjoint_from`
  (the changed paths every proof is disjoint from), `omissions_per_basis`, `fallback` (`code`,
  `subject`), `note` (the ETS note plus the per-basis note) and `untrusted_text_fields`. The ETS `omitted` member is not reused, because it
  counts list cuts. An unreadable or malformed provider or coverage file is `blocked`
  (`provider-unavailable`), and its optional `refusal` names the refused document and class
  (AFU-V1-127).
- The coverage basis is tried first. A changed path that neither the impact graph owns, nor a
  reviewed link targets, nor any coverage record names is `e2e-unmapped-change` for that basis. The
  TypeScript `e2e-runtime-dependency` and `executable-config-unresolved` frontiers do not unbound the
  closure under this profile, as under the Playwright profile: reviewed links and coverage prove the
  runtime dependency, and the runner config is a global path reconciled through discovery.

### Navigation map

- `AFU-V1-025`: (amendment accepted, decision 0514) `corvint flows navigate` MUST derive an `application-navigation-map/0` from flow
  intents and run evidence. It contains states (a route template or API operation, with a stable state
  ID), transitions (action, locator, preconditions, a readiness condition to wait for, expected
  observations, effect class, and any declared recovery transition), and each transition's verified
  state. Authentication and session setup are precondition flows that name credentials only by
  fixture ID. Map and packet transitions MUST separately carry the supporting links' aggregate
  `review_state`, omitted when there are no supporting links. Supporting links originate at the
  step, its navigation `expect` outcomes, or any variation listing the step and that variation's
  outcomes. `stale` takes precedence over `unreviewed` (any non-reviewed, non-inferred review state),
  then `inferred` (inferred links only), then `reviewed` (all non-inferred links reviewed).
  Inferred links do not dilute reviewed links. Test-evidence `verification` remains independent;
  neither a verified step nor an absent `review_state` asserts reviewed linkage or flow completeness.
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

- `AFU-V1-057`: (accepted, decision 0512) Navigation traffic
  input MUST accept at most 256 filenames, counting repeated filenames and empty files.
  More filenames MUST fail before opening any traffic file with `navigation-bound-exceeded`.
  The shared 8,192-record bound remains unchanged; refusals emit no navigation output.
- `AFU-V1-058`: (accepted, decision 0512) Navigation MUST index traffic
  once by the exact flow/step pair, preserving record order within each pair. Traffic attribution
  and filtering MUST take O(R + S) work and O(R) auxiliary space for R records and S steps,
  rather than scanning or cloning all R records for every step. Existing admitted map and packet
  output MUST remain byte-identical, including effects, grants and verification state.

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
  `step_id`, `method`, `form_submit`) read with the flow-input discipline; the proposed
  AFU-V1-057 limit is 256 filenames (duplicates and empty files count), refused before any
  traffic file is opened with `navigation-bound-exceeded`; with no locator or text
  field, a record can only raise a class. A `method` other than `GET`, or `form_submit` true, raises a
  declared `read` to `write-irreversible` (`effect_basis` `observed-traffic`); an absent `effect` is
  `write-irreversible` (`undeclared`); other declared classes stand (`declared`).
- Map: `application-navigation-map/0` has `revision`, `states` (`state_id` `ui:<route>` or
  `api:<METHOD> <path>`, `kind` `route` or `api-operation`, `template`) and per flow `status` (as
  `flows gaps`), `precondition_flows` and `transitions`, one per intent step. A transition carries
  `action`, `state`, `locator`, the flow's `preconditions`, `ready`, `input_fixture`, `expect` (the
  outcome records), `effect_class`, `effect_basis`, `recovery` and `verification`. A step is
  `verified` only when a variation that lists it has every required evidence pair verified; a step
  with no navigation entry has no state or locator. Step verification is evidence-only; the additive,
  optional `review_state` (`stale`, `unreviewed`, `inferred` or `reviewed`) separately reports supporting
  link review under the AFU-V1-025 amendment (V1-0281; accepted, decision 0514), including in packet recovery steps.
  Existing schema identities and `verification` values are unchanged. A precondition flow that is not declared or that
  closes a cycle refuses the map.
- Packet: `--goal` writes `application-navigation-packet/0` with `goal`, `max_effect`, the referenced
  `states`, `flows` (each with its `status`) and `steps`: the precondition flows depth first, each
  once, then the goal, in intent step order. Steps some other step names as its `recovery` move to
  `recovery`. Each step carries `grant` `granted`, or `requires-grant` with `grant_needed` when its
  class is above `max_effect`. More than 256 steps, or more than the 8,192 traffic records, is
  `navigation-bound-exceeded`. `--max-effect` without `--goal` is refused.
  Consumers may decode older packets without `review_state`; absence does not attest review.
  Execution's existing exact canonical-packet comparison still applies: saved packets from before
  this additive field must be regenerated when their supporting links now produce review state.
- Observer gate: `origins.json` in the flows directory is a closed `application-flow-origins/0`
  document with `disposable`, unique canonical `http` or `https` origins (`scheme://host[:port]`,
  lower case, no user, path, query or fragment), read at the evaluated commit, never the working
  tree; an absent file lists none. A `read` or `write-reversible` transition is admitted anywhere;
  any other class, unknown included, only against a listed origin, else
  `observer-origin-not-disposable`.

The separate accepted (decision 0455) [navigation execution profile](application-flow-navigation-execution-v0.md)
connects these unchanged packets to the explicit experimental companion. Its closed committed
execution input supplies operations and observable outcome mappings; prose is never executable.
The initial consumer accepts evidence/registry/traffic-free UI packets only, preserves verification
labels and grants no per-test `LOCALLY_OBSERVED` authority. Qualification is recorded separately.

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
  prose). Unanchored text is `UNPROVEN`, never `PROVEN`. An anchor comment written inside a fenced
  code block (a `` ``` `` or `~~~` fence) is example text, not a live claim, and is ignored.

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

- `AFU-V1-059`: (accepted, decision 0512) Impact graph construction MUST
  accept the caller context and propagate it through every supported language adapter. Repository
  walks MUST check cancellation at every directory-entry callback, including opted-in skipped
  directories; source reads and cached-file filtering MUST check before each operation. Cancellation
  MUST return the context error with no partial graph. Walk caches MUST be scoped to one build so
  concurrent callers cannot reuse another caller's cancelled walk. A filesystem syscall or one
  bounded source parse already in progress may finish before the next check.

- `AFU-V1-034`: `corvint-mcp --tool-profile flows` MUST advertise read-only `corvint.flows.map`,
  `corvint.flows.gaps`, `corvint.flows.impact` and `corvint.flows.navigate`, following the
  closed-selector rule of `MCPV0-026`, which this requirement amends to admit the value `flows`. Without the selector,
  `corvint-mcp` output is unchanged. Amendment (V1-0349): a committed flows read never fetches. Its
  Git runs under Core's sanitized environment (`GIT_NO_LAZY_FETCH=1` and an empty
  `GIT_ALLOW_PROTOCOL`), and an intent blob missing from a partial clone is refused as unavailable
  (`flows-refused` over MCP), never as a byte-limit breach.
- `AFU-V1-035`: Repository-authored text in any flows response is returned inside the untrusted-data
  envelope, and is never an instruction to the calling agent.

### Security and bounds

- `AFU-V1-036`: Every write MUST resolve under the repository root without following a symlink.
  `record` and `import` create their files with `O_EXCL`; `docs` rendering replaces a page through a
  confined temporary file in the same directory and a rename. Every input MUST be rejected unless `Lstat` shows a regular file
  before it is opened. A write path whose first component is `.git`, case-folded, is refused with
  the named code `git-path-refused`.
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
- matrix or topology dimensions in run-registry aggregation: every contribution shares one identity,
  and the DCP-V1-026 topology binding stays with the `internal/doccorpus` receipt path;
- an MCP tool for `flows stability`;
- any hosted or networked service.

## Trust boundary, limits, and failure modes

Source code, UI text, imported documents and runner reports are untrusted data. Authority comes
only from repository-owned flow intents with valid review anchors, and from evidence that holds at the
evaluated revision. Review is self-attested: an anchor proves a committed change, not who approved it. The failure modes this spec prevents:

- A title rename silently re-matches another test: title keys are `inferred`, and a changed key
  makes the link `stale`.
- A retry-passed test reads as passed: every attempt is kept, and the test is `flaky`.
- Repeated runs hide an unstable history: a manual rerun never fills a planned repetition, earlier
  attempts stay in the stability report, and a missing, repeated, mismatched or contradictory
  contribution refuses the aggregate instead of shrinking its denominator.
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
| AFU-V1-006 | `TestAFUV1BehaviorProviderV1BytesUnchanged`, `TestAFUV1BehaviorProviderV2MultiRepository`, `TestBehaviorV2ProducerCorpusRoundTrip`, `TestBehaviorV2RepositoryGaps`, `TestBehaviorV2RefusesMalformedOrConflictingRecords`, `TestBehaviorProviderV2CLI`; bounded producer and corpus ingestion delivered; external discovery/runtime qualification remains unobserved |
| AFU-V1-007 | `TestAFUV1ReviewAnchorValidAndStale` |
| AFU-V1-008 | `TestAFUV1ReviewAnchorValidAndStale`, `TestAFUV1ReviewAnchorNotAncestor`, `TestAFUV1ReviewAnchorMustChangeIntent`, `TestAFUV1InferredExcludedFromReviewed`, `TestAFUV1ReviewLinkMustExistAtAnchor`, `TestAFUV1ReviewEvidenceTargetUnavailable`, `TestAFUV1ReviewContentIdentityRestoredTarget`, `TestAFUV1FlowsQueryGoldens` (the `review` denominator and limitation in `map`) |
| AFU-V1-009 | `TestAFUV1InferredExcludedFromReviewed`, `TestAFUV1FlowsQueryGoldens` (inferred links outside the denominator, `inferred-only`, no evidence pair from an inferred test) |
| AFU-V1-010 | `TestAFUV1ReverseLookupsDerived`, `TestAFUV1ExportLeavesRepositoryByteIdentical`, `TestAFUV1FlowsCLIExportIsReadOnly`, `TestAFUV1FlowsCLIReverseLookups` (a non-canonical `--path` refused), `TestAFUV1FlowsQueriesAreReadOnly` |
| AFU-V1-011 | `TestAFUV1RunEvidenceClosedSchema`, `TestAFUV1NegativeControlFailed`, `TestAFUV1AdapterObservesFailedControl` (each of the three adapters observes a control failing, and Playwright and JUnit one timing out, in the same report and verifies the subject only against that expectation), `TestAFUV1FlowsCLIPassingControlNeverVerifies` (a control that passed never verifies) |
| AFU-V1-012 | `TestAFUV1PlaywrightAdapterKeepsEveryAttempt`, `TestAFUV1AdapterObservesFailedControl`, `TestAFUV1JUnitAdapterKeepsEveryAttempt`, `TestAFUV1JUnitTimedOutAttempt` (the JUnit timed-out mapping), `TestAFUV1GoTestAdapterKeepsEveryAttempt`, `TestAFUV1PlaywrightProviderKeepsEveryAttempt` (unprofiled compatibility), `TestAFUV1012QualifiedReceiptIngest`, `TestAFUV1012QualifiedReceiptRefusals`; qualified `/3` carries every attempt, while `/0`, `/1`, `/2` keep refusing the field; live matrix retained under `evidence/issues-167-175/` |
| AFU-V1-013 | `TestAFUV1FailedAttemptThenPassIsFlaky`, `TestAFUV1PlaywrightUnexpectedNeverPassed`, `TestAFUV1JUnitTimedOutAttempt` (a timed-out then passed JUnit test is flaky), the retry-passed case of each adapter test; repeated runs aggregate through the AFU-V1-041 run registry (`TestAFUV1RunStabilityCounts`) |
| AFU-V1-014 | `TestAFUV1StaticNeverVerified`, `TestAFUV1PlaywrightUnexpectedNeverPassed`; ingest emits `INGESTED` and the schema accepts `LOCALLY_OBSERVED`; the flow-scoped AFU-V0-010 observer emits no per-test record, an accepted gap (decision 0417) |
| AFU-V1-041, AFU-V1-042 | `TestAFUV1RunStabilityCounts` (counts, verdict, manual reruns kept out of the threshold, every attempt reported), `TestAFUV1RunStabilityRefusals` (each named refusal), `TestAFUV1RunRegistryReadAtHead` (the committed bytes, never the working tree), `TestAFUV1FlowsCLIStabilityIsReadOnly` |
| AFU-V1-015 | `TestAFUV1FlowsQueryGoldens` (`map.golden.json`), `TestAFUV1EvidenceStateOrder`, `TestAFUV1VerificationRequiresStability`, `TestAFUV1PassingRunNeedsStability`, `TestAFUV1MapAndGapsRegistryQualification` (including mixed pass and fail across current records, and a passed record without cleanup `done`), `TestAFUV1ReadRunEvidenceDiscipline` |
| AFU-V1-016 | `TestAFUV1FlowsQueryGoldens` (`gaps.golden.json`, every gap code reached), `TestAFUV1EvidenceStateOrder`, `TestAFUV1VerificationRequiresStability`, `TestAFUV1PassingRunNeedsStability`, `TestAFUV1MapAndGapsRegistryQualification`, `TestAFUV1ReadRunEvidenceDiscipline`, `TestAFUV1ZeroVariationFlowIncomplete` |
| AFU-V1-017 | `TestAFUV1FlowsQueryGoldens` (`impact.golden.json`: a direct hit and a hit through the impact graph), `TestAFUV1FlowsCLIReverseLookups` (unresolvable base), `TestAFUV1FlowsImpactDirtyWorktreeUnknown` (an uncommitted edit makes the scope `UNKNOWN`, never a confident no-hit) |
| AFU-V1-018 | `TestAFUV1FlowsQueriesAreReadOnly` (`map`, lookup, `gaps`, `impact`, `ingest` and `export` leave the repository, `.git` included, byte-identical) |
| AFU-V1-019 | `TestPlaywrightDiscoveryUnnamedProject`, `TestAFUV1SelectionInventoryDefaultProject`, `TestAFUV1019UnnamedProjectSelection`, `TestAFUV1019UndiscoveredTestForbidsNarrowing`, `TestAFUV1040SelectionCorpusReport` (the `undiscovered-test`, `missing-discovery`, `stale-discovery` and `test-file` cases) |
| AFU-V1-020 | `TestAFUV1020ExclusionProofsPerBasis` (a reviewed-links proof, a declared link without a review anchor, an inferred-only link), `TestAFUV1040SelectionCorpusReport` |
| AFU-V1-021 | `TestAFUV1020ExclusionProofsPerBasis` (a coverage proof, stale coverage evidence, an incomplete tier), `TestAFUV1021CoverageStaleWhenStaticReachChanged` (the `spec-after-coverage` corpus case: a test file changed after its coverage run, outside its covered paths), `TestAFUV1040SelectionCorpusReport` |
| AFU-V1-022 | `TestAFUV1022EveryFallbackCodeYieldsFullSuite` (one subtest per closed code), `TestAFUV1040SelectionCorpusReport` (global-path cases), `TestAFUV1069UnreachedApplicationPathFailsClosed` (`e2e-repository-unbound`) |
| AFU-V1-023 | `TestAFUV1020ExclusionProofsPerBasis`, `TestAFUV1040SelectionCorpusReport` (per-basis counts, both notes) |
| AFU-V1-024 | `TestAFUV1024StrictAndCoverageBytesUnchanged` (goldens captured before S4), `TestAFUV1024E2ESafeRefusesMalformedInput`, `TestAffectedSelectionArguments` |
| AFU-V1-067 | `TestAFUV1067RepositoryArguments`, `TestAFUV1069QualifiedGlobalPaths` (provider `repositories` validation) |
| AFU-V1-068 | `TestAFUV1068SeparateTestRepositorySelects` (committed and uncommitted A changes, the `repositories` row), `TestAFUV1068AncestryRewrittenRefused` (grafted and shallow checkouts), `TestAFUV1069UnreachedApplicationPathFailsClosed` (grafted checkout, A path a qualified path cannot name) |
| AFU-V1-069 | `TestAFUV1069UnreachedApplicationPathFailsClosed`, `TestAFUV1069QualifiedGlobalPaths` |
| AFU-V1-070 | `TestAFUV1070CoverageRepositoryCommitRequired`, `TestAFUV1068SeparateTestRepositorySelects` (coverage basis), `TestAFUV1069QualifiedGlobalPaths` (coverage `repositories` validation), `TestAFUV1070CoverageTierPathsValidated` (tier path validation) |
| AFU-V1-071 | `TestAFUV1071QualifiedLinkTargets`, `TestAFUV1068SeparateTestRepositorySelects` (reviewed-links basis) |
| AFU-V1-072 | `TestAFUV1024StrictAndCoverageBytesUnchanged`, `TestAFUV1040SelectionCorpusReport` (the corpus report bytes are unchanged) |
| AFU-V1-057 | `TestAFUV1057TrafficFileBound`, `TestAFUV1057TrafficRecordBound`, `TestAFUV1057TrafficFileBoundCLI` |
| AFU-V1-058 | `TestAFUV1058TrafficIndexParity`, existing navigation map and packet goldens; `internal/appflows/navigate.go` builds one exact-pair index |
| AFU-V1-025 | `TestAFUV1025NavigationMapGolden` (`navigate.golden.json`: states, readiness, expected outcomes, recovery, verified and unverified steps, credentials by fixture ID, read-only), `TestAFUV1025PreconditionFlowsResolve`; V1-0281 amendment (accepted, decision 0514): `TestAFUV1025StepReviewState`, `TestAFUV1025PacketReviewState`, `TestAFUV1039AcceptanceFixture` |
| AFU-V1-026 | `TestAFUV1026LocatorsOnlyFromIntent` (no locator for an unmapped step; traffic with page text refused), `TestAFUV1026NavigationLocatorShapes` |
| AFU-V1-027 | `TestAFUV1027EffectClasses` (raised by observed `POST`, never lowered, undeclared), `TestAFUV1027EffectRaisedNeverLowered` |
| AFU-V1-028 | `TestAFUV1028PacketRequiresGrant` (`navigate-checkout.golden.json`), `TestAFUV1028ScriptedAgentCompletesGoals` (a scripted agent completes each mapped fixture goal from the packet alone, including a recovery, performs no write under the default `read`, and cannot run an unmapped goal), `TestAFUV1028PacketBound` |
| AFU-V1-029 | `TestAFUV1029ObserverRefusesNonDisposableOrigin` (listed, unlisted, non-canonical and unknown-class cases; absent, uncommitted and malformed `origins.json`); observer integration (V1-0252, decision 0455): `TestNavigationExecutionAdmission` and `TestNavigationReceiptCannotInventSuccess` (`navigation_execution.go` applies `AdmitTransition` to every step before startup and rechecks every allowed transition in the receipt; `tools/web-flows/navigation-policy.mjs` admits each request before forwarding), and the real-browser case `AFU-V1-029 observer execution at committed disposable origin` in `tools/web-flows/test/navigation-e2e.mjs` (absent, uncommitted and unlisted origins refused with zero writes), locally qualified by `script/web-flows-gate` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-08-navigation-execution-integration.md`); the AFU-V0 manifest observer executes no navigation transition and carries no effect class |
| AFU-V1-030 | `TestAFUV1030DocsRenderGoldenAndByteStable` (`docs.golden.md`, `docs.claims.golden.json`, regeneration at a later commit byte-identical, argument refusals) |
| AFU-V1-031 | `TestAFUV1031ClaimStateOrder`, `TestAFUV1030DocsRenderGoldenAndByteStable` (every state marked on the page, `PROVEN` unmarked) |
| AFU-V1-032 | `TestAFUV1032DocsCheckDrift` (a lost `PROVEN` and a hand-edited page fail; the check is read-only), `TestAFUV1032WaiverExpiry` (unexpired, expired and malformed waivers), `TestAFUV1032WaiverExpiryBoundary` (expired on its expiry date), `TestAFUV1032WaiverKeepsUnrenderedClaim` (a waived claim that is no longer rendered keeps its committed form) |
| AFU-V1-033 | `TestAFUV1033AnchoredMarkdown` (a changed unanchored document passes, an unknown anchor fails the check and refuses the render), `TestAFUV1033AnchorParsing`, `TestAFUV1032DocsCheckDrift` (the coverage row and the anchored claim's lost `PROVEN`), `TestAFUV1033NonRegularMarkdownSkipped` (a symlinked `.md` is unanchored, not a failure) |
| AFU-V1-034 | `TestAFUV1034FlowsToolProfile` (golden default and task-review `tools/list` bytes without the selector, the four tools read-only under it, unknown, empty, duplicate and mixed selectors refused, flows tools `-32602` under the other profiles), `TestAFUV1034FlowsToolsMatchCLIVerbs` (each tool's receipt equals its CLI verb's output on the shop and navigation fixtures, the CLI's argument refusals are `-32602`, a verb refusal is `flows-refused`), `TestAFUV1034CommittedFlowsReadNeverFetchesAPromisorObject` (V1-0349: a missing intent blob in a blob:none clone is refused with no fetch, also through a Git that drops `GIT_NO_LAZY_FETCH`), `TestSanitizedGitEnvironmentRefusesAPromisorFetch` (the shared environment's empty `GIT_ALLOW_PROTOCOL` alone refuses the fetch); `TestAFUV1034FlowsCompiledConformance`, `TestAFUV1034FlowsOfficialSchema` run the compiled process on both protocols and the pinned official schema; `TestAFUV1034FlowsCoverageCompiledConformance` (V1-0280) calls the fifth tool, `corvint.flows.coverage` (AFU-V1-049), through the compiled process on the flowcoverage fixture and, with `CORVINT_MCP_OFFICIAL_SCHEMA` set, checks it against the pinned schema; `TestAFUV1034FlowsToolProfile` pins byte goldens of every profile's `tools/list` (V1-2021) |
| AFU-V1-059 | `TestAFUV1059BuildCancellation`, `TestAFUV1059SourceWalkCancellation`, `TestAFUV1059AdaptersCancel`, `TestAFUV1059FlowsImpactCancelledMidWalk` |
| AFU-V1-035 | `TestAFUV1035FlowsTextStaysInsideEnvelope` (a hostile navigation step string returns only between the envelope prefix and suffix, with no `structuredContent`) |
| AFU-V1-036 | `TestAFUV1036DocsReplaceConfined`, `TestAFUV1036DocsGitPathRefused`, `TestAFUV1InputRegularBeforeOpen`, `TestAFUV1InputSwapAfterLstatRefused`, `TestAFUV1ManifestRegularBeforeOpen`, `TestAFUV1ImportRefusesCaseVariantName`, `TestAFUV1RecordConfinedToRoot`, `TestAFUV1RecordGitPathRefused`, `TestAFUV1ImportNeverOverwrites`, `TestAFUV1IntentClosedSchema` (symlinked `--flows`) |
| AFU-V1-037 | `TestAFUV1IntentBoundsRefused`, `TestAFUV1IntentCountBoundedBeforeRead`, `TestAFUV1IntentCountBoundedWithoutRetired`, `TestAFUV1ImportCombinedFlowBound`, `TestAFUV1ImportScreensAndBoundsSource`, `TestAFUV1RunEvidenceBoundsIncomplete`, `TestAFUV1FlowsCLIIngest`, `TestAFUV1ReadRunEvidenceDiscipline` |
| AFU-V1-038 | `TestAFUV1IntentSecretScreened`, `TestAFUV1ImportScreensAndBoundsSource`, `TestAFUV1RunEvidenceSecretsDropped`, `TestAFUV1PlaywrightProviderScrubsEveryAttempt` (the provider receipt scrubs every attempt and the last-attempt fields); the screen runs in `EncodeRunEvidence`, the only run-evidence encoding, and `flows ingest` writes only what it encodes (`TestAFUV1FlowsCLIIngest`) |
| AFU-V1-039 | `TestAFUV1039AcceptanceFixture` over the committed fixture `cmd/corvint/testdata/flows/acceptance` (a synthetic, hand-written report; separate real browser qualifications are retained below): the UI flow `checkout` and the API flow `orders-api` are complete with every variation verified in `map`; `returns` reports exactly `unmapped-flow` and `profile` exactly `stale-link` (its evidence verified) in `gaps`; both have flow status `incomplete` (step verification is evidence-only) in `map`, `gaps`, the `navigate` map and each one's `navigate --goal` packet, and `docs` renders them `UNPROVEN` and `STALE` while the two verified flows' claims are `PROVEN` |
| AFU-V1-040 | `TestAFUV1040SelectionCorpusReport`: the frozen corpus `cmd/corvint/testdata/e2e-safe-corpus.json` (21 labelled, fault-injected cases over five tests, `spec-after-coverage` added by decision 0416) and its report `cmd/corvint/testdata/e2e-safe-corpus.report.json`; `coverage` omits 15 with 0 unsafe (reduction 15/105), `reviewed-links` omits 3 with 0 unsafe (reduction 3/105), no basis withdrawn |
| AFU-V1-054 | proposed (V1-0985): `TestAFUV1BehaviorProviderV1BytesUnchanged` (/1 bytes unchanged), `TestBehaviorProviderV1LegacyMemberDecodes` (/1 decode keeps the legacy /1 member; no alias) |
| AFU-V1-055 | proposed (V1-0985): `TestBehaviorProviderV2EmitsNeutralMembersOnly`, `TestAFUV1BehaviorProviderV2MultiRepository` (/2 record and corpus report emit only neutral members) |
| AFU-V1-056 | proposed (V1-0985): `TestBehaviorProviderV2RefusesLegacyMember` (/2 request refuses the legacy /1 member and an `e2e` alias) |

Live qualification: the companion surfaces are qualified on Beamfall with one UI flow and one API
flow. The Core profile is qualified by the corpus report (AFU-V1-040) plus one real change against
the Corvint Playwright fixture suite (`conformance/interactive-alpha/fixture`) and one on Beamfall.
All three were observed locally on 2026-09-28 and retained in `evidence/issues-167-175/`:

- Beamfall companion: UI checkout and API phase read passed; two negative controls failed; reviewed/stale/unmapped links, retained run evidence, CLI/MCP equality and documentation labels verified. Application source `7caa4d1950a51a009928ea2d474e33a9ba074828`; tested fixture commit `363a2fd3423ee755f7eba72541f85c1bc6861fbb`. The backend is the in-process mock; production services are NOT_OBSERVED.
- Corvint Playwright fixture: the selected counter test passed before the +1→+2 source fault and failed afterward.
- Beamfall real change: the selected API test passed before the phase-ID source fault and failed afterward. Both live selections fall back to the full relevant suite and omit zero tests. The frozen corpus remains the evidence for exclusion-basis narrowing; these live runs claim no reduction.

The companion browser uses the pinned Playwright 1.63.0 bundled Chromium tuple. Cleanup and SIGTERM interruption were checked. Host Chrome drift is explicitly refused. AFU-V1-006 external runtime qualification, AFU-V1-029 observer integration and the accepted AFU-V1-014 gap remain outside this qualification.

V1-2690 (2026-10-25) moves the companion `tools/web-flows` runner to the declared Playwright 1.64.0
tuple (Node v24.11.1, bundled headless shell 1248). Observe and navigation evidence readers admit
exactly 1.64.0 and the retained 1.63.0; the 2026-09-28 qualification above stays a 1.63.0 record
and is not re-run by this change.

## Rollout, rollback, and compatibility

Slices, each its own change with tests:

1. S1 is the intent model, export and import, review anchors, and the two input and output hardening
   fixes (AFU-V1-001..010, 036).
2. S2 is run evidence and the three adapters (AFU-V1-011..014, 037, 038). It delivered the record,
   the adapters, the bounds and the hygiene, and the unprofiled provider receipt now carries and
   scrubs every attempt; the qualified `/3` external profile closes AFU-V1-012, decision 0417 accepts
   the AFU-V1-014 observer gap and adds the JUnit timed-out mapping, and the AFU-V1-041 run registry
   and AFU-V1-042 `flows stability` close the AFU-V1-013 aggregation.
3. S3 is `map`, `gaps` and `impact`, and the `/2` behavior provider (AFU-V1-006, 015..018).
4. S4 is the Core `e2e-safe` profile and its corpus (AFU-V1-019..024, 040).
5. S5 is the navigation map (AFU-V1-025..029).
6. S6 is proven documentation (AFU-V1-030..033).
7. S7 is the MCP tool profile (AFU-V1-034, 035). Delivered: `--tool-profile flows`, including compiled-process and official-schema conformance.
8. S8 is the acceptance fixture and live qualification (AFU-V1-039).

Rollback removes the `e2e-safe` value, the new `flows` subcommands and the MCP profile. Flow intents,
waivers, claim sidecars and run registries are repository data and need no migration. `/1` provider readers are
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
| AFU-V1-025..029 | implemented (029 observer integration through the experimental NEX-V0 companion; see the matrix): `internal/appflows/navigate.go`, `origins.go`, `navigation_execution.go`, the `navigation` member in `intent.go`, `cmd/corvint/flows_navigate.go`, `tools/web-flows/navigation.mjs` |
| AFU-V1-030..033 | implemented: `internal/appflows/docs.go` (`RenderDocs`, `CheckDocs`, `ReplaceConfined`), `cmd/corvint/flows_docs.go` |
| AFU-V1-036 | implemented (tests in the matrix): `internal/appflows/input.go` (`ReadFile` and `readSource` read only after `Lstat` shows a regular file; `WriteConfined` creates with `O_EXCL` under `os.OpenRoot` after `confinedName` and `realParents`; `gitPath` and the `git-path-refused` code), used by `record` (`report.go`) and `import` (`import.go`); `internal/appflows/docs.go` (`ReplaceConfined`, a confined temporary file and rename); `internal/appflows/intent.go` (`FlowsDir`, a real `--flows` directory) |
| AFU-V1-039 | implemented and locally qualified (limits above): `cmd/corvint/testdata/flows/acceptance`, `cmd/corvint/flows_acceptance_test.go` |
| AFU-V1-006 | partial (see the matrix): `internal/doccorpus/behavior.go` |
| AFU-V1-054..056 | proposed (V1-0985): `internal/doccorpus/behavior.go` (`BehaviorRevisions`), `behavior_v2.go` |
| AFU-V1-041, AFU-V1-042 | implemented: `internal/appflows/runregistry.go`, `internal/appflows/query_stability.go`, `cmd/corvint/flows.go` (`flows stability` and query qualification) |
| AFU-V1-011..014, 038 | implemented (014 accepted observer gap; see the matrix): `internal/appflows/runevidence.go`, `runingest.go`, `internal/runhygiene/runhygiene.go`, `internal/jstestprovider/playwright.go`, `receipt.go`, `projection.go`, `external.go` |
| AFU-V1-037 | partial (tests in the matrix): run evidence is incomplete under `run-evidence-byte-bound`, `run-evidence-record-bound`, `run-evidence-attempt-bound`, `run-evidence-link-bound` or `run-evidence-traversal-bound` (`internal/appflows/runevidence.go`), enforced in `runingest.go`. The byte bound applies before any decode. The traversal bound applies before decode for Playwright JSON and `go test -json` and during the JUnit token walk. The record, attempt and link bounds apply during the JUnit and `go test -json` walks, but for Playwright JSON and the `/3` receipt only after the byte-bounded document is decoded. The `/3` receipt has no traversal bound of its own. Those two orderings do not yet meet "before decode and walk"; `flows ingest` writes no record when one is exceeded (`cmd/corvint/flows.go`); intent, import, docs and read-side bounds in `internal/appflows/intent.go`, `import.go`, `docs.go`, `query.go` |
| AFU-V1-065..066 | implemented (V1-1676): `internal/appflows/selection.go` (`MaxSelectionTests`, `MaxSelectionPaths`, `MaxSelectionCoveragePaths`, `MaxSelectionEvidencePaths`, `boundCodes`, `global`, `proofDigest`, `evidenceDiff`), `internal/appflows/review.go` (`anchorFacts`), `internal/appflows/input.go` (`decodeElement`, V1-2164) |
| AFU-V1-067..072 | implemented (V1-1675): `internal/appflows/repositories.go` (`bindRepositories`, `bindRepository`, `ancestryRewritten`, `repositoryChanges`, `splitQualified`), `internal/appflows/selection.go` (`repositoryStale`, `evidenceDiffAt`, `globalCodes`, `qualified`, `validCoverage`), `internal/appflows/review.go` (`EvaluateLinksAcross`, `repositoryTargetState`), `internal/appflows/intent.go` (`reviewed_at_repository`), `cmd/corvint/affected.go` (`--repository-base`), `internal/extevidence/repository.go` (`ValidRepositoryID`) |
| AFU-V1-019..024, 040 | implemented: `internal/appflows/selection.go` (`SelectE2E`), `cmd/corvint/affected.go`, `internal/liveverify/affected/typescript/playwright_discovery.go` (`VerifyPlaywrightDiscovery`), `internal/extevidence/selection.go` (`SelectionNote`); corpus `cmd/corvint/testdata/e2e-safe-corpus.json` |
| AFU-V1-059 | accepted, decision 0512: `internal/liveverify/affected/graph.go`, `source.go`, `walk.go`, the eight language adapters, `internal/appflows/impact.go` and its CLI/MCP callers |
| AFU-V1-034..035 | implemented and locally qualified: `internal/mcp/bridge/flows.go`, `bridge.go`, `internal/mcp/cli/main.go`, `internal/appflows/impact.go` (`FlowImpactAt`) |
| AFU-V1-125 | implemented (V1-2414): `internal/flowdocs/generate.go` (`FlowLimitCode`, the whole-scope `candidates` count), `cmd/corvint/docs_flows.go` (wire-code lift); `TestAFUV1125FlowLimitIsTyped`, `TestAFUV1125FlowLimitCLICode` |
| AFU-V1-126..127 | experimental under the standing lane auto-accept (V1-2788, GitHub #1888): `internal/appflows/input.go` (`decode`, `maskFlowTierKeys`, the distinct Decode refusals), `internal/appflows/selection.go` (`Refusal`, `selectionInputError`, `selectionRefusalSubject`); `TestAFUV1126FlowTierIdentifierKeyIsNotSecret`, `TestAFUV1126E2ESafeFlowTierPasswordKey`, `TestAFUV1127ProviderUnavailableNamesRefusal`, `TestAFUV1024E2ESafeRefusesMalformedInput` |
| AFU-V1-128..130 | experimental under the standing lane auto-accept (V1-2871, GitHub #1915): `internal/appflows/import.go` (`importPlaywrightList`, `playwrightListDir`, `listSpecs`, `disambiguate`, `dropCollisions`, `ImportSkipped`), `cmd/corvint/flows.go` (`runFlowsImport`, `flowsIntentHelp`); `TestAFUV1128PlaywrightListResolvesDescribeKeys`, `TestAFUV1128FlowsCLIPlaywrightListLinksResolve`, `TestAFUV1ImportOpenAPIAndPlaywright`, `TestAFUV1130FlowsHelpSaysTestLinksOnly` |
| AFU-V1-251..252 | experimental under the standing lane auto-accept (V1-2924, GitHub #1930): `internal/appflows/import.go` (`mergeProjects`, `disjointProjects`, `dropCollisions`), `internal/appflows/query.go` (`variationGaps`); `TestAFUV1251PlaywrightListMergesProjects`, `TestAFUV1252NoTestNamesInferredLinks`, `TestAFUV1FlowsQueryGoldens` |
| AFU-V1-131 | experimental under the standing lane auto-accept (V1-2873, V1-2874): `internal/appflows/runingest.go` (`parsePlaywrightRunAt`, `RunHeader.Root`, `Ingested.Unjoined`), `cmd/corvint/flows.go` (`flowSubcommand` diagnostic writer, `runFlowsIngest`); `TestAFUV1131IngestJoinsPlaywrightListImport` |
| AFU-V1-112..120 | implemented (GitHub #1587): `internal/appflows/coverage_ingest.go` (`IngestCoverage`, `parseV8`, `parseIstanbul`, `parseLCOV`, `parseRSpec`, `bind`, `joinFile`, `v8Lines`, `checkLink`), `internal/doccorpus/line_anchor.go` (`LineIndex.Bytes`), `cmd/corvint/flows_ingest_coverage.go`, `cmd/corvint/flows.go`, `internal/appflows/runevidence.go` (`validateCoverageArtifacts`) |

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

## Owned emitted attempt-ingest error codes

These refusals describe the existing `playwright-receipt` ingest boundary (AFU-V1-012 and
PWP-V3-005); they add no new receipt state or execution authority.

| Code | Meaning |
| --- | --- |
| `external-attempt-run-incomplete` | The receipt reports infrastructure failure or cancellation, lacks external lifecycle evidence, changed its inputs, or did not retire runner descendants. |
| `external-attempt-runner-identity-mismatch` | The canonical receipt cannot be decoded, has no runner version, or disagrees with the ingest header's runner version. |
| `external-attempt-test-identity-missing` | A retained test has no test ID or named project. |

### Issue 316/317 compatibility and rollback

The owner-requested corrections in issues 316 and 317 admit Playwright's explicit empty identity and
restore the defined stability requirement for verified flows. Existing map/gaps schemas retain their
names, add the closed stability states above and optional `stability_detail`, and conservatively
report incomplete evidence to callers that omit a registry. The opt-in MCP flow map/gaps/navigation
inputs accept the same optional repository-relative `registry`; map lookup selectors exclude it.
No evidence record, registry or strict/coverage selection wire shape changes. Rollback reverts this
slice; per-run evidence must remain labelled unqualified until the stability gate is restored.


## Source-derived non-Go documentation (issue 338)

Human-owned intent: generate and maintain source-anchored application flow documentation in the
fixed eight-section layout, with generated trust and independently retained runtime evidence.
This source producer is separate from the accepted-outcome `appflows/docs.go` renderer.
No source observation is accepted intent, a runtime sequence, proof of a side effect, an inferred
accepted variation, or a replacement for independent application/browser qualification.

### Requirements

- **AFU-V1-043**: `docs flows generate --revision FULL_SHA --scope PATH --output-dir FRESH_DIR`
  reads immutable native-index source, emits an adoption provider and generation manifest, and
  materializes eight Markdown files per logical flow. Literal Ruby/Rails routes, named methods/jobs,
  class-level validation/hooks, JavaScript/TypeScript named functions/arrows (`.js`, `.mjs`, `.cjs`,
  `.jsx`, `.ts`, `.tsx`; ES and CommonJS modules since V1-0461,
  `TestFlowDocsJavaScriptModuleExtensions`) and literal route or Angular registrations, React JSX conditional candidates and literal Angular HTML attributes form
  the closed construct matrix. A named arrow is a declaration only when the arrow is the direct
  initializer, optionally wrapped in `useCallback`, `useMemo`, `memo` or `forwardRef` (bare or
  `React.`-prefixed); an arrow passed to a member call (`messages.map((m) => ...)`) is not. An
  HTTP-verb registration (`get`, `post`, `put`, `patch`, `delete`) needs a `/`-rooted path literal
  followed by a handler argument; a member read such as `headers.get("Content-Length")` or a
  single-argument client call is not a binding (since V1-0461 Beamfall qualification,
  `TestFlowDocsWebMemberCallsAreNotDeclarations`). Calls to the recognized validation, permission, error, persistence
  and integration vocabulary are observations of source only. Native lexical masks exclude comments,
  strings and Ruby heredocs; web regex/arithmetic suffixes and JSX text are conservatively masked.
- **AFU-V1-044**: Logical flow and paragraph IDs depend on provider namespace, repository-relative
  path, declaration identity and claim kind/callee, never line, revision or output ordering. Repeated
  logical declarations/claim identities remain unresolved. Removal is retirement, never inferred
  rename. Every claim has an operation-span anchor and, when available, a separate declaration
  symbol anchor. Source-derived corpus subjects, claims and structural/same-file lexical dependency
  relations retain generated trust and explicit unresolved dispatch limits.
- **AFU-V1-045**: `docs flows check --revision FULL_SHA --previous GENERATION_JSON
  [--output-dir DIR]` strictly rederives the closed predecessor from original immutable source before
  reporting each flow, paragraph and claim fresh/stale/unresolved. Same identity plus same span hash
  remains fresh after relocation; changed spans are stale; absent/ambiguous identities are unresolved
  and appear in retirement delta. A flow binds its declaration and its static call inventory, which
  the flow diagram renders, so an added, removed or changed call is stale (since V1-0461,
  `TestFlowDocsCallInventoryDriftIsStale`); control keywords (`if`, `function`, `return` and similar)
  are never listed as bare calls; a member call that spells one (`promise.catch(h)`,
  `iterator.return(v)`) is a call (`TestFlowDocsMemberKeywordCallsAreDrift`). This changed the
  generator to version 2, and the AFU-V1-043 member-call declaration fix to version 3: a version 1
  or 2 manifest cannot be rederived byte for byte, so check, finalize and `generate --previous`
  refuse it with the typed `flowdocs-generator-stale` code even at its own revision; regenerate it
  once with the current binary
  (`TestFlowDocsPredecessorGeneratorManifestIsTypedStale`). Optional byte checking compares the original rendered directory,
  including missing/extra/changed files, without writing anything. Exit 1 means drift; exit 2 refusal.
- **AFU-V1-046**: Materialization requires a new directory under existing nonsymlink parents and
  exclusively creates confined files. Refuse human-file overwrite, symlinks, malformed paths and
  bounds above 256 flows (refused with the typed code of AFU-V1-125), 2,048 Markdown pages plus two metadata files, 64 MiB rendered payload and
  256 MiB generation manifest (including an existing corpus up to 128 MiB), for a 320 MiB total
  output ceiling. Corpus input retains the corpus profile's 128 MiB bound. Render escaped prose and local generated links; diagrams use fixed
  Mermaid syntax labelled static/unresolved. Never dump arbitrary source bodies or literals.
- **AFU-V1-047**: Optional `--corpus FILE` is revalidated with corpus Open and must retain the exact
  source repository/revision. Original journeys, ordered steps, navigating/asserting test links,
  join confidence, explicit coverage members, tickets and intent comparison records remain available
  under original corpus identities. Span overlap is only a locator; it never upgrades semantic join
  confidence. Missing accepted variation inventory/runtime observations remain unknown. Security
  rendering uses only admitted severity/total summaries, never restricted local detail.
- **AFU-V1-048**: `docs flows finalize --previous GENERATION_JSON --provider-revision FULL_SHA
  --provider-path PATH` verifies committed provider bytes and returns a usable corpus manifest after
  successfully compiling it. Source A and later provider B remain distinct immutable revisions.
  No automatic commit, rewritten source anchor or provider self-reference is permitted. Caller-owned
  corpus inputs/providers are retained with missing generation source inputs added explicitly.
  Finalization enumerates regular superproject files into exact file scopes; opaque gitlinks retain
  their superproject path/commit identity as unknown provider module records, never blob inputs or
  nested source coverage. Other unsupported entries still refuse. Changes to opaque identities are
  reported separately and make the read-only check nonzero. Generic corpus inventory stays strict.

### Failure modes, evidence and rollback

Extraction is bounded lexical analysis, not a Ruby/JS/HTML interpreter or full parser. Dynamic
routes, metaprogramming, computed registrations, unsupported templates, multiline declaration forms,
ambiguous duplicate calls and body extents above 512 lines remain unknown. Gitlinks are opaque.
The generated variation denominator never treats each source conditional as an accepted variation.
Source-span overlap cannot prove that a Playwright test validates a generated flow. A supplied
original runtime record may prove only its own accepted variation under the existing corpus rules.

Imported rendered records receive stable paragraph IDs and retain all original anchors. Freshness
checks revalidate every dependency anchor, conservatively marking repeated span matches unresolved;
historical runtime-only anchors cannot establish current source behavior. Added identities are drift.

Executable acceptance: `TestFlowDocsCLIEndToEnd` exercises the actual CLI, eight files, source A /
provider B, Build/Open and changed-authorization-only staleness across line insertion.
`TestFlowDocsLexicalFalsePositives`, `TestFlowDocsScaleDeterminismSafetyAndGitlink` and
`TestFlowDocsNoRawSourceSecrets` cover exclusion, 150-flow/1,200-page scale, opaque gitlinks,
strict predecessor rederivation, read-only determinism and output safety. Broader runtime/parity
qualification must retain its own original receipt and cannot be inferred from these fixtures.
Rollback removes the `docs flows` dispatch and generated producer; existing accepted-outcome AFU,
corpus adoption artifacts and their original trust/evidence contracts remain unchanged.

## Complete variation coverage (issue340)

Owner-requested bounded compiler; accepted matrices remain human-owned. The complete invocation,
closed input shapes and byte-binding limits are documented in `docs/FLOW-COVERAGE.md`.

- **AFU-V1-049**: `flows coverage --denominator FILE --receipts FILE [--revision FULL_SHA]`
  loads closed committed denominator/run inventories. Generation and corpus inventories are fully
  rederived; intents inventory hashes the complete sorted canonical FlowIntent array. Every documented
  flow and every loaded accepted variation must be represented exactly; proposed/empty/unmapped
  matrices remain blockers. Current committed or dirty inventory additions cannot yield subset green.
  Accepted actor/preconditions/entry/actions/outcomes and original source-category citations remain
  visible. No generation, test title or exclusion record can accept a proposed matrix.
- **AFU-V1-050**: Proof admits original canonical `/3` bytes and rederives exact test/config/package/
  served-build bindings through immutable regular Git content and exact reporter path mappings.
  Require configured and observed retries0, complete unambiguous schedule joins, qualified lifecycle,
  every asserting outcome, and a distinct accepted control observed failing within the same immutable
  execution binding group. Infrastructure never acts as a control. A first pass with configured
  retries above0 is MISSING_TEST. Retain every declared planned/manual receipt; conflicting/retried
  comparable current history is FLAKY. Comparability uses bound build/test/config/package bytes and execution configuration, not source/test commit IDs or caller app relabeling; metadata-only commits cannot hide failure history. Historical changed groups remain visible without blocking
  separately qualified current evidence. Aggregate expected-control failure does not invalidate a
  qualified passing subject. Missing served-build binding is MISSING_TEST.
- **AFU-V1-051**: Closed statuses are PROVEN, API_PROVEN, FLAKY, STALE, MISSING_TEST and EXCLUDED.
  Cited/required binding changes yield STALE. Exclusion needs a committed nongenerated human
  author/sign-off reference binding exact accepted matrix and source revision. It is local project
  authority, not authenticated identity. PROVEN/API_PROVEN means immutable byte binding plus declared
  app identity, never attested deployment lineage, observed clean execution tree or independently
  verified control adequacy. Retain `/3`'s unknown failed-assertion location and external server cleanup.
- **AFU-V1-052**: CLI exits1 for any missing/flaky/stale row,2 for malformed/incomplete inputs,0 only
  for a nonempty complete green inventory. Opt-in MCP flows profile adds EnvelopeOnly read tool
  `corvint.flows.coverage`; default V0 tools remain unchanged. Offset/limit paging occurs only after
  complete evaluation and carries full-report digest, total and global verdict/counts. Equal CLI/MCP
  page requests return identical compiler bytes. Bounds:512 documented flows,8,192 expanded rows,
  128 original receipts of4MiB each,64MiB aggregate receipts/report and300KiB page. No silent truncation.
- **AFU-V1-053**: Explicit `--write-back FRESH_DIR` creates confined no-clobber documentation even for
  valid reports containing gaps. Generation retains eight pages, adding E2E coverage to functional
  overview; rename the immutable source manifest to source-generation.json and bind every actual
  output hash in application-flow-coverage-writeback/1, including denominator/run/full-report hashes.
  Corpus/intents input writes one per-flow section. `--check DIR` is read-only. Preserve320MiB total
  output bound and existing generation/corpus input limits; permit only two additional coverage
  metadata files. Escape source-derived text. Missing tests carry bounded skeleton requests naming
  accepted entry/outcomes and existing validated fixtures/page objects, with undiscovered inputs explicit.

Acceptance: `TestFlowCoverageBoundProofAndWriteback`, `TestFlowCoverageInventoryFreshness`,
`TestFlowCoverageRetryAndInfrastructure`, `TestFlowCoverageDenominatorOmissions`,
`TestFlowCoverageGroupHistoryAndPaging`, `TestFlowCoverageExclusionAndEntryAuthority`,
`TestFlowCoverageCLI`, `TestFlowCoverageMCPParity`, `TestDefaultCoverageAbsent`.
The real two-variation browser qualification must retain original receipts, source/test/config/build
commits, native exit codes, actual write-back/check/no-clobber results and interruption descendant
cleanup. Unit fixtures cannot establish live qualification. Final source-bound qualification and
native completion are separate from implementation checks.

Failure modes: favorable run selection, omitted accepted matrix members, cross-build controls,
retry-budget confusion, stale documentation manifests and fabricated deployment provenance must
fail closed. Scope completeness is only for the declared inventory, not the entire application or
unretained executions. Rollback removes this additive compiler/CLI/MCP tool and regenerates docs
with the preceding source generator; it never rewrites accepted intents or previous evidence.

## Neutral member for the behavior provider's end-to-end repository (V1-0985)

The `/1` behavior provider's `revisions` object names its end-to-end test repository with a
member called the legacy /1 member here. Its exact spelling lives only in the `BehaviorRevisions`
declaration in `internal/doccorpus/behavior.go`. Renaming it would break the frozen `/1` bytes, so
`/1` keeps it. `/2` (AFU-V1-006) is the existing next version and upgrade path. It already
replaces the three fixed members with neutral ones, so no new profile or version is added.

- **AFU-V1-054**: (proposed (V1-0985)) The `/1` behavior provider and the `/1`-family wires that share
  its `revisions` shape keep the legacy /1 member. `/1` encoding stays byte-identical, and `/1`
  decoding keeps accepting the member. `/1` gains no alias: the closed decoder refuses a neutral
  spelling of the member in a `/1` record.
- **AFU-V1-055**: (proposed (V1-0985)) A `/2` provider record, and the corpus report built from it,
  carries the end-to-end repository only through neutral members. These are the provider `source`
  and one `repositories` entry with that root commit and revision. Neither emits the legacy /1
  member or any `revisions` object.
- **AFU-V1-056**: (proposed (V1-0985)) A `/2` producer request that carries the legacy /1 member in
  `registry.revisions` is refused before output. So is a request that names a neutral alias of it,
  such as `e2e`. A producer upgrading from `/1` moves that repository into `source` and
  `repositories`. Tests read the member's spelling from its declaration and never repeat it, and
  specs call it the legacy /1 member.

Acceptance: `TestAFUV1BehaviorProviderV1BytesUnchanged` and
`TestBehaviorProviderV1LegacyMemberDecodes` (AFU-V1-054),
`TestBehaviorProviderV2EmitsNeutralMembersOnly` and `TestAFUV1BehaviorProviderV2MultiRepository`
(AFU-V1-055), and `TestBehaviorProviderV2RefusesLegacyMember` (AFU-V1-056), all in
`internal/doccorpus`.

Failure modes: a rename or alias in `/1` would change its pinned digest or silently admit a second
spelling. A `/2` record that re-emits the fixed members would carry the legacy /1 member into the
new version. Both fail the tests above. Repository history and the closed `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/BUILD-LOG.md` keep
earlier spellings. Rollback removes these tests and this section. No wire bytes changed, so no
producer or reader needs to roll back.

## Large end-to-end suites (issue 1079, V1-1676)

The S4 bounds of 1,024 tests and 4,096 paths per list sent any provider with a few thousand
Playwright tests to `e2e-bound-exceeded` and the full suite. The result also repeated every
reviewed flow's link list in each omitted test's proof and the changed paths in every omission, so
its size grew with tests times links. This section raises the bounds to fixed, documented ceilings
and states each proof once.

- **AFU-V1-065**: The e2e-safe profile MUST plan, without `e2e-bound-exceeded`, a provider whose
  inventory holds at most 16,384 tests, whose changed-path and `global_paths` lists and each
  coverage tier's `paths` list each hold at most 65,536 paths, and whose coverage records name at
  most 131,072 paths in total. The changed paths between base and the distinct coverage evidence
  commits the selection reads MUST total at most 131,072 across those commits. Above any of these
  ceilings the result MUST stay the unchanged closed refusal: the full suite with
  `e2e-bound-exceeded` naming the exceeded list (`inventory`, `coverage`, `changed-paths`,
  `global-paths`, `coverage-tier-paths`, `coverage-paths` or `evidence-paths`). The evidence
  cache that would pass 131,072 changed paths is dropped at that read, so memory held for evidence
  diffs never exceeds that count. Memoized reach answers across evidence commits stop being cached
  at 131,072 and are then recomputed, which changes no answer. Global-path matching looks up each
  changed path and its slash-delimited prefixes in the `global_paths` set, so it is linear in the
  changed paths. The ceilings are fixed constants, not operator configuration. The 5 MiB document
  bound still applies first, so an oversized provider or coverage file stays `blocked`
  (`provider-unavailable`).
- **AFU-V1-066**: Profile version decision: the result schema moves from `e2e-safe-selection/0` to
  `e2e-safe-selection/1`. Each omitted test's `proof` MUST be the digest (`sha256:` and the hex
  SHA-256 of the canonical JSON of the proof with its `digest` member absent: keys sorted at every
  level, no insignificant whitespace, UTF-8 without HTML escaping and no trailing newline) of
  exactly one entry in the
  top-level `proofs` list. That list holds each distinct proof once, sorted by digest. The changed
  paths every proof is disjoint from are stated once, as the top-level `disjoint_from`. Omissions
  under one flow set share one `reviewed-links` proof. Omission order, bases, per-basis counts,
  fallback codes and every other member keep their `/0` meaning. Consumers of `/0` see a new
  schema string and must migrate. `/0` is not emitted alongside `/1`.

Measured at the ceiling (`TestAFUV1065CeilingMeasurement` with `CORVINT_E2E_SCALE_TESTS=16384`,
Apple M-series host, 2026-10-17): 16,384 inventory tests, 64 generated flows of 256 tests with
reviewed links. The result was `narrow-selection-allowed` with 16,383 omissions and 68 distinct
proofs. Elapsed time was 7.49 s for the whole command and the receipt was 5,952,721 bytes. Total
allocation was 2,077,947,160 bytes and the heap system size was 82,116,608 bytes. The CPU profile
puts about 2.9 s in `SelectE2E`. Most of the rest is the existing secret screen on decode and
intent loading. Since V1-2164, `Decode` screens the provider document once and each inventory
element is decoded strictly without a second screen, which cut allocation at the ceiling by about
37% (`BenchmarkAFUV1065InventoryDecodeAtCeiling`). Without the shared proofs, each omission would repeat its flow's links and the
changed paths, so output would grow with tests times links.

Acceptance: `TestAFUV1065LargeInventoryNarrows` (3,072 generated tests above the old 1,024 bound
narrow with shared proofs), `TestAFUV1065InventoryAboveCeilingRefused` (16,385 tests: only
`e2e-bound-exceeded` on `inventory`, full suite of 16,385, no proofs),
`TestAFUV1022EveryFallbackCodeYieldsFullSuite` (the coverage-path ceiling) and
`TestAFUV1065CeilingMeasurement` (opt-in measurement, skipped by default) for AFU-V1-065, all in
`cmd/corvint`; and in `internal/appflows`, `TestAFUV1065CoverageTierListCeiling` (65,536 paths in
one tier plan, 65,537 refuse on `coverage-tier-paths`), `TestAFUV1065EvidenceCacheBounded`
(evidence diffs cached up to exactly 131,072 changed paths; the next read drops the cache and
refuses), `TestAFUV1065ReachMemoBounded` and `TestAFUV1065GlobalPathMatchingScales` (65,536
changed against 65,536 global paths in under 2 s, with the exact prefix semantics).
For the `/1` proof wire, `TestAFUV1020ExclusionProofsPerBasis` (every stated digest is recomputed
from the emitted proof JSON with the `digest` member removed; each omission names one proof;
`disjoint_from` stated once) and `TestAFUV1040SelectionCorpusReport` in `cmd/corvint`, and
`TestAFUV1066ProofDigestOmitsDigestMember` in `internal/appflows`. `TestAFUV1065InventoryDecodeScreensOnce`
(a 16,384-test provider is secret-screened exactly once; a secret, unknown key or missing project
inside an element is still refused) covers the single decode screen.

Non-goals: no operator-configurable bound, no streaming or paged output, no change to `MaxFlows`
(512), links per flow (1,024) or variations (256), and no change to which tests are selected or
omitted.

Failure modes: a ceiling that is too high could exhaust memory or exceed the output budget. The
measurement above bounds it at about 82 MiB of heap and 6 MB of output at 16,384 tests. A digest
that does not bind its proof would let an omission point at a different proof. The test recomputes
each digest from the emitted proof JSON alone, as a consumer would. A consumer pinned to `/0` would misread `proof` as an object.
The schema string changes, so a closed decoder refuses `/1` instead of misreading it.

Rollback: revert the constants to 1,024 and 4,096, restore the inline `/0` proof and per-omission
`disjoint_from`, and remove this section and its tests. No repository data, provider or coverage
format changes, so no producer needs to roll back.

## Separate application repositories (issue 1078, V1-1675)

A browser suite often lives in its own repository T, apart from the application repository A it
drives. The question "this change to A: which tests in T must run?" had no e2e-safe answer:
`--repository` was refused, the changed paths came only from T's diff, and coverage records and
link targets could name only T-relative paths. This section lets the project declare application
repositories, binds each one to immutable Git content, and keeps every unknown fail-closed.
Definitions: a *qualified path* is `ID:PATH`, a declared repository ID, a colon and a
repository-relative canonical path. Root paths never contain a colon in a provider, coverage or
intent document, so the two forms cannot be confused there.

- **AFU-V1-067**: Authority: the provider MAY declare `repositories`, a list of objects with `id`
  (an EEP repository identifier, unique) and `origin` (the full ID of a parentless commit of that
  repository). Only a declared ID can contribute changed paths or evidence. Under
  `--selection-profile e2e-safe`, `affected` MUST accept `--repository ID=DIR` (DIR relative to
  `--root` unless absolute) only together with `--base` and exactly one `--repository-base
  ID=FULL_COMMIT` for each bound ID. A `--repository-base` without its `--repository`, a
  duplicate, a non-full commit ID, or `--repository-base` under any other profile is an argument
  error (exit 2). The `strict` and `coverage` profiles keep their `--repository` behavior.
- **AFU-V1-068**: Binding: each bound repository MUST be the top level of a Git checkout whose HEAD
  reaches the declared `origin` as a parentless commit and whose `--repository-base` resolves to a
  commit. Its changed paths are the tree diff base..HEAD plus its worktree status paths, each
  qualified with its ID, and they join the root's changed paths for every rule of AFU-V1-019..023
  and AFU-V1-065 (including the `changed-paths` ceiling and `disjoint_from`). HEAD and the status
  are read before and after the diff; a change between the reads unbinds the repository. The result
  MUST carry `repositories`, sorted by ID, each with `id`, `origin`, `state` (`bound` or
  `unbound`), `reason` (for `unbound`), `base`, `revision` (HEAD), `tree` (HEAD's tree) and
  `worktree_paths` (the number of uncommitted changed paths, which no commit pins). A checkout
  whose ancestry Git rewrites, through an `info/grafts` file in its common Git directory (which Git
  honors even with replace objects disabled) or a shallow boundary, and a checkout with a changed
  path that is not a canonical qualified path (one holding a colon, say), MUST NOT bind.
- **AFU-V1-069**: Fail closed: a declared repository that is not bound or does not bind, and a
  bound ID the provider does not declare, MUST yield the full relevant suite with the new closed
  code `e2e-repository-unbound` (subject: the ID), which joins the AFU-V1-022 set. A root changed
  path that begins with a declared `ID:` is `e2e-unmapped-change`. A qualified changed path has no
  impact-graph edge, so it is explained only by a coverage record or a reviewed link that names
  it; otherwise it is `e2e-unmapped-change` exactly as an unexplained root path is. A path that
  begins with a declared `ID:` never takes root impact-graph ownership or reach. The built-in
  global file names apply inside each repository, and `global_paths` MAY name qualified prefixes.
- **AFU-V1-070**: Coverage: a coverage record's tier paths MAY be qualified, and a record names
  `repositories`, a map from declared ID to the full commit of that repository the run observed.
  A `coverage` exclusion MUST additionally hold, for every declared repository, that the record
  names its commit, that the commit is an ancestor of that repository's base, and that between
  them no covered qualified path, no global path of that repository and no qualified link target of
  a flow linking the test changed; otherwise the record is `e2e-map-stale` for that test. A record
  naming an undeclared ID, or a tier path that is neither a canonical root path nor a canonical
  qualified path of a declared ID, is malformed (`blocked`, `provider-unavailable`).
- **AFU-V1-071**: Reviewed links: a `source` link target MAY name a qualified path. A declared link
  with such a target and `reviewed_at` MUST also carry `reviewed_at_repository`, the full commit of
  that repository the review saw; the member is refused on any other link. The link is `reviewed`
  only when the root anchor rules of AFU-V1-008 hold, that commit is an ancestor of the
  repository's base, and the target's content (the whole file, or the line span) is the same at
  that commit and at the base. When the repository is not bound, which is always the case outside
  this profile, the state is `target-unpinned`, never `reviewed`.
- **AFU-V1-072**: Compatibility: without declared or bound repositories the `e2e-safe-selection/1`
  bytes are unchanged and `repositories` is absent; the schema string stays `/1` because the
  member is additive and optional. Reading and binding are read-only: Corvint runs only Git reads
  in each repository and writes nothing there.

Acceptance: in `cmd/corvint`, `TestAFUV1067RepositoryArguments` (AFU-V1-067: each argument
refusal, and a declared repository bound with its base is accepted),
`TestAFUV1068SeparateTestRepositorySelects` (AFU-V1-068, AFU-V1-070, AFU-V1-071: T holds the suite
and A the application; an A change selects the T test whose coverage names it, omits the other
on the coverage basis or the reviewed-links basis, and binds A's base, revision and tree),
`TestAFUV1069UnreachedApplicationPathFailsClosed` (AFU-V1-069: an A path no record or link names
is `e2e-unmapped-change`; an unbound or undeclared repository, a wrong origin, a checkout grafted
onto the origin and an A change `x:y.ts` beside a T file `app:x:y.ts` are
`e2e-repository-unbound`; A's `Gemfile` is `e2e-global-path-changed`),
`TestAFUV1070CoverageRepositoryCommitRequired` (AFU-V1-070: a record without the A commit, and an
A covered path changed since that commit, are `e2e-map-stale`) and
`TestAFUV1024StrictAndCoverageBytesUnchanged` with the existing corpus (AFU-V1-072); in
`internal/appflows`, `TestAFUV1071QualifiedLinkTargets` (AFU-V1-071: intent validation of the
qualified target and `reviewed_at_repository`, and `target-unpinned` without a bound repository),
`TestAFUV1069QualifiedGlobalPaths`, `TestAFUV1068AncestryRewrittenRefused` (AFU-V1-068: grafted
and shallow checkouts) and `TestAFUV1070CoverageTierPathsValidated` (AFU-V1-070: undeclared and
non-canonical tier paths).

Non-goals: no planning snapshot per repository, no impact graph over an application repository
(qualified paths gain no static reach), no qualified `test`, `assertion` or `evidence` targets,
no discovery or inventory outside the root, and no change to the `strict` and `coverage`
`--repository` contract (EEP-V1). `flows export` omits a link with a qualified target, since the
exported traceability format has no repository member, and docs maintenance records a qualified
target as an absent root input rather than pinning the application file.

Failure modes: binding the wrong checkout under a declared ID would let foreign evidence narrow;
the declared `origin` must be a parentless commit of its HEAD, and a grafted or shallow checkout,
whose ancestry Git rewrites, does not bind. An application path that cannot be written as a
canonical qualified path would parse back as a root path and gain root ownership; it unbinds the
repository. A coverage tier path naming an undeclared ID or a non-canonical path would miss the
change it names; it makes the coverage malformed. An application change outside the
bound range (an uncommitted edit, a base after the change) would go unseen; status paths join the
changed set, and the base is the operator's explicit input bound into the result. A link reviewed
against one application commit and evaluated at another would be trusted stale; its content is
compared at `reviewed_at_repository` and at the base. An unbound declared repository would drop
its changes silently; it is `e2e-repository-unbound`.

Rollback: revert this section, the `repositories` members of the provider, coverage record and
result, `reviewed_at_repository`, `--repository-base` and their tests, and restore the e2e-safe
`--repository` refusal. Providers and intents that do not use the new members need no change.

## Symbol-anchor re-resolution in flows check (issue 1223, V1-1971)

- **AFU-V1-090**: (accepted under the owner's standing auto-accept (2026-10-11)) `docs flows
  check` re-resolves each imported corpus anchor that names a symbol by `DCP-V1-140..142` after the
  exact-span relocation of AFU-V1-045, and reports the result as the row's `symbol`. A `live` or
  `moved` symbol is `fresh` with `after` at the live range; a `changed` symbol is `stale` with the
  estimated live range; a `gone` symbol is `stale` with no `after`. An `unresolved` symbol keeps the
  exact-span verdict, so missing evidence never upgrades a row.

Acceptance: `TestFlowDocsImportedSymbolAnchorReresolution` (a 20-line insertion is `fresh` and
`moved` at line 21, a reformat is `fresh`, a one-token edit is `stale` and `changed`, a deletion is
`stale` and `gone` with no `after`).

Failure modes: a whitespace-only reformat would read as a content change; the region digest drops
comments and whitespace outside literals. A same-name symbol in another file would be taken for
the moved one; only an equal region moves across paths. Rollback: revert this section, the
`symbol` member and `revalidateImported`; exact-span relocation is unchanged.

## Corpus repository mismatch refusal (issue 1479, V1-2314)

- **AFU-V1-091**: (accepted under the owner's standing auto-accept (2026-10-11)) `docs flows
  generate --corpus FILE` parses the corpus artifact and compares its primary repository and
  revision with the generation source before corpus Open rederives it against `--root`. On a
  mismatch it refuses with the typed `flowdocs-corpus-repository-mismatch` code, whose message names
  the corpus primary repository and revision and the generation repository and revision. When the
  corpus declares the generation repository only in `additional_repositories`, the message says so.
  That form is not accepted: the supported corpus has the documented repository at `--revision` as
  its primary (AFU-V1-047), and `docs/FLOW-DOCUMENTATION.md` states that shape.

Acceptance: `TestFlowDocsCorpusRepositoryMismatchIsTypedUpFront` (an evidence-primary corpus that
declares the application as an additional repository, and a corpus of the application at an older
revision, are both refused with the typed code naming both repositories; the matching corpus is
accepted).

Non-goals: accepting the generated repository as a declared additional repository with roots
resolved against an explicit corpus root. That needs a corpus-root input and cross-repository anchor
rules for generated claims, and stays unbuilt.

Failure modes: before this check, Open joined the corpus's additional roots and primary revisions
against `--root` and failed with an unrelated checkout or bad-object error, or the corpus-refused
`invalid additional repository declaration`. Rollback: revert this section, the documentation
paragraph, `corpusSourceMatches` and its test; the later equality check after Open still refuses a
mismatch, with the old messages.

## Declared flow inventory for flows generate (issue 1478, V1-2313)

Human-owned intent: a team that already keeps a reviewed flow inventory can keep those flow pages
current with `generate` and `check`, instead of receiving one flow per extracted declaration.
Inventory membership is caller-declared grouping of source observations; it never establishes
accepted intent, runtime order or a semantic join. Requirements accepted under the owner's standing
auto-accept (2026-10-11).

- **AFU-V1-101**: `docs flows generate` accepts optional `--flows FILE`, a closed
  `corvint-flow-inventory/1` object `{"schema", "flows": [{"id", "title", "members": [{"path",
  "symbol"?}], "entry_points"?: [{"path", "symbol"?}]}]}`. An id is unique lowercase `[a-z0-9._-]`
  of at most 128 bytes; a title is 1 to 200 bytes of printable UTF-8; each flow has at least one
  member; a member path is confined like `--scope`; a symbol is the declaration name generation
  reports (for example `Billing::Charge#call`), and a member without one takes every unambiguous
  declaration in its path. Unknown members, another schema, duplicate ids or members, a path-only
  member overlapping another member of the same path in one flow, and an entry point that is not a
  member of its flow are refused with `flowdocs-refused`, never repaired
  (`TestFlowDocsInventoryRefusals`).
- **AFU-V1-102**: With an inventory, generation emits exactly one flow per declared flow that has
  at least one resolved member, in inventory order, with identity `flow` of `inventory:<id>` and the
  declared title. The flow aggregates every resolved member's static calls, lexical claims and
  revalidated corpus context; each member also gets a member paragraph (in `entry-points` for a
  declared entry point, otherwise `technical-deep-dive`) bound at its declaration. Paragraph and claim
  identities are keyed by flow id, member path and declaration name. Same-file and same-name
  relations between flows are not derived for inventory flows and a gap says so
  (`TestFlowDocsInventoryAggregatesDeclaredFlows`, `TestFlowDocsCLIInventory`).
- **AFU-V1-103**: A member that does not resolve at the revision is reported in the generation
  manifest's `inventory.flows[].unresolved` with its path, symbol and reason (path has no unambiguous
  supported declaration; symbol not declared in the path; symbol declaration ambiguous) and in its
  flow README, never dropped. A declared flow with no resolved member is reported with
  `generated: false` and no pages (`TestFlowDocsInventoryAggregatesDeclaredFlows`).
- **AFU-V1-104**: In-scope unambiguous declarations that belong to no declared flow are listed in
  `inventory.unassigned` by path and declaration name and are not turned into flows. Without
  `--flows` the generation manifest has no `inventory` member and the output is byte-identical to
  the declaration-per-flow behaviour (`TestFlowDocsInventoryAggregatesDeclaredFlows`, plus the
  unchanged existing flowdocs suite).
- **AFU-V1-105**: The manifest retains the declared inventory, so `Open`, `finalize`,
  `generate --previous` and `check` rederive with the same inventory. `check` keeps per-flow,
  per-paragraph and per-claim fresh/stale/unresolved against the inventory identities: an unchanged
  member stays fresh, a changed claim span is stale, and a removed member's paragraph and claims are
  unresolved and retired (`TestFlowDocsInventoryCheckIdentities`).
- **AFU-V1-106**: The inventory file is a bounded regular file of at most 4 MiB with 1 to 256 flows
  and at most 8,192 members in total; a member outside `--scope` is refused because generation never
  reads it; resolution considers at most 16,384 in-scope declarations, above which generation refuses
  and asks for a narrower scope (`TestFlowDocsInventoryRefusals`).

Failure modes: a stale inventory naming renamed declarations reports them unresolved rather than
guessing the rename; overlapping members across different flows are allowed and give each flow its
own identities for the shared declaration; flow-diagram call lines name lines without a path, so a
multi-file flow's diagram is read with the member paragraphs. Rollback: revert this section,
`flowinventory.go`, `flowbuild.go` and the `--flows` flag; the declaration-per-flow path is unchanged
and manifests without an inventory remain valid.

## Per-test coverage ingest (GitHub #1587)

Human-owned intent: a team that already collects coverage per test can turn it into observed
`tested_by` evidence that is pinned to the committed source and to the run that produced it, and
can see where a declared or supplied test link is contradicted by what the test actually executed.
Coverage is generated, observed evidence of execution, never of assertion or review. Requirements
accepted under the owner's standing auto-accept (2026-10-11).

`corvint flows ingest-coverage --format v8|istanbul|rspec|lcov --from FILE --evidence FILE
[--evidence FILE ...] --revision FULL_COMMIT [--test-key KEY] [--links FILE] [--strip-prefix
PREFIX]` prints one `corvint-coverage-ingest/1` report on standard output.

- **AFU-V1-112**: The report names the repository, the full revision and its tree, and pins every
  input (the coverage file, each `--evidence` file and the links file) by role, SHA-256 and byte
  count, never by local file name. Each covered test binds to exactly one `test-run-evidence/0`
  record whose test key equals it, and the report keeps that record's run ID, project, runner,
  source and record digest. Every relation, conflict, link plan and unknown is sorted, the report
  is generated trust, and its bytes are identical across runs; the command writes nothing to the
  repository, the index or a trace store (`TestAFUV1112CoverageIngestDeterministicPinned`,
  `TestAFUV1112FlowsIngestCoverageCLI`).
- **AFU-V1-113**: V8 coverage (an array of script entries, or `{"result": [...]}` as Playwright and
  `NODE_V8_COVERAGE` write it) is applied to the committed blob in UTF-16 units, with nested
  function ranges overriding their parents, only when the entry's `source` equals the blob or, when
  no source is given, the file is plain JavaScript (`.js`, `.mjs`, `.cjs`). An `http(s)` URL is
  `v8-url-needs-source-map`, a non-file scheme such as `node:` is `path-outside-repository`, a
  differing source is `v8-source-mismatch`, and a transpiled file without its source is
  `v8-transpiled-without-source`. Source maps are not resolved. Identical script entries for one
  file (same source digest and ranges) are applied once, and the aggregate source scanning and
  painting work across all scripts is bounded: past it the file is the unknown
  `coverage-bound-exceeded` with no relation (`TestAFUV1113CoverageIngestV8`).
- **AFU-V1-114**: A single-test Istanbul `coverage-final.json` (statement map and counts) or LCOV
  file (`SF`, `DA`, `end_of_record`) is accepted with `--test-key`, or without it when the run
  evidence holds exactly one record. Absolute paths under the checkout, its resolved real path or
  `--strip-prefix` map to repository paths; any other path is `path-outside-repository`. LCOV
  `TN:` sections must name one test: a trace with a second distinct `TN:` name is refused with
  `coverage-refused` rather than attributing another test's lines to the key
  (`TestAFUV1114CoverageIngestIstanbulAndLCOV`).
- **AFU-V1-115**: RSpec per-example coverage is a JSON object from example ID (`./` trimmed, for
  example `spec/x_spec.rb[1:1]`) to path to a line-count array, or to `{"lines": [...]}` as
  SimpleCov writes it, with `null` for non-executable lines. `--test-key` is refused for this format
  because each example names its own key; an example with no run record is the unknown
  `no-run-record` (`TestAFUV1115CoverageIngestRSpec`).
- **AFU-V1-116**: For a joined test, each covered file at the revision with at least one executed
  line becomes an `observed` `tested_by` relation from `source:<path>` to `test:<key>` with its
  instrumented and executed line counts, executed line spans and `external-provider` anchors pinned
  to the blob. Each `corvint-coverage-links/1` link (`declared` or `supplied`, with subject, test
  and reference) gets a plan: `corroborated` when the test executed a subject line, `conflicted`
  with a typed `conflicts_with` record (`coverage-contradiction`, anchored to the instrumented lines)
  when the subject was instrumented but none of its lines ran, and `unknown` with
  `subject-not-instrumented`, `test-not-covered` or the run's reason otherwise. A covered file
  whose executed lines cannot be anchored to the blob (for example a minified line over the 64 KiB
  excerpt bound), or a subject whose instrumented lines cannot be anchored, is the unknown
  `coverage-unanchorable`: no relation, corroboration or conflict is emitted from it
  (`TestAFUV1116CoverageIngestContradictedLinks`, `TestAFUV1116CoverageIngestUnanchorableLines`).
- **AFU-V1-117**: Coverage whose run record is from another commit or tree is `stale`
  (`coverage-stale`), and from a dirty worktree is `stale` (`run-source-dirty`); a test whose every
  attempt was skipped is `test-not-run`. A record whose `coverage_artifacts` (AFU-V1-120) do not pin
  the coverage input's SHA-256 is `unjoined` (`coverage-not-bound-to-run`), so an older or foreign
  coverage file next to a valid record yields no relation. None of these joins: no relation or conflict is emitted for
  them. A covered line past the end of the committed file is `coverage-line-out-of-range`, an
  executed blank line is `coverage-line-blank`, and a file absent at the revision is
  `source-not-at-revision`; each is an unknown with a reason, never a guess
  (`TestAFUV1117CoverageIngestStaleNotJoined`).
- **AFU-V1-118**: An unknown format, a missing input, a revision that is not a full commit, an
  ambiguous single-test binding, malformed coverage, another links schema or an invalid link is
  refused with `coverage-refused`, as is an LCOV trace naming a second `TN:` test; a revision that is not a commit of the repository with
  `coverage-invalid-revision`; and a bound with `coverage-bound-exceeded`: each input is a regular
  file of at most 5 MiB (a larger coverage, evidence or links input is `coverage-bound-exceeded`,
  never `coverage-refused`), at most 2,048 tests, 4,096 files, 4,194,304 lines, 1,048,576 V8 ranges and
  4,096 links. Unknowns past 4,096 are counted in `unknowns_omitted`, and an anchor list past 64
  anchors is truncated with a limitation (`TestAFUV1118CoverageIngestRefusals`).
- **AFU-V1-119**: `flows ingest-coverage` with a missing required flag exits 2 with its usage line,
  `corvint flows --help` lists the command, and a refusal is printed as its typed diagnostic
  (`TestAFUV1112FlowsIngestCoverageCLI`).
- **AFU-V1-120**: A `test-run-evidence/0` record MAY carry `coverage_artifacts`, the SHA-256 of
  each coverage file its run wrote, and `flows ingest` sets them from a repeatable
  `--coverage-artifact ID=FILE` flag (a malformed value exits 2 with the usage line). Decode accepts
  at most 64 entries with unique IDs and valid digests, never on a `STATIC` record, and omits the
  member when empty, so existing records keep their bytes. Coverage ingest joins a test only through
  a record that pins its coverage input this way (`TestAFUV1120RunEvidenceCoverageArtifacts`,
  `TestAFUV1112FlowsIngestCoverageCLI`).

Non-goals: source-map resolution for bundled or transpiled V8 coverage, so V1-1182 (Playwright V8
via pinned source maps, `AFU-V1-021`) is not satisfied by this slice; branch or function coverage;
writing the relations into a corpus provider record or a flow store; joining a test by anything but
its exact run-record key; treating executed lines as asserted behavior.

Failure modes: coverage for a test with no or several run records, from another revision or a dirty
worktree, without a pinned link from its run record, or for a skipped test is reported but not
joined; coverage that does not line up with the
committed blob (a mismatched V8 source, an out-of-range or blank line) is an unknown rather than a
shifted relation; an over-bound input is refused before decode. Rollback: revert this section,
`internal/appflows/coverage_ingest.go`, `cmd/corvint/flows_ingest_coverage.go`, the
`ingest-coverage` entry and help block in `cmd/corvint/flows.go` and `LineIndex.Bytes`; no stored
state depends on the report.

## Extractor inventory flow import (issue 1583)

Human-owned intent: an adopter who ran a repository-declared extractor profile
(`DCX-V1-390..399`) can seed proposed flow intents from its endpoints and UI surfaces instead of
typing them, while every imported flow stays `proposed` with `inferred` links until review.
Requirements accepted under the owner's standing auto-accept (2026-10-11).

- **AFU-V1-122**: `flows import --format extractor-inventory` accepts only a
  `corvint-extractor-inventory/1` document, bounded and secret-screened like the other import
  sources; another schema is refused naming the accepted one, and the `--format` refusal lists
  every format. The import keeps AFU-V1-004's all-or-nothing write rules
  (`TestAFUV1122ImportExtractorInventoryRefusals`).
- **AFU-V1-123**: It writes one proposed `api` flow per `endpoint` subject (identity from the
  `METHOD path` name, one `call` step and variation) and one proposed `ui` flow per `ui_surface`
  subject (identity `ui-` plus the path, one `visit` step and variation). Job, event, data-entity,
  symbol and interaction subjects, relations and the inventory's gaps are not flows and are not
  imported (`TestAFUV1122ImportExtractorInventory`).
- **AFU-V1-124**: Each imported flow carries one `inferred` source link per subject anchor, with
  the anchor's path and exact line span; a flow subject without an anchor or a derivable identity
  refuses the whole import (`TestAFUV1122ImportExtractorInventory`,
  `TestAFUV1122ImportExtractorInventoryRefusals`).

Non-goals: no outcomes, actors, preconditions or test links are invented from an extracted route,
and the import does not re-verify the inventory against Git; `flows check` and review do.
Failure modes: two subjects whose names normalize to one flow ID refuse the import as a repeated ID;
an inventory with no endpoint or UI subject imports nothing. Rollback: revert this section, the
`extractor-inventory` importer in `internal/appflows/import.go` and its help text; the other
formats are unchanged.

## Typed flow-limit refusal (V1-2414)

Human-owned intent: an operator whose generation scope declares too many flows learns which bound
refused it and by how much, from a code they can match, instead of an untyped `flowdocs-refused`.
Accepted under the owner's standing auto-accept (2026-10-11).

- **AFU-V1-125**: Without `--flows`, `docs flows generate` MUST refuse a scope with more than
  `MaxFlows` (256) flow candidates with the owned code `flowdocs-flow-limit-exceeded`. Its message
  names the scope, the `MaxFlows` limit and its value, and the observed candidate count of the whole
  scope: generation keeps counting candidates past the limit, without building their flows, so the
  count is exact rather than the first candidate over the bound. Exactly 256 candidates still
  generate. A candidate is an in-scope declaration that is not a duplicate within its file, the same
  set that becomes a flow below the limit.

Acceptance: `TestAFUV1125FlowLimitIsTyped` (300 candidates over two files are refused with the
typed code naming `300 flow candidates` and the limit; a 256-candidate scope generates 256 flows);
`TestAFUV1125FlowLimitCLICode` (a 257-candidate scope exits 2 with stderr `code`
`flowdocs-flow-limit-exceeded`; it failed with `internal-error` before the CLI lift).
It failed before the change with `flowdocs-refused: flow limit exceeds 256; narrow scope`.

Non-goals: raising the limit; typing the declared-inventory bounds (`in-scope declarations exceed
16384`, `flow inventory must declare 1 to 256 flows`), which keep `flowdocs-refused`.

Failure modes: the CLI's `docs flows` error path lifts this code, like `flowdocs-generator-stale`,
to its own stderr wire `code` (`cmd/corvint/docs_flows.go`); any other `doccorpus.Error` still
reaches stderr as `internal-error`. Counting past the limit extracts every in-scope source file once,
as a generation under the limit already does, but skips anchor and symbol lookup for candidates
past the limit. Rollback: revert this section,
`FlowLimitCode` and the candidate count; the refusal returns to the untyped message.

## E2E-safe flow-ID screening and refusal subject (GitHub #1888, V1-2788)

Human-owned intent: an adopter whose flow IDs are derived from spec file names (for example
`cov-tests-e2e-admin-auth-password-recovery-spec-ts`) keeps `e2e-safe` selection, and an adopter
whose provider or coverage file is refused learns which document and which class of refusal, without
bisecting the provider. Status: experimental under the standing lane auto-accept.

- **AFU-V1-126**: The `application-flow-selection-provider/1` secret screen MUST NOT read a
  `flow_tiers` member name as a credential name when that name is literally a quoted identifier
  (`[A-Za-z][A-Za-z0-9_-]{0,63}`, no escape) of the top-level `flow_tiers` object. When the
  whole-text screen hits, the provider is screened again with each such name replaced by a neutral
  name, and it is refused as secret-shaped only when that second screen still hits. Every value
  (including each tier string), every other member name, a `flow_tiers` name outside the identifier
  shape, and the coverage file stay screened exactly as written; every other `Decode` caller is
  unchanged.
- **AFU-V1-127**: A `blocked` / `provider-unavailable` `e2e-safe-selection/1` result MUST carry
  `refusal` (`code` `provider-unavailable`, `subject` `<document>:<class>`), where document is
  `provider` or `coverage` and class is one of the closed set `unreadable` (absent, not a regular
  file, or changed while read), `byte-limit`, `secret-shaped`, `invalid-json`, `invalid-schema`
  (unknown, missing or mistyped member), `trailing-input`, `invalid-path` (not repository-relative)
  or `invalid-content` (a decoded document failing its semantic validation). The member never
  carries refused text and is absent on every other state. `fallback` keeps its meaning and stays
  empty when blocked. The member is additive; the schema string stays `e2e-safe-selection/1`, and a
  consumer that decodes the result as a closed object must accept the new optional member.

Acceptance: `TestAFUV1126FlowTierIdentifierKeyIsNotSecret` (a password-bearing flow ID is accepted
through the provider path while generic `Decode` still refuses it; a secret tier value, a
non-identifier or over-long name, a credential assignment in a test key or global path, a nested
`flow_tiers` member and a top-level `password` member stay refused);
`TestAFUV1126E2ESafeFlowTierPasswordKey` (the issue's reproduction through `corvint affected` no
longer blocks); `TestAFUV1127ProviderUnavailableNamesRefusal` (each class for each document, and no
secret in the subject or error); `TestAFUV1024E2ESafeRefusesMalformedInput` (the CLI result carries
`coverage:invalid-schema`).

Non-goals: exempting coverage `repositories` map names or any other map (a repository ID next to a
40-hex commit is kept screened, since that pair is shaped like a token assignment); decoding JSON
escapes before screening; per-member or per-line refusal detail.

Failure modes: a credential stored as a tier string under an identifier-shaped `flow_tiers` name is
no longer caught by the name rule; it is still caught by every value-shape rule (vendor tokens,
credentialed URLs, private keys, JWTs). A mask walk over malformed JSON stops early, leaving the
rest screened as written; the parse then refuses it. A subject outside the closed set is a defect,
and an unclassified read error is reported as `unreadable`, never as success.

Rollback: revert this section, `maskFlowTierKeys` and the `decode` exemption (flow IDs are refused
again), and the `Refusal` member with `selectionRefusalSubject` (blocked results return to the bare
`provider-unavailable`).

## Playwright list import links (GitHub #1915, V1-2871)

Human-owned intent: an adopter who imports `playwright test --list --reporter=json` output gets
test links that `flows map` and `flows gaps` resolve, keyed the way `affected` and the JavaScript
test provider key the same tests, and one same-title test or one existing flow does not cost them
the whole import. Status: experimental under the standing lane auto-accept.

- **AFU-V1-128**: A `playwright-list` import MUST resolve each listed spec's `file` against the
  list's `config.rootDir`, or, without one, the single `testDir` every `config.projects[]` entry
  declares, to a clean repository-relative path under `--root` (both sides symlink-resolved). The
  import is refused, writing nothing, when the list declares neither, or when that directory is not
  an existing absolute directory inside `--root`. A spec whose file does not resolve under `--root`
  is skipped and reported (AFU-V1-129). Each link's `path` is that repository path and its
  `test_key` is the path, the enclosing describe titles (outermost first; the top-level file suite
  adds none) and the title joined by ` > `; a `corvint-test-key` annotation still overrides the
  key.
- **AFU-V1-129**: An imported flow ID MUST be the file stem, the describe titles and the title,
  normalized as every imported ID is. Listed tests with different keys that normalize to one ID
  each get the suffix `-` plus the first 8 hex digits of SHA-256 of their own key, so the suffix
  is stable across lists. A listed test is skipped and reported, while every other test is
  written, when its key is shared by another listed test, when its flow ID belongs to an existing
  intent, or when its flow or variation ID is declared elsewhere or retired. `corvint flows import`
  then prints the written paths and exits 1 with `imported N listed tests; skipped M:` and one
  `skipped "<test key>": <reason>` line per skipped test on stderr.
- **AFU-V1-130**: `corvint flows --help` MUST say that a playwright-list import supplies test
  links only: one proposed `ui` flow per listed test, with its file resolved under the root and
  keyed path > describe > title. Steps, actors and outcomes are authored by hand. A collision is
  reported per test, and the command exits 1.
- **AFU-V1-131**: `corvint flows ingest --format playwright-json` MUST key each test the way a
  `playwright-list` import of the same tests keys its links (AFU-V1-128): the spec file resolved
  through the report's `config.rootDir` (or the single `testDir` of its projects) to a
  repository path under `--root`, then the describe titles and the title joined by ` > `; a
  `corvint-test-key` annotation still overrides the key. A test whose file cannot be placed keeps
  its legacy `file > title` key, is still ingested, and is reported as unjoined; every
  `playwright-receipt` test is reported unjoined, since its key is the qualified test digest.
  Each `flows` verb receives the stderr diagnostic writer (V1-2873); `flows ingest` writes one
  `flows ingest: <from>: test "<key>" does not join playwright-list import keys: <reason>` line
  per unjoined test and still exits 0. Status: experimental under the standing lane auto-accept.
- **AFU-V1-251**: A `playwright-list` import MUST join the listed specs that share a repository
  path, describe path, title and test key and whose tests name projects with no project in
  common into one test, imported once with every project in its variation's `projects`, so a list
  that repeats each test per project (projects sharing a testDir) yields no collision. Specs that
  share a project, or a test without a project name, stay apart and are skipped as before
  (AFU-V1-129), with the reason saying the duplicate is within one project. Failure mode (#1930):
  144 tests of a two-project suite were skipped as ambiguous. Rollback: drop `mergeProjects`.
  Status: experimental under the standing lane auto-accept (V1-2924; GitHub #1930).
- **AFU-V1-252**: A variation's `no-test` gap MUST read `only inferred test links, which count once
  reviewed` when it has an inferred test link and no declared one, and `no declared test link`
  otherwise; the codes are unchanged. Failure mode (#1930): `inferred-only` and `no-test ... no
  declared test link` read as contradictory after an import. Rollback: restore the single detail
  in `variationGaps`. Status: experimental under the standing lane auto-accept (V1-2924; GitHub
  #1930).

Acceptance:
- `TestAFUV1131IngestJoinsPlaywrightListImport` imports a playwright-list, ingests a Playwright
  JSON report of the same tests and requires every ingested key to equal an imported link key with
  silent stderr; a report without `config` is still ingested, exit 0, with one unjoined diagnostic
  per test naming the reason.
- `TestAFUV1128PlaywrightListResolvesDescribeKeys` covers:
  - a `testDir` list with same-title tests under two describes;
  - a case-only describe pair, which gets digest suffixes;
  - a true duplicate and a file that escapes the root, each skipped and reported;
  - a re-import that writes nothing and reports every test;
  - refusal of a root outside `--root`, a missing root, and a list with no root.
- `TestAFUV1128FlowsCLIPlaywrightListLinksResolve` covers:
  - import from a list with `testDir: './tests'`;
  - `flows map --path`/`--test-key` resolving the imported links;
  - `flows gaps` clearing `no-test` once a link is declared;
  - a CLI re-import that exits 1.
- `TestAFUV1ImportOpenAPIAndPlaywright` covers the annotation override and the describe-path ID.

Non-goals:
- Run-evidence keys from `flows ingest-run` and the prebound run path are unchanged. They stay
  `<rootDir-relative file> > <title>`, so run evidence joins an imported link only through a
  `corvint-test-key` annotation (a known gap).
- The other import formats stay all-or-nothing.
- No list is generated or executed, and per-project `testDir` lists with differing directories
  are not reconciled.

Failure modes:
- A list whose recorded `rootDir` came from another checkout fails as outside `--root`. Regenerate
  the list in this checkout.
- A collision that this rule does not anticipate still refuses the whole import through
  validation, never with a partial overwrite.
- A partial import's exit 1 is a scripting-visible change from the former exit 2 refusal.
- Flow IDs, keys and paths from earlier imports differ from the new form, so re-importing an old
  list beside its earlier flows writes new flows next to them.

Rollback: revert this section with `playwrightListDir`, `listSpecs`, `disambiguate`,
`dropCollisions` and `ImportSkipped`. Playwright import then returns to rootDir-relative keys
without a describe path, and to an all-or-nothing refusal.
