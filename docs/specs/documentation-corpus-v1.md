# Revision-pinned documentation corpus V1

Owner: Russell Lewis
Date: 2026-09-19
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner request [issue 31](https://github.com/beamfall/corvint/issues/31),
`AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`, `docs/specs/deployment-neutral-index-platform-v0.md`,
`docs/specs/source-documentation-draft-v0.md`, `docs/specs/external-evidence-provider-v0.md`.

## Agent digest
- Claim: A native compiler builds revision-pinned documentation corpora for bounded CLI, evidence, rendering and capability-gated MCP queries.
- Status: proposed/experimental; this contract does not accept generated intent or qualify HDC.
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
- `DCP-V1-012`: `docs corpus` exposes manifest/build/info/search/get/locate/related/coverage/gaps/journey/
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
  tools equivalent to docs_info/search/get/locate/find_related/coverage/gaps/get_journey/trace are
  advertised only when their backing capability is present (including present-zero). Revalidate on
  calls, preserve typed failures, frame model-facing text using `repoenvelope`, refuse terminator
  collisions and match CLI structured receipt bytes. Existing core and draft MCP surfaces stay frozen.
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
`golf_e2e` and `docs_corpus`; the registry, migration manifest, live discovery and runtime must agree
with the corpus manifest's caller-supplied `behavior_revisions`. Any changed member blocks recorded
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
The contract SHA-256 uses canonical registry bytes with an empty digest and all test runtime fields
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

Acceptance: `TestBehaviorContractCorpusRoundTrip`, `TestBehaviorContractGaps`,
`TestBehaviorOrderedRuntime`, `TestBehaviorQualifiedReceiptEndToEnd`, `TestBehaviorAcceptanceAmendment` and
`TestBehaviorExactProjectObservation` exercise DCP-V1-004,
DCP-V1-007..013 and DCP-V1-019. Owner acceptance and real consumer fixtures remain promotion gates.

One local Git repository may supply up to eight explicit immutable revisions. A provider can be
committed after the source it describes; its record anchors still name the earlier source revision.
An input scope is a literal directory or exact file, with a complete declared inventory at its own
revision. A provider artifact is an input of purpose `provider`, never automatically source text.
The manifest can be generated by an explicit read-only inventory command and edited before build.
A profile is data, never code or a template interpreter. External/provider evidence remains attributed;
structural rederivation proves identity, not semantic truth, human acceptance or provider honesty.

Bounds: 4 MiB per manifest/provider/artifact/receipt, 4096 input paths, 4096 subjects/claims/relations
per collection, 128 journeys with 128 steps each, 64 KiB per evidence excerpt, 1024-byte paths/queries,
256 results, 8 revisions and 16 providers. Whole-build output overflow refuses instead of truncating.
Query limits disclose withheld rows. Unsupported analyzer input may be inventoried with an explicit
gap; explicitly requesting an unavailable provider implementation fails the build.

## Failure modes

`corpus-refused` covers malformed/canonical/digest/closure/authority/span/bound failures.
`corpus-input-unavailable` covers missing, unsafe or oversized local inputs.
`corpus-provider-unavailable` covers undeclared or unsupported provider implementations.
`corpus-invalid-revision` covers an unsupported immutable source revision.
The separate MCP startup uses `corpus-unavailable` when its configured corpus cannot be opened;
call failures retain the specific native error and never return successful empty evidence.

## Non-goals and simpler baseline

A manually authored Markdown page remains the baseline. No cross-repository federation, remote
provider transport, automatic observation collection, human-review authentication, semantic anchor
repair, inferred UI journeys, universal test adequacy, HDC/MkDocs qualification or accepted technical
intent is delivered by this experimental profile. No change to the default one-binary local boundary.
The optional MCP companion must be started explicitly and opens no network connection.

## Acceptance and rollback

Build/read identity and two-commit source/provider proof precede integration. Focused tests cover
closed schemas, closure before admission, non-vacuous spans, hostile trust promotion, deterministic
bytes, native receipt joins, capability absence/zero, source/provider drift, symlinks, output bounds,
CEM base binding, native opt-in parity, MCP gating/parity and human-prose preservation. Run the
frozen labelled evaluation and real self-corpus proof, then independent integrated review and the
repository's clean-commit `make gate` and dogfood CEM/OCM/strict completion loop. Test execution and
pinned results belong in `docs/BUILD-LOG.md`; tests never accept this spec. Promotion requires owner
acceptance and all scoped evidence; a failed bound/trust/parity case blocks completion. Rollback
removes the optional corpus entry points/packages/profile artifacts; existing native commands,
original sources, retained observations and human documentation require no migration.

## Traceability

| Requirements | Implementation boundary | Required evidence |
| --- | --- | --- |
| DCP-V1-001..011, DCP-V1-018 | `internal/doccorpus`, immutable `internal/contextindex` adapter | Determinism, input closure, anchor/state/receipt/staleness hostile fixtures |
| DCP-V1-012..015 | `cmd/corvint`, corpus projection API | CLI and native parity, CEM base/provenance and exact observation joins |
| DCP-V1-016 | `internal/mcp/corpusbridge`, `cmd/corvint-corpus-mcp` | Capability gating, transport parity and hostile text |
| DCP-V1-017 | Corpus render and maintenance API | Human byte/permission preservation, malformed/stale/tampered refusal |
| DCP-V1-019..020 | Conformance fixtures and independent example adapter | Labelled evaluation and actual self-corpus receipt |

## Open decisions

Owner acceptance and any promotion beyond the local experimental profile remain open. Additional
render formats, cross-repository inputs, provider transports and authenticated review attestations
require separate contracts; absent capabilities must remain visible until then.
