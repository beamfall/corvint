# Decision 0501 — PGO-V0-009 Tasks pre-reap group kill import edge, accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("Accept ... the decision 0501 record for #749"), covering the decision 0397 addendum
"PGO-V0-009 addendum — Tasks pre-reap group kill".

## Context

Decision 0492 accepted `PGO-V0-009`. Its doctor-plugin and task-store clauses need the waitid
pre-reap step in `internal/groupreap`. Decision 0397 rule 2 refuses Core imports from
`internal/tasks/` except for named edges. The addendum to decision 0397 proposed one more exact edge
while the RC3 process-group lane (PR #749, V1-1036, V1-1047, V1-1056) was being repaired.

## Decision

The owner accepts the decision 0397 addendum as written:

1. Exactly three files may import `internal/groupreap`: `internal/tasks/cli/doctor.go`,
   `internal/tasks/store/gate_reap_waitid.go` and `internal/tasks/store/pool_process_unix.go`.
   The rest of `internal/tasks/cli` and `internal/tasks/store` stays refused, and these files admit
   no other Core package.
2. `TestImportViolationControls` keeps negative controls for `doctor_unix.go`,
   `gate_reap_nowaitid.go` and `lease_gate.go` importing `internal/groupreap`, and for another Core
   package imported from the admitted files. It keeps positive controls for the three admitted edges.
3. Rule 5 is unchanged: `internal/groupreap` is already in the source-archive subset.

## Rollback

Revert the PGO-V0-009 doctor and store callers and remove this three-file edge. The other edges
stay.
