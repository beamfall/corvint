# Technical-brain dogfood suite

Owner: Russell Lewis
Frozen: 2026-08-22
Intent status: accepted
Delivery status: not-started
Authoritative inputs: `docs/PRODUCT.md`, `docs/TECHNICAL-BRAIN.md`, `docs/LEGACY-ADOPTION.md`

## Agent digest
- Claim: Frozen Corvint workflows must prove bounded cited context improves engineering outcomes before technical-brain promotion.
- Status: accepted/not-started
- Exists: accepted requirements plus partial deterministic measurement support in `benchmarks/dogfood_measure.py`.
- Blocked on: frozen tasks, labelled answers, held-out retrieval, and complete outcome measurements.
- Read next: User and job; Requirements; Traceability.

## User and job

Corvint must prove on its own repository that it is a useful first context call for common engineering
work. The same local-first artifacts must serve one developer and a large organization; scale may
add policy and authorized composition, never a separate truth model or mandatory service.

Every answer is revision-pinned, bounded, source-cited, epistemically classified, explicit about
exclusions and unknowns, and progressively expandable without repeating source bodies.

## Requirements

- `BRAIN-DOG-001`: butterfly-effect MUST return exact direct static references/dependencies,
  bounded consequence candidates, affected specs/tests/contracts, and unexamined dynamic surfaces
  for a change. It MUST NOT label static reachability as runtime causality.
- `BRAIN-DOG-002`: E2E discovery MUST map requirements and journeys to exact scenario/test witnesses
  and return `UNKNOWN` for missing or unobserved states; it MUST NOT equate coverage with proof.
- `BRAIN-DOG-003`: selective-test guidance MUST name mandatory repository gates first and a smaller
  cited candidate set for iteration. Until outcome gates pass, it MUST NOT claim tests are safe to
  skip.
- `BRAIN-DOG-004`: onboarding MUST get a new engineer from one question to a cited architecture,
  workflow, vocabulary, owning specs, relevant code/tests, and runnable next actions without a broad
  repository read.
- `BRAIN-DOG-005`: ticket routing MUST return the likely owning scope/team, code/tests/specs, required
  gates, alternatives, and unknown ownership. It remains advisory and MUST NOT assign or mutate work.
- `BRAIN-DOG-006`: code review MUST combine exact change/CEM evidence, consequences, tests, intent,
  ownership, conflicts, and unknowns without asserting correctness or merge safety.
- `BRAIN-DOG-007`: spec baseline MUST render ordinary Markdown with every atomic statement classified
  as `PROVED`, `OBSERVED`, `INFERRED`, `CONFLICTED`, or `UNKNOWN`; generated intent starts as an
  inferred draft and cannot promote itself.
- `BRAIN-DOG-008`: migration/deprecation planning MUST enumerate exact in-repository consumers,
  schemas/contracts, compatibility evidence, removal blockers, rollout/rollback work, and unknown
  external consumers.
- `BRAIN-DOG-009`: release narration MUST cite every shipped claim to a revision-range fact and keep
  failed, deferred, and `NOT_RUN` evidence visible. A fabricated promotion or pass is invalid.
- `BRAIN-DOG-010`: incident assistance MUST return matching runbooks, authority/owners, recent
  relevant changes, failure evidence, bounded read-only diagnostic argv candidates, and unknown
  runtime surfaces. Candidates require human authorization and MUST NOT be labelled intrinsically
  safe or executed by default.
- `BRAIN-DOG-011`: each workflow MUST expose its deterministic facts separately from semantic
  candidates and human-owned decisions.
- `BRAIN-DOG-012`: each workflow MUST ship with one frozen Corvint-on-Corvint development task and a
  separate held-out evaluation manifest containing exact revision, expected evidence and unknowns,
  forbidden claims, cheaper baseline argv, evaluator identity, and sample/repeat rule.
- `BRAIN-DOG-013`: the suite MUST measure input-token reduction, source opens, task/review correctness,
  critical-evidence misses, latency, and unnecessary output bytes without hiding hard-gate failures
  in an aggregate score.
- `BRAIN-DOG-014`: adapters and generated views MUST be optional, removable, incrementally refreshed,
  and byte-identical to a clean rebuild for the same inputs.
- `BRAIN-DOG-015`: the compiled serve path MUST automatically widen across supported deterministic
  indexes without a runtime model call, avoid recomputing unchanged blobs, and meet p95 warm budgets
  of one second for the first packet and 250 ms for handle expansion on the frozen reference corpus.
- `BRAIN-DOG-016`: when a held-out task's labelled evidence is inside supported, authorized source
  types, Corvint MUST find it or name the exact unsupported/access gap without requiring the caller to
  formulate a repository-wide search. Any caller-authored broad search is recorded as a product miss.
- `BRAIN-DOG-017`: an agent session MUST be able to emit a private, append-only working-set receipt
  containing task/spec identity, evidence handles, hypotheses, decisions, unknowns, changed hunks,
  observations, outcome, and context-cost counters without storing prompt, source, command-output, or
  external-body contents.
- `BRAIN-DOG-018`: checkpoint, handoff, merge, revert, and close MUST compile the working set into a
  content-addressed session delta that an independent updater can verify. Only merged, verified facts
  may refresh durable product knowledge; inference, conflict, failed work, and reverted work retain
  those states. Local and authorized team stores MUST consume the same storage-neutral receipt.
- `BRAIN-DOG-019`: a successful verified session MAY produce a reusable minimum-witness task capsule;
  later packets MUST deduplicate already supplied handles and invalidate or widen only capsule items
  affected by evidence drift. Failed or blocked sessions MUST NOT become successful context exemplars.
- `BRAIN-DOG-020`: PR/merge, session, CI/test, release, incident-closeout, and reconciliation
  automations MUST be idempotent, revision/version-bound, coalescing, and last-good preserving. Model
  work runs only over changed, cited inputs and remains an `INFERRED` proposal that cannot promote
  intent, test success, or its own authority.

## Measurement contract

Before implementation, development labels live under
`benchmarks/technical-brain-dogfood/<workflow>/development/`; they may guide the build and never count
as outcome evidence. After the interface and evaluated Corvint build are frozen, an independent
labeller creates a held-out manifest under an access boundary the producer cannot read. Agent outputs
are locked before its private oracle is revealed.

Each held-out packet has two hash-bound projections:

- `query.json`, revealed identically to both arms, contains the manifest ID, full Git commit OID,
  exact question, authorized source roots, repository instructions, execution/tool/network/time
  envelope, sample/repeat rule, output schema, fixed baseline harness argv/protocol, and fixed
  treatment invocation argv;
- `oracle.json`, sealed until outputs lock, contains repository-relative expected-evidence paths plus
  immutable selectors, expected unknowns, forbidden claims, workflow rubric, thresholds, kill rule,
  and evaluator identity.

Before either arm runs, the labeller publishes a Git-committed `commitment.json` containing the query
digest and `SHA-256(32-byte random salt || canonical oracle bytes)` while keeping the salt and oracle
sealed. After outputs lock, both projections and the salt are published under the workflow's
`heldout/` directory; a verifier recomputes the commitment. Results bind the commitment commit, both
projection digests, and frozen Corvint build digest and begin `NOT_RUN`.
Held-out questions or revisions differ from development labels and are evaluated independently.
Suite medians and rates use all ten held-out workflows; an oracle that needs more observations names
the minimum sample, and no outcome is computed below it.

Both arms receive identical checked-in repository bytes—including any checked-in Corvint sidecar—plus
identical non-Corvint instructions, time, network, execution, and tool permissions, including project
gates, test discovery/execution, local CLI help, `rg`, and `git diff|show|log`. `query.json` enumerates
the only treatment additions: the exact Corvint executable digest and disposable Corvint-generated cache,
packet, and report paths. The baseline cannot invoke Corvint but may inspect all checked-in raw files.

## Corvint development tasks and held-out rubrics

The named tasks are development cases only. The third column is the workflow-level rubric copied
into a different held-out case's sealed oracle; it is not evidence from the named case.

| Workflow | Corvint-on-Corvint development task | Held-out rubric and minimum |
|---|---|---|
| Butterfly effect | change `context_corvint_cem.verify_cem` and find CLI, report/workflow, CI, interop, and test consequences | 100% labelled direct spec/importer/test recall on at least 10 changes; otherwise retain file-level impact only |
| E2E discovery | ask what proves `CEM-PILOT-014` | claim precision at least 0.70 and zero false-covered verdicts on at least 30 labelled claims |
| Selective tests | change `context_corvint_cem_workflow.py` | zero change-attributable failures outside candidates on at least 30 historical changes; otherwise advisory ordering only |
| Onboarding | use a fresh linked worktree to produce and inspect one CEM | median under 15 minutes and incorrect hard failures below 5% across at least 10 fresh runs |
| Ticket routing | route a linked-worktree report-default failure | beat exact-text search with zero extra critical ownership/gate misses across at least 20 tickets |
| Code review | review the CEM pilot using its checked-in sidecar | at least 20% fewer missed-evidence findings, at least 70% citable hunks, and below 5% incorrect hard failures in the preregistered 30-by-30 trial |
| Spec baseline | backfill the report module contract | statement precision at least 0.70 and zero automatic promotions across at least 30 labelled statements |
| Migration | propose a `cem/0.1` wire change | 100% known in-repo consumer and conformance-vector recall across at least 10 contract changes |
| Release narration | explain the CEM pilot release state | zero invented validation/pass claims and under 5% correction on at least 20 labelled claims across three revision ranges; 30 releases remain optional longitudinal evidence |
| Incident assistance | diagnose `evidence-drift` | beat the equal-tool baseline time-to-first-correct diagnostic with zero harmful recommendations across at least 10 incidents |

## Suite-level value gate

Across the separate held-out tasks, Corvint must reduce median input tokens by at least 30% with non-inferior task
and review correctness, zero additional critical-evidence misses, no increase in unnecessary source
opens, at least 25% lower total tokens, and at least 80% of answers accepted without a broad rescan.
Count every billed token across the full multi-turn run, including tool results and replayed history;
packet bytes alone are not a savings result. On supported held-out evidence, Corvint must require zero
caller-authored repository-wide discovery searches. A workflow that misses its own gate is removed,
narrowed, or retained as clearly labelled navigation; suite breadth cannot rescue it.

## Non-goals and rollback

This suite does not require a daemon, hosted account, graph/vector database, automatic editor,
central policy system, autonomous incident action, automatic ticket assignment, or test skipping.
Rollback deletes derived fixtures, views, and optional policy; repository source and accepted intent
remain untouched.

## Partial Claude accounting behavior

`benchmarks/dogfood_workers.py` records observed assistant request identities separately from
usage snapshots, per main/sidechain partition. The existing selection expression is preserved:
`usage.requestId` when truthy, otherwise the outer `requestId`; with non-dictionary usage only the
outer field is available. Only the selected nonempty string is usable, with its exact bytes and
no whitespace normalization. Differing string locations do not conflict; `message.id` is unused.
Selected invalid or absent identities leave that chain's turns and token totals `NOT_OBSERVED`,
without assigning anonymous work to a later or adjacent keyed event.

A keyed assistant record with absent, null or other non-dictionary usage is deferred. A later
same-ID dictionary can supply its observed snapshot; missing non-snapshot chunks before or after
it do not create another turn. At EOF a distinct observed request with no dictionary keeps the
chain's four token totals unknown while retaining its exact known request count. Actual usage
dictionaries retain field-wise sticky uncertainty across missing, malformed or decreasing values.
No terminal marker, complete capture or full billing guarantee is inferred from EOF or snapshots.

All-sidechain promotion depends on assistant-record presence, so observed main work without
usage cannot be replaced by an observed child. Anonymous side work likewise stays separate from
known main work. No-assistant transcripts retain the existing unknown result. Tool, compaction
and unparsed-record counters retain their independent meanings; `sourceOpens` counts recognized
tool-event forms, not guaranteed source-file reads. Cost and retry telemetry remain unknown.
These are partial synthetic measurement semantics under BRAIN-DOG-013/DOGFOOD-008, not a completed
outcome campaign, full worker roster or technical-brain promotion.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| BRAIN-DOG-001..012, 014, 017..020 | not started | frozen tasks, labelled answers, and outcome measurements pending |
| BRAIN-DOG-013 | `benchmarks/dogfood_measure.py`; `benchmarks/dogfood_workers.py` | partial measurement support only: deterministic latency/resource/output recorder plus an all-worker cost receipt (turns, input/cache/output tokens, source opens, broad searches, compactions, retries) parsed from Claude Code and Codex transcripts, with `NOT_OBSERVED` for missing telemetry and cost always `NOT_OBSERVED`; correctness and reviewer-miss scoring remain `NOT_OBSERVED`/`NOT_RUN`; `tests/test_dogfood_workers.py::DogfoodWorkersTest.test_brain_dog_013_claude_request_snapshots_preserve_uncertainty` retains field-wise uncertainty across repeated request snapshots (synthetic diagnostic only); `benchmarks/dogfood_workers_cli_test.go::TestDogfoodWorkersCLI` independently exercises the public receipt CLI with seven synthetic workers, verifies identity hashes, request/chain controls and field-wise uncertainty; this is partial accounting coverage, not an outcome campaign; `tests/test_dogfood_workers.py::DogfoodWorkersTest.test_brain_dog_013_claude_missing_usage_reconciliation` and `benchmarks/dogfood_workers_reconciliation_cli_test.go::TestDogfoodWorkersMissingUsageCLI` separately cover request/usage reconciliation and main/side presence through synthetic receipts |
| BRAIN-DOG-015 | `benchmarks/dogfood_measure.py` | partial measurement support only: latency threshold recording; compiled serving, automatic widening, incrementality, and expansion measurement remain `NOT_RUN` |
| BRAIN-DOG-016 | not started | held-out retrieval and gap-behavior evidence remain `NOT_RUN` |
