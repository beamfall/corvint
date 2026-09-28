# Completion refusal fixture identity

PRs #302, #305, #309, #311 and #312 failed `TestCALV0017_CompletionRefusals` because
its dangling-commit fixture called `git commit-tree` without an author/committer identity.
The helper deliberately disables global and system Git configuration. A workstation can
supply a guessed identity; a CI runner need not. With `user.useConfigOnly=true`, the
unmodified test reproduces exit 128 locally.

The fixture now supplies its test-only identity and disables identity guessing. The Git
output helper retains stderr on failure. The CAL-V0-017 refusal assertions are unchanged;
all completion tests passed under the same hostile Git configuration after the fix.
Rollback restores the prior test fixture; no product or wire behavior changes.

Corvint query and tracked-path impact were used before editing; original receipts remain in
the private Git evidence directory. `affected` was used to select checks; any retained
language/frontier unknowns remain unresolved. The initial dogfood-change attempt at an
empty range reported `cem-prepare: git-diff-failed`, OCM not produced and missing outcome;
these are pre-change failures, not a passing gate. Final binding follows source commit.
The frozen selected check reruns completion tests on the final bound commit. Full `make gate`
is NOT_RUN under the owner's scoped-work policy. Independent review covers this repair
and the other open-PR CI repairs. No retrieval/ranking or learning change is made, so frozen
retrieval evaluations and optional providers are not applicable. Billed token/cost and a
paired baseline are NOT_OBSERVED; no savings claim is made.
