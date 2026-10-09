# V1-0759: re-import keeps requiresPool and requiredRoles

Intent: a re-import of a changed foreign block must not silently drop the `requiresPool` and
`requiredRoles` keys that a `REFINE` set on a shadow `IMPORT` record (ticket V1-0759).

Findings:

- Confirmed, not only suspected. `REFINE` (operator role) on an `IMPORT` record sets both keys:
  the IMPORTER restriction only narrows the importer role, and `checkRecord` only requires the pool
  to exist in policy. `importer.record` built the next revision from `TicketKeys` alone, so a
  re-import with a changed block wrote both keys as absent. The new store test reproduced it
  before the fix (`requiresPool: "db" -> null`).
- Fix: `importer.record` copies both keys from the previous revision before `chain()` compares the
  acceptance-relevant fields, as it already keeps `executionPrerequisites`, `escalations` and
  `createdAt`. Proposed requirement `CTS-V0-007` in `docs/specs/corvint-tasks-store-init-v0.md`.
- The remaining optional record keys cannot reach an `IMPORT` record: `ATTACH_EVIDENCE`, know-how,
  external reviews and operator notes refuse a non-native (or shadow) record
  (`internal/tasks/mutation/apply.go`, `know_how.go`, `external_review.go`, `operator_note.go`).
- A changed block always changes `source`, which is acceptance-relevant, so the next revision bumps
  `acceptanceRevision` exactly once in any case; the test checks that one bump and the byte-identical
  keys. The removal was therefore a silent data loss more than an extra bump.

Verification: `TestCTSV0007_ReimportKeepsRefinedPoolAndRoles` (`internal/tasks/store`) fails before
and passes after the change; `go test ./internal/tasks/importer ./internal/tasks/store`, `go vet`,
gofmt and the spec doc gates.

NOT_RUN: `make gate`, full `go test ./...`, dogfood CEM binding (lane rules).

Rollback: remove the carry-over loop in `importer.record`; no store migration.
