# Issue 835: behavior annotation contract, producer and missing annotations

Date: 2026-10-12. Base: `eb53998e009756488d4006dbd3d6876d44260ec5`. Branch:
`claude/issue-835-behavior-events`. Scope: GitHub #835 batched with ticket V1-1222. Requirements
`DCP-V1-080..082` are in `docs/specs/documentation-corpus-v1.md`.

## Intent

`witness-playwright` (`DCP-V1-049..052`) credits a test only when its passing attempt published
behavior annotations. Until now, nothing documented those annotations, nothing produced them, and a
zero-witness result did not say what was missing. Issue 835 asks for three things: the annotation
contract in the help and the guide, a producer that cannot forge a passing assertion, and per-test
naming of the missing annotation types. V1-1222 asks for a guide walkthrough from the receipt
producer to the witness, with a tested sample REQUEST.json.

## Decisions

- **Help.** The contract is one help constant, `behaviorAnnotationHelp`. Both `corvint docs --help`
  and the `witness-playwright` topic print it; `discover-playwright` does not, because it never reads
  annotations. A test checks the event keys against the `BehaviorEvent` JSON tags by reflection, so
  the help cannot drift from the wire.
- **Producer shape.** The producer is a CommonJS Playwright fixture,
  `internal/doccorpus/behavior-events.cjs`. It is not an `expect` wrapper that callers construct.
  The fixture owns a closure-private recorder and exposes a frozen API with three members: `expect`,
  `negativeControl` and `cleanup`. It resolves `expect` itself from the project's
  `playwright/test`, so a caller cannot inject a matcher that always passes.
- **Assertions.** An assertion is recorded after the genuine matcher resolves, and only if
  `testInfo.errors` did not grow and the session is still open. Recording happens only for a closed
  table of exact matchers. A RegExp, an array, any option except `timeout`, `.not`, a frame
  locator, or a locator on an unobserved page records nothing and warns once. In those cases the
  tuple would not be the literal registered tuple, so the producer fails closed rather than guess.
- **Page events.** Page events come from Playwright's own `framenavigated` events for main-frame
  http(s) commits in the `context` fixture. The `id` is the URL path. A path repeated on the same
  page is recorded once. A page with an observed opener is recorded as `popup`, and a redirected
  navigation request as `redirect`. The producer never emits `frame` or `setup`.
- **Foreign annotations and overflow.** If foreign `corvint-behavior-*` annotations are present, or
  there are more than 4096 events, the producer publishes one non-JSON event instead of the events.
  The importer then reports `event-malformed`, never a witness.
- **Cleanup.** The cleanup outcome is published only when callbacks were registered. With no
  callbacks the test stays `cleanup-unobserved`, because an empty cleanup is not evidence that
  cleanup ran.
- **`missing_annotations`.** The field is additive and `omitempty`, so existing bundles keep their
  bytes. It is set only on annotation-stage reasons and lists the never-published types in the
  closed order `events`, `cleanup`, `fixture`, `role`. Admission, reasons, details and the witness
  wire are unchanged.
- **Guide test.** The guide's REQUEST.json sample is closed-decoded by a test. Its receipt anchor
  metadata is overlaid onto a real request, which then imports a witness. Placeholders follow the
  existing `PROVIDER_MEMBER` precedent of `DCP-V1-045`.

## Evidence

- Live run: Playwright 1.61.1, Node v22.23.3, chromium-headless-shell 1228, darwin arm64.
  Playwright was taken from an existing local install; nothing was downloaded. The run exited 0
  with 3 passed. The output is retained unedited in
  `internal/doccorpus/testdata/behavior-events/live-1.61.1/`.
- The "lists items" annotations equal the committed fixture report's annotations, and they import
  to exactly one witness.
- The redirect-and-popup test decodes to the following events:
  - `page:redirect`
  - `page:main-frame`
  - `page:popup`, with parent `page-1`
  - `assertion`

  With no cleanup registered, it reports `cleanup-unobserved` and `missing_annotations`
  `["cleanup"]`.
- The test of inexact and frame assertions recorded only its page event.
- Node unit tests (`TestDCPV1081ProducerNodeUnit`) cover the following:
  - matcher failure
  - inexact tuples
  - impostor locators
  - the frozen API
  - page tracking
  - a missing `_opener`
  - cleanup order and failure
  - foreign annotations and overflow
  - a matcher that resolves after close

## Limits

- The importer cannot tell producer annotations from annotations a test pushes by hand.
  Annotations pushed after the producer's teardown, for example by a later fixture, are not
  detected. An author who replaces Playwright objects, or passes a fake `test`, is out of scope.
- The producer reads the private Playwright fields Locator `_frame` and `_selector` and Page
  `_opener`. These are qualified live on 1.61.1 only. The committed witness fixtures use 1.63.0
  reports, and the producer has not been run on 1.63.
- Only the `context` fixture is observed.
- Page ids carry the path only, never the query or fragment, but a path may itself carry a secret.
- `pushState` commits count as `main-frame`.
- Fixture and role names are self-declared through test options, not verified against the
  application.
- Warnings are emitted once per key per worker.

## NOT_RUN

- External utility qualification on an adopter repository remains `NOT_OBSERVED`.
- No live run on Playwright 1.63.
- `make gate` was not run, under the owner's scoped-issue preference.
- `corvint affected` selected affected units that were not run, beyond the focused packages and doc
  gates listed in the handoff. Its scope is `UNKNOWN` because of the language-frontier unknowns.

## Rollback

Revert the change commits. The `missing_annotations` field is additive and `omitempty`, and no
consumer or golden depends on it. Removing the producer and the help text has no wire effect.
