# 2026-11-11: run evidence, the corvint watch append and the review repairs (V1-5448)

Change `run-evidence`, task 4 completed. Requirements RVD-V1-002, RVD-V1-004, RVD-V1-008,
RVD-V1-010, RVD-V1-011 and RVD-V1-012 in `docs/specs/run-evidence-v1.md`. Follows
`2026-11-11-run-evidence.md`, whose difference 2 this entry replaces.

## Decisions

1. `corvint watch` writes a `corvint-test-run/0` receipt for each save that runs tests. Watch
   already runs `go test -json`; the stream is teed into the Go adapter's parser
   (`sessiontest.ReadGoEvents`), and `sessiontest.WatchReceipt` builds the receipt from the same
   `Receipt` type, `NewReceipt` and `Finish` that `corvint test` uses: tool `corvint watch`, scope
   `NARROWED`, no manifest digest (a watch plan is not a `corvint test` manifest), source the HEAD
   and dirty-path digests the save was planned against. It is appended through the one writer
   `corvint test` calls (`appendRunEvidence`), so the stored record has the same shape and the
   strict receipt reader accepts it. A package the stream never ended is NOT_RUN and the receipt
   INCOMPLETE. `--dry-run` writes nothing. The earlier objection, that a watch receipt would bind
   a manifest digest no manifest has, is met by leaving the digests empty rather than inventing one.
2. The RVD-V1-008 "removed verb" scenario is gated on change `source-removal`, which the R38 order
   delivers later; `corvint runs` is not removed here. Its test skips with the gate as its reason.

## Independent review findings repaired

- A torn log tail (a writer that died mid-line) made the next append join its line to the partial
  one. The writer now rewrites the log whole when it does not end in a newline.
- The append lock waited without bound. It is taken with `LOCK_NB` and retried for 5 s, then the
  append fails as `run-evidence-write-failed` and the run keeps its exit code.
- `Read` was unbounded on a damaged store. It reads at most 4096 entries and the cap plus one
  record of bytes, newest first, and lists a bounded number of directory entries when the log is
  unreadable; a store past a bound reports `run-evidence-corrupt`.
- The log and lock files open with `O_NOFOLLOW`, and an appended line is synced.
- `TestRVDV1011StoredResultsDoNotChangePlan` held by construction, since nothing read the store. It
  is replaced by import-ratchet source scans: `TestRVDV1011NoSelectionPackageReadsTheStore` (no
  selection package imports the store or the test-evidence reader or calls an appflows store
  reader, with a spy that proves the scan sees one) and, under the old name, which
  `traceability-tests-check --range` still resolves for round 1's commit, the confinement of store
  references inside appflows and cmd/corvint.
- A read-purity digest test now covers `corvint test-evidence`'s `storedRuns` on an intact and a
  damaged store.
- The spec states that the store is per worktree, so `git worktree remove` deletes it.

## Evidence

Focused package tests and the commands are listed in the lane handback.
