# Independent Gate A authority review — 2026-09-19

Scope: read-only review of Corvint `6a423ac091d848b8ac5b49e8002c61c252993ac3` and the task-store working tree named by the handoff. Reviewer: native Codex, task-fit Astra/high for conflicting authority and pre-edit requirements; no nested delegation, no full gates, no repository edits. Parent supplied fresh weekly usage 9%.

Path keys: **C** = `/private/tmp/corvint-taskman-native-planning-20260919`; **T** = `/Users/russelllewis/projects/corvint-tasks`.

**Verdict: REPAIR before Corvint Go edits.** Missing historical copies do not ban unambiguous experimental fixture work. The concrete pre-edit blocker is the required baseline pin; the independent authority conflict concerns promoting the native planning profile as accepted Corvint behavior.

## HIGH — Do not turn a digest-pinned draft into the required CONFIG_PIN

T/docs/SPEC.md:1268–1281 defines `taskman-perf-baseline/0` as frozen before any Corvint Go edit and **pinned by CONFIG_PIN**. T/docs/SPEC.md:2464–2469 repeats the before-first-edit order. T/docs/reviews/2026-09-19-corvint-implementation-handoff.md:51–54 explicitly preserves it. A JSON file, Git commit, digest manifest, or synthetic audit-test receipt does not establish that the required pin operation occurred.

The observed executor CLI implements ticket mutation, pause/unpause, init, reconciliation and receipt audit, but not CONFIG_PIN/reconfigure: T/internal/cli/cli.go:80–150. The only CONFIG_PIN occurrences in implementation inspection were the accepted-operation enum and synthetic journal tests; T/internal/journal/audit_test.go:681–690 deliberately inserts a future-profile pin and expects UNKNOWN semantic coverage. The low-level publication path can store pinned files (T/internal/store/store.go:94–95), but that does not supply a callable validated CONFIG_PIN operation.

Required repair: either deliver and verify the missing fixture CONFIG_PIN path in its owning executor task first, or obtain an explicit owner-recorded, narrowly scoped pre-edit pin mechanism/exception. Do not fabricate the receipt or silently equate a file freeze with CONFIG_PIN. Before proposing the exception, prepare the complete measurable baseline draft and exact missing dependencies so the owner can review a concrete decision. This review does not authorize changing the task-store checkout.

## HIGH — Native priority-first is a real accepted-contract divergence

C/docs/decisions/0052-owner-accepts-first-wave-proposals-and-authorises-build-2026-09-04.md:18–20 accepts ATM revision 6. C/docs/plans/AGENT-TASK-MANAGER-2026-09-04.md:86 and :105–110 require every queue's read side to use WQO snapshots. Its ATM-V0-002 at :189–206 requires the WQO emitted wave, retains MAXIMUM/GREEDY/NONE, and forbids recomputing/substituting candidates. ATM-V0-002a at :210–221 drops disappeared members and tolerates checkpoint drift rather than whole-plan refusal.

T/docs/SPEC.md:1317–1336 instead defines separate queue/policy/head/intent/reservation identities and irreversible priority-first selection order within a preview. T/docs/SPEC.md:2313–2317 explicitly names A1/A2/A5 as amendments, not compatible restatements. T/docs/decisions/0001-task-control-plane.md:61–65 accepts those only for the task-store repository and leaves the Corvint amendment outstanding; T/docs/SPEC.md:2509–2511 repeats that dependency. No relevant Corvint amendment was found in the current decision/registry search.

The handoff's :46–60 and :69–72 supplies specific authority to implement a **read-only fixture prototype** of this new behavior, while forbidding silent accepted-authority amendments. Thus do not label the prototype as satisfying the old ATM-V0-001/002/002a, do not alter WQO, and do not claim that decision 0052 already authorizes native priority-first admission.

Required owner record before accepted-profile promotion: cite the current source revision/digests; adopt native A1/A2/A5 explicitly for Corvint (stable queueId, native canonical read contract, priority-first profile, whole-plan freshness); preserve old WQO semantics for observations and foreign waves; retain executor-only mutations/admission; resolve source-provenance gaps openly; update the registry with the resulting scope. Do not bulk-accept A3/A4/A6/A7 without reviewing their separate execution/completion/configuration risks. An experimental spec may record the handoff's limited implementation authority now; it must not impersonate this owner acceptance.

## MED — Missing frozen originals remain unresolved, not silently replaced

T/docs/decisions/0001-task-control-plane.md:24–46 says ATCP and the review were uncommitted, and all six originals are identified by temporary path plus exact SHA-256. The named historical base is not proof those uncommitted documents existed in Git. Existing filesystem evidence T/docs/reviews/2026-09-19-writer-evidence/source-recovery-search.json reports 206 candidates and no matches; T/docs/reviews/2026-09-19-writer-repair.md:70–73 records its scope and limits.

Additional bounded recovery in this review examined all reachable Git blobs at the six exact source paths: eight blobs, zero digest matches; historical base 0a227a94df6ad02e35c01965d05c46be9b367905 is unavailable. Full evidence is `/private/tmp/corvint-taskman-evidence-20260919/authority-history-recovery.json`. This is not a claim that no backup or unreachable object exists anywhere. Do not repeat broad filesystem searches absent a new location lead. Current ATM/WQO documents remain current repository authority but are not recovered frozen copies.

The missing originals block assertions about exact ATCP conformance and unresolved amendments/qualification. They do not independently block the explicitly requested, unambiguous local fixtures (handoff :47–50).

## MED — Runtime absence blocks GP promotion, not automatically baseline preregistration

T/docs/SPEC.md:1278–1281 explicitly permits `atmBudgets:null` before measurement and calls it NOT_RUN. Therefore absent runtime/MAX_ADMITTED measurements alone do **not** make baseline document preparation impossible. Preserve all STOPPED/IDLE_INITIALIZED/MAX_ADMITTED rows and declare unavailable observations NOT_RUN; do not substitute zero workers for MAX_ADMITTED. Actual GP passing still requires these measurements and exact old-Go/new-Go comparisons (T/docs/SPEC.md:2475–2490). This distinction avoids conflating the pre-edit pin blocker with the later performance promotion hold.

## Attainable local scope and execution order

1. Now: freeze factual source/authority evidence, propose the explicit experimental spec and acceptance fixtures, enumerate actual performance rows and required controls, prepare a baseline draft with every missing field/condition visible. Do not call it a completed freeze if CONFIG_PIN or required identity fields are missing.
2. After a valid pre-edit freeze or explicit recorded alternative: build only a fixture-gated, read-only native adapter/planner with mutationAuthority:false; prove priority-first versus maximum-cardinality counterexample, deterministic ties, canonical queue/policy/acceptance/head/intent/reservation binding, barriers/dependencies, reservation collisions, incomplete coverage refusal or conservative serial fallback, and no store/repository mutation. Test-owned reservation data is labelled fixture observation, never live runtime support.
3. Keep executor stale-plan validation, atomic admission, runtime, reservations, completion and CONFIG_PIN ownership in corvint-tasks. No real queue, foreign write, dispatch, qualification or performance claim follows from planner tests.
4. Preserve exact CEM/OCM profile investigation and normal Corvint review/gates for any eventual source change. Freeze source and commit before gates; do not invoke full gates now to certify an unbuilt profile or an unresolved baseline.

No LOW findings. No evidence justifies broadening the implementation into executor/runtime work merely to avoid recording these boundaries.

## Review of the parent's proposed resolution

Reviewed `/private/tmp/corvint-taskman-evidence-20260919/prerequisite-amendment-proposal.md` (lines 1–28). **PASS as a proposed owner decision; not an accepted implementation gate.** A explicitly addresses the A1/A2/A5 conflict without asserting recovery. B explicitly requests new corpus/harness authority instead of calling it recovered infrastructure. C openly moves CONFIG_PIN from the pre-edit boundary to the promotion boundary and requires the identical document to be pinned later. D retains the actual performance predicates and refuses invented measurements. E confines implementation to fixtures and retains runtime/GP promotion dependencies. These changes resolve the identified authority blockers only after owner approval; the actual complete preregistration must then be frozen before dependent source edits.

Two execution clarifications: (1) prepare/measure the replacement harness outside Corvint Go source first, so implementing the prerequisite harness does not itself violate the before-any-Go-edit order; freeze its exact path/version/digest with the baseline. (2) Proposal line 24's recovery alternative still needs the outstanding explicit Corvint A1/A2/A5 amendment record; recovering originals and implementing CONFIG_PIN alone would not automatically amend decision 0052. Neither clarification adds a new owner permission requirement beyond the concrete proposal already needed.

The parent's harness retirement/corpus availability/current concurrent gate findings were supplied as independent evidence from its owned investigation, not re-tested here. This review independently established the accepted authority conflict, source-provenance failure, and CONFIG_PIN ordering requirement.
