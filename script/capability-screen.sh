#!/usr/bin/env bash
# capability-screen.sh: before filing a friction ticket as a missing capability, find the
# docs/CAPABILITIES.md rows that already deliver it (ASD-V0-771, V1-3697). V1-3649 filed
# "corvint test --dry-run prints 77 KB" while row `corvint test` already named
# `--dry-run --summary` (at most 4 KiB).
#
#   script/capability-screen.sh [--catalog FILE] 'FRICTION TEXT'
#
# Matches each `corvint <word>` (and each corvint-* binary name) in the text against the catalog
# rows whose first cell is that command in backticks, and lists the text's --flags each matched row
# also names, then every --flag the row names. Prints `MATCH LINE: ROW flags=... row-flags=...` per row and exit 0, or `NONE` and exit 1 when no
# row matches; 2 on a usage error or an unreadable catalog. A MATCH means: read the row before filing.
set -uo pipefail
export LC_ALL=C
catalog=docs/CAPABILITIES.md
if [[ ${1:-} == --catalog ]]; then (($# >= 2)) || exit 2; catalog=$2; shift 2; fi
(($# == 1)) && [[ -n $1 ]] || { printf "usage: script/capability-screen.sh [--catalog FILE] 'TEXT'\n" >&2; exit 2; }
[[ -r $catalog ]] || { printf 'capability-screen: cannot read %s\n' "$catalog" >&2; exit 2; }
text=$1
cmds=$(grep -oE 'corvint(-[a-z0-9]+)*( [a-z][a-z0-9-]*)?' <<< "$text" | sort -u)
flags=$(grep -oE -- '--[a-z][a-z0-9-]*' <<< "$text" | sort -u | tr '\n' ' ')
[[ -n $cmds ]] || { echo NONE; exit 1; }
awk -v cmds="$cmds" -v flags="$flags" '
  BEGIN { n = split(cmds, c, "\n"); m = split(flags, f, " ") }
  /^\| `/ {
    key = $0; sub(/^\| `/, "", key); sub(/`.*/, "", key)
    for (i = 1; i <= n; i++) {
      w = c[i]; b = w; sub(/ .*/, "", b)
      if (key == w || (key == b && !(w in seen))) {
        hit = ""
        for (j = 1; j <= m; j++) if (index($0, f[j]) > 0) hit = hit (hit == "" ? "" : ",") f[j]
        row = $0; all = ""
        while (match(row, /--[a-z][a-z0-9-]*/)) {
          t = substr(row, RSTART, RLENGTH); row = substr(row, RSTART + RLENGTH)
          if (!((NR, t) in had)) { had[NR, t] = 1; all = all (all == "" ? "" : ",") t }
        }
        printf "MATCH %d: %s flags=%s row-flags=%s\n", NR, key, hit, all; found = 1; if (key == w) seen[w] = 1; break
      }
    }
  }
  END { if (!found) { print "NONE"; exit 1 } }' "$catalog"
