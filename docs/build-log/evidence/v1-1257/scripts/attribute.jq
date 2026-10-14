# input: runs.jsonl slurped; $prs: array of {n,mergedAt,merge,head,branch,created}
# output: one line per (pr, run) attribution
. as $runs
| $prs[] as $p
| $runs[]
| select(.created_at <= $snap)
| select(
    ((.head_branch == $p.branch) and (.event == "pull_request" or .event == "workflow_run")
       and .created_at >= $p.created and .created_at <= $p.mergedAt)
    or ((.head_branch | startswith("gh-readonly-queue/main/pr-\($p.n)-")) and .created_at <= $p.mergedAt)
    or ((.head_branch == "main") and (.head_sha == $p.merge) and (.event == "push" or .event == "workflow_run"))
  )
| [$p.n, .id, .path, .event, .head_branch, .head_sha, .created_at, .conclusion, .run_attempt] | @tsv
