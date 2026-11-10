# Tasks: proof-repositories

Each task names its requirement IDs and its proof.

- [ ] 1 File the ticket, reserve prefix `PRR-V1` and set `requirementRefs` to the eleven IDs. Proof:
   `ticket show` read back.
- [ ] 2 First commit: create `proof-repositories-v1.md`, its `docs/specs/README.md` row and the
   regenerated index. Requirements: PRR-V1-001..011. Proof: spec diff.
- [ ] 3 Manifest format and the fetch-at-pin tool, with the Playwright flow file hash (A16) and the
   mirror URL field, filled only after the owner creates each mirror (N4). Requirements:
   PRR-V1-003. Proof: pin-mismatch, pin-unavailable and flow-file-not-frozen tests.
- [ ] 4 Measure test count, suite time, 90-day green rate (Actions API formula) and merged PRs for
   each named repository; record owner swap notes where a check fails; then commit and hash the
   rule. Requirements: PRR-V1-001, 002, 010. Proof: measured values with sources; rule hash;
   rule-hash-mismatch and held-out-swap tests.
- [ ] 5 Assemble the four family pools: check each candidate against the licence, activity and
   untouched rules (candidates include fastapi, solidus, payload, helm with its test count, and
   go-task; pool membership is NOT_DECIDED); write the four pool files with at least 5
   candidates each and their untouched searches; seal and hash them before the first candidate
   tag is pushed. Requirements: PRR-V1-005, 011. Proof: four pool hashes
   committed before the tag; pool-too-small, touched-candidate and missing-untouched-search
   refusal tests.
- [ ] 6 Run the draw once after the first candidate tag (REL-V1-002): seed is that tag's commit SHA;
   commit the draw record, then the drawn list with each repository's pin and check results. A
   drawn repository that proves unusable is reported, not replaced or redrawn (H3). Requirements:
   PRR-V1-004. Proof: draw record reproduced independently from the seed and pool hashes;
   deterministic-draw and no-redraw tests.
- [ ] 7 Build the drawn repositories' fault sets between the candidate tag and the freeze: generate
   and freeze 100 faults per drawn repository through FIH-V1, and freeze the 50-PR replay lists
   (A12). Schedule risk: none of this can start before the tag. Requirements: PRR-V1-006. Proof:
   four committed fault-set hashes.
- [ ] 8 Held-out run gate and verdict: no flows, candidate or RC tag only, blocking conditions.
   Requirements: PRR-V1-007, 008, 009. Proof: refusal-ceiling, miss-bound, rerun-comparison and
   early-run tests; four held-out verdicts at the RC SHA.
- [ ] 9 Build-log entry; complete the ticket through the Tasks gate. Proof: `receipt audit`.
