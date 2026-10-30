# Decision 0208 — contextindex retrieval, packet, and budget hypotheses settled

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-13).

Four unconfirmed hypotheses from the 2026-09-13 bug hunt (`docs/agent-memory/ideas.md`) were judged
against the accepted `GPK-V0-*` clauses and the retired Python oracle at `54735d98^`.

1. **Relevance floor before packet budget — not a defect; ordering now stated.** Reachable: a
   budget can drop the only row resting on two query words and ship one-word rows as `BUDGETED`.
   The oracle has no floor (`DR-0008`), so no parity bytes decide it. `GPK-V0-039` now states that
   the floor judges the ranking's emitted packet before `GPK-V0-040`'s budget narrowing. Judging
   after the budget would withdraw an answerable task as `below-relevance-floor`, an abstention
   reason the receipt never measured, while the budgeted packet already names its cause
   (`BUDGETED`, `omitted_results`, the budget omission line). Rejected alternative: making the
   supporting row budget-critical, which changes budget selection with no oracle or clause behind
   it. Pinned by `TestEvalQueryRelevanceFloorPrecedesPacketBudget`.
2. **`BUDGETED` overwriting the ranked state — oracle parity, not a defect.** The oracle computes
   the same precedence (`src/context_corvint_learning.py` lines 522-525 at `54735d98^`), no clause orders
   `NEEDS_WIDENING` above `BUDGETED`, and `GPK-V0-045` keeps `abstention.reason` in the compacted
   packet. No change.
3. **`readTemplate` substitution bodies — a defect under `GPK-V0-027`(c); repaired.** A `}` or
   backtick inside a string literal, or a `}` closing an object literal, inside `${...}` moved the
   template boundary: a nested template's `import("./ghost")` became an edge, and `` `${"`"}` ``
   swallowed a later live import behind a false `WEB_SOURCE_UNPARSED`. The clause makes every byte
   of a template literal, substitutions included, a non-specifier. `skipSubstitution` now tokenises
   the body as code. `analyzerSchemaID` moves to `corvint-analyzer/49` (renumbered at merge). Pinned by
   `TestWebImportsLexesSubstitutionBodiesAsCode`. `GPK-V0-027`(c) now states the substitution rule.
4. **`.ts`/`.tsx` siblings sharing one extensionless reverse-import target — oracle parity, not a
   defect.** The oracle strips the web extension before comparing (`src/context_corvint.py` lines 749-758
   at `54735d98^`), and `GPK-V0-027`(c) names no extension-precedence rule. Choosing TypeScript's
   resolution order would be a rule whose only authority is candidate behaviour (`GPK-V0-033`). No
   change.

Rollback: revert this decision's commit. That restores the byte-scanned substitution body and
`corvint-analyzer/47`, removes the two spec sentences and both tests, and restores the ideas entry.
