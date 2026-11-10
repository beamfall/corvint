# Tasks: affected-plan-kinds

Build in this order. Each task names its requirement IDs and its proof.

1. Keep the `APK-V0` reservation; set the ticket's `requirementRefs` to APK-V0-001..004, 006..010.
   Proof: `corvint-tasks ticket show V1-5251` read back.
2. First commit: spec text for APK-V0-001..010 (005 marked retired) and the AFU-V1-1651/1655
   rewrites, each line beginning `(proposed 2026-11-09; V1-5251; delivery experimental)`.
   Requirements: all. Proof: spec diff; `make spec-requirements` row check.
3. Merge the two readers into one `.corvint/flows.json` reader in `internal/appflows`.
   Requirements: APK-V0-001, AFU-V1-1651, AFU-V1-1655. Proof: both-files and no-declaration fixture
   tests.
4. Move graph-dependent join code to the tests layer; drop the baselined edges it removes.
   Requirements: APK-V0-006. Proof: import ratchet with the touched baseline entries removed.
5. Add `kindSource` and the G13 signals; make `unit` the default. Requirements: APK-V0-002. Proof:
   undeclared and conflict fixture tests.
6. Add the `tier` member (`line | file | flow | full` plus `exact`, owner decision R16
   (2026-11-09)); flow-selected tests carry `flow`. Requirements: APK-V0-003, APK-V0-008. Proof:
   receipt fixture asserting tier per selection.
7. Rename `unlinked` to `unassigned`; implement the link-record removal basis keyed by the
   canonical test ID (after change `canonical-test-id` lands). Requirements: APK-V0-004. Proof:
   stale-link and line-evidence fixture tests.
8. Select every accepted linked test of any kind or runner for a reached or edited flow.
   Requirements: APK-V0-008. Proof: the three-runner fixture and the flows-edit fixture.
9. UNKNOWN plus all E2E for failed flow or inventory evidence, using `inventory-absent` and
   `inventory-stale` from change `test-inventory`. Requirements: APK-V0-009. Proof: one fixture per
   reason.
10. Reach definition and `reach-unknown`. Requirements: APK-V0-010. Proof: transitive, unresolved
    and RSpec system fixtures.
11. Agreement with `corvint flows impact` after change `flows-status-report` makes it native.
    Requirements: APK-V0-007. Proof: same-inputs comparison test.
12. Update `docs/CAPABILITIES.md` and command help; build-log entry; Tasks gate. Proof: docs diff,
    `receipt audit`.
