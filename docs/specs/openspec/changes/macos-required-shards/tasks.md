# Tasks: `macos-required-shards`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [ ] 1.1 File the ticket and reserve prefix `MRS-V1` (001 to 007); collect the open hosted-macOS
      flake tickets and link them. Requirements: all, MRS-V1-004. Proof: `corvint-tasks ticket show`
      with `requirementRefs` and the linked flake IDs.
- [ ] 1.2 First commit: create `docs/specs/macos-required-shards-v1.md` from the spec delta, its
      README row and the regenerated index, naming the MCJ-V0 and CIN-V0 rows it supersedes for the
      aggregate. Requirements: MRS-V1-001 to 007. Proof: `make spec-requirements-check` passes.
- [ ] 2.1 Release workflow: exhaustive macOS `go test` and `go vet` in both modules on the candidate
      and RC SHA. Requirements: MRS-V1-001. Proof: a workflow run on a candidate tag.
- [ ] 2.2 Flake filing rule and the flake ticket sweep. Requirements: MRS-V1-004. Proof: every open
      flake ticket at `v1-0` P1.
- [ ] 2.3 `.github` PR: `merge_group` trigger, no `continue-on-error`, no DEFER skip, docs-only
      success with reason, advisory timing. Requirements: MRS-V1-002, MRS-V1-003. Proof: a PR run and
      a merge-queue run where the aggregate reports.
- [ ] 2.4 Owner adds `go-macos` to the required checks on `main`. Requirements: MRS-V1-002. Proof:
      `gh api` ruleset read-back.
- [ ] 2.5 AGENTS.md windows-vet rule for removal batches, and the Windows scope sentence.
      Requirements: MRS-V1-005, MRS-V1-006. Proof: diff read-back.
- [ ] 2.6 RC evidence row. Requirements: MRS-V1-007. Proof: the acceptance record lists the macOS run
      on the RC SHA.
- [ ] 3.1 Build-log entry; complete the ticket through the Tasks gate. Requirements: all. Proof:
      `corvint-tasks ticket show` and `receipt audit`.
