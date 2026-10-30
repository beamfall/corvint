#!/usr/bin/env bash
# usage: measure-import-cal018.sh EXPORT OUTDIR [--allow-loaded]
# Re-measures CAL-V0-018 criterion 2 (V1-0436): the first `corvint-tasks import`
# of the 2,894-item Beamfall export into a fresh ROADMAP fixture store must take
# under 5 minutes on a host whose load is below its CPU count. EXPORT is the
# corvint-tasks-import/0 JSONL export. Only the export pinned in
# https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-15-v1-0436-incremental-import.md (SHA-256 `pinned`
# below, not overridable) can qualify. OUTDIR must not exist; the source tree,
# binary, fixture store and results are written only there.
#
# The script builds corvint-tasks from `git archive HEAD` (uncommitted edits
# never reach the binary), creates the store (init is outside the timed span),
# times the import, then requires `receipt audit` OK/CONSISTENT/AGREES and an
# immediate reimport that writes nothing and leaves every store byte unchanged.
# Load averages are read before and after the import; no background sampler is
# started. The run is QUALIFYING only when the input digest matches, every load
# reading is below the CPU count and the import took under 300 s. With
# --allow-loaded the script runs on a loaded host or another input and labels
# the result NONQUALIFYING (a smoke check of the procedure, never evidence).
set -euo pipefail
export LC_ALL=C

export_file=${1:?usage: measure-import-cal018.sh EXPORT OUTDIR [--allow-loaded]}
out=${2:?usage: measure-import-cal018.sh EXPORT OUTDIR [--allow-loaded]}
allow_loaded=${3:-}
pinned=be368b0524acacf375dcd28bf104646b691f9a88382f892ac58c6556c07c3d01
queue_id=${QUEUE_ID:-queue:beamfall:main}
prefix=${PREFIX:-BF}
repo=$(git rev-parse --show-toplevel)

export_file=$(cd "$(dirname "$export_file")" && pwd -P)/$(basename "$export_file")
if [ -e "$out" ]; then
  echo "measure-import-cal018: $out already exists" >&2
  exit 2
fi
if [ -n "$allow_loaded" ] && [ "$allow_loaded" != "--allow-loaded" ]; then
  echo "measure-import-cal018: unknown flag $allow_loaded" >&2
  exit 2
fi

sha256() { if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }
cpus() { getconf _NPROCESSORS_ONLN; }
loads() {
  if [ -r /proc/loadavg ]; then cut -d' ' -f1-3 /proc/loadavg
  else sysctl -n vm.loadavg | tr -d '{}' | awk '{print $1, $2, $3}'; fi
}
below() { awk -v n="$2" '{ for (i = 1; i <= NF; i++) if ($i + 0 >= n) exit 1 }' <<<"$1"; }
tree_sum() { (cd "$1" && find .taskman .git/taskman -type f | sort | xargs shasum -a 256) | shasum -a 256 | cut -d' ' -f1; }

mkdir -p "$out"
out=$(cd "$out" && pwd -P)
summary="$out/summary.txt"
note() { echo "$1" | tee -a "$summary"; }

digest=$(sha256 "$export_file")
ncpu=$(cpus)
note "source: $(git -C "$repo" rev-parse HEAD) (built from git archive HEAD)"
note "go: $(GOTOOLCHAIN=local go env GOVERSION) $(go env GOOS)/$(go env GOARCH)"
note "export: $export_file sha256=$digest bytes=$(wc -c <"$export_file" | tr -d ' ') lines=$(wc -l <"$export_file" | tr -d ' ')"
note "cpus: $ncpu"

mkdir "$out/src"
git -C "$repo" archive HEAD | tar -x -C "$out/src"
GOTOOLCHAIN=local go build -C "$out/src" -o "$out/corvint-tasks" ./cmd/corvint-tasks

root="$out/store"
mkdir -p "$root/.taskman"
git -C "$root" init -q -b main
jq -S -c -n --arg q "$queue_id" --arg p "$prefix" '{
  profile:"taskman-queue/0", queueId:$q, repositoryAuthorityId:("repo:" + ($q|split(":")[1])), prefix:$p,
  nextSerial:"1", schemaVersion:"0", canonicalWriter:"ROADMAP", foreignAdapterId:null,
  intentBranch:"main", fixture:true, executionCutover:null, importMapSha256:null,
  writeBarrier:{reason:"NONE", since:null}}' >"$root/.taskman/queue.json"
jq -S -c -n '{
  profile:"taskman-policy/0", policyVersion:"1", roles:{},
  capacity:{maxActiveAttempts:"1", maxWorkersTotal:"1", classes:[]},
  budgets:{lane:{inputTokens:"2000000", cacheCreationTokens:"1000000", cacheReadTokens:"100000000",
    outputTokens:"400000", turns:"400", wallClockMinutes:"90"}, ticketMultiplier:"4",
    requireEnforcedFields:["turns","wallClockMinutes"]},
  retries:{admissionsPerRevision:"3", repairRounds:"2", malformedReviewRetry:"1", gateRerunOnStale:"1", reconcileAttempts:"3"},
  retention:{evidenceDays:"90"},
  gates:[{gateId:"verify", kind:"COMMAND", argv:["make","verify"], cwd:"WORKTREE", env:[], timeoutSeconds:"1800",
    expected:{exitCode:"0", reducer:null}, evidence:[], inputs:[], sharedResource:null, reusable:true, required:true}],
  serialFallback:"BLOCK", integrationRequiredKinds:[], allowEmptyObligationsKinds:["CHORE"],
  reviewLane:{required:true}, docsLane:{required:false}, cemRequired:false, ocmRequired:false,
  runtimes:[], environment:{allowedEnvKeys:[]}}' >"$root/.taskman/policy.json"
(cd "$root" && "$out/corvint-tasks" init >"$out/init.json")
jq -e '.outcome == "OK"' "$out/init.json" >/dev/null

before=$(loads)
note "load before import: $before"
if ! below "$before" "$ncpu" && [ -z "$allow_loaded" ]; then
  note "verdict: NOT_RUN (load $before is not below $ncpu CPUs)"
  exit 3
fi

rc=0
(cd "$root" && /usr/bin/time -p "$out/corvint-tasks" import --file "$export_file" >"$out/import.json" 2>"$out/import.time") || rc=$?
after=$(loads)
note "load after import (1/5/15 min): $after"
note "import exit: $rc"
grep -E '^(real|user|sys) ' "$out/import.time" | tee -a "$summary"
real=$(awk '$1 == "real" { print $2 }' "$out/import.time")
note "import: $(jq -c '{outcome, written: .items[0].written, unchanged: .items[0].unchanged, receipts: (.items[0].receipts | length)}' "$out/import.json")"

audit_rc=0
(cd "$root" && "$out/corvint-tasks" receipt audit >"$out/audit.json") || audit_rc=$?
note "receipt audit: $(jq -c '{outcome, item: (.items[0] | {headSeq, structuralConsistency, projectionAgreement})}' "$out/audit.json" 2>/dev/null || echo unreadable)"

pre=$(tree_sum "$root")
reimport_rc=0
(cd "$root" && "$out/corvint-tasks" import --file "$export_file" >"$out/reimport.json") || reimport_rc=$?
post=$(tree_sum "$root")
note "reimport: $(jq -c '{outcome, written: .items[0].written, unchanged: .items[0].unchanged}' "$out/reimport.json") store-bytes-unchanged=$([ "$pre" = "$post" ] && echo yes || echo no)"

if [ "$rc" -ne 0 ] || ! jq -e '.outcome == "OK"' "$out/import.json" >/dev/null ||
  [ "$audit_rc" -ne 0 ] || ! jq -e '.outcome == "OK" and .items[0].structuralConsistency == "CONSISTENT" and .items[0].projectionAgreement == "AGREES"' "$out/audit.json" >/dev/null ||
  [ "$reimport_rc" -ne 0 ] || ! jq -e '.outcome == "OK" and (.items[0].written | tonumber) == 0' "$out/reimport.json" >/dev/null ||
  [ "$pre" != "$post" ]; then
  note "verdict: FAIL (import, audit or reimport did not complete cleanly)"
  exit 1
fi
if [ "$digest" = "$pinned" ] && below "$before $after" "$ncpu" && awk -v r="$real" 'BEGIN { exit !(r < 300) }'; then
  note "verdict: QUALIFYING PASS (${real}s under 300 s; every load reading below $ncpu CPUs)"
elif [ "$digest" = "$pinned" ] && below "$before $after" "$ncpu"; then
  note "verdict: QUALIFYING FAIL (${real}s is not under 300 s)"
  exit 1
else
  note "verdict: NONQUALIFYING (input digest or load condition not met; ${real}s)"
fi
