# Decision 0502 — Top-level `ticket create` usage (CAL-V0-212) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("Accept CAL-V0-212 (#750), VSC-V0-077 and VSC-V0-078 (#752), and the decision 0501 record for #749.").

## Context

V1-0628 (GitHub #430): the per-verb `ticket create --help` already omits `--target` and
`--expected-revision` and documents the optional payload `localToken` (CAL-V0-051), but the
top-level `help` usage line implied that `ticket create` takes them. The V1-0628 lane proposed
`CAL-V0-212` in `docs/specs/corvint-tasks-agent-leases-v0.md`. AGENTS.md invariant 8 keeps
acceptance human-owned.

## Decision

The owner accepts `CAL-V0-212` as written:

- The top-level `help` usage MUST NOT advertise `--target` or `--expected-revision` for
  `ticket create`, which refuses both (CAL-V0-051).
- It MUST list `ticket create` on its own usage line that names the optional payload `localToken`
  as the way to choose the queue-local ID, and MUST list the target-taking `ticket` mutations
  separately.
- No ID grammar, collision rule, alias or wire change.

## Limits

This decision settles intent only. The evidence is `TestCALV0212_TopLevelHelpCreateUsage` named in
the lane's build log. `make gate` and a live-store check are `NOT_RUN`.

## Rollback

Revert this decision and return the `CAL-V0-212` status text to proposed. To withdraw the behavior,
also revert the top-level `help` usage split, then regenerate `docs/specs/REQUIREMENTS.tsv`. No
stored state changes.
