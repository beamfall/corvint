# Decision 0018 — change mode summarises coverage per changed path

Date: 2026-09-01. Status: accepted. Authority: repository owner, verbatim instruction "do 1-5.
you can make the correct choice on claim shape and summon an expert if needed" (2026-09-01),
given in reply to the recommendation "Rule on the packet's claim shape, then build the coverage
summary. Item 1 showed the mutation rows carry per-test signal but `failed_results` misreports a
covered change as 79 failures. The fix is a per-changed-path summary in `proof.affected`
(FPK-V0-017 amendment, needs a decision record since it changes wire semantics)." The claim-shape
choice made under that delegation: affected-test rows keep their per-test verdicts, claim wording
("affected test for C"), and `detail` (FPK-V0-005 unchanged); coverage is aggregated per changed
path.

## Scope

The instruction amends `FPK-V0-017` in `docs/specs/falsifiable-packet-v0.md`:

1. `proof.affected.paths`, sorted by path, carries one entry per changed path `C` that at least
   one affected-test row claims: `path`, `covered_by` (sorted test paths whose row is `PASS`),
   `reached_not_covering` (the `FAIL` rows for `C`), `not_run` (the `NOT_RUN` rows for `C`).
   `graph_digest`, `scope`, `selected`, and `unknown` stay.
2. `proven_results`, `failed_results`, and `unproven_results` count affected-test rows per path,
   once each: proven when `covered_by` is non-empty, failed when it is empty and a `FAIL` row
   exists, unproven when it is empty with no `FAIL` row and a `NOT_RUN` row. Packet rows keep
   per-row counting; `proof.counts` stays per row, so the reader still sees every `FAIL` under
   `test-kills-mutant`.
3. `state` follows the new summary counts through the existing `proveState`; no new state.
4. Row order is plan order, which since AFP-V0-007 puts the changed unit's own tests first.
5. Each row's claim wording and `detail` are unchanged.

Rationale: the falsifier answers "does this test pin the change" per test, which is the
evidence; the packet's question is "is the change covered", which is per path. Conflating them
turned one covered eight-line hunk into seventy-nine failed claims and buried the one PASS
(`benchmarks/results/prove-change-77d4d62-first-run.json`). Reachability remains fully disclosed
in the rows and in `counts`.

## Explicit exclusions

No change to the row vocabulary, the falsifier assignment, the mutation runner, the affected-plan
selector, or `prove-observe` (which recounts `proof.counts` per row); no coverage claim beyond the
tests the selector reached, so a `scope` of `UNKNOWN` still forbids reading absent rows as no
coverage; no promotion, cutover, tagging, signing, publication, or release gate.

## Consequences

`affectedPaths` folds the rows per path and `summarizeProof` counts each path once; the same
change also bounds the three Git streams change mode reads (`boundedOutput`) at bound+1 bytes
before rejection, with the error codes and messages unchanged.
