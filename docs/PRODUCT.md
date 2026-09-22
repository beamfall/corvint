# Product contract

## Job

Corvint is the **change typechecker for agentic software**. For an engineer or coding agent entering
an unfamiliar repository, it compiles the smallest evidence set sufficient for one intended change,
then reports the exact obligations still unwitnessed before the change may be called done.

Corvint is the project's default context plane: the first read-only query surface when a human,
agent, IDE, CI job, or incident tool needs information about the code or product. It does not
replace authoritative systems. It resolves across them and must do one of three things: answer with
revision-pinned evidence, route the caller to the authoritative missing evidence, or abstain with a
precise unknown frontier.

The product and executable are Corvint. The `corvint-*` wire/profile identities, `corvint.*` MCP
tools, and canonical `.corvint` and `.context-corvint` paths are protocol and repository-state
contracts. `cmd/corvint-*` paths are source package names.

The accepted [0.6 verified local-workflow scope](decisions/0330-verified-local-workflow-scope-2026-09-22.md) and [portfolio reading map](PORTFOLIO-0.6.md) define the current Core qualification target. Its status is **NOT_QUALIFIED**; installed Codex/Claude use, six evidence classes, and candidate-bound gates remain open.

## Product loop

```text
corvint init | corvint adopt
  -> mechanically compiled repository evidence state
  -> minimum-witness context supplied before editing
  -> evidence-associated change
  -> bounded frontier at agent stop
  -> merge-time independent re-verification
  -> preserved/added/stale/vanished knowledge delta
  -> verified witnesses reused by the next agent
```

This **bidirectional witness loop** is the product breakthrough: context is no longer disposable
input and a diff is no longer opaque output. Every merged change leaves behind the verified context
needed to understand the next change. Git stores the durable ledger; content-addressed derived
artifacts remain disposable.

A receipt is useful only when every included item explains why it is present, names its
authority and confidence, binds to immutable source evidence, states what was excluded, and tells
the agent how to verify the result.

## Thirty-day wedge: Change Frontier

**The Change Frontier is the first user-visible product; CEM and OCM are its portable proof
primitives.** For each textual diff
hunk, a small vendor-neutral sidecar records immutable Git blob spans selected by its producer, or
an explicit unknown/mechanical disposition. A deterministic verifier resolves those identities at review
and merge, accepts only exact-unique evidence relocation, reports ambiguous, stale, or deleted evidence, and preserves unknowns. Corvint is the reference CEM
producer and verifier; an agent may use any retriever—or no Corvint retriever at all—to create one.

For one repository, one exact base/target change, and one pinned intent scope, `corvint frontier`
reports three bounded sets: material hunks without a qualifying basis, intent IDs without a
qualifying change-and-test witness, and required tests without pinned observation. Empty means only
that this declared obligation universe is closed. It never means Corvint proved the entire program.

The adoption target is a clean local setup and first useful frontier within 15 minutes, followed by
median per-change authoring overhead no greater than three minutes. The staging implementation now
includes `docs/CHANGE-EVIDENCE-MAP.md`, the normative schema, positive/negative conformance vectors,
a strict verifier, four producer/verifier commands, and focused tests. That is reference-
implementation evidence only; CEM is not interoperable until unrelated producers and consumers pass
the same vectors. Documentation is not delivery evidence.

Change Frontier ships candidate-only under
[`specs/change-frontier-v0.md`](specs/change-frontier-v0.md). Decision 0012 withdraws the former
sealed-cohort precision and recall gates. At the first release cut, preregister a third-party-PR
replay protocol before selecting or replaying evidence; its frozen sample, measures, and thresholds
are separate work and remain `NOT_RUN` until then. The independently stated evidence-gaming blocker
and prospective stronger-harness trial remain, and the wire is not interoperable until an
independent consumer reproduces the reference bytes.

Optional work-tracking adapters connect Jira or Linear issues to GitHub, GitLab, or Bitbucket pull
requests, commits, reviews, CI outcomes, code, and tests. Mutable ticket and review text is
provenance, not automatic authority: receipts pin external IDs, revisions, URLs, timestamps, Git
SHAs, and content hashes so later edits cannot silently rewrite the evidence used for a change.

E2E adapters connect acceptance criteria and user journeys to framework-neutral scenarios, steps,
fixtures, assertions, application surfaces, API operations, execution results, and retained artifact
references. Corvint combines declared test structure with revision-bound observations; it does not
equate one passing trace with complete behavioral coverage.

## Two activation doors

- `corvint init` is step zero for a new agentic repository. It mechanically inventories the current
  Git state, instructions, manifests, accepted specs, tests, CI, and ownership, then starts the
  evidence ledger from commit one.
- `corvint adopt` is the first step for a legacy product. It compiles the same complete path
  denominator, then incrementally harvests recoverable Git history, documents, RFCs, PRDs, TDDs,
  decisions, tests/E2E, CI, ticket/PR identities, ownership, runbooks, incidents, and configured
  external sources. Unrecoverable rationale remains `UNKNOWN`.

Both doors must return a useful cited receipt in under ten minutes without a build, database,
server, manual ontology, external credential, or mandatory model. Missing access is a named gap.
No setup path may manufacture historical intent.

## Accepted deployment direction

Corvint has one transport-neutral native-Go indexing and query kernel and one portable immutable index
format. Local files, cloud object storage with bounded range reads, pinned review deltas, and living
documentation are deployment profiles over that data plane, not separate engines. The governing
contract is [`specs/deployment-neutral-index-platform-v0.md`](specs/deployment-neutral-index-platform-v0.md).

This is accepted product direction, not delivery evidence. The local profile remains one binary
with no account, network, mutable/external database service, hosted service, Python runtime, or
permanent daemon. An optional, separately-built, loopback-only, read-through local console may be
started explicitly by the operator (decision 0081); it is not part of the default local product,
installs no service, holds no database, opens no outbound connection, and carries no authority. A benchmark-selected immutable embedded encoding is permitted derived state, not
a service. Hosted MCP,
provider synchronization, review pinning, documentation maintenance, and advisory vector segments
remain separate, versioned profiles and are `not-started` until their numbered requirements and
promotion gates pass. Python is prohibited in the indexing/query engine; a pinned isolated renderer
may use Python only under the renderer contract.

## Differentiation

Specification tools organize intent. Repository maps and code graphs organize code. Corvint connects
intent, decisions, code, tests, history, and outcomes, then verifies that the delivered change still
satisfies that evidence. It integrates with existing specification and agent tools rather than
replacing their authoring or editing workflows.

The underlying next-generation engine is a **Minimum Witness Compiler**: a proof-obligation compiler
that turns a change or proposition into typed obligations, finds the smallest bounded evidence set
that satisfies mechanically checkable obligations, and returns explicit gaps for everything it
cannot prove. It supports CEM first and a revision-aware executable claim ledger later. The claim
ledger is currently a local extraction/verifier **prototype**, not an integrated product capability.
Corvint never claims to simulate the application or converts structural reachability, coverage, or
one passing trace into a behavioral fact.

The adoption thesis is the **receipt, not a proprietary retrieval engine**. Corvint succeeds at
ecosystem scale when editors, agent harnesses, CI systems, specification tools, and competing
context engines can all emit or consume the same conformance-tested receipt. Corvint's compiler is
the reference producer and the first useful product; the revision-pinned evidence contract is the
durable boundary. A better retriever should be able to replace Corvint's ranker without weakening the
proof, provenance, abstention, or reproducibility guarantees.

Corvint does not compete on the presence of related context. Search, repository maps, graphs,
language servers, embeddings, memories, hooks, and MCP transport are replaceable producers and
delivery surfaces. Corvint owns the portable context-in/change-out contract: what was mechanically
known before an edit, which evidence was available, what the change cites, which declared
obligations remain open, what was independently observed, and which prior knowledge became stale or
vanished. Empty frontier means closure only over the exact declared universe, never whole-program
correctness.

Corvint does not claim to have invented evidence-carrying changes or proof-carrying coding. July 2026
proposals using those category ideas, plus agentdiff's hunk/line attribution work, validate the timing
rather than Corvint's uniqueness. CEM's narrower bet is a source-content-free, portable interchange
from each textual hunk to immutable producer-selected evidence, with deterministic patch/span/drift verification
and no dependency on one retriever, agent, IDE, or CI vendor. SLSA addresses build provenance, MCP
transports context, and agentdiff attributes edits; the current market inference—not an exhaustive
claim—is that none alone provides this hunk-to-evidence association interchange. Here,
source-content-free means that source and diff bodies are omitted; paths and digests are still
sensitive and the format is neither anonymous nor automatically safe to publish.

## Product pillars

1. **Change frontier and CEM evidence-carrying changes.** Every textual hunk in an agent-produced diff cites immutable
   evidence the producer declares as its basis, carries an explicit unknown, or uses a byte-proven
   mechanical exception. CI re-verifies the map at merge. `corvint frontier` turns unresolved CEM
   hunks, intent obligations, and test observations into the bounded stop condition. This is the
   launch wedge, not a later ecosystem feature.
2. **Minimum witnesses and executable test claims.** Typed, deterministic proof obligations bound
   what Corvint may assert. Tests and accepted specifications may form a claim ledger that answers a
   capability question with a proving witness or an explicit unknown frontier. This replaces the
   aspirational "behavioral twin as a model" framing; the ledger remains a prototype.
3. **Evidence lineage and conservation.** Every merge classifies prior evidence records as
   preserved, reverified, superseded, retired, stale, or vanished; nothing silently disappears.
   Incremental application must reproduce a clean rebuild byte-for-byte. This makes `corvint why`,
   evidence debt, selective tests, generated-document maintenance, and future minimum context
   compound from honest history rather than generated memory. The conservation wire remains an
   experimental novelty hypothesis until clean/incremental equality and planted-loss detection pass
   on an untouched transition corpus.

Each pillar has a kill criterion. Corvint does not keep a pillar because it sounds futuristic:

- the Change Frontier remains candidate-only; its replacement preregistered third-party-PR replay
  protocol is due at the first release cut and is `NOT_RUN`, while any later stronger-harness trial
  retains its separately stated reviewer-miss, critical-miss, and latency criteria;
- generic test-claim extraction must reach 0.70 precision and its unknown frontier must outperform
  coverage percentage at predicting missed behavior;
- evidence transitions must detect every planted silent loss, produce byte-identical incremental
  and clean roots, and never falsely close a critical transition.

## Breakthrough doctrine

Corvint is not successful merely because it indexes more sources. Each major capability must deliver
an agent behavior that ordinary text search, retrieval-augmented generation, code graphs, or
standalone specification tools cannot provide, and must improve a public or replayable real-world
task before it expands Core.

The enabling technical advances are:

- a **bidirectional witness loop** that carries verified context through the edit, stop decision,
  merge verification, and next session;
- an **evidence-conserving knowledge transition** that accounts for every prior record and makes
  silent knowledge disappearance mechanically visible;
- a **Minimum Witness Compiler** that turns a proposition or change into typed proof obligations and
  the smallest bounded, mechanically verifiable witness set;
- **anchored extraction** that admits an offline model-proposed semantic edge only when immutable
  source spans exist and a deterministic verifier confirms lexical support for the claim;
- **Merkle-diff receipts** that reuse immutable per-blob evidence and compute what changed between
  two receipts from changed blobs rather than rebuilding an entire repository;
- **ranker-disagreement abstention** that compares independent lexical, dependency, co-change, and
  test-claim retrievers and explains their disagreement instead of relying on vocabulary shims;
- **epistemic receipts** that expose authority, freshness, derivation, uncertainty, contradictions,
  exclusions, and verification rather than presenting retrieval as fact;
- **closed-loop calibration** that learns ranking only from explicit successful outcomes and
  held-out evaluation, while preserving deterministic authoritative evidence;
- a **vendor-neutral context plane** that gives humans, IDEs, CI, incidents, and different agent
  harnesses the same inspectable model.

Models are a last-mile semantic resolver, never the repository index. Corvint exhausts applicable
Git, syntax, manifest, declared-link, lexical, history, co-change, test, CI, and ownership resolvers
first. Only an exact unresolved frontier item may be routed to the smallest calibrated eligible
model. Escalation requires a recorded anchoring failure, material retriever disagreement, declared
high-risk ambiguity, or runtime failure. Model output remains an anchored proposal; without a
relation-specific mechanical verifier Corvint does not call a model and returns `UNKNOWN`.

Futuristic ideas stay as adapters or experiments until they beat a baseline on named tasks. Corvint
does not earn a breakthrough by adopting embeddings, a graph database, autonomous agents, or a UI;
those are implementation options, not product outcomes.

Retrieval state and proposition verdict are separate dimensions. `READY`, `NEEDS_WIDENING`, and
`OUT_OF_SCOPE` describe whether Corvint assembled an adequate packet. `PROVED`, `REFUTED`,
`CONFLICTED`, and `UNKNOWN` describe a bounded proposition. A high-ranked or `READY` packet never
means a proposition is true.

## Flagship workflows

- **CEM evidence-carrying editing:** compile the evidence for a change and bind every textual diff
  hunk to its declared immutable basis without pretending the map proves model-internal causality.
- **Capability questions:** answer with an executable claim proved at revision R or return an
  explicit unknown frontier with the nearest claims.
- **Evidence drift:** show which relied-upon facts changed, became dangling, or need re-verification
  between two revisions.
- **Change consequences and test selection:** traverse only proven structural and claim edges,
  explain every selected test, and preserve mandatory project gates.
- **Code-in-place understanding:** answer feature, purpose, state, contract, ownership, incident,
  and impact questions for the symbol or range under an IDE cursor.
- **Agent context:** give every supported coding harness the same bounded, revision-pinned change
  capsule and receipt instead of maintaining vendor-specific knowledge models.

## Committed integration and workflow goals

These are product commitments, not claims about the current alpha. They all consume the same
revision-pinned evidence, minimum-witness, frontier, and update contracts rather than becoming
separate knowledge stores.

| Goal | Corvint product contract |
|---|---|
| AI coding, debugging, security analysis, and technical research | Corvint is the first context call: return the smallest cited packet, exact expansion handles, and what remains unknown. |
| Codex integration | Maintained Codex adapters use a plugin on plugin-capable surfaces and standalone skills, hooks, and MCP on the IDE surface. Support is claimed and tested per surface, never for “Codex” as one undifferentiated host. |
| Claude Code integration | A maintained Claude Code plugin combines namespaced skills, hooks, and MCP tools so Corvint context, session evidence, change receipts, and stop/frontier checks participate in the native agent loop. |
| Gemini CLI integration | A maintained Gemini CLI extension combines context, commands, hooks, skills, and MCP tools so Corvint performs automatic discovery, minimum-witness injection, session/change capture, and frontier checks. |
| OpenCode integration | A maintained OpenCode plugin uses stable session, tool, and file events plus Corvint MCP tools. It remains `FALLBACK` unless a pinned host version exposes and passes safe frontier/continuation conformance; beta APIs stay behind a versioned adapter. |
| Pi extension | A maintained Pi-facing extension automatically supplies task/change context, records session evidence and outcomes, and discovers stale or missing knowledge through the shared receipt API. |
| DeepSeek Harness plugin | A maintained plugin for the actual DeepSeek Harness (`deepseek.com/harness`) supplies Corvint context through its Cordis plugin services/events, records Corvint injections and verified session deltas in the harness trajectory, and participates in automatic discovery without creating a separate knowledge model. |
| Human product and technical documentation | Render cited, paragraph-anchored Markdown suitable for MkDocs or another repository-owned documentation site; deliver locally or through a reviewable pull request, never silently overwrite accepted prose. |
| Automatic E2E intelligence | Discover existing journeys and states, link E2E tests to executable claims, propose missing tests, run the justified subset while preserving mandatory gates, and ingest pinned results without treating one trace as exhaustive behavior. |
| Pull-request and merge maintenance | PR updates compute review context and evidence drift; an exact merged commit automatically marks knowledge preserved, stale, contradicted, retired, or needing reverification and regenerates only affected derived views. This does not authorize Corvint to merge code or promote inferred intent. |

Support is reported per `(host, surface, host version, adapter version, OS)` as `FULL`, `FALLBACK`,
or `UNSUPPORTED`; a product name alone is never marked `FULL`. Full support has one testable meaning
for Codex, Claude Code, Gemini CLI, OpenCode, Pi, and DeepSeek Harness. Corvint ships an installable,
versioned native package; detects the repository and
revision; exposes query and expansion operations; injects bounded task/change context; records only
the evidence actually observed plus explicit session outcomes; runs change-drift and frontier checks
at the platform's safe lifecycle points; obeys its permission and secret model; degrades explicitly
when a capability is unavailable; and passes a published compatibility/conformance matrix against
every supported platform release. MCP and the Corvint CLI remain portable fallbacks, but an MCP
connection alone is not `FULL`. A missing lifecycle capability, failed conformance case, or untested
host release is `FALLBACK`, not `FULL`. Adapters translate lifecycle events into the shared Corvint
contract and must not fork the graph, receipt, CEM, or epistemic semantics.

Corvint is scoped to these end-user workflows; each is production-ready only once it beats its
baseline under the acceptance rule below, and sequencing and demand-gating for undelivered ones are
tracked in `ROADMAP.md`:

1. see what a change may break, with proven effects separated from bounded candidates and unknowns;
2. find and propose missing tests from uncovered claims, paths, states, and regressions;
3. run only evidence-linked tests in addition to, never instead of, mandatory project gates;
4. orient a new engineer with role-appropriate minimum-witness onboarding capsules;
5. route a ticket to the likely owning team with evidence and an abstention path;
6. review a change against intent, evidence, effects, tests, ownership, and unresolved frontier;
7. backfill an `inferred-draft` specification from what code and tests demonstrably do;
8. plan removals and migrations using consumers, contracts, data, flags, history, and deprecation evidence;
9. narrate what shipped from the exact merged diff, claims, tests, tickets, and known limitations; and
10. orient responders during an incident with owners, runbooks, recent changes, known failure modes,
    diagnostics, and explicit uncertainty.

Each workflow must beat its manual-search or existing-tool baseline on held-out tasks before Corvint
labels it production-ready. Transport integrations may arrive earlier; they do not make an
unproven workflow complete.

The common question vocabulary includes `find`, `explain`, `why`, `capabilities`, `flows`, `state`,
`impact`, `tests`, `incident`, `owner`, `change`, `status`, and `provenance`. These are views over one
evidence model, not independent products or duplicated indexes.

## North-star measurement

`successful agent tasks with zero missed critical evidence`

For CEM evidence-carrying changes Corvint also tracks reviewer-found missed evidence, uncited material
hunks, legitimately uncitable hunks, receipt drift at merge, and task success at equal context budgets.

Public evaluation reports must also include time to first correct file, top-five success, critical
recall, serialized-result-byte-weighted precision, packet bytes, abstention accuracy, latency, and task completion
rate. A benchmark without a competing baseline and held-out cases is not launch evidence.

Activation and semantic-enrichment reports additionally publish complete path classification,
mechanical yield, model-call and strongest-model escalation rates, model input/output tokens,
verifier acceptance, independently labelled accepted-edge precision, abstention, first-use latency,
and cost. A model-everything baseline is required before claiming token or cost savings.

## Current evidence, not aspiration

Corvint is an extraction alpha, not a finished context oracle and not yet evidence of a 100,000-star
product. On 2026-08-22 the 31-case, five-repository **development** corpus passed its configured V4
thresholds: zero critical misses, 1.0 recall, 1.0 top-five success, 1.0 abstention and epistemic-state
accuracy, 1.0 byte-budget compliance, and 0.804952 serialized-byte-weighted precision
(`benchmarks/results/v4-development.json`, `corvint_commit` `3fe7ac60459076005964bd8a0f729b028550879f`). The original blind-v1 cases
were then observed and used to repair the engine, so they are development cases thereafter, not
independent launch proof.

The untouched blind-v2 first run is the more important warning: five deliberately difficult cases
produced 7/10 critical misses, 0.071429 recall, 0.4 top-five success, 0.0 abstention and
epistemic-state accuracy, and 0.182141 precision
(`benchmarks/results/blind-v2-first-run.json`). Two cases target V4 foundations and three target
V6 claim-ledger behavior, so this is a challenge diagnosis rather than one V4 release score. It
nevertheless proves that current structural retrieval cannot yet recover complete feature flows,
test claims, or negative capabilities reliably. Repairs must improve general methods and then pass
new untouched cases; the observed cases can never be relabelled held out.

The subsequent untouched V4 blind-v3 first run confirmed the gap on the advertised V4 contract:
8/10 critical misses, 0.181818 recall, 0.258638 precision, 0.6 top-five success, and 0.0 abstention
and epistemic-state accuracy (`benchmarks/results/blind-v3-first-run.json`). This fails the V4
generalization gate. It inverts the sequence: Corvint should not spend the next month pretending a
broader ranker will become a product oracle. Ship and test CEM interoperability and typed proof
obligations first; improve retrieval behind that stable, honest boundary.

## V4 release gates

1. During the staged migration overlap, a clean install takes less than 60 seconds on a supported
   Python environment. The promoted native local profile must meet the same bound without Python.
2. Corvint runs on Beamfall plus four unrelated public repositories without project code changes.
3. The public corpus contains at least 25 positive, negative, impact, and abstention cases.
4. Critical evidence misses are zero; abstention and byte-budget compliance are 1.0.
5. Serialized-result-byte-weighted precision is at least 0.80 and top-five task success is at least 0.90.
6. Every receipt binds its Git revision, evidence spans and hashes, exclusions, and verification.
7. An installation and first useful query fit in a 60-second recorded demonstration.
8. A content-addressed warm query completes in under one second and is byte-identical to a cold
   query at the same revision.
9. A fresh untouched V4-targeted partition passes after every observed case is frozen as
   development; first-run failures remain published and are never relabelled held out.

## Default local-profile non-goals

- Hosted indexing, accounts, telemetry, or repository upload
- A graph database or vector database
- Semantic embeddings
- Cross-repository federation
- Autonomous code editing
- A graphical interface
- A new specification language
- Automatic collection of developer activity

The accepted deployment-neutral direction does not add these to the default local profile. Hosted,
remote-range, documentation, review, and advisory-vector work may proceed only as separately
versioned descendants of `DNIP-V0`, with their own evidence and promotion gates. The accepted Go
production-kernel migration is the sole current language-migration exception.

The 30-day CEM wedge specifically does **not** require an IDE, dashboard, hosted service, graph or
vector database, new agent, universal repository map, automatic code editor, new specification
language, Jira/E2E ingestion, or complete claim ledger. It verifies CEM structure and cited
evidence; it does not prove that an edit is correct, complete, causally necessary, or safe to merge.

## Default ecosystem scope

Corvint's default distribution targets Go, Python, JavaScript/TypeScript/TSX, Java/Kotlin, C#/.NET,
Rust, Ruby, PHP, HTML/templates, and the CSS family (CSS, CSS Modules, Sass/SCSS, and Less). Support
is phased by the roadmap and becomes official only after ecosystem-specific dependency, test,
impact, and abstention cases pass. Generic syntax extraction remains the fallback; it is never
advertised as full language support.
