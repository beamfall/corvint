# Decision 0516 — GPK-V0-083, TCP-V0-065 and the TCP-V0-006 amendment accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner answer "Accept both, include
them" in the orchestrator chat, 2026-10-11 UTC, to the question whether to accept the requirements
proposed on pull requests #770 and #771 (this decision covers #770, V1-0266 and V1-0315).

## Context

V1-0266 lets a confident symbol whose support strictly exceeds the competitive records lead a
standing record packet (`GPK-V0-083` in `docs/specs/go-production-kernel-migration-v0.md`).
V1-0315 adds a bounded top-level `exclusions` member that reports corpus gaps in the task-context
packet without changing ranking (`TCP-V0-065` and the `TCP-V0-006` amendment in
`docs/specs/task-context-packet-v0.md`). AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `GPK-V0-083`, `TCP-V0-065` and the `TCP-V0-006` amendment as written in the
spec text carried by this pull request. Their status text changes from "proposed, pending owner
acceptance" to "accepted, decision 0516".

## Limits

Delivery stays experimental, and the frozen-evaluation limits recorded in the V1-0266 and V1-0315
build-log entries still apply. This decision is not a standing rule for other requirements.

## Rollback

Revert this decision and restore the proposed status text. The behavior rollback is the one each
build-log entry names: removing the placement restores the analyzer schema and audit pins with it,
and snapshots are derived state that rebuild without migration.
