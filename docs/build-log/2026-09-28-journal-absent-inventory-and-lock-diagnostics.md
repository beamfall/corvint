# Journal-absent inventory and completion lock diagnostics

The owner's explicit two-issue fix request accepts the narrow CTS-V0-002 amendment: four inventory
verbs can inspect validated current worktree intent when `.git/taskman` itself is absent. Stable
local edits are included, so the response does not claim immutable Git or journal authority.
The snapshot is null; journal history, sequence, generation, attempts and liveness remain
unobserved. Receipt audit, other reads, init and mutations retain their existing requirements.
V1-0310's broader missing task-store authority remains open.

A standalone clone can inspect published ticket bytes/status and compare its committed intent with
the published revision. It cannot reproduce the primary checkout's journal receipt audit,
`headSeq` or `projectionAgreement`. Existing incomplete/corrupt journals never trigger fallback;
intent drift is bounded to four attempts and journal appearance refuses the unaudited result.

LCP-V0-002 now distinguishes an existing final operation lock from permission denial and other
creation failures. Public codes remain bounded and path-free; Go error unwrapping preserves the
filesystem cause. Lock ownership and enrollment state are unchanged by a failed acquisition.

## Evidence and limits

Base: `decf36d6354a369c0afaa05b7c34ab1cf1e29c87`. Task scratch:
`/private/tmp/corvint-portability-lock-20260928`. `baseline.json` retains the original standalone
clone refusal and real EACCES/EEXIST collision. `red.log` and `lock-red.log` retain failing
regressions; `green.log` retains the first focused passing proof. CTS-V0-002 traceability names
four-verb/no-write, strict existing-journal and race regressions. LCP-V0-002 names real permission
and contention checks plus deterministic cause-preservation coverage (which does not depend on
unprivileged filesystem credentials).

Pre-execution independent review in `gate-a.md` passed without HIGH findings. Final selected
checks, independent review, committed clone proof, CEM/OCM binding, explicit local outcome and
native ticket completion remain separate closeout evidence; this entry does not claim them passed.
No repository-wide gate is claimed. The existing V1-0323/V1-0171 full-gate requirements require
explicit terminal disposition rather than an inference that focused checks are equivalent.

## Self-development routes

Used: pre-change query and impact (retained `prechange-query.json` and `prechange-impact.json`,
including omissions/refusals), context (`context.json`), affected selection (`affected.json`),
and explicit dogfood enrollment (`plan.json`, `begin.json`). The initial `dogfood-change` attempt
is retained in `initial.log`: CEM was NOT_PRODUCED (`git-diff-failed`), with OCM and outcome
prerequisites unresolved. These are pre-change observations, not final completion claims.
The same keyed enrollment must bind the final target and retain selected-check/report evidence.
Not applicable: learning/ranking evaluations, provider execution, web flows, mutation testing,
console/release promotion and external integration profiles; these fixes change inventory read
admission and local filesystem diagnostics only. No token, savings or external-adoption claim.

Rollback restores strict journal-backed inventory reads and the former lock diagnostic mapping;
no intent/journal migration or deletion is needed. Restoring the old mapping also restores its
misleading permission diagnosis.
