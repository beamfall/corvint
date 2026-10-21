#!/usr/bin/env bash
# usage: script/scratch-sweep.sh [--root DIR]... [--repo DIR]... [--keep PATH]... [--archive DIR]
#                                [--age-hours H] [--apply]
#
# Reclaims closed-session scratch and stale worktree registrations (V1-1978, ASD-V0-022,
# docs/LANE-ORCHESTRATION.md section 6). Dry-run is the default; only --apply changes anything.
#
# Registrations: for each --repo, `git worktree prune` entries whose directory is gone
# (WOULD-PRUNE|PRUNED <repo> <entry>).
#
# Scratch: each immediate subdirectory of each --root is removed only when every check passes,
# otherwise KEEP with the first failing reason:
#   keep-list          it is, contains or is inside a --keep PATH
#   archive-dest       it is, contains or is inside the --archive DIR
#   in-use             a running process names it in its arguments or has its working directory
#                      inside it (canonical and /tmp-vs-/private/tmp spellings), or ps or lsof failed
#   recent             a file outside .git, or .git/logs/HEAD, changed within --age-hours (24)
#   contains-checkout  it holds a nested .git at any depth (sweep that directory as a --root)
#   unreadable         the nested-checkout search failed
#   has-worktrees      a clone with registered linked worktrees, or the list failed
#   git-unreadable     a Git inspection failed
#   multiple-stashes   more than one stash entry (only refs/stash would be bundled)
#   needs-archive      uncommitted, untracked, stashed or unpushed work and no --archive DIR
#   changed            under --apply, the use or Git state changed between the decision and removal
# A checkout with such work is archived first, to --archive DIR/<name>-<UTC time>/: status.txt,
# staged.patch (`git diff --binary --cached`), worktree.patch (`git diff --binary`), untracked.list
# and untracked.tgz, and repo.bundle (HEAD, branches and stash not reachable from a remote-tracking
# ref); a failed archive step keeps the checkout (ARCHIVE-FAILED). Under --apply the in-use, recent
# and Git-state checks are repeated immediately before archiving and before removal; a process that
# starts after that last check is not seen. A linked worktree's registration is pruned after
# removal. The --archive DIR is created only under --apply. Output lines:
#   KEEP <reason> <dir> | WOULD-REMOVE|REMOVED|REMOVE-FAILED <dir>
#   WOULD-ARCHIVE-REMOVE <dir> | ARCHIVED <dir> <archive> | ARCHIVE-FAILED <dir>
#   SUMMARY entries=N keep=N remove=N archive=N prune=N applied=<yes|no> free-before=<KiB> free-after=<KiB|->
set -uo pipefail
export LC_ALL=C

roots=()
repos=()
keeps=()
archive=
age_hours=24
apply=0
refuse() { printf 'scratch-sweep: REFUSE %s\n' "$*" >&2; exit 2; }
while [[ $# -gt 0 ]]; do
  case $1 in
    --root) [[ -n ${2-} ]] || refuse "root-required"; roots+=("$2"); shift 2 ;;
    --repo) [[ -n ${2-} ]] || refuse "repo-required"; repos+=("$2"); shift 2 ;;
    --keep) [[ -n ${2-} ]] || refuse "keep-required"; keeps+=("$2"); shift 2 ;;
    --archive) [[ -n ${2-} ]] || refuse "archive-required"; archive=$2; shift 2 ;;
    --age-hours) [[ ${2-} =~ ^[1-9][0-9]*$ ]] || refuse "invalid-age-hours ${2-}"; age_hours=$2; shift 2 ;;
    --apply) apply=1; shift ;;
    -h | --help) sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) refuse "unknown-argument $1" ;;
  esac
done
((${#roots[@]} + ${#repos[@]} > 0)) || refuse "root-or-repo-required (pass --root DIR and/or --repo DIR)"

resolve() { (cd "$1" 2> /dev/null && pwd -P); }
# canon PATH: the absolute canonical spelling of PATH, resolving its nearest existing ancestor.
canon() {
  local p=$1 parent
  [[ $p == /* ]] || p="$PWD/$p"
  if [[ -d $p ]]; then resolve "$p" && return 0; fi
  parent=$(canon "$(dirname "$p")") || return 1
  [[ $parent == / ]] && parent=
  printf '%s/%s\n' "$parent" "$(basename "$p")"
}
real_keeps=()
for k in ${keeps[@]+"${keeps[@]}"}; do
  r=$(canon "$k") || refuse "keep-unusable $k"
  real_keeps+=("${r%/}")
done
archive_real=
if [[ -n $archive ]]; then
  if ((apply)); then mkdir -p "$archive" 2> /dev/null || refuse "archive-unusable $archive"; fi
  archive_real=$(canon "$archive") || refuse "archive-unusable $archive"
  archive_real=${archive_real%/}
fi
# snapshot: the process arguments and working directories. Either one failing leaves it empty,
# which keeps every candidate as in-use.
snapshot() {
  processes=$(ps -axww -o args= 2> /dev/null) && processes+=$'\n' || processes=
  cwds=
  command -v lsof > /dev/null 2>&1 && cwds=$(lsof -a -d cwd -Fn 2> /dev/null | sed -n 's/^n//p')
  [[ -n $cwds ]] && cwds+=$'\n' || processes=
}
snapshot
# in_use DIR LOGICAL: 0 when a process names DIR (any spelling) or works inside it.
in_use() {
  local s
  [[ -n $processes ]] || return 0
  for s in "$1" "$2" "${1#/private}"; do
    [[ $processes == *"$s/"* || $processes == *"$s "* || $processes == *"$s"$'\n'* ]] && return 0
    [[ $cwds == *"$s"$'\n'* || $cwds == *"$s/"* ]] && return 0
  done
  return 1
}
free_kib() { df -Pk "$1" 2> /dev/null | awk 'NR == 2 { print $4 }'; }
free_dir=${roots[0]:-${repos[0]}}
free_before=$(free_kib "$free_dir")
n_entries=0 n_keep=0 n_remove=0 n_archive=0 n_prune=0

for repo in ${repos[@]+"${repos[@]}"}; do
  if ! entries=$(git -C "$repo" worktree prune --dry-run --verbose 2>&1); then
    printf 'KEEP git-unreadable repo %s\n' "$repo"
    continue
  fi
  while IFS= read -r line; do
    [[ -n $line ]] || continue
    n_prune=$((n_prune + 1))
    if ((apply)); then printf 'PRUNED %s %s\n' "$repo" "$line"; else printf 'WOULD-PRUNE %s %s\n' "$repo" "$line"; fi
  done <<< "$entries"
  ((apply)) && git -C "$repo" worktree prune
done

keep() { n_keep=$((n_keep + 1)); printf 'KEEP %s %s\n' "$1" "$2"; }
# inside DIR PATH: 0 when DIR is PATH, contains it, or is inside it.
inside() { [[ $1 == "$2" || $1 == "$2"/* || $2 == "$1"/* ]]; }
kept_by_list() {
  local k
  for k in ${real_keeps[@]+"${real_keeps[@]}"}; do inside "$1" "$k" && return 0; done
  return 1
}
recent() {
  local dir=$1
  [[ -n $(find "$dir" -path "$dir/.git" -prune -o -mmin "-$((age_hours * 60))" -print 2> /dev/null | head -n 1) ]] && return 0
  [[ -f $dir/.git/logs/HEAD && -n $(find "$dir/.git/logs/HEAD" -mmin "-$((age_hours * 60))" 2> /dev/null) ]]
}
# git_state DIR: prints HEAD, the porcelain status, the unpushed count and the stash count.
git_state() {
  local head status unpushed stash
  head=$(git -C "$1" rev-parse -q --verify HEAD 2> /dev/null) || head=unborn
  status=$(git --no-optional-locks -C "$1" status --porcelain 2> /dev/null) || return 1
  unpushed=$(git -C "$1" rev-list --count HEAD --branches --not --remotes 2> /dev/null) || return 1
  stash=$(git -C "$1" stash list 2> /dev/null | wc -l | tr -d ' ') || return 1
  printf '%s\n%s\n%s\n%s' "$head" "$unpushed" "$stash" "$status"
}
# unchanged DIR LOGICAL STATE: under --apply, re-read use, recency and Git state just before acting.
unchanged() {
  snapshot
  in_use "$1" "$2" && return 1
  recent "$1" && return 1
  [[ -z $3 ]] && return 0
  [[ $(git_state "$1") == "$3" ]]
}
# archive_checkout DIR NAME UNPUSHED STASH -> prints the archive directory; fails on any step.
archive_checkout() {
  local dir=$1 out
  out="$archive_real/$2-$(date -u +%Y%m%dT%H%M%SZ)"
  mkdir "$out" || return 1
  git -C "$dir" status --branch --porcelain=v1 > "$out/status.txt" || return 1
  git -C "$dir" diff --binary --cached > "$out/staged.patch" || return 1
  git -C "$dir" diff --binary > "$out/worktree.patch" || return 1
  git -C "$dir" ls-files -z --others --exclude-standard > "$out/untracked.list" || return 1
  if [[ -s $out/untracked.list ]]; then
    (cd "$dir" && tar --null -T "$out/untracked.list" -czf "$out/untracked.tgz") || return 1
  fi
  if (($3 > 0 || $4 > 0)); then
    local refs=(HEAD --branches)
    (($4 > 0)) && refs+=(refs/stash)
    git -C "$dir" bundle create "$out/repo.bundle" "${refs[@]}" --not --remotes > /dev/null 2>&1 || return 1
    git -C "$dir" bundle list-heads "$out/repo.bundle" > /dev/null 2>&1 || return 1
  fi
  printf '%s\n' "$out"
}
remove() {
  local dir=$1 common=$2
  if /bin/rm -rf "$dir"; then
    printf 'REMOVED %s\n' "$dir"
    [[ -n $common ]] && git --git-dir="$common" worktree prune 2> /dev/null
    return 0
  fi
  printf 'REMOVE-FAILED %s\n' "$dir"
}

for root in ${roots[@]+"${roots[@]}"}; do
  root_real=$(resolve "$root") || refuse "root-unusable $root"
  [[ $root_real != / ]] || refuse "root-unusable /"
  root_logical=$root
  [[ $root_logical == /* ]] || root_logical="$PWD/$root_logical"
  root_logical=${root_logical%/}
  for dir in "$root_real"/*/; do
    dir=${dir%/}
    [[ -d $dir && ! -L $dir ]] || continue
    logical="$root_logical/${dir##*/}"
    n_entries=$((n_entries + 1))
    if kept_by_list "$dir"; then keep keep-list "$dir"; continue; fi
    if [[ -n $archive_real ]] && inside "$dir" "$archive_real"; then keep archive-dest "$dir"; continue; fi
    if in_use "$dir" "$logical"; then keep in-use "$dir"; continue; fi
    if recent "$dir"; then keep recent "$dir"; continue; fi
    if ! nested=$(find "$dir" -mindepth 1 -path "$dir/.git" -prune -o -name .git -print -quit 2> /dev/null); then
      keep unreadable "$dir"
      continue
    fi
    if [[ -n $nested ]]; then keep contains-checkout "$dir"; continue; fi
    if [[ ! -e $dir/.git ]]; then
      n_remove=$((n_remove + 1))
      if ((apply == 0)); then printf 'WOULD-REMOVE %s\n' "$dir"; continue; fi
      if unchanged "$dir" "$logical" ""; then remove "$dir" ""; else n_remove=$((n_remove - 1)); keep changed "$dir"; fi
      continue
    fi
    common=
    if [[ -f $dir/.git ]]; then
      common=$(git -C "$dir" rev-parse --path-format=absolute --git-common-dir 2> /dev/null) || { keep git-unreadable "$dir"; continue; }
    else
      wt=$(git -C "$dir" worktree list --porcelain 2> /dev/null) || { keep has-worktrees "$dir"; continue; }
      if (($(grep -c '^worktree ' <<< "$wt") > 1)); then keep has-worktrees "$dir"; continue; fi
    fi
    state=$(git_state "$dir") || { keep git-unreadable "$dir"; continue; }
    { read -r _head; read -r unpushed; read -r stash; IFS= read -r -d '' status || true; } <<< "$state"
    if ((stash > 1)); then keep multiple-stashes "$dir"; continue; fi
    if [[ -z ${status%$'\n'} ]] && ((unpushed == 0 && stash == 0)); then
      n_remove=$((n_remove + 1))
      if ((apply == 0)); then printf 'WOULD-REMOVE %s\n' "$dir"; continue; fi
      if unchanged "$dir" "$logical" "$state"; then remove "$dir" "$common"; else n_remove=$((n_remove - 1)); keep changed "$dir"; fi
      continue
    fi
    if [[ -z $archive_real ]]; then keep needs-archive "$dir"; continue; fi
    if ((apply == 0)); then
      n_archive=$((n_archive + 1))
      printf 'WOULD-ARCHIVE-REMOVE %s\n' "$dir"
      continue
    fi
    if ! unchanged "$dir" "$logical" "$state"; then keep changed "$dir"; continue; fi
    n_archive=$((n_archive + 1))
    if ! out=$(archive_checkout "$dir" "${dir##*/}" "$unpushed" "$stash"); then
      printf 'ARCHIVE-FAILED %s\n' "$dir"
      continue
    fi
    printf 'ARCHIVED %s %s\n' "$dir" "$out"
    if unchanged "$dir" "$logical" "$state"; then remove "$dir" "$common"; else keep changed "$dir"; fi
  done
done

free_after=-
((apply)) && free_after=$(free_kib "$free_dir")
printf 'SUMMARY entries=%d keep=%d remove=%d archive=%d prune=%d applied=%s free-before=%s free-after=%s\n' \
  "$n_entries" "$n_keep" "$n_remove" "$n_archive" "$n_prune" "$( ((apply)) && echo yes || echo no)" "${free_before:--}" "${free_after:--}"
