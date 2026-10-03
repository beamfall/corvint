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
- Exists: `internal/testacceptance`, `cmd/corvint-tests-accept`; focused checks and actual local browser negative assessment; NEA-V0-008/009 attach per-test repeat, cleanup and order/isolation evidence and render a bound Markdown change-request body.
- Blocked on: final V1-0556 freshness binding/checks/integration; whole V1-0541/#394 remains open for qualified new-profile surviving Strength and actual flaky/order evidence, plus native completion.
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
| `request-bound` (internal/testacceptance/input.go:147) | Schema, environment ID, repeat count, timeout, test count or input count falls outside the closed request limits. |
| `path-not-absolute` (internal/testacceptance/input.go:93), `path-symlink-or-absent` (internal/testacceptance/input.go:98), `file-not-bounded-regular` (internal/testacceptance/input.go:103) | A file path is not clean and absolute, has a missing/symlink component, or is not a regular file within the file-size bound. |
| `git-observation-failed` (internal/testacceptance/input.go:119) | The bounded Git observation fails to start, finish successfully or clean up its owned group, or overflows output. |
| `repository-pin-invalid` (internal/testacceptance/input.go:125), `repository-root-mismatch` (internal/testacceptance/input.go:129), `repository-commit-drift` (internal/testacceptance/input.go:133), `repository-tree-drift` (internal/testacceptance/input.go:137), `repository-not-clean` (internal/testacceptance/input.go:141) | Repository root/OID pins are malformed, the observed root/HEAD/tree differs, or tracked/untracked work prevents a clean observation. |
| `input-pin-invalid` (internal/testacceptance/input.go:159), `input-outside-roots` (internal/testacceptance/input.go:167), `input-not-tracked` (internal/testacceptance/input.go:171), `input-byte-drift` (internal/testacceptance/input.go:175), `configuration-not-pinned` (internal/testacceptance/input.go:180) | An input path/hash is invalid or duplicated, outside both repositories, untracked or byte-mismatched; config/package/lock inputs must be pinned inside the test repository. |
| `test-identity-invalid` (internal/testacceptance/input.go:186), `test-identity-duplicate` (internal/testacceptance/input.go:191) | Test ID/file/line/title/project is invalid or duplicated; file/title pairs must be unique. |
| `control-tool-required` (internal/testacceptance/input.go:195), `control-command-invalid` (internal/testacceptance/input.go:199), `control-tool-byte-mismatch` (internal/testacceptance/input.go:203) | Control plan, tool identity and command must be present together; the command must be one bounded regular executable whose observed bytes match both pins. |
| `control-binding-mismatch` (internal/testacceptance/input.go:208), `control-environment-not-safe` (internal/testacceptance/input.go:212), `control-bound` (internal/testacceptance/input.go:216) | Control target/repository/config/runner identities differ, declared environment keys exceed LANG/LC_ALL, or control timeout/wall-clock/attempt/count ceilings are exceeded. |
| `command-invalid` (internal/testacceptance/input.go:222), `executable-drift` (internal/testacceptance/input.go:226), `argv-invalid` (internal/testacceptance/input.go:230) | Runner/server executable or argv count/hash/file shape is invalid, observed executable bytes drift, or an argument exceeds its size/character limits. |
| `server-entrypoint-not-pinned` (internal/testacceptance/input.go:235), `runner-prefix-invalid` (internal/testacceptance/input.go:240), `runner-entrypoint-invalid` (internal/testacceptance/input.go:245) | The server entrypoint must be pinned inside the product repository; the runner prefix cannot exceed executable plus one independently byte-pinned CLI entrypoint. |
| `ready-url-not-loopback` (internal/testacceptance/input.go:250), `build-root-invalid` (internal/testacceptance/input.go:253), `runner-version-invalid` (internal/testacceptance/input.go:261) | Readiness requires an HTTP loopback IP without user/query/fragment; build root must be clean/absolute inside the product root; declared runner version must be nonempty and bounded. |
| `self-worker-invalid` (internal/testacceptance/execute.go:296), `self-worker-unreadable` (internal/testacceptance/execute.go:305), `report-bound` (internal/testacceptance/execute.go:383) | The trusted companion executable is not a bounded regular absolute file, its bytes cannot be read, or final JSON serialization/size prevents report publication. |
| `verified-approved-control-survived` (internal/testacceptance/report.go:82), `qualified-baseline-identity-unknown` (internal/testacceptance/report.go:85) | A verified approved control survived, so Strength is SURVIVED and the test is rejected; a killed control with unqualified baseline identity leaves Strength NOT_MEASURED and acceptance blocked. |
| `isolation-index-invalid` (internal/testacceptance/execute.go:137), `isolation-not-observed` (internal/testacceptance/execute.go:225), `isolation-incomplete` (internal/testacceptance/report.go:201) | An isolation job names no declared test, an isolation run observed another declared test, or fewer than two passed/failed isolated rows were retained; order evidence stays incomplete. |

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
| NEA-V0-009 | Markdown change-request body | `TestNEAV0009ChangeRequestBody` (bindings, row, `UNVALIDATED` substitution, no title text); actual browser `NEA-V0-007/fixed-body` |

Focused conformance and independent review qualify only their named local paths. Synthetic tests
can test fail-closed classification but cannot qualify actual browser acceptance. Record actual
browser versions and failures in the build log. Tests never accept proposed human intent.

## Qualification and rollback

Availability smoke observed Node22.23.3 and Chromium153.0.8010.12; not the provider's qualified
Node22.23.2 tuple. Actual disposable negative assessment passed: stable asserting BLOCKED, nonasserting REJECTED on surviving response mutation, flaky REJECTED with requested probes. Observed execution order remains UNKNOWN. V1-0556 owns the separate observed per-test freshness slice and its final delivery gates. On 2026-10-01 the same tuple also passed NEA-V0-008/009: the flaky test was
run alone twice (passed, failed), recorded `nondeterministic-in-isolation` with file order
`not-varied`, and the body rendered both revisions, build and per-test rows. The
`failures-not-reproduced-in-isolation` (order-dependent) branch is covered by synthetic rows only.
Promotion needs a separately reviewed observed freshness contract and actual
qualified identity/environment/browser evidence. Keep the issue open through integration/native
completion. Rollback: stop invoking the optional companion and revert its isolated commits;
retain assessment evidence. Version incompatible request/report changes and re-run focused checks.

## Optional native freshness profile and remaining whole-issue qualification

Request/assessment /1 explicitly opts into `corvint-playwright-freshness/0`, retains full canonical
baseline/control native bytes and rederives observed freshness and qualified killed strength for
every repeat. Source-phase actual stable acceptance is observed on the named PTF tuple; final
frozen checks, CEM/OCM and integration/native completion remain separate gates.

Whole #394/V1-0541 remains PARTIAL. The old-profile actual nonasserting and flaky witnesses remain
historical evidence, not new-profile qualification. In current /1, a survived control rejects with
`negative-control-survived`, but Strength remains NOT_MEASURED because the native join qualifies
only kills. No actual /1 mixed/flaky order/isolation witness has been retained. Completing those
existing requested capabilities needs a separately admitted parent slice, without a third freshness
reader repair, a new wire shape or broad tuple promotion. The fixed rendered body and per-test order
attachment already exist; their new-profile negative consuming paths still need actual witnesses.

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
The original N32 result remains unqualified, and `provider-validity-incomplete` remains a separate
blocker. This observer repair neither retroactively qualifies that run nor authorizes another
campaign.
