#!/usr/bin/env bash
# lane-remote-edit.sh: apply a unified diff to a lane worktree on an ssh lane host (ASD-V0-768,
# V1-3834). Remote edits were written as `ssh HOST "perl -pi -e '...'"` one-liners, and one
# apostrophe or dollar sign in the edit broke the nested quoting. The patch travels on stdin, never
# in the command line, so its bytes need no quoting at all.
#
#   git diff | script/lane-remote-edit.sh [--check] HOST WORKTREE
#
# Runs `cd WORKTREE && git apply - && git diff --stat` on HOST with the patch on stdin (with
# --check, `git apply --check --stat -`, which validates and prints the patch's stat without writing).
# WORKTREE is an absolute path or one under ~, made of letters, digits and . _ / ~ - only, so it
# is safe inside the remote command. LANE_REMOTE_SSH overrides
# the ssh program (tests use a local stub). Exit 0 when the patch applies, 1 when git apply refuses
# it (nothing is written), 2 on a usage error or an empty patch.
set -uo pipefail
export LC_ALL=C
refuse() { printf 'lane-remote-edit: REFUSE %s\n' "$*" >&2; exit 2; }
check=''
[[ ${1:-} == --check ]] && { check=' --check'; shift; }
(($# == 2)) || refuse 'usage: git diff | script/lane-remote-edit.sh [--check] HOST WORKTREE'
host=$1 wt=$2
[[ $host =~ ^[A-Za-z0-9._@-]+$ ]] || refuse "bad-host '$host': use an ssh alias or user@host"
[[ $wt =~ ^[A-Za-z0-9._/~-]+$ && ( $wt == /* || $wt == '~'/* ) && $wt != *..* ]] ||
  refuse "bad-worktree '$wt': use an absolute or ~/ path of letters, digits and . _ / ~ -"
patch=$(cat; printf x) && patch=${patch%x}
[[ -n $patch ]] || refuse 'empty-patch: pipe a unified diff on stdin'
if [[ -n $check ]]; then remote="cd $wt && git apply --check --stat -"; else remote="cd $wt && git apply - && git diff --stat"; fi
if printf '%s' "$patch" | ${LANE_REMOTE_SSH:-ssh} "$host" "$remote"; then exit 0; fi
printf 'lane-remote-edit: FAIL git apply refused the patch on %s:%s; nothing was written\n' "$host" "$wt" >&2
exit 1
