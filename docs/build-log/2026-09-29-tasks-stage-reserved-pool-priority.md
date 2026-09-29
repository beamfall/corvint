# Stage-reserved pool member priority

Issue 371 and native ticket V1-0488 clarify CAL-V0-029 and CAL-V0-034: a claim with a stage first
considers free members reserved for that exact stage, then free unreserved members. Each tier keeps
the policy member order. A busy, preparing, quarantined, or health-failing matching member does not
remove unreserved fallback capacity; a different-stage reservation remains excluded. Claims without
a stage continue to admit only unreserved members.

The allocator, health-probe path, and read-only preview capacity use one ordered eligibility helper.
Focused transaction tests cover tier order, other-stage exclusion, no-stage compatibility, and
preview capacity. Store tests cover exact reserved selection, busy fallback, and health-failure
quarantine followed by unreserved fallback. Repository-wide `make gate` is NOT_RUN under the scoped
issue policy; final CEM binding, independent review, and native completion are retained by the task
coordinator.
