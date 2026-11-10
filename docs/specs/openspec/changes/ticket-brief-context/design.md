# Design: `ticket-brief-context`

## Decisions

- The brief ships in 1.0 non-blocking with no claim of measured benefit; the keep-or-remove
  measurement runs in 1.0.x (owner decisions R28 and R29, 2026-11-09; review docs F10).
- Measurement runs use the owner's subscription logins on the owner's machines; no API spend is
  needed (owner decisions R39 and R40, 2026-11-09).
- Tickets are read through `corvint-tasks ticket show --json`, keeping the products loosely coupled
  (answer 13).
- MCP exposure follows the table of change `mcp-tool-table` (owner decision R30; TBC-V1-008).

## Failure modes

- Ticket not found or store unreadable: typed refusal, exit 2.
- A `requirementRefs` ID absent from `REQUIREMENTS.tsv`: listed as `dangling-requirement`, not
  dropped.
- No declared paths: the plan item is `NOT_OBSERVED`; no paths are guessed.
- No flow cited: the flow item is `NOT_OBSERVED`.
- `corvint-tasks` not installed: typed refusal naming it, exit 2.
- Measurement NOT_RUN at the 1.0 release candidate: recorded as NOT_RUN; the brief ships
  non-blocking with no benefit claim (TBC-V1-006).

## Acceptance evidence

- Fixture store with tickets covering each item and each missing-link case; Go tests per
  requirement.
- At 1.0: the redesigned verb on Corvint labelled non-blocking, and the committed task list and hash
  for the 1.0.x measurement. In 1.0.x: both arms' transcripts and the result against the threshold
  (TBC-V1-006).
- NOT_RUN at the time of writing: everything.

## Rollback

Revert the change's commits; the ranked packet returns from the reverted diff. If the 1.0.x
measurement fails, the removal of the brief is itself the rollback. IDs are retired, not reused.

## Open questions for the owner

- Set the keep threshold (proposed: at least 20% fewer median tool calls, no lower success rate).
- Settled by owner decisions R39 and R40 (2026-11-09): the measurement uses the owner's
  subscription logins; no API spend. Earlier text: "The measurement needs agent runs with API spend;
  G21 records that spend is not yet arranged."
- Amend AGENTS.md invariant 4 to drop the `--learn-slot-weights` exception once `slot_weights.go`
  is removed?
