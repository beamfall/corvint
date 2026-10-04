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

The four source files were applied unchanged from the preparation's independently reviewed
`SOURCE.patch`, which already carried its review corrections. Before commit, `cmp` confirmed each
file byte-identical to the preparation leaf checkout. Nothing was rewritten. The new spec
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

A fresh independent review of the delivered diff returned PASS-WITH-NITS. It found no HIGH issue.
It found one MED: stale OPEN questions from an earlier acceptance revision count toward the 16-open
bound but cannot be answered or superseded, so they can lock a ticket's capacity. It also found
several LOW items:

- one policy decision covers every operation;
- untested supersession refusal branches and the untested event byte cap;
- refusal assertions that check only for an error, not its code;
- shorthand refusals that do not name the open questions;
- repeated blob decoding.

The traceability rows were corrected. The source findings are recorded as open in the spec under
Unresolved decisions rather than patched here, so the reused reviewed source stays byte-identical.

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
