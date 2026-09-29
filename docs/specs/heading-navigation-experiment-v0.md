# Heading navigation experiment V0

Owner: Russell Lewis
Date: 2026-09-29
Requirement prefix: `HNE-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner request to implement the PageIndex-inspired comparison on 2026-09-29;
`AGENTS.md`; `documentation-corpus-v1.md`; `retrieval-eval-comparability-v0.md`.

## Agent digest
- Claim: An offline harness compares native lexical document ranking with deterministic heading-aware expansion under equal serialized context-byte budgets.
- Status: proposed/experimental; no production retrieval change or promotion.
- Exists: `tools/heading-nav-bench` and an author-visible pinned pilot fixture.
- Blocked on: independent external utility evidence; adaptive agent navigation remains unmeasured.
- Read next: Requirements; Evaluation protocol; Failure modes and rollback.

## Human intent and boundary

Evaluate the useful PageIndex ideas before changing Core: structural navigation, separate outlines
and source reads, and measured retrieval cost. The existing corpus compiler preserves literal
paragraphs and native search ranks document subjects lexically. Existing experimental source views
already expand pinned handles. This experiment adds a local comparison consumer, not a second
production index or a new authority layer. Native lexical search remains unchanged.

PageIndex references: its `pageindex/page_index_md.py` heading extraction, `agent_tools.py` separation
of document structure and page content, and `tree_optimize.py` routing-versus-scan cost model at
https://github.com/VectifyAI/PageIndex. No PageIndex source is copied or dependency installed.

## Requirements

- `HNE-V0-001`: The explicit fixture pins a full Git commit, source paths, timestamp, questions and
  gold spans. Build the native documentation corpus from those immutable inputs and retain fixture,
  corpus and source identities. Dirty live source cannot supply experimental evidence. Invalid
  identities/spans refuse; gold and answer notes never affect retrieval or enter reader packets.
- `HNE-V0-002`: Both arms use the same actual native corpus lexical document ranking. Baseline reads
  document prefixes in rank order. Candidate uses deterministic lexical section selection and
  heading ancestry with exact original source spans. Preambles and parent introductions remain
  representable. Fenced pseudo-headings are not sections; unsupported syntax stays explicit.
- `HNE-V0-003`: Every packet, including JSON escaping, question, metadata, provenance and outline,
  fits the same configured serialized UTF-8 byte budget including its terminal newline. Truncation
  is line-granular with explicit omissions; an envelope that cannot fit refuses. Outline labels
  are navigation only. Only the union of actually emitted original text spans counts as evidence.
- `HNE-V0-004`: Reports retain per-arm/per-case evidence-hit numerators and gold denominators,
  full-span misses, actual packet bytes, timings and limitations. A zero denominator is undefined.
  A paired same-model reader trial scores answer correctness, authority choice and unsupported
  conclusions separately. Missing billed tokens, cache usage or host effort telemetry is
  `NOT_OBSERVED`; bytes never stand in for tokens or savings.
- `HNE-V0-005`: The harness remains explicitly invoked and experimental. Only the chosen output
  directory receives artifacts; production command bytes, rankings, schemas, frozen evaluation
  corpora and learning remain unchanged. Retain failures and losing arms. A local pilot cannot
  promote production or establish generalization, complete-task savings or adaptive navigation.

## Evaluation protocol

The pilot fixture has six documents, eight questions, ambiguous shared headings, cross-document
obligations, an amended accepted decision and a deliberate unsupported-algorithm question. It is
visible to the author: this is a development pilot, never a blind holdout. Freeze its raw SHA-256
before first run. Both arms receive the same 6,000-byte context budget per question and native
ranking. Candidate selection is deterministic lexical routing, not an LLM traversal. Baseline is
native lexical ranking plus a sequential prefix-read consumer policy; it is not the strongest
possible agent using ordinary search, `git show`, requirement IDs or arbitrary range reads.

After packet generation, each question/arm pair receives its own fresh ephemeral Codex CLI reader
process with only that packet, the frozen `testdata/reader-prompt.txt`, and the model/effort,
160-word answer limit and scoring rules in `testdata/rubric.json`. No reader sees another case,
gold or source lookup. Tool use invalidates a trial. These are workflow constraints, not provider
spend controls.
Record actual reader runtime and missing token telemetry. Root grades explicit answers against
frozen rubric atoms and actually supplied original spans; an independent reviewer checks scoring and conclusions.
Reader trials measure grounded answers from fixed packets, not adaptive tool-use behavior.

Before adoption, repeat on independently authored unseen tasks and compare against a stronger
search-plus-targeted-range baseline. Measure full task outcomes and billed tokens with an observable
harness. Do not choose a winner by tuning the fixture or budget after seeing this pilot.

## Failure modes and rollback

Bad labels and broad `Requirements` sections can defeat hierarchy routing. Early lexical document
misses affect both arms. Outline overhead competes with evidence text. Generated summaries can
omit exceptions, so this first version uses original headings only. Parent headings cannot confer
accepted authority; document status and source must justify it. Citation coverage does not establish
semantic support, exhaustive discovery, tested behavior or legal interpretation.

Rollback removes the standalone harness and its experimental registry entry. No Core data or
schema migration is involved. Publishing a draft is not merge approval or native ticket completion.

## Acceptance evidence

Focused harness and `internal/doccorpus` tests, harness vet, required documentation checks, frozen
pilot outputs, paired reader answers, independent review and sealed change evidence. Repository-wide
`make gate` is `NOT_RUN` under the owner's scoped-work policy. Task V1-0474 remains open until its
required integration and native completion disposition succeed.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| HNE-V0-001 | `tools/heading-nav-bench/main.go` | `TestImmutableSource`, `TestGoldIsolation` |
| HNE-V0-002 | `tools/heading-nav-bench/main.go` | `TestSections`, `TestFencesAndSetext`, `TestDeterministicSectionSelection` |
| HNE-V0-003 | `tools/heading-nav-bench/main.go` | `TestBudget` |
| HNE-V0-004 | `tools/heading-nav-bench/main.go` | `TestFullSpanRecall`, frozen reader rubric and retained trial |
| HNE-V0-005 | `tools/heading-nav-bench/main.go` | `TestGoldIsolation`, unchanged production diff and explicit pilot invocation |

See `docs/build-log/2026-09-29-heading-navigation.md` for the actual result and limitations.
