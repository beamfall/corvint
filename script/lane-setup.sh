#!/usr/bin/env bash
# usage: script/lane-setup.sh --repo SHARED --dir DIR --branch BRANCH [--base REF]
#
# Creates one lane checkout the way docs/LANE-ORCHESTRATION.md section 6 prescribes:
# `git -C SHARED worktree add -b BRANCH DIR REF` (REF default origin/main). Before anything it
# refuses a shallow SHARED clone with the `git fetch --unshallow` recovery (ASD-V0-062, V1-3230),
# since merge-base, `corvint affected` and the dogfood ranges all need the full history, and a
# missing REF or an existing DIR. After the worktree is added it installs the shared commit-msg
# hook that refuses AI co-author and "Generated with" trailers (ASD-V0-061, V1-3227); a hook it
# cannot install is named, and the lane is still created. Exit 2 on any refusal.
set -uo pipefail
export LC_ALL=C
refuse() { printf 'lane-setup: REFUSE %s\n' "$*" >&2; exit 2; }
repo='' dir='' branch='' ref=origin/main
while (($#)); do
  case $1 in
    --repo | --dir | --branch | --base) [[ $# -ge 2 && -n $2 ]] || refuse "$1 needs a value"
      case $1 in --repo) repo=$2 ;; --dir) dir=$2 ;; --branch) branch=$2 ;; --base) ref=$2 ;; esac
      shift 2 ;;
    *) printf 'usage: script/lane-setup.sh --repo SHARED --dir DIR --branch BRANCH [--base REF]\n' >&2; exit 2 ;;
  esac
done
[[ -n $repo && -n $dir && -n $branch ]] || refuse 'usage: --repo, --dir and --branch are required'
here=$(cd "$(dirname "$0")" && pwd) || exit 2
git -C "$repo" rev-parse --git-dir > /dev/null 2>&1 || refuse "not-a-repository $repo"
"$here/require-full-history.sh" lane-setup "$repo" || exit 2
[[ ! -e $dir ]] || refuse "dir-exists $dir"
sha=$(git -C "$repo" rev-parse --verify -q "$ref^{commit}") || refuse "unknown-base $ref (fetch the shared repository first)"
git -C "$repo" worktree add -q -b "$branch" "$dir" "$sha" || refuse "worktree-add-failed $dir"
printf 'lane-setup: %s on %s at %s\n' "$dir" "$branch" "$sha"
guard=$dir/script/check-ai-trailers.sh
[[ -x $guard ]] || guard=$here/check-ai-trailers.sh
(cd "$dir" && "$guard" --install) || printf 'lane-setup: the ai-trailer commit-msg hook was not installed; see above\n' >&2
exit 0
