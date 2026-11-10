# Tasks: `tasks-integration`

## Tasks

Build tasks in order. Each names its requirement IDs and its proof.

1. File the ticket in `.taskman/` (milestone `v1-0`), reserve prefix `TIN-V1` (001 to 014, with 005
   retired) and set `requirementRefs` to the thirteen live IDs. Proof: `corvint-tasks ticket show`.
2. First commit: create `docs/specs/tasks-integration-v1.md`, its `docs/specs/README.md` row and the
   regenerated index. Proof: `make spec-requirements` lists the IDs.
3. With corvint-tasks: agree `taskman-ticket/1`, `taskman-receipt-ref/1` and the supported-command
   list; vendor them pinned by SHA-256; add the fixture check (TIN-V1-004, 013). Proof: the fixture
   and unlisted-command scenarios.
4. Receipt ID and the I1 gate example policy using `receipt put`, carried by change
   `tasks-policy-v15` (TIN-V1-006, 012). Proof: the audit scenarios with Corvint uninstalled.
5. I2 resolver (TIN-V1-007). Proof: one resolved and one refused citation.
6. Gate records the review report as advisory, after change `review-evidence-check` fixes its JSON
   (TIN-V1-008). Proof: the advisory-finding scenario.
7. RC signing check reading the record of change `tasks-conformance`, version pair and tag order
   (TIN-V1-001 to 003). Proof: the refusal scenarios.
8. Initialize the `beamfall/core` trial store by pull request: seed reviewed by 2026-12-01, dry
   run by 2026-12-04, pull request on 2026-12-08, merged by the owner (TIN-V1-014; C23). Proof:
   the merged pull request and `receipt audit` in that store. Then the 21-day trial on Corvint
   and `beamfall/core`, 2026-12-08 to 2026-12-29 (TIN-V1-009; C22). Proof: the trial record with
   the ID and hash diff.
9. Spec deletion after corvint-tasks PR #4 merges (TIN-V1-010, 011). Proof: the cited merge.
10. Build-log entry; complete the ticket through the Tasks gate. Proof: `corvint-tasks receipt audit`
    passes.

Earlier task list, kept for traceability:


- [ ] 1.1 File the ticket, reserve prefix `TIN-V1` (001 to 012, with 005 retired) and set
      `requirementRefs` to the eleven live IDs.
- [ ] 1.2 First commit: create `tasks-integration-v1.md`, its `docs/specs/README.md` row and the
      regenerated index.
- [ ] 2.1 Agree ticket schema v1 and receipt reference v1 with corvint-tasks; add the fixture.
- [ ] 2.2 Receipt ID (SHA-256 of file bytes) and I1 gate example policy using `receipt put`.
- [ ] 2.3 I2 resolver; review JSON output for the gate.
- [ ] 2.4 RC signing check and trial record.
- [ ] 3.1 Tests under "Acceptance evidence"; build-log entry; complete through the Tasks gate.
