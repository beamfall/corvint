# Context ranking V0

Owner: Russell Lewis
Date: 2026-11-01
Intent status: proposed
Delivery status: experimental

Authoritative inputs: native tickets V1-3273, V1-0219, V1-3710, V1-0114, V1-0116 and V1-2716,
decision 0027 (corroboration order), decision 0377 and `TCP-V0-048..050` in the
[task-context packet contract](task-context-packet-v0.md), and the span-mode contract
`TCP-V0-025..029` in the same spec. These requirements add opt-in orderings and span modes to
`corvint context` and remove per-call allocation from term counting. Every ordering and span mode
is behind an environment flag that is off by default; with every flag unset the packet bytes are
the recipe golden and the span golden, unchanged. No index, snapshot, pack, root verb or wire
profile changes.

## Agent digest
- Claim: Opt-in RRF, additive and acronym orderings, span hit ranges, fallbacks and merging, and allocation-free term matching, each measured on the frozen bench.
- Status: proposed; experimental; RRF and additive failed the frozen gate and stay opt-in; the acronym rule and span modes stay opt-in pending owner decision.
- Exists: `CORVINT_CONTEXT_RRF`, `CORVINT_CONTEXT_ADDITIVE`, `CORVINT_CONTEXT_ACRONYMS=prose`, `CORVINT_CONTEXT_SPANS=hits|merged`, and `matchTerms` with a text-sized interner.
- Blocked on: the decision 0027 history-backed corpus for TCP-V0-050(b) and the ContextBench dataset for V1-2716 AC1.
- Read next: Requirements; Measurements; Failure modes; Rollback.

## User and current state

Human-owned inputs: the six tickets above. `corvint context` admits rows through fixed slots and
orders them by decision 0027's corroboration count; a fixed slot score (definition 800, paired
test 650) outranks any lexical row, so a test can outrank its source and an all-capital prose
word such as `CEM` can admit an unrelated declaring file (V1-3710). TCP-V0-048..050 proposed a
reciprocal-rank-fusion ordering that was never implemented (V1-0219). Span mode returns a
declaration's first 80 lines without naming the hit, silently drops a span that overruns the
budget, and silently drops overlapping candidates (V1-0114, V1-0116). `countTerms` built an
8192-entry interner on every call, about 438 KB per line (V1-3273).

## Requirements

- `CRK-V0-001`: (proposed 2026-11-01, not accepted; V1-3273) `countTerms` and `scanWords` MUST
  size their one-shot interner from the text they read, `min(interningHint, len(text)/3+1)`,
  and MUST return the same terms and counts as the worker-sized counter.
- `CRK-V0-002`: (proposed 2026-11-01, not accepted; V1-3273) `matchTerms(text, wanted)` MUST
  return the sorted distinct `countTerms` terms of `text` that `wanted` holds, without building
  `text`'s term-count map; `lineTokens` and `roleTerms` MUST use it, so their results are
  unchanged.
- `CRK-V0-003`: (proposed 2026-11-01, not accepted; V1-3273) Benchmarks
  `BenchmarkCountTermsLine`, `BenchmarkLineTokens`, `BenchmarkRoleTerms` and
  `BenchmarkScanWordsFile` MUST report allocations per call, and the before and after readings
  MUST be recorded.
- `CRK-V0-004`: (proposed 2026-11-01, not accepted; V1-0219) With `CORVINT_CONTEXT_RRF=on` the
  ordering step after admission MUST implement TCP-V0-048 and TCP-V0-049: `F(p)` summed over the
  admitting relation and every corroborating relation with `k = 60`, `rank_r(p)` counted across
  every slot call that yields relation `r`'s kind with the subject removed (the first position a
  path takes in a relation is kept), a descending stable sort whose ties keep slot order, and the
  `; rrf F (KIND@RANK, ...)` reason. Admission, caps, membership, the score rise of 50 per
  corroboration and the `; also` summary MUST be corroboration's.
- `CRK-V0-005`: (proposed 2026-11-01, not accepted; V1-3710) With `CORVINT_CONTEXT_ADDITIVE=on`
  the ordering step MUST sort rows, descending and stable, by `L(p) + B(kind) + 50` per
  corroborating relation, where `L(p)` is the path's share of the lexical band from the same BM25
  walk the lexical fill ranks by (0 when the walk did not reach it) and `B` is the fixed table
  mentioned 300; pair, definition, calls-local, calls-imported and ruby-constant 150;
  reverse-import and reference 100; cochange, sibling and test 50; any other kind 0. The row's
  score MUST be `min(lexical base + key, 999)` and its reason MUST gain
  `; additive KEY (lexical L, KIND +B, also +C)`. The table was fixed before any reading.
- `CRK-V0-006`: (proposed 2026-11-01, not accepted; V1-3710) With
  `CORVINT_CONTEXT_ACRONYMS=prose` a task identifier that is an all-capital ASCII word of at least
  two letters and has weight below 3 (not quoted as code) MUST NOT be a definition-slot name; a
  quoted one and any mixed-case name MUST be unaffected.
- `CRK-V0-007`: (proposed 2026-11-01, not accepted; V1-0219, V1-3710) Each flag MUST be read
  exactly: `CONTEXT_RRF` and `CONTEXT_ADDITIVE` only at `on`, `CONTEXT_ACRONYMS` only at `prose`.
  Unset, `off` or any other value, including another case, MUST leave the recipe golden
  byte-identical. When RRF and additive are both on, RRF MUST govern and no row carries an
  additive reason. The acronym rule composes with either ordering.
- `CRK-V0-008`: (proposed 2026-11-01, not accepted; V1-0219, V1-3710) Each ordering MUST be
  read with one binary, flag off and on, over the frozen `tools/retrieval-bench --arms context`
  subsets `v2_code2test`, `v2_comment2context`, `v2_edit2ripple` and `v2_trace2code` at limit
  20, recording recall@5/10/20, MRR and the gold rank buckets. An ordering that loses recall@20
  on any subset fails the gate and MUST stay opt-in or be removed; the failed reading MUST be
  recorded as a failed evaluation. Promotion to default needs its own owner decision.
- `CRK-V0-009`: (proposed 2026-11-01, not accepted; V1-0114) With `CORVINT_CONTEXT_SPANS=hits`
  (or `merged`) each span row MUST carry `hit_start_line` and `hit_end_line`, the matched range the
  span grew from: the declaring line for a definition span, the matched line for an enclosing or
  window span, and the call line for a call site. Removing those two members MUST give the `on`
  packet byte for byte when no fallback applies. `on` MUST be unchanged.
- `CRK-V0-010`: (proposed 2026-11-01, not accepted; V1-0114) Under `hits` and `merged`, a
  declaration clipped to the 80-line span cap MUST say so in its reason; a hit that lies inside a
  declaration but past its clip MUST return the hit window (radius 5, clamped to the file) with a
  reason naming the declaration and its length; and a span that would overrun the remaining line
  budget MUST fall back to its hit window, with a reason, when that window is smaller and fits,
  else be counted omitted as under `on`. No span crosses a file boundary.
- `CRK-V0-011`: (proposed 2026-11-01, not accepted; V1-0116) Under `merged`, a candidate span
  that overlaps or is adjacent to selected spans of the same path MUST be folded with them,
  transitively, into one row at the earliest joined position, keeping the head's role, symbol,
  reason, authority and the union of hit ranges, when the union is at most 80 lines and fits the
  budget; the row MUST list each other contributor in `merged_from` with its role, symbol, range,
  reason and hit range. `mergeSpans` MUST be a pure, deterministic function of the selected spans
  and the candidate, and leave the other rows' order unchanged.
- `CRK-V0-012`: (proposed 2026-11-01, not accepted; V1-0114, V1-0116) Each span mode MUST be read
  on the same frozen subsets, recording frozen file recall, packet bytes, span rows, lines used and
  a lane diagnostic of gold-span line coverage against each sample's `gold_spans`. A mode whose
  packet bytes rise or whose recall falls against `on` MUST stay opt-in.

## Non-goals

- Promoting any ordering or span mode to default; that is a later owner decision.
- Tuning `k`, the slot bonus table or the span radii on the evaluation sets.
- Embeddings, network access, a new index or any persisted state.
- Changing slot admission, caps or which rows a packet admits.
- Running the decision 0027 history-backed reading or ContextBench without their corpora.

## Failure modes

- RRF discards slot precedence for single-relation rows (TCP-V0-050's named risk): measured as a
  recall@20 loss on `v2_trace2code`; it stays opt-in.
- Additive lets a strong lexical row displace a structural relation past `--limit`: measured as a
  recall@20 loss on `v2_comment2context`; it stays opt-in.
- The acronym rule hides a real all-capital type the task means: only an unquoted word is
  affected, a quoted one still admits.
- A hit-window fallback drops lines a clipped declaration would have kept: the first variant,
  which fell back on every clip, lost gold-span coverage and was replaced by the clip note.
- Folding overlaps adds `merged_from` bytes that `on` avoids by dropping the overlap silently:
  measured as a packet-byte rise; `merged` stays opt-in.
- A flag value in another case or spelling changes output: every flag is read exactly and tested
  at `ON` and `yes`.

## Acceptance evidence

- `TestMatchTermsEqualsCountTermsFiltered`, `BenchmarkCountTermsLine`, `BenchmarkLineTokens`,
  `BenchmarkRoleTerms`, `BenchmarkScanWordsFile` (`internal/contextindex/termtable_match_test.go`,
  `termtable_alloc_bench_test.go`): CRK-V0-001..003.
- `TestContextOrderFuseRanks`, `TestContextOrderRecordRelationRanks` (CRK-V0-004),
  `TestContextOrderAdditive` (CRK-V0-005, CRK-V0-007), `TestContextOrderAcronymProse`
  (CRK-V0-006), `TestContextOrderDefaultBytes` (CRK-V0-007), in
  `internal/contextindex/context_order_test.go`.
- `TestContextSpanHitRange` (CRK-V0-009), `TestContextSpanFallbacks` (CRK-V0-010),
  `TestContextSpanMerge` and `TestMergeSpansPure` (CRK-V0-011), in
  `internal/contextindex/span_merge_test.go`; the existing `TestContextSpansDefaultBytes` keeps
  `on` and unset unchanged.
- CRK-V0-003, CRK-V0-008 and CRK-V0-012 readings: `docs/build-log/2026-11-01-ctx-u15.md`.

## Measurements

Recorded in `docs/build-log/2026-11-01-ctx-u15.md`. In short: RRF fails CRK-V0-008 (recall@20
.764026 to .759076 on `v2_trace2code`); additive fails it (recall@20 .504167 to .491667 on
`v2_comment2context`); the acronym rule alone loses no recall on any subset; `hits` keeps frozen
recall and gold-span coverage equal to `on` with about 2% more packet bytes; `merged` keeps
recall but raises packet bytes against both `on` and `hits`, so it fails V1-0116's byte criterion.
TCP-V0-050(b) and V1-2716's ContextBench rerun are NOT_RUN for want of their corpora.

## Rollback

Unset `CORVINT_CONTEXT_RRF`, `CORVINT_CONTEXT_ADDITIVE` and `CORVINT_CONTEXT_ACRONYMS`, and set
`CORVINT_CONTEXT_SPANS` to `on` or unset it; the default wire never changed and no state persists.
To remove the code, delete `internal/contextindex/context_order.go`, `span_merge.go` and their
tests, and restore `orderRows` to `corroborate` and the span fields. CRK-V0-001..002 roll back by
reverting `termtable.go`, `lookup.go` and `rolesummary.go`; their outputs are unchanged.
