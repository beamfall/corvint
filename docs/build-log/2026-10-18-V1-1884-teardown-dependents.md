# V1-1884: teardown dependents (`TEARDOWN-DEPENDENT-LEFTOVER`, issue 1168)

Ticket V1-1884, GitHub issue 1168. Requirements TSA-V0-110..119 in
[test-source-analysis-v0](../specs/test-source-analysis-v0.md), accepted under the owner's
standing auto-accept of lane requirements (2026-10-11). This is not a new owner decision; it
applies that standing acceptance to this lane's requirements.

## Decisions

- **The graph is declared, never guessed.** Which calls create and delete a resource kind and
  which actions produce a dependent come only from a committed, closed
  `corvint-side-effect-declaration/0` file: versioned, pinned to a 40- or 64-hex revision of the
  system under test, with unknown fields and trailing data refused (`tests-declaration-invalid`).
  Deriving the graph from the system under test's source, schema or migrations is the named
  residual; the report states `graphSource: "declaration"`.
- **Incomplete means advisory.** A resource whose graph has a gap (an undeclared action, an
  unassigned create, an unread `beforeAll`/`around`/`:all` hook calling a declared name, an
  undeclared teardown call before the parent delete, an unknown teardown order, an uncertain file)
  keeps every finding `advisory` with closed reasons. Only `declared` findings exit 1. The verdict
  set is `leftover`, `unknown` and `no-leftover-declared`; nothing is ever called safe.
- **Order.** Same-hook and same-body order is line order; `finally`/`ensure` runs before every
  hook; an inner group's hook runs before an outer one. Two hooks of one group are treated as
  unknown order even where the framework defines it, which only weakens a finding to advisory.
- **Receipt join.** Only a JS provider receipt binds: exactly one `testFileDigests` key naming the
  file and a digest equal to the committed blob's sha256. Rows must be anchored at the parent
  delete line under a unique literal title. A DELETE 409/422 is attached with the dependent kind
  when the resource has exactly one; a recorded conflict with no static finding becomes an advisory
  `recorded-teardown-conflict` finding. Ruby documents abstain (`receipt-not-js`).
- **Reuse.** JS/TS reading reuses the TSA-V0-010 lexer and declaration model; RSpec reading reuses
  the logical-line Ruby helpers of the actor-auth reader, not `validity_rspec.go`, which an open
  peer change edits.

## Evidence

Focused tests, synthetic fixtures only: `internal/testsource/teardown_dependents_test.go` (8 tests:
declaration refusals, JS and RSpec phases and order, leftover/parent-first/dependent-first/
cascade/inert in Playwright and RSpec, each incompleteness reason, the receipt join and its four
abstentions, framework routing and bounds) and `cmd/corvint/tests_teardown_test.go`
(`TestTestsTeardownCLI`). Live qualification on a real suite: NOT_RUN.

## Residuals

Graph derivation from the system under test; instance-level identity (one dependent delete clears
every action of its kind); helper files, shared contexts, fixture finalizers and `beforeAll`
resources; kinds taken from a receiver (`record.destroy`) or factory argument (`create(:order)`);
Ruby receipt join and request-route-to-kind mapping.
