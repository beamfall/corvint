#!/usr/bin/env bash
# check-dr-numbers.sh: refuse a divergence register that defines one DR- number twice (ASD-V0-642,
# V1-3764). Each lane allocates its next DR number from its own base, so two unmerged lanes can both
# add DR-0047 and the batch merge joins the two headings with no textual conflict.
#
#   script/check-dr-numbers.sh [FILE]     (default conformance/divergence-register.md)
#
# A heading line `#... DR-NNNN` defines a number. Exit 0 with a PASS line (also when FILE does not
# exist), 1 with one FAIL line per duplicated number naming every defining line, 2 on a usage error.
set -uo pipefail
export LC_ALL=C
(($# <= 1)) || { printf 'usage: script/check-dr-numbers.sh [FILE]\n' >&2; exit 2; }
file=${1:-conformance/divergence-register.md}
if [[ ! -e $file ]]; then
  printf 'check-dr-numbers: PASS no register at %s\n' "$file"
  exit 0
fi
[[ -r $file ]] || { printf 'check-dr-numbers: cannot read %s\n' "$file" >&2; exit 2; }
out=$(awk -v f="$file" 'match($0, /^#+[ \t]+DR-[0-9]+/) {
    id = substr($0, RSTART, RLENGTH); sub(/^#+[ \t]+/, "", id)
    n[id]++; at[id] = at[id] (n[id] > 1 ? ", " : "") f ":" NR; total++
  }
  END {
    bad = 0
    for (id in n) if (n[id] > 1) { print "check-dr-numbers: FAIL " id " is defined " n[id] " times (" at[id] "); renumber the later entry to the next free DR number"; bad++ }
    if (bad) exit 1
    print "check-dr-numbers: PASS " total + 0 " DR numbers in " f " are distinct"
  }' "$file" | sort)
rc=$?
printf '%s\n' "$out"
exit "$rc"
