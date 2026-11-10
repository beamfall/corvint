#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# cutover-payloads.sh: build the store-cutover seed payloads from dumped survivors by applying
# the reviewed taxonomy mapping table (TSC-V1-009, TSC-V1-010, TSC-V1-013; change
# docs/specs/openspec/changes/tasks-store-cutover). It reads files only and writes only --out.
#
# usage: script/cutover-payloads.sh --dump DUMP.jsonl --out DIR [--table F] [--vocab F]
#          [--requirements F] [--anchors F] [--repo R] [--rev REV]
#   DUMP.jsonl  one `corvint-tasks ticket show ID` item (.items[0]) per line, survivors only
#   --table     the mapping table (default: the change folder's taxonomy-mapping.tsv)
#   --anchors   TSV with columns ticket, priority, lifecycle, disposition, target (the M0 release
#               mapping); a MAPPED row's target is added to requirementRefs
#   --rev       the tree touch paths are checked against (default: origin/main)
# Writes DIR/payloads.jsonl ({oldId, payload} in dependency order, without localToken; the seed
# loop adds localToken = oldId), DIR/order.txt, DIR/gate-input.jsonl (`ticket-gate.sh seed` input:
# each payload with member oldId, oldest first), DIR/manifest.jsonl (one row per ticket),
# DIR/unplaced.tsv (oldId, field, detail) and DIR/table.sha256; prints the counts.
# Exit 0 when every ticket is placed, 1 when one is not (the seed stops, TSC-V1-009), 2 malformed.
set -eu

die() { printf 'cutover-payloads: %s\n' "$*" >&2; exit 2; }
usage() {
  printf 'usage: %s --dump DUMP.jsonl --out DIR [--table F] [--vocab F]\n' "$0" >&2
  printf '         [--requirements F] [--anchors F] [--repo R] [--rev REV]\n' >&2
  exit 2
}

repo=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
dump='' out='' table='' vocab='' reqs='' anchors='' rev=origin/main
while [ $# -gt 0 ]; do
  case $1 in
    --dump) test $# -ge 2 || usage; dump=$2; shift 2 ;;
    --out) test $# -ge 2 || usage; out=$2; shift 2 ;;
    --table) test $# -ge 2 || usage; table=$2; shift 2 ;;
    --vocab) test $# -ge 2 || usage; vocab=$2; shift 2 ;;
    --requirements) test $# -ge 2 || usage; reqs=$2; shift 2 ;;
    --anchors) test $# -ge 2 || usage; anchors=$2; shift 2 ;;
    --repo) test $# -ge 2 || usage; repo=$2; shift 2 ;;
    --rev) test $# -ge 2 || usage; rev=$2; shift 2 ;;
    *) usage ;;
  esac
done
test -n "$dump" && test -n "$out" || usage
change=$repo/docs/specs/openspec/changes/tasks-store-cutover
table=${table:-$change/taxonomy-mapping.tsv}
vocab=${vocab:-$repo/docs/specs/ticket-vocabulary.json}
reqs=${reqs:-$repo/docs/specs/REQUIREMENTS.tsv}
for f in "$dump" "$table" "$vocab" "$reqs"; do test -r "$f" || die "cannot read $f"; done
test -z "$anchors" || test -r "$anchors" || die "cannot read $anchors"
command -v jq > /dev/null || die "jq is required"
git -C "$repo" rev-parse --verify -q "$rev^{tree}" > /dev/null || die "no tree $rev in $repo"
jq -e -s 'all(.[]; type == "object" and (.record | type) == "object")' "$dump" > /dev/null 2>&1 \
  || die "$dump is not one ticket show item per line"
mkdir -p "$out"

rules=$(grep -v '^#' "$table" | tail -n +2 | jq -R -s -c \
  'split("\n") | map(select(length > 0) | split("\t")
   | {field: .[0], match: .[1], value: .[2]})')
printf '%s' "$rules" | jq -e 'length > 0 and all(.[]; .field != null and .match != null
  and .value != null)' > /dev/null || die "$table has a row without field, match and value"
known=$(cut -f1 "$reqs" | tail -n +2 | jq -R -s -c \
  'split("\n") | map(select(length > 0) | {key: ., value: true}) | from_entries')
if [ -n "$anchors" ]; then
  anc=$(tail -n +2 "$anchors" | jq -R -s -c 'split("\n") | map(select(length > 0) | split("\t")
    | select(.[3] == "MAPPED") | {key: .[0], value: (.[4] | split(","))}) | from_entries')
else
  anc='{}'
fi

# Touch paths absent from the tree (TSC-V1-013); a directory path ends in "/".
missing='[]'
for p in $(jq -r '.record.effects.touchPaths[]? | select(type == "string")' "$dump" | sort -u); do
  q=${p%/}
  git -C "$repo" cat-file -e "$rev:$q" 2> /dev/null \
    || missing=$(printf '%s' "$missing" | jq -c --arg p "$p" '. + [$p]')
done

jq -s -c --argjson rules "$rules" --argjson known "$known" --argjson anc "$anc" \
  --argjson missing "$missing" --slurpfile vocab "$vocab" '
  $vocab[0] as $v
  | def old: .ticketId | split(":") | last;
  def idof: if type == "string" then split(":") | last else null end;
  def rows($f): $rules | map(select(.field == $f));
  (map(old)) as $ids
  | ([$v.components | to_entries[] | .key as $k | .value.roots[] | {root: ., k: $k}]
     + [rows("component")[] | select(.match | startswith("root="))
        | {root: (.match | ltrimstr("root=")), k: .value}]) as $roots
  | ([rows("component")[] | select(.match | startswith("ignore-path="))
      | .match | ltrimstr("ignore-path=")]) as $ignored
  | ([rows("component")[] | select(.match | startswith("secondary="))
      | .match | ltrimstr("secondary=")]) as $secondary
  | map(. as $it | .record as $r | ($it | old) as $id
    | ($r.labels // []) as $ls
    | ($r.title | capture("^(?<p>[^\\s:]+): (?<rest>.+)$") // null) as $tp
    | ([rows("type")[] | . as $x | ($x.match | split("=")) as $m
        | select(($m[0] == "kind" and $r.kind == $m[1])
            or ($m[0] == "label" and ($ls | index($m[1]) != null)))] | first) as $tr
    | (if $tr == null or $tr.value == "-" then null else $tr.value end) as $type
    | (if $tp == null then null else ($tp.p | ascii_downcase) as $p
        | ([rows("component")[] | select(.match == "alias=\($p)") | .value] | first)
          // (if $v.components | has($p) then $p else null end) end) as $tcomp
    | ([($r.effects.touchPaths // [])[] | . as $q | select($missing | index([$q]) | not)]) as $paths
    | ($paths | map(select(. as $p | $ignored | index([$p]) | not))
        | map(. as $p | [$roots[] | .root as $o | select($o == $p or ($p | startswith($o))
            or ($o | endswith("/")) and $p + "/" == $o)] | max_by(.root | length) | .k?)
        | map(select(. != null)) | unique
        | if length > 1 then map(select(. as $c | $secondary | index([$c]) | not)) else . end
        ) as $pcomps
    | (if ($pcomps | length) == 1 then $pcomps[0] else null end) as $pcomp
    | ($tcomp // $pcomp) as $comp
    | (($r.requirementRefs // []) | map(select($known[.] // false))) as $refs0
    | ($anc[$id] // null) as $a
    | (($refs0 + ($a // [])) | unique) as $refs
    | ($r.dependencies // []) as $deps
    | ($deps | map(select((.ticketId | idof) as $d | $ids | index($d) != null))) as $kdeps
    | (($r.supersedes | idof) as $s | if $s != null and ($ids | index($s)) != null
        then $r.supersedes else null end) as $sup
    | (if $type == null then null else $v.types[$type].kind end) as $kind
    | {id: $id,
       unplaced: ([if $type == null then {field: "type", detail: ("kind \($r.kind)"
                     + (if $tr == null then ": no rule" else ": rule value -" end))} else empty end,
                   if $comp == null then {field: "component", detail: ("touch paths "
                     + (($r.effects.touchPaths // []) | join(" ")) + " name "
                     + (if ($pcomps | length) == 0 then "no component"
                        else "components \($pcomps | join(" "))" end)
                     + (if $tp == null then "" else "; title prefix \($tp.p) is none" end))}
                   else empty end]),
       row: {oldId: $id,
             payload: {acceptanceCriteria: ($r.acceptanceCriteria // []), body: $r.body,
               capabilities: ($r.capabilities // []), dependencies: $kdeps,
               dueDate: $r.dueDate, effects: (($r.effects // {}) | .touchPaths = $paths),
               estimateMinutes: $r.estimateMinutes, executionClass: $r.executionClass,
               kind: ($kind // $r.kind),
               labels: ([if $comp == null then empty else "component:\($comp)" end,
                         if $type == null then empty else "type:\($type)" end] | sort),
               milestone: "v1-0", order: $r.order, owner: $r.owner, priority: $r.priority,
               requiredGates: ($r.requiredGates // []), requirementRefs: $refs,
               source: $r.source, supersededBy: null, supersedes: $sup,
               title: (if $tp != null then $tp.rest else $r.title end)}},
       manifest: {oldId: $id, newId: $id, localToken: $id, oldStatus: $it.status,
         lifecycle: $it.lifecycle, holds: ($it.holds // []), approvals: ($r.approvals // []),
         operatorNote: $it.operatorNote,
         latestProgress: (($it.progress // []) | if type == "array" then last else . end),
         milestone: (if $r.milestone == "v1-0" then null else {from: $r.milestone, to: "v1-0"} end),
         kind: (if $kind != null and $kind != $r.kind then {from: $r.kind, to: $kind}
                else null end),
         title: (if $tp != null then {from: $r.title} else null end),
         component: (if $comp == null then null else {value: $comp,
           from: (if $tcomp != null then "title-prefix" else "touch-path" end)} end),
         droppedLabels: $ls,
         droppedRefs: (($r.requirementRefs // []) - $refs0), addedAnchor: $a,
         droppedTouchPaths: (($r.effects.touchPaths // []) - $paths),
         droppedDependencies: ($deps - $kdeps),
         droppedSupersedes: (if $r.supersedes != null and $sup == null then $r.supersedes
                             else null end),
         droppedSupersededBy: $r.supersededBy, duplicateOverrides: []}}) | .[]' "$dump" \
  > "$out/mapped.jsonl" || die "the mapping transform failed"

# Dependency order: a dependency or a superseded ticket is created first.
jq -r '.id as $i | ((.row.payload.dependencies[].ticketId | split(":") | last)
  , (.row.payload.supersedes // empty | split(":") | last), $i) | "\(.) \($i)"' \
  "$out/mapped.jsonl" > "$out/edges.txt"
tsort "$out/edges.txt" > "$out/order.txt" 2> "$out/tsort.err" \
  || die "dependency cycle: $out/tsort.err"
test ! -s "$out/tsort.err" || die "dependency cycle: $out/tsort.err"
jq -c -s --rawfile order "$out/order.txt" '(INDEX(.id)) as $m
  | $order | split("\n") | map(select(length > 0) | $m[.] | .row) | .[]' "$out/mapped.jsonl" \
  > "$out/payloads.jsonl"
jq -c -s 'sort_by(.id | ltrimstr("V1-") | tonumber? // 0) | .[] | .row.payload + {oldId: .id}' \
  "$out/mapped.jsonl" > "$out/gate-input.jsonl"
jq -c '.manifest' "$out/mapped.jsonl" > "$out/manifest.jsonl"
jq -r '.id as $i | .unplaced[] | [$i, .field, .detail] | @tsv' "$out/mapped.jsonl" \
  > "$out/unplaced.tsv"
shasum -a 256 "$table" > "$out/table.sha256"
total=$(wc -l < "$out/mapped.jsonl" | tr -d ' ')
unplaced=$(cut -f1 "$out/unplaced.tsv" | sort -u | wc -l | tr -d ' ')
printf 'tickets %s / payloads %s / unplaced %s / missing touch paths %s\n' "$total" \
  "$(wc -l < "$out/payloads.jsonl" | tr -d ' ')" "$unplaced" \
  "$(printf '%s' "$missing" | jq length)"
test "$unplaced" -eq 0 || exit 1
