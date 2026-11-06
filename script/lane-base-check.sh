#!/usr/bin/env bash
# lane-base-check.sh: refuse a lane worktree that was not cut from the lane base (ASD-V0-772,
# V1-3061). `git worktree add DIR -b BRANCH` without a start point cuts the branch from the host
# checkout's current HEAD, which on an ssh lane host is often days behind origin/main, and the lane
# then builds on the wrong base.
#
#   script/lane-base-check.sh BASE [DIR]
#
# Refuses (exit 1) when DIR (default .) is the shared checkout rather than a linked worktree, when
# the branch has no readable reflog (so where it was cut cannot be shown), when the branch's oldest
# reflog entry is not BASE, or when BASE is not an ancestor of HEAD; each REFUSE line names the fix.
# A branch cut elsewhere from a HEAD that merely contains BASE is refused, not passed on ancestry.
# Prints `PASS branch=... base=...` and exit 0 otherwise; exit 2 on a usage error or a BASE that is
# not a commit.
set -uo pipefail
export LC_ALL=C
g() { env -u GIT_CONFIG_GLOBAL git -C "$dir" "$@"; }
(($# == 1 || $# == 2)) || { printf 'usage: script/lane-base-check.sh BASE [DIR]\n' >&2; exit 2; }
dir=${2:-.}
base=$(g rev-parse --verify -q "$1^{commit}") || { printf 'lane-base-check: %s is not a commit\n' "$1" >&2; exit 2; }
fix="cut the lane with: git worktree add <lanes>/<LANE> -b <branch> $base"
gd=$(g rev-parse --path-format=absolute --git-dir) cd_=$(g rev-parse --path-format=absolute --git-common-dir)
[[ $gd != "$cd_" ]] || { printf 'lane-base-check: REFUSE shared-checkout %s is the host checkout; %s\n' "$dir" "$fix"; exit 1; }
branch=$(g symbolic-ref -q --short HEAD) || { printf 'lane-base-check: REFUSE detached HEAD; %s\n' "$fix"; exit 1; }
first=$(g reflog show --format=%H "refs/heads/$branch" -- 2> /dev/null | tail -1)
if [[ -z $first ]]; then
  printf 'lane-base-check: REFUSE no-reflog %s has no reflog, so its creation base cannot be shown (core.logAllRefUpdates off?); %s\n' "$branch" "$fix"; exit 1
fi
if [[ $first != "$base" ]]; then
  printf 'lane-base-check: REFUSE wrong-base %s was created at %s, not %s; %s\n' "$branch" "$first" "$base" "$fix"; exit 1
fi
g merge-base --is-ancestor "$base" HEAD || { printf 'lane-base-check: REFUSE base-not-ancestor %s is not in %s; %s\n' "$base" "$branch" "$fix"; exit 1; }
printf 'PASS branch=%s base=%s\n' "$branch" "$base"
