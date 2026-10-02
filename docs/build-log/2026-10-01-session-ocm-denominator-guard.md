# Session handoff OCM denominator guard

Date: 2026-10-01
Owner intent: native V1-0521; governing spec `docs/specs/session-context-dividend-v0.md`.

## Finding

Before `dc0b832a`, `SESSION-V0-017..019` sat after the `## Requirements` section and
`SESSION-V0-020..022` before it. `requirementsFromBlob` selects exactly one level-2 Requirements
section, so the session OCM counted 16 of the 22 registered clauses. `dc0b832a` moved both slices
into level-3 subsections of the one section without renaming any ID; it carried no regression test.

## Decision

`TestSessionContextDividendSpecEnumeratesEveryObligation` (`internal/lrfrepo`) reads the committed
spec through the OCM enumerator and requires exactly `SESSION-V0-001..022`, and requires the
`REQUIREMENTS.tsv` rows located in that spec to be the same 22 IDs. Against the pre-repair spec blob
(`dc0b832a^`) it fails, listing 16 obligations; on the repaired spec it passes.

A scratch scan of all 160 registered specs found 79 registered IDs in eight other specs lying outside
their `## Requirements` section (for example `cem-0.2-canonical-binding.md`, 32). Those may be
deliberate and are out of scope here; they are reported to the queue owner, not changed.

Rollback: delete the test and the traceability paragraph; no runtime behavior changes.
