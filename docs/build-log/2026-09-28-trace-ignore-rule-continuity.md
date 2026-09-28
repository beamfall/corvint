# Trace ignore-rule continuity (V1-0458)

GPK-V0-050 already admits tracked root and nested `.gitignore` in local trace records without
indexing them. The recorder used that authority, but current-tree reads and migration seeded their
history caches from indexed sources alone. Consequently a successful finish could poison the next
query; an empty commit retained the failure because the cache is keyed by tree. A different-tree
ancestor happened to work through the existing Git inventory path.

The repair extracts the existing admission rule into `internal/tracerepopaths` and shares it across
recording, the dogfood stability check, reading and migration. Historical inventory, secret screening,
path validation, dirty-tree checks and ranking are unchanged. There is no trace schema migration.
Rollback is a code revert; no stored trace is rewritten by the repair.

Independent plan review rejected an adapter-to-adapter import because same-package cross-adapter
tests would form a Go import cycle. The neutral helper resolves that finding. Regression-first
execution failed for root/nested rules in both Git object formats at HEAD and a same-tree ancestor;
different-tree historical reads passed. The legacy migration regression failed on the same omission.
The repaired adapter regressions pass, including snapshot-backed observed reads, non-indexing,
legacy dry-run nonmutation, archive-byte retention and migrated-record readback.

A separately built candidate replays the original preserved minimal fixture with the unchanged
`ignore rules` query: one local trace is loaded, migration dry-run succeeds, and every trace-store
file hash remains unchanged. The compiled CLI regression exercises real finish, its committed CEM,
the unchanged query, then change/check/seal on the same checkout. Its initial run failed at query
with the original `unsupported-query-trace-state` error; the repaired sequence passes in 18.423s.
Independent code review found no code defects and requested explicit invalid-record recovery
guidance, now included in the agent route. Frozen validation and final evidence binding remain
pending at this source checkpoint; this entry is not release promotion.

Corvint context, impact and affected routes were used. Context omitted four ranked results; impact
omitted nineteen, including five named direct callers. The affected plan does not prove exhaustive
coverage. The initial daily coordinator retains missing pre-change query-receipt and missing-input
notes; the actual initial context/impact, failure logs and original trace replay are retained under
`/tmp/corvint-1-0-orchestrator-20260928`. No stored trace or task was removed or reworded to pass.
Frozen learning/ranking evaluation is not applicable: admission is restored to the existing accepted
path set, with no new ranking or learning behavior. Full repository validation remains at the final
1.0 release boundary. Production integration and native ticket completion are separate closeout steps.
