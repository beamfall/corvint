# Batch 1 closeout evidence: flows S5-S8 and JS runner gaps

Date: 2026-10-14. Base: `495eda08cf3dc4d8122b169a4cc16d9c4e599b19` (origin/main).
Tickets: V1-0252, V1-0253, V1-0254, V1-0255 (`docs/specs/application-flow-understanding-v1.md`,
decision 0385) and V1-0612, V1-0620 (`docs/specs/test-runner-execution-v0.md`).

## Intent

All six tickets were open although their delivery had merged. This entry re-checks each acceptance
criterion against original evidence at the base: merged commits, focused tests rerun here, and
retained qualification records. It is the evidence anchor for the closeout. It adds no requirement
and no runtime change. The full gate needed by the four flows tickets is not run in this lane.

## Change

`application-flow-understanding-v1.md` still described AFU-V1-029 observer integration as partial and
blocking. That was stale. Navigation execution (NEX-V0, accepted by decision 0455) merged in #675
(`73a6734c0`, `231152133`, `37bc213a1`). `internal/appflows/navigation_execution.go:225` applies
`AdmitTransition` to every step before startup. `:346` rechecks every allowed transition in the
receipt. `tools/web-flows/navigation.mjs:28` admits each request through the policy before forwarding.
`tools/web-flows/test/navigation-e2e.mjs:87` is the real-browser case. The matrix row, the
traceability row and the Agent digest now say this. The header delivery status is unchanged
(`partially delivered`), because AFU-V1-006 external runtime qualification is still unobserved.

## Focused tests rerun at the base (all PASS; `GOTOOLCHAIN=local`, go1.27.2, `-count=1`)

- `internal/appflows`: 14 `TestAFUV1025..036` tests, plus `TestNavigationExecutionAdmission`,
  `TestNavigationExecutionClosedMapping`, `TestNavigationProviderIdentity` and
  `TestNavigationReceiptCannotInventSuccess`.
- `cmd/corvint`: `TestAFUV1025NavigationMapGolden`, `TestAFUV1026LocatorsOnlyFromIntent`,
  `TestAFUV1027EffectClasses`, `TestAFUV1028PacketRequiresGrant`,
  `TestAFUV1028ScriptedAgentCompletesGoals`, `TestAFUV1030DocsRenderGoldenAndByteStable`,
  `TestAFUV1032DocsCheckDrift`, `TestAFUV1032WaiverExpiry`, `TestAFUV1032WaiverKeepsUnrenderedClaim`,
  `TestAFUV1033AnchoredMarkdown`, `TestAFUV1033NonRegularMarkdownSkipped`,
  `TestAFUV1034FlowsToolsMatchCLIVerbs` and `TestAFUV1039AcceptanceFixture`.
- `cmd/corvint-mcp`: `TestAFUV1034FlowsToolProfile` and `TestAFUV1035FlowsTextStaysInsideEnvelope`.
- `internal/mcp/bridge`: three `TestAFUV1034Flows*` tests.
- `conformance/mcp-2026-07-28`: the whole package (35 PASS, 3 official-schema SKIP), including
  `TestAFUV1034FlowsCompiledConformance` on both protocol versions.
  `TestAFUV1034FlowsOfficialSchema` was skipped because `CORVINT_MCP_OFFICIAL_SCHEMA` is unset and no
  local copy of the pinned schema exists.
- `tools/web-flows`: `node --test test/navigation.test.mjs` (4/4, NEX-V0-003 policy).
- `internal/testrunner`: `TestHistoricalPlanByteIdentity`, `TestSelectionAdmissionIsClosed`,
  `TestSelectionMatchesFreshSessionsAndKeepsNativeIdentity`, `TestSelectionRefusesInexactMatches` and
  `TestSelectionIsAdditiveToHistoricalBytes`.
- `internal/testrunner/dynamic`: `TestNightwatchSelectionAcrossFreshSessions` and
  `TestStorybookSkipTagsFixedArgv`.
- `cmd/corvint-test-runner`: `TestNightwatchSelectionPreAdmittedAcrossFreshSessions` and
  `TestHistoricalPlanAndReceiptBytesSurviveSelectionContract`.
- Doc-reading packages selected by `corvint affected`: `internal/specindex`, `internal/flowdocs` and
  `internal/flowcoverage`.

Retained-evidence integrity: all 35 artifacts listed in `evidence/issues-167-175/manifest.json`
match their recorded SHA-256 at the base.

## V1-0252 (S5 navigation map): READY_TO_CLOSE, pending the orchestrator's full gate

- AC "a deterministic scripted agent completes each fixture goal from the packet alone": met by
  `TestAFUV1028ScriptedAgentCompletesGoals`.
- AC "AFU-V1-025..029 navigation goldens, effect raising from observed traffic, requires-grant
  marking and the observer refusal": met.
  - Goldens: `TestAFUV1025NavigationMapGolden`.
  - Effect raising: `TestAFUV1027EffectClasses` and `TestAFUV1027EffectRaisedNeverLowered`.
  - Requires-grant: `TestAFUV1028PacketRequiresGrant`.
  - Observer refusal: `TestAFUV1029ObserverRefusesNonDisposableOrigin`, plus the integrated execution
    gate above.
- The 2026-09-28 browser FAIL at `84fa719af` was superseded by the approved `d1aec4ebc` repair
  (2026-09-29 PASS). At the main merge, `script/web-flows-gate` passed again (2026-10-08 build log).
  Merges: #201 `81c4b8515`, #675 `c6574c7a7`.
- Not rerun here: the browser gate needs Playwright 1.63.0's bundled Chromium revision 1243. That
  browser is absent from the local Playwright cache (1223/1228/1234/1248), and no browser was
  downloaded.
- Remaining: `make gate` (full-gate) is orchestrator-owned.

## V1-0253 (S6 proven documentation): READY_TO_CLOSE, pending the orchestrator's full gate

- AC "AFU-V1-030..033: docs goldens, drift failure on a lost PROVEN, waiver expiry and the anchored
  Markdown case": met. Docs goldens: `TestAFUV1030DocsRenderGoldenAndByteStable`. Drift failure:
  `TestAFUV1032DocsCheckDrift`. Waiver expiry: `TestAFUV1032WaiverExpiry` and
  `TestAFUV1032WaiverExpiryBoundary`. Anchored Markdown: `TestAFUV1033AnchoredMarkdown` and
  `TestAFUV1033AnchorParsing`.
- AC "replace through a confined temporary file and rename (AFU-V1-036)": met by
  `TestAFUV1036DocsReplaceConfined` and `TestAFUV1036DocsGitPathRefused`.
- Merge: #202 `484d03176`. Full gate is orchestrator-owned.

## V1-0254 (MCP `--tool-profile flows`): READY_TO_CLOSE, pending the orchestrator's full gate

- AC "MCP conformance passes with and without the selector, and the default profile bytes are
  unchanged": met.
  - `TestAFUV1034FlowsToolProfile` covers the golden default and task-review `tools/list` bytes
    without the selector, and refused selectors.
  - With the selector: `TestAFUV1034FlowsCompiledConformance` passes on 2026-07-28 and 2025-11-25.
  - Without the selector: the whole `conformance/mcp-2026-07-28` package was rerun after review. 35
    tests pass, including the default-profile black-box traffic cases (`startServer` with no
    `--tool-profile`). The three official-schema tests skip because no local schema copy exists.
  - `TestAFUV1035FlowsTextStaysInsideEnvelope` passes.
- The official-schema run is retained, not rerun. `evidence/issues-167-175/README.md` records that
  the pinned schema (SHA-256 `ef70b61f...0203`) was supplied, and `flows-conformance-repaired.log`
  has a hash-bound `ok`.
- Merge: #205 `26d211e73`. Full gate is orchestrator-owned.

## V1-0255 (S8 acceptance fixture and live qualification): READY_TO_CLOSE, pending the orchestrator's full gate

- AC "AFU-V1-039: the committed acceptance fixture passes": met by `TestAFUV1039AcceptanceFixture`
  over `cmd/corvint/testdata/flows/acceptance`.
- AC "retained live qualification records exist for the companion and the Core profile": met by
  hash-verified records in `evidence/issues-167-175/`, observed 2026-09-28.
  - Companion on Beamfall, one UI flow (pricing checkout) and one API flow (phase read):
    `beamfall-playwright.json`, `beamfall-companion-surfaces.json`, `beamfall-mcp.jsonl` and
    `live-cleanup.json`.
  - Core profile: the AFU-V1-040 corpus report and `change-qualifications.json`. The latter holds the
    Corvint fixture counter fault (1 pass before, 1 fail after) and the Beamfall phase-ID fault. Both
    use a full-suite fallback with 0 omitted.
- Limits retained from the spec: the Beamfall backend is the in-process mock (production NOT_OBSERVED),
  no reduction is claimed for the live selections, and hosted CI is NOT_OBSERVED.
- Merge: #206 `af2fe826d`. Full gate is orchestrator-owned.

## V1-0612 (Storybook skip tags): OPEN_RESIDUAL

- AC1 (a fixed, pinned tag-selection mechanism with no injected argv): implemented.
  - TRE-V0-044, `dynamic/build.go` `storybookSkipTags`; `TestStorybookSkipTagsFixedArgv` passes.
    Merge: #803 `dec710840`.
  - TRE-V0-044 is still `proposed`. Acceptance under the owner's standing auto-accept is an
    orchestrator act after review, and this lane does not record it.
- AC2 (real Storybook 10.6.1 / test-runner 0.26.0 / Jest 30.5.2 run through the common executor
  with pass, deliberate failure and tagged skip): NOT_RETAINED.
  - No installed tree exists on this host. The `storybook` and `@storybook/test-runner` packages are
    absent from `/private/tmp/cem10-build/dynamic/storybook-npm/node_modules`, and
    `storybook-compat-next/runtime` holds only empty `fixture` and `home` directories.
  - The tarballs exist in the npm cache, but a browser tuple for the runner would need a download.
  - Missing run: one common-executor `corvint-test-runner run` with the `storybook-test-runner`
    profile and `Target=skip-test`. It must produce PASSED, FAILED and SKIPPED with expected exit 1,
    and preserve the input hashes of the original optional-config startup failure.
- AC3 (native report identity and inventory, no-tests/setup failure, ordinary and browser-descendant
  cleanup, independent review, immutable change evidence): NOT_RETAINED. It needs the same live run.

## V1-0620 (Nightwatch session-bearing IDs): OPEN_RESIDUAL

- AC1 (a closed, typed, versioned selection contract that keeps historical bytes): met.
  - TRE-V0-021, accepted by decision 0454.
  - Tests: `TestSelectionIsAdditiveToHistoricalBytes`,
    `TestHistoricalPlanAndReceiptBytesSurviveSelectionContract` and `TestHistoricalPlanByteIdentity`.
- AC2 (pre-admit the selection before launch; fresh differing sessions match while native identity
  is kept): met with synthetic sessions.
  - TRE-V0-022/023.
  - Tests: `TestNightwatchSelectionPreAdmittedAcrossFreshSessions`,
    `TestSelectionMatchesFreshSessionsAndKeepsNativeIdentity` and
    `TestNightwatchSelectionAcrossFreshSessions`.
- AC3 (missing, duplicate, extra, aliased and wrong selections incomplete): met by
  `TestSelectionRefusesInexactMatches` and `TestSelectionAdmissionIsClosed`.
- Merges: #675 `f9963ed5a`, `844866611` and `ae70ed261`.
- AC4 (actual pinned real-browser pass/fail/skip with changed sessions): NOT_RETAINED.
  - No Nightwatch 3.16.0 or ChromeDriver install exists on this host:
    `/private/tmp/cem10-build/dynamic/browser-npm/node_modules` has no `nightwatch` package, and
    `chromedriver` is not on `PATH`.
  - `stable-next/nightwatch-selection-qualification/{session-a,session-b,wrong-expectation}` hold only
    dangling links and browser profiles, with no receipt.
  - Host Chrome is 155.0.8059.80, not the pinned 154.0.8037.59.
  - Missing run: two `corvint-test-runner run` invocations of one pre-admitted `expectedSelection`
    against the pinned Nightwatch, ChromeDriver and Chrome tuple. They must observe two different real
    session IDs, a complete PASSED/FAILED/SKIPPED observation each, and a `wrong-expectation` run that
    stays incomplete.

## Failure modes and limits

- The full gate is NOT_RUN by charter. The orchestrator runs `make gate` for V1-0252..0255.
- No browser, schema or npm package was downloaded. Real-toolchain runs are reported as retained
  evidence or NOT_RETAINED, never as passes.
- Corvint `affected` selected 63 units for this doc-only change. Only the doc-reading
  `internal/specindex`, `internal/flowdocs` and `internal/flowcoverage` were run. Its language-frontier
  unknowns are retained.

## Review

One Codex read-only round returned CONCERNS with four findings, all confirmed and fixed:

- The digest credited S5 to the issue-175 evidence. It now cites the 2026-10-08 qualification.
- `:346` was described as a pre-dispatch gate. It is a receipt recheck, and request admission is
  in `navigation.mjs`.
- V1-0254's without-selector conformance was not shown. The default-profile conformance package
  was rerun.
- The flows verdicts did not show the outstanding full gate. Each now names it.

## Rollback

Revert this commit. That restores the earlier "partial" AFU-V1-029 wording and removes this entry.
No runtime, wire or requirement text changed.
