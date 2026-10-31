#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/qualify-tasks-basic.sh OUT_DIR [--tasks-binary PATH] [--suite-run FILE]
#
# Qualifies the basic Corvint Tasks external-agent lifecycle (TBS-V1-451..460) on the compiled
# current source: it builds corvint-tasks from this checkout, runs the CAL-V0-019 suite, then on an
# isolated disposable NON-fixture queue in OUT_DIR/repo it runs ticket creation, plan preview,
# claim, renew, submit, a real command gate, Git integration, native completion, readback and
# receipt audit, plus the named refusal and recovery cases. It never touches another queue.
#
# Every case has a declared expected outcome and code set. Each case appends one JSON line to
# OUT_DIR/results.jsonl with its argv, exit status, observed outcome and codes, the expectation
# and pass:true|false. A mismatch is never counted passed: the run prints FAIL lines and exits 1.
# The first two cases prove reachability: the queue reads back fixture:false and a claim before the
# execution cutover refuses CUTOVER_MISSING.
#
#   --tasks-binary PATH  use an already built corvint-tasks instead of `go build` (identity is still
#                        recorded: its sha256 and `version` output); the summary names it SUPPLIED.
#   --suite-run FILE     reuse a retained `go test -json` CAL-V0-019 run instead of running it. The
#                        file must name every suite test as passed; the summary names it SUPPLIED,
#                        and only a self-run suite with a self-built binary prints QUALIFIED.
#   QUALIFY_TASKS_BASIC_TAMPER=CASE  test-only injection: replaces CASE's expected code with an
#                        impossible one, proving a mismatch fails the run (script/qualify-tasks-basic_test.sh).
#
# OUT_DIR must be absent or empty (exit 2 otherwise). Exit 0 when every case passed, 1 otherwise.
# Last line: `QUALIFIED cases=N` (binary and suite both self-produced), `LIFECYCLE_PASSED cases=N
# binary=B suite=S` (some input SUPPLIED), or `FAILED cases=N failed=M`.
set -euo pipefail

usage() { echo "usage: script/qualify-tasks-basic.sh OUT_DIR [--tasks-binary PATH] [--suite-run FILE]" >&2; exit 2; }
[[ $# -ge 1 && -n $1 ]] || usage
out=$1; shift
tasks_bin=""; suite_run=""
while [[ $# -gt 0 ]]; do
  case $1 in
    --tasks-binary) [[ $# -ge 2 ]] || usage; tasks_bin=$2; shift 2 ;;
    --suite-run) [[ $# -ge 2 ]] || usage; suite_run=$2; shift 2 ;;
    *) usage ;;
  esac
done
if [[ -e $out && ( ! -d $out || -n $(ls -A "$out") ) ]]; then
  echo "refused: $out exists and is not an empty directory" >&2
  exit 2
fi
source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
templates=$source_root/internal/tasks/cli/testdata/external-agents
mkdir -p "$out"
out=$(CDPATH='' cd -- "$out" && pwd -P)
results=$out/results.jsonl
: >"$results"

sha256() { shasum -a 256 "$1" | cut -d' ' -f1; }

# Identity: source, toolchain, binary.
src_commit=$(git -C "$source_root" rev-parse HEAD)
src_tree=$(git -C "$source_root" rev-parse 'HEAD^{tree}')
src_dirty=false
[[ -z $(git -C "$source_root" status --porcelain) ]] || src_dirty=true
go_version=$(cd "$source_root" && GOTOOLCHAIN=local go env GOVERSION)
binary_source=BUILT
if [[ -n $tasks_bin ]]; then
  binary_source=SUPPLIED
  mkdir -p "$out/bin"; cp "$tasks_bin" "$out/bin/corvint-tasks"
else
  (cd "$source_root" && GOTOOLCHAIN=local go build -o "$out/bin/corvint-tasks" ./cmd/corvint-tasks)
fi
tasks=$out/bin/corvint-tasks
"$tasks" version >"$out/version.json"

# The CAL-V0-019 suite.
suite_source=RUN
if [[ -n $suite_run ]]; then
  suite_source=SUPPLIED
  cp "$suite_run" "$out/cal019.jsonl"
else
  suite_status=0
  (cd "$source_root" && GOTOOLCHAIN=local go test -json -count=1 -timeout 30m \
    -run '^TestCALV0019_' ./internal/tasks/store) >"$out/cal019.jsonl" || suite_status=$?
  if [[ $suite_status -ne 0 ]]; then
    echo "FAIL case=cal019-suite: go test exited $suite_status; run retained at $out/cal019.jsonl" >&2
    echo "FAILED cases=0 failed=1"
    exit 1
  fi
fi
suite_names=$(sed -n '/^var QualificationSuite = \[\]string{/,/^}/s/^[[:space:]]*"\(TestCALV0019_[A-Za-z]*\)",$/\1/p' \
  "$source_root/internal/tasks/transaction/qualification.go")
[[ -n $suite_names ]] || { echo "refused: QualificationSuite not found in transaction/qualification.go" >&2; exit 1; }
for name in $suite_names; do
  grep -q "\"Action\":\"pass\",\"Package\":\"github.com/Beamfall/corvint/internal/tasks/store\",\"Test\":\"$name\"" "$out/cal019.jsonl" ||
    { echo "FAIL case=cal019-suite: $name has no pass event in $out/cal019.jsonl" >&2; echo "FAILED cases=0 failed=1"; exit 1; }
done
if grep -q '"Action":"fail"' "$out/cal019.jsonl"; then
  echo "FAIL case=cal019-suite: the run has a fail event" >&2; echo "FAILED cases=0 failed=1"; exit 1
fi

jq -cnS --arg commit "$src_commit" --arg tree "$src_tree" --arg dirty "$src_dirty" --arg go "$go_version" \
  --arg bsrc "$binary_source" --arg bsha "$(sha256 "$tasks")" --slurpfile ver "$out/version.json" \
  --arg ssrc "$suite_source" --arg ssha "$(sha256 "$out/cal019.jsonl")" --arg tests "$suite_names" \
  --arg os "$(uname -s)" --arg arch "$(uname -m)" \
  '{profile:"tasks-basic-qualification-identity/0",sourceCommit:$commit,sourceTree:$tree,sourceDirty:($dirty=="true"),
    goVersion:$go,binary:{source:$bsrc,sha256:$bsha,version:($ver[0].items[0]|del(.formats))},
    suite:{source:$ssrc,sha256:$ssha,tests:($tests|[splits("[[:space:]]+")|select(.!="")])},host:{os:$os,arch:$arch}}' \
  >"$out/identity.json"

# A disposable non-fixture queue from the maintained external-agent templates. The required gate
# is a real command: it passes exactly when the candidate tree holds a/PASS.
repo=$out/repo
mkdir -p "$repo/.taskman"
sed -e 's/"prefix":"APP"/"prefix":"QB"/' -e 's/"queueId":"queue:example:main"/"queueId":"queue:qual:main"/' \
  -e 's/"repositoryAuthorityId":"repo:example"/"repositoryAuthorityId":"repo:qual"/' \
  "$templates/queue.json" >"$repo/.taskman/queue.json"
sed -e 's/"argv":\["make","verify"\]/"argv":["test","-f","a\/PASS"]/' "$templates/policy.json" >"$repo/.taskman/policy.json"
grep -q '"fixture":false' "$repo/.taskman/queue.json" && grep -q '"prefix":"QB"' "$repo/.taskman/queue.json" ||
  { echo "refused: queue template no longer has the expected shape" >&2; exit 1; }
grep -q '"argv":\["test","-f","a/PASS"\]' "$repo/.taskman/policy.json" ||
  { echo "refused: policy template has no make verify gate to replace" >&2; exit 1; }
git_q() { git -C "$1" -c user.name=qualify -c user.email=qualify@example.invalid "${@:2}"; }
git -C "$repo" init -q -b main
git_q "$repo" commit -q --allow-empty -m "qualification base"
export CORVINT_TASKS_ACTOR=qualify ATM_ACTOR=qualify
(cd "$repo" && "$tasks" init --request-id init >"$out/init.json")

cases=0; failed=0
last=$out/last.json
# check NAME EXPECT_OUTCOME EXPECT_CODES [FIELD=VALUE] -- ARGV...: runs ARGV in the queue
# repository and compares the result's outcome, its sorted codes and an optional item field.
check() {
  local name=$1 want_outcome=$2 want_codes=$3 want_field=""
  shift 3
  if [[ $1 != -- ]]; then want_field=$1; shift; fi
  shift
  [[ ${QUALIFY_TASKS_BASIC_TAMPER:-} != "$name" ]] || want_codes=TAMPERED
  local status=0
  (cd "$repo" && "$tasks" "$@") >"$last" 2>"$out/last.err" || status=$?
  cases=$((cases + 1))
  local verdict
  verdict=$(jq -cnS --rawfile raw "$last" --arg name "$name" --arg want_o "$want_outcome" --arg want_c "$want_codes" \
    --arg want_f "$want_field" --argjson status "$status" '
($raw | try fromjson catch null | if type == "object" then . else null end) as $d
| (if $d == null then "UNPARSED" else $d.outcome end) as $outcome
| (($d.codes // []) | sort) as $codes
| ((($d.items // [])[0]) // {}) as $item
| (if $want_f == "" then {ok: true, cur: null} else
     ($want_f | index("=")) as $i | $want_f[$i + 1:] as $v
     | (reduce ($want_f[:$i] | split("."))[] as $p ($item; if type == "object" then .[$p] else null end)) as $cur
     | {ok: (($cur | tojson) == $v or (($cur | type) == "string" and $cur == $v)), cur: $cur} end) as $field
| ($want_c | split(",") | map(select(. != "")) | sort) as $want
# TBS-V1-452: one taskman-command-result/0 envelope; OK exits 0 and every other outcome exits 1.
| (if $want_o == "OK" then 0 else 1 end) as $want_exit
| {case: $name, argv: $ARGS.positional, exit: $status, profile: $d.profile, outcome: $outcome, codes: $codes,
   expected: {profile: "taskman-command-result/0", exit: $want_exit, outcome: $want_o, codes: $want,
     field: (if $want_f == "" then null else $want_f end)},
   observedField: $field.cur,
   warnings: [($d.warnings // [])[] | select(tostring | contains("actor binding") | not)],
   pass: ($d.profile == "taskman-command-result/0" and $outcome == $want_o and $codes == $want and $field.ok
     and $status == $want_exit)}
' --args -- "$@")
  printf '%s\n' "$verdict" >>"$results"
  if jq -e .pass <<<"$verdict" >/dev/null; then
    echo "PASS case=$name"
  else
    failed=$((failed + 1))
    echo "FAIL case=$name $(jq -r '"observed \(.profile) \(.exit) \(.outcome) \(.codes) \(.observedField) expected \(.expected)"' <<<"$verdict")"
  fi
}
item() { jq -r --arg k "$1" '((.items // [])[0] // {})[$k] // ""' "$last"; }
payload() {
  jq -c --arg title "$1" --arg path "$2" '.source.sourceQueueId = "queue:qual:main" | .title = $title
    | .effects = {coverage: "QUALIFIED", externalUnbounded: false, resources: [], touchPaths: [$path]}' \
    "$templates/ticket-create.json"
}
at=2026-10-30T00:00:00Z

# Reachability: a non-fixture queue, and its execution gate refuses before the cutover.
check init-nonfixture OK "" fixture=false -- queue status
check create-1 OK "" -- ticket create --request-id c1 --issued-at $at --payload "$(payload "basic lifecycle" a/)"
t1=$(item ticketId)
check create-2 OK "" -- ticket create --request-id c2 --issued-at $at --payload "$(payload "colliding scope" a/)"
t2=$(item ticketId)
check claim-before-cutover REFUSED CUTOVER_MISSING -- claim "$t1" --holder agent-a --request-id k0
check cutover-execution OK "" -- cutover --execution --decision qualify-basic --qualification "$out/cal019.jsonl"

# Plan, claim, replay, collision, renew and fencing.
check plan-preview OK "" -- plan preview
check claim OK "" -- claim "$t1" --holder agent-a --request-id k1
attempt=$(item attemptId); gen=$(item generation)
check claim-replay OK "" replayed=true -- claim "$t1" --holder agent-a --request-id k1
check claim-scope-collision REFUSED RESOURCE_COLLISION -- claim "$t2" --holder agent-b --request-id k2
check renew OK "" -- renew --attempt "$attempt" --generation "$gen" --request-id r1
check renew-fenced-generation REFUSED FENCED -- renew --attempt "$attempt" --generation $((gen + 1)) --request-id r2

# Candidates in a separate worktree: out of scope, failing gate, wrong and dirty candidate, pass.
wt=$out/candidate
git -C "$repo" worktree add -q -b candidate "$wt" main
mkdir -p "$wt/a"; echo x >"$wt/a/x"; echo outside >"$wt/OUTSIDE"
git_q "$wt" add -A; git_q "$wt" commit -q -m "out of scope"
check submit-out-of-scope REFUSED OUT_OF_SCOPE -- submit --attempt "$attempt" --generation "$gen" --request-id s0 --tree "$(git -C "$wt" rev-parse 'HEAD^{tree}')"
git_q "$wt" rm -q OUTSIDE; git_q "$wt" commit -q -m "fails the gate"
check submit-failing-candidate OK "" -- submit --attempt "$attempt" --generation "$gen" --request-id s1 --tree "$(git -C "$wt" rev-parse 'HEAD^{tree}')"
echo dirty >"$wt/a/untracked"
check gate-dirty-worktree ERROR DIRTY_WORKTREE -- gate run --attempt "$attempt" --generation "$gen" --request-id g0 --gate verify --worktree "$wt"
rm -f "$wt/a/untracked"
check gate-fails OK "" gateResult.state=FAILED -- gate run --attempt "$attempt" --generation "$gen" --request-id g1 --gate verify --worktree "$wt"
echo ok >"$wt/a/PASS"; git_q "$wt" add -A; git_q "$wt" commit -q -m "passes the gate"
pass_commit=$(git -C "$wt" rev-parse HEAD)
check gate-wrong-candidate ERROR STALE_TREE -- gate run --attempt "$attempt" --generation "$gen" --request-id g2 --gate verify --worktree "$wt"
check submit-passing-candidate OK "" -- submit --attempt "$attempt" --generation "$gen" --request-id s2 --tree "$(git -C "$wt" rev-parse 'HEAD^{tree}')"
check gate-passes OK "" gateResult.state=PASSED -- gate run --attempt "$attempt" --generation "$gen" --request-id g3 --gate verify --worktree "$wt"
check complete-unintegrated REFUSED COMMIT_NOT_INTEGRATED,STALE_TREE -- complete --attempt "$attempt" --generation "$gen" --request-id d0 --commit "$pass_commit"

# A newer candidate makes the passing gate stale; integrate it, rerun the gate, then complete.
echo z >"$wt/a/z"; git_q "$wt" add -A; git_q "$wt" commit -q -m "final candidate"
final_commit=$(git -C "$wt" rev-parse HEAD)
check submit-final-candidate OK "" -- submit --attempt "$attempt" --generation "$gen" --request-id s3 --tree "$(git -C "$wt" rev-parse 'HEAD^{tree}')"
git -C "$repo" merge -q --ff-only "$final_commit"
check complete-stale-gate REFUSED GATE_STALE -- complete --attempt "$attempt" --generation "$gen" --request-id d1 --commit "$final_commit"
check gate-final OK "" gateResult.state=PASSED -- gate run --attempt "$attempt" --generation "$gen" --request-id g4 --gate verify --worktree "$wt"
check complete-fenced-generation REFUSED FENCED -- complete --attempt "$attempt" --generation $((gen + 1)) --request-id d2 --commit "$final_commit"
check complete OK "" outcome=COMPLETED -- complete --attempt "$attempt" --generation "$gen" --request-id d3 --commit "$final_commit"
check complete-replay OK "" replayed=true -- complete --attempt "$attempt" --generation "$gen" --request-id d3 --commit "$final_commit"
check readback-completed OK "" status=COMPLETED -- ticket show "$t1"

# Recovery: the freed scope admits the other ticket; a release cancels it and it is claimable again.
check claim-after-completion OK "" -- claim "$t2" --holder agent-b --request-id k3
attempt2=$(item attemptId); gen2=$(item generation)
check release OK "" -- release --attempt "$attempt2" --generation "$gen2" --request-id x1
check reclaim-after-release OK "" -- claim "$t2" --holder agent-c --request-id k4
check receipt-audit OK "" -- receipt audit

if [[ $failed -gt 0 ]]; then
  echo "FAILED cases=$cases failed=$failed"
  exit 1
fi
# TBS-V1-454: only darwin/arm64 and linux/amd64 on go1.27.3 can qualify.
tuple=$(uname -s)/$(uname -m)/$go_version
case $tuple in
  Darwin/arm64/go1.27.3 | Linux/x86_64/go1.27.3) tuple_ok=true ;;
  *) tuple_ok=false ;;
esac
if [[ $binary_source == BUILT && $suite_source == RUN && $src_dirty == false && $tuple_ok == true ]]; then
  echo "QUALIFIED cases=$cases"
else
  echo "LIFECYCLE_PASSED cases=$cases binary=$binary_source suite=$suite_source sourceDirty=$src_dirty tuple=$tuple supportedTuple=$tuple_ok"
fi
