#!/usr/bin/env bash
# usage: script/doc-gates-summary.sh CMD [ARG...]
# ASD-V0-782 (V1-3884): the doc-gates recipe runs its `make --keep-going TARGET...` through this
# wrapper. Output passes through unchanged; a copy of stderr is kept, and when CMD fails the run ends
# with one line naming every failed doc gate (an ARG that make reported as `*** [TARGET]` or
# `*** [Makefile:N: TARGET]`) and the command that reruns one alone, so the failing sub-gate is the
# last thing in a long CI log. A failure that names no ARG says so. The exit status is CMD's.
# DTL-V0-006: a `-j` ARG must carry a positive integer (the recipe's -j$(DOC_GATE_JOBS)); an empty
# DOC_GATE_JOBS would leave a bare -j, an unbounded job pool, so it exits 2 before CMD runs.
set -uo pipefail
(($#)) || { printf 'usage: script/doc-gates-summary.sh CMD [ARG...]\n' >&2; exit 2; }
for a in "$@"; do
  [[ $a == -j* && ! ${a#-j} =~ ^[1-9][0-9]*$ ]] || continue
  printf 'doc-gates-summary: DOC_GATE_JOBS must be a positive integer, got %q (from %s); refusing an unbounded or invalid job pool\n' "${a#-j}" "$a" >&2
  exit 2
done
# Help source changes need the generated command trees before the catalog gate.
if git diff HEAD --name-only -- 'cmd/*help*.go' 'cmd/**/*help*.go' 'internal/*/cli/*help*.go' 2>/dev/null | grep -E '.' >/dev/null ||
  git diff HEAD -- 'cmd/*.go' 'cmd/**/*.go' 'internal/*/cli/*.go' 2>/dev/null | grep -E '^[-+].*(Usage:|usage:|--help|helpText|help text)' >/dev/null; then
  printf 'doc-gates: HINT help text changed; check stale capcatalog trees with make capability-catalog, then stage internal/capcatalog/ (CAP-V0-040)\n' >&2
fi
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
