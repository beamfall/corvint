# 2026-10-18 V1-1888: report test files no configured runner selects

Ticket: V1-1888 (a test-looking file that no runner selects never runs, yet looks like coverage).
GitHub #1172.

## Decision

`TSA-V0-140` to `TSA-V0-146` are added to `docs/specs/test-source-analysis-v0.md`, the spec that
owns the committed test-source readings. They are accepted under the owner's standing auto-accept
of lane requirements (2026-10-11).

- `corvint tests unselected [--rev REV]` reads committed runner configuration and test-looking
  JS/TS files. It prints `corvint-test-file-selection/0`: the runners, the inventory of every
  test-looking file with its selection, and one `TEST-FILE-UNSELECTED` finding per unselected
  file. Each finding has one reason: `no-runner-for-framework`, `outside-include`, `excluded`
  (with the pattern) or `suffix-mismatch`.
- Four runners are modelled (Jest, Vitest, Playwright, Mocha) with their documented defaults and
  a literal-only configuration subset. A spread, a merge, a projects/workspace split, an
  unsupported key, a non-literal value, YAML, unparsable JSON or a pattern outside the glob dialect
  makes the runner uncertain, and its files abstain. Abstention is never a finding.
- A file's framework comes from its imports. Globals and fixture-imported tests are compatible with
  several runners. With unmodelled runner evidence (Cypress, Karma, ...), those files abstain
  rather than being reported.
- No runner evidence at all abstains (`no-runner-configuration`), so the existing validity fixtures
  without a package.json keep their verdicts.
- Witness credit: `corvint tests validity` now calls `testsource.SelectedSourceValidity`. That
  gives each test of a named unselected file an invalid `TEST-FILE-UNSELECTED` finding, so its
  hygiene is `INELIGIBLE` under TSA-V0-051 and it cannot witness a claim.
  - The wrapper leaves `SourceValidity` untouched, because peer lanes edit it.
  - The finding code lives in `internal/testsource`, not `internal/testvalidity`.
  - An unknown rule's hygiene reason is its rule ID, so no `testvalidity` change is needed.

## Evidence

Synthetic Git repositories in `internal/testsource/unselected_test.go` and
`cmd/corvint/tests_unselected_test.go`. They cover each reason, the named exclude pattern, selected
files not reported, every abstain reason, the glob dialect, the validity refusal and the CLI exit
codes 0, 1 and 2.

## Residuals

- The doc-corpus legacy crosswalk and the Playwright witness import read receipts, not this report,
  and do not yet consume the flag (NOT_PRODUCED here; owned by the doccorpus and witness lanes).
- RSpec, Go and unmodelled runners are not modelled. Their files are absent or abstain.
- No live qualification on a real suite (NOT_RUN).
- The spec header's intent line does not list V1-1888, which avoids conflicts with peer lanes
  editing the same lines. The section and its requirements carry the ticket.
