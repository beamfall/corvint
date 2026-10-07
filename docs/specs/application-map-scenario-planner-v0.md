# Application Map Scenario Planner V0 — fail-closed E2E plans from plain-language steps

Owner: Russell Lewis
Date: 2026-10-07
Intent status: accepted
Delivery status: experimental
Authoritative inputs: owner request [issue 660](https://github.com/beamfall/corvint/issues/660)
(native ticket V1-0959), `AGENTS.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
`docs/specs/application-map-v0.md` (AMAP-V0, the map this planner reads),
`docs/specs/application-flow-understanding-v1.md` (AFU-V1 flow intents) and the orchestrator notes
of 2026-10-07 on [issue 658](https://github.com/beamfall/corvint/issues/658) (V1-0957,
run-verified steps, which will supply verification statuses).

## Agent digest
- Claim: A plain-language multi-step request becomes one capped, deterministic E2E plan over application maps that fails closed on unmapped or stale steps.
- Status: accepted (decision 0446; V1-0959); experimental. AMSP-V0-001..010 are implemented in `internal/appmap/plan.go`, `corvint flows appmap plan` and the corpus MCP tool `corvint.map_plan`, over the committed AMAP-V0 fixture plus a two-app overlay; no adopter-scale qualification.
- Exists: step resolution by explicit `flow:<id>` or weighted term coverage, one browser session per app, `goto`/`stay`/`follow`/`in-screen` navigation, route-parameter handoff, a Playwright draft and the verification seam over AMAP-V0-014 overlay facts.
- Blocked on: owner acceptance. Receipt-bound verification (V1-0957, RVN-V0-006) reaches the CLI through `--receipt`/`--bind`; it covers steps only, so selectors and shown methods stay `unverified` (owner question 9), and the corpus MCP passes no receipts.
- Read next: Requirements; Verification seam; Failure modes; Owner questions.

## User and measurable job

An agent asked to write an end-to-end test for a scenario that spans several flows, often across
more than one application, has to work out which mapped flow serves each step. It then has to
reuse one browser session per application, avoid navigating again to the screen it is already
on, carry route parameters from the step that reaches them to the steps that need them, and
assert each step's declared outcome. When a step has no mapped flow, the agent must say so
instead of improvising a path. Issue 660 describes the adopter workflow this replaces. That
workflow's private product and file names are deliberately not recorded here.

Measured jobs, each over `internal/appmap/testdata/fixture` plus `internal/appmap/testdata/plan`:

1. **Plan**: the four-step request `book a tee time; check the slot status; buy a gift card;
   change the club settings` gives one deterministic `COMPLETE` plan under its byte cap. The
   plan has two sessions (one per app), `stay` navigation for step 2, and a `clubId` handoff
   from step 1 to step 4.
2. **Fail closed**: a step that matches no flow, matches several equally, or resolves to a flow
   with an unplaced step reads `UNMAPPED`. A step whose anchors changed reads `STALE`. Each
   such step lists the exploration it needs, and the plan reads `INCOMPLETE`.
3. **Authority**: no step or plan reads more than `candidate`, except `run-verified`, which
   requires every element to carry a fresh `VERIFIED@<evaluated revision>` from the overlays.
4. **Draft**: the skeleton has one `test.step` per request step. Each step names its outcome
   assertions, and it is served by the CLI and the corpus MCP.

## Verified current state

At `43c65094` (issue 657 branch): AMAP-V0 compiles one map per app and serves `screen`,
`flow`, `find` and `scaffold` projections, all for a single flow of a single map. Nothing
composes several flows, shares sessions, models navigation between consecutive flows, or hands
data from one flow to the next. The corpus MCP serves documentation-corpus tools only.

## Definitions

- **Request step**: one plain-language instruction. The steps are either given as a list or
  split from one request at `;` and line breaks, with a leading `1)`, `(2)` or `3.` enumerator
  removed.
- **Resolution**: the single map flow a request step names.
- **Session**: one browser context and page per application, reused by every step of that
  application.
- **Handoff**: one route parameter, recorded with its producing step (or setup) and the steps
  that consume it.
- **Verification status**: one of `VERIFIED@<rev>`, `UNVERIFIED_AT_HEAD`, `CONTRADICTED` or
  `unverified`, read per map element from `run-verification` overlay facts.

## Requirements

Every requirement below is (accepted by decision 0446; V1-0959).

- `AMSP-V0-001`: `map_plan` MUST accept 1..8 application maps, each naming a distinct app,
  and 1..16 request steps, each 1..512 bytes of single-line UTF-8 text with no control
  character. A whole request is split per Definitions. Anything outside these bounds MUST
  refuse with `appmap-invalid-query` before any Git read. The output is one closed
  `application-map-plan/0` document. The planner MUST read only the maps and Git, write
  nothing, and run no browser. (accepted by decision 0446; V1-0959)
- `AMSP-V0-002`: Each request step MUST resolve to at most one flow. `flow:<flow_id>` names a
  flow exactly. Otherwise the planner scores inverse-document-frequency weighted term coverage
  over each flow's ID, step actions and outcome behaviours, after lower-casing, stop-word
  removal and plural/`ing`/`ed` folding. A term that no flow uses still counts in the
  denominator. The best flow is chosen only when its coverage is strictly above 0.5 and
  strictly above the runner-up. Otherwise the step reads `UNMAPPED` with reason
  `no-matching-flow` or `ambiguous-flow` and at most 3 candidates. The result MUST NOT depend
  on the order of the maps. (accepted by decision 0446; V1-0959)
- `AMSP-V0-003`: Freshness MUST come from one Git read at the evaluated revision. That read
  covers the anchors of the resolved flows, the screen lineage of each flow step, the shown
  reuse methods, the closest asserting spec and the setup scenarios. The evaluated revision
  defaults to `HEAD`. Each action reports
  `lineage_freshness`, and each reused method reports its own freshness. When Git cannot
  evaluate the anchors, or the revision does not resolve to a commit, the result is `UNKNOWN`,
  never `FRESH`.
  (accepted by decision 0446; V1-0959)
- `AMSP-V0-004`: The plan MUST open one session per application that has at least one resolved
  step. Each session names a unique page variable and records the union of its screens'
  permissions, the map's `test_join` and one setup. The setup is the scenario file that covers
  the most of that application's steps, or `UNKNOWN no-scenario`. It also lists every flow
  precondition, including `flow:<id>` requirements, as `unverified`. A `flow:<id>`
  precondition met by an earlier MAPPED step of the same plan records that step as
  `satisfied_by_step`. (accepted by decision 0446; V1-0959)
- `AMSP-V0-005`: A step MUST read, in this order of precedence:
  - `UNMAPPED`: the step is unresolved (AMSP-V0-002), the flow has no steps
    (`no-flow-steps`), any flow step is unplaced or abstract (`unplaced-step`), or the flow
    declares no outcome (`no-declared-outcome`).
  - `STALE`: an anchor of the step reads STALE (`stale-anchors`).
  - `CONTRADICTED`: a verification status is `CONTRADICTED` (`run-contradicted`).
  - `UNKNOWN`: the step's freshness is unknown (`freshness-unknown`).
  - `MAPPED`: none of the above.

  Every step that is not MAPPED MUST carry its confidence (`none`, `stale`, `contradicted` or
  `unknown`) and at least one exploration entry. The entry names the need (`flow-intent`,
  `flow-steps`, `navigation-step`, `outcome`, `rebuild-map`, `re-explore` or `freshness`), what
  to do, and the element IDs involved. The plan reads `COMPLETE` only when every step is
  MAPPED. Otherwise it reads `INCOMPLETE` and lists each gap. The planner MUST NOT compose
  navigation or code for a step that is not MAPPED.
  (accepted by decision 0446; V1-0959)
- `AMSP-V0-006`: Navigation MUST be composed over MAPPED steps only:
  - The first action of a step is `stay` when its screen is the screen the same session's
    previous MAPPED step ended on, and `goto` otherwise.
  - A later action is `in-screen` when it stays on the previous action's screen, and `follow`
    otherwise.
  - A step that is not MAPPED resets its session's position to unknown.

  Each route parameter of a `goto` screen MUST be consumed from an existing handoff, or from a
  new `setup` handoff. Each parameter first reached by a `follow` action MUST be produced by
  that step, together with the URL template it is captured from. Handoff never crosses
  applications: a parameter is keyed by `<app>:<name>`, so the same name in two apps is two
  handoffs. (accepted by decision 0446; V1-0959)
- `AMSP-V0-007`: Verification MUST be read from the AMAP-V0-014 overlays in
  `Options.Overlays`: each overlay's `Facts` is called once per plan with the sorted unique step,
  selector and shown-method IDs, and only facts with source `run-verification`
  (`appmap.VerificationSource`, the RVN-V0-006 fact shape) about those IDs count. Such a fact's
  kind is its status: `VERIFIED`, `UNVERIFIED_AT_HEAD`, `CONTRADICTED` or `unverified`; its
  `text` is not read. No overlay, or no fact for an ID, reads
  `unverified`. A `VERIFIED` fact reads `VERIFIED@<rev>` only when its revision `<rev>` is a full
  object ID equal to the evaluated revision and the element's anchor is FRESH; otherwise it reads
  `UNVERIFIED_AT_HEAD`. Any other kind, or a `VERIFIED` fact without a full object ID, reads
  `unverified` and is reported `verification-invalid`. Several facts about one element resolve
  to the most restrictive (`CONTRADICTED`, then `unverified`, then `UNVERIFIED_AT_HEAD`, then
  `VERIFIED@`). An element ID that two supplied maps hold (as a step, selector or method,
  whether or not a planned step selects that map) cannot be attributed, so it reads
  `unverified` and is reported `verification-ambiguous`. An overlay error is reported
  `verification-unavailable` and changes nothing else. Past 4096 facts, every fact is discarded
  (a dropped one may be the contradiction), so every element reads `unverified`, and this is
  reported `verification-bound-exceeded`. A MAPPED step reads `run-verified` only when every one of its elements stands `VERIFIED@`,
  including each action's selector and shown method; otherwise it reads `candidate`. A step's
  `VERIFIED` fact MUST NOT verify its selector or methods (its `selector_evidence` label is not
  read), and the RVN-V0 producer emits step facts only, so a step with a selector or a shown
  method stays `candidate` (owner question 9). Plan `authority` is always
  `candidate`. Candidate research, overlays and setup preconditions MUST never raise any of
  these values. (accepted by decision 0446; V1-0959)
- `AMSP-V0-008`: With `draft`, the plan MUST include a Playwright skeleton that is guarded by
  `test.fixme` and has a proposed path beside the closest asserting spec. The skeleton has one
  browser context per session and exactly one `test.step` per request step. A MAPPED step's
  body contains:
  - its navigation, written as `goto`, as a `toHaveURL` guard for `stay`, or as `waitForURL`
    for `follow`;
  - a `routeParam` capture for each handoff the step produces;
  - a call to the reused page-object method only when that method is FRESH and `Callable`
    (public, no declared parameters), its class binding is unique, and some file in the map
    imports that class by name (`import { Class }`, not `import type`) with both that file and
    the class file FRESH at the evaluated revision; otherwise a TODO on the step's locator, and
    a class with no FRESH named-import evidence is printed as an `UNRESOLVED import` comment, never
    as an import;
  - one `TODO assert outcome <id>` line naming the matcher, locator and value of each declared
    outcome.

  A step that is not MAPPED throws `<STATUS> step <n>: <reason>` and prints its exploration.
  Template text MUST be escaped for JavaScript, no emitted line may contain a line terminator
  (CR, LF, U+2028 or U+2029; repository text in comments is flattened), and every route
  parameter MUST be read through
  `param("<app>:<name>")`, which throws when the parameter is unbound.
  (accepted by decision 0446; V1-0959)
- `AMSP-V0-009`: The plan MUST fit `budget` bytes (256..65536, default 16384) or `full`
  (1 MiB); the two cannot be combined. The head (schema, maps, evaluated revision, budget,
  authority, status, steps, sessions, handoff, gaps and draft) is never trimmed, and a budget
  it does not fit MUST refuse with `appmap-budget-too-small`. Only `unknowns` may be trimmed,
  with its omitted count reported. The same inputs at the same revision MUST give the same
  bytes. (accepted by decision 0446; V1-0959)
- `AMSP-V0-010`: The planner MUST be served by two surfaces:
  - `corvint flows appmap plan --map FILE... (--step TEXT... | --request TEXT) [--draft]
    [--budget N | --full] [--revision REV] [--receipt FILE]... [--bind STEP_ID=TEST_KEY]...`,
    which writes the library bytes and nothing else. `--receipt` and `--bind` are decoded
    and refused exactly as by RVN-V0-001 and RVN-V0-002, except that a binding may name a step
    of any `--map`; with them, the CLI passes one RVN-V0-006 run-verification overlay per map
    (`appmap.LoadPlanVerification`), and without them none;
  - the read-only, idempotent corpus MCP tool `corvint.map_plan`. It is listed only when
    `corvint-corpus-mcp` is started with 1..8 `--map FILE` arguments; each file must be local
    to `--root` after symlink resolution, and the maps are loaded once at startup. The tool
    takes exactly one of `steps` (1..16 non-empty strings of at most 512 bytes) or `request`
    (at most 8192 bytes), plus optional `revision` (at most 128 bytes), `budget` (an integral
    JSON number in 256..65536) and `draft` (a boolean). Arguments are decoded strictly: an
    unknown key, a `null`, a wrong type or an out-of-range value refuses. The MCP offers no
    `full`, so every response stays under the 1 MiB message cap; the CLI `--full` is the
    escape. It takes no receipts and passes no overlay, so every element it plans reads
    `unverified`. Its text result is the plan in the untrusted-data envelope, and
    `structuredContent` is the plan object.

  On both surfaces, invalid arguments refuse (CLI exit 2; MCP `Invalid params`), and a planner
  refusal keeps its code. (accepted by decision 0446; V1-0959)

## Wire contract

`application-map-plan/0` head fields, in order: `schema`, `maps[]{app, map_revision,
map_digest}`, `evaluated_revision`, `budget`, `full`, `authority` (`candidate`), `status`
(`COMPLETE`|`INCOMPLETE`), `steps[]`, `sessions[]`, `handoff[]`, `gaps[]{index, status, reason}`
and, with draft, `draft{proposed_path, imports[], lines[]}`. These are followed by the trimmable
section `unknowns` and its omission count.

A step carries `index`, `request`, `status`, `reason`, `confidence`, `app`, `flow`,
`match{basis, coverage, terms}`, `candidates[]`, `flow_anchor`, `session`, `actions[]`,
`asserts[]`, `produces[]`, `consumes[]`, `spec` and `exploration[]{need, detail, refs}`.

An action carries `step`, `action`, `screen`, `url`, `status`, `navigation`,
`lineage_freshness`, `selector{id, …, verification}`, `verification`, `methods[]{id, ref,
freshness, verification}` and `methods_total`.

The planner owns no new error code. It reuses `appmap-invalid-query`,
`appmap-budget-too-small` and `appmap-invalid-map` from AMAP-V0. An unresolvable `revision` is
not refused: it makes freshness `UNKNOWN` (AMSP-V0-003).
Unknown kind is `verification`, with reasons `verification-unavailable`,
`verification-invalid`, `verification-ambiguous` and `verification-bound-exceeded`.

## Verification seam

The planner reads verification from the issue-657 overlay seam (`Overlay.Facts`, AMAP-V0-014)
through `PlanOptions.Options.Overlays`, using only facts whose source is `run-verification`
(`appmap.VerificationSource`). The one fact contract is the RVN-V0-006 shape that the issue-658
producer (V1-0957) emits: `{element_id: STEP_ID, source: "run-verification", kind: STATUS,
revision?, text: <StepVerification JSON>, authority: "learned"}`. The planner reads the status
from `kind` and the cited application revision from `revision`; `text` stays for projection
readers. `flows appmap plan --receipt FILE --bind STEP_ID=TEST_KEY` builds the overlays with
`appmap.LoadPlanVerification(maps, receipts, binds)` and `Verification.Overlay(map, options)`,
one per map. The producer emits nothing for an unbound step and nothing for selector or method
IDs, so those read `unverified` and keep their step at `candidate`. Because RVN-V0 `VERIFIED`
cites the receipt's application revision and the planner requires that revision to equal the
evaluated one, a receipt verifies a plan only when planned at its own revision; at a later
revision it reads `UNVERIFIED_AT_HEAD` even when the step's anchors are unchanged. The corpus MCP
passes no overlay. This slice implements no ledger or run.

## Non-goals and simpler baseline

- No browser execution, test run, receipt binding or write of the draft.
- No step-level matching inside a flow: a request step is one whole flow.
- No cross-application data handoff, and no inference of outcome assertion code.
- No ranking, learning or persisted state. The MCP map set is fixed at startup.
- The simpler baseline is one `scaffold` call per flow, followed by manual merging. That
  approach opens a page per flow, re-navigates to the screen it is already on, and loses
  parameters between flows.

## Trust boundary, limits and failure modes

- Request text, maps, flow intents and overlay facts are untrusted. Request text is matched,
  never executed. Values printed in the draft are quoted or escaped, and comments are flattened
  to one line.
- Failure modes, each mapped to its outcome:
  - no matching flow, or a tie: `UNMAPPED` with candidates;
  - a flow step with no single screen: `UNMAPPED unplaced-step`;
  - no declared outcome: `UNMAPPED no-declared-outcome`;
  - changed anchors: `STALE` with `rebuild-map`;
  - Git unavailable: `UNKNOWN`;
  - a contradicting receipt: `CONTRADICTED`;
  - an overlay error or an invalid verification fact: reported, with the element read as
    `unverified`;
  - a budget too small for the head: refused;
  - a map path outside `--root`: the MCP refuses to start;
  - a reused method that is not `Callable` (takes arguments or is not public): not called,
    with a TODO;
  - a page-object class with no FRESH named import in the suite (for example a default export,
    or a map older than the import): `UNRESOLVED import` comment, not called;
  - an element ID shared by two supplied maps, even one no step selects: `unverified`, reported;
  - more than 4096 verification facts: all discarded, reported;
  - repository text containing line terminators: flattened into its comment;
  - an unverified selector on an otherwise verified step: the step stays `candidate`;
  - MCP arguments outside the schema (unknown key, `null`, wrong type, out of range):
    `Invalid params`.
- Limits: term matching is lexical, so synonyms do not match; one flow per step; a precondition
  is only listed, never checked; the setup scenario is chosen by coverage, not by content.

## Deterministic acceptance and traceability

| Requirement | Evidence |
| --- | --- |
| AMSP-V0-001 | `TestAMSPV0001RequestSplitAndInputBounds` |
| AMSP-V0-002 | `TestAMSPV0002MultiStepPlanOnFixture`, `TestAMSPV0005UnmappedStepsFailClosed` |
| AMSP-V0-003 | `TestAMSPV0005StaleAndUnknownFreshness` |
| AMSP-V0-004 | `TestAMSPV0002MultiStepPlanOnFixture` |
| AMSP-V0-005 | `TestAMSPV0005UnmappedStepsFailClosed`, `TestAMSPV0005StaleAndUnknownFreshness` |
| AMSP-V0-006 | `TestAMSPV0002MultiStepPlanOnFixture`, `TestAMSPV0005UnmappedStepsFailClosed`, `TestAMSPV0006HandoffStaysInItsApp` |
| AMSP-V0-007 | `TestAMSPV0007VerificationSeam`, `TestAMSPV0007ReceiptVerificationOverlay`, `TestAMSPV0010PlanReceiptVerificationCLI`, `TestAMSPV0007UnverifiedSelectorBlocksRunVerified`, `TestAMSPV0007CollidingElementIDsStayUnverified`, `TestAMSPV0007FactCapOverflowDoesNotPromote` |
| AMSP-V0-008 | `TestAMSPV0008DraftSkeleton`, `TestAMSPV0008URLHelpersEscape`, `TestAMSPV0008MethodWithArgumentsNotCalled`, `TestAMSPV0008DefaultExportNotNamedImport`, `TestAMSPV0008StaleImportEvidenceNotTrusted`, `TestAMSPV0008DraftLinesStayOneLine` |
| AMSP-V0-009 | `TestAMSPV0009BudgetRefusesNotTruncates`, `TestAMSPV0002MultiStepPlanOnFixture` |
| AMSP-V0-010 | `TestAMSPV0010FlowsAppmapPlanCLI`, `TestAMSPV0010PlanReceiptVerificationCLI`, `TestAMSPV0010CorpusMCPMapPlan` |

Implementation: `internal/appmap/plan.go`, `cmd/corvint/flows_appmap_plan.go`,
`cmd/corvint-corpus-mcp/mapplan.go`. Build log:
`docs/build-log/2026-10-07-application-map-scenario-planner.md`.

## Rollout, rollback and compatibility

The slice is additive and experimental. Rollback removes:
- `internal/appmap/plan.go`, `internal/appmap/plan_test.go` and
  `internal/appmap/testdata/plan`;
- `cmd/corvint/flows_appmap_plan.go` and its test;
- the `plan` dispatch line in `cmd/corvint/flows_appmap.go` and the help reference in
  `cmd/corvint/help.go`;
- `cmd/corvint-corpus-mcp/mapplan.go` and its test;
- the `--map` arguments and planner field in `cmd/corvint-corpus-mcp/main.go`;
- this spec's index rows.

No stored state needs migration. Without `--map`, the corpus MCP behaves exactly as before.

## Follow-ups (proposed tickets)

1. Wire the V1-0957 receipt overlay into the corpus MCP (the CLI is wired; the MCP needs a
   strict, root-confined receipt argument).
2. Step-level matching, so one request step can name part of a flow.
3. Generate outcome assertion code from the declared matcher, locator and value.
4. Adopter-scale qualification of resolution precision against a labelled request set.
5. Emit a reused method call only when its name is a JavaScript identifier (review round 3).
6. After a step that resolves to no app, forget every session's position, so the next step
   navigates instead of staying (review round 3).

## Owner questions

Decision 0446 keeps every V0 default below; each question stays open for a later owner answer.

1. Is flow-level resolution with a strict coverage above 0.5 the right fail-closed default,
   and is the light suffix folding acceptable? It folds `settings` to `sett` on both sides.
2. Should `stay` after a flow's last action be trusted? Today it is guarded by `toHaveURL`.
   Should a `goto` to a direct URL also skip the flow's own earlier navigation actions?
3. Should handoff ever cross applications, for example an ID created in one app and used in
   another?
4. Should `--artifact` become optional in the corpus MCP, so it can serve map-only use?
5. Should the MCP reload maps when their files change, instead of loading them once at
   startup?
6. Should every map have to come from the one repository at `--root`, as it must today?
7. Should the MCP offer a `full` mode, for example with a streamed or paged response? Today it
   has none, because a full draft could exceed the 1 MiB message cap.
8. Should overlay facts carry an app or map binding (for example an app-qualified element ID), so
   that an element ID shared by two applications can still be verified? Today such an ID reads
   `unverified`.
9. Should a step's passing receipt also verify that step's selector (RVN-V0 already labels it
   `selector_evidence: run-verified`) or its reused page-object methods? Today it does not: a
   bound test may reach the step through another locator or without the method, so they stay
   `unverified` and `run-verified` is reachable only for a step with no selector and no shown
   method. Should a VERIFIED status also carry over to a later evaluated revision when the
   step's anchors are unchanged, as the RVN-V0 projections allow?
