# Playwright External Provider V0

Owner: Russell Lewis
Date: 2026-09-20
Intent status: proposed (acceptance pending explicit owner confirmation)
Delivery status: experimental
Profile: `corvint-playwright-external/0`
Inputs: GitHub issue #19; AGENTS.md invariants 1–8; decision 0179.

## Agent digest

- Claim: Explicit external-server Playwright runs retain attributable project outcomes without application-server ownership.
- Status: proposed; experimental pending acceptance and live qualification.
- Exists: `internal/jstestprovider`, `cmd/corvint-js-test-provider`, `internal/testvaliditydoc`.
- Read next: Requirements; Wire and trust boundary; Acceptance and rollback.
- Does not promote the prose-only JS/TS experiment, Vitest, LPCV authority or omitted-test coverage.

## Requirements

- `PWP-V0-001`: External mode requires readiness URL and declared app identity, rejects server commands, observes readiness before and after execution, and never starts or stops the application. External descendant cleanup stays unknown.
- `PWP-V0-002`: A controlled config imports the original, disables webServer and selects the provider reporter. Bind original config digest, override, runner version, exact argv, declared environment and observed test-file digests; changed or unknown inputs cannot yield passing execution.
- `PWP-V0-003`: Each outcome identifies source location, full title, project name, resolved browser/device configuration, config and argv. The same file under different projects stays distinct; missing or ambiguous identity abstains.
- `PWP-V0-004`: Preserve attempt status/retry, flaky, assertion failure, test timeout and interruption. Browser/fixture, server, reporter, global and boundary failures remain infrastructure. Empty/malformed reports and unexplained nonzero exits cannot be green.
- `PWP-V0-005`: Emit and retain the canonical closed profile. MCP discovery recomputes projections and preserves identities, retries, infrastructure, readiness, external cleanup responsibility and unknown freshness. Carried projections confer no authority.
- `PWP-V0-006`: Cancellation joins only owned Playwright descendants and observes external server survival. Unknown runner cleanup or lifecycle/project identity prevents passing projections.
- `PWP-V0-007`: Qualification runs a checked-in real Playwright browser fixture covering pass, assertion failure, timeout, browser infrastructure, two projects, cancellation, server survival, inherited webServer suppression and retained MCP discovery. A skipped live fixture is never qualification success.

## Wire and trust boundary

This optional companion runs trusted local project code, not hostile code. Declared app identity is
a caller assertion, not proof of served content. Readiness is HTTP 2xx at two instants, not continuous
availability. Missing app-build digests stay unknown. Only runner descendants are owned.
The default native binary and read-only MCP contract are unchanged.

The envelope remains `receipt`, `testProjections`, `runProjection`; `receipt.profile` selects the
closed shape. Canonical bytes are compact Go encoding/json UTF-8 plus LF, sorted map keys and
declared struct field order. Readers reject noncanonical profile bytes. Retention is explicit/local;
only declared environment keys are collected. Default execution bound is five minutes, output/report
bound 4 MiB, readiness fifteen seconds. No selector, coverage or repository-pass claim is added.

## Acceptance and rollback

Baseline: ordinary Playwright plus its JSON report. Go regressions cover malformed/unknown paths;
explicit live qualification uses an installed pinned Playwright/browser runtime. Record exact versions
and gates in BUILD-LOG. Rollback removes this profile/option; old experimental receipts stay readable
and retained evidence is not deleted. New wire fields require a profile revision. Reporter/config
changes rerun live qualification. Acceptance and passing qualification are both promotion conditions.

## Traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| PWP-V0-001..007 | `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `cmd/corvint-js-test-provider/main.go`, `internal/testvaliditydoc/document.go` | `TestQualifiedPlaywrightLive`, `TestExternalReadiness`, `TestQualifiedReceiptProjection` |
