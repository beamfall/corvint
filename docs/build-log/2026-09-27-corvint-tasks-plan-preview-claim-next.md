
## 2026-09-27 CAL-V0-008 and CAL-V0-014: plan preview and `claim --next` (S4)

`corvint-tasks plan preview` renders a `taskman-plan/0` object under `taskman-priority-first/0`
as a pure read, and `claim --next` claims the first `SELECTED` entry of the same plan inside the
lease transaction. The plan lives in `internal/tasks/transaction/plan.go`. The Tasks boundary bars
importing `internal/taskman`, so the planner mirrors TCP-00 §4.3 without reusing that code.

The plan's eligibility predicate is the claim's own: barrier, required budget fields, the ticket
view's blockers and unknowns, and retry exhaustion. A `SELECTED` entry is therefore never refused by
the claim that follows. `COVERAGE_UNKNOWN` is excluded, because an undeclared ticket claims
`WHOLE_REPOSITORY` instead. Only `OPEN` and `HELD` tickets are planned. Worker capacity is reported
but never decides, since an `external-agent` attempt holds zero workers. `claim --next` reaps every
expired lease before it plans, because the plan reads every reservation, and a colliding-only reap
would leave stale deferrals. The verb `CLAIM_NEXT` is part of the request digest preimage, so a
`claim --next` retry replays the same attempt and cannot be confused with a named claim.

`plan preview` on a store whose head generation is zero uses the empty reservation set. The
journal audit would otherwise refuse hand-written fixture intents that carry no afterimage.
`plan record` stays `NOT_RUN`, because this spec pins no plans.
