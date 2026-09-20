# Playwright External Provider V0/V1

Owner: Russell Lewis
Date: 2026-09-20
Intent status: accepted
Delivery status: validated (`/0` Playwright 1.60.0/1.63.0; `/1` Docker-backed Playwright 1.63.0)
Profiles: `corvint-playwright-external/0`, `corvint-playwright-external/1`.
Inputs: GitHub issues #19, #43; AGENTS.md invariants 1–8; decision 0179.
Owner acceptance: in the 2026-09-20 issue-resolution task, the owner explicitly approved accepting
and shipping this PWP-V0 profile while retaining the default offline boundary and rollback gates.
The owner subsequently requested issue #43's typed application-attestation revision with the same
external-ownership boundary.

## Agent digest
- Claim: Explicit external-server Playwright runs retain attributable project outcomes without application-server ownership; `/1` additionally binds a typed, pre/post application attestation.
- Status: accepted; validated (`/0` local Playwright 1.60.0/1.63.0 and `/1` Docker-backed Playwright 1.63.0).
- Exists: `internal/jstestprovider`, `cmd/corvint-js-test-provider`, `internal/testvaliditydoc`.
- Read next: Requirements; Wire and trust boundary; Acceptance and rollback.
- Blocked on: no implementation gap; owner-selected checks and separate live witnesses govern final completion. Other Playwright versions, Vitest and LPCV authority remain unqualified. The qualification host had Docker but no Compose frontend, so the checked-in closed Compose JSON manifest was executed by the fixture's equivalent project-scoped Docker build/run path.

## Requirements

- `PWP-V0-001`: External mode requires readiness URL and declared app identity, rejects server commands, observes readiness before and after execution, and never starts or stops the application. External descendant cleanup stays unknown.
- `PWP-V0-002`: A controlled config imports the original, disables webServer and selects the provider reporter. Bind original config digest, override, runner version, exact argv, declared environment and observed test-file digests; changed or unknown inputs cannot yield passing execution.
- `PWP-V0-003`: Each outcome identifies source location, full title, project name, resolved browser/device configuration, config and argv. The same file under different projects stays distinct; missing or ambiguous identity abstains.
- `PWP-V0-004`: Preserve attempt status/retry, flaky, assertion failure, test timeout and interruption. Browser/fixture, server, reporter, global and boundary failures remain infrastructure. Empty/malformed reports and unexplained nonzero exits cannot be green.
- `PWP-V0-005`: Emit and retain the canonical closed profile. MCP discovery recomputes projections and preserves identities, retries, infrastructure, readiness, external cleanup responsibility and unknown freshness. Carried projections confer no authority.
- `PWP-V0-006`: Cancellation joins only owned Playwright descendants and observes external server survival. Unknown runner cleanup or lifecycle/project identity prevents passing projections.
- `PWP-V0-007`: Qualification runs a checked-in real Playwright browser fixture covering pass, assertion failure, timeout, browser infrastructure, two projects, cancellation, server survival, inherited webServer suppression and retained MCP discovery. A skipped live fixture is never qualification success.

### Application-attested revision

- `PWP-V1-001`: `/1` is available only in external-server mode and requires a typed `corvint-application-attestation-command/0` provider and canonical `corvint-application-attestation-config/0`; the `/0` caller label cannot select `/1`, `/0` remains readable, and neither profile can carry the other profile's identity fields.
- `PWP-V1-002`: The closed provider configuration binds the expected application root commit, revision, tree, dirty policy and optional dirty digest, build/image kind and digest, configuration/Compose kind and digest, and instance kind.
- `PWP-V1-003`: Each closed canonical `corvint-application-attestation/0` observation binds the actual application root commit, revision, tree, dirty state/digest, build/image, configuration/Compose, instance/container ID, start generation and health state.
- `PWP-V1-004`: The receipt binds the provider profile, normalized argv, original executable path and SHA-256, canonical configuration path and SHA-256, declared environment, and the SHA-256 of each exact canonical output. The provider executable is launched from a private content copy and its original executable/configuration are rechecked before the post observation.
- `PWP-V1-005`: The provider runs before and after Playwright. Unavailable or malformed identity, unhealthy or contradictory output, expectation mismatch, provider/config drift, application repository/build/configuration drift, instance/start-generation drift, and test-repository drift are explicit infrastructure reasons and cannot project passing evidence.
- `PWP-V1-006`: `/1` binds one clean test repository by root commit, revision and tree before Playwright and after Playwright/provider completion, ignoring ambient Git repository/configuration redirects, plus the existing exact Playwright runner, browser/project, bound source/configuration, argv and declared environment identities in the same receipt.
- `PWP-V1-007`: Corvint supplies only canonical configuration bytes on provider stdin and owns only the provider and Playwright process groups. It never starts, stops, restarts, cleans or sends a lifecycle verb to the externally owned application.
- `PWP-V1-008`: Qualification includes a deterministic, locally owned Docker Compose fixture and a generic command-provider fixture. Negative controls prove that a healthy wrong revision, healthy wrong image, restarted container and test-repository drift cannot produce passing evidence. Fixture cleanup waits for provisioning and retries after interruption; a skipped Docker fixture is never qualification success.

## Wire and trust boundary

This optional companion runs trusted local project code, not hostile code. In `/0`, declared app
identity remains a caller assertion, not proof of served content. `/1` replaces that label with the
typed command-provider observation above. Readiness is HTTP 2xx at two instants, not continuous
availability. Only the provider and runner descendants are owned.
The default native binary and read-only MCP contract are unchanged.

The envelope remains `receipt`, `testProjections`, `runProjection`; `receipt.profile` selects the
closed shape. Canonical bytes are compact Go encoding/json UTF-8 plus LF, sorted map keys and
declared struct field order. Readers reject noncanonical profile bytes. Retention is explicit/local;
only declared environment keys are collected. Default execution bound is five minutes, output/report
bound 4 MiB, readiness fifteen seconds. Application provider configuration/output is 64 KiB per
document and each observation defaults to five seconds. No selector, coverage or repository-pass
claim is added.

## Acceptance and rollback

Baseline: ordinary Playwright plus its JSON report. Go regressions cover malformed/unknown paths;
explicit live qualification uses an installed pinned Playwright/browser runtime. Record exact versions
and gates in BUILD-LOG. Rollback removes this profile/option; old experimental receipts stay readable
and retained evidence is not deleted. `/1` is additive and `/0` remains readable. Further wire fields
require another profile revision. Reporter/config
changes rerun live qualification. Acceptance and passing qualification are both promotion conditions.

## Traceability

Qualification runtimes: Playwright **1.60.0** and **1.63.0**, each with its installed Chromium
browser, tested locally on Darwin. External mode refuses other runner versions until their
effective-fixture metadata is qualified. Both in-process reporter ABIs expose literal fixture defaults and nested `test.use`
overrides; the provider resolves those with project options and preserves the effective browser,
viewport and device settings. Missing metadata, executable option fixtures or custom browser/context/
page fixtures produce unknown identity and never passing execution. A device label remains `unknown`
unless declared in project metadata; effective device parameters are retained independently.
The added qualified tuple is `@playwright/test@1.63.0` / Chromium 153.0.8010.12
(`chromium-1243`, macOS arm64).

Relative global setup/teardown modules resolve from the original config directory. Imported CommonJS
source inputs are hashed at collection and compared again before publication. Configuration loaded
outside that observable module cache is unsupported rather than assumed bound. Secret-shaped and
oversized documents are refused before stdout/retention. A receipt carrying qualified-only metadata
without the exact profile discriminator is refused by the legacy reader.

Run the explicit fixture with `CORVINT_PLAYWRIGHT_MODULES` naming an already-installed `node_modules`
directory and `go test -count=1 -timeout 5m ./internal/jstestprovider -run TestQualifiedPlaywrightLive -v`.
No browser or npm package is downloaded by the provider or ordinary Go gate. The live matrix includes
real pass/assertion/timeout/browser-infrastructure cases, retry/flaky state, inherited webServer
suppression, relative hooks, two projects, literal, executable and custom-fixture overrides,
cancellation, and retained MCP discovery.

For an externally managed application already listening at `http://127.0.0.1:3002`, run this exact
provider command from the application root after replacing the bound config and test paths with the
application's checked-in paths:

`corvint-js-test-provider e2e --dir . --config playwright.config.ts --package-json package.json --lockfile package-lock.json --runner-version 1.63.0 --external-server --app-identity app-at-3002 --server-ready-url http://127.0.0.1:3002 --test-file tests/e2e/example.spec.ts --test-arg tests/e2e/example.spec.ts --retain`

| Requirements | Implementation | Evidence |
|---|---|---|
| PWP-V0-001..007 | `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `cmd/corvint-js-test-provider/main.go`, `internal/testvaliditydoc/document.go` | `TestQualifiedPlaywrightLive`, `TestExternalReadiness`, `TestQualifiedReceiptProjection` |
| PWP-V1-001..008 | `internal/jstestprovider/application_attestation.go`, `internal/jstestprovider/external.go`, `cmd/corvint-js-test-provider/main.go`, `internal/testvaliditydoc/document.go` | `TestApplicationAttestationCommandProvider`, `TestApplicationAttestationNegativeControls`, `TestAttestedReceiptNeverPassesWrongOrRestartedApplication`, `TestApplicationAttestationDockerComposeQualification` |
