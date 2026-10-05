# Escalation answer claim delivery: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699), ESC-V0-005, and the Gate A decisions
the owner accepted as written on 2026-10-04 (decision 0428). The owner asked for this fifth slice,
501 claim delivery, after the CLI and reads slice. Contract:
`docs/specs/corvint-tasks-escalations-v0.md`.

## Decision

- Admission copies the audited record's ANSWERED entries at the record's acceptance revision into
  the optional `escalationAnswers` attempt key, as sorted (request ID, origin digest, head digest)
  triples. It reads the same record as `TicketRecordSha256` and the operator-note pin. OPEN,
  SUPERSEDED and stale-acceptance entries are never pinned. A no-answer attempt omits the key and
  keeps its exact bytes. The decoder is closed: 1 to 64 entries, request IDs strictly increasing,
  and an empty array is refused.
- Delivery reads only the claim receipt's POST attempt, as the operator note does, and resolves its
  pins against the evidence store, never the current ticket. A later answer therefore moves neither
  the claim nor its exact replay, and claim-next takes its own snapshot.
- `ResolveEscalationAnswers` is pure and shares one encoder and the 256 KiB guidance bound with the
  reducer's answer selection. It checks that each origin is the request's OPEN event and that each
  head is that request's ANSWER, bound to the origin by digest and source. Missing material is
  `MISSING_EVIDENCE`; any other refusal is stored material that disagrees with the pin and renders
  as `JOURNAL_FORKED`.
- The claim result always carries `escalationAnswers` beside `operatorNote`. It is `CURRENT` with
  the answer array (empty when nothing was pinned), or `UNAVAILABLE` with its code and the pinned
  references, plus a warning to replay the exact claim and never claim again. Delivery consumes
  and acknowledges nothing.
- The guidance bound needs no blob read under the claim lock. Every escalation write re-selects the
  same-acceptance answers under `GUIDANCE_CAPACITY`, and only escalation writes add an answer; an
  acceptance change can only remove answers from the set. Full claim output stays under 1 MiB by
  the same argument: 256 KiB of guidance, a note event of at most 64 KiB, and the claim base.

These were agent design choices under the owner's request; none changes an accepted decision.

## Evidence

- `TestESCV0005_ClaimDeliversTheAnswersPinnedByItsAdmission` runs the native writer and lease
  store: a no-answer claim without the pin, two questions with one answered, a claim that delivers
  only the answer, a later answer that changes neither the live claim nor its exact replay, and a
  claim-next that delivers both.
- `TestESCV0005_ClaimPinsAndResolvesAnswers` covers admission filtering and resolver parity with
  the reducer, plus missing, swapped, cross-bound and wrong-ticket material.
- `TestESCV0005_AttemptPinsTheAdmittedAnswers` covers codec round trip and refusals.
  `TestESCV0005_ClaimResultCarriesPinnedAnswers` covers rendering.

## NOT_RUN

- A store-level witness of a missing pinned blob. Deleting a referenced event makes the replay's
  receipt audit refuse first, as it does for operator notes, so the `UNAVAILABLE` path is witnessed
  only at the resolver and renderer.
- A 1 MiB output witness, a compiled-binary claim, and the repository-wide gate, under the owner's
  focused-test preference.
- Native holds, dispatcher retry, the distinct ESC-V0-010 stage and the `dispatch status` section.

## Rollback

Before any claim pins an answer, revert this change. After that, journal audit decodes pinned
attempt POSTs, so keep the attempt codec and remove only the pinning and the delivery.
