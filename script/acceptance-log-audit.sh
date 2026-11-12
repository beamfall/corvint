#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

# ACR-V1-006, ACR-V1-007: compare docs/specs/openspec/acceptance-log.tsv with the committed
# requirement index and the plan's checklist. One line per finding:
#   MISSING_ROW    a PROD requirement of the index, or a plan O-row, with no log row
#   UNKNOWN_ROW    a row whose id is no PROD requirement, plan O-row or change folder
#   DUPLICATE_ROW  a second row for one id
#   MODE_MISMATCH  a prod row whose mode disagrees with the index (owner with owner, auto with
#                  standing, pending-owner with unrecorded); the line names both modes
#   MISSING_CHANGE_ROW  a change folder, active or archived, with no change row (ACR-V1-013)
#   HEADER_MISMATCH     a change folder whose proposal.md `Intent status:` disagrees with its
#                       row: pending-owner wants "proposed...", auto and owner "accepted..."
#                       (ACR-V1-014)
#   INDEX_MISMATCH      an active change folder whose README index row is missing or does not
#                       read waiting for owner, auto-accepted or accepted for its row (ACR-V1-015)
# then a total line. It reports and exits 0; --strict exits 1 on any finding. A missing or
# malformed log, index or plan, or a row with no date, ownerRef or rationale, exits 2. It cannot tell who wrote a row (ACR-V1-008): identity
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

readme=$root/docs/specs/openspec/README.md
folders=
active=
archived=
headers=
if test -d "$changes"; then
  # A change is an immediate folder of changes/ or of changes/archive/, never a nested one.
  active=$(find "$changes" -mindepth 1 -maxdepth 1 -type d ! -name archive | sed 's|.*/||' | LC_ALL=C sort)
  if test -d "$changes/archive"; then
    archived=$(find "$changes/archive" -mindepth 1 -maxdepth 1 -type d | sed 's|.*/||' | LC_ALL=C sort)
  fi
  folders="$active
$archived"
  # One line per folder: its name, a tab and the first `Intent status:` of its proposal.md.
  for name in $active; do
    status=$(sed -n 's/^Intent status: *//p' "$changes/$name/proposal.md" 2>/dev/null | head -n 1)
    headers="$headers$name	$status
"
  done
  for name in $archived; do
    status=$(sed -n 's/^Intent status: *//p' "$changes/archive/$name/proposal.md" 2>/dev/null | head -n 1)
    headers="$headers$name	$status
"
  done
fi
# One line per README index row: the change name, a tab and the row's last cell.
index_rows=
if test -r "$readme"; then
  index_rows=$(sed -n 's/^| `\([^`]*\)` |.*| *\([^|]*[^| ]\) *|$/\1	\2/p' "$readme")
fi

status=0
FOLDERS=$folders ACTIVE=$active ARCHIVED=$archived HEADERS=$headers INDEXROWS=$index_rows \
awk -F'\t' -v strict="$strict" -v logname="$log" -v indexname="$index" '
  function bad(message) { printf "acceptance-log-audit: %s\n", message > "/dev/stderr"; failed = 1; exit 2 }
  BEGIN {
    want["owner"] = "owner"; want["auto"] = "standing"; want["pending-owner"] = "unrecorded"
    # ACR-V1-014 and ACR-V1-015: what a header begins with and what the index says per mode.
    head["owner"] = "accepted"; head["auto"] = "accepted"; head["pending-owner"] = "proposed"
    shown["owner"] = "accepted"; shown["auto"] = "auto-accepted"; shown["pending-owner"] = "waiting for owner"
    count = split(ENVIRON["FOLDERS"], names, "\n")
    for (i = 1; i <= count; i++) {
      if (names[i] == "") continue
      folder[names[i]] = 1
      # An archived folder is named YYYY-MM-DD-<change>.
      short = names[i]
      if (sub(/^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]-/, "", short)) folder[short] = 1
    }
    activeCount = split(ENVIRON["ACTIVE"], active, "\n")
    archivedCount = split(ENVIRON["ARCHIVED"], archived, "\n")
    n = split(ENVIRON["HEADERS"], lines, "\n")
    for (i = 1; i <= n; i++) {
      tab = index(lines[i], "\t")
      if (tab) header[substr(lines[i], 1, tab - 1)] = substr(lines[i], tab + 1)
    }
    n = split(ENVIRON["INDEXROWS"], lines, "\n")
    for (i = 1; i <= n; i++) {
      tab = index(lines[i], "\t")
      if (tab) indexed[substr(lines[i], 1, tab - 1)] = substr(lines[i], tab + 1)
    }
  }
  # checkChange applies ACR-V1-013 to ACR-V1-015 to one folder; key is its log id.
  function checkChange(name, key, isActive,   status, mode, row) {
    if (!(key in seen) || seen[key] != "change") {
      printf "MISSING_CHANGE_ROW\t%s\n", name; changeMissing++
      return
    }
    mode = mode_[key]
    status = header[name]
    if (status == "" || index(status, head[mode]) != 1) {
      printf "HEADER_MISMATCH\t%s\tlog %s\theader %s\n", name, mode, (status == "" ? "(none)" : status); headerMismatch++
    }
    # Test membership before reading: reading indexed[name] would create the element.
    row = (name in indexed) ? indexed[name] : "(none)"
    if (isActive && row != shown[mode]) {
      printf "INDEX_MISMATCH\t%s\tlog %s\tindex %s\twant %s\n", name, mode, row, shown[mode]; indexMismatch++
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
    if ($4 !~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$/) bad(logname ":" FNR ": date " $4 " is not YYYY-MM-DD")
    if ($5 == "" || $6 == "") bad(logname ":" FNR ": ownerRef and rationale must not be empty")
    rows++
    if ($1 in seen) { printf "DUPLICATE_ROW\t%s\t%s\n", $1, $2; duplicate++; next }
    seen[$1] = $2; mode[$1] = $3; mode_[$1] = $3; listed[++listedCount] = $1
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
    for (i = 1; i <= archivedCount; i++) {
      if (archived[i] == "") continue
      short = archived[i]
      sub(/^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]-/, "", short)
      checkChange(archived[i], short, 0)
    }
    for (i = 1; i <= activeCount; i++) {
      if (active[i] != "") checkChange(active[i], active[i], 1)
    }
    printf "total\trows %d\tmissing %d\tunknown %d\tduplicate %d\tmismatch %d\tchange-missing %d\theader %d\tindex %d\n", \
      rows, missing, unknown, duplicate, mismatch, changeMissing, headerMismatch, indexMismatch
    if (strict && missing + unknown + duplicate + mismatch + changeMissing + headerMismatch + indexMismatch > 0) exit 1
  }
' "$index" "$log" "$plan" || status=$?
exit "$status"
