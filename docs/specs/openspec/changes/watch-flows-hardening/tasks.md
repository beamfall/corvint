# Tasks: watch-flows-hardening

Build alongside or after changes `watch-seven-runners`, `affected-plan-kinds` and
`flow-link-proposals`. Each task names its requirement IDs and its proof.

- [ ] 1 Reserve `WFF-V1` 001..008; file the ticket; set `requirementRefs`. Proof: `ticket show` read
   back.
- [ ] 2 First commit: create `watch-flows-hardening-v1.md`; README entry; index. Requirements: all
   WFF-V1. Proof: spec diff, REQUIREMENTS.tsv rows.
- [ ] 3 Watch planning errors become widened `plan` events; exit 2 only on source, index or output
   failure. Requirements: WFF-V1-001. Proof: broken-then-fixed `_test.go` watch fixture test.
- [ ] 4 Keyed warm state with named inputs; doubt drops the item. Requirements: WFF-V1-002. Proof:
   HEAD-move cache test equal to a cold plan.
- [ ] 5 One variation-membership function used by the join and by `flows impact`; member-without-
   variation widening. Requirements: WFF-V1-003, 004. Proof: join and impact fixture tests.
- [ ] 6 `link_not_variation` report and the unassigned run for step and outcome test links.
   Requirements: WFF-V1-005. Proof: fixture test.
- [ ] 7 `e2e.unanchored` report, cut at 50 with an exact count. Requirements: WFF-V1-006. Proof:
   fixture test, including widening to every E2E test with reason `change-unanchored` (R42 A1).
- [ ] 8 Deprecated reader selects every statically reached consumer. Requirements: WFF-V1-007.
   Proof: two-flow fixture test.
- [ ] 9 Snapshot plans in repositories with flows carry `E2E_SNAPSHOT_UNJOINED`. Requirements:
   WFF-V1-008. Proof: guidance review fixture test.
- [ ] 10 Independent review; focused tests from `corvint affected`; dogfood bind and seal. Proof:
    CEM and review report.
