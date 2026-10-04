# Unbounded-reader ratchet records a set, not a count (V1-0752)

## Finding

The AFP-V0-025 ratchet compared the number of unbounded test packages with one recorded integer.
An integer does not merge the way the tree does. Two changes that each add an unbounded package
and raise the ceiling from 42 to 43 make the same one-line edit, Git merges it cleanly to 43, and
`main` then holds 44 packages and fails a check both changes passed. The introducing merge
(`6bf78ff7`, 56 against 45) and `internal/delta` the same day (`dd485091`) were the related case
of packages whose own last check predated the ratchet; see
`2026-10-04-unbounded-reader-merge-skew.md`. The main ruleset does not require an up-to-date
branch, so nothing reran the check against the newer base.

## Change

- `.corvint/unbounded-readers.json` is now `corvint-unbounded-reader-set/0`: `units` lists the 42
  directories the previous count admitted, one per line, strictly ascending; `reasons` is
  unchanged and must name directories in `units`. No package was added or removed.
- `tools/unbounded-readers` fails when an unbounded test package is not in `units` or a `units`
  entry is no longer unbounded, and names the directories in both cases. A stale entry was a
  notice under the count ("lower the ceiling"); it is now a failure, so a gain is kept in the
  change that makes it. A swap of one package for another, which the count could not see, fails.
- AFP-V0-025 states the set and its residual skews.

Whether a directory is unbounded depends on that package's test files and its declaration, and
its entry travels in the same change, so the merged record is correct whenever the two sides
touch different packages. Entries that land on abutting lines conflict textually, which blocks
the merge before it completes.

## Evidence and limits

- `TestAFPV0025ConcurrentAdditionsMergeToAPassingRecord` builds a repository, cuts two branches
  that each add an unbounded test package and its entry, checks each alone, merges them with
  `git merge` and checks the result: all pass. It then adds a package with no entry and requires
  the failure to name it.
- `TestAFPV0025RatchetFailsOffTheRecordedSet` covers an unrecorded unit, a stale unit, a swap, the
  old `ceiling` record and the closed grammar.
- `make unbounded-readers-check` passes on this change with the same 42 units as `cd70ba0a`.
- Not closed: a pull request whose last check ran before the ratchet reached its base; a change
  to the graph's classification that merges beside a new package; two changes to one package
  that disagree about its reads. `main`'s push run fails and names the directories. Requiring
  up-to-date branches or a merge queue would close these; both are owner-held repository
  settings and were not changed. Hosted behavior of this change is `NOT_OBSERVED` until its
  own CI run.
- `make gate` was `NOT_RUN` (owner's scoped-work preference).

## Rollback

Revert this change: the record returns to `{"profile":"corvint-unbounded-reader-ceiling/0",
"ceiling":42,...}` and the tool to the count comparison.
