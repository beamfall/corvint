#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# AFP-V0-047: collect one completed CI run's forward-row inputs and classify them.
# The ci-control-plane forward-rows job runs this default-branch copy with gh and
# jq only; it downloads small artifacts and never runs code of the tested commit.
# Environment: GH_TOKEN, REPO, RUN_ID, EVENT (merge_group|pull_request), HEAD_SHA,
# HEAD_BRANCH, PR_NUMBER and PR_BASE_SHA (may be empty), FORWARD_SHARDS, FORWARD_EPOCH,
# RUNNER_TEMP, GITHUB_OUTPUT and GITHUB_STEP_SUMMARY. Writes
# $RUNNER_TEMP/forward-row/row.json and the step output `name`; a run that is
# not a forward row (not a queue branch of main, no pull request, or a run that
# executed neither a full shard nor the shadow) writes neither and says why.
# A change to any `.github/` path ran its own workflow and forward-row scripts,
# so it could report any outcome it liked: such a run, or one whose change set
# GitHub cannot list in full (300 files or more), is never a row either.
# AFP-V0-099: the selection run's ci-narrowing-* records go to the classifier,
# which names a narrowed pull-request run's row NARROWED and makes a queue
# failure of a package that run narrowed out a MISS. AFP-V0-733: a queue run that
# narrowed itself, or reused a narrowed pull-request run, holds no full outcomes and is
# no row.
# AFP-V0-481/483: the row prices its selectors with ../cishards/package-costs.json of this same
# trusted checkout, and a complete row also writes the step output `cost`, its cost artifact name.
# CIN-V0-001 (Codex P2): the row's epoch is the forwardEpoch its producing run stamped into every
# retained selection document, never FORWARD_EPOCH: a run that predates an epoch advance and
# completes, or is rerun, after it keeps its own epoch. A missing, mixed or other epoch writes no
# row, so such a run is never counted toward FORWARD_EPOCH, not even as INCOMPLETE.
set -u
here=$(cd "$(dirname "$0")" && pwd)
dir="$RUNNER_TEMP/forward-row"
mkdir -p "$dir/selection" "$dir/outcomes" "$dir/reuse" "$dir/narrowing"
collect=""
note() {
  collect="$collect$1
"
}
say() { printf '%s\n' "$1" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"; }
names() { gh api "repos/$REPO/actions/runs/$1/artifacts?per_page=100" --jq '.artifacts[] | select(.expired | not) | .name'; }
fetch() { gh run download "$1" --repo "$REPO" --pattern "$2" --dir "$3" > /dev/null 2>&1; }
hex40() { printf '%s' "$1" | grep -Eqx '[0-9a-f]{40}'; }
digits() { printf '%s' "$1" | grep -Eqx '[0-9]+'; }
# own BASE HEAD: prints clean, github (the change touches .github/) or unprovable.
own() {
  gh api "repos/$REPO/compare/$1...$2" --jq 'if (.files | length) >= 300 then "unprovable"
    elif any(.files[]; (.filename | startswith(".github/")) or ((.previous_filename // "") | startswith(".github/")))
    then "github" else "clean" end' 2> /dev/null
}

if ! digits "$RUN_ID" || ! hex40 "$HEAD_SHA"; then
  say "forward row: run $RUN_ID at $HEAD_SHA is not a well-formed run; no row"
  exit 0
fi
target="" sha="" head="" pr="" selection_run="" outcomes_run="$RUN_ID" reuse=""
case "$EVENT" in
merge_group)
  case "$HEAD_BRANCH" in
  gh-readonly-queue/main/pr-*) ;;
  *)
    say "forward row: $HEAD_BRANCH is not a merge-queue branch of main; no row"
    exit 0
    ;;
  esac
  pr=${HEAD_BRANCH#gh-readonly-queue/main/pr-}
  pr=${pr%%-*}
  kind=queue sha=$HEAD_SHA
  parents=$(gh api "repos/$REPO/commits/$sha" --jq '.parents | if length == 2 then "\(.[0].sha) \(.[1].sha)" else empty end' 2> /dev/null) || parents=""
  base=${parents% *} head=${parents#* }
  if ! hex40 "$base" || ! hex40 "$head"; then
    note "the parents of queue commit $sha are unreadable or not two"
    base="" head=""
  fi
  if run_names=$(names "$RUN_ID"); then
    if printf '%s\n' "$run_names" | grep -qx ci-reuse; then
      if fetch "$RUN_ID" ci-reuse "$dir/reuse" && [ -f "$dir/reuse/ci-reuse/decision.json" ]; then
        reuse="$dir/reuse/ci-reuse/decision.json"
        # AFP-V0-733: a queue run that reused a narrowed pull-request run (AFP-V0-734) has no
        # full outcomes anywhere; the main push of its commit runs the full suite instead.
        if jq -e '.mode == "REUSE" and .scope == "NARROWED"' "$reuse" > /dev/null 2>&1; then
          say "forward row: queue run $RUN_ID reused a narrowed pull-request run (AFP-V0-734), so no run holds its full outcomes; no row"
          exit 0
        fi
        if jq -e '.mode == "REUSE"' "$reuse" > /dev/null 2>&1; then
          outcomes_run=$(jq -r '.runId' "$reuse")
          digits "$outcomes_run" || {
            note "the REUSE decision names no reused run"
            outcomes_run=""
          }
        fi
      else
        note "the ci-reuse decision of run $RUN_ID could not be downloaded"
      fi
    fi
    # AFP-V0-733: a queue run whose own shards narrowed (AFP-V0-731) ran no full suite either.
    # Records that cannot be downloaded leave the row INCOMPLETE, never CLEAN.
    if printf '%s\n' "$run_names" | grep -q '^ci-narrowing-'; then
      if fetch "$RUN_ID" 'ci-narrowing-*' "$dir/queue-narrowing"; then
        if find "$dir/queue-narrowing" -name narrowing.json -type f -exec cat {} + 2> /dev/null |
          jq -e -s 'any(.[]; .mode == "NARROWED")' > /dev/null 2>&1; then
          say "forward row: queue run $RUN_ID narrowed its own shards (AFP-V0-731), so it holds no full outcomes; no row"
          exit 0
        fi
      else
        note "the ci-narrowing records of queue run $RUN_ID could not be downloaded"
      fi
    fi
  else
    note "the artifacts of run $RUN_ID are unreadable"
  fi
  if [ -n "$head" ]; then
    selection_run=$(gh api "repos/$REPO/actions/workflows/ci.yml/runs?event=pull_request&head_sha=$head&status=success&per_page=20" \
      --jq '[.workflow_runs[]] | max_by(.run_started_at) | .id // empty' 2> /dev/null) || selection_run=""
    digits "$selection_run" || {
      note "no successful pull_request run of head $head is readable, so no selection"
      selection_run=""
    }
  fi
  ;;
pull_request)
  pr=$PR_NUMBER
  digits "$pr" || {
    say "forward row: run $RUN_ID names no pull request of this repository; no row"
    exit 0
  }
  kind=pr base=${PR_BASE_SHA:-} head=$HEAD_SHA selection_run=$RUN_ID
  hex40 "$base" || {
    note "pull_request run $RUN_ID names no base commit"
    base=""
  }
  if ! run_names=$(names "$RUN_ID"); then
    note "the artifacts of run $RUN_ID are unreadable"
  elif ! printf '%s\n' "$run_names" | grep -Eq '^(ci-outcomes-pull_request-|corvint-test-shadow-)'; then
    say "forward row: pull_request run $RUN_ID executed no full shard and no shadow plan (DOCS, batched or skipped); no row"
    exit 0
  fi
  ;;
*)
  say "forward row: a $EVENT run is not a forward row"
  exit 0
  ;;
esac
digits "$pr" || {
  say "forward row: no pull request number in $HEAD_BRANCH; no row"
  exit 0
}
if [ -n "$outcomes_run" ]; then
  fetch "$outcomes_run" 'ci-outcomes-*' "$dir/outcomes" || note "run $outcomes_run retained no ci-outcomes artifact"
fi
if [ -n "$selection_run" ]; then
  fetch "$selection_run" 'corvint-test-shadow-*' "$dir/selection" || note "run $selection_run retained no corvint-test-shadow artifact"
  # A run that narrowed nothing has no record; one whose listing is unreadable cannot show that.
  if selection_names=$(names "$selection_run"); then
    if printf '%s\n' "$selection_names" | grep -q '^ci-narrowing-'; then
      fetch "$selection_run" 'ci-narrowing-*' "$dir/narrowing" || note "the ci-narrowing records of run $selection_run could not be downloaded"
    fi
  else
    note "the artifacts of selection run $selection_run are unreadable, so its narrowing is unknown"
  fi
fi
# AFP-V0-802: the retained shadows identify the immutable tested merge, not the
# mutable PR merge ref. Treat their target only as a hint until GitHub proves its
# parents, event ancestry and that parent 0 belongs to main. Never execute it.
if [ "$kind" = pr ]; then
  if ! hex40 "$base"; then
    say "forward row: pr-base-invalid-oid; no row"
    exit 0
  fi
  target=$(find "$dir/selection" -name selection.json -type f -exec cat {} + | jq -er -s \
    --arg head "$head" --arg run "$RUN_ID" '
      if length > 0 and all(.[]; .head == $head and (.runId | tostring) == $run)
      then map(.target) | unique | if length == 1 then .[0] else empty end
      else empty end' 2> /dev/null) || target=""
  if ! hex40 "$target"; then
    say "forward row: pr-base-target-unreadable; no row"
    exit 0
  fi
  parents=$(gh api "repos/$REPO/commits/$target" --jq '.parents | if length == 2 then "\(.[0].sha) \(.[1].sha)" else empty end' 2> /dev/null) || parents=""
  effective=${parents% *} second=${parents#* }
  if ! hex40 "$effective" || [ "$second" != "$head" ]; then
    say "forward row: pr-base-parents-mismatch; no row"
    exit 0
  fi
  ancestor=$(gh api "repos/$REPO/compare/$base...$effective" --jq '.status' 2> /dev/null) || ancestor=""
  case "$ancestor" in ahead | identical) ;; *) say "forward row: pr-base-not-ancestor; no row"; exit 0 ;; esac
  upstream=$(gh api "repos/$REPO/compare/$effective...main" --jq '.status' 2> /dev/null) || upstream=""
  case "$upstream" in ahead | identical) ;; *) say "forward row: pr-base-not-main; no row"; exit 0 ;; esac
  base=$effective
  # Inspect both the tested merge and the PR head: a fabricated merge artifact
  # must not hide a PR-owned .github/ edit by supplying another merge tree.
  case $(own "$base" "$head") in
  clean) ;;
  *) say "forward row: the pull request touches .github/ or is too large to prove it does not, or its change set is unreadable; no row"; exit 0 ;;
  esac
fi
if [ -n "$base" ]; then
  case $(own "$base" "${target:-${sha:-$head}}") in
  clean) ;;
  github | unprovable)
    say "forward row: the change $base...${target:-${sha:-$head}} touches .github/ or is too large to prove it does not; no row"
    exit 0
    ;;
  *)
    if [ "$kind" = pr ]; then
      say "forward row: the change set $base...$target is unreadable; no row"
      exit 0
    fi
    note "the change set $base...${sha:-$head} is unreadable"
    ;;
  esac
fi
# AFP-V0-052: a go-product-shard job whose first step, "Set up job", failed never started a
# step of its own: the hosted runner failed before the job ran. Its shard numbers go to the
# classifier, which reads them as infrastructure only when they are exactly the missing shards.
# An unreadable job list names none, so the row stays INCOMPLETE.
epoch=$(find "$dir/selection" -name selection.json -type f -exec cat {} + 2> /dev/null | jq -er -s '
  if length > 0 then (map(.forwardEpoch) | unique) as $e
    | if ($e | length) == 1 and ($e[0] | type) == "number" and $e[0] >= 0 and $e[0] == ($e[0] | floor) then $e[0] else empty end
  else empty end' 2> /dev/null) || epoch=""
if [ -z "$epoch" ] || [ "$epoch" != "$FORWARD_EPOCH" ]; then
  say "forward row: the selection run ${selection_run:-none} stamped no single forward epoch, or epoch ${epoch:-none} rather than $FORWARD_EPOCH (CIN-V0-001); no row"
  exit 0
fi
infra=""
if [ -n "$outcomes_run" ]; then
  infra=$(gh api "repos/$REPO/actions/runs/$outcomes_run/jobs?filter=latest&per_page=100" --jq '.jobs[]
    | select((.name | test("^go-product-shard \\([0-9]+\\)$")) and .conclusion == "failure"
             and .steps[0].name == "Set up job" and .steps[0].conclusion == "failure")
    | .name | capture("\\((?<n>[0-9]+)\\)$").n' 2> /dev/null) || {
    say "forward row: the jobs of run $outcomes_run are unreadable, so no hosted set-up failure is named"
    infra=""
  }
fi
class=$(ROW_KIND=$kind ROW_PR=$pr ROW_SHA=$sha ROW_HEAD=$head ROW_RUN=$RUN_ID ROW_SELECTION_RUN=$selection_run \
  ROW_OUTCOMES_RUN=$outcomes_run ROW_SHARDS=$FORWARD_SHARDS ROW_EPOCH=$epoch \
  ROW_SELECTION_DIR="$dir/selection" ROW_OUTCOMES_DIR="$dir/outcomes" ROW_NARROWING_DIR="$dir/narrowing" ROW_REUSE=$reuse ROW_COLLECT=$collect ROW_INFRA=$infra \
  ROW_COSTS="$here/../cishards/package-costs.json" sh "$here/classify.sh" "$dir/row.json")
case "$class" in CLEAN | FLAKY | MISS | INCOMPLETE | INFRA | NARROWED) ;; *) class=INCOMPLETE ;; esac
row_sha=$(jq -r '.sha // empty' "$dir/row.json" 2> /dev/null) || row_sha=""
hex40 "$row_sha" || row_sha=$HEAD_SHA
failing=$(jq -r 'if (.counts.failed // 0) > 0 then 1 else 0 end' "$dir/row.json" 2> /dev/null) || failing=0
[ "$failing" = 1 ] || failing=0
name="ci-forward-$class-pr$pr-$row_sha-e$epoch-f$failing"
echo "name=$name" >> "$GITHUB_OUTPUT"
# AFP-V0-483: a complete row also names its shadow cost, so the bar can aggregate it from artifact
# names alone: each selector's share of Full in permille (or na) and whether it missed (1, 0 or n).
# It repeats the row name's class and failing flag, and is uploaded after the row, so bar.jq binds
# it only to that row.
case "$class" in
CLEAN | FLAKY | MISS)
  cost=$(jq -r 'def slot: if . == null or .permille == null then "na" else ("0000" + (.permille | tostring))[-4:] end;
      def miss: if . == null then "n" elif .missed > 0 then "1" else "0" end;
      .shadowCost as $c
      | "s\($c.selection | slot)-b\($c.changed | slot)-c\($c.hop1 | slot)-d\($c.hop2 | slot)-m\([$c.selection, $c.changed, $c.hop1, $c.hop2] | map(miss) | join(""))"' \
    "$dir/row.json" 2> /dev/null) || cost=""
  if printf '%s' "$cost" | grep -Eqx 's([0-9]{4}|na)-b([0-9]{4}|na)-c([0-9]{4}|na)-d([0-9]{4}|na)-m[01n]{4}'; then
    echo "cost=ci-forward-cost-pr$pr-$row_sha-e$epoch-$class-f$failing-$cost" >> "$GITHUB_OUTPUT"
  fi
  ;;
esac
say "forward row \`$name\` ($kind run $RUN_ID; selection from run ${selection_run:-none}; outcomes from run ${outcomes_run:-none})"
jq -r '"counts: \(.counts // {} | tojson); incomplete: \(.incomplete // [] | .[0:5] | join("; "))"' "$dir/row.json" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}" 2> /dev/null || true
