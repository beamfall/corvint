# Typed escalation pure foundation: issue 502

Human-owned intent: GitHub issue 502 (native ticket V1-0699; the native mapping was not
re-read in this session). The new contract `docs/specs/corvint-tasks-escalations-v0.md` records
ESC-V0-001..011 as proposed intent with experimental delivery. No owner acceptance of this intent
is recorded. The owner-delegated decisions of 2026-10-04 cover issues 500 and 501, not 502.

## Decision

The first slice is the pure four-file seed of the preparation plan's ESC502-011. Its parts:

- `ticket` wire codecs for `taskman-escalation-request/0` and `taskman-escalation-event/0`, and
  the optional reference;
- a `transaction` reducer `ApplyEscalation`;
- answer selection, derived holds and effective work revision.

The slice depends on no code from the in-flight 499 or 501 slices. It is therefore based directly
on public main `4b10a02144faed4a77b36eba4b0d1cbc315706d3` rather than stacked. `ticket` imports only
`wire`, and nothing calls the new code. Every proposal reports ActorAuthentication and Durability
as NOT_OBSERVED. The administrative cross-holder supersession route stays unsupported.

## Delivered

The four source files are reused byte-for-byte from the independently reviewed preparation source
patch. That patch already carried its review corrections. Codex rewrote none of it. The new spec
maps the plan's ESC502-001..011 to ESC-V0-001..011. It keeps the integration obligations visible:

- the native writer, stage, material and replay;
- `ESCALATION_PENDING` holds;
- the CLI and reads;
- claim delivery through the 501 claim-snapshot path;
- dispatcher infrastructure retry and composition with the 499 tiers.

Those obligations stay unmet requirements, not delivered behavior.

## Review findings carried

The preparation's independent Gate A required three corrections, all already present in the reused
source:

- shorthand-answer replay keeps the resolved request ID and previous revision in event material;
- transaction and event counts are separate;
- the origin is the OPEN blob digest.

A later same-reviewer M1 plan correction PASS covers integration planning only. It grants no
native qualification and was not executed here.

Two minor source observations remain open; neither blocks the pure slice:

- a `blockedBy` relation may name its own ticket;
- a blocked question without a relation holds through the question itself.

## Evidence

Focused tests and vet ran under `GOTOOLCHAIN=local` on go1.27.1, all exit 0:

- `go vet ./internal/tasks/ticket/ ./internal/tasks/transaction/` exited 0;
- `go test -count=1 ./internal/tasks/ticket/ ./internal/tasks/transaction/` exited 0;
- all six `TestIssue502_*` tests passed.

NOT_RUN:

- native writer and replay races;
- compiled CLI;
- claim-snapshot delivery;
- dispatcher restart and retry;
- capacity under the real stage;
- repository-wide gate;
- native completion.
