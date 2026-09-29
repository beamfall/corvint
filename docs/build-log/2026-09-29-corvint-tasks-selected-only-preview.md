# Corvint Tasks selected-only plan preview

Issue 370 adds one opt-in projection of the existing CAL-V0-014 plan. `plan preview
--selected-only` returns the closed `taskman-plan-selected/0` profile with all selected ticket IDs,
their exact total, and `complete:true`. The ordinary detailed `taskman-plan/0` response and the
priority-first planner remain unchanged. Both forms are pure reads.

The retained regression fixture plans a 358-ticket queue with capacity for five tickets and checks
that the compact result contains those five IDs in plan order, reports total `5`, is explicitly
complete, has no detailed entries, and leaves both intent and state trees byte-identical.
