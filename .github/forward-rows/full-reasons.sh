#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# CIN-V0-8422 (V1-5196): tally why the AFP-V0-098 narrowing gate recorded FULL over the last N
# completed runs of a workflow, from the ci-narrowing-* artifacts each shard uploads (decision.json
# beside the record, CIN-V0-8421; an older run without decision.json falls back to the record's
# mode and reasons under the code `legacy`). gh and jq only; nothing of a tested commit runs.
#
#   full-reasons.sh N [EVENT]
#
# Environment: GH_TOKEN, REPO; WORKFLOW (default ci.yml); FULL_REASONS_DIR (scratch, default a
# mktemp directory removed on exit). N is 1 to 100 completed runs, newest first; EVENT
# (pull_request, merge_group, push) filters them. Prints one line per run,
# `run RUN_ID EVENT CONCLUSION MODE CODE,...` (MODE is NARROWED when every shard narrowed, FULL
# when any shard recorded FULL, none when no shard uploaded a decision, so the gate was skipped or
# refused before it ran: narrowing off, no MET verdict, a DOCS or REUSE docs-plan, or an
# unretained record), then a tally `code COUNT RUNS` per FULL reason code (the first five run ids),
# and a final `runs N full F narrowed W none U` line. Exit 0 unless gh cannot list the runs (exit 1).
set -u
n=${1:-}
event=${2:-}
workflow=${WORKFLOW:-ci.yml}
printf '%s' "$n" | grep -Eqx '[1-9][0-9]?|100' || {
  echo "full-reasons.sh: N must be 1 to 100, got '${n}'" >&2
  exit 2
}
case $event in '' | pull_request | merge_group | push) ;; *)
  echo "full-reasons.sh: EVENT must be pull_request, merge_group or push, got '$event'" >&2
  exit 2
  ;;
esac
if [ -n "${FULL_REASONS_DIR:-}" ]; then
  dir=$FULL_REASONS_DIR
  mkdir -p "$dir" || exit 1
else
  dir=$(mktemp -d "${TMPDIR:-/tmp}/full-reasons.XXXXXX") || exit 1
  trap '/bin/rm -rf "$dir"' EXIT
fi
query="per_page=$n&status=completed"
[ -z "$event" ] || query="$query&event=$event"
gh api "repos/$REPO/actions/workflows/$workflow/runs?$query" \
  --jq '.workflow_runs[] | select((.id | type) == "number") | "\(.id) \(.event) \(.conclusion // "none")"' > "$dir/runs" 2> "$dir/runs.err" || {
  echo "full-reasons.sh: gh could not list the runs of $workflow: $(head -c 200 "$dir/runs.err")" >&2
  exit 1
}
# codes DIR: every FULL reason code of the ci-narrowing-* artifacts under DIR, one per line, and
# the mode on the first line (FULL, NARROWED or none).
codes() {
  found=0 full=0
  : > "$1/codes"
  for d in "$1"/ci-narrowing-*; do
    [ -d "$d" ] || continue
    if [ -f "$d/decision.json" ]; then
      mode=$(jq -r 'select(.profile == "corvint-pr-narrowing-decision/0") | .mode' "$d/decision.json" 2> /dev/null) || mode=""
      [ "$mode" = FULL ] || [ "$mode" = NARROWED ] || continue
      found=1
      [ "$mode" != FULL ] || {
        full=1
        jq -r '.reasonCodes[]? | select(type == "string" and length > 0)' "$d/decision.json" >> "$1/codes" 2> /dev/null ||
          echo unreadable >> "$1/codes"
      }
    elif [ -f "$d/narrowing.json" ]; then
      mode=$(jq -r 'select(.profile == "corvint-pr-narrowing/0") | .mode' "$d/narrowing.json" 2> /dev/null) || mode=""
      [ "$mode" = FULL ] || [ "$mode" = NARROWED ] || continue
      found=1
      [ "$mode" != FULL ] || {
        full=1
        # An older record names no codes; keep its first reason's first words under `legacy`.
        jq -r '.reasonCodes[]? // empty' "$d/narrowing.json" 2> /dev/null | grep . >> "$1/codes" ||
          jq -r '.reasons[]? | "legacy:" + (split(" ")[0:4] | join("-"))' "$d/narrowing.json" >> "$1/codes" 2> /dev/null ||
          echo legacy >> "$1/codes"
      }
    fi
  done
  if [ "$found" = 0 ]; then echo none; elif [ "$full" = 1 ]; then echo FULL; else echo NARROWED; fi
  sort -u "$1/codes"
}
: > "$dir/tally"
runs=0 fulls=0 narrowed=0 nones=0
while read -r id ev conclusion; do
  printf '%s' "$id" | grep -Eqx '[0-9]+' || continue
  runs=$((runs + 1))
  mkdir -p "$dir/$id"
  gh run download "$id" --repo "$REPO" --pattern 'ci-narrowing-*' --dir "$dir/$id" > /dev/null 2>&1 || :
  result=$(codes "$dir/$id")
  mode=$(printf '%s\n' "$result" | sed -n 1p)
  list=$(printf '%s\n' "$result" | sed -n '2,$p' | paste -sd ',' -)
  case $mode in
  FULL)
    fulls=$((fulls + 1))
    printf '%s\n' "$result" | sed -n '2,$p' | while read -r code; do echo "$code $id" >> "$dir/tally"; done
    ;;
  NARROWED) narrowed=$((narrowed + 1)) ;;
  *) nones=$((nones + 1)) ;;
  esac
  echo "run $id $ev $conclusion $mode ${list:--}"
done < "$dir/runs"
if [ -s "$dir/tally" ]; then
  awk '{ count[$1]++; if (count[$1] <= 5) runs[$1] = runs[$1] (runs[$1] == "" ? "" : ",") $2 }
    END { for (c in count) printf "%d %s %s\n", count[c], c, runs[c] }' "$dir/tally" | sort -k1,1rn -k2,2 |
    awk '{ printf "code %s %d %s\n", $2, $1, $3 }'
fi
echo "runs $runs full $fulls narrowed $narrowed none $nones"
