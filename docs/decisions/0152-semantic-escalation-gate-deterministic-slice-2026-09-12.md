# Decision 0152 — The Semantic Escalation Gate's first slice is an unwired deterministic gate

Date: 2026-09-12. Status: accepted. Authority: delegated owner call, 2026-09-12.

`docs/specs/semantic-escalation-gate-v0.md` was accepted by decision 0047 with delivery
not-started. Its first executable slice, `internal/semescalate`, is the deterministic router only:
no real provider, no network, no process, and no filesystem access. Five narrowing calls were
needed where the spec leaves room.

1. One call per gap. SEG-009 allows a second call to a stronger calibrated model; with no model
   registry or calibration there is no stronger model, so the slice caps each gap at one call and
   leaves SEG-009 undelivered.
2. An absent or unrecognised trigger is refused as `NO_CALL/UNSUPPORTED_INPUT`. SEG-003 has no
   dedicated reason for an unnamed gap, and inventing one would change the stable reason list.
3. A provider without an exact model revision, a calibration digest, and the proposal schema is
   `NO_CALIBRATED_MODEL`. This is the refusal half of SEG-007 only; least-cost selection among
   several providers stays undelivered because its ordering is an unresolved spec decision.
4. Every attempted call, including a timeout or output-limit failure, is recorded in the in-memory
   derivation ledger, so an identical identity never calls again (SEG-010 forbids same-input
   retries; SEG-017 requires zero repeat calls). Ledger persistence and retention stay unresolved.
5. The response decoder rejects unknown fields, so a model-authored authority, verdict, or status
   field invalidates the whole response as `SCHEMA_INVALID` rather than being silently dropped.

The package is imported by no production package (`TestNoProductionPackageImportsTheGate`), so no
default command, serving, or ranking path can reach it (AGENTS.md invariants 3 and 7; SEG-016).

Rollback: delete `internal/semescalate`, restore the spec's delivery status to not-started with the
single pending traceability row, and revert the INDEX and README rows. Nothing else depends on it.
