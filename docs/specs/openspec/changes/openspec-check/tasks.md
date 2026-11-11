# Tasks: `openspec-check`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

Ticket V1-5399 exists (filed by the orchestrator on 2026-11-11); task 1.1 stays unticked until the
orchestrator sets its `requirementRefs`.

## Tasks

- [ ] 1.1 File the ticket and reserve prefix `OSC-V1` (001 to 012). Requirements: all. Proof:
      `corvint-tasks ticket show` with `requirementRefs` listing OSC-V1-001 to 012.
- [x] 1.2 First commit: create `docs/specs/openspec-check-v1.md` from the spec delta, its README row
      and the regenerated index; amend PRS-V1-018 in the same commit. Requirements: OSC-V1-001 to
      012, PRS-V1-018. Proof: `make spec-requirements-check` passes.
- [ ] 2.1 Layout and requirement-ID rules with fixtures. Requirements: OSC-V1-001, OSC-V1-002.
      Proof: Go tests over passing and failing fixture folders.
- [ ] 2.2 Commit binding by trailers and the propose-stage rule. Requirements: OSC-V1-003,
      OSC-V1-004. Proof: a fixture history where code precedes its folder fails; one where the folder
      is in the parent passes.
- [ ] 2.3 Apply-stage rules: tick order and canonical spec update. Requirements: OSC-V1-005. Proof:
      fixtures with an out-of-order tick and with a ticked task whose ID is missing from the
      canonical spec both fail; a ticked task for a spec in another repository is reported as
      `external-spec-not-verified` and does not fail.
- [ ] 2.4 Archive-stage rules. Requirements: OSC-V1-006. Proof: fixtures for a finished folder left
      under `changes/`, for an edited archived folder and for an archived folder with an unticked
      task all fail; a folder moved by `openspec archive <name> --skip-specs` passes.
- [ ] 2.5 Boundary rules. Requirements: OSC-V1-007. Proof: fixtures with a root `openspec/`
      directory, with no `config.yaml` and with a `docs/specs/openspec/specs/` tree all fail.
- [ ] 2.6 Health class header line, generator projection and check. Requirements: OSC-V1-008.
      Proof: generator test; a spec without the line fails.
- [ ] 2.7 Acceptance link to the auto-accept log. Requirements: OSC-V1-009. Proof: a proposal marked
      accepted with no log entry fails.
- [ ] 2.8 Carry-over trace for PRS-V1-018 (c) and (d). Requirements: OSC-V1-010. Proof: traceability
      rows to SRG-V0-009, 010 and 012 and their existing tests.
- [ ] 2.9 Make target, `doc-gates` and `focused-docs` wiring, baseline file, history refusal.
      Requirements: OSC-V1-011. Proof: a shallow-clone run refuses with `history-unavailable`.
- [ ] 2.10 Make target `openspec-validate` with the pinned OpenSpec command version and telemetry
      off. Requirements: OSC-V1-012. Proof: the target passes on the repository, fails on a fixture
      folder with a requirement that has no scenario, and reports NOT_RUN when the command is absent
      outside CI.
- [ ] 2.11 CI step that installs the pinned OpenSpec command and runs `make openspec-validate`, in
      its own `.github` pull request. Requirements: OSC-V1-012. Proof: the job fails when the
      command is absent or at another version.
- [ ] 3.1 Run on the repository once the health class lines are written; shrink the baseline.
      Requirements: all. Proof: check output.
- [ ] 3.2 Build-log entry; complete the ticket through the Tasks gate. Requirements: all. Proof:
      `corvint-tasks ticket show` and `receipt audit`.
