# Decision 0440 — rc3 cleanup requirements accepted

Date: 2026-10-07. Status: accepted by the owner. On 2026-10-07 the owner replied "accept" to the
orchestrator's list of the five proposed cleanup requirements for V1-0928 to V1-0931.

## Context

The owner asked whether Corvint cleans up files it no longer needs and asked to keep Corvint lean
and efficient. A read-only audit confirmed that the snapshot, temporary-file and private-ledger
bounds hold, and filed five gaps as V1-0928 to V1-0932. Two delivery lanes each drafted proposed
requirements, and Codex reviewed each lane until it reported no blocking finding. AGENTS.md
invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts these requirements as written in their specs at the integration head:

| Requirement | Spec | Ticket | Summary |
|---|---|---|---|
| `IDX-SNAP-V0-027` | `docs/specs/index-snapshot-v0.md` | V1-0928 | Only an `index` write sweeps the worktree's own superseded `.corvint/index` store, removing only recognised entries through descriptor-bound directory handles and naming what it leaves. |
| `UPD-V0-007` | `docs/specs/operator-update-v0.md` | V1-0929 | `apply` and `rollback` lock the update state directory. An owned `transaction-<digits>` directory with no receipt that is more than 30 minutes old is swept. A successful apply keeps only the latest rollback-capable transaction. |
| `CAL-V0-143` | `docs/specs/corvint-tasks-agent-leases-v0.md` | V1-0930 | Each dispatcher worker output stream is capped at 8 MiB live plus one 8 MiB `.1` file. |
| `CAL-V0-144` | same | V1-0930 | Finished worker directories are retired beyond 32 quiet ones. Removal needs a mark and a later confirming pass. The confirming pass runs on its own schedule, and does no work when nothing is pending. |
| `ATR-V0-015` | `docs/specs/corvint-tasks-attempt-runner-v0.md` | V1-0931 | After a detached supervisor finishes, at most 16 quiet ended runs are kept across terminal attempts. |

The owner also accepts these limits:

- Update rollback depth is one step.
- An interrupted update is swept only by a run at least 30 minutes later.

V1-0932 adds tests only and needs no requirement.

## Limits

This decision settles intent only. Delivery status is unchanged:

- The live macOS update lifecycle, Linux dispatcher runtime and live dispatcher qualification were
  not run.
- `make gate` was not run.
- Each spec states its residual races. For V1-0933 (a retry racing run retirement) and V1-0934 (the
  per-pass scan bound), see their own tickets.

## Rollback

Revert this decision and restore the "proposed, pending owner acceptance" markers in the affected
specs, `docs/specs/README.md` and `docs/specs/INDEX.json`, then regenerate
`docs/specs/REQUIREMENTS.tsv`.
