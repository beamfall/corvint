#!/bin/sh
# Create and initialize an isolated, execution-disabled corvint-taskman planning
# store for local demonstrations and planning seeds. The store is a fixture
# (`fixture: true`, `executionCutover: null`): it never admits or dispatches
# work and is never a project's real queue. Nothing here touches Corvint's
# repository state. Usage: atm-fixture-store.sh <atm-binary> <store-root>
# [queueId] [prefix]. The store root must not already exist.
set -eu
atm=$1
root=$2
queue_id=${3:-queue:corvint:planning}
prefix=${4:-IPR}
if [ -e "$root" ]; then
  echo "atm-fixture-store: $root already exists; refusing to reinitialize" >&2
  exit 2
fi
mkdir -p "$root/.git" "$root/.taskman"
jq -S -c -n --arg q "$queue_id" --arg p "$prefix" '{
  profile:"taskman-queue/0", queueId:$q, repositoryAuthorityId:("repo:" + ($q|split(":")[1])), prefix:$p,
  nextSerial:"1", schemaVersion:"0", canonicalWriter:"NATIVE", foreignAdapterId:null,
  intentBranch:"main", fixture:true, executionCutover:null, importMapSha256:null,
  writeBarrier:{reason:"NONE", since:null}}' > "$root/.taskman/queue.json"
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
  runtimes:[], environment:{allowedEnvKeys:[]}}' > "$root/.taskman/policy.json"
(cd "$root" && "$atm" init --request-id "fixture-init-$(basename "$root")")
