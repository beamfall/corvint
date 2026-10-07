# 2026-10-07: application map scenario planner — fail-closed E2E plans (V1-0959)

## Intent

[Issue 660](https://github.com/beamfall/corvint/issues/660) (native V1-0959) asks for a planner
over the application map. It turns a plain-language multi-step request into a typed end-to-end
plan: one session per app, no redundant navigation, data handed between steps, shared setup, and
named outcome assertions. Every step without a mapped flow, or with stale anchors, is reported
instead of guessed. This slice delivers:

- the proposed contract `docs/specs/application-map-scenario-planner-v0.md` (AMSP-V0-001..010,
  each pending owner acceptance);
- `appmap.Plan` in `internal/appmap/plan.go`;
- `corvint flows appmap plan`;
- the corpus MCP tool `corvint.map_plan`;
- a two-app fixture overlay (`internal/appmap/testdata/plan`) on top of the issue-657 fixture.

The adopter workflow behind the issue is cited by issue number only. Its product and file names
are deliberately kept out of the repository.

## Decisions

- **New AMSP-V0 spec over the AMAP-V0 map.** The planner consumes the map unchanged. The only
  issue-657 file it touches is one `plan` dispatch line (plus the error text) in
  `cmd/corvint/flows_appmap.go`. The AMAP-V0 spec is not edited.
- **A request step is one whole flow.** Steps resolve by explicit `flow:<flow_id>`, or by IDF
  weighted term coverage. The coverage must be strictly above 0.5 and strictly above the
  runner-up; a tie reads `ambiguous-flow`. Before the threshold was made strict, "delete the
  club" matched `edit-club-settings` at exactly 0.5. The fixture now pins that case.
- **Fail-closed status precedence.** Status is decided in the order UNMAPPED, STALE,
  CONTRADICTED, UNKNOWN, then MAPPED. Only MAPPED steps get navigation or code. The step after
  a gap always uses `goto`, because the page state is unknown at that point. A flow with no
  declared outcome is UNMAPPED, because the plan could not name the assertion that issue 660
  requires.
- **Navigation.** The first action of a step is `stay` when the same session's previous MAPPED
  step ended on its screen; the draft guards that with `toHaveURL`. Otherwise it is `goto`.
  Within a step, actions are `in-screen` or `follow` (`waitForURL`).
- **Handoff.** A route parameter is produced by the first `follow` action that reaches it,
  captured with `routeParam` from the URL template, and otherwise bound from setup.
  `param()` throws when the parameter is unbound. Handoff stays within one app.
- **Setup.** Each session's setup is the scenario file covering most of its steps.
  Preconditions are listed as `unverified`. A `flow:<id>` precondition met by an earlier MAPPED
  step records `satisfied_by_step`. Precondition flows reach the map only as screen
  requirements, so the planner reads them from there.
- **Reuse calls are conservative.** The draft calls a page-object method only when it is FRESH,
  is a `method:` reuse entry, is `Callable` (public, no declared parameters), and has a unique
  class binding. Otherwise the step keeps a TODO on its locator. Two counting rules follow
  from this: a `path:line` spec selector is not counted as reuse, and a method that is not
  `Callable` is not called. The second rule followed the issue-657 `Callable` field (`NoArgs`
  in round 4, renamed in round 6), which was merged mid-lane.
- **No new error codes.** Refusals reuse `appmap-invalid-query`, `appmap-budget-too-small` and
  `appmap-invalid-map`. In the MCP, a failure with no error code maps to an internal RPC error,
  not to a new tool code.
- **MCP surface.** `corvint-corpus-mcp` takes up to eight optional `--map FILE` pairs after
  `--root`/`--artifact`. Each path must be local to the root after symlink resolution. The maps
  are loaded once at startup, and the tool is listed only when maps are configured. The input
  schema is closed (`additionalProperties: false`, with exactly one of `steps` or `request`),
  and the handler decodes it strictly to the same bounds. There is no MCP `full`, because a
  full draft could exceed the 1 MiB message cap; the CLI `--full` remains the escape (owner
  question 7).

## Verification seam (issue 658)

Verification is read through the issue-657 overlay seam (`Overlay.Facts`, AMAP-V0-014), as the
orchestrator directed once issue 657 closed at `6a1dab87`. Each overlay in
`PlanOptions.Options.Overlays` is asked once per plan, with the sorted step, selector and
shown-method IDs. Only facts of kind `run-verification` (`appmap.VerificationFactKind`) count.
Both surfaces pass no overlays today, so every element reads `unverified` and every MAPPED step
reads `candidate`.

A `VERIFIED` fact reads `VERIFIED@<rev>` only when its `revision` is the evaluated revision and
the anchor is FRESH; otherwise it reads `UNVERIFIED_AT_HEAD`. An invalid fact reads
`unverified` and is reported. When facts conflict, the most restrictive wins, so one
`CONTRADICTED` defeats any number of `VERIFIED`. An overlay error is reported and changes
nothing else. Plan `authority` is always `candidate`.

The earlier `Verifier` interface and its exported status constants were removed. A trial merge
with `origin/claude/gh658` at `ce09732d` showed that its `verify.go` declares
`UnverifiedAtHead`, `Contradicted` and `Unverified` in the same package, which broke the build.
The plan's constants are now unexported.

The gh658 head at that commit exposes `VerifySteps` and `StepVerification{Status, Revision}`,
but no overlay yet. To feed the planner, V1-0957 needs an `Overlay` that emits one
`run-verification` fact per step, selector and shown-method ID, with `text` set to the status
and `revision` set to its revision. It also needs to pass that overlay in
`cmd/corvint/flows_appmap_plan.go` and `cmd/corvint-corpus-mcp/mapplan.go`. Its step-level
`SelectorEvidence` covers the selector, but only if the overlay also emits a fact for the
selector ID; until then a step stays `candidate`, which is the fail-closed reading. No receipt
binding is implemented here. `TestAMSPV0007VerificationSeam` pins the contract with a fake
overlay.

## Evidence

- Focused tests (GOTOOLCHAIN=local, go1.27.1):
  - `go test -count=1 ./internal/appmap/ ./cmd/corvint-corpus-mcp/` passes.
  - `go test -count=1 -run 'AMSP|AMAP' ./cmd/corvint/` passes.
  - `gofmt -l` is clean, and `go vet` passes on all three packages.
- Fixture result: the four-step request gives a `COMPLETE` plan with two sessions,
  `stay` on step 2, a `clubId` handoff 1→4 and precondition `satisfied_by_step: 1`. The output
  is byte-identical across map order.
- Regressions:
  - `TestAMSPV0008MethodWithArgumentsNotCalled` failed before the `Callable` guard.
  - The `methods_total` and strict-threshold cases are pinned in
    `TestAMSPV0002MultiStepPlanOnFixture` and `TestAMSPV0005UnmappedStepsFailClosed`.

## Codex review

Round 1 (`codex exec`, read-only, diff `43c65094..4874a24a`) raised five P2 findings. Each was
verified, repaired, and pinned by a regression that failed before the repair:

1. **Handoff leaked across apps.** The draft kept one global `params` map, so a `clubId` from
   one app could satisfy another app's route. Parameters are now keyed `<app>:<name>` in
   `params.set`, `param()` and `urlExpr`. Pinned by `TestAMSPV0006HandoffStaysInItsApp`.
2. **An unverified selector could still give `run-verified`.** Selector verification now takes
   part in the step's all-verified check. Pinned by
   `TestAMSPV0007UnverifiedSelectorBlocksRunVerified`. The selector `id` is now printed, so
   its verification can be traced.
3. **A named import was assumed for every page-object class.** A default-export class got
   `import { Class }`. The draft now imports and calls a class only when some map file has a
   resolved, non-type named import of exactly that class. Otherwise it prints an
   `UNRESOLVED import` comment. Pinned by `TestAMSPV0008DefaultExportNotNamedImport`.
4. **MCP argument validation was weaker than the schema.** It accepted `null`s, both `steps`
   and `request`, out-of-range or fractional budgets, and a string budget. The handler now
   decodes strictly. Pinned by the invalid-argument cases in `TestAMSPV0010CorpusMCPMapPlan`;
   eleven of them were accepted by the round-1 code.
5. **An MCP `full` response could exceed 1 MiB.** `full` was removed from the MCP. A test pins
   that a 16-step, maximum-budget draft stays under `protocol.MaxMessageBytes`.

Round 2 (diff `43c65094..239f1699`) confirmed that the round-1 fixes hold. It raised four new
P2 findings, each verified, repaired, and pinned by a regression that failed on `239f1699`:

1. **Comment injection.** Session permissions and some other repository text were
   interpolated into `//` comments without flattening. A newline could close the comment and
   the guarded callback, which yields module-level code. Every emitted draft line is now
   flattened: CR, LF, U+2028 and U+2029 become spaces. This is safe because code text is always
   quoted. Pinned by `TestAMSPV0008DraftLinesStayOneLine`.
2. **Verification leaked across apps with colliding element IDs.** AMAP-V0 IDs carry no app,
   and a selector ID is derived from locator content. An ID printed by two apps now reads
   `unverified`, reported `verification-ambiguous`. Pinned by
   `TestAMSPV0007CollidingElementIDsStayUnverified`. App-qualified facts are owner question 8.
3. **The fact cap could drop a contradiction and keep `run-verified`.** On overflow every
   fact is now discarded, so nothing is promoted. A dropped `CONTRADICTED` therefore reads
   `candidate` rather than `CONTRADICTED`; that is fail-closed for promotion, but the
   contradiction is not shown. Pinned by `TestAMSPV0007FactCapOverflowDoesNotPromote`.
4. **Stale import evidence.** With a map older than a switch to `export default`, the method
   anchor stayed FRESH while the import statement was stale. Named-import evidence now
   requires the importing file and the class file to be FRESH; both anchors are added to the
   plan's freshness set. Pinned by `TestAMSPV0008StaleImportEvidenceNotTrusted`.

The same default-export assumption exists in the issue-657 `ProjectScaffold`
(`internal/appmap/scaffold.go`). It is outside this slice and is reported to the orchestrator,
not changed here.

## Dogfood friction

- `corvint --version` reported 1.0.0-rc.2. At lane start, `corvint affected --base 43c65094`
  returned `PLAN_ONLY`.
- The lane-start context packet did not surface the corpus MCP files that this slice extends.
- The session hook reported `corvint-event-rejected:dogfood-event-deadline` (FALLBACK
  degraded).

## Limits and NOT_RUN

- `make gate`: NOT_RUN (lane rule; scoped issue work runs focused tests only).
- CEM bind/seal: NOT_PRODUCED (lane rule; the orchestrator owns it).
- Adopter-scale qualification: NOT_RUN. Matching is lexical, synonyms do not match, and
  resolution precision has no labelled request set.
- Receipt-bound verification: NOT_PRODUCED (V1-0957).
