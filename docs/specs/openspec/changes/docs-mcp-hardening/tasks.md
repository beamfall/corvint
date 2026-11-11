# Tasks: `docs-mcp-hardening`

Build tasks in order. Each names its requirement IDs and its proof.

- [x] 1 File the ticket in `.taskman/` (milestone `v1-0`), reserve prefix `DMF-V1` and set
   `requirementRefs` to DMF-V1-001 to DMF-V1-011 and DMF-V1-013 (R42 B1 to B5 decide the open
   questions). Proof: `corvint-tasks ticket show` lists the twelve IDs. V1-5409 filed; DMF-V1 reserved.
- [x] 2 First commit: create `docs/specs/docs-mcp-hardening-v1.md`, its `docs/specs/README.md` row
   and the regenerated index (DMF-V1-001 to 011 and 013). Proof: `make spec-requirements` output
   lists the twelve IDs.
- [x] 3 Waived claims render as WAIVED in generation and check (DMF-V1-001). Proof: fixture test
   with a waived PROVEN claim shows the mark in the page and the sidecar; flow-doc goldens
   re-recorded.
- [x] 4 `docs maintain --check` (DMF-V1-003). Proof: fixture test, stale block exits 1, fresh exits
   0, `--preview` still exits 0.
- [x] 5 Citation check changes and `--json` (DMF-V1-004). Proof: fixture tests for a missing
   top-level file, a fenced citation and an unpinned citation; JSON output test.
- [x] 6 `worktree` member on every check report (DMF-V1-005). Proof: fixture test with a dirty
   tracked file for each check.
- [x] 7 Per-claim byte-failure detail (DMF-V1-006). Proof: fixture test where one claim changes
      state.
- [x] 8 Output directory in the manifest; no clean result without comparing (DMF-V1-007). Proof:
   fixture tests with a recorded directory, a missing directory and no directory.
- [x] 9 `corvint-docs docs check` (DMF-V1-002), after tasks 4 to 8. Proof: fixture tests for
   all-pass, one stale part and nothing found; run on the Corvint repository with the JSON kept.
- [ ] 10 Tool and property descriptions for the 19 remaining docs tools (DMF-V1-008; R42 B9). Proof:
    `tools/list` golden re-recorded; a test that fails on any two docs tools with the same
    description or a property with none. No tool is removed without a strict-subset fixture test.
- [ ] 11 Remove `docs_recommend_tests` and point descriptions to `corvint mcp` (DMF-V1-009; R42 B2).
    Proof: `tools/list` golden without the tool and a test that descriptions name `corvint mcp`.
- [ ] 12 Error shape (DMF-V1-010). Proof: tests for a busy server, a cancelled call, a child failure
    with stderr and an invalid argument, each checking `code`, `message`, `retryable` and `next`.
- [ ] 13 Artifact reload (DMF-V1-011). Proof: test that rebuilds the artifact between two calls and
    gets an answer with the new digest; test that a corrupt rebuild is refused with a reason.
- [ ] 14 `docs consume` stale message (DMF-V1-013; R42 B5). Proof: fixture test where the draft's
    commit moved, checking the message names both revisions and the re-draft command. (The `how` gap
    task moved to `how-rewrite` as HOW-V1-012.)
- [ ] 15 Independent review of the whole change, then the affected-plan tests for the touched
    packages. Proof: review report and test receipts kept with the ticket.
