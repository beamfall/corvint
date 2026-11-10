# Change proposal: cut the Corvint task store over to a fresh store of 1.0 tickets

Change name: `tasks-store-cutover`
Tickets: none filed by this proposal. The cutover gets one ticket in the new store after it runs;
no write verb of `corvint-tasks` was run against the live store while writing this change.
Owning capability spec: a new `docs/specs/tasks-store-cutover-v1.md` (prefix `TSC-V1`), written
when this proposal is accepted. It is not written by this change.
Gating change: `ticket-taxonomy` (the new ticket categorization scheme, designed separately). This
cutover SHALL NOT run until that change is accepted (TSC-V1-001).
Requirement IDs this change adds: TSC-V1-001 to TSC-V1-024 (a `grep -rn "TSC-"` under
`prodspecs/docs/specs` found no hit on 2026-11-09; no collision)
Files in this change (G14 layout): `proposal.md` (this file: why, what changes, non-goals,
impact), `design.md` (owner decisions applied, facts and their verification, cutover steps, proof,
rollback, losses, other hosts, failure modes, open decisions), `tasks.md` (ordered tasks and
acceptance evidence), `specs/tasks-store-cutover/spec.md` (ADDED Requirements).
Owner decisions applied: 2026-11-09 purge of old closed tickets, T1, T2 and T3; see `design.md`.
Intent status: proposed (written 2026-11-09; only the owner can accept it)
Delivery status: not-started
Written before dispatch: yes. Nothing in the live store has been changed.

Store facts below were read from the live store with read-only verbs (`queue status`,
`receipt audit`, `archive verify`, `ticket show`) on 2026-11-09 at journal head seq 24185. Each
fact in `design.md` is marked VERIFIED or NOT_VERIFIED.

## Why

The live store at `/Users/russelllewis/projects/corvint` holds 5,280 tickets: 1,029 OPEN, 7 HELD,
2,560 COMPLETED and 1,684 ARCHIVED (VERIFIED, `queue status`, head seq 24185). The owner said on
2026-11-09: "1029 open ticket is nuts"; "we should just remove the closed ones."; "Purge old
closed". `corvint-tasks` has no purge verb (VERIFIED: `help --json` lists no such verb;
`ticket archive` keeps the record). The journal is hash-chained, so removing records in place
would break `receipt audit`.

A classification of the 1,029 OPEN tickets (`scratchpad/tix/merged.tsv`) found 164 that 1.0
needs. The rest are later work (414), Tasks-product work (160), work on code the cut removed (176),
obsolete (48), lane-process work held back (49) or undecided (18). A planner reading the queue today
sees six non-1.0 tickets for every 1.0 ticket.

The owner chose (T1) a fresh store, keeping the old one as a verified archive; (T2) only 1.0
tickets are seeded; (T3) seeded tickets are created in the shape that `ticket-taxonomy` defines.

## What Changes

1. The old store is exported at the then-current head, verified, checksummed and kept read-only
   outside the repository. Its state directory is moved aside, never deleted.
2. A new store is initialized in the same repository from the current `policy.json` and
   `queue.json` only.
3. The 1.0 survivors are created in the new store with `ticket create`, in dependency order, in the
   `ticket-taxonomy` shape: the 164 NEED-1.0 tickets plus any UNSURE ticket the owner assigns to
   1.0. A survivor whose milestone is not `v1-0` (43 in `survivors-draft.tsv`) is corrected to
   `v1-0` as it is created.
4. The `v1-0` release object is recreated with the seeded members only and no predecessor.
5. The 160 MOVE-TASKS tickets are written to the flat list at cutover. The store of
   `/Users/russelllewis/projects/corvint-tasks` is initialized as its own change within three
   days of corvint-tasks PR #4 merging and seeded from the 23 P1 `v1-0` tickets (decided
   2026-11-09 under R42, C15). That repository has no initialized store today (VERIFIED:
   `queue status` there refused `UNINITIALIZED`).
6. The NEED-LATER tickets and the later-list rows of the C13 verdicts are published as one
   generated, read-only list file in the repository, proposed path `docs/backlog/post-1.0.tsv`
   (R42 (panel pick) C14). They are not seeded. HEAVY-HOLD, CLOSE-REMOVED, CLOSE-OBSOLETE and all
   COMPLETED or ARCHIVED tickets are neither seeded nor listed; they stay readable in the archive
   (R42 (panel pick) C13).
7. A seed manifest records every field the new store does not carry for each seeded ticket.
8. Every other host with its own state directory (WSL, Unraid) is re-seeded from the new store's
   export.

## Capabilities

New capability `tasks-store-cutover` (TSC-V1-001 to TSC-V1-024): preconditions, archive,
survivor set, seeding, release, the Tasks-repo move, the flat list, proof, rollback and other
hosts. It defines a one-time operation, not a new verb.

## Impact

- `.taskman/tickets/` in the repository goes from 2,005 tracked files (VERIFIED, `git ls-files`
  on the primary checkout) to about 164 to 182; `.taskman/releases/` keeps only `v1-0`.
- Every open lane, claim and attempt in the old store ends. Lanes stay stopped through the cutover.
- Ticket links in commits, build-log entries, PR bodies and memory notes that name non-seeded
  tickets resolve only in the archive.
- Hosts that pull the cutover commit and still hold the old state directory read `INTENT_DIVERGED`
  until re-seeded (INFERRED in the rehearsal; NOT_VERIFIED).

## Non-goals

- Adding a purge, compact-tickets or cutover verb to `corvint-tasks`.
- Designing the ticket categorization scheme (that is `ticket-taxonomy`).
- Re-triaging the 1,029 classifications beyond the 18 UNSURE tickets.
- Initializing the `corvint-tasks` repository store (that needs its own project scope).
- Carrying history (revisions, progress, attempts, gates, evidence, release chain) into the new
  store.
- Using `corvint-tasks import --file` (refused for a NATIVE-written queue; see `design.md`).
- Changing `policy.json` (milestone M0 policy v15 is its own change, `tasks-policy-v15`).
