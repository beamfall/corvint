# Application Navigation Ledger V0

Owner: Russell Lewis
Date: 2026-10-10
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner request V1-1132, issue #774; `AGENTS.md`;
`docs/SPEC-DRIVEN-DEVELOPMENT.md`; AMAP-V0 and RVN-V0.

## Agent digest
- Claim: A revision-evaluated edge ledger reuses qualified receipts and fails closed on unproven assumptions.
- Status: proposed, pending owner acceptance (V1-1132); experimental.
- Exists: smallest single-map CLI slice; synthetic receipts only.
- Blocked on: owner acceptance and live adopter qualification.
- Read next: Requirements; Wire and limits; Traceability; Follow-ups.

## User job and current state

The adopter needs to reuse a previously proven navigation transition without launching a browser.
At base `55aa6bb9`, `internal/appmap/verify.go` verifies individual declared step/test bindings;
`types.go` stores ordered flow steps, stable screen/selector IDs, free-text actions and flow anchors.
No existing field reliably encodes click versus redirect versus form submission. Router templates
are URL templates, not component source files. This slice makes neither missing fact up.

## Requirements

- `ANL-V0-001`: Derive one edge per distinct (from screen ID, to screen ID, trigger, selector ID)
  from consecutive resolved flow steps. Use the destination step's exact action when it is
  `goto`, `click`, `redirect` or `form submit`; otherwise trigger is `UNKNOWN`. The selector ID
  belongs to that destination step. Retain same-screen edges, sort by content-derived edge ID,
  count unresolved pairs separately. An UNKNOWN trigger describes only declared adjacency, not
  an observed interaction. (proposed, pending owner acceptance (V1-1132))
- `ANL-V0-002`: Reuse RVN verification for each common test key bound to both endpoint steps,
  including sidecar declarations. Both endpoints must verify against the same receipt and app
  revision to prove a crossing. Report PROVEN, UNVERIFIED_AT_HEAD, CONTRADICTED or UNPROVEN;
  per-key evaluated unknowns and unplaced failures block proof; a key with no supplied receipt
  does not block another key's proof. Check both router lineages and the flow
  anchor at evaluated and receipt revisions; a changed flow selector or router URL template
  demotes placed evidence. Keep learned authority and receipt/test/flow/step references.
  (proposed, pending owner acceptance (V1-1132))
- `ANL-V0-003`: Deduplicate crossings by test key; choose the proven test with fewest declared
  flow steps, breaking ties lexically. Print other proven test keys as redundant-crossers with
  a total count. Compute single-app proven/total coverage. A placed contradiction outranks a
  pass; unknown or stale evidence prevents promotion. (proposed, pending owner acceptance (V1-1132))
- `ANL-V0-004`: Accept a closed JSON sidecar with tests declaring `proves` and `assumes` edge
  IDs. Proves binds that test to both steps of each occurrence but never creates a passing
  outcome. Check the union of repeatable `--require` and all assumes against the full ledger;
  absent, unproven, contradicted and stale edges fail with exit 1 and a sorted unsatisfied list.
  Invalid inputs refuse with exit 2. (proposed, pending owner acceptance (V1-1132))
- `ANL-V0-005`: Emit deterministic `application-edge-ledger/0` JSON with evaluated revision,
  map digest, a digest of the complete ledger (digest field empty during hashing), and byte-capped
  projection with omitted counts. Trimming never changes coverage or assumption verdict. Use
  AMAP budget rules, safe bounded receipt/sidecar reads and no writes, browser or network.
  (proposed, pending owner acceptance (V1-1132))

## Wire and limits

`corvint flows appmap edges --map FILE [--receipt FILE]... [--revision REV]
[--declarations FILE] [--require EDGE_ID]... [--budget N | --full]`

Sidecar: `{"schema":"application-edge-declarations/0","tests":[{"test_key":"KEY",
"proves":["edge:HEX"],"assumes":["edge:HEX"]}]}`. At most 64 test declarations, 64 IDs per
list, 64 command requirements, 4096 adjacent pairs and 256 distinct test keys per pair.
Sidecars use the RVN regular-file reader capped at 4 MiB; receipts retain RVN's 16-file limit.
Default output cap is 6144 bytes. Counts cover complete lists even when rows are omitted.
Reused errors: `appmap-invalid-query`, `appmap-budget-too-small` and RVN receipt errors.
Ledger-internal verification reasons: `endpoint-unverified` (an endpoint lacks proof),
`crossing-evidence-mismatch` (endpoint proofs differ in receipt or revision), and
`flow-anchor-unverified` (flow freshness cannot support the receipt). These prevent PROVEN;
this slice reports the aggregate UNPROVEN state without projecting every rejected crossing.
Map/receipt digests are identity, not authenticity. Binding remains owner-labelled run evidence;
UNKNOWN trigger proof means the declared screen adjacency only. No claim of observed step ordering.

## Implementation plan and rollback

Add `internal/appmap/ledger.go`, a CLI file and focused tests; minimal dispatch/help and exit-code
hooks. Reuse the private RVN verifier and freshness cache. Run focused baseline, implementation
tests, vet and requested doc gates; independent review before handoff. Remove these additive hooks
and files to roll back; no store or map-format migration. Keep requirement IDs stable.

## Traceability

| Requirement | Implementation | Evidence |
| --- | --- | --- |
| ANL-V0-001 | `internal/appmap/ledger.go` | `TestANLV0001Identity` |
| ANL-V0-002 | `internal/appmap/ledger.go` | `TestANLV0002EvidenceAndDrift`, `TestANLV0002MixedEvidenceFailsClosed` |
| ANL-V0-003 | `internal/appmap/ledger.go` | `TestANLV0003Redundancy` |
| ANL-V0-004 | `cmd/corvint/flows_appmap_edges.go` | `TestANLV0004EdgesCLI` |
| ANL-V0-005 | `internal/appmap/ledger.go` | `TestANLV0005Budgets` |

## Follow-ups and promotion

Proposed follow-ups: typed trigger declarations and spec annotation parsing; component/template
file and page-object anchors absent from AMAP; multi-map per-app aggregation; costs beyond declared
step counts; test-plan integration; live adopter qualification. UNKNOWN trigger is not a typed
trigger acceptance claim. Promotion needs owner acceptance plus demonstrated useful reuse on an
owner-controlled suite; synthetic checks alone retain experimental status. Kill or revise the
projection if declared adjacency cannot safely support that suite. Live qualification is NOT_RUN.
