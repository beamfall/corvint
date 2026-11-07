# Ranking self-review repair: retain continued CI commands

Owner request: repair only the P1 command-inventory omission in
`script/batch-precheck.sh`, against base
`39c03198086681fb24f509fbd00b34696d3268c4`. This refines `ASD-V0-036` under the
owner's standing acceptance and the explicit self-review repair instruction.

A final literal block containing `make required-check` followed by a backslash
and an indented shell comment executes the make command in Bash. Removing the
comment before continuation processing left that command pending forever. A
separate ordinary step made the inventory nonempty, so sync checking accepted
an incomplete inventory.

Decision: retain comments and blank lines for ordinary command parsing; build a
separate comment/blank-normalized view solely for exact trusted-block matching.
A standalone comment cannot start a continuation. At a literal-block or
single-line command boundary, an unresolved continuation becomes an existing
unknown-command row and its pending state is cleared. The local parser remains
conservative: it refuses the reported comment form through the existing refusal
path rather than trying to interpret arbitrary shell syntax. No wire shape,
error code, exit code, diagnostic template, golden or assertion changes.

The expanded `script/batch-precheck_test.sh` covers the reported final comment
form beside ordinary steps, an unfinished block, an unfinished single-line run,
a continuation before another step, and a standalone backslash comment followed
by a required check. Existing trusted-block and normal-continuation assertions
remain intact. The owning requirement and acceptance table carry the same
boundary. Rollback: revert this parser repair, its fixtures and this requirement
clarification together; the omission would return.

An isolated probe executes the original parser functions at
`f507c6b6694ede9eb5cd7131ace519c105b91d59` and the repaired functions on the same
workflow. Before: exactly one make row (`ordinary-check`), no required-check row,
exit 0. After: the ordinary row plus an unknown row naming `required-check`,
exit 0 for inventory extraction; the sync checker refuses such unknown rows.
The two inventories and exact probe workflow are retained outside the repository
in `/home/russell/lanes/ranking-evidence/selfrepair-probe*`. The JSON result's
SHA-256 is `0c512aa92d54d93b6c33cf2faec823ba713996f8489871fb6306beb1cc97912d`.

Focused verification uses the complete existing shell fixture suite, the real
workflow sync check, shell syntax/Bash compatibility, staged spec generation and
line citations. Exact terminal results and raw log locations are retained in the
lane handback. The first fixture run reproduced the three new omissions, but an
in-flight script edit contaminated its later cases with a shell parse error;
that run is not final acceptance evidence. The fresh full suite uses fixed script
bytes for its entire run.

Applicable self-development routes: context, query, affected, impact and CEM/OCM
review/binding. Native context/query outputs are retained. The installed affected
binary reported stale source; the current lane binary was used again before
verification: UNKNOWN, 149 selected units and 82 unknowns (78 language frontiers,
four unowned dirty paths). Range impact refused the dirty repair tree as
`unsupported-impact-worktree`; the initial dogfood pass observed the existing
bound prechange query/impact receipts, retaining their original chronology.
Optional providers, LSP, fresh frozen ranking trials and live qualification are
inapplicable to this shell parser repair. No Go package changed in this round;
prior package results and their limits remain in the lane handback.

Independent review is assigned to the orchestrator. Automatic guarding-test
attribution remains NOT_PRODUCED under the lane's heavy-only test rule. Exhaustive
gates, push/PR publication, Tasks writes and sealing are excluded by the owner.
