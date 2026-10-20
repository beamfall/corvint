# Shared exact runtime tuple table for receipts and freshness

Date: 2026-10-20
Human intent: GitHub #1270, V1-2009. Owner decision (2026-10-20): keep decision 0448, admit Node
patch releases one exact release at a time, and fix the clash with one shared exact-tuple table.
Contract: `docs/specs/playwright-external-provider-v0.md` `PWP-V0-039`, `PWP-V0-040` (and the
`PWP-V0-034` refusal document, which gains the optional `runtimeTuple` member).
Base: `f3ca72248bb693534f006181301889bf797cc073`.

## Decision

- Receipts (`ReceiptRuntimeTuple`, the bundled and system per-test predicates) and per-test
  freshness (`freshRowUnknown`, the `RunFreshE2E` runner check) now read one table of exact
  `(profile, runner, Node, browser source)` rows in `runtime_tuple.go`. The freshness browser
  identity's Node must equal the receipt's Node instead of repeating a literal.
- The rows are the tuples that already had live evidence; classification is unchanged for every
  receipt. `1.60.0` stays a legacy branch outside the table because it has no Node/browser binding.
- A tuple refusal names the first failing field (runner, Node, browser source) and the sorted
  admitted values for it. The external version refusal and a new pre-run
  `freshness-runtime-tuple-unqualified` refusal carry it in the refusal document; the step-negation
  tuple refusal appends the same text. The browser source is only observable after a run, so it is
  named by `AdmittedRuntimeTupleMismatch`, not a pre-run refusal.

## Admitted and unadmitted

- Admitted (unchanged, all previously live-qualified): 1.63.0/v22.23.2 bundled `/0`..`/3`, system
  `/0`..`/2`; 1.63.0/v24.11.1 bundled `/0`; 1.64.0/v22.23.2 bundled `/0`..`/3`, system `/0`;
  1.64.0/v24.11.1 bundled `/0`, `/3`; freshness/0 on 1.63.0/v22.23.3 bundled.
- Unadmitted: freshness on 1.64.0 (no live freshness run; `RunFreshE2E` previously refused any
  runner other than 1.63.0, so none could exist), freshness on Node v22.23.2 or v24.11.1 (no live
  freshness run), and every Node major/minor range (decision 0448). Consequently no single Node
  release yet admits both an external receipt and a freshness receipt; the table now makes that
  visible in one place, and the refusal names it. Adding a freshness row needs its own live
  `TestPTFV0*` run on that tuple.

## Evidence

- `TestPWPV0039ReceiptsAndFreshnessAdmitFromOneTable` failed with base `freshness.go` restored
  (`freshness did not admit from the shared table: UNKNOWN`) and passes with the change.
- `TestPWPV0040TupleRefusalNamesFieldAndAdmittedValues` failed with base `runner.go` restored
  (`freshness runner refusal untyped: freshness-config-unsupported`) and passes with the change.
- The two exact-byte `PWP-V0-034` version-refusal expectations were updated for the new member.

## NOT_RUN

- Live Playwright runs: `NOT_RUN`, because no tuple was admitted and receipt classification did not
  change.
