# New E2E Test Acceptance V0

Owner: Russell Lewis
Date: 2026-09-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: human issue https://github.com/beamfall/corvint/issues/394;
`AGENTS.md`; `docs/SPEC-DRIVEN-DEVELOPMENT.md`; existing JS provider and behavior falsification contracts.

## Agent digest
- Claim: A separate companion repeats new Playwright tests, executes approved negative controls and produces one bound assessment and fixed PR body.
- Status: proposed/experimental. Request /0 stays negative-only with unknown freshness. Opt-in request /1 uses the separate proposed PTF profile and actual observed native joins; caller pins never promote freshness.
- Exists: `internal/testacceptance`, `cmd/corvint-tests-accept`; focused checks and actual local browser negative assessment; NEA-V0-008/009 attach per-test repeat, cleanup and order/isolation evidence and render a bound Markdown change-request body; NEA-V0-011 names each run's provider infrastructure cause; NEA-V0-012..016 add the opt-in caller-owned authenticated-disposable app profile and NEA-V0-017 splits blocked verdicts into infrastructure and freshness-unknown.
- Blocked on: parent V1-0541 final clean-bound standalone N2/N32 qualification, CEM/OCM, integration and native completion. Child V1-0556 is integrated and natively completed; its evidence does not complete the parent.
- Read next: Requirements; Trust boundary; Qualification and rollback.

## User and measurable job

A maintainer challenges a new test before review, obtaining actual repeat durations and assertion
strength evidence without manually joining commands. The baseline is separate provider/falsification
runs and manual report assembly. At public base `8f4b866a`, `ReceiptTestProjection` delegates to
`ToTestProjection`, which supplies no positive execution freshness for ordinary outcomes. Declared
commits cannot replace observed freshness. That /0 path remains negative-only. The separate
`playwright-per-test-freshness-v0.md` profile adds optional request/assessment /1 with complete
native receipts, observed source/runtime/dependency currency and assertion-control joins to every
baseline. Its bounded local stable acceptance has source-phase evidence; it does not promote /0,
Core/MCP/flows, generic falsifier authority or the whole requested capability.

## Requirements

- `NEA-V0-001`: A closed bounded request pins clean product/test Git commits and trees, tracked
  test/config/package/lock files, environment label, executable bytes, exact declared test identities
  and independently approved falsification plans/tool identities. Freeze the request digest before execution.
- `NEA-V0-002`: Execute every declared new test 2 to 100 times with retries zero and one worker.
  Retain actual provider receipt digests, outcomes, durations and attempts. Missing/extra/duplicate
  tests, retries or skips cannot be silently dropped. Check immutable inputs before and after execution.
- `NEA-V0-003`: Execute approved negative controls through the separately pinned existing behavior falsification
  executable and verify its retained receipt. Match exact criterion/test/product/config/environment
  identities. A surviving control rejects; absent, unsupported, malformed or infrastructure evidence blocks.
- `NEA-V0-004`: Preserve Association, Hygiene, Freshness and Execution provider axes; derive only
  Strength from matched actual controls. Observed failures/flakes/skips or surviving controls reject.
  Any remaining unknown required axis blocks. Request /0 Freshness UNKNOWN prevents accepted
  outcomes; request /1 requires the separately observed PTF closure and native control join.
- `NEA-V0-005`: Mixed repeat outcomes trigger original and reversed file-filter order probes.
  Retain requested order separately from observed schedule; filters do not prove runner execution order.
  A flaky test remains rejected even if the probe is inconclusive or unavailable.
- `NEA-V0-006`: Repeat and control execution run in a sanitized self-worker, without inherited
  credential environment keys. Retain actual owned-group and observed-descendant cleanup facts and
  observer limitations on every worker. Unknown cleanup blocks; known survivors reject; interruption joins cleanup.
- `NEA-V0-007`: Emit one bounded report tied to request/source identities, environment and build,
  explicit unknowns and a fixed PR body containing only validated IDs/enums/counts. No source text,
  author prose, caller template or credentials enter generated bodies. No outward writes.
- `NEA-V0-008`: Each per-test assessment carries its own repeat counts (passed, failed, other),
  observed duration range, worst cleanup fact over the runs that included it and its control, and
  order evidence. A test with mixed repeat outcomes is additionally run alone exactly
  `IsolationRepeats` (2) times through a title-anchored grep on its own file. Order evidence records
  the original/reversed probe states, whether the requested file order could vary at all
  (`not-varied` for one file), and one isolation status: `failures-not-reproduced-in-isolation`,
  `nondeterministic-in-isolation`, `fails-in-isolation` or `isolation-incomplete`. Only passed/failed
  probe rows are evidence; a missing row, a run reason or another declared test observed in an
  isolation run leaves the status incomplete. Isolation never changes the verdict: the test stays rejected.
- `NEA-V0-009`: The body is a fixed Markdown record containing the overall verdict, the product and
  test-repository commits and trees, environment label, Corvint build, companion executable and request
  digests, repeat count, one row per test (verdict, counts, durations, Strength, cleanup, order and
  isolation states) and each test's reasons and the report unknowns. Every rendered value must match
  its closed identifier, OID, digest, build or reason-code shape; anything else renders `UNVALIDATED`.

- `NEA-V0-010`: (accepted, decision 0507) Optional operator-only
  `accept --temp-root` selects an existing clean absolute directory with no symlink components.
  The parent allocates a private per-run child, propagates it as TMPDIR to all workers, native
  providers and control executors, and removes that child after joined execution or cancellation.
  Workers still require exactly PATH/LANG/LC_ALL/TMPDIR; fixed values remain unchanged except
  TMPDIR may name an existing canonical private directory. Request environment keys cannot
  select the root. Omission preserves TMPDIR=/tmp and the existing wire/digest.

- `NEA-V0-011`: (proposed, standing lane auto-accept 2026-10-11; V1-2431) A run that carries the
  `provider-infrastructure` reason MUST also name the provider's cause in `runs[].infrastructure`:
  the receipt's infrastructure reason code, then `:` and its detail only when the detail is itself
  a code (`^[a-z0-9][a-z0-9-]{0,63}$`); a reason that is not a code reads `unrecognized`. Provider
  free text (stderr, paths, URLs) is never copied. Failure mode: a live run fails with a bare
  `provider-infrastructure`, so a missing browser, an unready server and a timeout cannot be told
  apart (lane cce2-1022u). Acceptance: `TestNEAV0011InfrastructureCause`. Rollback: remove the
  field; a report without it reads as before.

- `NEA-V0-012`: (proposed, standing lane auto-accept 2026-10-11; V1-2688, GH #1789) A /0 request
  MAY declare `external_app` with profile `authenticated-disposable`: the operator has already
  started a disposable app that is reached at the loopback `ready_url` and logs in through a
  Playwright storage-state file. The request then MUST omit `server` and MUST pin a one-element
  `health` command by executable digest. The companion never starts, signals or stops that server;
  it only polls `ready_url` before the runner starts and once after it exits, and neither poll
  follows a redirect. The provider mode itself also refuses a ready URL whose host is not a
  loopback IP literal with `caller-owned-server-not-loopback`. Its cleanup facts
  cover only the processes it owns: workers, the runner and the health hook. A /1 request that
  declares `external_app` is refused with `external-app-freshness-unsupported`; a profile other
  than `authenticated-disposable` with `external-app-profile-invalid`; a declared `server` with
  `external-server-command-present`; an invalid or drifted hook with `external-health-invalid`.
  Failure mode: an app that stops answering by the time the runner exits gives the run the
  `provider-infrastructure` reason with cause `server-unavailable-at-publish`.
- `NEA-V0-013`: (proposed; V1-2688) `external_app.environment` declares at most 16 non-secret
  `{key, value}` pairs. Their values are part of the request, so the approval digest pins them.
  Keys MUST match `^[A-Z][A-Z0-9_]{0,63}$` and be unique. Keys that the worker fixes, or that a
  shell, runtime, loader or network stack reads as configuration, are refused with
  `external-environment-key-reserved`: `PATH`, `LANG`, `TMPDIR`, `HOME`, `SHELL`, `ENV`,
  `BASH_ENV`, `SHELLOPTS`, `BASHOPTS`, `IFS`, `PS4`, `PROMPT_COMMAND`, `CDPATH`, `GLOBIGNORE`,
  `JAVA_TOOL_OPTIONS`, `GCONV_PATH`, `LOCPATH`, `NLSPATH`, `HOSTALIASES`, `RES_OPTIONS`,
  `LOCALDOMAIN`, `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, `NO_PROXY`, `CURL_CA_BUNDLE`, and the
  prefixes `CORVINT_`, `NODE_`, `NPM_`, `YARN_`, `PNPM_`, `COREPACK_`, `DENO_`, `BUN_`, `LD_`,
  `DYLD_`, `LC_`, `PLAYWRIGHT_`, `PW_`, `GIT_`, `SSL_`, `OPENSSL_`, `ELECTRON_`, `XDG_`, `PYTHON`,
  `PERL`, `RUBY`. Failure mode: this list is closed in code, so a configuration variable of a
  runtime it does not name is admitted; such a value is still pinned by the approval digest. A key with a credential segment (`TOKEN`, `SECRET`,
  `PASSWORD`, `PASSWD`, `KEY`, `APIKEY`, `AUTH`, `COOKIE`, `SESSION`, `CREDENTIAL`, `CREDENTIALS`,
  `PRIVATE`, `BEARER`, `JWT`) is refused with `external-environment-key-credential`. A value that
  exceeds 1024 bytes, carries NUL, CR or LF, or matches the shared secret screen is refused with
  `external-environment-value-secret`. Any other shape error is `external-environment-invalid`.
  The worker requires exactly the fixed PATH/LANG/LC_ALL/TMPDIR plus these pairs
  (`worker-environment-unsafe` otherwise). The runner and the health hook receive them. Negative
  control executors keep the fixed four-key environment.
- `NEA-V0-014`: (proposed; V1-2688) `external_app.auth_files` pins one to four storage-state files
  by SHA-256. Each MUST be a clean absolute regular file of at most 1 MiB with no symlink
  component, outside both repository roots, not also a tracked input, and a JSON object whose keys
  are a subset of `cookies` and `origins`. A file that breaks these rules is refused with
  `auth-file-invalid`; bytes that differ from the pin are refused with `auth-file-drift`. Under the
  profile, tracked inputs and runner/health argv are also screened: a secret-shaped input is
  `input-secret-shaped` and a secret-shaped argument is `argv-secret-shaped`. The digest is
  rechecked after the reset hook, before the runner starts, and after every run, so drift adds the
  run reasons `auth-file-drift` and `inputs-drifted` (drift after the reset also skips the runner).
  Storage-state contents are never copied into a report, a body or a diagnostic. Before
  publication, every cookie value and origin storage value of at least 8 bytes is searched for in
  the serialized report, and a match refuses the report with `auth-material-in-report`.
- `NEA-V0-015`: (proposed; V1-2688) Before each repeat or probe run the worker runs the health hook
  with phase `reset`, and after the run with phase `health`. Each invocation reads the hook once,
  checks those bytes against the pin and runs them from a private copy, so a hook replaced after
  validation fails without executing. The hook runs in the test repository
  root under the declared worker environment and a 10-second limit, and reads one
  `corvint-new-e2e-external-health/0` JSON document (`phase`, `kind`, `ordinal`) on stdin. Its stdout is
  discarded unread and never retained. A pass needs exit status 0, no timeout or overflow, and the
  owned group cleaned up. A failed reset adds `external-reset-failed` and skips the runner; a
  failed health check adds `external-health-failed`. Each run records its hook results in
  `external_hooks` as phase, `passed` or `failed`, and cleanup.
- `NEA-V0-016`: (proposed; V1-2688) A report from an `external_app` request carries `external` with
  profile `authenticated-disposable`, `server_ownership: caller`, the ready URL, the health hook
  digest, `environment_sha256` (the digest of the declared pairs) and each auth file's path and
  digest only. It adds the unknowns `external-server-state-caller-owned` and
  `disposable-account-caller-declared`, omits `provider-server-cleanup-unknown` (the companion
  owns no server), and renders the body binding row `Authentication | authenticated-disposable`.
  Requests without `external_app` keep their prior bytes and digests, and so do their reports
  except for the NEA-V0-017 `blocked` field and body line. The adopter guide
  `docs/EXTERNAL-APP-ACCEPTANCE.md` documents the profile and is named in `corvint-test` usage.
- `NEA-V0-017`: (proposed; V1-2688) A /0 assessment or report whose verdict is `blocked` names
  `blocked` as either `infrastructure` or `freshness-unknown`. It is `infrastructure` when a repeat
  run carries any reason, the cleanup state is unknown, repeat evidence or a test row is
  incomplete, control cleanup is unknown, or the control is `invalid`, `not-run` or `blocked`.
  Otherwise everything ran clean and no rejection was found, so acceptance is blocked only because
  /0 cannot observe freshness (`freshness-unknown`). A test with no declared control (`missing`)
  is not an infrastructure failure; its reasons still list `negative-control-incomplete`. The report is `infrastructure` when any blocked
  assessment is. The body adds `**Blocked reason: <reason>**` after the overall verdict and keeps
  the row format. Non-blocked verdicts and /1 omit the field.

## Temporary output scope (V1-0686)

The direct producers are worker `corvint-new-test-output-*` (runner --output), native provider
`corvint-playwright-*` (generated config/reporter, JSON report, Playwright output and staged
application attestation executable/config; scoped runs override native project outputDir inside
that scratch without admitting a caller --output option), and executor `corvint-behavior-control-*` (staged
control/cleanup executable). They use os.TempDir; scoped runs place all beneath the parent-owned
`corvint-acceptance-run-*` child. Control hooks receive that same validated TMPDIR without
adding request-declared environment keys. Cleanup failure refuses report publication. The parent removes residual worker files even when
forced worker retirement prevents deferred cleanup. Abrupt parent SIGKILL/crash cannot run
cleanup: the operator must reserve and retire the selected root, retaining failure evidence.

This is cooperative output placement, not filesystem containment. Node/Playwright/Chromium
profiles and temporary artifacts that honor TMPDIR follow the root; hard-coded paths, browser
installation/cache defaults, arbitrary reporters, server/build outputs, hook workspace artifacts
and transitive imports/runtime caches remain unqualified and may write elsewhere. Reserve those
outputs explicitly by actual producer provenance, with INCOMPLETE coverage where unknown; an
equality-only GENERATED_OUTPUT family label is not a path reservation. Default runs require
reservations for /tmp families and transitive outputs. No hostile concurrent filesystem or
complete browser interruption claim is added. Rollback omits --temp-root; no migration occurs.

## External authenticated-disposable app profile (NEA-V0-012..017)

Threat model. The operator owns the external app, its database and the disposable account, and
supplies them through a pinned health hook and pinned storage-state files. The risks this profile
adds are: (1) the companion could be pointed at a non-local service, so the URL stays an HTTP
loopback IP and the companion never contacts anything else; (2) the companion could kill or
mutate a server it does not own, so it never starts, signals or stops the server, and its cleanup
claims cover only its own process groups; (3) real credentials could leak into durable evidence,
so declared values are screened, credential-named keys are refused, storage-state files stay
outside the repositories, and their contents never reach a report, body or diagnostic (a
fail-closed search refuses publication if any cookie or storage value appears); (4) ambient
configuration could change the run without approval, so every key, value and file is pinned in
the approval digest and rechecked after each run. The account is disposable only because the
operator declares it so. The companion cannot verify that, nor whether the server state is fresh
or the storage state still logs in; those stay report unknowns. Storage-state files are
authentication material by design: the profile limits their exposure but does not make them safe
to share.

| Code | Observed condition |
|---|---|
| `external-app-freshness-unsupported` | `external_app` declared under request /1. |
| `external-app-profile-invalid` | Profile is not `authenticated-disposable`. |
| `external-server-command-present` | `server` is declared although the caller owns the server. |
| `external-health-invalid` | Health argv is not one bounded regular executable whose bytes match its pin. |
| `external-environment-invalid`, `external-environment-key-reserved`, `external-environment-key-credential`, `external-environment-value-secret` | Declared environment shape, reserved key, credential-named key, or secret-shaped/oversized value (NEA-V0-013). |
| `auth-file-invalid`, `auth-file-drift` | Storage-state path, size, location, shape or duplication is invalid, or its bytes differ from the pin. |
| `input-secret-shaped`, `argv-secret-shaped` | A tracked input or a runner/health argument matches the shared secret screen under the profile. |
| `auth-material-in-report` | A storage-state value would appear in the serialized report; nothing is published. |
| `external-reset-failed`, `external-health-failed` | Run reasons: the reset hook failed (runner skipped) or the post-run health hook failed. |
| `server-unavailable-at-publish` | Provider infrastructure cause: the caller-owned server did not answer after the runner exited. |
| `caller-owned-server-not-loopback` | Provider admission: the caller-owned server mode was given a ready URL that is not `http` on a loopback IP literal, or that carries credentials, a query or a fragment (NEA-V0-012). |
| `caller-owned-server-composition-unsupported` | Provider admission: the caller-owned server mode was combined with a provider-started server, the external PWP mode, freshness, or an empty ready URL. |

Non-goals: starting or provisioning the app, creating or logging in the account, refreshing
storage state, judging that the account is disposable, forwarding declared values to negative
control executors (a BBF plan declares its own environment and authentication, BBF-V0-217), and
any non-loopback or credential-bearing URL.

Rollback: stop declaring `external_app`. The fields are omitted when absent, so other requests,
digests and reports are unchanged. Reverting the change removes the profile and the `blocked`
field; any consumer that read `blocked` sees it absent again.

## Trust boundary and resource limits

Operator-approved executable/config/plan bytes are trusted code, not third-party author authority.
The approval digest is separately supplied; coherence is not authenticated operator/hook semantics.
Run only disposable local fixtures under independently approved plans. Source roots must be clean,
absolute Git roots with pinned immutable contents. Trusted fixtures/config may execute code; CLI
sanitizes worker environment to PATH/LANG/LC_ALL/TMPDIR and excludes outward credentials.
JSON inputs 4 MiB, tests at most 32, repeats at most 100, processes at most 60 seconds per baseline,
report 16 MiB. Captures are bounded. Failures retain reason enums, never raw child output.
A 20ms observer can miss fast detach and is bounded observation, not full containment. Provider
identity/version limitations remain explicit. No `RequireDescendantCleanup` flag: current runtime
rejects that unsupported contract before process launch.

## Failure modes and reason codes

The following codes describe existing validation or report behavior. A refusal prevents execution
or publication of a report; a retained Strength reason classifies the actual control observation.
They confer no positive freshness or acceptance qualification.

| Codes | Observed condition |
|---|---|
| `input-bound` (internal/testacceptance/input.go:34), `input-depth-bound` (internal/testacceptance/input.go:52), `input-trailing` (internal/testacceptance/input.go:41), `input-shape` (internal/testacceptance/input.go:46) | JSON exceeds the input/depth bound, carries trailing data, or cannot decode into the closed request shape. |
| `request-bound` (internal/testacceptance/input.go:150) | Schema, environment ID, repeat count, timeout, test count or input count falls outside the closed request limits. |
| `path-not-absolute` (internal/testacceptance/input.go:93), `path-symlink-or-absent` (internal/testacceptance/input.go:98), `file-not-bounded-regular` (internal/testacceptance/input.go:103) | A file path is not clean and absolute, has a missing/symlink component, or is not a regular file within the file-size bound. |
| `git-observation-failed` (internal/testacceptance/input.go:119) | The bounded Git observation fails to start, finish successfully or clean up its owned group, or overflows output. |
| `repository-pin-invalid` (internal/testacceptance/input.go:125), `repository-root-mismatch` (internal/testacceptance/input.go:129), `repository-commit-drift` (internal/testacceptance/input.go:133), `repository-tree-drift` (internal/testacceptance/input.go:137), `repository-not-clean` (internal/testacceptance/input.go:141) | Repository root/OID pins are malformed, the observed root/HEAD/tree differs, or tracked/untracked work prevents a clean observation. |
| `input-pin-invalid` (internal/testacceptance/input.go:162), `input-outside-roots` (internal/testacceptance/input.go:170), `input-not-tracked` (internal/testacceptance/input.go:174), `input-byte-drift` (internal/testacceptance/input.go:178), `configuration-not-pinned` (internal/testacceptance/input.go:183) | An input path/hash is invalid or duplicated, outside both repositories, untracked or byte-mismatched; config/package/lock inputs must be pinned inside the test repository. |
| `test-identity-invalid` (internal/testacceptance/input.go:189), `test-identity-duplicate` (internal/testacceptance/input.go:194) | Test ID/file/line/title/project is invalid or duplicated; file/title pairs must be unique. |
| `control-tool-required` (internal/testacceptance/input.go:198), `control-command-invalid` (internal/testacceptance/input.go:202), `control-tool-byte-mismatch` (internal/testacceptance/input.go:206) | Control plan, tool identity and command must be present together; the command must be one bounded regular executable whose observed bytes match both pins. |
| `control-binding-mismatch` (internal/testacceptance/input.go:211), `control-environment-not-safe` (internal/testacceptance/input.go:215), `control-bound` (internal/testacceptance/input.go:219) | Control target/repository/config/runner identities differ, declared environment keys exceed LANG/LC_ALL, or control timeout/wall-clock/attempt/count ceilings are exceeded. |
| `command-invalid` (internal/testacceptance/input.go:225), `executable-drift` (internal/testacceptance/input.go:229), `argv-invalid` (internal/testacceptance/input.go:233) | Runner/server executable or argv count/hash/file shape is invalid, observed executable bytes drift, or an argument exceeds its size/character limits. |
| `server-entrypoint-not-pinned` (internal/testacceptance/input.go:238), `runner-prefix-invalid` (internal/testacceptance/input.go:243), `runner-entrypoint-invalid` (internal/testacceptance/input.go:248) | The server entrypoint must be pinned inside the product repository; the runner prefix cannot exceed executable plus one independently byte-pinned CLI entrypoint. |
| `ready-url-not-loopback` (internal/testacceptance/input.go:253), `build-root-invalid` (internal/testacceptance/input.go:256), `runner-version-invalid` (internal/testacceptance/input.go:264) | Readiness requires an HTTP loopback IP without user/query/fragment; build root must be clean/absolute inside the product root; declared runner version must be nonempty and bounded. |
| `temporary-root-invalid` (internal/testacceptance/execute.go:383), `temporary-root-allocation-failed` (internal/testacceptance/execute.go:387), `temporary-root-cleanup-failed` (internal/testacceptance/execute.go:392) | Selected root is noncanonical/absent, per-run allocation fails, or post-join scope removal fails; no report is published. |
| `self-worker-invalid` (internal/testacceptance/execute.go:378), `self-worker-unreadable` (internal/testacceptance/execute.go:407), `report-bound` (internal/testacceptance/execute.go:489) | The trusted companion executable is not a bounded regular absolute file, its bytes cannot be read, or final JSON serialization/size prevents report publication. |
| `verified-approved-control-survived` (internal/testacceptance/report.go:95), `qualified-baseline-identity-unknown` (internal/testacceptance/report.go:98) | A verified approved control survived, so Strength is SURVIVED and the test is rejected; a killed control with unqualified baseline identity leaves Strength NOT_MEASURED and acceptance blocked. |
| `isolation-index-invalid` (internal/testacceptance/execute.go:164), `isolation-not-observed` (internal/testacceptance/execute.go:273), `isolation-incomplete` (internal/testacceptance/report.go:243) | An isolation job names no declared test, an isolation run observed another declared test, or fewer than two passed/failed isolated rows were retained; order evidence stays incomplete. |

## Non-goals

Core registration, remote endpoints, writer credentials, connector publication, promoting old /0
provider projections or freshness, hostile concurrent filesystem containment, arbitrary
third-party test execution, or full browser tuple qualification. Optional output is stdout for
operator-controlled redirection; no product file writer or store migration.

## Acceptance evidence and traceability

| Requirements | Implementation | Evidence |
|---|---|---|
| NEA-V0-001 | closed request/pinning | malformed, approval drift and dirty-root tests |
| NEA-V0-002, NEA-V0-005 | actual provider repeats/probes | disposable browser stable, nonasserting and alternating fixtures |
| NEA-V0-003, NEA-V0-004 | control execution/projection | actual killed/survived controls; blocked stable freshness |
| NEA-V0-006 | sanitized worker/observer | sentinel isolation, nested group cancellation and cleanup tests |
| NEA-V0-007 | fixed report/body | bounded body and unknown preservation tests |
| NEA-V0-008 | per-test summary, isolation probes | `TestNEAV0008AttachedOrderEvidence` (all isolation statuses, cleanup attribution, grep anchoring); actual browser `NEA-V0-008/attached-order-evidence` |
| NEA-V0-010 | validated per-run root and standalone worker scope | `TestNEAV0010TempRootValidation`, `TestNEAV0010StandaloneScope`, `TestNEAV0010NativeTempScope`, `TestNEAV0010NativeOutputSelection` |
| NEA-V0-011 | named infrastructure cause per run | `TestNEAV0011InfrastructureCause` (code reason and code detail joined; free-text detail dropped; non-code reason `unrecognized`; `markInfrastructure` sets the reason and the cause together; field absent without a failure; a worker-level run with a forced provider failure is NOT_RUN); actual browser NEA-V0-002 through `corvint-test accept worker` PASS 2026-10-25 |
| NEA-V0-012..016 | `internal/testacceptance/external.go` (`validateExternal`, `externalHook`, `externalReport`), `execute.go` (`RunWorker`, `ExecuteWithWorker`), `internal/jstestprovider/caller_owned.go` (`runCallerOwned`) | `TestNEAV0012ExternalProfileValidation`, `TestNEAV0012CallerOwnedServerRun`, `TestNEAV0013DeclaredEnvironment`, `TestNEAV0014AuthFilePinning`, `TestNEAV0014AuthMaterialNeverPublished`, `TestNEAV0015HealthHookFailure`, `TestNEAV0016LegacyWireUnchanged`, `TestCallerOwnedServerAdmission`, `TestCallerOwnedServerUnavailableAtPublish` |
| NEA-V0-017 | `internal/testacceptance/report.go` (`blockedReason`) | `TestNEAV0017BlockedReasonSplit` |
| NEA-V0-009 | Markdown change-request body | `TestNEAV0009ChangeRequestBody` (bindings, row, `UNVALIDATED` substitution, no title text); actual browser `NEA-V0-007/fixed-body` |

Focused conformance and independent review qualify only their named local paths. Synthetic tests
can test fail-closed classification but cannot qualify actual browser acceptance. Record actual
browser versions and failures in the build log. Tests never accept proposed human intent.

## Qualification and rollback

Availability smoke observed Node22.23.3 and Chromium153.0.8010.12; not the provider's qualified
Node22.23.2 tuple. Actual disposable negative assessment passed: stable asserting BLOCKED, nonasserting REJECTED on surviving response mutation, flaky REJECTED with requested probes. Observed execution order remains UNKNOWN. V1-0556 delivered the separate observed per-test freshness slice and completed its integration and native gates on 2026-10-02. On 2026-10-01 the same tuple also passed NEA-V0-008/009: the flaky test was
run alone twice (passed, failed), recorded `nondeterministic-in-isolation` with file order
`not-varied`, and the body rendered both revisions, build and per-test rows. The
`failures-not-reproduced-in-isolation` (order-dependent) branch is covered by synthetic rows only.
Promotion needs a separately reviewed observed freshness contract and actual
qualified identity/environment/browser evidence. Keep the issue open through integration/native
completion. Rollback: stop invoking the optional companion and revert its isolated commits;
retain assessment evidence. Version incompatible request/report changes and re-run focused checks.

## Optional native freshness profile and remaining whole-issue qualification

Request/assessment /1 explicitly opts into `corvint-playwright-freshness/0`, retains full canonical
baseline/control native bytes and rederives observed freshness and measured control strength for
every repeat. Child V1-0556 is integrated at `c5685a3389eea49fb0a465c343a6ee6bb37665ec` and
natively completed (receipt 2169); its source, frozen checks and scoped positive evidence remain
separate from the parent qualification below. Historical source-phase descriptions in its build
log describe their original observation boundary, not its current native completion state.

The parent complete-attempt join now derives KILLED or SURVIVED only when every planned control
attempt has matching verified generic and native outcomes, complete current closure and every
baseline. A measured survivor rejects and preserves exact generic CoverageGaps. Incomplete,
unsupported or inconsistent evidence remains NOT_MEASURED with conservative survivor rejection.
Pure `TestPTFV0ParentCompleteAttemptOutcome` regressions cover supported attempt bounds, mixed
outcomes, incomplete transcripts and generic-valid native forgeries. They are not live qualification.

The mandatory standalone witnesses are `TestPTFV0ParentStandaloneStableAndSurvivedLive` and
`TestPTFV0ParentStandaloneFlakyN32Live`. The latter consumes one immutable random N32 batch at
its final clean source/spec/metadata/committed-CEM binding. It requires actual mixed repeats,
original/reversed probes and two isolated rows, without selecting an isolation pattern. One file
retains `not-varied`; requested filters never prove observed schedule. Both tests retain actual
bounded JSON and complete body fields. `TestPTFV0ParentBodyBinding` independently rejects corrupt
binding entries and per-test rows; the build discriminator proves only absolute Go launch.

Whole #394/V1-0541 stays OPEN through final qualification, integration and native completion.
The final keyed evidence, including a failed or homogeneous N32 batch, is retained separately;
no source-time statement predicts its outcome. Old-profile witnesses and the parent's earlier N2
scope mismatch are not final new-profile qualification. See the current parent build-log entry
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-02-new-e2e-remaining-qualification.md` for review and scope limits.

### Bounded descendant witnesses (NEA-V0-006/007)

The observer retains at most 4096 resident PID/start identities including its immutable root
anchor. Still-visible zombies retain ancestry and deduplication slots but are not live cleanup
targets. Only a successful bounded, wholly parsed snapshot showing absence or a different start
retires a resident identity. Failed, incomplete, overflowed, malformed or duplicate-PID snapshots
cannot justify retirement. Matching owned descendants remain tracked after reparenting; a reused
PID gains no ownership without a current exact matching owned parent. Removal of an old identity
precedes capacity checks for an independently admitted replacement.

Cleanup considers all owned unresolved identities independently of displayed evidence.
`Processes` retains at most 4095 descendant witness rows, deterministically prioritizing unresolved
identities, then other resident identities, then historical witnesses. It is not an exhaustive
historical process list. Fixed limitations disclose omitted witnesses and any failure-related
omissions; omission never makes unresolved cleanup or resident overflow count as absence.
The existing fields, `Scope`, 4 MiB snapshot stdout bound, PID/start representation, transient
snapshot/signal recovery semantics and observation limitations remain unchanged. The aggregate
16 MiB report limit still refuses over-limit reports; a per-observer row bound does not promise
that every maximum-sized aggregate fits.

Source and focused evidence for this repair are recorded in the build-log entry for V1-0689.
The original N32 result remains unqualified because its outer descendant observer exhausted its
bound. Retained receipt inspection subsequently established that `provider-validity-incomplete`
comes from the flaky fixture's observed FAILED execution rows; all baseline Association, Hygiene
and Freshness axes are ASSOCIATED, ELIGIBLE and CURRENT. This reason preserves expected rejection
under NEA-V0-004 and PTF-V0-008; it is not a separate missing-evidence defect. The observer repair
neither retroactively qualifies that run nor authorizes another campaign. Any separately authorized
post-repair campaign must preserve the original consumed marker, enrollment and raw evidence.
