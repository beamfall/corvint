#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/doc-gates.sh GATE...
#
# ASD-V0-779 (V1-3615): `make doc-gates` runs every documentation gate in one call and reports
# every failure of that call. Each GATE is a make target, or `conformance/use-cases-v0`, which
# runs `go run ./conformance/use-cases-v0`. The gates run on DOC_GATES_JOBS workers (default a
# quarter of the CPUs, 1 to 6), each into its own log; each gate's output prints as one block,
# headed `doc-gates: PASS|FAIL GATE (Ns)`, as soon as it finishes, so a fast gate's failure shows
# within seconds and blocks never interleave (this needs no make --output-sync, which macOS make
# 3.81 lacks). A failing gate never stops another. The run ends with one line per failed gate,
# `doc-gates: FAIL N of M gates: ...` and the `make doc-gates DOC_GATES_RUN='...'` command that
# reruns only those, and exits 1; with every gate passed it prints `doc-gates: PASS M gates` and
# exits 0. Per-gate exit and seconds go to the git directory's doc-gates.timings.tsv.
# DOC_GATES_GO names the go binary for the conformance gate (default go).
set -uo pipefail
root=$(git rev-parse --show-toplevel 2> /dev/null) || { printf 'doc-gates: not in a Git worktree\n' >&2; exit 2; }
cd "$root" || exit 2
(($#)) || { printf 'usage: script/doc-gates.sh GATE...\n' >&2; exit 2; }
jobs=${DOC_GATES_JOBS:-}
if ! [[ $jobs =~ ^[1-9][0-9]*$ ]]; then
  cpus=$(getconf _NPROCESSORS_ONLN 2> /dev/null || sysctl -n hw.ncpu 2> /dev/null || echo 4)
  jobs=$((cpus / 4))
  ((jobs < 1)) && jobs=1
  ((jobs > 6)) && jobs=6
fi
command -v perl > /dev/null 2>&1 || { printf 'doc-gates: perl is required for the output lock\n' >&2; exit 2; }
dir=$(mktemp -d "${TMPDIR:-/tmp}/doc-gates.XXXXXX") || exit 2
# The workers run in their own process group, led by xargs ($xpid). On any exit, an interrupt
# included, the runner signals that whole group (each worker shell, its make and their
# descendants) before it removes the run directory, so nothing outlives the run or keeps printing.
xpid=''
cleanup() {
  if [[ -n $xpid ]]; then
    kill -TERM -- "-$xpid" 2> /dev/null
    sleep 0.2
    kill -KILL -- "-$xpid" 2> /dev/null
  fi
  rm -rf "$dir"
}
trap cleanup EXIT
trap 'exit 130' INT TERM
export DOC_GATES_DIR=$dir DOC_GATES_MAKE=${MAKE:-make} DOC_GATES_GO=${DOC_GATES_GO:-go}
# DOC_GATES_LOCK names the output lock file (default inside the run directory).
export DOC_GATES_LOCK=${DOC_GATES_LOCK:-$dir/lock}

# gate N GATE runs one gate into its log, then prints its block under the output lock.
gate() {
  local n=$1 name=$2 rc t0=$SECONDS secs
  if [[ $name == conformance/use-cases-v0 ]]; then
    GOTOOLCHAIN=local "$DOC_GATES_GO" run ./conformance/use-cases-v0 > "$DOC_GATES_DIR/$n.log" 2>&1 < /dev/null
  else
    "$DOC_GATES_MAKE" --no-print-directory "$name" > "$DOC_GATES_DIR/$n.log" 2>&1 < /dev/null
  fi
  rc=$?
  secs=$((SECONDS - t0))
  printf '%s %s\n' "$rc" "$secs" > "$DOC_GATES_DIR/$n.rc"
  # flock(2) through perl, whose descriptor survives the exec ($^F): the kernel drops the lock
  # when its holder exits or is killed, so a dead holder never stalls the other workers, which an
  # O_EXCL or mkdir lock left behind would (and uutils mkdir is not atomic, V1-4251).
  # A lock, perl or print failure fails this gate (rc 97) and the worker, so the run cannot pass
  # without the gate's block.
  if ! DOC_GATES_HEAD="doc-gates: $( ((rc)) && echo FAIL || echo PASS) $name (${secs}s)" DOC_GATES_LOG=$DOC_GATES_DIR/$n.log \
    perl -e '$^F = 1023; use Fcntl qw(:flock); open(my $l, ">>", shift) or die "doc-gates: lock: $!\n";
      flock($l, LOCK_EX) or die "doc-gates: flock: $!\n"; exec @ARGV or die "doc-gates: exec: $!\n"' "$DOC_GATES_LOCK" \
    sh -c 'printf "%s\n" "$DOC_GATES_HEAD" && sed "s/^/  | /" "$DOC_GATES_LOG"'; then
    printf '97 %s\n' "$secs" > "$DOC_GATES_DIR/$n.rc"
    printf 'doc-gates: FAIL %s: its output could not be printed under the lock %s (gate exit %s)\n' "$name" "$DOC_GATES_LOCK" "$rc" >&2
    return 1
  fi
}
export -f gate

i=0
for name in "$@"; do
  i=$((i + 1))
  printf '%s\t%s\n' "$i" "$name"
done > "$dir/gates"
tr '\t' '\n' < "$dir/gates" | perl -e 'setpgrp(0, 0) or die "doc-gates: setpgrp: $!\n"; exec @ARGV or die "doc-gates: exec: $!\n"' \
  xargs -n 2 -P "$jobs" bash -c 'gate "$1" "$2"' _ &
xpid=$!
wait "$xpid"
xrc=$?
xpid=''

# The timings are written to this run's own file and renamed into place, so two runs in one
# worktree never interleave rows in one file.
timings=$(git rev-parse --git-path doc-gates.timings.tsv 2> /dev/null) || timings=''
printf 'target\texit\tseconds\n' > "$dir/timings.tsv"
failed=()
while IFS=$'\t' read -r n name; do
  read -r rc secs < "$dir/$n.rc" 2> /dev/null || { rc=1 secs=0; printf 'doc-gates: FAIL %s left no exit status\n' "$name"; }
  printf '%s\t%s\t%s\n' "$name" "$rc" "$secs" >> "$dir/timings.tsv"
  [[ $rc == 0 ]] || failed+=("$name")
done < "$dir/gates"
if [[ -n $timings ]]; then
  /bin/cp "$dir/timings.tsv" "$timings.$$.tmp" 2> /dev/null && /bin/mv -f "$timings.$$.tmp" "$timings" || /bin/rm -f "$timings.$$.tmp"
fi
((xrc)) && printf 'doc-gates: FAIL the gate runner exited %s: a worker could not run or print its gate\n' "$xrc"
if ((${#failed[@]})); then
  for name in "${failed[@]}"; do printf 'doc-gates: FAIL %s\n' "$name"; done
  printf 'doc-gates: FAIL %d of %d gates: %s\n' "${#failed[@]}" "$#" "${failed[*]}"
  printf "rerun only those: make doc-gates DOC_GATES_RUN='%s'\n" "${failed[*]}"
  exit 1
fi
((xrc)) && exit 1
printf 'doc-gates: PASS %d gates\n' "$#"
