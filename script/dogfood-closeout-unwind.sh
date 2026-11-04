#!/usr/bin/env bash
# usage: dogfood-closeout-unwind.sh START
# DWC-V0-746 (V1-4072): make dogfood-closeout-precheck runs this when the precheck of the sealed head
# refuses.
# START is the head the closeout recorded before the bind. When every commit since START is a bind or
# seal commit (it changes only .corvint/change.cem.json or .corvint/changes/*.cem.json) and no
# tracked file is modified, the commits are dropped with git reset --keep START, so the repair starts
# from the lane's own head and a refreshed plan holds no sealed-CEM hunk. Anything else is left as it
# is, naming why and the command to unwind by hand. The dropped head stays reachable and is named.
set -uo pipefail
export LC_ALL=C
start=${1-}
prefix=dogfood-closeout
head=$(git rev-parse --verify -q 'HEAD^{commit}') || { printf '%s: unwind: no HEAD\n' "$prefix" >&2; exit 2; }
if [[ ! $start =~ ^[0-9a-f]{40}$ ]] || ! git cat-file -e "$start^{commit}" 2> /dev/null; then
  printf '%s: unwind: NOT_RUN no recorded start head (%s); the bind and seal commits stay at %s\n' "$prefix" "${start:-none}" "$head" >&2
  exit 0
fi
if [[ $head == "$start" ]]; then
  printf '%s: unwind: head unchanged at %s; nothing to unwind\n' "$prefix" "$start" >&2
  exit 0
fi
keep() {
  printf '%s: unwind: NOT_RUN %s; HEAD stays %s (to unwind by hand: git reset --keep %s)\n' "$prefix" "$1" "$head" "$start" >&2
  exit 0
}
git merge-base --is-ancestor "$start" "$head" || keep "the start head $start is not an ancestor of HEAD"
[[ -z $(git status --porcelain --untracked-files=no) ]] || keep "tracked files are modified"
commits=$(git rev-list --reverse "$start..$head") || keep "git rev-list failed"
[[ -z $(git rev-list --merges "$start..$head") ]] || keep "the range holds a merge commit, which diff-tree lists no paths for"
for commit in $commits; do
  while IFS= read -r path; do
    case $path in
      .corvint/change.cem.json | .corvint/changes/*.cem.json) ;;
      *) keep "commit $commit changes $path, which is not a bind or seal path" ;;
    esac
  done < <(git diff-tree --no-commit-id --name-only -r --root "$commit")
done
# Review: a commit made while the range was inspected is never dropped: HEAD must still be the head
# inspected right before the reset.
now=$(git rev-parse --verify -q 'HEAD^{commit}') || keep "HEAD cannot be read again"
[[ $now == "$head" ]] || keep "HEAD moved to $now while the bind and seal commits were inspected"
git reset -q --keep "$start" || keep "git reset --keep $start refused"
printf '%s: unwind: dropped %d bind and seal commit(s) %s; HEAD is %s again (the sealed head %s stays reachable: git reset --keep %s restores it)\n' \
  "$prefix" "$(wc -w <<< "$commits" | tr -d ' ')" "$(tr '\n' ' ' <<< "$commits" | sed 's/ $//')" "$start" "$head" "$head" >&2
