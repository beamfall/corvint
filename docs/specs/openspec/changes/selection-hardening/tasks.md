# Tasks: selection-hardening

Build SLF-V1-007 after changes `runner-contract` and `source-removal` (owner decision R32). Build
SLF-V1-001 to SLF-V1-005 on the plugins as they stand; build SLF-V1-005 alongside `jest-provider`
and RNC-V1-014 instances. SLF-V1-006 needs `fault-injection-harness`. Each task names its
requirement IDs and its proof.

- [x] 1 File the ticket, reserve prefix `SLF-V1` and set `requirementRefs` to the eight IDs. Proof:
   `ticket show` read back. Done: ticket V1-5401; prefix `SLF-V1` reserved.
- [x] 2 First commit: create `selection-hardening-v1.md`, its `docs/specs/README.md` row and the
   regenerated index. Requirements: SLF-V1-001..008. Proof: spec diff.
- [ ] 3 Python unresolved absolute import frontier, OUTBOUND at the importer, and the
   never-Uncovered rule. Requirements: SLF-V1-002. Proof: flat-layout fixture shows UNKNOWN or the
   importer's test selected, never Uncovered.
- [ ] 4 Python import roots from configuration and layout. Requirements: SLF-V1-001. Proof:
   `pythonpath` and namespace-package fixtures; rule held disabled until task 8.
- [ ] 5 Glob imports and manual mocks, with computed-form frontiers. Requirements: SLF-V1-003, 004.
   Proof: Vite glob fixture, Jest mock fixture, computed-target fixture.
- [ ] 6 Literal Vitest and Jest configuration evaluation, with the five added key-shaped markers
   (R42 A6). Requirements: SLF-V1-005. Proof: literal and computed config fixtures, a `roots:`
   fixture; dirty-config fixture runs in full.
- [ ] 6a Run the finding 11 settle experiment (Go-only edit, TypeScript unknowns) now; if confirmed,
    fix it as a blocker before the Go replay (R42 A8). Proof: the settle record and, if confirmed,
    the fixture test of the fix.
- [ ] 7 One effect and verdict function; affected, test and MCP print it; non-empty hints.
   Requirements: SLF-V1-007. Proof: three-surface parity test.
- [ ] 8 Fault-injection cases per narrowing rule; enable each rule only after its cases pass.
   Requirements: SLF-V1-006. Proof: FIH-V1 report per rule.
- [ ] 9 Split test and driver counts. Requirements: SLF-V1-008. Proof: summary and JSON golden
      tests.
- [ ] 10 Tests under "Acceptance evidence"; NMS-V1 before and after figures; build-log entry;
    complete through the Tasks gate. Proof: retained reports, `receipt audit`.
