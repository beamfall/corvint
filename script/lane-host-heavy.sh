#!/usr/bin/env bash
# lane-host-heavy.sh: the source of the host wrapper ~/lanes/heavy.sh on each lane host
# (ASD-V0-766, V1-4057; docs/LANE-ORCHESTRATION.md section 2, V1-3222). The installed copy was
# written by hand outside the repository, so the GIT_CONFIG_GLOBAL isolation script/heavy.sh got
# (ASD-V0-909) never reached it: a caller's GIT_CONFIG_GLOBAL naming a directory made its
# `git rev-parse` fail, and the wrapper refused or ran the old prototype. Install it with
#   install -m 0755 script/lane-host-heavy.sh ~/lanes/heavy.sh
# and check the installed copy with `script/lane-host-check.sh --only wrapper` (DRIFT names a copy
# that differs from this file).
#
# usage: ~/lanes/heavy.sh <command...>, run from inside a lane worktree. The wrapper's own Git call
# runs with GIT_CONFIG_GLOBAL unset; the command keeps the caller's environment. It delegates to the
# worktree's script/heavy.sh when that script answers --print-lock-dir (it reads
# ~/.config/corvint/heavy.env, the host's one semaphore) and refuses otherwise (exit 2).
top=$(env -u GIT_CONFIG_GLOBAL git rev-parse --show-toplevel 2> /dev/null)
script=${top:+$top/script/heavy.sh}
if [[ -z $script || ! -x $script ]] || ! env -u GIT_CONFIG_GLOBAL HEAVY_SLOTS=0 "$script" --print-lock-dir > /dev/null 2>&1; then
  printf 'heavy.sh: REFUSED no-delegate: run from a Corvint worktree whose script/heavy.sh answers --print-lock-dir (got %s); rebase the lane onto origin/main\n' "${script:-no worktree at $PWD}" >&2
  exit 2
fi
exec "$script" "$@"
