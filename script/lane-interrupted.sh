#!/usr/bin/env bash
# usage: script/lane-interrupted.sh record --dir LANE_DIR [--env LANE_ENV] [--lane NAME] [--base SHA]
#                                          [--reason TEXT] [ID...]
#        script/lane-interrupted.sh list [--repo REPO]
#        script/lane-interrupted.sh clear --dir LANE_DIR
#
# Interrupted-lane marker (ASD-V0-788, NEW-D6, docs/LANE-ORCHESTRATION.md section 17). A lane
# session that ends without its handback (a usage-limit kill, a crash, a closed SSH session) leaves
# commits and edits in its worktree but no record of which tickets are left. The marker is derived
# from Git state alone, so the orchestrator writes it on the lane's host after the fact; the dead
# lane does nothing.
#
# record reads LANE, BASE and TICKETS from LANE_ENV (the lane.env of `lane-dispatch.sh inputs`);
# --lane, --base and positional IDs override it. It writes <worktree git dir>/corvint/lane-interrupted
# (inside .git, so the worktree stays as it was) and prints the same lines:
#   lane-interrupted/0
#   lane=<LANE>  dir=<worktree>  branch=<branch or detached>  base=<40 hex>  head=<40 hex>
#   commits=<first-parent commits in BASE..HEAD>  recorded=<UTC>  reason=<TEXT, default unknown>
#   cited=<ID@sha12 ...>    each ID a non-merge first-parent commit message in BASE..HEAD names
#                           as a whole token, with the newest such commit
#   remaining=<ID ...>      every other ID, in dispatch order
#   dirty=<n>               `git status --porcelain` entries, each listed as `path <XY> <path>`
# One key per line. Commits a merged batch tip brought in never count, and cited is not delivered:
# the resumed lane screens each ID again. A rerun replaces the marker.
#
# list prints one line per marker among REPO's worktrees (default: this checkout's repository) and
# exits 1 when any exists, 0 when none:
#   INTERRUPTED lane=<LANE> head=<12 hex> remaining=<n> dirty=<n> dir=<worktree>
# A marker that does not parse prints `INTERRUPTED lane=? reason=malformed <file>` and still counts.
#
# clear removes the marker once the resumed lane has handed back. Exit 2 on a refusal; a refusal
# writes nothing.
set -uo pipefail
export LC_ALL=C
me=lane-interrupted
refuse() { printf '%s: REFUSE %s\n' "$me" "$*" >&2; exit 2; }
usage() { sed -n '2,5p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }

# marker_path DIR: the marker file of the worktree at DIR.
marker_path() {
  local gd
  gd=$(git -C "$1" rev-parse --absolute-git-dir 2> /dev/null) && [[ -n $gd ]] || return 1
  printf '%s/corvint/lane-interrupted\n' "$gd"
}

# The cited and remaining lines from a `git log -z --format=%H%n%B` file and the IDs.
cite_perl='
use strict;
use warnings;
my ($log, @ids) = @ARGV;
open my $fh, "<", $log or exit 3;
local $/ = "\0";
my @commits;
while (my $rec = <$fh>) {
  chomp $rec;
  my ($sha, $body) = split /\n/, $rec, 2;
  push @commits, [$sha, defined $body ? $body : ""];
}
my (@cited, @left);
for my $id (@ids) {
  my ($hit) = grep { $_->[1] =~ /(?<![\w-])\Q$id\E(?![\w]|-\w)/ } @commits;
  if ($hit) { push @cited, $id . "@" . substr($hit->[0], 0, 12) } else { push @left, $id }
}
print "cited=", join(" ", @cited), "\nremaining=", join(" ", @left), "\n";
'

# env_value FILE KEY: the value of KEY=... in a lane.env, read as data, never sourced.
env_value() {
  sed -n "s/^$2=//p" "$1" | tail -n 1
}

record() {
  local dir='' envf='' lane='' base='' reason=unknown ids=() given=0 id sha head branch marker tmp n
  while (($#)); do
    case $1 in
      --dir | --env | --lane | --base | --reason)
        [[ $# -ge 2 ]] || refuse "usage: $1 needs a value"
        case $1 in --dir) dir=$2 ;; --env) envf=$2 ;; --lane) lane=$2 ;; --base) base=$2 ;; --reason) reason=$2 ;; esac
        shift 2
        ;;
      -*) refuse "unknown-option $1; usage: record --dir LANE_DIR [--env FILE] [--lane L] [--base REV] [--reason R] [ID...]" ;;
      *) ids+=("$1"); given=1; shift ;;
    esac
  done
  [[ -n $dir ]] || refuse 'dir-required: pass --dir LANE_DIR, the lane worktree'
  if [[ -n $envf ]]; then
    [[ -f $envf && -r $envf ]] || refuse "env-unreadable $envf; fix: pass the lane.env the dispatch wrote, or --lane and --base"
    [[ -n $lane ]] || lane=$(env_value "$envf" LANE)
    [[ -n $base ]] || base=$(env_value "$envf" BASE)
    ((given)) || read -r -a ids <<< "$(env_value "$envf" TICKETS)"
  fi
  [[ $lane =~ ^[a-z0-9][a-z0-9.-]*$ ]] || refuse "bad-lane '$lane'; fix: pass --lane or LANE= in --env (lower-case letters, digits, . and -)"
  ((${#ids[@]})) || refuse 'no-tickets: name the dispatched IDs (positional, or TICKETS= in --env)'
  for id in ${ids[@]+"${ids[@]}"}; do
    [[ $id =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] || refuse "bad-id '$id'; fix: pass ticket IDs of letters, digits, ., _ and - only"
  done
  [[ $reason != *$'\n'* && $reason != *$'\r'* ]] || refuse 'bad-reason: one line only'
  [[ -n $base ]] || refuse 'base-required: pass --base or BASE= in --env'
  marker=$(marker_path "$dir") || refuse "not-a-worktree $dir; fix: pass --dir LANE_DIR, the lane worktree"
  dir=$(git -C "$dir" rev-parse --show-toplevel 2> /dev/null) || refuse "not-a-worktree $dir; fix: pass --dir LANE_DIR, the lane worktree"
  head=$(git -C "$dir" rev-parse -q --verify HEAD^{commit}) || refuse "no-head $dir; fix: the worktree has no commit; redispatch the lane instead of recording it"
  sha=$(git -C "$dir" rev-parse -q --verify "$base^{commit}") || refuse "base-unknown $base; fix: git fetch origin, then pass the dispatch BASE as a full sha"
  base=$sha
  git -C "$dir" merge-base --is-ancestor "$base" "$head" || refuse "base-not-ancestor $base is not an ancestor of $head in $dir; fix: pass the BASE the lane was dispatched from"
  branch=$(git -C "$dir" symbolic-ref -q --short HEAD) || branch=detached

  mkdir -p "${marker%/*}" || refuse "marker-dir-unwritable ${marker%/*}; fix: make the Git directory writable, then rerun"
  tmp=$(mktemp "${marker%/*}/.lane-interrupted.XXXXXX") || refuse "marker-dir-unwritable ${marker%/*}; fix: make the Git directory writable, then rerun"
  {
    git -C "$dir" log -z --first-parent --no-merges --format='%H%n%B' "$base..$head" > "$tmp.log" &&
      git -C "$dir" status --porcelain > "$tmp.status" &&
      n=$(git -C "$dir" rev-list --count --first-parent "$base..$head") &&
      {
        printf 'lane-interrupted/0\nlane=%s\ndir=%s\nbranch=%s\nbase=%s\nhead=%s\ncommits=%s\nrecorded=%s\nreason=%s\n' \
          "$lane" "$dir" "$branch" "$base" "$head" "$n" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$reason" &&
          perl -e "$cite_perl" "$tmp.log" ${ids[@]+"${ids[@]}"} &&
          printf 'dirty=%s\n' "$(grep -c '' "$tmp.status")" &&
          sed 's/^\(..\) /path \1 /' "$tmp.status"
      } > "$tmp" && mv -f "$tmp" "$marker"
  } || {
    /bin/rm -f "$tmp" "$tmp.log" "$tmp.status"
    refuse "record-failed: could not read $dir or write $marker; fix: rerun record once the worktree and Git directory are readable"
  }
  /bin/rm -f "$tmp.log" "$tmp.status"
  cat "$marker"
  printf '%s: recorded %s; resume the lane in %s at %s with the remaining IDs and keep its dirty paths (docs/LANE-ORCHESTRATION.md section 17)\n' \
    "$me" "$marker" "$dir" "${head:0:12}" >&2
}

list() {
  local repo=. common f found=0 lane head left dirty dir
  while (($#)); do
    case $1 in
      --repo) [[ $# -ge 2 ]] || refuse 'usage: --repo needs a value'; repo=$2; shift 2 ;;
      *) refuse "unknown-argument $1; usage: list [--repo DIR] | clear --dir LANE_DIR" ;;
    esac
  done
  common=$(git -C "$repo" rev-parse --git-common-dir 2> /dev/null) || refuse "not-a-repository $repo; fix: pass --repo, a checkout of the repository"
  [[ $common == /* ]] || common=$(cd "$repo" && cd "$common" && pwd) || refuse "not-a-repository $repo; fix: pass --repo, a checkout of the repository"
  shopt -s nullglob
  for f in "$common/corvint/lane-interrupted" "$common"/worktrees/*/corvint/lane-interrupted; do
    [[ -f $f ]] || continue
    found=1
    if [[ $(sed -n 1p "$f") != lane-interrupted/0 ]] || ! grep -q '^remaining=' "$f" || ! grep -Eq '^head=[0-9a-f]{40}$' "$f"; then
      printf 'INTERRUPTED lane=? reason=malformed %s\n' "$f"
      continue
    fi
    lane=$(sed -n 's/^lane=//p' "$f")
    head=$(sed -n 's/^head=//p' "$f")
    left=$(sed -n 's/^remaining=//p' "$f" | wc -w | tr -d ' ')
    dirty=$(sed -n 's/^dirty=//p' "$f")
    dir=$(sed -n 's/^dir=//p' "$f")
    printf 'INTERRUPTED lane=%s head=%s remaining=%s dirty=%s dir=%s\n' "$lane" "${head:0:12}" "$left" "${dirty:-?}" "$dir"
  done
  return $((found))
}

clear_marker() {
  local dir='' marker
  while (($#)); do
    case $1 in
      --dir) [[ $# -ge 2 ]] || refuse 'usage: --dir needs a value'; dir=$2; shift 2 ;;
      *) refuse "unknown-argument $1; usage: list [--repo DIR] | clear --dir LANE_DIR" ;;
    esac
  done
  [[ -n $dir ]] || refuse 'dir-required: pass --dir LANE_DIR'
  marker=$(marker_path "$dir") || refuse "not-a-worktree $dir; fix: pass --dir LANE_DIR, the lane worktree"
  if [[ -f $marker ]]; then
    /bin/rm -f "$marker" || refuse "clear-failed $marker; fix: make the marker writable, then rerun clear"
    printf '%s: cleared %s\n' "$me" "$marker"
  else
    printf '%s: no marker in %s\n' "$me" "$dir"
  fi
}

case ${1-} in
  record) shift; record "$@" ;;
  list) shift; list "$@" ;;
  clear) shift; clear_marker "$@" ;;
  -h | --help) sed -n '2,31p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
  *) usage ;;
esac
