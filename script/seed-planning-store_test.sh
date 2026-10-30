#!/bin/sh
set -eu

# IPR-03: bootstrapping the eleven IPR slices into an isolated, execution-disabled
# planning store must be idempotent (replay, not duplicate), interruption-safe
# (a partial run completes on rerun) and refuse a changed source digest without
# writing anything. Fixtures live under /private/tmp (atm refuses a symlinked
# /tmp path) in per-run unique directories; nothing here is ever rm -rf'd, since
# deleting under /private/tmp prompts the operator on every call.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
atm=${ATM:-/private/tmp/corvint-public-release-20260912/bin/atm}
if [ ! -x "$atm" ]; then
  echo "seed-planning-store_test: no atm binary at $atm (set ATM=/path/to/atm)" >&2
  exit 1
fi

run_root="/private/tmp/corvint-public-release-20260912/seed-test-$(date +%s)-$$"
mkdir -p "$run_root"
data="$source_root/script/seed-planning-store-data.json"
roadmap="$source_root/docs/plans/integrated-product-roadmap-2026-09-12.md"

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

# --- Case 1: full seed produces all eleven tickets with the stated dependencies. ---
store1="$run_root/store1"
GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=init.defaultBranch GIT_CONFIG_VALUE_0=fixture-default \
  "$source_root/script/seed-planning-store.sh" "$atm" "$store1" "$roadmap" >/dev/null
[ "$(git -C "$store1" symbolic-ref --short HEAD)" = main ] || fail "fixture did not pin the queue intent branch"
count=$(cd "$store1" && "$atm" roadmap | jq '.items | length')
[ "$count" = 11 ] || fail "expected 11 roadmap tickets after full seed, got $count"
for pair in "IPR-02:IPR-01" "IPR-04:IPR-01" "IPR-05:IPR-04" "IPR-06:IPR-01" \
            "IPR-07:IPR-06" "IPR-08:IPR-06" "IPR-11:IPR-10"; do
  blocked=${pair%%:*}
  blocker=${pair##*:}
  deps=$(cd "$store1" && "$atm" ticket show "$blocked" | jq -r '.items[0].record.dependencies[].ticketId')
  case "$deps" in
    *"ticket:corvint:planning:$blocker"*) ;;
    *) fail "$blocked missing dependency on $blocker" ;;
  esac
done
ipr09deps=$(cd "$store1" && "$atm" ticket show IPR-09 | jq -r '.items[0].record.dependencies | length')
[ "$ipr09deps" = 2 ] || fail "IPR-09 expected 2 dependencies (IPR-07, IPR-08), got $ipr09deps"
ipr10deps=$(cd "$store1" && "$atm" ticket show IPR-10 | jq -r '.items[0].record.dependencies | length')
[ "$ipr10deps" = 4 ] || fail "IPR-10 expected 4 dependencies (IPR-02,03,05,09), got $ipr10deps"
ipr01deps=$(cd "$store1" && "$atm" ticket show IPR-01 | jq -r '.items[0].record.dependencies | length')
[ "$ipr01deps" = 0 ] || fail "IPR-01 expected no dependencies, got $ipr01deps"

# --- Case 2: rerunning is a replay - no new receipts, still 11 tickets. ---
# shellcheck disable=SC2012
before=$(ls "$store1/.git/taskman/receipts" | wc -l | tr -d ' ')
"$source_root/script/seed-planning-store.sh" "$atm" "$store1" "$roadmap" >/dev/null
# shellcheck disable=SC2012
after=$(ls "$store1/.git/taskman/receipts" | wc -l | tr -d ' ')
[ "$before" = "$after" ] || fail "rerun wrote new receipts: before=$before after=$after"
count2=$(cd "$store1" && "$atm" roadmap | jq '.items | length')
[ "$count2" = 11 ] || fail "rerun changed ticket count to $count2"

# --- Case 3: interruption after a few creates completes on the next run. ---
store2="$run_root/store2"
"$source_root/script/atm-fixture-store.sh" "$atm" "$store2" >/dev/null
digest=$(shasum -a 256 "$roadmap" | awk '{print $1}')
short=$(printf '%s' "$digest" | cut -c1-8)
i=0
while [ "$i" -lt 5 ]; do
  token=$(jq -r ".tickets[$i].localToken" "$data")
  payload=$(jq -S -c --argjson i "$i" --arg d "$digest" --arg q "queue:corvint:planning" '.tickets[$i] as $t | {
    localToken: $t.localToken, acceptanceCriteria: $t.acceptanceCriteria, body: $t.body,
    capabilities: [], dependencies: [], dueDate: null,
    effects: {coverage: "UNKNOWN", externalUnbounded: false, resources: [], touchPaths: []},
    estimateMinutes: null, executionClass: "MANUAL", kind: "FEATURE",
    labels: ($t.labels | sort), milestone: $t.milestone, order: $t.order, owner: $t.owner,
    priority: $t.priority, requiredGates: [], requirementRefs: ($t.requirementRefs | sort),
    source: {kind: "IMPORT", sourceItemId: $t.localToken, sourceQueueId: $q, sourceRevisionSha256: $d},
    supersededBy: null, supersedes: null, title: $t.title
  }' "$data")
  (cd "$store2" && "$atm" ticket create --request-id "ipr-seed-${short}-create-${token}" \
    --issued-at 2026-09-12T00:00:00Z --payload "$payload" >/dev/null)
  i=$((i + 1))
done
partial=$(cd "$store2" && "$atm" roadmap | jq '.items | length')
[ "$partial" = 5 ] || fail "interruption fixture expected 5 tickets before resume, got $partial"
"$source_root/script/seed-planning-store.sh" "$atm" "$store2" "$roadmap" >/dev/null
resumed=$(cd "$store2" && "$atm" roadmap | jq '.items | length')
[ "$resumed" = 11 ] || fail "resume after interruption expected 11 tickets, got $resumed"
ipr10deps2=$(cd "$store2" && "$atm" ticket show IPR-10 | jq -r '.items[0].record.dependencies | length')
[ "$ipr10deps2" = 4 ] || fail "resumed store: IPR-10 expected 4 dependencies, got $ipr10deps2"

# --- Case 4: a source-digest mismatch refuses and writes nothing. ---
tampered="$run_root/tampered-roadmap.md"
printf '# tampered plan\n' > "$tampered"
store3="$run_root/store3"
status=0
"$source_root/script/seed-planning-store.sh" "$atm" "$store3" "$tampered" >/tmp/seed-test-mismatch.out 2>&1 || status=$?
[ "$status" -ne 0 ] || fail "digest mismatch was not refused"
[ ! -e "$store3" ] || fail "digest mismatch still created $store3"
grep -q "source-digest mismatch" /tmp/seed-test-mismatch.out || fail "refusal message missing source-digest mismatch text"

printf 'seed-planning-store: full seed, replay, interruption-resume and digest-mismatch refusal pass\n'
