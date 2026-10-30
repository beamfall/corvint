# Decision 0091 — `baseSpan` in the `cem status` drift row is a go-defect

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

Commit `31fd2ca0` added a `baseSpan` member to every drift row of the canonical `cem status`
envelope so a reader could tell a moved span from an unmoved span that the frozen classifier still
calls `relocated`. The frozen Python-oracle expectation for parity case `cem-status-bound` never
carried that member, so manifest replay failed on a stdout digest mismatch.

The divergence adjudicates as `go-defect`, and the member is removed:

- No spec determines the drift-row member set. `CEM-PILOT-004` (`docs/specs/cem-pilot-kit.md`)
  names what `cem status` reports without enumerating drift-row fields, so the oracle contradicts
  no clause and the divergence cannot be a `python-defect`.
- `GPK-V0-002` makes a field addition a wire change owned by its existing spec, not a migration
  exception, and `GPK-V0-003` forbids the migration from creating a `go` wire variant. A member only
  the Go kernel emits is exactly that variant.
- Every case in `conformance/cli-parity-v0/manifest.json` has `support: "parity"`, so there is no
  non-parity surface to move the member onto.
- The expectation cannot be re-authored from the Go binary: `GPK-V0-033` forbids transcribing
  expectations from candidate code, and the manifest closure test refuses `candidateUsed: true`.

The reader gap is real and stays open as a fix. `baseSpan` may return only after `CEM-PILOT-004`
is amended to require the recorded span in each drift row, and a spec-authored expectation replaces
the frozen capture under the migration contract.

Rollback: revert this decision's commit, which restores the member and the parity failure.
