# Decision 0493 — CAL-V0-024 and CAL-V0-074 amendments accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept CAL-V0-024 and CAL-V0-074 (decision 0493)").

## Context

The RC3 cut (decision 0491) includes two Corvint Tasks bugs:

- V1-1090: `submit` refused `OUT_OF_SCOPE` for files beneath a `PATH` key that named a directory
  without a trailing `/`. CAL-V0-021 already makes such a key exact, so the refusal was correct,
  but it did not say why.
- V1-1063: a replacement owner recovering a dead owner's `STOPPING` program with a result other
  than `NO_EXEC` was refused `MALFORMED` ("usage must derive from retained host output"), leaving
  the program stuck in `STOPPING`.

AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts both amendments to `docs/specs/corvint-tasks-agent-leases-v0.md` as written:

- `CAL-V0-024` (V1-1090): coverage is still decided by the CAL-V0-021 key rule. The
  `OUT_OF_SCOPE` refusal MUST also name any `PATH` key without a trailing `/` that an offending
  path lies beneath, with the rule that only a directory key ending in `/` covers the paths
  beneath it. Scope matching is unchanged.
- `CAL-V0-074` (V1-1063): a replacement owner that recovers a dead owner's `STOPPING` program
  through the recovery evidence MUST record the move to `FINISHED` with usage unobserved when the
  result class is not `NO_EXEC`, as CAL-V0-210 already does for a stop the attempt proved. Token
  counters and turns are unchanged.

## Limits

This decision settles intent only. The evidence is the focused tests named in the lane's build
log. The whole store package, `make gate` and live host qualification of the recovery are
`NOT_RUN`.

## Rollback

Revert this decision and return both amendments to proposed. To withdraw the behavior, also revert
the V1-1090 change in `internal/tasks/transaction/lease_gate.go` and the V1-1063 change in
`internal/tasks/store/program.go`, then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state
format changes.
