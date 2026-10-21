# V1-1322: a merged-forward lane binds against an unmoved BASE

Spike ticket V1-1322. A long-lived lane that merges `origin/main` and then runs dogfood with its
original BASE makes `ocm-prepare` stop at a bound. The merge pulled mainline's changes into
`BASE..HEAD`. Decisions are delegated to the orchestrator under decision 0548.

## Finding

The main point is covered by the open branch `claude/batch-1021c`, commit `d6f12df67`
(V1-1463/V1-1927, requirement DCW-V0-110). Its `docs/DOGFOOD.md` batch-check section says a lane
that merged mainline forward uses the mainline commit it merged as `BASE`. That commit is also the
lane's merge base with mainline. Against the original base, the check fails `unbound-commits` and
names the merge and its merged parent as the `BASE` to rerun with. So the batch check already gives
the corrected BASE.

Two of the ticket's acceptance points are still open:

- **AC2.** When `ocm-prepare` stops, it does not name the bound that fired: path count, hunk count
  or byte size.
- **AC3.** `dogfood-change`, which runs at the start of a change, does not yet refuse early with
  the corrected BASE. Only the later batch check names it.

## Proposed recommendation

1. Take the documented rule from `claude/batch-1021c` as the answer.
2. File one follow-up build for AC2 and AC3:
   - `ocm-prepare` names the bound and its limit when it stops.
   - `dogfood-change` refuses early, with the corrected BASE, when `BASE..HEAD` contains a merge
     whose first parent descends from BASE and whose second parent is on `origin/main`.

   Refuse rather than recompute BASE silently. That keeps the operator's BASE authoritative
   (invariant 3) and makes the fix visible in the transcript.
