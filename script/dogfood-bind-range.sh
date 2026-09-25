#!/usr/bin/env bash
# usage: dogfood-bind-range.sh BASE TARGET
# Binds the already-landed range BASE..TARGET to one CEM after the fact
# (docs/DOGFOOD.md section 4, DOGFOOD-011). DOGFOOD_CITATIONS optionally names a
# four-column citation plan written against this range's private prepared map, and
# DOGFOOD_UNKNOWN a three-column plan of hunks that no base evidence supports.
# The main worktree, its sidecar and its dogfood report are never written.
set -uo pipefail
export LC_ALL=C

# Read the executable selection before Git, temporary files, or a selected executable can run.
corvint_bin_set=${CORVINT_BIN+x}
corvint_bin=${CORVINT_BIN-}

base_arg=${1:?usage: dogfood-bind-range.sh BASE TARGET}
target_arg=${2:?usage: dogfood-bind-range.sh BASE TARGET}
repo=$(cd "$(dirname "$0")/.." && pwd)
run_tmp=
worktree=
child_pid=

refuse() {
  printf 'dogfood-bind-range: REFUSE %s\n' "$1" >&2
  exit 2
}

fail() {
  printf 'dogfood-bind-range: FAIL %s\n' "$1" >&2
  exit 1
}

cleanup() {
  if [[ -n ${child_pid:-} ]]; then
    kill -TERM "$child_pid" 2>/dev/null || :
    wait "$child_pid" 2>/dev/null || :
    child_pid=
  fi
  if [[ -n ${worktree:-} ]]; then
    git -C "$repo" worktree remove --force "$worktree" 2>/dev/null || :
    worktree=
  fi
  if [[ -n ${run_tmp:-} ]]; then
    rm -rf "$run_tmp"
    run_tmp=
  fi
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

run_child() {
  local output=$1 error_file=$2
  shift 2
  "$@" > "$output" 2> "$error_file" &
  child_pid=$!
  wait "$child_pid"
  child_status=$?
  child_pid=
}

failure_reason() {
  local reason
  reason=$(sed -n 's/.*"code"[[:space:]]*:[[:space:]]*"\([A-Za-z0-9_-]*\)".*/\1/p' "$1" | head -1)
  printf '%s' "${reason:-exit-$2}"
}

resolve_corvint_bin() {
  local expected_version
  expected_version=$(cat "$repo/VERSION")
  [[ -n $expected_version ]] || refuse project-version-unavailable
  if [[ -z $corvint_bin_set ]]; then
    corvint_bin="$run_tmp/corvint"
    run_child /dev/null "$run_tmp/corvint-build.stderr" \
      env GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go build -o "$corvint_bin" ./cmd/corvint
    [[ $child_status -eq 0 ]] || refuse current-tree-corvint-build-failed
    return
  fi
  run_child "$run_tmp/corvint-version.stdout" "$run_tmp/corvint-version.stderr" "$corvint_bin" --version
  if [[ $child_status -ne 0 || ! $(cat "$run_tmp/corvint-version.stdout") =~ ^"Corvint $expected_version (build "(0|[1-9][0-9]*)")"$ ]]; then
    refuse "corvint-version-mismatch expected=$expected_version"
  fi
}

# snapshot_plan SOURCE SNAPSHOT copies a plan once and applies the shared row limits.
snapshot_plan() {
  local snapshot=$2 size rows last controls
  (umask 077; head -c 4194305 "$1" > "$snapshot") || return 1
  size=$(wc -c < "$snapshot")
  rows=$(wc -l < "$snapshot")
  [[ $size -le 4194304 && $rows -le 256 ]] || return 1
  [[ $size -gt 0 ]] || return 0
  last=$(tail -c 1 "$snapshot" | od -An -t x1 | tr -d '[:space:]')
  [[ $last == 0a ]] || return 1
  controls=$(tr -d '\11\12\40-\176\200-\377' < "$snapshot" | wc -c)
  [[ $controls -eq 0 ]]
}

validate_citation_plan() {
  local snapshot="$run_tmp/citations.snapshot"
  snapshot_plan "$DOGFOOD_CITATIONS" "$snapshot" || return 1
  awk -F '\t' 'NF != 4 || $1 == "" || $2 == "" || $3 == "" || $4 == "" { exit 1 }' "$snapshot"
}

# Each unknown row is a distinct HUNK, a registered unknown reason, and a visible detail token.
validate_unknown_plan() {
  local snapshot="$run_tmp/unknown.snapshot"
  snapshot_plan "$DOGFOOD_UNKNOWN" "$snapshot" || return 1
  awk -F '\t' 'NF != 3 || $1 == "" || $2 !~ /^(no-evidence|insufficient-evidence|conflicting-evidence)$/ ||
    $3 !~ /^[a-z0-9][a-z0-9.-]*$/ || seen[$1]++ { exit 1 }' "$snapshot" || return 1
  local hunk reason detail
  while IFS=$'\t' read -r hunk reason detail; do
    [[ $detail != removed-intent.* ]] || removed_intent_pin_at_base "$detail" || return 1
  done < "$snapshot"
}

# citation_plan_matches_map binds a nonempty citation plan to the map prepared in this run, as the
# bind loop does (DCW-V0-019): a numeric selector must be a canonical ordinal no larger than the
# hunk count, and every unknown hunk must be named by ordinal or ID in the citation or unknown plan.
citation_plan_matches_map() {
  local plans=("$run_tmp/citations.snapshot")
  [[ -z ${DOGFOOD_UNKNOWN:-} ]] || plans+=("$run_tmp/unknown.snapshot")
  awk -F '\t' '
    FILENAME == ARGV[1] {
      if ($0 == "  \"hunks\": [") { inside = 1; next }
      if (substr($0, 1, 3) == "  ]") inside = 0
      if (!inside) next
      if ($0 == "    {") count++
      if ($0 ~ /^      "id": "/) { value = $0; sub(/^      "id": "/, "", value); sub(/",?$/, "", value); id[count] = value }
      if ($0 ~ /^      "disposition": "unknown",?$/) owed[count] = 1
      next
    }
    FILENAME == ARGV[2] && $1 ~ /^[+0-9]/ { if ($1 !~ /^[1-9][0-9]*$/ || $1 + 0 > count) bad = 1 }
    { named[$1] = 1 }
    END {
      if (bad) exit 1
      for (hunk in owed) if (!(hunk in named) && !(id[hunk] in named)) exit 1
    }' "$map" "${plans[@]}"
}

# Decision 0165: a removed-intent.OID.START-END detail must name a regular blob in BASE's tree
# and a nonempty byte span inside it; it stays a NOT_PRODUCED note, never evidence.
removed_intent_pin_at_base() {
  [[ $1 =~ ^removed-intent\.([0-9a-f]{40}|[0-9a-f]{64})\.([0-9]+)-([0-9]+)$ ]] || return 1
  local oid=${BASH_REMATCH[1]} start=${BASH_REMATCH[2]} end=${BASH_REMATCH[3]} size
  git -C "$repo" ls-tree -r "$base" |
    awk -v oid="$oid" '$1 ~ /^100(644|755)$/ && $3 == oid { found = 1 } END { exit !found }' || return 1
  size=$(git -C "$repo" cat-file -s "$oid") || return 1
  [[ ${#end} -le 12 ]] || return 1
  ((10#$start < 10#$end)) || return 1
  ((10#$end <= size))
}

base=$(git -C "$repo" rev-parse --verify --quiet "$base_arg^{commit}") || refuse base-unavailable
target=$(git -C "$repo" rev-parse --verify --quiet "$target_arg^{commit}") || refuse target-unavailable
head=$(git -C "$repo" rev-parse 'HEAD^{commit}') || exit 2
git_dir=$(git -C "$repo" rev-parse --absolute-git-dir) || exit 2
[[ $base != "$target" ]] || refuse empty-range
git -C "$repo" merge-base --is-ancestor "$base" "$target" || refuse base-not-ancestor-of-target
git -C "$repo" merge-base --is-ancestor "$target" "$head" || refuse target-not-landed
[[ $(git -C "$repo" rev-list --count "$base..$target") -le 256 ]] || refuse range-exceeds-256-commits
if [[ -n ${DOGFOOD_CITATIONS:-} && ! -f $DOGFOOD_CITATIONS ]]; then
  refuse citation-plan-unavailable
fi
if [[ -n ${DOGFOOD_UNKNOWN:-} && ! -f $DOGFOOD_UNKNOWN ]]; then
  refuse unknown-plan-unavailable
fi

evidence="$git_dir/corvint"
mkdir -p "$evidence" || exit 2
run_tmp=$(mktemp -d "$evidence/dogfood-bind-range.XXXXXX") || exit 2
range_label="${base:0:12}..${target:0:12}"
prepared_copy="$evidence/bind-range.$range_label.cem.json"
resolve_corvint_bin

# A private, never-checked-out worktree at TARGET holds the map, so the tracked
# sidecar in the main worktree and its private report stay untouched.
worktree="$run_tmp/worktree"
git -C "$repo" worktree add --detach --no-checkout --quiet "$worktree" "$target" 2> "$run_tmp/worktree.stderr" ||
  { worktree=; refuse private-worktree-unavailable; }
run_child "$run_tmp/cem-prepare.json" "$run_tmp/cem-prepare.stderr" \
  "$corvint_bin" --root "$worktree" cem prepare --base "$base" --target "$target" --replace
if [[ $child_status -ne 0 ]]; then
  fail "cem-prepare $(failure_reason "$run_tmp/cem-prepare.stderr" "$child_status")"
fi
map="$worktree/.corvint/change.cem.json"
# The uncited prepared map is the one a citation plan is written against.
cp "$map" "$prepared_copy" || exit 2
if [[ -n ${DOGFOOD_CITATIONS:-} ]]; then
  validate_citation_plan || fail 'cem-cite invalid-citation-plan'
fi
if [[ -n ${DOGFOOD_UNKNOWN:-} ]]; then
  validate_unknown_plan || fail 'cem-mark invalid-unknown-plan'
fi
if [[ -z ${DOGFOOD_CITATIONS:-} ]]; then
  printf 'dogfood-bind-range: NOTE cem-cite NOT_PRODUCED citation-plan-not-provided\n' >&2
else
  [[ ! -s $run_tmp/citations.snapshot ]] || citation_plan_matches_map || fail 'cem-cite citation-plan-map-mismatch'
  while IFS=$'\t' read -r hunk evidence_path lines relation; do
    run_child "$run_tmp/cem-cite.json" "$run_tmp/cem-cite.stderr" \
      "$corvint_bin" --root "$worktree" cem cite --map .corvint/change.cem.json --hunk "$hunk" \
      --evidence-path "$evidence_path" --lines "$lines" --relation "$relation"
    if [[ $child_status -ne 0 ]]; then
      fail "cem-cite $(failure_reason "$run_tmp/cem-cite.stderr" "$child_status")"
    fi
  done < "$run_tmp/citations.snapshot"
fi
# An explicit unknown stays in the map and in the binding message; it is never cited.
unknown_count=0
unknown_notes=
if [[ -n ${DOGFOOD_UNKNOWN:-} ]]; then
  while IFS=$'\t' read -r hunk reason detail; do
    run_child "$run_tmp/cem-mark.json" "$run_tmp/cem-mark.stderr" \
      "$corvint_bin" --root "$worktree" cem mark --map .corvint/change.cem.json --hunk "$hunk" \
      --disposition unknown --reason "$reason"
    if [[ $child_status -ne 0 ]]; then
      fail "cem-mark $(failure_reason "$run_tmp/cem-mark.stderr" "$child_status")"
    fi
    unknown_count=$((unknown_count + 1))
    unknown_notes+="NOT_PRODUCED hunk=$hunk reason=$reason detail=$detail"$'\n'
    printf 'dogfood-bind-range: NOTE cem-mark NOT_PRODUCED hunk=%s reason=%s detail=%s\n' \
      "$hunk" "$reason" "$detail" >&2
  done < "$run_tmp/unknown.snapshot"
fi

# The binding commit is TARGET's tree with only the sidecar replaced, so its
# canonical patch is exactly BASE..TARGET and CEM-CB-009 holds at its target.
blob=$(git -C "$repo" hash-object -w "$map") || fail binding-commit-failed
if ! GIT_INDEX_FILE="$run_tmp/binding.index" git -C "$repo" read-tree "$target" ||
  ! GIT_INDEX_FILE="$run_tmp/binding.index" git -C "$repo" update-index --add \
    --cacheinfo "100644,$blob,.corvint/change.cem.json"; then
  fail binding-commit-failed
fi
tree=$(GIT_INDEX_FILE="$run_tmp/binding.index" git -C "$repo" write-tree) || fail binding-commit-failed
unknown_message=()
[[ -z $unknown_notes ]] || unknown_message=(-m "${unknown_notes%$'\n'}")
binding=$(git -C "$repo" commit-tree "$tree" -p "$target" \
  -m "chore: retroactively bind $range_label to a CEM" ${unknown_message[@]+"${unknown_message[@]}"} \
  -m 'Corvint-Dogfood-Binding: retroactive' 2> "$run_tmp/commit-tree.stderr") || fail binding-commit-failed

run_child "$run_tmp/cem-status.json" "$run_tmp/cem-status.stderr" \
  "$corvint_bin" --root "$worktree" cem status --map .corvint/change.cem.json \
  --expected-base "$base" --target "$binding" --max-unknown "$unknown_count" --max-mechanical 0
cp "$run_tmp/cem-status.json" "$evidence/bind-range.$range_label.cem-status.json" || exit 2
if [[ $child_status -ne 0 ]]; then
  if [[ -s $run_tmp/cem-status.json ]]; then
    sed -n '1p' "$run_tmp/cem-status.json"
    fail cem-policy
  fi
  fail "cem-status $(failure_reason "$run_tmp/cem-status.stderr" "$child_status")"
fi
sed -n '1p' "$run_tmp/cem-status.json"
for step in prechange-query prechange-impact local-outcome ocm-aggregate; do
  printf 'dogfood-bind-range: NOTE %s NOT_PRODUCED retroactive-binding\n' "$step" >&2
done
printf 'dogfood-bind-range: PASS retroactive binding=%s range=%s..%s\n' "$binding" "$base" "$target"
printf '  next: git merge --no-ff -s ours -m "chore: merge retroactive binding %s" %s\n' "$range_label" "$binding"
