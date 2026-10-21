# 2026-10-21 gh-issues lane: RSpec denial advisory, #1358 fixture, nightly #1116 evidence, bun provider, latency contention

Tickets: V1-2124 (GitHub #1366), V1-2125 (GitHub #1358), V1-2121 and V1-2122 (GitHub #1116),
V1-2111, V1-2191 (GitHub #1405), V1-2192 (GitHub #1406).

## Problem

- #1366: an RSpec request example that asserts only a 401/403 refusal status of a mutating request
  proves the caller saw a refusal, not that nothing changed; a handler that writes before it
  denies stays green.
- #1358: the issue reports that a block handed to a helper that never yields passes validity.
- #1116: the nightly Release gates failed on runs 39121848489, 39258757168 and 39416257984.
- #1405: affected selection already recognised `bun:test` suites (`bun-test` units), but the JS
  provider ran only Vitest and Playwright, so a Bun suite produced no receipt and no verdict.

## Decisions

- `TSA-V0-564` and `TSA-V0-565` are added to `docs/specs/test-source-analysis-v0.md` (IDs offset
  +100 above the next free ID on origin/main, 464). They are accepted under the owner's standing
  auto-accept of lane requirements (2026-10-11).
- `RB-DENIAL-WITHOUT-NO-EFFECT` is advisory only and changes no axis, as the issue asks. The
  abstentions `RB-DENIAL-VERB-UNRESOLVED` (no literal request call, or a let/subject shadowing the
  request helper) and `RB-DENIAL-MATCHER-CUSTOM` (a refusal or negated block matcher the reader
  does not know) are advisory too: an `abstained` severity would turn ELIGIBLE hygiene into
  ABSTAINED, which the issue rules out ("never makes the example INELIGIBLE") and which would
  regress every unrelated denial example.
- The request is read in the example body, the subject and lets it (or a hook) calls by name, and
  the before(:each)/let!/subject! hooks of its group chain; `get`/`head` denials are never flagged.
- #1358 is already delivered by V1-1866 (`b4cbca6f8`, `4aaef55de`; TSA-V0-084). The lane adds the
  issue's exact shape as `TestRSpecIssue1358NonYieldingHelper`: the multi-line commented helper in
  the spec file and in an included shared_context is `RB-BLOCK-NEVER-YIELDED` at the helper
  definition; the rewrite and the yield, `block_given?` and `&block` helpers stay clean; a helper
  in a support module abstains with `RB-HELPER-UNRESOLVED` (pinned, not flagged).
- #1116: every failure in the three failing runs is already fixed on main, by root cause, without
  skips: `TestATRV0004` group-kill race (`00c904e78`, V1-2103), `TestTQDV0021_RealProbes` helper fd
  and the no-Python conformance exemption (`7f46e42dc`, V1-1780, V1-1970),
  `TestMCPV0038IndexDeadlineHasDistinctCode` caller deadline (`f8c33fba7`, V1-1771), and the other
  groupreap callers (`becfcd18d`, V1-2111). The macOS `internal/mcp/bridge` package passed in run
  39416257984.

- #1405: `LPCV-V0-254..262` (IDs offset +100 above origin/main's LPCV-V0-153), accepted under
  the owner's standing auto-accept (2026-10-11), add `corvint-js-test-provider bun`: `bun test
  --reporter=junit` under procgroup containment, the exact argv (preload, `--max-concurrency`,
  `--test-name-pattern`, files), a `bun` receipt member (producer build from embedded VCS info,
  bun executable digest and observed version, `bun.lock` digest and version, preload digests,
  declared isolation, exit, start/publish repository identity), and three typed pre-run refusals
  (`bun-version-unqualified`, `bun-lockfile-unqualified`, `bun-isolation-concurrency-undeclared`)
  that exit 3 with a `corvint-bun-test-refusal/0` document. The qualified set is exactly bun
  1.3.11, the version a live run observed. Example-level selection is `--test-name-pattern`:
  bun 1.3.11 has no `file:line` filter. A failing `beforeAll` hook (an unnamed, line-less case)
  is one infrastructure outcome. `bun:test` joins `vitest`/`jest` as a test-framework module in
  the JS flow reading. The receipt joins through the existing example join unchanged. The
  projection check is chained onto the existing return line so no cited line moves.
- #1406: `AHI-171` (next free AHI ID 071, offset +100), accepted under the owner's standing
  auto-accept, records load averages, logical CPUs and the run queue (Linux) at each OpenCode timing
  window's start and end, `NOT_OBSERVED` where the host does not expose a value, and classifies the
  decisive attempt `PASS`, `LATENCY_HOST_CONTENDED` or `LATENCY_REGRESSION`. A latency-only,
  complete-sample miss whose window peaked above 0.5 load per CPU makes the report `RETRY_LATER`
  with a retry hint, a typed error and `qualify-opencode` exit 75; the consumer record still
  requires `PASS`, and limits are unchanged. The threshold is declared, not measured: the issue's
  failing host ran at about 11.5 load on 12 CPUs (0.96). The same-window baseline (issue item 2)
  is a non-goal here.

## Evidence

- `TestRSpecDenialWithoutNoEffect`, `TestRSpecDenialRequestOnChain`: pass; both fail with the
  `denialFindings` call removed from `exampleFindings`.
- `TestRSpecIssue1358NonYieldingHelper`: passes; fails when `neverYields` is forced false.
- `./internal/testsource` full package: pass.
- Local repeats on this host at the lane base: `TestATRV0004*` and `TestTQDV0021_RealProbes` at
  `-count=15`, `TestMCPV0038IndexDeadlineHasDistinctCode` at `-count=30`: all pass.

- Bun: `TestLPCVV0255_RunBunBindsCommandIdentityAndTree`, `TestLPCVV0258_BunRefusalsAreTypedAndPreRun`,
  `TestLPCVV0260_ParseBunJUnitLiveReport` (a report bun 1.3.11 wrote), `TestLPCVV0260_ParseBunJUnitHookAndShape`,
  `TestLPCVV0261_BunFixtureSelectedRunAndJoined`, `TestProviderBunRefusalExitsAdmissionRefused` and the
  extended `TestProviderHelpPrintsEverySubcommandUsage`: pass; without the change they do not compile
  (new API) or, for the help test, omit `bun `. `TestLPCVV0262_LiveBunQualification` passed with
  `CORVINT_BUN_EXECUTABLE` set to the pre-installed bun 1.3.11 (darwin arm64; nothing downloaded).
  A live CLI run on a scratch fixture (file: dependency, `bun.lock` version 1) produced the
  receipt with 4 outcomes, exit 1 explained by the failed test, and an unchanged clean tree; the
  three refusals exited 3 with their documents.
- `TestAHI171LatencyHostContention` (and `TestAHI056LatencyResample`) pass; without the change the
  test does not compile (new API). Its live subtest read this macOS host's load through `sysctl`.

## NOT_RUN

- A passing scheduled nightly Release-gates run after these fixes is not yet observed; the
  hosted-runner timing classes are covered by local repeats only.
- Real-suite qualification of the denial reading against an adopter's Rails suite.
- Bun live qualification in CI (no bun installed there); bun releases other than 1.3.11, Linux
  and Windows bun; an adopter's real Bun repository with an external service.
- AHI-171: no live qualification campaign under synthetic CPU load (the issue's acceptance
  scenario), so the PR references #1406 without closing it; Linux `/proc/loadavg` reading not run
  here; no same-window baseline.

## Rollback

Remove `internal/testsource/validity_rspec_denial.go`, its test and the `denialFindings` call, the
three rule constants, the help-text sentence in `cmd/corvint/tests.go`, the TSA-V0-564/565 section
and the `TestRSpecIssue1358NonYieldingHelper` file and traceability mention. For the bun profile,
revert `internal/jstestprovider/bun.go`, the `Bun` receipt member, the chained projection check, the
`bun` subcommand, the `bun:test` library entry, their tests and fixtures, and the LPCV-V0-254..262
section; `bun` then exits 2 as an unknown subcommand. For AHI-171, revert
`internal/opencodequalification/contention.go`, its `native.go` call sites and the exit-75 branch. Nothing is persisted.
