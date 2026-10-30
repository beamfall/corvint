# Decision 0162 — Change Frontier V0 gains a numbered read-only requirement

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/agent-memory/fixes.md` (2026-09-12) found that
`cmd/corvint/no_mutation_test.go`'s `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` asserts
`corvint frontier` leaves the repository byte-identical (AGENTS.md invariant 4), but
`docs/specs/change-frontier-v0.md` — frozen 2026-08-23, amended only by decisions 0006 and 0012 —
had no numbered requirement stating that guarantee, unlike the sibling read verbs the same test
covers (`CKN-V0-007`, `TSS-V0-006`, `GPK-V0-007`, `GPK-V0-037`, `DSE-V0-001`, `MTV-V0-007`,
`SOL-V0-007`). The test already covers the behavior; only the spec-side requirement ID was missing.

The owner call: add `CF-V0-033`, worded like the equivalent read-only clauses above, stating that
`corvint frontier` and the library it wraps write no repository, `.git`, or `.corvint` state on any
path, and tracing it to the existing test's `frontier` subtest. This states behavior the shipped
`cmd/corvint/frontier.go` and `internal/frontier` already have (no write call exists in either);
it asserts no new behavior and changes no runtime bytes.

This decision resolves only the `change-frontier-v0.md` half of the fixes.md entry. Its
`change-witness-relation-v0.md` half does not resolve the same way: `cmd/corvint/witness.go` calls
`internal/witness` (the unwitnessed-change-surface report, `Profile = "corvint-witness/0"`), not
`internal/changewitness` (the `ocm-change-witnessed-v0` evaluator `change-witness-relation-v0.md`
owns, which its own Traceability section says "no Frontier profile, verifier, or command consumes").
Adding a `CWR-V0-0xx` requirement there and citing the CLI test's `witness` subtest would attribute
that test to a package it does not exercise. `docs/agent-memory/fixes.md` is narrowed rather than
closed to record this remaining gap.

Consequences: `docs/specs/change-frontier-v0.md` gains `CF-V0-033` in a new "Repository read-only
guarantee" subsection, its `Amendments` header line, and a new traceability row citing
`TestCLIReadVerbsLeaveTheRepositoryByteIdentical`'s `frontier` subtest
(`cmd/corvint/no_mutation_test.go`). No production code changes.

Rollback: revert this decision's commit. That removes `CF-V0-033`, the header amendment note, and
the traceability row, and restores the original fixes.md entry in full.
