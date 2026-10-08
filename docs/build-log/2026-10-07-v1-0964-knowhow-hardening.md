# 2026-10-07: V1-0964 know-how notes hardening

## Intent

Ticket V1-0964. Decision 0444 answered owner questions 5 and 6 of the know-how notes profile
(`docs/specs/corvint-tasks-know-how-notes-v0.md`, from V1-0955 / GitHub #655) with yes. Three gaps
remained:
- `attempt` and `generation` on a `KNOWHOW_ADD` were stored as asserted;
- a secret hit was VALIDATION_FAILED/MALFORMED with the detail prefix `KNOWHOW_SECRET_DETECTED:`;
- the archive round trip, competing-writer, redo, UNAVAILABLE-delivery and commit-race witnesses
  were NOT_RUN.

## Change

- Proposed KHN-V0-008 to KHN-V0-015 (pending owner acceptance).
- Two new §11 detail codes, bringing the closed set to 76. Both are classified as not retryable
  until the input changes, and neither is ever persisted.
  - `PROVENANCE_UNVERIFIED` (KHN-V0-008).
  - `SECRET_DETECTED` (KHN-V0-010).
- The amendment note sits in the KHN spec, with a pointer and a retry-table entry in the agent
  lease contract.
- `mutation.CheckKnowHowProvenance(ledger, home, attempt, generation)` is the one reusable check.
  - The attempt must be in the audited inventory and belong to the home ticket.
  - The generation must be current or a recorded prior generation.
  - A generation without an attempt refuses, and so does a nil (unobserved) ledger.
  - The detail never echoes the asserted values.
  - Liveness is deliberately left to the caller through `AttemptProvenance.Live`.
- Wiring (KHN-V0-009). For an add that names an attempt or a generation:
  - the writer fast route declines;
  - `reviewAudit` adds every `attempts/` path;
  - `transaction.Model` loads the attempts;
  - the Mutate context carries `KnowHowAttempts`.
  The check runs in `knowHowStep` after the secret screen, and only on fresh writes.
- Compatibility (KHN-V0-011). Every `SECRET_DETECTED` detail keeps the prefix as a deprecated
  alias for one transition window. The spec's compatibility table states the effect on each
  reader:
  - prefix matchers keep working;
  - `code == MALFORMED` matchers must switch;
  - older strict code-set readers fail closed;
  - stored bytes are unchanged.

## Witnesses

All witnesses are deterministic; none uses a sleep or timing race.
- Archive (`TestKHNV0012_ArchiveRoundTripKeepsKnowHow`): export and verify. The archived record is
  byte-identical to its projection, and its ADD and RETRACT entries decode.
- Competing writers (`TestKHNV0013_CompetingWritersOneWinner`).
  - The writer lock serializes the mutation itself, so the only overlap window is composition
    before the lock. The test forces that overlap in a fixed order.
  - The loser gets REVISION_CONFLICT twice with the store digest unchanged.
  - Once rebuilt, it appends note 2, and note 1 is unchanged.
  - A goroutine race test would prove nothing more and would be timing-dependent.
- Redo (`TestKHNV0013_RedoBindsAPendingKnowHowReceipt`): head and projection are restored behind a
  linked receipt. The next writer redoes it, the note exists exactly once, and the request
  replays.
- UNAVAILABLE delivery (`TestKHNV0014_UnreadableInventoryDeliversUnavailable`).
  - An unparseable intent ticket file makes the inventory load fail while the claim replay still
    succeeds.
  - The result is UNAVAILABLE with code MALFORMED, null notes, the trust label and a warning.
  - After the file is removed, the note is delivered.
- Commit race (`TestKHNV0015_CommitRaceResolvesOneCommit`).
  - A writer wrapper on the existing `askAtCommit` seam commits a change between the commit answer
    and the path questions.
  - The pin returns the old commit and the old blob, while HEAD has moved.
  - It passed with `-count=3`.

## Decision 0397

No amendment. The V1-0955 addendum limits `internal/secretscreen` importers to
`internal/tasks/mutation/know_how.go` and its test. The new provenance file does not import it,
and `TestImportDirection` still passes.

## Conflict points with V1-0987 (WORKER grant)

Not implemented here. A WORKER path should call `CheckKnowHowProvenance` and then test
`ledger[attempt].Live` and the holder itself. The likely textual conflicts are:
- `mutation/know_how.go` (`knowHowStep`) and the `mutation.Context` struct;
- `wire/codes.go`, `wire/retry.go` and the code-count test (76) if V1-0987 adds codes;
- `intent/policy.go` grants;
- `cli/know_how.go` flags and help;
- the `store/writer_route.go` and `store/external_review.go` predicates, and
  `transaction/model.go` `validateInput`;
- the KHN spec (requirement IDs, non-goals, traceability), `REQUIREMENTS.tsv` and the specs README
  row;
- the agent lease contract's retry table and amendment preface.

## Not run

- Durable two-process qualification.
- An older binary reading a 76-code result.
- Archive import into a fresh store.
- The whole `internal/tasks/store` package. Only focused `-run` selections were run, because the
  whole package takes about 20 minutes.
- Corvint dogfood ran in degraded fallback mode (`corvint-event-rejected:dogfood-event-deadline`).
