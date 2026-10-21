# V1-0953: the next stage of a staged binding begins in the same clone (LCP-V0-030)

Date: 2026-10-20. Ticket: V1-0953. Base: origin/main 5cf75daf0.

## Problem

A batch over the hunk or obligation cap is bound in stages: bind, finish and seal `BASE..M1`, then
enroll `M1-seal..M2`. In one clone, the stage-2 `corvint dogfood begin` refused
`prior-completion-stale` (`internal/localcompletion/lifecycle.go`, `Begin`): the stage-1 enrollment
is satisfied but bound to its old target, so it stops evaluating as satisfied once `HEAD` moves. The
refusal named no next action, and the only non-destructive route was a fresh clone per stage.

## Decision (delegated in-ticket fork)

Of the two acceptable outcomes in the ticket, `begin` now admits the next stage, under a narrow
test: the stale satisfied enrollment must carry a terminal record with check exit 0 over its report
set, a review acknowledging that same report set, and a bound target that is an ancestor of, or
equal to, the new plan's base. The enrollment is then archived as a completed generation, as for a
currently satisfied one. Every other stale satisfied enrollment, and any failed ancestry probe,
keeps the refusal with no local write; the CLI message now names the staged route. The
cross-session `worktree-prior-completion-stale` refusal is unchanged, because another session's
enrollment is not this session's completed stage.

Rejected: admitting any stale satisfied enrollment (it would hide a completion invalidated by a
rewrite), and adding a sealed-terminal state (decision 0534 does not adopt it).

## Evidence

- `TestBeginNextStageAfterSealedStage`: the stage-2 begin archives the stage-1 generation and enrolls
  the new base; a side-branch target, a failed final check, an unreviewed report set and a missing
  terminal record still refuse with byte-identical state. With the supersession test disabled, the
  admitting subtest fails with `prior-completion-stale`.
- `TestDogfoodBeginPriorCompletionStaleNamesTheRoute`: the refusal keeps its code and names
  LCP-V0-030.
- `docs/DOGFOOD.md` (Enrolled local completion policy) describes the route.

Rollback: remove `supersededByNextStage` from `Begin`; archived generations stay readable.
