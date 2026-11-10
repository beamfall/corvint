# Tasks: narrowing-measurement

Build after changes `per-test-durations`, `run-evidence` and `fault-injection-harness` (owner
decision R38 (2026-11-09) build order for testing). Each task names its requirement IDs and its
proof.

- [ ] 1 File the ticket, reserve prefix `NMS-V1` (`script/requirement-id-reservations.sh reserve`)
   and set `requirementRefs` to the eleven IDs. Proof: `ticket show` read back.
- [ ] 2 First commit: create `narrowing-measurement-v1.md` with the definition lines, its
   `docs/specs/README.md` row and the regenerated index. Requirements: NMS-V1-001..011. Proof: spec
   diff.
- [ ] 3 Kill switch in affected, test, watch, the Action, agent hooks and MCP, through one parser.
   Requirements: NMS-V1-008. Proof: global, per-runner, CLI-typo and hook-typo tests.
- [ ] 4 Measurement tool and report schema, reading durations from `per-test-durations`.
   Requirements: NMS-V1-001, 002, 003, 004, 006, 009. Proof: fixture-corpus tests.
- [ ] 5 Replay source builder and the 200-PR Go feasibility replay; the held-out replay lists are
      built for the four repositories of the seeded draw (owner decision H3 (2026-11-10)).
      Requirements: NMS-V1-005, 010. Proof: replay report.
- [ ] 6 Freeze the bar file before the RC. Requirements: NMS-V1-005. Proof: committed bar file and
      hash.
- [ ] 7 Verdicts on the M5a candidate SHA, confirmed on the RC SHA; demotion record. A regression
      on the RC SHA is reported as a blocking miss and not as a demotion (owner decision H2
      (2026-11-10)). Requirements: NMS-V1-007. Proof: per-runner reports at both SHAs and a
      fixture RC regression reported as a blocking miss.
- [ ] 8 Agreement figure from the soak window. Requirements: NMS-V1-011. Proof: release report
      section.
- [ ] 9 Build-log entry; complete the ticket through the Tasks gate. Proof: `receipt audit`.
