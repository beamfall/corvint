# V1-1650: checkpoint-resumed writes and reads across note receipts

Owner input: [issue 1072](https://github.com/beamfall/corvint/issues/1072), owner P0. After an
operator note, every write and most reads audited the whole journal from receipt 1: about 45 s of
CPU per write at 47k receipts. ON-V0-012..014 were added to
`docs/specs/corvint-tasks-operator-notes-v0.md`. They amend the CAL-V0-116 decline bullet and the
CAL-V0-061 tail rule; no CAL-V0 ID was added, because CAL-V0 is at its 256 bound. They are accepted
under the owner's standing auto-accept of lane requirements (2026-10-11). Delivery stays
experimental.

## Root cause, confirmed before the fix

Both resumed walks start with no note references and with `policyWalked` false: `walkWriter` (writer
checkpoint) and `walkTail` (read checkpoint). For any tail note receipt whose pre-ticket or
pre-policy was last posted at or before the checkpoint, `noteAudit.bindEvent` therefore declined
with "note transition pre-state precedes the checkpoint". CAL-V0-116 mandated that decline.

- Writes fell to the complete route, which also re-based the writer checkpoint, so the cost recurred
  after the next note. Lease HEARTBEAT and RELEASE share the writer audit and declined the same way.
- Reads fell back to FULL.

`TestONV0012_WriterResumesAcrossNoteReceipts` was written first. On unmodified `origin/main`
(`94cf903888d3a932915401c42b0e7a7a676752cd`), the write after the first note declined with exactly
that message (`fast.declined: UNSUPPORTED: intent/tickets/AT-0053.json: checkpoint unusable: note
transition pre-state precedes the checkpoint`).

## Design

- A resumed walk records the receipt it resumed after (`chain.resumed`).
- `bindEvent` keeps the old decline only for a complete walk, where it is unreachable for a
  well-formed chain.
- A resumed walk reads the note's pre-ticket and pre-policy with the existing `priorPost`: from the
  one receipt that last posted each, whose post must equal the checkpoint entry's digest and whose
  bytes must hash to it. Any failure there is `errCheckpoint`, so the complete route or the FULL
  audit decides (ON-V0-014).
- For a pre-ticket last posted before the writer checkpoint, its note reference must also equal the
  reference the writer checkpoint carries (`sameNote`).
- The prior event is read content-addressed, as before.

Deviations from the charter's design:

- The charter proposed seeding `chain.notes` from the writer checkpoint. Instead, the reference is
  cross-checked at the pre-ticket. The effect is the same, and the walk's "walked" bookkeeping is
  unchanged for the V1-1618 lane, which edits the same functions.
- A note event reads up to **two** digest-bound prefix receipts (pre-ticket and pre-policy), not
  one. The policy is usually last posted far before the checkpoint.
- Reads use the read checkpoint's existing entry digests, with no profile bump and no reading of the
  writer checkpoint. CAL-V0-061 already gives entries that trust, so no detection is weakened. The
  profile is unchanged, so rollback is a plain revert.

## Measurements

Profile: `TestONV0012_NoteInterleavedHistoryProfile`, run with `CORVINT_TASKS_NOTE_PROFILE` and
`CORVINT_TASKS_HISTORY_SIZES`.

- Each of three repetitions sets a note on a ticket last posted in the prefix. It then reads from
  the retained read checkpoint, then does a CREATE.
- The figures are medians in ms, wall / process CPU.
- Hardware: Apple M2 Max, 12 CPUs, 64 GB, go1.27.2.

| receipts | build | mutate after note | read after note | note set | declines | read mode |
|---|---|---|---|---|---|---|
| 2000 | before | 17306 / 2935 | 11882 / 2169 | 542 / 95 | 3/3 | FULL |
| 8000 | before | 54714 / 9717 | 38495 / 7187 | 840 / 118 | 3/3 | FULL |
| 2000 | after | 108 / 82 | 12 / 11 | 112 / 75 | 0 | CHECKPOINT_PLUS_TAIL |
| 8000 | after | 116 / 89 | 9.5 / 10.5 | 113 / 88 | 0 | CHECKPOINT_PLUS_TAIL |
| 47000 | after | 219 / 171 | 9.9 / 11.2 | 224 / 188 | 0 | CHECKPOINT_PLUS_TAIL |

Load averages during the runs:

- Before: about 64-67, observed with `uptime`; the report recorded `NOT_OBSERVED`.
- After, as recorded in each report: 2000: 67.2 74.8 70.7; 8000: 42.7 67.6 68.3; 47000: 26.8 44.5 57.6.

The "before" wall times are inflated by load. Its CPU grows about 1.2 ms per receipt, which
extrapolates to roughly 57 s at 47k and matches the issue's 45 s. A "before" run at 47k was not made
(NOT_RUN): three full audits per phase would exceed the 600 s command bound.

## Other candidate causes (inspected, not changed)

- Tail over 256, or head 4096 or more past `FullSeq`: fast writes re-base the writer checkpoint at a
  64-receipt tail, so the 256 bound is reached only when receipts accrue without writer-route writes.
  The 4096 bound is reached only when scheduled refreshes keep failing. Neither caused this issue.
- Compaction (`store/compact.go`): a non-dry-run compaction invalidates the writer checkpoint and
  removes the read checkpoint. The next write and the next read each run one complete audit; a
  one-time cost by design.
- `refreshWriterCheckpoint` (`store/writer_checkpoint.go`): this is an inference from reading the
  code; it was not measured.
  - Once the head is 512 or more past `FullSeq`, every writer-route write schedules a refresh. The
    refresh runs a complete audit in the writing process after the lock is released.
  - A refresh that publishes nothing leaves `FullSeq` behind, so the next write pays a complete
    audit again. That happens on an intent-only divergence, a cleanup observation, a moved snapshot
    that the CAL-V0-265 prefix walk does not settle, a lock failure, supersession, or a newer
    checkpoint.
  - With this fix, the refresh is now the remaining per-512-receipt complete audit on the write path.
    A refresh that fails persistently would bring back a per-write full audit until 4096.
  - This is reported for a separate ticket, not fixed here.

## Deviations and NOT_RUN

- The first "before" profile invocation (sizes 2000 and 8000 together) exceeded the 600 s command
  bound, and the tool moved it to the background. It completed with exit 0. Later invocations used
  one size each.
- Whole-package `internal/tasks/store` did not fit the 600 s bound under load. Two attempts are
  recorded:
  - The non-CAL group was killed by `timeout 585`.
  - The note, writer-checkpoint, liveness, lease-resume, pool-resume and history-profile tests were
    run instead, in three groups.
- `internal/tasks/journal` and the cli checkpoint and note tests ran whole.
- NOT_RUN: `make gate`; `go test ./...`; the rest of `internal/tasks/store`; the other packages the
  `corvint affected` plan selected through unbounded-reader edges; a live 47k-receipt store; a
  "before" run at 47k.
- Dogfood intents: the first `make dogfood-finish` bind refused `ocm-aggregate:
  too-many-obligations`, because the agent-leases spec alone carries 256 CAL-V0 requirements. That
  spec was dropped from the intents file (DOGFOOD.md, "Choosing intents when the aggregate refuses a
  bound"); the owning spec, operator notes (ON-V0-012..014), stays. The leases constraints this
  change still relies on are the CAL-V0-116 writer decline list and the CAL-V0-061 tail rule. The
  journal hunks cite the amended CAL-V0-116 decline list in the citation plan. Omission is not a
  claim that those constraints hold; the focused tests above are the evidence.

## Review round 1 (Codex)

- P1: the read resume skipped a tail post of a ticket whose prior reference it did not know. A
  forged tail REFINE that dropped a prefix note reference, followed by a valid note event for
  another ticket, kept `CHECKPOINT_PLUS_TAIL`; the complete audit and the writer route (through the
  writer checkpoint's note reference) refused it as JOURNAL_FORKED. The read now binds that
  reference from the one prefix receipt the checkpoint entry names, by digest, and falls back on a
  difference (ON-V0-013). `TestONV0014_ReadAndWriterResumesBindPrefixState` asserts the read and
  writer resumes refuse the same forged tails, with the no-checkpoint decision as oracle.
- P2: a prefix receipt with another `seq` but the same posted record and digest was accepted on both
  resumed routes. The prefix receipt's sequence, previous-digest presence and head generation are
  now checked against its chain position (ON-V0-014). Its previous digest's value is not
  verifiable without reading the receipt before it, and stays unchecked on resumed routes; the
  complete audit checks it.
- Cost: the read resume now reads one prefix receipt for each ticket first posted in its tail by a
  non-note receipt. This was not re-measured.
- Both regression tests failed with the round-0 `operator_note.go` (the read stayed in checkpoint
  mode; the writer completed the prefix-seq case fast) and pass with the fix.

## Concurrent note writes (issue comment, LOCK_TIMEOUT)

The issue reported that at about 47.9k receipts, 9 concurrent `ticket note set` calls from
separate processes, each on its own ticket, left 5 failing with LOCK_TIMEOUT. Each write held the
store lock for a complete audit.

Profile: `TestONV0012_ConcurrentNoteWritesAfterNote`, run with `CORVINT_TASKS_NOTE_CONCURRENCY` and
`CORVINT_TASKS_HISTORY_SIZES`.

- Setup: a CREATE and then a NOTE_SET on the first ticket.
- Then 9 processes re-executing the test binary (fresh processes, as CLI calls are) are released at
  the same instant. Each sets a note on a different ticket last posted in the prefix.
- The test fails on any LOCK_TIMEOUT or any write that does not commit.
- Per-write wall times include waiting for the store lock.

| receipts | build | committed | LOCK_TIMEOUT | writer declines | per-write wall, min / max ms | report load (1/5/15 min) |
|---|---|---|---|---|---|---|
| 2000 | before (journal at the lane base) | 4 | 5 | 4 (note pre-state precedes the checkpoint) | 15449 / 31413 | 70.4 63.3 59.6 |
| 2000 | after | 9 | 0 | 0 | 668 / 6631 | 53.9 57.8 55.4 |
| 20000 | after | 9 | 0 | 0 | 153 / 1516 | 56.7 61.2 59.6 |
| 47000 | after | 9 | 0 | 0 | 986 / 9183 | 60.9 55.5 56.4 |

The "before" row reproduces the issue's 5 of 9 at 2000 receipts. Each complete audit took about
15 s under load, so the queue passed the 30 s lock wait. After the fix, the last of the nine writes
commits within 9.2 s at 47k receipts. A first 47k attempt was killed by the 580 s command bound
during fixture setup, under the load of other lanes, and wrote no result. The retry completed in
442 s.
