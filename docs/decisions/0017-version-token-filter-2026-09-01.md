# Decision 0017 — a `vN` task token narrows symbols only where some path carries a version term

Date: 2026-09-01. Status: accepted. Authority: repository owner, verbatim instruction "do 1 and 2
in parallel" (2026-09-01), given in reply to this recommendation: "Fix abstention, the axis the
bet says it can win. Nine of the twelve abstentions on the abstention release come from the `vN`
version-token filter, a recorded bug. Fix it, then build the answerability signal the ideas
backlog describes, since the relevance floor cannot separate no-gold from positives. Rerun the
abstention release and publish selective-success numbers, whatever they are."

## Scope

The instruction accepts one amendment to `GPK-V0-043` in
`docs/specs/go-production-kernel-migration-v0.md`, the clause that owns `EvalQuery` symbol
ranking: a task's `v[0-9]+` tokens narrow the symbol universe only when at least one symbol path in
that universe carries some `vN` term; when no path carries any, the tokens are ignored by the
narrowing. Before this decision both runtimes skipped every symbol whose path terms contained none
of the task's version tokens, so an issue text carrying `v18` or `v20` from a "Versions" section
emptied the universe of any repository without versioned paths and the packet abstained with
`no-relevant-candidates`: 9 of Corvint's 12 abstentions on the `v2_abstention` release and all 3 on
`v2_code2test` came from this filter rather than from a relevance judgement, and 0 of the 70
answered no-gold tasks carried a `vN` token (probe over all 188 samples, 2026-09-01).

The rule is implemented in Go only. The Python oracle (`src/context_corvint.py`, the symbol loop's
`version_terms` skip) keeps its filter, so the two runtimes are expected to disagree on a
versioned task against an unversioned tree; that is a `python-defect` under `GPK-V0-033`, `src/`
is deliberately NOT repaired, and the disagreement is registered as `DR-0015` in
`conformance/divergence-register.md` and discriminated by the parity case `query-version-token`
(fixture `query-version-token`: one Go symbol, no versioned path, task
`enforce session revocation v18`; Go answers with the symbol, the pinned oracle abstains).

## Explicit exclusions

This decision does not change how a version token narrows where a versioned path exists: with any
`vN` path term in the universe, every symbol whose path carries none of the task's tokens is still
skipped, exactly as the oracle does. It does not add an answerability signal, alter the
`GPK-V0-039` relevance floor, or change record, document, marker, or learned-history ranking. It
does not touch `harness event --event user-prompt` beyond the shared `EvalQuery` path it already
uses. It does not change any promotion, cutover, merging, tagging, signing, publication, or release
gate.

## Consequences

`evalRankSymbols` drops the task's version tokens when `evalAnyVersionedPath` finds no `vN` path
term among the prepared symbols. The bench abstention arm's version-token abstentions become
answers or relevance-floor withdrawals on their own merits; the conformance corpus gains its fifth
`knownDivergence` row.
