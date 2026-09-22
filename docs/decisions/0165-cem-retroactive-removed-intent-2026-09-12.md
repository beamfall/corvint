# Decision 0165 — removed base intent is a pinned unknown detail, not CEM evidence

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/agent-memory/fixes.md` (2026-09-12) recorded that a retroactive binding of a fixing commit
cannot cite the backlog entry that commit removes, even when that entry was the only recorded
intent. `cem cite` reads the span from `baseRevision`, but its `CEM-CB-005` precheck and the
canonical target-drift check both classify a removed span as `stale` or `deleted` and refuse it
(`internal/cem/verify/verify.go:425-480@92b33007`), so `DOGFOOD-BIND-007` marks the hunk unknown
with a free-text detail that names nothing checkable. The entry's proposed fix was a `cem/0.3`
profile adding per-evidence `"scope":"base-only"`.

The owner call is the smallest resolution consistent with the frozen 0.2 profile:

1. `cem/0.2` is unchanged. Its six closed fields (`CEM-CB-002`), 0.1-parity drift rules
   (`CEM-CB-003`, decision 0098), relation and unknown-reason enumerations, and verifier behavior
   stay as they are. Removed intent is never admitted as map evidence, so a hunk whose only intent
   was removed stays `unknown` (invariant 2).
2. A `cite-span-not-stable` refusal classified `stale` or `deleted` now names the refused span as
   the detail token `removed-intent.<base blob OID>.<start>-<end>` (zero-based half-open bytes). The
   `ambiguous` refusal is unchanged because that intent still exists at the target. This amends
   `CEM-CB-005`'s refusal message only; the code and every wire byte are unchanged.
3. `DOGFOOD-BIND-007` records that token as the unknown row's detail. `script/dogfood-bind-range.sh`
   accepts a `removed-intent.` detail only when the OID is a regular (`100644`/`100755`) blob in
   BASE's tree and the span is nonempty and within its size; otherwise the plan fails
   `cem-mark invalid-unknown-plan`. The note is thereby pinned to immutable base content
   (invariant 1) without being evidence. The script does not re-check that the change removed the
   span; the producer's refusal is that classification.
4. The `cem/0.3` base-only-scope design is not adopted and its acceptance status does not change: it
   stays backlog under decision 0059 item 8. It would need every CEM consumer updated together
   (`ocm_read.go` dispatches exact `cem/0.2` else 0.1, OCM closes on any `supported` hunk, and
   frontier, TCQ, and LRF pin `cem/0.2`), which this call does not undertake.

Alternatives rejected: a new relation or unknown reason would change the frozen 0.1-parity
enumerations; relaxing target drift for base-pinned evidence would break `CEM-CB-003`; deferring
backlog removal to a later range cannot repair already-landed ranges, which are the retroactive case.

Consequences: `internal/cem/workflow/commands.go` adds the pin to the `stale`/`deleted` refusal
(`TestCiteRemovedIntentRefusalNamesBasePin`); `script/dogfood-bind-range.sh` validates the pin
(`script/dogfood-bind-range_test.sh`); `docs/specs/cem-0.2-canonical-binding.md` `CEM-CB-005` and its
traceability row, and `docs/DOGFOOD.md` `DOGFOOD-BIND-007`, are amended; the fixes.md entry is
removed.

Rollback: revert this decision's commit. The refusal message loses the pin, the script again accepts
any `[a-z0-9.-]` detail without a base check, and the fixes.md entry returns.
