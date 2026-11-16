#!/usr/bin/env bash
# lane-host-tt-verify.sh: the source of the lane-host helper ~/lanes/tt-verify.sh, which runs
# `make verify` for one corvint-tasks commit (V1-5650; docs/LANE-ORCHESTRATION.md section 14).
# Install it with
#   install -m 0755 script/lane-host-tt-verify.sh ~/lanes/tt-verify.sh
#
# usage: ~/lanes/tt-verify.sh COMMIT
#
# One run at a time per host: a second run refuses (exit 6) and never touches the first run's
# tree. Each run gets its own detached worktree, removed on exit, and its own log, kept under the
# lanes directory and named by commit and start time. The last line printed is the verdict:
#   tt-verify commit=SHA rc=N secs=N ok=N fail=PKGS timeout=PKGS log=PATH
# `fail` lists the packages whose tests failed and `timeout` those go test killed at its timeout;
# each is `none` when empty.
#
# Exit: make verify's status; 2 usage or not a commit; 3 worktree; 5 no flock or no pinned
# toolchain; 6 another run is live.
# Environment: TT_REPO (default ~/projects/corvint-tasks), TT_LANES (default ~/lanes).
set -u
[ $# -eq 1 ] || { echo "usage: tt-verify.sh COMMIT" >&2; exit 2; }
R=${TT_REPO:-$HOME/projects/corvint-tasks}
D=${TT_LANES:-$HOME/lanes}
C=$(git -C "$R" rev-parse --verify -q "$1^{commit}") || { echo "tt-verify: $1 is not a commit in $R" >&2; exit 2; }
S=$(git -C "$R" rev-parse --short=12 "$C")
command -v flock > /dev/null || { echo "tt-verify: flock is not installed; it guards the one-run-per-host rule" >&2; exit 5; }

exec 9> "$D/tt-verify.lock" || exit 3
if ! flock -n 9; then
  echo "tt-verify: another run is live on this host ($(cat "$D/tt-verify.owner" 2> /dev/null)); wait for its verdict line" >&2
  exit 6
fi
echo "commit=$S pid=$$ start=$(date +%T)" > "$D/tt-verify.owner"

W="$D/tt-verify-$S.wt"
L="$D/tt-verify-$S-$(date +%Y%m%dT%H%M%S).log"
cleanup() {
  cd / && git -C "$R" worktree remove --force "$W" 2> /dev/null
  : > "$D/tt-verify.owner"
}
trap cleanup EXIT
trap 'exit 130' INT TERM
# A tree left by a killed run is cleared here, under the lock, so it is never a live run's tree.
[ ! -e "$W" ] || git -C "$R" worktree remove --force "$W" || { echo "tt-verify: cannot clear $W" >&2; exit 3; }
git -C "$R" worktree add -q --detach "$W" "$C" || { echo "tt-verify: cannot add worktree $W" >&2; exit 3; }
cd "$W" || exit 3

# The Makefile pins one Go patch and runs with GOTOOLCHAIN=local, so put the pinned toolchain
# first on PATH when the host default differs (V1-5645).
want=$(sed -n 's/^WANT_GO[[:space:]]*:=[[:space:]]*//p' Makefile | head -1)
if [ -n "$want" ] && [ "$(GOTOOLCHAIN=local go env GOVERSION 2> /dev/null)" != "$want" ]; then
  found=
  for b in "$HOME/sdk/$want/bin" "$HOME"/go/pkg/mod/golang.org/toolchain@v0.0.1-"$want".*/bin; do
    if [ -x "$b/go" ]; then found=$b; break; fi
  done
  [ -n "$found" ] || { echo "tt-verify: the Makefile wants $want; it is not the host default and not under ~/sdk/$want or the module cache" >&2; exit 5; }
  PATH="$found:$PATH"; export PATH
fi

echo "tt-verify commit=$C go=$(GOTOOLCHAIN=local go env GOVERSION 2> /dev/null) load=$(cut -d' ' -f1-3 /proc/loadavg 2> /dev/null) start=$(date +%T)" | tee "$L"
t0=$(date +%s)
if [ -x script/heavy.sh ]; then script/heavy.sh make verify >> "$L" 2>&1; rc=$?
else make verify >> "$L" 2>&1; rc=$?; fi
secs=$(($(date +%s) - t0))

# A package whose FAIL line follows go test's timeout panic was killed by the timeout; report it
# apart from ordinary failures.
timeouts=$(awk '/^panic: test timed out after/ {t=1} /^FAIL\t/ { if (t) print $2; t=0 }' "$L" | sort -u | paste -sd, -)
fails=$(awk '/^panic: test timed out after/ {t=1} /^FAIL\t/ { if (!t) print $2; t=0 }' "$L" | sort -u | paste -sd, -)
grep -E '^(--- FAIL|panic:)' "$L" | cut -c1-200 | head -20
[ "$rc" -eq 0 ] || tail -n 6 "$L" | cut -c1-240
echo "tt-verify commit=$S rc=$rc secs=$secs ok=$(grep -c '^ok' "$L") fail=${fails:-none} timeout=${timeouts:-none} log=$L" | tee -a "$L"
exit "$rc"
