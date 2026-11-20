# Flow-link proposals: schema delivery and blocked integration

Date: 2026-11-18
Base: `a359919975da48761fd263869cc2f000dae1fb79`
Change: `docs/specs/openspec/changes/flow-link-proposals/`
State: PARTIAL; task 8 remains unticked and no repository flow acceptance was run.

## Scope and decisions

The canonical FLP-V1 spec preserves all eleven requirements and their scenarios. Its intent stays
proposed as task 1 requests; PR #235 records acceptance of the parent change under G14. Delivery
is experimental. The trace table distinguishes executable codec tests from untested integration.

Task 2 adds link records under `corvint-flows/1`, following G6's mandatory version increment.
The version 0 member list and golden are unchanged. The writer emits version 1. The existing
unsupported-version detail assertion now expects the exact supported list `0,1`; it still checks
exact equality. The golden member walker now accounts for scalar arrays, so the new evidence
array participates in the existing equality assertion. No existing assertion was removed.

Builder decisions within FLP-V1-006: the declaration's `links` array holds the specified fields;
variation IDs already identify their owning flow uniquely. Evidence items are opaque citation
strings. Confidence never grants acceptance. The acceptance predicate requires a nonblank name,
a valid UTC timestamp and a full commit object ID. It does not establish freshness or agreement.
The declaration retains its 64 KiB bound, also enforced before the writer returns bytes.

Enabling the legacy path-level selector for this new schema would claim unsupported authority.
Version 1 therefore requests the existing FULL E2E fallback with `E2E_INVENTORY_UNDERIVABLE` and
no trusted partial inventory. The existing runner reports the unenumerated E2E obligation as
incomplete; a full executed suite is not claimed. Other selections are unchanged. This is a
conservative
partial witness for FLP-V1-007, not delivery of accepted-link narrowing or reporting.

## Exact integration blockers

1. `FlowVariation` in `internal/appflows/intent.go` has no route or entry-symbol contract.
   `TestRunEvidence` and `RunAttempt` in `internal/appflows/runevidence.go`, and `Row` in
   `internal/runevidence/record.go`, expose outcomes but no observed routes. Design decisions 1,
   3, 5 and 7 require route evidence and a digest of each variation's routes. Adding those fields
   would change closed contracts outside this charter. Task 3 stops here; tasks 4 and 5 depend on
   its proposals. Resume when the owning contracts expose the required route data and bindings.
2. `line-evidence` is an explicit prerequisite in the proposal's build order. At this base there
   are no LEV-V1 rows in `docs/specs/REQUIREMENTS.tsv`, and `E2ESelection` in
   `internal/liveverify/affected/e2eplan.go` has no tier. Task 6 requires LEV-V1-009's flow tier;
   this lane cannot invent that wire field. The current join uses legacy per-file discovery and
   reviewed intent links, not the canonical per-test inventory. Tasks 6 and 7 await that integration
   and the route contract; stale, disagreement and dangling-link cases are NOT_PRODUCED.

The charter explicitly requires stopping a step whose design premise is absent. No weights,
route derivation, synthetic evidence, command stubs or replacement tier vocabulary are invented.

## Verification and self-use

The affected plan selected `internal/appflows`, `internal/command/flows` and `internal/specindex`.
It retained six unknowns: four language frontiers and two unselected binary consumers. Its summary
reported UNKNOWN; its selection verdict reported narrowed with Python/shell full-suite additions.
Those broad suites and consumers are NOT_RUN under the lane's focused-test policy.

The first full `internal/appflows` test run failed only because the golden walker omitted scalar
arrays. After repair, all existing and new tests of that package passed. The new tests are
`TestFLPV1006LinkRecordRoundTrip`, `TestFLPV1006MissingAcceptance`,
`TestFLPV1006ClosedRecord` and `TestFLPV1007PendingIntegrationNeverNarrows`.

Self-use attempt: `corvint-docs flows propose --links` at the base with this lane's worktree edits,
using the installed binary at `/home/russell/.local/bin/corvint-docs`. It exited 2 with
`{"code": "invalid-arguments", "error": "flows requires --manifest FILE", "ok": false}`.
Proposals are NOT_PRODUCED: this base has no proposal command and its input contracts are blocked.
No `flows accept` command was run. A person's acceptance and the full trace-to-tests matrix remain
outstanding, so task 8 cannot be ticked.

Raw command output stays under `/home/russell/lanes/flp-evidence/`; the handback records command
results, digests and outstanding checks. Independent review belongs to the orchestrator. Dogfood,
CEM binding and sealing were skipped under decision 0594's explicit charter override.

## Retained check results

| Check | Result |
| --- | --- |
| `script/heavy.sh go test -count=1 -timeout 30m ./internal/appflows` | PASS, 12.174 s package time |
| `script/heavy.sh go test -count=1 -timeout 30m ./internal/command/flows` | PASS, 19.491 s package time |
| `script/heavy.sh go vet ./internal/appflows` | PASS |
| `script/heavy.sh make windows-vet` | PASS, both modules; Go 1.27.4 |
| `make -s spec-requirements-check line-citations-check openspec-check long-lines-check` | PASS; 65 existing OpenSpec baseline findings |
| `make changed-checks BASE=a359919975da48761fd263869cc2f000dae1fb79` | PASS |
| `make traceability-tests-check` | PASS for retained citations; does not supply missing tests |

The command logs include the required heavy wrapper reasons. No identity overrides were exported
for tests or gates. The line-citation repinner found no unchanged citation content to move; the
check retained its existing 184 skipped basename citations. Initial failed test output is retained
separately from the successful run. These SHA-256 digests bind the recorded evidence:

| External evidence filename | SHA-256 |
| --- | --- |
| `affected.json` | `c395f115b23a6bc3654830b1a0fd7f3471005afe6b85b7779da6e3a50afc6a37` |
| `appflows-test.log` | `ec039c2b0304311100dcaecb3fb2582b13cddd0b5f3a6a51b07e855eea1a46bd` |
| `appflows-test-final.log` | `ec65d6f8c18b5ecd21c9b66fcce9b1872cf0145ab922c7b659631b7461d6d3c5` |
| `appflows-vet.log` | `9328acb2c09fa0975d7d3dde93d886da0103f71f26eecfd07ae2ebc75b784475` |
| `flows-command-test.log` | `f133547fc342d9233906205896e20a8ce2e1d36680d6855728b6896b2c964733` |
| `windows-vet.log` | `596d55eccd1147e39da0c945b4becb8e727e04718225ef260d718a395ba6dac7` |
| `self-use-proposals.stderr` | `919f67078cd9245046fd4cbd302dc0bcaf16b8263145e3b33d1b9f95ab7940c3` |
| `traceability-tests-check.log` | `50adc7f8a49919103ec91903e9aa003b00b667b5dfdd1e4aed8dab52a8fd94cc` |
