# Subjectless lexical counterpart admission — V1-0431

Base: `b4fb66a6252ca94c53031cf627ee9da79bc6e0c1`.
Owning intent: TCP-V0-004, TCP-V0-011, TCP-V0-014 and TCP-V0-015 in
`docs/specs/task-context-packet-v0.md`.

## Finding and decision

A full `context --task 'record response status when flushing' --limit 8` fixture
returned two matching sources, only the first source's test, and five unrelated
lexical tests. The command regression fails on the base and passes with this change.
The existing BM25 order was correct: score 300 is the public lexical relation weight,
not the lexical match strength. No scoring formula or corpus golden was changed.

After ranking and authority reservation, the subjectless packet admits at most three
additional naming counterparts of selected lexical sources. It replaces the weakest
unrelated lexical tests, retaining every selected source and authority row. Selected
lexical counterparts behind unrelated tests can be promoted within the same cap;
existing stronger relations remain unchanged. Candidate and withheld reporting names
the pair generator that actually ran. The multi-signal test slot remains capped at one.
Opt-in named-test frames retain their own candidate policy.

The analyzer schema advances conservatively from 90 to 91 because its existing audit
pins all contextindex production source, including consumer-only changes. There is no
new extraction table or wire field. Rollback reverts this admission pass and its TCP
amendment, retaining failed qualification evidence.

## Verification and remaining qualification

The full-command failing/passing regression and focused context/lexical/slot-weight
units pass. Tests cover source retention, root authority, limits 1/2/4/8/12/30,
the three-pair cap, existing-counterpart promotion, truthful unexamined coverage,
and a stronger lexical match whose path sorts after the weaker one. Requirement
locators are regenerated from the staged owning spec.

Frozen retrieval evaluations, the held-out cobra qualification, independent review,
and final CEM/OCM/dogfood closure remain pending. The coordinator explicitly deferred
expensive final checks until V1-0411 finishes its already-planned changes to the same
compiler; this is not ticket completion. Cobra cases were not opened. Exhaustive
repository gates are NOT_RUN under the owner's scoped-work instruction.

## Dogfood and retained evidence

Pre-change query and impact receipts were retained in the worktree's private Git
`corvint` directory. The initial no-diff dogfood attempt retained its NOT_PRODUCED
reasons. The keyed local completion enrollment remains active. `affected` ran before
focused tests; its mechanical recommendation of the whole repository gate is retained
without overriding the owner's explicit scoped-check policy.

Private reproduction, regression logs, measurement, plan and enrollment receipt:
`/private/tmp/corvint-bugfix-20260928/v1-0431/`. The scratch command runner's interruption
regression proves its owned descendant is removed. No token-saving claim is made;
billed tokens and paired baseline are NOT_OBSERVED. Applicable self-development routes:
query/impact and affected used; final CEM/OCM/frontier/frozen evaluation deferred;
learning, external providers, hosted qualification, and release publication not applicable.
