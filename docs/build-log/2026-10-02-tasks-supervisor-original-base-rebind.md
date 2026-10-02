# Supervised Codex evidence rebound against the original base

Date: 2026-10-02. Owner intent: PR #456 / issue #354, native V1-0475.
Scope: repair the combined change-evidence binding while preserving every existing
executable, test, spec, generated index and historical build-log byte.

## Binding correction

The original change starts at `ff3da727e95a1999cbc4a58a471cdd498693f902`.
The first source commit and its repair remain in ancestry. The historical repair CEM
bound only `ba9d75c4ce461929ba234aa06990ccf8232c085f` through
`45e100dcbd5ef1c7f183df9d7e2a9d90d6a66a0b`. Its passing report describes that
narrower range; it does not close the original-base combined change.

Four ordinary inverse commits remove the latest seal, its binding, the first seal
and its binding, in that order. Each changes CEM artifacts only. This avoids two
reverted seals colliding at the shared CEM path and preserves both original source
commits without a history rewrite. The old maps remain recoverable in Git ancestry
and are copied to the repair evidence directory. A fresh CEM must bind the combined
diff against the original base, using inspected base spans rather than old hunk
ordinals. Rollback is another reviewed ordinary commit; do not erase the old reports
or substitute a later base to hide the original-base refusal.

## Enrollment, chronology and limits

This is a distinct repair enrollment in its own clone, begun before the inverse
commits under native attempt `attempt:corvint:main:55fd3300aa3bfc9024e079aa6206c667`
(generation 115, admission receipt 1881). The old enrollment key was not recovered.
The new enrollment neither resumes nor satisfies any old enrollment. Original
pre-change query and impact chronology remains `NOT_OBSERVED`; the new receipts
were collected at the prior sealed head and cannot establish original-base context.
Historical local outcome trace `dae606aa1f246c35b07c44b718cc6fef3b9fc63335583c10a25c6c4cbebb232f`
is retained in the author clone. It is not an enrollment key or a new repair result.

The owning scope remains `docs/specs/corvint-tasks-agent-leases-v0.md`.
CAL-V0-059 and CAL-V0-060 were added after the original base; neither ID is in that
base's declared requirements. Do not fabricate original-base OCM links, retrofit
historical context, or treat passing unit tests as complete requirement coverage.
The earlier report has zero of 60 requirements linked. Fresh OCM and frontier
reports must retain their actual denominator, unassessed requirements, static
coverage and open obligations. Structural CEM closure does not close those gaps.

## Selected verification and review boundary

The frozen plan selects the focused CAL-V0-059/060 units in `internal/tasks/intent`
and `internal/tasks/store`, the `internal/specindex` tests, and vet for those three
packages. It also selects all five focused documentation checks
(`spec-requirements-check`, `requirement-definitions-check`,
`traceability-tests-check`, `decision-numbers-check`, `line-citations-check`),
plus `go-format-check` and `eol-policy-check`. Checks run through the enrolled
workflow on the clean binding commit. No cross-commit check reuse is enabled.
Their actual receipts and independent review are retained with the repair; this
entry records the plan, not a prospective passing result.

Corvint query and path impact were used before this repair. Range `affected` is
used before selected verification. Its omitted paths, documentation reachability
and language-frontier unknowns remain visible; broad gate advice does not supersede
the owner's focused-test instruction. `make gate` is `NOT_RUN`. Optional mutation,
learning and unrelated feature routes are not applicable to this evidence repair.
CEM, OCM and frontier reports are inspected before the independent review boundary;
acknowledgment, final finish, seal and publication follow that review.

The preserved implementation remains a partial delivery of issue #354. Live Codex
qualification at non-low effort and a stage longer than one hour are `NOT_RUN`.
Applied provider effort is `NOT_OBSERVED`. Multi-repository programs, other host
supervisor adapters, per-role models and automatic continuation remain open.
V1-0475's full acceptance remains `PARTIAL`; this repair does not complete it.
