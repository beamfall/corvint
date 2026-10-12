# 2026-10-12 Playwright config-input bounds and behavior state matchers (GitHub #844, #845)

Branch `claude/gh-844-845`, base `origin/main` 35c365a9e. Tickets V1-1317 (#844) and V1-1318 (#845).
Requirements `PWP-V0-021` and `DCP-V1-083`, `DCP-V1-084` (proposed in this change).

## #844: config-input bounds (`PWP-V0-021`)

Decision: raise `externalMaxConfigInputs` from 256 to 4096 and add `externalMaxConfigInputBytes`,
1 MiB over the summed path and digest lengths. Above either bound `bindQualifiedReport` returns
`config-inputs-overflow: observed <N> config inputs, limit 4096` or `... config input bytes, limit
1048576` (outer reason still `report-identity-unknown`). `report-output-overflow` is left only to
`readBoundedReport`, the 4 MiB report-file bound.

Why these numbers: the reporter records every non-`node_modules` module in the runner's module
cache, which includes every listed spec and its imports, so a mid-size suite passed 256 after two
added support modules (issue #844). 4096 is sixteen times that and still re-reads and digests each
input at bind time. The byte bound is a quarter of the 4 MiB provider-document bound
(`internal/testvaliditydoc` `MaxInputBytes`) that reads the retained receipt, leaving the rest for
tests and attempts. Neither bound is measured against a real large suite: `NOT_OBSERVED`.

Decision: the keep-reporters qualification record keeps its own 256-input bound
(`keepReportersMaxConfigInputs`), because its 256 KiB record bound (`PWP-V0-017`) cannot hold 4096
inputs. A keep-reporters run above 256 inputs is therefore incomplete for qualification, as before.
The retained freshness-row check (`freshRowUnknown`) uses the same two bounds as binding.

## #845: valueless state matchers (`DCP-V1-083`, `DCP-V1-084`)

Decision: `behavior.expect` records `toBeVisible`, `toBeHidden`, `toBeEnabled` and `toBeChecked`
with value `visible`, `hidden`, `enabled`, `checked`, matching the `toBeVisible` to `visible` naming
in `tools/web-flows/scan.mjs` and the flow-coverage fixture. Recording follows the `DCP-V1-081`
conditions (genuine matcher passed, no new attempt error, observed main-frame Locator).

Decision (fail closed): only no argument or a timeout-only options object records; the matcher
receives a fresh `{timeout}` snapshot. State options (`visible`, `enabled`, `checked`,
`indeterminate`), including ones that restate the default, non-object or extra arguments run the
matcher and record nothing. `behavior.expect` keeps offering no `.not`; negated states go through
`toBeHidden` or `negativeControl`. This keeps a recorded event meaning exactly "the positive state
held".

Decision: no `behavior-adapter` code change. It already compares matcher, locator and value byte for
byte; `DCP-V1-084` documents the canonical reviewed form, and a new test proves both equality and
that a non-canonical value stays a mismatch. The Go test is a new file so the peer-held
`internal/doccorpus/` Go sources are not edited.

## Evidence

- `go test ./internal/jstestprovider/` (focused: `TestQualifiedReportConfigInputBounds`,
  `TestRunUnit_ReportBoundIsItsOwnAndNotTheExternalProviders`, keep-reporters and freshness tests).
- `node --test internal/doccorpus/behavior-events_test.cjs` and
  `TestDCPV1081ProducerNodeUnit`, `TestDCPV1084StateMatcherAssertionIsWitnessed`.

## NOT_RUN

- Live Playwright run with more than 256 config inputs, and a live state-matcher producer run
  (the producer is qualified on Playwright 1.61.1 only, `DCP-V1-081`).
- `make gate` and the full `./...` suite (owner's scoped-issue preference).
