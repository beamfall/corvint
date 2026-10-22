# feat1: CCE prove ladder, oracle origin, head test tree, controls, fake runner and shadow report

Date: 2026-10-22. Lane: claude/feat1-cce-sk-1022e (base f29ddc0db). Tickets: CCE V1-1666, V1-1667,
V1-1679, V1-1680, V1-1682, V1-1687, V1-1733; site knowledge V1-1898..V1-1902.

## Owner authority

- On 2026-10-22 the owner said "I want the CCE stuff in 1.0" and moved the CCE and site-knowledge
  tickets into the 1.0 milestone. The lane records this as acceptance of the CCE-V0 intent: the
  spec's `Intent status` now reads `accepted (owner, 2026-10-22)`.
- CCE-V0-026..038 are accepted under the owner's standing auto-accept of lane requirements
  (2026-10-11). The numbers were checked against origin/main, which defines 001..025 and 086.
- The owner's 2026-10-18 decision ("Kill V0 compilation and pivot to the discrimination prover over
  hosted tests") had not been recorded in the spec. It is now, together with owner question 9:
  does the 2026-10-22 acceptance resume compilation, or does the kill stand with only the prover?

## Problems

- V1-1679:
  - `prove` ran base page objects at base, so a verdict could measure test drift.
  - The ladder had no `REGRESSION_DETECTED`, `SCENARIO_BROKEN` or `NEW_REACH` rungs.
  - Only the two template controls existed.
  - An empty outcome value was admitted.
- V1-1680: no scenario said whether its oracle predates the diff.
- V1-1687: every ladder verdict was covered only by live tests, which no CI lane runs.
- V1-1682: there was no shadow report.

## Decisions

- **Head test tree (CCE-V0-017, 030).** The tree is the compiled scenario, the generated
  configuration, the transitive relative imports, and the root manifest and lockfiles. It is
  overlaid on each side's application tree, so both sides report one digest.
  - An unresolved import, or a tree over 512 files, reads `test-tree-unresolved`.
  - A non-test application script that imports a tree file reads
    `test-tree-overlaps-application`.
  - Both read before any run.
- **Ladder (CCE-V0-009, 026).** The ladder follows the amended order.
  - The failure kind comes from the message with escape codes stripped: "element(s) not found"
    reads `locator-unresolved`.
  - Receipts carry no error location yet, so the receipt-location clause is PARTIAL, and
    `cleanup-failed` is not detected.
- **Oracle origin (CCE-V0-016, 027).**
  - The origin comes from a join on the base catalog.
  - Only `CHANGED_IN_DIFF` caps positive verdicts, with `oracle-self-certified`.
  - `NEW_IN_DIFF` keeps its verdict but never scores.
- **Controls (CCE-V0-018, 029).**
  - `omitted-event` applies to the expected step and to each fixture setup. Any failure or
    timeout of the perturbed run reads killed.
  - A perturbation on the assertion line refuses `e2e-hook-event-not-found`.
- **Seams (CCE-V0-028).** `prove` reaches the runner and controls through `proveEnv.side` and
  `proveEnv.controls`. A scripted fake runner drives every rung offline.
- **Shadow (CCE-V0-011, 012, 031..033).** `corvint-test e2e shadow --window FILE` reads recorded
  results and never runs either side.
  - An escape is a missed real failure.
  - The zero-escapes denominator is the bug-fix and red-ci entries the candidate ran in; zero
    fails the criterion.
  - The Holm family is every hosted test that caught a mutant.
  - Bounds are exact Clopper-Pearson.
  - The frozen gate is a compiled constant.
- **V1-1733.** The lane wrote the pivot section and CCE-V0-034..038 (NOT_STARTED). The hosted-test
  input and the actualbudget rerun are not built.

## Evidence

- `internal/changee2e` and `cmd/corvint-test` pass in full.
- The new tests:
  - `TestCCEV0028LadderOfflineFakeRunner`
  - `TestCCEV0017TestTreeHeldAtHead`
  - `TestCCEV0017TestTreeRefusals`
  - `TestCCEV0018OmittedEventControls`
  - `TestCCEV0026FailureKind`
  - `TestCCEV0016OracleOrigin`
  - `TestCCEV0024OutcomeVacuous`
  - `TestCCEV0011ShadowRegistrationRefuses`
  - `TestCCEV0011ShadowMissesAndEscapes`
  - `TestCCEV0012ShadowBoundsAndHolm`
  - `TestCCEV0011ShadowUsefulnessMinutesCost`
  - the `shadow` case in `TestCCEV0015SubcommandsReachable`
- Mutation checks each failed the expected test:
  - overlay off
  - overlap off
  - cap off
  - failure-kind split off
  - origin join
  - `UNKNOWN` catch
  - Holm
  - registration
- Statistical references: 0.95^59 is about 0.0485; P(X<=1 | 10, 0.3) = 0.149308; the Clopper-Pearson
  upper bound for 1/10 is 0.394163.

## NOT_RUN

- **Live Playwright qualification.**
  - The toolchains-2026-10-17 Node 22.23.2 install has a dangling `npx`.
  - With the lane-1270 Node and Playwright 1.64.0 bundled Chromium, every side reads
    `infrastructure:report-identity-unknown:project-location-unknown`. The lane base reads the same.
- **Tests outside the lane.** Tier 2 affected units, any adopter shadow window, and the V1-1733
  actualbudget rerun were not run.
- **CI lane (V1-1687).** Adding it needs a `.github` change under the owner's ci-control-plane.

## Skipped

- **Site knowledge V1-1898..V1-1902.** SKH-V0-009 requires the owner to confirm the first adopter
  before stage 1 starts (owner question 1). SKH-V0-014 needs a real-adopter sample. The
  2026-10-22 move into 1.0 does not answer either.
- **V1-1667.** It needs owner acceptance of the fold spec.

## Rollback

Revert this change. No stored state migrates, `cmd/corvint` is untouched, and the new
`e2e-shadow-window/0` and `e2e-shadow/0` schemas have no other reader.
