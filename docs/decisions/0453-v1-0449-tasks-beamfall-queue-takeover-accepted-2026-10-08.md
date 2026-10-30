# Decision 0453 — Corvint Tasks takes over the Beamfall production queue

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("approve all, and you can self approve these tickets"), covering native ticket V1-0449.

## Context

Decision 0426 made full Beamfall roadmap takeover by Corvint Tasks a 1.0 requirement and reserved
production authority activation for a final concrete owner approval. V1-0449 supplied the
non-fixture release, staging and reconcile path (`CAL-V0-027` in
`docs/specs/corvint-tasks-agent-leases-v0.md`), whose acceptance text states that it does not switch
Beamfall or establish complete takeover.

## Decision

The owner accepts the decision that Corvint Tasks takes over the Beamfall production queue.

## Limits

This records the decision only. Nothing was executed: no Beamfall workspace, Flow-Proof store or
Tasks store was read for writing, migrated, imported or cut over. The takeover itself (export,
non-fixture store creation, qualified `cutover --execution`, writer fencing of the prior queue and
post-cutover recovery) remains a manual owner step. The CAL-V0 rehearsal and production-migration
obligations, and the decision 0426 qualification requirements, still apply before and during that
step. No spec status changes, and V1-0449 stays open.

## Rollback

Revert this decision. No state change exists to undo.
