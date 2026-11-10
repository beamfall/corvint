# Tasks: ruby-providers

Build after changes `runner-contract`, `canonical-test-id`, `test-inventory` and `run-evidence`;
line-level selection after `line-evidence` (owner decision R38 (2026-11-09) build order). Each task
names its requirement IDs and its proof.

1. File the ticket, reserve prefix `RBR-V1` and set `requirementRefs` to the twelve IDs. Proof:
   `ticket show` read back.
2. First commit: create `ruby-providers-v1.md`, its `docs/specs/README.md` row and the regenerated
   index. Requirements: RBR-V1-001..012. Proof: spec diff.
3. Static Ruby lister for `corvint inventory refresh`, with engine instances. Requirements:
   RBR-V1-001. Proof: Rails application and engine fixtures.
4. Wire the compiled-in Ruby runner into `corvint test` for RSpec, then Minitest (Rails and
   non-Rails paths), at tier `file`. Requirements: RBR-V1-003, 004, 007. Proof: fixture tests.
5. Per-test results, kinds, environment record, advisory Minitest command. Requirements:
   RBR-V1-005, 006, 009, 010. Proof: fixture tests.
6. Line-level selection after the FIH-V1 verdict. Requirements: RBR-V1-002. Proof: both scenarios.
7. Harness adapters. Requirements: RBR-V1-011. Proof: harness call tests.
8. Retire the Ruby provider binary. Requirements: RBR-V1-008, 012. Proof: `binaries.json` diff.
9. Tests under "Acceptance evidence"; build-log entry; complete through the Tasks gate. Proof:
   retained receipts, `receipt audit`.
