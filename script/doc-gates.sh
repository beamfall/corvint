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
dir=$(mktemp -d "${TMPDIR:-/tmp}/doc-gates.XXXXXX") || exit 2
trap 'rm -rf "$dir"' EXIT
export DOC_GATES_DIR=$dir DOC_GATES_MAKE=${MAKE:-make} DOC_GATES_GO=${DOC_GATES_GO:-go}

# gate N GATE runs one gate into its log, then prints its block under a lock file.
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
  # bash's own O_EXCL create: some mkdir(1) builds are not atomic under contention.
  until (set -o noclobber; : > "$DOC_GATES_DIR/lock") 2> /dev/null; do sleep 0.05; done
  printf 'doc-gates: %s %s (%ss)\n' "$( ((rc)) && echo FAIL || echo PASS)" "$name" "$secs"
  sed 's/^/  | /' "$DOC_GATES_DIR/$n.log"
  rm -f "$DOC_GATES_DIR/lock"
}
export -f gate

i=0
for name in "$@"; do
  i=$((i + 1))
  printf '%s\t%s\n' "$i" "$name"
done > "$dir/gates"
tr '\t' '\n' < "$dir/gates" | xargs -n 2 -P "$jobs" bash -c 'gate "$1" "$2"' _

timings=$(git rev-parse --git-path doc-gates.timings.tsv 2> /dev/null) || timings=''
[[ -n $timings ]] && printf 'target\texit\tseconds\n' > "$timings"
failed=()
while IFS=$'\t' read -r n name; do
  read -r rc secs < "$dir/$n.rc" 2> /dev/null || { rc=1 secs=0; printf 'doc-gates: FAIL %s left no exit status\n' "$name"; }
  [[ -n $timings ]] && printf '%s\t%s\t%s\n' "$name" "$rc" "$secs" >> "$timings"
  [[ $rc == 0 ]] || failed+=("$name")
done < "$dir/gates"
if ((${#failed[@]})); then
  for name in "${failed[@]}"; do printf 'doc-gates: FAIL %s\n' "$name"; done
  printf 'doc-gates: FAIL %d of %d gates: %s\n' "${#failed[@]}" "$#" "${failed[*]}"
  printf "rerun only those: make doc-gates DOC_GATES_RUN='%s'\n" "${failed[*]}"
  exit 1
fi
printf 'doc-gates: PASS %d gates\n' "$#"
