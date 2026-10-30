#!/usr/bin/env bash
# AFP-V0-670 (V1-2987): exact-tree reuse decision for a gate job set of one workflow.
#
# A merge-queue entry reuses a successful pull-request run of its second parent when that run's
# record job ($RECORD_JOB, which runs no repository code) printed exactly one line naming this
# tree, this first parent and this second parent, and every gate job passed there. A main push
# reuses a successful merge-queue run of this exact commit in which every gate job passed. Either
# way the tested commit must be a two-parent merge whose .github tree equals its first parent's.
# Any mismatch, refusal, or unreadable listing or log prints a FULL note and leaves `mode` unset,
# so the gate runs in full. Only GitHub listings and the record job's log are read; artifacts are
# never trusted. It writes only $GITHUB_OUTPUT, $GITHUB_STEP_SUMMARY and $RUNNER_TEMP.
#
# Inputs: GATE (record label), WORKFLOW (workflow file name), GATE_JOBS (JSON array of exact job
# names), RECORD_JOB (the record job's name), GH_TOKEN and the GITHUB_* runner variables.
set -eu
full() {
  echo "Gate reuse (AFP-V0-670) $GATE: $1; the gate runs in full." >> "$GITHUB_STEP_SUMMARY"
  exit 0
}
case "$GATE" in '' | *[!a-z0-9-]*) echo 'GATE must be a lowercase label' >&2; exit 2 ;; esac
test "$(git rev-parse HEAD)" = "$GITHUB_SHA"
tree=$(git rev-parse --verify -q "$GITHUB_SHA^{tree}")
if ! base=$(git rev-parse --verify -q "$GITHUB_SHA^1") || ! head=$(git rev-parse --verify -q "$GITHUB_SHA^2") ||
  git rev-parse --verify -q "$GITHUB_SHA^3" > /dev/null; then
  full 'the tested commit is not a two-parent merge'
fi
if ! git diff --quiet "$base" "$GITHUB_SHA" -- .github; then
  full 'the tested commit changes .github'
fi
api="$GITHUB_API_URL/repos/$GITHUB_REPOSITORY/actions"
get() {
  curl -fsSL --max-time 30 --max-filesize 4194304 -H 'Accept: application/vnd.github+json' \
    -H "Authorization: Bearer $GH_TOKEN" -o "$2" "$1" ||
    curl -fsSL --max-time 30 --max-filesize 4194304 -H 'Accept: application/vnd.github+json' -o "$2" "$1"
}
d="$RUNNER_TEMP/gate-reuse-$GATE"
mkdir -p "$d"
n=$(jq -r 'if type == "array" and length > 0 and all(type == "string") and (unique | length) == length then length else empty end' <<< "$GATE_JOBS")
test -n "$n"
case "$GITHUB_EVENT_NAME" in
merge_group)
  re="^([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z )?corvint-ci-gate-tree/0 gate=$GATE tree=[0-9a-f]{40} base=[0-9a-f]{40} head=[0-9a-f]{40} target=[0-9a-f]{40}\$"
  get "$api/workflows/$WORKFLOW/runs?head_sha=$head&event=pull_request&status=success&per_page=20" "$d/runs.json" ||
    full 'the pull-request run listing is unreadable'
  ids=$(jq -r --arg sha "$head" --arg repo "$GITHUB_REPOSITORY" --arg path ".github/workflows/$WORKFLOW" '.workflow_runs[]?
    | select(.head_sha == $sha and .event == "pull_request" and .conclusion == "success"
      and .path == $path and .repository.full_name == $repo and (.id | type == "number"))
    | .id' "$d/runs.json" | head -n 5)
  for id in $ids; do
    get "$api/runs/$id/jobs?filter=latest&per_page=100" "$d/jobs-$id.json" || continue
    # Exactly one latest job of each gate name and of the record job, every one passed on the
    # pull-request head in this run; the record job's id is kept.
    job=$(jq -r --arg sha "$head" --argjson id "$id" --argjson want "$GATE_JOBS" --arg rec "$RECORD_JOB" '($want + [$rec]) as $names
      | [.jobs[]? | select(.name as $n | any($names[]; . == $n))]
      | if length == ($names | length) and (map(.name) | unique | length) == ($names | length)
          and all(.conclusion == "success" and .head_sha == $sha and .run_id == $id)
        then (.[] | select(.name == $rec) | .id | select(type == "number")) else empty end' "$d/jobs-$id.json") || continue
    test -n "$job" || continue
    get "$api/jobs/$job/logs" "$d/log-$job.txt" || continue
    # Exactly one record line in that job's own log, naming this tree, base and head.
    c=$(tr -d '\r' < "$d/log-$job.txt" | grep -cE "$re" || true)
    test "$c" = 1 || continue
    line=$(tr -d '\r' < "$d/log-$job.txt" | grep -E "$re")
    test "$(printf '%s\n' "$line" | sed -E 's/.* tree=([0-9a-f]{40}) .*/\1/')" = "$tree" || continue
    test "$(printf '%s\n' "$line" | sed -E 's/.* base=([0-9a-f]{40}) .*/\1/')" = "$base" || continue
    test "$(printf '%s\n' "$line" | sed -E 's/.* head=([0-9a-f]{40}) .*/\1/')" = "$head" || continue
    printf '{"profile":"corvint-ci-gate-reuse/0","gate":"%s","mode":"REUSE","event":"merge_group","tree":"%s","base":"%s","head":"%s","target":"%s","runId":%s,"recordJobId":%s}\n' \
      "$GATE" "$tree" "$base" "$head" "$GITHUB_SHA" "$id" "$job" > "$d/decision.json"
    cat "$d/decision.json" >> "$GITHUB_STEP_SUMMARY"
    echo "Gate reuse (AFP-V0-670) $GATE: tree $tree on base $base passed in pull-request run $id (record job $job); not rerun." >> "$GITHUB_STEP_SUMMARY"
    echo 'mode=REUSE' >> "$GITHUB_OUTPUT"
    exit 0
  done
  full "no successful pull-request run of $head recorded tree $tree on base $base"
  ;;
push)
  test "$GITHUB_REF" = refs/heads/main || full 'the push is not to main'
  get "$api/workflows/$WORKFLOW/runs?head_sha=$GITHUB_SHA&event=merge_group&status=success&per_page=20" "$d/runs.json" ||
    full 'the merge-queue run listing is unreadable'
  ids=$(jq -r --arg sha "$GITHUB_SHA" --arg repo "$GITHUB_REPOSITORY" --arg path ".github/workflows/$WORKFLOW" '.workflow_runs[]?
    | select(.head_sha == $sha and .event == "merge_group" and .conclusion == "success"
      and .path == $path and .head_repository.full_name == $repo
      and (.head_branch | type == "string" and startswith("gh-readonly-queue/main/")) and (.id | type == "number"))
    | .id' "$d/runs.json" | head -n 5)
  for id in $ids; do
    get "$api/runs/$id/jobs?filter=latest&per_page=100" "$d/jobs-$id.json" || continue
    # Exactly one latest job of each gate name, every one passed on this commit in this run.
    if jq -e --arg sha "$GITHUB_SHA" --argjson id "$id" --argjson want "$GATE_JOBS" '[.jobs[]? | select(.name as $n | any($want[]; . == $n))]
      | length == ($want | length) and (map(.name) | unique | length) == ($want | length)
        and all(.conclusion == "success" and .head_sha == $sha and .run_id == $id)' "$d/jobs-$id.json" > /dev/null; then
      printf '{"profile":"corvint-ci-gate-reuse/0","gate":"%s","mode":"REUSE","event":"push","tree":"%s","base":"%s","head":"%s","target":"%s","runId":%s}\n' \
        "$GATE" "$tree" "$base" "$head" "$GITHUB_SHA" "$id" > "$d/decision.json"
      cat "$d/decision.json" >> "$GITHUB_STEP_SUMMARY"
      echo "Gate reuse (AFP-V0-670) $GATE: passed on $GITHUB_SHA in merge-queue run $id; not rerun." >> "$GITHUB_STEP_SUMMARY"
      echo 'mode=REUSE' >> "$GITHUB_OUTPUT"
      exit 0
    fi
  done
  full "no successful merge-queue run passed every gate job on $GITHUB_SHA"
  ;;
*)
  full "event $GITHUB_EVENT_NAME never reuses"
  ;;
esac
