# 2026-10-18 V1-1737: anchoring rejection-only witnesses

Lane V1-1737 (GitHub #1099). Requirements `LPCV-V0-080..083`
(`docs/specs/live-proof-carrying-verification-v0.md`), accepted under the owner's standing
auto-accept (2026-10-11). Base 2ab0e74421147cb16d2da183419e1ad2d991fa52.

## Intent

A test whose every decisive assertion checks a rejection passes for the right reason only when the
same request can succeed for an allowed actor. When a route is broken or a fixture is missing, every
actor gets 403, the positive test fails and the rejection tests still pass. Those passes must not
be credited. The test-validity document now states, per test, whether a rejection-only pass is
`ANCHORED`, `UNANCHORED` (warning) or `ANCHOR-FAILED` (blocks credit), and states uncertainty when
the route or actor is unknown.

## Decisions

- **Declared evidence, not inferred.** Receipts carry no HTTP responses or assertion
  classification, and Corvint cannot observe either. The narrowest honest model is a closed,
  per-attempt `corvint-request-outcomes` annotation, written by the test or its fixture and treated
  as provider authority. It mirrors the request-stub pipeline (`LPCV-V0-072..079`): it is parsed
  into the attempt detail, works on unprofiled receipts only, and is refused on profiled receipts
  with `external-profile-has-request-outcomes`.
- **The consumer is a document member, not a sixth axis.** `rejectionAnchor` sits beside
  `routeStubs` on each test. It is present only when some attempt carries a record, so legacy
  documents keep their bytes. The five axes and the run projection never change.
  `blocksCredit: true` is the machine-readable "do not credit".
- **Uncertainty first.** No request gives `UNKNOWN anchor-route-evidence-missing`. No actor gives
  `UNKNOWN anchor-actor-evidence-missing`. Siblings match only on the same actor and an exact
  `(method, route)` key.
- **A failure outranks a success.** An observed failed positive sibling, or a declared failed one,
  gives `ANCHOR-FAILED` even when another test succeeded on the route. This over-blocks when the
  sibling failed for an unrelated reason. That is deliberate: a 403-for-everyone run and an
  unrelated failure cannot be told apart from the receipt.
- The record uses the generic closed-member check `irregularJSONMemberOf`, which is shared with the
  route-stub record.

## Evidence

- Focused tests: `./internal/jstestprovider` (`TestLPCVV0080*`, plus the `TestLPCVV0072*`,
  `TestLPCVV0076*` and `TestPWPV3*` regressions) and `./internal/testvaliditydoc` (`TestLPCVV008*`,
  plus the `TestLPCVV007*` regressions).
- The three acceptance criteria of #1099 map to `TestLPCVV0082PositiveSiblingFailedBlocksCredit`,
  `TestLPCVV0082PositiveSiblingPassedAnchors` and `TestLPCVV0082UnanchoredClearedByDeclaredSibling`.
- No live Playwright run is retained: `NOT_RUN`.

## Residuals

- There is no automatic producer: no fixture records responses and classifies decisive `expect`
  calls. Until one exists, the anchor covers only tests whose authors declare the record.
- UI-state and error-code rejections that make no request stay `UNKNOWN`.
- No Corvint consumer reads `blocksCredit` yet. The doccorpus behaviour witnesses use profiled
  receipts, which cannot carry the record without an external profile revision.
- The TypeScript mirror, Vitest and Go test sessions are not covered.
