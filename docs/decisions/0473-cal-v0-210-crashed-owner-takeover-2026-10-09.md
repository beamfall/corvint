# Decision 0473 — replacement-owner takeover after a proved stop (CAL-V0-210) accepted

Date: 2026-10-09. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-09
("CAL-V0-210 accepted"), covering native ticket V1-0795.

## Context

V1-0795 found that when a supervised program's owner crashed between dispatch and its `FINISHED`
record, the program stayed `SPAWNING` or `STOPPING` and every replacement owner was refused, so the
ticket's claim and reservation were held indefinitely. CAL-V0-074 admits an owner change from those
phases only with a retained leader boot record and a proved host stop. The requirement is
`CAL-V0-210` in `docs/specs/corvint-tasks-agent-leases-v0.md`. AGENTS.md invariant 8 keeps
acceptance human-owned.

## Decision

The owner accepts `CAL-V0-210` as written. A replacement owner may settle a dead owner's `SPAWNING`
or `STOPPING` program `FINISHED`, at epoch plus one with the worktree unchanged, only when the
program's current attempt (same ID and generation) is supervised by that program and records no
worker, no lane and quiescence `PROVED`. A move from `STOPPING` whose result class is not `NO_EXEC`
records its usage unobserved. Every other owner change keeps the existing fences; an unproved stop
stays refused. The requirement, the authoritative-inputs line, the requirement table row, the
failure-mode row and the delivery status record the acceptance.

## Limits

This decision settles intent. The evidence is focused tests, including
`TestCALV0074_NoExecCrashTakeover`, which fails on the base (`docs/build-log/2026-10-09-v1-0795-crashed-owner-takeover.md`).
Live host qualification is `NOT_RUN`; the race run of the whole store package did not complete
within the per-package timeout. V1-0795 is not completed by this decision.

## Rollback

Revert this decision and the V1-0795 change: remove `BoundStoppedAttempt` from
`internal/tasks/transaction/program.go` and its use in `internal/tasks/store/program.go`, the
`dispatched` test hook point, the CAL-V0-210 section, table rows and delivery-status clause, then
regenerate `docs/specs/REQUIREMENTS.tsv`. Stores are unchanged until a takeover is written; a
written takeover is an ordinary `FINISHED` program record.
