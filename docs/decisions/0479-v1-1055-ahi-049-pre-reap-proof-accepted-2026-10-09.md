# Decision 0479 — V1-1055 pre-reap quiescence proof (AHI-049 amendment) accepted

Date: 2026-10-09. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-09
("Accept both"), covering native ticket V1-1055.

## Context

Decision 0477 accepted `AHI-049` for V1-1040. After the leader was reaped, `internal/procgroup`
`Run` still probed the group by number. `CRR-V0-003`(d) in `docs/specs/compat-replay-runner-v0.md`
required that post-reap probe and bound it to the leader's start-time identity. Once the leader is
reaped, its group ID can be reused, so a numeric probe after the reap can reach an unrelated group.
AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts both parts of V1-1055:

1. The amended `AHI-049` text in `docs/specs/agent-harness-integration-v0.md`. `Run` proves the
   group quiescent while the exited leader is held unreaped. Every group signal, probe and proof
   goes through the `groupOwnership` lock that the release also takes, so no group operation
   follows the release. A group the held proof cannot show to be zombie-only stays present, and
   cleanup is then reported as unproved.
2. The replacement of `CRR-V0-003`(d)'s post-reap signal-0 probe, and its start-time identity
   binding, with that pre-reap proof.

Acceptance evidence is the tests the requirement names, including
`TestV11055NoNumericGroupProbeAfterReap` and `TestV11055NoGroupOperationAfterConcurrentRelease`.
See `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-09-v1-1055-procgroup-post-reap-probe.md`.

## Limits

This decision settles intent. The evidence is focused tests on Darwin; Linux was vet-only.
`make gate` and live host qualification are `NOT_RUN`. Foreign in-process reaping stays
unsupported (`AHI-050`). V1-1055 is not completed by this decision.

## Rollback

Revert this decision and the V1-1055 change: the `internal/procgroup` pre-reap proof and
`groupOwnership` lock, `internal/groupreap` `HeldGroupQuiet`, their tests, and the `AHI-049` and
`CRR-V0-003`(d) text. Then regenerate `docs/specs/REQUIREMENTS.tsv`. No store or wire format
changes.
