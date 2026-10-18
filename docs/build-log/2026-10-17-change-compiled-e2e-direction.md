# Change-compiled E2E: owner direction and proposed CCE-V0 spec

Owner input, 2026-10-17 (Claude Code session). The owner proposed that Corvint understand the
anatomy of an end-to-end test as lego blocks (page objects, paths, inputs and outputs, results,
outcomes and scripts), keep a catalog of them, and, when code changes, build the E2E test that
checks the change on demand, locally or in CI, instead of hosting a large suite.

After an assessment of the existing pieces and two risks (the oracle problem and impact-analysis
false negatives), the owner chose the direction, verbatim: "Draft the spec as augment-first: hosted
blocks, generated scenarios, a shadow evaluation against the existing suite before any test is
retired. File the umbrella ticket and the first slice."

## Recorded

- `docs/specs/change-compiled-e2e-v0.md` (CCE-V0-001..014), intent `proposed`, delivery
  `not-started`. Its requirements carry no acceptance marker: they are agent-drafted from the owner's
  direction and inherit `proposed` until the owner reads them back.
- Native tickets: umbrella V1-1666 depends on first slice V1-1665.

## Design choices in the draft

- The oracle is always a declared AFU-V1 outcome (matcher, locator, value); the compiler never
  infers an expected value, selector or path. This keeps project authority above generated code.
- A compiled scenario counts only with a discrimination verdict from a live run: pass at head, fail
  at base on the outcome's own assertion, and fail under the BBF-V0 `wrong-locator` and
  `wrong-expected-value` controls. A scenario that passes on both sides is `PRESERVES`, a
  preservation check only.
- Unknown reach reads `UNKNOWN`, never `NONE`, and nothing in V0 narrows hosted selection.
- Retirement is a proposal behind a frozen shadow gate (at least 30 changes, 100% per-candidate and
  95% total seeded-regression parity, zero escapes). Corvint never deletes or disables a hosted
  test. The gate values are owner question 4.

## Not done

No implementation, tests or qualification. Delivery stays `not-started`.
