# Tasks: `tasks-hardening`

## Tasks

Build tasks in order, in the `beamfall/corvint-tasks` repository. Each names its requirement IDs and
its proof.

1. File the ticket in the corvint-tasks `.taskman/` store, link it from a `v1-0` ticket in this
   repository, reserve prefix `TKF-V1` (001 to 012) and set `requirementRefs`. Proof:
   `corvint-tasks ticket show`.
2. First commit: create `docs/specs/tasks-hardening-v1.md` in corvint-tasks with the requirements of
   `specs/tasks-hardening/spec.md`. Proof: the spec index lists TKF-V1-001 to 012.
3. Expired leases in reads: `choose`, `RecordedClaimability`, the ticket-view attempt oracle and
   `queue status` (TKF-V1-001, 002). Proof: the expired-lease scenarios and the read-only replay on a
   copy of the Corvint store with unchanged file hashes.
4. Lease extension on submit, gate run and heartbeat; help text (TKF-V1-003, 004). Proof: the
   near-expiry, after-expiry and heartbeat-only scenarios; existing fencing tests still pass.
5. `finish` verb (TKF-V1-005). Proof: the gate-failure and rerun scenarios with receipts and a clean
   `receipt audit`.
6. Completion health share and states (TKF-V1-006). Proof: the 89% scenario and a run on the Corvint
   store copy.
7. `next` read and `roadmap` default (TKF-V1-007, 008). Proof: the scenarios and unchanged store
   files.
8. Counter names and stale progress (TKF-V1-009, 010). Proof: the scenarios.
9. `NOT_FOUND` code and advisory/list compaction (TKF-V1-011, 012). Proof: the scenarios.
10. Hand `finish`, `next`, the renamed counters and `NOT_FOUND` to change `tasks-conformance` for the
    supported-command list and schema v1 before it freezes. Proof: the list and schema name them.
11. Build-log entry in corvint-tasks; complete the ticket with `finish` through the Tasks gate.
    Proof: `corvint-tasks receipt audit` passes and the completion kind is VERIFIED.
