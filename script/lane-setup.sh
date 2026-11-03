#!/usr/bin/env bash
# usage: script/lane-setup.sh --repo SHARED --dir DIR --branch BRANCH [--base REF] [--rebuild-corvint]
#
# Creates one lane checkout the way docs/LANE-ORCHESTRATION.md section 6 prescribes:
# `git -C SHARED worktree add -b BRANCH DIR REF` (REF default origin/main). Before anything it
# refuses a shallow SHARED clone with the `git fetch --unshallow` recovery (ASD-V0-072, V1-3230),
# since merge-base, `corvint affected` and the dogfood ranges all need the full history, and a
# missing REF or an existing DIR. After the worktree is added it installs the commit-msg hook that
# refuses AI co-author and "Generated with" trailers (check-ai-coauthor-trailers.sh --install-hook,
# ASD-V0-831, V1-3337); a hook it cannot install is named and exits 1 (the lane is still created).
# It also turns rerere off in the lane worktree's own config, since every linked worktree shares
# rr-cache and MERGE_RR.lock (ASD-V0-965, V1-3651); a config write it cannot make likewise exits 1.
# Last it judges the installed corvint against the new lane's base with script/lane-binary-check.sh
# (ASD-V0-757, V1-3761): a STALE or UNVERIFIED binary is named with both builds and exits 1 (the
# lane is still created); --rebuild-corvint rebuilds and installs it from the lane instead.
# ASD-V0-2203 first checks the host binary against fetched origin/main, before creating the lane;
# a stale host refuses even if --base pins a revision matching the installed binary.
# Exit 2 on any refusal. Relative SHARED and DIR resolve against the caller's working directory.
set -uo pipefail
export LC_ALL=C
refuse() { printf 'lane-setup: REFUSE %s\n' "$*" >&2; exit 2; }
repo='' dir='' branch='' ref=origin/main rebuild=()
while (($#)); do
  case $1 in
    --repo | --dir | --branch | --base) [[ $# -ge 2 && -n $2 ]] || refuse "$1 needs a value"
      case $1 in --repo) repo=$2 ;; --dir) dir=$2 ;; --branch) branch=$2 ;; --base) ref=$2 ;; esac
      shift 2 ;;
    --rebuild-corvint) rebuild=(--rebuild); shift ;;
    *) printf 'usage: script/lane-setup.sh --repo SHARED --dir DIR --branch BRANCH [--base REF] [--rebuild-corvint]\n' >&2; exit 2 ;;
  esac
done
[[ -n $repo && -n $dir && -n $branch ]] || refuse 'usage: --repo, --dir and --branch are required'
here=$(cd "$(dirname "$0")" && pwd) || exit 2
# Both paths resolve against the caller's directory once, so git (run in SHARED) and the checks
# and hook install after it name the same lane directory.
repo=$(cd "$repo" 2> /dev/null && pwd) || refuse "not-a-repository $repo"
[[ $dir == /* ]] || dir=$PWD/$dir
git -C "$repo" rev-parse --git-dir > /dev/null 2>&1 || refuse "not-a-repository $repo"
"$here/require-full-history.sh" lane-setup "$repo" || exit 2
[[ ! -e $dir ]] || refuse "dir-exists $dir"
sha=$(git -C "$repo" rev-parse --verify -q "$ref^{commit}") || refuse "unknown-base $ref (fetch the shared repository first)"
# ASD-V0-2203: judge the host's tool against origin/main even when a lane pins an older base.
# No implicit fetch: the orchestrator refreshes origin/main before dispatch.
if ! "$here/lane-binary-check.sh" ${rebuild[@]+"${rebuild[@]}"} --against origin/main "$repo"; then
  refuse 'host-binary-not-current (fix: fetch origin and run lane-binary-check.sh --rebuild in a clean checkout at origin/main)'
fi
git -C "$repo" worktree add -q -b "$branch" "$dir" "$sha" || refuse "worktree-add-failed $dir"
printf 'lane-setup: %s on %s at %s\n' "$dir" "$branch" "$sha"
if ! git -C "$repo" config extensions.worktreeConfig true || ! git -C "$dir" config --worktree rerere.enabled false; then
  printf 'lane-setup: %s was created, but rerere was not disabled in its worktree config; run git -C %s config extensions.worktreeConfig true and git -C %s config --worktree rerere.enabled false\n' "$dir" "$repo" "$dir" >&2
  exit 1
fi
guard=$dir/script/check-ai-coauthor-trailers.sh
[[ -x $guard ]] || guard=$here/check-ai-coauthor-trailers.sh
if ! (cd "$dir" && "$guard" --install-hook); then
  printf 'lane-setup: %s was created, but the ai-trailer commit-msg hook was not installed; see above\n' "$dir" >&2
  exit 1
fi
if ! "$here/lane-binary-check.sh" ${rebuild[@]+"${rebuild[@]}"} "$dir"; then
  printf 'lane-setup: %s was created, but the installed corvint does not match its base; see above\n' "$dir" >&2
  exit 1
fi
exit 0
