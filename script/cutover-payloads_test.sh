#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
set -eu

# TSC-V1-009, TSC-V1-010, TSC-V1-013: script/cutover-payloads.sh applies the mapping table to a
# small dump: facet labels from title prefix and touch path, the label-over-kind type rule, the
# stripped title, dropped unknown refs and missing touch paths, edges filtered to survivors,
# dependency order, the manifest records, and exit 1 with an UNPLACED row for a MANUAL ticket.

here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
tool=$here/cutover-payloads.sh
root=$(mktemp -d "${TMPDIR:-/tmp}/cutover-payloads-test.XXXXXX")
trap 'rm -rf "$root"' EXIT INT TERM
fail() { printf 'cutover-payloads_test: FAIL %s\n' "$1" >&2; exit 1; }

repo=$root/repo
change=$repo/docs/specs/openspec/changes/tasks-store-cutover
mkdir -p "$change" "$repo/internal/testplan"
cp "$here/../docs/specs/ticket-vocabulary.json" "$repo/docs/specs/ticket-vocabulary.json"
cp "$here/../docs/specs/openspec/changes/tasks-store-cutover/taxonomy-mapping.tsv" "$change/"
printf 'id\tfile\n' > "$repo/docs/specs/REQUIREMENTS.tsv"
printf 'PROD-TEST-001\tdocs/specs/PRODUCT-PLAN-1.0.md\n' >> "$repo/docs/specs/REQUIREMENTS.tsv"
printf 'package testplan\n' > "$repo/internal/testplan/plan.go"
git -C "$repo" init -q
git -C "$repo" add -A
git -C "$repo" -c user.name=t -c user.email=t@example.invalid commit -q -m fixture

# item ID KIND TITLE LABELS PATHS REFS DEPS
item() {
  jq -n -c --arg id "$1" --arg kind "$2" --arg title "$3" --argjson labels "$4" \
    --argjson paths "$5" --argjson refs "$6" --argjson deps "$7" '
    {ticketId: ("ticket:corvint:main:" + $id), status: "OPEN", lifecycle: {state: "OPEN"},
     holds: [], operatorNote: null, progress: [],
     record: {kind: $kind, title: $title, labels: $labels, milestone: "post-1-0",
       requirementRefs: $refs, body: "b", acceptanceCriteria: ["a"], capabilities: [],
       dependencies: ($deps | map({gateId: null, obligation: "COMPLETED",
         ticketId: ("ticket:corvint:main:" + .)})),
       effects: {coverage: "QUALIFIED", externalUnbounded: false, resources: [],
         touchPaths: $paths},
       supersedes: null, supersededBy: null, approvals: [], executionClass: "AUTONOMOUS",
       requiredGates: [], source: null, order: "0", owner: null, priority: "P1",
       dueDate: null, estimateMinutes: null}}'
}
{
  item V1-0001 BUG 'affected: miss a renamed file' '["bugs"]' \
    '["internal/testplan/plan.go","gone/file.go"]' '["PROD-TEST-001","GONE-001"]' '["V1-0002"]'
  item V1-0002 FEATURE 'Speed up planning' '["optimizations"]' '["internal/testplan/"]' '[]' '[]'
  item V1-0003 MANUAL 'Accept the release' '[]' '["internal/testplan/"]' '[]' '["V1-0999"]'
} > "$root/dump.jsonl"

rc=0
sh "$tool" --repo "$repo" --rev HEAD --dump "$root/dump.jsonl" --out "$root/out" \
  > "$root/log" 2> "$root/err" || rc=$?
test "$rc" -eq 1 || fail "exit $rc, want 1 for an unplaced ticket: $(cat "$root/err")"
grep -q 'tickets 3 / payloads 3 / unplaced 1 / missing touch paths 1' "$root/log" ||
  fail "counts: $(cat "$root/log")"
test "$(cat "$root/out/unplaced.tsv")" = "$(printf 'V1-0003\ttype\tkind MANUAL: rule value -')" ||
  fail "unplaced: $(cat "$root/out/unplaced.tsv")"
test "$(grep -n -e V1-0001 -e V1-0002 "$root/out/order.txt" | cut -d: -f2 | tr '\n' ' ')" = \
  'V1-0002 V1-0001 ' || fail "dependency order: $(tr '\n' ' ' < "$root/out/order.txt")"
test "$(jq -r .oldId "$root/out/payloads.jsonl" | tr '\n' ' ')" = \
  "$(tr '\n' ' ' < "$root/out/order.txt")" || fail "payloads not in order.txt order"

p1=$(jq -c 'select(.oldId == "V1-0001") | .payload' "$root/out/payloads.jsonl")
test "$(printf '%s' "$p1" | jq -c '[.labels, .title, .milestone, .requirementRefs,
  .effects.touchPaths, (.dependencies | length)]')" = \
  '[["component:affected","type:bug"],"miss a renamed file","v1-0",["PROD-TEST-001"],'\
'["internal/testplan/plan.go"],1]' || fail "V1-0001 payload: $p1"
p2=$(jq -c 'select(.oldId == "V1-0002") | .payload | [.labels, .kind]' "$root/out/payloads.jsonl")
test "$p2" = '[["component:affected","type:optimization"],"CHORE"]' || fail "V1-0002: $p2"
p3=$(jq -c 'select(.oldId == "V1-0003") | .payload | [.labels, .dependencies]' \
  "$root/out/payloads.jsonl")
test "$p3" = '[["component:affected"],[]]' || fail "V1-0003: $p3"

m1=$(jq -c 'select(.oldId == "V1-0001") | [.title.from, .droppedLabels, .droppedRefs,
  .droppedTouchPaths, .milestone, .component.from]' "$root/out/manifest.jsonl")
test "$m1" = '["affected: miss a renamed file",["bugs"],["GONE-001"],["gone/file.go"],'\
'{"from":"post-1-0","to":"v1-0"},"title-prefix"]' || fail "V1-0001 manifest: $m1"
m2=$(jq -c 'select(.oldId == "V1-0002") | [.kind, .component.from]' "$root/out/manifest.jsonl")
test "$m2" = '[{"from":"FEATURE","to":"CHORE"},"touch-path"]' || fail "V1-0002 manifest: $m2"
m3=$(jq -c 'select(.oldId == "V1-0003") | .droppedDependencies[].ticketId' \
  "$root/out/manifest.jsonl")
test "$m3" = '"ticket:corvint:main:V1-0999"' || fail "V1-0003 manifest: $m3"
test "$(jq -r .oldId "$root/out/gate-input.jsonl" | tr '\n' ' ')" = 'V1-0001 V1-0002 V1-0003 ' ||
  fail "gate input order"

rc=0
sh "$tool" --repo "$repo" --rev HEAD --dump "$root/log" --out "$root/bad" 2> /dev/null || rc=$?
test "$rc" -eq 2 || fail "malformed dump exit $rc, want 2"
printf 'cutover-payloads_test: ok\n'
