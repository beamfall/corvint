# Revision-pinned documentation corpus V1

Owner: Russell Lewis
Date: 2026-09-19
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner request [issue 31](https://github.com/beamfall/corvint/issues/31),
owner requests [issue 40](https://github.com/beamfall/corvint/issues/40),
[issue 42](https://github.com/beamfall/corvint/issues/42) and
[issue 53](https://github.com/beamfall/corvint/issues/53), and the owner-authorized adoption slice
[issue 339](https://github.com/beamfall/corvint/issues/339), [issue 399](https://github.com/beamfall/corvint/issues/399), [issue 1162](https://github.com/beamfall/corvint/issues/1162) and [issue 1225](https://github.com/beamfall/corvint/issues/1225),
`AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/specs/deployment-neutral-index-platform-v0.md`,
`docs/specs/source-documentation-draft-v0.md`, `docs/specs/external-evidence-provider-v0.md`.

## Agent digest
- Claim: A native compiler builds revision-pinned documentation corpora for bounded CLI, evidence, rendering and capability-gated MCP queries.
- Status: proposed/experimental; DCP-V1-053 accepted (decision 0519; V1-1196); DCP-V1-054 and DCP-V1-055 accepted (decision 0520; V1-1226, V1-1231); DCP-V1-056 and DCP-V1-057 accepted under the owner's standing auto-accept (2026-10-11; GitHub #805); DCP-V1-060..069 legacy inventory producer accepted under the owner's standing auto-accept (2026-10-11; GitHub #811); DCP-V1-070..079 (test-management case import, GitHub #812) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-080..082 (behavior annotation contract, producer and missing annotations, GitHub #835; V1-1222) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-083..084 (valueless state matchers, GitHub #845; V1-1318) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-085 (encoder-pool-independent allocation, V1-1380) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-086..092 (test-knowledge records, GitHub #1162; V1-1868) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-093..099 (flow-level relations, GitHub #1225; V1-1973) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-140..142 (symbol-anchor re-resolution, GitHub #1223; V1-1971) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-143..145 (lazy corpus MCP load, reused freshness and path batches, GitHub #1236 and #1237; V1-1982) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-146 (verbatim oversized paragraphs, over-bound line gaps and located refusals, GitHub #1242; V1-1983) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-150..152 (asserting E2E gap conditions and unauthenticated review, GitHub #1245; V1-1988) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-165..169 (case search by ID and ranked any-word match, GitHub #1246; V1-1989) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-160..163 (multi-scope /2 manifest inventory, GitHub #1241; V1-1985) accepted under the owner's standing auto-accept (2026-10-11); this contract does not accept generated intent or qualify HDC.
- Exists: native compiler/reader, explicit integrations, separate MCP, renderer, independent adapter and labelled evaluation.
- Blocked on: owner acceptance and independent external utility qualification; local completion requires the gates below.
- Read next: Requirements; Input and authority boundary; Acceptance and rollback.

## User job and verified current state

Compile reusable documentation knowledge for a repository without a separate database or retrieval
service. A consumer can locate original evidence, ask what changed code touches, inspect explicit
unknowns and render generated blocks while retaining human prose. Existing `internal/doccompiler`
drafts one owner Markdown source and one Go package. `internal/contextindex` already extracts
immutable source, symbols and relations; `internal/testvaliditydoc` decodes retained native receipts
and recomputes five-axis projections. Neither existing seam grants semantic adequacy or accepted
intent. The frozen core MCP surface and CEM wire remain unchanged.

## Requirements

- `DCP-V1-001`: The explicit manifest, normalized provider and artifact use closed versioned JSON
  schemas. Unknown members, duplicate members/IDs, trailing documents and noncanonical artifact
  bytes refuse. Compilation is deterministic for identical declared inputs, native builder and
  supplied timestamp; canonical ordering is by stable IDs, never filesystem traversal or wall time.
- `DCP-V1-002`: Provenance carries builder identity/revision, shared native compiler/schema/toolchain identity, profile identity,
  revision and digest, repository identity and source commit/tree, supplied build timestamp, every
  provider implementation version and artifact revision, input manifest/digest and artifact digest.
  Source revision, provider-artifact revision and provider implementation version are distinct.
  Entry-point executable hashes do not identify the shared compiler. Dirty/unrecorded build revisions
  remain explicit limitations and provide no immutable build attestation.
- `DCP-V1-003`: Each literal source/provider scope has a complete immutable Git inventory checked
  before analyzer admission. Missing, undeclared, multiply claimed inputs without an explicit merge
  rule, unknown providers, mismatched revision/blob/digest and generated outputs used as source refuse.
  Explicit disjoint union is the only merge rule; duplicate record IDs still refuse. Excluded or
  unsupported declared source inputs remain gaps with reasons, never successful empty extraction.
- `DCP-V1-004`: Every claim, relationship and journey step carries revalidatable evidence or an
  explicit unknown reason. Anchors pin repository, commit, path/artifact, blob/content hash, exact
  nonempty line span and span hash, optional symbol/region, authority, evidence kind and inclusion
  reason. A structurally valid anchor does not validate a semantic assertion; declarations retain
  external-provider authority and generated excerpts make only exact extraction claims.
- `DCP-V1-005`: Derivation (`generated`, `source-derived`, `observed`, `declared`, `imported`), trust
  (`generated`, `verified`, `reviewed`), claim state (`supported`, `conflicted`, `unknown`) and freshness
  (`fresh`, `stale`, `unknown`) are independent. Generated claims cannot acquire verified/reviewed trust.
  Imported review labels remain attributed declarations, never authenticated Core review or intent.
- `DCP-V1-006`: Subjects use generic types: repository, package, module, symbol, endpoint, UI surface,
  capability, use case, interaction, test, journey, document, external system, data entity, change,
  finding and business term. Core contains no adopter/framework-specific taxonomy. Relations use only
  `implements`, `calls`, `imports`, `documents`, `tested_by`, `journey_for`, `depends_on`, `affects`,
  `related_to`, `conflicts_with`, `supersedes`, retaining direction and evidence. Co-citation creates
  no relation; declarations create no test observation or journey.
- `DCP-V1-007`: Native providers reuse indexed source/symbol/document evidence and explicitly distinguish
  test declarations from native retained observations. Observation inputs pass `testvaliditydoc.Decode`
  and `ProjectPinned`; carried projections are ignored. Preserve exact receipt digest, test package/name,
  run/source identity, omissions, native execution/association/hygiene/adequacy axes and preview limits.
  Freshness separately rebinds original source digests; opaque Go identity and E2E app-build identity
  stay unknown, and explicit source/build staleness stays stale. Exact identities,
  never test-name similarity, join observations to tests, subjects and journey steps.
- `DCP-V1-008`: Journeys contain independently supplied ordered evidence, preconditions, action types,
  expected observations, run/test identities, cleanup and limitations. A separate pinned
  `corvint-corpus-journey-observations/1` artifact must contain the entire ordered step list, exact
  native receipt digest/source/test identity and successful cleanup. Every step joins the same run;
  incomplete runs and unknown/stale freshness cannot qualify. States distinguish `verified`,
  `generated_not_verified`, `missing_journey`, `non_ui`, `blocked`. Only exact matching retained run
  and step witnesses qualify recorded verification; passing a run never proves step assertion,
  adequacy or completeness. No route/navigation/declaration invents steps.
- `DCP-V1-009`: Every capability is explicitly declared with backing records, native tools/provider
  identities, row count, denominator, reproducible rule and reason when absent. Distinguish absent or
  unsupported, present with zero records, valid-filter no-match, not-found, not-collected, stale and
  provider-unavailable. Tools are gated by declarations after validation, never file/table existence.
- `DCP-V1-010`: Coverage always names value, denominator, definition/rule, revision and limitations.
  Zero denominator is explicitly undefined; no universal behavioral coverage percentage is emitted.
  Gaps include unsupported analyzers, incomplete extraction, unobserved tests/journeys and conflicts.
- `DCP-V1-011`: Readers verify canonical artifact/digest and rederive all compiler-owned records from
  original pinned inputs before answering. Missing/tampered provider artifacts, moved spans, source
  drift, changed builder/profile and mutable checkout differences refuse or appear in a separate
  freshness overlay. A later provider commit alone does not stale unchanged source bytes.
- `DCP-V1-012`: `docs corpus` exposes manifest/build/info/search/get/locate/related/coverage/gaps/journey/stability/
  trace/validate through a shared bounded native reader. Search reuses native lexical tokenization;
  results sort deterministically and retain omissions. Every response carries contract/artifact/source
  revisions, trust, freshness, citations, limitations and typed misses. Reads never write repository,
  trace, index, observation or task state.
- `DCP-V1-013`: Explicit native corpus integration supplies first-class separately attributed
  documentation to query/context, path/symbol-to-subject/relationship/test/journey evidence to impact,
  and retained observation links to test-validity. No-corpus invocations retain previous bytes and
  behavior. Affected advice refuses narrowing when source-to-obligation/test/journey evidence is
  incomplete or stale and always preserves mandatory repository checks; it executes no test.
- `DCP-V1-014`: CEM integration projects original Git documentation spans at the CEM base through
  existing citation semantics and exposes a digest-bound provenance sidecar naming artifact, claim,
  evidence ID, source and limits. A generated claim is not itself original authority. Unavailable or
  different base spans refuse; CEM wire fields and existing CEM behavior do not change.
- `DCP-V1-015`: Task/work consumers may attach read-only corpus evidence without changing accepted
  task intent, claims, dispatch, scheduling or authorization. Corpus evidence never owns task state.
- `DCP-V1-016`: A separate stdio documentation-corpus MCP server directly calls the native reader;
  tools equivalent to docs_info/validate/search/get/locate/find_related/coverage/gaps/get_journey/get_stability/trace are
  advertised only when their backing capability is present (including present-zero). Revalidate on
  calls, preserve typed failures, frame model-facing text using `repoenvelope`, refuse terminator
  collisions and match CLI structured receipt bytes. Existing core and draft MCP surfaces stay frozen.
  The separately owner-approved MCPV0-021..023 shared compatibility amendment permits explicit
  `--protocol-version 2025-11-25`; the required `--root` and `--artifact` arguments, capability
  gating, evidence bounds and experimental/non-shipped status remain unchanged.
- `DCP-V1-017`: Declared profiles render Markdown or JSON, optionally grouped as a directory plan,
  with explicit generated markers, evidence citations, states and limitations. Output is deterministic
  and remains generated. Preview is read-only; apply is an explicit operation. Maintenance freshly
  rederives output and verifies the page digest, preserves all human bytes/permissions, and refuses
  malformed/duplicate markers, tampered blocks, accepted intent, symlinks and concurrent replacement.
  Apply pins the parent descriptor, captures the original inode, and publishes without clobbering a
  competing destination. It retains the captured inode at a reported recovery path even on success,
  preserving late writes from an editor holding it open. Explicit later operator cleanup may remove
  recovery files. Publication has a brief absent-path window and is not a crash-atomic transaction.
  Existing draft/consume/maintain contracts are unchanged.
- `DCP-V1-018`: Input byte bounds apply before decode; record/revision/traversal bounds apply before
  expansion, and bounded in-memory output is checked before publication. Paths are local, symlink-safe and network-free. No provider or test execution, daemon,
  second database, embedding engine, implicit source repair, commit, PR, merge or publication occurs.
  Hostile imported text is data, not instructions. Resource/cancellation failures remain explicit.
- `DCP-V1-019`: Generic fixtures cover Go, JavaScript/TypeScript, Python or Ruby, unsupported analyzers,
  no UI/tests, conflicting evidence, stale/moved anchors, hostile imports and separate provider/source
  commits. An independent example adapter supplies a product flow without adding domain concepts to
  Core. Corvint compiles and queries an actual committed slice of its own repository.
- `DCP-V1-020`: A frozen labelled evaluation records per-case truth/denominators, search precision and
  recall, abstention accuracy, false-positive relationships, latency and receipt bytes, with corpus,
  source, profile and builder identities. Synthetic evidence remains labelled; no unmeasured savings,
  universal relevance, independent real-world utility or HDC qualification is claimed.
- `DCP-V1-021`: The opt-in `corvint-corpus-behavior-stability-provider/1` profile aggregates only
  immutable qualified Playwright receipt inputs. Each contribution binds its receipt digest plus
  application and test Git revisions, configuration and behavior-contract digests, runner/version,
  browser/project, worker/retry policy, environment class/digest and fixture schema/digest. All
  identities are identical within an aggregate unless the repository-owned policy explicitly names
  that exact matrix dimension. Receipt test/config inputs are rebound through explicit source-path
  mappings to their Git bytes and the behavior test's exact path, digest and line anchor; carried or
  inferred identity is never substituted.
- `DCP-V1-022`: Planned repetitions use a complete unique ordinal set. Playwright retries remain
  ordered attempts inside one repetition, while manual reruns use separately identified contributors
  and never fill a planned ordinal or threshold. Missing iterations, duplicate receipt digests,
  stale/changed inputs, cross-revision identities, inexact test/project joins and contradictory
  receipt/attempt evidence refuse the aggregate instead of reducing its denominator.
- `DCP-V1-023`: Aggregates separately expose planned, started, completed, passed, failed, timed-out,
  interrupted, infrastructure-failed, skipped, flaky, retry-consumed, cleanup-failed and manual-rerun
  raw counts. Every repetition and attempt has cleanup status. Any failed/unknown cleanup prevents a
  clean verdict. Earlier assertion, synchronization, product, fixture or infrastructure failures and
  their receipt-bound artifacts remain in contributing attempts even when a retry or manual rerun passes.
- `DCP-V1-024`: One repository-owned digest-bound policy defines independent one-spec, feature-batch
  and suite thresholds. The selected scope, policy ID/digest, raw counts and threshold result remain
  visible. No policy may turn a partial or contradictory aggregate into a verdict.
- `DCP-V1-025`: Corpus `stability` and capability-gated MCP `docs_get_stability` rederive the aggregate
  and return all contributing receipt identities. Stability remains a separate artifact and coverage
  axis from issue-40 behavior-contract joins; neither axis claims test adequacy, behavior parity,
  current served content, narrowing authority or promotion authority.
- `DCP-V1-026`: The repository policy and every observed run bind separate full-artifact topology
  evidence. Their closed identity includes CI nodes, total shards, Playwright workers per node,
  database sharing/isolation mode, sorted project set, split algorithm/version and resource class.
  Each observation binds its exact receipt digest and policy ID. Declared and observed topology must
  match exactly before threshold evaluation, and the receipt's verified executed project must belong
  to that project set. A fully passing four-node run cannot qualify a six-node policy, and malformed,
  unbound or mismatched topology refuses the aggregate.
- `DCP-V1-027`: The opt-in native behavior adapter accepts one closed, bounded
  `corvint-behavior-adapter-request/1` document. Each generic entity mapping names one input, one
  RFC-6901 record-list pointer and relative pointers for its scalar or list fields. The caller marks
  the reviewed flow and variation inventories that form the normative documentation contract. Each
  globally unique `variation_id` carries explicit preconditions, actions, observable facts, expected
  outcomes, allowed projects and exact tests. Nested anchors, assertions, semantic test claims,
  ordered events, runtime runs and observation links use closed wire shapes; the adapter never
  invents review anchors, repairs identities or sorts event order.
- `DCP-V1-028`: One deterministic `corvint-behavior-adapter-result/1` bundle contains an exact
  `corvint-corpus-behavior-provider/1` record, the normalized normative variation projection,
  canonical test semantic claims and complete migration, live-discovery and runtime sidecar bytes
  with their SHA-256 identities. The provider retains the issue-40 legacy /1 member and digest
  algorithm. Sidecars remain separately publishable inputs, avoiding self-reference.
- `DCP-V1-029`: The adapter preserves the same canonical variation set in both directions across
  flow, variation, exact test ID, Playwright project, assertion and semantic test claim. Preconditions,
  actions and observable facts match exactly; every expected outcome matches criterion, behavior,
  matcher, locator and value. Allowed projects, required flow pages/events/negative controls and
  runtime witnesses reconcile in both directions. Source- or test-discovered candidates remain
  proposed; generation never rewrites the normative documentation projection, expands its denominator
  or emits a confirmed missing-E2E finding without an explicit current reviewed absence.
- `DCP-V1-030`: The result separately reports documented criteria, source-discovered candidates,
  discovered project executions, linked contracts and runtime-witnessed contracts. Every row names
  value, denominator, revision, rule and limitations; empty or incomplete input is `unknown` or
  `unreviewed`, and a zero denominator is undefined.
- `DCP-V1-031`: Reconciliation reports every documented or discovered orphan, criterion/test/flow
  contradiction, missing reverse link, assertion-free execution, stale anchor, missing or out-of-order
  page/assertion event, absent negative control, missing immutable runtime witness, semantic mismatch,
  undocumented tested behavior, documented untested behavior, unreviewed normative input and missing
  or extra project witness. Caller labels never qualify a receipt. Every adapter result, including one
  later accepted by the corpus compiler, retains `full-relevant-suite` fallback and no narrowing authority.
- `DCP-V1-032`: An optional prior result yields exact added, removed and content-changed canonical
  variation IDs plus lost flow/test, semantic-claim and assertion reverse links even when aggregate
  counts are unchanged; semantic fields participate in the content digest. The prior result must pass
  closed schema, stable provider/contract/repository lineage, self-consistent historical revisions,
  artifact and contract digests, uniqueness and semantic-link validation before it can affect a delta;
  a later revision of the same repositories remains comparable. Semantic-link validation tolerates a
  criterion or claim naming a variation absent from the normative set when DCP-V1-031 reconciliation
  itself retains and reports that same reference; a claim naming a variation reconciliation would not
  have retained still refuses (decision 0337). Lost reverse-link entries are printable corpus text,
  never the NUL-joined form corpus text fields forbid elsewhere, and are validated as such when a
  prior result is read. Diagnostics name the input, exact
  mapped JSON field, revision, digest and corrective action; multi-field refusal selection is
  deterministic and preserves RFC-6901 empty path tokens. Output
  ordering and bytes are deterministic, and new mapping, result and diagnostic vocabulary contains no
  adopter-specific repository, product, organization or path terms; caller data and the legacy
  compatibility member remain attributed input.

- `DCP-V1-033`: The opt-in `corvint-corpus-input/2` and `corvint-evidence-corpus/2`
  profiles admit a lossless supplied-record import within explicit capacity limits: 100,000 each
  subjects, claims and relations, 4,096 journeys, 100,000 typed details, 16 providers and 128 total
  provider records/shards. Each adoption provider uses `corvint-corpus-adoption-provider/1` with
  either one `record` or sorted unique `shards`, never both. Shards share provider ID, version and
  source identity; every shard is an exact pinned manifest input. Validate aggregate counts,
  global IDs and cross-shard joins. Refuse overflow, missing/reused shards and identity drift.
  Provider shards remain at most 64 MiB each; all pinned inputs together and the artifact are
  bounded to 128 MiB. Existing `/1` and independent behavior/receipt limits remain unchanged.
- `DCP-V1-034`: Adoption records retain typed behavior, constraint, side-effect, integration and
  permission claims; flows with all eight canonical sections; stable paragraph IDs and acyclic
  retirement redirects; asserting versus navigating test links with explicit confidence and exact
  file/title/project; reproducible explicit-membership coverage definitions; historical ticket
  records; and reported-intent versus observed-claim comparisons. Each detail belongs to one owned
  normalized record; the combined record and detail are at most 1 MiB. Validate referenced record kinds and active paragraph
  membership. These are attributed declarations, never accepted intent, test adequacy or independent
  runtime verification. Existing source-span rebinding and trust downgrade apply without exceptions.
- `DCP-V1-035`: Restricted findings use a separate local-input family. Admit only aggregate count
  and severity totals into artifacts, rendering and read projections. Never emit original finding
  IDs, local detail paths or bodies. Preload all provider shards before admitting generic evidence;
  refuse any public reference to restricted tokens, including cross-shard typed fields and manifest
  metadata. Restriction input is bounded to 1 MiB in aggregate. The provider owns classification;
  this is not a semantic secret detector or permission to publish the original local input. Live
  freshness diagnostics redact paths outside the pinned inventory, including paths created later.
- `DCP-V1-036`: Report supplied-record parity per provider and source kind with records in, admitted,
  dropped and reasons; restricted drops are explicit. Do not infer completeness of the original
  repository. `/2` inventory, get/trace/search and other read operations support stable bounded
  offset pagination tied to the artifact digest, explicit total omissions and `next_offset`.
  Every normalized record family is retrievable. Typed details and citations describe only the
  returned page. Capability summaries retain totals with explicit omitted-ID counts; use inventory
  for the full list. Freshness reason lists are deduplicated and bounded with explicit omissions. CLI, all existing `--corpus` consumers and corpus MCP open the larger artifact;
  receipt bytes remain 4 MiB and result pages at most 256 records. Impact omissions retain the
  mandatory full-check fallback and never silently narrow tests. Old profiles retain their wire
  and capability behavior; inventory and MCP offset are advertised only for `/2`.
- `DCP-V1-037`: Qualify the full Build/Open/read path with a labelled synthetic corpus containing at
  least 150 flows, 10,000 claims, 20,000 paragraphs, 35,000 relationships, 30,000 inventory records,
  200 journeys and 4,000 tickets. Retain exact counts, bytes, complete-command timings, CLI/native
  consumer/MCP parity, stale-source behavior and hostile shard/restricted-input cases. Synthetic
  qualification is not an external adopter utility claim. Focused checks and independent review
  govern this scoped issue slice under the owner's standing repository policy.

- `DCP-V1-038`: CLI and local corpus MCP expose all existing reads, including validate and
  inventory, with the same capability gates and native structured receipt. Typed concept, claims
  by path/symbol, eight-section flow, explicit upstream/downstream dependencies, test recommendation,
  navigation, vocabulary and recorded intent reads preserve typed details and citations. No relation
  is inferred from shared citations; business terms resolve only documented records/relations.
  Test recommendations retain `full-relevant-suite-required` under ETS-V0: declared links and review
  labels cannot establish exhaustive adequacy. Gap kinds distinguish no tests, missing exact reviewed
  asserting E2E links (`DCP-V1-150`..`152`), missing manual disposition, stale journeys and stale anchors;
  unknown is not absence.
- `DCP-V1-039`: Native answers use `corvint-corpus-receipt/2` for the additive trust envelope; artifacts
  remain unchanged. Every native answer carries source revision, corpus digest, builder, generated content
  status, freshness, bounded citations, limitations and retirement envelope. Imported trust labels
  remain attributed declarations. Optional caller-declared retirement names evaluated time, age limit
  and/or source/target revision-bound merge distance/limit; future build times, negative/overflow or
  mismatched identities refuse. Unobserved age/distance stays unknown; policy expiry is distinct from
  paragraph redirects and never upgrades authority. Empty results say `not documented` while retaining
  capability-absent, filter-miss, not-found and pagination-exhaustion machine distinctions.
- `DCP-V1-040`: The separate experimental `corvint-corpus-index/1` companion wraps a canonical `/2`
  corpus and a deterministic rederived ID/path/symbol/term/edge index. Producer Build revalidates source.
  The experimental republish path may carry that complete source-validation result in a non-wire
  opaque immutable compiler token. Warm reconstruction validates complete cached records against
  pinned original inputs under the ordinary provider predicates and requires exact prior corpus bytes;
  caller artifacts or cache identity alone cannot mint a token. The ordinary cold API remains an
  independent verification oracle.
  Admission requires an externally operator-pinned complete SHA256; self-consistent embedded hashes
  alone grant no provenance. Read-only consumers have no live source or Git input and report historical
  evidence with freshness unknown, producer authentication NOT_OBSERVED and validation
  `index-digest-validated; source-revalidation-unavailable`. Maximum wrapper bytes 256 MiB, corpus
  bytes 128 MiB, existing per-family record limits, 1,000,000 postings, 16 MiB unique key bytes and
  existing per-record/receipt bounds refuse overflow; malformed offsets or altered postings refuse.
  Per-path fanout is at most 100,000 references. Index ordering and identical producer inputs yield byte-identical outputs. A labelled synthetic
  25,000-claim/5,000-symbol qualification retains counts, allocations, bytes and query parity.
  The ordinary open/query allocation ceiling is 1 GiB. Race-instrumented runs retain all functional
  checks and report observed allocations with the ordinary allocation budget `NOT_RUN`;
  instrumented allocation costs do not qualify that ordinary budget;
  no general external scale, production index promotion or DNIP benchmark-gate claim follows.
- `DCP-V1-041`: A companion switch-over harness consumes bounded closed recorded questions and
  complete expected receipts. It reports per-tool agreement/refusal and stable missing/extra/changed
  record identities; strict complete receipt comparison includes details, trust/freshness, capability
  state, omissions, retirement, citations, limitations and full-suite fallback. Identical inputs
  produce identical reports. Record equality with different evidence semantics is disagreement;
  supplied-question agreement grants no authority to retire the previous server.
  Strong record IDs reject duplicates. Records without strong IDs use canonical-content digests
  and occurrence numbers even when only one occurrence exists. Adding or removing an identical weak
  record changes only that occurrence in missing/extra diagnostics; reordering preserves diagnostic
  keys. Weak content changes report missing/extra, while strong-ID content changes report changed.
- `DCP-V1-042`: The separately started experimental hosted companion serves only embedded digest-pinned
  artifacts through bounded JSON-over-HTTP queries; it does not claim MCP Streamable HTTP compatibility.
  Local source-revalidated stdio MCP remains separate and unchanged in transport. No source/Git/provider,
  shell, credential or writing API is admitted. At most two concurrent requests, 16 KiB input,
  five-second query context deadline, header/body timeouts and cancellation bound the transport;
  explicit non-loopback exposure requires `--allow-remote` and an operator-owned external TLS/auth
  boundary. Shutdown retires the listener and in-flight requests. Local conformance is not external
  deployment/authentication/security/interoperability qualification; those axes stay NOT_OBSERVED.
- `DCP-V1-043`: The `corvint-corpus-parity` companion is qualified through its own command boundary.
  Every read, producer, open, query, comparison or encoding failure exits nonzero with empty stdout
  and a stderr diagnostic; no failure may exit zero with an absent or partial artifact. Two command
  builds from identical inputs emit byte-identical indexed artifacts; query emits the trust-enveloped
  native receipt; parity over a recorded question set binds index and recording digests, reports
  per-tool agreement and repeats byte-identically. A wrong operator pin refuses without a report.
- `DCP-V1-044`: `corvint docs corpus behavior-adapter --input REQUEST.json [--previous RESULT.json] --check`
  validates the whole request. It neither emits nor writes an adapter result or artifact. It emits one
  deterministic `corvint-behavior-adapter-check/1` report that lists every build stage in build order
  as `passed`, `refused`, `not-evaluated` or `not-applicable`, and lists every refusal it can determine
  in that same order, each with its stage, code and message. Items are inputs, mappings,
  observations, mapped records and discovery executions, and each item is evaluated independently
  of the other items. Within one item, evaluation stops at the item's first refusal, and exactly one
  `not-evaluated` entry then names the item: `remaining checks for <item> not evaluated after
  <refusal>`. A stage or item whose dependency was refused is listed as `not-evaluated` with the
  stages that blocked it. An identity-uniqueness refusal belongs to the first item, not already
  refused, whose identity is invalid or repeats an earlier one. When a count bound on inputs,
  mappings, observations or discovery executions refuses, the items it bounds are not evaluated, and
  one `not-evaluated` entry says so. The report keeps at most 1024 refusal entries and about 1 MiB of
  them, always including the first; a terminal `not-evaluated` entry `further entries omitted after
  <N>` marks any omission. Each entry's message is capped at 4 KiB, cut on a UTF-8 boundary and
  followed by ` … [truncated <N> bytes]`. Both caps apply as entries are recorded, so the entries a
  check retains never exceed what the report can list. The check computes and retains no
  reconciliation frontier, coverage, provider subjects or delta key sets, none of which can refuse a
  build; the delta check runs the same canonical encodings as the build without keeping their keys. No check is silently omitted, and the report
  does not list every independent refusal within one item.
  The report is accepted exactly when the build would succeed, and its first refusal equals the build's
  refusal truncated by that same cap; the build's own error is unchanged. The command exits 0 when accepted, 1 when refused and 2 when an input file cannot be read;
  `--check` takes no value and applies to no other corpus operation. The report retains
  `full-relevant-suite` fallback, and its vocabulary contains no adopter-specific term. Status:
  (accepted, decision 0482; V1-1084; GitHub #717).
- `DCP-V1-045`: The behavior-adapter guide (`docs/DOCUMENTATION-CORPUS.md`) states the input-anchor
  placement rule: every adapter input anchor names the provider repository, equal to the request
  `source`, which equals the provider entry of `revisions`. It names the refusal correction given
  otherwise. The guide also gives a minimal valid schema-2 migration identity record (`schema`,
  `contract_id`, `source_revision`, `documentation_revision`, `revisions`) and the self-referencing
  revision pitfall: the record describes an earlier provider commit and is committed, and anchored,
  in a later one. A test closed-decodes the guide's example and builds an accepted request from it
  under that rule. Status: (accepted, decision 0482; V1-1086; GitHub #717).
- `DCP-V1-046`: `corvint docs corpus discover-playwright --migration MIGRATION.json --config PATH
  --playwright-list LIST.json [--receipt RECEIPT.json]` converts a caller-run
  `playwright test --list --reporter=json` report into the canonical
  `corvint-playwright-discovery/1` record with mode `live-playwright-list`: the schema-2 migration's
  revision set, one full-file config anchor and one execution per listed test and project, each a
  one-line anchor at Playwright's reported line. Every anchor is the Git blob at the migration's
  source revision in the repository whose root commit is the migration's E2E repository, and the
  working-tree bytes the listing observed must equal that blob. Corvint never runs Playwright; output
  is deterministic and the command writes nothing. Status: (accepted, decision 0489;
  V1-1082; GitHub #717).
- `DCP-V1-047`: An execution's identity is `playwright:` plus Playwright's own per-project test ID.
  With `--receipt`, a canonical qualified Playwright receipt for the same config path, config bytes
  and test-file bytes supplies the identity instead for the one receipt test with the same file,
  line, project and title; more than one match refuses. The produced record, unchanged, is admitted
  by `behavior-adapter` as its discovery input, and each registered test whose ID equals an
  execution identity joins that execution in `discovered_project_executions`. Status: (accepted, decision 0489; V1-1082; GitHub #717).
- `DCP-V1-048`: `discover-playwright` refuses, with `corpus-refused` and no output, a listing that
  is filtered (any `test` argument other than `--list`, `--reporter=json` and `--config`), sharded,
  errored, of another config, rooted outside the repository, has a repeated or missing test ID,
  location or project, or is empty or over the execution bound; a listing whose project/file pairs
  differ from those the config selects in the working tree, or that omits a pair the config selects
  among the regular files at the source revision, under the same static-membership and
  skipped-directory refusals as `affected` Playwright discovery (a clarification from review round
  2, not a change of accepted intent); a config or test file that is not a
  regular blob at the source revision or differs in the working tree; a repository that is not the
  migration's; a malformed migration, including a `documentation_revision` that is not a Git object
  ID or differs from `revisions.docs_corpus.revision`; and a receipt that is not qualified or ran other config or
  test bytes. Status: (accepted, decision 0489; V1-1082; GitHub #717).
- `DCP-V1-049`: `corvint docs corpus witness-playwright --input REQUEST.json --receipt-input ID
  --report REPORT.json` validates the behavior-adapter request, then emits one deterministic
  `corvint-behavior-witness-import/1` bundle that binds the contract ID and digest, source revision,
  the retained receipt input (ID, revision, path, SHA-256), the caller-run Playwright JSON report's
  SHA-256, and for every registered test either a witness or an unwitnessed entry. The receipt input
  must be a retained request input other than the migration and discovery inputs. Corvint never runs
  Playwright. Status: (accepted, decision 0489; V1-1083; GitHub #717).
- `DCP-V1-050`: A witness is produced only when the qualified receipt test with the registered ID
  ran the registered project, file bytes, line and title, passed on its only attempt (retry 0) with
  a `PASSED` projection, no other receipt outcome shares its file, line, project and full title, and
  the report has exactly one result for that file, line, project and full title path (file, describe
  and test titles joined with ` > ` as the receipt's `fullName` is) whose status is `expected` with
  one passed result carrying an explicit retry 0, behavior-event annotations that form one unique
  passing sequence numbered from 1 with a closed kind, identity and browser context, exactly one
  `passed` cleanup annotation and the registered fixtures and roles. The
  witness carries the canonical `corvint-behavior-run/1` document (run digest = receipt digest), its
  SHA-256, the report test ID and digest, and an observation link whose run identity, project and
  source paths match `behavior-adapter`'s receipt check. Once the caller retains that document and
  wires it as the test's runtime, `runtime_witnessed_contracts` counts the contract. Events are read
  from the report, never copied from the declared flow. Status: (accepted, decision 0489;
  V1-1083; GitHub #717).
- `DCP-V1-051`: Every other registered test stays unwitnessed with exactly one named reason and is
  never credited: `foreign-project`, `source-mismatch`, `receipt-test-missing`, `failed`, `skipped`,
  `flaky-after-retry` (including any retried pass), `report-ambiguous` (receipt outcomes sharing a
  test identity or file, line, project and full title, or several report results for one),
  `report-test-missing` (including a result with an absent or null retry), `report-receipt-disagree`,
  `events-unobserved`, `event-malformed` (an event that does not decode or breaks the sequence,
  uniqueness, passing, kind, identity or browser-context rules), `cleanup-unobserved`,
  `cleanup-failed` or `fixture-role-mismatch`. Status: (accepted, decision 0489; V1-1083;
  GitHub #717).
- `DCP-V1-052`: Missing or untrustworthy shared evidence yields uncertainty for every test, never
  certainty: an unqualified receipt (`receipt-unqualified`), an incomplete, omitted or stale one
  (`receipt-incomplete`), one of other config bytes (`config-mismatch`), or a report of another config
  file or runner version (`report-foreign-config`) leaves all tests unwitnessed with that reason. The
  bundle states the fallback `full-relevant-suite` and its limitations. Status: (accepted, decision
  0489; V1-1083; GitHub #717).
- `DCP-V1-053`: `corvint docs --help`, and the `docs corpus discover-playwright --help` and
  `docs corpus witness-playwright --help` topics (amended by `DCP-V1-056`), name the qualified
  receipt producer: the stdout of `corvint-js-test-provider e2e --external-server` is
  the canonical qualified Playwright receipt, and `--retain` writes the same bytes under
  `.corvint/test-evidence/`. Managed-server `e2e` output is not a qualified receipt.
  `discover-playwright --receipt` reads that file unchanged. `witness-playwright --receipt-input`
  names a request input, not a file. The only conversion step is that the caller commits the
  receipt bytes unchanged at a tracked path and adds them to the request as an observed
  `external-provider` input with a full-file anchor. The help gives one end-to-end example (provider
  `e2e` run, retained receipt, `discover-playwright --receipt`, `witness-playwright
  --receipt-input`). A focused test runs the example's discovery step on the fixture with the
  provider's canonical encoding, admits the witness step's arguments, and checks every flag of the
  producer step against the provider's `e2e` flags. Status: (accepted, decision
  0519; V1-1196; GitHub #794).
- `DCP-V1-055`: `discover-playwright` applies the `TJAA-V0-021` grep/grepInvert omission rules of
  `affected` Playwright discovery to the same listing. When a selected (project, file) pair is missing
  and its project declares `grep` or `grepInvert`, the producer reads that file's listed cases. It
  accepts the omission whether it is explained or unverified. An accepted omission has no listed
  execution and adds none: the record holds only listed executions, so a registered test the omission
  may hide is not discovered, and is not invented. The source-revision membership check accepts the
  same omissions. Cases come only from specs that have a listed execution, attributed to the file that
  execution carries. An unexplained omission still refuses, as does a selected file at the source
  revision that the listing omits and the working tree lacks. A focused test runs the GitHub #790
  three-project repro through the listed-tests producer with the revision's paths, and checks both
  refusals. Status: (accepted, decision
  0520; V1-1231; GitHub #800).
- `DCP-V1-056`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #805)) `corvint help docs corpus` and `corvint docs corpus --help` (or `-h`) print one
  corpus help topic, not the generic `docs` orientation: it lists every operation the corpus parser
  admits, one per line with a one-line purpose, and points at `corvint docs corpus OPERATION --help`.
  `corvint docs corpus OPERATION --help` prints that operation's own usage, each option it admits
  with its required or optional form and a one-line description, and a `Relative to --root:` line
  naming exactly the `DCP-V1-054` local input options that operation admits; the two Playwright
  operations also print the `DCP-V1-053` receipt guidance, and `import-cases` and `search-cases`
  the `DCP-V1-070..079` test-management export guidance. Both forms exit 0 with empty stderr, read
  no stdin and inspect no repository (`GPK-V0-059`), and follow the `GPK-V0-062`/`GPK-V0-064` help
  scan: `--help` after admitted options and values is still help. The parser and the help topics
  read one operation table (`cmd/corvint/docs_corpus_help.go`), so help cannot name an option the
  parser refuses. An unknown operation is not a help topic: `docs corpus NAME --help` falls through
  to the corpus parser with its refusal order unchanged. The parser checks each option before the
  operation, so `--help` is refused first as `unsupported corpus option` (`invalid-arguments`, exit
  2). Non-goal: `corvint help docs corpus OPERATION` is not a topic. Acceptance:
  `TestDocsCorpusHelpListsOperations`, `TestDocsCorpusOperationHelpPrintsFlags`,
  `TestDocsCorpusOperationFlagMatrix`.
  Rollback: remove the corpus help topics; `docs corpus ... --help` prints `docs` help again.
- `DCP-V1-057`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #805)) `corvint help TOPIC` with an unknown topic stays the `invalid-arguments` refusal,
  exit 2, empty stdout, with the existing envelope members. Its message follows the argparse
  invalid-choice convention: `unknown help topic: 'TOPIC' (choose from ...)` lists every
  `topLevelCommands` verb in order, then the nested topics `harness event`, `docs corpus`,
  `docs flows` and `docs maintain`, each of which `corvint help` accepts. Acceptance:
  `TestUnknownHelpTopicListsValidTopics`, `TestUnknownHelpTopicIsConventionalError`.
- `DCP-V1-060`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) `corvint docs corpus discover-legacy --runner jasmine|mocha --revision FULL_COMMIT
  --path PATH [--input REGISTRY.json]` is an experimental static producer. It reads the Jasmine or
  Mocha/Cypress test files at `--path` (one Git blob, or every test-named `.js`, `.ts`, `.jsx`,
  `.tsx`, `.mjs` or `.cjs` file under a directory, skipping `node_modules`; `.` is the whole tree)
  from Git at the full commit and prints one closed `corvint-legacy-inventory/1` record with status
  `generated`. Its `legacy` array uses the existing `BehaviorLegacyCase` shape unchanged, so the
  cases can be pasted into a behavior registry. Every anchor is an `external-provider` source span
  bound to the blob's digest. It runs no test, browser, Node or package manager and downloads
  nothing.
- `DCP-V1-061`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) Each statically named `it` (and its runner's aliases) under statically titled
  suites appears exactly once, with ID `legacy:<path>::<suite titles > case title>`, the title path
  split into `suite` and `case`, and an anchor spanning the call. When a title path repeats in a
  file, the first occurrence keeps the plain ID, each later one gets an ordinal ` #n` suffix
  (n from 2), and every occurrence has a `duplicate-title` unknown. `xit`, `xdescribe`,
  `xcontext`, `xspecify`, `.skip`, a case without a callback, a jasmine `pending(` call and any
  case inside a disabled suite have state `disabled`; everything else is `executable`. A focused
  `fit`, `fdescribe` or `.only` case stays in the inventory with a `focused` unknown. A case inside
  a dynamic scope (a loop, a callback that is not a suite, or a call that is not a statement of its
  own, such as an unbraced `if` body or `flag && it(...)`, with automatic semicolon insertion
  applied at line breaks) or with a non-static title is not invented: it is a
  `case-in-dynamic-scope` or `dynamic-title` unknown. A runner call inside a template substitution
  is a `runner-call-in-template` unknown, a case whose callback is not an inline function has a
  `case-callback-not-static` unknown, a case that calls `this.skip()` has a `runtime-skip` unknown,
  and a case whose span cannot be anchored is a `case-span-unanchorable` unknown.
- `DCP-V1-062`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) Assertions in a case body become ordinal criteria `c1..cn` with matcher, locator
  and value tuples: `expect(subject).matcher(value)` and `expectAsync`, `assert(...)` and
  `assert.X(...)`, Cypress `.should(chainer, value)` and `.and(...)`, and a `cy`-rooted `.contains`.
  A leading Jasmine `withContext(...)` is skipped. String escapes are decoded (`\n`, `\xHH`, `\uHHHH`,
  surrogate pairs, `\u{...}`, identity escapes); a legacy octal, malformed or lone-surrogate escape
  refuses the file. A single static
  string value is unquoted; no argument is `(no expected value)`; anything else is the layout-free
  canonical token text. An assertion the reader cannot turn into one tuple (a `should` callback, a
  computed matcher, a nested helper, a chain that continues after the first matcher such as
  `.and.equal(...)` or `['and']`, a Cypress `.and(...)` not chained from a `.should(...)`, a matcher
  reached through a computed member such as `['should']`, a `.contains` whose second argument is
  neither a literal nor a plain options object, an assertion inside a function the case defines
  but does not pass directly as a call argument, or an assertion or Cypress `.contains`/`.and`
  inside a template substitution) is listed under `unknown` as
  `unparsed-assertion` or `should-callback` with its case and anchor; it is never dropped, and a
  case with no static assertion has a `no-static-assertion` unknown. Only an `expect` that is not a
  member (`chai.expect`, `globalThis.expect`) is extracted. A helper is proven only as a `function`
  or arrow expression passed whole as one direct argument of a plain call; a declared, assigned,
  parenthesised, immediately invoked or otherwise grouped function, an object or class method,
  getter or setter, and a class body are unproven, and an assertion inside one is an
  `unparsed-assertion` unknown, never a criterion. After extraction a fail-closed residue check
  scans the body's code tokens: every `expect`, `expectAsync`, `assert`, `should`, `and` or
  `contains`, every precondition word (`task`, `request`, `intercept`, `fixture`, `visit`, `exec`,
  `route`, `goto`), every `cy`, `browser` or `page` chain and every runner suite, case or hook call
  that no extraction consumed adds an unknown (`unparsed-assertion`, `unparsed-command` or
  `runner-call-in-case`); so does such a word as a string anywhere inside a bracket, an unbound
  matcher member (`to` or a `toX...` name other than a built-in such as `toString`), a `cy`, or a
  `browser`/`page` precondition, reached as a member, a precondition word reached through a
  computed member or such a root, an optional (`?.`) or otherwise computed `cy`/`browser`/`page`
  chain, a bare `cy` value, a runner function used as a value (`beforeEach.call(...)`) and a
  template substitution naming one. A runner function so used at suite scope is a
  `runner-reference-not-static` unknown, and a hook so used gives every case in that scope
  `hook-not-static`. A method named by any keyword other than a statement header, or by a header
  keyword inside an object literal, is unproven. A keyword used as a property name ends an
  expression for the regex/division decision, and a numeric literal is scanned to its JavaScript
  grammar; a literal followed directly by an identifier character refuses the file. The residue
  check is a diagnostic; the verdict comes from a closed-world allowlist. A case is admitted only
  when every token of the file structure (runner suite, case and hook calls with one static string
  title and an anonymous, parameterless, synchronous callback with a block body, separated by `;`
  or line breaks), of each
  enclosing hook body and of its own callback and body belongs to a recognized statement form:
  `expect(<operand>)`, optionally `.withContext(<literals>)`, then either exactly one call of a
  listed Jasmine/Jest matcher with literal arguments, never negated, where `toHaveLength` and
  `toHaveSize` take only a decimal integer literal of at least 1, or a chai chain that, less an
  optional leading `to`, equals one admitted chainer; a `cy` chain of known queries with literal
  arguments, optionally `.contains(<content>)` or `.contains(<selector>, <content>)`, then
  `.should('<chainer>', <values>)` and `.and(...)` with one admitted chainer each, where no `.and`
  or further `.should` follows `have.attr` or `have.length` (they may yield a new subject) and a
  query after `contains` has a `.should` directly after it; a `cy`, `browser` or `page`
  precondition call with literal arguments and an optional `.as('<name>')` (the only form
  admitted in a hook). The admitted chainers are a closed set of whole strings, never composed
  from words, each a positive assertion on the subject: `be.visible` (it is visible), `exist`
  alone (it exists), `be.checked`, `be.disabled` and `be.enabled` (that state holds), each with no
  value; `contain` (it contains the value), `contain.text` and `include.text` (its text contains
  the value), `have.text` (its text equals the value), `have.value` (its value equals the value),
  `have.class` (it has the class) and `have.length` (it has exactly that many elements, a decimal
  integer literal of at least 1), each with one value; and `have.attr` (it has the attribute,
  with the given value when a second is supplied). In a chai chain an entry with no value is not
  called and an entry with a value is called with it. A negated, composed (`exist.and.be.a`),
  zero-length or unlisted chainer is refused as a whole. Operands that are literals (strings without
  substitution, numbers, `true`, `false`, `null`, arrays and plain objects of those) or plain
  identifier member paths; and `;`, line breaks and comments. Anything else, including any other
  call, a computed member, an escaped identifier, a regex-ambiguous `/`, an alias or declaration,
  `.call`/`.apply`, a member-qualified runner name, a template substitution, an import, a
  callback parameter, `await`, an `async` or expression-bodied callback, a chainer outside the
  admitted set, a negated Jasmine matcher, a `contains` options argument, and a single string expected value whose
  unquoted text could read as another literal's canonical text (it starts with a digit, `.`, `-`,
  `+`, `[`, `{`, `(` or a quote, or with `true`, `false` or `null`), is a `not-allowlisted` unknown naming the first refused token and its region
  (file structure, an enclosing hook, case callback or case body). An admitted statement whose key
  token no extraction consumed is also `not-allowlisted`. A case with more assertion tuples than
  the registry's per-case criterion bound (4,096) has no criteria and a `criteria-over-bound`
  unknown; its list is never truncated. A case with more distinct preconditions than that bound
  has no fixtures and a `fixtures-over-bound` unknown, and discovery fails when the inventory's
  distinct preconditions exceed it, as it does for cases and unknowns. Over-refusal is accepted: helper calls, Cypress actions,
  `afterEach`, `done` and `async` callbacks, awaited Playwright or WebdriverIO calls, subject
  changing continuations, every chai or Cypress assertion outside the admitted chainer set
  (including `equal`, `be.true` and type checks), `contains` options, ambiguous string values and regex literals make a
  case unknown, never parity. The allowlist does not prove what runs outside the case: other cases' bodies and hooks
  of non-enclosing suites (cross-test state is a runtime-baseline question), getters on operand
  paths, custom matchers or commands that override a known name, and support files. A case with
  any unknown never reaches `parity-candidate` (`DCP-V1-066`). LF, CR, CRLF, U+2028 and U+2029 are each a line terminator: each ends a `//`
  comment, separates statements for ASI and counts as a JavaScript line break; anchor lines stay
  LF-delimited as for every corpus anchor.
- `DCP-V1-063`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) Preconditions are content-bound references. Every top-level call statement in an
  enclosing `beforeEach`/`before` hook, and every `cy.visit`, `browser.get`, `browser.url`,
  `page.goto` (`visit`), `cy.intercept`, `cy.route` (`intercept`), `cy.task`, `cy.exec` or
  `cy.fixture` (`setup`) call in the case body (a member named by one bracketed static
  identifier-name string, as in `cy['task']`, resolves to that member; any other computed member is
  a `DCP-V1-062` residue unknown, and in a hook a `hook-not-static` doubt), is one `preconditions` entry with kind, callee, the
  full canonical call text (layout-free: a space separates only two word tokens or two operators),
  its SHA-256 and an anchor. The case's `fixtures` list holds
  `kind:callee@sha256:<digest>` in execution order: run-once (`before`/`beforeAll`) hooks outer to
  inner, then per-case hooks outer to inner, then the body in source order; a repeated identity
  keeps its first place. Any change to a seed name, URL, intercept argument or call order changes
  the list. A body precondition nested in a callback, conditional or template substitution is
  still recorded and adds a `nested-precondition` unknown. A hook the reader cannot fully bind (no
  inline callback, a call nested in a conditional, loop or callback, a non-statement call, or a
  recorded call that passes a callback or template substitution) gives every case under it a
  `hook-not-static` unknown. A hook at file root (`root-level-hook`) or in a dynamic scope
  (`hook-in-dynamic-scope`) is an unknown, not a precondition, and every case under it also has a `hook-not-static` unknown, as does every case in
  a scope with a hook call inside a template substitution. Canonical text writes a line break that
  ends a restricted production (after `return`, `break`, `continue` or `yield`, or before `++` or
  `--`) as the `;` ASI inserts; a backslash before U+2028 or U+2029 is a line continuation and a
  template's CR or CRLF cooks to LF. A case reaches `parity-candidate` only when every enclosing
  hook body holds nothing but allowlisted precondition statements (`DCP-V1-062`); any other hook
  statement, such as a helper call or a Cypress action, is a `not-allowlisted` unknown for every
  case under it, so a precondition the reader cannot bind is never silently omitted from
  `fixtures`. An awaited precondition, or one in an `async` or expression-bodied hook or case, is
  also `not-allowlisted`: the fixture identity does not record completion order.
- `DCP-V1-064`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) The record is pinned and deterministic: it names the repository identity and the
  full revision, reads only Git objects (never the working tree), writes nothing, and repeated runs
  on the same inputs give identical bytes.
- `DCP-V1-065`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) With `--input REGISTRY.json` (a closed behavior registry) the record adds a
  `crosswalk` with status `generated` and the registry's SHA-256. For each legacy case it lists at
  most five targets (tests and flows) ordered by score, then kind, then ID, each with its score and
  reasons: equal tuples, equal values, title-word overlap, equal preconditions, and required pages
  the case visits. Candidates are suggestions only: the producer never writes or emits a
  `legacy_criteria` relation or a review, and the same inputs give identical bytes.
- `DCP-V1-066`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) The crosswalk re-checks every existing reviewed `legacy_criteria` mapping in the
  registry against the fresh inventory. A `same` or `stronger` mapping whose target criterion no
  longer asserts the extracted legacy tuple is `refused` (`expected-value-differs` when matcher and
  locator agree, otherwise `observable-dropped`); the command still prints the full record and
  exits 1. A mapping to an undiscovered case or criterion is `unresolved`. Other relations, role
  requirements, disabled cases, cases with unknowns, a mapping criterion the target does not
  declare (`target-criterion-not-declared`), a mapping without a review at the registry's source
  revision (`mapping-review-missing-or-stale`), and a retained tuple whose target assertion lacks a
  reviewed annotation and behavior binding (`target-assertion-not-reviewed`) are `not-parity`. Only
  a mapping with none of these is `parity-candidate`, with the reason `runtime-baseline-required`.
- `DCP-V1-067`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) A target whose fixture identities differ from the legacy case's makes the mapping
  `not-parity`, naming `precondition-digest-differs`, `precondition-not-discovered`,
  `precondition-missing-from-target` or `precondition-order-differs`. Because the fixture identity
  embeds the digest, the existing behavior compiler reports the same change as a
  `legacy-precondition-mismatch` gap and never parity.
- `DCP-V1-068`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) The producer refuses with `corpus-refused` an unknown runner, a revision that is
  not a full commit ID, a non-canonical, absolute or parent-traversing `--path`, a path absent at
  the revision, a directory without test files, a non-regular blob (selected alone or as a
  test-named entry of a selected directory), an open or malformed registry, a registry that is not
  a schema 2 registry with a pinned contract and full revisions or that has invalid or duplicate
  flow, source behavior, test, fixture, role or mapping identities (flows, source behaviors and
  tests share one identity namespace, as in the behavior compiler; a mapping is identified by
  its criterion, legacy case and legacy criterion), and an inventory over
  `MaxRecords`. A file the lexer cannot classify (for example JSX, invalid UTF-8, an unterminated
  literal, a legacy octal escape, a raw line break in a string or regex, or a `/` after `}` whose
  regex or division reading depends on statement context) is a `lex-failed` unknown, a file over `MaxBytes` is `file-over-bound`, and a
  file without static cases is `no-static-cases`; none of them invents a case.
- `DCP-V1-069`: (accepted under the owner's standing auto-accept (2026-10-11) after Codex review PASS round 11; GitHub #811) `corvint docs --help` lists the `discover-legacy` usage and describes its output,
  the generated crosswalk, and the exit 1 on a refused mapping. The CLI prints exactly the
  producer's bytes, exits 1 with the record on a refused mapping, and exits 2 with the named
  error for a `DCP-V1-068` refusal, a missing required option or an option the operation does not
  take.

- `DCP-V1-070`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #812)) `corvint docs corpus import-cases --format testrail-json|csv --input EXPORT` reads
  one caller-exported test-management file, a root-relative local input of at most 32 MiB and 4096
  cases. `testrail-json` is the `{suites, sections, cases}` document the TestRail API returns; members
  Corvint does not read are ignored. `csv` needs a header row with `ID` and `Title`, and reads
  `Suite`, `Section`, `Section Hierarchy` (` > ` separated), `Preconditions`, `Steps`,
  `Expected Result`, `Steps (Step)` and `Steps (Expected Result)`; a row with an empty `ID`
  continues the previous case's separated steps. Each case is joined with its suite and its section
  chain, root first, into one path. The output is `corvint-legacy-cases/1`, which names the source
  format, path, sha256 and case count, is deterministic, and is sorted by case ID. The command writes
  nothing and contacts no service.
- `DCP-V1-071`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #812)) Every imported case is a `BehaviorLegacyCase` in the existing legacy inventory shape,
  keyed by its case ID normalized to `C` plus 1..10 digits. `suite` is the path joined by ` > `,
  `case` is the title, `state` is `manual`, and fixtures, roles and criteria are empty with no
  runtime. Its evidence anchor names the export path and sha256, the region `case:<ID>`, the case
  digest as the span digest, authority `external-provider` and kind `imported`. The anchor has no
  repository or revision, so a behavior registry does not admit it until the caller binds it to a
  committed revision; the record states that limit.
- `DCP-V1-072`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #812)) Preconditions, steps and expected results have HTML stripped before use: tags and
  comments are removed, `script` and `style` bodies are dropped, entities are decoded and whitespace
  is collapsed. A tag ends at the first `>` outside a quoted attribute value, and tag names match
  whole and ASCII case-insensitively on the original bytes (`scripture` is not `script`). An
  unterminated tag, comment or `script`/`style` body drops the rest of the field. A `<` that does
  not open a tag is kept as text. Titles and path elements are entity-decoded and
  whitespace-collapsed.
- `DCP-V1-073`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #812)) `--content` is `ids-paths-titles` (default) or `full`. Under `ids-paths-titles`, no
  precondition, step or expected-result text appears in any import or search output. Each case
  still carries a `digest`, the sha256 of its full stripped and bounded content, so a step edit
  changes the digest. `full` adds the stripped preconditions and steps and sets `local_only: true`,
  and its limitations say to keep the record out of commits.
- `DCP-V1-074`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #812)) Oversized fields are bounded, never silently truncated. A title over 1024 bytes, a
  path element over 512, preconditions over 8 KiB, step content or expected result over 4 KiB, or
  more than 64 steps is cut at a UTF-8 boundary. The case's `bounded` list then names the field,
  its stripped size and the bytes or steps kept. A path deeper than 32 sections is refused.
- `DCP-V1-075`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #812)) `corvint docs corpus search-cases --format F --input EXPORT --query TEXT [--limit
  1..256]` searches the bounded, stripped title, suite/section path, preconditions and step content
  of every case. A case matches when every query word (a lowercase run of letters or digits, 1..32
  words, at most 1024 bytes) is a prefix of a word in one of those fields. A result is the case
  ID, path, title and the fields that matched, and never step or expected-result text. Results are
  sorted by case ID and truncated to the limit, with the full match count in `total`. A case whose
  feature words appear only in its section path is found.
- `DCP-V1-076`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #812)) Every search result is the auditable receipt `corvint-legacy-case-search/1`. It
  holds the query, its words, the match rule, the searched fields, the searched source (format, path,
  sha256 and case count), `state` `found` or `none-found`, and the cases whose searched text was
  bounded. A `none-found` therefore states what was searched and what was not read. Expected results
  are not searched, and the receipt says so.
- `DCP-V1-077`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #812)) The separate `corvint-corpus-mcp`, started with `--legacy-cases`, lists the read-only
  tool `corvint.docs_search_cases` with the closed arguments `format`, `input`, `query` and
  `limit`. The tool returns the CLI's exact `search-cases` bytes framed as untrusted repository
  data, with the receipt as structured content. Without the flag, the tool list is unchanged.
- `DCP-V1-078`: Each of the following is refused with `corpus-refused` and no partial output:
  - an unknown format or content policy;
  - a malformed document;
  - an unknown section or suite, including a suite named by any section in a case's chain, or a
    section cycle;
  - a duplicate or invalid case ID, or a case without a title;
  - a CSV without `ID` and `Title`;
  - an empty export, or one over the case bound;
  - a query without words.

  An option that does not apply to the operation is an argument error. A non-canonical input path
  keeps the `DCP-V1-054` refusal. Status: accepted under the owner's standing auto-accept (2026-10-11; GitHub #812).
- `DCP-V1-079`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #812)) Manual legacy cases never count as runtime parity. `manual` joins `executable` and
  `disabled` as an admitted legacy state. A legacy case that is not `executable` has no runtime
  baseline, so a contract mapped to it reports the `legacy-runtime-unknown` gap and no runtime
  parity. Every import record states `runtime_parity: false`.
- `DCP-V1-080`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #835; V1-1222)) `corvint docs --help` and `corvint docs corpus witness-playwright --help` state the
  behavior annotation contract that `witness-playwright` reads (`DCP-V1-050`). The four annotation
  types are `corvint-behavior-event`, `corvint-behavior-cleanup`, `corvint-behavior-fixture` and
  `corvint-behavior-role`. The help names every event key, the sequence, `passed`, non-empty and
  `kind:id` uniqueness rules, the 4096-event bound, the kinds, the navigation values and the
  `parent_page` rule for a popup. It also names the one-`passed`-cleanup rule, the fixture and role
  order, the rule that the events equal each registered flow's `ordered_events` and the
  registered assertion tuples, the annotation-stage reasons, the producer and a pointer to the
  guide walkthrough. `discover-playwright --help` does not print it. `docs/DOCUMENTATION-CORPUS.md`,
  section "Playwright behavior witness walkthrough", walks the chain from the qualified receipt
  producer and the separate caller-run JSON report run, through `discover-playwright` and the
  evidence commit to `witness-playwright`, and back
  to `behavior-adapter`. It includes a marked sample REQUEST.json whose receipt is the observed
  `external-provider` full-file `inputs[]` entry (V1-1222). A test closed-decodes that sample, and
  imports a witness through a real request whose receipt input carries the sample's anchor
  metadata.
- `DCP-V1-081`: `internal/doccorpus/behavior-events.cjs` is a Playwright fixture producer. It
  publishes the four annotation types at test teardown, and it records only what ran:
  - An `assertion` event is recorded only after the genuine Playwright `expect` matcher resolves.
    That `expect` comes from the project's own `playwright/test` or `@playwright/test`, never from
    the caller. The test attempt must also have gained no error while the matcher ran, and the
    recording session must still be open.
  - The event's `matcher`, `locator` and `value` are the called method, the Locator's selector and
    the literal expected argument. They are therefore the registered tuple byte for byte.
  - Only a closed set of exact string matchers, a non-negative integer `toHaveCount` and the
    valueless state matchers of `DCP-V1-083` are recorded, with no option but `timeout`, on a real Locator in the main frame of an observed
    page. A recorded matcher receives a fresh options object holding only an own `timeout`, so an
    inherited or non-enumerable option never reaches it. Any other assertion still runs, but records nothing and warns once.
  - A `negative-control` event is recorded only after a `to...` matcher, including `.not`, passes.
  - A `page` event is recorded only for a real main-frame commit of an http(s) URL, observed from
    Playwright's own `framenavigated` event in the `context` fixture. Its `id` is the URL path.
    `navigation` is `popup` for the first commit of a page opened by an observed page, `redirect`
    for a redirected request, and `main-frame` otherwise.
  - The recorder is private and its API is frozen. `corvint-behavior-*` annotations published by
    anything else, or more than 4096 events, replace the events with one malformed event, so the
    test is never witnessed.
  - Cleanup callbacks run last-registered first. The producer publishes `passed` only when at least
    one callback ran and every one returned, `failed` when one threw, and nothing when none was
    registered.
  - Live Playwright 1.61.1 annotations equal the committed fixture report byte for byte and import
    to a witness. Status: accepted under the owner's standing auto-accept (2026-10-11; GitHub #835).
- `DCP-V1-082`: (accepted under the owner's standing auto-accept (2026-10-11; GitHub #835)) An unwitnessed test with an annotation-stage reason (`events-unobserved`,
  `event-malformed`, `cleanup-unobserved`, `cleanup-failed` or `fixture-role-mismatch`) adds
  `missing_annotations`. It lists the needed annotation types that the passing attempt never
  published, in the closed order `events`, `cleanup`, `fixture`, `role`. `events` and `cleanup` are
  always needed. `fixture` and `role` are needed only when the test registers fixtures or roles.
  The field is omitted when nothing is missing, and for every other reason. Reasons, details,
  admission and the witness wire are unchanged, so existing bundles without the field keep their
  bytes.
- `DCP-V1-083`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #845; V1-1318)) `behavior.expect` in `behavior-events.cjs` also records the closed set of valueless
  state matchers `toBeVisible`, `toBeHidden`, `toBeEnabled` and `toBeChecked`. Each is recorded with
  its canonical state name as the event `value`: `visible`, `hidden`, `enabled` and `checked`. As for
  the other matchers, the event is recorded only after the genuine matcher passed under the
  `DCP-V1-081` conditions, and only when the call has no argument or one options object whose only
  own enumerable key is `timeout`. The matcher then receives a fresh object holding only that
  `timeout`. Every other form still runs the matcher but records nothing and warns once. That covers
  a state option (`visible`, `enabled`, `checked`, `indeterminate`), even when it restates the
  default, a non-object argument and extra arguments. `behavior.expect` offers no `.not`, so a
  recorded state event always means the positive state held; a hidden element is asserted with
  `toBeHidden`, and a negated state belongs in `negativeControl`.
- `DCP-V1-084`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #845; V1-1318)) A reviewed expected outcome and a declared test assertion for a valueless state
  matcher state the same `matcher` and canonical `value` as `DCP-V1-083`, for example `toBeVisible`
  with `visible`. `behavior-adapter` compares the runtime assertion event with the reviewed tuple
  byte for byte, as for every other matcher, so an equal tuple keeps the variation's runtime
  witness. Any other non-empty value, such as `true`, never equals the recorded event and reports
  `assertion-contradiction` and `missing-assertion-event`. An empty reviewed outcome or declared
  check value is not compared at all: the adapter refuses it as an invalid declaration, as for
  every matcher. The adapter, the witness wire and every reason code are unchanged.
- `DCP-V1-085`: (accepted under the owner's standing auto-accept (2026-10-11) (V1-1380)) The
  canonical-form checks of `doccorpus.ParseArtifact` and `corpusindex.Open`, and the digest that
  `doccorpus` computes over an encoded value, stream the deterministic encoding through a comparer or
  hash instead of materializing it, and the re-encodes `corpusindex.Open` keeps are built in a buffer
  pre-sized from the input. No whole encoded output is buffered by the json/v2 encoder pool, which a
  GC cycle, a P migration or race instrumentation empties, so allocation follows the input size; the
  encoder's working buffer still grows to the largest single token, and only that part can depend on
  pool state. The encoded bytes, digests, output bounds and refusal messages are unchanged: a value
  whose encoding exceeds its bound is still refused with `output bound exceeded`. The `DCP-V1-040`
  1 GiB ordinary allocation ceiling and its race guard are unchanged.

- `DCP-V1-086`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1162;
  V1-1868)) A `records` provider may supply reviewed test knowledge from any test framework as
  one closed, versioned record `corvint-corpus-test-knowledge/1` that sets `test_knowledge` and no
  subject, claim, relation, journey, observation, capability, detail, restricted-finding or behavior
  member. The record requires corpus manifest `/2`. `test_knowledge` holds the pinned source
  `revision` and 1 to 4096 `records`, and each record holds exactly: a provider-qualified unique
  `id`; `kind` (`unit`, `request`, `model`, `component`, `integration`, `end-to-end`,
  `legacy-suite` or `test-case-export`); `framework`; `title`; the `test` anchor (repository, file,
  lines, blob and digests); 1 to 64 exercised `sources` anchors; the `entry` boundary and `actor`;
  `setup`; `stubs`, each with a `boundary` of `external` or `in-app`; the demonstrated `behavior`;
  `asserted` values; related `flows` (flow or variation IDs); `verdict` (`bound`, `partial`,
  `unrelated` or `defect`); `gap`; `docs` consistency (`consistent`, `contradicts` with the named
  `claim`, or `undocumented`); `trust` (`run+reviewed`, `run-unreviewed`, `read-only` or
  `legacy-reference`); an optional run `receipt` anchor; and `reviews`. Unknown members, unknown
  kinds, verdicts, docs states, stub boundaries or trust levels, duplicate IDs and any other schema
  are refused with `corpus-refused`, and no artifact is produced.
- `DCP-V1-087`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1162;
  V1-1868)) Field rules are fail-closed. A `gap` is present exactly when the verdict is
  `partial`; a `docs.claim` is present exactly when the state is `contradicts`; a `receipt` is
  present exactly when the trust level is `run+reviewed` or `run-unreviewed`; and `reviews` is
  non-empty exactly when the trust level is `run+reviewed`. Every free-text member (framework,
  title, entry, actor, setup, stub names, behavior, asserted values, gap, reviews and anchor
  reasons) is bounded UTF-8 text and passes the shared secret-shaped text scan; a match refuses the
  record. Lists are bounded at 64 entries and 16 reviews, and flow IDs are unique.
- `DCP-V1-088`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1162;
  V1-1868)) Records are pinned. Every anchor names the manifest repository, the
  `external-provider` authority, a closed evidence kind, a canonical path, a Git blob, a content
  SHA-256 and a non-empty line span with its span SHA-256, and carries no symbol or region. The
  `test` and `sources` anchors carry the set `revision`. A `receipt` anchor names the revision
  and path of the run receipt input it cites.
- `DCP-V1-089`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1162;
  V1-1868)) The build joins each record after every provider is imported. A set whose `revision`
  is not the corpus revision, whether it names another commit or none, is `stale` and none of its
  records joins, counts or is cited. Otherwise each `test` and `sources` anchor is compared with
  the declared input for its path at the corpus revision: an undeclared path makes the record
  `unanchored`. Both are reported with reasons and the record is not joined. Any other mismatch is
  a forged binding and is refused: a different blob, or an equal blob with a different content
  digest, span digest or a span beyond the file. A `receipt` is compared the same way with the declared input at
  its own revision; an undeclared `receipt` makes the record `unanchored` and any mismatch is
  refused. A `joined` record lists, sorted: the claims whose
  evidence anchors share a path and overlap the lines of its test or source anchors; its declared
  flows that resolve to `flow` or `variation` subjects, with the rest under `unresolved_flows`; and
  its test and source paths. The join adds one `test_knowledge` artifact member and never rewrites
  a subject, claim, relation, journey or observation record or its evidence.
- `DCP-V1-090`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1162;
  V1-1868)) Trust is labelled, never promoted. A join reports its `trust_level` and
  `counts_toward_coverage`, which is true only for a joined `run+reviewed` record with verdict
  `bound` or `partial`. Its `selection` is `blocks` for a joined `defect` at any trust level,
  `candidate` exactly when it counts toward coverage, and `excluded` otherwise. No trust level marks
  a claim or flow verified: journey status, evidence trust and the verified-journey metric are
  unchanged, and every join carries that limitation.
- `DCP-V1-091`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1162;
  V1-1868)) The native reader, and therefore the corpus MCP tools, the indexed companion and the
  HTTP companion, expose joins labelled by trust. `get` returns a join by ID in any status, and
  `inventory` pages every join. `flow` returns the joined records naming the flow. `claims` and
  `locate` return the joined records whose test or source path is the queried path; while any join
  exists, `get`, `locate`, `claims` and `recommend-tests` are answered even when their generic
  capability is absent, with a limitation saying so. `coverage` reports
  `flows_with_reviewed_test_knowledge` over the flow subjects, plus one row per trust level with
  its joined and counted record counts and no record IDs. `gaps` reports `stale-test-knowledge`,
  `unanchored-test-knowledge` and `unresolved-test-knowledge-flow`, and a flow's `no-tests` gap is
  cleared only by a counted record. `recommend-tests` adds `candidate` records for the path; a
  joined `defect` on the path withholds every candidate, lists at most 32 defect IDs in
  `blocked_by` with their total in `blocked_by_count` and `blocked_by_truncated` when more exist,
  and adds a missing-evidence line. The selection state stays `full-relevant-suite-required` and narrowing
  stays disallowed.
- `DCP-V1-092`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1162;
  V1-1868)) The join is deterministic: joins are ordered by ID and the artifact re-derives
  byte for byte under `Open`. Aggregate test-knowledge records are bounded at 4096 per corpus, and
  each readable join stays within the 1 MiB record read bound. Republication parity compares each
  join by ID through a streamed digest, so member size never reports a change and an encoding
  failure is an error, not a change. No database, service or network input is added.
- `DCP-V1-093`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1225; V1-1973)) A `corvint-corpus-flow-relations/1` provider record carries only a
  closed `flow_relations` set: a full Git `revision` and 1 to 4096 `records`. The provider needs
  manifest `/2`, carries no other provider member, and a shard admits the member only under its
  explicit schema. Record IDs are unique and qualified by the provider ID. Unknown members, enum
  values and duplicate IDs refuse with `corpus-refused`.
- `DCP-V1-094`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1225; V1-1973)) Each record has a declaring `flow`, a `kind`, a `review` state of `reviewed` or
  `unreviewed`, and an optional `source` anchor. The kind fixes exactly one target member:
  `depends-on` carries `target`, another flow; `produces` and `consumes` carry `name`, an event or
  job name; `external` carries `system`, an external system name. A missing or extra target member,
  a `depends-on` on its own flow, or secret-shaped text refuses. A source anchor is pinned to the set
  revision with the test-knowledge anchor pin rules.
- `DCP-V1-095`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1225; V1-1973)) The join runs after every provider is imported and keeps a separate
  `flow_relations` artifact member ordered by ID. A set at another revision is `stale`. A record
  whose flow, or `depends-on` target, is not a flow or variation subject is `unresolved`. A source
  anchor on an undeclared path is `unanchored`. A source anchor whose bytes differ from the declared
  input refuses. A record is `traversable` only when it is `joined` and `reviewed`; an unreviewed
  record keeps the reason `unreviewed: declared relation awaits review`.
- `DCP-V1-096`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1225; V1-1973)) `impact` and `affected` with `--corpus` add a `flow_relations` member,
  `corvint-corpus-flow-trace/1`, whenever the corpus has flow relations. Seeds are the flow and
  variation subjects whose evidence anchors a changed path, plus the declaring flow of a joined
  relation whose source anchor is a changed path. The walk goes downstream: `B depends-on A`
  reaches B from A; `A produces J` with `B consumes J` reaches B from A through J; `A external S`
  reaches system S as a terminal. Each reached flow or system reports its depth, its node path
  (including the event or job name) and the relation IDs on that path; the first breadth-first path
  in sorted edge order wins. The trace is labelled `project-declared-corpus` with a limitation that
  it ranks below project authority; it never sets `narrowing_allowed` and never narrows required
  checks.
- `DCP-V1-097`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1225; V1-1973)) The walk is bounded to a configurable depth in flow hops: 1 to 16, default 3,
  set by `--corpus-depth=N` on `impact` and `affected` only (it requires `--corpus`). Edges beyond
  the depth set `depth_truncated` and add an unknown. A relation that would extend the walk from a
  reached node but is unreviewed or not joined is never traversed; it is listed in `withheld` with
  its ID, node, kind, review state, status and reason, and adds the impact unknown `unreviewed or
  unjoined flow relation reported, not traversed`.
- `DCP-V1-098`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1225; V1-1973)) The reader exposes the same relations. `trace` on a flow or variation subject
  appends the same flow trace from that flow; the native request member `depth` (1 to 16) applies
  only to `trace`, and the corpus MCP tool `corvint.docs_trace` accepts it. `get` returns a join by
  ID in any status, `inventory` pages every join, and `gaps` reports `stale-flow-relation`,
  `unresolved-flow-relation`, `unanchored-flow-relation` and `unreviewed-flow-relation`.
- `DCP-V1-099`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1225; V1-1973)) The join and the walk are deterministic: the artifact re-derives byte for byte
  under `Open`. Relations are bounded at 4096 per corpus; `reached` and `withheld` each list at most
  256 entries and the trace stays within 512 KiB, with a limitation naming any withheld page.
  Republication parity compares each join by ID. No database, service or network input is added.

- `DCP-V1-140`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1223;
  V1-1971)) A symbol-named anchor (`path:START-END#Symbol@REV`) is re-resolved in the live tree.
  The pinned blob must hold exactly one declaration of the symbol inside the span. Its region is
  compared by a normalized digest: comments, blank lines, indentation and other whitespace outside
  literals are dropped, and literal and heredoc text is kept byte for byte. The live symbol is
  looked up in the same path first, then in the other indexed paths of the same repository and
  language family. The state is `live` (same lines and digest), `moved` (same digest at other
  lines or another path, with the live range), `changed` (the symbol is found in the same path but
  the digest differs, with an `estimated` live range) or `gone` (the symbol is absent from a
  complete inventory). Ruby and TypeScript/JavaScript reuse the index's Ruby scanner, web lexer and
  symbol extents; no new grammar is added.
- `DCP-V1-141`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1223;
  V1-1971)) Every live reader (`docs corpus` queries, the corpus integrations and the corpus MCP
  tools) reports each cited symbol anchor under the receipt's `symbol_anchors`, beside the
  unchanged artifact `freshness`. Each row names the anchor, its state, a reason, both normalized
  digests and the live location. A reader with no live tree (the indexed companion and `Query`)
  omits the member. Re-resolution reads Git objects only, at most 64 candidate paths and 4 MiB per
  file under the existing read budget, and writes nothing.
- `DCP-V1-142`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1223;
  V1-1971)) Missing evidence is `unresolved` with a reason, never a guessed state: another
  language, another repository, a working-tree change to the anchored or a candidate path, a
  pinned or live file that does not parse or normalize, several declarations or matches, a
  same-name symbol elsewhere whose region differs, and an inventory gap (an unparsed, excluded or
  noted file of the same family) that could hide the symbol. A cited symbol anchor the reader did
  not evaluate is `unresolved`; when re-resolution fails, every cited symbol anchor is unevaluated
  and the receipt carries the failure as a limitation instead of refusing the read.
- `DCP-V1-143`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1237;
  V1-1982)) The corpus MCP server starts from the configured artifact's digest and top-level
  `schema` and `capabilities` alone: tool advertisement needs no rederivation, and an artifact whose
  header cannot be read still fails startup with `corpus-unavailable`. The first tool call opens
  and rederives the artifact (`DCP-V1-011`) and keeps it; a later call reuses it only while the
  artifact bytes still match the startup digest and every pinned revision still resolves to its
  tree, and otherwise rederives. A rederivation failure is that call's typed failure. Each result
  reports `load` as `first-call` or `reused` with the load time.
- `DCP-V1-144`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1236;
  V1-1982)) Live readers re-resolve only the symbol anchors cited by the records a query returns
  (`DCP-V1-141`), so an uncited anchor costs nothing, and the corpus MCP server reuses one call's
  live freshness, including those re-resolutions, while the repository observation (object
  format, commit, tree, status digest and dirty paths) is unchanged. The reuse key is the state
  the freshness was built from, never an earlier observation, so a change during a call cannot be
  reused. Each call re-resolves under its own budget; parsed sources shared between calls are
  charged to that budget as if read. Reuse, including concurrent calls, never changes a receipt
  byte relative to a fresh reader.
- `DCP-V1-145`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1236;
  V1-1982)) Path tools accept exactly one of `path` and `paths`, a duplicate-free list of 1 to 100
  paths. A batch answers with `corvint-corpus-batch/1`: per path, in request order, the receipt
  the single-path call returns (without its trailing newline), or an `unresolved` row naming the
  path and code. A batch always fits one MCP message: a receipt that would push the result frame
  past the 1 MiB message bound is withheld as `output-bound-exceeded`, so the caller asks for those
  paths again, never receives a transport error. A call that outlives the server's per-call deadline returns `deadline-exceeded`
  (a batch names the unanswered paths instead, including when the deadline passes while the
  artifact or freshness is still being prepared), and the server keeps serving later calls. Every
  result carries `_meta["corvint/timing"]`: load, verify, freshness, query and total milliseconds
  and process CPU milliseconds.
- `DCP-V1-146`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1242;
  V1-1983)) A native Markdown paragraph is a verbatim blank-line excerpt. A paragraph over the
  64 KiB excerpt bound is split at line boundaries into consecutive verbatim excerpts, each within
  the bound and identified by its first line; joined in order they reproduce the paragraph byte
  for byte, and it is never truncated. A single line over the bound is a `not-collected` gap on
  its file subject naming the line and the bound, and the rest of the file and corpus still build;
  the file excerpt anchors the first non-blank line within the bound, and a file with no such
  line is a `not-collected` gap with no claims. A record-bound or anchor refusal names the path
  and line span it concerns; a `corvint-corpus-input/1` record-bound refusal also says a `/2`
  manifest raises the bound. Corpora that built before this requirement are unchanged.
- `DCP-V1-150`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1245;
  V1-1988)) A flow reports no asserting E2E gap exactly when one of its relations carries a
  `test_link` with role `asserting`, join confidence `exact`, evidence state `supported` and
  evidence trust `reviewed`. Provider import never grants `reviewed` trust (`DCP-V1-039`), so only a
  Core-authenticated review path clears the gap.
- `DCP-V1-151`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1245;
  V1-1988)) When the flow's only unmet condition is that a link's review is provider-reported
  (`reported_trust: reviewed`, trust `generated`), `gaps` reports the distinct, weaker kind
  `asserting-e2e-tests-unauthenticated-review`, whose reason states that the review is attributed,
  not authenticated, instead of `missing-asserting-e2e-tests`. Adequacy remains unknown.
- `DCP-V1-152`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1245;
  V1-1988)) Otherwise the flow reports `missing-asserting-e2e-tests`, whose reason names the failed
  condition: `no test link`, or the closest link's failures in the fixed order `not asserting`,
  `not exact`, `not supported`, then `review not authenticated` or `not reviewed` (the first link
  in relation-index order wins a tie). The gap kinds stay open strings; artifacts are unchanged.
- `DCP-V1-165`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1246; V1-1989)) `search-cases
  --id ID` and the `corvint.docs_search_cases` argument `id` take a case ID as `C12345` or `12345`
  (the `DCP-V1-071` ID form) instead of a query and return exactly that case, with `matched_fields`
  `["id"]`; an ID no case holds is `none-found`. The receipt then carries `id`, `mode` `id`, the
  searched field `id` and an empty word list.
- `DCP-V1-166`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1246; V1-1989)) `--match any`
  (argument `match: "any"`) ranks instead of requiring every word. A fixed English stop-word list is
  dropped from the query in this mode only and listed under `stop_words_dropped`; a query of only
  stop words is refused. A case matches when at least one remaining word prefixes a word of a
  searched field; each such word scores the best weight of the fields it matched (title 4, path 2,
  preconditions 1, steps 1). Results are ordered by score descending, then case-ID order, each hit
  carries `score`, `total` counts every matching case, and the receipt carries `mode` `any` and a
  match rule naming the weights.
- `DCP-V1-167`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1246; V1-1989)) A query that is
  exactly one `C<digits>` word in the `DCP-V1-071` ID form, with no other character, is the
  `DCP-V1-165` lookup when a case holds that ID, keeping the query in `query`. When no case holds it,
  the query is searched as before, so the result of every other query, and of that query against an
  export without that case, is unchanged.
- `DCP-V1-168`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1246; V1-1989)) `match` defaults to
  `all`, the `DCP-V1-075` rule. A default or explicit `all` search emits the same bytes as before
  this change: it carries no `mode`, `score` or `stop_words_dropped` member.
- `DCP-V1-169`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1246; V1-1989)) The tool schema adds
  `id` (pattern `^C?[1-9][0-9]{0,9}$`) and `match` (`all` or `any`) and requires only `format` and
  `input`. Exactly one of `query` and `id` is required, and `match` is admitted only with `query`;
  any other combination or an unknown `match` value is invalid params from the tool and
  `invalid-arguments` from the CLI, which admits `--query`, `--id`, `--match` and `--limit` as
  optional `search-cases` options.
- `DCP-V1-160`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1241;
  V1-1985)) `docs corpus manifest --schema 2` emits a `corvint-corpus-input/2` manifest whose
  inventory bound is the /2 record limit (100,000); `--schema 1` or no `--schema` emits
  `corvint-corpus-input/1` bounded by 4,096 files. Any other `--schema` value is a usage error.
  Without `--schema` and with one `--scope`, the output is byte-identical to the single-scope
  command it extends.
- `DCP-V1-161`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1241;
  V1-1985)) `--scope` may repeat; the command emits one manifest with one `scopes` entry per path,
  sorted by byte order whatever the argument order, and every tracked file of every scope as an
  input. A duplicate scope, or a scope inside another requested scope, is refused
  (`corpus-refused`) naming the paths, so every input belongs to exactly one scope.
- `DCP-V1-162`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1241;
  V1-1985)) Inventory bound refusals are typed `corpus-refused` and raised before any input is read:
  an empty scope names the scope; an inventory over the schema's record bound names the scopes,
  the tracked-file count, the schema and its bound, and a /1 refusal also names the /2 bound.
- `DCP-V1-163`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1241;
  V1-1985)) A multi-scope /2 manifest from this command is accepted unchanged by
  `docs corpus build`.

## Input and authority boundary

### Experimental behavior contracts (issue 40)

The opt-in `corvint-corpus-behavior-provider/1` record retains the normal provider fields and adds
`behavior_contracts`, a closed schema-2 registry. This is a synthetic interoperability candidate;
the consuming repository's actual `docs/migrations/test-behavior-contracts.json` and migration
manifest have not been supplied or qualified. No exact compatibility claim is made.

The registry pins contract ID/digest, source and documentation revisions, a full-file migration
manifest anchor, flows, source-discovered behaviors and exact Playwright test/project executions.
The schema-2 migration manifest contains `schema`, `contract_id`, `source_revision`,
`documentation_revision` and `revisions`. The latter pins repository IDs and commits for `app`,
the legacy /1 member and `docs_corpus`; the registry, migration manifest, live discovery and runtime
must agree with the corpus manifest's caller-supplied `behavior_revisions`. Any changed member blocks recorded
verification. External repository expectations remain caller-declared; local anchors still rebind
through immutable Git. Each flow retains its derivation, documentation anchor, criterion IDs, exact
test IDs, required page IDs, negative-control IDs and ordered complete event identities. Generated
prose remains generated even when its separately recorded runtime witness verifies.

Assertions bind stable behavior/criterion IDs to an exact reviewed annotation span/digest in the test
source and exact matcher, locator and expected value. Runtime assertion events must match every one
of those identities, not merely the matcher, route or test title. Every event names browser context,
page and frame; page events additionally distinguish main-frame, frame, redirect, popup (with parent
page) and setup navigation. Comparison preserves complete sequence and scope without flattening.

The registry's discovery anchor names a full `corvint-playwright-discovery/1` artifact with mode
`live-playwright-list`, the revision set, exact configuration anchor and discovered execution
IDs/projects/source anchors. The retained native run must use that configuration. The discovery
inventory supplies the project-execution denominator, including executions with no contract, which
remain unreviewed gaps. The issue comment's 463 executions in 117 files is a consumer observation,
not reproduced locally; synthetic tests assert their own bound denominator instead.
The contract SHA-256 uses canonical registry bytes with an empty digest and all target/legacy runtime fields
omitted. Runtime and provider artifacts are committed separately, avoiding self-referential hashes.

An optional runtime anchor names a full `corvint-behavior-run/1` artifact containing matching
contract/revisions, native receipt digest, test/project, retry, cleanup and ordered events. Its
observation joins a retained qualified Playwright receipt by exact `test_id` and `project` as well
as title. Recorded verification requires a passing native test projection, no run-level interruption,
pinned test/configuration bytes and source location, the same successful retry, passed cleanup, and
the complete declared event order including page,
assertion and negative-control observations. Synthetic fixtures test the join; they are not live
browser evidence. Provider honesty, assertion adequacy and runtime authenticity remain unknown.
Native run execution is not a passed-suite summary. Retained E2E app freshness remains explicitly
unknown even after source rebinding; only its exact `retained-app-build-identity-unverifiable` reason
is admissible for recorded verification. Stale bytes and all other unresolved freshness reasons
refuse. This status never asserts current served content or promotes the native freshness axis.

Contradictions, missing reverse links, stale source revisions, assertion-free tests, missing pages
and missing negative controls stay gaps. An empty inventory yields `unreviewed-join`; only a
current explicit review anchor with a nonempty discovered test inventory may emit the distinct
provider-reported `confirmed-missing_e2e` finding. Neither is proof of exhaustive absence.
Coverage exposes independent documented-flow, source-discovered-behavior, discovered-project-
execution and verified-contract denominators; zero remains undefined. All outcomes preserve full
relevant-suite fallback. Rollback removes this opt-in profile without changing legacy inputs.

The optional `legacy` inventory pins each suite/case ID and exact source file/revision/digest/span,
`executable` or `disabled` state, fixture/role preconditions, and extracted criterion IDs with
anchored matcher/locator/expected-value tuples. Every target criterion needs a reviewed
`legacy_criteria` relation to a specific case/criterion: `same`, `stronger`, `new`, `obsolete` or
`blocked`. Both same and stronger must retain the original observable tuple; stronger may add
assertions, never replace that result. Every legacy criterion needs a reverse mapping from a fully
eligible parity target (observable, runtime and preconditions all qualify), including
branches consolidated into one target test. Missing inventory/mappings stay unreviewed, never parity.

`legacy_runtime_parity` is separate from target `verified_tests`/journey verification. Only an
executable baseline with a pinned observed `corvint-legacy-behavior-run/1` witness can qualify: exact
contract/revision set, case/source digest, fixture/role preconditions, all extracted criteria,
native receipt/test/project/retry identity and passed cleanup. The supported qualifier is the
existing retained native Playwright receipt boundary, including source/configuration rebinding and
unknown served-app freshness; unsupported legacy runners retain unknown runtime baseline. Target
run preconditions must also match, and mappings must retain identical baseline preconditions.
New/obsolete/blocked classifications never count as runtime parity. Source extraction, generated
prose or product review cannot substitute for legacy execution; disabled/unavailable baselines emit
`legacy-runtime-unknown`. Exact consumer legacy input and live legacy execution are NOT_OBSERVED.
These remain external provider declarations, not semantic equivalence or authenticated execution.

Acceptance: `TestBehaviorContractCorpusRoundTrip`, `TestBehaviorContractGaps`,
`TestBehaviorOrderedRuntime`, `TestBehaviorQualifiedReceiptEndToEnd`, `TestBehaviorAcceptanceAmendment` and
`TestBehaviorExactProjectObservation`, `TestBehaviorLegacyParity` and
`TestBehaviorLegacyAndStabilityIntegration` exercise `DCP-V1-004`,
`DCP-V1-007..013` and `DCP-V1-019`. Owner acceptance and real consumer fixtures remain promotion gates.

### Experimental behavior-provider adapter (issue 53)

`corvint docs corpus behavior-adapter --input REQUEST.json [--previous RESULT.json] [--check]` is a read-only
producer for the issue-40 profile. The request supplies caller-owned documents and immutable input
identities plus generic mappings for reviewed flows and variations, source candidates, tests, live
Playwright discovery and runtime witnesses. Each canonical variation retains explicit semantic fields
and allowed project/test identities; tests name the same variation in a closed claim. Only outer record
and scalar/list fields are remapped. Closed nested wire objects are decoded unchanged so mapping cannot
manufacture an anchor, review, assertion, semantic claim or event sequence.

The result is a deterministic bundle rather than a self-extending provider record. It retains the
provider, the normalized normative variation projection, separately publishable migration, discovery
and runtime sidecars, a five-denominator coverage projection, a complete reconciliation frontier and
an incremental identity delta. Source/test proposals cannot enter or rewrite the normative projection.
The existing corpus compiler remains the validating and Git-rebinding boundary. The adapter's
`runtime-witnessed` row means only that mapped closed records and a native qualified-receipt document
join structurally; it is not `verified_tests`, current application freshness, authenticated runtime
or narrowing authority.

Acceptance: `TestBehaviorAdapterBuildOpen`, `TestBehaviorAdapterConformance`,
`TestBehaviorAdapterMappingParity`, `TestBehaviorAdapterDelta`,
`TestBehaviorAdapterReverseLinkKeysDoNotCollide`, `TestBehaviorAdapterMissingReverseLinkNamesEachTest`,
`TestBehaviorAdapterArtifactsMatchPreviousValidator` and
`TestBehaviorAdapterCLI` exercise `DCP-V1-027..032`. The check mode in `DCP-V1-044` is exercised by
`TestBehaviorAdapterCheckParity`, `TestBehaviorAdapterCheckReportsEveryRefusal`,
`TestBehaviorAdapterCheckIndependentRefusals`, `TestBehaviorAdapterCheckListsDependentChecks`,
`TestBehaviorAdapterCheckStopsAtFirstItemRefusal`, `TestBehaviorAdapterCheckBoundsReport`,
`TestBehaviorAdapterCheckCapsMessages`, `TestBehaviorAdapterCheckBoundsRetention`,
`TestBehaviorAdapterCheckRetainsNoFrontier`, `TestBehaviorAdapterCheckRetainsNoAmplifiedKeys`, `TestBehaviorAdapterBuildBoundMessage` and the check subtest of `TestBehaviorAdapterCLI`. The guide example in `DCP-V1-045` is exercised by
`TestBehaviorAdapterGuideMigrationExample`. Exact consumer inputs, live browser execution and external
utility qualification remain `NOT_OBSERVED`.

### Experimental Playwright discovery and witness producers (issue 717)

`discover-playwright` and `witness-playwright` produce the two caller-run inputs the behavior
adapter already checks, so an adopter need not hand-write them (`DCP-V1-046..052`). Both read files
the caller produced; neither runs Playwright, a browser or the application. `discover-playwright`
reuses the issue-709 listing validation (`typescript.PlaywrightListedTests`) so its refusals match
`corvint affected discovery`. `witness-playwright` emits a bundle, not a request: the caller retains
each witness document byte-for-byte, adds it as an observed input, names it as the test's runtime
evidence and appends the observation link; `behavior-adapter` then rechecks the join.

Limits: the listing, report and receipt are caller-declared; there is no `--project` filter, since
narrowing the listing would narrow the discovered denominator. The report digest is retained in the
bundle, not in the closed `corvint-behavior-run/1` wire, so the corpus compiler does not re-verify the
report. A qualified receipt is required for witnessing and for identities that the adapter's receipt
check accepts; its producer is `corvint-js-test-provider e2e --external-server`, and the chain from
that run to `witness-playwright` is in the docs help (`DCP-V1-053`). Only a first-attempt pass is credited. Fixture tests use a real Playwright 1.63.0
listing and report of a generic project with a synthetic qualified receipt; live browser execution
by Corvint and external utility qualification remain `NOT_OBSERVED`.

Behavior annotation producer (issue 835). The annotation contract is in the docs help and the guide
walkthrough (`DCP-V1-080`). `behavior-events.cjs` publishes the annotations from the test's own
Playwright run (`DCP-V1-081`), and an unwitnessed test names the annotation types it never published
(`DCP-V1-082`). The valueless state matchers `toBeVisible`, `toBeHidden`, `toBeEnabled` and
`toBeChecked` are recorded with their state name as the value (`DCP-V1-083`), which a reviewed
outcome states the same way (`DCP-V1-084`). Admission is unchanged: the importer still cannot tell producer annotations from
hand-published ones, so the producer narrows accidental and casual forgery, not a determined author
who replaces Playwright objects. The producer reads the private Playwright fields Locator `_frame`
and `_selector` and Page `_opener`. It is qualified live on Playwright 1.61.1 only, observes only
the `context` fixture, never produces `frame` or `setup` navigations, and publishes a URL path,
which may itself carry a secret, as the page `id`.

Acceptance: `TestPlaywrightDiscoveryProducerRoundTrip`, `TestPlaywrightDiscoveryProducerRefusals`,
`TestPlaywrightWitnessImporter`, `TestPlaywrightListedTestsMultiProject` and
`TestDocsCorpusPlaywrightProducersCLI`.

### Experimental test-management case import (issue 812)

`import-cases` and `search-cases` read a test-management export the caller produced, such as a
TestRail API dump or CSV export (`DCP-V1-070..079`). Corvint never calls the service. The import
yields `manual` legacy cases for review and search, not runtime baselines. Under the default
`ids-paths-titles` policy, the record and the search receipt can be shared, because only IDs, paths,
titles and digests leave the export. A `full` record is local-only.

Limits:
- HTML is stripped by a bounded tag scanner, not a full HTML parser.
- Expected results are not searched.
- Other TestRail custom fields are ignored.
- The legacy anchor is not admissible into a behavior registry until the caller binds it to a
  committed revision.
- The search reads one export per call.

Acceptance: `TestImportLegacyCasesFormats`, `TestLegacyCasesIDsPathsTitlesWithholdsSteps`,
`TestImportLegacyCasesFullStripsHTML`, `TestImportLegacyCasesBoundsOversized`,
`TestSearchLegacyCasesReceipt`, `TestLegacyCasesRefusals`, `TestBehaviorLegacyParity`,
`TestDocsCorpusLegacyCasesCLI` and `TestDCPV1077CorpusMCPSearchCases`.

### Experimental test-knowledge records (issue 1162)

A `corvint-corpus-test-knowledge/1` provider record carries reviewed knowledge about test examples
from any framework (`DCP-V1-086..092`). The corpus validates the closed record, compares every
anchor with the pinned corpus inputs, and joins current records to claims, flows and source paths
in a separate `test_knowledge` artifact member. Stale and unanchored records are reported, not
joined. Only a joined `run+reviewed` record with verdict `bound` or `partial` counts toward
coverage or becomes a test-selection candidate, a joined `defect` blocks selection, and no trust
level marks a claim or flow verified. Trust levels are provider-reported: the corpus does not
authenticate reviewers or re-run receipts.

Limits:
- Claims are joined by path and line overlap with a claim's evidence anchors, not by semantics.
- A generated per-flow "what the tests demonstrate" render section is not produced.
- A record joins only when its set is pinned at the corpus revision; an older set is stale.

Acceptance: `TestDCPV1086TestKnowledgeSchemaIsClosed`, `TestDCPV1087TestKnowledgeFieldRules`,
`TestDCPV1088TestKnowledgeAnchorsArePinned`, `TestDCPV1089TestKnowledgeJoin`,
`TestDCPV1090TestKnowledgeTrustRules`, `TestDCPV1091TestKnowledgeQueryExposure` and
`TestDCPV1092TestKnowledgeDeterministicAndBounded`.

### Experimental flow-level relations (issue 1225)

A `corvint-corpus-flow-relations/1` provider record declares relations between flows that no
source index derives (`DCP-V1-093..099`): `depends-on` another flow, `produces` or `consumes` an
event or job, and `external` calls to a named system. Each record has an optional source anchor
and a review state. `impact` and `affected` with `--corpus` walk reviewed joined relations
downstream from the flows a change touches, to `--corpus-depth=N` hops (default 3), and attribute
each reached flow to its relation path, for example `A -> job:J -> B`. An unreviewed, stale,
unresolved or unanchored relation is reported in `withheld` and is never traversed. `trace` and the
corpus MCP tool `corvint.docs_trace` walk the same relations from one flow. The trace is
project-declared corpus data ranked below project authority, and it never narrows required checks.

Limits:
- Event and job names match by exact string; the corpus does not resolve aliases or schemas.
- `corvint docs corpus trace` has no depth flag and uses the default depth; the native request
  member and the MCP tool take `depth`.
- Seeds come only from flow subject evidence anchors and joined relation source anchors.
- Review state is provider-reported; the corpus does not authenticate reviewers.

Acceptance: `TestDCPV1093FlowRelationSchemaIsClosed`, `TestDCPV1094FlowRelationKindFields`,
`TestDCPV1095FlowRelationJoinStatus`, `TestDCPV1096FlowRelationImpactAttribution`,
`TestDCPV1097FlowRelationUnreviewedAndDepth`, `TestDCPV1097CorpusDepthFlag`,
`TestDCPV1098FlowRelationTraceAndGaps`, `TestDCPV1098TraceToolDepth` and
`TestDCPV1099FlowRelationDeterministicAndBounded`.

### Multi-repository behavior declarations (issue 330)

`corvint docs corpus behavior-provider --input REQUEST.json` is a read-only producer for
`corvint-corpus-behavior-provider/2` (AFU-V1-006). The closed request has `schema` equal to
`corvint-behavior-provider-request/2`, `provider_id`, `provider_version`, `source` (repository ID
and revision), and `registry` (the existing BehaviorRegistry shape). `registry.revisions` is absent;
`registry.repositories` supplies root commits and revisions. `contract_sha256` must be empty;
the producer sorts unique repository members and computes the declaration digest, excluding runtime.
It never repairs a pre-signed registry. Other declaration order is preserved. Existing 4 MiB wire
and MaxRecords bounds apply, including aggregate anchors and events. Invalid identities, paths,
anchor shapes, conflicting root commits, and duplicate declarations refuse before output.

A committed provider enters the existing manifest `records` boundary. The optional manifest
`behavior_repositories` member is the caller's sorted unique expected repository/revision set;
it is independent of the provider record. Ingestion validates local immutable anchor bytes and
retains the original `/2` registry. Missing expectations, missing members, stale member/anchor
revisions, unavailable external repository bytes and unresolved joins become explicit corpus gaps.
Malformed records and forged local bindings refuse. External repositories are not opened, fetched
or trusted from caller paths. `/2` has no fixed three-repository conversion and cannot invoke `/1`
runtime reconciliation. Discovery, migration and runtime claims remain unqualified, every verified
set stays empty, and `full-relevant-suite` fallback is unconditional. Legacy/stability registries and
non-behavior provider record families are refused in this bounded `/2` producer/ingest profile.
Existing `/1` wire bytes and behavior remain unchanged; `/2` never emits the legacy /1 member (AFU-V1-055).

Acceptance: `TestBehaviorV2ProducerCorpusRoundTrip`, `TestBehaviorV2RepositoryGaps`,
`TestBehaviorV2RefusesMalformedOrConflictingRecords`, `TestBehaviorProviderV2CLI`, and the retained
`TestAFUV1BehaviorProviderV1BytesUnchanged`. Rollback removes the `/2` command/dispatch and optional
manifest member; `/1` remains available. No external runtime qualification follows from these tests.

### Experimental repeated Playwright stability evidence (issue 42)

The opt-in `corvint-corpus-behavior-stability-provider/1` record retains the issue-40 behavior
registry but adds a separate `corvint-playwright-stability/1` registry. Stability bytes are excluded
from the behavior-contract digest, and compiled stability reports are stored outside
`behavior_contracts`, so behavior coverage and repeated-run stability remain separately reviewable.

Each aggregate selects one of the repository policy's `one-spec`, `feature-batch` or `suite`
thresholds. Planned repetitions must be exactly present once; retries stay nested attempt evidence;
manual reruns carry distinct IDs and do not affect planned counts. Exact immutable receipt decoding
revalidates the test/project, runner, config, app/test revisions, environment and effective fixture
identity. Native run/test projections are recomputed, retry ordinals must be contiguous, and receipt
test/config digests must resolve through explicit source-path mappings at the declared test revision
to the exact behavior-test anchor. Policy-owned worker/retry/environment-class/fixture-schema labels
remain attributed declarations bound by the policy and receipt digests, not independent runtime
discovery.

Qualified `corvint-playwright-external/1` contributions bind the application revision through the
fully validated before/after application attestation, including a distinct application repository.
The `/0` profile retains its declared application identity and corpus-revision admission. Invalid
attestation or a contribution/attestation revision mismatch refuses aggregation; `/1` never falls
back to the empty legacy declared-identity field. `TestPlaywrightStabilityAttestedProfile` covers
same-repository and cross-repository positives and mismatched/invalid attestation controls.

The policy's declared execution topology and every contribution's observed topology are separate
canonical, full-file anchored inputs. Topology covers CI nodes/shards, Playwright workers per node,
database mode, project set, split algorithm/version and resource class; observations additionally
bind the exact receipt digest. All fields must match the repository policy before outcome counting.
The issue comment reports a consumer policy of six isolated CircleCI nodes and an active job with
`parallelism: 4`, but the exact consumer files and run artifacts were not supplied, so that concrete
contradiction remains `NOT_OBSERVED` here rather than being imported as evidence.

Counts intentionally overlap where facts overlap: a flaky repetition with a failed or timed-out first
attempt retains that earlier category, is also `flaky`, and consumes a retry. This prevents a later pass
from erasing its earlier failure. Failed or unknown planned/manual repetition or attempt cleanup prevents `clean`; malformed, omitted, duplicate,
stale, cross-revision and contradictory inputs refuse compilation. The aggregate retains receipt
paths/revisions/digests, every attempt classification and supporting receipt artifact. The dedicated
corpus/MCP join rederives those bytes and carries explicit no-adequacy/no-parity limitations.

Acceptance: `TestPlaywrightStabilityAggregateEndToEnd`,
`TestPlaywrightStabilityNegativeControls`,
`TestPlaywrightStabilityPreservesEarlierFailureAndPolicyScopes` and
`TestPlaywrightStabilityCountsEarlierTimeoutWithoutErasingRecovery`,
`TestPlaywrightStabilityCountsEarlierInfrastructureWithoutErasingRecovery` and
`TestPlaywrightStabilityRejectsDeclaredObservedTopologyMismatch`,
`TestPlaywrightStabilityTopologyIdentity`, `TestPlaywrightStabilityTopologyBindings` and
`TestStabilityToolIsCapabilityGated` exercise DCP-V1-021..026. Fixtures are synthetic; live repeated
browser execution, consumer policy qualification, behavior parity and owner acceptance remain open.

One local Git repository may supply up to eight explicit immutable revisions. A provider can be
committed after the source it describes; its record anchors still name the earlier source revision.
An input scope is a literal directory or exact file, with a complete declared inventory at its own
revision. A provider artifact is an input of purpose `provider`, never automatically source text.
The manifest can be generated by an explicit read-only inventory command and edited before build.
A profile is data, never code or a template interpreter. External/provider evidence remains attributed;
structural rederivation proves identity, not semantic truth, human acceptance or provider honesty.

Legacy `/1` bounds: 4 MiB per manifest/provider/artifact/receipt, 4096 input paths, 4096 subjects/claims/relations
per collection, 128 journeys with 128 steps each, 64 KiB per evidence excerpt, 1024-byte paths/queries,
256 results, 8 revisions and 16 providers. Whole-build output overflow refuses instead of truncating.
The opt-in adoption `/2` capacity and read projection limits are defined by DCP-V1-033..037.
Query limits disclose withheld rows. Unsupported analyzer input may be inventoried with an explicit
gap; explicitly requesting an unavailable provider implementation fails the build.

### Experimental legacy inventory producer and crosswalk (issue 811)

`discover-legacy` turns an existing Jasmine or Mocha/Cypress suite into the legacy inventory shape
the behavior registry already accepts (`DCP-V1-060..069`), so an adopter starts a migration from
evidence rather than a hand-typed list. It reuses the static tokenizer approach of the Playwright
producers: comment- and literal-aware, no external parser, and a file it cannot classify is
reported, not guessed. The inventory and crosswalk are generated suggestions. A reviewer still
writes every `legacy_criteria` relation, and the behavior compiler still needs a qualified
`corvint-legacy-behavior-run/1` baseline before any runtime parity.

Limits: helper, custom-command, support and fixture bodies are not followed (a precondition binds
its call text, not the seed file); roles are always empty; global support hooks and root-level
hooks in other files are not seen; an assertion or any statement other than an allowlisted
precondition inside an enclosing hook makes every case under it `not-allowlisted`; a call nested
inside a conditional, loop or callback in a hook is not recorded as a precondition, and every case
under that hook carries a `hook-not-static` unknown instead. The closed-world allowlist
(`DCP-V1-062`) over-refuses by design: a case that uses any helper call, Cypress action,
declaration, `await`, `async` or expression-bodied callback, function-calling or subject-changing
chainer continuation, `contains` option, ambiguous string value or other unlisted form is unknown,
never parity, and a case over the 4,096-criterion bound is unknown rather than truncated. Other
cases' bodies, hooks of
non-enclosing suites, getters on operand paths and custom matchers or commands overriding a known
name are not proven; cross-test state needs the runtime baseline.

Acceptance: `TestLegacyDiscoveryInventory`, `TestLegacyDiscoveryDeterministicReadOnly`,
`TestLegacyCrosswalk`, `TestLegacyPreconditionDigestCompileGap`, `TestLegacyDiscoveryRefusals`,
`TestLegacyDiscoveryStaticEdgeCases`, `TestLegacyLexerCanonicalAndEscapes` and
`TestDocsCorpusDiscoverLegacyCLI`.

## Failure modes

`corpus-refused` covers malformed/canonical/digest/closure/authority/span/bound failures.
`corpus-input-unavailable` covers missing, unsafe or oversized local inputs.

- `DCP-V1-054`: Every `corvint docs corpus` local input file (`--manifest`, `--artifact`,
  `--input`, `--previous`, `--cem`, `--retirement`, `--migration`, `--playwright-list`, `--receipt`,
  `--report`) is a canonical path relative to `--root` that sits under it: no leading `/`, no `..`
  component, no `.`, `./`, `//` or trailing `/`, at most 1024 bytes of UTF-8, and no backslash,
  NUL, tab, CR or LF. A nonempty value that breaks the rule is refused before any read with the
  unchanged code `corpus-refused` and error shape, and the message names the flag and the broken
  rule, for example `--migration: local input paths must be canonical and relative to --root (got
  an absolute path)`; the rule is one of an absolute path, a parent traversal, the root itself, a
  non-canonical path, an over-1024-byte path, a non-UTF-8 path, or a backslash, NUL, tab, CR or LF.
  Empty values keep their existing handling: an empty or whitespace-only required option is an
  argument error and an empty optional one counts as omitted. The bounded reader's own refusal keeps the prefix `invalid
  local input path` and names the same rule, including `got an empty path`. `corvint docs --help`
  states the root-relative rule for these flags. What the reader admits is unchanged; absolute paths
  stay refused. Acceptance: `TestDocsCorpusLocalInputPathRefusals`,
  `TestDocsCorpusHelpStatesRootRelativeInputs` and `TestLocalInputPathRuleNamesTheBrokenRule`.
  Status: (accepted, decision 0520; V1-1226; GitHub #796).
`corpus-provider-unavailable` covers undeclared or unsupported provider implementations.
A test-knowledge record refuses with `corpus-refused` for an unknown member or enum value, a
broken field rule, a secret-shaped free-text value, a malformed or unpinned anchor, a bound
breach, use under manifest `/1`, or a forged binding (any anchor mismatch at the corpus
revision). A set revision other than the corpus revision, or an undeclared anchored file, is not a refusal: the record is reported `stale` or `unanchored` and is
not joined (`DCP-V1-089`).
A flow-relation record refuses with `corpus-refused` for an unknown member, kind or review state,
a kind without exactly its own target, name or system, secret-shaped text, an unpinned or forged
source anchor, a bound breach or use under manifest `/1`. A stale, unresolved or unanchored record
is not a refusal: it is reported, listed in `withheld` when reached, and never traversed
(`DCP-V1-095`, `DCP-V1-097`). An invalid `--corpus-depth` or `depth` is an argument error.
`corpus-invalid-revision` covers an unsupported immutable source revision.
The separate MCP startup uses `corpus-unavailable` when its configured corpus cannot be read or
its header cannot be decoded; rederivation happens on the first call (`DCP-V1-143`), and call
failures retain the specific native error and never return successful empty evidence.

The experimental JSON HTTP companion returns `corvint-corpus-http-refusal/1` with
`state: abstained` for these DCP-V1-042 transport/query refusals:

| Code | HTTP status | Trigger |
| --- | ---: | --- |
| `unknown-operation` | 404 | Path is not `/query`, or the URL has a query string. |
| `read-query-post-required` | 405 | Request method is not POST; `Allow: POST` is returned. |
| `json-required` | 415 | Content-Type is not exactly `application/json`. |
| `concurrency-bound` | 503 | Both admitted request slots are occupied. |
| `invalid-query` | 400 | Bounded closed request decoding or operation-specific argument validation fails. |
| `query-refused` | 422 | The native indexed reader refuses the query. |

## Non-goals and simpler baseline

A manually authored Markdown page remains the baseline. No cross-repository federation, remote
provider transport, automatic observation collection, human-review authentication, semantic anchor
repair, inferred UI journeys, universal test adequacy, HDC/MkDocs qualification or accepted technical
intent is delivered by this experimental profile. No change to the default one-binary local boundary.
The optional MCP companion must be started explicitly and opens no network connection.
Test-knowledge records (`DCP-V1-086..092`) do not authenticate reviewers, re-run or re-verify run
receipts, judge assertion adequacy, rewrite claims, render a per-flow test section, or mark any
claim, flow or journey verified. Rollback of that slice removes the `test_knowledge` provider
schema, artifact member and reader rows; corpora without test-knowledge providers build the same
bytes as before.

## Acceptance and rollback

Build/read identity and two-commit source/provider proof precede integration. Focused tests cover
closed schemas, closure before admission, non-vacuous spans, hostile trust promotion, deterministic
bytes, native receipt joins, capability absence/zero, source/provider drift, symlinks, output bounds,
CEM base binding, native opt-in parity, MCP gating/parity and human-prose preservation. Run the
frozen labelled evaluation and real self-corpus proof, then independent integrated review and the
repository's applicable clean-commit checks and dogfood CEM/OCM/strict completion loop. Scoped issue
work uses focused checks under `AGENTS.md`; full release promotion retains its governing gate. Test
execution and pinned results belong in one `docs/build-log/` entry; tests never accept this spec. Promotion requires owner
acceptance and all scoped evidence; a failed bound/trust/parity case blocks completion. Rollback
removes the optional corpus entry points/packages/profile artifacts; existing native commands,
original sources, retained observations and human documentation require no migration.

## Traceability

| Requirements | Implementation boundary | Required evidence |
| --- | --- | --- |
| DCP-V1-001..011, DCP-V1-018 | `internal/doccorpus`, immutable `internal/contextindex` adapter | Determinism, input closure, anchor/state/receipt/staleness hostile fixtures; `TestLoadInputRefusesOverBoundBlobBeforeBody` (a bound+1 Git blob is refused from its header) |
| DCP-V1-012..015 | `cmd/corvint`, corpus projection API | CLI and native parity, CEM base/provenance and exact observation joins |
| DCP-V1-016 | `internal/mcp/corpusbridge`, `cmd/corvint-corpus-mcp` | Capability gating, transport parity and hostile text |
| DCP-V1-017 | Corpus render and maintenance API | Human byte/permission preservation, malformed/stale/tampered refusal |
| DCP-V1-019..020 | Conformance fixtures and independent example adapter | Labelled evaluation and actual self-corpus receipt |
| DCP-V1-021..026 | `internal/doccorpus/stability.go`, corpus reader and MCP bridge | End-to-end aggregate, policy scopes, prior-attempt retention, source rebinding, exact declared/observed topology, cleanup and adversarial controls |
| DCP-V1-027..032 | `internal/doccorpus/behavior_adapter.go`, `cmd/corvint/docs_corpus.go` | Mapped bundle through existing Build/Open, denominators, frontier/delta, bounds and issue-53 adversarial fixtures |

| DCP-V1-038..042 | `internal/doccorpus/typed_query.go`, `trust.go`, `query_index.go`, `internal/corpusindex`, `internal/corpusserve`, separate companion commands | `TestCorpusTypedQueryConformance`, `TestCorpusRetirementBindings`, `TestCorpusCompleteReadAndTypedToolSchema`, `TestIndexedCorpusReproductionAndProvenance`, `TestIndexedCorpusCapacityQualification`, `TestIndexedCorpusSwitchParity`, `TestHostedCorpusHTTPConformance`, `TestHostedCorpusConcurrencyAndCleanup` |
| DCP-V1-043 | `cmd/corvint-corpus-parity` | `TestCorpusParityCommandEndToEnd`, `TestCorpusParityCommandFailuresExitNonzero` |
| DCP-V1-044 | `internal/doccorpus/behavior_adapter_check.go`, `behavior_adapter.go`, `cmd/corvint/docs_corpus.go` | `TestBehaviorAdapterCheckParity`, `TestBehaviorAdapterCheckReportsEveryRefusal`, `TestBehaviorAdapterCheckIndependentRefusals`, `TestBehaviorAdapterCheckListsDependentChecks`, `TestBehaviorAdapterCheckStopsAtFirstItemRefusal`, `TestBehaviorAdapterCheckBoundsReport`, `TestBehaviorAdapterCheckCapsMessages`, `TestBehaviorAdapterCheckBoundsRetention`, `TestBehaviorAdapterCheckRetainsNoFrontier`, `TestBehaviorAdapterCheckRetainsNoAmplifiedKeys`, `TestBehaviorAdapterBuildBoundMessage`, `TestBehaviorAdapterCLI` (`DCP-V1-044 check`) |
| DCP-V1-045 | `docs/DOCUMENTATION-CORPUS.md` | `TestBehaviorAdapterGuideMigrationExample` |
| DCP-V1-046..048 | `internal/doccorpus/behavior_playwright_discovery.go`, `internal/liveverify/affected/typescript/playwright_listed_tests.go`, `playwright_discovery_list.go`, `cmd/corvint/docs_corpus_playwright.go` | `TestPlaywrightDiscoveryProducerRoundTrip`, `TestPlaywrightDiscoveryProducerRefusals`, `TestPlaywrightListedTestsMultiProject`, `TestDocsCorpusPlaywrightProducersCLI` |
| DCP-V1-049..052 | `internal/doccorpus/behavior_playwright_witness.go`, `cmd/corvint/docs_corpus_playwright.go` | `TestPlaywrightWitnessImporter`, `TestDocsCorpusPlaywrightProducersCLI` |
| DCP-V1-053 | `cmd/corvint/docs.go`, `cmd/corvint-js-test-provider/main.go` | `TestDocsHelpNamesPlaywrightReceiptProducer`, `TestDocsHelpProducerExampleUsesE2EFlags` |
| DCP-V1-056 | `cmd/corvint/docs_corpus_help.go`, `cmd/corvint/docs_corpus.go`, `cmd/corvint/help.go` | `TestDocsCorpusHelpListsOperations`, `TestDocsCorpusOperationHelpPrintsFlags`, `TestDocsCorpusOperationFlagMatrix` |
| DCP-V1-057 | `cmd/corvint/help.go` | `TestUnknownHelpTopicListsValidTopics`, `TestUnknownHelpTopicIsConventionalError` |
| DCP-V1-060..061 | `internal/doccorpus/behavior_legacy_discovery.go`, `behavior_legacy_lex.go` | `TestLegacyDiscoveryInventory` (`DCP-V1-060 DCP-V1-061`) |
| DCP-V1-062 | `internal/doccorpus/behavior_legacy_discovery.go`, `behavior_legacy_lex.go`, `behavior_legacy_allow.go` | `TestLegacyDiscoveryInventory` (`DCP-V1-062`), `TestLegacyDiscoveryStaticEdgeCases` (`DCP-V1-062`), `TestLegacyResidueFailsClosed` (`DCP-V1-062`) |
| DCP-V1-063 | `internal/doccorpus/behavior_legacy_discovery.go`, `behavior_legacy_allow.go` | `TestLegacyDiscoveryInventory` (`DCP-V1-063`), `TestLegacyDiscoveryStaticEdgeCases` (`DCP-V1-063`), `TestLegacyLexerCanonicalAndEscapes`, `TestLegacyResidueFailsClosed` (`DCP-V1-063`) |
| DCP-V1-064 | `internal/doccorpus/behavior_legacy_discovery.go` | `TestLegacyDiscoveryDeterministicReadOnly` |
| DCP-V1-065..066 | `internal/doccorpus/behavior_legacy_crosswalk.go` | `TestLegacyCrosswalk` (`DCP-V1-065`, `DCP-V1-066`) |
| DCP-V1-067 | `internal/doccorpus/behavior_legacy_crosswalk.go`, `behavior.go` | `TestLegacyCrosswalk` (`DCP-V1-067`), `TestLegacyPreconditionDigestCompileGap` |
| DCP-V1-068 | `internal/doccorpus/behavior_legacy_discovery.go`, `behavior_legacy_lex.go` | `TestLegacyDiscoveryRefusals`, `TestLegacyDiscoveryStaticEdgeCases` (`DCP-V1-068`), `TestLegacyCrosswalk` (`DCP-V1-068`), `TestLegacyLexerCanonicalAndEscapes` |
| DCP-V1-069 | `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go`, `docs_corpus_legacy.go`, `docs.go` | `TestDocsCorpusDiscoverLegacyCLI`, `TestDocsCorpusOperationFlagMatrix` |
| DCP-V1-070..071 | `internal/doccorpus/legacy_cases.go`, `cmd/corvint/docs_corpus_legacy_cases.go` | `TestImportLegacyCasesFormats`, `TestDocsCorpusLegacyCasesCLI` |
| DCP-V1-072..073 | `internal/doccorpus/legacy_cases.go` | `TestImportLegacyCasesFullStripsHTML`, `TestLegacyCasesIDsPathsTitlesWithholdsSteps` |
| DCP-V1-074 | `internal/doccorpus/legacy_cases.go` | `TestImportLegacyCasesBoundsOversized` |
| DCP-V1-075..076 | `internal/doccorpus/legacy_cases.go`, `cmd/corvint/docs_corpus_legacy_cases.go` | `TestSearchLegacyCasesReceipt`, `TestImportLegacyCasesBoundsOversized`, `TestDocsCorpusLegacyCasesCLI` |
| DCP-V1-077 | `cmd/corvint-corpus-mcp/legacycases.go` | `TestDCPV1077CorpusMCPSearchCases` |
| DCP-V1-078 | `internal/doccorpus/legacy_cases.go`, `cmd/corvint/docs_corpus.go`, `cmd/corvint/docs_corpus_help.go` | `TestLegacyCasesRefusals`, `TestDocsCorpusLegacyCasesCLI` |
| DCP-V1-079 | `internal/doccorpus/behavior_legacy.go` | `TestBehaviorLegacyParity` (`manual case baseline`) |
| DCP-V1-080 | `cmd/corvint/docs.go`, `cmd/corvint/docs_corpus_help.go`, `docs/DOCUMENTATION-CORPUS.md` | `TestDocsHelpNamesBehaviorAnnotationContract`, `TestDCPV1080GuideWitnessRequestExample` |
| DCP-V1-081 | `internal/doccorpus/behavior-events.cjs`, `internal/doccorpus/testdata/behavior-events/live-1.61.1` | `TestDCPV1081ProducerNodeUnit`, `TestDCPV1081LiveProducerIsWitnessed` |
| DCP-V1-082 | `internal/doccorpus/behavior_playwright_witness.go` | `TestPlaywrightWitnessImporter` (`DCP-V1-082 zero-witness result names the missing annotation types per test`), `TestDCPV1081LiveProducerIsWitnessed` |
| DCP-V1-083 | `internal/doccorpus/behavior-events.cjs`, `docs/DOCUMENTATION-CORPUS.md` | `TestDCPV1081ProducerNodeUnit` (Node cases `DCP-V1-083: valueless state matchers record their canonical state name after they pass`, `DCP-V1-083: state options, extra arguments, .not and failures record nothing; hidden options never reach a recorded matcher`); live Playwright state-matcher run `NOT_RUN` |
| DCP-V1-084 | `internal/doccorpus/behavior_adapter.go` (unchanged byte-for-byte comparison) | `TestDCPV1084StateMatcherAssertionIsWitnessed` (canonical pair witnessed; non-empty other value mismatches; empty value refused) |
| DCP-V1-085 | `internal/doccorpus/encoding.go`, `internal/corpusindex/index.go` | `TestDCPV1085StreamedEncodingIsEncode`, `TestDCPV1085ParseAllocationIgnoresEncoderPool`, `TestDCPV1085OpenAllocationIgnoresEncoderPool`, `TestIndexedCorpusCapacityQualification` |
| DCP-V1-086..092 | `internal/doccorpus/test_knowledge.go`, `providers.go`, `shards.go`, `compile.go`, `query.go`, `typed_query.go`, `pagination.go`, `internal/corpusrepublish/parity.go` | `TestDCPV1086TestKnowledgeSchemaIsClosed`, `TestDCPV1087TestKnowledgeFieldRules`, `TestDCPV1088TestKnowledgeAnchorsArePinned`, `TestDCPV1089TestKnowledgeJoin`, `TestDCPV1090TestKnowledgeTrustRules`, `TestDCPV1091TestKnowledgeQueryExposure`, `TestDCPV1091TestKnowledgeOnlyCorpus`, `TestDCPV1092TestKnowledgeDeterministicAndBounded`, `TestDCPV1092ParityLargeTestKnowledge` |
| DCP-V1-093..099 | `internal/doccorpus/flow_relations.go`, `providers.go`, `shards.go`, `compile.go`, `query.go`, `typed_query.go`, `integration.go`, `pagination.go`, `internal/corpusrepublish/parity.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/corpus_integration.go` | `TestDCPV1093FlowRelationSchemaIsClosed`, `TestDCPV1094FlowRelationKindFields`, `TestDCPV1095FlowRelationJoinStatus`, `TestDCPV1096FlowRelationImpactAttribution`, `TestDCPV1097FlowRelationUnreviewedAndDepth`, `TestDCPV1097CorpusDepthFlag`, `TestDCPV1098FlowRelationTraceAndGaps`, `TestDCPV1098TraceToolDepth`, `TestDCPV1099FlowRelationDeterministicAndBounded` |
| DCP-V1-140..142 | `internal/contextindex/symbol_region.go`, `rubysymbols.go`, `internal/doccorpus/symbol_anchor.go`, `query.go`, `pagination.go`, `cmd/corvint/docs_corpus.go`, `corpus_integration.go` | `TestSymbolAnchorReresolution` (Ruby and TypeScript: insertion is `moved` with the new range, a one-token edit is `changed`, deletion is `gone`, a reformat is `live` or `moved`), `TestSymbolAnchorUncertainty`, `TestSymbolAnchorReaderReceipt`, `TestNormalizedCodeLines` |
| DCP-V1-143..145 | `internal/mcp/corpusbridge/corpusbridge.go`, `cputime_unix.go`, `internal/doccorpus/query.go`, `symbol_anchor.go`, `pagination.go`, `cmd/corvint-corpus-mcp/main.go` | `TestDCPV1143LazyLoadAndMemo`, `TestDCPV1143StartupRefusesUnreadableArtifact`, `TestDCPV1144LiveFreshnessReusedUntilTheRepositoryChanges`, `TestDCPV1144ConcurrentCallsShareOneOverlay`, `TestDCPV1144SharedOverlayMatchesAFreshReader`, `TestSymbolAnchorReaderReceipt`, `TestDCPV1145PathsBatchEqualsUnion`, `TestDCPV1145BatchDeadlineNamesUnresolved`, `TestDCPV1145BatchDeadlineBeforeLoadNamesEveryPath`, `TestDCPV1145CallDeadlineAndTiming`, `TestDCPV1145BatchFitsOneFrame`; measured on a 100,000-claim corpus in docs/build-log/2026-10-19-V1-1982-corpus-mcp-lazy-batch.md |
| DCP-V1-146 | `internal/doccorpus/compile.go` | `TestDCPV1146OversizedParagraphStaysVerbatim`, `TestDCPV1146OversizedLineIsAGap`, `TestDCPV1146RefusalsNameTheSpan`; measured on Corvint's docs scope in docs/build-log/2026-10-19-V1-1983-docs-corpus-located-refusals.md |; | DCP-V1-150..152 | `internal/doccorpus/typed_query.go`, `docs/DOCUMENTATION-CORPUS.md` | `TestDCPV1150ProviderReportedReviewYieldsUnauthenticatedGap`, `TestDCPV1151AuthenticatedReviewClearsAssertingGap`, `TestDCPV1152AssertingGapNamesTheFailedCondition` |; | DCP-V1-165..169 | `internal/doccorpus/legacy_cases.go`, `cmd/corvint/docs_corpus_legacy_cases.go`, `cmd/corvint/docs_corpus_help.go`, `cmd/corvint-corpus-mcp/legacycases.go` | `TestDCPV1165LegacyCaseIDLookup`, `TestDCPV1166LegacyCaseAnyMatchRanks`, `TestDCPV1167LegacyCaseQueryIDRule`, `TestDCPV1168LegacyCaseAllModeBytesUnchanged`, `TestDCPV1169CorpusMCPSearchCasesModes`, `TestDCPV1165DocsCorpusSearchCasesModesCLI`, `TestDocsCorpusOperationFlagMatrix` |; | DCP-V1-160..163 | `internal/doccorpus/compile.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go` | `TestDCPV1160SchemaTwoManifestOfTwentyThousandFilesBuilds`, `TestDCPV1160DefaultSingleScopeUnchanged`, `TestDCPV1160SchemaFlagValues`, `TestDCPV1161ScopesSortedAndOverlapRefused`, `TestDCPV1162BoundRefusalNamesBoundScopeAndCount` |
| DCP-V1-033..037 | `internal/doccorpus/adoption.go`, `shards.go`, `pagination.go`, native CLI and corpus MCP | Capacity qualification, typed round-trip, import parity, shard closure, restricted canaries and paginated read parity |

## Open decisions

Owner acceptance and any promotion beyond the local experimental profile remain open. Additional
render formats, cross-repository inputs, provider transports and authenticated review attestations
require separate contracts; absent capabilities must remain visible until then.

## Experimental query-parity companion usage and rollback

The issue-399 slice extends the existing experimental corpus contract; it does not promote
Core or accept generated intent. `corvint docs corpus` typed reads use `--query`, `--path` or
`--id` as named above and optional `--retirement POLICY.json`. Local MCP advertises matching
`corvint.docs_*` tools only when their backing capability is present.

Build the optional indexed artifact with `corvint-corpus-parity --mode build --root REPO
--artifact corpus.json`, retaining stdout as the artifact and independently pinning its SHA256.
`--mode query --index FILE --sha256 DIGEST --request QUERY.json` returns a historical native
receipt. `--mode parity` uses `corvint-corpus-question-recording/1` with unique question IDs,
`tool` naming the recorded server tool, a normalized native `request`, and complete expected
`answer` receipts. Strict
comparison deliberately reports different trust/freshness or identity as disagreement.

`corvint-corpus-http --index FILE --sha256 DIGEST` starts an optional loopback JSON endpoint:
POST `/query` with Content-Type `application/json` and a closed native request. This transport
has no live repository. An external hosting operator owns TLS, access control and deployment
qualification; no production hosted serving claim is made by local fixtures.

Rollback the issue-399 query/receipt additions and remove the optional index/parity/HTTP
companions. Existing `/1` and `/2` artifact bytes remain valid: resident query indexes are
unserialized derived state. These additions do not change provider import or Core transport.
