# Tasks: cut the Corvint task store over to a fresh store of 1.0 tickets

Change name: `tasks-store-cutover`. Intent status: proposed. Each task names its requirement IDs
(`specs/tasks-store-cutover/spec.md`) and its proof. Tasks run in the order listed. Steps refer to
`design.md` "Cutover steps". Tasks 1 to 7 change nothing in the live store except the
completions, lease releases and decisions they name; task 8 onward is the cutover window.

## Tasks

1. Owner accepts or amends this change and answers the open decisions in `design.md`.
   Requirements: all. Proof: the acceptance record names this change's commit SHA and each answer.
2. Wait for `ticket-taxonomy` acceptance; record its SHA. Requirements: TSC-V1-001. Proof: the
   accepted taxonomy record.
3. Apply the R42 (panel pick) C13 verdicts to the 18 UNSURE tickets and the HEAVY-HOLD group;
   apply the C17 table to the 7 HELD tickets and the 2 `v1-0` HEAVY-HOLD tickets (decided
   2026-11-09 under R42, C17); freeze the survivor list. Requirements: TSC-V1-002,
   TSC-V1-008. Proof: the frozen list with one decision per UNSURE and HELD ticket.
4. Reconcile the in-flight survivors against `origin/main`; complete delivered ones in the old
   store through the normal completion workflow. Requirements: TSC-V1-003. Proof: per ticket, the
   merged PR or the reason it stays open.
5. Release the expired leases (V1-4399, V1-5162, V1-5212 or whatever `queue status` lists then);
   stop lanes on every host. Requirements: TSC-V1-004. Proof: `queue status` with `liveAttempts`
   empty.
6. Write and review the taxonomy mapping table and the payload script; extend `seed.py` to record
   overrides and touchPaths edits in the manifest. Requirements: TSC-V1-009, TSC-V1-010,
   TSC-V1-012, TSC-V1-013. Proof: the table and a dry run over the dumped survivors with zero
   unplaced tickets.
7. Rehearse the whole recipe on a restored copy of the then-current archive in a scratch repository
   with an `origin` at `origin/main`. Requirements: TSC-V1-024. Proof: rehearsal counts, refusals
   and the TSC-V1-019 proof run against the copy.
8. Steps 1 and 2: audit and export at the final head, verify and checksum. Requirements:
   TSC-V1-005. Proof: `archive verify` output, SHA-256, head seq and head receipt hash, and the
   SHA-256 read back from the WSL and Unraid copies (C16).
9. Steps 3 and 4: dump survivors, build payloads, manifest and `v1-0` payload; re-check the head.
   Requirements: TSC-V1-004, TSC-V1-010. Proof: payload count equals the frozen list; head
   unchanged.
10. Steps 5 and 6: move the old state aside and init. Requirements: TSC-V1-006, TSC-V1-007.
    Proof: moved paths listed; init receipt `000000000001.json`; `nextSerial` value.
11. Steps 7 and 8: seed, release, holds, notes, approvals. Requirements: TSC-V1-011 to
    TSC-V1-015. Proof: seed log with `fails=0`; `release create` receipt.
12. Step 9: flat list, with the 160 MOVE-TASKS tickets as rows of it (C15). Requirements:
    TSC-V1-016, TSC-V1-017, TSC-V1-018. Proof: row counts equal the group counts after task 3
    and task 4 changes.
13. Step 10: proof. Requirements: TSC-V1-019; on failure TSC-V1-020. Proof: the outputs listed in
    `design.md` "Proof after cutover".
14. Step 11: one commit, one PR, build-log entry. Requirements: TSC-V1-021, TSC-V1-023. Proof: the
    PR diff touches only `.taskman/`, the manifest and the flat list.
15. Step 12: re-seed other hosts, then restart lanes. Requirements: TSC-V1-022. Proof: each host's
    `queue status` head receipt hash equals the new store's.

## Acceptance evidence

- `receipt audit` CONSISTENT / AGREES on the old store at the final head and on the new store.
- Archive file name, SHA-256 and `archive verify` output, re-verified after the cutover.
- `queue status` ticket count equal to the frozen survivor count; no live attempt.
- `ticket show` comparison log for every seeded ID; `release show v1-0` member list.
- Count reconciliation: survivors + flat list + MOVE-TASKS + archive-only = old total.
- Seed manifest, flat list and build-log entry in the cutover commit.
- One `queue status` per re-seeded host.
