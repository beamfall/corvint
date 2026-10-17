# Shared-path change policy V0

Owner: Russell Lewis
Date: 2026-10-17
Requirement prefix: `SPC-V0`
Intent status: accepted under the owner's standing auto-accept (2026-10-11; V1-1591)
Delivery status: experimental
Authoritative inputs: V1-1591 / GitHub #1034 owner request; `../../AGENTS.md` invariants 2, 4, 7 and 8; `../DOGFOOD.md`.

## Agent digest
- Claim: A shared-path check names commits that change shared test infrastructure without a labelled commit, listing or run citation; a local splitter separates them.
- Status: accepted under the owner's standing auto-accept (2026-10-11; V1-1591)/experimental
- Exists: `internal/sharedpath` and the experimental `corvint changes check-shared` and `corvint changes split-shared` subcommands.
- Blocked on: no repository ships a `.corvint/shared-paths.json` yet; the notes listing and the run citation stay manual follow-ups; live-session detection sees only configured markers and Git's own in-progress state.
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
  `changes-shared-config-unavailable`.
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
  MUST probe merging head into base with `git merge-tree --write-tree`, which writes only
  unreachable objects. A conflicted path matching a `generated` glob is listed in
  `exemptConflicts` and is not a finding; any other conflicted path emits
  `shared-path-merge-conflict` (`clean-merge`).
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
  `--apply` it writes nothing.
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

## Boundaries and failure modes

Non-goals: pushing, force-pushing or opening pull requests; rewriting any ref but the named
branch; splitting merges or renames by content; writing notes or run citations; detecting an agent
session that leaves no configured marker; enforcing the policy in CI. `split-shared --apply` is an
explicit, operator-invoked writer of one local branch ref and unreachable objects; it is outside
the read-command invariant by design, like `index`. `check-shared` is a read command.

Failure modes: a live agent that leaves no marker and holds no Git lock is not detected, so the
user names the branch deliberately; a path renamed into a shared directory is one deletion and one
addition (renames are not detected); a ref whose head moves between the expected-head check and the
compare-and-swap aborts with the head-mismatch code and leaves only unreachable objects, which
`git gc` collects. Hooks are disabled (`core.hooksPath=/dev/null`) and signing is off for split
commits. Git stderr is never reflected.

## Rollback

An applied split prints `restore`, `git update-ref refs/heads/NAME OLD NEW`, which puts the old
head back while the old commits are still reachable from the reflog. Removing the feature deletes
`internal/sharedpath` and the two subcommand branches in `cmd/corvint/changes.go`; no stored state
depends on them.

## Traceability

| Requirement | Implementation | Focused evidence |
|---|---|---|
| SPC-V0-001 | `LoadConfig`, `Match`, `Config.Class` | `TestSPCV0001ConfigIsClosedAndClassifiesPaths` |
| SPC-V0-002 | `Check` | `TestSPCV0002CheckFindingsNameCommitPathAndMissingElement` |
| SPC-V0-003 | `mergeProbe` | `TestSPCV0003GeneratedConflictsAreExempt` |
| SPC-V0-004 | `SplitShared`, `overlay`, `commitTree` | `TestSPCV0004SplitKeepsTreeIdentityAndMessage` |
| SPC-V0-005 | `refuseLiveSession`, `SplitShared` | `TestSPCV0005SplitRefusesUnsafeStates` |
| SPC-V0-006 | `manualFollowUps` | `TestSPCV0006ManualFollowUpsAreOnlyListingAndCitation` |
| SPC-V0-007 | expected-head check and `update-ref` compare-and-swap | `TestSPCV0007HeadMismatchAbortsWithoutChanges` |
| SPC-V0-008 | `runSharedPath` | `TestSPCV0008ChangesSharedCLI` |
