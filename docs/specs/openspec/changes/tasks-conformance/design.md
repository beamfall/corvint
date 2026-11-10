# Design: `tasks-conformance`

## Decisions

- Ownership. corvint-tasks owns the schemas, fixture, command list and record. Corvint vendors the
  fixture and the command list, pinned by SHA-256 (TIN-V1-004, TIN-V1-013), and reads the record
  (TIN-V1-001, TIN-V1-002).
- Schema v1. JSON schema files `taskman-ticket/1` and `taskman-receipt-ref/1` under the Tasks
  repository's `protocol/` or `schemas/` directory, with a fixture of valid and invalid documents. A
  receipt reference carries the receipt ID defined by TIN-V1-012 (SHA-256 of the receipt file bytes,
  settled without the owner).
- Supported-command list. A JSON file listing each command, its arguments, the requirement it serves
  (PROD-TASK-001 to 009, I1 or I2), and the schema of its JSON output. A command not on the list is
  printed as experimental by `corvint-tasks help`. The list contains no `mcp` command.
- Record producer. The record is produced by one command at the tag commit. Whether that is
  `corvint-tasks qualify` with a conformance config or a dedicated script in the Tasks repository is
  not decided (Open). The record profile is `taskman-conformance/1` and holds: Tasks version and
  commit; for each of PROD-TASK-001 to 009 the test IDs and outcomes; for each listed command its
  test and a schema check of its output; the concurrency run; the kill -9 run; the audit result; and
  the SHA-256 of the schema files and command list.
- Concurrency. At least 8 separate OS processes claim from one store with overlapping scopes for a
  fixed number of rounds. Pass: every claimed ticket has exactly one live claim at any time, read
  from the store event log after the run.
- Kill -9 recovery. A writer is killed with SIGKILL at injected points during a write. Pass: `receipt
  audit` and `queue status` succeed, and a diff of ticket IDs and content hashes before and after
  shows nothing lost or duplicated other than the interrupted write, which is either fully present
  or absent.
- Trial. The trial in Corvint and `beamfall/core` uses the frozen Tasks RC binary for 21 days and
  records a ticket ID and content hash diff at start and end (TIN-V1-009), with at least 50 ticket
  writes and 5 completions per repository (REL-V1-010). A store format change restarts the clock.
- Release. corvint-tasks builds the three targets with checksums and attestation by the same tag
  workflow shape as the change `release-process`; the Tasks tag precedes the Corvint RC (TIN-V1-003).

## Failure modes

- Fewer than 8 processes, or a double claim: the record marks the check failed; TIN-V1-002 refuses.
- A listed command whose output fails schema v1: the record marks that command failed.
- A schema change without a fixture update: the fixture check fails in both repositories.
- `beamfall/core` has no store at trial start: the trial cannot start; filed as a blocker. Whether
  the untouched-repository scenario may be met by the trial evidence instead is not decided (Open).
- PR #4 still open: the Tasks specs stay retired in place in this repository (TIN-V1-010).

## Acceptance evidence

- The `taskman-conformance/1` record at the Tasks 1.0 tag commit with every check passing.
- The published schema files, fixture and command list, and their SHA-256 values matching Corvint's
  vendored pins.
- The two trial records at day 21.
- The Tasks release artifacts for the three targets with checksums and attestation.

## Rollback

- Schema v1 and the command list are additive files; withdrawing them leaves `/0` formats in place.
- If the conformance pack cannot pass by the freeze, the owner decides in the acceptance record
  whether the Corvint RC waits (G9 default) or demotes the coupling; this change does not decide it.
