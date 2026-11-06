#!/usr/bin/env bash
# lane-scratch.sh: print this lane's own scratch directory, creating it with mode 0700
# (ASD-V0-765, V1-4056). One shared scratch directory let one lane's copy overwrite another
# lane's files; every lane now gets ROOT/LANE, and the directory remembers which lane owns it.
#
#   script/lane-scratch.sh [--root ROOT] [--lane LANE]
#
# ROOT defaults to LANE_SCRATCH_ROOT, else $HOME/lane-scratch. LANE defaults to the base name of
# the current worktree's top level (one worktree per lane, docs/LANE-ORCHESTRATION.md section 6).
# The directory holds a `.lane` marker: `lane=LANE` and, when the caller is inside a worktree,
# `worktree=TOP`. A directory whose marker names another lane, or another worktree for the same
# lane name, is refused (exit 2), so two lanes never write into one scratch directory. The marker
# is read, checked and written under a `.lane.lock` directory (mkdir is atomic) and published by
# rename, so two concurrent callers never both acquire one directory. Prints the absolute
# directory on stdout; exit 2 on a usage error or a refusal.
set -uo pipefail
export LC_ALL=C
refuse() { printf 'lane-scratch: REFUSE %s\n' "$*" >&2; exit 2; }
root=${LANE_SCRATCH_ROOT:-${HOME:-}/lane-scratch} lane=''
while (($#)); do
  case $1 in
    --root | --lane) [[ $# -ge 2 && -n $2 ]] || refuse "usage: $1 needs a value"
      case $1 in --root) root=$2 ;; --lane) lane=$2 ;; esac
      shift 2 ;;
    *) printf 'usage: script/lane-scratch.sh [--root ROOT] [--lane LANE]\n' >&2; exit 2 ;;
  esac
done
top=$(env -u GIT_CONFIG_GLOBAL git rev-parse --show-toplevel 2> /dev/null) || top=''
if [[ -z $lane ]]; then
  [[ -n $top ]] || refuse 'no-lane: not inside a worktree; use --lane LANE'
  lane=${top##*/}
fi
[[ $lane =~ ^[A-Za-z0-9._-]+$ && $lane != . && $lane != .. ]] ||
  refuse "bad-lane '$lane': use letters, digits, dot, dash or underscore"
[[ -n $root && $root != / ]] || refuse "bad-root '$root': set LANE_SCRATCH_ROOT or pass --root"
[[ $root == /* ]] || root=$PWD/$root
dir=$root/$lane marker=$root/$lane/.lane lock=$root/$lane/.lane.lock
mkdir -p "$root" && mkdir -p -m 0700 "$dir" || refuse "mkdir-failed $dir; use a writable --root"
# The directory may predate this call (or belong to another account): enforce 0700 and refuse
# when the mode cannot be set, instead of handing out a broader directory.
chmod 0700 "$dir" || refuse "chmod-failed $dir is not 0700 and cannot be changed; use a --root you own"
# Ownership is decided under a lock: mkdir either creates the lock directory or fails, so one
# caller at a time reads, checks and writes the marker. A lock left by a killed caller is
# reported, never removed here.
tries=0
until mkdir "$lock" 2> /dev/null; do
  tries=$((tries + 1))
  ((tries < 100)) || refuse "lock-busy $lock held for 5s; run rmdir $lock if no lane-scratch is running"
  sleep 0.05
done
trap 'rm -f "$lock/marker"; rmdir "$lock" 2> /dev/null' EXIT
publish() { # write the marker to a sibling and rename it into place, so readers see all or nothing
  printf '%s\n' "$want" > "$lock/marker" && mv -f "$lock/marker" "$marker" ||
    refuse "marker-write-failed $marker; use a writable --root"
}
want="lane=$lane"
[[ -n $top ]] && want+=$'\n'"worktree=$top"
if [[ -e $marker ]]; then
  have=$(cat "$marker") || refuse "marker-unreadable $marker; use a writable --root"
  other=$(sed -n 's/^lane=//p' <<< "$have")
  [[ $other == "$lane" ]] || refuse "foreign-scratch $dir belongs to lane $other; use your own lane name"
  owner=$(sed -n 's/^worktree=//p' <<< "$have")
  if [[ -n $top && -n $owner && $owner != "$top" ]]; then
    refuse "foreign-scratch $dir belongs to worktree $owner, not $top; use --lane with a distinct name"
  fi
  # A marker written outside any worktree (the orchestrator staging inputs) is adopted by the
  # first worktree that asks.
  [[ -n $top && -z $owner ]] && publish
else
  publish
fi
printf '%s\n' "$dir"
