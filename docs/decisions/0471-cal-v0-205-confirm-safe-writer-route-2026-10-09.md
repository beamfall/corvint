# Decision 0471 — pool confirm-safe on the writer checkpoint (CAL-V0-205) accepted

Date: 2026-10-09. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-09,
relayed to the implementing lane by the coordinating session, covering native ticket V1-1045 and
GitHub issue 701.

## Context

V1-1045 found that `corvint-tasks pool confirm-safe` took the complete lease route: under the
preparation gate it decoded every receipt in the journal before committing. On a queue of about
21,100 receipts that took minutes, so concurrent heartbeat, claim and confirm-safe calls failed
with `LOCK_TIMEOUT`, and a caller timeout left a lane quarantined. The writer-checkpoint route
(CAL-V0-116, accepted by decision 0439) already served claim, claim-next, renew, heartbeat and
release, but not confirm-safe. The requirement is `CAL-V0-205` in
`docs/specs/corvint-tasks-agent-leases-v0.md`. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `CAL-V0-205` as written. `POOL_CONFIRM_SAFE` joins the CAL-V0-116 writer-route
lease verbs, and the route's accepted guarantee change (a prefix rewrite is not detected by a fast
writer, decision 0439) extends to it. Every decline of the route still hands the request to the
complete route. The V1-1045 section status, the requirement, the authoritative-inputs line, the
requirement table row and the delivery status record the acceptance.

## Limits

This decision settles intent. Live qualification on the 21,100-receipt store is `NOT_RUN`; the
evidence is focused tests and a synthetic measurement
(`docs/build-log/2026-10-09-v1-1045-confirm-safe-writer-route.md`). Without a bound writer
checkpoint, confirm-safe still runs the complete audit under the preparation gate. The other pool
verbs keep the complete route. V1-1045 is not completed by this decision.

## Rollback

Revert this decision and the V1-1045 change: remove `transaction.LeasePoolSafe` from
`writerLeaseVerb`, the test, the V1-1045 section, the table row and the delivery-status clause, then
regenerate `docs/specs/REQUIREMENTS.tsv`. Stores are unchanged, because both routes write the same
bytes.
