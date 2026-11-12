# Tasks: `spec-set-cleanup`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order. A task marked
"with Bn" is applied in the same merge commit as `source-removal` batch Bn (SSC-V1-006); it waits
for that batch's gate and is not folded into another batch. Every batch adds one build-log entry
(owner decision R34 (2026-11-09)). Target: tasks 1.1 to 4.2 and the B1 to B5 tasks by 2026-12-01;
the B6, B7, B8b, B10 and B11 tasks land with their gated batches.

## Tasks

- [ ] 1.1 File the ticket and reserve prefix `SSC-V1` (001 to 008). Requirements: all. Proof:
      `corvint-tasks ticket show` with `requirementRefs` listing SSC-V1-001 to 008.
- [ ] 1.2 First commit: create `docs/specs/spec-set-cleanup-v1.md` from the spec delta, its README
      row and the regenerated index. Requirements: SSC-V1-001 to 008. Proof:
      `make spec-requirements-check` passes.
- [ ] 2.1 C1: rewrite `docs/SPEC-DRIVEN-DEVELOPMENT.md` to the outline in `design.md`, and point
      `docs/specs/openspec/README.md`, `docs/DOGFOOD.md` and
      `docs/decisions/PROPOSED-openspec-workflow.md` at it. Requirements: SSC-V1-001. Proof: a
      search of active documents finds the stage descriptions only in that file.
- [ ] 2.2 C1: rewrite `AGENTS.md` sections 4 and 7 to the outline in `design.md` (the B6 lines
      wait for task 3.6). Do not edit the product invariants (section 3): ACR-V1-012 in change
      `acceptance-record` owns every edit of invariants 3, 4, 5, 7 and 8. Requirements: SSC-V1-001.
      Proof: diff review shows no change inside the product invariants; reference check report.
- [ ] 2.3 C1: apply the C1 rows of `docs-verdicts.md` (rewrite `docs/README.md`, `docs/PRODUCT.md`,
      the C1 rows of `docs/AGENT-ROUTES.md`; archive `BEAMFALL-INTEGRATION.md`, `PORTFOLIO-0.6.md`,
      `SPEC-TOOLCHAIN-INTEGRATION.md`, `TECHNICAL-BRAIN.md`, `plans/`, `lrf-0.schema.json`; delete
      `V1-WORKFLOWS.md` with its readers edited and `make documentation-paths` rerun).
      Requirements: SSC-V1-001, SSC-V1-002. Proof: `make documentation-paths-check` passes.
- [ ] 2.4 C1: write the `Health:` line on every spec and the archive lines on the C1 rows of
      `specs-verdicts.md`. Requirements: SSC-V1-002, SSC-V1-003. Proof: the `openspec-check` health
      rule passes; `make spec-requirements-check` passes; `REQUIREMENTS.tsv` row count unchanged.
- [ ] 2.5 C1: add the reference check to `doc-gates` in report mode, with passing and failing
      fixtures. Requirements: SSC-V1-004. Proof: Go or shell tests over the fixtures; its report on
      the repository.
- [ ] 2.6 After the owner accepts SSC-V1-005: split the generated index and update its readers.
      Requirements: SSC-V1-005. Proof: generator test; `cmd/corvint/core_freeze_test.go`,
      `internal/docmaintain/maturity_test.go` and the `internal/gateselect` tests pass.
- [ ] 3.1 With B1: archive the three B1 spec rows; delete `docs/worklist.json`. Requirements:
      SSC-V1-002, SSC-V1-006. Proof: health rule and reference check on the B1 merge commit.
- [ ] 3.2 With B2: archive the B2 spec rows and `docs/TEST-CLAIM-QUALIFICATION.md`; settle
      `tcq-0.schema.json`; edit `docs/CAPABILITIES.md`. Requirements: SSC-V1-002, SSC-V1-006.
      Proof: as 3.1.
- [ ] 3.3 With B3: add the RSP-V0-026 to 032 status line to `runner-selection-plugins-v0.md`.
      Requirements: SSC-V1-002, SSC-V1-006. Proof: as 3.1.
- [ ] 3.4 With B4: archive the five B4 spec rows and `docs/EXTENSION-ECOSYSTEM.md`; delete
      `docs/EXTERNAL-EVIDENCE-PROVIDERS.md`. Requirements: SSC-V1-002, SSC-V1-006. Proof: as 3.1
      plus `make documentation-paths-check`.
- [ ] 3.5 With B5: archive the fourteen B5 spec rows; rewrite `native-shell-affected-test-v0.md`
      to shell only and `language-plugins-v0.md` as their rows say; edit `docs/CAPABILITIES.md`. Requirements: SSC-V1-002, SSC-V1-006. Proof: as 3.1.
- [ ] 3.6 With B6: archive the ten B6 spec rows and `docs/CHANGE-EVIDENCE-MAP.md`; delete
      `docs/CHANGE-FLOW.md` and the two CEM schemas; rewrite `ARCHITECTURE.md`, `GLOSSARY.md`,
      `SELF-DEVELOPMENT.md`, the B6 lines of `AGENTS.md` and `AGENT-ROUTES.md`, the OCM section of
      `protocol/integrations/openspec/README.md` and line 50 of the Codex `SKILL.md`.
      Requirements: SSC-V1-001, SSC-V1-002, SSC-V1-006. Proof: as 3.4.
- [ ] 3.7 With B7: archive the seven B7 spec rows; re-decide `index-snapshot-v0.md`. Requirements:
      SSC-V1-002, SSC-V1-006. Proof: as 3.1.
- [ ] 3.8 With B8a: archive `eol-policy-gate-v0.md`. Requirements: SSC-V1-002, SSC-V1-006. Proof:
      as 3.1.
- [ ] 3.9 With B8b: archive the ten B8b spec rows (`greptile-gate-v0.md` is already retired,
      decision 0598); reduce `docs/LANE-ORCHESTRATION.md` to a
      redirect; rewrite the lane rows of `AGENT-ROUTES.md` and `SELF-DEVELOPMENT.md`.
      Requirements: SSC-V1-001, SSC-V1-002, SSC-V1-006. Proof: as 3.1.
- [ ] 3.10 With B9: archive the twelve B9 sweep rows. Requirements: SSC-V1-002, SSC-V1-006. Proof:
      as 3.1.
- [ ] 3.11 With B10, after `beamfall/corvint-tasks` PR #4 merges: delete the 37 Tasks specs, point
      live links at the corvint-tasks repository, and record their prefixes as taken. Requirements:
      SSC-V1-003, SSC-V1-007. Proof: `make spec-requirements-check`; the `openspec-check` ID rule
      refuses one of the moved prefixes in a fixture.
- [ ] 3.12 With B11: archive or keep `gate-ledger-v0.md` and `go-archive-gate-v0.md` as the action
      cut-over decides. Requirements: SSC-V1-002, SSC-V1-006. Proof: as 3.1.
- [ ] 4.1 C2: fold the eight MERGE rows into their named targets, then archive the sources.
      Requirements: SSC-V1-002, SSC-V1-003. Proof: each target spec carries the folded
      requirements; `make spec-requirements-check` passes.
- [ ] 4.2 C3: archive `docs/specs/SPEC-HEALTH-2026-11-09.md`; switch the reference check to
      blocking. Requirements: SSC-V1-004. Proof: `make doc-gates` passes with the check blocking.
- [ ] 4.3 Revert drill on a scratch branch: `git revert -m 1` of the C1 merge. Requirements:
      SSC-V1-008. Proof: the previous headers return and `make spec-requirements-check` passes.
- [ ] 5.1 Archive this change folder to `changes/archive/YYYY-MM-DD-spec-set-cleanup/` once every
      task above is ticked and SSC-V1-001 to 008 are in `docs/specs/spec-set-cleanup-v1.md`.
      Requirements: all. Proof: `openspec-check` archive rule passes.
