# Decision 0163 — Corvint Witness Report V0 gains a read-only requirement

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/agent-memory/fixes.md` (2026-09-12) found that `cmd/corvint/no_mutation_test.go`'s
`TestCLIReadVerbsLeaveTheRepositoryByteIdentical` asserts `corvint witness` leaves the repository
byte-identical (AGENTS.md invariant 4), but no spec owns that guarantee. Decision 0162 resolved the
sibling `frontier` half of the same entry by adding `CF-V0-033` to `change-frontier-v0.md`, which
already owned `frontier`. `witness` has no such owner: `cmd/corvint/witness.go` calls
`internal/witness` (the unwitnessed-change-surface report, `Profile = "corvint-witness/0"`), not
`internal/changewitness` (the `ocm-change-witnessed-v0` evaluator `change-witness-relation-v0.md`
owns, whose own Traceability section says "no Frontier profile, verifier, or command consumes it").
Adding a `CWR-V0-0xx` requirement there and citing the CLI test's `witness` subtest would attribute
that test to a package it does not exercise. `internal/witness` also has no entry in
`docs/specs/go-production-kernel-migration-v0.md`: `parseWitnessInvocation`'s own comment states it
intercepts `witness` ahead of the shared parser "so the report adds no verb to the parity-compared
top-level command vocabulary" `GPK-V0-007` enumerates, so folding `witness` into that requirement's
verb list would misattribute a verb the migration deliberately excludes from its parity set.

The owner call: neither existing spec is the right owner. `docs/specs/corvint-witness-v0.md` is a
new, minimal contract scoped to the `corvint witness` CLI report and its read-only guarantee only.
It adds `AGW-V0-001`, worded like the equivalent read-only clauses in `change-frontier-v0.md` and
`go-production-kernel-migration-v0.md`, stating that `corvint witness` writes no repository,
`.git`, or `.corvint` state on any path, and traces it to the existing CLI test's `witness` subtest.
This states behavior the shipped `cmd/corvint/witness.go` and `internal/witness` already have (no
write call exists in either); it asserts no new behavior and changes no runtime bytes.

Consequences: a new `docs/specs/corvint-witness-v0.md` with `AGW-V0-001`, a `docs/specs/README.md`
row, and a `docs/specs/INDEX.json` entry. `docs/agent-memory/fixes.md`'s entry is fully closed:
both the `frontier` half (decision 0162) and this `witness` half now have owning requirements. No
production code changes.

Rollback: revert this decision's commit. That removes `corvint-witness-v0.md`, its README row and
INDEX entry, and restores the fixes.md entry's `witness` half.
