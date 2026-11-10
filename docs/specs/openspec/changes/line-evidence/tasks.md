# Tasks: line-evidence

Build in this order. Each task names its requirement IDs and the proof that closes it.

1. Create `docs/specs/line-evidence-v1.md` with LEV-V1-001 to LEV-V1-014 from the spec delta as
   proposed, and regenerate `docs/specs/REQUIREMENTS.tsv`.
   Requirements: LEV-V1-001 to LEV-V1-014. Proof: the spec toolchain check passes.
2. Add the tier enum, `exact` flag and one-line reason to the plan output.
   Requirements: LEV-V1-009, LEV-V1-010. Proof: plan golden files showing each tier and reason.
3. Add the shared reader with freshness and widening reasons.
   Requirements: LEV-V1-003, LEV-V1-004. Proof: unit tests for absent, stale per cause, damaged
   section and unqualified.
4. Add `corvint line-evidence refresh` writing the private store, with Go first.
   Requirements: LEV-V1-001, LEV-V1-002, LEV-V1-005. Proof: Go fixture refresh; second refresh on
   the same tree gives an equal index digest.
5. Add Vitest and Jest collection through `internal/jslinecov`, and RSpec and Minitest through
   `internal/rubylinecov`.
   Requirements: LEV-V1-005. Proof: one fixture per runner where a one-line change selects the
   covering tests at tier `file` with `line-evidence-unqualified`.
6. Add pytest file-level selection with its label.
   Requirements: LEV-V1-006. Proof: pytest fixture plan shows tier `file`.
7. Add the Playwright rules and the opt-in experimental E2E profile.
   Requirements: LEV-V1-007, LEV-V1-008. Proof: fixture where line evidence adds and ranks but never
   removes a browser test.
8. Wire the reader into `corvint affected`, `corvint test`, `corvint watch` and MCP; drop the Go-only
   watch refusal.
   Requirements: LEV-V1-012. Proof: watch fixture for Vitest; read-only test for each consumer.
9. Add argument files and the `argv-limit` widening.
   Requirements: LEV-V1-014. Proof: a fixture selecting more names than the limit.
9a. Apply the working-tree rule to one-shot `affected`, `test` and the MCP plan tool.
    Requirements: LEV-V1-015. Proof: staged-edit and untracked-file fixtures on each surface.
10. Add the Action refresh and cache step (with change `github-action`).
    Requirements: LEV-V1-013. Proof: fixture workflow showing a cache hit narrows and a miss widens.
11. Enable tier `line` per runner as each FIH result lands.
    Requirements: LEV-V1-011. Proof: the FIH record per runner (owner decision R19) cited in the spec.
12. Record the trace and the delivery state in the spec.
    Requirements: all. Proof: the trace check maps every LEV-V1 ID to a test or measured record.
