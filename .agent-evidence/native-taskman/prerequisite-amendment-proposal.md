# TCP-03 prerequisite amendment — owner decision proposed

Status: PROPOSED, not accepted. No Corvint source has changed.
Base: 6a423ac091d848b8ac5b49e8002c61c252993ac3.

## Verified conflicts

1. Corvint decision 0052 accepts ATM revision 6. Its ATM-V0-001/002 use WQO snapshots and the emitted MAXIMUM/GREEDY wave. Task-store decision 0001 accepts native priority-first planning only for corvint-tasks and explicitly leaves the Corvint amendment outstanding. The Corvint ATCP file is absent. None of the six historical source copies has been recovered by its recorded digest.
2. Task-store SPEC 3.5 and 9.4 require the baseline before any Corvint Go edit, with CONFIG_PIN and historical conformance/perf-v0 infrastructure. Corvint decision 0088 retired the paired runner; its current main.go always returns retired-python-protocol. The historical Corvint corpus d995231016b184fc00ea922c9ec8634fb300f0e7 and pre-retirement harness 9ca27f9a are unavailable in local Git. The Beamfall corpus fa3b1e7fe5bc6c10e4b09b2729f364780f567a48 is available.
3. CONFIG_PIN production, admission and live reservations are not delivered in corvint-tasks. Its CLI lists config and plan among omitted verbs. No real MAX_ADMITTED performance condition can be measured today. A concurrent full gate was observed; this host cannot establish an uncontended baseline while it runs.

## Proposed owner decision

A. Accept a separate Corvint native-queue planning extension using task-store SPEC 4.3 and 8 A1/A2/A5 as the named implementation contract for this extension. Preserve ATM/WQO behavior for foreign/shadow proposals. Do not mark the missing ATCP source recovered, reconstruct its text, or claim its unknown requirements implemented. Preserve every historical source digest and qualification hold.

B. Replace the retired harness dependency with a new native old-Go/new-Go GP protocol. Keep decision 0088's Python retirement. Repin the unavailable Corvint corpus to the immutable current base above; retain the available Beamfall corpus in isolated temporary materialization, without editing its live checkout. This corpus substitution is an explicit owner amendment, not recovery of the old bytes.

Build the replacement harness outside Corvint Go source and bind its actual source path and digest before the freeze; this grants no bootstrap Corvint Go edit before preregistration. Missing runtime permits an explicit atmBudgets:null preregistration, as SPEC 3.5 already allows, but cannot produce a passing GP result.

C. Permit freezing the complete pre-edit baseline as a content-addressed file outside source, with its exact digest recorded in the implementation decision, until the executor implements CONFIG_PIN. Require that same document to be pinned by a real CONFIG_PIN before promotion. A file or synthetic receipt must never be reported as a journal pin.

D. Preserve GP's other obligations: enumerate all current CLI commands; explicit COLD/WARM states and corpus/state identities; baseline source and executable digests; five warmups and at least 100 fresh-process samples per row; alternating old/new order; exact stdout/stderr/exit parity; wall p50/p95, CPU, RSS, allocations and I/O witnesses; one-sided 95% upper confidence bound on candidate-minus-baseline p95 <= 0 and the required CPU/RSS tests; unchanged absolute budgets; load conditions, failed/noisy rows and interruption cleanup evidence. Unsupported measurements remain NOT_RUN. An unmeasurable output or budget predicate requires a separate explicit resolution; this proposal grants no exception.

E. Permit fixture-only implementation after the replacement preregistration is actually frozen. Keep the product baseline and all real queues unchanged. Do not promote or claim no slowdown until every GP row passes, including real IDLE_INITIALIZED and MAX_ADMITTED conditions and measured executor budgets. Runtime/reservations, atomic whole-plan freshness refusal, canonical pinning and completion remain executor dependencies. Native Go gates, independent review and exact CEM/OCM completion remain required for the delivered slice.

Alternative: retain every existing prerequisite and resume only after the exact historical sources/corpus/harness are recovered, CONFIG_PIN is implemented, and the Corvint A1/A2/A5 amendment is explicitly recorded. No Corvint Go edits occur under that alternative until the specified freeze is possible.

## Why owner confirmation is required

The implementation handoff explicitly says to surface an accepted-contract conflict before choosing an interpretation and forbids silently amending accepted authority. These are changes to named governing inputs and the pre-edit pinning rule, not permission to begin ordinary implementation. Approval of this document would accept only A–E above; publication, real-queue cutover, real task execution and performance qualification are not granted.
