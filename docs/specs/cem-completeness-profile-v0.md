# CEM Completeness Profile V0

Owner: Russell Lewis
Date: 2026-10-28
Intent status: experimental under the standing lane auto-accept (2026-10-28; V1-2947)
Delivery status: experimental
Authoritative inputs: V1-2947 owner request (astra audit section 8), agent-drafted; V1-2962 /
GitHub #1948 for `CCP-V0-010`; `cem-stable-v1.md` (`CEM-V1-017`, `CEM-V1-034`, `CEM-V1-145`,
`CEM-V1-310`, `CEM-V1-633..635` and "Omission soundness limits"); `protocol/cem-completeness-1/`;
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-25-cem-omission-soundness.md`; AGENTS.md invariants 2, 3 and 8.

## Agent digest
- Claim: A separate `cem-completeness/1` profile refuses omitted runs, narrowed selections and missing receipts against an independent run inventory.
- Status: experimental under the standing lane auto-accept (2026-10-28; V1-2947)/experimental
- Exists: `internal/cem/verify/completeness.go` (`verify.Completeness`), `interop/cem01-go/completeness.go`, `protocol/cem-completeness-1/` (Apache-2.0 packet, 15 vectors), `stableCounts` in `internal/cem/workflow/stable_read.go`.
- Blocked on: no CLI verb or gate wiring; independent-adopter interoperability NOT_OBSERVED; V1-2962 items 1 (predicate counts and ceilings) and 3 (`not-instrumented` map state) need new wire versions and an owner decision.
- Read next: Requirements; Non-goals; Traceability.

## User and baseline

A consumer that grants runner input from a CEM wants to know the map cites every run that
happened. The stable reader proves reference integrity only: every cited receipt, plan and
artifact exists and matches its digest. A self-consistent map can leave out a failed retry, cite a
plan narrower than the required selection, or leave a receipt out, and still ACCEPT
(`CEM-V1-633`, "Omission soundness limits"). The baseline is that stated limit: the gate reports
`runnerSelectionCompleteness` and `sourceInventoryCompleteness` `NOT_OBSERVED` (`CEM-V1-634`).
Success is a verifier that, given an inventory from someone other than the map's producer,
refuses each of those omissions by name, and never passes without one.

## Requirements

- `CCP-V0-001`: (standing lane auto-accept, 2026-10-28; V1-2947) Completeness MUST be a separate
  verifier profile, `cem-completeness/1`, versioned apart from the stable reference-integrity
  verifier. It MUST NOT call, change or reinterpret `verify.Stable`, the portable stable reader,
  the stable result, the gate or `CEM-V1-633..635`. Its result reports `referenceIntegrity:
  NOT_EVALUATED`; a caller pairs a completeness PASS with a stable ACCEPT over the same `mapSha256`.
- `CCP-V0-002`: (standing lane auto-accept, 2026-10-28; V1-2947) The verifier MUST take the map
  bytes, an artifact root, an optional run inventory, a caller-asserted map producer and a target
  revision. The inventory supplies the required-test selection, every run including retries with
  its terminal outcomes, and the source identities (base and target revisions, each run's input
  file digests).
- `CCP-V0-003`: (standing lane auto-accept, 2026-10-28; V1-2947) The inventory, profile
  `cem-run-inventory/1`, MUST be one strict JSON object: valid UTF-8, no duplicate or unknown
  member, no trailing data, at most 4 MiB and 4096 required tests plus runs. `producer` is
  non-empty; revisions are 40 or 64 lowercase hex; `requiredTests` and `runs` are present arrays;
  required tests are unique; each run has `attempt >= 1`, a receipt digest and a
  `(runner, project, attempt)` key unique in the inventory, an `inputFiles` object and at least one
  outcome with a unique id and a state from the closed set `PASSED`, `FAILED`, `SKIPPED`, `FLAKY`,
  `TIMED_OUT`, `INTERRUPTED`, `UNKNOWN`, `NOT_REPORTED`. Any violation refuses with
  `inventory-invalid` and nothing else is judged.
- `CCP-V0-004`: (standing lane auto-accept, 2026-10-28; V1-2947) The inventory MUST NOT state its
  own independence. The verifier derives it and carries the provenance (inventory digest, producer,
  map producer, independence, `authentication: NOT_OBSERVED`): `SELF_AUTHORED` when the producer
  equals the map producer, `NOT_OBSERVED` (reason `inventory-independence-not-observed` in `notObserved`) when no map
  producer was supplied or the inventory is
  invalid, `INDEPENDENT` otherwise. A self-authored inventory adds `inventory-self-authored` to
  `notObserved`, so it can refute a map but never confirm one.
- `CCP-V0-005`: (standing lane auto-accept, 2026-10-28; V1-2947) The runner adapter MUST read a
  `runnerReceipts` row as a runner receipt only when its label is
  `corvint-test-runner-receipt/0` (`CEM-V1-310`) and its plan and receipt decode with the runner
  plan and receipt profiles and agree on the runner. An opaque row is `receipt-opaque`, an
  unreadable labelled row `receipt-not-readable`, and a readable receipt with `complete: false`
  `receipt-incomplete`, each in `notObserved`.
- `CCP-V0-006`: (standing lane auto-accept, 2026-10-28; V1-2947) With a valid inventory the
  verifier MUST refuse, with one explicit `missing` entry each: a non-passing inventory outcome
  whose run the map does not cite (`failed-run-omitted`, naming test, attempt and terminal
  outcome); an uncited all-passing run, or a required test a cited plan selects but no cited
  receipt records (`receipt-missing`); a required test no readable cited plan selects
  (`selection-narrowed`); a revision or input-file digest that differs
  (`source-identity-mismatch`); a run whose runner or project differs (`run-identity-mismatch`);
  a differing, missing or extra outcome (`terminal-outcome-mismatch`); a cited run the inventory
  does not list (`run-not-in-inventory`); and an invalid map or unreadable artifact
  (`map-invalid`, `artifact-unavailable`). Narrowing that cannot be decided because a row is opaque
  or unreadable is `selection-not-observed` in `notObserved`.
- `CCP-V0-007`: (standing lane auto-accept, 2026-10-28; V1-2947) The outcome MUST be `REFUSE` when
  `missing` is non-empty, else `NOT_OBSERVED` when `notObserved` is non-empty, else `PASS`. Without
  an inventory the result is `NOT_OBSERVED` with `inventory-not-supplied` and never PASS; without a
  target it is at best `NOT_OBSERVED` (`target-not-supplied`). A map that cites a failed run
  completely can PASS: completeness judges what the map omits, not whether the runs passed.
- `CCP-V0-008`: (standing lane auto-accept, 2026-10-28; V1-2947) The result, profile
  `cem-completeness-result/1`, MUST be closed and deterministic: entries carry only `reason`,
  `runner`, `project`, `test`, `receiptSha256`, `attempt`, `terminalOutcome` and `source`, omit
  empty members, and sort by reason, runner, project, test, receiptSha256, source,
  terminalOutcome (byte order), then attempt (numeric). `missing` and `notObserved` are always
  arrays.
- `CCP-V0-009`: (standing lane auto-accept, 2026-10-28; V1-2947) `protocol/cem-completeness-1/`
  MUST publish, under Apache-2.0, the profile description and vectors covering at least an
  omitted failed retry, a narrowed selection, a missing receipt, a self-authored inventory, no
  inventory and a matching complete inventory. The native verifier and the portable reader
  `interop/cem01-go` MUST both run every vector and agree exactly on outcome, independence and
  entries.
- `CCP-V0-010`: (standing lane auto-accept, 2026-10-28; V1-2962, GitHub #1948) `cem status` and
  `cem report` (JSON and Markdown) over a `cem/1.0` map the stable parser admits MUST always emit
  the per-disposition `counts` (`total`, `supported`, `unknown`, `mechanical`). Without ceilings
  the read adds `counts` alone: no `policyIssues`, no `policyState`, `ok` and the exit stay the
  verifier's, and `stableVerification` is unmodified. With ceilings `CEM-V1-145` and `CEM-V1-637`
  apply unchanged. `cem verify`, the MCP report preview and the `review` projection add nothing.
  This amends `CEM-V1-017` and `CEM-V1-034`.

## Non-goals

- No CLI verb, MCP tool or gate wiring for the profile; the gate keeps `CEM-V1-634`.
- No authentication of the map producer, the inventory producer or the target: they are
  caller-asserted and reported `NOT_OBSERVED`.
- No change to either stable reader, the stable result, the cem/1.0 map or a sealed CEM.
- Independent-adopter interoperability is NOT_OBSERVED.
- V1-2962 item 1 (the `cem/v2` predicate carries per-disposition counts and ceilings) is BLOCKED:
  `CEM-V1-065` closes the predicate to `{baseRevision, patchSha256, target}` and existing
  verifiers reject unknown members, and `CEM-V1-638` says it records no ceilings. It needs a new
  predicate version and an owner decision.
- V1-2962 item 3 (a distinct `not-instrumented` map coverage state) is BLOCKED: `TCQ-V0-052`
  freezes the state vocabulary to `covered` and `uncovered`, and `TCQ-V0-091` requires a new map
  profile version. `cem cover` keeps reporting `notInstrumented` in its result.

## Failure modes

The inventory is believed as given: an inventory that omits the same run as the map agrees with it,
and independence is only the inequality of two caller-asserted names. Matching is by receipt
digest and `(runner, project)`, so a producer that re-labels a runner or project refuses rather
than matches. An opaque or unreadable cited row hides its selection, so narrowing becomes
`selection-not-observed`, never PASS. Counts on a status or report are the map's own dispositions;
they are not a verdict, and a map the parser refuses gets none.

## Acceptance evidence

`TestCCPV0009NativeCompletenessVectors` and `TestCCPV0009PortableCompletenessVectors` run all 15
vectors through both implementations with identical expected results.
`TestCCPV0006RequiredTestSelectedButUnrecordedRefusesReceiptMissing` covers a selected but
unrecorded test and an incomplete receipt; `TestCCPV0003InventoryRefusals` covers malformed,
unknown-member, duplicate-key, wrong-profile, producer-less, null-array and trailing inventories.
`TestCCPV0010StatusAndReportAlwaysEmitCounts`, the stable read and report envelope tests and the
`cem status` core-freeze golden cover `CCP-V0-010`. Independent-adopter runs: NOT_OBSERVED.

## Rollout and rollback

Additive. The profile is a library and packet nobody calls by default; rollback deletes
`internal/cem/verify/completeness*.go`, `interop/cem01-go/completeness*.go`,
`protocol/cem-completeness-1/`, its `.corvint/test-read-scopes.json` row and this spec. Rolling
back `CCP-V0-010` removes `stableCounts`, restores the `CEM-V1-017`/`CEM-V1-034` text, the test key
sets and the `cem-status` core-freeze golden; a consumer that started reading `counts` without
ceilings then reads none again, as before.

## Traceability

| Requirement | Code | Evidence | Status |
|---|---|---|---|
| CCP-V0-001 | `internal/cem/verify/completeness.go` | `TestCCPV0009NativeCompletenessVectors` | experimental |
| CCP-V0-002 | `internal/cem/verify/completeness.go`, `interop/cem01-go/completeness.go` | `TestCCPV0009NativeCompletenessVectors`, `TestCCPV0009PortableCompletenessVectors` | experimental |
| CCP-V0-003 | `internal/cem/verify/completeness.go`, `interop/cem01-go/completeness.go` | `TestCCPV0003InventoryRefusals`, `TestCCPV0009PortableCompletenessVectors` | experimental |
| CCP-V0-004 | `internal/cem/verify/completeness.go`, `interop/cem01-go/completeness.go` | `TestCCPV0009NativeCompletenessVectors`, `TestCCPV0009PortableCompletenessVectors` | experimental |
| CCP-V0-005 | `internal/cem/verify/completeness.go`, `interop/cem01-go/completeness.go` | `TestCCPV0006RequiredTestSelectedButUnrecordedRefusesReceiptMissing`, `TestCCPV0009NativeCompletenessVectors` | experimental |
| CCP-V0-006 | `internal/cem/verify/completeness.go`, `interop/cem01-go/completeness.go` | `TestCCPV0006RequiredTestSelectedButUnrecordedRefusesReceiptMissing`, `TestCCPV0009NativeCompletenessVectors`, `TestCCPV0009PortableCompletenessVectors` | experimental |
| CCP-V0-007 | `internal/cem/verify/completeness.go`, `interop/cem01-go/completeness.go` | `TestCCPV0009NativeCompletenessVectors`, `TestCCPV0009PortableCompletenessVectors` | experimental |
| CCP-V0-008 | `internal/cem/verify/completeness.go`, `interop/cem01-go/completeness.go` | `TestCCPV0009NativeCompletenessVectors`, `TestCCPV0009PortableCompletenessVectors` | experimental |
| CCP-V0-009 | `protocol/cem-completeness-1/` | `TestCCPV0009NativeCompletenessVectors`, `TestCCPV0009PortableCompletenessVectors` | experimental |
| CCP-V0-010 | `internal/cem/workflow/stable_read.go` | `TestCCPV0010StatusAndReportAlwaysEmitCounts`, `TestCoreStableReportJSONEmbedsVerifierResult`, `TestCoreVerbsEmitTheFrozenProfiles` | experimental |
