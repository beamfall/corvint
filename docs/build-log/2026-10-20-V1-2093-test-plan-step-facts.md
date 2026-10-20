# 2026-10-20 V1-2093: step-local preconditions for `test-plan consolidate`

Ticket: V1-2093 (GitHub #1320). `corvint test-plan consolidate` had one precondition channel,
`requires`. A step that installs and removes its own stub (HTTP 429, a network failure, a mocked
clock) had three wrong encodings: `requires` (each value opened its own compatible set and read
`conflicting-state`), nothing (an undeclared effect), or a fake element ID in `action`.

## Decision

`TCN-V0-013` to `TCN-V0-015` are added to `docs/specs/test-consolidation-planner-v0.md`. They are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11).

- `TCN-V0-013`: an optional `step_facts` set per variation, with the `requires` fact syntax and
  bounds. It never creates a requires conflict or a precedence edge. It is part of the canonical
  input, so only `input_digest` (and the header and `table_digest` that carry it) changes.
- `TCN-V0-014`: one key in both `requires` and `step_facts` of one variation refuses
  `test-plan-invalid-input`.
- `TCN-V0-015`: step facts are part of the TCN-V0-005 duplicate key.

Absent and empty `step_facts` are the same declaration and canonicalize identically. Inputs
without step facts therefore keep their digests, and the committed golden plans are unchanged.

## Choices left open by the issue

- The issue also suggested that an omitted `step_facts` should be a missing anchor. That would
  abstain every existing input, so the member stays optional. An omitted stub is labelled
  fail-open in the spec, in the same class as an omitted `changes` fact.
- A key in both `changes` and `step_facts` is not refused. The issue named `requires` only.
- The page-state view (ANL-V0-012) does not read step facts. It is governed by its own spec and is
  unchanged here.

## Out of scope

The issue's related limitation, one `--root`/`--revision` pair shared by `--map` and `--tests`
(a proposed `--tests-root`/`--tests-revision`), is not taken in this slice.

## Evidence

Focused `internal/testplan` tests: `TestStepFactsNeverConflictOrOrder`, `TestStepFactsDecoding`,
`TestStepFactKeyAlsoRequiredRefuses`, `TestStepFactsInDuplicateKey`, and
`TestInputOrderDoesNotChangeOutput` with step facts permuted. The existing worked-example and
console-qualification goldens pass unchanged. Removing step facts from the duplicate key makes
`TestStepFactsInDuplicateKey` fail.
