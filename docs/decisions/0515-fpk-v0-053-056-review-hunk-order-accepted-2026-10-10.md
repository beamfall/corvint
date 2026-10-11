# Decision 0515 — FPK-V0-053..056 review hunk order accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer "accepted" in the
orchestrator chat, 2026-10-11 UTC, to the question whether to accept FPK-V0-053..056 on pull
request #768 (V1-0408 partial).

## Context

V1-0408 adds `internal/revieworder`, a pure experimental library projection that orders review
hunks by cited authorship, witnessing and coverage factors, with unknowns kept explicit.
FPK-V0-053..056 in `docs/specs/falsifiable-packet-v0.md` define it. AGENTS.md invariant 8 keeps
acceptance human-owned.

## Decision

The owner accepts FPK-V0-053, FPK-V0-054, FPK-V0-055 and FPK-V0-056 as written in the spec text
carried by this pull request. Their status text changes from "proposed, pending owner acceptance
(V1-0408)" to "accepted, decision 0515".

## Limits

Delivery stays experimental: there is no CLI and no promotion of the slice, and V1-0408 remains
partial. This decision is not a standing rule for other requirements.

## Rollback

Revert this decision and restore the proposed status text; the library has no persisted state or
wire format.
