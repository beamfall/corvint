# Spec verdicts: every file under `docs/specs/` (change `spec-set-cleanup`)

Status: proposed (written 2026-11-09; only the owner can accept it). Requirements: SSC-V1-002,
SSC-V1-003, SSC-V1-006, SSC-V1-007.

One row per file tracked directly under `docs/specs/` in the prodspecs tree (288 files;
`openspec/` change folders are not specs and are not listed).

## Method

- SPEC-HEALTH class: the class row in `docs/specs/SPEC-HEALTH-2026-11-09.md` (KEEP 71, MERGE 19,
  RETIRE 19, OUT-OF-1.0 59). "retired" means the spec has no SPEC-HEALTH row because its header
  already reads `Delivery status: retired`.
- Batch: the removal batch of the `source-removal` change (B0 to B11) in which the row is applied,
  so that a spec is archived in the same change that removes its code (SSC-V1-006). Cleanup-only
  batches, which remove no code:
  - C1: workflow documents, the Health line on every spec, and specs whose code is already gone;
  - C2: MERGE folds into the named target spec, then the source is archived;
  - C3: SPEC-HEALTH itself is archived and the reference check becomes blocking.
  `-` means no batch changes the row.
- Code in integ: the spec's `implementation` paths from `INDEX.json` that exist in integ 9ec3a29d0
  (present/declared), and the number of files outside `docs/` in integ 9ec3a29d0 that contain the
  spec's requirement prefix followed by `-` ("hits"). A prefix shared by several specs (ASD-V0,
  AFP-V0, DCW-V0, TCP-V0) gives the same hit count to each of them; the count is a pointer, not
  proof of use.
- Class: one of `active`, `archive in place`, `move to beamfall/corvint-tasks`, `delete`, or
  `undetermined` with what is missing.
  - `archive in place` keeps the file at its path with the archive marking of SSC-V1-002.
  - `move to beamfall/corvint-tasks` applies to the 37 specs whose retirement note says decision
    0594 moved them to that repository: the 32 `corvint-tasks-*` files plus
    `native-taskman-planning-v0`, `scope-lease-v0`, `tasks-criterion-binding-v0`,
    `tasks-mcp-read-profile-v0` and `work-queue-observation-v0`. The live copy is already there; this
    repository deletes its copy in B10, after corvint-tasks PR #4 merges (RMV-V1-027).
  - `delete` is not used for any other file: RMV-V1-026 never moves or deletes a spec file, and
    git history keeps every deleted copy (SSC-V1-008).
- Where SPEC-HEALTH and a removal batch disagree, the row follows the owner answer behind the batch
  (G24, G26) and says so; where no owner answer settles it, the row is `undetermined`.

## Counts

- active: 82
- archive in place: 168
- move to beamfall/corvint-tasks: 37
- delete: 0
- undetermined: 1 (`test-attack-v0.md`, decided with the fault-injection proposal)

The counts changed on 2026-11-09 under R42, E8 and D11: eleven rows left `undetermined` and
`greptile-gate-v0.md` moved from archive in place to active.

## Table

| File | Prefix | SPEC-HEALTH | Class | Batch | Code in integ 9ec3a29d0 | Reason |
|---|---|---|---|---|---|---|
| `FIX-REGISTRY.tsv` | - | n/a (not a spec) | active | - | no paths | Diagnostic fix registry read by diagnostic-repair-contract-v0 (KEEP). |
| `FRONTIER-DECISION-BRIEF-2026-08-29.md` | - | retired | archive in place | C1 | no paths | Already retired; add the Health line only. |
| `INDEX.json` | - | n/a (not a spec) | active | C1 | no paths | Generated index; SSC-V1-005 changes it to list active specs only (format change is open item 1). |
| `INTENT-STATUS-FROZEN.tsv` | - | n/a (not a spec) | active | - | no paths | Read by script/gen-spec-requirements.sh (integ 9ec3a29d0). |
| `PRODUCT-PLAN-1.0.md` | PROD-DOCS | none | active | - | no paths; 0 hits | The 1.0 product plan; every active spec serves one of its PROD IDs. |
| `README.md` | - | n/a (not a spec) | active | C1 | no paths | Generated table plus entry rules; table lists active specs, archived specs move to an archived section. |
| `REQUIREMENTS.tsv` | - | n/a (not a spec) | active | - | no paths | Generated; keeps archived IDs as history rows and never reuses them (SSC-V1-003). |
| `SPEC-HEALTH-2026-11-09.md` | - | none | archive in place | C3 | no paths | Input to this cleanup; archived once every row here is applied. |
| `additive-narrowing-v0.md` | ADN-V0 | KEEP | active | - | no paths; 23 hits | SPEC-HEALTH KEEP; serves PROD-TEST-001, PROD-TEST-002. |
| `adoption-setup-report-v0.md` | ADS-V0 | OUT-OF-1.0 | active | - | no paths; 6 hits | `corvint adopt --setup`. Decided 2026-11-09 under R42, E8: `adopt` is a 1.0 first-run verb and gets a first-run product requirement in the plan (wording is a deferred plan edit). Low confidence: what `main.go` still writes was not traced. |
| `affected-cli-consistency-v0.md` | ACX-V0 | KEEP | active | - | no paths; 41 hits | SPEC-HEALTH KEEP; serves PROD-TEST-001. |
| `affected-edit-oracle-v0.md` | AEO-V0 | KEEP | active | - | 1/1 paths; 2 hits | SPEC-HEALTH KEEP; serves PROD-TEST-002. |
| `affected-plan-python-playwright-v0.md` | AFP-V0 | KEEP | active | - | no paths; 670 hits | SPEC-HEALTH KEEP; serves PROD-TEST-011. |
| `affected-plan-reuse-v0.md` | AFP-V0 | KEEP | active | - | no paths; 670 hits | SPEC-HEALTH KEEP; serves PROD-TEST-012. |
| `affected-plan-selection-v0.md` | AFP-V0 | KEEP | active | - | no paths; 670 hits | SPEC-HEALTH KEEP; serves PROD-TEST-001, PROD-TEST-002. |
| `affected-plan-v0.md` | AFP-V0 | KEEP | active | - | 7/7 paths; 670 hits | SPEC-HEALTH KEEP; serves PROD-TEST-001, PROD-TEST-002, PROD-TEST-012. |
| `affected-reader-precision-v0.md` | AFP-V0 | MERGE | archive in place | C2 | no paths; 670 hits | Merges into affected-zero-unknown-v0.md, then archived. |
| `affected-zero-unknown-v0.md` | AZU-V0 | KEEP | active | - | no paths; 120 hits | SPEC-HEALTH KEEP; serves PROD-TEST-002. |
| `agent-harness-integration-v0.md` | AHI | retired | archive in place | C1 | 3/3 paths; 86 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `agent-instruction-doctor-v0.md` | IID-V0 | OUT-OF-1.0 | archive in place | B7 | 3/3 paths; 0 hits | Part of the context packet; superseded by ticket-brief-context. |
| `aggregate-local-outcome-v0.md` | ALO-V0 | OUT-OF-1.0 | archive in place | B6 | 5/5 paths; 9 hits | CEM out of 1.0 (G8); tracerecordrepo goes in B1, cem paths in B6. |
| `analyzer-candidate-profiles.md` | ACP | OUT-OF-1.0 | archive in place | B5 | 9/9 paths; 36 hits | B5 (G26) removes the analyzers. |
| `analyzer-capability-contract-v0.md` | ACC-V0 | retired | archive in place | C1 | no paths; 4 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `analyzer-python-native-candidate.md` | PNC | OUT-OF-1.0 | archive in place | B5 | 2/2 paths; 7 hits | B5 (G26) removes internal/analyzerpython and its binary. |
| `application-flow-navigation-execution-v0.md` | NEX-V0 | KEEP | active | - | 3/3 paths; 2 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-008. |
| `application-flow-understanding-v0.md` | AFU-V0 | MERGE | archive in place | C2 | 3/3 paths; 5 hits | Merges into application-flow-understanding-v1.md, then archived. |
| `application-flow-understanding-v1.md` | AFU-V1 | KEEP | active | - | 6/6 paths; 132 hits | SPEC-HEALTH KEEP; serves PROD-TEST-004, PROD-TEST-005, PROD-TEST-010, PROD-DOCS-004, PROD-DOCS-005, PROD-DOCS-008, PROD-INT-002. |
| `application-map-scenario-planner-v0.md` | AMSP-V0 | KEEP | active | - | 2/2 paths; 6 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-007. |
| `application-map-v0.md` | AMAP-V0 | KEEP | active | - | 2/2 paths; 151 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-006. |
| `application-navigation-ledger-v0.md` | ANL-V0 | KEEP | active | - | 2/2 paths; 46 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-006. |
| `applied-intelligence-breakthroughs-v0.md` | - | OUT-OF-1.0 | archive in place | B9 | no paths | Programme statement; context-quality experiments are out of 1.0. |
| `authoring-step-scope-v0.md` | ASS-V0 | retired | archive in place | C1 | 1/1 paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `authority-trigger-index-v0.md` | ATI-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `batch-integration-checks-v0.md` | ASD-V0 | MERGE | archive in place | B8b | no paths; 179 hits | Merge target corvint-self-development-gates-v0 is itself out of 1.0; archived with the lane tooling. |
| `beamfall-shadow-dogfood-v1.md` | BSD | RETIRE | archive in place | C1 | no paths; 5 hits | Superseded; successor is itself retired. Delivery set to retired. |
| `browser-behavior-falsification-v0.md` | BBF-V0 | retired | archive in place | C1 | no paths; 4 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `capability-catalog-v0.md` | CAP-V0 | KEEP | active | - | 2/2 paths; 76 hits | SPEC-HEALTH KEEP; serves PROD-INT-003. |
| `cem-0.2-canonical-binding.md` | CEM-CB | RETIRE | archive in place | C1 | no paths; 14 hits | Retired by decision 0525 but delivery reads superseded; set to retired. |
| `cem-0.3-structural-mechanical.md` | CEM-SM | RETIRE | archive in place | C1 | no paths; 27 hits | Retired by decision 0525 but delivery reads superseded; set to retired. |
| `cem-2.0-intent.md` | CEM2-V0 | OUT-OF-1.0 | archive in place | B6 | no paths; 0 hits | CEM out of 1.0 (G8); not built. |
| `cem-additive-profiles-v0.md` | CAX-V0 | OUT-OF-1.0 | archive in place | B6 | no paths; 34 hits | CEM out of 1.0 (G8). |
| `cem-completeness-profile-v0.md` | CCP-V0 | OUT-OF-1.0 | archive in place | B6 | no paths; 9 hits | CEM out of 1.0 (G8); B6 removes protocol/cem-completeness-1. |
| `cem-content-keyed-claims-v0.md` | CKC-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `cem-criterion-experiments-v0.md` | CEX-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `cem-external-interop-v0.md` | CEM-EXT | RETIRE | archive in place | C1 | no paths; 0 hits | Retired by decision 0525 but delivery reads superseded; set to retired. |
| `cem-go-portability-probe.md` | CEM-GO | RETIRE | archive in place | C1 | no paths; 2 hits | Retired by decision 0525 but delivery reads superseded; set to retired. |
| `cem-pilot-kit.md` | CEM-PILOT | OUT-OF-1.0 | archive in place | B6 | 2/2 paths; 14 hits | CEM out of 1.0 (G8); B6 removes interop/cem01-go. |
| `cem-reviewer-trial-v0.md` | CRT-V0 | retired | archive in place | C1 | 1/1 paths; 7 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `cem-stable-bundle-v0.md` | CSB-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `cem-stable-v1.md` | CEM-V1 | OUT-OF-1.0 | archive in place | B6 | 4/4 paths; 262 hits | CEM out of 1.0 (G8); B6b removes the verbs and protocol/cem-1.0. |
| `cem2-discovery-replay-v0.md` | VRP-V0 | OUT-OF-1.0 | archive in place | B6 | no paths; 4 hits | CEM out of 1.0 (G8); not built. |
| `change-compiled-e2e-v0.md` | CCE-V0 | retired | archive in place | C1 | no paths; 22 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `change-frontier-profile-1.md` | CF-V1 | retired | archive in place | C1 | no paths; 39 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `change-frontier-v0.md` | CF-V0 | retired | archive in place | C1 | no paths; 16 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `change-witness-relation-v0.md` | CWR-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `ci-control-plane-early-post-v0.md` | CCEP-V0 | MERGE | archive in place | B8b | no paths; 2 hits | Merge target corvint-self-development-gates-v0 is itself out of 1.0; archived with the lane tooling. |
| `ci-narrowing-v0.md` | CIN-V0 | KEEP | active | - | no paths; 76 hits | SPEC-HEALTH KEEP; serves PROD-TEST-012. |
| `closed-vocabulary-intake-v0.md` | CVI-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `compaction-kernel-v0.md` | CKN-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `compat-replay-runner-v0.md` | CRR-V0 | retired | archive in place | C1 | 1/1 paths; 7 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `compat-trial-v0.md` | CTR-V0 | retired | archive in place | C1 | no paths; 8 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `confidently-wrong-trial-v0.md` | CWT-V0 | OUT-OF-1.0 | archive in place | B9 | 2/2 paths; 33 hits | Context-quality trial; out of 1.0. Removal of tools/cw-trial is not scheduled in any batch. |
| `context-evolution-program-v0.md` | CEP-V0 | OUT-OF-1.0 | archive in place | B9 | 3/3 paths; 21 hits | Context-quality programme; out of 1.0. |
| `context-ranking-v0.md` | CRK-V0 | OUT-OF-1.0 | archive in place | B7 | no paths; 13 hits | B7 removes the ranking files; superseded by ticket-brief-context. |
| `core-bundled-apps-v0.md` | CMS-V0 | KEEP | active | - | no paths; 29 hits | SPEC-HEALTH KEEP; serves PROD-INT-004, PROD-DOCS-011. |
| `core-compatibility-freeze-v1.md` | CCF-V1 | KEEP | active | - | 1/1 paths; 39 hits | SPEC-HEALTH KEEP; serves PROD-INT-007. |
| `corpus-republish-v0.md` | RCP-V0 | retired | archive in place | C1 | 1/1 paths; 2 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `correlated-witness-collapse-x0.md` | CWC-X0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `corvint-1.0-product-and-release-v1.md` | PRS-V1 | KEEP | active | - | no paths; 48 hits | SPEC-HEALTH KEEP; serves PROD-INT-008. |
| `corvint-rebrand-v0.md` | CRB-V0 | OUT-OF-1.0 | archive in place | B9 | 11/11 paths; 14 hits | Record of a finished migration. |
| `corvint-self-development-companion-v0.md` | CSD-V0 | MERGE | archive in place | B8b | no paths; 5 hits | Merge target corvint-self-development-gates-v0 is itself out of 1.0; archived with the lane tooling. |
| `corvint-self-development-gates-v0.md` | ASC-V0 | OUT-OF-1.0 | archive in place | B8b | no paths; 8 hits | Lane and batch process gates; out of 1.0 with the lane tooling. |
| `corvint-self-development-selection-v0.md` | ASD-V0 | MERGE | archive in place | B8b | no paths; 179 hits | Merge target corvint-self-development-gates-v0 is itself out of 1.0; archived with the lane tooling. |
| `corvint-self-development-v0.md` | ASD-V0 | KEEP | active | B6, B8b | 3/3 paths; 179 hits | Kept (PROD-INT-009); its CEM and lane routes are removed in B6 and B8b. |
| `corvint-tasks-agent-leases-v0.md` | CAL-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 49 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-agent-workflow-v0.md` | TAW-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-attempt-runner-v0.md` | ATR-V0 | retired | move to beamfall/corvint-tasks | B10 | 1/1 paths; 21 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-basic-stability-v1.md` | TBS-V1 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-cli-v0.md` | TCL-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 14 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-closed-ticket-index-v0.md` | TCX-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-closeout-debt-v0.md` | TCD-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 4 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-create-payload-v0.md` | TCT-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-defect-register-v0.md` | TDR-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-delivery-lifecycle-v0.md` | TLC-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-dispatch-signals-v0.md` | DSG-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-doctor-v0.md` | TQD-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 7 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-escalations-v0.md` | ESC-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 1 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-evidence-attachments-v0.md` | TEA-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 1 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-external-reviews-v0.md` | ERG-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 2 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-intent-worktree-v0.md` | CTW-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 1 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-journal-retention-v0.md` | JRT-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 1 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-journal-segments-v0.md` | JSG-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 6 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-know-how-notes-v0.md` | KHN-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 1 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-lane-flow-v0.md` | TLF-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 3 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-lane-ticket-read-v0.md` | TCP-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 106 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-lease-gate-latency-v0.md` | LGL-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 5 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-obligation-ledger-v0.md` | TOL-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 7 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-operator-notes-v0.md` | ON-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 3 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-pending-unpause-recovery-v0.md` | PUR-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-pool-safe-reuse-v0.md` | PSR-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 14 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-release-membership-and-resume-v0.md` | TRM-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 1 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-source-containment-v0.md` | SCC-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-store-init-v0.md` | CTS-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 3 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-store-relocation-v0.md` | TRL-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 2 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-ticket-worktrees-v0.md` | TWT-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-tasks-user-service-v0.md` | SERVICE500 | retired | move to beamfall/corvint-tasks | B10 | no paths; 2 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `corvint-test-fold-v0.md` | CTF-V0 | retired | archive in place | C1 | no paths; 4 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `corvint-witness-v0.md` | AGW-V0 | retired | archive in place | C1 | 1/1 paths; 5 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `cross-audience-truth-nondivergence-v0.md` | CATN-V0 | OUT-OF-1.0 | archive in place | B9 | 1/1 paths; 5 hits | Experiment; internal/docviews fate is source-removal open item 5. |
| `cross-repository-breakage-map-v0.md` | BKM-V0 | OUT-OF-1.0 | archive in place | B1 | 1/1 paths; 10 hits | B1 removes internal/breakagemap. |
| `daily-change-evidence-workflow-companion-v0.md` | DWC-V0 | retired | archive in place | C1 | no paths; 17 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `daily-change-evidence-workflow-v0.md` | DCW-V0 | retired | archive in place | C1 | 2/2 paths; 71 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `decision-number-gate-v0.md` | DNG-V0 | OUT-OF-1.0 | active | - | 1/1 paths; 2 hits | Lint kept by the removal panel; owns script/check-decision-numbers.sh. |
| `dependency-source-evidence-v0.md` | DSE-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `deployment-neutral-index-platform-v0.md` | DNIP | OUT-OF-1.0 | archive in place | B4 | 1/1 paths; 8 hits | Not started; B4 adds its status line; AGENTS.md invariant 7 stops citing it (design.md outline). |
| `diagnostic-repair-contract-v0.md` | DRC-V0 | KEEP | active | - | 2/2 paths; 17 hits | SPEC-HEALTH KEEP; serves PROD-INT-007. |
| `direct-native-cli-authority-v0.md` | DCLI-V0 | retired | archive in place | C1 | 1/1 paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `documentation-ci-v0.md` | DCI-V0 | KEEP | active | - | 2/2 paths; 6 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-003. |
| `documentation-citation-gate-v0.md` | DCG-V0 | KEEP | active | - | 2/2 paths; 8 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-003, PROD-INT-005. |
| `documentation-corpus-extraction-delta-v1.md` | DCX-V1 | KEEP | active | - | 4/4 paths; 72 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-009. |
| `documentation-corpus-inventory-v1.md` | DCI-V1 | KEEP | active | - | no paths; 17 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-009. |
| `documentation-corpus-search-ranking-v1.md` | DCS-V1 | KEEP | active | - | 2/2 paths; 49 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-009. |
| `documentation-corpus-v1.md` | DCP-V1 | KEEP | active | - | 3/3 paths; 197 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-009. |
| `documentation-corpus-verified-reads-v1.md` | DCR-V1 | KEEP | active | - | 3/3 paths; 51 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-009. |
| `dogfood-citation-plan-precheck-v0.md` | DCW-V0 | retired | archive in place | C1 | no paths; 71 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `dogfood-lane-friction-v0.md` | DCW-V0 | retired | archive in place | C1 | no paths; 71 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `dogfood-query-abstention-v0.md` | QAT-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `dogfood-retroactive-binding-v0.md` | DOGFOOD-BIND | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `dotnet-affected-selection-v0.md` | DNAS-V0 | KEEP | archive in place | B5 | 1/1 paths; 0 hits | B5 (G26) removes internal/liveverify/affected/dotnet; SPEC-HEALTH KEEP predates G26. |
| `eol-policy-gate-v0.md` | EPG-V0 | OUT-OF-1.0 | archive in place | B8a | 2/2 paths; 0 hits | B8a removes check-eol-policy. |
| `error-code-ownership-gate-v0.md` | ECO-V0 | OUT-OF-1.0 | active | - | 2/2 paths; 2 hits | Lint kept by the removal panel. |
| `expanded-range-impact-v0.md` | ERI-V0 | OUT-OF-1.0 | archive in place | B7 | 2/2 paths; 10 hits | B7 removes the old impact engine; superseded by change-report-impact. |
| `experimental-source-views-v0.md` | ESV-V0 | retired | archive in place | C1 | 1/1 paths; 7 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `expert-audit-followup-v0.md` | EAF-V0 | OUT-OF-1.0 | archive in place | B9 | 2/2 paths; 18 hits | Record of audit repairs. |
| `external-evidence-provider-mcp-v0.md` | EEP-MCP | MERGE | archive in place | C2 | 2/2 paths; 2 hits | Merges into external-evidence-provider-v0.md, then archived. |
| `external-evidence-provider-remote-v0.md` | EEP-REMOTE | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `external-evidence-provider-transports-v0.md` | EEP-TR | MERGE | archive in place | C2 | 2/2 paths; 12 hits | Merges into external-evidence-provider-v0.md, then archived. |
| `external-evidence-provider-v0.md` | EEP-V0 | KEEP | active | B4 | 3/3 paths; 42 hits | Kept as an internal format (RMV-V1-013); B4 removes examples/evidence-provider and the doc it links. |
| `external-evidence-provider-v1.md` | EEP-V1 | MERGE | archive in place | C2 | 2/2 paths; 4 hits | Merges into external-evidence-provider-v0.md, then archived. |
| `external-evidence-provider-v2.md` | EEP-V2 | MERGE | archive in place | C2 | 2/2 paths; 9 hits | Merges into external-evidence-provider-v0.md, then archived. |
| `external-frontier-obligations-v0.md` | EFO-V0 | RETIRE | archive in place | C1 | 2/2 paths; 0 hits | Command absent from source; internal/extevidence stays for external-evidence-provider-v0. |
| `external-test-selection-v0.md` | ETS-V0 | KEEP | active | - | 2/2 paths; 4 hits | SPEC-HEALTH KEEP; serves PROD-TEST-004. |
| `external-test-selection-v1.md` | ETS-V1 | MERGE | archive in place | C2 | 2/2 paths; 3 hits | Merges into external-test-selection-v0.md, then archived. |
| `falsifiable-packet-v0.md` | FPK-V0 | retired | archive in place | C1 | 1/1 paths; 46 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `flow-document-maintenance-v0.md` | FDM-V0 | KEEP | active | - | 3/3 paths; 3 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-003, PROD-DOCS-005. |
| `gate-ledger-v0.md` | GL-V0 | OUT-OF-1.0 | active | B11 | 5/5 paths; 24 hits | Active until the action cut-over replaces tools/gate-ledger (RMV-V1-028); stays for 1.0 if B11 slips. |
| `gate-verdict-agreement-v0.md` | GVA-V0 | retired | archive in place | C1 | no paths; 1 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `genesis-backfill.md` | GENESIS | OUT-OF-1.0 | active | - | 2/2 paths; 2 hits | `corvint init` and `adopt` inventory; internal/genesis stays. Decided 2026-11-09 under R42, E8: `init` is a 1.0 first-run verb with a product requirement. Low confidence, as for ADS-V0. |
| `go-archive-gate-v0.md` | GAG-V0 | KEEP | active | B11 | 3/3 paths; 11 hits | Active until the action cut-over replaces script/go-archive-gate (RMV-V1-028); archived in B11, or stays for 1.0 if B11 slips past 2026-12-15. |
| `go-format-gate-v0.md` | GFG-V0 | OUT-OF-1.0 | active | - | 2/2 paths; 0 hits | Lint kept by the removal panel. |
| `go-line-selection-v0.md` | GLS-V0 | KEEP | active | - | no paths; 61 hits | SPEC-HEALTH KEEP; serves PROD-TEST-008. |
| `go-live-test-provider-v0.md` | GLTP-V0 | KEEP | active | - | 2/2 paths; 24 hits | SPEC-HEALTH KEEP; serves PROD-TEST-006, PROD-TEST-007. |
| `go-only-cutover-v0.md` | GOC-V0 | OUT-OF-1.0 | archive in place | B9 | 3/3 paths; 27 hits | Record of a finished migration. |
| `go-production-kernel-migration-v0.md` | GPK-V0 | OUT-OF-1.0 | archive in place | B9 | 6/6 paths; 142 hits | Migration record; AGENTS.md invariant 7 stops citing it (design.md outline). |
| `go-test-evidence-v0.md` | GTE-V0 | OUT-OF-1.0 | archive in place | C1 | no paths; 2 hits | Decided 2026-11-09 under R42, E8: `tools/gotest-evidence` has no importer in `cmd` or `internal`; only CI cost and line-coverage files name it. |
| `gopls-client-session-v0.md` | GCS-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `greptile-gate-v0.md` | GRG-V0 | MERGE | active | - | no paths; 3 hits | Decided 2026-11-09 under R42, D11: `greptile-gate.yml` is kept for 1.0 until the owner removes the required status (RMV-V1-035), so its spec stays active and is not archived in B8b. Earlier class, superseded by D11: archive in place, B8b. |
| `harness-authority-relation-v0.md` | HAR-V0 | RETIRE | archive in place | C1 | no paths; 0 hits | Superseded by decision 0009. |
| `heading-navigation-experiment-v0.md` | HNE-V0 | OUT-OF-1.0 | archive in place | B9 | 1/1 paths; 1 hits | Experiment; tools/heading-nav-bench removal is not scheduled in any batch. |
| `host-lifecycle-qualification-v1.md` | HLQ-V1 | retired | archive in place | C1 | no paths; 2 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `human-documentation-compiler-v0.md` | HDCV0 | KEEP | active | - | 2/2 paths; 11 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-002. |
| `immutable-delta-v0.md` | DLT-V0 | retired | archive in place | C1 | 1/1 paths; 15 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `import-ratchet-v0.md` | IRT-V0 | KEEP | active | - | no paths; 5 hits | SPEC-HEALTH KEEP; serves PROD-INT-001, PROD-DOCS-011. |
| `incremental-closeout-v0.md` | ICL-V0 | retired | archive in place | C1 | no paths; 2 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `index-snapshot-v0.md` | IDX-SNAP-V0 | KEEP | active | B7 | 2/2 paths; 89 hits | Kept; B7 removes the snapshot path only if the watch measurement does not need it (RMV-V1-023). Re-decided in B7. |
| `java-affected-test-v0.md` | JAT-V0 | KEEP | archive in place | B5 | 1/1 paths; 0 hits | B5 (G26) removes internal/liveverify/affected/java; SPEC-HEALTH KEEP predates G26. |
| `js-live-test-provider-v0.md` | - | KEEP | active | - | 2/2 paths | SPEC-HEALTH KEEP; serves PROD-TEST-006. |
| `kotlin-android-candidate-v0.md` | KA-V0 | OUT-OF-1.0 | archive in place | B5 | 2/2 paths; 4 hits | B5 (G26) removes the analyzer. |
| `kotlin-jvm-affected-test-v0.md` | KAT-V0 | KEEP | archive in place | B5 | 1/1 paths; 3 hits | B5 (G26) removes internal/liveverify/affected/kotlin; SPEC-HEALTH KEEP predates G26. |
| `lane-check-tooling-v0.md` | ASD-V0 | MERGE | archive in place | B8b | no paths; 179 hits | Merge target corvint-self-development-gates-v0 is itself out of 1.0; archived with the lane tooling. |
| `lane-dogfood-tooling-v0.md` | DTL-V0 | MERGE | archive in place | B8b | no paths; 23 hits | Merge target corvint-self-development-gates-v0 is itself out of 1.0; archived with the lane tooling. |
| `lane-shell-portability-v0.md` | ASD-V0 | MERGE | archive in place | B8b | 1/1 paths; 179 hits | Merge target corvint-self-development-gates-v0 is itself out of 1.0; archived with the lane tooling. |
| `lane-verification-tooling-v0.md` | LVT-V0 | MERGE | archive in place | B8b | no paths; 23 hits | Merge target corvint-self-development-gates-v0 is itself out of 1.0; archived with the lane tooling. |
| `language-plugins-v0.md` | LGP-V0 | KEEP | active | B5 | no paths; 34 hits | Kept; B5 edits the provider list (internal/langplugin/langplugin.go). |
| `learned-trace-admission-v0.md` | LTA-V0 | OUT-OF-1.0 | archive in place | B1, B7 | 5/5 paths; 15 hits | Decided 2026-11-09 under R42, E8: B1 and B7 remove its code; G25 item 3 makes `context` work from declared links only, with no ranking; 1.0 learns nothing from use (E3). |
| `lexical-relevance-floor-v0.md` | LRF-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `live-proof-carrying-verification-v0.md` | LPCV-V0 | KEEP | active | - | 2/2 paths; 200 hits | SPEC-HEALTH KEEP; serves PROD-TEST-007, PROD-TEST-008. |
| `local-admin-console-v0.md` | LAC-V0 | retired | archive in place | C1 | no paths; 1 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `local-completion-policy-v0.md` | LCP-V0 | OUT-OF-1.0 | archive in place | B6 | 1/1 paths; 12 hits | Completion for enrolled CEM changes (G8). Whether B6 removes internal/contextindex/local_completion_context.go is NOT_VERIFIED. |
| `local-model-gateway-v0.md` | LMG-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `local-observability-dashboard-v0.md` | LOD-V0 | retired | archive in place | C1 | no paths; 2 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `local-trace-producer-migration-v0.md` | LTPM-V0 | retired | archive in place | C1 | 3/3 paths; 13 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `lsp-editor-context-v0.md` | LEC-V0 | RETIRE | archive in place | C1 | 1/1 paths; 0 hits | LSP cut by decision 0594; internal/gitstatus is shared and stays. |
| `lsp-editor-definition-v0.md` | LED-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `lsp-editor-qualification-v0.md` | LEQ-V0 | RETIRE | archive in place | B4 | 2/2 paths; 1 hits | B4 removes tools/lsp-editors and script/qualify-lsp-editors.py. |
| `lsp-qualification-evaluation-policy-v0.json` | - | n/a (not a spec) | archive in place | B4 | no paths | Only reader is lsp-quality-platform-v0.md (no code reader in integ 9ec3a29d0); goes with the LSP scripts. |
| `lsp-quality-platform-v0.md` | LQP-V0 | RETIRE | archive in place | B4 | 1/1 paths; 3 hits | B4 removes script/measure-lsp-quality.py. |
| `lsp-snapshot-overlay-v0.md` | LSO-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `lsp-stdio-experiment-v0.md` | LSE-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `lsp-wrapped-backends-v0.md` | LWB-V0 | RETIRE | archive in place | C1 | no paths; 0 hits | LSP cut by decision 0594; no code. |
| `map-coverage-join-v0.md` | MCJ-V0 | KEEP | active | - | no paths; 12 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-006, PROD-TEST-004. |
| `mcp-multi-root-v0.md` | MMR-V0 | KEEP | active | - | 2/2 paths; 31 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-001, PROD-DOCS-010, PROD-TEST-009. |
| `mcp-server-2026-07-28-v0.md` | MCPV0 | KEEP | active | - | 3/3 paths; 89 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-001. |
| `mcp-test-validity-profile-v0.md` | MTV-V0 | OUT-OF-1.0 | archive in place | B2 | 4/4 paths; 14 hits | B2 (G24) removes cmd/corvint-test-validity-mcp. |
| `migration-evidence-ratchet-v0.md` | MER-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `native-cem-adapter.md` | - | OUT-OF-1.0 | archive in place | B6 | 1/1 paths | One dependency rule for internal/cem, which B6 splits and removes. |
| `native-jvm-bridge-analyzer-candidates-v0.md` | NJB | OUT-OF-1.0 | archive in place | B5 | 4/4 paths; 4 hits | B5 (G26) removes the analyzer. |
| `native-observer-prearm-v0.md` | NPO-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `native-shell-affected-test-v0.md` | NSA-V0 | KEEP | active | B5 | 2/2 paths; 12 hits | Shell provider stays; B5 removes the cfamily half, so the spec is rewritten to shell only in B5. |
| `native-taskman-planning-v0.md` | NTP-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 3 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `necessity-labels-v0.md` | NEC-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `new-e2e-test-acceptance-v0.md` | NEA-V0 | retired | archive in place | C1 | 1/1 paths; 7 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `non-go-impact-v0.md` | NGI-V0 | OUT-OF-1.0 | archive in place | B7 | 7/7 paths; 55 hits | B7 removes the old impact engine; superseded by change-report-impact. |
| `observation-corpus-authority-v0.md` | OCA-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `observed-ui-evidence-v0.md` | OUE-V0 | OUT-OF-1.0 | archive in place | C1 | no paths; 19 hits | Decided 2026-11-09 under R42, E8: PROD-DOCS-008 is served by two active specs, `application-flow-understanding-v1` and `playwright-external-provider-v0`. |
| `ocm-intent-forms-v0.md` | OIF-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `ocm-v0-dogfood.md` | OCM-V0 | retired | archive in place | C1 | no paths; 11 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `operator-update-v0.md` | UPD-V0 | KEEP | active | - | 2/2 paths; 4 hits | SPEC-HEALTH KEEP; serves PROD-INT-006. |
| `optional-artifact-check-scripts-v0.md` | OACS-V0 | OUT-OF-1.0 | active | B5 | 3/3 paths; 3 hits | Decided 2026-11-09 under R42, E8: active, trimmed in B5. The release reproducibility scripts stay; the analyzer requirements retire with B5. |
| `outcome-calibration-v0.md` | OCL-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `performance-profiling-and-read-budget-v0.md` | PRB-V0 | KEEP | active | - | 6/6 paths; 41 hits | SPEC-HEALTH KEEP; serves PROD-TEST-007, PROD-DOCS-001. |
| `pi-workflow-v0.md` | PWV-V0 | retired | archive in place | C1 | no paths; 1 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `playwright-external-provider-v0.md` | PWP-V0 | KEEP | active | - | 3/3 paths; 54 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-008. |
| `playwright-per-test-freshness-v0.md` | PTF-V0 | MERGE | archive in place | C2 | 1/1 paths; 9 hits | Merges into playwright-external-provider-v0.md, then archived. |
| `playwright-run-evidence-v0.md` | PWR-V0 | KEEP | archive in place | B2 | 2/2 paths; 29 hits | Decided 2026-11-09 under R42, E8: G24 removes internal/playwrightruns, its only package, in B2; run records move to PROD-TEST-020 (owner decision R7). |
| `playwright-suite-interaction-minimizer-v0.md` | PSM-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `postmerge-ci-host-v0.md` | PCH-V0 | retired | archive in place | C1 | no paths; 1 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `postmerge-connectors-v0.md` | PMC-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `postmerge-metrics-v0.md` | PMM-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `postmerge-replay-v0.md` | PMR-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `postmerge-runtime-v1.md` | PMR-V1 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `postmerge-runtime-v2.md` | PMR-V2 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `process-group-owner-v0.md` | PGO-V0 | KEEP | active | - | 1/1 paths; 35 hits | SPEC-HEALTH KEEP; serves PROD-TEST-006. |
| `proof-carrying-context-optimization-v0.md` | PCCO-V0 | RETIRE | archive in place | C1 | 2/2 paths; 6 hits | Decided 2026-11-09 under R42, E8: G25 keeps the context ranking files removed. The superseding decision MUST name decision 0047, which accepted the programme. |
| `proof-preserving-abstraction-v0.md` | PPA-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `proof-tour-v0.md` | PT-V0 | OUT-OF-1.0 | archive in place | B6 | 2/2 paths; 2 hits | B6 edits script/proof-tour.sh to drop `corvint cem`; the tour itself is not a 1.0 product. Script fate NOT_VERIFIED. |
| `protected-local-execution-v0.md` | PLE-V0 | retired | archive in place | C1 | no paths; 2 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `protected-pi-runtime-v0.md` | PPI-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `public-release-v0.md` | PUB-V0 | KEEP | active | - | no paths; 62 hits | SPEC-HEALTH KEEP; serves PROD-INT-006. |
| `pulse-snapshot-lease-v0.md` | PSL-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `qualified-lifecycle-v0.md` | QLF-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `receipt-bundle-v0.md` | RCB-V0 | retired | archive in place | C1 | no paths; 2 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `release-artifact-integrity-v0.md` | ARTIFACT-V0 | KEEP | active | - | 6/6 paths; 6 hits | SPEC-HEALTH KEEP; serves PROD-INT-006. |
| `repository-guidance-v0.md` | RGV-V0 | OUT-OF-1.0 | archive in place | B7 | 2/2 paths; 7 hits | Superseded by review-evidence-check and feature-map-from-flows (G25). |
| `reproduction-bundle-v0.md` | RPB-V0 | RETIRE | archive in place | C1 | no paths; 0 hits | Command absent from source and catalogue. |
| `requirement-definition-gate-v0.md` | RDG-V0 | KEEP | active | - | 1/1 paths; 6 hits | SPEC-HEALTH KEEP; serves PROD-INT-005, PROD-INT-011. |
| `retrieval-bench-diagnostics-v0.md` | RBD-V0 | OUT-OF-1.0 | archive in place | B9 | 1/1 paths; 3 hits | Experiment; tools/retrieval-bench removal is not scheduled in any batch. |
| `retrieval-eval-comparability-v0.md` | REC-V0 | retired | archive in place | C1 | no paths; 2 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `retriever-disagreement-v0.md` | RDS-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `revision-cache-dirty-worktree-v0.md` | DIRTY-CACHE | KEEP | active | - | 1/1 paths; 30 hits | SPEC-HEALTH KEEP; serves PROD-TEST-007. |
| `ruby-affected-test-adapter-v0.md` | RUBY-AFFECTED-V0 | KEEP | active | - | 1/1 paths; 21 hits | SPEC-HEALTH KEEP; serves PROD-TEST-011. |
| `run-verified-navigation-v0.md` | RVN-V0 | KEEP | active | - | 2/2 paths; 7 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-008. |
| `runner-selection-plugins-v0.md` | RSP-V0 | KEEP | active | B3 | no paths; 109 hits | Kept; B3 adds the status line for RSP-V0-026..032 (runner protocol removed). |
| `rust-affected-test-understanding-v0.md` | RUST-AFFECTED-V0 | KEEP | archive in place | B5 | 1/1 paths; 0 hits | B5 (G26) removes internal/liveverify/affected/rust; SPEC-HEALTH KEEP predates G26. |
| `rust-analyzer-candidate-v0.md` | RAC | OUT-OF-1.0 | archive in place | B5 | 2/2 paths; 2 hits | B5 (G26) removes the analyzer. |
| `scope-lease-v0.md` | SCL-V0 | retired | move to beamfall/corvint-tasks | B10 | 1/1 paths; 8 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `self-observation-ledger-v0.md` | SOL-V0 | OUT-OF-1.0 | active | - | 2/2 paths; 26 hits | Decided 2026-11-09 under R42, E8: the ledger stays in 1.0 and is documented as local-only data; it attaches to the telemetry and uninstall requirement PROD-INT-006. Low confidence: the ledger writer was not traced; the verdict changes if it is dead code. |
| `semantic-escalation-gate-v0.md` | SEG | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `session-context-dividend-v0.md` | SESSION-V0 | retired | archive in place | C1 | no paths; 1 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `session-test-runner-v0.md` | STR-V0 | KEEP | active | - | 6/6 paths; 94 hits | SPEC-HEALTH KEEP; serves PROD-TEST-006, PROD-TEST-012. |
| `shader-glsl-metal-analyzer-candidate-v0.md` | SHA-V0 | OUT-OF-1.0 | archive in place | B5 | 2/2 paths; 0 hits | B5 (G26) removes the analyzer. |
| `shared-path-change-policy-v0.md` | SPC-V0 | MERGE | archive in place | B8b | no paths; 2 hits | Merge target corvint-self-development-gates-v0 is itself out of 1.0; archived with the lane tooling. |
| `site-knowledge-howto-v0.md` | SKH-V0 | retired | archive in place | C1 | no paths; 36 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `snapshot-batch-v0.md` | SBQ-V0 | RETIRE | archive in place | B7 | 1/1 paths; 7 hits | Command absent; the snapshot path goes in B7. |
| `source-documentation-draft-v0.md` | SDD-V0 | KEEP | active | - | 4/4 paths; 32 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-002. |
| `source-evidence-axes-v0.md` | SEA-V0 | RETIRE | archive in place | C1 | no paths; 0 hits | Console cut by decision 0594; not started. |
| `spec-requirement-index-generator-v0.md` | SRG-V0 | KEEP | active | - | 2/2 paths; 40 hits | SPEC-HEALTH KEEP; serves PROD-INT-005, PROD-INT-011. |
| `spec-toolchain-adapters-v0.md` | STA-V0 | RETIRE | archive in place | B1 | no paths; 22 hits | Reader removed; B1 removes script/spec-toolchain-conformance.sh. |
| `sql-native-ratchet-gate-v0.md` | SNR-V0 | OUT-OF-1.0 | archive in place | B5 | 1/1 paths; 3 hits | B5 removes the sql-native ratchet. |
| `stable-operations-v0.md` | SOP-V0 | KEEP | active | - | 6/6 paths; 10 hits | SPEC-HEALTH KEEP; serves PROD-INT-006, PROD-INT-007. |
| `stable-readiness-record-v1.md` | SRR-V1 | KEEP | active | - | 1/1 paths; 4 hits | SPEC-HEALTH KEEP; serves PROD-INT-007. |
| `structured-data-analyzer-candidate-v1.md` | SDA | OUT-OF-1.0 | archive in place | B5 | 2/2 paths; 0 hits | B5 (G26) removes cmd/corvint-analyzer-structured-data. |
| `swift-affected-test-understanding-v0.md` | SATS-V0 | KEEP | archive in place | B5 | 1/1 paths; 0 hits | B5 (G26) removes internal/liveverify/affected/swift; SPEC-HEALTH KEEP predates G26. |
| `swift-apple-analyzer-candidate-v0.md` | SAC | OUT-OF-1.0 | archive in place | B5 | 2/2 paths; 4 hits | B5 (G26) removes the analyzer. |
| `task-context-packet-v0.md` | TCP-V0 | OUT-OF-1.0 | archive in place | B7 | 2/2 paths; 106 hits | Superseded by ticket-brief-context (R28, R29; G25). |
| `tasks-criterion-binding-v0.md` | TCB-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 4 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `tasks-mcp-read-profile-v0.md` | TMR-V0 | retired | move to beamfall/corvint-tasks | B10 | no paths; 0 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `technical-brain-dogfood.md` | BRAIN-DOG | retired | archive in place | C1 | no paths; 4 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `test-attack-v0.md` | TAT-V0 | OUT-OF-1.0 | undetermined | - | 1/1 paths; 1 hits | Decided 2026-11-09 under R42, E8: decided with the fault-injection proposal. internal/liveverify/mutate stays and is ticketed (RMV-V1-017); the spec is kept if the miss-proof set reuses it. |
| `test-claim-qualification-v0.md` | TCQ-V0 | OUT-OF-1.0 | archive in place | B2 | 1/1 paths; 87 hits | B2 (G24) removes tcq internals; the identifiers other packages use stay without a spec of their own. |
| `test-consolidation-planner-v0.md` | TCN-V0 | OUT-OF-1.0 | archive in place | B2 | 2/2 paths; 19 hits | B2 (G24) removes `corvint test-plan` and internal/testplan. |
| `test-runner-execution-v0.md` | TRE-V0 | KEEP | active | - | 2/2 paths; 91 hits | SPEC-HEALTH KEEP; serves PROD-TEST-006. |
| `test-selection-precision-v0.md` | TSP-V0 | KEEP | active | - | no paths; 95 hits | SPEC-HEALTH KEEP; serves PROD-TEST-001, PROD-TEST-002. |
| `test-source-analysis-v0.md` | TSA-V0 | OUT-OF-1.0 | active | B2 | no paths; 178 hits | Decided 2026-11-09 under R42, E8: active, trimmed in B2. The validity requirements retire; RMV-V1-009 keeps the rest. |
| `touch-set-surprise-v0.md` | TSS-V0 | retired | archive in place | C1 | no paths; 0 hits | Already retired (decision 0594 or earlier); add the Health line only. |
| `traceability-test-gate-v0.md` | TTG-V0 | KEEP | active | - | 1/1 paths; 3 hits | SPEC-HEALTH KEEP; serves PROD-INT-005. |
| `trial-replay-journal-v0.md` | TRJ-V0 | RETIRE | archive in place | C1 | no paths; 0 hits | Not started; tools/cw-trial goes with confidently-wrong-trial-v0 (B9). |
| `trusted-project-navigation-v0.md` | TPN-V0 | KEEP | active | - | 2/2 paths; 3 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-002. |
| `typescript-javascript-affected-adapter-v0.md` | TJAA-V0 | KEEP | active | - | 1/1 paths; 101 hits | SPEC-HEALTH KEEP; serves PROD-TEST-011. |
| `unplanned-read-events-v0.md` | URE-V0 | OUT-OF-1.0 | archive in place | B1 | 1/1 paths; 5 hits | B1 removes internal/unplannedread; AGENTS.md invariant 4 is rewritten (design.md outline). |
| `upgrade-checklist-v0.md` | UCL1142-V0 | RETIRE | archive in place | C1 | 1/1 paths; 1 hits | Command absent from source and catalogue. |
| `use-case-conformance-v0.md` | UCV0 | OUT-OF-1.0 | archive in place | B9 | 1/1 paths; 10 hits | Promotion evidence for context jobs; out of 1.0. |
| `verification-planner-observer-v0.md` | VPO-V0 | OUT-OF-1.0 | archive in place | B9 | no paths; 5 hits | Deferred; no implementation path. |
| `verified-absence-frontier-v0.md` | VAF | RETIRE | archive in place | C1 | no paths; 1 hits | Superseded; successor change-frontier-v0 is itself retired. |
| `vscode-extension-v0.md` | VSC-V0 | OUT-OF-1.0 | archive in place | B4 | 2/2 paths; 18 hits | Extension moves to beamfall/corvint-vscode (R33); archived here when B4 removes extensions/vscode. |
| `windows-test-portability-v0.md` | ASD-V0 | MERGE | active | - | 2/2 paths; 179 hits | Decided 2026-11-09 under R42, E8: active and standalone. It owns `make windows-vet`, which owner decision R36 keeps; it is not merged into an archived spec. |
| `work-queue-observation-v0.md` | WQO-V0 | retired | move to beamfall/corvint-tasks | B10 | 4/4 paths; 42 hits | Decision 0594 moved Tasks to beamfall/corvint-tasks, which holds the live copy; deleted here after corvint-tasks PR #4 (RMV-V1-027). |
| `workspace-declaration-v0.md` | WSD-V0 | KEEP | active | - | no paths; 15 hits | SPEC-HEALTH KEEP; serves PROD-DOCS-006. |
