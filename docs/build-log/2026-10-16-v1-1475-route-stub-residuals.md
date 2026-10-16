# 2026-10-16 V1-1475: request-stub residuals of V1-1458

Lane V1-1475, the residual of V1-1458 (PR #957). Requirements `LPCV-V0-076..079`
(`docs/specs/live-proof-carrying-verification-v0.md`), accepted under the owner's standing
auto-accept (2026-10-11). Decision 0534 item 5 (PR #972, open when this lane was cut) settles the
external profile question. Base 6f5272b5f22fcb779c755f338b0bc0a3d3184ba9.

## Dispositions

| Residual | Disposition |
|---|---|
| Routes on `browser.newContext()` contexts | Recorded (`LPCV-V0-076`). A worker-scoped `browser` fixture override adopts each context made by `newContext` or `newPage` while a test with the producer runs. |
| `page.routeWebSocket` | Recorded with `api` `routeWebSocket` (`LPCV-V0-076`). `hits` counts the sockets its handler received. |
| `routeFromHAR` | Recorded separately with `api` `routeFromHAR` (`LPCV-V0-076`). `update: true` registers no stub and stays a non-goal. |
| Producer-absent case | An explicit `NOT_PRODUCED` reason (`route-stub-producer-absent`, or `route-stub-external-profile` for a profiled receipt), only when asked with `--route-stubs` (`LPCV-V0-077`). |
| External profile revision | None for 1.0 (`LPCV-V0-078`, decision 0534). |

Control-path stubs (#872) stay out of scope.

## Decisions

- **The explicit reason is opt-in.** The acceptance requires the producer-absent reason without
  changing legacy bytes. Adding the member by default would change every JavaScript document, and
  CLI stderr must stay empty on success, so a stderr note was rejected. `--route-stubs` (Go
  `ProjectRouteStubs`) states the reason; the default `Project` is unchanged
  (`TestLPCVV0077RequestedMemberKeepsDefaultBytes`). `--discover` and the MCP bridge do not take
  the option; this is a recorded non-goal.
- **`routeFromHAR` is named by its call site, not by a separate override.** Playwright's HAR router
  registers through the instance `route` that is already instrumented. A `node_modules` frame
  naming `routeFromHAR` between the producer and the caller marks the record. The caller's file and
  line stay the registration site.
- **`newPage` keeps its own override.** Playwright 1.61.1's `Browser.newPage` calls
  `this.newContext`, so the override is redundant there. It is kept so a browser whose `newPage`
  creates its context directly is still recorded; a unit test pins that case.
- **`api` is closed.** It is omitted for a plain `route`. A present member must be
  `routeWebSocket` or `routeFromHAR`; empty, null and `route` are refused.
- **V8 `async ` frames.** Call sites awaited through Playwright's client can appear as
  `at async file:line:col`; the prefix is stripped before resolving the frame.

## Evidence

- Live run `internal/jstestprovider/testdata/route-stubs/live-1.61.1-residuals/`: Playwright
  1.61.1, Node v22.23.3, the installed chromium-headless-shell 1228, darwin arm64, 5/5 passed. No
  browser was downloaded. `TestLPCVV0079LiveResidualsParse` reads it. A rerun of the V1-1458 suite
  with the new producer gave records identical to the retained `live-1.61.1` ones.
- Mutations, each killed by the focused tests and reverted: `newContext` adoption, `newPage`
  adoption, `routeWebSocket` hit counting, HAR call-site detection, `async ` frame stripping, `api`
  allow-list, empty/null `api` refusal, the requested producer-absent member, the
  external-profile reason, the finding's `api`, and `--route-stubs` requiring `--receipt`.
- `NOT_RUN`: no qualified `PWP-V0-008` tuple, no Linux or Windows run, the TypeScript mirror, and
  `make gate` (not requested).

## Limits

Contexts made outside a running test (`beforeAll`, worker fixtures), by
`launchPersistentContext`, or on a browser the test launches itself are not recorded.

## Rollback

Revert this change. The `browser` override, the `api` member and `--route-stubs` go away. Retained
records that carry `api` are then refused as unknown-field input until they are regenerated.
