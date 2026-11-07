#!/bin/sh
# usage: script/dogfood-merge-base.sh [--paths] [MERGE]
#
# The BASE of a lane that merged the mainline forward (DWC-V0-7601, V1-4721, V1-0527): the merged
# mainline commit, which is the second parent of the merge. With no MERGE it reads the newest merge
# on HEAD's first-parent chain, so the lane's later bind and seal commits do not move it. It prints
# that commit's full SHA, or with --paths the paths whose recorded resolution differs from Git's own
# merge (`git show --remerge-diff`): the conflicted paths the build-log line names and the plan
# cites (docs/DOGFOOD.md "Rebinding after a conflicted merge-forward"). `make dogfood-finish
# BASE=merge-forward` (and every dogfood target that takes BASE) runs it.
#
# It refuses, exit 2, when there is no merge, the merge does not have exactly two parents, the
# merge is not in HEAD, or the merged parent is not on the mainline (DOGFOOD_MAINLINE_REF, default
# origin/main): a merged lane branch is not mainline, so its commits still need a binding of their own.
# With --paths it also refuses when `git show --remerge-diff` itself fails.
set -eu
export LC_ALL=C

refuse() {
  printf 'dogfood-merge-base: REFUSE %s\n  fix: %s\n' "$1" "$2" >&2
  exit 2
}

paths=0
if [ "${1-}" = --paths ]; then
  paths=1
  shift
fi
[ $# -le 1 ] || refuse usage 'pass at most one MERGE commit: script/dogfood-merge-base.sh [--paths] [MERGE]'
mainline=${DOGFOOD_MAINLINE_REF:-origin/main}

if [ $# -eq 1 ]; then
  merge=$(git rev-parse --verify --quiet "$1^{commit}") ||
    refuse merge-not-found "name a commit: $1 resolves to none"
  git merge-base --is-ancestor "$merge" HEAD ||
    refuse merge-not-in-head "$merge is not in HEAD's history; check out the lane that made the merge"
else
  merge=$(git rev-list --first-parent --merges -n 1 HEAD)
  [ -n "$merge" ] ||
    refuse no-merge-forward 'HEAD has no merge on its first-parent chain; merge the mainline forward (git merge origin/main), or pass the lane'"'"'s original BASE'
fi

# shellcheck disable=SC2046 # one line of SHAs: the merge, then its parents.
set -- $(git rev-list --parents -n 1 "$merge")
[ $# -eq 3 ] ||
  refuse merge-not-two-parent "$merge has $(($# - 1)) parents; a merge-forward has two, the lane then the mainline"
base=$3

git rev-parse --verify --quiet "$mainline^{commit}" > /dev/null ||
  refuse mainline-ref-missing "fetch the mainline (git fetch origin main) or set DOGFOOD_MAINLINE_REF to it"
git merge-base --is-ancestor "$base" "$mainline" ||
  refuse merged-parent-not-mainline "$merge merged $base, which is not on $mainline; a merged lane branch is not mainline, so bind from the lane's original BASE (docs/DOGFOOD.md \"Check a merge batch of sealed changes\")"

if [ "$paths" = 1 ]; then
  # POSIX sh has no pipefail, so Git's status is taken before the sort: a failed remerge-diff is
  # refused, never printed as a merge without conflicted paths.
  listed=$(git show --remerge-diff --format= --name-only "$merge") ||
    refuse remerge-diff-failed "git show --remerge-diff $merge exited $?; it needs Git 2.36 or later and every object of the merge, so repair the error above and rerun"
  [ -z "$listed" ] || printf '%s\n' "$listed" | sort -u
else
  printf '%s\n' "$base"
fi
