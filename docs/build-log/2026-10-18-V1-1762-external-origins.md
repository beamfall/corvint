# 2026-10-18 V1-1762: test validity reports dependence on undeclared external origins

GitHub #1106. A Playwright run could depend on a live public CDN module, a third-party script or
an external API. Nothing in test validity said so. A bulk failure caused by a CDN module that
rendered nothing read as a product regression or a flaky locator. This lane adds a small,
fixture-driven external-origin dependence report.

The new requirements `PWP-V0-022..027` in `docs/specs/playwright-external-provider-v0.md` are
accepted under the owner's standing auto-accept of lane requirements (2026-10-11), for V1-1762.
There is no CAL-V0 change. IDs were checked against origin/main, whose highest is `PWP-V0-021`.
No open PR adds a PWP ID: #1101 adds `LPCV-V0-080..083`, and #1108 only rewrites the spec index.

## Design

- **Placement.** The logic lives in the new files `internal/testvaliditydoc/external_origins.go`
  and `cmd/corvint-js-test-provider/external_origins.go`. The only edit to an existing source file
  is the subcommand switch and usage in `cmd/corvint-js-test-provider/main.go`. This keeps the lane
  clear of the files #1101 edits.
- **Inputs.** The report reads two closed, bounded documents:
  - `corvint-network-evidence/0`, one per run, giving the requests of each test.
  - `corvint-external-origins-policy/0`, giving the declared environment origins and the
    deterministic allow-list with reasons.

  Unknown members, trailing data and over-bound input refuse with a named code.
- **Origins.** Each origin is canonicalised to scheme, host and non-default port. Locators drop the
  query and fragment, so no query value reaches the report. A URL with userinfo is refused.
- **Blocking and incidental requests.**
  - Blocking: document, script, module, stylesheet, websocket and other, plus fetch/xhr unless the
    producer states `awaited: false`.
  - Incidental: beacon, image, font and media.
  - Only an undeclared origin with a blocking request raises `external-origin-dependence`. There is
    one finding per (origin, resource type), naming the first run and first test that made the
    request.
- **Correlation.** For each test that passed in one run and failed or timed out in another at the
  same revision, the report compares the first such pair, keyed by (resource type, locator). Each
  differing request is named as `missing-in-failed`, `missing-in-passed`, `status-differs`,
  `follow-ups-differ` or `slower-in-failed`.
- **Verdict.** A test whose `requests` is absent is listed as `network-evidence-absent`. The verdict
  is then `UNKNOWN` unless a finding already makes it `ENVIRONMENT_DEPENDENT`. Missing evidence
  never yields `CLEAN`.
- **CLI.** `corvint-js-test-provider external-origins --policy FILE --network FILE...` prints the
  `corvint-external-origins/0` report, which lists every external origin. A refusal writes its code
  to stderr and exits 2.

## Non-goals

- No producer. A Playwright fixture or reporter that records `page.on('request')` and response
  timing per test, or a HAR/trace importer, is follow-up work. Today the evidence is hand-made or
  comes from a project's own recorder.
- No live browser run, and no network access by Corvint.
- No join into `corvint test-validity`, the retained receipt, the five-axis projection, the
  passing predicate or MCP discovery. The report is diagnostic only.
- No inference of which assertion read a resource.

## Limits

- Images, fonts and media are always incidental, even when a visual assertion depends on them.
- `other` is treated as blocking.
- Correlation keys on the first occurrence of each (type, locator) and pairs only the first passing
  run with the first failing run per test.
- The `slower-in-failed` threshold is a fixed 1000 ms.
- Bounds:
  - 16 runs.
  - 4096 tests per run.
  - 1024 requests per test.
  - 4 MiB per evidence document.
  - 64 KiB and 256 origins for the policy.

## Evidence

- Fixture-driven Go tests in `internal/testvaliditydoc` (8) and `cmd/corvint-js-test-provider` (1)
  use the fixtures and golden report in `internal/testvaliditydoc/testdata/external-origins/`.
- Live Playwright: `NOT_RUN`.
- Producer evidence: `NOT_PRODUCED`.
