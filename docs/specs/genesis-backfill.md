# Genesis codebase backfill

Owner: Russell Lewis
Frozen: 2026-08-22
Intent status: accepted
Delivery status: experimental-first-inventory
Authoritative inputs: `docs/TECHNICAL-BRAIN.md`, `docs/LEGACY-ADOPTION.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`

## Agent digest
- Claim: Genesis backfill inventories brownfield repository knowledge before any source is activated as evidence.
- Status: accepted/experimental-first-inventory
- Exists: the accepted inventory contract and experimental activation surfaces.
- Blocked on: qualified repository/external-source adapters and promotion evidence for the complete backfill workflow.
- Read next: User and job; Trust model; Requirements.

## User and job

Given read access to one or more Git repositories and optional Jira, Confluence, or other knowledge
sources, Corvint must build the first useful, maintainable technical brain for an unfamiliar product.
It must recover structure, observed behavior, history, stated intent, ownership, tests, flows,
incidents, and unknowns without requiring a working build or pretending inferred documentation is
truth.

The first useful answer should arrive within five minutes. Deeper backfill is resumable and
incremental; it is not a blocking all-or-nothing crawl.

Corvint has exactly two activation doors:

- `corvint init` starts the evidence ledger for a new agentic repository from its current Git state;
- `corvint adopt` recovers the mechanically available technical history of an existing product.

Both commands compile the same canonical inventory. `init` prioritizes current instructions,
manifests, accepted intent, tests, CI, and ownership. `adopt` additionally schedules bounded Git and
configured external-source history. Neither command requires a build, server, database, ontology,
credential, or model call to return its first useful receipt.

## Trust model

Git proves immutable content identity and implemented structure. An owner-assigned repository
authority ID establishes repository authority identity. Pinned test/runtime records
observe bounded behavior. Accepted repository specifications govern intent. Jira, Confluence,
pull requests, incidents, and other external systems provide dated claims and context. Conflicting
witnesses remain separate. No lower-authority source silently overwrites a higher-authority source.

Every derived statement is `PROVED`, `OBSERVED`, `INFERRED`, `CONFLICTED`, or `UNKNOWN` and carries
its exact source identity, version/content hash, access scope, derivation identity, revision scope,
exclusions, and freshness. Observation/retrieval time for mutable external sources belongs in a
separate run envelope; it is not inserted into immutable Git facts or canonical rebuild identity.

## Requirements

- `GENESIS-001`: `corvint init` and `corvint adopt` MUST accept one or more local Git repositories, pin an exact commit
  and tree per repository, namespace identities by an owner-assigned opaque repository authority ID
  and revision, and perform no implicit fetch. Paths, remotes, hostnames, and commits alone are not
  authority identity; repositories without an assigned ID remain locally useful but non-composable.
- `GENESIS-002`: zero-execution inventory MUST classify every tracked path as included, excluded,
  unsupported, or unknown and identify languages, manifests, entry points, routes, jobs, schemas,
  configuration, tests, specs, decisions, ownership, and generated/vendor boundaries.
- `GENESIS-003`: Git history enrichment MUST recover cited change, ownership, ticket/PR reference,
  co-change, migration, deprecation, and release candidates without treating history as product
  intent.
- `GENESIS-004`: Jira MUST be an optional read-only adapter that ingests stable issue/project IDs,
  versions, links, status, timestamps, configured fields, and content digests while preserving
  unavailable, deleted, and access-denied states.
- `GENESIS-005`: Confluence MUST be an optional read-only adapter for versioned pages, spaces,
  links, attachments, diagrams, ADRs, runbooks, product notes, and incidents. Unsupported attachment
  formats remain explicit rather than silently omitted.
- `GENESIS-006`: external sources MUST use one adapter envelope with source kind/instance, stable
  subject ID, source version, content digest, retrieval time, access scope, typed links, deletion
  state, and adapter version. Source-instance authority uses an owner-assigned opaque ID and records
  a vendor-stable instance ID when available; hostnames and project/space names alone are not
  identity. Without either stable vendor identity or an approved owner authority ID, results remain
  local-only and non-composable. Full mutable bodies are quarantined locally and read only on demand.
- `GENESIS-007`: adapter execution MUST be opt-in, least-privilege, bounded, resumable, rate-limit
  aware, idempotent, and safe to interrupt. Missing credentials produce a coverage gap, not failed
  repository setup.
- `GENESIS-008`: composition MUST obey ACL non-interference: a caller receives no path, identifier,
  digest, aggregate, or derived claim from a repository or source it cannot read. Inaccessible
  evidence becomes `UNKNOWN` or `REDACTED`, never success. Caches, fragments, checkpoints, and
  derived claims MUST bind source instance, scope digest, access-context ID, and adapter version;
  mismatches require invalidation and reconciliation, and raw results from different contexts MUST
  NOT be unioned.
- `GENESIS-009`: the backfill MUST build durable cited claims and deterministic views for architecture,
  features, flows, tests, ownership, incidents, migrations, releases, gaps, and conflicts. Empty
  template sections are forbidden.
- `GENESIS-010`: `corvint spec backfill` MUST emit repository-native Markdown with status
  `inferred-draft`; only repository-owned review may promote it to accepted intent.
- `GENESIS-011`: Jira/PR/Confluence relationships MUST retain both exact links and the extraction
  method. Lexical, model, or co-change associations remain candidates until anchored or approved.
- `GENESIS-012`: incremental sync MUST process only changed Git blobs and changed external versions,
  retain tombstones for deletions, mark dependents stale or needing reverification, and reproduce a
  clean rebuild byte-for-byte for identical authorized inputs.
- `GENESIS-013`: multi-repository answers MUST compose immutable per-repository receipts without
  shared mutable truth, distributed-transaction claims, or cross-repository authority. Missing or
  advanced members produce `PARTIAL`, `CONFLICTED`, or `UNKNOWN`.
- `GENESIS-014`: answers MUST use progressive disclosure and minimum witnesses so agents can expand
  exact handles instead of reopening broad source or repeated external bodies.
- `GENESIS-015`: all caches, external snapshots, generated documents, and indexes MUST be disposable;
  removing Corvint MUST leave source, accepted specs, trackers, and source-system permissions unchanged.
- `GENESIS-016`: all external bodies MUST remain inert untrusted data. Their contents MUST NOT alter
  Corvint policy, invoke tools, execute commands, set trust/authority state, widen scope, or bypass
  repository-owned promotion.
- `GENESIS-017`: after the initial build, query serving MUST use content-addressed incremental indexes,
  automatically widen across authorized repository and adapter evidence under the caller's budget,
  and avoid caller-authored broad search when labelled evidence lies in supported source types. The
  frozen warm corpus targets p95 below one second; a miss returns an exact frontier, never fake speed.
- `GENESIS-018`: Genesis MUST accept independently verifiable session deltas containing evidence
  handles, outcomes, and proposed knowledge changes. It MUST deduplicate prior context, reject stale,
  failed-as-successful, unmerged-as-current, cross-authority, or cross-ACL contributions, and remain
  storage-neutral so the same receipt can update a local store or authorized team service.
- `GENESIS-019`: before any model is eligible, Genesis MUST exhaust the deterministic resolvers
  applicable to that exact frontier item: Git object and history identity; syntax and manifest
  parsing; declared spec, ticket, pull-request, ownership, test, CI, route, schema, configuration,
  and documentation links; exact lexical joins; and bounded co-change candidates. The receipt MUST
  report the mechanical denominator, admitted records, rejected candidates, and unresolved items by
  resolver. A resolver that was unavailable or budget-limited is a gap, not an exhausted resolver.
- `GENESIS-020`: model use MUST be opt-in and provider-neutral. A repository policy supplies an
  ordered registry of model classes with capability, context, locality, privacy, latency, and cost
  limits. Genesis MUST choose the least-cost eligible class with frozen measured acceptance evidence
  for the exact extraction schema, language/task profile, and model revision. It may escalate only after a named anchor/verifier rejection,
  material independent-retriever disagreement, a declared high-risk ambiguity, or a model/runtime
  failure. Price, popularity, or a producer self-rating MUST NOT establish adequacy.
- `GENESIS-021`: every model call MUST be restricted to immutable handles needed for one unresolved
  frontier item and produce a receipt binding the frontier ID, provider/model/version or local model
  digest, prompt/profile/schema digests, ordered input handles, input/output token counts when
  observable, output digest, verifier result, rejection reason, and escalation reason. Prompts,
  source bodies, credentials, and raw model output remain private local data unless an explicit
  repository retention policy says otherwise.
- `GENESIS-022`: a model result MAY create only a `MODEL_PROPOSED_ANCHORED`/`INFERRED` candidate
  after its immutable spans, output schema, and relation-specific mechanical predicates verify. It
  MUST NOT become accepted intent, prove behavior or absence, authenticate an observation, retire an
  obligation, or promote its own authority. When no relation-specific verifier exists, the item
  remains `UNKNOWN` and no model call is justified merely to generate prose.
- `GENESIS-023`: unchanged immutable inputs and an unchanged routing profile MUST reuse the prior
  verified candidate without another model call. A model or prompt/profile change creates a new
  derivation; it never rewrites the earlier record. Corvint MUST expose mechanical yield, model-call
  rate, strongest-class escalation rate, model tokens, verifier acceptance, independently labelled
  precision, abstention, latency, and cost without collapsing them into one confidence score. Model-
  call rate uses all deterministic semantic-gap IDs as its denominator; strongest-class escalation
  uses actual model calls. Mechanical yield is reported per frozen extraction schema/corpus rather
  than over generic repository paths.
- `GENESIS-024`: activation MUST emit a canonical, source-content-free receipt whose operational
  state is `COMPLETE`, `PARTIAL`, or `INVALID`. `COMPLETE` means only that every tracked path and
  configured source cursor was classified within the declared activation scope. `PARTIAL` names
  unsupported formats, missing access, shallow/missing history, parser failures, budget stops, and
  the semantic frontier. `INVALID` advances no reusable knowledge. Semantic unknowns do not make a
  mechanically complete inventory invalid.
- `GENESIS-025`: (proposed 2026-09-22, V1-0008, not accepted) an activation MUST end within its time
  budget and never wait on Git past it. The default budget is 120 s total with 30 s per Git read
  (`defaultLimits`), under the ten-minute fallback bound. A caller deadline or budget that expires
  once the repository has opened and the revision has resolved yields a `PARTIAL` receipt that
  still pins revision and tree, names the single gap `git-timeout`, lists no entries, and records
  zero model calls. A deadline that expires after the repository opens but before the revision
  resolves yields an `INVALID` receipt with the single gap `git-timeout` and no revision. A Git
  read that hangs while the repository opens yields the `INVALID` receipt `invalid-repository`.
  Measured on this repository, 20 runs each, network denied: `init` p95 0.259 s and `adopt` p95
  0.257 s (`docs/BUILD-LOG.md`, 2026-09-22 V1-0008). Falsifier: an activation that outlives its
  deadline by more than the per-read Git cleanup, or a timed-out receipt outside these three
  outcomes.

## Backfill sequence

```text
pin repositories and adapter cursors
  -> zero-execution inventory and orphan frontier
  -> Git history and ownership
  -> external identities/links/digests
  -> deterministic declared/lexical/structural/co-change joins
  -> unresolved semantic frontier
  -> optional smallest-adequate-model proposals on exact frontier items
  -> mechanical anchoring or explicit rejection/unknown
  -> deterministic docs and inferred specs
  -> human promotion where desired
  -> change-only sync plus periodic clean reconciliation
```

Stages checkpoint immutable inputs and may resume without repeating completed work. The default
five-minute path prioritizes inventory, entry points, project instructions, owning specs, mandatory
gates, and the first cited query. Full history and external bodies are background or explicit work.

## Current implementation boundary

The experimental first slice implements one local repository per invocation for `corvint init` and
`corvint adopt`. It pins the commit/tree, classifies every tracked entry within declared bounds, emits
a content-free receipt and bounded independently verifiable agent summary, identifies absent specification evidence as a
semantic frontier rather than an activation failure, and makes zero model calls. `adopt` currently
returns `PARTIAL` with `HISTORY_NOT_SCANNED`; history enrichment, multi-repository orchestration,
external adapters, inferred-spec emission, incremental sync, and semantic model routing are not yet
implemented. The CLI emits the sealed summary by default to conserve context; `--full-receipt`
streams the complete canonical inventory without writing it. Summary samples per source class
order by classification (`UNKNOWN`, `UNSUPPORTED`, `EXCLUDED`, `INCLUDED`), then path, then
`pathSha256`; an entry with an unsafe or non-UTF-8 path has no path and orders as the empty path, so
it is sampled rather than aborting the summary (`TestSummaryOrdersUnsafePathSamplesWithoutPanicking`).
An entry with no path is never read from Git: its blob is not charged to the total blob budget and
does not count in `inspectedEntries` or `inspectedBlobBytes` (`TestInventoryDoesNotReadUnsafePathBlobs`).
When the blob read fails, the `git-read-failed` (or `git-timeout`) gap counts only the entries the
read would have requested: a regular blob with a path and a known size, not excluded, without a
binary-asset suffix, and within the per-blob limit; binary-asset, oversize, and size-unavailable
entries keep only their own reason (`TestBlobReadFailureCountsOnlyEntriesItWouldRead`).
This boundary is intentional and visible rather than simulated.

## External adapter contract

Every read-only adapter implements four bounded operations:

```text
discover(scope, checkpoint, budget) -> records, next checkpoint, completeness
fetch(object identity, version, budget) -> quarantined body, digest
children(object identity, version, budget) -> links, comments, attachments
reconcile(scope, prior manifest, budget) -> added, changed, deleted, access-lost, unknown
```

Stable vendor object IDs govern identity; Jira keys, Confluence titles/slugs, URLs, and parents are
mutable display metadata. If a source has no strong version, Corvint uses the canonical content digest
as weak identity, records observation time separately as provenance, and does not claim historical
reproducibility. Credentials remain in the host secure store; receipts contain only an opaque local
access-context ID.

Explicit deletion, or absence after a complete enumeration under the identical scope and access
context, may create a tombstone. A forbidden/ambiguous-not-found response, scope contraction,
incomplete pagination, missing page, or rate limit becomes `ACCESS_LOST` or `UNKNOWN`, never deletion.
Attachments are metadata-only by default; large binaries, diagrams, and OCR require explicit bounded
policy, and any extracted semantics remain anchored `INFERRED` claims.

An unconfigured adapter reports `NOT_CONFIGURED`. Genesis never substitutes synthetic external data
and never widens permissions merely to improve coverage.

Quarantined bodies use private local storage and a finite repository-owned retention limit. Access
revocation, scope removal, adapter removal, or explicit purge deletes bodies, indexes, and derived
caches for that access context while retaining only policy-permitted tombstone identity/provenance.
No cached body survives merely because another access context can read the same source object.

## Deterministic acceptance matrix

| Contract | Required fixtures |
|---|---|
| Repository/source identity | same commit in two authority IDs stays distinct; mutable path/remote/key/title changes preserve assigned identity; missing repository or source authority identity blocks composition only; vendor-ID/no-vendor-ID cases remain distinct |
| Git isolation | hostile Git configuration, missing/shallow objects, and a network trap prove no implicit fetch and explicit unavailable state |
| ACL non-interference | high/low access contexts over the same scopes prove cache separation and forbid restricted paths, IDs, digests, selectors, counts, and claims in low-access output |
| Resume and limits | interruption at every checkpoint, rate limiting, item/byte/time cap boundaries, and incomplete pagination return a stable partial frontier and resume byte-identically |
| Versions and deletion | strong and weak versions, explicit deletion, complete-enumeration absence, ambiguous forbidden/not-found, scope contraction, and access loss produce their distinct states |
| External-content safety | prompt-like Jira/Confluence bodies, links, comments, diagrams, and attachments remain inert and cannot cause commands, policy changes, scope widening, or trust promotion |
| Retention and purge | cap expiry, access revocation, scope removal, adapter removal, and explicit purge remove context-bound bodies/caches without deleting allowed tombstone provenance or another context's data |
| Incremental maintenance | changed object/blob, tombstone, stale dependent, missed-event reconciliation, and clean-versus-incremental builds produce identical authorized outputs |
| Determinism | genuine-pass, fabricated-fail, and no-input cases independently re-derive every count, citation, coverage denominator, freshness result, and completeness state |
| Mechanical-first routing | model trap proves zero calls while an applicable deterministic resolver can decide; unavailable and budget-limited resolvers remain visible rather than being treated as exhausted |
| Smallest adequate model | frozen registry chooses the least eligible calibrated class; anchor rejection, disagreement, high risk, and runtime failure are the only escalation fixtures; reorder and self-rating attacks do not change selection |
| Model authority ceiling | fabricated semantic output, valid-but-unsupported prose, missing relation verifier, and prompt injection cannot close intent, behavior, observation, absence, retirement, or supersession obligations |
| Model receipt and reuse | exact inputs/profile reproduce the prior derivation without a call; changed blob, schema, prompt profile, or model identity yields a distinct receipt; secrets and source bodies never enter the durable receipt |

## Budgets and failure behavior

The initial implementation supports at most four repositories and two external adapters per run,
with repository-configured byte/item/time caps. It stops with a resumable partial receipt before a
limit, rate limit, permission boundary, parse failure, or missing object becomes a false-complete
result. Cost must scale with changed/cited bytes and changed sources, not organization headcount.

No daemon, hosted account, graph/vector database, organization-wide crawl, credential broker,
automatic write-back, central policy engine, or mandatory model is required. Cross-source model
extraction is attempted only for an exact mechanically unresolved item, accepted only as an
anchored candidate, and content-addressed before reuse.

### Inventory gap and frontier codes

The native inventory receipt (`internal/genesis`) emits the kebab-case codes below (decision 0100).
Each row cites the first emitting site and states only the condition checked there. An entry reason
marked `UNSUPPORTED` or `UNKNOWN` also adds a `gaps` entry counting the entries with that reason. A
Git code (`git-*`, `invalid-git-output-budget`, `malformed-*`) is the single gap of an `INVALID`
receipt when repository opening or resolution fails, and otherwise a tree-read, blob-read, or
status gap of a `PARTIAL` receipt (a malformed tree entry stays `INVALID`).

| Code | First emitting site | At the cited site |
|---|---|---|
| `binary-asset` | `internal/genesis/classifier.go:162@c98f3c07` | an entry reason when the path has a binary-asset suffix; `UNSUPPORTED`, and the blob is not read |
| `binary-content` | `internal/genesis/classifier.go:172@0b731582` | an entry reason when the UTF-8 blob bytes contain a NUL byte; `UNSUPPORTED` |
| `blob-budget-exhausted` | `internal/genesis/classifier.go:166@dc6dac29` | an entry reason when reading the blob would exceed the total blob budget; `UNKNOWN` |
| `blob-size-unavailable` | `internal/genesis/classifier.go:160@30cad9a9` | an entry reason for a regular blob with no recorded size; `UNKNOWN` |
| `blob-too-large` | `internal/genesis/classifier.go:164@4008e6fc` | an entry reason when the blob size exceeds the per-blob limit; `UNSUPPORTED`, and the blob is not read |
| `blob-unavailable` | `internal/genesis/classifier.go:168@1b6cd8ab` | an entry reason for a readable blob whose bytes were not returned, including after a failed blob read; `UNKNOWN` |
| `caller-declared-prefix` | `internal/genesis/classifier.go:152@ba7ddec9` | an entry reason when the path matches a caller-declared exclusion prefix; `EXCLUDED`, so it adds no gap |
| `declared-source-absent` | `internal/genesis/inventory.go:427` | a `semanticFrontier` reason, with count 0, for each of `INSTRUCTIONS`, `SPECIFICATION`, `TEST`, `CI`, and `OWNERSHIP` whose count is zero; none is emitted when the entry budget omitted a tracked entry or a tracked entry has an unsafe or non-UTF-8 path (no path, so no source classes), because those entries could hold that class (`TestSemanticFrontierDoesNotClaimAbsenceOverOmittedEntries`, `TestInventoryDoesNotClaimAbsenceOverUnsafePathEntries`) |
| `dirty-worktree` | `internal/genesis/inventory.go:88@c22fd511` | a `gaps` entry with count 1 when the worktree status read reports a change; `dirtyState` is `DIRTY` and the state is `PARTIAL` |
| `entry-budget-exhausted` | `internal/genesis/inventory.go:58` | a `gaps` entry whose `count` is the number of tracked tree entries beyond the entry limit (tracked minus retained entries), added only when that number is nonzero; it also counts as unresolved, so the state is `PARTIAL` |
| `git-input-budget-exceeded` | `internal/genesis/repository.go:249@bdc997ba` | the Git standard input exceeds the 4 MiB batch input limit |
| `git-output-budget-exceeded` | `internal/genesis/repository.go:266@13f6065c` | Git output exceeds the read limit; the status read treats it as dirty instead |
| `git-read-failed` | `internal/genesis/repository.go:275@fe40410c` | any other Git read failure; also the fallback code at the tree, blob, and status read sites |
| `git-timeout` | `internal/genesis/repository.go:268@63270391` | a Git read exceeds its per-operation timeout |
| `git-unavailable` | `internal/genesis/repository.go:270@899cb4fe` | the Git process cannot start |
| `gitlink` | `internal/genesis/classifier.go:154@67ddbd73` | an entry reason for mode `160000` with object type `commit`; `UNSUPPORTED` |
| `invalid-activation` | `internal/genesis/inventory.go:28@bde5dbae` | the activation is neither `init` nor `adopt`; `INVALID` receipt |
| `invalid-authority-id` | `internal/genesis/inventory.go:31@0156e107` | a supplied authority ID fails the authority character and length check; `INVALID` receipt |
| `invalid-commit-object` | `internal/genesis/repository.go:101@e2b25ed1` | the resolved commit line does not decode as an object ID of the repository format; `INVALID` receipt |
| `invalid-exclusions` | `internal/genesis/inventory.go:35@29afeebb` | the excluded prefixes do not normalize; `INVALID` receipt |
| `invalid-git-output-budget` | `internal/genesis/repository.go:246@cd962ce6` | a Git read requests an output limit below zero or above the tree plus total blob budget |
| `invalid-inventory-receipt` | `internal/genesis/inventory.go:288@54cab4a8` | a summary error: the receipt does not verify or the sample limit is outside 0 to 20; never in a receipt |
| `invalid-repository` | `internal/genesis/repository.go:42@b48db0ea` | the root cannot be made absolute (later sites in `openRepository` refuse an unresolvable, non-directory, or invalid-layout root); `INVALID` receipt |
| `invalid-revision` | `internal/genesis/repository.go:81@bfa83d25` | the revision fails the revision grammar while the root is a valid repository layout; `INVALID` receipt |
| `invalid-tree-object` | `internal/genesis/repository.go:311@deb9a2c8` | the commit tree ID does not decode as an object ID; `INVALID` receipt |
| `malformed-blob-batch` | `internal/genesis/repository.go:502@297fa385` | a blob batch header line is unterminated (later sites refuse a mismatched header, size, or trailing bytes); a blob-read gap in a `PARTIAL` receipt |
| `malformed-tree-entry` | `internal/genesis/repository.go:366@b3356a32` | a tree listing record is empty or unterminated (later sites refuse other framing faults); `INVALID` receipt |
| `mechanical-inventory-only` | `internal/genesis/inventory.go:419` | a `semanticFrontier` reason for each source class with a nonzero count other than `ASSET` and `LOCKFILE` |
| `non-utf8-or-binary` | `internal/genesis/classifier.go:170@c59c7a96` | an entry reason when the blob bytes are not valid UTF-8; `UNSUPPORTED` |
| `receipt-budget-exceeded` | `internal/genesis/inventory.go:265@dfa500e7` | the sealed non-`INVALID` receipt fails to encode or encodes over the receipt byte limit; replaced by an `INVALID` receipt |
| `special-tree-entry` | `internal/genesis/classifier.go:158@0f1ed0c3` | an entry reason for any other entry that is not a `100644` or `100755` blob; `UNSUPPORTED` |
| `summary-budget-exceeded` | `internal/genesis/inventory.go:312@124c0a71` | a summary error: the summary without samples already encodes over 16 KiB of canonical UTF-8; never in a receipt |
| `symlink` | `internal/genesis/classifier.go:156@66f525fa` | an entry reason for mode `120000` with object type `blob`; `UNSUPPORTED` |
| `tracked-text` | `internal/genesis/classifier.go:147@23a04d34` | the entry reason every classifier entry starts with and keeps when no later case matches; `INCLUDED`, so it adds no gap |
| `unsafe-or-non-utf8-path` | `internal/genesis/classifier.go:150@e370fea5` | an entry reason when the tracked path is unsafe or not UTF-8, so the entry has no path; `UNSUPPORTED` |

## Dogfood and promotion

1. Corvint-on-Corvint: ingest Corvint Git alone; answer the ten technical-brain frozen tasks; backfill one
   inferred spec; prove incremental and clean output equality.
2. Beamfall: ingest the authorized Beamfall repositories, local Git history, and a small read-only
   ticket/wiki sample; measure cross-repo identity, ACL, conflicts, and token substitution.
3. Brownfield trial: run on at least two unrelated public or authorized legacy products with a
   second human labeller.

Promote inventory only when every tracked path is classified and unbuildable fixtures succeed.
Promote generated documents only above 0.70 independently labelled statement precision and zero
critical fabrication. Promote anchored semantic edges only above 0.90 precision, with at least 70%
mechanical yield, model calls on at most 30% of semantic-gap items, at least 50% fewer model-input
tokens than an all-largest-model baseline, zero authority promotion, and zero model calls for items
without an applicable verifier. Strongest-class escalation MUST remain below 5% unless the sealed
corpus demonstrates a narrower class cannot meet the same precision. Promote the full
backfill only with at least 30% lower median agent input tokens, at least 25% lower total tokens over
complete multi-turn runs, non-inferior correctness, zero extra critical misses, byte-identical
clean/incremental results, warm p95 query latency below one second on the frozen reference corpus,
and no access-boundary leak.

Kill or narrow any adapter that cannot preserve versions, deletions, access scope, resumability, and
explicit gaps. Kill automatic synthesis if it becomes a template dump, requires broad rereads, or
exceeds 10% correction on critical material. Kill the semantic router below 0.85 independently
labelled accepted-edge precision, above 50% model-call rate, without material token savings, or after
any model proposal closes an obligation without a deterministic verifier. Keep multi-repository composition deferred if it needs
shared mutable state or exposes cross-ACL metadata.

## Traceability

The `src/context_corvint_genesis.py`/`src/corvint_cli.py` citations below are historical implementation
references, not live authority: decision 0012 R0
(`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`) rules the Python implementation
non-authoritative and slated for separate removal. The native surface is `internal/genesis` plus
`cmd/corvint` (`docs/specs/go-production-kernel-migration-v0.md`, `GPK-V0-032`).

| Requirement | Implementation | Evidence |
|---|---|---|
| `GENESIS-001` | `src/context_corvint_genesis.py`; `src/corvint_cli.py` (single local repository) | hostile Git environment, exact identity, read-only CLI tests; multi-repository orchestration pending |
| `GENESIS-002` | `src/context_corvint_genesis.py` mechanical inventory | tracked denominator, classification, source-class, boundary, hostile-path, and budget tests; language-specific parsing pending |
| `GENESIS-019` | resolver-accounting and zero-call receipt fields | mechanical resolver denominator and zero-model trap tests; remaining resolver families pending |
| `GENESIS-024` | canonical inventory receipt and bounded summary | determinism, tamper, dirty, budget, no-spec, invalid-input, unsafe-path frontier, unsafe-path blob read, blob-read-failure count, unsafe-path summary order, and summary-bound tests |
| `GENESIS-025` (proposed) | `defaultLimits`, `runRaw`, the tree-read fallback in `CompileRepositoryInventory` | `TestActivationFallsBackToABoundedReceiptWhenGitHangs` (`internal/genesis/activation_fallback_test.go`); timings in `docs/BUILD-LOG.md` (2026-09-22 V1-0008) |
| `GENESIS-003..018`, `GENESIS-020..023` | not implemented | Corvint, Beamfall, external-source, semantic-routing, and brownfield gates pending |
