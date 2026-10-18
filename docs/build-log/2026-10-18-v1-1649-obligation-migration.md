# V1-1649: obligation ledger migration blockers

Owner input: [issue 1071](https://github.com/beamfall/corvint/issues/1071). Migrating a real
4,562-row per-ticket ledger hit five blockers:

- the 64-byte alias cap, whose refusal did not name the limit;
- plan findings for witnessed regression rows and optional non-core rows;
- `BLOCKED` and `DEFECT` rows counted as open for planning;
- no state for a partly witnessed row;
- no witness provenance.

TOL-V0-055..059 were added to `docs/specs/corvint-tasks-obligation-ledger-v0.md`; no CAL-V0 ID was
added. They are accepted under the owner's standing auto-accept of lane requirements (2026-10-11).
Delivery stays experimental. The highest TOL ID on `origin/main` was TOL-V0-054, and the only open
PR touching the spec (#1085) used IDs up to TOL-V0-034, so there was no collision.

## Design

- **Alias limit (TOL-V0-055).** `ObligationsMaxAliasBytes` goes from 64 to 128. The refusal states
  the alias's length and the `1..128 bytes` limit. Neither the grammar nor the event profile
  changes.
- **Plan reruns (TOL-V0-056).** `rerun` is an optional per-test member, so a plan without it keeps
  its exact bytes and output.
  - With `rerun`, a test may list no new obligations. One test may not name an id in both lists.
  - A `WITNESSED` rerun id is reported in `reruns`. Any other state is `RERUN_NOT_WITNESSED`, which
    is a failing finding with its own `preflight --plan` remedy.
- **Plan rows (TOL-V0-057).** `UNASSIGNED` now applies only to core `OPEN` and `PARTIAL` rows.
  `SPLIT` still applies to any pending row listed under two tests, so a listed `BLOCKED` or `DEFECT`
  row cannot be split silently.
  - This changes the observable TOL-V0-020 result for non-core rows. The two tests that pinned the
    old behaviour (`TestTOLV0020_PlanCheck`, `TestTOLV0026_PreflightPlanCheck`) were updated.
    TOL-V0-020's text carries a pointer to the amendment.
- **`PARTIAL` state (TOL-V0-058).** `PARTIAL` is a sixth state, set only through `set` with a
  reason. One helper, `ticket.ObligationPending`, now holds the pending set
  (`OPEN|PARTIAL|DEFECT|BLOCKED`). The report classifier, the source check, the review check and the
  run-batch causes all use it instead of their own three-state lists. `Counts` already counted only
  `WITNESSED` as witnessed, so `PARTIAL` raises `total` alone.
- **Witness provenance (TOL-V0-059).** Provenance is an optional closed object on the witness
  payload, copied to each credited entry's evidence.
  - Its members are optional, but it must name at least one, so an empty object cannot hide in the
    bytes.
  - It is screened with the existing secret screen.
  - It is never checked against Git or the report. It is caller-supplied context, not evidence, so
    the TOL-V0-013 recomputation and the audit are unchanged.

## Findings retained

- The shared secret screen treats three dot-joined runs of 10 or more bytes as token-shaped (its
  JWT rule). The fixture's first 78-byte alias,
  `CHECKOUT-REGRESSION.PAYMENT-FLOW.SAVED-CARD.…`, was refused `SECRET_DETECTED`, and real dotted
  adopter ids can be too.
  - The screen is shared and out of scope here. The fixture uses dashes, and the spec records the
    limit as a failure-mode row.
  - Filed for the orchestrator as a residual question.
- Old binaries refuse the new members fail-closed (TOL-V0-034 precedent). Whether CAL-V0-131
  requires a `/1` profile for these optional members stays the open owner question already recorded
  for aliases and sources.

## Not delivered here

V1-1648 (GitHub #1070) was not started in this lane and has no requirement IDs here. It covers:

- an operator qualification verb with a store-local version allow-list;
- a declared separate test repository for witness and audit.

## Evidence

The focused tests in `internal/tasks/cli` are `TestTOLV0055_AliasLimit128`,
`TestTOLV0056_0057_PlanRerunAndNonCore`, `TestTOLV0058_PartialNeverWitnessed` and
`TestTOLV0059_WitnessProvenance`. The existing TOL suite and `TestCALV0131` pass. The adopter's
end-to-end migration is `NOT_RUN`.

## Review fix: empty provenance flags (Codex P2)

The independent review found that an explicitly empty provenance flag, such as
`--test-commit "$UNSET_VAR"`, was dropped as an absent member, so the witness wrote without it.

- The witness verb now tracks which flags were given. Every given provenance flag is validated, an
  empty value included, and refuses `MALFORMED` before any write.
- An empty OID fails the verb's OID check. Empty prose fails the writer's 1-byte minimum.
- TOL-V0-059 keeps its ID; its text and failure-mode table are amended under the owner's standing
  auto-accept (2026-10-11).
- `TestTOLV0059_WitnessProvenance` covers an empty value on each of the four flags. It failed
  before the fix and passes after it.
