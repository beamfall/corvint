# Design: cut the Corvint task store over to a fresh store of 1.0 tickets

Change name: `tasks-store-cutover`. Intent status: proposed. Requirement IDs refer to
`specs/tasks-store-cutover/spec.md`.

## Owner decisions applied (2026-11-09)

- "1029 open ticket is nuts"; "we should just remove the closed ones."; "Purge old closed".
- T1 "Fresh store": a new store is initialized; the old one is kept as a verified archive
  (TSC-V1-005, TSC-V1-006, TSC-V1-007).
- T2 "1.0 tickets only": only 1.0 survivors are seeded (TSC-V1-008).
- T3 new ticket categorization: designed in change `ticket-taxonomy`; seeded tickets take that
  shape and this cutover waits for its acceptance (TSC-V1-001, TSC-V1-009).

## Facts and their verification

VERIFIED means checked read-only on 2026-11-09 by the author of this change. NOT_VERIFIED means
taken from another agent's notes and not re-checked.

| Fact | Status and evidence |
|---|---|
| 5,280 tickets: OPEN 1,029, HELD 7, COMPLETED 2,560, ARCHIVED 1,684 | VERIFIED, `queue status` |
| Journal head seq 24185, head receipt `f4a2aaf2...6122` | VERIFIED, `queue status` |
| `receipt audit` CONSISTENT / AGREES at 24185 | VERIFIED |
| No purge verb; `ticket archive` keeps the record | VERIFIED, `help --json` |
| Archive `...-head24185.tar` SHA-256 `b8f11f84...2bf6` | VERIFIED, matches its `SHA256` file |
| `archive verify` on it: exportedAtSeq 24185, 58,945 files, same head receipt hash | VERIFIED |
| Three live attempts, all `LEASE_EXPIRED`: V1-4399, V1-5162, V1-5212 | VERIFIED, `queue status` |
| `merged.tsv` groups: 164/414/160/176/48/49/18 | VERIFIED, `cut -f2 \| uniq -c` |
| 102 of 164 NEED-1.0 not OPEN in lifecycle | VERIFIED, `survivors-draft.tsv` column `lc` |
| NEED-1.0 with milestone `post-1-0` | VERIFIED as 43 (the brief said 40) |
| Old `v1-0` release lists 377 tickets | VERIFIED, `.taskman/releases/v1-0.json` |
| `queue.json` `nextSerial` 5263 in working tree, 2446 on `origin/main` | VERIFIED |
| 2,005 tracked files under `.taskman/` on the primary checkout | VERIFIED, `git ls-files` |
| `corvint-tasks` repository store not initialized | VERIFIED, refused `UNINITIALIZED` |
| Tool version `1.0.0-rc.3.dev+build.144`, go1.27.3 | VERIFIED, `corvint-tasks version` |
| Classification of each ticket is correct | NOT_VERIFIED (agent classification, sampled only) |
| `import --file` refused for a NATIVE queue | NOT_VERIFIED (rehearsal note) |
| Duplicate-check refusals and override behavior | NOT_VERIFIED (rehearsal note) |
| touchPaths refusals for removed paths | NOT_VERIFIED, not observable without `origin` |
| `archive restore` round trip byte-identical | NOT_VERIFIED (rehearsal note) |
| Other hosts read `INTENT_DIVERGED` after pull | NOT_VERIFIED (inferred in rehearsal) |
| `release create` refuses unknown members and predecessors | NOT_VERIFIED (rehearsal note) |

### Recipe steps re-checked

The recipe `scratchpad/tix/seed-recipe.md` and its helpers `to_payload.py` and `seed.py` were
read in full. In a new scratch repository (`scratchpad/tsc-rehearse.Gd8W`) these steps were re-run:

1. `init` with only `policy.json` and `queue.json` copied from the live repository: OK.
2. `ticket show V1-0019` on the live store, `to_payload.py` with a one-line seeded list: the
   dependency on V1-0018 was dropped and reported; the operator note was reported uncarried.
3. `ticket create --request-id seed-V1-0019`: created `ticket:corvint:main:V1-0019`, status OPEN
   (old lifecycle IN_PROGRESS).
4. The same create again: `replayed true`, nothing written.
5. `receipt audit`: head 2, CONSISTENT, AGREES; `queue status` tickets 1.

Not re-checked: duplicate overrides, `release create`, holds, `archive restore`, `import`, timing.

## Survivor set

| Group | Count | Disposition |
|---|---|---|
| NEED-1.0 | 164 | Seeded, milestone `v1-0`, minus those completed under TSC-V1-003 |
| UNSURE | 18 | R42 (panel pick) C13 verdict table below (TSC-V1-002) |
| MOVE-TASKS | 160 | Flat list at cutover; Tasks store within 3 days of PR #4 (C15; TSC-V1-016) |
| NEED-LATER | 414 | Flat list (TSC-V1-017) |
| HEAVY-HOLD | 49 | Archive only, not seeded or listed (R42 (panel pick) C13); 2 carry `v1-0` |
| CLOSE-REMOVED | 176 | Archive only (TSC-V1-018) |
| CLOSE-OBSOLETE | 48 | Archive only |
| HELD | 7 | Not in `merged.tsv` (it covers OPEN only); C17 table below; none seeded here |
| COMPLETED, ARCHIVED | 4,244 | Archive only |

1,029 + 7 + 4,244 = 5,280, the old store's total at head 24185.

UNSURE verdicts (R42 (panel pick) C13). Later-list rows go to the TSC-V1-017 file; none is seeded.

| ID | Verdict |
|---|---|
| CORE-ROADMAP-SECTION-DISCOVERY-1 | Refile in beamfall/core; not seeded |
| V1-0014 | Close as superseded (G8) |
| V1-0184 | Close-removed (dogfood ceremony cut by G8) |
| V1-0473 | Close-removed (bound to the removed proof format) |
| V1-1419 | Later list; goes away at the CI cut-over (G18, G27) |
| V1-1666 | Close-removed (proof lab cut) |
| V1-1843 | Close-removed (R14, G23: provider binaries retired) |
| V1-2281 | Close-obsolete; the untouched-repo check is new work under V1-0019 |
| V1-2381 | Later list, attached to the G26 `liveverify/mutate` ticket |
| V1-2387 | Close-removed (process bookkeeping) |
| V1-2555 | Close-obsolete after reading V1-3177; 1.0 cuts a new candidate |
| V1-2643 | Later list |
| V1-3403 | Refile in beamfall/core; not seeded |
| V1-4084 | Later list; constraint recorded in the agent qualification runbook (R39) |
| V1-4129 | Close-obsolete (no reproduction kept) |
| V1-4172 | Close-removed if it is the dogfood hook (G8); otherwise later list (one check) |
| V1-4819 | Later list; not seeded (G23 replaces the 2026-11-06 acceptance) |
| V1-4856 | Later list, with validity reuse struck (G24) |

V1-4382 is not seeded and goes to the later list. V1-2691 (HEAVY-HOLD, in flight) is finished or
released before the cutover.

HELD and `v1-0` HEAVY-HOLD verdicts (decided 2026-11-09 under R42, C17). All seven HELD tickets
carry hold `pr50-close-exclude`; PR #50 merged 2026-11-02. None of the nine is seeded in the
Corvint store (TSC-V1-002).

| ID | Priority | Subject | Verdict |
|---|---|---|---|
| V1-4243 | P1 | Journal seal 97 to 102 s at 40k receipts | Tasks later list; seal time is one timed row of the C8 profile; returns to 1.0 only if the budget set at the qualification candidate fails |
| V1-3763 | P2 | `test --dry-run` selects 181 units for a mostly-spec change | Not seeded; its case becomes one scenario on the doc-only-change requirement's delivery ticket |
| V1-3788 | P2 | `affected` selects 77 units for a Makefile and script change | Not seeded; obsolete under G15b (a Makefile change is never narrowed); the unmerged `mkr-w37` lane is abandoned; named as a loss (TSC-V1-023) |
| V1-3827 | P2 | `--scope-from-diff` refuses when the diff exceeds scope | Not seeded; reverted and declined; the `finish` verb (TKF-V1-005, C2) covers the need (overlap inferred from titles, NOT_VERIFIED in the ticket body); named as a loss (TSC-V1-023) |
| V1-4229 | P2 | Ticket write decodes every ticket | Seeded in the Tasks store, remaining work only, as the write-path item of the C8 profile |
| V1-4263 | P2 | Refused `ticket create` costs 820 to 850 ms and 310 MB | Tasks later list |
| V1-1148 | P3 | Scale generator | Tasks later list |
| V1-4841 | HEAVY-HOLD, `v1-0` | Lane-host ticket | Not seeded, per C13; stays in the archive |
| V1-5019 | HEAVY-HOLD, `v1-0` | Lane-host ticket | Not seeded, per C13; its in-flight attempt is finished or released before the quiet-store step (TSC-V1-003, TSC-V1-004) |

The UNSURE set includes `CORE-ROADMAP-SECTION-DISCOVERY-1`, whose ID is not in the `V1` prefix;
whether `localToken` accepts it in this queue is NOT_VERIFIED (failure mode 9).

## Cutover steps

Preconditions, all required before step 1: `ticket-taxonomy` accepted (TSC-V1-001); 18 UNSURE
decided (TSC-V1-002); in-flight survivors reconciled and delivered ones completed in the old store
(TSC-V1-003); expired leases released, no live attempt, lanes stopped on Mac, WSL and Unraid
(TSC-V1-004); the field mapping table to the taxonomy reviewed (TSC-V1-009); rehearsal on a copy
passed (TSC-V1-024).

1. In the real repository, on a branch off `origin/main`: `queue status` and `receipt audit`;
   record head seq H and head receipt hash; audit must be CONSISTENT / AGREES.
2. `archive export > /Users/russelllewis/corvint-store-archives/corvint-main-<date>-head<H>.tar`;
   `archive verify` it; record SHA-256; check exportedAtSeq = H and lastReceiptSha256 = step 1
   hash (TSC-V1-005). Also `tar` the on-disk `.taskman/` tree beside it. Copy the archive, its
   `SHA256` file, `verify.json` and `export.envelope.json` to the WSL benchmark host and to the
   Unraid array, and check each copy against the recorded SHA-256 (C16).
3. `ticket show` every frozen survivor from the old store into a dump directory; build payloads
   with the reviewed mapping (old fields to taxonomy shape, milestone `v1-0`, dependencies and
   supersedes filtered to survivors, `localToken` per decision 2, R42 C12); sort topologically; write
   the seed manifest (TSC-V1-010). Build the `v1-0` release payload filtered to survivors.
4. `queue status` again: head must still be H, else go back to step 1 (TSC-V1-004).
5. Move `.git/taskman` to `/Users/russelllewis/corvint-store-archives/state-head<H>/`; move the
   on-disk `.taskman/tickets/` and `.taskman/releases/` there too; `git rm -r --cached` both
   trees (TSC-V1-006).
6. Keep `.taskman/policy.json` and `.taskman/queue.json` with `nextSerial` at least the old value
   (TSC-V1-007); `corvint-tasks init`.
7. Seed with the replay-safe loop (TSC-V1-011 to TSC-V1-013); expect zero failures. Fix refusals
   per TSC-V1-012 / TSC-V1-013 and rerun.
8. `release create --target v1-0` (TSC-V1-015); reapply holds, notes and approvals marked wanted
   (TSC-V1-014).
9. Write the flat list (TSC-V1-017), with the 160 MOVE-TASKS tickets as rows of the same file
   (C15; TSC-V1-016). The Tasks store is not initialized by this cutover.
10. Proof (below). On failure, rollback.
11. One commit with `.taskman/`, manifest and flat list; one PR (TSC-V1-021); build-log entry
    naming losses, archive and checksum (TSC-V1-023).
12. After merge, re-seed every other host (TSC-V1-022), then restart lanes.

## Proof after cutover (TSC-V1-019)

- `receipt audit`: `structuralConsistency CONSISTENT`, `projectionAgreement AGREES`.
- `queue status`: `tickets` = survivor count; `byStatus` OPEN + HELD = survivor count; no live
  attempt; `openWithoutMilestone` 0.
- `ticket search --milestone v1-0 --count` = survivor count.
- `ticket show` for every seeded ID, compared field by field to its payload.
- `release show v1-0`: member list equals the seeded list.
- `archive verify` on the archive again, SHA-256 equal to the recorded value.
- Counts reconcile: survivors + listed later + MOVE-TASKS + archive-only = old total.

## Rollback (TSC-V1-020)

Before push:
1. `mv .git/taskman /Users/russelllewis/corvint-store-archives/state-new-<date>`.
2. Move the whole `.taskman/` directory aside (restore refuses `INTENT_DIVERGED` if a differing
   intent file is present, per the rehearsal note).
3. `corvint-tasks archive restore` on
   `/Users/russelllewis/corvint-store-archives/corvint-main-<date>-head<H>.tar`.
4. `receipt audit` CONSISTENT / AGREES; `queue status` head seq H and the old ticket count.
5. Reset the unpushed branch to its base.

After push: steps 1 to 4, then revert the cutover commit through a PR, then on each re-seeded host
move its state directory aside and restore the same archive. Faster path when nothing was written
after step 5: move `state-head<H>` back to `.git/taskman` and untar the saved `.taskman/`
(NOT_VERIFIED; not rehearsed).

## What is lost and where it stays readable

The new store does not carry: links to non-seeded tickets (dependencies, supersedes), revision
history and `acceptanceRevision`, created and updated times, holds (re-placed with a new time),
progress rows, operator notes, approvals, attempts, claims, leases, gate results, evidence, CEM
verdicts, obligation plans, release history (old `v1-0` at revision 149 per the rehearsal note,
NOT_VERIFIED) and the predecessor chain, and all non-1.0 releases. All of it stays readable by
restoring the archive into a scratch directory and reading it with the normal read verbs. The
manifest keeps the parts that matter for open work.

## Other hosts

WSL (`russell@.128:2222`) and Unraid (`agent@.3:2222`) lanes and any other worktree with its own
`.git/taskman` keep the old store until re-seeded. Lanes there stop before step 1. After merge,
each host moves its state directory aside and restores an export of the new store (or reruns init
and seed from the committed payloads). Each host's `queue status` must show the new head receipt
hash before a lane starts (TSC-V1-022). Which hosts hold a state directory today is NOT_VERIFIED.

## Failure modes

1. Head moves after export: export is stale; repeat steps 1 and 2 (TSC-V1-004, TSC-V1-005).
2. Init refuses `INTENT_DIVERGED`: ticket or release files left in `.taskman/`; move them.
3. Duplicate check refuses similar survivors; overrides bounded and recorded (TSC-V1-012). IDF
   shifts as tickets are added, so a rerun may pass where a first try failed.
4. touchPaths refusals for paths the cut removed (TSC-V1-013).
5. Dependency cycle among survivors: create without the back edge, then `set-dependencies`.
6. Stale `nextSerial` reuses old numbers (TSC-V1-007).
7. A delivered ticket is seeded as open work, or an undelivered one is closed (TSC-V1-003).
8. Taxonomy mapping cannot place a ticket (TSC-V1-009).
9. A non-`V1` ID (one UNSURE ticket) is refused as `localToken`; it is created with a new ID and
   the old ID kept in the manifest.
10. Another host writes to its old store after the cutover and its work is lost: lanes are
    refused until re-seeded (TSC-V1-022).
11. The flat list becomes a second queue: TSC-V1-017 forbids claiming from it.
12. The archive is unreadable later because the tool format changes: keep the binary version with
    the archive and re-verify on tool upgrades, at the Tasks RC tag and at stable (C16).
13. The single archive copy on this Mac is lost: two off-host copies exist (TSC-V1-005). The
    Unraid array has needed a manual unlock after a reboot, so that copy may be unreadable when
    wanted; that is why there are two (C16).
14. For about a week the 23 P1 Tasks tickets live only in the flat list, with no claims or gates,
    while `tasks-hardening` work is active (C15). The three-day rule of TSC-V1-016 bounds it.

## Open decisions for the owner

1. Decided, R42 (panel pick) C14: the later list is one generated, read-only file in the
   repository (proposed `docs/backlog/post-1.0.tsv`; TSC-V1-017). The MOVE-TASKS sub-question
   is decided 2026-11-09 under R42, C15: the 160 are rows of the same file until the Tasks store
   is seeded (TSC-V1-016).
2. Decided, R42 (panel pick) C12: seeded tickets keep their old IDs (`localToken`); the serial
   continues from 5263 (TSC-V1-010).
3. Decided, R42 (panel pick) C11: the cutover runs at M0, after the 1.0 product plan and the ticket
   scheme are accepted, as one cutover before the Tasks trial clock starts (TSC-V1-001).
4. Decided, R42 (panel pick) C13: the 18 UNSURE tickets follow the verdict table above.
5. Decided, R42 (panel pick) C13: HEAVY-HOLD (49) are not seeded and stay in the archive.
6. Decided 2026-11-09 under R42, C15: the 160 wait in the flat list and the Corvint cutover does
   not wait for the Tasks store. The store of `beamfall/corvint-tasks` is initialized as its own
   change (`tasks-store-init`, in that repository) within three days of PR #4 merging (target
   2026-11-16) and seeded through the seed gate with only tickets that name a PROD-TASK, TKF, TTX
   or TCF requirement. Starting pool: the 23 P1 `v1-0` tickets. The 137 `post-1-0` tickets (56
   P2, 81 P3) go to that repository's later list (TSC-V1-016). Reason recorded by the panel: PR
   #4 had not merged, so seed gate rule 6 (paths must exist at the seed commit) could not pass
   there. Owner act that remains: the owner's machine-wide instruction file says Tasks gaps
   belong in the Corvint queue; once the Tasks store exists that line needs the owner's edit.
7. Decided 2026-11-09 under R42, C16: the archive tar (220,508,160 bytes), its `SHA256` file,
   `verify.json` and `export.envelope.json` are kept for the whole 1.x line. The moved state
   directory is kept until 30 days after stable (2027-02-04 if stable is 2027-01-05), then
   deleted after `archive verify` passes. Two off-Mac copies, on the WSL benchmark host and on
   the Unraid array, are each checked against the recorded SHA-256 and re-checked at the Tasks RC
   tag and at stable. No cloud upload (TSC-V1-005, TSC-V1-006).
8. Decided 2026-11-09 under R42, C17: the 7 HELD tickets and the 2 HEAVY-HOLD tickets marked
   `v1-0` follow the C17 table above. None is seeded in the Corvint store; V1-4229 is seeded in
   the Tasks store; V1-4243, V1-4263 and V1-1148 go to the Tasks later list (TSC-V1-002,
   TSC-V1-023).
