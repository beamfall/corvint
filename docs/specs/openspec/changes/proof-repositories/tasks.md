# Tasks: proof-repositories

Each task names its requirement IDs and its proof.

1. File the ticket, reserve prefix `PRR-V1` and set `requirementRefs` to the eleven IDs. Proof:
   `ticket show` read back.
2. First commit: create `proof-repositories-v1.md`, its `docs/specs/README.md` row and the
   regenerated index. Requirements: PRR-V1-001..011. Proof: spec diff.
3. Manifest format and the fetch-at-pin tool. Requirements: PRR-V1-003. Proof: pin-mismatch and
   pin-unavailable tests.
4. Measure test count, suite time, 90-day green rate (Actions API formula) and merged PRs for each
   named repository; record owner swap notes where a check fails; then commit and hash the rule.
   Requirements: PRR-V1-001, 002, 010. Proof: measured values with sources; rule hash;
   rule-hash-mismatch and held-out-swap tests.
5. Check fastapi, solidus and payload; write the three pool files; freeze the held-out list.
   Requirements: PRR-V1-004, 005, 011. Proof: frozen list and pool hashes; freeze-refusal,
   touched-repository and short-pool tests.
6. Generate and freeze 100 faults per held-out repository through FIH-V1. Requirements: PRR-V1-006.
   Proof: three committed fault-set hashes.
7. Held-out run gate and verdict: no flows, RC only, blocking conditions. Requirements:
   PRR-V1-007, 008, 009. Proof: refusal-ceiling, miss-bound, rerun-comparison and early-run tests;
   three held-out verdicts at the RC SHA.
8. Build-log entry; complete the ticket through the Tasks gate. Proof: `receipt audit`.
