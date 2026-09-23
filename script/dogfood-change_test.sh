#!/usr/bin/env bash
set -euo pipefail

# This wrapper asserts on captured output with ripgrep (rg); without it every rg call below fails
# as a bare "command not found" partway through, which reads as a real regression instead of a
# missing prerequisite. Fail closed here, before any fixture is built.
if ! command -v rg >/dev/null 2>&1; then
    printf 'dogfood-change_test: REFUSE unsupported-environment-missing-rg\n' >&2
    exit 1
fi

source_root=$(cd "$(dirname "$0")/.." && pwd)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-dogfood-change-test.XXXXXX")
active_runner=
active_descendant=
active_pid_file=
# Independent scenario groups run as background jobs, each in its own clone or log.
phase_jobs=

poll_for_file() {
  local path=$1 deadline=$((SECONDS + 60))
  while [[ ! -f $path ]]; do
    if (( SECONDS >= deadline )); then
      printf 'timed out waiting 60 s for file: %s\n' "$path" >&2
      return 1
    fi
    sleep 0.02
  done
}

poll_for_exit() {
  local pid=$1 deadline=$((SECONDS + 60))
  while kill -0 "$pid" 2>/dev/null; do
    (( SECONDS < deadline )) || return 1
    sleep 0.02
  done
}

# Invoked through the signal/exit trap below.
# shellcheck disable=SC2329
cleanup() {
  for job in $phase_jobs; do
    kill -TERM -- "-$job" 2>/dev/null || :
    wait "$job" 2>/dev/null || :
  done
  if [[ -n $active_runner ]]; then
    kill -TERM "$active_runner" 2>/dev/null || :
    wait "$active_runner" 2>/dev/null || :
  fi
  if [[ -z $active_descendant && -n $active_pid_file && -f $active_pid_file ]]; then
    active_descendant=$(cat "$active_pid_file")
  fi
  if [[ -n $active_descendant ]]; then
    kill -TERM "$active_descendant" 2>/dev/null || :
    wait "$active_descendant" 2>/dev/null || :
  fi
  rm -rf "$test_root"
}
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/repo/script" "$test_root/repo/docs/specs" "$test_root/bin"
cp "$source_root/script/dogfood-change.sh" "$source_root/script/dogfood-check.sh" "$test_root/repo/script/"
cp "$source_root/.gitignore" "$test_root/repo/"
printf '%s\n' '0.4.0a4' > "$test_root/repo/VERSION"
# Literal backticks are requirement syntax, not shell interpolation.
# shellcheck disable=SC2016
printf '%s\n' '# Intent A' '' '## Requirements' '' '- `TEST-A-001`: test.' > "$test_root/repo/docs/specs/intent-a.md"
# shellcheck disable=SC2016
printf '%s\n' '# Intent B' '' '## Requirements' '' '- `TEST-B-001`: test.' > "$test_root/repo/docs/specs/intent-b.md"
printf '%s\n' docs/specs/intent-a.md docs/specs/intent-b.md > "$test_root/intents.txt"
printf 'base\n' > "$test_root/repo/script/source.sh"
mkdir -p "$test_root/repo/.corvint"
printf '{"stale": true}\n' > "$test_root/repo/.corvint/change.cem.json"
git -C "$test_root/repo" init -q -b main
git -C "$test_root/repo" -c user.name=t -c user.email=t@example.invalid add .
git -C "$test_root/repo" -c user.name=t -c user.email=t@example.invalid commit -qm base
base=$(git -C "$test_root/repo" rev-parse HEAD)
clean_no_change_output=$(cd "$test_root/repo" && script/dogfood-check.sh "$base")
test "$clean_no_change_output" = 'dogfood-check: PASS no-change-clean-worktree'
printf 'changed\n' > "$test_root/repo/script/source.sh"
uncommitted_check_status=0
uncommitted_check_output=$(cd "$test_root/repo" && script/dogfood-check.sh "$base" 2>&1) || \
  uncommitted_check_status=$?
test "$uncommitted_check_status" = 2
printf '%s\n' "$uncommitted_check_output" | \
  rg -q '^dogfood-check: REFUSE uncommitted-change-not-in-base-target$'
printf '\nlocal-only\n' >> "$test_root/repo/.gitignore"
git -C "$test_root/repo" -c user.name=t -c user.email=t@example.invalid add script/source.sh .gitignore
git -C "$test_root/repo" -c user.name=t -c user.email=t@example.invalid commit -qm target

cat > "$test_root/bin/corvint" <<'EOF'
#!/usr/bin/env bash
set -eu
if [[ ${1:-} == --version ]]; then
  printf 'Corvint 0.4.0a4 (build 12)\n'
  exit 0
fi
root=$2
action=$3
sub=${4:-}
mkdir -p "$root/.corvint"
printf '%s\n' "$*" >> "$DOGFOOD_TEST_LOG"
if [[ $action == dogfood-observe ]]; then
  [[ ${DOGFOOD_OBSERVE_FAIL:-0} == 0 ]] || exit 9
  exit 0
fi
if [[ $action == impact ]]; then
  case ${DOGFOOD_TEST_IMPACT:-produced} in
    produced) printf 'current impact stderr\n' >&2 ;;
    unsupported)
      printf '%s\n' '{"code": "unsupported-impact-range", "error": "impact range exceeds the 256-path bound", "ok": false}' >&2
      exit 2
      ;;
    malformed) printf '%s\n' '{"code": "unsupported-impact-range", "error": "shape", "extra": true, "ok": false}' >&2; exit 2 ;;
    invalid-escape) printf '%s\n' '{"code": "unsupported-impact-range", "error": "bad\q", "ok": false}' >&2; exit 2 ;;
    raw-tab) printf '{"code": "unsupported-impact-range", "error": "bad\tvalue", "ok": false}\n' >&2; exit 2 ;;
    wrong-exit) printf '%s\n' '{"code": "unsupported-impact-range", "error": "wrong exit", "ok": false}' >&2; exit 7 ;;
    nonempty) printf '%s\n' 'partial'; printf '%s\n' '{"code": "unsupported-impact-range", "error": "partial output", "ok": false}' >&2; exit 2 ;;
    crash) printf '%s\n' 'process failed' >&2; exit 7 ;;
  esac
fi
if [[ $action == query && ${DOGFOOD_TEST_QUERY:-} == unreachable-trace ]]; then
  printf '%s\n' '{"code": "unsupported-query-trace-state", "error": "local trace store contains unreachable revision: 1111111111111111111111111111111111111111", "ok": false}' >&2
  exit 2
fi
if [[ $action == query && ${DOGFOOD_TEST_QUERY:-} == authority-trace-state ]]; then
  printf '%s\n' '{"code": "unsupported-query-trace-state", "error": "native Go authority-start query requires an absent clean-tree local trace store", "ok": false}' >&2
  exit 2
fi
if [[ $action == cem && $sub == status ]]; then
  maximum=0
  shift 4
  while [[ $# -gt 0 ]]; do
    if [[ $1 == --max-unknown ]]; then maximum=$2; break; fi
    shift
  done
  unknown=${DOGFOOD_TEST_CEM_UNKNOWNS:-0}
  if [[ $unknown -gt $maximum ]]; then
    printf '%s\n' '{"counts":{"unknown":'"$unknown"'},"policyIssues":[{"code":"max-unknown-exceeded"}],"verification":{"valid":true}}'
    exit 1
  fi
  printf '%s\n' '{"counts":{"unknown":'"$unknown"'},"policyIssues":[],"verification":{"valid":true}}'
  exit 0
fi
if [[ $action == cem && $sub == prepare ]]; then
  if [[ -n ${DOGFOOD_TEST_CEM_PREPARE_CODE:-} && $* != *--replace* ]]; then
    printf '{"code": "%s", "error": "transient", "ok": false}\n' "$DOGFOOD_TEST_CEM_PREPARE_CODE" >&2
    exit 2
  fi
  # An existing map stands in for one bound to an earlier range: the real
  # prepare refuses it with this CEM-PILOT-018 line unless --replace is present.
  if [[ -f $root/.corvint/change.cem.json && $* != *--replace* ]]; then
    printf '{"error": "cannot read CEM map: the existing map records a different base or patch; pass --replace to regenerate", "ok": false}\n' >&2
    exit 2
  fi
  printf '{}\n' > "$root/.corvint/change.cem.json"
  if [[ -n ${DOGFOOD_TEST_UNSAFE_MAP:-} ]]; then
    rm "$root/.corvint/change.cem.json"
    ln -s "$DOGFOOD_TEST_UNSAFE_MAP" "$root/.corvint/change.cem.json"
  fi
fi
if [[ $action == cem && $sub == cite && ${DOGFOOD_TEST_CITES:-0} == 1 ]]; then
  map= output= hunk= lines=
  shift 4
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --map) map=$2; shift 2 ;;
      --output) output=$2; shift 2 ;;
      --hunk) hunk=$2; shift 2 ;;
      --lines) lines=$2; shift 2 ;;
      --evidence-path|--relation) shift 2 ;;
      *) exit 99 ;;
    esac
  done
  output=${output:-$map}
  printf '%s\t%s\t%s\t%s\n' "$map" "$output" "$hunk" "$lines" >> "$DOGFOOD_TEST_CITE_LOG"
  if [[ $lines == unstable ]]; then
    printf '{"code":"cite-span-not-stable","ok":false}\n' >&2
    exit 2
  fi
  for path in "$map" "$output"; do
    if [[ $path == /* || $path == ../* || $path == */../* ]]; then
      printf '{"code":"invalid-arguments","ok":false}\n' >&2
      exit 2
    fi
    if [[ -L $root/.corvint || -L $root/$path || ( -e $root/$path && ! -f $root/$path ) ]]; then
      printf '{"code":"publish-failed","ok":false}\n' >&2
      exit 2
    fi
  done
  if [[ -n ${DOGFOOD_TEST_CITE_PAUSE:-} && $(wc -l < "$DOGFOOD_TEST_CITE_LOG") -eq $DOGFOOD_TEST_CITE_PAUSE ]]; then
    descendant=
    stop_child() {
      if [[ -n $descendant ]]; then
        kill -TERM "$descendant" 2>/dev/null || :
        wait "$descendant" 2>/dev/null || :
        descendant=
      fi
    }
    trap stop_child EXIT
    trap 'exit 143' HUP INT TERM
    sleep 30 &
    descendant=$!
    printf '%s\n' "$descendant" > "$DOGFOOD_TEST_PID_FILE"
    wait "$descendant"
  fi
  current=$(cat "$root/$map")
  items=
  if [[ $current != '{}' ]]; then
    items=${current#'{"cites":['}
    items=${items%']}'}
  fi
  if [[ ,$items, != *,$hunk,* ]]; then
    items=${items:+$items,}$hunk
  fi
  (umask 077; printf '{"cites":[%s]}\n' "$items" > "$root/$output")
  if [[ -n ${DOGFOOD_TEST_MUTATE_PLAN:-} ]]; then
    printf 'malformed\n' > "$DOGFOOD_TEST_MUTATE_PLAN"
  fi
  printf '{"map":"%s","ok":true}\n' "$output"
  exit 0
fi
if [[ $action == ocm && $sub == prepare ]]; then
  if [[ -n ${DOGFOOD_TEST_OCM_PREPARE_CODE:-} ]]; then
    printf '{"code": "%s", "error": "refused", "ok": false}\n' "$DOGFOOD_TEST_OCM_PREPARE_CODE" >&2
    exit 2
  fi
  if [[ ${DOGFOOD_TEST_CITES:-0} == 1 && -z ${DOGFOOD_TEST_ALLOW_STAGE_COLLISION:-} ]]; then
    for stage in "$root"/.corvint/.cem-citations.*.json; do
      if [[ -e $stage || -L $stage ]]; then
        printf '{"code":"stage-not-cleaned-before-ocm","ok":false}\n' >&2
        exit 2
      fi
    done
  fi
  shift 4
  # An existing map stands in for one bound to an earlier target: the real
  # prepare refuses it with map-outdated unless --replace is present.
  replace=0
  for arg in "$@"; do
    [[ $arg == --replace ]] && replace=1
  done
  while [[ $# -gt 0 ]]; do
    if [[ $1 == --map ]]; then
      if [[ -e $root/$2 && $replace == 0 ]]; then
        printf '{"code":"map-outdated","ok":false}\n' >&2
        exit 2
      fi
      printf '{}\n' > "$root/$2"
      break
    fi
    shift
  done
fi
if [[ $action == ocm && $sub == link && " $* " =~ " --obligation "(${DOGFOOD_TEST_OCM_LINK_REFUSE:-^})" " ]]; then
  printf '{"code": "claim-obligation-mismatch", "error": "refused", "ok": false}\n' >&2
  exit 2
fi
if [[ $action == ocm && $sub == status && -n ${DOGFOOD_TEST_OCM_STATUS_EXIT:-} ]]; then
  exit "$DOGFOOD_TEST_OCM_STATUS_EXIT"
fi
if [[ $action == dogfood-ocm && $sub == status ]]; then
  if [[ -n ${DOGFOOD_TEST_EXPECTED_INTENTS:-} ]]; then
    expected=$(cat "$DOGFOOD_TEST_EXPECTED_INTENTS")
  else
    expected=$(printf '%s\n' docs/specs/intent-a.md docs/specs/intent-b.md)
  fi
  actual=$(cat "$root/.corvint/change.ocm-intents" 2>/dev/null || :)
  [[ $actual == "$expected" ]] || { printf '{"code":"intent-scope-drift","ok":false}\n' >&2; exit 2; }
  ordinal=0
  while IFS= read -r _; do
    ordinal=$((ordinal + 1))
    map=$(printf '%s/.corvint/change.ocm.%03d.json' "$root" "$ordinal")
    [[ $(cat "$map") == '{}' ]] || { printf '{"code":"intent-scope-drift","ok":false}\n' >&2; exit 2; }
  done <<< "$expected"
  printf '%s\n' '{"aggregate":{"coverage":{"linked":0,"total":2,"unknown":2},"state":"ready-for-review"},"mutates":false,"ok":true,"scopeSetSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","scopes":[{"coverage":{"linked":0,"total":1,"unknown":1},"map":".corvint/change.ocm.001.json","mapSha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","path":"docs/specs/intent-a.md","state":"ready-for-review"},{"coverage":{"linked":0,"total":1,"unknown":1},"map":".corvint/change.ocm.002.json","mapSha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","path":"docs/specs/intent-b.md","state":"ready-for-review"}],"testExecution":{"state":"NOT_RUN"},"tool":"dogfood-ocm-status","worklist":[{"claimIds":[],"disposition":"unknown","hunkIds":[],"id":"TEST-A-001","reason":"insufficient-evidence","scope":1,"selector":1},{"claimIds":[],"disposition":"unknown","hunkIds":[],"id":"TEST-B-001","reason":"insufficient-evidence","scope":2,"selector":1}]}'
  exit 0
fi
if [[ $action == dogfood-record ]]; then
  # The real recorder refuses a dirty tree; cem-prepare rewrites the tracked
  # CEM, so the script must record before it prepares.
  if [[ -n $(git -C "$root" status --short --untracked-files=no) ]]; then
    printf '%s\n' '{"code":"record-index-failed","error":"trace recording requires a clean Git tree","ok":false}' >&2
    exit 2
  fi
  if [[ -n ${DOGFOOD_TEST_ADMISSION_FAIL:-} ]]; then
    printf '%s\n' '{"code":"malformed-path","ok":false}' >&2
    exit 2
  fi
  base=
  target=
  shift 3
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --base) base=$2; shift 2 ;;
      --target) target=$2; shift 2 ;;
      *) shift ;;
    esac
  done
  if git -C "$root" diff --name-only "$base" "$target" | rg -q '^script/source[.]sh$'; then
    printf '%s\n' '{"admitted":["script/source.sh"],"admitted_sha256":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","base":"0000000000000000000000000000000000000000","candidate_sha256":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","candidates":[".gitignore","script/source.sh"],"mutates":true,"ok":true,"state":"recorded","target":"1111111111111111111111111111111111111111","tool":"dogfood-record","tree":"2222222222222222222222222222222222222222"}'
  else
    printf '%s\n' '{"admitted":[],"admitted_sha256":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","base":"0000000000000000000000000000000000000000","candidate_sha256":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","candidates":[".gitignore"],"mutates":false,"ok":true,"state":"no-source-paths","target":"1111111111111111111111111111111111111111","tool":"dogfood-record","tree":"2222222222222222222222222222222222222222"}'
  fi
  exit 0
fi
printf '{"ok": true}\n'
EOF
chmod +x "$test_root/bin/corvint"
cat > "$test_root/bin/wrong-corvint" <<'EOF'
#!/usr/bin/env bash
printf 'Corvint 0.4.0a1\n'
EOF
chmod +x "$test_root/bin/wrong-corvint"
cat > "$test_root/bin/disagree-corvint" <<'EOF'
#!/usr/bin/env bash
set -eu
if [[ ${1:-} == --version ]]; then
  printf 'Corvint 0.4.0a4 (build 12)\n'
  exit 0
fi
printf '{"different":true}\n'
EOF
chmod +x "$test_root/bin/disagree-corvint"
mkdir -p "$test_root/go-bin"
cat > "$test_root/go-bin/go" <<'EOF'
#!/usr/bin/env bash
set -eu
printf '%s\n' "$*" >> "$DOGFOOD_TEST_GO_LOG"
[[ $1 == build && $2 == -o ]]
[[ $# == 4 && $4 == ./cmd/corvint || $# == 5 && $4 == -trimpath && $5 == ./cmd/corvint ]]
source=$DOGFOOD_TEST_FAKE_CORVINT
if [[ $3 == *corvint-base && -n ${DOGFOOD_TEST_BASE_CORVINT:-} ]]; then
  source=$DOGFOOD_TEST_BASE_CORVINT
fi
cp "$source" "$3"
EOF
chmod +x "$test_root/go-bin/go"
run_dogfood_check() {
  env PATH="$test_root/go-bin:$PATH" \
    DOGFOOD_TEST_FAKE_CORVINT="$test_root/bin/corvint" \
    DOGFOOD_TEST_BASE_CORVINT="${DOGFOOD_TEST_BASE_CORVINT:-}" \
    DOGFOOD_TEST_GO_LOG="$test_root/go.log" DOGFOOD_TEST_LOG="$test_root/corvint.log" \
    DOGFOOD_TEST_CEM_UNKNOWNS="${DOGFOOD_TEST_CEM_UNKNOWNS:-0}" \
    DOGFOOD_TEST_EXPECTED_INTENTS="${DOGFOOD_TEST_EXPECTED_INTENTS:-}" \
    CORVINT_BIN="${CORVINT_BIN:-$test_root/bin/corvint}" \
    DOGFOOD_TEST_PID_FILE="${DOGFOOD_TEST_PID_FILE:-}" \
    script/dogfood-check.sh "$@"
}
printf '1\tdocs/specs/intent-a.md\t1:1\tspecification\n' > "$test_root/citations.tsv"
: > "$test_root/corvint.log"
wrong_status=0
wrong_output=$(cd "$test_root/repo" && CORVINT_BIN="$test_root/bin/wrong-corvint" \
  script/dogfood-change.sh "$base" 2>&1) || wrong_status=$?
test "$wrong_status" = 2
test "$wrong_output" = 'dogfood-change: REFUSE corvint-version-mismatch expected=0.4.0a4'
evidence=$(git -C "$test_root/repo" rev-parse --absolute-git-dir)/corvint
mkdir -p "$evidence"
printf 'stale impact stderr\n' > "$evidence/prechange-impact.stderr"
git clone -q "$test_root/repo" "$test_root/default-repo"
: > "$test_root/default-corvint.log"
: > "$test_root/default-go.log"
# Each parallel phase owns a process group so interrupted harness cleanup reaches its children.
set -m
(
  cd "$test_root/default-repo"
  default_env=(env PATH="$test_root/go-bin:$PATH" DOGFOOD_TEST_FAKE_CORVINT="$test_root/bin/corvint"
    DOGFOOD_TEST_GO_LOG="$test_root/default-go.log" DOGFOOD_TEST_LOG="$test_root/default-corvint.log")
  "${default_env[@]}" DOGFOOD_CITATIONS="$test_root/citations.tsv" \
    DOGFOOD_INTENTS_FILE="$test_root/intents.txt" DOGFOOD_VERIFY='test gate' \
    DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  built_corvint=$(git rev-parse --absolute-git-dir)/corvint/corvint
  test -x "$built_corvint"
  git -c user.name=t -c user.email=t@example.invalid add .corvint/change.cem.json
  git -c user.name=t -c user.email=t@example.invalid commit -qm cem
  "${default_env[@]}" DOGFOOD_CITATIONS="$test_root/citations.tsv" \
    DOGFOOD_INTENTS_FILE="$test_root/intents.txt" DOGFOOD_VERIFY='test gate' \
    DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  rg -Fq '"anchor": {"state": "NOT_OBSERVED", "mergeBase": null}' .corvint/dogfood-report.json
  "${default_env[@]}" script/dogfood-check.sh "$base"
  DOGFOOD_TEST_IMPACT=unsupported "${default_env[@]}" DOGFOOD_CITATIONS="$test_root/citations.tsv" \
    DOGFOOD_INTENTS_FILE="$test_root/intents.txt" DOGFOOD_VERIFY='test gate' \
    DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  rg -q '"name": "prechange-impact", "status": "NOT_PRODUCED", "reason": "unsupported-impact-range"' .corvint/dogfood-report.json
  rg -q '^  ,"contextAbstentionEvidenceSha256": "sha256:[0-9a-f]{64}"$' .corvint/dogfood-report.json
  DOGFOOD_TEST_IMPACT=unsupported "${default_env[@]}" script/dogfood-check.sh "$base"
  cp "$(git rev-parse --absolute-git-dir)/corvint/prechange-impact.stderr" "$test_root/prechange-impact.stderr"
  printf 'drift\n' >> "$(git rev-parse --absolute-git-dir)/corvint/prechange-impact.stderr"
  context_drift_status=0
  DOGFOOD_TEST_IMPACT=unsupported "${default_env[@]}" script/dogfood-check.sh "$base" >/dev/null 2>&1 || context_drift_status=$?
  test "$context_drift_status" = 1
  cp "$test_root/prechange-impact.stderr" "$(git rev-parse --absolute-git-dir)/corvint/prechange-impact.stderr"
  for impact_failure in malformed invalid-escape raw-tab wrong-exit nonempty crash; do
    impact_status=0
    DOGFOOD_TEST_IMPACT=$impact_failure "${default_env[@]}" DOGFOOD_CITATIONS="$test_root/citations.tsv" \
      DOGFOOD_INTENTS_FILE="$test_root/intents.txt" DOGFOOD_VERIFY='test gate' \
      DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base" >/dev/null 2>&1 || impact_status=$?
    test "$impact_status" != 0
    rg -q '"complete": false' .corvint/dogfood-report.json
    rg -q '"name": "prechange-impact", "status": "NOT_PRODUCED"' .corvint/dogfood-report.json
  done
  DOGFOOD_TEST_IMPACT=unsupported "${default_env[@]}" DOGFOOD_CITATIONS="$test_root/citations.tsv" \
    DOGFOOD_INTENTS_FILE="$test_root/intents.txt" DOGFOOD_VERIFY='test gate' \
    DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  printf 'trailing-junk' >> "$(git rev-parse --absolute-git-dir)/corvint/prechange-impact.argv"
  argv_drift_status=0
  DOGFOOD_TEST_IMPACT=unsupported "${default_env[@]}" script/dogfood-check.sh "$base" >/dev/null 2>&1 || argv_drift_status=$?
  test "$argv_drift_status" = 1
  DOGFOOD_TEST_IMPACT=unsupported "${default_env[@]}" DOGFOOD_CITATIONS="$test_root/citations.tsv" \
    DOGFOOD_INTENTS_FILE="$test_root/intents.txt" DOGFOOD_VERIFY='test gate' \
    DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  git branch dogfood-anchor "$base"
  git config --local corvint.dogfood.anchor refs/heads/dogfood-anchor
  "${default_env[@]}" DOGFOOD_CITATIONS="$test_root/citations.tsv" \
    DOGFOOD_INTENTS_FILE="$test_root/intents.txt" DOGFOOD_VERIFY='test gate' \
    DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  rg -Fq '"anchor": {"state": "OBSERVED", "mergeBase": "'"$base"'"}' .corvint/dogfood-report.json
  "${default_env[@]}" script/dogfood-check.sh "$base"
  anchor_status=0
  anchor_output=$("${default_env[@]}" script/dogfood-check.sh HEAD 2>&1) || anchor_status=$?
  test "$anchor_status" = 2
  test "$anchor_output" = 'dogfood-check: REFUSE base-not-anchored'
  git config --local corvint.dogfood.anchor ''
  empty_anchor_status=0
  empty_anchor_output=$("${default_env[@]}" script/dogfood-check.sh "$base" 2>&1) || empty_anchor_status=$?
  test "$empty_anchor_status" = 2
  test "$empty_anchor_output" = 'dogfood-check: REFUSE anchor-ref-unavailable'
  test "$(rg -c '^build -o .*/corvint/corvint (-trimpath )?./cmd/corvint$' "$test_root/default-go.log")" = 15
  # dogfood-check builds the base verifier inside a random private extraction directory; without
  # -trimpath that absolute path is embedded and baseVerifierSha256 changes on every run.
  test "$(rg -c '^build -o .*/corvint/corvint(-base)? -trimpath ./cmd/corvint$' "$test_root/default-go.log")" = 6
  test "$(rg -c '^build -o .*/corvint/corvint-base ' "$test_root/default-go.log")" = 3
) &
phase_jobs="$phase_jobs $!"
(
  cd "$test_root/repo"
  CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test \
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  rg -q '"complete": true' .corvint/dogfood-report.json
  rg -q '"id":"TEST-A-001"' .corvint/dogfood-report.json
  rg -q '"id":"TEST-B-001"' .corvint/dogfood-report.json
  test "$(git status --short)" = " M .corvint/change.cem.json"
  test "$(rg -c ' cem prepare .* --replace$' "$test_root/corvint.log")" = 1
  rg -q '"name": "cem-prepare", "status": "PRODUCED", "reason": "none"' .corvint/dogfood-report.json
  test "$(cat "$evidence/prechange-impact.stderr")" = 'current impact stderr'
  test "$(rg -c ' ocm prepare ' "$test_root/corvint.log")" = 2
  test "$(rg -c ' ocm status ' "$test_root/corvint.log")" = 2
  test "$(rg -c ' dogfood-observe ' "$test_root/corvint.log")" = "$(rg -c '"name":' .corvint/dogfood-report.json)"
  dirty_check_status=0
  dirty_check_output=$(CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" \
    script/dogfood-check.sh "$base" 2>&1) || dirty_check_status=$?
  test "$dirty_check_status" = 2
  printf '%s\n' "$dirty_check_output" | rg -q '^dogfood-check: REFUSE dirty-worktree$'
  git -c user.name=t -c user.email=t@example.invalid add .corvint/change.cem.json
  git -c user.name=t -c user.email=t@example.invalid commit -qm cem
  # A transient prepare failure is reported, never answered by regenerating the committed map.
  printf '{"cites":[1]}\n' > .corvint/change.cem.json
  git -c user.name=t -c user.email=t@example.invalid commit -qam cited
  replace_count=$(rg -c ' cem prepare .* --replace$' "$test_root/corvint.log")
  transient_status=0
  DOGFOOD_TEST_CEM_PREPARE_CODE=git-timeout CORVINT_BIN="$test_root/bin/corvint" \
    DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base" >/dev/null 2>&1 ||
    transient_status=$?
  test "$transient_status" = 1
  test "$(rg -c ' cem prepare .* --replace$' "$test_root/corvint.log")" = "$replace_count"
  git diff --quiet HEAD -- .corvint/change.cem.json
  rg -q '"name": "cem-prepare", "status": "NOT_PRODUCED", "reason": "git-timeout"' .corvint/dogfood-report.json
  git -c user.name=t -c user.email=t@example.invalid reset -q --hard HEAD~1
  CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test \
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  run_dogfood_check "$base"
  rg -q '"baseVerifierSha256": "sha256:[0-9a-f]{64}"' .corvint/dogfood-report.json
  rg -q '"treeVerifierSha256": "sha256:[0-9a-f]{64}"' .corvint/dogfood-report.json
  disagreement_status=0
  disagreement_output=$(DOGFOOD_TEST_BASE_CORVINT="$test_root/bin/disagree-corvint" \
    run_dogfood_check "$base" 2>&1) || disagreement_status=$?
  test "$disagreement_status" = 1
  test "$disagreement_output" = $'dogfood-check: NOTE unbound-commits NOT_OBSERVED previous-cem-base-unavailable\ndogfood-check: FAIL verifier-disagreement'
  rg -Fq '"outputsAgree": false' .corvint/dogfood-report.json
  wrong_status=0
  wrong_output=$(CORVINT_BIN="$test_root/bin/wrong-corvint" run_dogfood_check "$base" 2>&1) || \
    wrong_status=$?
  test "$wrong_status" = 2
  test "$wrong_output" = $'dogfood-check: NOTE unbound-commits NOT_OBSERVED previous-cem-base-unavailable\ndogfood-check: REFUSE corvint-version-mismatch expected=0.4.0a4'

  no_source_base=$(git rev-parse HEAD)
  printf '\nsecond-local-only\n' >> .gitignore
  git -c user.name=t -c user.email=t@example.invalid add .gitignore .corvint/change.cem.json
  git -c user.name=t -c user.email=t@example.invalid commit -qm non-source
  CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test \
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$no_source_base"
  rg -q '"name": "local-outcome", "status": "NOT_PRODUCED", "reason": "no-source-paths"' .corvint/dogfood-report.json
  rg -q '"complete": true' .corvint/dogfood-report.json
  run_dogfood_check "$no_source_base"

  if CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TEST_ADMISSION_FAIL=1 \
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$no_source_base"; then
    printf 'dogfood-change accepted malformed admission\n' >&2
    exit 1
  fi
  rg -q '"name": "local-outcome", "status": "NOT_PRODUCED", "reason": "malformed-path"' .corvint/dogfood-report.json

  DOGFOOD_OBSERVE_FAIL=1 CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" \
    DOGFOOD_TASK=test DOGFOOD_CITATIONS="$test_root/citations.tsv" \
    DOGFOOD_INTENTS_FILE="$test_root/intents.txt" DOGFOOD_VERIFY='test gate' \
    DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  rg -q '"complete": true' .corvint/dogfood-report.json
  # Both scope maps exist from the earlier pass; each was regenerated with --replace.
  rg -q ' ocm prepare .* --replace$' "$test_root/corvint.log"
  rg -q '"name": "ocm-prepare-002", "status": "PRODUCED", "reason": "none"' .corvint/dogfood-report.json

  printf 'drift\n' >> .corvint/change.ocm.001.json
  if run_dogfood_check "$base"; then
    printf 'dogfood-check accepted drifted map\n' >&2
    exit 1
  fi
  printf '{}\n' > .corvint/change.ocm.001.json

  printf '%s\n' docs/specs/intent-a.md > .corvint/change.ocm-intents
  if run_dogfood_check "$base"; then
    printf 'dogfood-check accepted drifted scope list\n' >&2
    exit 1
  fi
  cp "$test_root/intents.txt" .corvint/change.ocm-intents

  missing_manifest_status=0
  missing_manifest_output=$(DOGFOOD_OBSERVE_FAIL=1 CORVINT_BIN="$test_root/bin/corvint" \
    DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_CITATIONS="$test_root/citations.tsv" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base" 2>&1) || \
    missing_manifest_status=$?
  if [[ $missing_manifest_status == 0 ]]; then
    printf 'dogfood-change accepted missing intent manifest\n' >&2
    exit 1
  fi
  rg -q '"name": "ocm-aggregate", "status": "NOT_PRODUCED", "reason": "missing-intent-scope"' .corvint/dogfood-report.json
  rg -q '"ocmStatus": null' .corvint/dogfood-report.json
  # A non-complete run must say which step failed and why on stderr, not exit
  # silently (regression check for the swallowed-reason bug).
  printf '%s\n' "$missing_manifest_output" | rg -q '^dogfood-change: FAIL not-complete$'
  printf '%s\n' "$missing_manifest_output" | rg -q '^  ocm-aggregate: missing-intent-scope$'
  printf '%s\n' "$missing_manifest_output" | \
    rg -Fxq '    fix: DOGFOOD_INTENTS_FILE must be the path of a sorted, LF-terminated file listing 1-16 repository-relative spec paths'
  printf '%s\n' "$missing_manifest_output" | rg -q '^  full report: .*/\.corvint/dogfood-report\.json$'

  # An authority-start trace-state refusal names the task wording as its subject.
  wording_status=0
  wording_output=$(DOGFOOD_TEST_QUERY=authority-trace-state CORVINT_BIN="$test_root/bin/corvint" \
    DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK='contributor workflow' \
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base" 2>&1) || \
    wording_status=$?
  test "$wording_status" = 1
  printf '%s\n' "$wording_output" | rg -q '^  prechange-query: unsupported-query-trace-state$'
  printf '%s\n' "$wording_output" | \
    rg -q '^  prechange-query: DOGFOOD_TASK wording selected the authority-start profile, '

  # Each refusal a first-time adopter hits names its fix (DCW-V0-014).
  input_status=0
  input_output=$(DOGFOOD_TEST_OCM_PREPARE_CODE=invalid-requirements-section \
    CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test \
    DOGFOOD_CITATIONS=$'1\tAGENTS.md\t1:1\tspecification' DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' script/dogfood-change.sh "$base" 2>&1) || input_status=$?
  test "$input_status" = 1
  printf '%s\n' "$input_output" | rg -Fxq -- '  cem-cite: citation-plan-unavailable'
  printf '%s\n' "$input_output" | \
    rg -Fxq -- '    fix: DOGFOOD_CITATIONS must be the path of a TSV file of ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION rows, not the rows themselves'
  printf '%s\n' "$input_output" | \
    rg -Fxq -- '    fix: intent must be a spec that exists at BASE and contains exactly one "## Requirements" heading'
  printf '%s\n' "$input_output" | rg -Fxq -- '  ocm-aggregate: intent-scope-drift'
  printf '%s\n' "$input_output" | \
    rg -Fxq -- '    fix: fix the ocm-prepare or ocm-status row above; otherwise the intents file changed during the run'
  printf '%s\n' "$input_output" | \
    rg -Fxq -- '    fix: set DOGFOOD_OUTCOME (passed, failed or blocked) and DOGFOOD_VERIFY_FILE (one verification command per line)'
  pending_status=0
  pending_output=$(DOGFOOD_TEST_OCM_PREPARE_CODE=excluded-artifact-mismatch DOGFOOD_TEST_CEM_UNKNOWNS=1 \
    DOGFOOD_TEST_OCM_STATUS_EXIT=1 \
    CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test \
    DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base" 2>&1) || pending_status=$?
  test "$pending_status" = 1
  printf '%s\n' "$pending_output" | \
    rg -Fxq -- '    fix: set DOGFOOD_CITATIONS to the path of a TSV plan with one row per hunk of .corvint/change.cem.json'
  printf '%s\n' "$pending_output" | rg -Fxq -- '  ocm-prepare-001: excluded-artifact-mismatch'
  printf '%s\n' "$pending_output" | \
    rg -Fxq -- '    fix: the worktree has uncommitted changes (often the prepared sidecar); commit them, then rerun make dogfood-change'
  printf '%s\n' "$pending_output" | rg -Fxq -- '  ocm-status-001: not-ready'
  printf '%s\n' "$pending_output" | \
    rg -Fxq -- '    fix: fix the ocm-prepare row with the same number first; if it was produced, the worktree has uncommitted changes (often the prepared sidecar); commit them, then rerun make dogfood-change'
  printf '%s\n' "$pending_output" | rg -Fxq -- '  cem-status: not-ready'
  printf '%s\n' "$pending_output" | \
    rg -q '^    fix: read verification\.issues and policyIssues in .*/cem-status\.json: excluded-artifact-mismatch means the sidecar is uncommitted, max-unknown-exceeded means DOGFOOD_CITATIONS does not cite every hunk$'
  printf '1\tAGENTS.md\n' > "$test_root/malformed-citations.tsv"
  malformed_status=0
  malformed_output=$(CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" \
    DOGFOOD_TASK=test DOGFOOD_CITATIONS="$test_root/malformed-citations.tsv" \
    DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base" 2>&1) || malformed_status=$?
  test "$malformed_status" = 1
  printf '%s\n' "$malformed_output" | \
    rg -Fxq -- '    fix: each row is ORDINAL<TAB>PATH<TAB>START:END<TAB>RELATION in worklist order, LF-terminated, at most 256 rows'
  missing_report_status=0
  rm -f .corvint/dogfood-report.json
  missing_report_output=$(run_dogfood_check "$base" 2>&1) || missing_report_status=$?
  test "$missing_report_status" = 1
  printf '%s\n' "$missing_report_output" | rg -Fxq -- 'dogfood-check: FAIL dogfood-report-missing'
  printf '%s\n' "$missing_report_output" | \
    rg -Fxq -- "  fix: run make dogfood-change BASE=$base on this HEAD until it reports complete"

  unreachable_output=$(DOGFOOD_TEST_QUERY=unreachable-trace CORVINT_BIN="$test_root/bin/corvint" \
    DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test \
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base" 2>&1) || :
  printf '%s\n' "$unreachable_output" | rg -Fxq -- '  prechange-query: unsupported-query-trace-state'
  printf '%s\n' "$unreachable_output" | \
    rg -q '^  local trace store: a recorded trace names a commit no longer reachable from HEAD; '
  if printf '%s\n' "$wording_output" | rg -q '^  local trace store:'; then
    printf 'dogfood-change blamed history for a wording refusal\n' >&2
    exit 1
  fi

  CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test \
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$base"
  cp .corvint/dogfood-report.json "$test_root/complete-report.json"
  sed 's/"complete": true/"complete": false/' "$test_root/complete-report.json" > .corvint/dogfood-report.json
  incomplete_output=$(run_dogfood_check "$base" 2>&1) || :
  printf '%s\n' "$incomplete_output" | rg -Fxq -- 'dogfood-check: FAIL dogfood-report-drift'
  printf '%s\n' "$incomplete_output" | \
    rg -Fxq -- "  fix: the report is not complete; resolve the rows make dogfood-change BASE=$base lists, then rerun it"
  stale_output=$(run_dogfood_check HEAD~1 2>&1) || :
  printf '%s\n' "$stale_output" | rg -Fxq -- 'dogfood-check: FAIL dogfood-report-drift'
  printf '%s\n' "$stale_output" | rg -Fq -- '  fix: the report binds another BASE or HEAD; rerun make dogfood-change BASE='
  cp "$test_root/complete-report.json" .corvint/dogfood-report.json
  cp .corvint/change.ocm-status.json "$test_root/ocm-status.json"
  printf 'drift\n' >> .corvint/change.ocm-status.json
  aggregate_output=$(run_dogfood_check "$base" 2>&1) || :
  printf '%s\n' "$aggregate_output" | rg -Fxq -- 'dogfood-check: FAIL intent-scope-drift'
  printf '%s\n' "$aggregate_output" | \
    rg -Fxq -- "  fix: the OCM maps changed after make dogfood-change; rerun make dogfood-change BASE=$base"
  cp "$test_root/ocm-status.json" .corvint/change.ocm-status.json

  bootstrap_base=$(git rev-parse HEAD)
  # Literal backticks are requirement syntax, not shell interpolation.
  # shellcheck disable=SC2016
  printf '%s\n' '# Bootstrap intent' '' '## Requirements' '' '- `BOOT-V0-001`: test.' > docs/specs/bootstrap.md
  printf '%s\n' docs/specs/bootstrap.md > "$test_root/bootstrap-intents.txt"
  git -c user.name=t -c user.email=t@example.invalid add docs/specs/bootstrap.md
  git -c user.name=t -c user.email=t@example.invalid commit -qm bootstrap
  DOGFOOD_TEST_CEM_UNKNOWNS=1 \
    DOGFOOD_TEST_EXPECTED_INTENTS="$test_root/bootstrap-intents.txt" \
    CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test \
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/bootstrap-intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$bootstrap_base"
  git -c user.name=t -c user.email=t@example.invalid add .corvint/change.cem.json
  git -c user.name=t -c user.email=t@example.invalid commit --allow-empty -qm bootstrap-cem
  DOGFOOD_TEST_CEM_UNKNOWNS=1 \
    DOGFOOD_TEST_EXPECTED_INTENTS="$test_root/bootstrap-intents.txt" \
    CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/corvint.log" DOGFOOD_TASK=test \
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/bootstrap-intents.txt" \
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed script/dogfood-change.sh "$bootstrap_base"
  rg -q '"complete": true' .corvint/dogfood-report.json
  rg -Fq '"dogfoodPolicy": {"bootstrapUnknown": 1, "maximumUnknownAfterBootstrap": 0}' .corvint/dogfood-report.json
  DOGFOOD_TEST_CEM_UNKNOWNS=1 \
    DOGFOOD_TEST_EXPECTED_INTENTS="$test_root/bootstrap-intents.txt" run_dogfood_check "$bootstrap_base"
  extra_unknown_status=0
  DOGFOOD_TEST_CEM_UNKNOWNS=2 \
    DOGFOOD_TEST_EXPECTED_INTENTS="$test_root/bootstrap-intents.txt" \
    run_dogfood_check "$bootstrap_base" >/dev/null 2>&1 || extra_unknown_status=$?
  test "$extra_unknown_status" = 1
  printf '%s\n' "$bootstrap_base" > "$test_root/check-base"
)

# DOGFOOD_VERIFY splits one recorder --verify per non-blank line; DOGFOOD_VERIFY_FILE is the
# file-backed alternative and a missing file is reported rather than silently skipped.
verify_repo="$test_root/verify-repo"
git clone -q "$test_root/repo" "$verify_repo"
: > "$test_root/verify-corvint.log"
printf 'file gate\n\ngo vet ./...\n' > "$test_root/verify.txt"
(
  cd "$verify_repo"
  verify_env=(env CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/verify-corvint.log"
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/intents.txt"
    DOGFOOD_OUTCOME=passed)
  "${verify_env[@]}" DOGFOOD_VERIFY=$'test gate\n\nsecond check' script/dogfood-change.sh "$base"
  rg -qF -- '--verify test gate --verify second check --outcome passed' "$test_root/verify-corvint.log"
  git diff --quiet HEAD -- .corvint/change.cem.json || {
    git -c user.name=t -c user.email=t@example.invalid commit -qm cem .corvint/change.cem.json
  }
  "${verify_env[@]}" DOGFOOD_VERIFY_FILE="$test_root/verify.txt" script/dogfood-change.sh "$base"
  rg -qF -- '--verify file gate --verify go vet ./... --outcome passed' "$test_root/verify-corvint.log"
  "${verify_env[@]}" DOGFOOD_VERIFY_FILE="$test_root/absent.txt" script/dogfood-change.sh "$base" || :
  rg -q '"name": "local-outcome", "status": "NOT_PRODUCED", "reason": "verify-file-unavailable"' .corvint/dogfood-report.json
  rm -f .corvint/dogfood-report.json
  "${verify_env[@]}" DOGFOOD_OUTCOME= DOGFOOD_VERIFY='test gate' script/dogfood-change.sh "$base" || :
  rg -q '"name": "local-outcome", "status": "NOT_PRODUCED", "reason": "outcome-input-not-provided"' .corvint/dogfood-report.json
  rm -f .corvint/dogfood-report.json
  "${verify_env[@]}" DOGFOOD_VERIFY=$'\n \t\n' script/dogfood-change.sh "$base" || :
  rg -q '"name": "local-outcome", "status": "NOT_PRODUCED", "reason": "outcome-input-not-provided"' .corvint/dogfood-report.json
) &
phase_jobs="$phase_jobs $!"

# DOGFOOD_OCM_LINKS applies only the author's explicit rows through ocm link, after each scope's
# prepare and before its status; each row is its own step and refusals name a fix (DCW-V0-018).
links_repo="$test_root/links-repo"
git clone -q "$test_root/repo" "$links_repo"
: > "$test_root/links-corvint.log"
printf 'docs/specs/intent-a.md\tTEST-A-001\t1,2\tscript/a_test.go\ttest:TestA/case:test-a,test:TestB\n' > "$test_root/links.tsv"
printf 'docs/specs/intent-a.md\tTEST-A-%s\t1\tscript/a_test.go\ttest:TestA\n' 001 002 003 > "$test_root/links-partial.tsv"
printf 'docs/specs/intent-b.md\tTEST-B-001\t1\tscript/b_test.go\ttest:TestB\n' >> "$test_root/links-partial.tsv"
mkdir "$test_root/links-invalid"
printf 'docs/specs/intent-c.md\tTEST-C-001\t1\tscript/a_test.go\ttest:TestA\n' > "$test_root/links-invalid/unlisted.tsv"
printf 'docs/specs/intent-a.md\tTEST-A-001\t1\tscript/a_test.go\ttest:TestA\r\n' > "$test_root/links-invalid/crlf.tsv"
printf 'docs/specs/intent-a.md\tTEST-A-001\t1\tscript/a_test.go\n' > "$test_root/links-invalid/fields.tsv"
printf 'docs/specs/intent-a.md\tTEST-A-001\t1,,2\tscript/a_test.go\ttest:TestA\n' > "$test_root/links-invalid/empty-item.tsv"
for _ in $(seq 257); do cat "$test_root/links-invalid/unlisted.tsv"; done |
  sed 's/intent-c/intent-a/' > "$test_root/links-invalid/rows.tsv"
: > "$test_root/links-empty.tsv"
(
  cd "$links_repo"
  links_env=(env CORVINT_BIN="$test_root/bin/corvint" DOGFOOD_TEST_LOG="$test_root/links-corvint.log"
    DOGFOOD_CITATIONS="$test_root/citations.tsv" DOGFOOD_INTENTS_FILE="$test_root/intents.txt"
    DOGFOOD_OUTCOME=passed DOGFOOD_VERIFY='test gate')
  "${links_env[@]}" script/dogfood-change.sh "$base" 2>/dev/null || :
  if rg -q ' ocm link ' "$test_root/links-corvint.log"; then exit 1; fi
  rg -qF '  ,"ocmLinkPlan": null' .corvint/dogfood-report.json
  git diff --quiet HEAD -- .corvint/change.cem.json || {
    git -c user.name=t -c user.email=t@example.invalid commit -qm cem .corvint/change.cem.json
  }
  "${links_env[@]}" DOGFOOD_OCM_LINKS="$test_root/links.tsv" script/dogfood-change.sh "$base"
  rg -q '"complete": true' .corvint/dogfood-report.json
  rg -qF "  ,\"ocmLinkPlan\": {\"sha256\": \"sha256:$(shasum -a 256 "$test_root/links.tsv" | awk '{print $1}')\", \"rows\": 1}" \
    .corvint/dogfood-report.json
  test "$(rg -c ' ocm link ' "$test_root/links-corvint.log")" = 1
  rg -qF -- "ocm link --map .corvint/change.ocm.001.json --cem .corvint/change.cem.json --obligation TEST-A-001 --test-path script/a_test.go --expected-base $base --target $(git rev-parse HEAD) --hunk 1 --hunk 2 --claim test:TestA/case:test-a --claim test:TestB" "$test_root/links-corvint.log"
  awk '/ ocm prepare .*ocm[.]001/ { p = NR } / ocm link / { l = NR } / ocm status --map .corvint\/change.ocm.001/ { s = NR }
    END { exit !(p < l && l < s) }' "$test_root/links-corvint.log"
  rg -q '"name": "ocm-link-001", "status": "PRODUCED", "reason": "none"' .corvint/dogfood-report.json
  if rg -q '"name": "ocm-link-002"' .corvint/dogfood-report.json; then exit 1; fi
  # Refused rows 2 and 4 do not stop rows 3 and 4; row 4 links through the second intent's map.
  : > "$test_root/links-corvint.log"
  links_output=$(DOGFOOD_TEST_OCM_LINK_REFUSE='TEST-A-002|TEST-B-001' "${links_env[@]}" \
    DOGFOOD_OCM_LINKS="$test_root/links-partial.tsv" script/dogfood-change.sh "$base" 2>&1) && exit 1
  test "$(rg -c ' ocm link ' "$test_root/links-corvint.log")" = 4
  rg -q ' ocm link --map .corvint/change.ocm.002.json .* --obligation TEST-B-001 ' "$test_root/links-corvint.log"
  for row in 001:PRODUCED:none 002:NOT_PRODUCED:claim-obligation-mismatch 003:PRODUCED:none \
    004:NOT_PRODUCED:claim-obligation-mismatch; do
    IFS=: read -r number status reason <<< "$row"
    rg -qF "\"name\": \"ocm-link-$number\", \"status\": \"$status\", \"reason\": \"$reason\"" .corvint/dogfood-report.json
  done
  rg -q '"name": "ocm-aggregate", "status": "PRODUCED"' .corvint/dogfood-report.json
  printf '%s\n' "$links_output" | rg -q '^  ocm-link-004: claim-obligation-mismatch$'
  test "$(printf '%s\n' "$links_output" | rg -c '^    fix: read .*/ocm-link-00[24][.]stderr: ')" = 2
  # Validation rejects each malformed plan (the empty list item included) before any link.
  for plan in "$test_root"/links-invalid/*.tsv; do
    links_output=$("${links_env[@]}" DOGFOOD_OCM_LINKS="$plan" script/dogfood-change.sh "$base" 2>&1) && exit 1
    printf '%s\n' "$links_output" | rg -q '^  ocm-links: invalid-ocm-link-plan$'
  done
  links_output=$("${links_env[@]}" DOGFOOD_OCM_LINKS="$test_root/links-empty.tsv" \
    script/dogfood-change.sh "$base" 2>&1) && exit 1
  printf '%s\n' "$links_output" | rg -q '^  ocm-links: empty-ocm-link-plan$'
  printf '%s\n' "$links_output" | rg -q '^    fix: DOGFOOD_OCM_LINKS names an empty file'
  test "$(rg -c ' ocm link ' "$test_root/links-corvint.log")" = 4
  links_output=$("${links_env[@]}" DOGFOOD_OCM_LINKS="$test_root/links-absent.tsv" \
    script/dogfood-change.sh "$base" 2>&1) && exit 1
  printf '%s\n' "$links_output" | rg -q '^  ocm-links: ocm-link-plan-unavailable$'
  printf '%s\n' "$links_output" | rg -q '^    fix: DOGFOOD_OCM_LINKS must be the path of a TSV file'
) &
phase_jobs="$phase_jobs $!"

# dogfood-check reports commits after the base's committed CEM base that no committed CEM binds,
# stays silent when every such commit is bound, and abstains when the base has no CEM.
(
unbound_repo="$test_root/unbound-repo"
mkdir -p "$unbound_repo/script" "$unbound_repo/.corvint"
cp "$source_root/script/dogfood-check.sh" "$unbound_repo/script/"
unbound_commit() {
  printf '%s\n' "$1" >> "$unbound_repo/work.txt"
  git -C "$unbound_repo" -c user.name=t -c user.email=t@example.invalid add -A
  git -C "$unbound_repo" -c user.name=t -c user.email=t@example.invalid commit -qm "$1"
  git -C "$unbound_repo" rev-parse HEAD
}
git -C "$unbound_repo" init -q -b main
unbound_b0=$(unbound_commit b0)
printf '{\n  "baseRevision": "%s",\n  "spec": "cem/0.2"\n}\n' "$unbound_b0" > "$unbound_repo/.corvint/change.cem.json"
unbound_s1=$(unbound_commit s1)
unbound_c1=$(unbound_commit c1)
unbound_c2=$(unbound_commit c2)
unbound_commit c3 >/dev/null
unbound_gap=$(cd "$unbound_repo" && script/dogfood-check.sh "$unbound_c2" 2>&1) || :
printf '%s\n' "$unbound_gap" | rg -q "^dogfood-check: NOTE unbound-commits count=2 window=$unbound_b0\\.\\.$unbound_c2\$"
test "$(printf '%s\n' "$unbound_gap" | rg '^  unbound ')" = "$(printf '  unbound %s\n' "$unbound_c2" "$unbound_c1")"
unbound_none=$(cd "$unbound_repo" && script/dogfood-check.sh "$unbound_s1" 2>&1) || :
if printf '%s\n' "$unbound_none" | rg -q 'unbound'; then exit 1; fi
unbound_absent=$(cd "$unbound_repo" && script/dogfood-check.sh "$unbound_b0" 2>&1) || :
printf '%s\n' "$unbound_absent" | rg -q '^dogfood-check: NOTE unbound-commits NOT_OBSERVED previous-cem-absent$'

# DOGFOOD-013/014: a seal commit is covered by its bind commit, a sealed HEAD is
# refused, and the newest seal names the previous binding when BASE has no CEM.
sealed_repo="$test_root/sealed-repo"
mkdir -p "$sealed_repo/script" "$sealed_repo/.corvint"
cp "$source_root/script/dogfood-check.sh" "$source_root/script/dogfood-seal.sh" "$sealed_repo/script/"
sealed_commit() {
  printf '%s\n' "$1" >> "$sealed_repo/work.txt"
  git -C "$sealed_repo" -c user.name=t -c user.email=t@example.invalid add -A
  git -C "$sealed_repo" -c user.name=t -c user.email=t@example.invalid commit -qm "$1"
  git -C "$sealed_repo" rev-parse HEAD
}
git -C "$sealed_repo" init -q -b main
sealed_b0=$(sealed_commit b0)
printf '{\n  "baseRevision": "%s",\n  "spec": "cem/0.2"\n}\n' "$sealed_b0" > "$sealed_repo/.corvint/change.cem.json"
sealed_s1=$(sealed_commit s1)
mkdir -p "$sealed_repo/.corvint/changes"
git -C "$sealed_repo" mv .corvint/change.cem.json ".corvint/changes/$sealed_s1.cem.json"
git -C "$sealed_repo" -c user.name=t -c user.email=t@example.invalid commit -qm seal
sealed_z1=$(git -C "$sealed_repo" rev-parse HEAD)
sealed_head_status=0
sealed_head=$(cd "$sealed_repo" && script/dogfood-check.sh "$sealed_b0" 2>&1) || sealed_head_status=$?
test "$sealed_head_status" = 2
printf '%s\n' "$sealed_head" | rg -q '^dogfood-check: REFUSE sealed-head$'
sealed_c1=$(sealed_commit c1)
sealed_c2=$(sealed_commit c2)
sealed_commit c3 >/dev/null
sealed_gap=$(cd "$sealed_repo" && script/dogfood-check.sh "$sealed_c2" 2>&1) || :
printf '%s\n' "$sealed_gap" | rg -q "^dogfood-check: NOTE unbound-commits count=2 window=$sealed_b0\\.\\.$sealed_c2\$"
test "$(printf '%s\n' "$sealed_gap" | rg '^  unbound ')" = "$(printf '  unbound %s\n' "$sealed_c2" "$sealed_c1")"
sealed_none=$(cd "$sealed_repo" && script/dogfood-check.sh "$sealed_z1" 2>&1) || :
if printf '%s\n' "$sealed_none" | rg -q 'unbound'; then exit 1; fi
# A rename of a bound CEM to any other name is not a seal and is not covered.
printf '{\n  "baseRevision": "%s",\n  "spec": "cem/0.2"\n}\n' "$sealed_c2" > "$sealed_repo/.corvint/change.cem.json"
sealed_commit s2 >/dev/null
git -C "$sealed_repo" mv .corvint/change.cem.json .corvint/changes/other.cem.json
git -C "$sealed_repo" -c user.name=t -c user.email=t@example.invalid commit -qm misnamed
sealed_c4=$(sealed_commit c4)
sealed_commit c5 >/dev/null
sealed_other=$(cd "$sealed_repo" && script/dogfood-check.sh "$sealed_c4" 2>&1) || :
printf '%s\n' "$sealed_other" | rg -q '^dogfood-check: NOTE unbound-commits NOT_OBSERVED window-cem-base-unavailable$'

# dogfood-seal commits only after the check passes, as one exact rename.
seal_repo="$test_root/seal-repo"
mkdir -p "$seal_repo/script" "$seal_repo/.corvint"
cp "$source_root/script/dogfood-seal.sh" "$seal_repo/script/"
printf '#!/usr/bin/env bash\nexit "${SEAL_TEST_CHECK_STATUS:-0}"\n' > "$seal_repo/script/dogfood-check.sh"
chmod +x "$seal_repo/script/dogfood-check.sh"
git -C "$seal_repo" init -q -b main
printf '{}\n' > "$seal_repo/.corvint/change.cem.json"
git -C "$seal_repo" add -A
git -C "$seal_repo" -c user.name=t -c user.email=t@example.invalid commit -qm bind
seal_bind=$(git -C "$seal_repo" rev-parse HEAD)
seal_fail_status=0
(cd "$seal_repo" && SEAL_TEST_CHECK_STATUS=1 script/dogfood-seal.sh HEAD) || seal_fail_status=$?
test "$seal_fail_status" = 1
test "$(git -C "$seal_repo" rev-parse HEAD)" = "$seal_bind"
seal_output=$(cd "$seal_repo" && GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid \
  GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid script/dogfood-seal.sh HEAD)
test "$seal_output" = "dogfood-seal: PASS sealed=.corvint/changes/$seal_bind.cem.json"
test "$(git -C "$seal_repo" diff-tree -r -M --no-commit-id --name-status HEAD^ HEAD)" = \
  "R100"$'\t'".corvint/change.cem.json"$'\t'".corvint/changes/$seal_bind.cem.json"
seal_again_status=0
(cd "$seal_repo" && script/dogfood-seal.sh HEAD 2>/dev/null) || seal_again_status=$?
test "$seal_again_status" = 2
) &
phase_jobs="$phase_jobs $!"

# Coordinator transaction regressions use a narrow fake public cite. Native
# map-byte/publisher parity is independently exercised by the real CLI probe.
citation_repo="$test_root/citation-repo"
git clone -q "$test_root/repo" "$citation_repo"
citation_evidence=$(git -C "$citation_repo" rev-parse --absolute-git-dir)/corvint
citation_artifacts=${DOGFOOD_TEST_ARTIFACTS:-$test_root/citation-artifacts}
mkdir -p "$citation_artifacts" "$test_root/collision-bin"
real_mktemp=$(command -v mktemp)
cat > "$test_root/collision-bin/mktemp" <<'EOF_COLLISION'
#!/usr/bin/env bash
set -eu
result=$("$DOGFOOD_TEST_REAL_MKTEMP" "$@")
if [[ -n ${DOGFOOD_TEST_COLLISION:-} && $* == *dogfood-change.XXXXXX* ]]; then
  path="$DOGFOOD_TEST_COLLISION_ROOT/.corvint/.cem-citations.${result##*/}.json"
  case "$DOGFOOD_TEST_COLLISION" in
    file) printf 'existing stage\n' > "$path" ;;
    directory) mkdir "$path" ;;
    fifo) mkfifo "$path" ;;
    symlink) ln -s "$DOGFOOD_TEST_SENTINEL" "$path" ;;
    dangling) ln -s "$DOGFOOD_TEST_SENTINEL.missing" "$path" ;;
    *) exit 99 ;;
  esac
  printf '%s\n' "$path" > "$DOGFOOD_TEST_COLLISION_RECORD"
fi
printf '%s\n' "$result"
EOF_COLLISION
chmod +x "$test_root/collision-bin/mktemp"
printf 'outside sentinel\n' > "$citation_artifacts/sentinel"
printf '{}\n' > "$citation_artifacts/prepared.json"
printf '1\tdocs/specs/intent-a.md\t1:1\tspecification\n' > "$citation_artifacts/one.tsv"
printf '2\tdocs/specs/intent-b.md\t1:1\tspecification\n' > "$citation_artifacts/second.tsv"
cat "$citation_artifacts/one.tsv" "$citation_artifacts/second.tsv" > "$citation_artifacts/multi.tsv"
cat "$citation_artifacts/multi.tsv" "$citation_artifacts/one.tsv" > "$citation_artifacts/duplicate.tsv"
: > "$citation_artifacts/empty.tsv"

assert_no_citation_stage() {
  local stage
  for stage in "$citation_repo"/.corvint/.cem-citations.*.json; do
    if [[ -e $stage || -L $stage ]]; then
      printf 'citation stage survived cleanup: %s\n' "$stage" >&2
      exit 1
    fi
  done
}

run_citation_case() {
  local name=$1 plan=$2 expected=$3 count=$4 pid stage
  citation_case="$citation_artifacts/$name"
  mkdir "$citation_case"
  git -C "$citation_repo" checkout -- .corvint/change.cem.json
  rm -f "$citation_repo/.corvint/dogfood-report.json"
  : > "$citation_case/cites.tsv"
  citation_env=(env PATH="$test_root/collision-bin:$PATH" DOGFOOD_TEST_REAL_MKTEMP="$real_mktemp"
    DOGFOOD_TEST_COLLISION="${citation_collision:-}" DOGFOOD_TEST_COLLISION_ROOT="$citation_repo"
    DOGFOOD_TEST_ALLOW_STAGE_COLLISION="${citation_collision:-}"
    DOGFOOD_TEST_COLLISION_RECORD="$citation_case/collision-path"
    DOGFOOD_TEST_SENTINEL="$citation_artifacts/sentinel"
    DOGFOOD_TEST_UNSAFE_MAP="${citation_unsafe_map:-}"
    DOGFOOD_TEST_MUTATE_PLAN="${citation_mutate_plan:-}"
    DOGFOOD_TEST_CITE_PAUSE="${citation_pause:-}" DOGFOOD_TEST_PID_FILE="$citation_case/child.pid"
    DOGFOOD_TEST_CITES=1 DOGFOOD_TEST_CITE_LOG="$citation_case/cites.tsv"
    DOGFOOD_TEST_LOG="$citation_case/corvint.log" CORVINT_BIN="$test_root/bin/corvint"
    DOGFOOD_CITATIONS="$plan" DOGFOOD_INTENTS_FILE="$test_root/intents.txt"
    DOGFOOD_VERIFY='test gate' DOGFOOD_OUTCOME=passed)
  citation_status=0
  if [[ -n ${citation_pause:-} ]]; then
    active_pid_file="$citation_case/child.pid"
    (
      cd "$citation_repo"
      exec "${citation_env[@]}" script/dogfood-change.sh "$base"
    ) > "$citation_case/stdout" 2> "$citation_case/stderr" &
    active_runner=$!
    poll_for_file "$citation_case/child.pid"
    pid=$(cat "$citation_case/child.pid")
    active_descendant=$pid
    kill -0 "$pid"
    stage=$(awk -F '\t' 'NR == 1 { print $2 }' "$citation_case/cites.tsv")
    test -f "$citation_repo/$stage"
    test "$(find "$citation_repo/$stage" -prune -perm 0600)" = "$citation_repo/$stage"
    cp "$citation_repo/$stage" "$citation_case/interrupted-stage.json"
    cmp "$citation_artifacts/prepared.json" "$citation_repo/.corvint/change.cem.json"
    kill -TERM "$active_runner"
    wait "$active_runner" || citation_status=$?
    active_runner=
    if poll_for_exit "$pid"; then
      pid=
      active_descendant=
      active_pid_file=
    fi
    test -z "$pid"
  else
    (cd "$citation_repo" && "${citation_env[@]}" script/dogfood-change.sh "$base") \
      > "$citation_case/stdout" 2> "$citation_case/stderr" || citation_status=$?
  fi
  test "$citation_status" = "$expected"
  test "$(wc -l < "$citation_case/cites.tsv" | tr -d '[:space:]')" = "$count"
  cp "$citation_repo/.corvint/change.cem.json" "$citation_case/final.json"
  cp "$citation_evidence/cem-cite.jsonl" "$citation_case/receipts.jsonl"
  test "$(find "$citation_evidence/cem-cite.jsonl" -prune -perm 0600)" = "$citation_evidence/cem-cite.jsonl"
  if [[ -f $citation_repo/.corvint/dogfood-report.json ]]; then
    cp "$citation_repo/.corvint/dogfood-report.json" "$citation_case/report.json"
  fi
  if [[ -z ${citation_collision:-} ]]; then assert_no_citation_stage; fi
}

# A background case group runs in its own clone of the state the serial cases start from.
use_citation_clone() {
  citation_repo="$test_root/$1"
  git clone -q "$test_root/repo" "$citation_repo"
  citation_evidence=$(git -C "$citation_repo" rev-parse --absolute-git-dir)/corvint
  cp "$citation_artifacts/direct-expected.json" "$citation_repo/.corvint/direct.cem.json"
}

# Freeze independent direct-call fake bytes before exercising coordinator output.
printf '{}\n' > "$citation_repo/.corvint/direct.cem.json"
while IFS=$'\t' read -r hunk path lines relation; do
  DOGFOOD_TEST_CITES=1 DOGFOOD_TEST_CITE_LOG="$citation_artifacts/direct-cites.tsv" \
    DOGFOOD_TEST_LOG="$citation_artifacts/direct-corvint.log" "$test_root/bin/corvint" \
    --root "$citation_repo" cem cite --map .corvint/direct.cem.json --hunk "$hunk" \
    --evidence-path "$path" --lines "$lines" --relation "$relation" >> "$citation_artifacts/direct-stdout"
done < "$citation_artifacts/multi.tsv"
cp "$citation_repo/.corvint/direct.cem.json" "$citation_artifacts/direct-expected.json"

run_citation_case one "$citation_artifacts/one.tsv" 0 1
rg -q '^\.corvint/change.cem.json\t\.corvint/change.cem.json\t' "$citation_case/cites.tsv"
run_citation_case empty "$citation_artifacts/empty.tsv" 0 0
cmp "$citation_artifacts/prepared.json" "$citation_case/final.json"
rg -q '"name": "cem-cite", "status": "PRODUCED", "reason": "none"' "$citation_case/report.json"

cat "$citation_artifacts/one.tsv" > "$citation_artifacts/unstable.tsv"
printf '2\tdocs/specs/intent-b.md\tunstable\tspecification\n' >> "$citation_artifacts/unstable.tsv"
run_citation_case unstable-second "$citation_artifacts/unstable.tsv" 1 2
cmp "$citation_artifacts/prepared.json" "$citation_case/final.json"
rg -q '"name": "cem-cite", "status": "NOT_PRODUCED", "reason": "cite-span-not-stable"' "$citation_case/report.json"
rg -q '"map":".corvint/.cem-citations.' "$citation_case/receipts.jsonl"
test "$(wc -l < "$citation_case/receipts.jsonl" | tr -d '[:space:]')" = 1

(
use_citation_clone citation-invalid-repo
for kind in extra leading-empty middle-empty trailing-empty nul cr del missing-lf blank; do
  plan="$citation_artifacts/invalid-$kind.tsv"
  cat "$citation_artifacts/one.tsv" > "$plan"
  case "$kind" in
    extra) printf '2\tpath\t1:1\tspecification\t\n' >> "$plan" ;;
    leading-empty) printf '\tpath\t1:1\tspecification\n' >> "$plan" ;;
    middle-empty) printf '2\t\t1:1\tspecification\n' >> "$plan" ;;
    trailing-empty) printf '2\tpath\t1:1\t\n' >> "$plan" ;;
    nul) printf '2\tpath\t1:\0001\tspecification\n' >> "$plan" ;;
    cr) printf '2\tpath\t1:1\tspecification\r\n' >> "$plan" ;;
    del) printf '2\tpa\177th\t1:1\tspecification\n' >> "$plan" ;;
    missing-lf) printf '2\tpath\t1:1\tspecification' >> "$plan" ;;
    blank) printf '\n' >> "$plan" ;;
  esac
  run_citation_case "invalid-$kind" "$plan" 1 0
  cmp "$citation_artifacts/prepared.json" "$citation_case/final.json"
  rg -q '"name": "cem-cite", "status": "NOT_PRODUCED", "reason": "invalid-citation-plan"' "$citation_case/report.json"
done
) &
phase_jobs="$phase_jobs $!"

run_citation_case corrected "$citation_artifacts/multi.tsv" 0 2
cmp "$citation_artifacts/direct-expected.json" "$citation_case/final.json"
awk -F '\t' 'NR == 1 { if ($1 != ".corvint/change.cem.json" || $2 == $1) exit 1; stage=$2 }
  NR == 2 { if ($1 != stage || $2 != ".corvint/change.cem.json") exit 1 }' "$citation_case/cites.tsv"
run_citation_case reapplied "$citation_artifacts/multi.tsv" 0 2
cmp "$citation_artifacts/direct-expected.json" "$citation_case/final.json"
run_citation_case duplicate "$citation_artifacts/duplicate.tsv" 0 3
cmp "$citation_artifacts/direct-expected.json" "$citation_case/final.json"
awk -F '\t' 'NR == 1 { stage=$2 } NR == 2 { if ($1 != stage || $2 != stage) exit 1 }
  NR == 3 { if ($1 != stage || $2 != ".corvint/change.cem.json") exit 1 }' "$citation_case/cites.tsv"

cp "$citation_artifacts/multi.tsv" "$citation_artifacts/mutable.tsv"
citation_mutate_plan="$citation_artifacts/mutable.tsv"
run_citation_case frozen-plan "$citation_mutate_plan" 0 2
citation_mutate_plan=
cmp "$citation_artifacts/direct-expected.json" "$citation_case/final.json"
test "$(cat "$citation_artifacts/mutable.tsv")" = malformed

(
use_citation_clone citation-bound-repo
awk 'BEGIN { for (i=0; i<256; i++) print "1\tdocs/specs/intent-a.md\t1:1\tspecification" }' > "$citation_artifacts/256.tsv"
run_citation_case at-row-bound "$citation_artifacts/256.tsv" 0 256
cmp "$citation_artifacts/one/final.json" "$citation_case/final.json"
cat "$citation_artifacts/256.tsv" "$citation_artifacts/one.tsv" > "$citation_artifacts/257.tsv"
run_citation_case over-row-bound "$citation_artifacts/257.tsv" 1 0
cmp "$citation_artifacts/prepared.json" "$citation_case/final.json"
# Exactly 4 MiB across 256 bounded argv rows; then a shape-valid byte overflow.
awk 'BEGIN { prefix="1\t"; suffix="\t1:1\tspecification\n";
  n=16384-length(prefix)-length(suffix); path=""; while (length(path) < n) path=path "x";
  for (i=0; i<256; i++) printf "%s%s%s",prefix,path,suffix }' > "$citation_artifacts/4mib.tsv"
test "$(wc -c < "$citation_artifacts/4mib.tsv" | tr -d '[:space:]')" = 4194304
run_citation_case at-byte-bound "$citation_artifacts/4mib.tsv" 0 256
cmp "$citation_artifacts/one/final.json" "$citation_case/final.json"
{ printf 'x'; cat "$citation_artifacts/4mib.tsv"; } > "$citation_artifacts/over-4mib.tsv"
run_citation_case over-byte-bound "$citation_artifacts/over-4mib.tsv" 1 0
cmp "$citation_artifacts/prepared.json" "$citation_case/final.json"
) &
phase_jobs="$phase_jobs $!"

for citation_collision in file directory fifo symlink dangling; do
  run_citation_case "collision-$citation_collision" "$citation_artifacts/multi.tsv" 1 0
  path=$(cat "$citation_case/collision-path")
  test -e "$path" || test -L "$path"
  cmp "$citation_artifacts/prepared.json" "$citation_case/final.json"
  rg -q '"reason": "citation-stage-exists"' "$citation_case/report.json"
  test "$(cat "$citation_artifacts/sentinel")" = 'outside sentinel'
  rm -rf "$path"
done
citation_collision=
citation_unsafe_map="$citation_artifacts/sentinel"
run_citation_case unsafe-native-map "$citation_artifacts/multi.tsv" 1 1
citation_unsafe_map=
rg -q '"reason": "publish-failed"' "$citation_case/report.json"
test "$(cat "$citation_artifacts/sentinel")" = 'outside sentinel'
rm "$citation_repo/.corvint/change.cem.json"
git -C "$citation_repo" checkout -- .corvint/change.cem.json

citation_pause=2
run_citation_case interrupted-stage "$citation_artifacts/duplicate.tsv" 143 2
citation_pause=
cmp "$citation_artifacts/prepared.json" "$citation_case/final.json"
for job in $phase_jobs; do
  wait "$job"
done
phase_jobs=
set +m
printf 'dogfood citation transaction regressions: PASS\n'

check_base=$(cat "$test_root/check-base")

cat > "$test_root/bin/slow-check-corvint" <<'EOF'
#!/usr/bin/env bash
set -u
if [[ ${1:-} == --version ]]; then
  printf 'Corvint 0.4.0a4 (build 12)\n'
  exit 0
fi
if [[ $3 == cem ]]; then
  printf '{"ok": true}\n'
  exit 0
fi
sleep 30 &
descendant=$!
printf '%s\n' "$descendant" > "$DOGFOOD_TEST_PID_FILE"
trap 'kill -TERM "$descendant" 2>/dev/null || :; wait "$descendant" 2>/dev/null || :; exit 143' TERM INT HUP
wait "$descendant"
EOF
chmod +x "$test_root/bin/slow-check-corvint"
active_pid_file="$test_root/check-descendant.pid"
(
  cd "$test_root/repo"
  exec env DOGFOOD_TEST_PID_FILE="$test_root/check-descendant.pid" \
    PATH="$test_root/go-bin:$PATH" DOGFOOD_TEST_FAKE_CORVINT="$test_root/bin/slow-check-corvint" \
    DOGFOOD_TEST_BASE_CORVINT="$test_root/bin/slow-check-corvint" \
    DOGFOOD_TEST_GO_LOG="$test_root/go.log" DOGFOOD_TEST_LOG="$test_root/corvint.log" \
    CORVINT_BIN="$test_root/bin/slow-check-corvint" script/dogfood-check.sh "$check_base"
) &
check_runner=$!
active_runner=$check_runner
poll_for_file "$test_root/check-descendant.pid"
check_descendant=$(cat "$test_root/check-descendant.pid")
active_descendant=$check_descendant
kill -TERM "$check_runner"
wait "$check_runner" 2>/dev/null || :
active_runner=
if poll_for_exit "$check_descendant"; then
  check_descendant=
  active_descendant=
  active_pid_file=
fi
if [[ -n $check_descendant ]]; then
  printf 'dogfood-check descendant %s survived interruption\n' "$check_descendant" >&2
  exit 1
fi

cat > "$test_root/bin/slow-corvint" <<'EOF'
#!/usr/bin/env bash
set -u
if [[ ${1:-} == --version ]]; then
  printf 'Corvint 0.4.0a4 (build 12)\n'
  exit 0
fi
sleep 30 &
descendant=$!
printf '%s\n' "$descendant" > "$DOGFOOD_TEST_PID_FILE"
trap 'kill -TERM "$descendant" 2>/dev/null || :; wait "$descendant" 2>/dev/null || :; exit 143' TERM INT HUP
wait "$descendant"
EOF
chmod +x "$test_root/bin/slow-corvint"
active_pid_file="$test_root/descendant.pid"
(
  cd "$test_root/repo"
  exec env DOGFOOD_TEST_PID_FILE="$test_root/descendant.pid" \
    DOGFOOD_TEST_LOG="$test_root/corvint.log" \
    CORVINT_BIN="$test_root/bin/slow-corvint" script/dogfood-change.sh "$base"
) &
runner=$!
active_runner=$runner
poll_for_file "$test_root/descendant.pid"
descendant=$(cat "$test_root/descendant.pid")
active_descendant=$descendant
kill -TERM "$runner"
wait "$runner" 2>/dev/null || :
active_runner=
if poll_for_exit "$descendant"; then
  active_descendant=
  active_pid_file=
  exit 0
fi
printf 'descendant %s survived interruption\n' "$descendant" >&2
exit 1
