#!/usr/bin/env bash
# usage: script/doc-gates-summary.sh CMD [ARG...]
# ASD-V0-782 (V1-3884): the doc-gates recipe runs its `make --keep-going TARGET...` through this
# wrapper. Output passes through unchanged; a copy of stderr is kept, and when CMD fails the run ends
# with one line naming every failed doc gate (an ARG that make reported as `*** [TARGET]` or
# `*** [Makefile:N: TARGET]`) and the command that reruns one alone, so the failing sub-gate is the
# last thing in a long CI log. A failure that names no ARG says so. The exit status is CMD's.
set -uo pipefail
(($#)) || { printf 'usage: script/doc-gates-summary.sh CMD [ARG...]\n' >&2; exit 2; }
log=$(mktemp "${TMPDIR:-/tmp}/doc-gates-summary.XXXXXX") || exit 2
trap '/bin/rm -f "$log"' EXIT
{
  "$@" 2>&1 1>&3 3>&- | tee "$log" >&2
  rc=${PIPESTATUS[0]}
} 3>&1
((rc == 0)) && exit 0
failed=()
reported=$(awk 'match($0, /\*\*\* \[[^]]*\]/) { t = substr($0, RSTART + 5, RLENGTH - 6); sub(/.*: /, "", t); print t }' "$log" | sort -u)
for a in "${@:2}"; do
  [[ $a == -* ]] && continue
  grep -qxF -- "$a" <<< "$reported" && failed+=("$a")
done
if ((${#failed[@]})); then
  printf 'doc-gates-summary: FAIL %d doc gate(s): %s\n' "${#failed[@]}" "${failed[*]}" >&2
  printf 'doc-gates-summary: rerun one alone with: make -s %s\n' "${failed[0]}" >&2
else
  printf 'doc-gates-summary: FAIL exit %s naming no doc gate; read the output above\n' "$rc" >&2
fi
exit "$rc"
