# Decision 0490 — Stage-slot wait (CTS-V0-008) accepted

Date: 2026-10-10. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-10
("accept CTS-V0-008").

## Context

V1-0825 reported that an unlocked journal audit could refuse `MALFORMED` `unassigned stage slot`
while a live writer was mid-publish. Under its writer lock, a native writer holds descriptor-less
staging slots `staging/aNN` for the whole publish, so an unlocked reader can capture them as if
they were a killed writer's orphans. The stage-slot race lane proposed `CTS-V0-008` in
`docs/specs/corvint-tasks-store-init-v0.md`.

Independent review took four rounds. The fixes added the writer-locked no-wait mode, extended the
same wait to archive `export`, and kept stage pauses from drawing on the CTS-V0-006 deadline. They
also replaced a timing-dependent regression test with one driven by the pending wait. AGENTS.md
invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `CTS-V0-008` as written:

- An unlocked journal audit or archive `export` that sees descriptor-less staging slots, with no
  `active.json` or `active.json.tmp` beside them, pauses with backoff. The pauses run 25 ms
  doubling to 400 ms and total exactly two seconds. The reader then reads again.
- If the slots outlast that budget, the original `MALFORMED` refusal is returned unchanged.
- An audit made under the writer lock refuses at once, because there the slots can only be
  orphans.
- A slot beside a stage descriptor or descriptor temp refuses at once.
- Stage pauses use neither the CTS-V0-006 deadline nor the moved-snapshot attempts.

The requirement status, the spec's Intent paragraph and the lane's build log
(`docs/build-log/2026-10-10-stage-slot-race.md`) record the acceptance.

## Limits

This decision settles intent only. The evidence is focused journal, archive, snapshot and store
tests (with `-race` where noted in the build log), the doc gates and the receipt check. The full
suite and `make gate` are `NOT_RUN`. The two-second budget counts requested pauses, not wall-clock
time. A heavily loaded host can still outlast it, and the reader then gets the original refusal.

## Rollback

Revert this decision and return the CTS-V0-008 status text to proposed, pending owner acceptance.
To withdraw the behavior, also revert the V1-0825 change:

- `internal/tasks/snapshot/stage_slot_wait.go`
- the stage-slot handling in `internal/tasks/journal/stage_read.go` and `audit.go`
- `internal/tasks/store/guards.go` (`lockedJournalReader`) and its call sites
- `internal/tasks/archive/stage_read.go`

Then regenerate `docs/specs/REQUIREMENTS.tsv`. No stored state changes: readers write nothing.
