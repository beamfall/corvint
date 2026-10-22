# Test-validity JS fixes: Playwright 1.64 frame link, Vitest declaration placement, rootDir specs

Date: 2026-10-21. Lane: claude/test-validity-js-1021p (base ef2a01db5). Tickets: V1-2015, V1-2090,
V1-2091 delivered; V1-2185 not delivered. Requirements `LPCV-V0-290` to `LPCV-V0-292` in
`docs/specs/live-proof-carrying-verification-v0.md` are accepted under the owner's standing
auto-accept (2026-10-11).

## Problem

- V1-2015: on the Playwright 1.64.0 tuple, 7 of 12 step-negation live cases failed with
  `step-fault-underivable` (`LPCV-V0-128`). A trace retained from a 1.64.0 run shows why. The
  library `Frame.expect` event has the keys `callId`, `class`, `method`, `params`, `startTime`,
  `title` and `type`, and no `stepId`. Its `callId` equals the runner expect step's id. Step
  negation linked frames only by `stepId`, so every assertion had no frame. That made derivation
  read `expected-value-not-literal` (network) and `no-dom-fault` (DOM).
- V1-2090: a Vitest anchor is a failure frame, and an unprofiled Playwright outcome may have no
  anchor. Their `file` and `line` stayed absent (`anchor-not-declaration` or
  `not-recorded-by-provider`) even when a bound source reading had exactly one declaration with
  the test's full name.
- V1-2091: a Playwright spec path relative to a non-root `testDir` or `rootDir` already bound
  through the unique retained-key suffix rule in `Locate`. No test or requirement pinned that
  behaviour.

## Decisions

- `internal/stepnegation/trace.go`: link by `stepId`, falling back to `callId` when `stepId` is
  absent. A present `stepId` wins. A link that names no runner expect step attaches nothing.
- `internal/testvaliditydoc/js_source.go`: after the existing placements, `placeJSDeclarations`
  places an unplaced JS outcome from bound readings. It reads each local retained key at most once,
  sharing the source cache, and matches a literal title path joined by a space or by ` > ` to the
  full name. One match is placed, several give the new closed reason `declaration-ambiguous`, and
  none leaves the location unchanged. The projection's axes do not change.
- V1-2091 is delivered as a pinning test plus `LPCV-V0-292`. No code change was needed.
- V1-2185 is not delivered. Shrinking the allowlists means moving the E2E runner (`RunE2E`,
  external server, freshness and negate paths, with 4 production callers and about 37 test call
  sites) into its own package. Moving only the readiness probes would shrink the allowlists but
  would not meet "the E2E runner is in its own package". The ticket stays open for a dedicated lane.

## Evidence

- `TestLPCVV0290Playwright164FrameLink`, `TestLPCVV0291VitestDeclarationPlacedByTitle` and
  `TestLPCVV0292PlaywrightRootDirRelativeSpec` pass. The first two fail with their fix removed (the
  stray-link and stepId-precedence cases are included). The third pins existing behaviour.
- Live `TestNegateLiveDiagnostic`, qualified mode, on the 1.64.0 tuple: playwright-1.64
  node_modules from `/private/tmp/cem10-build/toolchains-2026-10-17`, Node v22.23.2 from the same
  directory, chromium headless shell revision 1248 from the user Playwright cache. With the fix:
  12 of 12 passed in 61.75 s (network-text 6.43 s, static-element 5.27, caught-assertion 5.90,
  pattern 1.98, soft 7.28, hard 5.27, collateral 12.75, third-party 5.37, failing-fixture 2.21,
  flaky 6.44, interruption 2.10, ambiguous 0.67). Control without the fix: 7 failed with
  `step-fault-underivable`, which reproduces V1-2015. The join stays gated (`LPCV-V0-129`).

## NOT_RUN

- A real Vitest run and a real Playwright run with a non-root `testDir` through the projection.
- The 1.63.0 tuple comparison: the toolchains' playwright node_modules has no `@playwright/test`
  1.63 package, so the 1.63 `stepId` shape is covered by the unit fixture only.
- Affected units outside the cap, and tier 2, are listed in the PR body.

## Rollback

Revert the lane's merge commit. 1.64.0 traces abstain again, Vitest and unprofiled locations return
to their earlier absences, and no retained format changes.
