# V1-1581: test reference check (`flows appmap refs`)

Owner input: [issue 1025](https://github.com/beamfall/corvint/issues/1025). The owner asked for a
static, read-only check that reports routes, selectors and mocked endpoint segments that tests
reference but the application source lacks. AMAP-V0-090..099 were added to
`docs/specs/application-map-v0.md` and are accepted under the owner's standing auto-accept of lane
requirements (2026-10-11). Delivery is experimental.

Design decisions:

- The check is an appmap verb, not a new `tests` command. It reads the test files already
  recorded in an `application-map/0` file (blob-checked at the map revision) and reuses the
  AMAP-V0-007 lexer, the V1-1134 dotted-call scan shape and the AMAP-V0-003 URL rules. Routes use
  the map's resolved screens. A new `urlPath` helper was extracted from `urlMatches` without
  changing behavior. The lexer token gains a `regex` member that keeps the literal's source.
- Selectors and endpoints are name-level only: the declared `--source` directories are tokenised
  into `[A-Za-z0-9_-]` runs at the map revision. Every report carries `check: "name-level"` and a
  fixed `limits` sentence.
- Templated, concatenated, regular-expression and glob values are prefixes. Three false-positive
  classes are labelled and do not fail: catch-all routes, regular-expression values, and names a
  source prefix ending in `-`/`_` suggests the application builds. Dynamic values, text-level
  locators, XPath and secret-shaped values are counted as unchecked with a reason.
- An unlabelled miss exits 1 after the report is written (the stats `gateFailure` path). Refusals
  reuse the owned `appmap-invalid-query`, `appmap-invalid-revision`, `appmap-budget-too-small` and
  `appmap-bound-exceeded` codes, so no error code was added.

Live qualification (`corvint flows appmap build` then `refs`, both run with a binary built from
this branch):

- AMAP-V0 fixture `internal/appmap/testdata/fixture`, committed to a scratch repository, with
  `--source app`. Exit 1, 1235 bytes against the default cap of 6144. 9 test files read, 9
  references, 7 checked, 2 unchecked (`text-level-selector`), 2 unknown screens, 3 unlabelled
  misses: `nav-teesheets`, `slot` and `slot-status`. The fixture's `app/` holds only the router,
  so these test ids are real absences. Every route, including the templated ``/#!/clubs/${...}``
  prefixes, matched a screen. Afterwards `git status --porcelain --ignored` was empty.
- `internal/appmap/testdata/refs` with `--source src --budget 1024`. Exit 1, document within 1024
  bytes, `omitted: {misses: 5, unchecked: 3}`. With `--full` it showed 30 references, 26 checked,
  4 unchecked, 3 unlabelled misses (`invoices`, `cancel-order`, `secondary`) and 3 labelled
  misses (`refunds` regular-expression, `/reports` catch-all-route, `order-row-7`
  app-templated-name).

Residuals: adopter-scale qualification against a real Playwright suite is NOT_RUN. Cypress and a
configured `testIdAttribute` are not read. Unanchored regular expressions are treated as path
prefixes and labelled.

Rollback: as in the spec's "Test reference check slice (V1-1581)" section.
