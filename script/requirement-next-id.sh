#!/usr/bin/env bash
# usage: script/requirement-next-id.sh PREFIX
#
# SRG-V0-764 (V1-3389): prints the next free requirement ID of PREFIX (for example ASD-V0) from the
# staged specs, read through script/gen-spec-requirements.sh --lines, so the lookup never depends
# on a spec's IDs appearing in increasing order. The first line is `next PREFIX-NNN after
# PREFIX-MMM FILE:LINE` (or `next PREFIX-001 none-defined`); then each definition whose number is
# lower than one defined above it in the same spec prints as `out-of-order ID FILE:LINE after ID
# LINE`, and a last `out-of-order=N` line counts them. It reports and exits 0: reordering a spec is
# a separate change. A malformed PREFIX exits 2; a refused generation exits with its status.
set -uo pipefail
export LC_ALL=C

prefix=${1:-}
if (($# != 1)) || [[ ! $prefix =~ ^[A-Z][A-Z0-9]*(-[A-Z0-9]+)*$ ]]; then
  printf 'requirement-next-id: REFUSE bad-prefix %q: pass one prefix such as ASD-V0 (SRG-V0-764)\n' "$prefix" >&2
  exit 2
fi
here=$(CDPATH='' cd -- "$(dirname "$0")" && pwd -P)
lines=$("$here/gen-spec-requirements.sh" --lines) || exit
awk -F'\t' -v prefix="$prefix-" '
  NR > 1 && index($1, prefix) == 1 && substr($1, length(prefix) + 1) ~ /^[0-9][0-9][0-9]$/ {
    n = substr($1, length(prefix) + 1) + 0
    rows[++count] = $1 "\t" $2 "\t" $3 "\t" n
    if (n > top) { top = n; top_id = $1; top_at = $2 ":" $3 }
  }
  END {
    if (count == 0) { printf "next %s001 none-defined\n", prefix }
    else if (top >= 999) { printf "next none: %s999 is used (%s)\n", prefix, top_at }
    else { printf "next %s%03d after %s %s\n", prefix, top + 1, top_id, top_at }
    for (i = 1; i <= count; i++) print rows[i] | "sort -t \"\t\" -k2,2 -k3,3n"
    close("sort -t \"\t\" -k2,2 -k3,3n")
  }' <<< "$lines" | awk -F'\t' '
  NR == 1 { print; next }
  {
    if ($2 != file) { file = $2; max = -1 }
    if ($4 < max) { printf "out-of-order %s %s:%s after %s line %s\n", $1, $2, $3, max_id, max_line; ++bad }
    else { max = $4; max_id = $1; max_line = $3 }
  }
  END { printf "out-of-order=%d\n", bad + 0 }'
