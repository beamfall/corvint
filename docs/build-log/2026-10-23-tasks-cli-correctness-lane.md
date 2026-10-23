# corvint-tasks command correctness lane (V1-0903, V1-2368, V1-2400, V1-0907, V1-0775)

Date: 2026-10-23. Base `7b3a8e55fc909b9692c684e47dc4c6787ac2419a` (origin/main). Requirement IDs:
CAL-V0-365..366 (V1-2368, in `corvint-tasks-attempt-runner-v0.md`) and CTS-V0-020..023 (V1-2400, in
`corvint-tasks-store-init-v0.md`), from the lane's reserved ranges. None was taken on main or in any
open pull request when numbered.

## V1-2368: interrupted worktree removal and removed-record compaction

Reproduced from code reading, then by test: the takeover rule in `transaction/program.go` admitted a
new owner only in `FINISHED`, `ADMITTED`, `WORKTREE_ADD` or `READY`, and `cleanupPhase` refused
`WORKTREE_REMOVE` as `ATTEMPT_LIVE`, so a cleanup stopped after its `WORKTREE_REMOVE` write left
the program stuck. Every removal also kept its record marked `removed`.

Decision: `WORKTREE_REMOVE` is a safe takeover phase (it is entered only once no live attempt can
reference the worktree). The new owner settles the pending removal before anything else: an absent
checkout is recorded removed, a checkout that still binds is removed, anything else keeps its record
and returns the phase; the settled item carries `takeover` `FINISHED` or `ROLLED_BACK`. Removed
records are dropped (CAL-V0-366), which also lets a path be registered again within the assignment.

Evidence: the takeover tests stop `program cleanup` in a child test process (`os.Exit` at the
removal hook), so the recorded owner really is gone. Against the old `workflow.go` and
`program.go` (with only the test hook added), all three takeover subtests fail with `ATTEMPT_LIVE`
and the compaction test with every record kept `Removed:true`.

## V1-2400: import map amendment

Decision: a new `IMPORT_MAP_UPDATE` operation (stage operation and receipt kind) on the
administrative writer, modelled on `POLICY_UPDATE`, with an expected-digest guard, an `OWNER`
binding, and an identity check (CTS-V0-021) that permits a source item to move under its ticket (a
rename or an added source queue) but refuses a dropped ticket, a moved source item, a ticket mapped
twice, or a record's own source mapped elsewhere. The stage maximum (7 artifacts, 1844 bytes) is
measured by the existing codec maxima test. The new receipt kind is a format extension: an older
binary refuses a store that recorded one, which the rollback note states.

## Independent review

One read-only review of the first commit found: a missing checkout settled as removed even when it
had been moved (fixed: git's private worktree directory must be gone; a moved case is tested); a
mapped rename that left the renamed ticket impossible to re-import, because the planner and
`importChain` still required the record's old source (fixed as CTS-V0-023, tested); a new map entry
naming a native ticket admitted (fixed in CTS-V0-021, tested); `newImportMapSha256` reported on a
refused CLI call (fixed); and untested paths (idle `OpenWorkflow` settlement, earlier
removed-marked records, map immutability after later writes), now tested. Not fixed: a
`CleanupWorktrees` retry on the same in-memory `Workflow` after a failed phase write misreports the
already-dropped record as `ROLLED_BACK` (persisted state is correct; the next process reads it).

## V1-0903: not reproduced

Fifty `archive export` runs on 10,000-ticket stores, none failing: 10 in-process and 20 binary runs
on a fixture store (1,251 receipts), 6 concurrent with `ticket list` reads, and 15 in-process plus
binary runs on a `historyStoreAt`-shaped store (10,000 tickets, 13,000 receipts), repeated under
three busy loops per CPU. The original lane's logs (`/private/tmp/claude-501/td-scale-tasks/`) are
on another host; its build log records a Darwin host at load 11 to 28 that also hit `ENFILE`. No
cause is recorded and no fix is made; the ticket stays open.

## V1-0907: already once per call path

Every CLI call path folds the review history at most once: `completionOffers.evidence`
(`internal/tasks/cli/external_review.go:84-87`, shared by `ticket show` and `plan preview`),
`gateState` (`external_review.go:668`), and the dispatcher tick, which carries the fold between
ticks (`internal/tasks/cli/dispatch.go:966-972`, CAL-V0-138). The ticket's acceptance (per-call
cost flat in history) needs the fold state persisted in the read checkpoint, which is a format
decision not made here.

## V1-0775: confirmed, not fixed

`TestATRV0241_MappedWriteAfterLeaseAuditRefusedByNextWrite` (lane tasksup-1021m) already
reproduces it, and passes here on Linux 6.18 ext4 (tmpfs and APFS NOT_RUN in this lane). Closing the window needs a content re-read of every audited
file under the writer lock on each lease write, which conflicts with accepted CAL-V0-263 (bounded
lock work); ATR-V0-241 records that trade as the owner's. Left open for that decision.
