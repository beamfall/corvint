# Decision 0444 — issue 655 know-how open questions answered

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-07 the owner replied "accept" to the
orchestrator's report that owner questions 5, 6 and 8 of the know-how note spec were still open
as V1-0964 and V1-0962.

## Context

Decision 0443 accepted `KHN-V0-001`..`007` and kept the current behaviour for owner questions 1
to 4 and 7 in `docs/specs/corvint-tasks-know-how-notes-v0.md`. Questions 5, 6 and 8 stayed open,
and their follow-up tickets propose an answer for each.

## Decision

The owner accepts the direction the follow-up tickets propose:

| Question | Answer | Ticket |
|---|---|---|
| 5. Dedicated secret code | Yes. A dedicated Tasks §11 refusal code (for example `SECRET_DETECTED`) replaces the MALFORMED detail prefix. | V1-0964 |
| 6. Verified `attempt`/`generation` | Yes. They are checked against the attempt ledger instead of being writer-asserted. | V1-0964 |
| 8. Core delivery | Yes. Know-how reaches `corvint_query` and Core context packets, labelled untrusted, under a byte cap and with no authority. | V1-0962 |

## Limits

This decision settles direction only. No requirement for these answers exists yet; each ticket
must add numbered requirements to the governing spec, and those come back for owner acceptance.
Current behaviour stays as delivered by V1-0955 until those tickets land.

## Rollback

Revert this decision and restore the "Questions 5 and 6 stay open as V1-0964 and question 8 as
V1-0962" sentence in the spec's owner-questions preface.
