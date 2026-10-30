# Decision 0477 — adapter exit kills procgroup groups (AHI-049) accepted

Date: 2026-10-09. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-09
("AHI-049 accepted"), covering native ticket V1-1040.

## Context

`AHI-048` records a process group in the `internal/groupreap` live registry only when its spawner
starts the child through `StartLive`, `Drain` or an `Owner`. `internal/procgroup` `Run` started its
own Setpgid leader with plain `command.Start`, so the adapter exit kill did not reach it, and
`captureSourceHandoff` reaches `Run` through `sourceGit` from a point the `AHI-017` watchdog can
abandon. The requirement is `AHI-049` in `docs/specs/agent-harness-integration-v0.md`. AGENTS.md
invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AHI-049` as written for V1-1040 (first carried in batch Q, landed by the RC3 process-group lane). Every Setpgid or Setsid spawner reachable from
an adapter abandon point records the group it leads for the `AHI-048` exit kill, or is listed as
unreachable in the inventory in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-09-v1-1040-adapter-exit-procgroup-registry.md`,
which is redone when a spawner or abandon point is added. `procgroup.Run` records its group with
`groupreap.StartLive` and releases it with `groupreap.ReleaseLive` before it allows the reap; a
failed exit observation releases the record at once, and `Run` then sends no group signal or probe
and reports cleanup as unproved. Acceptance evidence is the tests the requirement names.

## Limits

This decision settles intent. The evidence is focused tests on Darwin (build log above). Linux,
`make gate` and live host qualification are `NOT_RUN`. V1-1040 is not completed by this decision.

## Rollback

Revert this decision and the V1-1040 change (`internal/procgroup` `StartLive`/`ReleaseLive` use,
`groupreap.ReleaseLive`, their tests and the `AHI-049` text), then regenerate
`docs/specs/REQUIREMENTS.tsv`. `procgroup.Run` groups are then again outside the exit kill. No store
or wire format changes.
