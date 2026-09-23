#!/usr/bin/env bash
# usage: dogfood-change.sh BASE
# Local outcome recording needs DOGFOOD_OUTCOME plus one verification source:
# DOGFOOD_VERIFY holds one recorder command per line (blank lines skipped), or
# DOGFOOD_VERIFY_FILE names a file of the same shape and takes precedence.
# DOGFOOD_OCM_LINKS optionally names the author's explicit OCM link plan.
set -uo pipefail
export LC_ALL=C

# This script asserts on evidence output with ripgrep (rg); without it every rg call below fails
# as a bare "command not found" deep into the run, which reads as a real defect instead of a
# missing prerequisite. Fail closed here, before any build or Git work starts.
if ! command -v rg >/dev/null 2>&1; then
    printf 'dogfood-change: REFUSE unsupported-environment-missing-rg\n' >&2
    exit 1
fi

# Read the executable selection before Git, temporary files, or a selected executable can run.
corvint_bin_set=${CORVINT_BIN+x}
corvint_bin=${CORVINT_BIN-}

base_arg=${1:?usage: dogfood-change.sh BASE}
repo=$(cd "$(dirname "$0")/.." && pwd)
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
base=$(git -C "$repo" rev-parse "$base_arg^{commit}") || exit 2
target=$(git -C "$repo" rev-parse 'HEAD^{commit}') || exit 2
task=${DOGFOOD_TASK:-"Dogfood change from ${base:0:12} to HEAD"}
report="$repo/.corvint/dogfood-report.json"
intent_snapshot="$repo/.corvint/change.ocm-intents"
aggregate_status="$repo/.corvint/change.ocm-status.json"
evidence="$git_dir/corvint"
mkdir -p "$evidence" "$repo/.corvint"
# Each step publishes its stderr as "$evidence/<step>.stderr". A step that does
# not run this time leaves its previous envelope there, which reads as current
# evidence for a reason this run never emitted, so clear them before starting.
# The glob is non-recursive and cannot reach a concurrent run's $run_tmp.
rm -f "$evidence"/*.stderr
run_tmp=$(mktemp -d "$evidence/dogfood-change.XXXXXX") || exit 2
rows="$run_tmp/steps.tsv"
: > "$rows"
child_pid=
intent_publish_tmp=
citation_stage=".corvint/.cem-citations.${run_tmp##*/}.json"
citation_stage_owned=0
citation_count=0
local_outcome_evidence_sha256=
context_abstention_evidence_sha256=
anchor_state=NOT_OBSERVED
anchor_merge_base=
bootstrap_unknown=0
ocm_links_ready=0
ocm_link_plan_sha256=
ocm_link_plan_rows=

resolve_anchor() {
  local configured merges config_result
  configured=$(git -C "$repo" config --local --get-all corvint.dogfood.anchor)
  config_result=$?
  [[ $config_result -eq 1 ]] && return
  if [[ $config_result -ne 0 || -z $configured ]]; then
    printf 'dogfood-change: REFUSE anchor-ref-unavailable\n' >&2
    exit 2
  fi
  if [[ $configured == *$'\n'* ]]; then
    printf 'dogfood-change: REFUSE multiple-anchor-refs\n' >&2
    exit 2
  fi
  merges=$(git -C "$repo" merge-base --all "$configured" "$target") || {
    printf 'dogfood-change: REFUSE anchor-ref-unavailable\n' >&2
    exit 2
  }
  if [[ -z $merges || $merges == *$'\n'* ]]; then
    printf 'dogfood-change: REFUSE anchor-merge-base-ambiguous\n' >&2
    exit 2
  fi
  anchor_state=OBSERVED
  anchor_merge_base=$merges
  if [[ $base != "$anchor_merge_base" ]]; then
    printf 'dogfood-change: REFUSE base-not-anchored\n' >&2
    exit 2
  fi
}

cleanup_citation_stage() {
  if [[ $citation_stage_owned -eq 1 ]]; then
    rm -f "$repo/$citation_stage" || return 1
    citation_stage_owned=0
  fi
}

cleanup() {
  if [[ -n ${child_pid:-} ]]; then
    kill -TERM "$child_pid" 2>/dev/null || :
    wait "$child_pid" 2>/dev/null || :
    child_pid=
  fi
  if [[ -n ${intent_publish_tmp:-} ]]; then
    rm -f "$intent_publish_tmp"
    intent_publish_tmp=
  fi
  cleanup_citation_stage || :
  rm -rf "$run_tmp"
}

interrupted() {
  local code=$1
  trap - EXIT HUP INT TERM
  cleanup
  exit "$code"
}

trap cleanup EXIT
trap 'interrupted 129' HUP
trap 'interrupted 130' INT
trap 'interrupted 143' TERM

resolve_corvint_bin() {
  local expected_version version_output return_code
  expected_version=$(cat "$repo/VERSION")
  if [[ -z $expected_version ]]; then
    printf 'dogfood-change: REFUSE project-version-unavailable\n' >&2
    exit 2
  fi
  if [[ -z $corvint_bin_set ]]; then
    corvint_bin="$evidence/corvint"
    GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local \
      go build -o "$corvint_bin" ./cmd/corvint 2> "$run_tmp/corvint-build.stderr" &
    child_pid=$!
    wait "$child_pid"
    return_code=$?
    child_pid=
    if [[ $return_code -ne 0 ]]; then
      printf 'dogfood-change: REFUSE current-tree-corvint-build-failed\n' >&2
      exit 2
    fi
    return
  fi
  "$corvint_bin" --version > "$run_tmp/corvint-version.stdout" 2> "$run_tmp/corvint-version.stderr" &
  child_pid=$!
  wait "$child_pid"
  return_code=$?
  child_pid=
  version_output=$(cat "$run_tmp/corvint-version.stdout")
  if [[ $return_code -ne 0 || ! $version_output =~ ^"Corvint $expected_version (build "(0|[1-9][0-9]*)")"$ ]]; then
    printf 'dogfood-change: REFUSE corvint-version-mismatch expected=%s\n' "$expected_version" >&2
    exit 2
  fi
}

append_step() {
  printf '%s\t%s\t%s\n' "$1" "$2" "$3" >> "$rows"
}

failure_reason() {
  local error_file=$1 return_code=$2 reason
  reason=$(sed -n 's/.*"code"[[:space:]]*:[[:space:]]*"\([A-Za-z0-9_-]*\)".*/\1/p' "$error_file" | head -1)
  if [[ -z $reason && $return_code -eq 1 ]]; then
    reason=not-ready
  fi
  if [[ -z $reason ]]; then
    reason="exit-$return_code"
  fi
  printf '%s' "$reason"
}

run_corvint() {
  local step=$1 output=$2
  shift 2
  local error_file="$evidence/$step.stderr" return_code reason
  "$corvint_bin" --root "$repo" "$@" > "$output" 2> "$error_file" &
  child_pid=$!
  wait "$child_pid"
  return_code=$?
  child_pid=
  if [[ $return_code -eq 0 ]]; then
    append_step "$step" PRODUCED none
    return 0
  fi
  reason=$(failure_reason "$error_file" "$return_code")
  append_step "$step" NOT_PRODUCED "$reason"
  return "$return_code"
}

run_prechange_impact() {
  local output="$evidence/prechange-impact.json"
  local error_file="$evidence/prechange-impact.stderr"
  local argv_file="$evidence/prechange-impact.argv"
  local artifact="$evidence/prechange-impact-abstention.json"
  local return_code reason stdout_sha256 stderr_sha256 argv_sha256
  local -a argv=("$corvint_bin" --root "$repo" impact --base "$base" --range-profile expanded-256 --limit 20)

  if ! rm -f "$artifact" || ! (umask 077; printf '%s\0' "${argv[@]}" > "$argv_file"); then
    append_step prechange-impact NOT_PRODUCED context-abstention-evidence-failed
    return 2
  fi
  "${argv[@]}" > "$output" 2> "$error_file" &
  child_pid=$!
  wait "$child_pid"
  return_code=$?
  child_pid=
  if [[ $return_code -eq 0 ]]; then
    append_step prechange-impact PRODUCED none
    return 0
  fi
  reason=$(failure_reason "$error_file" "$return_code")
  if [[ $return_code -ne 2 || $reason != unsupported-impact-range || -s $output ]] ||
    [[ $(wc -l < "$error_file") -ne 1 ]] ||
    ! rg -q '^\{"code": "unsupported-impact-range", "error": "[A-Za-z0-9 ._/:()-]+", "ok": false\}$' "$error_file"; then
    if [[ $reason == unsupported-impact-range ]]; then
      reason=context-abstention-invalid
    fi
    append_step prechange-impact NOT_PRODUCED "$reason"
    return "$return_code"
  fi
  stdout_sha256=$(shasum -a 256 "$output" | awk '{print $1}')
  stderr_sha256=$(shasum -a 256 "$error_file" | awk '{print $1}')
  argv_sha256=$(shasum -a 256 "$argv_file" | awk '{print $1}')
  if [[ ! $stdout_sha256 =~ ^[0-9a-f]{64}$ || ! $stderr_sha256 =~ ^[0-9a-f]{64}$ ||
    ! $argv_sha256 =~ ^[0-9a-f]{64}$ ]]; then
    append_step prechange-impact NOT_PRODUCED context-abstention-evidence-failed
    return 2
  fi
  if ! (umask 077; cat > "$artifact" <<EOF
{"argvSha256":"sha256:$argv_sha256","base":"$base","exitStatus":"2","profile":"corvint-dogfood-context-abstention/0","reason":"unsupported-impact-range","status":"NOT_PRODUCED","stderrSha256":"sha256:$stderr_sha256","stdoutSha256":"sha256:$stdout_sha256","step":"prechange-impact","target":"$target"}
EOF
  ); then
    append_step prechange-impact NOT_PRODUCED context-abstention-evidence-failed
    return 2
  fi
  context_abstention_evidence_sha256=$(shasum -a 256 "$artifact" | awk '{print $1}')
  if [[ ! $context_abstention_evidence_sha256 =~ ^[0-9a-f]{64}$ ]]; then
    append_step prechange-impact NOT_PRODUCED context-abstention-evidence-failed
    return 2
  fi
  append_step prechange-impact NOT_PRODUCED "$reason"
  return 0
}

# run_cem_prepare derives the map, regenerating an existing one that prepare
# refused to resume: the previous change commits its sidecar, so on this
# repository every next change starts with an outdated map at the frozen path.
# Only that exact CEM-PILOT-018 refusal is retried; any other failure, such as a
# transient git-timeout, is reported so a committed cited map is never rewritten.
run_cem_prepare() {
  local output=$1 error_file="$evidence/cem-prepare.stderr" return_code
  local outdated='{"error": "cannot read CEM map: the existing map records a different base or patch; pass --replace to regenerate", "ok": false}'
  "$corvint_bin" --root "$repo" cem prepare --base "$base" --target "$target" > "$output" 2> "$error_file" &
  child_pid=$!
  wait "$child_pid"
  return_code=$?
  child_pid=
  if [[ $return_code -eq 0 ]]; then
    append_step cem-prepare PRODUCED none
    return 0
  fi
  if [[ $return_code -ne 2 || $(cat "$error_file") != "$outdated" ]]; then
    append_step cem-prepare NOT_PRODUCED "$(failure_reason "$error_file" "$return_code")"
    return "$return_code"
  fi
  run_corvint cem-prepare "$output" cem prepare --base "$base" --target "$target" --replace
}

render_report() {
  local complete
  complete=$(awk -F '\t' '$2 != "PRODUCED" && !($1 == "local-outcome" && $2 == "NOT_PRODUCED" && $3 == "no-source-paths") && !($1 == "prechange-impact" && $2 == "NOT_PRODUCED" && $3 == "unsupported-impact-range") { failed=1 } END { print failed ? "false" : "true" }' "$rows")
  {
    awk -F '\t' -v base="$base" -v target="$target" -v complete="$complete" '
    BEGIN {
      print "{"
      print "  \"profile\": \"corvint-dogfood-change/0\","
      print "  \"base\": \"" base "\","
      print "  \"target\": \"" target "\","
      print "  \"complete\": " complete ","
      print "  \"steps\": ["
    }
    {
      if (seen) print ","
      printf "    {\"name\": \"%s\", \"status\": \"%s\", \"reason\": \"%s\"}", $1, $2, $3
      seen=1
    }
    END {
      print ""
      print "  ],"
    }
    ' "$rows"
    if [[ -s $aggregate_status ]]; then
      printf '  "ocmStatus": '
      sed -n '1p' "$aggregate_status"
    else
      printf '  "ocmStatus": null\n'
    fi
    if [[ -n $local_outcome_evidence_sha256 ]]; then
      printf '  ,"localOutcomeEvidenceSha256": "sha256:%s"\n' "$local_outcome_evidence_sha256"
    else
      printf '  ,"localOutcomeEvidenceSha256": null\n'
    fi
    if [[ -n $context_abstention_evidence_sha256 ]]; then
      printf '  ,"contextAbstentionEvidenceSha256": "sha256:%s"\n' "$context_abstention_evidence_sha256"
    else
      printf '  ,"contextAbstentionEvidenceSha256": null\n'
    fi
    if [[ $anchor_state == OBSERVED ]]; then
      printf '  ,"anchor": {"state": "OBSERVED", "mergeBase": "%s"}\n' "$anchor_merge_base"
    else
      printf '  ,"anchor": {"state": "NOT_OBSERVED", "mergeBase": null}\n'
    fi
    if [[ -n $ocm_link_plan_sha256 ]]; then
      printf '  ,"ocmLinkPlan": {"sha256": "sha256:%s", "rows": %d}\n' "$ocm_link_plan_sha256" "$ocm_link_plan_rows"
    else
      printf '  ,"ocmLinkPlan": null\n'
    fi
    printf '  ,"dogfoodPolicy": {"bootstrapUnknown": %d, "maximumUnknownAfterBootstrap": 0}\n' "$bootstrap_unknown"
    printf '  ,"dogfoodCheck": null\n'
    printf '}\n'
  } > "$report"
  while IFS=$'\t' read -r step status reason; do
    "$corvint_bin" --root "$repo" dogfood-observe \
      --step "$step" --status "$status" --reason "$reason" >/dev/null 2>&1 &
    child_pid=$!
    wait "$child_pid" || :
    child_pid=
  done < "$rows"
}

run_dogfood_record() {
  local output=$1 error_file="$evidence/local-outcome.stderr" return_code reason state
  shift
  "$corvint_bin" --root "$repo" dogfood-record "$@" > "$output" 2> "$error_file" &
  child_pid=$!
  wait "$child_pid"
  return_code=$?
  child_pid=
  if [[ $return_code -ne 0 ]]; then
    reason=$(failure_reason "$error_file" "$return_code")
    local_outcome_row="NOT_PRODUCED\t$reason"
    return "$return_code"
  fi
  local_outcome_evidence_sha256=$(shasum -a 256 "$output" | awk '{print $1}')
  state=$(sed -n 's/.*"state":"\([a-z-]*\)".*/\1/p' "$output")
  case "$state" in
    recorded) local_outcome_row="PRODUCED\tnone" ;;
    no-source-paths) local_outcome_row="NOT_PRODUCED\tno-source-paths" ;;
    *) local_outcome_row="NOT_PRODUCED\tinvalid-record-admission-output"; return 2 ;;
  esac
}

# Fills verify_args with one --verify per non-blank command line; the recorder
# still applies its own 512-character and shell-syntax bounds to each line.
collect_verify_commands() {
  local line
  verify_args=()
  while IFS= read -r line || [[ -n $line ]]; do
    [[ $line == *[![:space:]]* ]] || continue
    verify_args+=(--verify "$line")
  done
}

validate_intent_manifest() {
  local source=${DOGFOOD_INTENTS_FILE:-} size last path previous='' count=0
  [[ -n $source && -f $source && ! -L $source ]] || return 1
  size=$(wc -c < "$source")
  [[ $size -gt 0 && $size -le $((16 * 513)) ]] || return 1
  last=$(tail -c 1 "$source" | od -An -t x1 | tr -d '[:space:]')
  [[ $last == 0a ]] || return 1
  while IFS= read -r path; do
    count=$((count + 1))
    [[ $count -le 16 && -n $path && ${#path} -le 512 ]] || return 1
    [[ $path != \#* && $path != /* && $path != *\\* && $path != */ && $path != *//* ]] || return 1
    [[ $path != . && $path != .. && $path != ./* && $path != */./* && $path != */. ]] || return 1
    [[ $path != ../* && $path != */../* && $path != */.. ]] || return 1
    [[ -z $previous || $previous < $path ]] || return 1
    previous=$path
  done < "$source"
  [[ $count -gt 0 ]] || return 1
  cp "$source" "$run_tmp/intents.snapshot"
}

# run_ocm_prepare derives one scope map, regenerating a map left by an earlier
# target: every rebind after a sidecar commit moves HEAD, so each frozen scope
# path holds an outdated map that prepare refuses without --replace.
run_ocm_prepare() {
  local step=$1 output=$2 map=$3 path=$4
  local error_file="$evidence/$step.stderr" return_code reason
  "$corvint_bin" --root "$repo" ocm prepare --map "$map" --cem .corvint/change.cem.json \
    --intent "$path" --expected-base "$base" --target "$target" > "$output" 2> "$error_file" &
  child_pid=$!
  wait "$child_pid"
  return_code=$?
  child_pid=
  if [[ $return_code -eq 0 ]]; then
    append_step "$step" PRODUCED none
    return 0
  fi
  reason=$(failure_reason "$error_file" "$return_code")
  if [[ $reason != map-outdated ]]; then
    append_step "$step" NOT_PRODUCED "$reason"
    return "$return_code"
  fi
  run_corvint "$step" "$output" ocm prepare --map "$map" --cem .corvint/change.cem.json \
    --intent "$path" --expected-base "$base" --target "$target" --replace
}

# load_ocm_link_plan freezes the optional DOGFOOD_OCM_LINKS plan and records its
# digest and row count (DCW-V0-018). Without it no obligation is linked and no row is reported.
load_ocm_link_plan() {
  local snapshot="$run_tmp/ocm-links.snapshot" size last controls
  ocm_links_ready=0
  [[ -n ${DOGFOOD_OCM_LINKS:-} ]] || return 0
  if [[ ! -f $DOGFOOD_OCM_LINKS ]]; then
    append_step ocm-links NOT_PRODUCED ocm-link-plan-unavailable
    return 0
  fi
  if ! (umask 077; head -c 4194305 "$DOGFOOD_OCM_LINKS" > "$snapshot"); then
    append_step ocm-links NOT_PRODUCED invalid-ocm-link-plan
    return 0
  fi
  size=$(wc -c < "$snapshot")
  if [[ $size -eq 0 ]]; then
    append_step ocm-links NOT_PRODUCED empty-ocm-link-plan
    return 0
  fi
  last=$(tail -c 1 "$snapshot" | od -An -t x1 | tr -d '[:space:]')
  controls=$(tr -d '\11\12\40-\176\200-\377' < "$snapshot" | wc -c)
  if [[ $size -gt 4194304 || $(wc -l < "$snapshot") -gt 256 || $last != 0a || $controls -ne 0 ]] ||
    ! awk -F '\t' 'NR == FNR { scope[$0] = 1; next }
      NF != 5 || $2 == "" || $4 == "" || $3 ~ /(^|,)(,|$)/ || $5 ~ /(^|,)(,|$)/ || !($1 in scope) { exit 1 }' \
      "$run_tmp/intents.snapshot" "$snapshot"; then
    append_step ocm-links NOT_PRODUCED invalid-ocm-link-plan
    return 0
  fi
  ocm_link_plan_sha256=$(shasum -a 256 "$snapshot" | awk '{print $1}')
  ocm_link_plan_rows=$(wc -l < "$snapshot" | tr -d '[:space:]')
  append_step ocm-links PRODUCED none
  ocm_links_ready=1
}

# append_link_list adds one FLAG per comma-separated item to the caller's link_args.
append_link_list() {
  local flag=$1 rest=$2,
  while [[ -n $rest ]]; do
    link_args+=("$flag" "${rest%%,*}")
    rest=${rest#*,}
  done
}

# run_ocm_links applies each author row naming this scope through the verified
# `ocm link`; a link is never inferred, and each row reports ocm-link-<plan row>.
run_ocm_links() {
  local map=$1 path=$2 intent obligation hunks test_path claims row=0 return_code
  local step output error_file link_args
  while IFS=$'\t' read -r intent obligation hunks test_path claims; do
    row=$((row + 1))
    [[ $intent == "$path" ]] || continue
    step=$(printf 'ocm-link-%03d' "$row")
    output="$evidence/$step.json"
    error_file="$evidence/$step.stderr"
    link_args=(ocm link --map "$map" --cem .corvint/change.cem.json --obligation "$obligation"
      --test-path "$test_path" --expected-base "$base" --target "$target")
    append_link_list --hunk "$hunks"
    append_link_list --claim "$claims"
    "$corvint_bin" --root "$repo" "${link_args[@]}" > "$output" 2> "$error_file" &
    child_pid=$!
    wait "$child_pid"
    return_code=$?
    child_pid=
    if [[ $return_code -eq 0 ]]; then
      append_step "$step" PRODUCED none
    else
      append_step "$step" NOT_PRODUCED "$(failure_reason "$error_file" "$return_code")"
    fi
  done < "$run_tmp/ocm-links.snapshot"
}

run_ocm_scopes() {
  local path ordinal=0 map failed=false
  load_ocm_link_plan
  while IFS= read -r path; do
    ordinal=$((ordinal + 1))
    map=$(printf '.corvint/change.ocm.%03d.json' "$ordinal")
    if run_ocm_prepare "ocm-prepare-$(printf '%03d' "$ordinal")" "$evidence/ocm-prepare-$(printf '%03d' "$ordinal").json" \
      "$map" "$path"; then
      [[ $ocm_links_ready == 0 ]] || run_ocm_links "$map" "$path"
    else
      failed=true
    fi
    run_corvint "ocm-status-$(printf '%03d' "$ordinal")" "$evidence/ocm-status-$(printf '%03d' "$ordinal").json" \
      ocm status --map "$map" --cem .corvint/change.cem.json \
      --expected-base "$base" --target "$target" || failed=true
  done < "$run_tmp/intents.snapshot"
  [[ $failed == false ]]
}

finish_ocm_aggregate() {
  local source=$DOGFOOD_INTENTS_FILE
  if ! cmp -s "$source" "$run_tmp/intents.snapshot"; then
    append_step ocm-aggregate NOT_PRODUCED intent-scope-drift
    return 1
  fi
  intent_publish_tmp="$repo/.corvint/.change.ocm-intents.$$"
  if ! cp "$run_tmp/intents.snapshot" "$intent_publish_tmp" || ! chmod 600 "$intent_publish_tmp" || ! mv "$intent_publish_tmp" "$intent_snapshot"; then
    append_step ocm-aggregate NOT_PRODUCED intent-scope-drift
    return 1
  fi
  intent_publish_tmp=
  run_corvint ocm-aggregate "$aggregate_status" dogfood-ocm status \
    --expected-base "$base" --target "$target"
}

count_bootstrap_unknowns() {
  local path count=0
  while IFS= read -r path; do
    if ! git -C "$repo" cat-file -e "$base:$path" 2>/dev/null; then
      count=$((count + 1))
    fi
  done < "$run_tmp/intents.snapshot"
  bootstrap_unknown=$count
}

validate_citation_plan() {
  local snapshot="$run_tmp/citations.snapshot" size last controls
  # Freeze at most the byte ceiling plus one sentinel byte; no caller read
  # or native work can grow with a concurrently enlarged plan.
  (umask 077; head -c 4194305 "$DOGFOOD_CITATIONS" > "$snapshot") || return 1
  size=$(wc -c < "$snapshot")
  [[ $size -le 4194304 ]] || return 1
  citation_count=$(wc -l < "$snapshot")
  [[ $citation_count -le 256 ]] || return 1
  [[ $size -gt 0 ]] || return 0
  last=$(tail -c 1 "$snapshot" | od -An -t x1 | tr -d '[:space:]')
  [[ $last == 0a ]] || return 1
  # Retain only forbidden control bytes, including NUL which Bash read
  # otherwise silently discards. Tabs and LF delimit the four-column rows.
  controls=$(tr -d '\11\12\40-\176\200-\377' < "$snapshot" | wc -c)
  [[ $controls -eq 0 ]] || return 1
  awk -F '\t' 'NF != 4 || $1 == "" || $2 == "" || $3 == "" || $4 == "" { exit 1 }' "$snapshot"
}

resolve_anchor
sealed_in_change=$(git -C "$repo" diff --name-only "$base" "$target" -- .corvint/changes) || exit 2
if [[ -n $sealed_in_change ]]; then
  printf 'dogfood-change: REFUSE sealed-cem-in-change\n' >&2
  printf '  BASE..HEAD adds a sealed CEM; revert or drop the seal commit, then rebind\n' >&2
  exit 2
fi
resolve_corvint_bin

run_corvint prechange-query "$evidence/prechange-query.json" \
  query --task "$task" --limit 1 || :
run_prechange_impact || :
# The recorder refuses a dirty tree and cem-prepare rewrites the tracked CEM,
# so the outcome is classified first and its row is reported in step order.
local_outcome_row=
verify_args=()
if [[ -n ${DOGFOOD_VERIFY_FILE:-} && ! -f $DOGFOOD_VERIFY_FILE ]]; then
  local_outcome_row="NOT_PRODUCED\tverify-file-unavailable"
elif [[ -n ${DOGFOOD_VERIFY_FILE:-} ]]; then
  collect_verify_commands < "$DOGFOOD_VERIFY_FILE"
elif [[ -n ${DOGFOOD_VERIFY:-} ]]; then
  collect_verify_commands <<< "$DOGFOOD_VERIFY"
fi
if [[ -n $local_outcome_row ]]; then
  :
elif [[ -z ${DOGFOOD_OUTCOME:-} || ${#verify_args[@]} -eq 0 ]]; then
  local_outcome_row="NOT_PRODUCED\toutcome-input-not-provided"
else
  run_dogfood_record "$evidence/local-outcome.json" \
    --base "$base" --target "$target" --task "$task" \
    "${verify_args[@]}" --outcome "$DOGFOOD_OUTCOME" || :
fi
run_cem_prepare "$evidence/cem-prepare.json" || :

if [[ ! -f $repo/.corvint/change.cem.json ]]; then
  append_step cem-cite NOT_PRODUCED cem-map-not-produced
elif [[ -z ${DOGFOOD_CITATIONS:-} ]]; then
  append_step cem-cite NOT_PRODUCED citation-plan-not-provided
elif [[ ! -f $DOGFOOD_CITATIONS ]]; then
  append_step cem-cite NOT_PRODUCED citation-plan-unavailable
else
  cite_output="$evidence/cem-cite.jsonl"
  (umask 077; : > "$cite_output")
  chmod 600 "$cite_output"
  cite_status=PRODUCED
  cite_reason=none
  if ! validate_citation_plan; then
    cite_status=NOT_PRODUCED
    cite_reason=invalid-citation-plan
  elif [[ $citation_count -gt 1 && ( -e $repo/$citation_stage || -L $repo/$citation_stage ) ]]; then
    cite_status=NOT_PRODUCED
    cite_reason=citation-stage-exists
  else
    cite_map=.corvint/change.cem.json
    cite_ordinal=0
    if [[ $citation_count -gt 1 ]]; then
      citation_stage_owned=1
    fi
    while IFS=$'\t' read -r hunk evidence_path lines relation; do
      cite_ordinal=$((cite_ordinal + 1))
      cite_args=(cem cite --map "$cite_map" --hunk "$hunk" --evidence-path "$evidence_path"
        --lines "$lines" --relation "$relation")
      if [[ $citation_count -gt 1 ]]; then
        cite_destination=$citation_stage
        if [[ $cite_ordinal -eq $citation_count ]]; then
          cite_destination=.corvint/change.cem.json
        fi
        cite_args+=(--output "$cite_destination")
      fi
      part="$run_tmp/cem-cite.json"
      error_file="$evidence/cem-cite.stderr"
      "$corvint_bin" --root "$repo" "${cite_args[@]}" > "$part" 2> "$error_file" &
      child_pid=$!
      wait "$child_pid"
      return_code=$?
      child_pid=
      if [[ $return_code -ne 0 ]]; then
        cite_status=NOT_PRODUCED
        cite_reason=$(failure_reason "$error_file" "$return_code")
        break
      fi
      # Intermediate receipts truthfully name the stage; only the last cite
      # publishes the original map. Keep their raw stdout as private evidence.
      cat "$part" >> "$cite_output"
      cite_map=$citation_stage
    done < "$run_tmp/citations.snapshot"
  fi
  if ! cleanup_citation_stage; then
      cite_status=NOT_PRODUCED
      cite_reason=citation-stage-cleanup-failed
  fi
  append_step cem-cite "$cite_status" "$cite_reason"
fi

: > "$aggregate_status"
if ! validate_intent_manifest; then
  append_step ocm-aggregate NOT_PRODUCED missing-intent-scope
elif run_ocm_scopes; then
  if finish_ocm_aggregate; then
    count_bootstrap_unknowns
  fi
else
  append_step ocm-aggregate NOT_PRODUCED intent-scope-drift
fi

run_corvint cem-status "$evidence/cem-status.json" \
  cem status --map .corvint/change.cem.json --expected-base "$base" --target "$target" \
  --max-unknown "$bootstrap_unknown" --max-mechanical 0 || :

printf 'local-outcome\t%b\n' "$local_outcome_row" >> "$rows"

# Each refusal a first-time adopter hits names its fix (DCW-V0-014,
# docs/DOGFOOD.md "Daily adopter path").
fix_hint() {
  case "$1:$2" in
    cem-cite:citation-plan-not-provided)
      printf 'set DOGFOOD_CITATIONS to the path of a TSV plan with one row per hunk of .corvint/change.cem.json' ;;
    cem-cite:citation-plan-unavailable)
      printf 'DOGFOOD_CITATIONS must be the path of a TSV file of ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION rows, not the rows themselves' ;;
    cem-cite:invalid-citation-plan)
      printf 'each row is ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION in worklist order, LF-terminated, at most 256 rows' ;;
    ocm-aggregate:missing-intent-scope)
      printf 'DOGFOOD_INTENTS_FILE must be the path of a sorted, LF-terminated file listing 1-16 repository-relative spec paths' ;;
    ocm-prepare-*:invalid-requirements-section)
      printf 'intent must be a spec that exists at BASE and contains exactly one "## Requirements" heading' ;;
    ocm-prepare-*:excluded-artifact-mismatch|prechange-impact:unsupported-impact-worktree|local-outcome:record-index-failed)
      printf 'the worktree has uncommitted changes (often the prepared sidecar); commit them, then rerun make dogfood-change' ;;
    ocm-status-*)
      printf 'fix the ocm-prepare row with the same number first; if it was produced, the worktree has uncommitted changes (often the prepared sidecar); commit them, then rerun make dogfood-change' ;;
    cem-status:not-ready)
      printf 'read verification.issues and policyIssues in %s: excluded-artifact-mismatch means the sidecar is uncommitted, max-unknown-exceeded means DOGFOOD_CITATIONS does not cite every hunk' "$evidence/cem-status.json" ;;
    ocm-links:ocm-link-plan-unavailable)
      printf 'DOGFOOD_OCM_LINKS must be the path of a TSV file of INTENT<TAB>REQUIREMENT<TAB>HUNKS<TAB>TEST_PATH<TAB>CLAIMS rows, not the rows themselves' ;;
    ocm-links:empty-ocm-link-plan)
      printf 'DOGFOOD_OCM_LINKS names an empty file; add at least one row, or unset DOGFOOD_OCM_LINKS so every requirement stays unassessed' ;;
    ocm-links:invalid-ocm-link-plan)
      printf 'each DOGFOOD_OCM_LINKS row is INTENT<TAB>REQUIREMENT<TAB>HUNK[,HUNK...]<TAB>TEST_PATH<TAB>CLAIM[,CLAIM...], LF-terminated, at most 256 rows, and INTENT is listed in DOGFOOD_INTENTS_FILE' ;;
    ocm-link-*)
      printf 'read %s/%s.stderr: each linked hunk must be cited in the committed sidecar, and each claim a test case or t.Run name at HEAD containing the exact requirement ID; otherwise delete the DOGFOOD_OCM_LINKS row so the requirement stays unassessed' "$evidence" "$1" ;;
    ocm-aggregate:intent-scope-drift)
      printf 'fix the ocm-prepare or ocm-status row above; otherwise the intents file changed during the run' ;;
    local-outcome:outcome-input-not-provided)
      printf 'set DOGFOOD_OUTCOME (passed, failed or blocked) and DOGFOOD_VERIFY_FILE (one verification command per line)' ;;
  esac
}

render_report
if awk -F '\t' '$2 != "PRODUCED" && !($1 == "local-outcome" && $2 == "NOT_PRODUCED" && $3 == "no-source-paths") && !($1 == "prechange-impact" && $2 == "NOT_PRODUCED" && $3 == "unsupported-impact-range") { failed=1 } END { exit failed ? 0 : 1 }' "$rows"; then
  printf 'dogfood-change: FAIL not-complete\n' >&2
  awk -F '\t' '$2 != "PRODUCED" && !($1 == "local-outcome" && $2 == "NOT_PRODUCED" && $3 == "no-source-paths") && !($1 == "prechange-impact" && $2 == "NOT_PRODUCED" && $3 == "unsupported-impact-range") { printf "%s\t%s\n", $1, $3 }' "$rows" |
    while IFS=$'\t' read -r step reason; do
      printf '  %s: %s\n' "$step" "$reason"
      hint=$(fix_hint "$step" "$reason")
      [[ -z "$hint" ]] || printf '    fix: %s\n' "$hint"
    done >&2
  # The authority-start refusal is selected by the task wording, not by the
  # change; a malformed store refuses any wording and names no profile.
  if rg -q '^\{"code": "unsupported-query-trace-state", "error": "native Go authority-start query ' \
    "$evidence/prechange-query.stderr"; then
    printf '  prechange-query: DOGFOOD_TASK wording selected the authority-start profile, which refuses a present local trace store; keep this receipt and the task (docs/DOGFOOD.md section 1)\n' >&2
  fi
  # Amending or rebasing after a recorded pass strands that trace; query and
  # the recorder then refuse every later run with this message.
  if rg -q -e '"error": "local trace store contains unreachable revision: ' \
    "$evidence/prechange-query.stderr" "$evidence/local-outcome.stderr" 2>/dev/null; then
    printf '  local trace store: a recorded trace names a commit no longer reachable from HEAD; restore that commit as an ancestor and add new commits instead of amending or rebasing (docs/DOGFOOD.md "Daily adopter path")\n' >&2
  fi
  printf '  full report: %s\n' "$report" >&2
  exit 1
fi
