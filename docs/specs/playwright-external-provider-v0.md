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
- Exists: PWP-V0-153..154 (experimental; V1-2913) name a complete external receipt's run execution `external-run-execution-per-test` and its freshness `external-application-currency-unverified` in test-validity. PWP-V0-090..093 (experimental; V1-2844) run a declared `--preflight` (http readiness, setup/seed command and storage-state probes with timeouts) before the runner; a failure writes a `preflight` receipt member and `phase: "preflight"` infrastructure naming the probe and artefact, with no test verdicts, and the run projection anchors name them. PWP-V3-013..016 (experimental) keep the launch identity for a test-type page/context fixture override that resolves through Playwright's own browser fixture, record a test that used no browser as `browser: "none"` with a closed reason that qualifies on the runner tuple alone, and name a remaining `project-location-unknown` refusal's tests by JSON pointer on a separate stderr line. PWP-V3-011..012 (experimental) stop the qualified-document secret screen reading provider-written path keys, test titles and `file:line` anchors as credential assignments, and name a remaining `qualified-document-secret-shaped` refusal's member as a JSON pointer on a separate stderr line. PWP-V3-010 applies the native row's run-evidence hygiene to `/0`, `/1` and `/3` failure messages and artifacts before projection (a timed-out action's browser launch line no longer refuses the document). PWP-V3-007 admits exact unnamed projects only in `/3`; no attestation composition is added. PWP-V0-019 names each unbound reported anchor file (for example a setup-project file) in the `test-file-unbound` detail and never binds it implicitly. PWP-V0-022..027 report a run's dependence on undeclared external origins from producer-stated network evidence (`corvint-js-test-provider external-origins`; no producer yet). PWP-V0-021 admits up to 4096 config inputs and 1 MiB of their paths and digests, and refuses beyond either as `config-inputs-overflow` stating the observed value and the limit. PWP-V0-028..031 truncate (never fail on) external console output past 4 MiB with an optional `--runner-output` tee, record `external.runner` (exit status, signal, stop signal), stop a timed-out external runner SIGINT-first with `--interrupt-grace`, and state both readiness rules in the `--server-ready-url` help. PWP-V0-032..034 qualify Playwright 1.64.0 for `/0` and `/3` on Node v22.23.2 with its own exact bundled headless-shell tuple, and write a typed `corvint-playwright-e2e-refusal/0` document naming the qualified runner versions on stdout when an `e2e` request is refused before running. PWP-V0-035..038 add, each on its own live run, `/1` and `/2` on 1.64.0 Node v22.23.2, Node v24.11.1 for `/0` and `/3` on 1.64.0, the exact system Chrome 155.0.8059.85 tuple on 1.64.0, and exit status 3 plus one `refusal:` stderr line for an `e2e` admission refusal. PWP-V0-039..040 admit receipts and freshness from one exact-tuple table and name the failing tuple field; PWP-V0-050..052 add freshness on 1.64.0 / Node v22.23.2 (bundled) after its own live run, so that one tuple satisfies both receipts, stream-digest a freshness command executable up to 256 MiB, and pass the reporter its freshness rows from the table. PWP-V0-053..054 make freshness acceptance read its admitted runners from the table and check the reporter's external Node pre-filter against it. `internal/jstestprovider`, `internal/procgroup`, `cmd/corvint-js-test-provider`, `internal/testvaliditydoc`, `internal/testacceptance`.
- Read next: Requirements; Wire and trust boundary; Acceptance and rollback.
- Blocked on: no implementation gap; owner-selected checks and separate live witnesses govern final completion. Proposed PWP-V0-009 (Node v24.11.1, `/0` bundled tuple) awaits owner acceptance. Proposed PWP-V0-010..013 (keep-reporters option, V1-0986) await owner acceptance; their live macOS run is recorded (V1-1002). PWP-V0-014..018 (provider-first order and the `qualify-keep-reporters` command) are accepted (decision 0461; V1-1028); a live in-process qualification passed on 1.64.0 / Node v24.11.1 after PWP-V0-055 (V1-1002), and the CLI path and Linux are `NOT_RUN`. Linux, `/1` and `/2` on Node v24.11.1, other Playwright versions, Vitest and LPCV authority remain unqualified (`/2` never projects passing, PWP-V2-006). The qualification host had Docker but no Compose frontend, so the checked-in closed Compose JSON manifest was executed by the fixture's equivalent project-scoped Docker build/run path.

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

- `PWP-V0-010`: (proposed (V1-0986; GitHub #670)) The opt-in `--keep-reporters` option, available only in external-server mode on `/0` to `/3`, keeps the project's original `reporter` list beside the provider reporter instead of replacing that list; `PWP-V0-014` fixes the order. The original value is normalized: undefined keeps no entries, a string keeps one entry, and an array keeps entries that are each a name or `[name]` or `[name, options]`. Any other value refuses `project-reporters-invalid` when the controlled config loads. A name beginning with `.` or an absolute name resolves against the original config directory. Any other name that is not one of the built-in reporters (`blob`, `dot`, `github`, `html`, `json`, `junit`, `line`, `list`, `null`) resolves through Node module resolution from that directory, or stays unchanged when it cannot be resolved. The built-in `blob`, `html`, `json` and `junit` reporters receive the original config directory as `configDir` unless the project set one, so their default and environment-named outputs stay where the project would put them. Their string `outputFile`, `outputFolder` and `outputDir` options resolve against the original config directory. No other option value is changed. Without the option, the controlled config and the receipt bytes are byte-identical to the replace-only form. An owned-server or freshness run with the option refuses `keep-reporters-unsupported-mode`.
- `PWP-V0-011`: (proposed (V1-0986; GitHub #670)) A keep-reporters receipt binds `receipt.external.projectReporters` with two members. `entries` lists every kept entry in order, at most 32. `effects` is described in `PWP-V0-012`. Each entry carries its resolved `name`, a `module` value and an `options` value:
  - `module` is `builtin` for a built-in reporter. It is `bound` when the exact module path is a reporter-observed config input; its `moduleDigest` must then equal that input's digest, so `PWP-V0-002` drift detection covers it. Otherwise it is `unknown`.
  - `options` is `absent` when the entry has no options. It is `bound` when the options are plain JSON data; the entry then carries `optionsDigest`, the SHA-256 of the sorted-key canonical JSON. It is `unknown` for functions, class instances, cycles, non-finite numbers, array holes or depth 32 and beyond.

  Option values themselves are never retained. The retained controlled config (`PWP-V0-002`) identifies keep-reporters mode.
- `PWP-V0-012`: (proposed (V1-0986; GitHub #670)) The provider never observes what a kept reporter does: files written, output, reporter errors or mutation of shared result objects. `effects` is therefore always `unknown`, which is recorded and never trusted. A keep-reporters receipt therefore never projects passing execution unless it carries a matching qualification record (`PWP-V0-016`); without one it stays retained, decodable and readable, as with `/2`. The binding can only remove passing authority, never add it. It must agree with the controlled config in both directions and use only the closed values above. Any violation refuses `project-reporters-invalid` at encoding and prevents a passing projection. A keep-reporters run whose reporter did not report the kept entries is the infrastructure failure `project-reporters-unobserved`. Unobserved (`null`) entries are valid only beside a run-level infrastructure failure. A replace-only report that carries entries is `report-unparseable`. The qualified runtime tuple (`PWP-V0-008`, `PWP-V0-009`), the `/2` sensitive-input policy and redaction, the `/3` attempt details and every existing passing prerequisite are unchanged. Kept-reporter output counts toward the 4 MiB output bound.
- `PWP-V0-013`: (proposed (V1-0986; GitHub #670)) Test-validity decoding and MCP discovery accept a canonical keep-reporters receipt on the same terms as a replace-only receipt and recompute its projections. A changed binding fails the canonical or shape check. Non-goals:
  - ingesting the outputs of kept reporters (foreign JSON or blob reports)
  - qualifying any kept reporter
  - changing which reporters run without the option

  Known limits:
  - A kept custom reporter that reads `config.configFile` sees the controlled config path.
  - ESM, `node_modules` or symlinked reporter modules may record `module` as `unknown`.
  - Live Playwright qualification of the option is `NOT_RUN`. It is unverified that Playwright 1.60 to 1.63 lets the injected `configDir` override its own, and that `FullConfig.reporter` reaches `onBegin` in the normalized shape. If either fails, outputs land in scratch or the run fails closed as `project-reporters-unobserved`.

  Owner questions:
  - This adds an opt-in member to the existing profiles rather than a new profile revision (see Acceptance and rollback).
  - The provider reporter runs last, after kept reporters could alter shared result objects. Should it run first, so that a qualified keep-reporters mode could later project passing? Proposed answer: yes, `PWP-V0-014`.
  - `optionsDigest` of a low-entropy secret option value would be guessable.

### Keep-reporters qualification (accepted (decision 0461); V1-1028; GitHub #686)

Intent: an adopter who must keep the project's own reporters can record, on the adopter's own host, local
evidence that the kept reporters did not change what the provider observed, and so get a passing
keep-reporters receipt on that exact runtime and reporter set.

- `PWP-V0-014`: (accepted (decision 0461; V1-1028; GitHub #686)) In keep-reporters mode the provider reporter is the first
  entry of the controlled `reporter` list, followed by the kept entries in their original order
  (`reporter: [[provider, {...}], ...keptReporters(original.reporter)]`). Playwright calls reporters in list
  order, so the provider copies every value it reports from `onBegin`, `onTestEnd` and `onEnd` before a kept
  reporter receives the same callback. The provider reports the kept entries after its own first entry. A
  retained receipt whose controlled config has the earlier provider-last order stays readable and never
  projects passing.
- `PWP-V0-015`: (accepted (decision 0461; V1-1028; GitHub #686)) `corvint-js-test-provider qualify-keep-reporters` takes the
  external-server e2e flags. It refuses watch mode, `--keep-reporters`, `--keep-reporters-qualification` and
  any run without `--external-server`, before any process starts. It runs the configuration once
  replace-only (the control run) and, only when the control run completed, once in keep-reporters mode (the
  keep run). It compares the two receipts and prints one canonical record with profile
  `corvint-playwright-keep-reporters-qualification/0` on stdout. It exits zero only for verdict `qualified`.
  With `--retain` each produced receipt is retained as its canonical document, and the record's
  `controlReceiptSha256` and `keepReceiptSha256` are the SHA-256 of those bytes. The verdict is `qualified`
  only when all of these hold:
  - Both runs are complete: no error, no run-level infrastructure failure, not cancelled, an external
    lifecycle, project reporters bound only in the keep run, and at most 256 config inputs, each an absolute
    path with a 64-hex digest.
  - The keep run's config is provider-first and every kept entry is observed with neither `module` nor
    `options` `unknown`.
  - Profile, runner and Node versions, config file and digest, test-file digests, package digest and declared
    environment are equal. Every control config input has the same digest in the keep run. Every config input
    of the keep run is recorded as `configInputs`, including the kept reporter modules and any file that both
    the config and a kept reporter load.
  - Every control outcome is fully qualified and every keep outcome meets every prerequisite except the
    keep-reporters one.
  - At least one outcome exists, and the outcomes correspond one to one by project name, full name and anchor.
    For each pair, name, state, retries, each attempt's state, retry and failure kind, project browser,
    device, config digest and `use`, and the artifact count are equal. Test IDs are not compared, because
    they bind the config inputs.
- `PWP-V0-016`: (accepted (decision 0461; V1-1028; GitHub #686)) `e2e --keep-reporters --keep-reporters-qualification FILE`
  reads at most 256 KiB, decodes the record strictly and refuses any record whose verdict is not
  `qualified`. Without `--keep-reporters` it refuses `keep-reporters-qualification-requires-keep-reporters`.
  The receipt carries the record unchanged as `receipt.external.projectReporters.qualification`. A carried
  record must be valid and qualified and sit beside a provider-first config, or encoding refuses
  `project-reporters-invalid`. A keep-reporters receipt projects passing only when it carries a qualification
  that matches it exactly and every existing prerequisite holds. "Matches exactly" means the same receipt
  profile, runner version and Node version, entries equal in order, and config inputs exactly equal to
  `configInputs`: no input changed, added or removed. Any change to the runtime, the reporter set, options, the
  config or any file it or a kept reporter loads abstains until the adopter qualifies again. The record never admits a
  runtime tuple: `PWP-V0-008` and `PWP-V0-009` still decide which runtimes are qualified (decision 0448).
- `PWP-V0-017`: (accepted (decision 0461; V1-1028; GitHub #686)) The record is closed. It has exactly these members:
  `profile`, `verdict`, `reasons`, `receiptProfile`, `runnerVersion`, `nodeVersion`, `entries`,
  `configInputs`, `tests`, `controlReceiptSha256` and `keepReceiptSha256`, in that order.
  - `reasons` is a sorted, unique array from the closed set below, and it is empty exactly when the verdict
    is `qualified`. A `not-run` reason makes the verdict `not-run`; otherwise any reason makes it
    `not-qualified`.
  - `entries` uses the `PWP-V0-011` entry shape, or is `null` when the keep run did not observe it.
  - `configInputs` holds at most 256 absolute paths, each with a 64-hex digest.
  - A `qualified` record additionally needs an external receipt profile, both versions, at least one test,
    both receipt digests, no `unknown` entry, and each `bound` module's digest equal to its `configInputs`
    value.
  - Encoding refuses a secret-shaped record. Decoding accepts only the exact canonical bytes with one
    trailing newline, and every refusal is the value-free `keep-reporters-qualification-invalid`.

  Reasons, `not-run`: `keep-reporters-control-run-incomplete`, `keep-reporters-keep-run-incomplete`.
  Reasons, `not-qualified`: `keep-reporters-control-unqualified`, `keep-reporters-keep-unqualified`,
  `keep-reporters-order-unsupported`, `keep-reporters-entries-unknown`, `keep-reporters-inputs-differ`,
  `keep-reporters-observation-differs`, `keep-reporters-no-tests`.
- `PWP-V0-018`: (accepted (decision 0461; V1-1028; GitHub #686)) Missing evidence never becomes a qualification. Each of
  these is reported on stderr and records `not-run` with a named incomplete-run reason, never `qualified`:
  - missing Playwright, an unqualified runtime or any other control-run failure (the keep run is then not
    started and is reported as `keep-reporters-keep-run-skipped`)
  - a kept reporter that refuses to load or makes the keep run fail
  - unobserved entries, or config inputs the record cannot bind
  - a cancelled, interrupted or partial run

  A mismatch is `not-qualified`. The command never retries, never infers a missing run and never signals
  processes it did not start.

Adopter steps: run `qualify-keep-reporters` with the same flags as the intended e2e run and save stdout as the
record. Then run `e2e --keep-reporters --keep-reporters-qualification <record>` with those flags. After any
runtime, reporter, option or reporter-file change, qualify again.

Non-goals: admitting Node or Playwright tuples; observing or trusting kept reporters' effects (`effects`
stays `unknown`); ingesting kept reporters' outputs; qualifying ESM, `node_modules` or other reporters whose
module stays `unknown`; a shared or signed qualification registry.

Known limits:
- Provider-first order protects only the values of the callback the provider has already copied. Mutation of
  objects that persist across callbacks is caught only by the differential comparison.
- Suites whose outcomes are not repeatable fail closed as `keep-reporters-observation-differs`.
- The record is local evidence with the same trust as a retained receipt. It proves that one pair of runs
  matched, not that a kept reporter can never interfere.
- The command, the flag and the passing keep-reporters projection are experimental while these requirements
  are experimental (intent accepted by decision 0461); they are not promoted or advertised as
  delivered until a live `qualified` record on a qualified tuple.
- Live Playwright qualification of the command is `NOT_RUN` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-08-gh686-playwright-keep-reporters-qualification.md`).

Owner questions:
- Answered by decision 0461: accept provider-first order (`PWP-V0-014`) as the answer to the `PWP-V0-013` ordering question.
- Decision 0448 rejected consumer-side qualification records that add Node tuples. This record adds no tuple
  and qualifies only reporter non-interference on an already qualified tuple. Is that within 0448's intent?
- Is a local, unsigned record enough authority for a passing keep-reporters projection, given that it has
  the same trust as the receipts it binds?

### Setup-project file binding (accepted under the owner's standing auto-accept (2026-10-11); V1-1242; GitHub #807)

Intent: an adopter whose Playwright config uses project dependencies (setup projects) learns which
reported files to bind, instead of an unexplained refusal of a run whose results all passed.

- `PWP-V0-019`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1242; GitHub #807)
  In external-server mode the `--test-arg` tokens select what Playwright runs and bind no file
  identity; only `--test-file` binds a test file (`PWP-V0-002`). Every file Playwright reports a
  result for must be bound, including the setup files that project dependencies run before the
  selected spec. A reported anchor file that is not bound refuses `report-identity-unknown` with the
  detail `test-file-unbound: ` followed by each distinct unbound anchor file, sorted bytewise and
  separated by `; `, each with its cause in parentheses: `not passed with --test-file`,
  `not digested by the reporter`, or `reporter digest differs from the --test-file digest`. At most
  8 files are named; any further files are summarized as `+N more`. The `/2` profile names no file:
  its detail is the fixed untrusted-output text, which `/2` risk-field redaction may further replace
  with `[REDACTED]` (`PWP-V2-003`, `PWP-V2-005`). Owned-server (non-external) e2e runs do not apply
  this binding check. The provider never binds a reported file implicitly and
  the refusal stays a refusal even when every result passed. `corvint-js-test-provider e2e` help
  states this rule.

Non-goal: an option that binds dependency-project files reported by Playwright. Such a file would be
observed only once, by the reporter's own digest, whereas a `--test-file` is digested by the provider
before and after the run (`input-identity-changed`); adding it would widen binding beyond that
observation. Failure mode addressed: a caller passes the target spec to both `--test-arg` and
`--test-file` and cannot tell which file is unbound. Rollback restores the bare `test-file-unbound`
detail and the earlier help text; retained receipts carrying a named detail stay readable because the
detail is free text, and none is deleted or rewritten.

### Runner timeout wording (accepted under the owner's standing auto-accept (2026-10-11); GitHub #832)

Intent: an operator whose run timed out learns which runner hit which bound and which flag raises it.

- `PWP-V0-020`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #832) A runner process that exceeds its bound is the
  infrastructure failure `timeout`, and its detail names the runner and the effective `--timeout`
  value: `<runner> did not complete within the --timeout <value> bound`, where `<value>` is the Go
  duration form (for example `5m0s`) of the bound actually applied, including the default when the
  flag is unset or not positive. The runner is `playwright test` in both external-server and
  owned-server e2e runs and `vitest` in unit runs. Only the detail changes: every `Reason` code, the
  classification order and the cancellation handling are unchanged, and an external-server run
  classifies a cancelled observation exactly as before.

Failure mode addressed: an external-server Playwright timeout was reported as `vitest did not
complete within the bound`. Rollback restores the earlier fixed details; retained receipts carrying
either wording stay readable because the detail is free text.

### Config-input bounds (accepted under the owner's standing auto-accept (2026-10-11); GitHub #844; V1-1317)

Intent: a suite whose module graph loads more than a few hundred project modules still binds, and a
suite above the bound learns that its config inputs, not its report output, were refused.

- `PWP-V0-021`: (accepted under the owner's standing auto-accept (2026-10-11); GitHub #844; V1-1317) The reporter-observed config-input set
  (`receipt.identity.configInputDigests`, every non-`node_modules` module in the runner's module
  cache) admits at most `externalMaxConfigInputs`, 4096, entries and at most
  `externalMaxConfigInputBytes`, 1 MiB, summed over each entry's path and digest lengths. The count
  replaces the earlier 256, which refused a mid-size suite after two added support modules; 4096
  covers a suite whose specs, page objects, fixtures and helpers each load as modules, and every
  entry is still re-read and digested at bind time. The byte bound keeps the set at a quarter of the
  4 MiB provider-document bound (`internal/testvaliditydoc` `MaxInputBytes`) that reads the
  retained receipt. A set above either bound refuses `report-identity-unknown` with the detail
  `config-inputs-overflow: observed <N> config inputs, limit 4096` or
  `config-inputs-overflow: observed <N> config input bytes, limit 1048576`; the count is checked
  first, and both are checked before `config-inputs-unobserved`. `report-output-overflow` names
  only a report file above the 4 MiB output bound. The `/2` profile names no count: its detail is
  the fixed untrusted-output text. A retained freshness receipt is checked against the same two
  bounds. The keep-reporters qualification record keeps its own 256-input bound (`PWP-V0-017`),
  which fits its 256 KiB record bound, so a run above 256 config inputs cannot be recorded as
  keep-reporters qualified.

Failure mode addressed: a mid-size suite was refused as `report-output-overflow` while its report
was small. Rollback restores the 256 count and the `report-output-overflow` detail and removes the
byte bound. Retained receipts stay readable because the detail is free text; a retained freshness
receipt with more than 256 inputs then abstains, and no receipt is deleted or rewritten.

### External-origin dependence (accepted under the owner's standing auto-accept (2026-10-11); V1-1762; GitHub #1106)

Intent: an operator whose browser tests passed or failed with a resource the test environment does
not control (a module from a live public CDN, a third-party script, an external API) learns from a
report, not from a hand-opened trace, which external origins the run depended on, which test first
depended on each, and which external requests differed between a passing and a failing run of the
same test at the same revision. A project accepts an external origin explicitly by allow-listing it
as deterministic, or removes the dependence by vendoring or mocking the resource.

- `PWP-V0-022`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1762; GitHub #1106) Network evidence is the closed `corvint-network-evidence/0`
  document of one run: `profile`, a `run` label, the full lower-case Git object id `revision` of the
  test repository, and `tests` in run order, each with `file`, full `title`, `outcome` (`passed`,
  `failed`, `timedOut`, `interrupted` or `skipped`) and `requests`. A request states `url`,
  `resourceType` (`document`, `script`, `module`, `stylesheet`, `websocket`, `fetch`, `xhr`,
  `beacon`, `image`, `font`, `media` or `other`), `status` (0 when no response arrived, else
  100..599), `durationMs`, `followUps` (the requests its response initiated) and, for `fetch` and
  `xhr` only, `awaited`. `status`, `durationMs` and `followUps` are required and non-null, so an
  absent observation never decodes as 0. Unknown members, trailing data, a repeated test in one run, a URL with
  userinfo or a scheme other than http, https, ws, wss, data, blob or about refuse
  `network-evidence-invalid`, as does a document above 4 MiB, 4096 tests, 1024 requests per test or
  1024-byte text, as does a port above 65535. A URL reduces to its canonical origin (lower-case
  scheme and host, an IP host in its canonical address form such as compressed IPv6, the port as a
  decimal number without leading zeros, default port elided) and a locator without query or fragment, so no query value reaches the report; data:,
  blob: and about: URLs have no origin and are not listed. The evidence is producer-stated: Corvint
  makes no network request and observes no browser to produce it.
- `PWP-V0-023`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1762; GitHub #1106) A request to an origin outside the declared environment is
  `blocking` when its type is `document`, `script`, `module`, `stylesheet`, `websocket` or `other`,
  or `fetch`/`xhr` unless stated `awaited: false`; it is `incidental` when its type is `beacon`,
  `image`, `font` or `media`, or a `fetch`/`xhr` stated not awaited. `other` is blocking because its
  dependence is unknown. Incidental traffic is listed and counted but raises no finding, so a run
  whose only external traffic is analytics beacons is `CLEAN`.
- `PWP-V0-024`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1762; GitHub #1106) The report lists every origin outside the declared environment
  once, in byte order, with its class (`undeclared` or `allowed`), its blocking and incidental
  request counts and the resource types seen. Each undeclared origin with a blocking request yields
  one `external-origin-dependence` finding per resource type, naming the origin, the resource type,
  the first run and first test (input run order, then run order) that made such a request, and the
  number of distinct tests that did.
- `PWP-V0-025`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1762; GitHub #1106) The project policy is the closed `corvint-external-origins-policy/0`
  document: `environment`, the canonical origins of the application under test and its declared
  mocks, whose requests are never listed; and `deterministic`, the allow-list of external origins
  the project accepts as deterministic (for example pinned content-hashed assets), each with a
  non-empty `reason`. Request origins are matched against the policy only after the canonical
  reduction of `PWP-V0-022`. An allow-listed origin is listed with class `allowed` and its reason and
  raises no finding. A non-canonical origin (a path, upper case, a default or zero-padded port, or
  an expanded IP address), an origin
  declared twice or in both lists, a missing reason, unknown members, more than 256 origins or a
  document above 64 KiB refuse `external-origins-policy-invalid`.
- `PWP-V0-026`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1762; GitHub #1106) For each test (file and title) that passed in one input run and
  failed or timed out in another at the same revision, the report pairs its first passing run with
  its first failing run and compares their external requests, keyed by resource type and locator
  (first occurrence per key). Each differing request is named with its origin, locator, type, both
  observations and its kinds: `missing-in-failed`, `missing-in-passed`, `status-differs`,
  `follow-ups-differ`, or `slower-in-failed` when the failing duration exceeds the passing one by at
  least 1000 ms. The pair's state is `EXTERNAL_DIFFERENCE`, `NO_EXTERNAL_DIFFERENCE`, or `UNKNOWN`
  with reason `network-evidence-absent` when either side lacks evidence. Runs at different
  revisions are never paired.
- `PWP-V0-027`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1762; GitHub #1106) `corvint-js-test-provider external-origins --policy FILE
  --network FILE...` (1 to 16 runs, in run order) prints the closed `corvint-external-origins/0`
  report: `verdict`, per-run `network` state (`RECORDED`, or `NOT_PRODUCED` when the run has no
  test or any test lacks evidence; each run's state is its own and another run never clears it), `origins`, `findings`, `unknown` tests and `correlations`. A test whose `requests` is
  absent or null is listed in `unknown` with reason `network-evidence-absent`; an empty array states
  that no request was made. The verdict is `ENVIRONMENT_DEPENDENT` when any finding exists, else
  `UNKNOWN` when any run is `NOT_PRODUCED`, else `CLEAN`: missing network evidence
  never yields `CLEAN`. A refusal writes its code to stderr and exits 2. The report is diagnostic:
  it never rewrites a receipt, the five-axis projection or the passing predicate.

Non-goals: no producer in this version (a Playwright fixture or reporter that records each test's
requests, or a HAR/trace importer, is follow-up work), no live browser run, no join into
`corvint test-validity` or the retained receipt, no inference of which assertions read a resource,
and no network access by Corvint. Failure modes addressed: a bulk failure caused by a live CDN
module that rendered nothing read as a product regression or flaky locator; an analytics beacon
raising a false dependence; a run without network evidence reading as clean. Acceptance evidence:
the fixture-driven Go tests in Traceability; live Playwright evidence is `NOT_RUN`. Origin parsing
imports `net`, `net/netip` and `net/url`, so it lives in its own package,
`internal/testvaliditydoc/externalorigins`, which only the JS test provider imports; the MCP tool
path's `internal/testvaliditydoc` stays free of network packages (`MTV-V0-007`). Rollback removes
the subcommand, the three profiles and that package; no receipt, projection or retained document
changes, and policy files left in a project are inert.

### Runner output, exit record and interrupt-first timeout (accepted under the owner's standing auto-accept (2026-10-11); V1-1800, V1-1802; GitHub #1131, #1133)

Intent: an operator whose external-server run is noisy, slow or stuck can read what the runner
printed, learns how the runner ended, and gets Playwright's own interrupted report and cleanup
before the provider escalates to termination. The observation of tests still comes only from the
report file.

- `PWP-V0-028`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1800; GitHub #1131) In external-server mode the runner's console output
  (stdout and stderr) is captured to `externalOutputLimit`, 4 MiB, per stream and truncated beyond
  it: the run continues and is never the infrastructure failure `output-overflow` for console
  volume. The report file keeps its own 4 MiB bound and `report-output-overflow`. `e2e
  --runner-output PATH` (also `qualify-keep-reporters`) writes every console byte of every runner
  process the invocation starts, untruncated and in arrival order per stream, to PATH;
  `--runner-output -` passes it through to the provider's stderr, so the printed document on
  stdout is unchanged. PATH must be a regular file: it is opened without following a final symlink
  and without blocking on a FIFO, any non-regular entry refuses `runner-output-unavailable`, and an
  existing file is set to mode `0600` and truncated (a new one is created `0600`). Each stream's tee
  is written by its own forwarder from a queue of at most 64 drain reads, after the bounded capture
  has the bytes, so the sink never holds the capture or the other stream: a sink that accepts no
  write for one second ends that tee early, and the run waits at most two seconds after its
  drains for the forwarders. Across the two streams the sink sees forwarding order, not the
  child's order. A tee write failure, such a stall or an unflushed end stops the tee only: the run
  and its receipt are unaffected and the provider writes `runner-output-incomplete` to stderr.
  Without the flag nothing is written beyond the bounded capture. The owned-server and unit modes keep their
  fail-on-overflow capture.
- `PWP-V0-029`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1800, V1-1802; GitHub #1131, #1133) Every external-server receipt carries
  `external.runner`, the closed process record of the `playwright test` runner: `exitObserved`
  (the provider reaped the runner), `exitStatus` (its exit code, -1 when not observed or ended by a
  signal), `signal` (the wait-status signal that terminated it, for example `interrupt` or
  `killed`, else empty), `stopSignal` (the last of `SIGINT`, `SIGTERM` or `SIGKILL` that the
  provider sent to the runner's owned process group before the runner's exit was observed, else
  empty: the runner ended on its own; a signal is recorded when the exit was not yet observed as
  it was sent, so a signal that makes the runner exit at once is still recorded) and `outputTruncated` (console capture passed its bound,
  `PWP-V0-028`). The record is diagnostic: it never changes a test state, the infrastructure
  reason or the projection. A receipt without the member (written before it) stays readable and
  canonical. The member is an additive exception to the profile-revision rule, like
  `projectReporters`: a reader that predates it fails closed through the canonical re-encoding check.
- `PWP-V0-030`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1802; GitHub #1133) When an external-server runner exceeds `--timeout`, the
  provider first sends `SIGINT` to the runner's owned process group, as an interactive Ctrl-C
  would, and waits up to `--interrupt-grace` (default `10s`) for the runner to exit; only then does
  the existing stop follow: `SIGTERM`, a 100 ms grace and `SIGKILL`. A run that ends during the
  interrupt grace keeps the infrastructure failure `timeout` (`PWP-V0-020`) and its report is read
  as before; a run-level report status `interrupted` from that run is the provider's own timeout
  interrupt and stays the `timeout` infrastructure failure, never a cancellation (V1-1812; only
  an interruption the provider did not cause projects as `INTERRUPTED`/`CANCELLATION`).
  `--interrupt-grace 0` restores the earlier `SIGTERM`-first stop; a negative value
  refuses `interrupt-grace-invalid`. Cancellation of the provider itself ends the grace at once.
  The process-group primitive (`internal/procgroup` `Spec.InterruptGrace`) is opt-in: every other
  caller, and an external run configured without a grace, keeps its earlier stop sequence.
  `--runner-output` and an explicit `--interrupt-grace` without `--external-server` refuse
  `runner-control-requires-external-server`.
- `PWP-V0-031`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1802; GitHub #1133) The `--server-ready-url` help states both readiness rules
  the code applies: an owned server (`e2e` without `--external-server`) is ready on any status
  below 500, following redirects; an external server is ready only on a 2xx answer and a redirect
  is not followed. Readiness behaviour is unchanged.

Non-goals: no live Playwright run, no change to the report-file bound, the passing predicate or any
profile's projection, no interrupt-first stop in owned-server or unit mode, and no readiness change.
Failure modes addressed: a runner that printed more than 4 MiB was killed and reported
`output-overflow` although its report was small; an operator could not see what the runner printed
or how it exited; a timeout `SIGTERM` interrupted Playwright before it could write its interrupted
report and stop its own workers; the readiness help promised a rule external mode does not apply;
(Codex round 1, V1-1800) a stopped `--runner-output -` reader held the drains past the run, a
symlinked or FIFO PATH was truncated through or blocked the open, an existing log kept a
permissive mode, and a signal that ended the runner at once was not recorded.
Acceptance evidence: the fake-runner Go tests in Traceability. Rollback removes the two flags, the
`runner` member and `Spec.InterruptGrace`/tee hooks, and restores fail-on-overflow capture; receipts
carrying `runner` then fail closed through the canonical check and are neither deleted nor rewritten.

### Playwright 1.64.0 and typed version refusal

- `PWP-V0-032`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2005; GitHub #1263) External mode admits the exact
  runner version `1.64.0` beside `1.60.0` and `1.63.0`; any other spelling (`1.64`, `1.64.1`,
  `v1.64.0`, `^1.64.0`, `1.65.0`) refuses `external-playwright-version-unqualified`. The bundled
  browser tuple is keyed by the exact runner version and never inherited across versions: `1.63.0`
  keeps registry `chromium-headless-shell` revision `1243` unchanged, and `1.64.0` admits only
  revision `1248`, manifest version `156.0.8078.4`, observed version
  `Google Chrome for Testing 156.0.8078.4`, path suffix
  `chromium_headless_shell-1248/chrome-headless-shell-mac-arm64/chrome-headless-shell` and executable
  SHA-256 `77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634`, headless, with no
  configured executable. A 1.64.0 run on the 1.63.0 tuple, or the reverse, abstains as an
  unqualified browser tuple. The configured system-Chrome tuple (`PWP-V0-008`) stays qualified on
  `1.63.0` with its own version; `1.64.0` admits only the version named by `PWP-V0-037`.
- `PWP-V0-033`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2005; GitHub #1263) `ReceiptRuntimeTuple` classifies a
  `1.64.0` receipt as a candidate only for the base `/0` and attempt `/3` profiles on Node
  `v22.23.2`; `PWP-V0-035` and `PWP-V0-036` add the tuples their own live runs observed, and every
  other profile/Node pair reads `unqualified`. The per-test freshness profile (`PTF-V0`) stays on `1.63.0`
  with Node `v22.23.3` (and, since `PWP-V0-050`, `1.64.0` with Node `v22.23.2`). Step negation reads the same classification, so a `/0` receipt on the
  `1.64.0` tuple is a negation candidate exactly when the per-test bundled tuple also matches.
- `PWP-V0-034`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2005; GitHub #1263) When the one-shot `e2e` command refuses a
  request with a closed refusal code before any server, runner or browser starts (an unqualified
  runner version, a missing external-mode input, an unsupported option, or a CLI admission refusal:
  `interrupt-grace-invalid`, `runner-control-requires-external-server` or
  `runner-output-unavailable`), the provider writes one `corvint-playwright-e2e-refusal/0` document to stdout instead of an empty one: the
  closed members `schema`, `code` (the refusal code, unchanged), `message` (the earlier error text)
  and `qualifiedRunnerVersions` (the exact versions external mode admits, in order), plus the
  `runtimeTuple` member for a tuple refusal (`PWP-V0-040`), encoded as compact JSON plus LF. The version refusal message names those versions:
  `external-playwright-version-unqualified: qualified runner versions: 1.60.0, 1.63.0, 1.64.0`.
  The exit status and stderr line are those of `PWP-V0-038`. The document is
  not a receipt, is never retained or read as evidence, and an infrastructure failure after the run
  starts, `watch` and `qualify-keep-reporters` write nothing new to stdout. Free-text usage errors
  without a closed code (a malformed application-attestation argv or config, sensitive-policy
  additions without `--sensitive-input-redaction`, path resolution, a keep-reporters qualification
  record that cannot be read, flag parsing) keep an empty stdout, because the document's `code`
  member admits only closed codes. When writing the document fails or is short, the original
  refusal stays on the stderr `refusal:` line, followed by
  `; refusal-output-incomplete: <write error>`; the exit status is still that of `PWP-V0-038`.

Non-goals: no `/1`, `/2`, Node 24, system-browser or Linux qualification on `1.64.0`, no semver
range admission, no change to any receipt member, profile, projection or the freshness profile, and
no new exit status. Failure modes addressed: a consumer pinned to `@playwright/test@1.64.0` could not
run external mode at all; the refusal left stdout empty, so a consumer saw no document and no list of
versions it could pin to; a version table that shared one bundled tuple would have let a 1.64.0 run
bind on 1.63.0 browser identity. Acceptance evidence: the unit tests in Traceability, plus the
retained live matrix on 1.64.0 and the 1.63.0 regression run
(`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-2005-playwright-164-qualification.md`). Receipts that built before
are byte-identical: no receipt struct or encoding changed and the 1.63.0 tuple values are
unchanged. Rollback removes `1.64.0` from the version list, its bundled tuple and its runtime-tuple
case, and the refusal document writer; receipts written on `1.64.0` then read `unqualified` and are
neither deleted nor rewritten.

### Application-attested revision

- `PWP-V1-001`: `/1` is available only in external-server mode and requires a typed `corvint-application-attestation-command/0` provider and canonical `corvint-application-attestation-config/0`; the `/0` caller label cannot select `/1`, `/0` remains readable, and neither profile can carry the other profile's identity fields.
- `PWP-V1-002`: The closed provider configuration binds the expected application root commit, revision, tree, dirty policy and optional dirty digest, build/image kind and digest, configuration/Compose kind and digest, and instance kind.
- `PWP-V1-003`: Each closed canonical `corvint-application-attestation/0` observation binds the actual application root commit, revision, tree, dirty state/digest, build/image, configuration/Compose, instance/container ID, start generation and health state.
- `PWP-V1-004`: The receipt binds the provider profile, normalized argv, original executable path and SHA-256, canonical configuration path and SHA-256, declared environment, and the SHA-256 of each exact canonical output. The provider executable is launched from a private content copy and its original executable/configuration are rechecked before the post observation.
- `PWP-V1-005`: The provider runs before and after Playwright. Unavailable or malformed identity, unhealthy or contradictory output, expectation mismatch, provider/config drift, application repository/build/configuration drift, instance/start-generation drift, and test-repository drift are explicit infrastructure reasons and cannot project passing evidence.
- `PWP-V1-006`: `/1` binds one clean test repository by root commit, revision and tree before Playwright and after Playwright/provider completion, ignoring ambient Git repository/configuration redirects, plus the existing exact Playwright runner, browser/project, bound source/configuration, argv and declared environment identities in the same receipt.
- `PWP-V1-007`: Corvint supplies only canonical configuration bytes on provider stdin and owns only the provider and Playwright process groups. It never starts, stops, restarts, cleans or sends a lifecycle verb to the externally owned application.
- `PWP-V1-008`: Qualification includes a deterministic, locally owned Docker Compose fixture and a generic command-provider fixture. Negative controls prove that a healthy wrong revision, healthy wrong image, restarted container and test-repository drift cannot produce passing evidence. Fixture cleanup waits for provisioning and retries after interruption; a skipped Docker fixture is never qualification success.
- `PWP-V1-009`: (accepted under the owner's standing auto-accept (2026-10-11; V1-0679)) The supported provider closure is the single executable file: it is staged as a private content copy with no sibling files, libraries or runtime directory. A launcher that resolves its runtime relative to its own path, such as Homebrew Node (`/opt/homebrew/Cellar/node@22/22.23.3_3/bin/node` and `node@24/24.22.0`, which load `@rpath/libnode.*.dylib` from `../lib`), is an explicit exclusion: its staged copy cannot start, and the run MUST record `application-attestation-provider-unavailable` with no observation, never a passing one. Corvint MUST NOT widen the closure by copying host libraries without a separately accepted qualification; such a provider is wrapped by a self-contained executable instead (the PTF fixture uses a native Go attestor). Evidence: `TestApplicationAttestationRelocatedLauncherFailsClosed` (fails when the provider runs from its original path); the 2026-10-22 Homebrew reproduction is in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-22-ctx-1022j.md`.

### Sensitive-input revision

- `PWP-V2-001`: `/2` is selected explicitly by `--sensitive-input-redaction`. It retains bounded nested action steps per retry attempt; `/0` and `/1` reject `/2` policy or step fields and remain byte-compatible.
- `PWP-V2-002`: Before reporter serialization, input actions matching the fixed defaults `fill`, `type`, `insertText`, `insert-text`, `insert text`, `pressSequentially`, and `press sequentially` replace the entire title with a canonical action kind plus the exact marker `[REDACTED]`. The grammar matches only a leading action after separators or a receiver identifier chain, with dot separators or adjacent slash separators. Word tokens use Unicode letters, numbers and marks with per-rune lowercase comparison; every other rune, including symbols, is a separator for both pattern normalization and matching. Indices address original runes, never transformed bytes. Casing, leading whitespace, action punctuation and receiver prefixes cannot bypass classification. Assertion/navigation prose is not searched for embedded actions. Call-bearing receiver expressions are not supported/qualified syntax: a bounded lexical scan rejects sensitive action tokens and sensitive quoted property names in those expressions with `sensitive-input-action-syntax-unsupported`, before serialization or retained-document acceptance. It does not implement a general JavaScript call parser. A rejected reporter attempt prevents later report serialization. No original value or digest is retained.
- `PWP-V2-003`: Redaction recurses through nested steps and retry attempts. Any sensitive action, including an already-redacted action, requires every nonempty risk-bearing field across every test and retry in the entire report to equal `[REDACTED]`: step/parent errors and attachments, test failure messages and artifacts, global reporter errors and infrastructure detail. Ingestion rejects violations without needing the original value, so an adapter cannot move a value to another test. Reporter serialization canonicalizes those fields. Raw action tails and declared fields produce only transient candidates: whole tails and individual top-level arguments, including final arguments, respecting quoted/escaped text and nested parentheses. Candidates scrub only report-wide risk fields and are never serialized or digested. Assertion titles, navigation/action metadata, enums, IDs, digests and bound identity remain byte-for-byte usable for criterion matching.
- `PWP-V2-004`: Provider-declared additional case-insensitive action-title patterns and metadata field names are closed and bounded additions. At most 16 action patterns and 32 field names are admitted, each nonempty and at most 128 UTF-8 bytes after whitespace trimming and per-rune lowercasing. Every admitted action pattern must normalize to at least one word using the same separator class as matching; zero-word declarations such as `***` and bound violations return fixed `sensitive-input-policy-invalid`. They cannot remove, replace, or weaken the defaults. The profile rejects depth over 32, more than 4096 total steps across retries, any step/error/attachment string over 64 KiB, and more than 64 validation findings before recursive path growth.
- `PWP-V2-005`: Reporter output and retained documents are untrusted. Ingestion rejects an unredacted sensitive action or noncanonical protected risk field with typed finding `sensitive-input-unredacted`, naming only its structural path and never echoing the value. Malformed retained `/2` documents return fixed typed `sensitive-input-document-invalid` diagnostics; unknown property names, values and decoder text are never echoed. Inputs whose kind cannot be decoded use that same value-free rejection because their profile is not trustworthy.
- `PWP-V2-006`: The conformance fixture contains a deliberately leaking payload that is rejected and a redacted payload that is accepted with action-level traceability. `/2` cannot project passing evidence until the changed reporter completes the required live matrix.

### Remaining Playwright 1.64.0 tuples and refusal exit status

- `PWP-V0-035`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2006; GitHub #1263) On runner `1.64.0`, `ReceiptRuntimeTuple` classifies the
  application-attested `/1` and sensitive-input `/2` profiles as candidates on Node `v22.23.2` only,
  with the `PWP-V0-032` bundled tuple. `/1` rests on the live Docker attestation run
  (`TestApplicationAttestationDockerComposeQualification` on 1.64.0) and `/2` on the live
  `TestPWPV0035SensitiveProfileLive` run, whose real `fill` step is retained redacted and whose
  encoded receipt never contains the typed value. `/2` still never projects passing execution
  (`PWP-V2-006`); its candidacy only stops it reading `unqualified` in discovery and abstention.
  `/1` and `/2` on any other Node, including `v24.11.1`, stay `unqualified`.
- `PWP-V0-036`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2006; GitHub #1263) On runner `1.64.0`, Node `v24.11.1` (official nodejs.org
  darwin-arm64 tarball, SHA-256 checked against the release `SHASUMS256.txt`) is a candidate for the
  base `/0` and attempt-retaining `/3` profiles' bundled `PWP-V0-032` tuple only, on the live
  matrix with the Node `v22.23.3` control abstaining. Neighbouring releases (`v24.11.0`, `v24.12.0`),
  identity/browser Node drift, `/1`, `/2` and the system-browser tuple stay unqualified.
- `PWP-V0-037`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2006; GitHub #1263) The configured system-Chrome tuple is keyed by exact runner
  version and never inherited across versions: `1.63.0` keeps `Google Chrome 153.0.8010.48`, and
  `1.64.0` admits only `Google Chrome 155.0.8059.85` at
  `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, configured and observed at the
  same path, headless, on `/0` under Node `v22.23.2`. `/1`, `/2`, `/3`, a headed or unstated
  `headless` run, Node `v24.11.1`, any other Chrome version and Chrome 155 on `1.63.0` abstain. The reporter mirrors the same table; the Go predicate is the
  authority.
- `PWP-V0-038`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2006; GitHub #1263, #1275) When the `e2e` command
  refuses with a typed refusal code before anything runs (every `PWP-V0-034` refusal, with or
  without `watch`; a watch request is admitted before its first status line), it exits `3` and writes exactly one stderr line, `refusal: <code>` followed by
  `: <detail>` when there is one, with carriage returns and line feeds in the detail escaped as `\r`
  and `\n`. Exit `0` stays an observed run (failed tests included), `1` an infrastructure,
  retention or keep-reporters failure, and `2` a usage error; `qualify-keep-reporters` keeps its
  status. The provider's `--help` states this exit-status contract.

Non-goals: no Linux, headed-browser, `/1`/`/2`-on-Node-24 or semver-range admission; no change to
any receipt member, profile, projection or the per-test freshness profile (`PTF-V0` stays on
`1.63.0` with Node `v22.23.3`); no new refusal code. Failure modes addressed: `/1` and `/2` receipts
and Node 24 receipts on 1.64.0 read `unqualified` although they ran correctly; a system-Chrome
version qualified on 1.63.0 could have been read as qualified on 1.64.0; a wrapper could not tell
a refused `e2e` request from a failed run without parsing stdout, and stderr called the refusal an
infrastructure failure. Acceptance evidence: the unit tests in Traceability plus the retained live
runs in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-2006-playwright-164-tuples.md`. Receipts that built before are
byte-identical: no receipt struct, encoding or bundled tuple changed. Rollback removes the 1.64.0
`/1`, `/2` and Node `v24.11.1` runtime-tuple cases, the 1.64.0 system-Chrome entry (Go and
reporter) and the exit-status branch; receipts written on those tuples then read `unqualified` and
are neither deleted nor rewritten, and refusals exit `1` again.

### Shared exact runtime tuple table

Human intent: GitHub #1270 and V1-2009; the owner's 2026-10-20 decision keeps decision 0448 (Node
patch releases are admitted one exact release at a time, never a major or minor range) and fixes the
receipt/freshness clash with one shared exact-tuple table.

- `PWP-V0-039`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2009; GitHub #1270) Receipts and per-test
  freshness admit from one table of exact `(profile, runner version, Node release, browser source)`
  rows, where browser source is `playwright-bundled` or `system`. `ReceiptRuntimeTuple`, the per-test
  bundled and system-browser predicates and the freshness row check all read that table; no consumer
  keeps its own runner or Node literal. The rows are exactly the tuples live runs observed:
  `1.63.0`/`v22.23.2` bundled for `/0`..`/3` and system for `/0`..`/2`; `1.63.0`/`v24.11.1` bundled
  for `/0`; `1.64.0`/`v22.23.2` bundled for `/0`..`/3` and system for `/0`; `1.64.0`/`v24.11.1`
  bundled for `/0` and `/3`; and `corvint-playwright-freshness/0` on `1.63.0`/`v22.23.3` bundled.
  Every value is an exact release; a range, a prefix or a neighbouring patch is never a row.
  `1.60.0` predates the Node and browser binding and stays outside the table unchanged. The
  per-test browser identity (bundled revision and digest, system Chrome version, the 1.64.0
  system headless condition) still decides each test. When this was accepted, freshness on `1.64.0`,
  and freshness on Node `v22.23.2` or `v24.11.1`, had no live freshness run and were not rows; a
  row is added only with its own live run, and `PWP-V0-050` adds the first such row.
- `PWP-V0-040`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2009; GitHub #1270) A tuple refusal names the
  first failing field, checked in the order runner, Node, browser source, and the exact values the
  shared table admits for that field given the fields before it, sorted. The
  `corvint-playwright-e2e-refusal/0` document carries it as the optional member
  `runtimeTuple` `{"profile","field","admitted"}` (field `runner`, `node` or `browser-source`),
  never echoing the observed value. The external runner-version refusal carries it with field
  `runner` and the admitted external versions; a per-test freshness request whose runner is not a
  freshness row refuses `freshness-runtime-tuple-unqualified` before anything starts, with the same
  member and the detail `runner not admitted for corvint-playwright-freshness/0; admitted runner
  values: 1.63.0, 1.64.0` (`1.64.0` since `PWP-V0-050`); the step-negation `runtime-tuple-unqualified` refusal appends the same field and
  admitted-values text. The browser source is observed only after a run, so it is named by
  `AdmittedRuntimeTupleMismatch` for consumers that hold a receipt, not by a pre-run refusal.

Non-goals: no Node major or minor range, no new admitted tuple, no freshness on `1.64.0` or on Node
`v22.23.2`/`v24.11.1`, no change to any receipt member, profile, projection or the reporter's mirror
table, and no discovery-document shape change. Failure modes addressed: receipts pinned Node
`v22.23.2` and freshness pinned `v22.23.3` in two separate literals, so the clash was invisible and
one could move without the other; a refusal did not say which tuple field failed or what would be
admitted. Acceptance evidence: `TestPWPV0039ReceiptsAndFreshnessAdmitFromOneTable` (a table row
change moves receipts and freshness together; it fails with freshness's base literal restored) and
`TestPWPV0040TupleRefusalNamesFieldAndAdmittedValues` (it fails with the base admission restored),
plus the updated `PWP-V0-034` document bytes; no live run, because no tuple was admitted. Receipts
are byte-identical and classify as before. Rollback restores the per-consumer literals and removes
the `runtimeTuple` member and the freshness pre-run refusal (freshness then fails
`freshness-config-unsupported` again); no retained receipt is deleted or rewritten.

### Freshness on Playwright 1.64.0 from the shared tuple

Human intent: GitHub #1270 and V1-2009, as amended by the owner on 2026-10-20: keep decision 0448's
one-exact-release rule, and let one Node tuple with Playwright 1.64.0 satisfy both the external
receipt and the per-test freshness receipt, admitting freshness on 1.64.0 only with a retained live
run on this host.

- `PWP-V0-050`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2009; GitHub #1270) The shared table gains exactly
  one row, `corvint-playwright-freshness/0` on runner `1.64.0`, Node `v22.23.2`, browser source
  `playwright-bundled`, because a live per-test freshness run passed on that exact tuple on this
  host (official nodejs.org darwin-arm64 build, `@playwright/test@1.64.0`, headless-shell revision
  `1248` with the `PWP-V0-032` digest). Runner `1.64.0` on Node `v22.23.2` with the bundled browser
  is then admitted by the external profiles and by freshness from the same row set, so one tuple
  satisfies both receipts. The per-test bundled browser identity still decides each test. Freshness
  on `1.64.0` with Node `v24.11.1`, `v22.23.3` or any other release, with the system browser, or on
  `1.63.0` with Node `v22.23.2` stays unadmitted; each needs its own live run. No range is admitted.
- `PWP-V0-051`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2009; GitHub #1270) The freshness command
  identity digests its executable (`argv[0]`) as a streamed SHA-256 of a regular, non-symlink file of
  at most 256 MiB, rechecked as the same file after opening, instead of reading it whole under the
  64 MiB application-attestation bound. An official Node release binary (about 113 MB on
  darwin-arm64) therefore binds; a larger, replaced, missing or non-regular executable yields no
  observation, and the run fails closed as before. The entrypoint keeps its 4 MiB read bound.
- `PWP-V0-052`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2009; GitHub #1270) For a freshness run the
  provider passes the reporter the table's bundled freshness `(runner, Node)` rows as the reporter
  option `freshnessTuples`, and the reporter's freshness pre-filter admits a test's bundled browser
  identity only when its runner and Node release match one of those rows. The reporter keeps no
  freshness runner or Node literal. Non-freshness controlled configs are byte-identical; the
  reporter's external Node pre-filter is unchanged, and Go remains the authority for every receipt.
- `PWP-V0-053`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2112; GitHub #1270) Per-test
  freshness acceptance (`corvint-tests-accept`, `internal/testacceptance`) admits a request's
  `runner_version` only when the shared table has a `corvint-playwright-freshness/0` row for that
  exact runner, instead of a copied `1.63.0` literal; any other runner still refuses
  `freshness-request-binding-invalid`. The request carries no Node release, so the run's observed
  Node and browser source are decided by the provider's freshness receipt predicate from the same
  table: a `1.64.0` request on Node `v22.23.2` with the bundled browser is admitted and projects
  like the `1.63.0` / `v22.23.3` baseline, and `1.64.0` on Node `v24.11.1` stays unqualified.
- `PWP-V0-054`: (accepted under the owner's standing auto-accept (2026-10-11); V1-2112; GitHub #1270) The
  reporter's external (non-freshness) Node pre-filter is declared once per browser source, keyed by
  runner (`externalBundledNodes`, `externalSystemNodes`), and must equal the shared table's
  external-profile `(runner, Node)` rows for that source; a Go test fails when either copy drifts
  or when the reporter keeps any other Node release literal. The copy is checked rather than
  injected so non-freshness controlled configs stay byte-identical (`PWP-V0-052`); the pre-filter
  admits the same tuples as before, and Go remains the authority for which profile may pass.

Non-goals: no Node major or minor range, no freshness on Node `v24.11.1` or `v22.23.3` with
`1.64.0`, no new receipt member, profile or projection, no Linux tuple, and no change to the
freshness environment (still exactly PATH, LANG, LC_ALL and TMPDIR, so the browser comes from the
default Playwright cache). Failure modes addressed: freshness refused every 1.64.0 run, so no tuple
satisfied both receipts; an official Node binary exceeded the 64 MiB read bound and the freshness
command reported `command unavailable`; the reporter kept its own copy of the freshness tuple and
returned no identity for an admitted row; freshness acceptance still refused every `1.64.0` request
on a copied runner literal; the reporter's external Node list could drift from the table unnoticed. Acceptance evidence: `TestPWPV0050OneTupleAdmitsExternalAndFreshness`,
`TestPWPV0051FreshCommandExecutableStreamDigest` and `TestPWPV0052ReporterFreshnessTuplesFromTable`
each fail with the base production code restored; `TestPWPV0053FreshnessAcceptanceAdmitsTableRunner`
fails on the copied `1.63.0` acceptance literal and `TestPWPV0054ReporterExternalNodesMatchTable`
on the reporter's inline Node literals (V1-2112); the updated `PWP-V0-039` and `PWP-V0-040` tests;
and the retained live run `TestPTFV0ActualLive` on `1.64.0`/`v22.23.2` (all eight cases pass), with
a control run on Node `v22.23.3` that reads freshness `UNKNOWN`, plus `TestQualifiedPlaywrightLive`
on the same tuple and toolchain (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-2009-freshness-164-live.md`). Linux,
freshness on `v24.11.1` and a fresh 1.63.0 freshness regression are `NOT_RUN`. Rollback removes the
row, the reporter option and the streamed digest; freshness on 1.64.0 then refuses
`freshness-runtime-tuple-unqualified` again, and no retained receipt is deleted or rewritten. Rolling back
`PWP-V0-053..054` restores the acceptance runner literal and the reporter's inline Node list; no
receipt, request or projection format changes.

### Keep-reporters live evidence and symlinked scratch exclusion (V1-1002)

Intent: the keep-reporters option and its qualification were built and unit-tested without a live
Playwright run. The first live run (V1-1002) showed that a keep-reporters qualification could never
pass on macOS, because the provider's own per-run scratch files were bound as config inputs.

- `PWP-V0-055`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1002) The
  reporter excludes the provider's private output directory from the config-input set by comparing
  module cache keys against that directory's real path in every profile, not only in the freshness
  profile. Node keys its module cache by real path. On a host whose temporary directory runs through
  a symlink (macOS `/var` to `/private/var`), the controlled `playwright.config.cjs` and
  `reporter.cjs` were therefore bound as config inputs under a per-run random path. Every external
  receipt then carried two inputs that differ on every run. `qualify-keep-reporters` always reported
  `keep-reporters-inputs-differ`, and a carried qualification could never match a later keep run
  (`PWP-V0-016`). Project modules, the original config and every non-`node_modules` module outside
  that directory are still bound unchanged.

Non-goals: no change to the receipt shape, profiles, bounds or projection rules; no new runtime
tuple; no change to which kept reporters are bound. Failure modes addressed: provider scratch files
bound as config inputs on a symlinked temporary directory; a live keep-reporters qualification that
could never be `qualified`. Acceptance evidence: the Node case `PWP-V0-055 provider output files are
never config inputs when the output path runs through a symlink` fails with the old comparison
restored. The live `TestPWPV0010KeepReportersLive` ran on Playwright 1.64.0, Node v24.11.1 and the
bundled headless shell on darwin-arm64. It kept list, html, json and junit reporters plus a shape
reporter. The html report went to its default folder, and json and junit went to the
environment-named outputs `PLAYWRIGHT_JSON_OUTPUT_NAME` and `PLAYWRIGHT_JUNIT_OUTPUT_NAME`. The run
had one passing and one failing test.

- All three outputs land under the original config directory.
- The observed `FullConfig.reporter` lists the provider reporter first, then the kept entries.
  html, json and junit carry the injected `configDir`.
- Playwright passes every reporter its own `configDir`, the provider's controlled-config
  directory. It spreads the entry options over that value, so the injected value wins.
- Without the injection, the json output is missing and the test fails.
- The unqualified keep receipt projects nothing passing.
- The live control and keep runs qualify (`qualified`, no reasons) only with `PWP-V0-055`. Before
  it, they reported `keep-reporters-inputs-differ`.
- A third keep run carrying that record projects the passing outcome as passing and the failing
  outcome as not passing (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-22-tests-1022f.md`).

Linux, Node v22.23.2, `/1` to `/3` keep runs and the `qualify-keep-reporters` CLI path are
`NOT_RUN`. Rollback restores the freshness-only real-path comparison. Retained receipts are
neither deleted nor rewritten, and on macOS a keep-reporters qualification then reports
`keep-reporters-inputs-differ` again.

### Every-attempt revision

Intent accepted by decision 0417 for `AFU-V1-012`; `--retain-attempt-details` selects the implemented
profile. Qualification is consuming-tuple specific and retains every failed or unavailable observation. `/3` is qualified only on the bundled Darwin arm64 / Node v22.23.2 / Playwright 1.63.0 tuple below and, by `PWP-V0-032..033` and `PWP-V0-036`, the bundled 1.64.0 tuple on the same OS under Node v22.23.2 or v24.11.1; it refuses the legacy 1.60 and system-browser tuples.

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
bound 4 MiB (external console output is truncated at it, not refused, `PWP-V0-028`), readiness fifteen seconds. Application provider configuration/output is 64 KiB per
document and each observation defaults to five seconds. No selector, coverage or repository-pass
claim is added.

## Refusal vocabulary

The provider's closed refusal codes are:

- `config-input-drift`
- `config-inputs-overflow` (more than `externalMaxConfigInputs`, 4096, reporter-observed config
  entries or more than `externalMaxConfigInputBytes`, 1 MiB, of their paths and digests; the detail
  states the observed value and the limit, `PWP-V0-021`)
- `config-inputs-unobserved` (checked only after `config-inputs-overflow`)
- `external-app-identity-required`
- `external-config-version-test-files-required`
- `external-input-bound-or-secret`
- `external-playwright-version-unqualified` (the message names the qualified runner versions; with
  every other pre-run `e2e` refusal it is also written to stdout as the `corvint-playwright-e2e-refusal/0`
  document, `PWP-V0-034`)
- `external-readiness-url-required`
- `external-server-command-forbidden`
- `freshness-runtime-tuple-unqualified` (a per-test freshness request whose runner version is not a
  freshness row of the shared runtime tuple table; the detail and the refusal document's
  `runtimeTuple` member name the field and the admitted values, `PWP-V0-040`)
- `external-unsupported-option` (a `--test-arg` option outside the external-mode allow-list; the
  detail names the option. Emitted before this list named it; documented by `PWP-V0-034`)
- `input-identity-changed`
- `interrupt-grace-invalid` (a negative `--interrupt-grace`, `PWP-V0-030`)
- `no-tests-observed`
- `preflight-declaration-invalid` (a `--preflight` declaration that is unreadable, oversized, secret-shaped or outside the closed `corvint-js-preflight/0` shape, `PWP-V0-090`)
- `preflight-requires-external-server` (a preflight outside external mode, or with a caller-owned server or freshness, `PWP-V0-090`)
- `preflight-failed` (infrastructure before tests: a declared probe did not pass; the failure names `phase`, `probe` and `artefact`, `PWP-V0-092`)
- `preflight-invalid`, `preflight-failure-has-test-verdicts`, `preflight-phase-without-preflight`, `preflight-requires-external-profile` (receipt shape errors, `PWP-V0-093`)
- `keep-reporters-unsupported-mode`
- `keep-reporters-qualification-invalid`
- `keep-reporters-qualification-not-qualified`
- `keep-reporters-qualification-requires-keep-reporters`
- `keep-reporters-keep-run-skipped`
- `keep-reporters-control-run-incomplete`, `keep-reporters-keep-run-incomplete` (`not-run` record reasons)
- `keep-reporters-control-unqualified`, `keep-reporters-keep-unqualified`, `keep-reporters-order-unsupported`,
  `keep-reporters-entries-unknown`, `keep-reporters-inputs-differ`, `keep-reporters-observation-differs`,
  `keep-reporters-no-tests` (`not-qualified` record reasons)
- `project-location-unknown`
- `project-reporters-invalid`
- `project-reporters-unobserved`
- `qualified-document-output-overflow`
- `qualified-document-secret-shaped`
- `report-identity-unknown`
- `report-output-overflow` (the report file exceeds the 4 MiB output bound; never a config-input count)
- `reporter-global-error`
- `run-status-unknown`
- `runner-cleanup-unknown`
- `runner-control-requires-external-server` (`--runner-output` or an explicit `--interrupt-grace` without `--external-server`, `PWP-V0-030`)
- `runner-output-unavailable` (the `--runner-output` path cannot be opened as a regular file: it cannot be created, or it is a symlink, FIFO or other non-regular entry, `PWP-V0-028`)
- `runner-process-invalid` (a retained `external.runner` record breaks its closed shape, `PWP-V0-029`)
- `runner-version-mismatch`
- `server-unavailable-at-publish`
- `test-attempt-identity-unknown`
- `test-attempt-state-unknown`
- `test-file-unbound` (the detail names each unbound anchor file and its cause, `PWP-V0-019`)
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
The accepted qualification (`PWP-V0-014`..`018`, decision 0461) adds the optional `qualification` member under
`projectReporters`, the record profile and the `qualify-keep-reporters` command. Acceptance evidence is the
Go and Node tests in Traceability plus one live `qualified` record on a qualified tuple, which is `NOT_RUN`.
Rolling it back removes the command, the flag and the member, and restores provider-last order. Receipts
carrying a qualification then fail closed through the canonical check, and retained records and receipts are
neither deleted nor rewritten.

## Traceability

Qualification runtimes: Playwright **1.60.0**, **1.63.0** and **1.64.0** (`PWP-V0-032`), each with its installed Chromium
browser, tested locally on Darwin. External mode refuses other runner versions until their
effective-fixture metadata is qualified. Both in-process reporter ABIs expose literal fixture defaults and nested `test.use`
overrides; the provider resolves those with project options and preserves the effective browser,
viewport and device settings. Missing metadata, executable option fixtures or custom browser
fixtures produce unknown identity and never passing execution; custom page/context fixtures and
tests that use no browser follow `PWP-V3-013..016`. A device label remains `unknown`
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
the registry identity, suffix and digest may not. Playwright `1.64.0` has its own bundled tuple
(`PWP-V0-032`): revision `1248`, manifest version `156.0.8078.4`, observed version
`Google Chrome for Testing 156.0.8078.4`, suffix
`chromium_headless_shell-1248/chrome-headless-shell-mac-arm64/chrome-headless-shell`, SHA-256
`77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634`, on Node v22.23.2 for `/0`,
`/1`, `/2` and `/3` (`PWP-V0-033`, `PWP-V0-035`) and on Node v24.11.1 for `/0` and `/3`
(`PWP-V0-036`). On 1.64.0 the configured system tuple is Google Chrome 155.0.8059.85 (`PWP-V0-037`).
`PWP-V0-009` admits the 1.63.0 bundled tuple on Node `v24.11.1` for `/0` only. Bundled headed
Chromium, Linux (amd64 and arm64) and every other Node tuple are `NOT_RUN` and remain
diagnostic-only. Issue #665 reported a consumer pinning Node v24.11.x, whose
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
qualification record is not adopted. `/1`, `/2` and `/3` on Node 24 need their own live runs first;
`PWP-V0-036` supplied that run for `/3` on 1.64.0 only.

To qualify another Playwright runner version (as `PWP-V0-032` did for 1.64.0): install the exact
`@playwright/test` and `playwright-core` versions from a lockfile with integrity, install its
package-selected `chromium-headless-shell`, and confirm the reporter internals the provider reads
are unchanged in that version's sources. Add the exact version to `qualifiedPlaywrightVersions`, its
bundled tuple to `qualifiedBundledBrowsers` (Go and reporter), and its profile/Node candidates to
`ReceiptRuntimeTuple`; then run the four live tests with that version's modules and the 1.63.0
regression. A consumer cannot qualify a version by configuration: the refusal document lists the
versions to pin. The 1.64.0 qualification command was:

`CORVINT_PLAYWRIGHT_MODULES=<node_modules with @playwright/test 1.64.0> CORVINT_PLAYWRIGHT_NODE=<node-v22.23.2-darwin-arm64>/bin/node CORVINT_PLAYWRIGHT_CONTROL_NODE=<node-v22.23.3-darwin-arm64>/bin/node GOTOOLCHAIN=local go test -count=1 -timeout 10m ./internal/jstestprovider -run 'TestQualifiedPlaywrightLive$|TestQualifiedPlaywrightLiveDevicesSpread$|TestQualifiedPlaywrightAttemptsLive$|TestQualifiedPlaywrightAttemptsLiveDevicesSpread$' -v`

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

For the bundled qualified path, omit `use.launchOptions.executablePath`; Playwright 1.64 (the declared
tuple on Node v24.11.1, V1-2690) selects its pinned headless shell and the provider binds its registry revision and executable digest. A configured
path instead selects the separately qualified system-Chrome tuple. The provider does not implement a
browser-path override. With the bundled path, the exact command is:

`corvint-js-test-provider e2e --dir . --config playwright.config.ts --package-json package.json --lockfile package-lock.json --runner-version 1.64.0 --external-server --app-identity app-at-3002 --server-ready-url http://127.0.0.1:3002 --test-file tests/e2e/example.spec.ts --test-arg tests/e2e/example.spec.ts --retain`

| Requirements | Implementation | Evidence |
|---|---|---|
| PWP-V0-001..008 | `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `cmd/corvint-js-test-provider/main.go`, `internal/testvaliditydoc/document.go` | `TestQualifiedPlaywrightLive`, `TestQualifiedPlaywrightLiveDevicesSpread`, `TestExternalReadiness`, `TestQualifiedReceiptProjection`, `TestPlaywright163UnqualifiedBrowserTupleAbstains`, `TestPlaywright163BundledBrowserTupleAbstainsOnDrift` |
| PWP-V0-009 (accepted, decision 0448) | `internal/jstestprovider/runtime_tuple.go`, `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `internal/testvaliditydoc/discover.go` | `TestPlaywright163Node24TupleAdmissionIsExact`, `TestReceiptRuntimeTupleClassification`, `TestDiscoverAbstentionReasons`; live `TestQualifiedPlaywrightLive` and `TestQualifiedPlaywrightLiveDevicesSpread` on Node v24.11.1 with the v22.23.3 control (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-07-gh665-playwright-node-tuple-abstention.md`) |
| PWP-V0-010..013 (proposed, V1-0986; GitHub #670) | `internal/jstestprovider/project_reporters.go`, `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `internal/jstestprovider/runner.go`, `internal/jstestprovider/projection.go`, `cmd/corvint-js-test-provider/main.go` | `TestExternalCommandDefaultConfigUnchanged`, `TestKeptReporterListResolution`, `TestKeptProjectReporterRunsBesideProvider`, `TestProjectReportersBindingShape`, `TestKeepReportersUnsupportedMode`, `TestKeepReportersFlagRequiresExternalServer`, `TestKeepReportersRetainedReceiptAccepted`, `TestQualifiedReporterSensitiveRedaction` (Node case `PWP-V0-011 keep mode reports kept entries`), live `TestPWPV0010KeepReportersLive` on 1.64.0 / Node v24.11.1 bundled (configDir override honoured, `FullConfig.reporter` shape recorded; V1-1002, `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-22-tests-1022f.md`); Linux `NOT_RUN` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-07-gh670-playwright-keep-reporters.md`) |
| PWP-V0-014..018 (accepted, decision 0461; V1-1028; GitHub #686) | `internal/jstestprovider/keep_reporters_qualification.go`, `internal/jstestprovider/project_reporters.go`, `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `internal/jstestprovider/runner.go`, `cmd/corvint-js-test-provider/main.go` | `TestKeptReporterListResolution`, `TestQualifyKeepReportersQualified`, `TestQualifyKeepReportersReasons`, `TestKeepReportersQualificationRecordClosed`, `TestKeepReportersQualifiedProjection`, `TestKeepReportersQualificationRunGuards`, `TestQualifyKeepReportersRunOrderAndNotRun`, `TestKeepReportersQualificationFlags`, `TestKeepReportersQualifiedReceiptProjects`, Node case `PWP-V0-011 PWP-V0-014 keep mode reports kept entries`; live `TestPWPV0010KeepReportersLive` qualifies and projects on 1.64.0 / Node v24.11.1 after `PWP-V0-055` (V1-1002); the `qualify-keep-reporters` CLI path and Linux `NOT_RUN` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-08-gh686-playwright-keep-reporters-qualification.md`) |
| PWP-V0-019 (accepted under the owner's standing auto-accept (2026-10-11); V1-1242; GitHub #807) | `internal/jstestprovider/external.go`, `cmd/corvint-js-test-provider/main.go` | `TestQualifiedReportNamesUnboundSetupProjectFiles`, `TestProviderHelpExplainsSetupProjectFileBinding`; live Playwright setup-project run `NOT_RUN` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-11-gh807-setup-project-files.md`) |
| PWP-V0-020 (accepted under the owner's standing auto-accept (2026-10-11); GitHub #832) | `internal/jstestprovider/external.go`, `internal/jstestprovider/runner.go` | `TestBoundaryTimeoutNamesRunnerAndFlag`, `TestE2EBoundaryFailure_WaitNotCompleted`; live Playwright timeout run `NOT_RUN` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-issue-832-e2e-timeout-wording.md`) |
| PWP-V0-021 (accepted under the owner's standing auto-accept (2026-10-11); GitHub #844; V1-1317) | `internal/jstestprovider/external.go`, `internal/jstestprovider/freshness.go`, `internal/jstestprovider/keep_reporters_qualification.go` | `TestQualifiedReportConfigInputBounds`, `TestRunUnit_ReportBoundIsItsOwnAndNotTheExternalProviders`; live Playwright run above 256 config inputs `NOT_RUN` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-12-gh-844-845-config-inputs-state-matchers.md`) |
| PWP-V0-022..027 (accepted under the owner's standing auto-accept (2026-10-11); V1-1762; GitHub #1106) | `internal/testvaliditydoc/externalorigins/external_origins.go`, `cmd/corvint-js-test-provider/external_origins.go`, `cmd/corvint-js-test-provider/main.go` | `TestPWPV0022NetworkEvidenceIsClosedAndOriginsAreCanonical`, `TestPWPV0023BlockingVersusIncidental`, `TestPWPV0023BeaconOnlyTrafficIsIncidental`, `TestPWPV0024CDNModuleDependenceFinding`, `TestPWPV0025AllowListedOriginRaisesNoFinding`, `TestPWPV0026PassFailCorrelationNamesDifferingRequests`, `TestPWPV0027MissingNetworkEvidenceIsUnknown`, `TestPWPV0027ReportOutputListsExternalOrigins`, `TestPWPV0027ExternalOriginsCommandPrintsReport`, `TestPWPV0027EmptyRunStaysUnknownBesideARecordedRun`, `TestPWPV0022RequestObservationsAreRequired`, `TestPWPV0025PortAndIPSpellingsMatchCanonicalOrigins`; live Playwright run `NOT_RUN` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-V1-1762-external-origins.md`) |
| PWP-V0-028..031 (accepted under the owner's standing auto-accept (2026-10-11); V1-1800, V1-1802; GitHub #1131, #1133) | `internal/procgroup/interrupt.go`, `internal/procgroup/process.go`, `internal/jstestprovider/external.go`, `internal/jstestprovider/receipt.go`, `internal/jstestprovider/projection.go`, `cmd/corvint-js-test-provider/runner_control.go`, `cmd/corvint-js-test-provider/runner_output_unix.go`, `cmd/corvint-js-test-provider/runner_output_other.go`, `cmd/corvint-js-test-provider/main.go`, `cmd/corvint-js-test-provider/negate.go` | `TestPWPV0028ExternalConsoleOverBoundTruncatesAndTees` (#1131: a 5 MiB fake runner completes and the tee receives every byte), `TestPWPV0028ExternalConsoleWithoutTee`, `TestPWPV0028TeesReceiveUntruncatedStreams`, `TestPWPV0028RunnerOutputFlagWritesFile`, `TestPWPV0028BlockedTeeSinkDoesNotHoldDrains`, `TestPWPV0028RunnerOutputRefusesSymlink`, `TestPWPV0028RunnerOutputRefusesFIFO`, `TestPWPV0028RunnerOutputExistingFileBecomes0600`, `TestPWPV0028RunnerOutputReportsEarlyEnd`, `TestPWPV0029RunnerRecordShape`, `TestPWPV0029SelfExitRecordsNoStopSignal`, `TestPWPV0029StopSignalSurvivesExitDuringSend`, `TestPWPV0030ExternalTimeoutInterruptsRunnerFirst` (#1133: the fake runner writes a file on SIGINT and the receipt records `SIGINT`), `TestReportStatusInterruptedByTimeoutIsNotCancelled` (V1-1812), `TestPWPV0030ExternalZeroGraceTerminatesFirst`, `TestPWPV0030ZeroInterruptGraceKeepsTerminateFirstStop`, `TestPWPV0030InterruptFirstStopLetsRunnerExitOnSIGINT`, `TestPWPV0030InterruptGraceEscalatesToSIGTERM`, `TestPWPV0030NegativeInterruptGraceIsInvalid`, `TestPWPV0030InterruptGraceFlag`, `TestPWPV0031ReadinessRulesByMode`, `TestPWPV0031ServerReadyURLHelpMatchesReadiness`; live Playwright run `NOT_RUN` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-18-V1-1800-js-provider-output-retention-interrupt-1131.md`) |
| PWP-V0-032..034 (accepted under the owner's standing auto-accept (2026-10-11); V1-2005; GitHub #1263) | `internal/jstestprovider/external.go`, `internal/jstestprovider/e2e_refusal.go`, `internal/jstestprovider/runtime_tuple.go`, `internal/jstestprovider/runner.go`, `internal/jstestprovider/freshness.go`, `internal/jstestprovider/qualified-reporter.cjs`, `cmd/corvint-js-test-provider/main.go`, `cmd/corvint-js-test-provider/runner_control.go` | `TestPWPV0032Playwright164BundledTupleIsExact`, `TestPWPV0033Playwright164ProfilesAndNodeAreExact`, `TestPWPV0034PreRunRefusalsAreTyped`, `TestPWPV0034E2EVersionRefusalWritesTypedDocument`, `TestPWPV0034CLIAdmissionRefusalWritesTypedDocument`, `TestPWPV0034RefusalWriteFailureKeepsDiagnostic`, `TestReceiptRuntimeTupleClassification`; live `TestQualifiedPlaywrightLive`, `TestQualifiedPlaywrightAttemptsLive`, `TestQualifiedPlaywrightLiveDevicesSpread` and `TestQualifiedPlaywrightAttemptsLiveDevicesSpread` on 1.64.0 and on 1.63.0 (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-2005-playwright-164-qualification.md`) |
| PWP-V0-035..038 (accepted under the owner's standing auto-accept (2026-10-11); V1-2006; GitHub #1263, #1275) | `internal/jstestprovider/runtime_tuple.go`, `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs`, `cmd/corvint-js-test-provider/main.go` | `TestPWPV0035Playwright164AttestedAndSensitiveTuples`, `TestPWPV0036Playwright164Node24TupleIsExact`, `TestPWPV0037Playwright164SystemChromeIsExact`, `TestPWPV0038E2ERefusalExitStatusAndStderrLine`, `TestPWPV0038WatchRefusalWritesNoStatusLine`, `TestPWPV0033Playwright164ProfilesAndNodeAreExact`, `TestReceiptRuntimeTupleClassification`; live `TestPWPV0035SensitiveProfileLive` (1.64.0 and 1.63.0), `TestApplicationAttestationDockerComposeQualification`, `TestQualifiedPlaywrightLive`, `TestQualifiedPlaywrightAttemptsLive` and the two devices-spread tests on 1.64.0 with Node v22.23.2 and v24.11.1, plus the 1.63.0 regression (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-19-V1-2006-playwright-164-tuples.md`); Linux `NOT_RUN` |
| PWP-V0-039..040 (accepted under the owner's standing auto-accept (2026-10-11); V1-2009; GitHub #1270) | `internal/jstestprovider/runtime_tuple.go`, `internal/jstestprovider/external.go`, `internal/jstestprovider/freshness.go`, `internal/jstestprovider/freshness_run.go`, `internal/jstestprovider/runner.go`, `internal/jstestprovider/e2e_refusal.go`, `internal/jstestprovider/negate.go` | `TestPWPV0039ReceiptsAndFreshnessAdmitFromOneTable`, `TestPWPV0040TupleRefusalNamesFieldAndAdmittedValues`, `TestPWPV0034E2EVersionRefusalWritesTypedDocument`, `TestReceiptRuntimeTupleClassification`; live run `NOT_RUN` (no tuple admitted; `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-2009-shared-runtime-tuple-table.md`) |
| PWP-V0-050..052 (accepted under the owner's standing auto-accept (2026-10-11); V1-2009; GitHub #1270) | `internal/jstestprovider/runtime_tuple.go`, `internal/jstestprovider/freshness_run.go`, `internal/jstestprovider/external.go`, `internal/jstestprovider/qualified-reporter.cjs` | `TestPWPV0050OneTupleAdmitsExternalAndFreshness`, `TestPWPV0051FreshCommandExecutableStreamDigest`, `TestPWPV0052ReporterFreshnessTuplesFromTable`, `TestPWPV0039ReceiptsAndFreshnessAdmitFromOneTable`, `TestPWPV0040TupleRefusalNamesFieldAndAdmittedValues`; live `TestPTFV0ActualLive` and `TestQualifiedPlaywrightLive` on 1.64.0 / Node v22.23.2 (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-20-V1-2009-freshness-164-live.md`); Linux and Node v24.11.1 freshness `NOT_RUN` |
| PWP-V0-053..054 (accepted under the owner's standing auto-accept (2026-10-11); V1-2112; GitHub #1270) | `internal/testacceptance/freshness.go`, `internal/jstestprovider/qualified-reporter.cjs` | `TestPWPV0053FreshnessAcceptanceAdmitsTableRunner`, `TestPWPV0054ReporterExternalNodesMatchTable`, `TestPWPV0052ReporterFreshnessTuplesFromTable`; no new live run (the admitted tuples are unchanged); live acceptance through `corvint-tests-accept` on 1.64.0 `NOT_RUN` |
| PWP-V0-055 (accepted under the owner's standing auto-accept (2026-10-11); V1-1002) | `internal/jstestprovider/qualified-reporter.cjs` | Node case `PWP-V0-055 provider output files are never config inputs when the output path runs through a symlink` (via `TestQualifiedReporterSensitiveRedaction`), live `TestPWPV0010KeepReportersLive` (`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-22-tests-1022f.md`) |
| PWP-V1-001..008 | `internal/jstestprovider/application_attestation.go`, `internal/jstestprovider/external.go`, `cmd/corvint-js-test-provider/main.go`, `internal/testvaliditydoc/document.go` | `TestApplicationAttestationCommandProvider`, `TestApplicationAttestationNegativeControls`, `TestAttestedReceiptNeverPassesWrongOrRestartedApplication`, `TestApplicationAttestationDockerComposeQualification` |
| PWP-V1-009 | `internal/jstestprovider/application_attestation.go` | `TestApplicationAttestationRelocatedLauncherFailsClosed` |
| PWP-V2-001..006 | `internal/jstestprovider/sensitive_input.go`, `internal/jstestprovider/sensitive_input_boundary.go`, `internal/jstestprovider/sensitive_input_grammar.go`, `internal/jstestprovider/qualified-reporter.cjs`, `internal/jstestprovider/external.go`, `internal/testvaliditydoc/document.go`, `cmd/corvint-js-test-provider/main.go` | `TestQualifiedReporterSensitiveRedaction`, `TestSensitiveInputEvidenceRedactionAndValidation`, `TestSensitiveInputNormalizationBoundsAndNoPanic`, `TestSensitiveInputAlreadyRedactedRiskFieldsFailClosed`, `TestSensitiveInputReceiverPrefixExtraction`, `TestSensitiveInputUnicodeGrammarAndReportScope`, `TestSensitiveInputAlreadyRedactedCrossTestRiskRejected`, `TestSensitiveInputArgumentCandidatesRespectStructure`, `TestSensitiveInputPolicyGrammarAgreement`, `TestSensitiveInputUnsupportedReceiverSyntaxRejected`, `TestSensitiveRetainedPolicyAndUnsupportedActionRejection`, `TestSensitiveRetainedDecodeNeverEchoesUnknownProperties`, `TestSensitiveInputConformanceFixtureRejectsLeakAndAcceptsRedaction`; live Playwright matrix `NOT_RUN` |
| PWP-V3-001..006 | `internal/jstestprovider/attempt_details.go`, `external.go`, `qualified-reporter.cjs`, `internal/appflows/runingest.go`, strict consumer and provider CLI | `TestPWPV3AttemptInventory`, `TestPWPV3ProfileBoundaries`, `TestAFUV1012QualifiedReceiptIngest`, `TestAFUV1012QualifiedReceiptRefusals`, `TestQualifiedPlaywrightAttemptsLive`, `TestQualifiedPlaywrightAttemptsLiveDevicesSpread`; local live log `evidence/issues-167-175/pwp3-live-passed.log` |
| PWP-V3-008 | `internal/jstestprovider/attempt_details.go`, `receipt.go`, `qualified-reporter.cjs` | `TestPWPV3008FailureLocation`, `TestQualifiedReporterSensitiveRedaction` (runs `qualified-reporter_test.cjs`) |
| PWP-V3-009 | `internal/jstestprovider/external.go`, `receipt.go`, `qualified-reporter.cjs` | `TestPWPV3009RunLevelErrorLocation`, `TestQualifiedReporterSensitiveRedaction` (runs `qualified-reporter_test.cjs`) |
| PWP-V3-010 | `internal/jstestprovider/external.go` (`externalHygiene` over the native `scrubPlaywrightOutcome`) | `TestPWPV3010ExternalBrowserLogTailScreened` (the reporter's message is secret-shaped as written; after hygiene the launch line is dropped, the timeout lines and call log stay, the attempt entry equals the last-attempt fields, the document encodes, a second pass is a no-op); live `TestCCEV0053LiveRecordsCoverage` under 20 busy-loop burners on a 12-core host: FAIL before (170.9 s, `qualified-document-secret-shaped`), PASS twice after (417.3 s) |
| PWP-V3-011..012 (experimental under the standing lane auto-accept; V1-2790; GitHub #1890) | `internal/jstestprovider/qualified_screen.go`, `internal/jstestprovider/projection.go` (`EncodeQualified`), `internal/jstestprovider/keep_reporters_qualification.go` (`EncodeKeepReportersQualification`), `cmd/corvint-js-test-provider/main.go` (`reportSecretShapedMember`) | `TestPWPV3011CredentialNamedPathsAndTitlesEncode` (the whole document is secret-shaped as written; it encodes), `TestPWPV3011SecretValuesStillRefused`, `TestPWPV3011NameValueInIdentifierRefuses`, `TestPWPV3011KeepReportersCredentialNamedInputs`, `TestPWPV3012PointerNeverEchoesText`, `TestPWPV3012PointerWithholdsDynamicKeys`, `TestPWPV3012ReportSecretShapedMember`; existing `TestQualifiedEncodingRejectsSecretAndBounds`, `TestPWPV3010ExternalBrowserLogTailScreened`, `TestCCEV0116EncodeRefusalReasonNamesCode`; live `--retain` run on a credential-named suite `NOT_RUN` |
| PWP-V3-013..016 (experimental under the standing lane auto-accept; V1-2791; GitHub #1891) | `internal/jstestprovider/qualified-reporter.cjs` (`effectiveUse`, `fixtureRegistry`, `fixtureClosure`, `testFixtureRoots`, `browserlessSteps`, `foreignLaunch`, `onTestEnd`, `onEnd`), `internal/jstestprovider/external.go` (`bindQualifiedReport`, `qualifiedPrerequisitesUnknown`, `browserlessReason`, `ProjectLocationUnknownTests`), `cmd/corvint-js-test-provider/main.go` (`reportProjectLocationUnknown`) | node `qualified-reporter_test.cjs` (`PWP-V3-013 page and context overrides keep the launch identity; browser overrides stay unknown`, `PWP-V3-013 an override in a worker that launched a browser outside the browser fixture is unknown`, `PWP-V3-014 a test that used no browser records browser none with a closed reason`, `PWP-V3-014 a test whose launch identity is known keeps it`, run by `TestQualifiedReporterSensitiveRedaction`), `TestPWPV3015BrowserlessTestsBindAndQualify`, `TestPWPV3016BrowserIdentityMissingRefusedAndNamed`, `TestPWPV3016ReportProjectLocationUnknown`; live `TestQualifiedPlaywrightLive` on 1.64.0 / Node v22.23.2 (custom page fixture passes with `corvintFixtureOverrides`; option-only Firefox override is browserless; headed, connect, environment-connect and executable-viewport controls now request `page` and stay refused); live 1.63.0 and the reporter user's own suite `NOT_RUN` |

The `/3` matrix passed locally on 2026-09-28. It retains failed-then-passed and infrastructure-then-assertion attempts. System Chrome 153.0.8010.54 was observed and refused; the older system tuple was NOT_OBSERVED in this run. `/3` does not inherit earlier-profile qualification.

### Owned emitted error codes

| Code | Emitted condition | Site |
|---|---|---|
| `application-attestation-config-invalid` | The provider config is noncanonical, has the wrong profile, or carries an invalid expectation. | `internal/jstestprovider/application_attestation.go:84@12af3b3a` |
| `application-attestation-config-unavailable` | The bounded provider config file cannot be read. | `internal/jstestprovider/application_attestation.go:80@b18eb77f` |
| `application-attestation-provider-drift` | The provider executable or configuration changes before post-run observation. | `internal/jstestprovider/external.go:265@db0a61bf` |
| `application-attestation-provider-required` | The attested profile lacks an absolute config path or provider argv. | `internal/jstestprovider/application_attestation.go:68@2bc06a44` |
| `application-attestation-provider-unavailable` | The provider executable is unresolved, unreadable, unstaged, or fails bounded execution. | `internal/jstestprovider/application_attestation.go:72@94973cec` |
| `application-attestation-requires-external-server` | Application attestation is requested outside external-server mode. | `internal/jstestprovider/runner.go:292@5ac3fd01` |
| `attested-external-profile-has-declared-identity` | An attested receipt also carries the legacy caller-declared application identity. | `internal/jstestprovider/projection.go:111@d5db6c8b` |
| `external-attestation-conflicts-with-caller-identity` | The attested request also supplies legacy caller identity or build-directory input. | `internal/jstestprovider/external.go:518@c7e353c5` |
| `external-attempt-document-invalid` | The imported document has an unknown shape, trailing data, a profile other than `/3`, or bytes that differ from the rederived canonical qualified document. | `internal/jstestprovider/attempt_details.go`, `DecodeAttemptReceipt` |
| `external-profile-has-attempt-details` | An earlier `/0`, `/1` or `/2` receipt carries the `/3` `attemptDetails` member (AFU-V1-012). | `internal/jstestprovider/projection.go:86@d7cd2a65` |
| `file-bound-exceeded` | A bounded attestation input cannot be read within its byte ceiling. | `internal/jstestprovider/application_attestation.go:164@5714ccbf` |
| `file-replaced` | The opened attestation input is not the file that was inspected before opening. | `internal/jstestprovider/application_attestation.go:160@1bec3465` |
| `invalid-canonical-input` | Canonical input is empty, oversized, or secret-shaped. | `internal/jstestprovider/application_attestation.go:171@12f8b66b` |
| `legacy-external-profile-has-attested-fields` | A legacy `/0` receipt carries `/1` attestation fields. | `internal/jstestprovider/projection.go:104@751bcb11` |
| `noncanonical-input` | Parsed input bytes differ from the canonical JSON encoding. | `internal/jstestprovider/application_attestation.go:187@0bb50e72` |
| `not-regular` | An attestation input path does not resolve to a regular file. | `internal/jstestprovider/application_attestation.go:151@9133b825` |
| `test-repository-drift` | The test repository identity differs between start and publish. | `internal/jstestprovider/external.go:284@0fe9d240` |

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

## V1-2373 attempt failure location amendment

- **PWP-V3-008**: A failed or timed-out `/3` attempt detail may carry an optional
  `failureLocation` (`file`, positive `line`). For the first error of the attempt that has a stack
  frame in the test's own file or a reported location, it is the innermost stack frame in the test's
  own file (so a failure inside a page object called from the test names the calling line of the
  test), and otherwise Playwright's reported error location. It is distinct from `anchor`, which in
  `/3` names the test declaration. The reporter emits it only from such a frame or location and
  never copies the anchor into it; a passed attempt, an empty file or a non-positive line refuses the receipt with
  `external-attempt-details-invalid`. The member is optional and additive: readers that do not use it
  ignore it, and earlier profiles are unchanged. It lets a consumer place a failure at an exact line
  (CCE-V0-009 step 6) instead of matching message text.

Regression: `TestPWPV3008FailureLocation` and the reporter regression in
`qualified-reporter_test.cjs`. Accepted under the owner's standing auto-accept of lane requirements
(2026-10-11). Live qualification of the member on a `PWP-V0-008` tuple is not claimed by this
amendment; until it runs, a consumer must treat an absent member as "location unknown".

## V1-2410 run-level error location amendment

- **PWP-V3-009**: A `/3` report may carry `errorLocations`, one entry per run-level reporter error
  in `errors` order, each either a `{file, line}` location Playwright reported for that error or
  `null`. Any other profile carrying the member refuses `report-unparseable`. When the counts match,
  the receipt's `reporter-global-error` infrastructure failure carries the first non-null entry as
  `location`; unpaired entries carry none. A `/3` run whose tests all passed with a nonzero exit and
  a located global error keeps `reporter-global-error` instead of `exit-status-unexplained`, so a
  consumer can place a worker fixture teardown failure (CCE-V0-048). The member is additive and
  never changes the receipt's qualification. Accepted under the owner's standing auto-accept of lane
  requirements (2026-10-11).

## V1-2620 external evidence hygiene amendment

- **PWP-V3-010**: Before projection, the external row applies the native row's run-evidence hygiene
  (AFU-V1-038: the sensitive-line drop, the secret screen and the attachment filter) to every
  `/0`, `/1` and `/3` test's failure message and artifacts, and to each `/3` attempt entry
  identically, so the final entry still equals the last-attempt fields (PWP-V3-002). The `/2`
  profile is validated and redacted as the reporter wrote it, because the screen's placeholder is
  its redaction marker and a pre-screened `/2` report would read as already redacted and fail
  closed. Failure mode fixed: the reporter joins every Playwright error of an attempt, and when a
  test times out with an action still pending, that action's error carries a "Browser logs" tail
  with the browser launch command line, whose `--password-store=basic` switch is secret-shaped;
  `EncodeQualified` then refused the whole qualified document (`qualified-document-secret-shaped`),
  the CCE control read `infrastructure_failed`, and the live verdict was `INCONCLUSIVE`
  `control-not-killed:control:omitted-event:setup-1` on a loaded host only (CI runs 40163191708,
  40196868668 and 40202373049; reproduced locally under CPU load). Verdict rules, states and the
  reporter are unchanged; a message no hygiene or screening rule touches (no sensitive line, body
  marker, NUL byte or secret-shaped span) is byte-identical. Proposed under
  the owner's standing auto-accept of lane requirements (2026-10-11); rollback removes the one call
  and its test.

## V1-2790 identifier member screening amendment

Status: experimental under the standing lane auto-accept (2026-10-11); V1-2790, GitHub #1890.

- **PWP-V3-011**: `EncodeQualified` and `EncodeKeepReportersQualification` keep the shared writer
  screen (`secretscreen.MatchString`) over the whole canonical document as their first check; a
  document it accepts is unchanged. When it matches, the encoder blanks, in place and at equal
  length, only the interiors of the provider's identifier members and screens the document again,
  then screens each identifier's own text with the credential-name vocabulary masked, except a
  name followed by an assignment: an `=`, or a `:` whose value token is not a plain word of at most
  seven letters (`password: too short` names a concept; `password=x` and `token: hunter2` assign
  and still refuse). Identifier
  members are: the member names of `receipt.identity.testFileDigests`,
  `receipt.identity.configInputDigests`, `receipt.external.projectReporters.qualification.configInputs`
  and the keep-reporters record's `configInputs`; the string values of `receipt.identity.configFile`,
  each test's `name`, `fullName`, `anchor.file`, each attempt detail's `anchor.file` and
  `failureLocation.file`, each schedule start's `fullName` and `file`,
  `receipt.infrastructure.location.file` and each `testProjections[].name`; and a projection anchor
  under `testProjections[].projection.<axis>.anchors[]` or `runProjection.<axis>.anchors[]` only
  when its text is exactly the `file:line` (or `file`) anchor of one of the receipt's own test or
  attempt anchors. Any other match still refuses `qualified-document-secret-shaped`.
  Failure mode fixed: the writer screen's credential-name assignment grammar reads
  `password-recovery.spec.ts:12` (a projection anchor, from any absolute path), a bare
  `"password-recovery.spec.ts":"<digest>"` key, or a title such as `password: too short` as a
  `password` assignment, so an ordinary passing run of a credential-named suite was refused and not
  retained.
- **PWP-V3-012**: A `qualified-document-secret-shaped` refusal is a `QualifiedSecretError` whose
  `Error()` is exactly `qualified-document-secret-shaped` (every reason-code consumer is unchanged)
  and whose `Pointer` names the first matching member as an RFC 6901 JSON pointer. A reference
  token is printed only when it is an array index or a member name the encoded Go type declares
  (collected from its JSON tags); any other token, such as an environment key, a file path or a
  `use` option, stops the pointer at its parent member, so it never carries a file path, a title,
  a dynamic key or the matched text. `corvint-js-test-provider` prints the unchanged refusal line, then one line
  `secret-shaped member: <pointer>` (`(document root)` for an empty pointer).

Non-goals: no change to `internal/secretscreen`, to any other writer, to decoders, to the profile
wire shape or to the refusal code; failure messages, step titles and metadata, attachments and
artifact paths, the environment, argv, `use`, URLs, project and reporter names stay screened in
place. Digest values stay screened in place.

Failure modes and known gap: a credential assignment (`name=value`, or `name: value` whose value is
not a short plain word) inside a path, a test title or a receipt anchor still refuses
(`TestPWPV3011NameValueInIdentifierRefuses`). The residual gap is a `name: value` whose value is a
plain word of at most seven letters, which is refused only by a value-shape rule (vendor token,
credentialed URL, private key, JWT-shaped triple, bearer credential). A refusal pointer withholds
every dynamic member name (`TestPWPV3012PointerWithholdsDynamicKeys`). The local credential-name mask copies the
shared vocabulary; a name the shared screen gains later is not masked, which refuses more, never
less. Malformed JSON stops the member walk, and members after it stay screened as written. The
exact member that refused the live run in #1890 was not observed; the `file:line` projection anchor
is the reproduced candidate. A live `--retain` run on a credential-named suite is `NOT_RUN`.

Rollback: restore the single `secretscreen.MatchString` refusal in both encoders, remove
`qualified_screen.go`, the CLI diagnostic line and their tests.

## V1-2791 browserless tests and page/context override amendment

Status: experimental under the standing lane auto-accept (2026-10-11); V1-2791, GitHub #1891.

- **PWP-V3-013**: On runner 1.63.0 and 1.64.0, a `page` and/or `context` fixture registered on the
  test type (`test.extend`) outside Playwright's own fixture file keeps the effective launch
  identity, and the reporter adds the sorted override names as `use.corvintFixtureOverrides` (part
  of the test identity), when every override resolves, through its cached fixture signature, to
  Playwright's own `browser` fixture; is not boxed; has a signature; and depends on no `playwright`
  or private `_` fixture. A test-type override of `browser`, `playwright`, `_browserOptions`,
  `_combinedContextOptions` or `_optionConnectOptions`, a page/context override from configuration
  or `test.use()`, any override on 1.60.0, and an override test whose worker reported a Playwright
  API launch or connect step outside the `browser` fixture keep unknown identity. The reporter
  recognizes fixtures by their step titles, so a project fixture whose `title` option differs from
  its name makes the fixture registry, and with it the override and every browserless closure,
  unknown. The reconstructed registry places a project option value right after the option's last
  defining registration, so a later `test.extend` override still wins, as in Playwright; a project
  value for a registered non-option fixture is unknown.
- **PWP-V3-014**: On 1.63.0 and 1.64.0, when a test's launch identity is otherwise unknown, the
  reporter records `project.browser: "none"` and `project.use: {"corvintBrowserless": reason}` only
  when no attempt reported a browser, context or page fixture step or any step outside the closed
  browserless vocabulary (hooks, test steps, attachments, other fixtures, generic and API-response
  `expect` matchers, API request calls; the `Worker Cleanup` hook is excluded) and its worker never
  launched a browser outside the `browser` fixture. The reason is closed: `no-browser-use` (the
  fixture closure of the test function, its describe hooks and modifiers and every auto fixture is
  known and reaches no browser, context or page fixture), else `not-run` (every attempt skipped),
  else `fixture-failed-before-browser` (every attempt infrastructure). Any other test keeps
  `browser: ""` and `use: null`. A test with a known identity keeps it.
- **PWP-V3-015**: `bindQualifiedReport` accepts `browser: "none"` only for runner 1.63.0 or 1.64.0,
  `use` exactly one `corvintBrowserless` member with a closed reason, and attempts that agree with
  it (`not-run`: all skipped; `fixture-failed-before-browser`: all infrastructure). Anything else,
  and an empty browser (a test that used a browser without a recorded identity), refuses the run as
  `report-identity-unknown` with the detail still exactly `project-location-unknown`. A
  `no-browser-use` test qualifies on the receipt runtime tuple being `candidate` alone, with every
  other prerequisite unchanged; `not-run` and `fixture-failed-before-browser` tests never project
  passing execution.
- **PWP-V3-016**: `ProjectLocationUnknownTests` names the tests that refusal rejects as
  `/receipt/tests/<index>` pointers, at most eight then `+N more`, carrying no reported text.
  `corvint-js-test-provider` prints `project-location-unknown tests: <pointers>` on a separate stderr
  line for a one-shot `report-identity-unknown` refusal; the refusal line is unchanged.

Non-goals: no change to 1.60.0 identity, to the qualified runner or browser tuples, to the refusal
codes or detail strings, to freshness (which still requires `chromium`), or to watch mode, which
prints no naming line. No browser identity is inferred for a test that used one.

Failure modes and known gaps: the reporter sees only reported steps, so a browser launched inside
a boxed step or an unreported helper in a test whose closure is known and browserless is not
observed; a worker-level launch is attributed only through reported `pw:api` launch/connect steps.
A test that requests only option fixtures (`browserName`, `viewport`, `headless`) under an
unqualified browser configuration is browserless and qualifies; the receipt makes no browser claim
for it. A step title or `expect` matcher Playwright adds later counts as browser use, which refuses
more, never less. Live 1.63.0 and the reporting user's own suite are `NOT_RUN`.

Rollback: remove the fixture-closure, browserless and override paths from the reporter, the
`browserlessReason` acceptance and qualification branch and `ProjectLocationUnknownTests` from
`external.go`, the CLI line, the live-fixture `page` parameters and their tests.

## V1-2844 declared preflight and infrastructure-before-tests amendment

Status: experimental under the standing lane auto-accept (2026-10-11); V1-2844, GitHub #1912.

Intent: an adopter whose external end-to-end suite depends on more than one readiness endpoint, on
setup or seed steps and on saved login state needs a run whose environment was not ready to say so
as infrastructure, naming what was not ready, instead of producing test failures that read as
regressions.

- **PWP-V0-090**: `corvint-js-test-provider e2e --external-server --preflight FILE` reads a closed
  `corvint-js-preflight/0` declaration: `baseUrl` (http or https, no credentials, query or
  fragment), `totalTimeoutMs` (1..3600000) and 1..32 `probes`, each with a unique `name`
  (`[a-z0-9][a-z0-9._-]{0,63}`), a `kind` and its own `timeoutMs` (1..600000). An `http` probe has
  a same-origin `path` and optional `expectStatus` (default any 2xx). A `command` probe has an
  `argv` of 1..64 tokens (a setup or seed step). A `storage-state` probe has a `file` inside
  `--dir`, optional `requireCookies`, `requireOrigins` (at most 32 each) and `minValidSeconds`. A
  probe carrying another kind's members, an unknown member, a declaration over 64 KiB, or a
  secret-shaped declaration refuses `preflight-declaration-invalid` before anything runs. A
  preflight without external mode, or with a caller-owned server or freshness, refuses
  `preflight-requires-external-server`. Both are typed `corvint-playwright-e2e-refusal/0`
  documents (`PWP-V0-034`).
- **PWP-V0-091**: After the `--server-ready-url` readiness check and before application
  attestation and the runner, the probes run in declared order inside the total budget. `http`
  polls GET until an admitted status answers, following no redirect. `command` runs in `--dir` as
  an owned process group with output bounded at 64 KiB and never retained; exit 0 passes.
  `storage-state` reads a Playwright storage-state file (at most 16 MiB) that must have `cookies`
  and `origins` arrays, every required cookie (session cookies, `expires: -1`, pass; any other
  must expire after now plus `minValidSeconds`) and every required origin, and records the file's
  SHA-256. The receipt member `preflight` binds the declaration path and SHA-256, `baseUrl`,
  `outcome` and each probe's `name`, `kind`, `state` (`passed`, `failed`, `not-run`), closed
  `reason`, `artefact` (the probed URL, `argv[0]` or the storage-state path), optional
  `artefactDigest`, a short provider-written `observed` fact (a status, an exit code, a cookie or
  origin name; never output or a cookie value) and `durationMs`. The first failure stops the
  preflight; later probes are `not-run` with `earlier-probe-failed`; a probe the total budget or a
  cancellation ended is `not-run` with `preflight-timeout` or `cancelled`. A declaration whose
  digest changed by the end of a run that reached the runner makes the run
  `input-identity-changed`.
- **PWP-V0-092**: A preflight that did not pass never starts Playwright. The receipt carries no
  tests and the infrastructure failure `preflight-failed` with `phase: "preflight"`, the failing
  `probe` and its `artefact`, and detail `<probe>: <reason>`. Every other infrastructure failure
  carries no phase, so a receipt distinguishes infrastructure before tests from failures observed
  after Playwright started and from test failures. The run projection keeps execution
  `INFRASTRUCTURE` and names `preflight-probe:<name>` and, when known,
  `preflight-artefact:<artefact>` as its anchors; retained-document decoding and MCP discovery
  recompute the same projection.
- **PWP-V0-093**: The receipt shape check requires a preflight member to appear only on an external
  profile other than freshness; probes in the closed vocabulary; at most one first non-passing
  probe, followed only by `earlier-probe-failed`; a passed outcome with no preflight phase; and a
  failed outcome whose infrastructure names exactly that probe and artefact and that carries no
  test. A preflight phase without a member, or any disagreement, is a shape error: encoding
  refuses it and no test can project passing. A passed preflight neither adds nor removes passing
  authority.

Non-goals: no new receipt profile; no retry policy beyond each probe's own polling; no HTTP method,
header, body or cookie injection; no reading of storage-state values; no starting, stopping or
reseeding the application on failure; the owned-server and caller-owned modes are unchanged.

Failure modes and known gaps: a `command` probe runs caller-declared code with the provider's
environment, exactly as `--test-arg` does; its side effects are the caller's. A storage-state file
can pass and still be rejected by the server (revoked session); only the declared structural
validity is checked. An http probe proves the endpoint answered once, not that it stays up; the
existing readiness-at-publish observation is unchanged. Live Playwright with a preflight is
`NOT_RUN` (no Node on the lane host); a failed preflight never reaches Node, and its receipt path is
covered by the focused tests.

Rollback: remove `preflight.go`, the `--preflight` flag and help text, the `preflight` receipt
member, the infrastructure `phase`/`probe`/`artefact` members, the run-projection anchors and the
shape check; receipts without a preflight are byte-identical.

| Requirement | Evidence |
| --- | --- |
| `PWP-V0-090` | `internal/jstestprovider/preflight_test.go` `TestPWPV0090PreflightDeclarationValidation`, `TestPWPV0092PreflightFailureReceiptHasNoTestVerdicts` (admission) |
| `PWP-V0-091` | `internal/jstestprovider/preflight_test.go` `TestPWPV0091PreflightProbesRunInOrderAndStopAtFirstFailure` |
| `PWP-V0-092` | `internal/jstestprovider/preflight_test.go` `TestPWPV0092PreflightFailureReceiptHasNoTestVerdicts`; `internal/testvaliditydoc/preflight_test.go` `TestPWPV0092RetainedPreflightFailureNamesProbe` |
| `PWP-V0-093` | `internal/jstestprovider/preflight_test.go` `TestPWPV0093PreflightShapeRemovesPassingAuthority` |

## V1-2913 complete external receipt projection amendment

Status: experimental under the standing lane auto-accept (2026-10-11); V1-2913, GitHub #1919.

Intent: a test-validity reader of an external-mode run whose tests all executed needs to tell it
from a run that produced no report. Before this amendment the run projection of a complete
external receipt was `UNSUPPORTED` `no-input-supplied` on every axis and each test's freshness read
`no-execution-report`, the same reasons a missing report gets.

- **PWP-V0-153**: When `corvint test-validity` (`internal/testvaliditydoc`) projects an external
  receipt (`/0`, `/1`, `/2` or `/3`) that carries at least one test and whose run projection has
  no execution state (no cancellation, no run-level infrastructure failure), the run execution
  stays `UNSUPPORTED` with reason `external-run-execution-per-test` instead of
  `no-input-supplied`: each test's own execution axis (`PASSED`, `FAILED`, ...) is the verdict.
  The run is not promoted to `PASSED` or `FAILED`, because `DCP-V1-007` forbids promoting a
  retained external witness's native unknown axes. Per-test execution is unchanged; a receipt
  with no tests keeps `no-input-supplied`.
- **PWP-V0-154**: The same projection states the freshness of an external run and of each of its
  tests as `UNKNOWN` with reason `external-application-currency-unverified` instead of
  `no-execution-report`: the application is owned and built outside the provider, and no execution
  identity binds the tested build to the current source. `STALE` and `CURRENT` are kept.

Non-goals: no change to the qualified envelope's embedded `testProjections` and `runProjection`,
which are frozen canonical wire (retained documents still decode byte for byte); no run-level
`PASSED`/`FAILED` fold from the tests (GitHub #1919 asks for one; it conflicts with `DCP-V1-007`
and the doccorpus stability join, and is left to the owner); no `CURRENT`
freshness for an external run, attested or not; no change to the owned-server or freshness
profiles.

Failure modes and known gaps: the qualified envelope's own projections still read the earlier
states, so a consumer that reads those embedded members instead of the test-validity projection
sees `no-input-supplied`. Live evidence of one passing and one failing preflight is `NOT_RUN`: the
lane host has Node v22.22.1 and Playwright 1.63.0, outside the qualified runner tuple (Playwright
1.64.0 on Node v22.23.2 or v24.11.1), so the provider refuses before any preflight.

Rollback: remove `internal/testvaliditydoc/external_projection.go` and its two call sites in
`projectJavaScript`; the projection returns to the embedded states.

| Requirement | Evidence |
| --- | --- |
| `PWP-V0-153` | `internal/testvaliditydoc/external_projection_test.go` `TestPWPV0153CompleteExternalReceiptNamesRunAndFreshness` (recorded `/3` sides `preserves.head`, `regression.head`) |
| `PWP-V0-154` | `internal/testvaliditydoc/external_projection_test.go` `TestPWPV0153CompleteExternalReceiptNamesRunAndFreshness` |

## V1-3636 expected-status amendment

Status: experimental under the standing lane auto-accept (2026-10-11); V1-3636.

Intent: a `test.fail()` test declares `expectedStatus: "failed"`. Before this amendment the
qualified reporter recorded the raw attempt status, so a body that failed as declared read as a
failure and a body that passed read as a pass, the reverse of Playwright's own outcome.

- **PWP-V0-960**: The qualified reporter records an attempt of a test whose `expectedStatus` is
  `failed` by Playwright's outcome: a `failed` attempt is recorded `passed` with no failure
  message, and a `passed` attempt is recorded `failed` (`failureKind` `assertion-or-test`) with the
  message `test was expected to fail (test.fail()) but passed`. Infrastructure, `timedOut`,
  `interrupted` and `skipped` attempts, and tests with any other `expectedStatus`, are unchanged.

Non-goals: no new state value and no `expectedStatus` member in the report (the Go decoder's
closed shape is unchanged). Failure modes: a live Playwright run of a `test.fail()` fixture is
`NOT_RUN` on the lane host (outside the qualified runner tuple). Rollback: remove the
`expectedStatus` branch in `onTestEnd`.

| Requirement | Evidence |
| --- | --- |
| `PWP-V0-960` | Node case `expectedStatus failed inverts the recorded state of a test.fail() test` in `internal/jstestprovider/qualified-reporter_test.cjs`, run by `TestQualifiedReporterSensitiveRedaction` |
