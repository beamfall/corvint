#!/usr/bin/env bash
# usage: script/lane-clean.sh --root DIR [--repo DIR] [--ref REF] [--active PATH]... [--apply]
#
# Lists merged lane checkouts under DIR and, only with --apply, removes them (ASD-V0-018,
# docs/LANE-ORCHESTRATION.md). Dry-run is the default.
#
# Each immediate subdirectory of --root that is a Git checkout (clone or linked worktree) is
# REMOVE only when every check passes, otherwise KEEP with the first failing reason:
#   active           it is (or is inside) an --active PATH
#   dirty            `git status --porcelain` is not empty (modified, staged or untracked files)
#   stash            it has stash entries
#   has-worktrees    a clone with registered linked worktrees (anywhere), or the list failed
#   unmerged-head    HEAD is not an ancestor of REF in --repo (or the commit is absent there)
#   unmerged-branch  a local branch head is not an ancestor of REF in --repo
#   git-unreadable   Git cannot read the checkout (any inspection exits non-zero)
# --repo defaults to this script's repository and --ref to origin/main; fetch --repo first, because
# the check reads its refs as they are and never fetches. A linked worktree is removed with
# `git worktree remove` from its own repository, a clone with rm -rf. Output lines:
#   KEEP <reason> <dir> | WOULD-REMOVE <dir> | REMOVED <dir> | REMOVE-FAILED <dir>
#   SUMMARY checkouts=N keep=N remove=N applied=<yes|no> free-before=<KiB> free-after=<KiB|->
set -uo pipefail

root=
repo=$(cd "$(dirname "$0")/.." && pwd)
ref=origin/main
apply=0
active=()
while [[ $# -gt 0 ]]; do
  case $1 in
    --root) root=${2-}; shift 2 || exit 2 ;;
    --repo) repo=${2-}; shift 2 || exit 2 ;;
    --ref) ref=${2-}; shift 2 || exit 2 ;;
    --active) active+=("${2-}"); shift 2 || exit 2 ;;
    --apply) apply=1; shift ;;
    -h | --help) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'lane-clean: REFUSE unknown-argument %s\n' "$1" >&2; exit 2 ;;
  esac
done
if [[ -z $root || ! -d $root ]]; then
  printf 'lane-clean: REFUSE root-required (pass --root DIR, an existing directory)\n' >&2
  exit 2
fi
if ! root=$(cd "$root" 2> /dev/null && pwd -P) || [[ -z $root || $root == / ]]; then
  printf 'lane-clean: REFUSE root-unusable (cannot resolve --root to a directory other than /)\n' >&2
  exit 2
fi
if ! target=$(git -C "$repo" rev-parse --verify --quiet "$ref^{commit}"); then
  printf 'lane-clean: REFUSE ref-unavailable %s in %s\n' "$ref" "$repo" >&2
  exit 2
fi
active_real=()
for a in ${active[@]+"${active[@]}"}; do
  [[ -e $a ]] && active_real+=("$(cd "$a" 2> /dev/null && pwd -P || printf '%s' "$a")") || active_real+=("$a")
done

merged() { # merged SHA: SHA exists in --repo and is an ancestor of the target
  git -C "$repo" cat-file -e "$1^{commit}" 2> /dev/null &&
    git -C "$repo" merge-base --is-ancestor "$1" "$target" 2> /dev/null
}
reason() { # reason DIR: prints the first KEEP reason, or nothing when the checkout may be removed
  local dir=$1 a head branch
  for a in ${active_real[@]+"${active_real[@]}"}; do
    if [[ $dir == "$a" || $dir == "$a"/* || $a == "$dir"/* ]]; then echo active; return; fi
  done
  local g=(git --no-optional-locks -C "$dir")
  head=$("${g[@]}" rev-parse --verify --quiet 'HEAD^{commit}') || { echo git-unreadable; return; }
  local status
  status=$("${g[@]}" status --porcelain --untracked-files=normal 2> /dev/null) || { echo git-unreadable; return; }
  [[ -z $status ]] || { echo dirty; return; }
  local out
  out=$("${g[@]}" stash list 2> /dev/null) || { echo git-unreadable; return; }
  [[ -z $out ]] || { echo stash; return; }
  if [[ -d $dir/.git ]]; then # a clone: rm -rf would orphan its linked worktrees, wherever they are
    out=$("${g[@]}" worktree list --porcelain 2> /dev/null) || { echo has-worktrees; return; }
    (($(grep -c '^worktree ' <<< "$out") <= 1)) || { echo has-worktrees; return; }
  fi
  merged "$head" || { echo unmerged-head; return; }
  out=$("${g[@]}" for-each-ref --format='%(objectname)' refs/heads 2> /dev/null) || { echo git-unreadable; return; }
  while IFS= read -r branch; do
    [[ -z $branch ]] || merged "$branch" || { echo unmerged-branch; return; }
  done <<< "$out"
}
remove_checkout() { # a linked worktree through its repository, a clone with rm -rf
  local common
  if [[ -f $1/.git ]]; then
    common=$(git -C "$1" rev-parse --path-format=absolute --git-common-dir 2> /dev/null) &&
      git --git-dir="$common" worktree remove "$1"
  else
    /bin/rm -rf -- "$1"
  fi
}
free_kib() { df -Pk "$root" | awk 'NR == 2 { print $4 }'; }

free_before=$(free_kib)
total=0 keep=0 remove=0
for dir in "$root"/*/; do
  dir=${dir%/}
  [[ -d $dir && ! -L $dir && -e $dir/.git ]] || continue
  total=$((total + 1))
  why=$(reason "$dir")
  if [[ -n $why ]]; then
    keep=$((keep + 1))
    printf 'KEEP %s %s\n' "$why" "$dir"
    continue
  fi
  remove=$((remove + 1))
  if ((apply == 0)); then
    printf 'WOULD-REMOVE %s\n' "$dir"
  elif remove_checkout "$dir"; then
    printf 'REMOVED %s\n' "$dir"
  else
    printf 'REMOVE-FAILED %s\n' "$dir"
  fi
done
free_after=-
((apply == 0)) || free_after=$(free_kib)
printf 'SUMMARY checkouts=%d keep=%d remove=%d applied=%s free-before=%s free-after=%s\n' \
  "$total" "$keep" "$remove" "$([[ $apply == 1 ]] && echo yes || echo no)" "$free_before" "$free_after"
