# V1-0953: the same-clone staged `begin` is reverted pending a binding design

Date: 2026-10-21. Lane: claude/batch-dogfood-flow (base 5cf75daf0). This entry reverts e56d9808b,
the change that added `supersededByNextStage` (proposed as `LCP-V0-030`, which is withdrawn
unshipped).

Independent review (Codex, one round) found that the supersession test compared stored digest
strings and ancestry only. If a check log or report was corrupted or removed after a successful
finish, a `begin` for another plan whose base equals the prior target still archived that
invalidated completion as complete, even with no HEAD movement.

A sound test has to re-validate the retained evidence at the prior target and admit only the
expected seal transition. That is a binding-semantics design this lane cannot decide
conservatively:

- `make dogfood-seal` renames the bound `.corvint/change.cem.json`, so the binding artifact is no
  longer at its recorded path.
- Later stage commits may change the intent files.
- The next stage's own runs rewrite the untracked report and OCM files.

The fail-closed `prior-completion-stale` refusal is therefore restored unchanged, and V1-0953 stays
open. The workaround remains a fresh worktree or clone per stage.

Rollback: re-apply e56d9808b together with evidence re-validation.
