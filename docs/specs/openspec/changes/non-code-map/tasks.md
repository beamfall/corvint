# Tasks: `non-code-map`

Build tasks in order. Each names its requirement IDs and its proof.

- [x] 1 File the ticket in `.taskman/` (milestone `v1-0`), reserve prefix `NCM-V1` and set
      `requirementRefs` to NCM-V1-001 to NCM-V1-009. Proof: `corvint-tasks ticket show` lists the
      nine IDs. V1-5424 filed; NCM-V1 reserved.
- [x] 2 First commit: create `docs/specs/non-code-map-v1.md`, its `docs/specs/README.md` row and the
      regenerated index. Proof: `make spec-requirements` output lists the nine IDs.
- [x] 3 Map schema and strict reader (NCM-V1-002, 007, 009). Proof: reader tests for missing proof,
      unknown version and unknown member, and a repeated-run byte comparison. V1-5424:
      `internal/noncodemap` (`TestNCMV1002*`, `TestNCMV1007*`, `TestNCMV1009*`).
- [x] 4 Planner change: full by default, unowned paths and unavailable runners, never-narrowed
      paths, the `.corvint/` rule with its R31 exclusions, stale entries (NCM-V1-001, 005, 006,
      008). Proof: planner tests for each scenario, each asserting the printed reason. V1-5424:
      the AFP-V0-012 selector (`internal/gateselect`, `TestNCMV1Selector*`).
- [x] 5 Proposal output from recorded reads (NCM-V1-003). Proof: a test that stdout holds the entry
      and the map file is unchanged. V1-5424: `corvint affected --propose-data-map`
      (`TestNCMV1003*`).
- [ ] 6 Link entries to FIH-V1 non-code fault cases (NCM-V1-004). Proof: one proven and one unproven
      entry per fixture repository. Waits for fault-injection-harness V1-5405; until then every
      accepted entry is `unproven` and none narrows (V1-5424 delivers the refusal side).
- [ ] 7 Fixture repositories and the per-runner proven entries under "Acceptance evidence";
      build-log entry; complete the ticket through the Tasks gate. Proof: `corvint-tasks receipt
      audit` passes. Open: no committed fixture repository exists for any runner, and no proven
      entry can exist before task 6; the Go cases are in-test fixtures in `internal/gateselect`.

Earlier task list, kept for traceability:

- 1.1 File the ticket, reserve prefix `NCM-V1` and set `requirementRefs` to the nine IDs.
- 1.2 First commit: create `non-code-map-v1.md`, its `docs/specs/README.md` row and the
  regenerated index.
- 2.1 Map schema, strict reader and planner change.
- 2.2 Proposal output from recorded reads.
- 2.3 Link entries to FIH-V1 non-code fault cases.
- 3.1 Tests under "Acceptance evidence"; build-log entry; complete through the Tasks gate.
