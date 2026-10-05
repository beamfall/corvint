# Typed escalation CLI and reads: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699) and the Gate A decisions the owner
accepted as written on 2026-10-04 (decision 0428). The owner asked for this fourth slice, CLI and
reads, after the writer slice. Contract: `docs/specs/corvint-tasks-escalations-v0.md`.

## Decision

- `ticket escalate` takes `--attempt` and `--claim-receipt` (the receipt sequence or the receipt
  name the claim prints) and no ticket argument. `store.ClaimSource` reads that immutable ADMIT
  receipt and its POST attempt, never the latest attempt projection, and fills ticket, generation,
  holder, acceptance revision and every digest. The caller supplies none of them, and the planner
  re-audits the source under the writer lock. A receipt that is absent, is not a completed
  admission of the attempt, or lacks a matching POST attempt abstains in the CLI as
  `MISSING_ADMISSION_CONTEXT` and writes nothing.
- `ticket answer` takes `--target` as ESC-V0-004 names it; `--request` and
  `--expected-request-revision` go together. The result adds `escalationCode`, the request IDs an
  ambiguous shorthand names, and the committed events, so a shorthand answer reports the request
  it resolved.
- The reads are pure. `list` orders entries by (ticket ID, request ID). Its cursor is the origin
  digest of the previous page's last entry, located among unfiltered entries, so a page resumes
  even after that entry's state changes; an unknown cursor is `MALFORMED`. Blobs are loaded only
  for tickets on the page, and the reducer's validator checks them. A ticket whose material fails
  shows its entries as `UNAVAILABLE` with the code and a warning, and is never repaired.
- `history` walks from the head to the origin. At each step it checks the digest, the queue,
  ticket and escalation identity, the origin binding, a revision decrement of exactly one, and
  for repeats. The chain may not be longer than the entry revision.
- Program is always `UNKNOWN` because no admitted producer mapping exists. A named `--program`
  matches nothing and warns. retryState is `NOT_OBSERVED`. Applicability compares the entry's
  acceptance revision with the record's.

These were agent design choices under the owner's request; none changes an accepted decision.

## Review

The independent Codex review found three minor issues and no blocker. All three are fixed:

- Stored material that the validator refuses was reported as `MALFORMED`. The record has already
  decoded, so every refusal other than `MISSING_EVIDENCE` is now `JOURNAL_FORKED`. The test
  witnesses a rewritten answer event. The other validator codes share the mapping but have no
  witness of their own.
- History rows lacked the question's age and provenance. Each row now carries the entry summary
  under `escalation`: original time, age, applicability and source.
- A bound refusal from the request codec, such as an oversized question, was rewritten to
  `MALFORMED`. Typed codes are now kept.

## Evidence

- `TestIssue502_EscalationCLIWritesAndReads` runs every verb against real claim receipts. It
  covers open via both receipt forms, replay, a mismatched receipt, ambiguous shorthand, a stale
  ticket CAS, exact and shorthand answers, a blocked question, all list filters, the cursor and
  limit bound, show, history with its cursor, read purity of the state directory, a deleted
  origin, an oversized question (`LIMIT_EXCEEDED`), history provenance and a rewritten event
  (`JOURNAL_FORKED`).
- The help inventory test covers the five new verbs.

## NOT_RUN

- The `dispatch status` escalation section, native holds, 501 claim delivery, dispatcher retry
  and the distinct ESC-V0-010 stage are later slices.
- `--text-stdin`, a clock-behind witness, the 1 MiB page cut and a forked-chain history witness.
- A compiled-binary run and the repository-wide gate, under the owner's focused-test preference.

## Rollback

Revert this change. It adds verbs and one read-only store helper, and no byte format; stored
questions and answers stay readable by the writer slice.
