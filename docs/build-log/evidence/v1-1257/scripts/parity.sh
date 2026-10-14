#!/bin/sh
# Usage: parity.sh RUNS_DIR > parity.tsv ; one row per (run, shard).
set -eu
here=$(cd "$(dirname "$0")" && pwd)
printf 'run_id\tpr\tshard\thead\tshadow_target\tdriver_target\tshadow_tool\tscope\tshadow_exit\tshadow_selected\tdriver_executed\tselected_not_executed\texecuted_not_selected\targv_prefix\tshadow_incomplete\tshadow_slices\tdriver_status\tdriver_failed\tfailed_not_selected\tclass\tselected_not_executed_list\n'
for r in "$1"/*/; do
  run=$(basename "$r")
  for k in 0 1 2 3 4 5 6 7; do
    jq -n -r --arg run "$run" --slurpfile sel "$r/corvint-test-shadow-$k/selection.json" \
      --slurpfile dry "$r/corvint-test-shadow-$k/dry-run.json" \
      --slurpfile out "$r/ci-outcomes-pull_request-$k/outcomes.json" -f "$here/parity.jq"
  done
done
