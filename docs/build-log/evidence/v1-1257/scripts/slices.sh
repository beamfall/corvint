#!/bin/sh
# Usage: slices.sh RUNS_DIR COSTS_DIR > slices.tsv
# For each run and shard holding a test slice in the shadow plan, compare the slice's
# -run/-skip name set with the top-level tests the driver ran for that package on that shard.
set -eu
pkg=github.com/Beamfall/corvint/internal/tasks/store
printf 'run_id\tshard\tshadow_slice\tshadow_flag\tpattern_names\tdriver_top_level_tests\tdriver_in_pattern\tdriver_outside_pattern\tclass\n'
for r in "$1"/*/; do
  run=$(basename "$r")
  for k in 0 1 2 3 4 5 6 7; do
    u=$(jq -c --arg p "go:$pkg" '[.invocations[] | .units[] | select(.unit == $p and .slice != "whole")] | first // empty' "$r/corvint-test-shadow-$k/dry-run.json")
    [ -n "$u" ] || continue
    stream="$2/$run/ci-shard-costs-$k/shard-$k.json"
    if [ ! -f "$stream" ]; then printf '%s\t%s\t-\t-\t-\t-\t-\t-\tNOT_OBSERVED\n' "$run" "$k"; continue; fi
    jq -R -n -r --argjson u "$u" --arg pkg "$pkg" --arg run "$run" --arg k "$k" '
      ($u.pattern | ltrimstr("^(") | rtrimstr(")$") | split("|")) as $P
      | [inputs | fromjson? | objects | select(.Package == $pkg and .Action == "run" and (.Test | type) == "string" and (.Test | contains("/") | not)) | .Test] | unique as $T
      | ($T - ($T - $P)) as $in | ($T - $P) as $outside
      | [$run, $k, $u.slice, $u.flag, ($P | length | tostring), ($T | length | tostring), ($in | length | tostring), ($outside | length | tostring),
         (if $u.flag == "-run" then (if ($outside | length) == 0 and ($in | length) > 0 then "match" else "DIFF" end)
          else (if ($in | length) == 0 and ($T | length) > 0 then "match" else "DIFF" end) end)] | @tsv' < "$stream"
  done
done
