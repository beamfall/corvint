# GH-832: name the runner and `--timeout` value in timeout details

## Finding

`corvint-js-test-provider e2e --external-server` reported a Playwright command timeout as
`timeout: vitest did not complete within the bound`. The reason was correct, but the external path
classified its Playwright observation with the Vitest classifier (`unitBoundaryFailure`), whose
fixed detail names `vitest`. No timeout detail named the bound an operator should raise.

## Decision

`PWP-V0-020` (accepted under the owner's standing auto-accept (2026-10-11)): every timeout detail reads
`<runner> did not complete within the --timeout <value> bound`, naming `playwright test` for both
e2e paths and `vitest` for unit runs, with the effective bound (default included) in Go duration
form. The external path keeps the Vitest classifier's order and its cancelled case through
`externalBoundaryFailure`, because `e2eBoundaryFailure` has no cancelled case; swapping to it would
have changed cancelled external observations. Reason codes, classification order and cancellation
handling are unchanged. No golden or receipt fixture embedded the old detail strings.

## Evidence

`TestBoundaryTimeoutNamesRunnerAndFlag` pins the three wordings and that external classification
matches the Vitest classifier's reason (and non-timeout detail) for cancelled, overflow,
start-failed, wait-not-completed and clean observations. A live Playwright timeout run is `NOT_RUN`.
Rollback restores the two fixed details; retained receipts stay readable because the detail is free
text.
