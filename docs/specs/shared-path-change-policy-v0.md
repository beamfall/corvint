# Shared-path change policy V0

Owner: Russell Lewis
Date: 2026-10-17
Requirement prefix: `SPC-V0`
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-1591); SPC-V0-013/014 accepted under the owner's standing auto-accept (2026-10-11; V1-1723); SPC-V0-015 accepted under the owner's standing auto-accept (2026-10-11; V1-1726)
Delivery status: experimental
Authoritative inputs: V1-1591 / GitHub #1034 owner request; `../../AGENTS.md` invariants 2, 4, 7 and 8; `../DOGFOOD.md`.

## Agent digest
- Claim: Names unlabelled commits that change shared test infrastructure, splits them out, and reports the shared-path base commits a branch lacks.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1591); SPC-V0-013/014 accepted under the owner's standing auto-accept (2026-10-11; V1-1723); SPC-V0-015 accepted under the owner's standing auto-accept (2026-10-11; V1-1726)/experimental
- Exists: `internal/sharedpath` and the experimental `corvint changes check-shared`, `corvint changes split-shared` and `corvint changes behind` subcommands; `behind --format doctor-plugin` feeds the `corvint-tasks doctor --plugins` surface.
- Blocked on: no repository ships a `.corvint/shared-paths.json` yet; the notes listing and the run citation stay manual follow-ups; live-session detection sees only configured markers and Git's own in-progress state; a built-in doctor or dispatch detector that discovers live ticket worktrees itself, and a claim-result notice, are not delivered (V1-1710 residual).
- Read next: Requirements; Boundaries and failure modes; Traceability.

## User and baseline

Several agent lanes edit shared test infrastructure (helpers, fixtures, scripts) next to their own
feature changes. A reviewer then cannot revert or audit the shared change on its own, and an
unrelated lane inherits it without knowing which affected run proved it. The baseline is reading
every commit by hand and rewriting history interactively. The job is a deterministic local check
that names what is missing, plus a mechanical split that a user runs on a local branch they name.

## Requirements

- `SPC-V0-001`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1591) The policy
  MUST be a closed, bounded JSON file (default `.corvint/shared-paths.json` under the root, at most
  64 KiB, profile `corvint-shared-paths/0`) with `shared` globs (required), `generated` globs,
  `marker` (the required commit subject prefix), `runTrailer` (default `Affected-Run`) and
  `sessionMarkers` (checkout-relative paths). Globs are relative slash paths whose segments use
  `path.Match`, with `**` matching zero or more whole segments; at most 256 globs per list. A path
  matching a `generated` glob is generated (never shared); otherwise a `shared` match is shared.
  An absent, oversized, unknown-field, wrong-profile or invalid file refuses with
  `changes-shared-config-unavailable`. The config and `--notes` files are opened without blocking
  and read only when they are regular files, so a FIFO or device refuses instead of hanging.
- `SPC-V0-002`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1591)
  `corvint changes check-shared --base REV [--head REV] [--config FILE] [--notes FILE]` MUST read
  the non-merge commits of `base..head` (at most 512) and, for each shared path a commit changes,
  emit one finding per missing element, naming `code`, `commit`, `path` and `missing`:
  `shared-path-unmarked` (`marker`: the subject lacks the marker), `shared-path-unlisted`
  (`listing`: the `--notes` file names no hexadecimal prefix of at least seven characters of the
  commit) and `shared-path-uncited` (`run-citation`: no `runTrailer` line, or one whose value is
  not `PASS <reference>`). A marked commit that also changes a non-shared, non-generated path
  emits `shared-path-mixed` (`separate-commit`) per such path. Without `--notes` the listing is
  `NOT_OBSERVED`, never a finding. Merge commits are skipped and named. Exit 0 without findings,
  1 with findings, 2 on refusal. The check writes no ref, index, worktree or trace file.
  Refusals: `changes-invalid-arguments`, `changes-shared-range-unavailable`,
  `changes-shared-notes-unavailable` (absent, non-regular or over 1 MiB), `changes-git-unavailable`.
- `SPC-V0-003`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1591) The check
  MUST probe merging head into base with `git merge-tree --write-tree` inside a private temporary
  bare repository (`GIT_DIR`, with the repository's object format, its object store as an alternate
  and its shallow file), removed afterwards, so the probe writes no object into the repository and
  reads no repository configuration, attributes or worktree: no custom merge or filter driver runs
  and nothing is written into the checkout (amended for Codex finding 1 on #1097, V1-1710). The
  probe therefore merges with Git's built-in drivers and default rename settings, which can differ
  from a merge under the repository's own attributes. Git's conflict exit status is kept: the
  conflicted paths are the unmerged file entries plus the paths named by `CONFLICT` messages, and a
  conflict that names no path still emits one pathless finding. A conflicted path matching a
  `generated` glob is listed in `exemptConflicts` and is not a finding; any other conflicted path
  emits `shared-path-merge-conflict` (`clean-merge`).
- `SPC-V0-004`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1591)
  `corvint changes split-shared --base REV --branch NAME [--apply --expect-head SHA]` MUST plan,
  and with `--apply` perform, the split of every commit in `base..NAME` that changes both shared
  and other paths into a shared-only commit (subject: marker plus original subject) followed by
  the remaining change with the original message. Both keep the original author, committer, dates
  and encoding; commits before the first split are reused unchanged. The rewritten head tree MUST
  be byte-identical to the old head tree, or the run refuses with `changes-shared-tree-mismatch`
  and moves nothing. The output (`corvint-shared-path-split/0`) prints `oldHead`, `newHead`, each
  split, and a restore command. It rewrites only `refs/heads/NAME`, through a private temporary
  index, and never pushes, fetches or contacts a remote (`pushed` is always false). Without
  `--apply` it writes nothing: every Git command runs with `GIT_OPTIONAL_LOCKS=0`, so planning's
  `git status` never refreshes the repository index.
- `SPC-V0-005`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1591) Before any
  write the splitter MUST refuse, leaving every ref unchanged: a configured session marker present
  in the checkout, or a Git index lock or a merge, cherry-pick, revert, rebase or bisect in progress
  (`changes-shared-live-session`); a detached HEAD (`changes-shared-detached-head`); NAME not the
  checked-out branch (`changes-shared-branch-mismatch`); any tracked change or untracked file
  (`changes-shared-dirty-tree`); a merge commit in range (`changes-shared-merge-in-range`); an
  invalid branch name (`changes-invalid-arguments`).
- `SPC-V0-006`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1591) An applied
  or planned split MUST list exactly `notes-listing` and `run-citation` as `manualFollowUps`
  (empty when nothing needs splitting): the splitter adds the marker and separation but never
  invents a listing or a run result.
- `SPC-V0-007`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1591) `--apply`
  MUST require `--expect-head`. When the branch head is not that commit the run refuses with
  `changes-shared-head-mismatch` before writing any object or ref; the final ref move is a
  compare-and-swap (`git update-ref NAME NEW OLD`) that refuses with the same code when the branch
  moved during the rewrite.
- `SPC-V0-008`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1591) Both
  subcommands are dispatched by `corvint [--root SOURCE] changes`, refuse with one JSON object
  `{"code":...,"ok":false}` on standard error and exit 2, and print one JSON report on standard
  output. Relative `--config` and `--notes` paths resolve under the root.

- `SPC-V0-009`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1710)
  `corvint changes behind --base REV [--head REV] [--config FILE] [--ticket ID --stale-hours N]
  [--format json|doctor-plugin]` MUST name the base commits not reachable from head (`head..base`,
  head defaulting to `HEAD`) that change a configured `shared` path. It first counts that range
  with `git rev-list --count` and, when it holds more than 512 commits, refuses with
  `changes-shared-range-too-large`, whose refusal object also names `limit` (512) and `commits`
  (the count), before listing or reading any commit. A non-merge commit counts only when it changes
  a `shared` path: commits that change only other or `generated` paths are ignored, and merge
  commits are skipped and named in `skippedMerges`. Each counted commit is listed oldest first with
  `commit`, `subject`, `committedAt` (its committer time, UTC), `arrivedVia` and `arrivedAt`
  (SPC-V0-013), `sharedPaths`, `headTouches` (the
  shared paths of that commit that head also changes since the merge base) and `conflictRisk`
  (`headTouches` is not empty). Base and head without a merge base refuse with
  `changes-shared-range-unavailable`. The command is a read command: it never fetches or contacts
  a remote, writes no ref, index, worktree, repository object or trace file, and runs no
  repository-configured merge or filter driver (SPC-V0-003). Exit 0 on success
  and 2 on refusal, under the SPC-V0-008 dispatch and refusal shape.
- `SPC-V0-010`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1710) The
  report's `dryMerge` MUST probe merging head into base under the SPC-V0-003 private-object-directory
  rule and give `result` as exactly one of `clean` (Git reports no conflict), `generated-only`
  (every conflicted path matches a `generated` glob) or `conflict`, with `conflicts` listing the
  other conflicted paths and `generatedConflicts` the generated ones, each sorted. Conflicted paths
  are those of SPC-V0-003, so a conflict without an unmerged file entry (a directory rename split
  names its directory) is `conflict`, and so is a conflict Git reports without naming any path
  (amended for Codex finding 2 on #1097, V1-1710).
- `SPC-V0-011`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1710) The JSON
  report (profile `corvint-shared-path-behind/0`, `mutates` false) MUST record the resolved `base`,
  `baseRef` (SPC-V0-014), `head` and `mergeBase`, `configDigest` (`sha256:` and the hexadecimal SHA-256 of the configuration
  file bytes), `observedAt`, `counted` (the commits in `head..base`), `behind`, `skippedMerges`,
  `dryMerge` and `findings`.
- `SPC-V0-012`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1710) With
  `--ticket ID` and `--stale-hours N` (both or neither; ID matches `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`,
  N is 1 to 8760) the report MUST emit one `SHARED_BASE_BEHIND` finding when the oldest counted
  commit (earliest `arrivedAt`, SPC-V0-013; amended for V1-1723 from the earliest committer time)
  reached base at least N hours before `observedAt`, naming `who` (the ticket), `commit` (that
  oldest commit), `ageSeconds` (measured from its `arrivedAt`), and a `detail` that names the
  ticket, the commit, its age in hours and the SPC-V0-014 base observation. `--format doctor-plugin` requires `--ticket` and prints only the
  findings as the TQD-V0-010 plugin array of `{kind, who, detail, remedy}` objects (`[]` when there
  is none) and exits 0, so an operator's `corvint-tasks doctor --plugins DIR` executable can carry
  the finding for a live ticket worktree. Invalid flags refuse with `changes-invalid-arguments`.
- `SPC-V0-013`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1723) Each
  counted commit MUST carry `arrivedVia`, the first-parent commit of base that introduced it, and
  `arrivedAt`, that commit's committer time (UTC). `arrivedVia` is the oldest commit on base's
  first-parent chain inside `head..base` from which the counted commit is reachable: a commit made
  directly on base arrives as itself, and a commit merged from a side branch arrives with the merge
  that brought it onto base. It is computed in memory from the one `rev-list --parents` listing
  of the range, so it adds no Git call per commit beyond one commit read per distinct arrival
  commit. The SPC-V0-012 age uses `arrivedAt`, so a commit authored long ago but merged recently
  is aged from its merge. `committedAt` stays the commit's own committer time. Falsifier: a commit
  whose committer time is older than the merge that brought it onto base, aged or reported by its
  committer time. Rollback: drop `arrivalCommits`, `arrivalTime` and the two members, and age by
  committer time again.
- `SPC-V0-014`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1723) The
  report MUST carry `baseRef`: `name` (the `--base` argument as given), `ref` (the full ref it
  names according to `git rev-parse --symbolic-full-name`, or `""` when it is not a ref, such as a
  commit ID or a revision expression), `lastObservedAt` (UTC, or `null`) and `observedVia`
  (`reflog`, `FETCH_HEAD` or `none`). `lastObservedAt` is the later of the ref's newest reflog
  entry and, only for a ref under `refs/remotes/`, the modification time of this worktree's
  `FETCH_HEAD`; `observedVia` names which one, and with neither the member is `null` and
  `observedVia` is `none`. The `SHARED_BASE_BEHIND` detail repeats that observation, or says it
  is unknown. These are reads only: no fetch, no reflog or `FETCH_HEAD` write. Falsifier: a
  remote-tracking base whose last fetch is older than its reflog or `FETCH_HEAD` says, reported
  without that time, or any write by the observation. Rollback: drop `observeBase`,
  `baseObservationDetail` and the member.
- `SPC-V0-015`: (accepted under the owner's standing auto-accept (2026-10-11); V1-1726) In a
  shallow repository (`git rev-parse --is-shallow-repository` is `true`), `check-shared` and
  `behind` MUST, after resolving base and head and before listing any commit or running the dry
  merge, refuse with `changes-shared-shallow-merge-base` when the merge base may lie beyond the
  shallow boundary: `git merge-base --all` finds no merge base or more than one (Git merges
  several merge bases recursively through their own merge bases, whose ancestry this check does
  not prove; independent review of PR #1623), or the one merge base is a commit the
  repository's `shallow` file lists (a boundary commit, whose parents the clone lacks) other than
  base or head themselves, or a commit of the symmetric difference `base...head` is a boundary
  commit, so a side branch of the range is cut off even though the merge base is exact. A merge
  base that is base or head is exact even at the boundary, because shallow ancestry is a subset
  of the full ancestry. Otherwise the reports are those of a full clone; the dry merge's private repository reads the same `shallow` file through
  `GIT_SHALLOW_FILE` (SPC-V0-003), without which `merge-tree` cannot walk a boundary commit and
  refuses `changes-git-unavailable`. A full repository is unaffected. The check fetches nothing
  and writes nothing. Failure modes: before this a merge base beyond the boundary refused as
  `changes-shared-range-unavailable` (`behind`) or `changes-git-unavailable` (`check-shared`),
  or, at a boundary merge base, returned a report computed from a merge base the clone could not
  prove, or (independent review of V1-1726) a range listing that silently omitted the commits
  behind a boundary commit inside the range, or (independent review of PR #1623) a criss-cross
  merge whose two visible merge bases keep their parents but whose own merge base lies past the
  boundary, so the shallow virtual merge base differed from the full clone's and the dry merge
  reported clean where the full clone conflicts. Limit: refusing every shallow range with several merge
  bases, including ones a full clone would merge identically, is deliberate: it is the simplest
  fail-closed rule. A second merge base reachable only past the
  boundary, while every visible merge base and every commit of `base...head` keeps its parents,
  is beyond what a shallow repository can show and is not detected; the symmetric difference is
  listed once, bounded by the 32 MiB Git output bound.
  Falsifier: a shallow clone whose merge base is beyond its boundary, or that has several merge
  bases, yielding a report, or one
  whose merge base keeps its parents yielding a report different from a full clone's. Rollback:
  drop `refuseShallowMergeBase`, its two calls and the code.

## Boundaries and failure modes

Non-goals: pushing, force-pushing or opening pull requests; rewriting any ref but the named
branch; splitting merges or renames by content; writing notes or run citations; detecting an agent
session that leaves no configured marker; enforcing the policy in CI; for `behind`, fetching the
base, updating or merging the branch, discovering live ticket worktrees inside `corvint-tasks`
(the doctor sees the finding only through an operator plugin that names the ticket and its head)
and a claim-result notice. `split-shared --apply` is an
explicit, operator-invoked writer of one local branch ref and unreachable objects; it is outside
the read-command invariant by design, like `index`. `check-shared` and `behind` are read commands.

Failure modes: a live agent that leaves no marker and holds no Git lock is not detected, so the
user names the branch deliberately; a path renamed into a shared directory is one deletion and one
addition (renames are not detected); a ref whose head moves between the expected-head check and the
compare-and-swap aborts with the head-mismatch code and leaves only unreachable objects, which
`git gc` collects. Hooks are disabled (`core.hooksPath=/dev/null`) and signing is off for split
commits. Git stderr is never reflected. `behind` reads only local refs, so a stale
remote-tracking base understates what is missing until the operator fetches; `baseRef` makes that
staleness visible (SPC-V0-014) but does not correct it. `FETCH_HEAD` is rewritten by any fetch in
the worktree, including one of another remote or refspec, so its time can overstate how recently
that ref was observed; a fetch that leaves the ref unchanged writes no reflog entry, so a reflog
time alone can understate it. The age is the committer time of the first-parent commit that
brought the change onto base (SPC-V0-013); a merge or rebase whose committer clock was wrong, or a
base history rewritten later, carries that error into the age. A shared change made inside
a merge commit's own resolution is not counted (merges are skipped and named). The merge probe ignores the
repository's attributes and configuration, so a path that a custom or `union` merge driver would
resolve can be reported as conflicted (a conservative report, never a hidden conflict).

## Rollback

An applied split prints `restore`, `git update-ref refs/heads/NAME OLD NEW`, which puts the old
head back while the old commits are still reachable from the reflog. Removing the feature deletes
`internal/sharedpath`, the two subcommand branches in `cmd/corvint/changes.go` and
`cmd/corvint/changes_behind.go`; no stored state depends on them. Removing only `behind` deletes
`internal/sharedpath/behind.go`, `cmd/corvint/changes_behind.go` and its dispatch line.

## Traceability

| Requirement | Implementation | Focused evidence |
|---|---|---|
| SPC-V0-001 | `LoadConfig`, `Match`, `Config.Class`, `readRegular` | `TestSPCV0001ConfigIsClosedAndClassifiesPaths`, `TestSPCV0001FIFOInputsRefusedWithoutBlocking` |
| SPC-V0-002 | `Check` | `TestSPCV0002CheckFindingsNameCommitPathAndMissingElement` |
| SPC-V0-003 | `mergeProbe`, `mergeConflicts`, `privateMergeRepository`, `conflictPaths` | `TestSPCV0003GeneratedConflictsAreExempt`, `TestSPCV0003MergeProbeWritesNoRepositoryObject`, `TestSPCV0009DryMergeRunsNoRepositoryMergeDriver`, `TestSPCV0010DirectoryRenameSplitIsConflict` |
| SPC-V0-004 | `SplitShared`, `overlay`, `commitTree` | `TestSPCV0004SplitKeepsTreeIdentityAndMessage`, `TestSPCV0004PlanningLeavesIndexBytesUnchanged` |
| SPC-V0-005 | `refuseLiveSession`, `SplitShared` | `TestSPCV0005SplitRefusesUnsafeStates` |
| SPC-V0-006 | `manualFollowUps` | `TestSPCV0006ManualFollowUpsAreOnlyListingAndCitation` |
| SPC-V0-007 | expected-head check and `update-ref` compare-and-swap | `TestSPCV0007HeadMismatchAbortsWithoutChanges` |
| SPC-V0-008 | `runSharedPath` | `TestSPCV0008ChangesSharedCLI` |
| SPC-V0-009 | `Behind`, `countBehind`, `runChangesBehind` | `TestSPCV0009BehindCountsOnlySharedCommits`, `TestSPCV0009BehindRefusesTooLargeRangeBeforeListing`, `TestSPCV0009BehindIsReadOnly`, `TestSPCV0009DryMergeRunsNoRepositoryMergeDriver` |
| SPC-V0-010 | `mergeConflicts`, `conflictPaths`, `dryMergeOf` | `TestSPCV0010DryMergeClassifiesCleanGeneratedOnlyAndConflict`, `TestSPCV0010DirectoryRenameSplitIsConflict` |
| SPC-V0-011 | `BehindReport`, `Config.Digest` | `TestSPCV0011BehindReportRecordsBaseHeadCommitsAndDigest` |
| SPC-V0-012 | `staleFinding`, `runChangesBehind` | `TestSPCV0012StaleFindingNamesTicketOldestCommitAndAge`, `TestSPCV0012ChangesBehindCLI` |
| SPC-V0-013 | `arrivalCommits`, `arrivalTime`, `staleFinding` | `TestSPCV0013AgeIsArrivalOnBase` (a side-branch commit committed 60 days before the merge that brought it onto base is aged from the merge; fails when every commit arrives as itself) |
| SPC-V0-014 | `observeBase`, `baseObservationDetail` | `TestSPCV0014BaseObservationRecordsRefAndLastObservedTime` (local branch reflog, remote-tracking reflog, a newer `FETCH_HEAD`, a commit-ID base and an expired reflog; fails when the observation is absent), `TestSPCV0009BehindIsReadOnly` |
| SPC-V0-015 | `refuseShallowMergeBase`, `Check`, `Behind`, `privateMergeRepository` | `TestSPCV0015ShallowMergeBaseBeyondBoundaryRefuses` (no visible merge base and a boundary merge base: both commands refuse with the code; the full clone's dry merge is `conflict`; fails without the check, as a report or `changes-shared-range-unavailable`), `TestSPCV0015ShallowRangeCutByBoundaryRefuses` (merge base is base, a boundary commit inside `main..feature`: both commands refuse; check-shared returned 3 of the full clone's 7 commits without the symmetric-difference check), `TestSPCV0015ShallowMergeBaseInsideBoundaryMatchesFullClone` (behind and check-shared equal the full clone's reports, and a boundary commit as base is exact; fails with `changes-git-unavailable` when `GIT_SHALLOW_FILE` is not passed), `TestSPCV0015ShallowCrissCrossMergeRefuses` (two merge bases that keep their parents and a two-commit `main...feature` in a depth-3 clone: both commands refuse; the full clone reports the `cmd/f.go` add/add conflict, and check-shared reported clean before the several-merge-bases rule) |
