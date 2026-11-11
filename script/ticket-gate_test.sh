#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

# TTX-V1-016, TTX-V1-017: one fixture per refusal code of script/ticket-gate.sh, each refused with
# its code and with no create run; the seed gate's held, folded and seeded lists; and the
# change-folder check of docs/specs/ticket-vocabulary.json (TTX-V1-001, TTX-V1-002, TTX-V1-005).

here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
gate=$here/ticket-gate.sh
root=$(mktemp -d "${TMPDIR:-/tmp}/ticket-gate-test.XXXXXX")
trap 'rm -rf "$root"' EXIT INT TERM
fail() { printf 'ticket-gate_test: FAIL %s\n' "$1" >&2; exit 1; }
tab=$(printf '\t')

repo=$root/repo
mkdir -p "$repo/docs/specs/openspec/changes/new-change" "$repo/.taskman" "$repo/internal" \
  "$root/bin"
cp "$here/../docs/specs/ticket-vocabulary.json" "$repo/docs/specs/ticket-vocabulary.json"
printf 'id\tfile\ttitle\tacceptance\tacceptedBy\n' > "$repo/docs/specs/REQUIREMENTS.tsv"
for id in PROD-TEST-001 PROD-INT-005 PROD-DOCS-001; do
  printf '%s\tdocs/specs/PRODUCT-PLAN-1.0.md\tx\tunrecorded\t-\n' "$id" \
    >> "$repo/docs/specs/REQUIREMENTS.tsv"
done
printf '{"policyVersion":"14"}\n' > "$repo/.taskman/policy.json"
# A fake corvint-tasks records every call; a create must never run on a refusal.
cat > "$root/bin/corvint-tasks" << EOF
#!/bin/sh
printf '%s\n' "\$*" >> "$root/calls.log"
case "\$*" in
  'ticket search'*) cat "$root/search.json" ;;
  'ticket create'*) cat > "$root/created.json"; printf '{"outcome":"OK"}\n' ;;
esac
EOF
chmod +x "$root/bin/corvint-tasks"
PATH=$root/bin:$PATH
export PATH
printf '{"items":[]}\n' > "$root/search.json"
: > "$root/empty.tsv"

base=$root/base.json
cat > "$base" << 'EOF'
{"title": "Affected plans include an unindexed source path",
 "body": "Version: 1.0.0-rc.3\nExpected: left out\nActual: selected\nRepro: corvint affected",
 "kind": "BUG", "owner": "corvint", "milestone": "v1-0", "priority": "P2", "order": "100",
 "labels": ["component:affected", "type:bug", "runner:go"], "dependencies": [],
 "acceptanceCriteria": ["The plan leaves out unindexed source paths."],
 "requirementRefs": ["PROD-TEST-001"], "source": null,
 "effects": {"coverage": "QUALIFIED", "externalUnbounded": false, "resources": [],
   "touchPaths": ["internal/testplan/plan.go"]},
 "capabilities": [], "requiredGates": [], "executionClass": null, "dueDate": null,
 "estimateMinutes": null, "supersedes": [], "supersededBy": [],
 "identityKey": "affected.over.c-unindexed-source-path"}
EOF
n=0
# variant NAME JQ: write $root/NAME.json from the base payload.
variant() { jq "$2" "$base" > "$root/$1.json"; }
# refused NAME CODE NEEDLE [ARGS...]: the payload is refused with CODE, the line holds NEEDLE,
# and no create runs even with --apply.
refused() {
  name=$1 code=$2 needle=$3; shift 3
  n=$((n + 1)); : > "$root/calls.log"
  set +e
  out=$("$gate" create --repo "$repo" "$@" --apply --request-id r1 "$root/$name.json" 2>&1)
  rc=$?
  set -e
  test "$rc" = 1 || fail "$n $name: exit $rc, want 1: $out"
  case $out in *"REFUSED$tab$code$tab"*"$needle"*) ;; *) fail "$n $name: no $code: $out" ;; esac
  if grep -q 'ticket create' "$root/calls.log"; then fail "$n $name: a create ran"; fi
}

# 1. The base payload passes; the key label and the computed product label are added.
n=$((n + 1))
out=$("$gate" create --repo "$repo" --index "$root/empty.tsv" "$base")
case $out in
  "PAYLOAD$tab"*'"key:affected.over.c-unindexed-source-path"'*'"product:testing"'*) ;;
  *) fail "$n base: $out" ;;
esac
case $out in *identityKey*) fail "$n base: identityKey left in the payload" ;; esac
# The store takes labels in sorted canonical byte order without duplicates.
printf '%s\n' "$out" | sed -n 's/^PAYLOAD	//p' | jq -e '.labels == (.labels | unique)' \
  > /dev/null || fail "$n base: labels not sorted: $out"

# 2-5. FACET_VALUE: a free-text label, a wrong product, a wrong kind, rig in v1-0.
variant label '.labels += ["ci-friction"]'
refused label FACET_VALUE 'label ci-friction' --index "$root/empty.tsv"
variant product '.labels = ["component:mcp", "product:testing", "type:bug"]
  | .identityKey = "mcp.over.c-unindexed-source-path"'
refused product FACET_VALUE 'product: component mcp has product docs' --index "$root/empty.tsv"
variant kind '.kind = "FEATURE"'
refused kind FACET_VALUE 'kind: type bug needs BUG' --index "$root/empty.tsv"
variant rig '.labels = ["component:rig", "type:bug"] | .identityKey = "rig.fail.c-lane-host"'
refused rig FACET_VALUE 'milestone: component rig needs post-1-0' --index "$root/empty.tsv"
# 6. rig in post-1-0 is still refused: its product is NOT_DECIDED (TTX-V1-002).
variant rigpost '.labels = ["component:rig", "type:bug"] | .identityKey = "rig.fail.c-lane-host"
  | .milestone = "post-1-0"'
refused rigpost FACET_VALUE 'product of component rig is NOT_DECIDED' --index "$root/empty.tsv"

# 7-8. FACET_REQUIRED: a testing ticket with no runner, an agents ticket with no agent.
variant runner '.labels = ["component:affected", "type:bug"]'
refused runner FACET_REQUIRED runner --index "$root/empty.tsv"
variant agent '.labels = ["component:agents", "type:bug"]
  | .identityKey = "agents.fail.c-install"'
refused agent FACET_REQUIRED agent --index "$root/empty.tsv"

# 9-11. ANCHOR_REQUIRED: a delivery with no requirement, UNPLANNED on v1-0, UNPLANNED with no
# reason line.
variant delivery '.labels = ["component:affected", "type:delivery", "runner:go"]
  | .kind = "FEATURE" | .requirementRefs = [] | .identityKey = "prod-test-001.impl"
  | .body = "Requirement: PROD-TEST-001\nSlice: impl\nMissing today: x"'
refused delivery ANCHOR_REQUIRED 'requirementRefs is empty' --index "$root/empty.tsv"
variant unplanned '.requirementRefs = ["UNPLANNED"] | .body += "\nUnplanned: found in a lane"'
refused unplanned ANCHOR_REQUIRED 'UNPLANNED needs milestone post-1-0' --index "$root/empty.tsv"
variant noreason '.labels = ["component:specs", "type:chore"] | .kind = "CHORE"
  | .milestone = "post-1-0" | .requirementRefs = ["UNPLANNED"]
  | .identityKey = "specs.chore.refresh.f-index"
  | .body = "Object: the index\nAction: refresh\nWhy now: stale"'
refused noreason ANCHOR_REQUIRED 'Unplanned: <reason>' --index "$root/empty.tsv"

# 12. ANCHOR_UNKNOWN names the missing ID.
variant unknown '.requirementRefs = ["PROD-TEST-001", "PROD-TEST-999"]'
refused unknown ANCHOR_UNKNOWN PROD-TEST-999 --index "$root/empty.tsv"

# 13-14. KEY_REQUIRED and KEY_MISMATCH (a key naming another component).
variant nokey 'del(.identityKey)'
refused nokey KEY_REQUIRED 'no identityKey' --index "$root/empty.tsv"
variant numkey '.identityKey = 123'
refused numkey KEY_MISMATCH 'identityKey is not a string' --index "$root/empty.tsv"
variant falsekey '.identityKey = false | .labels += ["key:affected.over.c-unindexed-source-path"]'
refused falsekey KEY_MISMATCH 'identityKey is not a string' --index "$root/empty.tsv"
variant mismatch '.identityKey = "release.fail.v-corvint-update"'
refused mismatch KEY_MISMATCH 'key component release' --index "$root/empty.tsv"

# 15-17. DUPLICATE_KEY: an open holder; a third ticket naming one of two holders; a finished
# holder, whose refusal names the reopen command.
k=affected.over.c-unindexed-source-path
printf 'V1-0100\tOPEN\t%s\tFirst\n' "$k" > "$root/one.tsv"
variant dup '.'
refused dup DUPLICATE_KEY 'held by V1-0100 OPEN' --index "$root/one.tsv"
case $out in *'ticket recur V1-0100'*) ;; *) fail "$n dup: no recur command: $out" ;; esac
printf 'V1-0100\tOPEN\t%s\tFirst\nV1-0101\tOPEN\t%s\tSecond\n' "$k" "$k" > "$root/two.tsv"
variant third '.distinctFrom = [{"id": "V1-0100", "reason": "another reader"}]'
refused third DUPLICATE_KEY 'distinctFrom leaves out V1-0101' --index "$root/two.tsv"
printf 'V1-0100\tCOMPLETED\t%s\tFirst\n' "$k" > "$root/done.tsv"
refused dup DUPLICATE_KEY 'ticket reopen --target V1-0100' --index "$root/done.tsv"
# 18. Naming every open holder with a reason passes, and the reasons become body lines.
n=$((n + 1))
variant named '.distinctFrom = [{"id": "V1-0100", "reason": "another reader"},
  {"id": "V1-0101", "reason": "another writer"}]'
out=$("$gate" create --repo "$repo" --index "$root/two.tsv" "$root/named.json")
case $out in *'Distinct from V1-0101: another writer'*) ;; *) fail "$n named: $out" ;; esac
# 19. Without --index the holders come from the store search, and the refusal uses them.
printf '{"items":[{"ticketId":"V1-0200","status":"OPEN","title":"Stored"}]}\n' \
  > "$root/search.json"
refused dup DUPLICATE_KEY 'held by V1-0200 OPEN'
grep -q "ticket search --label key:$k" "$root/calls.log" || fail "$n: no holder search"
printf '{"items":[]}\n' > "$root/search.json"

# 20-21. EFFECTS_REQUIRED: no touch path with coverage INCOMPLETE; a top-level directory.
variant effects '.effects = {"coverage": "INCOMPLETE", "externalUnbounded": false,
  "resources": [], "touchPaths": []}'
refused effects EFFECTS_REQUIRED 'no touch path' --index "$root/empty.tsv"
variant topdir '.effects.touchPaths = ["internal"]'
refused topdir EFFECTS_REQUIRED 'touch path internal is a single' --index "$root/empty.tsv"

# 22-25. TEMPLATE_FIELD: no Version line; a prefixed title; no acceptance; a suspected P1 bug
# with no Repro.
variant version '.body = "Expected: a\nActual: b\nRepro: c"'
refused version TEMPLATE_FIELD Version --index "$root/empty.tsv"
variant title '.title = "affected: wrong count"'
refused title TEMPLATE_FIELD title --index "$root/empty.tsv"
variant criteria '.acceptanceCriteria = []'
refused criteria TEMPLATE_FIELD acceptanceCriteria --index "$root/empty.tsv"
variant suspected '.labels += ["suspected"] | .priority = "P1"
  | .body = "Version: 1\nExpected: a\nActual: b"'
refused suspected TEMPLATE_FIELD Repro --index "$root/empty.tsv"

# 26. BODY_TOO_LONG at 1,500 bytes.
variant long '.body += "\n" + ([range(0; 1500)] | map("x") | join(""))'
refused long BODY_TOO_LONG bytes --index "$root/empty.tsv"

# 27. VOCAB_POLICY: a policy facets block that disagrees with the vocabulary.
printf '{"policyVersion":"15","facets":{"components":["affected"]}}\n' > "$root/policy.json"
refused dup VOCAB_POLICY 'facets.components' --index "$root/empty.tsv" \
  --policy "$root/policy.json"

# 28. A key over 60 bytes is stored as 53 bytes, a hyphen and 6 hex digits; label fits 64.
n=$((n + 1))
lk=affected.over.c-a-very-long-locus-name-that-runs-well-past-the-sixty-byte-limit
variant longkey ".identityKey = \"$lk\""
out=$("$gate" create --repo "$repo" --index "$root/empty.tsv" "$root/longkey.json")
label=$(printf '%s\n' "$out" | sed -n 's/^PAYLOAD	//p' |
  jq -r '.labels[] | select(startswith("key:"))')
test "${#label}" = 64 || fail "$n longkey: label $label"
prefix=$(printf '%s' "$lk" | head -c 53)
case $label in "key:$prefix-"??????) ;; *) fail "$n longkey: $label" ;; esac

# 29. A pass with --apply pipes the payload to one create.
n=$((n + 1)); : > "$root/calls.log"
"$gate" create --repo "$repo" --index "$root/empty.tsv" --apply --request-id r9 "$base" > /dev/null
grep -q '^ticket create --request-id r9 --payload-stdin$' "$root/calls.log" || fail "$n apply"
jq -e '.labels | index("key:affected.over.c-unindexed-source-path")' "$root/created.json" \
  > /dev/null || fail "$n apply: created payload has no key label"

# 30. Malformed input exits 2.
n=$((n + 1)); printf 'not json\n' > "$root/bad.json"
set +e; "$gate" create --repo "$repo" --index "$root/empty.tsv" "$root/bad.json" 2> /dev/null
rc=$?; set -e
test "$rc" = 2 || fail "$n malformed: exit $rc"

# 31. Seed gate: a survivor with no touch path is held; two with one key and no distinctFrom
# fold the newer into the older; two naming each other are both seeded.
n=$((n + 1))
{
  jq -c '. + {oldId: "V1-5180"}' "$base"
  jq -c '. + {oldId: "V1-5203"}' "$base"
  jq -c '. + {oldId: "V1-5300", identityKey: "affected.miss.c-renamed-path"}
    | .effects.touchPaths = []' "$base"
  jq -c '. + {oldId: "V1-4973", identityKey: "affected.wrong.o-count",
    distinctFrom: [{id: "V1-5013", reason: "count of tests"}]}' "$base"
  jq -c '. + {oldId: "V1-5013", identityKey: "affected.wrong.o-count",
    distinctFrom: [{id: "V1-4973", reason: "count of packages"}]}' "$base"
} > "$root/seed.jsonl"
out=$("$gate" seed --repo "$repo" --index "$root/empty.tsv" --out "$root/seed" "$root/seed.jsonl")
case $out in
  *"seed${tab}3"*"held${tab}1"*"folded${tab}1"*) ;;
  *) fail "$n seed counts: $out" ;;
esac
grep -q "^V1-5300${tab}EFFECTS_REQUIRED${tab}" "$root/seed/held.tsv" ||
  fail "$n held: $(cat "$root/seed/held.tsv")"
grep -q "^V1-5203${tab}V1-5180${tab}$k\$" "$root/seed/folded.tsv" || fail "$n folded"
test "$(jq -r .oldId "$root/seed/seed.jsonl" | tr '\n' ' ')" = 'V1-5180 V1-4973 V1-5013 ' ||
  fail "$n seeded list"

# 32. Every change folder of this repository has exactly one component. The folder names are
# copied into a fixture and the roots are replaced, so the test reads no component root.
n=$((n + 1))
changes=$here/../docs/specs/openspec/changes
copy=$root/copy
mkdir -p "$copy/docs/specs/openspec/changes/archive" "$copy/internal"
for d in "$changes"/* "$changes"/archive/*; do
  test -d "$d" || continue
  case ${d##*/} in archive) continue ;; esac
  mkdir -p "$copy/docs/specs/openspec/changes/${d#"$changes"/}"
done
jq '.components[].roots = [] | .components.affected.roots = ["internal/"]' \
  "$repo/docs/specs/ticket-vocabulary.json" > "$root/vocab.json"
"$gate" vocab --repo "$copy" --vocab "$root/vocab.json" > "$root/vocab.out" ||
  fail "$n vocab: $(cat "$root/vocab.out")"
grep -q "^total${tab}.*${tab}0 findings\$" "$root/vocab.out" || fail "$n vocab total"
# 33. A change folder with no component and a missing root are both findings.
n=$((n + 1))
mkdir "$copy/docs/specs/openspec/changes/new-change"
jq '.components.rig.roots = ["tools/no-such-root/"]' "$root/vocab.json" > "$root/vocab2.json"
set +e; out=$("$gate" vocab --repo "$copy" --vocab "$root/vocab2.json"); rc=$?; set -e
test "$rc" = 1 || fail "$n vocab fixture: exit $rc"
case $out in *"NO_COMPONENT${tab}new-change"*) ;; *) fail "$n vocab fixture: $out" ;; esac
case $out in *"MISSING_ROOT${tab}tools/no-such-root/"*) ;; *) fail "$n missing root: $out" ;; esac

printf 'ticket-gate_test: ok (%s cases)\n' "$n"
