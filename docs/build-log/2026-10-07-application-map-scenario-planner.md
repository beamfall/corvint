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
  is a `method:` reuse entry, declares no parameters, and has a unique class binding.
  Otherwise the step keeps a TODO on its locator. Two counting rules follow from this: a
  `path:line` spec selector is not counted as reuse, and a method that takes arguments is not
  called. The second rule followed the issue-657 round-4 `NoArgs` field, merged mid-lane.
- **No new error codes.** Refusals reuse `appmap-invalid-query`, `appmap-budget-too-small` and
  `appmap-invalid-map`. In the MCP, a failure with no error code maps to an internal RPC error,
  not to a new tool code.
- **MCP surface.** `corvint-corpus-mcp` takes up to eight optional `--map FILE` pairs after
  `--root`/`--artifact`. Each path must be local to the root after symlink resolution. The maps
  are loaded once at startup, and the tool is listed only when maps are configured. The input
  schema is closed (`additionalProperties: false`, with exactly one of `steps` or `request`).

## Verification seam (issue 658)

`appmap.Verifier.Status(ctx, elementIDs)` is the narrow input, called once per plan with the
sorted step, selector and shown-method IDs. `PlanOptions.Verifier` is nil on both surfaces
today, so every element reads `unverified` and every MAPPED step reads `candidate`.

`VERIFIED@<rev>` counts only for the evaluated revision and a FRESH anchor; anything else is
`UNVERIFIED_AT_HEAD`. An invalid value reads `unverified` and is reported. A verifier error is
reported and changes nothing else. Plan `authority` is always `candidate`.

V1-0957 is expected to supply a receipt-bound `Verifier` keyed by AMAP-V0 element IDs, and to
wire it into `cmd/corvint/flows_appmap_plan.go` and `cmd/corvint-corpus-mcp/mapplan.go`. No
receipt binding is implemented here. `TestAMSPV0007VerificationSeam` pins the contract with a
fake verifier.

## Evidence

- Focused tests (GOTOOLCHAIN=local, go1.27.1):
  - `go test -count=1 ./internal/appmap/ ./cmd/corvint-corpus-mcp/` passes.
  - `go test -count=1 -run 'AMSP|AMAP' ./cmd/corvint/` passes.
  - `gofmt -l` is clean, and `go vet` passes on all three packages.
- Fixture result: the four-step request gives a `COMPLETE` plan with two sessions,
  `stay` on step 2, a `clubId` handoff 1→4 and precondition `satisfied_by_step: 1`. The output
  is byte-identical across map order.
- Regressions:
  - `TestAMSPV0008MethodWithArgumentsNotCalled` failed before the `NoArgs` guard.
  - The `methods_total` and strict-threshold cases are pinned in
    `TestAMSPV0002MultiStepPlanOnFixture` and `TestAMSPV0005UnmappedStepsFailClosed`.

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
