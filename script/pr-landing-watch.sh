#!/usr/bin/env bash
# usage: script/pr-landing-watch.sh [--act] [--alerts FILE] [--state-dir DIR] [--flakes FILE]
#                                   [--repo OWNER/NAME] PR
#
# One pass over one pull request's landing (ASD-V0-833, V1-1735). Prints exactly one decision:
#   MERGED | CLOSED | WAITING
#   RERUN run=ID attempt=K/3   a failed head run whose failing Go tests all match the flake list
#   REQUEUE attempt=K/2        auto-merge is off after a bot removed the PR from the merge queue
#   ESCALATE reason=REASON     real-failure, rerun-cap, requeue-cap or human-dequeue
# It never loops or sleeps: run it once per CI or merge-queue event. --act performs the rerun
# (`gh run rerun ID --failed`) or the re-arm (`gh pr merge PR --auto`); without it nothing on
# GitHub changes, but the counts it would spend are still recorded. A human dequeue is never
# re-armed. Each ESCALATE is appended once per PR, reason and head commit to the alerts file
# (default STATE_DIR/alerts.log) for a session Monitor to tail.
#
# --flakes defaults to script/known-flakes.txt (one extended regular expression per line matched
# against the whole test name, # comments). --state-dir defaults to
# ${TMPDIR:-/tmp}/corvint-landing-watch. GH names the gh executable (default gh).
# Exit 0 on a non-escalating decision, 3 on ESCALATE, 2 on a usage error or a failed gh read.
#
# Example: script/pr-landing-watch.sh --act --repo beamfall/corvint 42
set -uo pipefail
export LC_ALL=C
usage() { sed -n '2,21p' "$0" | sed 's/^# \{0,1\}//'; }
die() { printf 'pr-landing-watch: %s\n' "$*" >&2; exit 2; }
here=$(cd "$(dirname "$0")" && pwd) || exit 2
gh=${GH:-gh}
act=0 alerts='' state=${TMPDIR:-/tmp}/corvint-landing-watch flakes=$here/known-flakes.txt repo='' pr=''
while (($#)); do
  case $1 in
    -h | --help) usage; exit 0 ;;
    --act) act=1; shift ;;
    --alerts | --state-dir | --flakes | --repo) [[ $# -ge 2 && -n $2 ]] || die "$1 needs a value"
      case $1 in --alerts) alerts=$2 ;; --state-dir) state=$2 ;; --flakes) flakes=$2 ;; --repo) repo=$2 ;; esac
      shift 2 ;;
    -*) usage >&2; die "unknown flag $1" ;;
    *) [[ -z $pr ]] || die "one PR at a time"; pr=$1; shift ;;
  esac
done
[[ $pr =~ ^[1-9][0-9]*$ ]] || { usage >&2; die "PR must be a pull request number"; }
[[ -r $flakes ]] || die "flake list $flakes is not readable"
mkdir -p "$state" || die "cannot create $state"
state=${state%/}
[[ -n $alerts ]] || alerts=$state/alerts.log
db=$state/pr-$pr
touch "$db" || die "cannot write $db"
repo_args=()
[[ -z $repo ]] || repo_args=(--repo "$repo")

get() { sed -n "s/^$1=//p" "$db" | tail -1; }
put() { { grep -v "^$1=" "$db"; printf '%s=%s\n' "$1" "$2"; } > "$db.tmp" && /bin/mv -f "$db.tmp" "$db"; }
escalate() { # escalate REASON DETAIL
  local key="alerted_$1_$head"
  printf 'ESCALATE reason=%s %s\n' "$1" "$2"
  if [[ -z $(get "$key") ]]; then
    printf '%s pr=%s reason=%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$pr" "$1" "$2" >> "$alerts"
    put "$key" 1
  fi
  exit 3
}

view=$("$gh" pr view "$pr" "${repo_args[@]}" --json state,autoMergeRequest,headRefOid \
  --jq '[.state, (.autoMergeRequest != null), .headRefOid] | @tsv') || die "gh pr view $pr failed"
IFS=$'\t' read -r pr_state armed head <<< "$view"
case $pr_state in
  MERGED) echo MERGED; exit 0 ;;
  CLOSED) echo CLOSED; exit 0 ;;
  OPEN) ;;
  *) die "unexpected pull request state '$pr_state'" ;;
esac
[[ $head =~ ^[0-9a-f]{40}$ ]] || die "no head commit for PR $pr"

# 1. A failed CI run of the head: rerun a listed flake, escalate anything else.
failed=$("$gh" run list "${repo_args[@]}" --commit "$head" --json databaseId,conclusion \
  --jq '.[] | select(.conclusion == "failure") | .databaseId') || die "gh run list failed"
run=$(printf '%s\n' "$failed" | grep -E '^[0-9]+$' | sort -n | head -1)
if [[ -n $run ]]; then
  log=$("$gh" run view "$run" "${repo_args[@]}" --log-failed) || die "gh run view $run failed"
  tests=$(printf '%s\n' "$log" | grep -oE -- '--- FAIL: [^ ]+' | cut -d' ' -f3 | sort -u)
  [[ -n $tests ]] || escalate real-failure "run=$run names no failing Go test"
  patterns=$(grep -vE '^[[:space:]]*(#|$)' "$flakes")
  unlisted=$(printf '%s\n' "$tests" | grep -vxE -f <(printf '%s\n' "${patterns:-^\$}") | tr '\n' ' ')
  [[ -z $unlisted ]] || escalate real-failure "run=$run tests=${unlisted% }"
  n=$(get "rerun_$head")
  n=${n:-0}
  ((n < 3)) || escalate rerun-cap "run=$run tests=$(printf '%s' "$tests" | tr '\n' ' ')"
  n=$((n + 1))
  put "rerun_$head" "$n"
  if ((act)); then "$gh" run rerun "$run" "${repo_args[@]}" --failed > /dev/null || die "gh run rerun $run failed"; fi
  printf 'RERUN run=%s attempt=%s/3\n' "$run" "$n"
  exit 0
fi

# 2. Auto-merge off: re-arm after a bot's merge-queue removal, escalate a human's.
if [[ $armed != true ]]; then
  nwo=$repo
  [[ -n $nwo ]] || nwo=$("$gh" repo view --json nameWithOwner --jq .nameWithOwner) || die "gh repo view failed"
  query='query($owner:String!,$name:String!,$pr:Int!){repository(owner:$owner,name:$name){pullRequest(number:$pr){
    timelineItems(last:1,itemTypes:[ADDED_TO_MERGE_QUEUE_EVENT,REMOVED_FROM_MERGE_QUEUE_EVENT]){nodes{__typename
    ... on RemovedFromMergeQueueEvent{createdAt actor{__typename login}}}}}}}'
  event=$("$gh" api graphql -f query="$query" -f owner="${nwo%%/*}" -f name="${nwo#*/}" -F pr="$pr" \
    --jq '.data.repository.pullRequest.timelineItems.nodes[] | [.__typename, (.actor.__typename // ""), (.actor.login // ""), (.createdAt // "")] | @tsv') ||
    die "gh api graphql failed"
  IFS=$'\t' read -r kind actor_type login at <<< "$event"
  if [[ $kind == RemovedFromMergeQueueEvent ]]; then
    [[ $actor_type == Bot || $login == *'[bot]' ]] || escalate human-dequeue "by=${login:-unknown} at=$at"
    n=$(get requeue)
    n=${n:-0}
    if [[ $(get requeue_event) != "$at" ]]; then
      ((n < 2)) || escalate requeue-cap "by=$login at=$at"
      n=$((n + 1))
      put requeue "$n"
      put requeue_event "$at"
    fi
    if ((act)); then "$gh" pr merge "$pr" "${repo_args[@]}" --auto > /dev/null || die "gh pr merge $pr --auto failed"; fi
    printf 'REQUEUE attempt=%s/2\n' "$n"
    exit 0
  fi
fi
echo WAITING
