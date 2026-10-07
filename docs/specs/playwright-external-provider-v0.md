# Playwright External Provider V0/V1/V2/V3

Owner: Russell Lewis
Date: 2026-09-20
Intent status: accepted
Delivery status: `/0` and `/1` validated; `/2` implemented, conformance-tested, live reporter matrix `NOT_RUN`; `/3` implemented; qualification evidence recorded with issue 175
Profiles: `corvint-playwright-external/0`, `corvint-playwright-external/1`, `corvint-playwright-external/2`, `corvint-playwright-external/3`.
Inputs: GitHub issues #19, #39, #43, #49, #50, #56, #665; AGENTS.md invariants 1–8; decision 0179.
Owner acceptance: in the 2026-09-20 issue-resolution task, the owner explicitly approved accepting
and shipping this PWP-V0 profile while retaining the default offline boundary and rollback gates.
The owner subsequently requested issue #43's typed application-attestation revision with the same
external-ownership boundary. Decision 0417 approves the every-attempt `/3` revision (owner answer
"approve /3 profile", 2026-09-25); its composition with `/1` and `/2` is an open owner question.

## Agent digest
- Claim: External-server Playwright receipts bind outcomes without owning the app; `/1` attests application identity and `/2` redacts sensitive input evidence.
- Status: accepted; `/0` and `/1` validated; `/2` implemented, conformance-tested, live reporter matrix `NOT_RUN`; `/3` implemented; qualification evidence recorded with issue 175.
- Exists: PWP-V3-007 admits exact unnamed projects only in `/3`; no attestation composition is added. `internal/jstestprovider`, `cmd/corvint-js-test-provider`, `internal/testvaliditydoc`.
- Read next: Requirements; Wire and trust boundary; Acceptance and rollback.
- Blocked on: no implementation gap; owner-selected checks and separate live witnesses govern final completion. Proposed PWP-V0-009 (Node v24.11.1, `/0` bundled tuple) awaits owner acceptance. Proposed PWP-V0-010..013 (keep-reporters option, V1-0986) await owner acceptance and a live run. Other Playwright versions, Vitest and LPCV authority remain unqualified. The qualification host had Docker but no Compose frontend, so the checked-in closed Compose JSON manifest was executed by the fixture's equivalent project-scoped Docker build/run path.

## Requirements

- `PWP-V0-001`: External mode requires readiness URL and declared app identity, rejects server commands, observes readiness before and after execution, and never starts or stops the application. External descendant cleanup stays unknown.
- `PWP-V0-002`: A controlled config imports the original, disables webServer and selects the provider reporter. Bind original config digest, override, runner version, exact argv, declared environment and observed test-file digests; changed or unknown inputs cannot yield passing execution.
- `PWP-V0-003`: Each outcome identifies source location, full title, project name, resolved browser/device configuration, config and argv. The same file under different projects stays distinct; missing or ambiguous identity abstains.
- `PWP-V0-004`: Preserve attempt status/retry, flaky, assertion failure, test timeout and interruption. Browser/fixture, server, reporter, global and boundary failures remain infrastructure. Empty/malformed reports and unexplained nonzero exits cannot be green.
- `PWP-V0-005`: Emit and retain the canonical closed profile. MCP discovery recomputes projections and preserves identities, retries, infrastructure, readiness, external cleanup responsibility and unknown freshness. Carried projections confer no authority.
- `PWP-V0-006`: Cancellation joins only owned Playwright descendants and observes external server survival. Unknown runner cleanup or lifecycle/project identity prevents passing projections.
- `PWP-V0-007`: Qualification runs checked-in real Playwright browser fixtures covering pass, assertion failure, timeout, browser infrastructure, two projects, a standard `devices['Desktop Chrome']` spread, cancellation, server survival, inherited webServer suppression and retained MCP discovery. A skipped live fixture is never qualification success.
- `PWP-V0-008`: Playwright 1.63 qualification is consuming-path specific. A passing projection requires a separately qualified Node, operating-system/architecture and effective browser tuple; another tuple remains diagnostic-only. A configured executable binds its exact version, channel/path and headless-shell availability. A Playwright-bundled executable additionally binds the registry executable name, package-pinned browser revision and manifest version, absolute executable path and executable SHA-256. Any missing or changed field abstains. Additional Node or browser tuples require an explicit qualification record and the complete live matrix below; matching only the package version never admits them. Consumer checkout and CI observations remain `NOT_OBSERVED` or `NOT_RUN` when unavailable.
- `PWP-V0-009`: (accepted by decision 0448; V1-0976) exactly one more tuple is admitted under `PWP-V0-008`: Darwin arm64 / Node `v24.11.1` / `@playwright/test@1.63.0` with the bundled headless shell named below, for the base `/0` profile only, on the retained live run recorded for issue #665. Node `v24.11.0` and every other `v24.x` release, the system-Chrome tuple on Node 24, and `/1`, `/2` and `/3` on Node 24 stay unqualified until their own live run. Every retained external receipt classifies its runner/Node pair as `candidate`, `unqualified` or `unobserved` through `ReceiptRuntimeTuple`, the same closed values the passing predicate applies first; discovery reports an `unqualified` or `unobserved` pair as `LPCV-V0-056` states. The live matrix MAY run against an explicitly provided Node binary (`CORVINT_PLAYWRIGHT_NODE`, an absolute path whose directory also holds `npx`), placed first on that test's `PATH` only, and MUST then observe that exact version; an optional `CORVINT_PLAYWRIGHT_CONTROL_NODE` names an unqualified Node whose run MUST abstain and be reported as `runtime-tuple-unqualified`.

### Keep-reporters option (proposed; V1-0986; GitHub #670)

- `PWP-V0-010`: (proposed (V1-0986; GitHub #670)) The opt-in `--keep-reporters` option, available only in external-server mode on `/0` to `/3`, appends the provider reporter as the last entry of the project's original `reporter` list instead of replacing that list. The original value is normalized: undefined keeps no entries, a string keeps one entry, and an array keeps entries that are each a name or `[name]` or `[name, options]`. Any other value refuses `project-reporters-invalid` when the controlled config loads. A name beginning with `.` or an absolute name resolves against the original config directory. Any other name that is not one of the built-in reporters (`blob`, `dot`, `github`, `html`, `json`, `junit`, `line`, `list`, `null`) resolves through Node module resolution from that directory, or stays unchanged when it cannot be resolved. String `outputFile`, `outputFolder` and `outputDir` options of the built-in `blob`, `html`, `json` and `junit` reporters resolve against the original config directory. No other option value is changed. Without the option, the controlled config and the receipt bytes are byte-identical to the replace-only form. An owned-server or freshness run with the option refuses `keep-reporters-unsupported-mode`.
- `PWP-V0-011`: (proposed (V1-0986; GitHub #670)) A keep-reporters receipt binds `receipt.external.projectReporters` with two members. `entries` lists every kept entry in order, at most 32. `effects` is described in `PWP-V0-012`. Each entry carries its resolved `name`, a `module` value and an `options` value:
  - `module` is `builtin` for a built-in reporter. It is `bound` when the exact module path is a reporter-observed config input; its `moduleDigest` must then equal that input's digest, so `PWP-V0-002` drift detection covers it. Otherwise it is `unknown`.
  - `options` is `absent` when the entry has no options. It is `bound` when the options are plain JSON data; the entry then carries `optionsDigest`, the SHA-256 of the sorted-key canonical JSON. It is `unknown` for functions, class instances, cycles, non-finite numbers, array holes or depth 32 and beyond.

  Option values themselves are never retained. The retained controlled config (`PWP-V0-002`) identifies keep-reporters mode.
- `PWP-V0-012`: (proposed (V1-0986; GitHub #670)) The provider never observes what a kept reporter does: files written, output, reporter errors or mutation of shared result objects. `effects` is therefore always `unknown`, which is recorded and never trusted. The binding can only remove passing authority, never add it. It must agree with the controlled config in both directions and use only the closed values above. Any violation refuses `project-reporters-invalid` at encoding and prevents a passing projection. A keep-reporters run whose reporter did not report the kept entries is the infrastructure failure `project-reporters-unobserved`. Unobserved (`null`) entries are valid only beside a run-level infrastructure failure. A replace-only report that carries entries is `report-unparseable`. The qualified runtime tuple (`PWP-V0-008`, `PWP-V0-009`), the `/2` sensitive-input policy and redaction, the `/3` attempt details and every existing passing prerequisite are unchanged. Kept-reporter output counts toward the 4 MiB output bound.
- `PWP-V0-013`: (proposed (V1-0986; GitHub #670)) Test-validity decoding and MCP discovery accept a canonical keep-reporters receipt on the same terms as a replace-only receipt and recompute its projections. A changed binding fails the canonical or shape check. Non-goals:
  - ingesting the outputs of kept reporters (foreign JSON or blob reports)
  - qualifying any kept reporter
  - changing which reporters run without the option

  Known limits:
  - A kept custom reporter that reads `config.configFile` sees the controlled config path.
  - ESM, `node_modules` or symlinked reporter modules may record `module` as `unknown`.
  - Live Playwright qualification of the option is `NOT_RUN`.

  Owner questions:
  - This adds an opt-in member to the existing profiles rather than a new profile revision (see Acceptance and rollback).
  - The provider reporter runs last, after kept reporters could alter shared result objects.
  - `optionsDigest` of a low-entropy secret option value would be guessable.

### Application-attested revision

- `PWP-V1-001`: `/1` is available only in external-server mode and requires a typed `corvint-application-attestation-command/0` provider and canonical `corvint-application-attestation-config/0`; the `/0` caller label cannot select `/1`, `/0` remains readable, and neither profile can carry the other profile's identity fields.
- `PWP-V1-002`: The closed provider configuration binds the expected application root commit, revision, tree, dirty policy and optional dirty digest, build/image kind and digest, configuration/Compose kind and digest, and instance kind.
- `PWP-V1-003`: Each closed canonical `corvint-application-attestation/0` observation binds the actual application root commit, revision, tree, dirty state/digest, build/image, configuration/Compose, instance/container ID, start generation and health state.
- `PWP-V1-004`: The receipt binds the provider profile, normalized argv, original executable path and SHA-256, canonical configuration path and SHA-256, declared environment, and the SHA-256 of each exact canonical output. The provider executable is launched from a private content copy and its original executable/configuration are rechecked before the post observation.
- `PWP-V1-005`: The provider runs before and after Playwright. Unavailable or malformed identity, unhealthy or contradictory output, expectation mismatch, provider/config drift, application repository/build/configuration drift, instance/start-generation drift, and test-repository drift are explicit infrastructure reasons and cannot project passing evidence.
- `PWP-V1-006`: `/1` binds one clean test repository by root commit, revision and tree before Playwright and after Playwright/provider completion, ignoring ambient Git repository/configuration redirects, plus the existing exact Playwright runner, browser/project, bound source/configuration, argv and declared environment identities in the same receipt.
- `PWP-V1-007`: Corvint supplies only canonical configuration bytes on provider stdin and owns only the provider and Playwright process groups. It never starts, stops, restarts, cleans or sends a lifecycle verb to the externally owned application.
- `PWP-V1-008`: Qualification includes a deterministic, locally owned Docker Compose fixture and a generic command-provider fixture. Negative controls prove that a healthy wrong revision, healthy wrong image, restarted container and test-repository drift cannot produce passing evidence. Fixture cleanup waits for provisioning and retries after interruption; a skipped Docker fixture is never qualification success.

### Sensitive-input revision

- `PWP-V2-001`: `/2` is selected explicitly by `--sensitive-input-redaction`. It retains bounded nested action steps per retry attempt; `/0` and `/1` reject `/2` policy or step fields and remain byte-compatible.
- `PWP-V2-002`: Before reporter serialization, input actions matching the fixed defaults `fill`, `type`, `insertText`, `insert-text`, `insert text`, `pressSequentially`, and `press sequentially` replace the entire title with a canonical action kind plus the exact marker `[REDACTED]`. The grammar matches only a leading action after separators or a receiver identifier chain, with dot separators or adjacent slash separators. Word tokens use Unicode letters, numbers and marks with per-rune lowercase comparison; every other rune, including symbols, is a separator for both pattern normalization and matching. Indices address original runes, never transformed bytes. Casing, leading whitespace, action punctuation and receiver prefixes cannot bypass classification. Assertion/navigation prose is not searched for embedded actions. Call-bearing receiver expressions are not supported/qualified syntax: a bounded lexical scan rejects sensitive action tokens and sensitive quoted property names in those expressions with `sensitive-input-action-syntax-unsupported`, before serialization or retained-document acceptance. It does not implement a general JavaScript call parser. A rejected reporter attempt prevents later report serialization. No original value or digest is retained.
- `PWP-V2-003`: Redaction recurses through nested steps and retry attempts. Any sensitive action, including an already-redacted action, requires every nonempty risk-bearing field across every test and retry in the entire report to equal `[REDACTED]`: step/parent errors and attachments, test failure messages and artifacts, global reporter errors and infrastructure detail. Ingestion rejects violations without needing the original value, so an adapter cannot move a value to another test. Reporter serialization canonicalizes those fields. Raw action tails and declared fields produce only transient candidates: whole tails and individual top-level arguments, including final arguments, respecting quoted/escaped text and nested parentheses. Candidates scrub only report-wide risk fields and are never serialized or digested. Assertion titles, navigation/action metadata, enums, IDs, digests and bound identity remain byte-for-byte usable for criterion matching.
- `PWP-V2-004`: Provider-declared additional case-insensitive action-title patterns and metadata field names are closed and bounded additions. At most 16 action patterns and 32 field names are admitted, each nonempty and at most 128 UTF-8 bytes after whitespace trimming and per-rune lowercasing. Every admitted action pattern must normalize to at least one word using the same separator class as matching; zero-word declarations such as `***` and bound violations return fixed `sensitive-input-policy-invalid`. They cannot remove, replace, or weaken the defaults. The profile rejects depth over 32, more than 4096 total steps across retries, any step/error/attachment string over 64 KiB, and more than 64 validation findings before recursive path growth.
- `PWP-V2-005`: Reporter output and retained documents are untrusted. Ingestion rejects an unredacted sensitive action or noncanonical protected risk field with typed finding `sensitive-input-unredacted`, naming only its structural path and never echoing the value. Malformed retained `/2` documents return fixed typed `sensitive-input-document-invalid` diagnostics; unknown property names, values and decoder text are never echoed. Inputs whose kind cannot be decoded use that same value-free rejection because their profile is not trustworthy.
- `PWP-V2-006`: The conformance fixture contains a deliberately leaking payload that is rejected and a redacted payload that is accepted with action-level traceability. `/2` cannot project passing evidence until the changed reporter completes the required live matrix.

### Every-attempt revision

Intent accepted by decision 0417 for `AFU-V1-012`; `--retain-attempt-details` selects the implemented
profile. Qualification is consuming-tuple specific and retains every failed or unavailable observation. `/3` is qualified only on the bundled Darwin arm64 / Node v22.23.2 / Playwright 1.63.0 tuple below; it refuses the legacy 1.60 and system-browser tuples.

- `PWP-V3-001`: `/3` is selected explicitly by a provider option in external-server mode. `/0`, `/1` and `/2` stay readable and byte-compatible, and each still refuses `attemptDetails` with `external-profile-has-attempt-details`; only a `/3` receipt may carry it.
- `PWP-V3-002`: A `/3` test outcome carries `attemptDetails` with one entry per Playwright attempt, in retry order starting at retry 0, using the existing `AttemptDetail` members (state, retry, duration, failure message, anchor, artifacts). The final attempt entry equals the test's existing last-attempt fields; a missing, duplicated, reordered or contradictory attempt refuses the receipt with `external-attempt-details-invalid`.
- `PWP-V3-003`: The existing per-attempt classification is unchanged: browser or fixture failure stays infrastructure on its own attempt, and an earlier failed attempt before a pass keeps the test `flaky`. `/3` adds no new state, outcome or pass rule.
- `PWP-V3-004`: Every `/3` attempt failure message and artifact is subject to the same report-wide risk-field rules as the last attempt. Until the owner settles composition (below), `/3` is refused together with `/2` sensitive-input redaction or `/1` application attestation with `external-attempt-details-composition-unsupported`, so no earlier attempt's text can bypass redaction.
- `PWP-V3-005`: The reporter emits `attemptDetails` only under `/3`; the provider validates the closed shape and the per-test attempt bound (32 attempts, matching the run-evidence attempt bound) before projection. `flows ingest --format playwright-receipt` accepts the canonical /3 document, checks the declared runner version and rederived receipt-wide qualification (readiness, ownership, config/test identity and admitted browser tuple), and maps each entry to one `RunAttempt` without inventing missing members. The unchanged run-evidence/0 vocabulary cannot represent an infrastructure attempt: such a record refuses with `external-attempt-test-unqualified` (or the adapter's explicit `external-attempt-infrastructure-unsupported`), never a fabricated assertion outcome.
- `PWP-V3-006`: `/3` is non-promotable until the changed reporter completes the live matrix of `PWP-V0-007` plus a retried-then-passed fixture and a retried infrastructure-then-assertion fixture on a `PWP-V0-008` qualified tuple. A skipped live fixture is never qualification success.

Open owner question: whether `/3` composes with `/1` (attempt details beside attested identity)
and `/2` (redacting every earlier attempt's failure message and artifacts), or stays a separate
unattested, unredacted profile. `PWP-V3-004` refuses both compositions until answered.

Non-goals: per-attempt browser steps outside `/2`, trace or video content, a new outcome state, and
changing `/0`..`/2` wire bytes.

Failure modes: an earlier attempt's failure text leaks a sensitive value (prevented by
`PWP-V3-004`); a reporter drops or reorders attempts so a flaky test reads as clean (refused by
`PWP-V3-002`); a `/3` receipt is promoted from Go regressions alone (blocked by `PWP-V3-006`).

## Wire and trust boundary

This optional companion runs trusted local project code, not hostile code. In `/0`, declared app
identity remains a caller assertion, not proof of served content. `/1` replaces that label with the
typed command-provider observation above. Readiness is HTTP 2xx at two instants, not continuous
availability. Only the provider and runner descendants are owned.
The default native binary and read-only MCP contract are unchanged. The `/2` evidence-safety boundary
ends at the closed receipt: providers must redact before writing their private report, declare only
bounded additive policy, and must expect Corvint to validate again at ingestion. Corvint never stores
the original input or a reversible digest. `/2` parsing and validation failures expose only fixed
typed details; decoder text and unknown property names are never copied into retained evidence.
This rule is product- and repository-agnostic.

The envelope remains `receipt`, `testProjections`, `runProjection`; `receipt.profile` selects the
closed shape. Canonical bytes are compact Go encoding/json UTF-8 plus LF, sorted map keys and
declared struct field order. Readers reject noncanonical profile bytes. Retention is explicit/local;
only declared environment keys are collected. Default execution bound is five minutes, output/report
bound 4 MiB, readiness fifteen seconds. Application provider configuration/output is 64 KiB per
document and each observation defaults to five seconds. No selector, coverage or repository-pass
claim is added.

## Refusal vocabulary

The provider's closed refusal codes are:

- `config-input-drift`
- `config-inputs-unobserved` (checked against at most `externalMaxConfigInputs`, 256, reporter-observed
  config entries; beyond that count the report is refused as `report-output-overflow` before this
  check runs)
- `external-app-identity-required`
- `external-config-version-test-files-required`
- `external-input-bound-or-secret`
- `external-playwright-version-unqualified`
- `external-readiness-url-required`
- `external-server-command-forbidden`
- `input-identity-changed`
- `no-tests-observed`
- `keep-reporters-unsupported-mode`
- `project-location-unknown`
- `project-reporters-invalid`
- `project-reporters-unobserved`
- `qualified-document-output-overflow`
- `qualified-document-secret-shaped`
- `report-identity-unknown`
- `report-output-overflow`
- `reporter-global-error`
- `run-status-unknown`
- `runner-cleanup-unknown`
- `runner-version-mismatch`
- `server-unavailable-at-publish`
- `test-attempt-identity-unknown`
- `test-attempt-state-unknown`
- `test-file-unbound`
- `test-identity-ambiguous`
- `test-state-unknown`
- `sensitive-input-policy-drift`
- `sensitive-input-policy-invalid`
- `sensitive-input-policy-required`
- `sensitive-input-policy-requires-profile-2`
- `sensitive-input-redaction-requires-external-server`
- `sensitive-input-unredacted`
- `sensitive-input-document-invalid`
- `sensitive-input-action-syntax-unsupported`
- `sensitive-input-depth-exceeded`
- `sensitive-input-step-bound-exceeded`
- `sensitive-input-string-bound-exceeded`
- `sensitive-input-finding-bound-exceeded`
- `legacy-external-profile-has-sensitive-input-fields`
- `attested-external-profile-has-sensitive-input-fields`
- `sensitive-external-profile-has-partial-attested-fields`
- `sensitive-attested-profile-has-declared-identity`

Each code is a typed refusal or incomplete-evidence reason; none is a passing verdict.

## Acceptance and rollback

Baseline: ordinary Playwright plus its JSON report. Go regressions cover malformed/unknown paths;
explicit live qualification uses an installed pinned Playwright/browser runtime. Record exact versions
and gates in BUILD-LOG. Rollback removes this profile/option; old experimental receipts stay readable
and retained evidence is not deleted. `/1` is additive and `/0` remains readable. Further wire fields
require another profile revision, so every profile refuses the unprofiled receipt's `attemptDetails`. Reporter/config
changes rerun live qualification. Acceptance and passing qualification are both promotion conditions.
`/2` therefore remains readable and conformance-tested but non-promotable while its live reporter
matrix is `NOT_RUN`; `/0` and `/1` retain their existing qualification. `/3` is the one revision
that carries `attemptDetails`; rolling it back removes the option, and retained `/3` receipts are
neither deleted nor rewritten.
The proposed keep-reporters option (`PWP-V0-010`..`013`) is an opt-in exception to the revision rule. Its
`projectReporters` member appears only with the keep-reporters controlled config, and a reader that predates
it fails closed through the canonical re-encoding check. Rolling it back removes the option. Retained
keep-reporters receipts are neither deleted nor rewritten.

## Traceability

Qualification runtimes: Playwright **1.60.0** and **1.63.0**, each with its installed Chromium
browser, tested locally on Darwin. External mode refuses other runner versions until their
effective-fixture metadata is qualified. Both in-process reporter ABIs expose literal fixture defaults and nested `test.use`
overrides; the provider resolves those with project options and preserves the effective browser,
viewport and device settings. Missing metadata, executable option fixtures or custom browser/context/
page fixtures produce unknown identity and never passing execution. A device label remains `unknown`
unless declared in project metadata; effective device parameters are retained independently.
A separate live regression reproduces the consumer's standard
`projects: [{name: 'chromium', use: {...devices['Desktop Chrome']}}]` configuration and requires the
resolved browser, nonempty user agent, 1280×720 viewport, config digest, stable test ID and qualified
bundled executable tuple to survive together. The reporting consumer's checkout and hosted CI remain
`NOT_OBSERVED`; the checked-in minimal fixture proves the reported configuration shape locally.
The qualified configured tuple is macOS arm64 / Node v22.23.2 / `@playwright/test@1.63.0` /
system Google Chrome 153.0.8010.48 at
`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, with no channel override and the
Playwright headless shell present. The qualified reproducible tuple uses the same OS, architecture,
Node and Playwright package with its default headless executable: registry name
`chromium-headless-shell`, revision `1243`, manifest version `153.0.8010.12`, observed version
`Google Chrome for Testing 153.0.8010.12`, path suffix
`chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell`, and executable
SHA-256 `a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282`. The cache root may move;
the registry identity, suffix and digest may not. `PWP-V0-009` admits the same bundled tuple on Node
`v24.11.1` for `/0` only. Bundled headed Chromium, Linux amd64 and every other Node tuple are
`NOT_RUN` and remain diagnostic-only. Issue #665 reported a consumer pinning Node v24.11.x, whose
discovery therefore read `source: "none"` with no tests; no consumer checkout, fixture or CI run was
observed for this spec (`NOT_OBSERVED`), and that absence is not qualification evidence. The
consumer-side fixture is a separate `NOT_OBSERVED` input, not a precondition of the tuple predicate.

To qualify another Node or bundled-browser tuple, pin `@playwright/test` and `playwright-core` in the
consumer lockfile, install the package-selected browsers without a system executable override, and
record the exact OS/architecture, Node version, registry name/revision/manifest version, observed
browser version, executable suffix and SHA-256. Add those exact values to the closed tuple predicate,
then run `TestQualifiedPlaywrightLive` with pass, assertion failure, timeout, retry, cancellation,
browser-infrastructure, two-project identity, external-server survival, retained discovery, behavior
and stability consumption, plus negative controls that change the Node version, revision, digest,
headed mode, configured executable and local-versus-remote browser source.
Only a reviewed spec amendment and a passing retained run admit the tuple; environment similarity,
semver compatibility or a successful ad hoc run does not. To run the matrix on a Node that is not the
host default, set `CORVINT_PLAYWRIGHT_NODE` to its absolute `bin/node` (`PWP-V0-009`); nothing is
installed or added to a global `PATH`. The Node v24.11.1 qualification command is:

`CORVINT_PLAYWRIGHT_MODULES=<node_modules with @playwright/test 1.63.0> CORVINT_PLAYWRIGHT_NODE=<node-v24.11.1-darwin-arm64>/bin/node CORVINT_PLAYWRIGHT_CONTROL_NODE=<an unqualified node, e.g. v22.23.3> GOTOOLCHAIN=local go test -count=1 -timeout 10m ./internal/jstestprovider -run 'TestQualifiedPlaywrightLive$|TestQualifiedPlaywrightLiveDevicesSpread$' -v`

Owner answer (decision 0448, V1-0976): further Node patch releases are admitted one exact release
at a time, through this predicate and a Corvint change backed by a live run; a consumer-side
qualification record is not adopted. `/1`, `/2` and `/3` on Node 24 need their own live runs first.

Relative global setup/teardown modules resolve from the original config directory. Imported CommonJS
source inputs are hashed at collection and compared again before publication. Configuration loaded
outside that observable module cache is unsupported rather than assumed bound. Secret-shaped and
oversized documents are refused before stdout/retention. A receipt carrying qualified-only metadata
without the exact profile discriminator is refused by the legacy reader.

Run the explicit fixture with `CORVINT_PLAYWRIGHT_MODULES` naming an already-installed `node_modules`
directory and `go test -count=1 -timeout 5m ./internal/jstestprovider -run TestQualifiedPlaywrightLive -v`.
No browser or npm package is downloaded by the provider or ordinary Go gate. The live matrix includes
real pass/assertion/timeout/browser-infrastructure cases, setup dependency hashing, global use,
project inheritance, retry/flaky state, repeat-each identity, two workers, inherited webServer
suppression, relative hooks, two projects, literal, executable and custom-fixture overrides,
cancellation, retained MCP discovery and a smoke run of the prior system-browser tuple. The primary
matrix launches the Playwright registry's bundled headless shell. A Playwright 1.63 receipt without
one exact qualified Node/browser tuple abstains rather than projecting green. The behavior and
stability corpus fixtures ingest the bundled tuple through the same `QualifiedReceiptBindingReady`
path used for retained receipts; they contain no system-browser exception.

For an externally managed application already listening at `http://127.0.0.1:3002`, run this exact
provider command from the application root after replacing the bound config and test paths with the
application's checked-in paths:

For the bundled qualified path, omit `use.launchOptions.executablePath`; Playwright 1.63 selects its
pinned headless shell and the provider binds its registry revision and executable digest. A configured
path instead selects the separately qualified system-Chrome tuple. The provider does not implement a
browser-path override. With the bundled path, the exact command is:

`corvint-js-test-provider e2e --dir . --config playwright.config.ts --package-json package.json --lockfile package-lock.json --runner-version 1.63.0 --external-server --app-identity app-at-3002 --server-ready-url http://127.0.0.1:3002 --test-file tests/e2e/example.spec.ts --test-arg tests/e2e/example.spec.ts --retain`

| Requirements | Implementation | Evidence |
|---|---|---|
| PWP-V0-001..008 | `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `cmd/corvint-js-test-provider/main.go`, `internal/testvaliditydoc/document.go` | `TestQualifiedPlaywrightLive`, `TestQualifiedPlaywrightLiveDevicesSpread`, `TestExternalReadiness`, `TestQualifiedReceiptProjection`, `TestPlaywright163UnqualifiedBrowserTupleAbstains`, `TestPlaywright163BundledBrowserTupleAbstainsOnDrift` |
| PWP-V0-009 (accepted, decision 0448) | `internal/jstestprovider/runtime_tuple.go`, `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `internal/testvaliditydoc/discover.go` | `TestPlaywright163Node24TupleAdmissionIsExact`, `TestReceiptRuntimeTupleClassification`, `TestDiscoverAbstentionReasons`; live `TestQualifiedPlaywrightLive` and `TestQualifiedPlaywrightLiveDevicesSpread` on Node v24.11.1 with the v22.23.3 control (`docs/build-log/2026-10-07-gh665-playwright-node-tuple-abstention.md`) |
| PWP-V0-010..013 (proposed, V1-0986; GitHub #670) | `internal/jstestprovider/project_reporters.go`, `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `internal/jstestprovider/runner.go`, `internal/jstestprovider/projection.go`, `cmd/corvint-js-test-provider/main.go` | `TestExternalCommandDefaultConfigUnchanged`, `TestKeptReporterListResolution`, `TestKeptProjectReporterRunsBesideProvider`, `TestProjectReportersBindingShape`, `TestKeepReportersUnsupportedMode`, `TestKeepReportersFlagRequiresExternalServer`, `TestKeepReportersRetainedReceiptAccepted`, `TestQualifiedReporterSensitiveRedaction` (Node case `PWP-V0-011 keep mode reports kept entries`); live Playwright `NOT_RUN` (`docs/build-log/2026-10-07-gh670-playwright-keep-reporters.md`) |
| PWP-V1-001..008 | `internal/jstestprovider/application_attestation.go`, `internal/jstestprovider/external.go`, `cmd/corvint-js-test-provider/main.go`, `internal/testvaliditydoc/document.go` | `TestApplicationAttestationCommandProvider`, `TestApplicationAttestationNegativeControls`, `TestAttestedReceiptNeverPassesWrongOrRestartedApplication`, `TestApplicationAttestationDockerComposeQualification` |
| PWP-V2-001..006 | `internal/jstestprovider/sensitive_input.go`, `internal/jstestprovider/sensitive_input_boundary.go`, `internal/jstestprovider/sensitive_input_grammar.go`, `internal/jstestprovider/qualified-reporter.cjs`, `internal/jstestprovider/external.go`, `internal/testvaliditydoc/document.go`, `cmd/corvint-js-test-provider/main.go` | `TestQualifiedReporterSensitiveRedaction`, `TestSensitiveInputEvidenceRedactionAndValidation`, `TestSensitiveInputNormalizationBoundsAndNoPanic`, `TestSensitiveInputAlreadyRedactedRiskFieldsFailClosed`, `TestSensitiveInputReceiverPrefixExtraction`, `TestSensitiveInputUnicodeGrammarAndReportScope`, `TestSensitiveInputAlreadyRedactedCrossTestRiskRejected`, `TestSensitiveInputArgumentCandidatesRespectStructure`, `TestSensitiveInputPolicyGrammarAgreement`, `TestSensitiveInputUnsupportedReceiverSyntaxRejected`, `TestSensitiveRetainedPolicyAndUnsupportedActionRejection`, `TestSensitiveRetainedDecodeNeverEchoesUnknownProperties`, `TestSensitiveInputConformanceFixtureRejectsLeakAndAcceptsRedaction`; live Playwright matrix `NOT_RUN` |
| PWP-V3-001..006 | `internal/jstestprovider/attempt_details.go`, `external.go`, `qualified-reporter.cjs`, `internal/appflows/runingest.go`, strict consumer and provider CLI | `TestPWPV3AttemptInventory`, `TestPWPV3ProfileBoundaries`, `TestAFUV1012QualifiedReceiptIngest`, `TestAFUV1012QualifiedReceiptRefusals`, `TestQualifiedPlaywrightAttemptsLive`, `TestQualifiedPlaywrightAttemptsLiveDevicesSpread`; local live log `evidence/issues-167-175/pwp3-live-passed.log` |

The `/3` matrix passed locally on 2026-09-28. It retains failed-then-passed and infrastructure-then-assertion attempts. System Chrome 153.0.8010.54 was observed and refused; the older system tuple was NOT_OBSERVED in this run. `/3` does not inherit earlier-profile qualification.

### Owned emitted error codes

| Code | Emitted condition | Site |
|---|---|---|
| `application-attestation-config-invalid` | The provider config is noncanonical, has the wrong profile, or carries an invalid expectation. | `internal/jstestprovider/application_attestation.go:84@12af3b3a` |
| `application-attestation-config-unavailable` | The bounded provider config file cannot be read. | `internal/jstestprovider/application_attestation.go:80@b18eb77f` |
| `application-attestation-provider-drift` | The provider executable or configuration changes before post-run observation. | `internal/jstestprovider/external.go:186@db0a61bf` |
| `application-attestation-provider-required` | The attested profile lacks an absolute config path or provider argv. | `internal/jstestprovider/application_attestation.go:68@2bc06a44` |
| `application-attestation-provider-unavailable` | The provider executable is unresolved, unreadable, unstaged, or fails bounded execution. | `internal/jstestprovider/application_attestation.go:72@94973cec` |
| `application-attestation-requires-external-server` | Application attestation is requested outside external-server mode. | `internal/jstestprovider/runner.go:243@ae7d4bc6` |
| `attested-external-profile-has-declared-identity` | An attested receipt also carries the legacy caller-declared application identity. | `internal/jstestprovider/projection.go:97@d5db6c8b` |
| `external-attestation-conflicts-with-caller-identity` | The attested request also supplies legacy caller identity or build-directory input. | `internal/jstestprovider/external.go:374@2fb84e28` |
| `external-attempt-document-invalid` | The imported document has an unknown shape, trailing data, a profile other than `/3`, or bytes that differ from the rederived canonical qualified document. | `internal/jstestprovider/attempt_details.go`, `DecodeAttemptReceipt` |
| `external-profile-has-attempt-details` | An earlier `/0`, `/1` or `/2` receipt carries the `/3` `attemptDetails` member (AFU-V1-012). | `internal/jstestprovider/projection.go:78@d7cd2a65` |
| `file-bound-exceeded` | A bounded attestation input cannot be read within its byte ceiling. | `internal/jstestprovider/application_attestation.go:164@5714ccbf` |
| `file-replaced` | The opened attestation input is not the file that was inspected before opening. | `internal/jstestprovider/application_attestation.go:160@1bec3465` |
| `invalid-canonical-input` | Canonical input is empty, oversized, or secret-shaped. | `internal/jstestprovider/application_attestation.go:171@12f8b66b` |
| `legacy-external-profile-has-attested-fields` | A legacy `/0` receipt carries `/1` attestation fields. | `internal/jstestprovider/projection.go:90@751bcb11` |
| `noncanonical-input` | Parsed input bytes differ from the canonical JSON encoding. | `internal/jstestprovider/application_attestation.go:187@0bb50e72` |
| `not-regular` | An attestation input path does not resolve to a regular file. | `internal/jstestprovider/application_attestation.go:151@9133b825` |
| `test-repository-drift` | The test repository identity differs between start and publish. | `internal/jstestprovider/external.go:205@0fe9d240` |

## Issue340 exact project identity amendment

- **PWP-V3-007**: The `/3` exact project identity permits an explicitly observed empty project name
  (Playwright's unnamed default project). Keep file/full title/project/config/use/test ID joins exact;
  missing Project or ambiguous identities still abstain. Earlier profiles retain their existing empty
  name refusal. Qualification requires an actual unnamed-project passing test and declared failing
  control with retries0, plus the existing interruption/runner descendant cleanup regression.
  Consumer coverage must check configured retries in Schedule separately from observed retry count.
  This does not compose `/3` with attested `/1` or promote deployment provenance: immutable consumer
  byte binding and caller-declared app identity remain distinct from an observed application attestation.

Regression: `TestProfileAttemptEmptyProject`; actual live evidence remains required for the changed
project admission boundary. Issue340 does not change the qualified runtime tuple or earlier profiles.
