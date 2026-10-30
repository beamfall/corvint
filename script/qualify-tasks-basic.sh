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
pyjson() { python3 -I -c "$@"; }

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

pyjson 'import json,sys; print(json.dumps({"profile":"tasks-basic-qualification-identity/0","sourceCommit":sys.argv[1],"sourceTree":sys.argv[2],"sourceDirty":sys.argv[3]=="true","goVersion":sys.argv[4],"binary":{"source":sys.argv[5],"sha256":sys.argv[6],"version":{k:v for k,v in json.load(open(sys.argv[7]))["items"][0].items() if k!="formats"}},"suite":{"source":sys.argv[8],"sha256":sys.argv[9],"tests":sys.argv[10].split()},"host":{"os":sys.argv[11],"arch":sys.argv[12]}},sort_keys=True))' \
  "$src_commit" "$src_tree" "$src_dirty" "$go_version" "$binary_source" "$(sha256 "$tasks")" "$out/version.json" \
  "$suite_source" "$(sha256 "$out/cal019.jsonl")" "$suite_names" "$(uname -s)" "$(uname -m)" >"$out/identity.json"

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
  verdict=$(pyjson '
import json,sys
name,want_o,want_c,want_f,status,path=sys.argv[1:7]
argv=sys.argv[7:]
try:
    d=json.load(open(path)); outcome=d.get("outcome"); codes=sorted(d.get("codes") or [])
    item=(d.get("items") or [{}])[0]
except Exception as e:
    d=None; outcome="UNPARSED"; codes=[]; item={}
profile=(d or {}).get("profile")
field_ok=True; observed_field=None
if want_f:
    k,v=want_f.split("=",1); cur=item
    for part in k.split("."):
        cur=cur.get(part) if isinstance(cur,dict) else None
    observed_field=cur; field_ok=json.dumps(cur)==v or str(cur)==v
want=sorted(c for c in want_c.split(",") if c)
# TBS-V1-452: one taskman-command-result/0 envelope; OK exits 0 and every other outcome exits 1.
want_exit=0 if want_o=="OK" else 1
ok=(profile=="taskman-command-result/0" and outcome==want_o and codes==want and field_ok
    and int(status)==want_exit)
print(json.dumps({"case":name,"argv":argv,"exit":int(status),"profile":profile,"outcome":outcome,"codes":codes,
  "expected":{"profile":"taskman-command-result/0","exit":want_exit,"outcome":want_o,"codes":want,"field":want_f or None},
  "observedField":observed_field,
  "warnings":[w for w in (d or {}).get("warnings",[]) if "actor binding" not in w],"pass":ok},sort_keys=True))
' "$name" "$want_outcome" "$want_codes" "$want_field" "$status" "$last" "$@")
  printf '%s\n' "$verdict" >>"$results"
  if [[ $verdict == *'"pass": true'* ]]; then
    echo "PASS case=$name"
  else
    failed=$((failed + 1))
    echo "FAIL case=$name $(pyjson 'import json,sys; v=json.loads(sys.argv[1]); print("observed",v["profile"],v["exit"],v["outcome"],v["codes"],v["observedField"],"expected",v["expected"])' "$verdict")"
  fi
}
item() { pyjson 'import json,sys; d=json.load(open(sys.argv[1])); print((d.get("items") or [{}])[0].get(sys.argv[2]) or "")' "$last" "$1"; }
payload() {
  pyjson 'import json,sys
p=json.load(open(sys.argv[1])); p["source"]["sourceQueueId"]="queue:qual:main"; p["title"]=sys.argv[2]
p["effects"]={"coverage":"QUALIFIED","externalUnbounded":False,"resources":[],"touchPaths":[sys.argv[3]]}
print(json.dumps(p))' "$templates/ticket-create.json" "$1" "$2"
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
