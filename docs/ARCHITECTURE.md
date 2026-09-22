# Architecture contract

## Layers

1. **Sources** read immutable content from a pinned Git tree and report mixed-worktree state.
2. **Adapters** recognize language symbols, imports, tests, specifications, decisions, and
   project-owned canonical records.
3. **Evidence IR** stores typed nodes and reasoned relationships with source spans, hashes,
   authority, confidence, and freshness.
4. **Minimum Witness Compiler** turns a change or proposition into typed proof obligations, ranks
   eligible evidence, and emits the smallest bounded witness set it can verify under the budget.
5. **Receipt** reports included evidence, exclusions, uncertainty, revision, and verification.
6. **Evaluator** scores frozen and held-out tasks before any richer retrieval method is promoted.

The evidence graph is a logical model, not a storage requirement. V4 builds it deterministically in
memory from Git. Future persistent derived storage is governed by the deployment-neutral contract
below and requires controlled scale evidence plus compatible receipts.

## Deployment-neutral data plane

The accepted direction is one native-Go indexing/query kernel and one portable immutable index
format reused by local files, authenticated object-range reads, exact review snapshots, and living
documentation. Git remains semantic authority; roots, packed segments, vectors, claims, and rendered
documentation are disposable or explicitly retained derived state. The format must support bounded
verified `ReaderAt` access and byte-identical roots and canonical receipt cores across delivery
modes; versioned delivery envelopes differ only in their enumerated mode fields.

This direction is `not-started`. It does not alter the delivery state of the in-memory V4 index,
local stdio MCP, Go migration, Pulse, or Human Documentation Compiler. The normative requirements,
phasing, failure semantics, and rollback live in
[`specs/deployment-neutral-index-platform-v0.md`](specs/deployment-neutral-index-platform-v0.md);
the rationale and alternatives live in
[`decisions/0001-deployment-neutral-immutable-index.md`](decisions/0001-deployment-neutral-immutable-index.md).

## Delivery-state discipline

This document contains both the implemented V4 foundation and later contracts. They are deliberately
separated so architectural ambition is not confused with working product:

| Capability | Delivery state on 2026-08-22 |
|---|---|
| Deterministic Git-pinned extraction, bounded packets, receipts, CLI evaluation | V4 alpha, exercised by the development corpus |
| Content-addressed fragments and byte-identical sub-second warm queries | V4 release work; not accepted until the final performance gate passes |
| `cem/0.1` interchange plus experimental `cem/0.2` canonical binding | Implemented in staging; pending independent review, packaging, interoperability, and outcome gates |
| Merkle receipt diffs and merge-time re-derivation | V5 contract supporting CEM freshness |
| Minimum Witness Compiler obligations and negative-scope certificates | V5-V6 architecture under implementation; current ranker is not yet this compiler |
| Ranker-disagreement abstention | V5 experiment, not current V4 behavior |
| Executable claim ledger | Local extraction/verifier prototype only; not integrated capability answering |
| Anchored semantic extraction | V6 experiment, not current V4 behavior |
| Outcome-labelled context commons | V9 opt-in experiment |
| Independent receipt/CEM producers and consumers | Immediate V5 interoperability gate |
| Mature neutral receipt standard and governance | V10 adoption target after real independent use |

No future layer may be presented as delivered because its schema exists here. Promotion requires a
named task, a frozen competitor or simpler baseline, an untouched evaluation partition, and a kill
criterion. The receipt contract must remain usable when any experimental retriever is disabled.

## Minimum Witness Compiler contract

The compiler's input is a revision-bound change or bounded proposition. It emits typed obligations,
candidate witnesses, deterministic verifier results, a bounded negative-scope certificate where
appropriate, and an explicit unknown frontier. Initial obligation kinds are deliberately small:

1. **Identity and freshness:** the revision, diff hunk, blob, and span are the objects claimed.
2. **Authority:** a cited specification, decision, policy, or owner has the declared status and
   scope at that revision.
3. **Structural consequence:** a mechanically supported declaration, reference, import, call, or
   changed-path relationship connects the cited objects.
4. **Verification:** a named test, assertion, gate, or check exists and supports only its bounded
   proposition; existence never means it passed.
5. **Negative scope:** no qualifying witness was found inside an explicitly enumerated search
   universe, with exclusions and unsupported surfaces reported.

Each obligation has `SATISFIED`, `REFUTED`, `CONFLICTED`, or `UNKNOWN` verifier status and exact
evidence. Deterministic verifiers may validate identity, hashes, spans, syntax relationships,
declared status, and bounded absence. They do not infer intent or semantic causality. Model and
retrieval output may propose a witness but can never set the verifier status.

A negative-scope certificate pins the revision, roots and file classes searched, adapters and
versions used, query/pattern semantics, exclusions, budgets, and unsupported dynamic or external
surfaces. It proves only `no qualifying witness found in this declared universe`; it never proves a
global non-capability. Anything outside the certificate remains in the unknown frontier.

Retrieval state and proposition verdict are orthogonal fields. `READY`, `NEEDS_WIDENING`, and
`OUT_OF_SCOPE` describe packet adequacy. `PROVED`, `REFUTED`, `CONFLICTED`, and `UNKNOWN` describe
the bounded proposition after obligation verification. A `READY` packet may still yield `UNKNOWN`
or `CONFLICTED`; a high rank is never a truth verdict.

## Change Evidence Map interoperability contract

The Change Evidence Map (CEM) is the first narrow application of the compiler: a sidecar association
map for agent edits. A `cem/0.1` map binds a schema version, full base revision, and exact patch digest to
every textual hunk. Each hunk carries content-derived path/range/body identity and either immutable
Git blob-span evidence, an explicit unknown, or a mechanically verified whitespace/line-ending
exception. Verification results are outputs, not producer assertions. The portable interchange is
source-content-free in a narrow wire-format sense: it carries identities, hashes, spans, typed
relations, and enumerated reasons, not source or diff bodies. Paths and digests remain sensitive;
the format is not anonymous or automatically safe to publish.

The experimental `cem/0.2` profile retains those evidence and hunk rules, records the sole fixed
sidecar exclusion, and makes independently supplied base and target revisions—not producer patch
bytes—the committed-change authority. The same bounded Git implementation serves CEM and OCM;
linked worktrees are reciprocally validated, while alternates and implicit fetch remain denied.

The verifier checks canonical structure, revision/diff/hunk identity, evidence span/hash freshness,
basis-reference structure and relation enums, explicit unknowns, and narrow mechanical exceptions. It reports rather than
guesses when evidence is absent or stale. On a changed target blob it accepts only one byte-exact
occurrence of the original span; zero is stale and multiple matches are ambiguous. It does not prove
that evidence semantically supports the edit or influenced its generation, nor that code is correct,
complete, causally necessary, secure, or safe to merge.

Corvint is one reference producer and verifier, not the mandatory retriever. The initial protocol is
interoperable only after at least one independent non-Corvint producer and two independent consumers
pass the same conformance fixtures. The initial schema, reference producer/verifier commands,
fixtures, and focused tests are implemented in this staging line; this is not yet proof of
interoperability or usefulness. V10 may
standardize mature semantics and governance; it is not the first moment other tools may participate.

Evidence-carrying changes and proof-carrying coding are emerging category ideas, not Corvint
inventions. CEM's architectural boundary is narrower: portable material-hunk-to-immutable-evidence
interchange plus deterministic patch/span/drift verification, neutral across retrievers, agents,
IDEs, and CI systems. Build provenance, context transport, and edit attribution are adjacent inputs,
not substitutes for the CEM relation by themselves.

## Evidence graph contract

Corvint models evidence that answers a concrete engineering question. It is not a mirror of every
object exposed by every connected tool. The graph has three compatibility tiers:

1. **Core evidence** is portable across repositories: repository, revision, file, source span,
   symbol, dependency, test, specification, decision, change, verification, and artifact.
2. **Standard adapters** map common ecosystems into core evidence: packages and lockfiles; API,
   schema, event, and configuration contracts; CI jobs and deployments; issues, pull requests,
   reviews, ownership, vulnerabilities, and runtime services.
3. **Project extensions** preserve domain-specific records without adding domain vocabulary to the
   core. Extensions must declare their source, authority, identity, and compatibility version.

The useful node families and their principal relationships are:

| Family | Examples | Relationships Corvint must answer |
|---|---|---|
| Source | repository, revision, branch, file, span, generated file | contains, renames, generates, supersedes |
| Code | symbol, type, module, package, executable | declares, references, imports, calls, implements, extends |
| Contract | API operation, schema field, database migration, event, config key, feature flag | exposes, consumes, validates, evolves, gates |
| Verification | unit/integration/E2E test, journey, step, fixture, assertion, static-analysis finding, benchmark, CI job | covers, exercises, asserts, fails-on, produces |
| Intent | specification, requirement, acceptance criterion, decision, issue | requires, constrains, decomposes, supersedes |
| Change | commit, branch, pull request, review, release | addresses, modifies, approves, rejects, introduces, fixes |
| Responsibility | person, team, CODEOWNERS rule, policy | owns, reviews, permits, blocks |
| Delivery | build, artifact, package, container, environment, deployment | builds-from, contains, deploys-to, promoted-from |
| Runtime | service, endpoint, queue/topic, database, trace/log/metric profile | serves, invokes, publishes, subscribes, observes |
| Supply chain | dependency, license, SBOM component, vulnerability, attestation | depends-on, affected-by, licensed-as, attests |
| Product surface | route, screen, component, template, style, asset, accessibility contract | renders, styles, navigates-to, localizes |

Every edge is an evidence claim, not a naked pair of identifiers. It carries a stable origin,
source span or external receipt, observed revision, validity interval where known, adapter and
schema version, authority, confidence, and derivation rule. Corvint distinguishes declared,
observed, inferred, and learned edges. A query may traverse weaker edges, but its receipt exposes
the distinction and authoritative evidence always wins.

Derived closures such as transitive impact, ownership rollups, and runtime-to-source correlation
are computed views rather than permanent node explosions. A new node or edge type enters Core only
when it improves a named benchmark task across multiple ecosystems; otherwise it remains an
adapter extension. This promotion rule is the graph's primary defense against overengineering.

### End-to-end evidence

E2E adapters compile a framework-neutral hierarchy of suite, scenario, step, fixture, assertion,
and artifact. Static evidence connects scenarios to routes, screens, accessibility selectors, API
operations, feature flags, services, test data, and source symbols when the relationship is
explicit. Observed evidence connects an immutable test execution to its Git revision, environment,
CI job, status, duration, retry/flaky state, trace, coverage, screenshot, video, and logs.

Artifacts remain references with hashes and retention metadata; Corvint does not copy large binaries
into the graph. Secrets and user data are redacted at ingestion. Runtime correlation may strengthen
an impact path, but a single observed trace never proves that unobserved code is unrelated. This
supports queries such as `which user journeys can this diff affect?`, `which E2E scenario verifies
this acceptance criterion?`, and `what changed since this scenario last passed?` without pretending
that execution coverage is complete.

### Executable claim ledger and known frontier

Corvint does not build a speculative behavioral twin or promise exhaustive program-state discovery.
The current claim extractor/verifier is a prototype for executable, revision-pinned claims, not an
integrated query guarantee. A claim states a bounded proposition, its scope and
preconditions, the test/assertion/specification that supports it, the code it exercises when known,
the revision where it last passed, and any counterevidence. Claims may describe purpose, entry
points, permissions, state transitions, invariants, limits, error/recovery behavior, and explicit
non-capabilities, but Corvint asserts only what the cited evidence demonstrates.

Corvint distinguishes four evidence classes:

1. **Stated behavior** from accepted specifications, decisions, tickets, and documentation.
2. **Implemented behavior** from routes, handlers, state machines, call/data flow, configuration,
   permissions, schemas, and integrations.
3. **Verified behavior** from tests, formal checks, static analysis, and acceptance criteria.
4. **Observed behavior** from revision-bound E2E runs and opt-in runtime telemetry.

The compiler reports contradictions between these classes. The unknown frontier contains reachable
code, declared behavior, acceptance criteria, and changed surfaces for which no applicable proving
claim exists. Dynamic dispatch, concurrency, arbitrary input, unavailable external systems, and
unobserved paths remain explicit `UNKNOWN` frontiers. `UNKNOWN` is useful and must never be converted
into `cannot happen`; code coverage alone is not a proving claim.

### Anchored semantic extraction

An optional offline extractor may propose semantic edges such as `test proves claim`, `decision
governs symbol`, or `handler implements capability`. The proposal enters the evidence graph only
when all of these conditions hold:

1. every endpoint is bound to a tracked Git blob hash and exact source span;
2. a deterministic edge-specific verifier confirms that the spans still exist and lexically support
   the proposed relationship;
3. the extractor identity, version, prompt/schema digest, verifier version, and rejection reason are
   recorded;
4. ambiguity, missing endpoints, unsupported syntax, or insufficient lexical support causes
   omission, never a guessed edge.

Under a separately accepted snapshot-independent extractor profile, accepted facts are stored as
content-addressed index fragments keyed by their input blob hashes and complete extractor/profile
identity. Only that proven profile may avoid reinvoking the extractor for unchanged blobs and replay
accepted fragments byte-identically. Current `ACC-V0-018` external analyzers bind the full snapshot
and request and therefore recompute under every changed tree; they do not inherit this optimization.
Model output remains a proposal, not authority; the receipt exposes that the relationship was
model-proposed and mechanically anchored. Promotion requires accepted-edge precision of at least
0.85 under independent labels and a critical-recall-at-five gain of at least 0.10.

### Deterministic retrieval ensemble and abstention

In the planned V5 ensemble, lexical match, dependency proximity, Git co-change, and test-claim
retrieval remain independent
rankers. The compiler records each ranker's candidates and reasons, then derives agreement features
such as top-k overlap and rank correlation. High agreement may produce `READY`; material disagreement
produces `NEEDS_WIDENING` with the disputed evidence; no supported candidates produces
`OUT_OF_SCOPE`. Agreement is an explainable uncertainty signal, not a calibrated probability until
outcome evaluation proves calibration. It replaces project-vocabulary abstention rules only after
it beats the registered score-gap baseline; until then those rules are acknowledged V4 compatibility
behavior, not the intended architecture.

### Merkle index and receipt diffs

Each source blob produces an immutable Core evidence fragment. A repository tree manifest references
those fragments, so a new revision rebuilds only changed Core blobs. External-analyzer fragments are
reusable only when their complete `ACC-V0-018` identity—including full snapshot and request—matches;
until an accepted ACC amendment proves a narrower snapshot-independent projection, a tree change
recomputes all selected external capability work rather than relabeling old facts. Receipt identity
includes the tree, normalized request digest, receipt schema and canonicalization versions, closed
capability/profile identity, compiler version, adapter versions, and fragment digests.
`receipt(R2) - receipt(R1)` reports
added, removed, moved, dangling, contradicted, and stale-verification evidence. Core merge-time
re-derivation scales with changed blobs; current external work scales with the complete selected
snapshot and is reported separately. Both remain byte-identical to a clean rebuild. A cache is
disposable derived state, not an authority or database.

Index construction resolves the repository's SHA-1 or SHA-256 storage format together with the
immutable tree identity. That single Git result governs clean-worktree blob verification and cache
OID validation; unsupported formats and mismatched OID widths fail closed. Root-cache identity binds
repository authority, tree OID, engine/profile digest, and the ordered aggregate of every fragment's
complete prevalidated `ACC-V0-018` capability-cache identity. Each external fragment binds its exact
registry/lock/plugin/release/protocol/artifact/projection/request/admission/verifier identity; Core
fragments bind an explicit Core-only identity. Query-result caches additionally bind the complete
canonical receipt key. There is no independent or configurable content-hash layer.

### CEM evidence-carrying changes

A CEM introduces textual diff hunks as first-class evidence subjects. Each hunk cites one or more
immutable Git blob spans, declares one of three explicit unknown codes, or uses one of two
byte-verifiable mechanical exceptions. The binding is separate from source comments and travels in
a local JSON sidecar; an external commit trailer, CI artifact, or signature may bind that sidecar.
At merge, Corvint resolves the exact path and blob at the target revision and reports stable,
same-path exact-unique relocation, ambiguous, stale, or deleted evidence. It never normalizes
whitespace, searches other paths, or fuzzy-reanchors a citation.

The vendor-neutral `cem/0.1` fields are only the spec identifier, full base commit, exact patch
digest, evidence records, and hunk records. Producer identity, test attestations, signatures,
obligation extensions, and semantic scoring remain outside 0.1. The staging reference workflow is
implemented but remains subject to independent interoperability, the 30/30 experiment, and the
15-minute adoption gate in the product contract.

### Outcome commons boundary

Outcome traces are local and opt-in. A shareable record contains outcome labels, receipt schema and
compiler versions, ranker feature values, selector kinds, and privacy-screened identifiers—never
source text, prompts, credentials, ticket content, or raw private paths. Raw blob hashes can permit
re-identification and are publishable only for public repositories or explicit consent; private
repositories use keyed pseudonymous identifiers. Shared outcomes are advisory training/evaluation
data and can never override repository-owned evidence.

## Client and harness boundary

The evidence compiler exposes one versioned request/receipt contract. The CLI and JSON-over-stdio
are the reference transports; MCP, LSP, editor plugins, and agent extensions are thin clients. LSP
maps the cursor's repository, revision, file, and source range to Corvint requests and returns compact
context, impact, test, ownership, and feature links without placing the graph in the language
server. Harness adapters translate native sessions and tool calls into the same contract.

Pi support uses its extension/SDK or headless RPC integration points; DeepSeek is treated as a model
provider behind a compatible harness, not as a separate Corvint intelligence stack. Codex, Claude,
Pi, ACP/MCP clients, and other harnesses therefore receive equivalent evidence and can be evaluated
against the same tasks. Integrations must not receive credentials, unredacted external evidence, or
repository content beyond the compiled receipt unless the user explicitly grants it.

## Authority order

Project-owned canonical records and accepted decisions outrank test markers, syntax/import
relationships, Git history, and explicit local task traces. Learned evidence is advisory and may
never override authoritative evidence or turn an otherwise insufficient packet into certainty.

## Mutation boundary

Query, impact, feature, and evaluation commands never modify repository content. They may write a
disposable, bounded, content-addressed cache under local Git metadata; cache entries are derived
only from an immutable Git tree and may be reused as revision-only context on a dirty worktree while
the receipt reports its mixed state. They never contain mutable worktree content, are never treated
as authority, and are safe to delete. Freshness covers Git-visible changes only; ignored files are
unobserved and never scanned. Recording a task outcome is explicit, local, bounded, secret-screened,
and tied to a clean Git revision. No command executes the recorded verification label.

## Project adapters

Project-specific paths, record formats, decision status rules, and verification commands belong in
adapters or project configuration. Beamfall conventions currently present in the extracted engine
are compatibility behavior to remove behind a Beamfall adapter before V4 exits alpha.

## External provenance adapters

Issue trackers and code hosts are optional sources. They compile Jira/Linear issues and
GitHub/GitLab/Bitbucket pull requests, commits, reviews, and CI results into the same Evidence IR,
but never outrank project-owned accepted specifications merely because they are remote. Credentials
remain in host secure stores; cached metadata is bounded and content-hashed; every receipt identifies
the external revision and Git SHA it used. Corvint Core remains useful without a network connection or
hosted account.
