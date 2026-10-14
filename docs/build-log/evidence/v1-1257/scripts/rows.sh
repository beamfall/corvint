#!/bin/sh
# usage: rows.sh ROWSDIR MERGED_TSV  -> forward-rows TSV (one line per retained ci-forward-* row.json)
rows=$1; merged=$2
printf 'artifact\tpr\tkind\tsha\thead\tepoch\tclass\trun_id\tselection_run_id\toutcomes_run_id\toutcomes_source\tscope\tselected\tlimits\tfailed\tmissed\tflaky\tincomplete\ttruncated\tcomplete\tpr_merged_at\tpr_merge_sha\tmerged_through_queue\n'
for d in "$rows"/ci-forward-*; do
  a=$(basename "$d")
  jq -r --arg a "$a" --rawfile m "$merged" '
    ($m | split("\n") | map(select(length>0) | split("\t")) | map({key: .[0], value: {at: .[1], sha: .[2]}}) | from_entries) as $mm
    | ($mm[(.pr|tostring)] // {at: "OPEN_AT_SNAPSHOT", sha: ""}) as $p
    | [$a, .pr, .kind, .sha, .head, .epoch, .class, .runId, .selectionRunId, .outcomesRunId, .outcomesSource,
       .selection.scope, .selection.selected, (.selection.limits | join("; ")),
       .counts.failed, .counts.missed, .counts.flaky, (.incomplete | length), .truncated,
       (if (.incomplete | length) == 0 and (.truncated | not) and .class != "INCOMPLETE" then "yes" else "no" end),
       $p.at, $p.sha,
       (if .kind == "queue" and .sha == $p.sha then "yes" else "no" end)] | @tsv' "$d/row.json"
done
