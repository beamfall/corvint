# Tasks: e2e-inventory-discovery

Build in this order, after changes `canonical-test-id` and `test-inventory` (owner decision R38
(2026-11-09) build order). Each task names its requirement IDs and its proof.

- [ ] 1 Reserve `AFU-V1` 146..149; mark 140..145 retired; set `requirementRefs`. Proof: `ticket
   show` read back; REQUIREMENTS.tsv rows.
- [ ] 2 First commit: add the four definition lines, mark the six old lines retired with their
   meanings, regenerate the index. Requirements: AFU-V1-146..149 and the retired 140..145. Proof:
   spec diff.
- [ ] 3 Read Playwright entries from the committed inventory keyed by canonical test ID; the plan
   read starts no child process. Requirements: AFU-V1-146. Proof: no-child-process test.
- [ ] 4 Rename `unlinked` to `unassigned` and count it. Requirements: AFU-V1-147. Proof: no-link
      fixture.
- [ ] 5 Absent, whole-stale, per-file and malformed paths. Requirements: AFU-V1-148. Proof: one
   fixture per path.
- [ ] 6 Change `ReconcileProvider` to union semantics. Requirements: AFU-V1-149. Proof:
   provider-omits fixture.
- [ ] 7 Docs (`docs/FLOW-COVERAGE.md`, `docs/CAPABILITIES.md`, help) naming `corvint inventory
   refresh`; build-log entry; Tasks gate. Proof: docs diff, `receipt audit`.
