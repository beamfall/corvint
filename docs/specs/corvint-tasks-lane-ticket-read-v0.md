# Corvint Tasks Lane Ticket Read V0

Owner: Russell Lewis
Date: 2026-11-03
Requirement prefix: `TCP-V0`
Intent status: accepted under the owner's standing auto-accept (2026-10-11)
Delivery status: implemented
Authoritative inputs: lane dfric charter (orchestrator, 2026-11-03) for V1-4440, which assigned the
`TCP-V0-2230..2249` block to Tasks work; `corvint-tasks-store-init-v0.md` (`CTS-V0-010`,
`CTS-V0-680`, `CTS-V0-821`).

## Agent digest
- Claim: A journal-absent ticket miss in a linked worktree says that no worktree sharing its `.git` holds a journal.
- Status: accepted under the owner's standing auto-accept (2026-10-11) / implemented
- Exists: `journalAbsentNotFound` and `sharedJournalHint` in `internal/tasks/cli/cli.go`.
- Blocked on: reading tickets on a journal-less lane host still needs an exported snapshot (`CTS-V0-680`) or the charter's ticket text.
- Read next: Requirements; Acceptance and evidence.

## User job and verified baseline

A lane works in a linked worktree of the host's primary checkout. On the WSL lane host, at base
`14e055d69e3d18979f4eb5e33aae8eb4f16848fd`, `corvint-tasks ticket show V1-4440 --json` refuses
`MALFORMED` (`TCL-V0-859`) with `journal-absent`: store discovery resolves the common Git
directory correctly (`/home/russell/projects/corvint/.git`), the journal location
`.git/taskman` does not exist there, and the committed `.taskman` snapshot and `origin/main` do not
hold the ticket. The installed binary and a build of this tree behave the same. The refusal told
the lane to `cd PRIMARY_WORKTREE`, but the primary worktree shares the same missing journal, so the
advice could not work.

## Requirements

- `TCP-V0-2230`: (accepted under the owner's standing auto-accept (2026-10-11); V1-4440) A journal-absent `corvint-tasks ticket show` miss MUST keep the `CTS-V0-010` refusal (outcome `REFUSED`, code `MALFORMED`, the `NOT_OBSERVED` wording, the origin and snapshot routes) and MUST add that this checkout is the primary worktree, or a linked worktree of the named primary worktree, which shares that one journal location with every linked worktree, so none of them holds a journal and `cd` among them cannot help; the journal is in another clone or on another host. The read MUST NOT create, initialise or migrate any store.

## Non-goals, trust boundary and failure modes

No store discovery change, no new journal location, no fetch, no snapshot export and no Tasks
mutation. A host without a journal still cannot observe tickets created after the committed
snapshot; the refusal names the existing snapshot route (`CTS-V0-680`) for that. Creating the
journal or a snapshot on this host is store state and stays an orchestrator decision.

## Acceptance and evidence

| Requirement | Implementation | Deterministic evidence |
|---|---|---|
| `TCP-V0-2230` | `sharedJournalHint` in `internal/tasks/cli/cli.go` | `TestTCPV02230_JournalAbsentMissNamesTheSharedJournal` (linked worktree and primary, refusal shape kept, no store created); existing `TestCTSV0010_JournalAbsentShowNamesTheStaleSnapshot`, `TestCTSV0821_JournalAbsentMissNamesOriginSyncStep` |

Results and limits are retained in `../build-log/2026-11-03-dfric-dogfood-friction.md`.

## Compatibility and rollback

The refusal only gains a sentence; outcome, codes and every earlier phrase are unchanged.
Revert `sharedJournalHint` and its call to roll back.

## Remaining decisions

Whether the WSL lane host gets a journal or a regularly exported snapshot is store state for the
orchestrator (V1-4440, sibling V1-4226).
