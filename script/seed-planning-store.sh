#!/bin/sh
# Bootstrap the IPR-01..IPR-11 slices from the human-owned Markdown plan
# (docs/plans/integrated-product-roadmap-2026-09-12.md) into an isolated,
# execution-disabled corvint-taskman planning store, using only existing ticket
# mutations (IPR-03). The Markdown stays authoritative; the store is a
# labelled, disposable copy. This is not a TCP-06 import or authority cutover.
# Usage: seed-planning-store.sh <atm-binary> <store-root> [roadmap.md]
#
# Idempotent and interruption-safe: ticket creation replays by stable
# request-id regardless of store state, and a dependency set is only issued
# when the ticket's current dependencies do not already match the desired
# set (an --expected-revision mutation cannot itself replay once the store
# has moved past the revision it was first issued against). Every mutation
# is issued with a fixed --issued-at (the roadmap's own "Updated" date) so
# the recorded mutation is byte-identical on every rerun; atm hashes the
# wall-clock default into the mutation, which would otherwise turn a rerun
# minutes later into a spurious REQUEST_ID_CONFLICT instead of a replay.
set -eu
atm=$1
root=$2
issued_at=2026-09-12T00:00:00Z
source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
roadmap=${3:-"$source_root/docs/plans/integrated-product-roadmap-2026-09-12.md"}
data="$source_root/script/seed-planning-store-data.json"

digest=$(shasum -a 256 "$roadmap" | awk '{print $1}')
expected=$(jq -r '.expectedDigestSha256' "$data")
if [ "$digest" != "$expected" ]; then
  echo "seed-planning-store: source-digest mismatch: $roadmap is $digest, seed data pins $expected" >&2
  echo "seed-planning-store: refusing; a changed source needs a reviewed refinement of $data, not an overwrite" >&2
  exit 3
fi
short=$(printf '%s' "$digest" | cut -c1-8)
queue_id=$(jq -r '.queueId' "$data")

if [ ! -e "$root" ]; then
  "$source_root/script/atm-fixture-store.sh" "$atm" "$root"
fi

count=$(jq '.tickets | length' "$data")
i=0
while [ "$i" -lt "$count" ]; do
  token=$(jq -r ".tickets[$i].localToken" "$data")
  payload=$(jq -S -c --argjson i "$i" --arg d "$digest" --arg q "$queue_id" '.tickets[$i] as $t | {
    localToken: $t.localToken, acceptanceCriteria: $t.acceptanceCriteria, body: $t.body,
    capabilities: [], dependencies: [], dueDate: null,
    effects: {coverage: "UNKNOWN", externalUnbounded: false, resources: [], touchPaths: []},
    estimateMinutes: null, executionClass: "MANUAL", kind: "FEATURE",
    labels: ($t.labels | sort), milestone: $t.milestone, order: $t.order, owner: $t.owner,
    priority: $t.priority, requiredGates: [], requirementRefs: ($t.requirementRefs | sort),
    source: {kind: "IMPORT", sourceItemId: $t.localToken, sourceQueueId: $q, sourceRevisionSha256: $d},
    supersededBy: null, supersedes: null, title: $t.title
  }' "$data")
  request_id="ipr-seed-${short}-create-${token}"
  (cd "$root" && "$atm" ticket create --request-id "$request_id" --issued-at "$issued_at" --payload "$payload" >/dev/null)
  i=$((i + 1))
done

i=0
while [ "$i" -lt "$count" ]; do
  token=$(jq -r ".tickets[$i].localToken" "$data")
  ticket_id="ticket:corvint:planning:$token"
  desired=$(jq -S -c --argjson i "$i" '.tickets[$i].dependsOn | map({gateId: null, obligation: "COMPLETED", ticketId: ("ticket:corvint:planning:" + .)}) | sort_by(.ticketId)' "$data")
  if [ "$desired" != "[]" ]; then
    current=$(cd "$root" && "$atm" ticket show "$token" 2>/dev/null | jq -S -c '.items[0].record.dependencies | sort_by(.ticketId)' 2>/dev/null || echo null)
    if [ "$current" != "$desired" ]; then
      revision=$(cd "$root" && "$atm" ticket show "$token" | jq -r '.items[0].revision')
      dep_payload=$(printf '{"dependencies":%s}' "$desired" | jq -S -c '.')
      request_id="ipr-seed-${short}-deps-${token}"
      (cd "$root" && "$atm" ticket set-dependencies --request-id "$request_id" --issued-at "$issued_at" --target "$ticket_id" --expected-revision "$revision" --payload "$dep_payload" >/dev/null)
    fi
  fi
  i=$((i + 1))
done

echo "seed-planning-store: seeded $count tickets (source digest ${short}) into $root"
