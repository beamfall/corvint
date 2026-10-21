# 2026-10-21 cce-a: sound CCE obligations, mutate run limits, witness edges, unrouted stubs

Lane cce-a (pre-1.0 ticket sweep). Base c9ca8beff43d8f548b054d3b3d40e3054cecf816. Tickets V1-1678,
V1-1711, V1-1483, V1-1484, V1-1485, V1-1577, V1-1491 and V1-1474. New and amended requirements are
accepted under the owner's standing auto-accept (2026-10-11): `CCE-V0-020` (amended), `CCE-V0-086`,
`FPK-V0-142` and `LPCV-V0-214`. New IDs follow the lane rule of +60 above the next free ID on
origin/main, so `FPK-V0-082..141` and `LPCV-V0-154..213` stay free.

## Problems

- **V1-1678 (P1).** `corvint-test e2e obligations` could report `NONE` unsoundly in four cases:
  - a reached screen with no flow;
  - map unknowns it ignored;
  - a truncated impact set;
  - screens, flows or outcomes removed at head, because base was never joined.
- **V1-1711.** A relative `--root` (including the default `.`) was applied twice by the map reads,
  so the command refused with `e2e-map-unavailable: evaluated revision is not a commit`.
- **V1-1484, V1-1577.** The mutate runner's per-run limit (budget/(mutants+1), at least 10 s) also
  bounded the request's first cold build into its fresh `GOCACHE`. Under host load the baseline was
  killed before any test ran, and the package tests ended without a verdict.
- **V1-1483.** A warm run killed at the per-run limit read "run ended without a verdict" and did not
  name the limit.
- **V1-1485.** When an enclosing sandbox forbids nesting, the refusal appeared only as one not-run
  per mutant run.
- **V1-1491.** The affected test helper `witnessResolves` checked a witness chain's origin and end,
  not each `Via` edge, so a forged middle hop resolved.
- **V1-1474.** `stub-never-matched` flagged a route stub that the test itself removed with
  `unroute` before any request could reach it.

## Decisions

- **`CCE-V0-020` (amended).** Obligations read `UNKNOWN`, with one reason per span from the closed
  set, in these cases:
  - a reached screen with no outcome;
  - related map unknowns;
  - any impact omission;
  - base screens, flows or outcomes removed at head;
  - fan-out past the limit, which is now `UNKNOWN` rather than a refusal.

  `NONE` requires the base `reach_exempt` globs in `.corvint/e2e-blocks.json`, and the report cites
  them.
- **`CCE-V0-086`.** A relative root is resolved to its absolute, symlink-resolved path before any Git
  or map read.
- **`FPK-V0-142`.** It has four parts:
  1. The cold baseline build is bounded only by the request context, and its duration is reported
     as `Report.Baseline`. Warm runs keep the per-run limit plus grace.
  2. A warm run killed at that limit reads `BUDGET_EXCEEDED`, with the detail "per-run limit of D
     reached after K of N mutants". On the grouped path, every claim in the group gets that result,
     with the group limit.
  3. One preflight per export runs `go version` under the sandbox launcher. A nesting refusal
     yields a single unsandboxed result with a supported limit, instead of per-run not-runs.
  4. The package tests read a budget from `CORVINT_MUTATE_TEST_BUDGET` (default 4 min) and skip on
     a budget verdict rather than failing.
- **V1-1491 (test only).** Each consecutive `Via` pair must be a selector-traversed edge: the later
  unit imports or advisory-imports the earlier one. The final hop may also be a test import or a
  built-binary exec.
- **`LPCV-V0-214`.**
  - The producer marks `"unrouted": true` on a route stub that `unroute(url[, handler])` or
    `unrouteAll` removes. It matches the way Playwright does, on the same instance only.
  - The closed decoding accepts the member only as `true`.
  - A zero-hit unrouted stub gets no finding.

  Suppression was chosen over a new finding code because an explicit removal is not a silent
  mismatch. The stub stays in the record either way.

## Evidence

All test runs were focused and run through `heavy.sh`.

- **changee2e.** `obligations_unknown_test.go` and the full package pass. Seven source mutations
  were each caught.
- **mutate.**
  - `perrun_test.go` has four `TestFPKV0142*` tests. The full package passes (136.8 s).
  - Four mutations were each caught: cold ignored, no timed-out reclassification, preflight
    disabled, and detail dropped.
- **affected.** A forged-edge case was added to
  `TestWitnessesResolveUnderTheGoStructuralRules_V1_0564_V1_0739`. It fails with the edge check
  removed, and the full package passes.
- **route stubs.**
  - The node unit case "unroute and unrouteAll mark the stubs they remove" passes.
  - `TestLPCVV0214AnnotationMarksUnrouted` and the `unrouted-before-hit` vector pass. The full
    `jstestprovider` and `testvaliditydoc` packages pass.
  - Three mutations were each caught: the projector guard, the decoder refusal and the producer
    marking.
- go vet and `gofmt -l` are clean on the touched packages.

## NOT_RUN

- A real CPU-loaded mutate run, the live Playwright `unroute` run, and the live CCE prove tests:
  `CORVINT_PLAYWRIGHT_MODULES` is not set here.
- `make gate`, which this lane's policy excludes.

## Not delivered in this lane

- **V1-1031.** Still open. Basic receipts reach `CURRENT` through `identity.testFileDigests`, but
  carry no test `id`, and an unprofiled test with an `id` is refused
  (`internal/testvaliditydoc/document.go`, `hasQualifiedMetadata`). Making `REUSED` reachable
  needs a witness-identity choice for basic receipts.
- **V1-1679.** A CCE feature, cut post-1.0.
- **V1-1687.** Its CI lane needs `.github/`, which this lane must not touch.
- **V1-1567.** Needs a native runner signal or an owner-accepted rationale.
- **V1-0837.** A large retrieval-runner feature, outside this lane's budget.

## Rollback

Revert the lane's commits. No stored state migrates. The one exception: a retained receipt that
carries `unrouted` is then refused by the closed decoding and reads `NOT_PRODUCED`, never zero hits.
