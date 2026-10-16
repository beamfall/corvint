#!/bin/sh
# Usage: parity.sh RUNS_DIR > parity.tsv ; one row per (run, shard).
# As docs/build-log/evidence/v1-1257/scripts/parity.sh, except that a shard whose shadow or
# driver document is absent is one NOT_OBSERVED row instead of a stop.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
printf 'run_id\tpr\tshard\thead\tshadow_target\tdriver_target\tshadow_tool\tscope\tshadow_exit\tshadow_selected\tdriver_executed\tselected_not_executed\texecuted_not_selected\targv_prefix\tshadow_incomplete\tshadow_slices\tdriver_status\tdriver_failed\tfailed_not_selected\tclass\tselected_not_executed_list\n'
for r in "$1"/*/; do
  run=$(basename "$r")
  for k in 0 1 2 3 4 5 6 7; do
    sel="$r/corvint-test-shadow-$k/selection.json" dry="$r/corvint-test-shadow-$k/dry-run.json" out="$r/ci-outcomes-pull_request-$k/outcomes.json"
    if [ ! -f "$sel" ] || [ ! -f "$dry" ] || [ ! -f "$out" ]; then
      printf '%s\t-\t%s\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\t-\tNOT_OBSERVED\t-\n' "$run" "$k"
      continue
    fi
    jq -n -r --arg run "$run" --slurpfile sel "$sel" --slurpfile dry "$dry" --slurpfile out "$out" -f "$here/parity.jq"
  done
done
