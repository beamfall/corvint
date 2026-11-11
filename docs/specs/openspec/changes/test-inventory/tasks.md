# Tasks: test-inventory

Build in this order. Each task names its requirement IDs and the proof that closes it.

- [x] 1 Create `docs/specs/test-inventory-v1.md` with TIV-V1-001 to TIV-V1-012 from the spec delta
   as proposed, and regenerate `docs/specs/REQUIREMENTS.tsv`. Requirements: TIV-V1-001 to
   TIV-V1-012. Proof: the spec toolchain check passes.
- [x] 2 Add the inventory schema, canonical writer and reader with instance keys.
   Requirements: TIV-V1-001, TIV-V1-003, TIV-V1-012. Proof: round-trip unit tests; an unknown added
   field from a newer writer is preserved on read.
- [x] 3 Add the absent, stale, per-file and invalid rules to the reader.
   Requirements: TIV-V1-006, TIV-V1-007, TIV-V1-008, TIV-V1-009. Proof: table-driven tests, one row
   per cause, each with its reason string.
- [x] 4 Add the listers: Go static, Vitest, Jest, Playwright (reusing `playwright-discovery/0`),
   pytest, RSpec and Minitest static. Requirements: TIV-V1-004, TIV-V1-005. Proof: one fixture
   repository per runner; refresh twice writes identical bytes.
- [x] 5 Add `corvint inventory refresh` with drift refusal and the size cap.
   Requirements: TIV-V1-002, TIV-V1-010, TIV-V1-011. Proof: CLI tests for the happy path, drift and
   oversize.
- [x] 6 Make `corvint affected`, `corvint test`, `corvint watch` and the MCP affected tool read the
   inventory and print its reasons.
   Requirements: TIV-V1-002, TIV-V1-006 to TIV-V1-009. Proof: a read-only test showing no repository
   file changes, and a plan for each reason.
- [ ] 7 Add the refresh step to the GitHub Action before planning (with change `github-action`).
   Requirements: TIV-V1-002. Proof: the Action's fixture workflow shows refresh before the plan.
- [x] 8 Record the trace and the delivery state in the spec.
   Requirements: all. Proof: the trace check maps every TIV-V1 ID to a test.
