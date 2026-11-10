#!/bin/sh
set -eu

# ACR-V1-006, ACR-V1-007: compare docs/specs/openspec/acceptance-log.tsv with the committed
# requirement index and the plan's checklist. One line per finding:
#   MISSING_ROW    a PROD requirement of the index, or a plan O-row, with no log row
#   UNKNOWN_ROW    a row whose id is no PROD requirement, plan O-row or change folder
#   DUPLICATE_ROW  a second row for one id
#   MODE_MISMATCH  a prod row whose mode disagrees with the index (owner with owner, auto with
#                  standing, pending-owner with unrecorded); the line names both modes
# then a total line. It reports and exits 0; --strict exits 1 on any finding. A missing or
# malformed log, index or plan exits 2. It cannot tell who wrote a row (ACR-V1-008): identity
# is by convention. ROOT defaults to the repository that holds this script.

usage() { printf 'usage: %s [--strict] [ROOT]\n' "$0" >&2; exit 2; }
strict=0
if test "${1:-}" = --strict; then strict=1; shift; fi
test $# -le 1 || usage
root=${1:-$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)}
log=$root/docs/specs/openspec/acceptance-log.tsv
index=$root/docs/specs/REQUIREMENTS.tsv
plan=$root/docs/specs/PRODUCT-PLAN-1.0.md
changes=$root/docs/specs/openspec/changes
for file in "$log" "$index" "$plan"; do
  test -r "$file" || { printf 'acceptance-log-audit: cannot read %s\n' "$file" >&2; exit 2; }
done

folders=
if test -d "$changes"; then
  folders=$(find "$changes" -mindepth 1 -maxdepth 2 -type d ! -name archive | sed 's|.*/||')
fi

status=0
FOLDERS=$folders awk -F'\t' -v strict="$strict" -v logname="$log" -v indexname="$index" '
  function bad(message) { printf "acceptance-log-audit: %s\n", message > "/dev/stderr"; failed = 1; exit 2 }
  BEGIN {
    want["owner"] = "owner"; want["auto"] = "standing"; want["pending-owner"] = "unrecorded"
    count = split(ENVIRON["FOLDERS"], names, "\n")
    for (i = 1; i <= count; i++) {
      folder[names[i]] = 1
      # An archived folder is named YYYY-MM-DD-<change>.
      short = names[i]
      if (sub(/^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]-/, "", short)) folder[short] = 1
    }
  }
  FILENAME == indexname {
    if (FNR == 1) {
      if ($0 != "id\tfile\ttitle\tacceptance\tacceptedBy") bad(indexname ": unexpected header")
      next
    }
    if (NF != 5) bad(indexname ":" FNR ": want 5 columns, got " NF)
    if ($1 ~ /^PROD-/) { prod[$1] = $4; order[++total] = $1; kind[$1] = "prod" }
    next
  }
  FILENAME == logname {
    if (FNR == 1) {
      if ($0 != "id\tkind\tmode\tdate\townerRef\trationale") bad(logname ": unexpected header")
      next
    }
    if (NF != 6) bad(logname ":" FNR ": want 6 columns, got " NF)
    if ($2 != "prod" && $2 != "o-row" && $2 != "change") bad(logname ":" FNR ": kind " $2 " is not prod, o-row or change")
    if (!($3 in want)) bad(logname ":" FNR ": mode " $3 " is not owner, auto or pending-owner")
    rows++
    if ($1 in seen) { printf "DUPLICATE_ROW\t%s\t%s\n", $1, $2; duplicate++; next }
    seen[$1] = $2; mode[$1] = $3; listed[++listedCount] = $1
    next
  }
  # The plan: a checklist row is a table row whose first cell is O<number>.
  /^\| O[0-9]+ \|/ {
    id = $0; sub(/^\| /, "", id); sub(/ \|.*/, "", id)
    if (!(id in kind)) { orow[id] = 1; order[++total] = id; kind[id] = "o-row" }
  }
  END {
    if (failed) exit 2
    for (i = 1; i <= total; i++) {
      id = order[i]
      if (!(id in seen)) { printf "MISSING_ROW\t%s\t%s\n", id, kind[id]; missing++ }
    }
    for (i = 1; i <= listedCount; i++) {
      id = listed[i]
      known = (seen[id] == "prod" && id in prod) || (seen[id] == "o-row" && id in orow) \
        || (seen[id] == "change" && id in folder)
      if (!known) { printf "UNKNOWN_ROW\t%s\t%s\n", id, seen[id]; unknown++; continue }
      if (seen[id] == "prod" && want[mode[id]] != prod[id]) {
        printf "MODE_MISMATCH\t%s\tlog %s\tspec %s\n", id, mode[id], prod[id]; mismatch++
      }
    }
    printf "total\trows %d\tmissing %d\tunknown %d\tduplicate %d\tmismatch %d\n", \
      rows, missing, unknown, duplicate, mismatch
    if (strict && missing + unknown + duplicate + mismatch > 0) exit 1
  }
' "$index" "$log" "$plan" || status=$?
exit "$status"
