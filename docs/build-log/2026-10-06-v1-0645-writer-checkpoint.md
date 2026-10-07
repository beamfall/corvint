# 2026-10-06: V1-0645 writer checkpoint (issues 641 and 642)

## Intent

Ticket V1-0645 (P1) is the writer checkpoint (C) that S20 of
`docs/specs/corvint-tasks-agent-leases-v0.md` deferred. The same change delivers owner requests
beamfall/corvint#641 and #642 (native ticket V1-0887).

- Issue 641: on the 13,121-receipt Flow-Proof queue every corvint-tasks write costs 30 to 43 s. The
  owner's rule (2026-10-06) is that writer cost must be milliseconds, not seconds. A heartbeat, renew
  or claim on a synthetic store of at least 13,000 receipts must take well under 1 s (median of 5,
  load band recorded) and stay flat from 3,000 to 13,000 receipts.
- Issue 642: heartbeats and renews are about 90% of queue writes, and each pays the full-history
  audit.
- Ticket criterion 1 asks for a decision on whether, and under which durability and rebinding rules,
  a writer may skip the receipt prefix. This entry and the spec section "V1-0645 writer checkpoint"
  are that decision.

## Decision

A writer may skip the receipt prefix when a derived writer checkpoint vouches for it. The contract
is `CAL-V0-115..119` in the spec, all PROPOSED until the owner accepts them with the batch 4 list.

- Durability. The checkpoint is derived state at `<git common dir>/taskman.writer-checkpoint`, beside
  the state directory and never inside it. It is written with a temporary file opened exclusively and
  a rename, without fsync. A missing, torn, foreign, corrupt or outrun checkpoint falls back to the
  complete audit, so it never refuses a valid store and never admits a write the complete route
  would refuse because of the checkpoint itself.
- Rebinding. The writer audit rebinds the queue ID, primary worktree, init digest and generation,
  and the named receipt's sequence, generation and digest (as CAL-V0-061 does). It requires receipt
  names 1..head on disk, walks the tail with the complete audit's validators, and compares every
  latest afterimage with its projection.
- Bounds. Tail at most 256, else decline. Head 4,096 or more past `FullSeq`, else decline. Advance
  the checkpoint at a tail of 64. Schedule a complete refresh at 512 past `FullSeq`. Never keep a
  checkpoint below an audited head of 128.
- Scope. `Mutate` (except REOPEN and the review operations) and the lease verbs CLAIM, CLAIM_NEXT,
  RENEW, HEARTBEAT and RELEASE. Every decline, refusal, possible replay or elided-path read hands
  the write to the complete route, which decides it.
- `receipt audit` is unchanged. It never reads, writes or removes either checkpoint. Its `--help`
  names both files and says that removing `<state directory>.writer-checkpoint` forces the next
  write through the complete audit.

Coordinator decisions on the lane's owner questions (owner delegated in-task decisions, 2026-10-06):

1. The lease lock hold rising from about 0.1 s to about 0.45 s (0.43 to 0.51 s measured) is accepted for this landing as a
   measured regression of the hold metric. Wall time fell, and the hold falls with the next cut. No
   optimistic-observe design now.
2. Fast writes also advance the read checkpoint (CAL-V0-119). This costs about 1 ms
   (`readCheckpoint` phase below), needs no extra fsync and no extra tree read, and removes the
   `TestCALV0062_OpenWorkflowRefusesBeforeMutation` behavior change.
3. Refresh interval 512 is kept. The write that triggers a refresh pays a complete audit after its
   lock is released. This is a known latency spike: at 13,000 receipts it is about the cost of a
   complete-route write (the first, unseeded mutate at 13,000 receipts took 8,841 ms; inference,
   because the refresh itself is not a separate profile phase). The follow-up lane makes it
   incremental.
4. No fsync on the writer checkpoint; recorded as a spec failure mode.
5. Threshold 128 and backstop 4,096 accepted.
6. Claim facts and the scope deriver run under the lock on the fast route; accepted.
7. Refused and replayed writes keep the complete route's cost; follow-up.
8. The guarantee change is recorded as PROPOSED in CAL-V0-116. A fast writer does not detect prefix
   receipt or request tamper, evidence or pinned content tamper, a stray request file other than its
   own, a consistent forged checkpoint, or non-cooperating private-state edits between audits.
   `receipt audit`, the complete route and the scheduled refresh still refuse all of them. File stat
   stamps are not adopted as evidence.
9. Fast-route tamper coverage is its own test (below), not only a skip.

## Change

- `internal/tasks/journal/writer_checkpoint.go`: closed binary codec, `AuditForWriter` and the tail
  walk. The journal package does not write or lock.
- `internal/tasks/transaction`: an inventory that summarizes receipts 1..Seq from the checkpoint
  counts, with archive cost and limits equal to the complete inventory, and an incomplete mark when
  the model reads an elided path.
- `internal/tasks/archive`: order-independent file-set cost, so the summarized and complete costs
  agree.
- `internal/tasks/store/writer_checkpoint.go` and `writer_route.go`: reading, retaining, advancing and
  refreshing the checkpoint; the fast mutate route (observe, model, bind, readCheckpoint, apply) and
  the fast lease route (observe, model, bind, readCheckpoint, commit).
- `internal/tasks/cli/command_help.go`: the `receipt audit` help note.
- The spec section and the S12, S20, status, slice, failure-mode and traceability edits.

## Measurements

All runs use `TestCALV0118_WriterHoldProfile` on synthetic stores only, on one macOS host. Each cell
is the median of 5 wall times in ms, with the lock hold after the slash. "Before" is fe5ace4a (the
profile and the order-independent cost, no checkpoint route), which predates the batch 3 merge
ce4efeb5. "After" is this branch. Every before sample took the complete route; every after lease
sample took the fast route. The first after mutate in each row is the complete route that seeds the
checkpoint (its time is in parentheses); the median includes it.

| Receipts | Tickets | Run | Load band | mutate | claim | renew | heartbeat | release |
|---|---|---|---|---|---|---|---|---|
| 2,000 | 880 | Before | 8.09 to 8.65 | 1,711 / 1,700 | 1,441 / 116 | 1,388 / 97 | 1,378 / 98 | 1,433 / 115 |
| 2,000 | 880 | After | 8.17 to 8.02 | 432 / 427 (1,757) | 470 / 461 | 424 / 415 | 421 / 413 | 431 / 422 |
| 3,000 | 880 | Before | 14.96 to 13.94 | 2,946 / 2,932 | 2,430 / 141 | 2,319 / 132 | 2,424 / 129 | 2,385 / 151 |
| 3,000 | 880 | After | 6.86 to 6.51 | 479 / 474 (2,439) | 523 / 510 | 441 / 429 | 449 / 440 | 475 / 466 |
| 13,000 | 880 | Before | 11.67 to 7.10 | 9,039 / 9,001 | 7,978 / 142 | 7,934 / 120 | 7,881 / 131 | 7,913 / 145 |
| 13,000 | 880 | After | 6.59 to 6.14 | 458 / 453 (8,841) | 483 / 473 | 436 / 428 | 455 / 446 | 480 / 471 |
| 13,000 | 9,990 | After | 7.35 to 18.37 | 3,308 / 3,301 (10,680) | 3,495 / 3,482 | 3,274 / 3,263 | 3,140 / 3,129 | 3,272 / 3,262 |

Load band is the 1-minute load average before and after the row. The 3,000 and 13,000 rows were run
back to back on the same generated stores (before 22:55 to 23:01, after 23:01 to 23:03). The before
run overlapped another lane's store tests, so its band was higher (15 to 7) than the after band (7
to 6). An earlier before run on a quieter host (load 4.9 to 5.8) measured 7,212 ms for a heartbeat
at 13,000 receipts, so the band does not explain the difference. The 2,000-receipt rows and the
9,990-ticket row come from separate runs.

- Heartbeat at 13,000 receipts: 7,881 to 455 ms (about 17x). Renew 7,934 to 436, claim 7,978 to
  483, release 7,913 to 480, mutate 9,039 to 458.
- After, 3,000 to 13,000 receipts: heartbeat 449 to 455, renew 441 to 436, claim 523 to 483, release
  475 to 480, mutate 479 to 458. Flat within the spread of the samples.
- Before, the same writes grow with receipts: heartbeat 1,378 (2,000), 2,424 (3,000), 7,881
  (13,000).
- Lock hold of a lease write: about 0.1 to 0.15 s before, about 0.43 to 0.51 s after.

Per-phase medians (ms), after, 13,000 receipts and 880 tickets:

- heartbeat: observe 139.0, model 168.2, bind 34.4, readCheckpoint 1.0, commit 100.9 (journal write
  85.4, fsync 18.5).
- mutate: observe 128.3, model 167.5, bind 30.8, readCheckpoint 1.0, apply 87.7.

Before, 13,000 receipts, heartbeat: snapshot read (the complete audit, outside the lock) 6,048,
validation 918, journal write 104, fsync 20.5, lock hold 131. Before, 2,000 receipts: snapshot read
900, validation 265, journal write 76, fsync 19, lock hold 98.

Ticket count. At 13,000 receipts and 9,990 tickets (the scale-tasks "D" shape is 10,000 tickets;
`wire.MaxTicketsPerQueue` is 10,000, so the profile's five creates would refuse LIMIT_EXCEEDED at
10,000, and the run used 9,990), heartbeat phases are observe 1,338, model 1,161, bind 505,
readCheckpoint 9.2, commit 120. Mutate phases are observe 1,223, model 1,188, bind 499, apply 114.
From 880 to 9,990 tickets the remaining cost grows about 7x. It follows the intent tree (tree reads,
model decode, the bind tree digest), not the receipt count. That load band also rose from 7.35 to
18.37 during the run. Not optimized here.

Results against the criteria:

- Well under 1 s at 13,000 receipts and 880 tickets: MET for heartbeat, renew and claim (436 to
  483 ms).
- Flat from 3,000 to 13,000 receipts: MET within run-to-run spread.
- Milliseconds (issue 641 wording): NOT_MET. The remaining cost is intent-tree work under the lock;
  it is the follow-up lane's target.
- At 9,990 tickets: 3,140 to 3,495 ms, NOT_MET; recorded, not bounded, by this slice.
- Duplicate-request and fork counterexamples still refused: `TestCALV0116_WriterRouteCounterexamples`.

## Tests

- `TestCALV0115_WriterCheckpointCodecIsClosed`: round trip, every truncation, every flipped byte, and
  each named field inconsistency is refused.
- `TestCALV0115_WriterCheckpointFallsBackToCompleteAudit`: removed, corrupt, torn, foreign, forged
  and outrun checkpoints each take the complete route with the same result.
- `TestCALV0115_ReceiptAuditIgnoresDerivedCheckpoints`: the help names both files; audit output is
  byte-identical with no checkpoints, with garbage checkpoints, and after removing them; neither file
  is read-modified or recreated.
- `TestCALV0116_WriterRouteParity`: served writes commit the same bytes and outcomes as the complete
  route.
- `TestCALV0116_WriterRouteCounterexamples`: prefix and tail duplicate requests, a stray own request,
  forks at the checkpoint receipt and in the tail, and a torn tail receipt.
- `TestCALV0116_WriterRouteTamperAtFastStages`: an intent edit after observe or after model, on both
  the mutate and lease routes, and a receipt planted after model. Each is refused or handed to the
  complete route.
- `TestCALV0116_PrefixTamperIsLeftToCompleteAudits`: pins the accepted guarantee change.
- `TestCALV0116_WriterFullBoundDeclines`, `TestCALV0117_WriterAdvanceAndScheduledRefresh`.
- `TestCALV0117_RefreshWriteInterleave`: a write between the refresh's audit and its retention does
  not let the refresh bind a checkpoint to an older head.
- `TestCALV0117_FileSetCostParity` and the three summarized-inventory tests.

Diagnostic run (not committed): with the threshold forced from 128 to 2, so that most store tests
take the fast route, seven tests failed before decision 2 and six after it. None showed an
acceptance beyond the guarantee change:

- `TestCALV0070_MutateRefusesChangesAfterMergedAudit`, `TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit`,
  `TestCALV0070_MutateRetriesAuditWithoutWatch`, `TestCALV0070_PinnedInventoryMutateParity` and
  `TestCALV0070_PinnedInventoryFailureFallsBackFresh` inject at complete-route stages the fast route
  never reaches. `TestCALV0116_WriterRouteTamperAtFastStages` is the fast route's equivalent.
- `TestCALV0012_WriterBehindNewerHeadSamplesAgain` runs its racing write from inside the clock
  callback. The fast route samples the clock under the lock, so the head cannot move between sample
  and commit; the case is not reachable on the fast route.
- `TestCALV0062_OpenWorkflowRefusesBeforeMutation` failed only before decision 2 and passes now.

## Integration note

The branch merges origin/main 0c27e35f (batch 3) at ce4efeb5. A diff against base 76f7f2ac
therefore includes batch 3, and `corvint affected --base 76f7f2ac...` selects a superset.

## Codex review round 1 (2026-10-07)

Codex reviewed 0c27e35f..e676fb42 and raised three P2 findings. Each was confirmed against the code and
fixed behind a test that failed first.

1. The fast lease route did not recheck the intent worktree branch before effects; the complete route
   does in `commitLease`. A switch to another branch with identical `.taskman` contents passed
   `bindObservation`. `leaseWriter` now compares `primaryBranch` with the observed branch after
   `bindObservation` and declines on a mismatch. The fast `Mutate` path already declined through
   `requireBranch`. Test: `TestCALV0116_FastWriteRechecksIntentBranch` (lease and mutate subtests,
   branch switched at the model stage).
2. A writer resume had no note-reference state for tickets last posted before the checkpoint, so the
   first tail post of such a ticket skipped the note check. A tail receipt that removed a note
   reference without its note event was served; the complete audit refuses it as `JOURNAL_FORKED`.
   The writer checkpoint now carries, per live ticket entry with a reference, the SHA-256 of the
   reference's canonical encoding, derived from the walk (or carried from the base checkpoint for
   tickets the tail did not post). The writer audit binds such a post to it and declines on a
   mismatch. A plain decline was rejected because it would decline the first tail post of almost
   every ticket. The profile stays `taskman-writer-checkpoint/0` because the format is unreleased; a
   checkpoint in the earlier layout fails decode and the writer falls back. Tests:
   `TestCALV0116_TailNoteReferenceChangeWithoutEvent` (after two checkpoint advances, one walked and
   one carried) and the note cases of `TestCALV0115_WriterCheckpointCodecIsClosed`.
3. An older refresh whose audit succeeded could reinstall a checkpoint after a newer refresh refused
   and removed it, because the freshness check only compared against a retained checkpoint. Refresh
   now reads an invalidation token (`<state directory>.writer-checkpoint.invalidated`) before its
   audit and again under the lock, and publishes nothing when it changed or could not be read. A
   refusing refresh replaces the token before removing the checkpoint. Only refresh needs it: the
   complete mutate route audits and retains under one lock, and the complete lease route is
   change-guarded before it retains. Test: `TestCALV0117_OlderRefreshCannotUndoInvalidation`.

CAL-V0-115..117 wording and the failure-mode table were updated; the requirements stay PROPOSED.

## Codex review round 2 (2026-10-07)

Codex reviewed e676fb42..0df9d4ca, confirmed the round 1 fixes, and raised one P2: invalidation was
not crash-safe. A refusing refresh replaced the token and then removed the checkpoint as a separate
step, and no writer read the token. A stop between the two left the checkpoint, and the next fast
write committed over the known corruption. The test
`TestCALV0117_InvalidationSurvivesStopBeforeRemoval` stops a refusing refresh at the new
`refresh.invalidated` stage and failed first: the next write was served on the fast route.

The fix binds consumption to the invalidation state rather than to the removal. The writer
checkpoint now carries the SHA-256 of the token that was current when the audit deriving it began,
before the trailer. Every consumer (`observeWriter` and the refresh's freshness check) uses it only
while the current token is readable and hashes to that digest; anything else declines to the complete
route. Every retention (complete mutate, complete lease, fast advance, refresh) binds to the token it
read before its audit and publishes nothing if that token was unreadable or has changed. Replacing
the token therefore unbinds at once, and the removal is cleanup. The profile stays
`taskman-writer-checkpoint/0` because the format is unreleased; a file in the earlier layout fails
decode and the writer falls back (a codec case covers it). An unreadable or empty token, as a crash
after its unsynced write could leave, disables the fast route until the operator removes it with the
checkpoint; this trades speed for never binding a checkpoint to a torn refusal.

## Follow-ups (not built here)

Already filed in the native queue:

- V1-0915: the fast route reads the intent tree twice and recomputes the bind tree digest (about
  33 ms at 880 tickets, about 500 ms at 9,990). This is the next cut toward the millisecond target.
- V1-0916: the 512-receipt refresh runs a complete audit in the triggering caller; make it
  incremental or move it off the caller.
- V1-0917 (suspected): the tail walk may miss a tail request file that duplicates a prefix request.
  The tail walk already refuses a posted path the checkpoint holds (`internal/tasks/journal/records.go`),
  but no test plants that case, so the ticket asks for a reproducing test first.
- V1-0918: evidence lstat and attempt reads grow with ticket count; refused and replayed writes keep
  the complete route's cost.
- V1-0919: optimistic observation outside the lock with a head recheck, to bring the hold back down.

Not filed here (the coordinator owns the queue for this lane):

- The ticket-proportional cost at 10,000 tickets (model decode and tree reads, beyond V1-0915).
- The O(n) receipts name listing.
- Note state: a tail note receipt whose pre-state precedes the checkpoint declines. (Ordinary tail ticket posts are now bound to the checkpoint's note references; see Codex review
  round 1.)
- The first write without a checkpoint pays the complete route.
- ENFILE and WatchChanges at 100,000 receipts.
- Review-fold state (`internal/tasks/cli/external_review.go`).

## Not run

- `make gate` (owner preference for scoped work).
- Codex review and dogfood/CEM (the coordinator runs them at batch integration).
- Live fleet and Flow-Proof qualification (synthetic stores only, by instruction).
- Three-OS vet.

## Rollback

Revert the change. Per store, deleting `<git common dir>/taskman.writer-checkpoint` forces the next
write through the complete audit. Delete the `taskman.writer-checkpoint.invalidated` token beside it
only together with the checkpoint and when no refresh is running. Deleting the token alone re-binds a
checkpoint published while it was absent, including one a refusal stopped before removing, and
deleting it during a refresh could let an older refresh reinstall a checkpoint a newer refresh
removed. Older runtimes ignore both checkpoint files. No journal, intent,
request, receipt or archive bytes change.
