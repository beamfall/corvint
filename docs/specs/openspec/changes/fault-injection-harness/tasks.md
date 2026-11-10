# Tasks: fault-injection-harness

Each task names its requirement IDs and its proof.

1. File the ticket, reserve prefix `FIH-V1` and set `requirementRefs` to the fifteen IDs. Proof:
   `ticket show` read back.
2. First commit: create `fault-injection-harness-v1.md`, its `docs/specs/README.md` row and the
   regenerated index. Requirements: FIH-V1-001..015. Proof: spec diff.
3. Harness core and Go adapter from `tools/affected-edit-oracle`. Requirements: FIH-V1-003, 005,
   006, 011, 013. Proof: Go parity test and import check.
4. Pre-registration format, revert query, operator lists, over-sample and seal refusal.
   Requirements: FIH-V1-001, 002, 014. Proof: changed-seed and too-few-non-code tests.
5. Baseline, quarantine and the omitted-quarantined count. Requirements: FIH-V1-004. Proof:
   flaky-baseline and omitted-quarantined tests.
6. Cost probe, then the remaining six adapters. Requirements: FIH-V1-008, 009, 010. Proof: probe
   report; missing-adapter and pin-mismatch tests.
7. Verdict writer: four counts, G3 sentence, Wilson bound, two N for code and non-code faults,
   line-level proven label. Requirements: FIH-V1-002, 007, 014, 015. Proof: verdict fixture tests.
8. Seal pre-registrations; run the seven sets; write verdicts. Requirements: FIH-V1-006, 007.
   Proof: seven verdict files at the RC SHA.
9. Held-out sets for fastapi, solidus and payload. Requirements: FIH-V1-012. Proof: three frozen
   hashes and verdicts.
10. Build-log entry; complete the ticket through the Tasks gate. Proof: `receipt audit`.
