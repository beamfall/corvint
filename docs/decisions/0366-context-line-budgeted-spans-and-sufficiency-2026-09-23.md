# Decision 0366 — Line-budgeted span rows and an evidence-set sufficiency check in `context`

Date: 2026-09-23. Status: proposed, experimental (ticket V1-0098; TCP-V0-025..029). Adds an opt-in
packet member and coverage block to the task-context packet and sets no default.

## Context

A `context` packet names files, each with one evidence line. An agent then reads whole files, or
guesses the lines, and nothing in the packet says whether the selected evidence carries what the
task names. The ticket asks for span rows with explicit line ranges and a call-site reason under a
declared line budget, and a per-anchor sufficiency statement that never reports `satisfied` on
missing evidence.

The index already holds what this needs: `langsymbols.go` declarations with their lines, the
`Words` postings of whole identifiers, and the reverse-import edges. A span is therefore a
query-side projection of a result row, not new index content.

## Decision

1. Spans (`internal/contextindex/span_rank.go`) are computed after the result rows are final:
   core rows from the declarations of task names in result files (else a named line, else the
   densest task-term line, each widened to its enclosing declaration), then call-site windows in
   the sources that name each core symbol, under a fixed 240-line budget, eight core rows, three
   per file, two call sites per core and 80 lines per row. No index encoding changes, so
   `analyzerSchemaID` is not bumped.
2. Sufficiency (`internal/contextindex/sufficiency.go`) checks each task anchor (mentioned tracked
   path, TCP-V0-016 specific name) against the selected spans' own pinned lines: `satisfied`,
   `insufficient` (the index has evidence no span carries) or `unknown` (the index cannot tell).
   One non-satisfied anchor keeps the set from `satisfied`, and every such anchor is named.
3. Both are behind `CORVINT_CONTEXT_SPANS=on`, the TCP-V0-019/022 shape: unset or any other value
   leaves the packet byte-identical to the recipe golden (`TestContextSpansDefaultBytes`). The
   flag changes no `results` byte (`TestContextSpansBudgetAndBounds`).
4. Opt-in rather than default-on. The span rows beat the ±15-line control on every subset with
   gold spans, but their absolute core-span recall is low (0.034 to 0.133), each packet grows by a
   mean of 99 to 144 span lines, and `satisfied` was rarely right (1 of 41 on `v2_code2test`, 1 of
   22 on `v2_comment2context`, 0 of 2 on no-gold `v2_abstention` samples). The measured result
   below records the losing samples; the flag is not promoted to default-on.

## Measured result

Frozen evaluation, 2026-09-23: `tools/retrieval-bench --arms context --context-packets` over the
five `v2_*` releases, every sample (no `--max-samples`), flag set empty (off) and `on`, with
`corvint` built from this branch. Every sample's ranked list is identical off and on, as TCP-V0-027
requires. The flag-off `v2_code2test` run lost 3 samples to the 30-second Git index deadline under
host load (reading 0.278/0.390/0.502); retried flag-off, their rankings equal the flag-on run's, so
the row below holds for both modes. `v2_edit2ripple` lost the same 3 samples in both modes:

| Subset | n | recall@5 | recall@10 | recall@20 | errors |
| --- | --- | --- | --- | --- | --- |
| `v2_trace2code` | 101 | 0.401 | 0.508 | 0.794 | 0 |
| `v2_code2test` | 106 | 0.288 | 0.399 | 0.512 | 0 on; 3 off, retried |
| `v2_comment2context` | 80 | 0.256 | 0.344 | 0.504 | 0 |
| `v2_edit2ripple` | 58 | 0.356 | 0.504 | 0.624 | 3 |
| `v2_abstention` | 82 | no gold (abstained 0.171) | | | 0 |

Span metrics (TCP-V0-029), scored offline from the flag-on capture:

| Subset | core-span recall | ±15-line control | wins / losses | verdicts sat / insuff / unknown | sufficiency precision | full-coverage base rate | mean lines used |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `v2_trace2code` | 0.133 | 0.077 | 7 / 4 | 0 / 33 / 68 | undefined (0 satisfied) | 0.109 | 122.6 |
| `v2_code2test` | 0.034 | 0.007 | 7 / 2 | 41 / 49 / 16 | 0.024 (1 of 41) | 0.009 | 134.5 |
| `v2_comment2context` | 0.091 | 0.016 | 11 / 1 | 22 / 42 / 16 | 0.045 (1 of 22) | 0.038 | 129.2 |
| `v2_edit2ripple` | not scorable (file-level gold only) | | | 1 / 53 / 1 (3 errors) | not scorable; the one `satisfied` sample's rows do include its gold file | | 144.1 |
| `v2_abstention` | no gold | | | 2 / 38 / 42 | 0.0 (0 of 2; a `satisfied` verdict on a no-gold sample is incorrect) | 0.0 | 98.8 |

Losing cases under TCP-V0-029: span recall below the control on `v2_trace2code` (4 samples),
`v2_comment2context` (1) and `v2_code2test` (2). No subset's sufficiency precision falls below
its base rate, but `satisfied` is still mostly wrong: 40 of 41 on `v2_code2test` and 21 of 22 on
`v2_comment2context` miss a gold span, and two no-gold `v2_abstention` samples read `satisfied`
on generic anchors (`TargetClosedError`; `mock`, `patch`). `satisfied` means the task's own
anchors are in the selected lines; this measurement shows that is not evidence the gold lines are.

## Consequences

Agents that opt in get exact line ranges to read and an explicit statement of which task anchors
the selected lines do not carry; the default product is byte-identical. A call site is a
whole-word use, not a resolved call, and `satisfied` means the task's own anchors are present in
the selected lines, not that the task is answered.

## Rollback

Unset `CORVINT_CONTEXT_SPANS`, or delete `span_rank.go`, `sufficiency.go`, their tests and the
`attachSpans` call in `TaskContext`. No state persists and the default wire never changed.
