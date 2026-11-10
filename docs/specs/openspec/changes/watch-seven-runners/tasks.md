# Tasks: watch-seven-runners

Build after changes `runner-contract`, `canonical-test-id` and `test-inventory` (owner decision R38
(2026-11-09) build order). Each task names its requirement IDs and its proof.

- [ ] 1 Reserve `WCH-V1` 001..014; file the ticket; set `requirementRefs`. Proof: `ticket show` read
   back.
- [ ] 2 First commit: create `watch-v1.md`; mark the amended LPCV lines; README entry; index.
   Requirements: all WCH-V1, LPCV-V0-1086, LPCV-V0-1087. Proof: spec diff, REQUIREMENTS.tsv rows.
- [ ] 3 Multi-runner watch loop on the runner contract, naming instances and canonical IDs;
   list-by-default; tiers. Requirements: WCH-V1-001, 002, 003, 008. Proof: per-runner fixture tests.
- [ ] 4 Inventory read and the absent and stale answers. Requirements: WCH-V1-014. Proof: fixture
      tests.
- [ ] 5 Kill switch through the NMS-V1-008 parser. Requirements: WCH-V1-009. Proof: global,
   per-runner and unknown-value tests.
- [ ] 6 Diff basis, debounce, ignore rules and the inotify limit. Requirements: WCH-V1-006, 010,
      011,
   012. Proof: fixture tests named in each requirement.
- [ ] 7 Save-to-answer harness; new budgets with enforced targets. Requirements: WCH-V1-004, 013.
   Proof: harness source and a retained report.
- [ ] 8 Snapshot measurement and decision. Requirements: WCH-V1-007. Proof: decision record.
- [ ] 9 Reference-machine runs (Linux and macOS); docs; build-log; Tasks gate. Requirements:
      WCH-V1-004,
   005. Proof: retained reports, `receipt audit`.
