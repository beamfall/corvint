# V1-1157: harness stop frontier carries a fixed cause (AHI-054, proposed)

Date: 2026-10-11. Ticket V1-1157 (GitHub #785). Status: proposed, pending owner acceptance.

## Decision recorded

The owner accepted, through the orchestrator on 2026-10-11, an additive optional reason field on
`corvint-harness-event/0` frontier entries with the profile staying `/0`, and declined any adopter
acknowledge or suppress flow that hides frontier items. `AHI-054` records both; the declined flow is
its non-goal.

## Design

- The `stop` frontier already has a `reason` member that names the degradation code
  (`frontier-authority-unavailable`). Reusing that key would change its meaning for existing
  consumers, so the new member is `cause`.
- The value is fixed, `no-accepted-closing-authority`. The event never consults a CEM, an OCM or
  `corvint frontier`, so a value such as "no verified CEM" would be invented certainty (product
  invariant 2). A committed `.corvint/change.cem.json` leaves the value unchanged.
- Only the `stop` event carries a frontier, so only it gains the member. The degradation list and
  `receiptId` are unchanged.
- The frozen `harness-stop` parity row records the 40 added bytes as `DR-0043`. The frontier lies
  outside the counted packet, so the runner applies the rewrite outside `packet_bytes`
  reconciliation, as it does the `DR-0038` identity rename.
- The Gemini CLI and OpenCode consumers read only `state` and `shouldContinue`; the test fixture core
  now emits the member, and their stop cases pass unchanged. A `legacy-frontier` fixture mode keeps
  a stop receipt without the member accepted; `cause` is optional on the wire. No consumer renders
  it (`AHI-021`, `AHI-022`).

## Evidence

`TestAHI054StopFrontierCarriesFixedCause`, `TestAHI054FrontierCauseDivergenceIsPinnedAndOutOfPacket`,
`TestGPKV0002ManifestReplay` and `TestHostAdapterJavaScriptHosts` (including the AHI-054 with-and-without-cause case, confirmed live by a deliberate failing run) pass on the change.

## Limits

A cause on the non-`stop` events and on `corvint-dogfood-event/0` is not delivered. The
`docs/AUTOMATION.md` session-hook guidance added on an unmerged branch (V1-1143) still says a
per-degradation reason is not implemented and needs updating when both land.
