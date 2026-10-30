#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-047: classify one forward row (AFP-V0-029) and write it to OUT.
# Usage: classify.sh OUT
# Environment: ROW_KIND (queue|pr), ROW_PR, ROW_SHA (queue commit; empty for a
# pull-request row, whose tested merge comes from its outcomes), ROW_HEAD (the
# pull-request head), ROW_RUN, ROW_SELECTION_RUN, ROW_OUTCOMES_RUN, ROW_SHARDS,
# ROW_EPOCH, ROW_SELECTION_DIR and ROW_OUTCOMES_DIR (gh run download trees:
# corvint-test-shadow-*/selection.json and ci-outcomes-*/outcomes.json),
# ROW_NARROWING_DIR (the selection run's ci-narrowing-*/narrowing.json, AFP-V0-099),
# ROW_REUSE (a ci-reuse decision.json, or empty) and ROW_COLLECT (newline-
# separated reasons the collector already found) and ROW_INFRA (newline-separated
# shards whose job failed at "Set up job", AFP-V0-052) and ROW_COSTS (package-costs.json,
# AFP-V0-481, or empty). Each file is read on its
# own, so one unreadable artifact marks itself and never hides the others. The
# row is always written: when classification fails or the row exceeds 64 KiB,
# a minimal INCOMPLETE row replaces it. Prints the class. The intermediate
# OUT.bundle and OUT.err stay beside OUT in the runner's temporary directory.
set -u
out=$1
here=$(cd "$(dirname "$0")" && pwd)
bundle="$out.bundle"
: > "$bundle"
add() { # KIND NAME FILE
  if [ -f "$3" ] && jq -c --arg kind "$1" --arg name "$2" '{kind: $kind, name: $name, doc: .}' "$3" > "$bundle.one" 2> /dev/null &&
    [ "$(wc -l < "$bundle.one")" -eq 1 ]; then
    cat "$bundle.one" >> "$bundle"
  else
    jq -n -c --arg kind "$1" --arg name "$2" '{kind: $kind, name: $name, unreadable: true}' >> "$bundle"
  fi
}
for d in "${ROW_SELECTION_DIR:-/nonexistent}"/corvint-test-shadow-*; do
  [ -d "$d" ] && add selection "${d##*/}" "$d/selection.json"
done
for d in "${ROW_OUTCOMES_DIR:-/nonexistent}"/ci-outcomes-*; do
  [ -d "$d" ] && add outcomes "${d##*/}" "$d/outcomes.json"
done
for d in "${ROW_NARROWING_DIR:-/nonexistent}"/ci-narrowing-*; do
  [ -d "$d" ] && add narrowing "${d##*/}" "$d/narrowing.json"
done
[ -z "${ROW_REUSE:-}" ] || add reuse ci-reuse "$ROW_REUSE"
# AFP-V0-481: ROW_COSTS names package-costs.json for the additive shadowCost field. A file that
# is missing or not one JSON document is left out, so the row only names the cost as unpriced.
set --
if [ -n "${ROW_COSTS:-}" ] && [ -f "$ROW_COSTS" ] && jq -e 'type == "object"' "$ROW_COSTS" > /dev/null 2>&1; then
  set -- --slurpfile costs "$ROW_COSTS"
fi
if ! jq -n -c "$@" --arg kind "${ROW_KIND:-}" --arg pr "${ROW_PR:-}" --arg sha "${ROW_SHA:-}" \
  --arg head "${ROW_HEAD:-}" --arg runId "${ROW_RUN:-}" --arg selectionRunId "${ROW_SELECTION_RUN:-}" \
  --arg outcomesRunId "${ROW_OUTCOMES_RUN:-}" --arg shards "${ROW_SHARDS:-}" --arg epoch "${ROW_EPOCH:-}" \
  --arg collect "${ROW_COLLECT:-}" --arg infra "${ROW_INFRA:-}" -f "$here/row.jq" < "$bundle" > "$out.tmp" 2> "$out.err" ||
  [ "$(wc -l < "$out.tmp")" -ne 1 ] || [ "$(wc -c < "$out.tmp")" -gt 65536 ]; then
  reason="row classification failed: $(head -c 200 "$out.err" 2> /dev/null | tr -d '\n')"
  jq -n -c --arg kind "${ROW_KIND:-}" --arg pr "${ROW_PR:-}" --arg sha "${ROW_SHA:-}" --arg reason "$reason" \
    '{profile: "corvint-pr-forward-row/0", kind: $kind, class: "INCOMPLETE",
      pr: (if $pr | test("^[0-9]+$") then $pr | tonumber else null end),
      sha: (if $sha == "" then null else $sha end), incomplete: [$reason], truncated: false}' > "$out.tmp" ||
    printf '{"profile":"corvint-pr-forward-row/0","class":"INCOMPLETE","incomplete":["row classification failed"],"truncated":false}\n' > "$out.tmp"
fi
mv -f "$out.tmp" "$out"
jq -r '.class' "$out" 2> /dev/null || echo INCOMPLETE
