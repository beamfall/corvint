# 2026-10-15 V1-1458: a passing test whose request stub never matched

Lane V1-1458 (GitHub #948). Requirements `LPCV-V0-072..075`
(`docs/specs/live-proof-carrying-verification-v0.md`), accepted under the owner's standing
auto-accept (2026-10-11). Base f81114713c96cf629462dfd9e91e7d370ee848c4.

## Intent

A Playwright test can pass while a `page.route` or `context.route` stub it registered never ran. A
mistyped glob, a moved endpoint or a late registration sends the request to the real backend, and
the green run is credited to a stub that did nothing. This slice adds an optional producer that
records every registered stub with its hit count and registration line. The test-validity document
then names each never-matched stub of a passing test. Missing producer output stays `NOT_PRODUCED`
and is never read as zero hits.

## Decisions

- **The consumer lives in the existing test-validity document.** `corvint test-validity --receipt`,
  `--discover` and the MCP bridge all project through `internal/testvaliditydoc`. The finding is an
  optional per-test `routeStubs` member there, not a new subsystem. It is not an axis and never
  changes the five-axis projection (`TestLPCVV0073FindingLeavesAxesUnchanged`).
- **The producer follows the behavior-events pattern.** `route-stubs.cjs` is an
  `extend(baseTest)` fixture module, as `internal/doccorpus/behavior-events.cjs` is. It overrides
  the `context` fixture, so a test that takes no browser fixture launches nothing and publishes no
  record. It instruments the instance `route`/`unroute` methods of the context and each of its
  pages, and restores them at teardown. Each handler is wrapped to count calls, and
  `unroute(url, handler)` still finds the wrappers.
- **The transport is one Playwright annotation.** The JSON reporter already carries
  `results[].annotations`, so `ParsePlaywrightJSON` reads one `corvint-route-stubs` annotation per
  attempt into `AttemptDetail.routeStubs`. Decoding is closed:
  - unknown fields and trailing data are refused;
  - more than 64 stubs is refused;
  - text over 1,024 bytes is refused;
  - a duplicate annotation is refused.

  Each refusal leaves the member absent. Pattern and file pass the product secret screen.
- **Unprofiled receipts only.** The `/0` to `/3` external profiles gain wire fields only through
  another profile revision. `qualifiedProfileShapeError` therefore refuses `routeStubs` on every
  profile (`external-profile-has-route-stubs`). The external path builds attempt details from
  `qualified-reporter.cjs`, not from `ParsePlaywrightJSON`, so no Corvint-produced profiled receipt
  can carry the member.
- **Legacy bytes.** The document member appears only when some attempt of the receipt carries a
  record, so a receipt without the producer projects byte for byte as before
  (`TestLPCVV0073ReceiptWithoutProducerKeepsBytes`). Once any record exists, every test states its
  own value: `RECORDED`, `NOT_PRODUCED` with `route-stub-record-absent` or
  `route-stub-record-invalid`, or `RECORDED` with `test-not-passed`.
- **The last attempt decides.** A hit in a failed first attempt says nothing about the attempt that
  passed (vector `last-attempt-decides`).
- **Go-only vectors.** `conformance/test-validity-v0/route-stubs.json` is a separate file, so the
  shared `vectors.json` and the TypeScript mirror are untouched. The path was added to the
  `internal/testvaliditydoc` read scope in `.corvint/test-read-scopes.json`.

## Failure modes

See the spec's request-stub failure-mode table. The main ones:
- an absent producer read as zero hits;
- a forged or duplicated annotation;
- an earlier attempt masking the passing one;
- a failing test flagged;
- a secret in a pattern;
- legacy byte drift.

## Acceptance evidence

- **Live run.** Playwright 1.61.1 with the already-installed chromium-headless-shell 1228 and Node
  v22.23.3; nothing was downloaded. It is retained unedited under
  `internal/jstestprovider/testdata/route-stubs/live-1.61.1/`. One stub was hit. A passing test
  carried two never-matched stubs, at lines 11 and 12. The test without a browser fixture published
  no record. `TestLPCVV0072LiveRecordParses` reads this run. It is diagnostic, not a `PWP-V0-008`
  qualified tuple.
- **CLI smoke.** `corvint test-validity --receipt` on the `zero-hit` vector printed both
  `stub-never-matched` findings with pattern, file and line.
- **Focused tests at BASE vs this change.** Run under host load about 62 to 75.

  | Package | BASE | This change |
  |---|---|---|
  | `internal/jstestprovider` | 332 pass, 7 skip | 338 pass, 7 skip, `-race` |
  | `internal/testvaliditydoc` | 94 pass | 105 pass, `-race` |
- **Mutations.** 14 of 14 were killed by the new tests, and 8 of 8 for the review fixes. One mutation was applied per behaviour:
  - producer: hit count, registration line, over-64 suppression;
  - parser: absent stays absent, duplicate refused, secret screen;
  - profiled refusal;
  - consumer: zero-hit finding, no finding when hit, absent `NOT_PRODUCED`, `test-not-passed`,
    last attempt, legacy byte omission, invalid record.

## Independent review

One Codex round (read-only, `codex exec -s read-only`) reviewed the first commit and reported
eight findings. All eight were fixed in a follow-up commit. Each fix has a regression test and one
mutation that the test kills:

1. A missing or null `hits` decoded as zero. Decoding is now presence-aware, through
   `RouteStubRecord.UnmarshalJSON`, on both the annotation path and the retained-receipt path.
2. A credential in an escaped RegExp pattern (`https:\/\/user:pass@`) passed the screen. The
   pattern is now also screened with its escapes removed.
3. Repeated JSON members were accepted. They are now refused at every level.
4. Teardown deleted own-property overrides installed by another fixture. They are now restored.
5. `unroute` missed registrations made before instrumentation. They are now removed as well.
6. A foreign annotation stayed trusted. The producer now adds its own record beside it, and the
   consumer refuses the duplicate.
7. A page resolving after teardown was instrumented. The session now closes first.
8. A non-string annotation description failed the whole report. Annotations now decode
   tolerantly.

## Limits and residuals

- The following are not recorded:
  - routes on contexts the test creates with `browser.newContext()`;
  - `routeWebSocket`.

  `routeFromHAR` is recorded only to the extent that it reaches the instrumented instance `route`;
  that is not separately qualified.
- Control-path stubs (GitHub #872) are out of scope.
- **Profiled receipts.** Carrying the record on `/0` to `/3` needs a new profile revision, which is
  an owner decision.
- **Producer adoption is opt-in.** A receipt where no attempt carries a record omits the member
  entirely. The spec defines that omission as `NOT_PRODUCED`; there is no per-test reason in that
  case.
- **Unrouted stubs.** A stub removed with `unroute` before any hit is still reported as never
  matched.

## Rollback

Remove the producer, the annotation parse and both `routeStubs` members. Decoding is closed, so a
retained receipt that carries the member is refused until it is regenerated without the producer.
