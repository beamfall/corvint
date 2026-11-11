# Design: `ticket-brief-context`

## Decisions

- The brief ships in 1.0 non-blocking with no claim of measured benefit; the keep-or-remove
  measurement runs in 1.0.x (owner decisions R28 and R29, 2026-11-09; review docs F10).
- Measurement runs use the owner's subscription logins on the owner's machines; no API spend is
  needed (owner decisions R39 and R40, 2026-11-09).
- Tickets are read through `corvint-tasks ticket show --json`, keeping the products loosely coupled
  (answer 13).
- MCP exposure follows the table of change `mcp-tool-table` (owner decision R30; TBC-V1-008).

Amendment of 2026-11-10 (drafted by the lane; the owner accepted PROD-INT-032 into 1.0 the same
day):

- The second form is keyed on paths, not on task text. Task text needs matching, which TBC-V1-002
  forbids and which the replay and the 2026-10-26 audit show does not help. A path needs no
  matching: every link below starts from it.
- No form keyed on a change range. `corvint impact` already reports tests, flows, docs, tickets and
  requirements for a range (CRI-V1-001), and two commands for one question would drift.
- The path form reuses the section state and reason of CRI-V1-002 so that agents and the MCP table
  read one shape for both reports. `total` and `truncated` are defined by TBC-V1-010; CRI-V1-008
  says only that a section is bounded.
- The ticket form is not changed. An agent that wants the six sections for a ticket's paths passes
  those paths to the path form. Printing them in the ticket form would change PROD-INT-030, which
  is accepted, so it is an open question below and not part of this amendment.
- The `requirements` section is never `OBSERVED`. The kept links cover only changes that were
  sealed, so a list of them is `INCOMPLETE` and an empty one is `NOT_OBSERVED`. An agent must not
  read an empty section as "no requirement governs this file".
- The link file keeps one entry per record, so no pinned blob or commit is lost by merging. The
  section groups entries by requirement ID and reports the newest by committer date, which is a
  fixed rule and not a ranking.
- Absence is judged per source. A section that still has one of its two sources keeps those items
  and is `INCOMPLETE`, so a missing ticket store does not hide the kept links.
- The coverage check (TBC-V1-013) is a scratch script over existing commands and the sealed
  records. It needs neither the export nor the builder, so it can run first and stop both.
- PROD-INT-030 is left as accepted. The extension is a new requirement, PROD-INT-032. It had a
  `pending-owner` log row (ACR-V1-008) until the owner accepted it on 2026-11-10, so the accepted
  requirement kept its acceptance while the owner decided on the extension.
- The CEM is not revived. Its sealed links are exported once to a plain link file (TBC-V1-012). The
  `cem/1.0` contract, the verb and the ceremony stay removed under G8. Links for work done after
  the export come from completed tickets.
- Kept links are reported at file level with their sealing commit and a `changedSince` flag. Line
  ranges are not re-anchored, because a moved range would be a guess (invariant 2).
- Sections are capped at 64 items in ID or path order. A cap with a stated total is not a ranking.

## Failure modes

- Ticket not found or store unreadable: typed refusal, exit 2.
- A `requirementRefs` ID absent from `REQUIREMENTS.tsv`: listed as `dangling-requirement`, not
  dropped.
- No declared paths: the plan item is `NOT_OBSERVED`; no paths are guessed.
- No flow cited: the flow item is `NOT_OBSERVED`.
- `corvint-tasks` not installed: typed refusal naming it, exit 2.
- Measurement NOT_RUN at the 1.0 release candidate: recorded as NOT_RUN; the brief ships
  non-blocking with no benefit claim (TBC-V1-006).

- Amendment of 2026-11-10: a path outside the repository is a typed refusal, exit 2. `--path` with
  `--base` or `--task` is a typed refusal that names `corvint impact`. An absent optional source
  (Tasks, flows file, `CODEOWNERS`, test evidence, link file) makes its section `NOT_OBSERVED` and
  never fails the command (TBC-V1-011).
- A kept link whose path no longer exists at HEAD is not listed for any other path. A renamed file
  loses its kept links; the export counts such links in its build-log record.
- A kept link whose span holds no requirement definition line carries no requirement ID. At
  `f6be4233c`, 4,927 of the 18,233 specification and decision links are of this kind (build-log
  entry). Such an item names the spec file and span only.
- The affected plan for a path may have scope `UNKNOWN` with many shared readers. The `tests`
  section states that verdict and the cap; it does not list the readers as covering tests.
- The link export meets a malformed or unsealed record: it is skipped and counted in the build-log
  record, never repaired.
- Coverage check below the owner's bar (TBC-V1-013): the path form is not built and TBC-V1-009 to
  TBC-V1-012 and TBC-V1-014 are retired.
- The spec the lane writes (`ticket-brief-v1.md`) names the deleted record directory in TBC-V1-012.
  `source-removal` checks deleted paths for zero hits outside its exclusion list (RMV-V1-003 item
  4), and a live spec is not on that list. The lane words the copied statement as "the sealed
  change records" with the directory named only in the build-log entry of the export.

## Acceptance evidence

- Fixture store with tickets covering each item and each missing-link case; Go tests per
  requirement.
- At 1.0: the redesigned verb on Corvint labelled non-blocking, and the committed task list and hash
  for the 1.0.x measurement. In 1.0.x: both arms' transcripts and the result against the threshold
  (TBC-V1-006).
- NOT_RUN at the time of writing: everything.
- Amendment of 2026-11-10: the link-coverage record on the frozen task set (TBC-V1-013); fixture
  repositories for the six sections, one with no Corvint files at all (TBC-V1-011); the export's
  input commit and output hash (TBC-V1-012); the pilot record or NOT_RUN (TBC-V1-014). Evidence
  that exists today is the replay and the link count in
  `docs/build-log/2026-11-10-context-replay-and-link-coverage.md`; it motivates the amendment and
  proves none of its requirements.

## Rollback

Revert the change's commits; the ranked packet returns from the reverted diff. If the 1.0.x
measurement fails, the removal of the brief is itself the rollback. IDs are retired, not reused.
Amendment of 2026-11-10: the owner accepted PROD-INT-032 on 2026-11-10. If the owner withdraws
it, the amendment's commits are reverted and TBC-V1-009 to TBC-V1-014 are retired. If it fails its
coverage check or the 1.0.x measurement, the path form and `.corvint/requirement-links.json` are
removed with it.

## Open questions for the owner

- Set the keep threshold (proposed: at least 20% fewer median tool calls, no lower success rate).
- Settled by owner decisions R39 and R40 (2026-11-09): the measurement uses the owner's
  subscription logins; no API spend. Earlier text: "The measurement needs agent runs with API spend;
  G21 records that spend is not yet arranged."
- Amend AGENTS.md invariant 4 to drop the `--learn-slot-weights` exception once `slot_weights.go`
  is removed?

### Added 2026-11-10 with the amendment

- Settled 2026-11-10: the owner accepted PROD-INT-032 into 1.0 ("Accept into 1.0"). Earlier text:
  "Accept PROD-INT-032 (the path form, TBC-V1-009 to TBC-V1-014) into 1.0, defer it to 1.0.x, or
  decline it?"
- Accept TBC-V1-012: keep the sealed requirement links in `.corvint/requirement-links.json` when
  `source-removal` deletes `.corvint/changes`? The lane's recommendation is yes. The records stay
  in Git history either way, so the export can also run later. The owner did not answer this
  separately. The lane reads it as covered by the acceptance of PROD-INT-032, whose Notes name the
  link file; the export still waits for the coverage check.
- Set the coverage bar of TBC-V1-013 (proposed: at least half of the tasks have a listed test that
  the closing commits changed or a listed requirement that the ticket cites). Still open after the
  acceptance of 2026-11-10; the check can run and be recorded before the bar is set.
- May the pilot of TBC-V1-014 run before the release candidate on the owner's subscription logins?
  It claims nothing and does not block.
- Should the 1.0.x keep-or-remove measurement also be moved before 1.0 stable? That would change
  PROD-INT-030 and owner decisions R28 and R29, so this amendment does not propose it.
- Should the pilot include one repository other than Corvint? There the flows and requirements
  sections are `NOT_OBSERVED`, so it tests the three sections every repository has.
- Should the ticket form also print the six sections for the ticket's declared paths? That changes
  PROD-INT-030, so it needs its own acceptance; this amendment leaves the ticket form as accepted.
- Should the brief be one row of the `mcp-tool-table` table, taking either `ticket` or `paths`?
  That table's design lists the context brief as open.
- NOT_VERIFIED: whether the commits of a completed ticket can be read from Tasks. A ticket record
  has `progress` entries with `branch` and `head` and a `completion` field (seen in V1-5272), but
  no completed ticket was checked. Where they cannot be read, the ticket-to-path part of the
  `requirements` section is `NOT_OBSERVED` until Tasks records them.
