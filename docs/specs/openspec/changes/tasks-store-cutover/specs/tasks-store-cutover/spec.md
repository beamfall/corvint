# Spec delta: tasks-store-cutover

Change name: `tasks-store-cutover`. Capability: `tasks-store-cutover` (prefix `TSC-V1`; the
owning spec `docs/specs/tasks-store-cutover-v1.md` is written when this change is accepted).
Intent status: proposed.

"Old store" means the live store of `/Users/russelllewis/projects/corvint` before the cutover.
"New store" means the store initialized by this cutover in the same repository. "Final head" means
the old store's journal head seq at the moment of the final export (TSC-V1-005).

## ADDED Requirements

### Requirement: TSC-V1-001 Gated on `ticket-taxonomy`

The cutover SHALL NOT start until the `ticket-taxonomy` change is accepted by the owner. The
accepted taxonomy's commit SHA SHALL be recorded in the cutover's build-log entry. The cutover
SHALL run at milestone M0, after the 1.0 product plan and `ticket-taxonomy` are accepted, as one
cutover before the Tasks trial clock starts (R42 (panel pick) C11).

#### Scenario: taxonomy still proposed
- WHEN the cutover is started and `ticket-taxonomy` has intent status proposed
- THEN the cutover stops before step 1 and nothing is exported or moved

### Requirement: TSC-V1-002 UNSURE tickets decided

Each of the 18 UNSURE tickets in `merged.tsv` SHALL carry a decision (seed for 1.0, list as later,
move to Tasks, or close) before the survivor list is frozen. The decisions are the R42 (panel pick)
C13 verdict table in `design.md`. No UNSURE ticket SHALL be seeded or dropped by agent judgment
outside that table. The 7 HELD tickets and the 2 HEAVY-HOLD tickets marked `v1-0` (V1-4841,
V1-5019) SHALL follow the C17 table in `design.md`; none of the nine SHALL be seeded in the new
store. V1-5019's in-flight attempt SHALL be finished or released before the quiet-store step
(TSC-V1-003, TSC-V1-004). (decided 2026-11-09 under R42, C17)

#### Scenario: one UNSURE ticket undecided
- WHEN the survivor list is frozen and V1-0014 has no row in the verdict table
- THEN the freeze is refused and the cutover does not start

#### Scenario: a HELD ticket in the seed list
- WHEN the frozen survivor list contains V1-4243, which the C17 table sends to the Tasks later list
- THEN the freeze is refused

### Requirement: TSC-V1-003 In-flight tickets reconciled

Every survivor whose old lifecycle is not OPEN (102 of 164 in `survivors-draft.tsv`) SHALL be
checked against `origin/main` before the freeze. A ticket whose acceptance is met by merged work
SHALL be completed in the old store and not seeded. Any other ticket is seeded OPEN, and its latest
lane, branch, head and PR are recorded in the seed manifest (TSC-V1-010).

#### Scenario: delivered but not closed
- WHEN a survivor is AWAITING_CLOSEOUT and its PR merged into `origin/main`
- THEN it is completed in the old store before the final export and is absent from the seed list

### Requirement: TSC-V1-004 Quiet store

Before the final export the old store SHALL have no live attempt (`queue status` `liveAttempts`
empty), every expired lease (V1-4399, V1-5162, V1-5212 on 2026-11-09) SHALL be released, every
lane on every host SHALL be stopped, and no write SHALL reach the old store after the final export.

#### Scenario: a lane writes after export
- WHEN the old store's head seq is greater than the final head after the export
- THEN the export is discarded and TSC-V1-005 is repeated at the new head

### Requirement: TSC-V1-005 Archive at the final head

The old store SHALL be exported with `archive export` at the final head, verified with
`archive verify`, checksummed with SHA-256 and stored under
`/Users/russelllewis/corvint-store-archives/` with the head seq in its name. Before export,
`receipt audit` SHALL report `structuralConsistency CONSISTENT` and `projectionAgreement AGREES`.
The `verify` output's `exportedAtSeq` and `lastReceiptSha256` SHALL equal the final head and the head receipt hash from `queue status`. The
archive of head 24185 is valid only if the final head is 24185.

The archive tar, its `SHA256` file, `verify.json` and `export.envelope.json` SHALL be kept for the
whole 1.x line. Two copies SHALL be made off this Mac, one on the WSL benchmark host and one on
the Unraid array. Each copy SHALL be checked against the recorded SHA-256 when it is made, and
again at the Tasks RC tag and at stable. No copy SHALL be uploaded to a cloud service. (decided
2026-11-09 under R42, C16)

#### Scenario: head moved past 24185
- WHEN `queue status` reports head seq 24190 at cutover
- THEN a new archive `corvint-main-<date>-head24190.tar` is exported and verified, and the 24185
  archive is kept but is not the rollback point

#### Scenario: an off-host copy does not match
- WHEN the copy on the Unraid array has a SHA-256 different from the recorded value
- THEN that copy is replaced from the Mac archive and checked again before it counts as a copy

### Requirement: TSC-V1-006 Old state moved, never deleted

The old state directory (`.git/taskman`) and the old `.taskman/tickets/` and
`.taskman/releases/` trees SHALL be moved to a backup directory beside the archive. No step of the
cutover SHALL delete them. The moved state SHALL be kept until 30 days after stable (2027-02-04
if stable is 2027-01-05). After that date it MAY be deleted, and only after `archive verify`
passes on the archive. (decided 2026-11-09 under R42, C16)

Earlier wording, superseded by C16: "Deleting them later needs a separate owner approval."

#### Scenario: disk cleanup proposed
- WHEN an agent proposes removing the moved state directory before 30 days after stable
- THEN the directory is not removed

#### Scenario: cleanup after the retention date
- WHEN 30 days have passed since stable and `archive verify` fails on the archive
- THEN the moved state directory is not deleted

### Requirement: TSC-V1-007 Fresh store from policy and queue only

The new store SHALL be initialized with `corvint-tasks init` in a repository whose `.taskman/`
holds only `policy.json` and `queue.json`. `policy.json` SHALL be byte-equal to the old store's.
`queue.json` SHALL keep queue ID and prefix, and its `nextSerial` SHALL be at least the old store's
`nextSerial` at the final head, so that no new ticket reuses an old number.

#### Scenario: stale serial
- WHEN `queue.json` is taken from `origin/main` with `nextSerial` 2446 and the old store is at 5263
- THEN init is not run until `nextSerial` is set to at least 5263

### Requirement: TSC-V1-008 Survivor set is 1.0 only

The new store SHALL contain exactly the frozen survivor list: the NEED-1.0 tickets, plus UNSURE
tickets and HELD tickets the owner assigned to 1.0, minus tickets completed under TSC-V1-003.
Every seeded ticket SHALL have milestone `v1-0`. No NEED-LATER, HEAVY-HOLD, MOVE-TASKS,
CLOSE-REMOVED or CLOSE-OBSOLETE ticket SHALL be seeded unless the owner reclassifies it. Under
the C13 and C17 tables no UNSURE ticket and no HELD ticket is assigned to 1.0 in this store.
(decided 2026-11-09 under R42, C17)

#### Scenario: survivor with milestone post-1-0
- WHEN a NEED-1.0 survivor has milestone `post-1-0` in the old store
- THEN it is created with milestone `v1-0` and the correction is recorded in the manifest

### Requirement: TSC-V1-009 Seeded in the taxonomy shape

Each seeded ticket SHALL be created in the shape the accepted `ticket-taxonomy` defines. The
mapping from old fields to the taxonomy SHALL be one reviewed table applied by script, not edited
per ticket by hand.

#### Scenario: a ticket the mapping cannot place
- WHEN the mapping yields no category for a survivor
- THEN the seed stops before that ticket and the owner or the taxonomy owner decides

### Requirement: TSC-V1-010 Seed manifest

A seed manifest SHALL list, per seeded ticket: old ticket ID, new ticket ID, dropped dependencies,
dropped `supersedes`/`supersededBy`, old status and lifecycle, holds, operator note text, approvals,
latest progress row, milestone correction, duplicate overrides and touchPaths edits. It SHALL be
committed with the new store in the cutover commit. Seeded tickets SHALL keep their old V1 IDs
(`localToken`), so the new ticket ID equals the old one; the store serial SHALL continue from 5263,
and the new categories SHALL live in fields, not in the ID (R42 (panel pick) C12).

#### Scenario: dependency on a non-seeded ticket
- WHEN a survivor depends on V1-0018, which is not seeded
- THEN the dependency is left out of the create payload and appears in the manifest row

### Requirement: TSC-V1-011 Create, not import

Survivors SHALL be created with `ticket create --payload-stdin`, one request ID per ticket
(`seed-<old token>`), in dependency order. `corvint-tasks import --file` SHALL NOT be used.

#### Scenario: seed rerun after a partial failure
- WHEN the seed loop is run again after stopping at ticket 90
- THEN tickets 1 to 89 answer `replayed true` and write nothing, and seeding resumes at 90

### Requirement: TSC-V1-012 Duplicate-check overrides are bounded

A `DUPLICATE_ID` refusal SHALL be overridden with `--not-duplicate-of` only for tickets the
refusal names that are themselves seeded survivors, and every override SHALL be recorded in the
manifest. A refusal that names a non-survivor or persists after seven retries SHALL stop the seed.

#### Scenario: two survivors look alike
- WHEN V1-5199 is refused as similar to seeded V1-5198
- THEN it is retried with `--not-duplicate-of` V1-5198 and the override is in the manifest

### Requirement: TSC-V1-013 Removed touchPaths

A create refused because an `effects.touchPaths` entry no longer exists SHALL be fixed by editing
the path to its current location or dropping it, recorded in the manifest. `--creates-paths` SHALL
be used only when the ticket's acceptance creates that path.

#### Scenario: path removed by the cut
- WHEN a survivor names `internal/docviews/` and the create is refused MALFORMED
- THEN the path is dropped or replaced, the change is in the manifest, and the create is retried

### Requirement: TSC-V1-014 Holds, notes and approvals reapplied

Holds, operator notes and approvals on survivors SHALL be reapplied in the new store only where the
manifest marks them still wanted. A survivor held in the old store SHALL NOT be seeded eligible
without either a new hold or an owner decision to release it.

#### Scenario: held survivor
- WHEN a seeded survivor was HELD in the old store
- THEN `ticket hold` is applied after create, unless the owner released it

### Requirement: TSC-V1-015 Release `v1-0` recreated

After all survivors exist, the `v1-0` release SHALL be created with `release create`, its title,
acceptance criteria and required gates copied from the old record, `ticketIds` filtered to seeded
tickets, and `predecessorReleaseIds` empty. No other release SHALL be recreated.

#### Scenario: release member not seeded
- WHEN the old `v1-0` record lists 377 tickets and 170 are seeded
- THEN the new `v1-0` lists the 170 and the other 207 are named in the manifest

### Requirement: TSC-V1-016 Tasks-product tickets move to the Tasks repository

At cutover the 160 MOVE-TASKS tickets SHALL be written to the generated flat list of TSC-V1-017
and SHALL NOT be seeded in the Corvint store. The Corvint cutover SHALL NOT wait for the Tasks
store. The store of `beamfall/corvint-tasks` (`/Users/russelllewis/projects/corvint-tasks`) SHALL
be initialized as its own small change in that repository (`tasks-store-init`) within three days
of corvint-tasks PR #4 merging (target 2026-11-16). It SHALL be seeded through the same seed gate
(TTX-V1-017) with only the tickets that name a PROD-TASK, TKF, TTX or TCF requirement. The
starting pool is the 23 P1 `v1-0` tickets. The 137 `post-1-0` tickets (56 P2, 81 P3) SHALL go to
that repository's later list. Until the Tasks store exists, a new Tasks gap SHALL be filed in the
Corvint store under the Tasks product. (decided 2026-11-09 under R42, C15)

Earlier wording, superseded by C15: "The MOVE-TASKS tickets SHALL be created in the store of
`/Users/russelllewis/projects/corvint-tasks` in that repository's own shape, after its store is
initialized under its own accepted scope. Until then they SHALL be listed, not seeded, in that
repository's declared backlog file, and not in the Corvint store."

#### Scenario: Tasks store not initialized
- WHEN `queue status` in the Tasks repository refuses `UNINITIALIZED` at cutover
- THEN the 160 tickets are rows of the TSC-V1-017 flat list and the Corvint cutover proceeds

#### Scenario: PR #4 merged
- WHEN corvint-tasks PR #4 merged three days ago
- THEN the Tasks store is initialized and holds the tickets of the 23 P1 pool that passed the
  seed gate, and no `post-1-0` ticket

### Requirement: TSC-V1-017 Later work as one flat list

Non-seeded later tickets (NEED-LATER, the later-list rows of the C13 verdict table, V1-4382 and
V1-4819) and, until the Tasks store is seeded, the 160 MOVE-TASKS tickets (decided 2026-11-09
under R42, C15) SHALL be published as one generated, read-only tab-separated file in the repository
(proposed `docs/backlog/post-1.0.tsv`), generated from the verified archive and never hand-edited,
with columns old ID, title, kind, priority, old milestone, group, one-line reason and archive
checksum, plus the archive file name and head seq in a header comment. The 49 HEAVY-HOLD tickets
SHALL be neither seeded nor listed; they stay in the archive (R42 (panel pick) C13, C14). It SHALL
NOT be a second queue: no agent SHALL claim work from it; work starts only by creating a ticket.

#### Scenario: someone wants a later item
- WHEN a later item becomes needed
- THEN a new ticket is created in the new store citing the old ID, and the row stays in the list

### Requirement: TSC-V1-018 Closed and removed work stays in the archive only

CLOSE-REMOVED, CLOSE-OBSOLETE, COMPLETED and ARCHIVED tickets SHALL be neither seeded nor listed.
They SHALL remain readable by restoring the archive into a scratch repository.

#### Scenario: reading a closed ticket
- WHEN someone needs closed ticket V1-3480
- THEN the archive is restored into a scratch directory and read with `ticket show` there

### Requirement: TSC-V1-019 Proof after cutover

The cutover SHALL be accepted only when, in the new store: `receipt audit` reports CONSISTENT and
AGREES; `queue status` reports `tickets` equal to the survivor count and no live attempt;
`ticket show` succeeds for every seeded ID and its title, body, acceptance criteria, kind,
priority and labels match the payload; `release show v1-0` lists exactly the seeded IDs; and the
archive still passes `archive verify` with the recorded checksum.

#### Scenario: a seeded ID cannot be read
- WHEN `ticket show` fails for one seeded ID
- THEN the cutover is not accepted and is fixed or rolled back (TSC-V1-020)

### Requirement: TSC-V1-020 Rollback

Until the cutover commit is pushed, rollback SHALL be: move the new `.git/taskman` and `.taskman/`
aside, run `archive restore` on the final-head archive, confirm `receipt audit` CONSISTENT and
AGREES at the final head, and reset the unpushed branch. After the push, rollback SHALL also
revert the cutover commit and repeat the restore on every re-seeded host.

#### Scenario: proof fails
- WHEN TSC-V1-019 fails
- THEN the old store is restored and `queue status` reports 5,280 tickets (or the final-head
  count) and the final head seq

### Requirement: TSC-V1-021 One commit, one PR

The new `.taskman/` tree, the seed manifest and the flat list SHALL land in one commit and one PR
that touches nothing else.

#### Scenario: unrelated file in the cutover PR
- WHEN the cutover PR also edits a Go file
- THEN the PR is split before it is armed

### Requirement: TSC-V1-022 Other hosts re-seeded

Every host or worktree with its own state directory SHALL, after pulling the cutover commit, move
its old state directory aside and restore the new store's export with `archive restore`. No lane
SHALL start on a host until its `queue status` reports the new store's head receipt hash.

#### Scenario: WSL not re-seeded
- WHEN a lane starts on WSL and `queue status` there reads `INTENT_DIVERGED`
- THEN the lane is refused and the host is re-seeded first

### Requirement: TSC-V1-023 Losses are named

The cutover's build-log entry SHALL name what the new store does not carry (TSC-V1-010 and
`design.md` "What is lost") and the archive file and checksum where it stays readable. It SHALL
name V1-3788 and V1-3827 as losses: neither is seeded or listed, and both stay in the archive
only. (decided 2026-11-09 under R42, C17)

#### Scenario: question about an old gate result
- WHEN someone asks for a gate result of a seeded ticket recorded before the cutover
- THEN the build-log entry points to the archive, which holds it

### Requirement: TSC-V1-024 Rehearsal on a copy first

The full recipe SHALL be run once against a copy of the final-head archive restored into a scratch
repository with an `origin` matching `origin/main`, so touchPaths checks run, before it runs on
the real repository. Its counts and refusals SHALL be recorded.

#### Scenario: rehearsal finds a refusal
- WHEN the rehearsal reports a refusal the recipe does not handle
- THEN the recipe is fixed and rehearsed again before the real cutover
