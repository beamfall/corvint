#!/usr/bin/env bash
# lane-dispatch-check.sh: screen a batch's lane ticket files before any lane starts (ASD-V0-645,
# V1-3768, V1-3770). Lanes whose tickets share files conflict at merge time (17 conflicted files and
# a fix made twice in one batch), a ticket without touch paths gives the scope claim and the
# RESOURCE_COLLISION check nothing to compare, and a ticket whose paths are absent at the lane base
# targets code the lane cannot see.
#
#   script/lane-dispatch-check.sh [--base REV] [--waive TICKET]... LANE=FILE...
#
# Each FILE is one lane's tickets: a JSON array of tickets carrying ticketId and
# record.effects.touchPaths (the form of a charter's tickets file). One line per finding:
#   EMPTY      a ticket has no touch paths (fill them, or --waive the ticket)
#   DUPLICATE  one ticket is in two lanes
#   OVERLAP    two lanes touch the same path, or a directory key (ending in /) holds the other path;
#              never waived: serialize the tickets or give them to one lane
#   ABSENT     with --base, a touch path is not at REV (a file by git cat-file -e, a directory key by
#              git ls-tree); --waive a ticket that creates its paths
# --waive takes a full ticket ID or its last segment (V1-3768) and waives EMPTY and ABSENT for that
# ticket only. Exit 0 with a PASS line, 1 on any finding, 2 on a usage error, an unreadable file,
# malformed JSON, an empty ticketId or one holding a control character, a base that is not a commit
# or a missing jq.
set -uo pipefail
export LC_ALL=C
me='lane-dispatch-check'
usage() { printf 'usage: script/lane-dispatch-check.sh [--base REV] [--waive TICKET]... LANE=FILE...\n' >&2; exit 2; }
refuse() { printf '%s: %s\n' "$me" "$*" >&2; exit 2; }
command -v jq > /dev/null 2>&1 || refuse 'jq is not installed'
base='' sha='' waive=() lanes=() files=()
while (($#)); do
  case $1 in
    --base) { (($# >= 2)) && [[ -n $2 ]]; } || usage; base=$2; shift 2 ;;
    --waive) { (($# >= 2)) && [[ -n $2 ]]; } || usage; waive+=("${2##*:}"); shift 2 ;;
    -*) usage ;;
    *=*) lanes+=("${1%%=*}"); files+=("${1#*=}"); shift ;;
    *) usage ;;
  esac
done
((${#lanes[@]})) || usage
if [[ -n $base ]]; then
  sha=$(git rev-parse --verify -q "$base^{commit}") || refuse "base $base is not a commit; use a commit SHA or a ref that names one"
fi
rows=$(mktemp "${TMPDIR:-/tmp}/lane-dispatch-check.XXXXXX") || refuse 'mktemp failed'
trap '/bin/rm -f "$rows"' EXIT
for ((i = 0; i < ${#lanes[@]}; i++)); do
  lane=${lanes[i]} file=${files[i]}
  [[ $lane =~ ^[A-Za-z0-9._-]+$ ]] || refuse "lane name '$lane' is not valid; use letters, digits, dot, dash or underscore"
  for ((j = 0; j < i; j++)); do [[ ${lanes[j]} != "$lane" ]] || refuse "lane $lane is named twice; use a distinct name for each lane"; done
  [[ -r $file ]] || refuse "lane $lane: cannot read $file; use a readable tickets file"
  jq -e 'type == "array" and all(.[]; type == "object" and (.ticketId | type) == "string"
      and ((.record.effects.touchPaths // []) | type == "array" and all(.[]; type == "string" and test("^[^\t\n]+$"))))' \
    "$file" > /dev/null 2>&1 || refuse "lane $lane: $file is not a JSON array of tickets with a string ticketId and string touchPaths; want the charter tickets-file form"
  # The rows below are unescaped TSV: a tab or newline in a ticket ID would shift its path into
  # another column and hide an overlap, so a non-empty ID without control characters is required.
  bad_id=$(jq -r 'map(.ticketId | select(. == "" or test("[[:cntrl:]]"))) | first // empty | @json' "$file") ||
    refuse "lane $lane: cannot read the ticket IDs of $file; run jq on it to see the error"
  [[ -z $bad_id ]] || refuse "lane $lane: ticketId $bad_id in $file is empty or holds a control character (tab, newline); use a non-empty ticket ID without control characters"
  # One row per ticket and path: lane, ticket, path ("" for a ticket with none).
  jq -r --arg lane "$lane" '.[] | .ticketId as $t | ((.record.effects.touchPaths // []) | if length == 0 then [""] else . end)[]
      | [$lane, $t, .] | join("\t")' "$file" >> "$rows" || refuse "lane $lane: cannot read the tickets of $file; run jq on it to see the error"
done
waived() { local w; for w in ${waive[@]+"${waive[@]}"}; do [[ $w == "${1##*:}" ]] && return 0; done; return 1; }
findings=0
finding() { printf '%s: %s\n' "$me" "$*"; findings=$((findings + 1)); }
tickets=$(cut -f1,2 "$rows" | sort -u | wc -l | tr -d ' ')
while IFS=$'\t' read -r lane t p; do
  if [[ -z $p ]]; then
    waived "$t" || finding "EMPTY lane=$lane ticket=$t: no touchPaths, so claims and collision checks have no path to compare; fill them before dispatch or pass --waive ${t##*:}"
  elif [[ -n $sha ]] && ! waived "$t"; then
    if [[ $p == */ ]]; then
      [[ -n $(git ls-tree -d "$sha" -- "${p%/}" 2> /dev/null) ]] && continue
    else
      git cat-file -e "$sha:$p" 2> /dev/null && continue
    fi
    finding "ABSENT lane=$lane ticket=$t path=$p is not at the lane base $sha; fix the path, rebase the lane, or pass --waive ${t##*:} for a ticket that creates it"
  fi
done < "$rows"
# DUPLICATE: one ticket (by its last segment) in two lanes.
while IFS=$'\t' read -r t ls; do
  finding "DUPLICATE ticket=$t is in lanes $ls; dispatch it to one lane"
done < <(cut -f1,2 "$rows" | sort -u | awk -F '\t' '{ id = $2; sub(/.*:/, "", id); if (!(id SUBSEP $1 in seen)) { seen[id SUBSEP $1] = 1; n[id]++; l[id] = l[id] (n[id] > 1 ? "," : "") $1 } }
  END { for (id in n) if (n[id] > 1) print id "\t" l[id] }' | sort)
# OVERLAP: paths of different tickets in different lanes that are equal or nested under a directory key.
while IFS= read -r line; do finding "$line"; done < <(awk -F '\t' '$3 != "" { L[NR] = $1; T[NR] = $2; P[NR] = $3; id = $2; sub(/.*:/, "", id); I[NR] = id }
  function covers(a, b) { return a == b || (a ~ /\/$/ && index(b, a) == 1) }
  END {
    for (i in P) for (j in P) {
      if (i + 0 >= j + 0 || L[i] == L[j] || I[i] == I[j]) continue
      if (covers(P[i], P[j]) || covers(P[j], P[i]))
        printf "OVERLAP lanes %s (%s %s) and %s (%s %s): serialize these tickets or dispatch them to one lane\n", L[i], T[i], P[i], L[j], T[j], P[j]
    }
  }' "$rows" | sort)
if ((findings)); then
  printf '%s: FAIL %d finding(s) in %d lanes, %d tickets\n' "$me" "$findings" "${#lanes[@]}" "$tickets"
  exit 1
fi
printf '%s: PASS %d lanes, %d tickets\n' "$me" "${#lanes[@]}" "$tickets"
