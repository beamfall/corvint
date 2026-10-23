# The capability catalog says `requiredFlags` is partial

Date: 2026-10-23. Requirements `CAP-V0-022` and `CAP-V0-023` in
`docs/specs/capability-catalog-v0.md` (owner request, 2026-10-23); `CAP-V0-024` to `CAP-V0-029`
are reserved and unassigned.

## Problem

`internal/capcatalog` derives `requiredFlags` only from `Usage:` synopses. An option that a help
text calls required only in prose is missed, and so is a form whose usage lines are alternatives.
The only notice was one clause in the `derivation` prose. A consumer that read an absent form, or
`{}`, as "no option required" was wrong. For example, at `77a49dfa` `migrate-traces` has
`requiredFlags: {}`, yet its help says "Exactly one of --dry-run and --apply is required".

## Decision

Take the smallest fail-closed fix, and never guess from prose.

- Structured source: the only code-declared required-option table is `corpusOperations` for
  `docs corpus`. It already generates those forms' usage lines (`DCP-V1-056`), so deriving from it
  changes no byte. The other commands check required options imperatively after `flag.FlagSet`
  parsing, and declare no table.
- So the catalog states the gap. A new top-level member, `requiredFlagsCoverage: "partial"`,
  is encoded between `derivation` and `globalFlags`, with a derivation entry that says to treat
  `requiredFlags` as a lower bound. The value is closed. It can change only when every form is
  derived from a structured source.
- The schema stays `corvint-capability-catalog/0`: the member is additive, as `profiling` was.

## Evidence

`TestCAPV0022RequiredFlagsCoverageIsPartial`, `TestCAPV0023ProseRequirementsAreNotDerived` and
`TestCAPV0022BinaryCatalogMarksRequiredFlagsPartial`. `catalog.json` was regenerated with
`make capability-catalog`. `make capability-catalog-check` now also runs the 022 and 023 tests.

## Limits

The marker covers the whole catalog, not each form. The incomplete forms are not enumerated,
because that would need the prose reading this change refuses.
