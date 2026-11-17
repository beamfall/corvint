# Snapshot dispatch and flows reader repair

Owner repair charter: r1cleanup, base df315e35056d15c7933b5ff97c605cedf5ef4919;
repair parent 37a979397d2ccace8f533f37be5bf9c01169af4b. Scope is the two supplied
findings, V1-3496 and V1-4818. V1-4805 is unchanged.

## Snapshot dispatch (ASD-V0-2201, ASD-V0-2420)

Clear CORVINT_TASKS_SNAPSHOT only inside the delivery-screen subprocess; retain
--unaudited on snapshot hosts. Queue verification and ticket reads still use the
verified snapshot copy. The snapshot fixture now admits only its implemented
queue status and ticket show reads, refusing all other verbs before dispatch.
This closes the fixture's permissive fallthrough class rather than special-casing
another forbidden verb. Regressions exercise delivery-screen with --unaudited,
a future unknown verb, successful snapshot-host lane.env emission and pinned
unaudited arguments. Delivery verdict admission remains the closed set CLEAR,
MENTIONED, NOT_OBSERVED; FUTURE_VERDICT proves its fail-closed default.
The fixture's source-bound record-reader declaration was refreshed; it still
reads only scratch build logs, not repository records.

## Explicit flows reader (LGP-V0-8600, NGI-V0-980)

The flows CLI profile supplies swiftimpact.Reader through
NewFlowsWithSwiftPMReader. NewFlows preserves its existing nil-reader behavior.
TestSwiftImpactThroughProfileConstructors verifies Sources/App/main.swift as a
reverse import of Sources/Core/Ledger.swift using the explicit reader carried by
both registry constructors, and compares native impact receipts.

The requested direct MCP Swift result is PARTIAL under the frozen contract.
The first experimental regression disproved the review's premise: both profiles
return invalid-arguments before reading Swift, because MCPImpactPathAdmitted and
the advertised schema admit only go, rb, js, jsx, mjs, cjs, ts, tsx. The frozen
schema and existing admission assertions were not changed. Native positional
impact has wider language support. TestProfileImpactRetainsClosedLanguageSet
covers Swift and an unknown .future suffix through both actual CLI constructors.
It does not claim a successful MCP Swift call. Such a call needs separate owner
authorization to widen the schema/admission contract. The failed experimental
run is retained as repair-test-cli.log; it is not passing evidence.

## Verification and limits

The installed corvint affected command refused self-binary-stale. A checkout
build through script/heavy.sh ran affected with the charter's full base before
tests: UNKNOWN, 391 selected, 298 excluded, 55 unknowns and 14 unresolved
packages. Both touched Go packages and the shell suite were selected. The full
fallback selection was not executed, per the owner's focused-only instruction.
All Go tests used script/heavy.sh, GOTOOLCHAIN=local and the mandated private
TMPDIR, without exported commit identity or a heavy lock-dir override.

- LANE_SCRIPTS_GROUPS=tools bash script/lane-scripts_test.sh: PASS, 15 groups.
- Whole internal/mcp/bridge: PASS, 5.030s, new constructor regression included.
- Whole internal/mcp/cli: FAIL, 6.098s, only the existing
  TestMCPV08300UnavailableLaunchHandshake deleted_spare/existing_spare failures.
  The new closed-admission regression passes. The artificial read-only ancestor
  /home/russell/lanes/.git causes the already recorded launch-root fixture issue.
- make spec-requirements: PASS, no generated index changes.
- check-line-citations.sh --repin: one moved citation, automatic second pass
  found no further move; the quoted content and hash were unchanged.
- make -s spec-requirements-check line-citations-check: PASS.
- make changed-checks BASE=df315e35056d15c7933b5ff97c605cedf5ef4919: PASS,
  including the refreshed shell reader source binding.

Independent review, whole-repository gates, Windows vet, Tasks writes and all
publication remain assigned to the host or forbidden by this repair charter.
CEM bind/check cannot succeed: docs/DOGFOOD.md records their retirement under
decision 0594; no replacement workflow is introduced. No seal is performed.
Earlier ticket-level external gaps remain open. Rollback: revert this repair's
wiring, fixtures, spec clarifications and source binding together; do not widen
MCP admission or restore snapshot fixture fallthrough as a workaround.

Raw evidence remains in /home/russell/lanes/r1cleanup-evidence/:

| File | Bytes | SHA-256 |
|---|---:|---|
| repair-affected.json | 326826 | a54dc67bbd1e8b5ee516c3bfd760b5530e00f17287b0432019e4db2dac1edd51 |
| repair-test-cli.log | 19307 | ad9ecbb0e829005f995d2d56b75d5ad902a32ea6398a77d196ad0d7cb38c948c |
| repair-test-cli-final.log | 18700 | b0c8dd45ae2a683134e455387ad0d3e531e6a3636f9b02f589390b9fb2528c16 |
| repair-test-bridge-final.log | 11028 | ffc73e6a22dc9bacacf32cb8ed7321c05149dadd7382196fb70c2f376a4ab338 |
| repair-lane-tools.log | 36 | 37f5fbf2225257afc642c3f0dcb03cfdb90087187232cd82495583d76920e990 |
| repair-spec-citations.log | 1003 | 92542890245c5408e2297e3999a031173084771a7b4fddbad1130c9aff921a84 |
