# Corvint technical-brain contract

Status: proposed product contract.

Corvint should become the first place an agent or engineer asks about a software product. Its job is
not to contain every byte or produce the most prose. Its job is to return the smallest trustworthy
context that lets the caller act, show exactly what supports it, expose what is missing, and maintain
that knowledge as the repository changes.

## Product promise

For a pinned repository revision, Corvint answers:

| Question | Minimum useful answer |
|---|---|
| Where is this feature? | Entry points, implementation symbols, tests, specs, aliases, and bounded exclusions. |
| How does it work? | A cited end-to-end flow separated into structure, observed behavior, inference, conflicts, and unknowns. |
| Why is it here? | Accepted decisions/specs and dated history; otherwise an explicit unknown rather than an invented purpose. |
| Can the product do this? | A proving executable claim at revision R, a contradiction, or the nearest claims plus an unknown frontier. |
| What happens if this changes? | Direct consequences, bounded indirect candidates, affected contracts/data/config/UI, and unexamined dynamic surfaces. |
| What tests should run? | Mandatory project gates first, then evidence-linked candidate tests with reasons and omissions. |
| What should I do during this incident? | Relevant runbooks, owners, recent changes, known failure modes, checks, and uncertainty; never autonomous operational authority by default. |
| What is undocumented? | Queryable orphan units and missing evidence links, not silence. |
| Does intent match reality? | Per-statement corroborated, contradicted, conflicted, or unknown results. |
| What changed in the knowledge? | A Merkle diff of added, removed, relocated, dangling, contradicted, and stale evidence. |

Every answer includes the exact revision, evidence selectors, authority, epistemic class, freshness,
exclusions, unsupported surfaces, and a bounded next action. An answer may be helpful while still
being `UNKNOWN`; usefulness does not require false certainty.

## The token-saving test

Corvint saves tokens only when an agent can safely avoid reopening most of the source it summarizes.
Short prose alone is not success. The response must be trustworthy enough to substitute for raw
context until the caller deliberately expands one cited locus.

Responses are therefore progressive:

1. a compact answer and state;
2. a minimum witness set;
3. stable handles for evidence, flows, claims, conflicts, gaps, and verification;
4. explicit expansion by handle; and
5. raw source only for unresolved or decision-critical spans.

Repeated questions reuse immutable, blob-addressed fragments. Callers receive no duplicate source
bodies, broad repository dumps, or speculative adjacent context by default.

Serving is a compiled fast path, not a fresh repository investigation. Corvint automatically widens
through exact identities, symbols, static relationships, claims/tests, history, and authorized
external indexes under one declared budget. Orthogonal retrievers run concurrently where useful;
disagreement triggers bounded widening rather than a confident guess. No unchanged blob is reparsed,
and no runtime model call is required to locate deterministic facts.

On a frozen representative corpus, a warm first packet targets p95 below one second and a handle
expansion p95 below 250 ms on the reference machine. The cold path still targets a first useful
answer within five minutes. For a held-out task whose labelled evidence lies inside supported source
types, the caller must not need to invent a repository-wide `rg`, Git, or document search: Corvint
either finds the minimum witnesses by automatic widening or names the exact unsupported/access gap.
Manual search remains an escape hatch and is counted as product failure, not normal workflow.

The corpus-substitution benchmark compares ordinary agents with Corvint-assisted agents on untouched
legacy tasks. Expansion is promoted only if all of these hold:

- at least 30% lower median input tokens;
- non-inferior task completion and review correctness;
- zero additional critical-evidence misses;
- a lower or equal rate of unnecessary source opens; and
- at least 80% of answers accepted without a broad repository rescan.

If agents routinely reread every cited source, Corvint has added ceremony rather than saved context and
the relevant generated layer is killed or narrowed.

Token accounting includes every billed input token over the complete multi-turn task, including
tool output and replayed history. Packet size is a diagnostic, not the savings metric. Promotion
also requires at least 25% lower total tokens; compressing the input while making the agent reason
or converse longer is not a win.

## Context dividend: sessions pay future sessions

During a task, Corvint maintains a private append-only working-set receipt containing the declared
intent and requirement IDs, evidence handles already expanded, hypotheses and unknowns, decisions,
changed hunks, verification observations, outcome state, and token/source-open counters. It stores
selectors, identities, hashes, and bounded agent-authored notes—not prompts, source bodies, command
output bodies, or an assertion that the agent's current belief is true.

At checkpoint, handoff, merge, revert, or close, a deterministic compiler emits a content-addressed
session delta. The knowledge updater independently re-verifies it against Git, CEM, OCM, accepted
intent, and pinned test/runtime attestations. Verified facts may refresh durable claims; unsupported
interpretations remain `INFERRED`; conflicts and failed/reverted outcomes remain visible; only a
merged revision may advance revision-scoped product knowledge.

A successful session can therefore produce a reusable minimum-witness task capsule. Similar later
tasks start from that capsule, deduplicated against handles already in their context, and Merkle-diff
invalidation removes or widens only stale parts. This is the context dividend: evidence discovery
paid for by one agent becomes safe context and documentation maintenance for later agents. Immutable
per-session receipts compose without a shared mutable session or mandatory database; a local Git
store and an authorized team service consume the same delta contract.

## Four loops, one substrate

```text
BACKFILL: inventory -> orphan queue -> claims -> rendered draft -> verify -> human promotion
SERVE:    question -> minimum witness -> answer/unknown -> selective expansion
SYNC:     merge -> evidence diff -> stale/reverify -> minimal regeneration -> republish

INTENT:   accepted spec -> capability/claim join -> corroborated | contradicted | conflicted | unknown
```

The three operational loops share immutable evidence fragments, inventory identities, derived flows,
claim records, aliases, conflicts, gaps, and verification results. `INTENT` is an independent axis:
specifications never author claims about behavior, and implementation never silently becomes intent.

## Change-triggered maintenance automations

The first integrations are explicit CLI commands and CI hooks, not a daemon. Every trigger is
idempotent, coalesces superseded work, binds exact input revisions/versions, preserves the last-good
artifact on failure, and produces a reviewable receipt.

| Trigger | Deterministic work | Optional model work |
|---|---|---|
| PR update or merge | Merkle-diff evidence, mark affected claims/paragraphs stale or needing reverification, refresh OCM/test candidates | Propose cited rewrites only for affected paragraphs |
| Verified session checkpoint/close | Deduplicate the working set, bind outcome and evidence, emit a reusable task capsule and updater delta | Propose anchored claims, aliases, or missing-documentation text as `INFERRED` |
| CI/test result | Attach pinned execution attestations, expire superseded observations, quarantine known-flaky proof candidates | Summarize only verified result metadata; never invent a pass or coverage claim |
| Tag or release | Derive exact revision-range facts, delivered/deferred requirements, tests, and unknowns | Phrase a cited release narrative |
| Human-resolved incident | Bind timeline, diagnostics, changes, owners, and outcome to a recurrence capsule | Propose a cited runbook delta; never execute or label commands intrinsically safe |
| Manual or scheduled reconciliation | Compare Git and adapter manifests, tombstone confirmed deletion, expose access loss and missed events | Extract anchored candidates from changed quarantined bodies only |

Unchanged content never invokes a model again. Model output is content-addressed to its producer,
prompt/profile digest, source handles, and output bytes, then mechanically checked for valid anchors.
It can update a generated view only as a proposal; it cannot silently edit accepted intent, claim a
test passed, or promote its own authority. Repeated maintenance requiring broad rereads is a failed
incremental design and must be repaired or removed.

Model selection begins only after the applicable deterministic resolvers produce one exact semantic
gap. No registered admission verifier means no model call and an explicit unknown. Otherwise Corvint
chooses the least-cost exact model revision whose frozen profile meets the task's schema, precision,
privacy, context, and budget requirements. One stronger call is permitted only for a named
admission failure that the stronger profile demonstrably handles better. Query serving never calls
a model; it consumes compiled derivations or returns the gap. The normative boundary is
[`specs/semantic-escalation-gate-v0.md`](specs/semantic-escalation-gate-v0.md).

## Inventory, flows, and aliases

At a pinned revision, Corvint classifies every tracked source unit as mapped, orphaned, excluded, or
unsupported. The denominator is always visible. An orphan is a successful answer containing the unit,
symbols, callers, candidate tests, and the honest statement that no flow explanation exists.

Flows are derived views over claims and relationships, not a required folder shape. A useful flow may
render an overview, entry points, sequence, behavior, implementation detail, entities, exceptions,
UI journey, and related flows, but empty boilerplate is forbidden.

Project vocabulary and business aliases are first-class, provenance-bearing records. Corvint may map
"Block Notes" to `TeetimeClosure` only when a checked-in glossary, accepted specification, ticket,
review, or human-approved alias says so. Text similarity alone produces a candidate, not an alias.

## Durable claims, disposable prose

Atomic claim records are keyed by immutable evidence and never require a flow or paragraph ID.
Generated prose renders those records. Each paragraph has a stable rendering ID and a sidecar mapping:

```text
paragraph ID -> claim IDs -> evidence/citations -> validation stamps
```

Claim identity survives reorganizing documentation. Paragraph split/merge retires old IDs with
provenance pointers so review links remain resolvable. A regeneration may touch only paragraphs whose
claims or dependencies were affected, unless it records a visible widening reason.

Paragraph anchoring proves traceability, not semantic truth. Promotion requires at least 0.95 recall
of affected paragraphs on labelled historical merges and at most 5% unrelated paragraph churn.

## Anti-fabrication battery

No producer certifies its own output. A separate process independently re-derives every advertised
count, identity, coverage denominator, citation, and freshness result.

Every gate ships with:

- a fabricated fixture proving it fails;
- a genuine fixture proving it does not reject the intended property; and
- a no-input case that returns `NOT_RUN` or failure, never success.

The battery checks citation completeness, anchor identity, existence of named routes/symbols/tests/
flags, template-like or unnaturally uniform claims, suspicious anchor clustering, duplicate claims,
unsupported certainty, corpus/manifest disagreement, and metrics copied from producer self-reports.

Grounded prose can still explain code incorrectly. Correction rate remains a headline metric, and
security-, payment-, permission-, migration-, and incident-response material always retains human
review until an explicit owner changes that policy.

## Evidence and conflicts

Behavioral evidence and intent are never ranked on one scale. Source proves implemented structure;
pinned test or runtime execution observes bounded behavior; accepted project specifications govern
intent. Tickets, pull requests, wikis, test plans, incidents, and design documents are dated context.

External ingestion is shallow by default: index structured identity, links, timestamps, and content
digests; read full bodies only on demand. Raw mutable text stays quarantined locally, never enters
authoring context as instructions, and remains retention-bounded. Missing credentials are coverage
gaps, not failed setup.

Disagreement creates a first-class conflict with both witnesses. Corvint does not blend sources into a
confident average. A flaky test remains linked but is quarantined from proving a claim; quarantine
thresholds must achieve at least 0.90 precision on labelled histories before enforcement.

## Stale-first maintenance

Merge processing is idempotent and keyed by exact commit. Direct edits to an anchored span mark the
dependent claim `STALE`. One-hop call/dependency effects are `NEEDS_REVERIFICATION`, because indirect
impact is a candidate blast radius rather than proof of invalidity.

The newest commit wins. A newer impact closes or supersedes regeneration against an older revision.
Failed refresh leaves the last-good artifact available with stale/reverification state visible and
does not advance the marker. Periodic reconciliation catches missed events.

Automatic regeneration remains advisory until at least 30 reviewed regeneration changes show:

- less than 5% human correction;
- zero critical fabrication;
- at least 0.95 affected-paragraph recall; and
- at most 5% unrelated paragraph churn.

Auto-merge is not part of the initial contract.

## Agent contribution path

Agents improve the brain through two local, structured operations:

- `flag_issue`: record what appears wrong, its stable subject, and cited evidence;
- `propose_update`: submit claim or rendering changes with citations and derivation provenance.

Neither writes authoritative source or accepted specifications. Repository mutation, pull-request
creation, or external messages require explicit authority. Every accepted contribution is attributable,
reviewable, revertible, and rechecked by a process other than its producer.

## Setup and maintenance experience

The target first run is:

1. install one reviewed executable;
2. run `corvint init` for a new repository or `corvint adopt` for an existing product, without project
   configuration or a successful build;
3. receive the first useful, cited answer within five minutes;
4. check in a small optional project profile for aliases, authority, exclusions, and mandatory gates;
5. enable advisory merge refresh; and
6. promote enforcement only after repository-specific evaluation passes.

No daemon, hosted account, graph/vector database, or new specification language is required. Cache
and rendered artifacts are disposable; Git remains the source of authority. Maintenance must be
incremental, byte-identical to a clean rebuild, observable, interruptible, and safe to remove without
changing source or accepted intent.

## Delivery order

1. CEM pilot: first-run workflow, reviewer report, independent conformance, measured 30x30 trial.
2. Inventory and orphan frontier across representative legacy repositories.
3. Durable claim/paragraph receipts and deterministic Markdown views.
4. Measured BACKFILL and corpus-substitution benchmark.
5. Read-only SERVE commands over the same artifact.
6. Advisory SYNC and per-statement INTENT drift.
7. Optional local history and external evidence adapters.
8. MCP/LSP and controlled write integrations only after CLI/JSON value is proven.

SQLite, embeddings, hosted services, UI, mandatory flow folders, authored dependency graphs, numeric
confidence scores, automatic spec acceptance, automatic PR creation, and auto-merge are deliberately
excluded until a measured bottleneck requires them.
