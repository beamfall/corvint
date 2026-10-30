# Revision-pinned documentation corpus V1

Owner: Russell Lewis
Date: 2026-09-19
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner request [issue 31](https://github.com/beamfall/corvint/issues/31),
owner requests [issue 40](https://github.com/beamfall/corvint/issues/40),
[issue 42](https://github.com/beamfall/corvint/issues/42) and
[issue 53](https://github.com/beamfall/corvint/issues/53), and the owner-authorized adoption slice
[issue 339](https://github.com/beamfall/corvint/issues/339), [issue 399](https://github.com/beamfall/corvint/issues/399), [issue 1162](https://github.com/beamfall/corvint/issues/1162), [issue 1225](https://github.com/beamfall/corvint/issues/1225) and [issue 1583](https://github.com/beamfall/corvint/issues/1583),
`AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/specs/deployment-neutral-index-platform-v0.md`,
`docs/specs/source-documentation-draft-v0.md`, `docs/specs/external-evidence-provider-v0.md`.

Parts of this contract moved to three companion specs because its Requirements section was over the
256-obligation OCM limit (V1-2525): [search ranking](documentation-corpus-search-ranking-v1.md)
(DCS-V1), [verified reads](documentation-corpus-verified-reads-v1.md) (DCR-V1) and [extraction,
delta and rebuild](documentation-corpus-extraction-delta-v1.md) (DCX-V1). Each moved ID keeps its
number under the new prefix, and the stub left at each moved section carries the crosswalk.

## Agent digest
- Claim: A native compiler builds revision-pinned documentation corpora for bounded CLI, evidence, rendering and capability-gated MCP queries.
- Status: proposed/experimental; DCP-V1-053 accepted (decision 0519; V1-1196); DCP-V1-054 and DCP-V1-055 accepted (decision 0520; V1-1226, V1-1231); DCP-V1-056 and DCP-V1-057 accepted under the owner's standing auto-accept (2026-10-11; GitHub #805); DCP-V1-060..069 legacy inventory producer accepted under the owner's standing auto-accept (2026-10-11; GitHub #811); DCP-V1-070..079 (test-management case import, GitHub #812) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-080..082 (behavior annotation contract, producer and missing annotations, GitHub #835; V1-1222) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-083..084 (valueless state matchers, GitHub #845; V1-1318) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-085 (encoder-pool-independent allocation, V1-1380) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-086..092 (test-knowledge records, GitHub #1162; V1-1868) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-093..099 (flow-level relations, GitHub #1225; V1-1973) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-140..142 (symbol-anchor re-resolution, GitHub #1223; V1-1971) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-143..145 (lazy corpus MCP load, reused freshness and path batches, GitHub #1236 and #1237; V1-1982) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-146 (verbatim oversized paragraphs, over-bound line gaps and located refusals, GitHub #1242; V1-1983) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-150..152 (asserting E2E gap conditions and unauthenticated review, GitHub #1245; V1-1988) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-165..169 (case search by ID and ranked any-word match, GitHub #1246; V1-1989) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-160..163 (multi-scope /2 manifest inventory, GitHub #1241; V1-1985) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-170..174 (flow section and record-kind filters, GitHub #1247; V1-1990) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-155..159 (constant-cost Git reads and typed capacity refusals, GitHub #1240; V1-1984) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-185..192 (typed homes for supplied gaps, readiness status and manual case links, GitHub #1244; V1-1987) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-175..179 (additional repository anchors, GitHub #1243; V1-1986) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-205..207 (absolute corpus MCP `--artifact` paths, GitHub #1256; V1-1997) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-200..203 (single-call MCP output bound pages or refuses, never -32603, GitHub #1255; V1-1996) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-193..197 (scaled query-index bounds, per-artifact index memo and build-time index refusal, GitHub #1254; V1-1995) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-210..215 (supplied test-run receipts, legacy case links, and supplied gaps in capabilities and the query index, GitHub #1244; V1-1993) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-216..217 (batched symbol-anchor, CEM projection and discovery path reads, V1-1991) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-175..179 (additional repository anchors, GitHub #1243; V1-1986) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-240 (bounded behavior-adapter reconciliation frontier; V1-1089) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-241 (`maintain --page` refusals name the flag and rule; V1-1227) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-242 (corpus MCP `--artifact` path refusals name the argument and rule; V1-1228) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-243 (per-path tree bound for query freshness and batch input walks; V1-2151) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-244 (build-time index check refuses every derivation refusal; V1-2002) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-250..257 (caller-owned retrieval evaluation `docs corpus eval`, GitHub #1480; V1-2315) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-260..268 (claim audit sampling, verdicts and measured accuracy `docs corpus audit`, GitHub #1481; V1-2316) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-280..281 (corpus MCP freshness reuse across additional repositories, GitHub #1487; V1-2328) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-282..284 (per-citation anchor state, null `verified_at` and the `docs_gaps` kind filter, GitHub #1492; V1-2329) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-285..286 (opt-in aggregate corpus MCP outcome counters and the floor summary, GitHub #1493; V1-2330) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-300..311 (widened supplied status, test-run ledger fields, claim-targeted gaps, migration mappings and multi-revision additional repositories; GitHub #1488, #1489, #1490, #1491; V1-2331..V1-2334) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-313 (searchable claim text, GitHub #1555; V1-2435) and DCP-V1-314..317 (stat-stamped corpus MCP freshness and pin reuse, GitHub #1554; V1-2434) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-318..319 (supplied status claim and intent comparison listings, GitHub #1556; V1-2436) accepted under the owner's standing auto-accept (2026-10-11); DCP-V1-320..322 (refusal `verified_at`, gap kind filters on the CLI and HTTP companion, supplied text origins; GitHub #1570; V1-2329, V1-2371), DCP-V1-323 (test-run `association` axis, GitHub #1571; V1-2451), DCP-V1-324..327 (coverage-ledger, flow-completeness and unresolved-concept details, GitHub #1572; V1-2452), DCP-V1-328 (claim hits in `docs_search`, GitHub #1573; V1-2453), DCP-V1-329..330 (subject alternate ids, GitHub #1574; V1-2454) DCP-V1-331 (field-weighted search ranking and kind filter, GitHub #1579; V1-2459) DCP-V1-332 (caller-revision comparison in the trust envelope, GitHub #1570; V1-2329, V1-2371) accepted under the owner's standing auto-accept (2026-10-11); the accepted DCX-V1-340..352, DCX-V1-360..369, DCX-V1-390..399 and DCX-V1-420..425 blocks moved with their status to the DCX-V1 companion spec, and the DCS-V1 and DCR-V1 blocks to theirs (V1-2525); this contract does not accept generated intent or qualify HDC.
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
  bounded to 128 MiB (amended by `DCX-V1-472`: provider record shards and the other inputs are
  each bounded to 128 MiB). Existing `/1` and independent behavior/receipt limits remain unchanged.
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
  bytes 128 MiB, existing per-family record limits, the query-index posting and unique-key-byte bounds of
  `DCP-V1-193` and
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
- `DCP-V1-240`: A behavior-adapter build retains at most 4096 reconciliation frontier diagnostics,
  whose encoding together takes at most 2 MiB: the longest prefix under the frontier order (kind,
  subject, input, field and detail, then revision, digest and correction) within both bounds, so the
  retained entries do not depend on reconciliation order. The result's `frontier_omitted` member
  counts the diagnostics past either bound and is absent when none were omitted. A previous result with a negative count is refused as invalid. The bound keeps the build's
  live heap and its encoded result within the output bound, so for a request whose only excess is its
  frontier the CLI build and `--check` (`DCP-V1-044`) both accept. Status: (accepted under the owner's
  standing auto-accept (2026-10-11); V1-1089).
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
  `docs flows` and `docs maintain`, each of which `corvint help` accepts both as separate words and
  as the one argument the list prints (`corvint help "docs corpus"`; V1-2308, GitHub #1476). Acceptance:
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
  and rederives the artifact (`DCP-V1-011`; amended by `DCR-V1-416` to the verified read) and keeps it; a later call reuses it only while the
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
- `DCP-V1-170`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1247;
  V1-1990)) A flow query (`docs corpus flow`, `corvint.docs_flow`) takes an
  optional section filter: one section name or a duplicate-free list of 1 to 64 names, each
  matched exactly against the flow's `FlowSection.Name`. The answer is the flow subject plus the
  paragraphs of only the named sections, in the usual record order; a section filter without `include` selects paragraphs only. A requested name the flow
  does not have answers no records with `state` `empty`, `miss` `none-found` and a
  `flow_selection` naming every available section and the unknown names; it never falls back to
  the whole flow.
- `DCP-V1-171`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1247;
  V1-1990)) A flow query takes an
  optional `include`: a duplicate-free, non-empty list from the closed set `claims`, `relations`,
  `test_knowledge`, `paragraphs`. Only the listed record kinds follow the flow subject. An unknown
  or repeated kind, an empty list, a section filter whose `include` omits `paragraphs`, or either
  filter on any operation other than flow is refused: `invalid-arguments` from the MCP tool and the
  CLI, `corpus-refused` from the library and indexed companion.
- `DCP-V1-172`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1247;
  V1-1990)) A filtered flow receipt
  carries `flow_selection`: every section of the flow in flow order with its paragraph count, the
  requested sections, the effective `include` kinds, and per kind (`claims`, `relations`,
  `test_knowledge`, `paragraphs`) the records the flow has (`available`) and the records the
  filter selected before paging (`selected`). A flow query with neither filter carries no
  `flow_selection` and its receipt bytes are unchanged.
- `DCP-V1-173`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1247;
  V1-1990)) `limit` and `offset` page
  the filtered answer exactly as they page an unfiltered one: the pages of a filtered query
  concatenate to its single-page answer, a repeated page is identical, and `flow_selection` counts
  do not change with the page.
- `DCP-V1-174`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1247;
  V1-1990)) The `corvint.docs_flow`
  input schema declares `section` (a string or array of strings) and `include` (an array over the
  closed set); no other corpus tool admits them, except `include: ["claims"]` on
  `corvint.docs_search` (`DCP-V1-328`). The CLI flow operation takes `--section NAME[,NAME]`
  and `--include KIND[,KIND]`, refused by the same shape rules before any artifact is read.
- `DCP-V1-185`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1987)) A `corvint-corpus-adoption-provider/1`
  record may carry `gaps`, a list of producer-supplied gap claims, each with a provider-qualified
  `id`, the `subject` it concerns, a `kind` (a non-empty identifier of at most 256 bytes), a
  `reason` and `evidence` that is anchored or an explicit unknown under the ordinary provider
  evidence rules. Every supplied gap ID is unique among all corpus record IDs and its subject
  exists. A supplied gap never raises trust: imported evidence stays `generated`, and a provider
  `reviewed` label is kept only as attributed `reported_trust`. Any other provider schema that
  carries `gaps` is refused.
- `DCP-V1-186`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1987)) The compiler sets each supplied gap's `origin` to
  `supplied` (an input naming another origin is refused) and retains it in the artifact's
  `supplied_gaps`, sorted by ID. `docs_gaps` returns supplied gaps after the computed gaps, selected
  by `id` matching the subject or the gap ID, and cites their evidence. Supplied gaps never add,
  remove or rewrite a computed gap.
- `DCP-V1-187`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1987)) A claim's typed detail may be `status`, a supplied
  readiness or proof status for the claim's subject: `state` is one of `blocked`, `ready` or
  `proven`, `reason` is required, and `supporting` lists at most 256 unique IDs of existing corpus
  records. `docs_get` on the subject returns the status claim with its detail. A status is
  supplied, not derived: `proven` grants no trust and changes no computed gap, journey state or
  coverage metric.
- `DCP-V1-188`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1987)) `docs_coverage` adds one row per supplied status
  state, `supplied_status:STATE`, counting status claims in that state over all status claims,
  labelled `origin: supplied`. A corpus without supplied statuses adds no row.
- `DCP-V1-189`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1987)) An adoption provider may declare `manual_case`
  subjects (an imported or legacy test-management case; no typed detail required) and
  `manual_case_for` relations from a `manual_case` subject to any subject. A link from any other
  subject kind is refused. `docs_gaps` treats a flow that is the target of such a link as having a
  documented manual disposition, so it reports no `missing-manual-cases` gap for that flow.
- `DCP-V1-190`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1987)) `import_parity` counts supplied gaps as
  `supplied_gap`, status claims as `claim/status`, and case subjects and links under their
  `subject/manual_case` and `relation/manual_case_for` kinds.
- `DCP-V1-191`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1987)) Under a `/2` manifest, an adoption provider and the
  corpus admit observations up to the record limit (100,000), as for subjects, claims and relations;
  supplied gaps share the same limit. `/1` manifests and non-adoption providers keep the 4,096
  observation bound.
- `DCP-V1-192`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1987)) A provider input without `gaps`, `status` details,
  `manual_case` subjects or `manual_case_for` relations builds to the same artifact bytes, and its
  `gaps` and `coverage` receipts keep their bytes. Incremental warm reuse falls back to a full
  import for a shard that carries supplied gaps.
- `DCP-V1-205`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1256;
  V1-1997)) `corvint-corpus-mcp --artifact` also accepts an absolute path below `--root`. The
  server rewrites it to the equivalent root-relative path before startup, so the session opens the
  same file through the same bounded reader and serves the same bytes as the relative spelling. A
  relative value is unchanged. Each `--map` takes the same rewrite before its planner loads it
  (amended 2026-10-20, V1-1998).
- `DCP-V1-206`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1256;
  V1-1997)) Containment is decided lexically against the absolute `--root` first, then against
  the symlink-resolved `--root` and the symlink-resolved directory of the artifact. A root reached
  through a symlinked ancestor (macOS `/var` -> `/private/var`) therefore admits either spelling
  of the artifact path. The bounded reader still refuses a root-relative path that escapes the
  root through a symlink.
- `DCP-V1-207`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1256;
  V1-1997)) An absolute `--artifact` that is not strictly below `--root` (outside it, or the root
  itself) exits 2 before any read with `corvint-corpus-mcp: --artifact: absolute path is not below
  --root`, not the generic `repository unavailable` refusal; such a `--map` exits 2 with
  `corvint-corpus-mcp: --map: absolute path is not below --root` (V1-1998). The `corvint docs corpus` CLI keeps
  its `DCP-V1-054` refusal of absolute local input paths; this requirement does not change it.
- `DCP-V1-242`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1228) A relative
  `corvint-corpus-mcp --artifact` that breaks a `DCP-V1-054` local input path rule exits 2 before any
  read with `corvint-corpus-mcp: --artifact: local input paths must be canonical and relative to
  --root (<rule>)`, naming the argument and the rule, not the generic `repository unavailable`
  refusal. No corpus tool argument is a local input path: tool calls read only the artifact the
  server validated at startup, so a call never meets this refusal. The admitted set is unchanged, and
  a canonical path that is missing or unreadable keeps `repository unavailable`. `--map` keeps its
  own refusals. Acceptance: `TestDCPV1242RelativeArtifactRefusalNamesArgumentAndRule`.
- `DCP-V1-200`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1255;
  V1-1996)) A single (non-batch) corpus MCP tool call always fits one MCP message. Its receipt is
  measured as it is framed: once as structured content and once as the JSON string of its
  enveloped text, against the 1 MiB message bound less the batch reserve (`MaxBatchBytes`). A
  receipt over that bound never reaches the transport, so the call never answers JSON-RPC
  `-32603` Internal error for its size.
- `DCP-V1-201`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1255;
  V1-1996)) Under a `/2` (paged) artifact, an over-bound call is answered again with a smaller
  `limit` (proportional to the measured cost, strictly fewer results each time) until the receipt
  fits. The answer is byte-identical to the receipt the same call returns with that smaller
  `limit`, so it carries `next_offset` and the page-withheld limitation, and paging with
  `next_offset` returns, in order, exactly the records of one unbounded call.
- `DCP-V1-202`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1255;
  V1-1996)) A receipt that cannot shrink (a `/1` artifact, which has no `offset`, or a page of one
  record) is refused as the tool failure `output-bound-exceeded`, whose message names the result
  count, the measured cost and the byte bound.
- `DCP-V1-203`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1255;
  V1-1996)) A call whose receipt fits the bound keeps its bytes; artifacts are unchanged.
- `DCP-V1-193`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1254;
  V1-1995)) The derived query index is bounded at 8,000,000 postings and
  64 MiB of unique key bytes, about twenty postings and 160 key bytes for each record the `/2`
  family bounds admit, and each stored posting (an identity, path, term, claim or edge reference)
  is counted exactly once. A corpus of at least 100,000 records whose index holds more than
  1,000,000 postings answers every indexed read.
- `DCP-V1-194`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1254;
  V1-1995)) An index over either bound is refused with
  `corpus-capacity-exceeded` and the message `indexed postings <observed> exceeds the limit of
  <bound>` or `indexed unique key bytes <observed> exceeds the limit of <bound>`. Past a bound the
  derivation stops storing postings but keeps counting, so the observed count is the full total the
  index would hold. The per-key, record-family and fanout refusals keep `corpus-refused`.
- `DCP-V1-195`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1254;
  V1-1995)) The corpus MCP bridge derives the query index once for each
  opened artifact, on the first call to an index-backed tool (`docs_concept`, `docs_flow`,
  `docs_claims`, `docs_dependencies`, `docs_recommend_tests`, `docs_gaps`, `docs_intent`,
  `docs_navigation`, `docs_vocabulary`), before any freshness work. Later calls and every path of a
  batch reuse that index, or its corpus refusal, until the artifact memo is replaced; a cancelled
  derivation is not kept. Receipts are the bytes the unmemoized query returns.
- `DCP-V1-196`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1254;
  V1-1995)) `corvint docs corpus build` refuses a corpus whose query index
  would exceed a `DCP-V1-193` bound with the `DCP-V1-194` refusal instead of writing an artifact the
  indexed reads refuse. Opening an artifact written before is unchanged: it still opens and serves
  the reads that need no index. `corvint docs flows` finalization refuses the same way, and a
  republish already does because its embedded index derives the query index (amended 2026-10-20,
  V1-2003).
- `DCP-V1-197`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1254;
  V1-1995)) Artifact bytes are unchanged, and for a corpus within the
  former bounds the index content, order and receipts are unchanged; only corpora the former bounds
  refused now index. Memory and latency at 100,000 records are retained in
  https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1995-corpus-index-bound.md.
- `DCP-V1-210`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1993)) A claim's typed detail may be `test_run`, a producer-supplied
  test-run receipt for the claim's subject (the test or flow that was run): `outcome` is one of
  `passed`, `failed`, `skipped` or `errored`, `run` is a required run identifier of at most 1,024
  bytes, `origin` must be `supplied`, `revision` is an optional full Git object ID and `at` an
  optional RFC 3339 time. A detail with any other outcome or origin, an empty or oversized run, an
  invalid revision or time, or a `test_run` detail on a subject or relation is refused
  (`corpus-refused`).
- `DCP-V1-211`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1993)) `docs_get` on the subject returns the test-run claim with its
  detail. `import_parity` counts test-run claims as `claim/test_run`. `docs_coverage` adds one row
  per supplied outcome, `supplied_test_run:OUTCOME`, counting test-run receipts with that outcome
  over all test-run receipts, labelled `origin: supplied`; a corpus without them adds no row. The
  compiler does not execute, derive or verify the run: a supplied `passed` grants no trust (evidence
  stays `generated`, a provider `reviewed` label stays `reported_trust`) and changes no computed gap,
  journey state or other coverage metric.
- `DCP-V1-212`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1993)) A `manual_case` subject's typed detail may be `legacy_case`
  with `id`, the canonical external case ID `C<digits>` (no leading zero) that a
  test-management export such as the one `docs_search_cases` reads names for that case. A
  non-canonical ID, a `legacy_case` detail on any other subject kind or on a claim or relation, and
  one case ID declared by two subjects are refused (`corpus-refused`).
- `DCP-V1-213`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1993)) `docs_get` with a canonical case ID that names no corpus record
  returns the `manual_case` subject that declares it and that subject's `manual_case_for`
  relations, with their details and citations. When no subject declares it, `docs_get` returns one
  gap of kind `legacy-case-unlinked` whose reason says that no `manual_case` subject declares the
  case and that the external export is not corpus evidence. The corpus never reads the export, so a
  link does not prove the case exists there. A record ID keeps its ordinary answer, a non-canonical
  case ID still finds nothing, and the `docs_search_cases` receipt is unchanged (DCP-V1-077).
- `DCP-V1-214`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1993)) The `gaps` capability lists each supplied gap ID after the
  computed gap offsets, and the query index admits supplied gaps as record kind `supplied_gap`
  under their ID and anchor paths, within the record limit. `docs_get` by a supplied gap ID returns
  the supplied gap, labelled `origin: supplied`, with or without the query index.
- `DCP-V1-215`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1244;
  V1-1993)) A provider input without `test_run` or `legacy_case` details
  or supplied gaps builds to the same artifact and query-index bytes, and its receipts keep their
  bytes. The incremental path counts claim kinds as a cold build does, so a shard with status or
  test-run claims compiles incrementally to the cold artifact bytes.
- `DCP-V1-175`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1243;
  V1-1986)) A manifest may name further repositories in
  `additional_repositories`, sorted by `id`: each member is `{id, revision, root}`, where `id` is
  the repository's root-commit identity, `revision` its one pinned commit and `root` the clean
  relative path (it may begin with `../`) from the primary repository root to a local checkout
  holding its objects. At most seven are declared; an `id` equal to the primary's or repeated, a
  revision shared with another repository, an absolute, unclean, empty or control-character root
  is refused (`corpus-refused`), and every declared revision counts toward the eight-revision
  bound. The member is omitted when empty.
- `DCP-V1-176`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1243;
  V1-1986)) Each additional repository is read only from its
  declared checkout, which must not resolve to the primary root or another declared checkout
  (`corpus-refused`): its revision must resolve there, and its root commit must equal the
  declared `id` or the build refuses (`corpus-refused`, repository identity mismatch); an absent
  or unreadable checkout is `corpus-input-unavailable` naming the repository. Scopes and inputs at a
  declared revision are inventoried and pinned in that checkout; any other revision is resolved
  only in the primary repository. Nothing is fetched, discovered or inferred from the locator.
- `DCP-V1-177`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1243;
  V1-1986)) An anchor names the repository that owns its
  revision: the primary repository, or the declared additional repository whose pinned revision
  it is. An anchor whose repository differs, including one naming an undeclared repository, is
  refused. Provider records still name the primary repository as their source, and native
  provider inputs stay at the primary revision.
- `DCP-V1-178`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1243;
  V1-1986)) Freshness checks the scopes and inputs of each
  additional repository against that repository's live checkout (commit and working tree), not
  the primary's, and prefixes each reason with `additional repository <id>: `; a change there
  makes the read `stale`, while an unrelated primary change does not. Because the primary
  observation no longer identifies the state, such an overlay carries no reusable observation
  key, and the MCP bridge verifies each memoized pin in the checkout that owns it.
- `DCP-V1-179`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1243;
  V1-1986)) A manifest without additional repositories, and
  the artifact built from it, are byte-identical to those built before this contract. The
  primary-only reads that do not yet span repositories (the native provider, test-knowledge
  anchors, `/2` behavior repositories, symbol re-resolution and the receipt's top-level
  `repository`) are unchanged and keep their existing gaps.
- `DCP-V1-155`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1240;
  V1-1984)) A corpus read checks the freshness of every pinned input for a number of Git
  operations that does not depend on the input count: inputs the live revision index holds clean
  need no Git operation, and the rest resolve together through one verified walk, which reads each
  directory level the paths pass through in one batch. Every tree read is self-hashed and linked to
  its verified parent OID, so the walk trusts Git no more than a single-path lookup does, and a
  path that is absent or lies below a non-tree is absent. One changed or absent input still makes
  the read `stale` and names that input.
- `DCP-V1-156`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1240;
  V1-1984)) Manifest inventory and build load the inputs the revision index does not hold as text
  (for example XML or CSV; HTML and ERB are indexed text since IDX-SNAP-V0-109) through batched reads: per pinned revision, one verified path
  walk (`DCP-V1-155`), then one header batch for every distinct regular blob and one body batch for
  every admitted blob. Every body must have its admitted size and self-hash to its OID. All reads
  pass through the same bounded Git runner and its operation budget; none bypasses them.
- `DCP-V1-157`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1240;
  V1-1984)) A corpus that exceeds a capacity limit is refused with `corpus-capacity-exceeded`, and
  the message names the limit and the count: a manifest's input, scope or provider count, the
  scope's input count at inventory, or the aggregate pinned input bytes (128 MiB; amended by
  `DCX-V1-472`, which also names the largest contributors). Count limits are
  checked before any Git operation, and the byte limit and every per-input refusal (missing,
  non-regular or over-bound input) before any body is read; the empty-manifest and per-input
  refusals keep their codes. A per-input refusal is now reported before a content mismatch of an
  earlier input.
- `DCP-V1-158`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1240;
  V1-1984)) An exhausted Git operation budget or Git output bound during corpus inventory, build or
  freshness is `corpus-capacity-exceeded`, naming the bound and the phase, and never surfaces as
  `internal-error`. The native CLI and the corpus MCP server map such a failure to the same code
  through one shared function.
- `DCP-V1-159`: (accepted under the owner's standing auto-accept (2026-10-11) (GitHub #1240;
  V1-1984)) Batched loading changes no artifact byte: a corpus that built before builds to the same
  bytes, apart from the builder identity and the digest that covers it.
- `DCP-V1-216`: (accepted under the owner's standing auto-accept (2026-10-11) (V1-1991)) The reads
  that consume many paths one at a time batch them like `DCP-V1-155`: cited symbol anchors on a
  read receipt (their paths, the pinned, live and same-name candidate blobs), the CEM claim
  projection's anchors at the map base, legacy discovery's test files and Playwright discovery's
  listed test files are each resolved by one verified path walk and read by one header batch and
  one body batch, so each phase stays within the `DCP-V1-155` operation bound whatever its anchor
  or path count. The batches only warm a cache in front of the unchanged per-path reads: a path or
  blob a batch does not hold, because the batch failed or a bound left it out, is read as before,
  so every result, refusal, code and order is byte-identical to the per-path reads; only a
  cancelled context fails a batch. The walk answers a path only when the per-path lookup would
  admit it: every parent directory resolved to a tree and the root and parent tree records stream
  within the per-path `MaxTreeBytes` bound, so the walk's larger aggregate bound never admits a path
  the per-path lookup refuses. A Git operation budget or output-bound failure of a symbol-anchor
  blob read, batched or per path, withholds every symbol state with that diagnostic rather than
  reading as an unavailable blob. Symbol-anchor prefetch reads at most 32 MiB ahead so unused
  bodies cannot exhaust the request's blob budget. Each phase keeps its own Git budget and the
  `DCP-V1-157` capacity typing.
- `DCP-V1-243`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2151) The
  `DCP-V1-155` walks that resolve live query-freshness inputs and `DCP-V1-156` staged batch inputs
  MUST keep per-path admission parity: a path whose `LookupTreeEntry` would stream its root and
  parent tree records past the per-path `MaxTreeBytes` bound is refused with that lookup's own
  `git-output-exceeded` refusal, byte for byte, although the walk's larger aggregate bound would
  admit it. Acceptance: `TestDCPV1243ResolvedInputsKeepPerPathTreeBound`.
- `DCP-V1-244`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2002) The
  `DCP-V1-196` build-time index check refuses with the same typed code and message every condition
  under which the query-time index derivation refuses the artifact, not only a `DCP-V1-193` bound:
  the `DCP-V1-194` capacity refusal, or `corpus-refused` for an indexed record family over its
  bound, an indexed key over 65536 bytes, a missing or duplicate indexed record identity, or a path
  whose fanout reaches the record bound. `corvint docs corpus build`, `docs flows` finalization and
  republish therefore never write an artifact whose indexed reads refuse. There is no derivation
  refusal it cannot pre-check: the derivation reads only the artifact. An interrupted check still
  fails closed with the context's error. Acceptance: `TestDCPV1244AdmitIndexRefusesEveryDerivationRefusal`.
- `DCP-V1-300`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1488; V1-2331) A supplied
  `status` detail's `state` is one of the closed set `blocked`, `ready`, `proven`, `partial`,
  `not-demonstrable`, `disputed`, `obsolete`, `not-established` or `unknown`, and it may carry an
  optional `qualifier`, one of `provisional` or `classification-only`. Any other state or qualifier
  is refused (`corpus-refused`, invalid supplied status). The widened states and the qualifier are
  supplied, not derived: they grant no trust and change no computed gap, journey state or coverage
  metric. This amends the three-state set of `DCP-V1-187`.
- `DCP-V1-301`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1488; V1-2331) `docs_coverage` counts a
  qualified status under `supplied_status:STATE:QUALIFIER`, never under `supplied_status:STATE`, so
  a provisional or classification-only `proven` is never counted as plain `proven`.
- `DCP-V1-302`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1488; V1-2331) A supplied `unknown` status
  is a recorded state: it is counted under `supplied_status:unknown` and returned with its detail,
  while a subject without a status claim has no status and adds to no status row.
- `DCP-V1-303`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1489; V1-2332) A supplied `test_run`
  detail may carry optional run-ledger fields, each from a closed set: `retries`, an integer from 0
  to 1,048,576; `flake_verdict`, one of `stable`, `flaky`, `unqualified` or `unknown`; `cleanup`,
  one of `passed`, `failed`, `partial`, `not-run` or `unknown`; `control_outcome`, one of the
  `DCP-V1-210` outcomes; and `validity`, a non-empty list of `{axis, status}` with each axis
  (`revision`, `environment`, `isolation`, `oracle`, `determinism`, or `association` under
  `DCP-V1-323`) at most once and a status of
  `passed`, `failed`, `unknown` or `not-applicable`. Any other value, a negative or oversized
  `retries`, a repeated axis or an empty `validity` list is refused (`corpus-refused`, invalid
  supplied test run ledger field). Absent fields are omitted, so existing receipts keep their
  bytes. The fields are supplied, never executed or verified, and grant no trust.
- `DCP-V1-304`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1489; V1-2332) A supplied gap's `subject`
  may name a claim as well as a subject, such as the claim that carries a run's `test_run` detail;
  a gap naming neither is refused (supplied gap subject missing). A bare run identifier is not a
  corpus record ID and is refused as a target, because run identifiers need not be unique.
  `docs_gaps` on a subject also returns the supplied gaps that target that subject's claims. This
  amends the subject rule of `DCP-V1-185` and the selection of `DCP-V1-186`.
- `DCP-V1-305`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1489; V1-2332) `docs_coverage` adds one row
  per supplied flake verdict, `supplied_test_run_flake:VERDICT`, and one per supplied cleanup
  outcome, `supplied_test_run_cleanup:OUTCOME`, each counting the test-run receipts with that value
  over the test-run receipts that carry the field, labelled `origin: supplied`. A corpus without
  either field adds no such row.
- `DCP-V1-306`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1491; V1-2334) An adoption provider may
  declare `migration_mapping` relations between two `inventory` subjects (units of a
  code-migration ledger). Each requires a typed `migration_mapping` detail with an `outcome` from
  `unchanged`, `modified`, `moved`, `renamed`, `copied`, `added`, `removed`, `ambiguous` or
  `unresolved`, a `basis` from `stable_id`, `region_hash`, `content_hash`, `path`, `symbol`,
  `declared` or `unknown`, and an optional `run` identifier of at most 1,024 bytes. The one-sided
  outcomes `added`, `removed` and `unresolved` name the same unit at both ends; every other
  outcome joins two distinct units. A missing detail, a value outside its set, a one-sided shape
  mismatch, or an end that is not an `inventory` subject is refused (`corpus-refused`). A unit's
  readiness is a `DCP-V1-300` status claim on that unit. Mappings are supplied, not derived, and
  grant no trust.
- `DCP-V1-307`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1491; V1-2334) `docs_get` (and `docs_trace`)
  on an inventory unit returns, after the unit's records, every `migration_mapping` relation
  reachable from it through mappings in either direction, in artifact order and without repeating
  a returned relation, citing their evidence. A corpus without migration mappings returns what it
  returned before.
- `DCP-V1-308`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1490; V1-2333) An additional repository may
  carry `revisions`, a non-empty, sorted list of further pinned commits of the same repository,
  read and identity-checked in the same declared checkout. Each must be a full Git object ID
  distinct from the repository's `revision` and from every other repository's revisions; every
  pinned revision counts toward the eight-revision bound, and exceeding it is a typed capacity
  refusal naming the pinned repository revision count. An anchor at a further revision names that
  repository and reports that revision in citations. The member is omitted when absent, so
  existing manifests keep their bytes. This amends the one-revision rule of `DCP-V1-175`.
- `DCP-V1-309`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1490; V1-2333) Freshness compares only an
  additional repository's declared `revision` with its live checkout; inputs at its further
  pinned revisions are immutable Git content and are never reported stale because the checkout
  moved on.
- `DCP-V1-310`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1490; V1-2333) `locate` and
  `recommend-tests` (`corvint.docs_locate`, `corvint.docs_recommend_tests`) accept an optional
  `repository`, the root-commit identity of the primary or a declared additional repository, and
  then return or select only records anchored at the path in that repository. A repository
  qualifier on any other operation, an undeclared repository (`corpus-refused`) or an empty MCP
  value (`invalid-arguments`) is refused.
- `DCP-V1-311`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1490; V1-2333) When the artifact declares
  additional repositories, a `locate` or `recommend-tests` receipt carries `result_repositories`,
  naming for each visible result the repository of its anchor at the path (else of its first
  anchor, else the primary). A single-repository receipt omits the member and keeps its bytes.
- `DCP-V1-312`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2345) `corvint docs
  corpus locate` and `corvint docs corpus recommend-tests` accept an optional `--repository
  ROOT_COMMIT` with the `DCP-V1-310` semantics of the MCP `repository` argument: the same receipt
  for a declared repository, `corpus-refused` for an undeclared one, and `invalid-arguments` for an
  empty value or for the flag on any other operation.
- `DCP-V1-280`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1487;
  V1-2328) The `DCP-V1-144` reuse key covers every declared checkout: the primary repository and
  each `DCP-V1-175` additional repository, observed in parallel (object format, commit, tree,
  status digest and dirty paths per checkout). For a single-repository artifact the key is the
  primary observation key unchanged; otherwise it is the length-prefixed concatenation of each
  checkout's key, the additional ones prefixed by their repository ID in manifest order. The key
  stored with a computed freshness is built from the live indexes that freshness was resolved
  from, never from an earlier observation. A change in any declared checkout therefore recomputes
  freshness, and the recomputed receipt keeps the `DCP-V1-177` stale limitation naming that
  repository; an unchanged set of checkouts reuses it. An observation failure in any checkout
  computes freshness rather than reusing it.
- `DCP-V1-281`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1487;
  V1-2328) Acceptance at scale: a primary plus two additional repositories with at least 8,000
  pinned inputs answers the second and later warm `docs_get` and `docs_search` calls with
  `freshness: reused`, a median `freshness_ms` under 100 ms and a p95 `total_ms` under 1 s; a
  one-file change in one additional repository recomputes, names that repository in the stale
  limitation, and the next call reuses again. Acceptance:
  `TestDCPV1280AdditionalRepositoryFreshnessReused`, `TestDCPV1281AdditionalRepositoryWarmCallsAtScale`.
- `DCP-V1-313`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1555;
  V1-2435) Claim text is searchable. The query index keeps, for every claim, the evidence terms of
  its ID and text (lowercased words of two or more characters, stop words dropped), charged
  against the `DCP-V1-193` posting and key-byte bounds. `claims` (`corvint.docs_claims`, `corvint
  docs corpus claims`) takes exactly one of a path (`path`/`paths`, `--path`) or a `query`
  (`--query`) of 1 to 1,024 bytes; a query returns the claims holding at least one of its terms,
  ranked by the number of distinct query terms they hold, a claim whose trimmed text equals the
  trimmed query case-insensitively first, ties ordered by claim ID, each claim once, paged like
  every read. No match abstains. A query with a path, a batch, an empty query or neither form is
  refused (`invalid-arguments` over MCP, a usage error from the CLI). Claim results carry their
  claim IDs, so an evaluation case's `expect.claims` scores the query (`DCP-V1-252`). The CLI and
  MCP receipts of one query are equal. Acceptance: `TestDCPV1313ClaimTextQueryReachesClaimRecall`,
  `TestDCPV1313CorpusClaimsQueryCLIParity`.
- `DCP-V1-314`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1554;
  V1-2434) The corpus MCP bridge keeps a checkout stamp: one digest, built without running Git,
  over each checkout that the reuse key observes or that owns a pinned revision. Per checkout it
  records the checkout root, every worktree entry except the repository's own `.git`, and every
  entry of the repository directory except large-file storage, loose objects included, each by
  name, mode, size, inode, change time and modification time; symlinks are recorded, never
  followed. The repository config is read only as a regular file of at most 1 MiB, opened without
  following a symlink and without blocking. The stamp is unavailable, and never matches, when any
  checkout is not an ordinary repository directory (a gitfile, a missing or non-Git directory),
  holds a nested `.git`, declares a common-dir link or an alternate object store, has a repository
  config that is not such a file or names an include or a worktree, exceeds 250,000 entries, has an entry changed within 2 s of the
  capture, or runs on a platform whose lstat result carries no change time (only Darwin and Linux
  are stamped). Directories are read in batches and the entry bound is checked after each batch,
  so an oversized directory is never read or sorted whole; batching does not change the digest of
  a checkout within the bound. A cancelled call stops the traversal at the next batch, leaves no
  stamp, and returns its cancellation without holding the corpus load slot (V1-3029). Acceptance:
  `TestDCPV1314StampChangesWithCheckoutState`,
  `TestDCPV1314StampUnavailableOutsideCoveredState`, `TestDCPV1314StampRefusesRacyEntries`,
  `TestDCPV1314StampBoundStopsBatchedReads`, `TestDCPV1314StampDigestIndependentOfBatch`,
  `TestDCPV1314StampStopsOnCancel`, `TestDCPV1314CancelledStampReleasesLoad`.
- `DCP-V1-315`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1554;
  V1-2434) Each warm call captures the stamp once, before its pin check and its freshness
  observation. When the reuse key is observed, the stamp is kept with the memoized overlay; a later
  call whose stamp equals it reuses the overlay without observing any checkout, so the reuse cost
  is a parallel lstat walk instead of a Git identity and status run per checkout. A different or
  unavailable stamp observes exactly as `DCP-V1-144` and `DCP-V1-280` define, and a replaced
  artifact memo drops the pairing. Fail-closed freshness is preserved: any change a later
  observation could see changes the stamp, and a change during the observation leaves a stamp that
  no later state matches. Acceptance: `TestDCPV1315WarmCallsSkipGitWhileStampUnchanged`.
- `DCP-V1-316`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1554;
  V1-2434) The `DCP-V1-143` pinned-revision verification is not repeated while the call's stamp
  equals the stamp captured before the last successful verification of the memoized artifact; a
  different or unavailable stamp verifies, and a failed verification drops the memo as before.
  `verify_ms` reports the stamp capture plus any verification. Acceptance:
  `TestDCPV1315WarmCallsSkipGitWhileStampUnchanged`. Amendment (V1-2542, V1-2544; experimental
  under the standing lane auto-accept): the reuse key is the stamp together with the ordered pin
  set it was verified for, because a pin that resolves inside an already stamped checkout leaves
  the stamp unchanged; a changed pin set verifies whatever the stamp. Non-goal: no change to what
  the stamp observes. Failure mode avoided: a pin set changed under an equal stamp answered from
  an unverified memo, which a loaded host reached once the fixture aged past the racy window.
  Acceptance: `TestDCPV1143LazyLoadAndMemo` (stamp unavailable and stamp available subtests).
- `DCP-V1-317`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1554;
  V1-2434) The `DCP-V1-281` acceptance fixture includes an additional checkout with more than
  10,000 tracked files (4,000, 12,000 and 1,000 files; 17,001 pinned inputs) and meets the same
  bounds: median warm `freshness_ms` under 100 ms and p95 warm `total_ms` under 1 s. Acceptance:
  `TestDCPV1281AdditionalRepositoryWarmCallsAtScale`.
- `DCP-V1-318`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1556;
  V1-2436) `claims` (`corvint.docs_claims`, `corvint docs corpus claims`) takes, in place of a
  path or a query, a supplied status filter: a `state` (`--state`) from the `DCP-V1-300` set, an
  optional `qualifier` (`--qualifier`) from its qualifier set, and an optional `run` (`--run`) of 1
  to 1,024 bytes. It returns the claims carrying a `DCP-V1-300` status detail in that state whose
  qualifier equals the named one, or is absent when none is named, so an unqualified listing's
  total equals the `supplied_status:STATE` coverage row (`DCP-V1-301`). With a run, only claims
  whose subject is an end of a `migration_mapping` relation (`DCP-V1-306`) with that `run` are
  kept. Each result is the claim record (its ID, subject and evidence anchors) with its status
  detail, in claim ID order, paged like every read; no match is `no-match`. A qualifier or run
  without a state, a state with a path, batch or query, a value outside its set, an empty or
  oversized value, or a filter on another operation is refused (`invalid-arguments` over MCP, a
  usage error from the CLI). The listing is supplied data and derives no readiness. The companion
  request parser admits the same shape. Acceptance: `TestDCPV1318StatusClaimListing`,
  `TestDCPV1318StatusAndIntentListingCLIParity`.
- `DCP-V1-319`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1556;
  V1-2436) `intent` (`corvint.docs_intent`, `corvint docs corpus intent`) takes its `id` (`--id`)
  optionally. Without one it lists every subject carrying an intent comparison detail, with that
  detail, in subject ID order and paged; an `intent` with an id answers as before. A present but
  empty id is refused. Acceptance: `TestDCPV1319IntentListing`,
  `TestDCPV1318StatusAndIntentListingCLIParity`.
- `DCP-V1-320`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1570; V1-2329, V1-2371) The
  `corvint-corpus-http-refusal/1` `trust_envelope` and the corpus MCP tool-failure envelope carry
  `verified_at: null` and the limitation `verified_at is null: the request was refused; no
  verification is recorded`, as every answered envelope does under `DCP-V1-283`. Acceptance:
  `TestDCPV1320And321HTTPRefusalAndGapKinds`, `TestDCPV1320ToolFailureVerifiedAtNull`.
- `DCP-V1-321`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1570; V1-2329, V1-2371) The `DCP-V1-284` gap kind filter is
  reachable from every reader: the CLI `gaps` operation takes `--kind KIND[,KIND]` and the HTTP
  companion `kinds`, under the same shape rules, with the same `kind_totals` and receipt as the
  MCP call. A malformed list is a usage error from the CLI and `query-refused` from the companion.
  Acceptance: `TestDCPV1321CorpusKindAndClaimSearchCLIParity`,
  `TestDCPV1320And321HTTPRefusalAndGapKinds`.
- `DCP-V1-322`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1570; V1-2329, V1-2371) A receipt carries `text_origins`, keyed by
  record ID and field, for each visible result whose provider is not the native provider:
  `name` of a subject, `text` of a claim and `reason` of a supplied gap, each
  `{"origin": "untrusted", "max_bytes": 65536}` (the supplied free-text cap). Native records are
  never listed. With no such result the field is absent and receipt bytes are unchanged. The
  marking is per page and identical on every reader. Acceptance:
  `TestDCPV1322SuppliedTextOrigins`.
- `DCP-V1-323`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1571; V1-2451) The `DCP-V1-303` validity axes include
  `association`: whether the run is bound to the claimed subject and revision. A visible test-run
  claim whose `association` axis is not `passed` adds the receipt limitation `test run ID
  association is STATUS: the run does not witness its claim`; the run grants no witness and no
  trust, and an absent axis changes nothing. Acceptance:
  `TestDCPV1323AssociationAxisWithholdsWitness`.
- `DCP-V1-324`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1572; V1-2452) A supplied claim detail `coverage_ledger` records one
  flow's coverage-ledger row: `scope` (`ui` or `non_ui`), `ledger_status` (`confirmed`,
  `partial` or `not-browser`), optional `browser_evidence` (a label of at most 128 bytes),
  optional `journeys` and `ordered_steps` (integers from 0 to 1,048,576), `verification`
  (`verified`, `partial`, `unverified` or `unknown`) and `limitations` (1 to 64 distinct entries
  of at most 4,096 bytes). A value outside its set, an unknown member or a detail on a record
  other than a claim is refused (`corpus-refused`, invalid supplied coverage ledger). Acceptance:
  `TestDCPV1324To326CoverageDetailShapes`.
- `DCP-V1-325`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1572; V1-2452) A supplied claim detail `flow_completeness` records
  `target_status` (a label), `coverage_complete` and `audit_complete` (each `{value, reason}` with a
  boolean value, present and not null, and a non-empty reason), and the source `revision` (a Git object ID) and
  repository-relative `path` that back them. Any other shape is refused (`corpus-refused`,
  invalid supplied flow completeness). Acceptance: `TestDCPV1324To326CoverageDetailShapes`.
- `DCP-V1-326`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1572; V1-2452) A supplied claim detail `unresolved_concept` records
  a concept known to exist but not yet placed: `concept` (at most 1,024 bytes), `unplaced` (a
  non-empty distinct subset of `ui-location` and `flow`) and a non-empty `reason`. The claim must
  carry at least one source anchor; an unanchored concept is refused (`corpus-refused`, supplied
  unresolved concept has no anchor). Any other shape is refused (invalid supplied unresolved
  concept). Acceptance: `TestDCPV1324To326CoverageDetailShapes`.
- `DCP-V1-327`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1572; V1-2452) The `DCP-V1-324..326` details are supplied data under
  the provider trust ceiling and never derive readiness or completeness. `docs_flow` returns them
  with the flow subject's claims; `docs_gaps` returns each such claim, after the supplied gaps,
  with its detail, filtered by `id` (the subject or the claim) and by `kind` (`coverage_ledger`,
  `flow_completeness` or `unresolved_concept`); `docs_coverage` reports supplied rows
  `supplied_coverage_ledger:STATUS`, `supplied_flow_coverage_complete:true|false`,
  `supplied_flow_audit_complete:true|false` and `supplied_unresolved_concepts:unplaced`, and such
  claims no longer count toward `subjects_with_claims`. Acceptance:
  `TestDCPV1327CoverageDetailsServed`.
- `DCP-V1-328`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1573; V1-2453) `docs_search` (`corvint docs corpus search
  --include claims`) takes `include: ["claims"]`, the only admitted value: claims whose ID or
  text match the query are ranked with the subject hits under `DCP-V1-331`, and the receipt's
  `result_kinds` maps each visible hit's ID to `subject` or `claim`. Without `include` the answer
  and receipt are unchanged. Any other `include`, or `include` with a section filter, is refused
  (`invalid-arguments` over MCP, a usage error from the CLI). Acceptance:
  `TestDCPV1328SearchIncludesClaims`, `TestDCPV1328And331SearchArguments`,
  `TestDCPV1321CorpusKindAndClaimSearchCLIParity`.
- `DCP-V1-329`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1574; V1-2454) A supplied subject may carry `alternate_ids`, 1 to 32
  distinct exact strings of at most 1,024 bytes. An alternate ID equal to any record ID or
  declared twice within the provider is refused at import (`corpus-refused`, duplicate or invalid
  subject alternate id); record IDs include the joined test-knowledge, flow-relation and
  claim-audit records, stability evidence (and its test and contract IDs) and legacy cases, checked
  after they are joined. So is an empty, oversized or repeated list (invalid subject alternate
  ids). Absent, the subject's bytes are unchanged. Acceptance:
  `TestDCPV1329AlternateIDsAdmission`, `TestDCPV1329AliasCannotShadowJoinRecords`.
- `DCP-V1-330`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1574; V1-2454) An operation that takes an `id`, and `gaps`, resolves
  an `id` that names no record but is the alternate ID of exactly one subject to that subject,
  exactly and case-sensitively, and records `alias_resolution: {alternate_id, id}` in the
  receipt. A record ID always wins: an alias is applied only when `get` finds no record for the
  `id`, including a join or stability record. An ambiguous or unknown alternate ID resolves to nothing and
  returns the reader's existing typed miss. Acceptance: `TestDCPV1330AlternateIDResolution`,
  `TestDCPV1329AliasCannotShadowJoinRecords`.
- `DCP-V1-331`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1579; V1-2459) `docs_search` ranks hits by a deterministic
  binary-term-frequency BM25 score: each distinct query term contributes its inverse document
  frequency `ln(1 + (N - df + 0.5) / (df + 0.5))` over the subjects (or claims, for claim hits),
  weighted 3 when it occurs in a subject's ID or name and 1 when it occurs only in the subject's
  claim text or in a claim hit. An exact name (or claim-text) match adds one shared bonus: the
  sum, over the query terms, of each term's largest possible contribution in either family
  (subject name, and claim text when claims are included), plus 1, so an exact hit of either
  family outranks every partial hit of both. Terms are summed in sorted
  order; only equal scores order by record ID. An optional `kind` (MCP `kind`, CLI `--kind`,
  companion `kinds`; the `DCP-V1-284` shape) keeps only subjects of those kinds and claims about
  them. The indexed and scanning readers return the same order. `DCS-V1-400..403` amend the name
  field and replace the shared bonus with a per-family normalized merge. Acceptance:
  `TestDCPV1331SearchRanking`, `TestDCPV1331ExactBonusAcrossFamilies`,
  `TestDCPV1328And331SearchArguments`, `TestDCPV1321CorpusKindAndClaimSearchCLIParity`.
- `DCP-V1-332`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1570; V1-2329, V1-2371) Every read
  takes an optional caller revision (MCP `caller_revision`, CLI `--caller-revision FULL_COMMIT`,
  companion request `caller_revision`): a full lowercase hex Git object ID. An empty or malformed
  value is refused (`invalid-arguments` over MCP, exit 2 from the CLI, `query-refused` over HTTP).
  When present, `trust_envelope.caller_revision` is `{revision, source_revision, relation,
  reason}` with relation `equal`, `descendant` (the caller's commit descends from the source
  revision), `ancestor` (the source revision descends from the caller's commit), `diverged` or
  `unknown`, and a fixed reason that never echoes Git output. Readers with a Git checkout (the CLI
  and the corpus MCP bridge) decide ancestry with `git merge-base --is-ancestor`; a shallow
  repository, an absent object or a failed read is `unknown`. A reader without Git access (the
  HTTP companion and the static index reader) answers `equal` only for the identical string and
  `unknown` otherwise; it never infers ancestry. Absent, the envelope bytes are unchanged.
  Acceptance: `TestDCPV1332RelateCallerRevision`, `TestDCPV1332CallerRevisionEnvelope`,
  `TestDCPV1332CallerRevisionArgument`, `TestDCPV1332HTTPCallerRevisionUnknown`,
  `TestDCPV1332CorpusCallerRevisionCLIParity`.
- `DCP-V1-282`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1492;
  V1-2329) Each `trust_envelope.citations` entry is the cited anchor plus `anchor_state`, one of
  `live`, `moved`, `changed`, `gone` or `unknown`, from the read-time check of a live reader: an
  anchor whose declared input (primary or `DCP-V1-175` additional repository) is unchanged is
  `live`; for a changed input the live content is read inside its checkout (a clean path must
  still hash to its committed live blob) and the cited lines that still hold the pinned span
  digest are `live`, a span intact only at other lines is `moved`, an absent span is `changed`,
  and a deleted path is `gone`. An anchor not bound to a declared input with its pinned blob, a
  static reader, an unreadable or non-regular path, or a read past the per-request bounds (8 MiB
  per file, 32 MiB read, 64 MiB hashed) is `unknown`, never `live`. A receipt citing a `moved`,
  `changed` or `gone` span reports `freshness: stale` in the receipt and its envelope; a citation
  state never makes a receipt fresher than its freshness overlay. `Receipt.citations` is
  unchanged. Acceptance: `TestDCPV1282PerCitationAnchorState`.
- `DCP-V1-283`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1492;
  V1-2329) `trust_envelope.verified_at` is always present. No verification of corpus content is
  recorded, so it is `null` and the envelope limitations say so (`verified_at is null: ...`);
  `content_status` stays `generated`. Acceptance: `TestDCPV1283UnverifiedCorpusVerifiedAtNull`.
- `DCP-V1-284`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1492;
  V1-2329) `docs_gaps` accepts an optional `kind`, one gap kind or a list of 1 to 32 distinct
  non-empty kinds of at most 128 bytes, and only on `docs_gaps` and, as a subject kind filter,
  `docs_search` (`DCP-V1-331`) (`invalid-arguments` otherwise).
  The answer keeps only computed and supplied gaps of those kinds, combined with the `id`
  filter, and `kind_totals` reports for every requested kind the number of gaps it matched before
  paging, zero for a kind with none. Without `kind` the receipt is unchanged. Acceptance:
  `TestDCPV1284GapsKindFilter`.
- `DCP-V1-285`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1493;
  V1-2330) `corvint-corpus-mcp --outcome-log FILE` is off by default and, when given, records
  every `tools/call` in `corvint-corpus-outcomes/1`: the configured artifact digest, the UTC
  window of recorded calls, and per tool name the call count, outcome counts (`answered`, a
  receipt miss kind `no-match`, `empty`, `evidence-not-found`, `page-exhausted` or
  `capability-absent`, `refused` for invalid params, `error` otherwise) and a fixed latency
  histogram. Only tool names the server lists are recorded, any other name counts as `unlisted`;
  arguments, query text, ids and paths are never written. The file is rewritten atomically after
  each call; recording never changes a result, and a write failure is reported once on stderr. A
  restart continues a log for the same artifact; a foreign, malformed, inconsistent or oversized
  (1 MiB) log refuses startup. Acceptance: `TestDCPV1285OutcomeLogAggregatesWithoutRequestText`.
- `DCP-V1-286`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #1493;
  V1-2330) `corvint-corpus-mcp --outcome-summary FILE [--floor FLOOR]` prints the answered share
  overall and per tool. FLOOR is JSON `{"answered_share", "tolerance", "tools"?}` with shares in
  [0, 1]; the command exits 1 naming each regression when the overall or a floored tool's answered
  share is below its floor minus the tolerance, or that scope has no recorded call, and exits 2 for
  an unreadable or malformed log or floor. Acceptance: `TestDCPV1286OutcomeSummaryFloor`.
- `DCP-V1-217`: (accepted under the owner's standing auto-accept (2026-10-11) (V1-1991)) Legacy and
  Playwright discovery still enumerate the revision through the verified revision filesystem walk,
  which costs one Git operation per directory it lists (`DLT-V0-003`); `DCP-V1-216` bounds the
  per-path reads, not the directory count. A level-batched enumeration is a recorded residual.
- `DCP-V1-241`: `corvint docs corpus maintain --page` names the flag and the broken rule when it refuses a
  page path, before any read, with the unchanged code `corpus-refused` and error shape, for example
  `--page: maintenance requires a Markdown page relative to --root outside Git metadata (got an
  absolute path)`. The rule is the first that fails, in this order: a `DCP-V1-054` local input path
  rule, then `got a page that is not Markdown` (no `.md` extension), then `got a page inside Git
  metadata` (a `.git` component, in any case). The set of admitted pages is unchanged. Acceptance:
  `TestDCPV1241CorpusMaintainPageRefusalNamesFlag`. Status: (accepted under the owner's standing
  auto-accept (2026-10-11); V1-1227).

### Static test discovery (`docs corpus discover-tests`, GitHub #1582)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-370..376 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-370 is DCX-V1-370,
DCP-V1-371 is DCX-V1-371, DCP-V1-372 is DCX-V1-372, DCP-V1-373 is DCX-V1-373, DCP-V1-374 is
DCX-V1-374, DCP-V1-375 is DCX-V1-375, DCP-V1-376 is DCX-V1-376.

### File and claim ranking in `docs_search` (GitHub #1597)

Moved to [`documentation-corpus-search-ranking-v1.md`](documentation-corpus-search-ranking-v1.md) as
DCS-V1-400..404 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-400 is DCS-V1-400,
DCP-V1-401 is DCS-V1-401, DCP-V1-402 is DCS-V1-402, DCP-V1-403 is DCS-V1-403, DCP-V1-404 is
DCS-V1-404.

### Served rank groups and derived declarations (GitHub #1665 rows 3 and 5)

Moved to [`documentation-corpus-search-ranking-v1.md`](documentation-corpus-search-ranking-v1.md) as
DCS-V1-479..486 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-479 is DCS-V1-479,
DCP-V1-480 is DCS-V1-480, DCP-V1-481 is DCS-V1-481, DCP-V1-482 is DCS-V1-482, DCP-V1-483 is
DCS-V1-483, DCP-V1-484 is DCS-V1-484, DCP-V1-485 is DCS-V1-485, DCP-V1-486 is DCS-V1-486.

### Query words, identifier tokens and named subjects (GitHub #1665 re-test leftovers)

Moved to [`documentation-corpus-search-ranking-v1.md`](documentation-corpus-search-ranking-v1.md) as
DCS-V1-540..546 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-540 is DCS-V1-540,
DCP-V1-541 is DCS-V1-541, DCP-V1-542 is DCS-V1-542, DCP-V1-543 is DCS-V1-543, DCP-V1-544 is
DCS-V1-544, DCP-V1-545 is DCS-V1-545, DCP-V1-546 is DCS-V1-546.

### Query-word coverage, one-word named subjects and multi-repository declarations (GitHub #1665 re-test of a681846)

Moved to [`documentation-corpus-search-ranking-v1.md`](documentation-corpus-search-ranking-v1.md) as
DCS-V1-547..550 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-547 is DCS-V1-547,
DCP-V1-548 is DCS-V1-548, DCP-V1-549 is DCS-V1-549, DCP-V1-550 is DCS-V1-550.

### Content-word coverage, ID-slug names and bounded declaration derivation (GitHub #1665 re-test of ec41e4e)

Moved to [`documentation-corpus-search-ranking-v1.md`](documentation-corpus-search-ranking-v1.md) as
DCS-V1-565..569 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-565 is DCS-V1-565,
DCP-V1-566 is DCS-V1-566, DCP-V1-567 is DCS-V1-567, DCP-V1-568 is DCS-V1-568, DCP-V1-569 is
DCS-V1-569.

### Caller-owned retrieval evaluation (`docs corpus eval`)

`corvint docs corpus eval --artifact PATH --cases PATH [--baseline PATH] [--format json|table]`
scores caller-owned cases against one corpus artifact. It is a read command: it writes only
standard output, never the artifact, the cases or the baseline. Status for `DCP-V1-250..257`:
(accepted under the owner's standing auto-accept (2026-10-11); GitHub #1480; V1-2315).

- `DCP-V1-250`: The case file is a closed `corvint-corpus-eval-cases/1` document with a file
  `origin` of `labelled`, `synthetic` or `generated` and 1 to 1024 cases. Each case has a unique,
  non-empty ID of at most 256 bytes, an `operation` that a corpus MCP tool serves, an optional
  object of tool `arguments`, optional `tags` and an optional per-case `origin`, and an `expect`
  with exactly one of `abstain: true` or answers (`subjects`, `claims`, `paths`), plus optional
  `must_not` record IDs. An unknown member, duplicate member, unknown operation or origin,
  non-object arguments, an empty or contradictory expectation, a duplicate or empty list item,
  or trailing data refuses the whole file with `corpus-refused` (`eval: ...`) before any case
  runs. Acceptance: `TestDCPV1250EvalRefusesMalformedCaseFiles`.
- `DCP-V1-251`: Each case runs through `corpusbridge.Registry.CallTimed`, the code path that serves
  the `corvint.docs_*` MCP tools, with the case's arguments unchanged. Scores derive only from the
  served structured receipt; `receipt_bytes` is the served text length and `latency_ms` is the
  call's measured total. A tool failure or transport error makes the case `error` with its code,
  never a pass. Acceptance: `TestDCPV1251EvalScoresThroughTheMCPToolPath`.
- `DCP-V1-252`: Each case reports `hit`, the 1-based `rank` of the first relevant row, `k` (the
  case's `limit`, else 20), `returned`, `precision_at_k` and `recall_at_k` as numerator,
  denominator and value, `abstained` (zero rows), an `abstention` class, `must_not_violations` and
  `unresolved` (amended by `DCX-V1-473`: a failing case also lists its returned rows). A row is
  relevant when its record ID, subject, nested relation record or subject,
  path or evidence anchor path matches an expected answer. A case passes only with no `must_not`
  violation, no unresolved expectation, and a hit (answer case) or an abstention (abstain case).
  Acceptance: `TestDCPV1251EvalScoresThroughTheMCPToolPath`.
- `DCP-V1-253`: An expected subject, claim, path or `must_not` ID that the artifact does not
  contain is listed in `unresolved` with its kind. It stays in the recall denominator as a miss and
  fails the case; it is never dropped or scored as found. Summaries (`overall`, `by_operation`,
  `by_tag`, `by_origin`) carry every ratio's numerator and denominator and the member case IDs;
  errored cases stay in the denominators. Acceptance:
  `TestDCPV1251EvalScoresThroughTheMCPToolPath`.
- `DCP-V1-254`: The report binds the case file (path, SHA-256, origin, count) and the corpus
  (artifact SHA-256, repository ID, revision, tree, profile, manifest and builder) and states its
  limitations. `--baseline` reads a prior report strictly and compares each case by status,
  rank, recall, precision, `must_not` count and abstention correctness, never by latency or bytes.
  Every regressed case is listed with its reasons and the command exits 1 with the report; an
  improvement never offsets a regression. Acceptance:
  `TestDCPV1254BaselineListsEveryRegression`, `TestDCPV1257CorpusEvalCLI`.
- `DCP-V1-255`: The comparison identity is the case-file SHA-256, the repository ID, the profile ID
  and the served receipt schema; a rebuilt artifact of the same repository and profile stays
  comparable. When any identity member or the case count differs, the baseline is
  `comparable: false` with the reasons (`case file changed`, `corpus identity changed`, `served
  receipt schema changed`), no case is compared, and the command exits 1. Acceptance:
  `TestDCPV1255ChangedIdentityIsNotComparable`.
- `DCP-V1-256`: A case inherits the file origin unless it names its own; a synthetic or generated
  file cannot contain a `labelled` case. Each result and `by_origin` group keeps its origin, so a
  synthetic score is never reported as a labelled one. Acceptance:
  `TestDCPV1250EvalRefusesMalformedCaseFiles`, `TestDCPV1251EvalScoresThroughTheMCPToolPath`.
- `DCP-V1-257`: `--format json` (the default) prints the deterministic report; `--format table`
  prints a per-case and per-group text view of the same report. An unknown format, a missing
  `--cases` or an unreadable local input is an argument or input error (exit 2); a refused case
  file or unavailable corpus is `corpus-refused`. `--cases` and `--baseline` follow the
  `DCP-V1-054` local input path rule. Acceptance: `TestDCPV1257CorpusEvalCLI`,
  `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags`.

### Claim audit sampling, verdicts and measured accuracy (`docs corpus audit`)

`corvint docs corpus audit sample --artifact PATH --seed N --size N [--stratify none|provider|kind|flow|trust] [--scope PATH]`
draws a reproducible claim sample, and `corvint docs corpus audit report --artifact PATH --sample PATH [--format json|table]`
measures accuracy from independent verdict records joined at build time. Both are read commands:
they write only standard output. A verdict measures correctness; it never changes a claim, its
evidence or its trust label. Status for `DCP-V1-260..268`: accepted under the owner's standing
auto-accept (2026-10-11); GitHub #1481; V1-2316.

- `DCP-V1-260`: A claim-audit provider (manifest `/2` only) carries only a closed
  `claim_audits` member of 1 to 4096 `corvint-corpus-claim-audit/1` records. Each record has a
  unique provider-qualified `id`, a `claim_id`, the claim's audit `span_sha256`, a `verdict` of
  `correct`, `incorrect`, `unsupported-by-citation` or `unverifiable`, a non-secret `reviewer`, a
  boolean `reviewer_independent_of_generator`, a `method` of `human`, `model` or `test`, 0 to 64
  `evidence` anchors and an optional bounded `note`. An unknown member, unknown verdict or method,
  secret-shaped reviewer or note, invalid anchor pin, evidence anchor whose binding does not match
  the declared input, other record kinds in the same provider, or an aggregate over 4096 refuses the
  build with `corpus-refused`. An evidence anchor on an undeclared path stays visible as an
  `unanchored` limitation. Joined verdicts are a separate artifact member that `Open` rederives;
  the audited claim is never rewritten. Acceptance: `TestDCPV1260ClaimAuditSchemaIsClosed`.
- `DCP-V1-261`: The audit span hash is SHA-256 over the schema, claim ID, claim text and, in order,
  each evidence anchor's repository, path, line range and span hash (`ClaimAuditSpan`; `audit
  sample` emits it per claim). A verdict whose span hash differs from the current claim's is
  `stale`, one whose claim is absent is `unresolved`; neither enters the headline figure, and the
  report counts stale verdicts beside it. Acceptance: `TestDCPV1261StaleClaimAuditVerdictIsExcluded`.
- `DCP-V1-262`: A verdict is independent only when the record says so and neither its reviewer nor
  its audit provider is the claim's provider. A `model` verdict on a claim whose derivation is
  `generated` is a generator-method verdict. Only a current, independent, non-generator verdict is
  a headline verdict; every other current verdict keeps its reasons and is excluded. Acceptance:
  `TestDCPV1262NonIndependentAndGeneratorVerdictsStayOutOfHeadline`.
- `DCP-V1-263`: `get`, `trace`, `claims` and every receipt that returns a claim attach
  `claim_audits` keyed by claim ID (current verdicts first, at most 32 per claim, with a
  withholding limitation beyond that), on the v1 and paginated v2 receipts and through the
  `corvint.docs_*` MCP tools. The claim keeps the text, evidence state and trust label it has in
  the same corpus built without audits, and the receipt trust is unchanged. Acceptance: `TestDCPV1263ClaimAuditViewsKeepTrust`,
  `TestDCPV1263MCPGetShowsClaimAudit`.
- `DCP-V1-264`: `gaps` reports `stale-claim-audit` and `unresolved-claim-audit` on the verdict, and
  every current `incorrect` verdict as `claim-audit-incorrect` on the claim and on its subject
  (the flow, when the subject is a flow), labelled `independent` or `non-headline`. Acceptance:
  `TestDCPV1261StaleClaimAuditVerdictIsExcluded`, `TestDCPV1263ClaimAuditViewsKeepTrust`.
- `DCP-V1-265`: `audit sample` takes a seed in 0..2^53-1, a size in 1..1024, an optional
  stratification (`provider`, `kind`, `flow` or `trust`; default `none`) and at most one `--scope`
  path matched against claim evidence anchor paths. Within each stratum claims are ranked by
  SHA-256 of the seed and claim ID; sizes are allocated proportionally by largest remainder (ties by
  stratum name); claims are emitted sorted by ID with their stratum, provider, derivation, trust,
  span hash and anchors. The same artifact, seed and options give byte-identical output. The sample
  binds the artifact SHA-256, repository, tree, profile and manifest, and records population and
  sampled counts per stratum; an empty population yields an empty sample with a limitation.
  Invalid options refuse. Acceptance: `TestDCPV1265AuditSampleIsDeterministic`.
- `DCP-V1-266`: `audit report` reads the sample strictly (closed `corvint-corpus-audit-sample/1`,
  unique non-empty claim IDs) and refuses a sample whose repository or profile differs. Each sampled
  claim is `missing` (not in this corpus), `unaudited` (no headline verdict), `conflicted` (headline
  verdicts disagree) or its single headline verdict. Accuracy is correct / (correct + incorrect +
  unsupported-by-citation) with a 95% Wilson score interval; `unverifiable` is counted separately.
  Every ratio carries its numerator and denominator; a zero denominator is undefined and the report
  abstains with a limitation instead of a value. Groups are `overall`, `by_stratum` and `by_method`
  (headline verdicts only); the pooled figure is labelled as not population-weighted. Acceptance:
  `TestDCPV1265AuditSampleIsDeterministic`, `TestDCPV1268CorpusAuditCLI`.
- `DCP-V1-267`: Excluded current verdicts (non-independent and generator-method counts, verdict
  counts and their own accuracy) are reported per group beside, never inside, the headline figure;
  unaudited, conflicted and missing claims stay outside the denominator and are listed. Acceptance:
  `TestDCPV1262NonIndependentAndGeneratorVerdictsStayOutOfHeadline`.
- `DCP-V1-268`: `audit sample` and `audit report` are also spelled `audit-sample` and
  `audit-report` with identical output. `--format json` (report default) prints the deterministic
  report and `--format table` its text view (`abstain (0/0)` when undefined). A missing or invalid
  `--seed`, `--size`, `--stratify` or `--sample`, more than one `--scope`, or an unknown format is an
  argument error (exit 2); `--sample` follows the `DCP-V1-054` local input path rule. Acceptance:
  `TestDCPV1268CorpusAuditCLI`, `TestDocsCorpusOperationFlagMatrix`,
  `TestDocsCorpusOperationHelpPrintsFlags`.

### Verified fast corpus reads (GitHub #1598)

Moved to [`documentation-corpus-verified-reads-v1.md`](documentation-corpus-verified-reads-v1.md) as
DCR-V1-410..419 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-410 is DCR-V1-410,
DCP-V1-411 is DCR-V1-411, DCP-V1-412 is DCR-V1-412, DCP-V1-413 is DCR-V1-413, DCP-V1-414 is
DCR-V1-414, DCP-V1-415 is DCR-V1-415, DCP-V1-416 is DCR-V1-416, DCP-V1-417 is DCR-V1-417, DCP-V1-418
is DCR-V1-418, DCP-V1-419 is DCR-V1-419.

### Streamed reads and verified reuse on the remaining paths (GitHub #1598 residual)

Moved to [`documentation-corpus-verified-reads-v1.md`](documentation-corpus-verified-reads-v1.md) as
DCR-V1-430..434 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-430 is DCR-V1-430,
DCP-V1-431 is DCR-V1-431, DCP-V1-432 is DCR-V1-432, DCP-V1-433 is DCR-V1-433, DCP-V1-434 is
DCR-V1-434.

### Parallel boundary scan of split members (V1-2571)

Moved to [`documentation-corpus-verified-reads-v1.md`](documentation-corpus-verified-reads-v1.md) as
DCR-V1-551..553 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-551 is DCR-V1-551,
DCP-V1-552 is DCR-V1-552, DCP-V1-553 is DCR-V1-553.

### Read hot spots: scope matching and stat-routed revision pins (GitHub #1610)

Moved to [`documentation-corpus-verified-reads-v1.md`](documentation-corpus-verified-reads-v1.md) as
DCR-V1-450..453 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-450 is DCR-V1-450,
DCP-V1-451 is DCR-V1-451, DCP-V1-452 is DCR-V1-452, DCP-V1-453 is DCR-V1-453.

### Verified delta reads (GitHub #1634)

Moved to [`documentation-corpus-verified-reads-v1.md`](documentation-corpus-verified-reads-v1.md) as
DCR-V1-460..464 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-460 is DCR-V1-460,
DCP-V1-461 is DCR-V1-461, DCP-V1-462 is DCR-V1-462, DCP-V1-463 is DCR-V1-463, DCP-V1-464 is
DCR-V1-464.

### Read-start freshness and decode allocation (GitHub #1598 residual)

Moved to [`documentation-corpus-verified-reads-v1.md`](documentation-corpus-verified-reads-v1.md) as
DCR-V1-465..468 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-465 is DCR-V1-465,
DCP-V1-466 is DCR-V1-466, DCP-V1-467 is DCR-V1-467, DCP-V1-468 is DCR-V1-468.

### Cross-revision delta worklist and carry-forward (`docs corpus delta`, `docs corpus carry-forward`)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-360..369 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-360 is DCX-V1-360,
DCP-V1-361 is DCX-V1-361, DCP-V1-362 is DCX-V1-362, DCP-V1-363 is DCX-V1-363, DCP-V1-364 is
DCX-V1-364, DCP-V1-365 is DCX-V1-365, DCP-V1-366 is DCX-V1-366, DCP-V1-367 is DCX-V1-367, DCP-V1-368
is DCX-V1-368, DCP-V1-369 is DCX-V1-369.

### Claim mention check and corpus patterns (`docs corpus patterns`)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-340, DCX-V1-341, DCX-V1-342, DCX-V1-343, DCX-V1-344, DCX-V1-345, DCX-V1-346, DCX-V1-347,
DCX-V1-348, DCX-V1-349, DCX-V1-350, DCX-V1-351, DCX-V1-352, DCX-V1-420, DCX-V1-421, DCX-V1-422,
DCX-V1-423, DCX-V1-424, DCX-V1-425, DCX-V1-440, DCX-V1-441, DCX-V1-442, DCX-V1-443, DCX-V1-444,
DCX-V1-445 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-340 is DCX-V1-340,
DCP-V1-341 is DCX-V1-341, DCP-V1-342 is DCX-V1-342, DCP-V1-343 is DCX-V1-343, DCP-V1-344 is
DCX-V1-344, DCP-V1-345 is DCX-V1-345, DCP-V1-346 is DCX-V1-346, DCP-V1-347 is DCX-V1-347, DCP-V1-348
is DCX-V1-348, DCP-V1-349 is DCX-V1-349, DCP-V1-350 is DCX-V1-350, DCP-V1-351 is DCX-V1-351,
DCP-V1-352 is DCX-V1-352, DCP-V1-420 is DCX-V1-420, DCP-V1-421 is DCX-V1-421, DCP-V1-422 is
DCX-V1-422, DCP-V1-423 is DCX-V1-423, DCP-V1-424 is DCX-V1-424, DCP-V1-425 is DCX-V1-425, DCP-V1-440
is DCX-V1-440, DCP-V1-441 is DCX-V1-441, DCP-V1-442 is DCX-V1-442, DCP-V1-443 is DCX-V1-443,
DCP-V1-444 is DCX-V1-444, DCP-V1-445 is DCX-V1-445.

### Repository-declared extractor profiles (issue 1583)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-390..399 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-390 is DCX-V1-390,
DCP-V1-391 is DCX-V1-391, DCP-V1-392 is DCX-V1-392, DCP-V1-393 is DCX-V1-393, DCP-V1-394 is
DCX-V1-394, DCP-V1-395 is DCX-V1-395, DCP-V1-396 is DCX-V1-396, DCP-V1-397 is DCX-V1-397, DCP-V1-398
is DCX-V1-398, DCP-V1-399 is DCX-V1-399.

### Whole-repository extraction, delta argument presence and derivation notes (GitHub #1700, #1667)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-560..564 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-560 is DCX-V1-560,
DCP-V1-561 is DCX-V1-561, DCP-V1-562 is DCX-V1-562, DCP-V1-563 is DCX-V1-563, DCP-V1-564 is
DCX-V1-564.

### Experimental compact per-file anchor table (GitHub #1664)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-487..489 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-487 is DCX-V1-487,
DCP-V1-488 is DCX-V1-488, DCP-V1-489 is DCX-V1-489.

### Corpus rebuild from the embedded manifest (GitHub #1667)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-495..498 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-495 is DCX-V1-495,
DCP-V1-496 is DCX-V1-496, DCP-V1-497 is DCX-V1-497, DCP-V1-498 is DCX-V1-498.

### Compact provider producer and rebuild reuse (GitHub #1664, #1667)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-530..534 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-530 is DCX-V1-530,
DCP-V1-531 is DCX-V1-531, DCP-V1-532 is DCX-V1-532, DCP-V1-533 is DCX-V1-533, DCP-V1-534 is
DCX-V1-534.

### Corpus diagnostics, builder upgrades, eval cost and ranking residuals (GitHub #1662, #1664 to #1667)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-470, DCX-V1-471, DCX-V1-472, DCX-V1-473, DCX-V1-474, DCX-V1-475, DCX-V1-476, DCX-V1-477,
DCX-V1-478, DCX-V1-503, DCX-V1-504, DCX-V1-505, DCX-V1-506 (V1-2525), because this spec's
Requirements section was over the 256-obligation OCM limit. Each former ID keeps its number under
the new prefix. Crosswalk: DCP-V1-470 is DCX-V1-470, DCP-V1-471 is DCX-V1-471, DCP-V1-472 is
DCX-V1-472, DCP-V1-473 is DCX-V1-473, DCP-V1-474 is DCX-V1-474, DCP-V1-475 is DCX-V1-475, DCP-V1-476
is DCX-V1-476, DCP-V1-477 is DCX-V1-477, DCP-V1-478 is DCX-V1-478, DCP-V1-503 is DCX-V1-503,
DCP-V1-504 is DCX-V1-504, DCP-V1-505 is DCX-V1-505, DCP-V1-506 is DCX-V1-506.

### Extraction enumeration of large revisions (GitHub #1662 residual)

Moved to
[`documentation-corpus-extraction-delta-v1.md`](documentation-corpus-extraction-delta-v1.md) as
DCX-V1-520..522 (V1-2525), because this spec's Requirements section was over the 256-obligation OCM
limit. Each former ID keeps its number under the new prefix. Crosswalk: DCP-V1-520 is DCX-V1-520,
DCP-V1-521 is DCX-V1-521, DCP-V1-522 is DCX-V1-522.

### Compiled-input as-of points and live lag (GitHub #1836)

A provider, observation or evidence input is compiled from an upstream source that can move after
the build. Its Git pin proves which bytes the corpus holds, not how far that source has advanced
since. The issue's `corpus_info`, `stale_state` and `stale_overlay_at` names are not Corvint fields;
they map to the `info` result plus the embedded manifest, to `freshness` (`fresh`, `stale`,
`unknown`) and to the live freshness overlay. The overlay records no timestamp.

- `DCP-V1-574`: (proposed; GitHub #1836; V1-2730) A compiled input (purpose `provider`,
  `observation` or `evidence`) may declare `as_of: {source, kind, value, probe?}` in the manifest
  (`corvint-corpus-input/1` and `/2`). The manifest is the artifact's build provenance. `source` is
  1 to 128 bytes of UTF-8 without control characters. `kind` is `timestamp` (RFC 3339 value),
  `sha256` (64 lowercase hex characters) or `sequence` (canonical unsigned decimal). `probe` is an
  optional canonical relative path in the primary checkout with no segment equal to `.git` in any
  letter case. Any other shape, including an explicit empty `as_of: {}`, or an `as_of` on a
  `source` input, refuses the build with `invalid input as_of`. An undeclared `as_of`
  is omitted, so existing manifests and artifacts keep their bytes. Acceptance:
  `TestDCPV1574InputAsOfValidation`.
- `DCP-V1-575`: (proposed; GitHub #1836; V1-2730) A live reader probes each declared input once
  per freshness overlay, and reads only the probe file. It uses the corpus reader's no-follow
  bounded file access. For `sha256`, it compares the probe's digest: equal is `fresh`, different
  is `stale` with no measurable lag. For `sequence`, it reads at most 64 bytes holding one decimal:
  equal is `fresh`; greater is `stale` with the lag in `entries`; smaller, or a value that does not
  parse, is `unknown`. For `timestamp`, it compares the probe's modification time, not its
  content: later than the value is `stale` with the lag in whole `seconds`; otherwise `fresh`. An
  input with no probe, or whose probe is missing, unsafe or over bound, is `unknown` with the
  reason. Probing never fails the read; only cancellation does. Acceptance:
  `TestDCPV1575DCPV1576InputAsOfLiveStates`.
- `DCP-V1-576`: (proposed; GitHub #1836; V1-2730) Every answer from an artifact that declares an
  as-of input carries a per-input limitation for each such input that is not `fresh`. A stale
  input gives `compiled input <path> (<source>) as of <kind> <value>; source has advanced[ by <lag>
  <unit>]: <reason>`. An unknown input gives `...; source position NOT_OBSERVED: <reason>`. The
  answer's `freshness` and `trust_envelope.freshness` are never `fresh` while any declared input is
  not: a stale input makes them `stale`, and an unknown input turns `fresh` into `unknown`. A
  reader with no live overlay reports every declared input `unknown` with the reason `no live
  reader probed the source`. Examples are the embedded companion index and `Query`/`QueryContext`
  callers. `docs corpus maintain` therefore refuses while a declared input is not fresh.
  Attribution is artifact-wide, as the existing overlay is: every answer counts as having used
  each declared input. Per-record attribution is `NOT_OBSERVED`. Limitations remain under the
  `/2` freshness diagnostic bound, which counts the reasons it withholds. Acceptance:
  `TestDCPV1575DCPV1576InputAsOfLiveStates`.
- `DCP-V1-577`: (proposed; GitHub #1836; V1-2730) The overlay reuse key (`CorpusObservation` and
  the overlay's `Observation`) appends the lstat record of every declared probe: size, modification
  time and mode, or `absent`. A probe compared by content (`sha256` or `sequence`) also appends its
  content digest, so a rewrite that keeps size and modification time still changes the key. A probe
  changed outside Git's status view, such as an ignored or
  untracked file, therefore forces a new overlay. A probe modified inside the 2-second racy window
  gets a key no later observation equals. An artifact without probes keeps the prior key.
  Acceptance: `TestDCPV1577ProbeChangesObservation`.
- `DCP-V1-578`: (proposed; GitHub #1836; V1-2730) The `info` and `validate` result lists
  `inputs_as_of` when any input declares `as_of`. Each entry has `path`, `revision`, `purpose`,
  `as_of`, `state`, an optional `live` position, `lag` and `lag_unit` when measurable, and
  `reason`. A republish (`corvint-corpus-republish`) can be chosen from that list. Wiring that tool
  to read it is not delivered here. Acceptance: `TestDCPV1575DCPV1576InputAsOfLiveStates`.
- `DCP-V1-579`: (proposed; V1-2865; experimental under the standing lane auto-accept) The CLI
  `--include` flag help, the `corvint.docs_search` MCP tool description and the
  `docs/DOCUMENTATION-CORPUS.md` guide each state that `include: ["claims"]` on a search adds claim
  hits to the subject hits, ranked together and labelled in `result_kinds` (`DCP-V1-328`), and point
  to the `claims` operation for claims only. Non-goal: search behavior, schemas and receipts are
  unchanged; only text changes. Failure mode: an adopter reading "include" as "restrict to" writes
  wrong case expectations (GitHub #1665 retest). Rollback: revert the three texts and delete the
  two tests; `SearchIncludeDescription` goes with them. Acceptance:
  `TestDCPV1579SearchIncludeHelpSaysClaimsAreAdded` (`cmd/corvint`) and
  `TestDCPV1579SearchDescriptionSaysClaimsAreAdded` (`internal/mcp/corpusbridge`) pin the text.

### Workspace repositories (GitHub #1956)

An application and its end-to-end test repository often live apart. A corpus built from one
`--root` never turned the test repository's docs and page objects into subjects or claims, so
`trace` and `gaps` returned nothing for them. These requirements reuse the shared workspace
declaration (`docs/specs/workspace-declaration-v0.md`, `workspace.Load`) rather than a new reader.

- `DCP-V1-600`: (proposed; GitHub #1956; V1-2975; experimental under the standing lane auto-accept)
  A `DCP-V1-175` additional repository may carry `alias`, a workspace alias
  (`[a-z][a-z0-9-]{0,31}`) distinct across repositories, omitted when empty. `docs corpus
  manifest` with a bound workspace inventories `--root` as before and each further alias as an
  additional repository with that alias. It is pinned at the alias's declared `revision`, else its
  `HEAD`, resolved to a full commit, and identified by its root commit. Its `root` is relative to
  `--root`. Its scopes and native inputs are pinned at that commit. A repeated or invalid alias is
  refused (`corpus-refused`).
- `DCP-V1-601`: (proposed; GitHub #1956; V1-2975; experimental under the standing lane auto-accept)
  The native provider admits inputs at an aliased additional repository's declared `revision`
  (not its further `revisions`) and reads that repository's index. Its file, symbol, excerpt,
  paragraph, import and gap IDs put `ALIAS:` before the path, for example
  `native:file:e2e:pages/login.md`. Anchors name that repository and revision. An additional
  repository without an alias still admits no native input. This amends the primary-only native
  provider of `DCP-V1-177` and `DCP-V1-179` for aliased repositories only.
- `DCP-V1-602`: (proposed; GitHub #1956; V1-2975; experimental under the standing lane auto-accept)
  `trace` takes exactly one of `--id` or `--path`, and `gaps` takes at most one. A path resolves to
  the native file subject ID of that path in `--repository`, else in the primary repository. The
  receipt records `path_resolution` `{repository, path, id}` and otherwise answers as for the ID.
  `--repository`, here and on `locate` and `recommend-tests`, accepts a declared alias as well as a
  root commit. It is refused on `trace` and `gaps` without `--path` (`invalid-arguments`), and for
  an undeclared repository (`corpus-refused`). The MCP tools are unchanged.
- `DCP-V1-603`: (proposed; GitHub #1956; V1-2975; experimental under the standing lane auto-accept)
  Without a bound workspace, or with `--no-workspace`, the manifest is the single-root manifest
  byte for byte. Its artifact keeps its bytes and its unqualified native IDs, with no `alias` or
  `additional_repositories` member.
- `DCP-V1-604`: (proposed; GitHub #1956; V1-2975; experimental under the standing lane auto-accept)
  Every `docs corpus` operation admits `--workspace FILE`, `--no-workspace` and repeated
  `--repo ALIAS=ROOT`, the flag form of a workspace. `--workspace` with `--repo` or
  `--no-workspace` is refused (`invalid-arguments`), as is a malformed `--repo`. A named workspace,
  else the `--repo` flags, binds; only `manifest` also discovers `.corvint/workspace.json` under
  `--root` (`WSD-V0-003`). The alias whose checkout is `--root` is the primary. A discovered or
  named workspace with no alias at `--root` is refused (`corpus-refused`). `manifest` routes
  `--scope ALIAS:PATH` to that alias and every other scope to `--root`. An alias with no scope
  would be ignored and is refused, naming it (`WSD-V0-005`). `build`, and each operation that opens
  an artifact through the verified read path, refuses a bound alias that the corpus does not
  declare at the same checkout (`corpus-refused`). `extract --repository ALIAS` reads that bound
  alias's checkout. The other operations read no repository, so the workspace is validated and
  otherwise unused.
- `DCP-V1-605`: (proposed; GitHub #1956; V1-2975; experimental under the standing lane auto-accept)
  A records-provider relation may join subjects of different repositories, for example an
  application doc `tested_by` a test-repository page. Its evidence is anchored in the repository
  that owns it. `trace` on either endpoint, by ID or by path, and `related`, return it.
  Non-goals: inferring cross-repository relations, workspace support in the corpus MCP bridge, and
  qualifying `DCS-V1-549` derived declaration IDs by alias. Failure modes: a moved checkout makes
  the corpus `corpus-input-unavailable` (`DCP-V1-176`), and a primary path spelled `ALIAS:PATH`
  can collide with an aliased ID and is refused as a duplicate record. Rollback: revert the
  `alias` member, the native qualifier, the path form and the workspace flags; manifests without
  `alias` are unaffected. Acceptance: `TestDCPV1600WorkspaceManifestPinsEachRepository`,
  `TestDCPV1600WorkspaceAliasRefusals`, `TestDCPV1602TraceAndGapsByPathInEitherRepository`,
  `TestDCPV1603SingleRootManifestByteIdentical`,
  `TestDCPV1605CrossRepositoryRelationTracesBothWays` (`internal/doccorpus`),
  `TestDCPV1604DocsCorpusWorkspaceManifest`, `TestDCPV1604DocsCorpusWorkspaceTraceAndGaps` and
  `TestDocsCorpusOperationFlagMatrix` (`cmd/corvint`).

- `DCP-V1-606`: (proposed; V1-3006; experimental under the standing lane auto-accept) The
  companions' artifact and request reads (`corpusindex.ReadFile` and `ReadFileContext`) MUST
  refuse a path that is not a regular file from its metadata before opening it, open without
  blocking (`O_NONBLOCK` on Darwin and Linux) and require the opened descriptor to be that regular
  file, read at most the byte limit, and check cancellation between bounded chunks, so a FIFO with
  no writer or a stalled read never hangs `corvint-corpus-http` startup. Failure modes: a platform
  without a nonblocking open relies on the metadata check alone. Rollback: restore the unchecked
  `os.Open` read.

### Named generated-input refusals (GitHub #1960)

`docs corpus build` refused a generated source input with only "generated output cannot be source
input", naming neither the file nor the rule, and stopped at the first one, so an operator had to
bisect the scope to find each offending file.

- `DCP-V1-607`: (proposed; GitHub #1960; V1-3053; experimental under the standing lane auto-accept)
  The `DCP-V1-003` generated-output refusal keeps the code `corpus-refused` and its leading text and
  names every `source` input that a generated-output check matched, in manifest input order, each
  as `repository=LABEL path=PATH revision=REVISION check=CHECK`, after the input count. `LABEL` is
  the `DCP-V1-600` alias of an aliased additional repository, `primary` for the primary repository,
  else the additional repository's identity. `CHECK` is the first matching check in this order:
  `corpus-marker` (a `<!-- corvint-corpus` marker), `corpus-schema` (a JSON object whose `schema`
  is a corpus schema), `code-generated-comment` (a `// Code generated ` or `# Code generated ` line),
  then the native `generated-path` and `generated-header` rules (`IDX-SNAP-V0-018`). The refusal is
  raised after every input has been read and verified, so an input changed after pinning still
  refuses first.
- `DCP-V1-608`: (proposed; GitHub #1960; V1-3053; experimental under the standing lane auto-accept)
  `docs corpus manifest`, single-root or workspace, applies the same checks to every input it pins
  and refuses with the `DCP-V1-607` text naming every generated input of every repository. Such a
  manifest could never build, and dropping the input would leave it undeclared in its scope
  (`DCP-V1-003`), so the remedy is a narrower scope. The checked inventory (`ManifestInventory`,
  `InventoryWorkspace`) still returns the pinned manifest alongside the refusal. The library
  `InventoryScopes` stays unchecked, because a caller such as flows finalize inventories a corpus
  provider record and then changes its purpose to `provider`. A scope without generated output
  inventories byte for byte as before.
  Non-goals: excluding generated inputs automatically, a new purpose for generated files, and
  changing which files count as generated. Failure modes: a scope with very many generated files
  yields a long single-line refusal, bounded by the schema's record limit. Rollback: restore the
  unnamed first-match refusal at build and the unchecked inventory. Acceptance:
  `TestDCPV1607BuildRefusalNamesEveryGeneratedInput`, `TestDCPV1607GeneratedHeaderCheckIsNamed`,
  `TestDCPV1608ManifestRefusesGeneratedInputsWhenPinning` (`internal/doccorpus`) and
  `TestDCPV1608DocsCorpusManifestNamesGeneratedInputs` (`cmd/corvint`).


- `DCP-V1-650`: (proposed; GitHub #1972; V1-3070; experimental under the standing lane auto-accept)
  `locate` (`corvint.docs_locate`) also takes a `query` (1 to 1024 bytes) in place of `path` or
  `paths`. The MCP tool takes exactly one of `path`, `paths` or `query`; a present empty `path`
counts as a path, here and for `docs_claims` (`DCP-V1-313`). A locate query ranks
  subjects as `search` does (`DCP-V1-331`, `DCS-V1-479`) but only `module` and `symbol`
  subjects. The receipt keeps operation `locate` and carries `result_ranks`. An optional `kind`,
  `module` or `symbol` or a list of them, narrows the query. Any other kind, a `kind` on the path
  form, or a `repository` on the query form is `invalid-arguments`. The library query refuses a
  foreign kind or a repository on a locate query (`corpus-refused`). A library request with both a
  path and a query keeps the path form, as permissive pure callers expect. Test knowledge never
  answers a locate query, so without the locate capability it is `unavailable`. With an empty path,
  `result_repositories` names each result's first anchor's repository. The path form, its
  `repository` qualifier and its receipt bytes are unchanged.
- `DCP-V1-651`: (proposed; GitHub #1972; V1-3070; experimental under the standing lane auto-accept)
  A locate query receipt carries `result_locations`. For each visible result ID, it lists that
  subject's line anchors in anchor order, each as `{repository, path, start_line, end_line,
  symbol?}`. An anchor without a line span is not listed. Every other receipt omits the member and
  keeps its bytes.
- `DCP-V1-652`: (proposed; GitHub #1973; V1-3071; experimental under the standing lane auto-accept)
  Started without `--protocol-version`, `corvint-corpus-mcp` speaks 2026-07-28. It answers a
  2025-11-25 `initialize` with `-32602`, the unsupported-version code a 2025-11-25 client expects,
  not `-32601`. The message names the version it speaks and the flag
  `--protocol-version 2025-11-25`. Its `data` is `{supported: ["2026-07-28"], flag, requested?}`;
  `requested` is echoed only when it is a non-empty string within the protocol-version byte bound.
  The answer comes from the shared server's opt-in `NameLegacyFlag`. The other legacy methods and
  servers that do not opt in keep the `MCPV0-006` `-32601`. The server keeps serving after the
  answer.
  Non-goals: version negotiation in one process, a default of 2025-11-25, the other MCP servers
  (V1-2507, V1-2513), the CLI `docs corpus locate`, and a repository qualifier on a locate query.
  Failure modes: a locate query that matches no module or symbol subject is `empty`/`no-match`; a
  module or symbol subject anchored without a line span is returned with an empty location list.
  Rollback: drop `query` and `kind` from `docs_locate`, the locate-query branch and
  `result_locations`, and the `NameLegacyFlag` option. Acceptance:
  `TestDCPV1650LocateByQueryReturnsAnchoredSourceSubjects` (`internal/doccorpus`),
  `TestDCPV1650DocsLocateQuery` (`internal/mcp/corpusbridge`),
  `TestDCPV1652LegacyInitializeNamesVersionAndFlag` (`internal/mcp/server`) and
  `TestDCPV1652LegacyInitializeWithoutFlagNamesIt` (`cmd/corvint-corpus-mcp`).
- `DCP-V1-653`: (proposed; GitHub #1977; V1-3080; experimental under the standing lane auto-accept)
  Amending `DCP-V1-016` for `corvint.docs_search` only: `corvint-corpus-mcp` answers it with the
  compact receipt by default, in both the structured content and the framed text. The receipt is
  derived from the full receipt by `DCP-V1-654` and `DCP-V1-655` and carries
  `receipt_view: "compact"`. An optional `receipt` argument, `compact` or `full`, selects the view.
  `full` returns the full receipt with the `DCP-V1-016` CLI byte parity, and so does every other
  tool. Any other `receipt` value, or `receipt` on another tool, is `invalid-arguments`. The CLI,
  the HTTP reader and the bridge `Call` used by corpus evaluation keep the full receipt.
- `DCP-V1-654`: (proposed; GitHub #1977; V1-3080; experimental under the standing lane auto-accept)
  A compact receipt omits `capabilities`, which describe the corpus, not the query; `docs_info`
  serves them. Amending `DCP-V1-328`, the `docs_search` `include` argument is a non-empty,
  duplicate-free list of `claims` and `capabilities`. `capabilities` keeps the member in a compact
  receipt; the corpus query sees `include ["claims"]` only when `claims` is listed.
- `DCP-V1-655`: (proposed; GitHub #1977; V1-3080; experimental under the standing lane auto-accept)
  A compact receipt has a top-level `anchors` table. It lists each distinct anchor once, by exact
  member equality, in first-seen order over `citations`, each result's `evidence.anchors`, the
  trust envelope's `citations`, then `result_ranks` in result id order (`match_anchor`,
  `DCS-V1-581`). Each entry is the anchor with an `id` (`a1`, `a2`, ...) and, when the trust
  envelope cites it, that citation's `anchor_state`. Every one of those places holds the id
  instead of the anchor. A `symbol_anchors` check names its table anchor by id, and its `live`
  location becomes that id when it equals the anchor's pinned revision, path, blob, digest, line
  span and span digest. The optional `declaration_line: 0` and `estimated: false` defaults
  emitted by the canonical doccorpus encoder are ignored for this comparison; a nonzero
  declaration line, `estimated: true`, an unknown member, or a missing pin member prevents
  collapse (V1-3095, GitHub #1992). Full receipt encoding is unchanged. A moved location stays
  an object. A check whose anchor is not in the table is unchanged. The top-level `limitations` lists each distinct limitation once, the
  receipt's own first, then the evidence and the trust envelope's in first-seen order. Each
  result's `evidence.limitations` and the trust envelope's `limitations` hold indexes into it.
  Every other member is the full receipt's. Expanding every id and index rebuilds each holder's
  anchors and limitations exactly.
  Non-goals: a compact view for other tools, a file table shared by anchors of one file,
  deduplicating the structured content against the text block, and changing the CLI or the
  corpus receipt schema. Failure modes: a client that read anchor objects or limitation strings
  in those places sees ids or indexes until it asks for `receipt: "full"`; the receipt's own
  limitations are no longer separable from the evidence's in the compact view. Rollback: drop the
  `receipt` argument, `capabilities` from `include` and the compaction in
  `cmd/corvint-corpus-mcp`. Acceptance: `TestDCPV1653CompactReceipt` and
  `TestDCPV1655CompactReceiptEncodedSymbolLocation` (`internal/mcp/corpusbridge`)
  and `TestDCPV1653SearchCompactReceiptOnTheWire` (`cmd/corvint-corpus-mcp`), with the
  `DCP-V1-016` parity tests asking for `receipt: "full"`; measured sizes are in the V1-3080 build
  log.

### Covered repositories, deployable units and the repository reconciliation (GitHub #1983)

`info` and `validate` did not list the additional repositories a corpus covers, no subject named a
repository, deployable unit or published package, and nothing compared what the covered
repositories deploy or consume with what the corpus covers. These requirements derive that
inventory only from what the manifest declares and the inputs it pins; a missing fact stays
unknown.

- `DCP-V1-700`: (proposed; GitHub #1983; V1-3087; experimental under the standing lane auto-accept)
  A manifest (`/1` or `/2`) may declare `repository_inventory: {repositories?}`. Each entry is
  `{repository, name?, aliases?, archived?}`. `repository` is a declared repository's root-commit
  identity or `DCP-V1-600` alias. `name` is 1 to 128 bytes of UTF-8 without control characters.
  `aliases` is 1 to 16 sorted, distinct names matching `[A-Za-z0-9][A-Za-z0-9._/@:-]{0,127}`,
  unique across entries. `archived` is `true` or absent. Entries are sorted by `repository`, at
  most one per repository. An undeclared, repeated or unsorted entry, an invalid name or alias, a
  present empty `repositories` list, or a declaration without a native provider is refused
  (`corpus-refused`, text starting `invalid repository inventory declaration` or `repository
  inventory requires a native provider`). An absent declaration is omitted, so existing manifests
  and artifacts keep their bytes.
- `DCP-V1-701`: (proposed; GitHub #1983; V1-3087; experimental under the standing lane auto-accept)
  When the manifest declares additional repositories or a repository inventory, the `info` and
  `validate` result lists `repositories`, primary first, then each additional repository in
  manifest order. Each entry has `id`, `role` (`primary` or `additional`), `name` and
  `name_source`, `revision`, and, when present, `alias`, `aliases`, `revisions` and `root`. It
  also has `freshness`, `archived` and `archived_reason`. `name_source` is `repository_inventory`
  for a declared name, `workspace_alias` for the alias, or `undeclared`, when `name` is the
  root-commit identity. `freshness` is that checkout's own live state (`fresh` or `stale`) from
  the live overlay, which observes each declared checkout (`DCP-V1-178`). Without a live overlay,
  as in a pure `Query`, it is `unknown` with `freshness_reason`. `archived` is `declared` when the
  inventory declares it. Otherwise it is `unknown`, because archive state is a hosting fact Git
  does not record. A single-repository manifest without the declaration keeps its `info` bytes.
- `DCP-V1-702`: (proposed; GitHub #1983; V1-3087; experimental under the standing lane auto-accept)
  With a repository inventory, the build adds one `repository` subject per covered repository,
  `NATIVE:repository:ROOT_ID`, owned by the first native provider. Its name is the declared name,
  else the alias, else the root-commit identity. Its `alternate_ids` are the declared aliases and
  the workspace alias, so `get` resolves them (`DCP-V1-330`). Its claim `...:identity` states the
  name, root commit, pinned revision and every alias, so `search` finds the subject by name and by
  each alias. Its evidence is `declared` and `unknown`, with the reason that the manifest's
  verified root-commit declaration, not a file line, identifies it.
- `DCP-V1-703`: (proposed; GitHub #1983; V1-3087; experimental under the standing lane auto-accept)
  Deployable units and packages are derived only from declared native source inputs at the
  primary revision or an aliased repository's declared revision, by a closed, versioned,
  line-oriented rule set. `dockerfile/1` applies to `Dockerfile`, `Dockerfile.*` and `*.dockerfile`.
  Its first `FROM` line yields a `deployable_unit`, `NATIVE:deployable:[ALIAS:]PATH`, named by the
  path, because a Dockerfile does not name the image it publishes. `helm-chart/1` applies to
  `Chart.yaml`. Its top-level `name:` yields a `deployable_unit` of that name.
  `github-workflow-job/1` applies to `.github/workflows/*.yml` and `*.yaml`. Each job key under
  `jobs:` whose name contains `deploy`, `release` or `publish` yields a `deployable_unit`,
  `...:PATH:JOB`. `package-json/1` applies to `package.json`. A string `name` without
  `private: true` yields a `package`. `go-mod/1` applies to `go.mod`. Its `module` line yields a
  `package`. Each record is anchored at the matched line, with reason `repository inventory rule
  RULE`, derivation `source-derived`, and a `...:derivation` claim naming the rule and line. A
  matching input the rule cannot read is an `inventory-evidence-unreadable` gap on its file
  subject. Examples are a Dockerfile without a single-line `FROM`, a chart without a top-level
  name, and an invalid `package.json`. No record is guessed. The new subject kind
  `deployable_unit` is added to the closed subject kinds. From derivation 9, `package-json/2`
  and `github-workflow-job/2` replace the `/1` rules, and the gem, unsupported-CI and first-party
  rules are added (`DCI-V1-006..010`).
- `DCP-V1-704`: (proposed; GitHub #1983; V1-3087; experimental under the standing lane auto-accept)
  The same rules read each deployed or consumed reference. These are a Dockerfile `FROM` image
  other than `scratch` or an earlier stage, a chart dependency `name`, a workflow `uses:` named as
  `OWNER/REPO` (local `./` and `docker://` excluded), a `package.json` `dependencies` member, and
  a `go.mod` `require` that is not `// indirect`. Each is a claim `NATIVE:reference:[ALIAS:]PATH:LINE:ECOSYSTEM:NAME`
  (`DCP-V1-706`) on the file's unit or package, else on the repository, anchored at its line. The reconciliation
  joins a reference by exact name to a covered record. An npm or Go reference joins a package of
  the same ecosystem, a chart reference joins a chart, and an image (without tag or digest) or an
  action joins a repository whose inventory declares that alias. A joined reference adds a
  `depends_on` relation `...:resolves` from its owner to that record. Any other reference is an
  `outside-corpus-reference` gap on its owner. Its reason names the claim, the reference and its
  line, and states that the source repository is outside the corpus or undeclared
  (`NOT_OBSERVED`). An archived declaration is an `archived-repository` gap on that repository.
- `DCP-V1-705`: (proposed; GitHub #1983; V1-3087; experimental under the standing lane auto-accept)
  Absent evidence is reported as unknown, never inferred. A covered repository with no
  deployable-unit evidence gets a `deployable-evidence-unknown` gap. One with no `package.json`
  or `go.mod` input gets a `package-evidence-unknown` gap. Every inventory gap is a `gaps` result,
  filtered by kind and paged as any other gap (`DCP-V1-284`). The records rederive under `Open`
  like every other native record.
  Non-goals: inferring an image name, deploy target or runtime state; hosting-side archive or
  ownership lookups; other CI systems or package ecosystems, which stay unknown; and a
  `docs corpus manifest` flag (the declaration is written into the manifest). Failure modes: a
  third-party dependency is reported outside the corpus, which is true but can be numerous and is
  bounded by the corpus record bound. A reference spread over several lines is not seen, and its
  repository then stays unknown. An unaliased additional repository admits no native input
  (`DCP-V1-601`), so its evidence is always unknown. Rollback: drop `repository_inventory`, the
  info listing, the inventory derivation and the `deployable_unit` kind; manifests without the
  declaration are unaffected. Acceptance: `TestDCPV1700DCPV1701InfoListsCoveredRepositories`,
  `TestDCPV1702RepositorySubjectsSearchableByNameAndAlias` and
  `TestDCPV1703DCPV1704DCPV1705DeployablesPackagesAndReconciliation` (`internal/doccorpus`).
- `DCP-V1-706`: (proposed; GitHub #1983; V1-3087; experimental under the standing lane auto-accept)
  One input line may hold several references, so a reference claim's ID follows its line with its
  ecosystem (`image`, `chart`, `action`, `npm` or `go`) and its name, for example
  `native:reference:package.json:4:npm:left-pad`. Its relation is that ID plus `:resolves`. Two
  `package.json` dependencies on one line, including on the `dependencies` member's own line, are
  two references and never a duplicate record that refuses the corpus. A repeat of the same
  ecosystem and name on one line is one reference. A `Chart.yaml` dependency list may be
  indented or not: `- name:` at the start of a line inside the `dependencies:` block is an entry.
  The block ends at the next top-level key. Failure mode: a dependency on a line before its
  `dependencies` member is not reconciled and is reported `inventory-evidence-unreadable`.
  Rollback: restore the path-and-line ID. Acceptance:
  `TestDCPV1706OneLineDependenciesAndUnindentedChartLists` (`internal/doccorpus`).

### Module-to-flow ownership gaps (GitHub #1984)

`gaps` checked only flow-to-test links and untraversable flow relations, and `coverage` counted
records per input. No answer named a production module that no flow owns, even one that claims
cite. One reported monolith had about 58 percent of its production files with no flow link.
These requirements assess ownership only inside a production scope the project declares.

- `DCP-V1-720`: (proposed; GitHub #1984; V1-3088; experimental under the standing lane auto-accept)
  A manifest (`/1` or `/2`) may declare `production_scope: {include, exclude?}`. Each list is
  sorted and distinct, with at most 256 patterns. `include` is required and non-empty, and a
  present `exclude` is non-empty. A pattern is `[REPOSITORY:]GLOB`. `REPOSITORY` is a declared
  alias or root-commit identity and, when absent, the primary repository. The glob's segments are
  `path.Match` patterns or a whole `**`, as in extractor profiles. A pattern whose prefix before
  its first `:` names no declared repository is refused, as is any other shape
  (`corpus-refused`, text naming `production scope`). A module subject is in scope when its first
  anchor's repository and path match an include pattern and no exclude pattern. Witness
  path-policy classes are not a scope source here. An absent declaration is omitted, so existing
  manifests and artifacts keep their bytes. Without it, nothing is counted. Every `gaps` answer
  from a corpus with a flow or variation subject, and every `gaps` answer whose kind filter names
  a module ownership kind, carries the limitation `module-to-flow ownership NOT_ASSESSED: the
  manifest declares no production_scope, so no module is counted`.
- `DCP-V1-721`: (proposed; GitHub #1984; V1-3088; experimental under the standing lane auto-accept)
  A module is owned when any of three facts holds. A flow or variation subject anchors its
  repository and path. A joined flow-relation record's source anchors it (`DCP-V1-096`). Or a
  declared relation joins a flow or variation subject to the module, or to a subject anchored at
  the module's path, in either direction. With a declared scope, `gaps` returns a
  `module-without-flow` gap `{subject, kind, reason}` for each in-scope module that is not owned.
  The reason names the module as `LABEL:PATH`, using the `DCP-V1-607` label, and the scope digest.
  These gaps follow the computed gaps and precede the supplied ones. They are selected by `id`,
  filtered and totalled by `kinds` (`DCP-V1-284`) and paged like every gap.
- `DCP-V1-722`: (proposed; GitHub #1984; V1-3088; experimental under the standing lane auto-accept)
  A module-without-flow module that a claim of a non-native provider cites, by subject or by an
  anchor at its path, also gets a `module-claims-without-flow` gap. It lists the citing claim IDs
  in `claims`, sorted, at most 32. `claims_omitted` counts the rest, and the reason states the
  total. This kind is a sub-classification. Its modules are also `module-without-flow` gaps, so
  a kind filter selects either set.
- `DCP-V1-723`: (proposed; GitHub #1984; V1-3088; experimental under the standing lane auto-accept)
  With a declared scope, `coverage` adds the row `module_flow_ownership`. It has `scope:
  "declared"` and `scope_digest`, the `sha256:` digest of the canonical scope. It also has
  `owned`, `unowned` and `out_of_scope` module counts, plus `value` (owned), `denominator` (owned
  plus unowned), `defined`, `definition`, `revision` and `limitations`. A module with no anchor
  counts as out of scope. Without a declaration, a corpus with a flow or variation subject gets
  the row with `scope: "undeclared"`, `defined: false`, no counts and the `DCP-V1-720`
  limitation. A corpus with neither keeps its coverage bytes.
- `DCP-V1-724`: (proposed; GitHub #1984; V1-3088; experimental under the standing lane auto-accept)
  The assessment is computed at query time from the artifact and its embedded manifest. It adds
  no artifact member and reads no repository. Non-goals: deriving a production scope, ownership
  through test knowledge or test subjects, transitive ownership through module imports, and
  classes from the witness path policy. Failure modes: a primary path containing `:` cannot be
  scoped unqualified, because its prefix would be taken as a repository. A module whose flow link
  is declared only in prose stays unowned until a relation or anchor declares it. Rollback: drop
  `production_scope`, the two gap kinds and the coverage row; manifests without the declaration
  are unaffected. Acceptance: `TestDCPV1720DCPV1721DCPV1722ModuleWithoutFlowGaps` and
  `TestDCPV1720DCPV1723UndeclaredScopeAndOwnershipCoverage` (`internal/doccorpus`).

### Config module retrieval and weak search coverage (GitHub #1996, V1-3099)

- `DCP-V1-760`: (accepted under standing lane authorization, 2026-10-28; V1-3099)
  An indexed, native-owned JavaScript or TypeScript module gains searchable claims for the
  top-level keys of exported object literals admitted by `DCS-V1-583`, including default
  exports and its existing wrapper forms. Each claim belongs to the module subject and pins
  the exact key line plus adjacent full-line `//` or complete `/* */` comments, at most 32
  preceding lines and the existing 64 KiB span bound. Blank lines, code, incomplete or
  over-bound blocks stop comment inclusion. The existing 128 keys/object, 256 keys/file,
  128 bytes/key and artifact record/output bounds remain. Unsupported or unparsed inputs
  derive no keys. Existing declaration subjects and their evidence stay unchanged. No nested
  property, unexported object, evaluated expression or configuration semantics are inferred.
- `DCP-V1-761`: (accepted under standing lane authorization, 2026-10-28; V1-3099)
  A search with hits gains an optional `search_quality` object with schema
  `corvint-corpus-search-quality/1`, outcome `weak`, reason
  `all-rare-query-terms-unmatched`, `corpus_subjects`, `rare_idf_threshold` and query-ordered,
  deduplicated `rare_unmatched_terms` when every rare query word is absent. Rarity uses
  corpus subject IDF `ln(1+(N-df+0.5)/(df+0.5)) > ln(2)`, with absent terms at df=0,
  independent of hit scores, pagination and kind filters. Subject postings include attached
  claims; claim inclusion does not change the rarity population. A word uses its most
  frequent evidence-term variant, preferring compound identifier terms exactly as
  `unmatched_terms` does; stop words are ignored. Any matched rare word suppresses the signal,
  unless the absent words carry most of the IDF mass (`DCI-V1-020`).
  This is query coverage advice, never proof that a hit is wrong: retain every ranked hit,
  score, citation and all existing receipt fields, including `miss` and `unmatched_terms`,
  with their existing bytes/meaning. Do not add the field to other operations, true misses,
  searches without unmatched rare words, or searches with a matched rare word.
- `DCP-V1-762`: (accepted under standing lane authorization, 2026-10-28; V1-3099)
  The added module claims advance corpus derivation from 7 to 8; verified reads require a
  rebuild of older artifacts. Existing artifact/receipt schemas and existing record shapes
  remain frozen; only new module claim records and the optional independently versioned
  quality member are added. Labelled unit cases `GH1996-config-module-keys` and
  `GH1996-common-only` cover pinned evidence, module-first ranking, scan/index parity,
  low-score rare-hit preservation and the additive wire. Roll back by restoring the previous
  compiler and rebuilding artifacts; consumers may ignore the optional quality member.

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
and runtime sidecars, a five-denominator coverage projection, a reconciliation frontier bounded by `DCP-V1-240` and
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
`corpus-platform-unsupported` covers every local input read on a platform other than darwin and
linux, refused before the input root is opened (`PRS-V1-022`, decision 0559).
`corpus-capacity-exceeded` covers a corpus over a named capacity limit and an exhausted Git
operation budget or output bound during inventory, build or freshness (`DCP-V1-157`, `DCP-V1-158`),
and a query index over its posting or key-byte bound (`DCP-V1-194`).
`docs corpus extract` refuses a non-full revision, a missing, non-regular, oversized or invalid
profile, and an inventory over the record bound with `corpus-refused` and no output; a source
file, shape or reference it cannot resolve is an inventory gap (`DCX-V1-395`), never a refusal or
a guessed subject.

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
An eval case file refuses whole with `corpus-refused` (`DCP-V1-250`); an unresolved expected ID
fails its case and is listed, never dropped (`DCP-V1-253`); a tool failure makes the case `error`
(`DCP-V1-251`); a baseline with a different comparison identity is reported not comparable and
exits 1 (`DCP-V1-255`); a regressed case exits 1 with every regression listed (`DCP-V1-254`).
A claim-audit record outside its closed schema or with a mismatched evidence binding refuses the
build (`DCP-V1-260`); a stale, unresolved, non-independent or generator-method verdict stays out
of the headline figure with its reason (`DCP-V1-261`, `DCP-V1-262`); a report with no headline
verdict abstains instead of reporting a value (`DCP-V1-266`); a sample from another repository or
profile is refused (`DCP-V1-266`).

An unresolved claim mention is a typed gap, never a refusal (`DCX-V1-344`); a dangling claim,
relationship, journey or observation reference refuses the build (`DCX-V1-347`); a stored
`claim_mentions` member that differs from its rederivation refuses `Open` (`DCX-V1-341`); a
corpus over an opt-in mention or pattern rate threshold refuses with `corpus-refused` and writes
nothing (`DCX-V1-346`, `DCX-V1-352`); an undefined rate or weighted figure abstains rather than
reporting a value (`DCX-V1-348`, `DCX-V1-351`); a sample with impossible strata counts is refused
(`DCX-V1-351`).
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

Every refusal `trust_envelope`, and the corpus MCP tool-failure envelope, carries
`verified_at: null` with the limitation that the request was refused and no verification is
recorded (`DCP-V1-320`).

Failure modes of the requirement blocks moved to the DCS-V1, DCR-V1 and DCX-V1 companion specs
(V1-2525) are stated in those specs.

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
The delta worklist (`DCX-V1-360..369`) does not repair or rewrite changed claims, re-anchor by
fuzzy or semantic match, classify journeys, observations or supplied gaps, list changed lines of
additional repositories as uncovered regions, re-resolve symbols across paths outside the primary
repository, or expose an MCP tool (`DCX-V1-420..425`). Carry-forward writes no file and grants no trust:
the caller commits and declares the record. Rollback removes the two operations and
`internal/doccorpus/delta*.go`; no artifact or manifest format changes. Rollback of `DCX-V1-420..425`
restores the primary-only anchor exclusion and per-record anchorless entries and removes the
`--uncovered`, `--anchorless` and `--format` delta options; no artifact, manifest or provider
format changes, and carry-forward records already committed stay valid provider records.
Carrying adoption records (`DCX-V1-440..445`) does not validate typed details beyond the next
build's own admission, carry journeys, observations or supplied gaps, or merge a partly changed
flow. Rollback restores the `unresolved` adoption-only classification and the generic-only
carry-forward record; adoption records already committed stay valid adoption provider records.
Verified fast reads (`DCR-V1-410..419`) add no daemon or `--serve` mode, persist no read index
or sidecar, write nothing on a read, change no search ranking or delta paging, and do not
authenticate an artifact's origin. Rollback points the CLI and MCP readers back at `Open`; the
artifact and manifest formats are unchanged and no stored state needs migration.
Verified delta reads (`DCR-V1-460..464`) change no receipt, stream, class, reason, admission or
artifact format (a malformed `--previous` may be refused with another `corpus-refused` message), verify no builder identity or pin of the previous corpus, make `carry-forward`
or a `--manifest` current side no faster, and do not speed up classification itself. Rollback
points the delta current side back at `OpenDeltaCurrent` and the previous side at
`ParseArtifact`, and removes the two delta flags; nothing is stored.
The read hot-spot fixes (`DCR-V1-450..453`) change no artifact, receipt, reason, refusal or
format, persist nothing and write nothing (the Git index is only read). They do not trust a
worktree file by its stat (a vouched entry is pinned from its committed blob), do not route the
pins of `Build`, `BuildContext` or `BuildEval`, and do not remove the second pin pass of a build.
Rollback restores the pairwise scope loops and drops the `withStatRoutedPins` context mark from
`BuildRevisionContext`; no stored state needs migration.
Streamed reads and verified reuse (`DCR-V1-430..434`) add no daemon, persisted index, sidecar,
cache or memory-limit setting, change no artifact, manifest, receipt or index content, and do not
make producers trust a rehashed artifact. Rollback points the CLI read and the joined `--corpus`
read back at `ReadCorpusFile` plus `OpenRead` and drops the decode-time claim terms; the delta
current side is `DCR-V1-460`'s and rolls back with it. No stored state needs migration.
Rebuild from the embedded manifest (`DCX-V1-495..498`) adds one input option to `build`, and
no cache, daemon, persisted state or artifact, manifest or receipt format change. It does not
make a read rebuild, admit an artifact from a changed derivation, or trust an artifact's records
beyond `DCX-V1-532`'s reuse: only its embedded manifest, checked against both recorded digests, and Git. Rollback removes
`--artifact` from `build`, restores `--manifest` as required, and drops the rebuild hint from
the `builder differs:` refusals; no stored state needs migration.
Diagnostics, builder upgrades, eval sessions and ranking (`DCX-V1-470..477`) add no cache,
daemon, flag, index key or persisted state, and change no artifact or receipt format other than
the builder engine string and the two optional eval result members. They do not authenticate an
artifact's origin, accept an artifact from a changed derivation, or add stemming to subject
names. Rollback restores the bare decode message, the exact builder comparison and engine
string (artifacts built in between are then refused and rebuilt), the single aggregate bound,
the per-case freshness, and the `DCS-V1-402` and `DCP-V1-313` orders. Caller-owned profiles
(`DCX-V1-478`, `DCX-V1-503..506`) carry their own bounds and rollback after `DCX-V1-506`.
Read-start freshness and decode allocation (`DCR-V1-465..468`) add no cache, daemon or
persisted observation, observe no additional checkout before its header, change no overlay,
receipt, refusal, index or timing field, and do not shorten the decode itself. Rollback removes
`WithPrimaryObservation` from the CLI read and restores the boxed element encode and the
append-grown postings; nothing is stored.

Non-goals of the moved requirement blocks are stated in the DCS-V1, DCR-V1 and DCX-V1 companion
specs.

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

| Requirement | Implementation | Evidence |
|---|---|---|
| DCP-V1-760 | `internal/doccorpus/module_object_claims.go`, `internal/doccorpus/compile.go` | `TestDCPV1760ConfigModuleKeysAndComments`, `TestDCPV1760ObjectCommentBounds`, `TestDCPV1760ConfigModuleExportForms` |
| DCP-V1-761 | `internal/doccorpus/search_rank.go`, `internal/doccorpus/types.go` | `TestDCPV1761SearchQualityUsesCorpusIDF`, `TestDCPV1761SearchQualityRarityBoundary`, `TestDCPV1761SearchQualityOnMCPWire` |
| DCP-V1-762 | `internal/doccorpus/encoding.go`, `docs/RELEASE-NOTES.md` | `TestDCXV1471DerivationVersionGuard`, `TestDCXV1564ReleaseNotesListEveryDerivation`, `TestDCPV1761SearchQualityUsesCorpusIDF`, `TestDCPV1761SearchQualityRarityBoundary`, `TestDCPV1761SearchQualityOnMCPWire` |

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
| DCP-V1-240 | `internal/doccorpus/behavior_adapter.go` | `TestDCPV1240BuildBoundsTheFrontier`, `TestDCPV1240BuildBoundsTheFrontierBytes`, `TestDCPV1240BoundedFrontierIsTheCanonicalPrefix`, `TestBehaviorAdapterCheckRetainsNoFrontier`; heap and time measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-behavior-adapter-bounded-frontier.md |
| DCP-V1-046..048 | `internal/doccorpus/behavior_playwright_discovery.go`, `internal/liveverify/affected/typescript/playwright_listed_tests.go`, `playwright_discovery_list.go`, `cmd/corvint/docs_corpus_playwright.go` | `TestPlaywrightDiscoveryProducerRoundTrip`, `TestPlaywrightDiscoveryProducerRefusals`, `TestPlaywrightListedTestsMultiProject`, `TestDocsCorpusPlaywrightProducersCLI` |
| DCP-V1-049..052 | `internal/doccorpus/behavior_playwright_witness.go`, `cmd/corvint/docs_corpus_playwright.go` | `TestPlaywrightWitnessImporter`, `TestDocsCorpusPlaywrightProducersCLI` |
| DCP-V1-053 | `cmd/corvint/docs.go`, `cmd/corvint-js-test-provider/main.go` | `TestDocsHelpNamesPlaywrightReceiptProducer`, `TestDocsHelpProducerExampleUsesE2EFlags` |
| DCP-V1-241 | `internal/doccorpus/apply.go`, `cmd/corvint/docs_corpus.go` | `TestDCPV1241CorpusMaintainPageRefusalNamesFlag` |
| DCP-V1-242 | `cmd/corvint-corpus-mcp/main.go` | `TestDCPV1242RelativeArtifactRefusalNamesArgumentAndRule` |
| DCP-V1-243 | `internal/doccorpus/batch_input.go`, `internal/doccorpus/query.go` | `TestDCPV1243ResolvedInputsKeepPerPathTreeBound` |
| DCP-V1-244 | `internal/doccorpus/query_index.go` `AdmitIndex` | `TestDCPV1244AdmitIndexRefusesEveryDerivationRefusal` |
| DCP-V1-250..257 | `internal/mcp/corpusbridge/eval.go`, `cmd/corvint/docs_corpus_eval.go`, `cmd/corvint/docs_corpus.go`, `cmd/corvint/docs_corpus_help.go` | `TestDCPV1250EvalRefusesMalformedCaseFiles`, `TestDCPV1251EvalScoresThroughTheMCPToolPath`, `TestDCPV1254BaselineListsEveryRegression`, `TestDCPV1255ChangedIdentityIsNotComparable`, `TestDCPV1257CorpusEvalCLI`, `TestDocsCorpusOperationFlagMatrix` |
| DCP-V1-260..268 | `internal/doccorpus/claim_audit.go`, `claim_audit_report.go`, `providers.go`, `compile.go`, `query.go`, `pagination.go`, `shards.go`, `cmd/corvint/docs_corpus_audit.go`, `cmd/corvint/docs_corpus.go`, `cmd/corvint/docs_corpus_help.go` | `TestDCPV1260ClaimAuditSchemaIsClosed`, `TestDCPV1261StaleClaimAuditVerdictIsExcluded`, `TestDCPV1262NonIndependentAndGeneratorVerdictsStayOutOfHeadline`, `TestDCPV1263ClaimAuditViewsKeepTrust`, `TestDCPV1263MCPGetShowsClaimAudit`, `TestDCPV1265AuditSampleIsDeterministic`, `TestDCPV1268CorpusAuditCLI`, `TestDocsCorpusOperationFlagMatrix` |
| DCP-V1-300..302 | `internal/doccorpus/adoption.go`, `query.go` | `TestDCPV1300WidenedStatusVocabulary`, `TestDCPV1300StatusOutsideVocabularyRefused` |
| DCP-V1-303..305 | `internal/doccorpus/adoption.go`, `providers.go`, `query.go` | `TestDCPV1303TestRunLedgerFieldsRoundTrip`, `TestDCPV1303TestRunLedgerUnknownValuesRefused`, `TestDCPV1304SuppliedGapTargetsRunClaim` |
| DCP-V1-306..307 | `internal/doccorpus/adoption.go`, `query.go` | `TestDCPV1306MigrationMappingLedger`, `TestDCPV1306MigrationMappingRefusals` |
| DCP-V1-308..311 | `internal/doccorpus/types.go`, `compile.go`, `query.go`, `typed_query.go`, `pagination.go`, `internal/mcp/corpusbridge/corpusbridge.go` | `TestDCPV1308AdditionalRepositoryAtTwoRevisions`, `TestDCPV1308AdditionalRevisionRefusals`, `TestDCPV1310LocateRepositoryQualifier`, `TestDCPV1310DocsLocateRepositoryArgument`, `TestDCPV1311SingleRepositoryLocateUnchanged` |
| DCP-V1-312 | `cmd/corvint/docs_corpus.go`, `cmd/corvint/docs_corpus_help.go` | `TestDCPV1312CorpusCLIRepositoryQualifierParity` (CLI and MCP receipts equal for the primary repository; matching corpus-refused and invalid-arguments refusals); `TestDocsCorpusOperationHelpPrintsFlags` |
| DCP-V1-280 | `internal/doccorpus/query.go`, `internal/mcp/corpusbridge/corpusbridge.go` | `TestDCPV1280AdditionalRepositoryFreshnessReused`, `TestDCPV1175AdditionalRepositoryAnchors` |
| DCP-V1-281 | `internal/doccorpus/query.go`, `internal/mcp/corpusbridge/corpusbridge.go` | `TestDCPV1281AdditionalRepositoryWarmCallsAtScale` |
| DCP-V1-313 | `internal/doccorpus/query_index.go`, `typed_query.go`, `internal/corpusindex/index.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go`, `cmd/corvint/docs_corpus_help.go` | `TestDCPV1313ClaimTextQueryReachesClaimRecall`, `TestDCPV1313CorpusClaimsQueryCLIParity` |
| DCP-V1-314 | `internal/mcp/corpusbridge/stamp.go`, `stamp_darwin.go`, `stamp_linux.go`, `stamp_other.go` | `TestDCPV1314StampChangesWithCheckoutState`, `TestDCPV1314StampUnavailableOutsideCoveredState`, `TestDCPV1314StampRefusesRacyEntries` |
| DCP-V1-315..316 | `internal/mcp/corpusbridge/corpusbridge.go`, `stamp.go` | `TestDCPV1315WarmCallsSkipGitWhileStampUnchanged` |
| DCP-V1-317 | `internal/mcp/corpusbridge/additional_freshness_test.go` | `TestDCPV1281AdditionalRepositoryWarmCallsAtScale` |
| DCP-V1-318 | `internal/doccorpus/typed_query.go`, `query.go`, `adoption.go`, `internal/corpusindex/index.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go`, `cmd/corvint/docs_corpus_help.go` | `TestDCPV1318StatusClaimListing`, `TestDCPV1318StatusAndIntentListingCLIParity` |
| DCP-V1-319 | `internal/doccorpus/typed_query.go`, `internal/corpusindex/index.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus_help.go` | `TestDCPV1319IntentListing`, `TestDCPV1318StatusAndIntentListingCLIParity` |
| DCP-V1-320 | `internal/doccorpus/trust.go`, `internal/corpusserve/http.go`, `cmd/corvint-corpus-mcp/main.go` | `TestDCPV1320And321HTTPRefusalAndGapKinds`, `TestDCPV1320ToolFailureVerifiedAtNull` |
| DCP-V1-321 | `cmd/corvint/docs_corpus.go`, `cmd/corvint/docs_corpus_help.go`, `internal/doccorpus/query.go` | `TestDCPV1321CorpusKindAndClaimSearchCLIParity`, `TestDCPV1320And321HTTPRefusalAndGapKinds` |
| DCP-V1-322 | `internal/doccorpus/receipt_annotations.go`, `types.go`, `pagination.go`, `query.go` | `TestDCPV1322SuppliedTextOrigins` |
| DCP-V1-323 | `internal/doccorpus/adoption.go`, `receipt_annotations.go` | `TestDCPV1323AssociationAxisWithholdsWitness` |
| DCP-V1-324..326 | `internal/doccorpus/adoption.go` | `TestDCPV1324To326CoverageDetailShapes` |
| DCP-V1-327 | `internal/doccorpus/query.go`, `adoption.go` | `TestDCPV1327CoverageDetailsServed` |
| DCP-V1-328 | `internal/doccorpus/search_rank.go`, `receipt_annotations.go`, `query.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go` | `TestDCPV1328SearchIncludesClaims`, `TestDCPV1328And331SearchArguments`, `TestDCPV1321CorpusKindAndClaimSearchCLIParity` |
| DCP-V1-329 | `internal/doccorpus/providers.go`, `types.go`, `query_index.go` | `TestDCPV1329AlternateIDsAdmission`, `TestDCPV1329AliasCannotShadowJoinRecords` |
| DCP-V1-330 | `internal/doccorpus/receipt_annotations.go`, `query.go`, `query_index.go` | `TestDCPV1330AlternateIDResolution` |
| DCP-V1-331 | `internal/doccorpus/search_rank.go`, `query.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go` | `TestDCPV1331SearchRanking`, `TestDCPV1331ExactBonusAcrossFamilies`, `TestDCPV1328And331SearchArguments`, `TestDCPV1321CorpusKindAndClaimSearchCLIParity` |
| DCP-V1-332 | `internal/doccorpus/caller_revision.go`, `query.go`, `trust.go`, `internal/cem/gitauth/ancestry.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go` | `TestDCPV1332RelateCallerRevision`, `TestDCPV1332CallerRevisionEnvelope`, `TestDCPV1332CallerRevisionArgument`, `TestDCPV1332HTTPCallerRevisionUnknown`, `TestDCPV1332CorpusCallerRevisionCLIParity` |
| DCP-V1-282 | `internal/doccorpus/anchor_state.go`, `internal/doccorpus/query.go`, `internal/doccorpus/trust.go` | `TestDCPV1282PerCitationAnchorState` |
| DCP-V1-283 | `internal/doccorpus/trust.go` | `TestDCPV1283UnverifiedCorpusVerifiedAtNull` |
| DCP-V1-284 | `internal/doccorpus/query.go`, `internal/doccorpus/anchor_state.go`, `internal/mcp/corpusbridge/corpusbridge.go` | `TestDCPV1284GapsKindFilter` |
| DCP-V1-285 | `cmd/corvint-corpus-mcp/outcomes.go`, `cmd/corvint-corpus-mcp/main.go` | `TestDCPV1285OutcomeLogAggregatesWithoutRequestText` |
| DCP-V1-286 | `cmd/corvint-corpus-mcp/outcomes.go` | `TestDCPV1286OutcomeSummaryFloor` |
| DCP-V1-056 | `cmd/corvint/docs_corpus_help.go`, `cmd/corvint/docs_corpus.go`, `cmd/corvint/help.go` | `TestDocsCorpusHelpListsOperations`, `TestDocsCorpusOperationHelpPrintsFlags`, `TestDocsCorpusOperationFlagMatrix` |
| DCP-V1-057 | `cmd/corvint/help.go` | `TestUnknownHelpTopicListsValidTopics`, `TestUnknownHelpTopicIsConventionalError`, `TestListedHelpTopicResolvesAsOneArgument` |
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
| DCP-V1-086..092 | `internal/doccorpus/test_knowledge.go`, `providers.go`, `shards.go`, `compile.go`, `query.go`, `typed_query.go`, `pagination.go`, `internal/corpusrepublish/parity.go` | `TestDCPV1086TestKnowledgeSchemaIsClosed`, `TestDCPV1091TestKnowledgeAcrossCorpusBridge`, `TestDCPV1087TestKnowledgeFieldRules`, `TestDCPV1088TestKnowledgeAnchorsArePinned`, `TestDCPV1089TestKnowledgeJoin`, `TestDCPV1090TestKnowledgeTrustRules`, `TestDCPV1091TestKnowledgeQueryExposure`, `TestDCPV1091TestKnowledgeOnlyCorpus`, `TestDCPV1092TestKnowledgeDeterministicAndBounded`, `TestDCPV1092ParityLargeTestKnowledge` |
| DCP-V1-093..099 | `internal/doccorpus/flow_relations.go`, `providers.go`, `shards.go`, `compile.go`, `query.go`, `typed_query.go`, `integration.go`, `pagination.go`, `internal/corpusrepublish/parity.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/corpus_integration.go` | `TestDCPV1093FlowRelationSchemaIsClosed`, `TestDCPV1094FlowRelationKindFields`, `TestDCPV1095FlowRelationJoinStatus`, `TestDCPV1096FlowRelationImpactAttribution`, `TestDCPV1097FlowRelationUnreviewedAndDepth`, `TestDCPV1097CorpusDepthFlag`, `TestDCPV1098FlowRelationTraceAndGaps`, `TestDCPV1098TraceToolDepth`, `TestDCPV1099FlowRelationDeterministicAndBounded` |
| DCP-V1-140..142 | `internal/contextindex/symbol_region.go`, `rubysymbols.go`, `internal/doccorpus/symbol_anchor.go`, `query.go`, `pagination.go`, `cmd/corvint/docs_corpus.go`, `corpus_integration.go` | `TestSymbolAnchorReresolution` (Ruby and TypeScript: insertion is `moved` with the new range, a one-token edit is `changed`, deletion is `gone`, a reformat is `live` or `moved`), `TestSymbolAnchorUncertainty`, `TestSymbolAnchorReaderReceipt`, `TestNormalizedCodeLines` |
| DCP-V1-143..145 | `internal/mcp/corpusbridge/corpusbridge.go`, `cputime_unix.go`, `internal/doccorpus/query.go`, `symbol_anchor.go`, `pagination.go`, `cmd/corvint-corpus-mcp/main.go` | `TestDCPV1143LazyLoadAndMemo`, `TestDCPV1143StartupRefusesUnreadableArtifact`, `TestDCPV1144LiveFreshnessReusedUntilTheRepositoryChanges`, `TestDCPV1144ConcurrentCallsShareOneOverlay`, `TestDCPV1144SharedOverlayMatchesAFreshReader`, `TestSymbolAnchorReaderReceipt`, `TestDCPV1145PathsBatchEqualsUnion`, `TestDCPV1145BatchDeadlineNamesUnresolved`, `TestDCPV1145BatchDeadlineBeforeLoadNamesEveryPath`, `TestDCPV1145CallDeadlineAndTiming`, `TestDCPV1145BatchFitsOneFrame`; measured on a 100,000-claim corpus in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1982-corpus-mcp-lazy-batch.md |
| DCP-V1-146 | `internal/doccorpus/compile.go` | `TestDCPV1146OversizedParagraphStaysVerbatim`, `TestDCPV1146OversizedLineIsAGap`, `TestDCPV1146RefusalsNameTheSpan`; measured on Corvint's docs scope in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1983-docs-corpus-located-refusals.md |; | DCP-V1-150..152 | `internal/doccorpus/typed_query.go`, `docs/DOCUMENTATION-CORPUS.md` | `TestDCPV1150ProviderReportedReviewYieldsUnauthenticatedGap`, `TestDCPV1151AuthenticatedReviewClearsAssertingGap`, `TestDCPV1152AssertingGapNamesTheFailedCondition` |; | DCP-V1-165..169 | `internal/doccorpus/legacy_cases.go`, `cmd/corvint/docs_corpus_legacy_cases.go`, `cmd/corvint/docs_corpus_help.go`, `cmd/corvint-corpus-mcp/legacycases.go` | `TestDCPV1165LegacyCaseIDLookup`, `TestDCPV1166LegacyCaseAnyMatchRanks`, `TestDCPV1167LegacyCaseQueryIDRule`, `TestDCPV1168LegacyCaseAllModeBytesUnchanged`, `TestDCPV1169CorpusMCPSearchCasesModes`, `TestDCPV1165DocsCorpusSearchCasesModesCLI`, `TestDocsCorpusOperationFlagMatrix` |; | DCP-V1-160..163 | `internal/doccorpus/compile.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go` | `TestDCPV1160SchemaTwoManifestOfTwentyThousandFilesBuilds`, `TestDCPV1160DefaultSingleScopeUnchanged`, `TestDCPV1160SchemaFlagValues`, `TestDCPV1161ScopesSortedAndOverlapRefused`, `TestDCPV1162BoundRefusalNamesBoundScopeAndCount` |; | DCP-V1-170..174 | `internal/doccorpus/flow_filter.go`, `typed_query.go`, `query.go`, `pagination.go`, `types.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go` | `TestDCPV1170FlowSectionFilter`, `TestDCPV1171FlowIncludeFilter`, `TestDCPV1172FlowSelectionReceipt`, `TestDCPV1173FlowFilterPagination`, `TestDCPV1174DocsFlowToolFilters`, `TestDCPV1174CorpusFlowFilterOptions`, `TestDocsCorpusOperationFlagMatrix`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1990-docs-flow-section-filter.md |; | DCP-V1-155..159 | `internal/cem/gitauth/pathset.go`, `internal/doccorpus/batch_input.go`, `capacity.go`, `compile.go`, `query.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go` | `TestDCPV1155ReadFreshnessGitOpsIndependentOfInputCount` (10,000 inputs, 16 Git operations per phase, one change is `stale`), `TestDCPV1155ResolvePathsMatchesSingleLookupsWithinOneOpPerLevel`, `TestDCPV1156ManifestAndBuildBatchNonIndexedInputs`, `TestDCPV1157CapacityRefusalsAreTypedAndEarly`, `TestDCPV1158BridgeTypesGitCapacityLikeTheCLI`, `TestDCPV1158CLITypesGitCapacityLikeTheBridge`, `TestDCPV1159BatchedPinningKeepsArtifactBytes`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1984-corpus-git-budget.md |
| DCP-V1-185..192 | `internal/doccorpus/adoption.go`, `types.go`, `providers.go`, `query.go`, `typed_query.go`, `pagination.go`, `shards.go`, `incremental.go`, `query_index.go`, `compile.go`, `internal/corpusrepublish/parity.go` | `TestDCPV1185SuppliedGapAdmittedAndLabelled`, `TestDCPV1185SuppliedRecordsNeverRaiseTrust`, `TestDCPV1185SuppliedRecordRefusals`, `TestDCPV1187SuppliedStatusQueryable`, `TestDCPV1189ManualCaseLinkSuppressesMissingManualCases`, `TestDCPV1191SchemaTwoObservationsUseRecordLimit`; byte identity measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1987-adoption-typed-homes.md |
| DCP-V1-205..207 | `cmd/corvint-corpus-mcp/main.go` | `TestDCPV1205AbsoluteArtifactInsideRoot`, `TestDCPV1206AbsoluteArtifactThroughSymlinkedRoot`, `TestDCPV1207AbsoluteArtifactOutsideRootRefused`, `TestDCPV1205AbsoluteMapInsideRoot` |; | DCP-V1-200..203 | `internal/mcp/corpusbridge/corpusbridge.go` | `TestDCPV1200SingleCallOverBoundPagesWithNextOffset`, `TestDCPV1202UnpagedSchemaRefusesOverBound`, `TestDCPV1143LazyLoadAndMemo` (a fitting receipt keeps its CLI bytes); https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1996-corpus-mcp-output-bound.md |; | DCP-V1-193..197 | `internal/doccorpus/query_index.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go` | `TestDCPV1193HundredThousandRecordCorpusAnswersIndexedReads`, `TestDCPV1194IndexRefusalNamesBoundAndObservedCount`, `TestDCPV1195IndexRefusalDerivedOncePerArtifact`, `TestDCPV1195IndexDerivedOncePerArtifact`, `TestDCPV1196BuildAdmittedRefusesOverBoundIndex`, `TestFlowDocsFinalizeRefusesOverBoundIndex`, `TestRepublishRefusesOverBoundIndex`; memory, latency and byte identity measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1995-corpus-index-bound.md |; | DCP-V1-210..215 | `internal/doccorpus/adoption.go`, `query.go`, `query_index.go`, `providers.go`, `incremental.go` | `TestDCPV1210SuppliedTestRunReceipt`, `TestDCPV1210ResidualRecordRefusals`, `TestDCPV1212LegacyCaseIDLinksToSubject`, `TestDCPV1214SuppliedGapsInCapabilitiesAndIndex`, `TestDCPV1215IncrementalCountsSuppliedClaimKinds`; byte identity measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1993-adoption-residuals.md |
| DCP-V1-146 | `internal/doccorpus/compile.go` | `TestDCPV1146OversizedParagraphStaysVerbatim`, `TestDCPV1146OversizedLineIsAGap`, `TestDCPV1146RefusalsNameTheSpan`; measured on Corvint's docs scope in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1983-docs-corpus-located-refusals.md |; | DCP-V1-150..152 | `internal/doccorpus/typed_query.go`, `docs/DOCUMENTATION-CORPUS.md` | `TestDCPV1150ProviderReportedReviewYieldsUnauthenticatedGap`, `TestDCPV1151AuthenticatedReviewClearsAssertingGap`, `TestDCPV1152AssertingGapNamesTheFailedCondition` |; | DCP-V1-165..169 | `internal/doccorpus/legacy_cases.go`, `cmd/corvint/docs_corpus_legacy_cases.go`, `cmd/corvint/docs_corpus_help.go`, `cmd/corvint-corpus-mcp/legacycases.go` | `TestDCPV1165LegacyCaseIDLookup`, `TestDCPV1166LegacyCaseAnyMatchRanks`, `TestDCPV1167LegacyCaseQueryIDRule`, `TestDCPV1168LegacyCaseAllModeBytesUnchanged`, `TestDCPV1169CorpusMCPSearchCasesModes`, `TestDCPV1165DocsCorpusSearchCasesModesCLI`, `TestDocsCorpusOperationFlagMatrix` |; | DCP-V1-160..163 | `internal/doccorpus/compile.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go` | `TestDCPV1160SchemaTwoManifestOfTwentyThousandFilesBuilds`, `TestDCPV1160DefaultSingleScopeUnchanged`, `TestDCPV1160SchemaFlagValues`, `TestDCPV1161ScopesSortedAndOverlapRefused`, `TestDCPV1162BoundRefusalNamesBoundScopeAndCount` |; | DCP-V1-170..174 | `internal/doccorpus/flow_filter.go`, `typed_query.go`, `query.go`, `pagination.go`, `types.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go` | `TestDCPV1170FlowSectionFilter`, `TestDCPV1171FlowIncludeFilter`, `TestDCPV1172FlowSelectionReceipt`, `TestDCPV1173FlowFilterPagination`, `TestDCPV1174DocsFlowToolFilters`, `TestDCPV1174CorpusFlowFilterOptions`, `TestDocsCorpusOperationFlagMatrix`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1990-docs-flow-section-filter.md |; | DCP-V1-155..159 | `internal/cem/gitauth/pathset.go`, `internal/doccorpus/batch_input.go`, `capacity.go`, `compile.go`, `query.go`, `internal/mcp/corpusbridge/corpusbridge.go`, `cmd/corvint/docs_corpus.go` | `TestDCPV1155ReadFreshnessGitOpsIndependentOfInputCount` (10,000 inputs, 16 Git operations per phase, one change is `stale`), `TestDCPV1155ResolvePathsMatchesSingleLookupsWithinOneOpPerLevel`, `TestDCPV1156ManifestAndBuildBatchNonIndexedInputs`, `TestDCPV1157CapacityRefusalsAreTypedAndEarly`, `TestDCPV1158BridgeTypesGitCapacityLikeTheCLI`, `TestDCPV1158CLITypesGitCapacityLikeTheBridge`, `TestDCPV1159BatchedPinningKeepsArtifactBytes`; measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1984-corpus-git-budget.md |; | DCP-V1-175..179 | `internal/doccorpus/types.go`, `compile.go`, `providers.go`, `query.go`, `internal/mcp/corpusbridge/corpusbridge.go` | `TestDCPV1175AdditionalRepositoryAnchors`, `TestDCPV1176AdditionalRepositoryRefusals`, `TestDCPV1178BridgeVerifiesPinsInOwningCheckout`, `TestDCPV1179SingleRepositoryManifestUnchanged`; byte identity measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-1986-corpus-additional-repositories.md |
| DCP-V1-033..037 | `internal/doccorpus/adoption.go`, `shards.go`, `pagination.go`, native CLI and corpus MCP | Capacity qualification, typed round-trip, import parity, shard closure, restricted canaries and paginated read parity |
| DCP-V1-216..217 | `internal/doccorpus/batch_paths.go`, `symbol_anchor.go`, `cem.go`, `behavior_legacy_discovery.go`, `behavior_playwright_discovery.go` | `TestDCPV1216SymbolAnchorsGitOpsIndependentOfAnchorCount` (5,000 moved anchors), `TestDCPV1216CEMProjectionGitOpsIndependentOfAnchorCount` (5,000 anchors), `TestDCPV1216LegacyDiscoveryGitOpsIndependentOfPathCount` (4,096 files, the record bound), `TestDCPV1216PlaywrightDiscoveryGitOpsIndependentOfPathCount` (2,000 files, 4,008 executions), each within 16 Git operations per phase and byte-identical to the per-path reads (each test bounds Git operations on its full input and compares bytes on a 1-in-10 stride sample: 500 anchors across every directory and more than `MaxResults` claims, and second legacy and Playwright repositories holding every tenth file with the same contents in the same single directory, since the batches have no chunk size and the per-path reference was most of their time; V1-2342, experimental under the standing lane auto-accept); `TestDCPV1216ProjectionKeepsPerPathTreeBound` (anchor under a tree past 4 MiB refused as per path), `TestDCPV1216SymbolFallbackBudgetWithholdsStates` (missing pinned blob, budget exhausted in the fallback, states withheld as per path), `TestDCPV1216GitHooksTravelWithTheContext` (the test budget and per-path override travel on the call's context, so the equivalence tests run under `t.Parallel` and `-race`); measured in https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-v1-1991-batched-path-reads.md and https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-testspeed-1021v-doccorpus-parallel-equivalence.md and https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-27-slow-flaky-1027.md |
| DCP-V1-574..578 | `internal/doccorpus/input_asof.go`, `types.go`, `compile.go`, `query.go` | `TestDCPV1574InputAsOfValidation`, `TestDCPV1575DCPV1576InputAsOfLiveStates`, `TestDCPV1577ProbeChangesObservation` |
| DCP-V1-579 | `cmd/corvint/docs_corpus_help.go`, `SearchIncludeDescription` in `internal/mcp/corpusbridge/corpusbridge.go`, `docs/DOCUMENTATION-CORPUS.md` | `TestDCPV1579SearchIncludeHelpSaysClaimsAreAdded`, `TestDCPV1579SearchDescriptionSaysClaimsAreAdded` |
| DCP-V1-600..605 | `internal/doccorpus/workspace_inventory.go`, `types.go`, `compile.go`, `query.go`, `cmd/corvint/docs_corpus_workspace.go`, `docs_corpus.go`, `docs_corpus_help.go` | `TestDCPV1600WorkspaceManifestPinsEachRepository`, `TestDCPV1600WorkspaceAliasRefusals`, `TestDCPV1602TraceAndGapsByPathInEitherRepository`, `TestDCPV1603SingleRootManifestByteIdentical`, `TestDCPV1605CrossRepositoryRelationTracesBothWays`, `TestDCPV1604DocsCorpusWorkspaceManifest`, `TestDCPV1604DocsCorpusWorkspaceTraceAndGaps`, `TestDocsCorpusOperationFlagMatrix` |
| DCP-V1-606 | `ReadFile`, `ReadFileContext`, `openInput` in `internal/corpusindex/index.go`, `open_unix.go`, `open_other.go`; `cmd/corvint-corpus-http/main.go` | `TestReadFileRefusesAFIFOWithoutBlocking` |
| DCP-V1-607..608 | `generatedCheck`, `generatedRefusal`, `repositoryLabel`, `inventoryScopes`, `ManifestInventory` in `internal/doccorpus/compile.go`, `workspace_inventory.go`; `docs corpus manifest` in `cmd/corvint/docs_corpus.go` | `TestDCPV1607BuildRefusalNamesEveryGeneratedInput`, `TestDCPV1607GeneratedHeaderCheckIsNamed`, `TestDCPV1608ManifestRefusesGeneratedInputsWhenPinning`, `TestDCPV1608DocsCorpusManifestNamesGeneratedInputs` |
| DCP-V1-650..651 | `LocateQueryKinds`, `ResultLocation` in `internal/doccorpus/types.go`; locate branch of `queryContext` in `query.go`; `setLocations` in `receipt_annotations.go`; `docs_locate` schema and arguments in `internal/mcp/corpusbridge/corpusbridge.go` | `TestDCPV1650LocateByQueryReturnsAnchoredSourceSubjects`, `TestDCPV1650DocsLocateQuery` |
| DCP-V1-652 | `NameLegacyFlag`, `legacyInitializeRefusal` in `internal/mcp/server/server.go`; `cmd/corvint-corpus-mcp/main.go` | `TestDCPV1652LegacyInitializeNamesVersionAndFlag`, `TestDCPV1652LegacyInitializeWithoutFlagNamesIt` |
| DCP-V1-653..655 | `CompactReceipt`, `SearchIncludes`, `FullView`, `SearchReceiptDescription` in `internal/mcp/corpusbridge/compact.go`, `corpusbridge.go`; docs_search compaction in `cmd/corvint-corpus-mcp/main.go` | `TestDCPV1653CompactReceipt`, `TestDCPV1655CompactReceiptEncodedSymbolLocation`, `TestDCPV1653SearchCompactReceiptOnTheWire`, `TestCorpusMCPTransport`, `TestCorpusSeparateCLIMCPBinaries` |
| DCP-V1-700..706 | `RepositoryInventory`, `validateRepositoryInventory`, `repositoryListing` and the inventory rules in `internal/doccorpus/repository_inventory.go`; `types.go`, `compile.go`, the `info`/`validate` branch and per-checkout freshness in `query.go`; `subjectKinds` in `providers.go` | `TestDCPV1700DCPV1701InfoListsCoveredRepositories`, `TestDCPV1702RepositorySubjectsSearchableByNameAndAlias`, `TestDCPV1703DCPV1704DCPV1705DeployablesPackagesAndReconciliation`, `TestDCPV1706OneLineDependenciesAndUnindentedChartLists` |
| DCP-V1-720..724 | `ProductionScope`, `validateProductionScope`, `moduleOwnership`, `moduleOwnershipMetric` in `internal/doccorpus/module_flow.go`; `types.go`, `compile.go`, the `gaps` and `coverage` branches of `queryContext` in `query.go` | `TestDCPV1720DCPV1721DCPV1722ModuleWithoutFlowGaps`, `TestDCPV1720DCPV1723UndeclaredScopeAndOwnershipCoverage` |

## Open decisions

Owner acceptance and any promotion beyond the local experimental profile remain open. Additional
render formats, cross-repository inputs beyond the declared additional repositories (see
`DCP-V1-175` to `DCP-V1-179` and the workspace repositories of `DCP-V1-600` to `DCP-V1-605`), provider transports and authenticated review attestations require
separate contracts; absent capabilities must remain visible until then.

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
