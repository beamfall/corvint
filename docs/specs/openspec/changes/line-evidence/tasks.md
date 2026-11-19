# Tasks: line-evidence

Build in this order. Each task names its requirement IDs and the proof that closes it.

- [x] 1 Create `docs/specs/line-evidence-v1.md` with LEV-V1-001 to LEV-V1-015 from the spec delta as
   proposed, and regenerate `docs/specs/REQUIREMENTS.tsv`.
   Requirements: LEV-V1-001 to LEV-V1-015. Proof: the spec toolchain check passes.
   Executed: `make -s spec-requirements-check` passed; accepted clauses remain stable.
- [ ] 2 Add the tier enum, `exact` flag and one-line reason to the plan output.
   Requirements: LEV-V1-009, LEV-V1-010. Proof: plan golden files showing each tier and reason.
   Executed: `TestLEVV1009TierReasonGolden` passed; four tiers and one-line reasons.
   PARTIAL: default absent-store annotations conflict with frozen baseline assertions; the
   overlay is limited to initialized stores and explicit watch indexes. No frozen golden changed.
- [ ] 3 Add the shared reader with freshness and widening reasons.
   Requirements: LEV-V1-003, LEV-V1-004. Proof: unit tests for absent, stale per cause, damaged
   section and unqualified.
   Executed: `TestLEVV1003Freshness`, `TestLEVV1004AbsentDamagedReadOnly` passed.
- [ ] 4 Add `corvint line-evidence refresh` writing the private store, with Go first.
   Requirements: LEV-V1-001, LEV-V1-002, LEV-V1-005. Proof: Go fixture refresh; second refresh on
   the same tree gives an equal index digest.
   Executed: `TestLEVV1001RefreshAndConsumers` passed; two actual Go refreshes matched bytes.
- [ ] 5 Add Vitest and Jest collection through `internal/jslinecov`, and RSpec and Minitest through
   `internal/rubylinecov`.
   Requirements: LEV-V1-005. Proof: one fixture per runner where a one-line change selects the
   covering tests at tier `file` with `line-evidence-unqualified`.
   PARTIAL: Vitest/Jest native fixtures passed; RSpec/Minitest implementations and fixtures
   exist, but native proof is NOT_RUN: Ruby and RSpec are unavailable on this host.
- [ ] 6 Add pytest file-level selection with its label.
   Requirements: LEV-V1-006. Proof: pytest fixture plan shows tier `file`.
   Executed: `TestLEVV1006PytestFileTier` passed.
- [ ] 7 Add the Playwright rules and the opt-in experimental E2E profile.
   Requirements: LEV-V1-007, LEV-V1-008. Proof: fixture where line evidence adds and ranks but never
   removes a browser test.
   Executed: `TestLEVV1007PlaywrightOnlyAddsAndRanks`, `TestLEVV1008ExperimentalProfile` passed.
- [ ] 8 Wire the reader into `corvint affected`, `corvint test`, `corvint watch` and MCP; drop the
   Go-only watch refusal. Requirements: LEV-V1-012. Proof: watch fixture for Vitest; read-only test
   for each consumer.
   Executed: `TestLEVV1001RefreshAndConsumers`, `TestLEVV1012VitestWatch` passed.
- [ ] 9 Add argument files and the `argv-limit` widening.
   Requirements: LEV-V1-014. Proof: a fixture selecting more names than the limit.
   Executed: `TestLEVV1014GoNamesWidenToPackages`, `TestLEVV1014FileOverflowWidens`,
   `TestLEVV1014GoSliceArgumentLimit`, `TestLEVV1014PytestArgumentFile` passed.
- [ ] 9a Apply the working-tree rule to one-shot `affected`, `test` and the MCP plan tool.
    Requirements: LEV-V1-015. Proof: staged-edit and untracked-file fixtures on each surface.
   Executed: `TestLEVV1001RefreshAndConsumers` passed on staged and untracked edits.
- [ ] 10 Add the Action refresh and cache step (with change `github-action`).
    Requirements: LEV-V1-013. Proof: fixture workflow showing a cache hit narrows and a miss widens.
- [ ] 11 Enable tier `line` per runner as each FIH result lands. Requirements: LEV-V1-011. Proof:
    the FIH record per runner (owner decision R19) cited in the spec.
- [ ] 12 Record the trace and the delivery state in the spec.
    Requirements: all. Proof: the trace check maps every LEV-V1 ID to a test or measured record.
   Recorded: canonical spec trace maps all 15 IDs; delivery remains experimental.
   Action/FIH work is NOT_PRODUCED; live Ruby proof is NOT_RUN.

Tasks after 2 retain their proof above but remain unticked to preserve the sequential task gate.
Task 5 needs the native Ruby fixtures on a host with Ruby and RSpec. Tasks 10 and 11 are outside
this lane, so task 12 records only its trace portion and does not claim complete delivery.
