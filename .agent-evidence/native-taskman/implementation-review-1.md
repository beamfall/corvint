# Integrated implementation review 1 — REPAIR

Reviewer: native Codex, Astra/high; one independent reviewer, no nested delegation. Reviewed new `internal/taskman/{capture,decode,json,planner,process_unix,process_other}.go`, opt-in CLI wiring and existing focused tests against NTP-V0-001..008. Parent owns concurrent documentation/test-anchor updates; no core source changed during this review. No repository files edited, no full gate run. Initial review; repair cycles used: zero.

C = `/private/tmp/corvint-taskman-native-planning-20260919`; T = `/Users/russelllewis/projects/corvint-tasks`.

## F1 HIGH — Whole-repository exclusion disappears against an empty resource set

**C/internal/taskman/planner.go:129–137, :314–325.** `collide` checks WHOLE_REPOSITORY only inside the pairwise resource loops. An empty effects/resource list is permitted by the native ticket decoder. If a higher-priority selected ticket holds WHOLE_REPOSITORY and the next qualified ticket has no resources, the loop executes zero times and the second ticket is also selected. The same problem lets an empty-resource ticket run alongside a live whole-repository reservation; a qualified whole-repository ticket can likewise bypass a live empty-resource reservation. The `!qualified` fallback check covers only the current ticket, not previously selected/live whole-repository scope.

This violates NTP-V0-005 and T/docs/SPEC.md:959, :1300 (WHOLE_REPOSITORY needs zero other live entries). Reproduced with an overlay-only test: both tickets emitted SELECTED. Fix by checking whole-repository ownership at the entry/set boundary independently of whether the other resource set is empty. Regress both orientations, selected versus live, including policy-added fallback scope.

## F2 HIGH — Every happy-path inner plan carries codes outside its claimed native profile

**C/internal/taskman/planner.go:183–233, :294–295, :315–333.** `taskman-plan/0` defines `reason:Code` and `blockers:[Code|ticket:*]` (T/docs/SPEC.md:1319–1323); Code is the closed §11 set (T/docs/SPEC.md:449–450, :2526–2540; T/internal/wire/codes.go:105). The producer emits unsupported codes including SELECTED, LIVE_COLLISION, SELECTED_COLLISION, RESERVATIONS_UNKNOWN, EXECUTION_CLASS, GATE_NOT_OBSERVED, DEPENDENCY_INCOMPLETE, CAPABILITY_NOT_OBSERVED, CAPACITY_CLASS_NOT_OBSERVED and CAPACITY_EXHAUSTED. The latter is an outcome, not a Code. These enter reason and often blockers, so the promised unchanged inner profile is not actually native-compatible.

Map inner reasons/blockers to existing native codes and keep more specific fixture diagnostics outside the closed inner profile. Examples: RESOURCE_COLLISION, GATE_UNKNOWN, DEPENDENCY_UNSATISFIED, CAPABILITY_UNAVAILABLE, TICKET_STATE, MISSING_EVIDENCE. Since the upstream plan schema has no dedicated selected/success Code, explicitly document an existing code's fixture meaning (e.g. DEVELOPMENT_MODE for SELECTED) rather than silently expanding §11. A future new code requires the owning contract change. Add a table-driven assertion that every emitted/history-consumed Code belongs to the exact source-bound enum; state remains the selected/deferred/blocked discriminator.

## F3 MED — Caller observations are only partially validated as the native records they claim to be

**C/internal/taskman/decode.go:166–205, :253–281; C/internal/taskman/json.go:32–48.** Unlike exported ticket records, reservation/history records come directly from the caller and receive no executor schema validation. `attemptId` accepts any Identifier, `ticketId` accepts only a `ticket:` prefix without queue/record membership, and reservation revision is discarded after checking positivity. History admits revision `0`, arbitrary reason strings and deferredSinceSeq later than its own plan head. Identifier and Path also share a 4096-byte bound even though native Identifier is 128 bytes and Path is 512 (T/internal/wire/primitives.go:164–167, :247–255); native attempt IDs are `attempt:<a>:<q>:<32 lowercase hex>` (T/docs/SPEC.md:158). Thus malformed/contradictory caller data can be blessed as a complete native observation and affect capacity/collisions or history reset, contrary to NTP-V0-001/002/006/007 and the closed native input claim.

Overlay reproductions: an ACTIVE reservation with acceptance revision 999 for a captured revision-1 ticket, invalid attempt ID and empty resources is accepted; a history entry with revision 0, invented reason and deferredSinceSeq 999999 is accepted. The first produces BLOCKED for the named ticket but SELECTED for another ticket despite no coherent resource evidence for the named live work.

Minimum repair: validate native identity grammar and queue binding, typed bounds, known ticket identities, positive history revisions, closed Code/blocker types and valid history sequence relationships. Reject future acceptance revisions and contradictory observation bindings. For older unreleased reservations, do **not** assume revision mismatch releases them: either retain their validated old resources conservatively or refuse the complete observation. The review does not require pretending fixture generation counters came from an unimplemented live runtime; clearly distinguish synthetic fixture generation from observed native state rather than introducing a false live-generation claim. No need to implement an executor attempt oracle for this fixture slice.

## Considered and not raised

- Sorting/deduplicating resources is appropriate normalization; duplicate resources alone do not grant headroom or remove collisions, and the original observation digest remains bound. No new rejection rule is needed merely to forbid harmless duplicate input resources.
- The trusted-local executor premise and fixture-only labels are explicit. Bracketed audit/snapshot equality, source revalidation, canonical policy identity, bounded pagination, fixed argv, bounded output and subprocess group cleanup are appropriate for this scope. They do not claim hostile-editor isolation, production runtime authority or atomic admission.
- Missing runtime/gate-result facts remain blocked; the existing GATE_PASSED refusal is conservative. WQO behavior is unchanged by the opt-in dispatch.
- Native closure handling intentionally marks unsupported/unparsed/unindexed coverage incomplete; no additional transitive semantic-independence claim is present.

## Verification evidence

Scratch overlay only: `/private/tmp/corvint-taskman-evidence-20260919/review-overlay.json` replaces the test file during compilation with `/private/tmp/corvint-taskman-evidence-20260919/review-planner_test.go`; no worktree file changed. Command:

`GOTOOLCHAIN=local GOCACHE=/private/tmp/corvint-taskman-evidence-20260919/review-gocache go test -overlay=/private/tmp/corvint-taskman-evidence-20260919/review-overlay.json -count=1 -timeout 1m -run '^TestReview' -v ./internal/taskman`

Exit 0, package 0.224s. Tests intentionally pass when reproductions are observed: TestReviewWholeRepositoryAgainstEmptyEffects, TestReviewReservationSnapshotContradiction, TestReviewMalformedHistoryAccepted. Initial default-cache attempt was sandbox-denied before setup; retry used a writable scratch cache. These are failure witnesses, not passing regressions. Parent's earlier focused-suite/native fixture evidence is retained, not rerun. Required native gate and exact final Gate B remain pending after repair.

Parent follow-up resolved: IMPORT under a NATIVE writer needs no executionCutover for fixture planning. T/internal/ticket/view.go:187–189 and T/docs/SPEC.md:345–346 require NATIVE writer; T/docs/SPEC.md:1290–1291 exempts fixture queues from cutover. Current capture enforces that writer, so no additional source/writer finding. Proposed native reason mappings are valid members; TICKET_STATE matches the executor's execution-class interpretation at internal/ticket/view.go:177–178.
