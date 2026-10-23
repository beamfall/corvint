# Tasks correctness lane: V1-0903, V1-2368, V1-2400, V1-0907 and V1-0775

Date: 2026-10-23. Base: `origin/main` 7b3a8e55. Lane scope: correctness bugs in corvint-tasks commands;
the journal resume and prefix-walk code belongs to another lane and is untouched.

## V1-0903 archive export intermittently exits rc=1 at 10,000 tickets (CTS-V0-020, proposed)

- **Not reproduced as a race.** On a synthetic 10,000-ticket-file store, eight sequential and eighteen
  parallel `archive export` runs under the default limit all passed, as did 25 runs beside a writer
  creating tickets without a pause on a smaller store.
- **Cause found by lowering the descriptor limit.** `ulimit -n` 64 through 200 refused
  `RESOURCE_EXHAUSTED` (`openat tmp: too many open files`); 256 passed at a 213-descriptor peak. The store had
  192 `requests/` shard directories and `archiveRead` kept a no-follow root open for every listed
  directory until the attempt ended. A store with all 256 shards populated needs more than the 256
  descriptors macOS gives a process by default, so whether an export fits depends on the populated
  shard count and the descriptors the caller already holds. That matches an intermittent rc=1 at
  10,000 tickets on macOS; the report's logs were not available, so that match is inferred, not observed.
- **Fix.** At most 32 reopenable directory roots stay open (least recently used closed first, never the
  one in use or its ancestors); the state dir and intent roots are path-bound and stay. A reopened root must
  be the directory the listing observed. Peak 44 descriptors on the same store, `ulimit -n 64` passes, the
  stream is byte-identical to the unbounded export.
- Tests `TestCTSV0020_*` (`internal/tasks/archive`) fail on the unbounded code: 211 descriptors held, no shard
  root closed.
- Limit: macOS was not run; the 10,000-ticket store is a generated journal padded with copied ticket
  files, because the product writers took 38 s for 200 tickets and about 15 minutes for the first 430 of a 10,000-ticket
  run, which I stopped.

## V1-2368 interrupted `WORKTREE_REMOVE` takeover and removed-record pile-up (CAL-V0-365..366, proposed)

- Reproduced on the old code by a child process that exits after the phase write: the replacement is
  refused `FENCED` `prior program owner not proved stopped`.
- A replacement owner now settles the in-flight removal from the recorded effect digest: `COMPLETED`
  (worktree gone and unregistered, record dropped) or `ROLLED_BACK` (still there, record kept, the cleanup
  then removes or reports it), with a typed `resolution` on the cleanup item. The transaction admits the
  takeover only with no attempt or a terminal `PROVED` one.
- A removed worktree leaves no record, and the next cleanup drops records earlier releases kept marked
  `removed`; `TestATRV0297` and `TestCALV0087` changed from "marked removed" to "no record".
- Tests fail with the transaction rule reverted (takeover refused) and with the compaction disabled
  (removed records held).

## V1-2400 import map cannot change after genesis (CTS-V0-021..023, proposed)

Earlier lanes skipped it as a design decision; this lane made these choices, each stated in the spec for
the owner to accept or change: a new `IMPORT_MAP_AMEND` stage operation and receipt kind (the receipt kind set
is closed, so a store that recorded one cannot roll back to an earlier binary); OWNER only; compare-and-set on the
current map digest; identities keep their source item, revision and applied sequence while `sourceQueueId` may
change (a source rename); refused `DUPLICATE_ID` naming the conflicts; refused under a barrier, for a
`NATIVE`-written queue, for an unmapped queue, and when nothing changes. Tests: amend with rename and an added
source queue, four identity conflicts, seven fail-closed refusals, replay, redo after an interrupted
publish, and the CLI. Disabling the conflict check, the digest check and the owner check in turn fails
the matching tests.

## V1-0907 review-fold state rebuilt on every call: already delivered, no change

- The dispatcher carries the fold in memory and folds only appended receipts
  (`internal/tasks/store/review_fold.go` `ReviewFold.Fold`, used at `internal/tasks/cli/dispatch.go`
  `reviews: &store.ReviewFold{}`; CAL-V0-138 of the agent leases spec; `BenchmarkCALV0138_DispatcherTickFold`,
  `TestCALV0138_*`).
- Each one-shot CLI path folds at most once per call: `completionOffers.evidence`
  (`internal/tasks/cli/external_review.go`, the `folded` flag; used by `ticket show` and `plan preview`),
  `gate state`, and `receipt audit` reusing the audit's own fold (`receiptMaterialBindings`).
- Residual, not changed: a one-shot read with a review reference still folds from receipt 1 in its own process;
  only receipt retirement bounds that.

## V1-0775 lease writes may reuse audit digests that miss mapped writes: real, left to the owner

- `TestATRV0241_MappedWriteAfterLeaseAuditRefusedByNextWrite` (existing) passes on Linux ext4 and on Linux tmpfs
  (2026-10-23): the claim commits under a mapped write after the audit, and the next write refuses.
- Closing the window needs a content re-read of every audited file under the writer lock, which CAL-V0-263
  forbids and ATR-V0-241 leaves to the owner; the code is in the preparation and resume path another lane owns.
  No code changed. A stat comparison under the lock is not a substitute: a mapped write to an already dirty page
  does not update the file's times.
