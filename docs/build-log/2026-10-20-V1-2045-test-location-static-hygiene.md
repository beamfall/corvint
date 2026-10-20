# V1-2045, V1-2046: structured test location and static source hygiene

Date: 2026-10-20. Tickets: V1-2045 (GitHub #1302), V1-2046 (GitHub #1303). Base: origin/main
da15cd7dd.

## Change

- `LPCV-V0-145..149` (V1-2045) give every `corvint-test-validity/0` test an optional `location`
  with `file`, `line` and `titlePath`. Any member that is not set is named in `location.absent`
  with one closed reason (`LPCV-V0-146`). The CLI receipt and discovery modes, static mode and the
  MCP tools all apply the same `testvaliditydoc.Locate`.
- `LPCV-V0-150..153` (V1-2046) add `corvint test-validity --static [PATH...]`. It reads committed
  `HEAD` RSpec and JS/TS test files through the discovery example readers, projects hygiene from
  source and keeps execution, freshness and strength `NOT_OBSERVED`. The document is a
  non-promotable preview, and the mode writes nothing.
- All nine requirements were accepted under the owner's standing auto-accept (2026-10-11) of lane
  requirements.

## Decisions

- An anchor sets `file` or `line` only when it is the declaration. A profiled Playwright receipt
  takes `test.location`. A stock Playwright anchor is the declaration only for an outcome that did
  not fail. Vitest and failed outcomes carry failure frames, so they say `anchor-not-declaration`
  unless discovery's bound source reading places the test.
- A JS/TS `file` is bound only to the one retained `testFileDigests` key the anchor names inside
  the root, so an absolute provider path never reaches the document.
- `titlePath` is literal-only. A computed title path would look authoritative but could differ
  from the runner's.
- An RSpec shared example keeps its definition `file` and `line` and reports its inclusion sites
  as `includedAt`. Its `titlePath` is absent as `shared-example-definition`. This amends the
  projection-byte clause of `LPCV-V0-135`, whose test now checks the shared location before it
  compares bytes.
- Static axes use a new `NOT_OBSERVED` state with reason `static-source-only`, not
  `NOT_MEASURED`. `ApplyExampleSource` changes strength only from `NOT_MEASURED`, so source rules
  cannot overwrite it.
- Static reads use `git --no-optional-locks` through the bounded `testsource` reader. The no-writes
  test settles the fixture index before its snapshot, because the test's own `git status` would
  otherwise rewrite racily-clean index entries.

## Evidence

The focused tests are listed in the spec's traceability row. Their packages pass with
`-count=1`:

- `internal/testvaliditydoc`, `internal/testsource`, `internal/testvalidity` and
  `internal/mcp/testvaliditybridge` (whole packages);
- `cmd/corvint` (the test-validity, help and corpus tests);
- the dependents `doccorpus`, `testplan`, `testevidence`, `companionrelease` and
  `playwrightminimize`.

`NOT_RUN`: a real Playwright run, a real RSpec run, and a formatter that emits
`shared_group_inclusion_backtrace`.

## Residuals

- The Playwright reporter and the RSpec formatter keep only joined names, not structured title
  paths. The title path therefore comes from literal source only.
- An unprofiled JS outcome or a Vitest outcome has no `file` or `line` until discovery places it.
- No static mode in MCP (`NOT_PRODUCED`).
