# Deployment-neutral immutable index platform V0

- Owner: Russell Lewis
- Date: 2026-08-26
- Intent status: accepted direction
- Delivery status: not-started
- Decision: [`../decisions/0001-deployment-neutral-immutable-index.md`](../decisions/0001-deployment-neutral-immutable-index.md)
- Authoritative inputs: the repository owner's 2026-08-26 architecture handoff,
  `AGENTS.md`, `docs/PRODUCT.md`, `docs/ARCHITECTURE.md`,
  `docs/specs/analyzer-capability-contract-v0.md`,
  `docs/specs/go-production-kernel-migration-v0.md`,
  `docs/specs/mcp-server-2026-07-28-v0.md`,
  `docs/specs/pulse-snapshot-lease-v0.md`, and
  `docs/specs/human-documentation-compiler-v0.md`
- Owner-handoff provenance: controlled record `owner-handoff/2026-08-26`, SHA-256
  `d2d13ba69af2fa2b3691fbcf6ddc9601eecfcebba2ea07fbf7ee5103954b7423`; the private archive path is
  deliberately not a publication input. Documentation-maintenance reference SHA-256:
  `068ad43cc546bbfc92b88e08d8c831e5724d1aeb592008b449bda6fafaa38c76`.

## Agent digest
- Claim: One immutable transport-neutral index contract must preserve evidence semantics across local, remote, and synchronized deployments.
- Status: accepted direction/not-started
- Exists: experimental local Go query and impact slices; the umbrella platform is not delivered.
- Blocked on: immutable-format selection, scale qualification, and separately accepted descendant profiles.
- Read next: User and measurable job; Verified current state; Platform invariants.

## User and measurable job

An engineer must be able to query an authorized local Git checkout with one terminating Go
process, while a cloud MCP worker, exact-commit review agent, and documentation compiler can reuse
the same canonical immutable index bytes and proof-carrying receipts. Git remains durable evidence
authority. Missing objects, permissions, evidence, or qualification produce gaps or abstention,
never invented certainty.

This document is an umbrella platform contract. Descendant slice specs may tighten limits or freeze
wires, but may not weaken these requirements. Nothing in this document is delivered merely because
the direction is accepted.

## Verified current state

At integration base `6b9233dd71c204ff33c38f20b9aab8cfc984d14f`:

- `internal/contextindex/index.go` retains repository-scale source structures in memory, while
  `internal/contextindex/git.go` admits repository bodies under a 128 MiB class boundary. No
  million-file Go pack benchmark exists.
- local Go query and impact slices exist, but general query/cutover remain experimental under the
  Go migration contract.
- MCP V0 is local stdio, one root, source-content-free, without HTTP, authentication, remote
  repositories, or multi-root selection.
- Pulse explicitly excludes a qualified watcher, daemon, database, persistent cache, and cloud
  service from its current stage.
- the Human Documentation Compiler is proposed and not started. Its renderer capsule and initial
  plan/compiler contracts do not constitute a delivered continuous maintenance system.
- object-store range latency, tenant isolation, cross-mode byte identity, a qualified pure-Go
  semantic encoder, and production-scale format/RSS claims are `NOT_RUN`.

## Definitions

- **Engine**: production indexing, retrieval, evidence compilation, root verification, and receipt
  generation. The engine is native Go and does not invoke Python.
- **Renderer**: a repository-owned, pinned downstream process such as MkDocs/Python,
  Docusaurus/Node, or Hugo. A renderer is outside the engine and requires its own containment tuple.
- **Canonical root**: an immutable manifest that binds every required packed section, profile,
  policy, adapter set, source snapshot, and ordered aggregate of fragment capability-cache identities.
- **Fragment**: content extraction keyed by object format, blob OID, extractor/adapter ABI,
  language/profile, and the complete prevalidated `ACC-V0-018` capability-cache identity that
  produced it; Core-only extraction uses an explicit Core identity.
- **Placement**: tree-specific path, mode, fragment, authority, and policy binding.
- **Catalog**: small mutable routing/control state. It is never evidence authority.
- **Database**: the complete system described here; the term does not require a DBMS. The local
  prohibition is against a mutable/external database service, not a benchmark-selected immutable
  embedded index encoding.
- **Canonical receipt core**: mode-neutral canonical bytes binding source identity, root/profile
  identity, result, evidence, exclusions, limits, uncertainty, and abstention.
- **Delivery envelope**: mode-specific authority, tenant, selector, generation, consistency,
  observation, lag, and freshness fields wrapping the canonical receipt core.

Every canonical receipt core binds this logical key:

```text
CanonicalReceiptKey {
  receiptSchemaVersion
  receiptCanonicalizationVersion
  objectFormat
  commitOid
  treeOid
  indexRootDigest
  engineProfileDigest
  adapterSetDigest
  retrievalProfileDigest
  policyDigest
  overlayDigest?
  requestDigest
  capabilityIdentityDigest
}
```

`requestDigest` is over the complete normalized mode-neutral query operation, arguments, limits,
requested evidence classes, and abstention policy after selector resolution; mutable selector and
consistency observations remain in the delivery envelope. `capabilityIdentityDigest` binds the
ordered closed set of requested and resolved Core/external-plugin capabilities, including exact
profile, registry snapshot, lock, plugin, release, build, manifest, protocol, artifact, projection,
host/target, clause, and verifier identities required by `ACC-V0`; Core-only is one explicit value.
Receipt schema and canonicalization versions are independent opaque identities. A cache lookup,
replay comparison, or cross-mode parity claim requires exact equality of every key field; no result
may be reused across a different request, capability set, schema, or canonicalization version.

Every delivery envelope binds tenant/repository authority where applicable, selector kind/name,
generation, requested consistency, pre/post authority observations, observed and indexed/published
heads, lag (`READY | INDEXING | STALE | UNKNOWN | ACCESS_LOST`), freshness
(`VALIDATED_AT | HISTORICAL`), named observer model, and observation window. Local envelopes omit
tenant/authentication fields rather than inventing them. No mutable branch is described as timelessly
`CURRENT`.

## Simpler baseline and non-goals

The simpler baseline is the current terminating resident Go index/query path plus local stdio MCP,
with disposable derived cache state and explicit unsupported profiles. P0 compares that baseline
against immutable SQLite and a custom pack; it does not presuppose either challenger wins.

This contract does not authorize separate local/cloud engines, a mutable external DBMS, a required
daemon, Python in the engine, automatic repository upload, source-repository documentation writes,
vector authority, silent fetch, accounts in the local profile, or any delivery/performance claim
before the named gate. Hosted, review, documentation, and vector descendants remain optional and
versioned.

## Requirements

### Immutable Go storage and query kernel

- `DNIP-IDX-001`: Production indexing and query execution MUST be Go-only. The promoted local
  binary MUST require no Python interpreter, Python package, cgo, package manager, account, network,
  hosted service, or resident daemon. Git remains the only required external executable.
- `DNIP-IDX-002`: One transport-neutral kernel and one canonical index encoding MUST serve local
  files, authenticated cloud object ranges, exact review deltas, and documentation jobs. Identical
  snapshot/profile inputs MUST produce byte-identical roots and canonical receipt cores across
  modes. Mode-specific delivery envelopes are not byte-equal; their exact normalization and
  permitted field differences are versioned and conformance-tested.
- `DNIP-IDX-003`: Extraction fragments MUST be keyed by object format, blob OID, adapter ABI, and
  language/profile plus the complete prevalidated capability-cache identity. For an external
  analyzer this includes every `ACC-V0-018` registry, lock, plugin, release, build, manifest,
  protocol, artifact, profile/projection, host/target, input/request, admission, and exact-verifier
  identity; a Core fragment binds the explicit Core-only value. The root binds the ordered aggregate
  of those identities, and query-result lookup binds the complete `CanonicalReceiptKey`. Tree
  placement MUST separately bind path, mode, fragment, authority, and policy. Any component change
  creates a different fragment/root candidate before lookup; it cannot relabel cached facts. Because
  current `ACC-V0-018` binds the full snapshot and request, an external fragment is not reused across
  a changed tree unless a separately accepted ACC amendment first defines and proves a narrower
  snapshot-independent projection. Core-only content fragments retain blob-level reuse.
- `DNIP-IDX-004`: Queries MUST use tree-scoped packed segments, not millions of independently opened
  per-blob files. The root MUST bind bounded directories for path/symbol dictionaries, lexical
  postings/statistics, typed structural/reference/import/test/authority edges, evidence/exclusions,
  reverse-documentation dependencies, and optional vector sections.
- `DNIP-IDX-005`: Every packed block MUST be independently checksummed and bound by the Merkle root.
  A query MUST verify every touched byte range before decoding it.
- `DNIP-IDX-006`: The kernel MUST expose a bounded `ReaderAt`-style interface. Local `pread` is the
  baseline; mmap is a measured challenger only for small immutable tables. Cloud workers fetch only
  required authenticated ranges through a bounded local block cache.
- `DNIP-IDX-007`: Queries MUST NOT deserialize the entire posting/vector corpus or retain all source
  bodies by default. Lower configured limits spill, degrade, widen, or abstain explicitly rather
  than OOM.
- `DNIP-IDX-008`: Publication MUST be manifest-last and crash-atomic: readers select either the old
  complete root or new complete root, never a hybrid. Engine/profile upgrades use side-by-side
  digest namespaces; no in-place migration is permitted.
- `DNIP-IDX-009`: Immutable SQLite MUST be benchmarked as the safety baseline. A custom pack may be
  selected only when it materially wins controlled scale/RSS/I/O outcomes while matching parity,
  corruption, crash, and rollback behavior.

### Local lifecycle, triggers, worktrees, overlays, quotas, and GC

- `DNIP-LOC-001`: `corvint query` MUST open immutable packs directly in one terminating process. An
  IDE, agent, or stdio MCP session MAY own a repository-scoped accelerator that exits on client EOF
  or bounded idle timeout. Permanent login daemons are not required.
- `DNIP-LOC-002`: Hooks, watchers, provider events, and editor notifications are acceleration hints
  only. Every query independently captures Git identity, selects an exact root, and revalidates when
  live currency is requested.
- `DNIP-LOC-003`: Hook integration MUST be explicit, composable, and non-destructive. A hook only
  enqueues bounded nonblocking invalidation; it MUST NOT index, vectorize, render docs, or leave
  descendants. Missing, duplicate, reordered, bypassed, or overflowed events affect warming only.
- `DNIP-LOC-004`: Linked worktrees MUST share immutable Git-common-directory fragments while keeping
  HEAD, dirty generation, and overlay identity separate per worktree. Previously indexed branch
  checkout is an atomic root selection.
- `DNIP-LOC-005`: Partial-clone missing objects produce an explicit frontier and MUST NOT trigger an
  implicit fetch. Dirty content requires a separately accepted content-addressed overlay; until
  qualified, Corvint retains explicit `freshness.state=mixed-worktree` behavior while top-level state describes the immutable-tree retrieval or proof result.
- `DNIP-LOC-006`: Local stores MUST enforce visible per-repository/user disk quotas and eviction
  state. GC retains active, leased, last-good, review, and published-documentation roots.
- `DNIP-LOC-007`: Default decoded-block cache is a proposed 64 MiB target. Query RSS <=128 MiB p99
  and indexing RSS <=256 MiB p99 on the million-file corpus are promotion targets, not current
  claims.
- `DNIP-LOC-008`: On battery, Corvint performs query-required core work only. Speculative warming,
  vectors, history mining, and documentation builds are deferred. Eight-hour active-idle target is
  median CPU <0.1% of one core, no periodic writes, and stable RSS.
- `DNIP-LOC-009`: Hook promotion requires controlled hook latency <=5 ms p95 and <=50 ms max, zero
  Git-operation failures, no escaped descendant, and exact one-shot query parity. Missing or failed
  hooks remain warming misses, never correctness failures.
- `DNIP-LOC-010`: Root publication, selection, leasing, and reclamation MUST share one explicit
  linearization protocol. Locally, one owner-private repository/profile writer lock serializes
  catalog mutation; hosted adapters MUST provide an equivalent linearizable compare-and-swap over a
  monotonically identified catalog epoch. A reader acquires a root-and-epoch lease before opening
  any segment in the same lock/transaction; failure to acquire causes bounded retry or abstention.
  Publication validates the complete manifest and compares the expected epoch before advancing the
  alias. GC first CASes `LIVE` to an epoch-bound `TOMBSTONED(markID)`. After the configured grace it
  may CAS that exact mark to `RECLAIMING(fenceToken)` only in the same linearizable operation that
  proves the root is not current/last-good and has no active lease. A lease, publication, or mark
  invalidation racing before that CAS wins and restores/retains `LIVE`; once `RECLAIMING`, new leases
  and publications naming the fenced root reject or retry until idempotent deletion reaches `DELETED`.
  Recovery resumes a fenced deletion but never deletes from `LIVE` or `TOMBSTONED` alone.
- `DNIP-LOC-011`: Lease ownership MUST be independently fenced after an owner crash. A local lease is
  held by an owner-private verified file descriptor and an OS lock whose release on process death is
  authoritative; its catalog record binds a boot/session nonce and process birth identity. Recovery
  removes an orphan only after acquiring the exclusive OS lock and atomically matching that record.
  Hosted leases belong to a linearizable coordinator session and monotonically increasing fencing
  token; expiry, explicit revocation, or owner-session loss must atomically advance the token before
  reclamation, and every later range/open/renew operation rejects the old token. Time may trigger the
  fencing CAS but is never by itself evidence of owner death. Cancellation releases owned leases and
  locks. Unreachable incomplete namespaces may be removed only after proving they were never
  selectable.

### Hosted MCP HTTP and repository authority

- `DNIP-MCP-001`: Hosted authenticated Streamable HTTP MCP MUST be a separate accepted profile. The
  local stdio MCP executable MUST NOT be exposed directly to the internet or silently gain a
  listener.
- `DNIP-MCP-002`: The server authenticates the principal and selects repositories only by
  server-issued opaque handles. Client paths, URLs, provider IDs, Git OIDs, and MCP metadata are
  never authorization.
- `DNIP-MCP-003`: Authorization MUST occur before catalog lookup and before every object/segment
  fetch. Read-only MCP/query workers MUST NOT directly mutate Git, source, documentation, trace,
  publication, aliases, queues, or control-plane state. A strict-consistency operation MAY send one
  authenticated, idempotent, deadline-bounded reconciliation/root-build request to the separate
  control plane. That request binds principal, opaque repository, expected commit/ref observation,
  policy/config epoch, request identity, and deadline; its enqueue/build/promotion receipt is
  returned separately. Timeout or duplicate/conflicting identity abstains and never weakens the
  requested consistency.
- `DNIP-MCP-004`: Webhook ingress and reconciliation are mutating control-plane APIs and MUST remain
  separate from read-only MCP tools, deployments, credentials, and authorization policy.
- `DNIP-MCP-005`: `PINNED(commit)` serves/builds only the exact root or returns
  `INDEX_NOT_READY`/bounded-wait. `DEFAULT_BRANCH_FAST` returns last-published with explicit lag.
  `DEFAULT_BRANCH_STRICT` resolves H0, waits/builds H0, resolves H1, and returns `VALIDATED_AT` only
  when H0 == H1; otherwise it retries within deadline or abstains. Every `VALIDATED_AT` envelope
  binds the named observer model and exact observation window; it is never a timeless currency
  claim.
- `DNIP-MCP-006`: The revision barrier is
  `atLeastObservation(repositoryHandle, refIdentity, generation, configEpoch, observedCommit)`, not
  commit ordering. The server first verifies that the durable provider observation at exactly that
  generation has the supplied ref/config/commit tuple, then waits within the caller deadline for an
  authorized published alias whose generation is at least that value in the same ref/config epoch.
  A newer returned commit is permitted and is reported exactly; a ref/config epoch change conflicts
  explicitly. A→B→A observations remain distinct by generation. Exact-commit readiness uses
  `PINNED(commit)` instead. Neither ancestry nor timestamps implement the barrier, and it never falls
  back to an older generation.
- `DNIP-MCP-007`: A hosted descendant MUST freeze a separate remote source-content policy before
  exposing any source body or span. The policy binds authorization, redaction, byte/line limits,
  tenant/repository/root identity, receipt fields, cache handling, and denial behavior. The default
  hosted profile remains source-content-free; local stdio policy does not silently transfer.

### Git-provider synchronization and generation publication

- `DNIP-SYNC-001`: Provider events MUST be authenticated and admitted through one durable inbox
  transaction keyed by provider, installation, repository, and delivery ID. That transaction stores
  the payload digest and the idempotent queue-work identity atomically. An identical duplicate
  resolves to the existing durable item; a delivery-ID/body conflict fails closed.
- `DNIP-SYNC-002`: Corvint MUST acknowledge only after the atomic inbox-plus-enqueue transaction
  commits. Consumers are idempotent by the durable inbox/work identity, retain a bounded terminal
  result and processing receipt, and may resume an unfinished item without creating a second logical
  build or promotion. Provider metadata resolves the actual configured ref or default branch; Corvint
  never hard-codes `master` globally. Conformance injects a crash before and after every inbox write,
  uniqueness check, queue write, commit, acknowledgement, claim, result write, and retry boundary;
  no schedule may lose an acknowledged event or strand a dedupe marker without executable work.
- `DNIP-SYNC-003`: Corvint generations are monotonic control-plane values. Timestamp order and commit
  ancestry MUST NOT order publication.
- `DNIP-SYNC-004`: Builders create/reuse the exact-commit root, upload and verify every segment, then
  publish the manifest last. A branch alias advances only after re-resolving the provider ref and
  atomically matching generation, branch/config epoch, and commit.
- `DNIP-SYNC-005`: Force-push A→B→A, branch rename/delete, repository transfer, permission loss,
  event storms, rate limits, missed events, and out-of-order build completion MUST preserve truthful
  historical roots and never publish stale state as current.
- `DNIP-SYNC-006`: Periodic bounded reconciliation repairs missed events; a strict query MAY trigger
  reconciliation only through `DNIP-MCP-003`'s separate authenticated trigger and remains bounded by
  its deadline, idempotency identity, authorization, and explicit lag states.

### Tenant-isolated remote ranges

- `DNIP-TEN-001`: Remote CAS and caches MUST be namespaced by tenant or access context. Cross-tenant
  global deduplication MUST NOT expose digest existence, size, timing, count, membership, error, or
  cache state. Raw CAS keys are never returned.
- `DNIP-TEN-002`: Range fetches MUST bind tenant/repository authority, object identity, manifest
  root, offset, and length; every range is authenticated and integrity-verified before decode.
- `DNIP-TEN-003`: Revocation returns `ACCESS_LOST`; it MUST NOT serve stale success or destructively
  delete historical evidence. Isolation tests cover lexical, structural, graph, vector, cache,
  receipt, error, and timing surfaces.

### Exact code-review snapshots

- `DNIP-REV-001`: `REVIEW_PINNED(baseOid, headOid)` resolves provider PR identifiers to exact
  authorized commits. It may reuse unchanged Core-only content fragments and compile changed-head
  Core fragments/placements. Under current `ACC-V0-018`, it MUST recompute all selected external
  capability work with the head's full snapshot/request identity and MUST NOT copy or relabel the
  base's external facts; narrower reuse requires a separately accepted ACC amendment and hostile
  clean/incremental parity gate.
- `DNIP-REV-002`: Review receipts bind base, head, both trees/roots, and the Merkle/evidence delta.
  A moving PR head is re-resolved before return or produces `PR_HEAD_MOVED`.
- `DNIP-REV-003`: Synthetic merges require explicit identity. Forks, shallow history, inaccessible
  commits, and missing objects produce typed uncertainty. Dirty review content requires a digested
  overlay and is never confused with committed head.
- `DNIP-REV-004`: Temporary review roots are leased long enough that disappearing provider refs do
  not erase reproducible receipts.

### Optional semantic/vector challenger

- `DNIP-VEC-001`: Vectors are optional immutable sections in the same root, never a separate
  canonical database. `CORE_EXACT` may publish with vectors visibly unavailable;
  `HYBRID_REQUIRED` waits for exact-profile completeness.
- `DNIP-VEC-002`: A `VectorProfile` binds embedding model, tokenizer, chunker, dimensions,
  quantization, metric, search algorithm, and algorithm parameters by digest. Published roots MUST
  NOT mix tree/model/profile bytes.
- `DNIP-VEC-003`: Progression is deterministic quantized exact scan, then fixed-build HNSW, IVF-PQ,
  and disk-oriented layouts only when measured scale gates justify each step. A managed vector
  service is a separately named experimental candidate source.
- `DNIP-VEC-004`: A canonical local/cloud semantic profile requires a pinned Go-compatible local
  encoder. The engine MUST NOT hide Python or a required network model.
- `DNIP-VEC-005`: Similarity maps back to manifest-pinned source spans and remains advisory. It MUST
  NOT establish authority, prove impact/absence, close obligations, or displace structured evidence.
- `DNIP-VEC-006`: Promotion requires held-out critical-recall@5 gain >=0.10, zero additional critical
  misses, top-five success >=0.90, byte-weighted precision >=0.80, abstention accuracy 1.0, and, for
  ANN, recall@50 >=0.99 versus exact scan plus material latency/RSS improvement.

### Living documentation compiler and maintenance

- `DNIP-DOC-001`: Each documentation repository owns a `DocumentationProfile` binding source/read
  and documentation/write repositories, exact refs, behavioral and governance authority,
  artifact schemas, topology, citation/identity rules, capability boundaries, quarantined inputs,
  validators/environment pins, exclusions, renderer, PR/approval policy, and permanent gaps.
- `DNIP-DOC-002`: The system distinguishes descriptive current behavior, prescriptive compiler
  rules, and named-owner intent. Contradicted intent is visible drift and MUST NOT silently rewrite
  descriptive behavior.
- `DNIP-DOC-003`: Evidence access is capability-separated into `METADATA_ONLY`, `QUARANTINED_RAW`,
  `STRUCTURED_CLAIMS`, and `PUBLISHABLE`. Raw source/external bodies and embedded instructions MUST
  NOT enter planning/authoring context; only structured records and agent-authored summaries cross.
- `DNIP-DOC-004`: Every publishable factual statement is falsifiable and cited. Source citations bind exact
  repository, commit/blob, path, span, and span digest. External citations bind immutable archived
  snapshots. Unplaceable claims use a null anchor plus `needs-source-check`; line 1 is not a token
  anchor merely because a file exists. Titles, navigation, ordering, visual structure, and fluent
  connective prose are not behavioral evidence and MUST NOT imply an uncited fact.
- `DNIP-DOC-005`: Core-only re-extraction reads changed source, not prior claim prose. Under current
  `ACC-V0-018`, every selected external capability recomputes against the new complete source
  snapshot/request identity before affected-document selection; no old external fact is retained or
  relabeled merely because its blob is unchanged. Prior claim IDs are handled separately and rebound
  only when the new claim matches; changed identity records supersession. Uncertain claims remain
  visible until resolved or proven inert; absence of a new extraction is not permission to drop one.
- `DNIP-DOC-006`: Immutable reverse dependencies bind source span → evidence → claim → clause →
  flow/page → navigation/config → site artifact → publication. Incremental regeneration touches only
  proven-affected content but publication performs a full strict renderer build until equivalence is
  separately proven.
- `DNIP-DOC-007`: Canonical claims and provenance are Git-tracked or stored in a durable, backed-up,
  exportable content-addressed archive. A site manifest binds source commits, index root, profile,
  claim/evidence roots, external snapshots, page inputs, renderer identity, validation receipts,
  outputs, provenance, and approval. Publication fails if any canonical input is missing.
- `DNIP-DOC-008`: Source repositories remain read-only. Normal mutations target only the
  documentation repository through scoped reviewed pull requests and passing gates. Local dirty
  previews are `PREVIEW_UNPUBLISHABLE`; cloud publication consumes exact clean roots and revalidates
  branch head immediately before publish.
- `DNIP-DOC-009`: Repository-owned MkDocs/Python, Docusaurus/Node, Hugo, or another renderer runs as
  an isolated pinned downstream authority with exact executable/version/lock/config/plugin/argv/
  environment/input/output identities. Corvint MUST NOT emulate renderer configuration semantics or
  silently install/substitute tools.
- `DNIP-DOC-010`: Independent validators never rewrite inputs to pass. Required gates cover schema,
  citations/spans/digests, reference resolution, anti-fabrication, write-time/content-digest bulk
  evidence, quarantine, write boundaries, privacy/secrets, permanent gaps, sensitive exclusions,
  and propagation dry run. No-input is `NOT_RUN`/failure. Every scenario has observed red-before-green
  evidence.
- `DNIP-DOC-011`: Sensitive defects remain access-controlled and excluded from publication. Corvint
  MUST NOT file external tickets, disclose exploitation detail, or claim resolution without explicit
  authority.
- `DNIP-DOC-012`: Claim lifecycle is explicit: `GENERATED`, `VERIFIED`, and `REVIEWED` are monotonic
  review states, while `STALE` is an orthogonal currency flag. Regeneration cannot inherit review,
  and publication cannot hide stale/uncertain/contradicted claims.
- `DNIP-DOC-013`: A claim or clause may be removed only when immutable evidence proves it inert,
  superseded, or out of profile. Valid empty-claim outputs remain valid and MUST NOT be bulk-filled
  or assigned line-1 anchors to satisfy counts.
- `DNIP-DOC-014`: A low-reliability facet remains available as visibly weak browsing evidence but
  MUST NOT drive routing, authority, publication, obligation closure, or absence. Its known-red
  reliability state is retained; promotion is not retried until new evidence or an owner-approved
  profile change reopens the gate.
- `DNIP-DOC-015`: Permanent gaps are first-class queryable records binding source/facet, reason,
  observation, and owner/profile decision. Corvint MUST NOT repeatedly retry, infer through, or treat
  them as empty/documented. Incomplete, inaccessible, or uningested material remains explicitly
  incomplete until a new observed input changes the gap.
- `DNIP-DOC-016`: Every update binds the exact triggering merge and source/documentation heads.
  Before proposing or publishing, Corvint merges documentation changes newer than the trigger and
  recomputes the affected set. A moved head supersedes/restarts the candidate rather than overwriting
  newer work. Generation, validation, merge, or publication failure retains the last-good public
  artifact and exposes a pending/stale state with exact failure provenance; it never silently
  advances or drops the update.

### Cross-mode conformance and controlled benchmarks

- `DNIP-XM-001`: Clean, incremental, local, cloud, review, corrupt-cache-rebuild, and no-cache modes
  MUST produce byte-identical roots and canonical receipt cores for the same snapshot/profile.
  Versioned delivery envelopes may differ only in their enumerated mode fields. Cache deletion or
  corruption changes performance only.
- `DNIP-XM-002`: Conformance includes randomized commit/checkout/merge/rewrite/crash/event schedules,
  force-push/rename/delete, out-of-order completion, crash at every publication/GC boundary, missing
  objects, read-only stores, and touched-range corruption. Promotion requires at least 1,000,000
  deterministic seeded schedules with every seed and minimized failure retained.
- `DNIP-XM-006`: Receipt conformance includes hostile cross-request, cross-capability,
  cross-schema/canonicalization-version, and selector-envelope replay. A one-byte semantic request or
  capability change MUST change the canonical key; mode translations of the same normalized request
  MUST retain the same key and core while their enumerated delivery-envelope fields may differ.
- `DNIP-XM-007`: Incremental/root-cache conformance changes one `ACC-V0-018` registry, lock, plugin,
  release, artifact, projection, request, admission, or verifier component at a time. No prior
  fragment/fact/root may be reused under the new identity, and the resulting incremental root MUST be
  byte-identical to a clean rebuild. GC schedules cover every transition and crash in
  `LIVE→TOMBSTONED→RECLAIMING→DELETED`, lease acquisition at each boundary, local process death and
  PID reuse, hosted owner-session loss/fencing, and repeated recovery.
- `DNIP-XM-003`: Benchmarks use reproducible 10K/100K/1M/10M-file and 1/10 GiB source corpora; 1,
  10, 100, and 10K changed blobs; and 1, 10, and 100 worktrees. Each performance profile uses at
  least 100 fresh-process measurements and reports p50/p95/p99/max, failures, CPU, heap, RSS/PSS,
  faults, bytes, syscalls, disk, inodes, amplification, and cache hit rate.
- `DNIP-XM-004`: Proposed targets are local warm query <=35 ms p95 at 100K and <=75 ms at 1M;
  fresh open+query <=150/300 ms; one-file delta <=100 ms; 100-file delta <=750 ms; one-percent
  changes <=3% clean bytes and <=10% clean wall time; cloud warm/cold range <=100/250 ms; hot-base
  100-blob/10 MiB review delta <=2 s. None is claimed before controlled evidence.
- `DNIP-XM-005`: Documentation gates require semantic equivalence between initial and incremental
  generation, 100% resolvable citations, zero fabricated claims, >=0.95 affected-paragraph recall,
  <=5% unrelated churn, >=90% clause correctness, >=80% minor-edit acceptance, and >=40% measured
  maintenance-latency improvement against the declared baseline.

### Compatibility, rollback, and rule ownership

- `DNIP-COMP-001`: Existing V0 local stdio MCP, Go migration, Pulse, and Human Documentation Compiler
  contracts remain scoped and binding. This contract authorizes direction, not silent implementation
  inside those profiles.
- `DNIP-COMP-002`: Each deployment/capability slice freezes versioned index, adapter, receipt,
  policy, and renderer/vector profiles before promotion. Unknown versions fail closed or abstain.
- `DNIP-COMP-003`: Rollback atomically selects the last-good root/profile and disables the new
  adapter/control plane without rewriting Git or historical receipts. Historical roots remain
  readable while retention policy permits.
- `DNIP-COMP-004`: A failed promotion retains exact failure evidence and leaves current production
  selection unchanged. Dates and implementation presence never promote delivery status.
- `DNIP-COMP-005`: The current checkout keeps its existing license until the owner-approved
  transition in `../decisions/0002-future-publication-transition.md` passes. Future public Corvint is
  one parentless root commit: AGPL product paths, Apache-2.0 paths exactly enumerated by that
  decision, no predecessor-project name in commit body/history, and no push/release before
  readiness. This
  architecture records that direction but does not itself change `LICENSE`, push, or release.

## Trust, resource, and failure policy

Repository bytes, Git/provider output, manifests, catalogs, ranges, caches, renderer inputs,
external evidence, events, and model output are untrusted. Identity, authorization, integrity,
canonicalization, publication, quarantine, and tenant-isolation failures fail closed. Missing
evidence, optional vectors, unavailable roots, or unqualified renderers produce typed gaps,
historical state, or abstention. Performance misses block promotion but never change correct output.

No silent cap, sampling, skipped input, background fetch, fallback to older consistency, or
cross-tenant deduplication oracle is permitted. Every bound reports the applied count/bytes/time and
reason.

## Required delivery sequence

1. `P0` freeze benchmark corpora and compare current resident index, immutable pack prototype, and
   immutable SQLite snapshot. Freeze bytes only after corruption and parity evidence. In parallel,
   deliver the accepted Analyzer Capability Contract's registry, lock, protocol, containment, and
   rollback prerequisites before any full-polyglot P1 shadow claim.
2. `P1` implement the local exact kernel, bounded streaming extraction, fragment CAS, packed
   segments, one-shot query, quotas, crash safety, GC, upgrades, and linked worktrees.
3. `P2` add client-owned event warming, hook composition, coalescing, query reconciliation, and
   battery/idle controls.
4. `P3` add exact base/head review roots and changed-blob deltas.
5. `P4` add authenticated object ranges, opaque repository handles, tenant isolation, and stateless
   cloud query workers.
6. `P5` add authenticated webhook ingress, durable queue, reconciliation, generation publication,
   consistency modes, and revision barriers.
7. `P6` deliver documentation profiles, quarantined evidence views, durable claims, full/incremental
   compilation, independent validators, PR-only publication, and pinned renderers.
8. `P7` run the semantic/vector challenger only after structured retrieval misses a registered gate.

Cloud, documentation maintenance, and vectors MUST reuse the accepted local kernel. They MUST NOT
create separate index semantics.

## Acceptance and promotion

Every normative scenario must have a corresponding gate observed red before green. P0 format
implementation cannot begin until the deployment-neutral index platform plan (an internal planning record, not part of the public tree)
passes independent Gate A with no HIGH concerns. Each later slice requires its own accepted wire/
profile spec, exact tests, scale evidence, self-dogfood evidence, and independent review.

One stale root labeled current, cross-tenant disclosure, unverified touched range, canonical
cross-mode mismatch, fabricated published behavior, missing retained claim input, source-repository
mutation, silent network/Python engine dependency, or escaped descendant blocks promotion and rolls
back the affected profile.

### Deterministic acceptance matrix

| Slice | Frozen baseline | Required red evidence | Green promotion evidence | Rollback proof |
|---|---|---|---|---|
| P0 format/capability foundation | current resident index and Core-only analyzer baseline | corpus drift, touched-block corruption, crash boundary, budget overflow, identity drift, cross-request/capability/schema replay, ambiguous provider, stale lock, executable substitution, escaped descendant | exact corpus digests; canonical-root/core-receipt parity; controlled CPU/RSS/I/O/inode report; explicit format decision; accepted registry/lock/protocol/containment fixtures | benchmark namespaces deleted and plugin profiles disabled without changing current cache/selection or Core-only behavior |
| P1 local | prior production local profile | truncation, unknown section, post-hash replacement, plugin/cache-identity substitution, partial publish, catalog-epoch conflict, lease-before-open failure, every GC state/fence race, orphan lease/PID reuse/session loss, crash residue, worktree/overlay confusion | clean/incremental/no-cache/corrupt-cache parity after every capability-identity change; bounded RSS/I/O; one-shot full-polyglot Beamfall shadow over every qualified exact plugin tuple; exact review | atomic last-good profile/root selection; fenced idempotent reclamation; disable affected locked plugin/profile; no in-place conversion |
| P2 warming | P1 one-shot | missed/reordered/overflowed hint, hook failure, escaped child, stale warmer, battery/idle write | exact P1 parity; hook <=5 ms p95/<=50 ms max; idle/battery/cleanup receipts | remove hooks/warmer and retain P1 behavior |
| P3 review | clean base/head rebuild | moving head, force push, missing/inaccessible object, synthetic-merge ambiguity | exact base/head roots, byte-identical head answer, retained Merkle/evidence delta | expire review lease only; base/head receipts remain reproducible |
| P4-P5 hosted | local canonical core | wrong tenant/root/range, revocation, replay, event loss/reorder, inbox/queue/ack crash at every boundary, stranded dedupe marker, duplicate consumer, A→B→A same-commit/different-generation barrier, config/ref epoch change, timing oracle | zero isolation disclosure; verified ranges; no lost acknowledged event or duplicate logical promotion; exact observation-generation barrier and consistency/observer receipts; local core parity | disable ingress/HTTP aliases; preserve authorized historical roots |
| P6 documentation | pinned manual/initial build | raw-instruction leakage, fake claim, stale/drop, no-input validator, source write, sensitive publish | every factual claim cited; lifecycle/gap preservation; initial/incremental equivalence; strict pinned build; reviewed PR | close proposal/PR; source and last published docs unchanged |
| P7 vector | structured-only retrieval | mixed profile/tree, nondeterminism, authority displacement, false absence | registered held-out gain and exact/ANN recall gates with zero new critical miss | omit vector section and return `CORE_EXACT` |

Every row retains exact executable, corpus, profile, seed, environment, and receipt identities.
Unsupported observations are `NOT_RUN`; a green neighboring row cannot inherit them.

## Traceability

| Requirements | Planned owner | First required evidence |
|---|---|---|
| `DNIP-IDX-*`, `DNIP-LOC-*`, `ACC-V0-*` | Corvint local Go kernel plus external analyzer capability boundary | P0 corpus/format and registry/lock/protocol/containment receipts; P1 parity, crash, RSS, quota, epoch/lease/GC, worktree, plugin-tuple, and no-daemon gates |
| `DNIP-MCP-*`, `DNIP-SYNC-*`, `DNIP-TEN-*` | future Corvint cloud profile | accepted HTTP/auth/event/storage wires; tenant/red-team and range-integrity receipts |
| `DNIP-REV-*` | Corvint review profile | exact base/head/delta corpus and moving-head/provider-loss gates |
| `DNIP-VEC-*` | experimental semantic adapter | held-out outcome and deterministic local/cloud parity receipts |
| `DNIP-DOC-*` | Corvint doc compiler plus repository adapter | quarantine, claim durability, incremental equivalence, red/green validators, PR/publish receipts |
| `DNIP-XM-*`, `DNIP-COMP-*` | shared conformance/release gate | cross-mode corpus, controlled benchmark report, rollback and last-good selection |

All rows are `NOT_RUN` at this freeze.

## Unresolved implementation selections

- custom packed format versus immutable SQLite after P0 evidence;
- exact durable claim/provenance store (Git-tracked records versus backed-up exportable CAS);
- cloud object store, durable queue, and identity provider adapters;
- first qualified renderer tuple and its containment runtime;
- first qualified Go-compatible semantic encoder, if structured retrieval reaches the P7 gate.

These are not unresolved product outcomes. Each is selected by the named evidence gate and recorded
in a descendant decision/spec before implementation or promotion.
