# 2026-10-20 V1-1786: batch-check reports each dropped conflict hunk

Ticket: V1-1786. `dogfood batch-check` accepted a merge derived with `-Xours` or `-Xtheirs` and
reported only an aggregate count of merges that dropped a conflict side. The
`merge-resolution-unbound` recovery told agents to merge with `-X ours`, which loses the other
side's conflicting hunks without a trace.

## Decision

`DCW-V0-073` is added to `docs/specs/daily-change-evidence-workflow-v0.md` and accepted under the
owner's standing auto-accept of lane requirements (2026-10-11).

- Detection: when a merge matches the `-Xours` or `-Xtheirs` derivation, the plain ort merge of
  its parents is recomputed in the same private scratch repository with
  `merge.conflictStyle=merge` fixed. Each conflict region is parsed, the kept side is rebuilt, and
  the rebuilt file must equal the merge commit's blob byte for byte, or the path is
  `hunk=UNKNOWN reason=conflict-not-parsed`. A whole-file conflict with no markers, such as a binary
  file, is one `whole` hunk identified by its stage blobs. A merge whose tree is its first parent's
  tree while Git would merge the other side (`-s ours`) is `hunk=UNKNOWN
  reason=second-side-discarded`, listing at most 20 differing paths.
- Identity: the hunk id is the SHA-256 of the path, the kept lines and the dropped lines,
  NUL-separated. It does not depend on line numbers or commit ids, so it can be written into the
  merge message before the merge is committed.
- Coverage: a hunk passes as `restored:<bind>` when a verified constituent whose base descends from
  the merge changes the path and `HEAD` holds the dropped lines contiguously. It passes as
  `disposition` when the merge's own message has `Corvint-Dropped-Hunk: <id> <reason>` with a
  non-empty reason. Every other hunk and every `UNKNOWN` fails the new code `merge-side-dropped`.
- Recovery: the `merge-resolution-unbound` `fix:` line and `docs/DOGFOOD.md` step 2 now say to
  rebase the conflicting change's own work commits onto the merge's first parent, re-resolve,
  rebind and reseal, then merge with plain `git merge --no-ff`. They name `-X ours` or `-X theirs`
  only as the thing that fails `merge-side-dropped`.
- Scratch output from `merge-tree` and `cat-file` is bounded; overflow refuses, so it is never
  parsed as partial evidence.

## Evidence

- `TestBatchCheckDCWV0073ReportsEachDroppedHunk`: two lanes edit lines 2 and 10 of an 11-line file
  and are merged with `-X ours`. Both hunks are listed per file and hunk and fail. A reasoned
  disposition covers hunk 1, and a reasonless line does not. A later sealed change restoring hunk 2
  then passes with `NOTE conflict-side-dropped merges=1 hunks=2 restored=1 dispositioned=1`, and the
  object store is unchanged.
- `TestBatchCheckDCWV0073FirstParentTreeDropsSide`: an `-s ours` merge fails as `UNKNOWN`.
- Fail without the fix: with `batch.go` restored to the base and `batch_dropped.go` removed, both
  tests fail. The `-X ours` batch exits 0, and the `-s ours` batch has no `merge-side-dropped`.

## Limits

- Hunk boundaries follow Git's merge window, so close conflict regions are one hunk. That is why
  the test spaces its conflicts eight lines apart.
- A file whose content holds an exact marker line may parse wrongly. In that case the rebuild
  check makes it `UNKNOWN` rather than passing.
- Restoration is a line-exact content check, not proof of intent. A reformatted restoration needs
  a disposition. Dispositions are read only from the merge commit's own message.
- The semantic correctness of a restoration or disposition, and the composition of the merged
  changes, remain NOT_OBSERVED.
