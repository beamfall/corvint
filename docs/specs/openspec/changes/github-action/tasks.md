# Tasks: `github-action`

Build tasks in order. Each names its requirement IDs and its proof.

1. File the ticket in `.taskman/` and reserve `GHA-V1` 001-018, 020-029, 030-033, 040-043. Proof:
   `corvint-tasks ticket show` lists the 35 IDs.
2. First commit: create `docs/specs/github-action-v1.md` with these definition lines; add status
   lines to `ci-narrowing-v0.md`; regenerate the spec index. Proof: `make spec-requirements` lists
   the 35 IDs.
3. Create `beamfall/corvint-action` with the Apache-2.0 licence (GHA-V1-015). Proof: `LICENSE` and
   README state Apache-2.0.
4. Binary: the plan command and its schema, the `corvint test` runner selector, the plan digest
   check, `CORVINT_SELECTION` parsing with exit 2 on an unknown value, the summary and shadow-report
   commands (GHA-V1-004, 005, 009, 011, 018, 020). Proof: unit tests for each scenario, including an
   unknown plan version, a digest mismatch and an unknown kill-switch value.
5. Release workflow in `beamfall/corvint` as its own `.github` pull request; trim `binaries.json` to
   the three platforms (GHA-V1-040 to 043). Proof: one release candidate published through the
   approval step, with `SHA256SUMS` and the approval event retained.
6. Action: download and per-binary verification, base resolution, fork rule, plan job, run job,
   result job with `verify-shards`, summary, refusals (GHA-V1-001 to 003, 006 to 008, 010, 012 to
   014, 017). Proof: action repository tests on Linux and macOS for every scenario in "Acceptance
   evidence".
7. Plan-equality conformance test in `beamfall/corvint` bound to the GHA-V1-018 schema (GHA-V1-004).
   Proof: the test passes on the fixture repositories.
8. Shadow jobs on Corvint's own CI as a separate `.github` pull request, from the M5a candidate, with
   `--accept-stale-binary` (GHA-V1-030, 033). Proof: build-log entry listing matching pull requests.
9. Preview items: PR comment workflow with the four `workflow_run` rules, docs drift step (advisory,
   opt-in blocking), flow gaps, ticket-gate output, starters with the `merge_group` trigger, cache
   reuse with the save rule and the restore digest, the `always-run` and `working-directory`
   inputs, and the action repository's static checks (GHA-V1-016, 021 to 029). The starters run
   on the seven `beamfall/corvint-qual-*` mirrors once the owner has created them (owner act,
   due 2026-11-27; N4).
   Proof: each preview test passes or is recorded NOT_RUN with that label.
10. Cut-over pull request after 30 matches, or defer to 1.0.x per GHA-V1-032 (GHA-V1-031, 032).
    Read the live ruleset first; replace only the `go-product` required status with the action's
    result job (D15). Proof: `git grep` finds no caller of a removed script or variable, and the
    pull request body records the required-status list before and after.
11. Rewrite the `docs/AUTOMATION.md` adopter recipe, including the threat model of GHA-V1-008;
    build-log entry; complete the ticket through the Tasks gate. Proof: `corvint-tasks receipt audit`
    passes.

Earlier task list, kept for traceability:

- [ ] 1.1 File the ticket; reserve `GHA-V1` 001-017, 020-026, 030-033, 040-043.
- [ ] 1.2 First commit: create `docs/specs/github-action-v1.md` with these definition lines; add
      status lines to `ci-narrowing-v0.md`; regenerate the spec index.
- [ ] 1.3 Create `beamfall/corvint-action` with the licence the owner chose.
- [ ] 2.1 Binary: plan, matrix, summary and shadow-report commands; `CORVINT_SELECTION` in the binary.
- [ ] 2.2 Action: download and verify, base resolution, plan, run, result job, summary, refusals.
- [ ] 2.3 Release workflow in `beamfall/corvint` (separate `.github` pull request); trim
      `binaries.json` per open question 1.
- [ ] 3.1 Action tests and the plan-equality conformance test.
- [ ] 3.2 Shadow jobs on Corvint's own CI (separate `.github` pull request); record matches.
- [ ] 3.3 Preview items: PR comment workflow, docs drift, flow gaps, ticket-gate output, starters,
      cache reuse.
- [ ] 4.1 Cut-over pull request after 30 matches, or defer to 1.0.x per GHA-V1-032.
- [ ] 4.2 Rewrite `docs/AUTOMATION.md` adopter recipe; build-log entry; complete the ticket.
