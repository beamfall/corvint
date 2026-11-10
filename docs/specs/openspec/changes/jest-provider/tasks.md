# Tasks: jest-provider

Build in this order. Each task names its requirement IDs and the proof that closes it.

1. Create `docs/specs/jest-runner-v1.md` with JST-V1-001 to JST-V1-010 from the spec delta as
   proposed, and regenerate `docs/specs/REQUIREMENTS.tsv`. File the G23 Jest ticket.
   Requirements: JST-V1-001 to JST-V1-010. Proof: the spec toolchain check passes; ticket ID cited.
2. Add Jest to the built-in runner set with its default command and instances.
   Requirements: JST-V1-001, JST-V1-006, JST-V1-007. Proof: unit tests for the default command,
   two roots and `full:jest`.
3. Add Jest discovery to the inventory.
   Requirements: JST-V1-002. Proof: inventory fixture listing Jest tests by canonical ID.
4. Add selection arguments and `exact`.
   Requirements: JST-V1-003. Proof: tests for escaping, anchoring and duplicate names.
5. Add report reading with outcomes, durations and NOT_RUN.
   Requirements: JST-V1-004, JST-V1-005. Proof: report fixtures, including a missing test and an
   unparsable report.
6. Add the unsupported-setup refusal.
   Requirements: JST-V1-008. Proof: fixtures for an old version and a custom reporter.
7. Run the end-to-end fixture through the four consumers and add the release binary check.
   Requirements: JST-V1-009, JST-V1-010. Proof: the end-to-end result; the binary list check.
8. Record the trace and the delivery state in the spec.
   Requirements: all. Proof: the trace check maps every JST-V1 ID to a test or measured record.
