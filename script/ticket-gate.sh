#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

# TTX-V1-016, TTX-V1-017: the interim create gate and the seed gate of change `ticket-taxonomy`.
# It applies TTX-V1-001 to TTX-V1-013 from docs/specs/ticket-vocabulary.json before any create,
# until a Tasks build enforces them.
#
#   create [opts] [--index TSV] [--apply --request-id ID] PAYLOAD.json
#       One gate payload: a ticket create payload plus the optional members identityKey (the full
#       key) and distinctFrom ([{"id","reason"}]). A pass prints `PAYLOAD<TAB><json>`: the create
#       payload with label key:<k> (stored form, at most 60 bytes), the computed product label and
#       one body line per distinctFrom entry; exit 0. --apply then pipes it to `corvint-tasks
#       ticket create --payload-stdin`, run in the store directory. A refusal prints one
#       `REFUSED<TAB>CODE<TAB>detail` line per broken rule, runs no create, and exits 1.
#   seed [opts] [--index TSV] --out DIR PAYLOADS.jsonl
#       One gate payload per line, oldest first, each with member oldId. Writes DIR/seed.jsonl
#       ({"oldId","payload"}), DIR/held.tsv (oldId, codes, details) and DIR/folded.tsv (oldId,
#       intoOldId, key), prints the three counts and exits 0. Two seeds with one key are both
#       seeded only when each names the other in distinctFrom; otherwise the newer is folded.
#   vocab [--repo R]
#       Checks that every change folder has exactly one known component, every component has a
#       product or a recorded productStatus, and every root exists. Exit 1 on a finding.
#
# opts: --repo R (default: the repository holding this script; vocabulary, requirements, policy
# and touch paths are read from it), --store S (directory where corvint-tasks runs; default R),
# --vocab F, --requirements F, --policy F. SIBLING lines (TTX-V1-010) go to stderr. Without
# --index, create reads key holders with `corvint-tasks ticket search --label key:<k>` in S.
# Exit 2 on unreadable or malformed input, or when the holder search fails.

me=ticket-gate
usage() {
  printf 'usage: %s create|seed|vocab [--repo R] [--store S] [--vocab F] [--requirements F]\n' \
    "$0" >&2
  printf '       [--policy F] [--index TSV] [--out DIR] [--apply --request-id ID] [FILE]\n' >&2
  exit 2
}
die() { printf '%s: %s\n' "$me" "$1" >&2; exit 2; }

test $# -ge 1 || usage
mode=$1; shift
case $mode in create|seed|vocab) ;; *) usage ;; esac
repo=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
store= vocab= reqs= policy= index= out= apply=0 request= input=
while test $# -gt 0; do
  case $1 in
    --repo) test $# -ge 2 || usage; repo=$2; shift 2 ;;
    --store) test $# -ge 2 || usage; store=$2; shift 2 ;;
    --vocab) test $# -ge 2 || usage; vocab=$2; shift 2 ;;
    --requirements) test $# -ge 2 || usage; reqs=$2; shift 2 ;;
    --policy) test $# -ge 2 || usage; policy=$2; shift 2 ;;
    --index) test $# -ge 2 || usage; index=$2; shift 2 ;;
    --out) test $# -ge 2 || usage; out=$2; shift 2 ;;
    --request-id) test $# -ge 2 || usage; request=$2; shift 2 ;;
    --apply) apply=1; shift ;;
    -*) usage ;;
    *) test -z "$input" || usage; input=$1; shift ;;
  esac
done
store=${store:-$repo}
vocab=${vocab:-$repo/docs/specs/ticket-vocabulary.json}
reqs=${reqs:-$repo/docs/specs/REQUIREMENTS.tsv}
policy=${policy:-$repo/.taskman/policy.json}
command -v jq > /dev/null 2>&1 || die 'jq is required'
test -r "$vocab" || die "cannot read vocabulary $vocab"
jq -e 'type == "object" and (.components | type) == "object"' "$vocab" > /dev/null 2>&1 ||
  die "malformed vocabulary $vocab"

if test "$mode" = vocab; then
  changes=$repo/docs/specs/openspec/changes
  test -d "$changes" || die "no change folders under $changes"
  folders=$(find "$changes" -mindepth 1 -maxdepth 1 -type d ! -name archive | sed 's|.*/||')
  if test -d "$changes/archive"; then
    folders="$folders
$(find "$changes/archive" -mindepth 1 -maxdepth 1 -type d | sed 's|.*/||')"
  fi
  roots=$(jq -r '.components[].roots[]' "$vocab")
  missing=
  for r in $roots; do test -e "$repo/$r" || missing="$missing$r
"; done
  findings=$(printf '%s\n' "$folders" |
    jq -R -s -r --slurpfile v "$vocab" --arg missing "$missing" '
    ($v[0]) as $v | (split("\n") | map(select(length > 0)) | unique) as $f
    | ($f[] | select(. as $x | $v.changeFolders | has($x) | not) | "NO_COMPONENT\t" + .),
      ($v.changeFolders | to_entries[] | select(.key as $k | $f | any(. == $k) | not)
        | "UNKNOWN_FOLDER\t" + .key),
      ($v.changeFolders | to_entries[] | select(.value as $x | $v.components | has($x) | not)
        | "UNKNOWN_COMPONENT\t" + .key + "\t" + .value),
      ($v.components | to_entries[]
        | select(.value.product == null and .value.productStatus == null)
        | "NO_PRODUCT\t" + .key),
      ($v.components | to_entries[] | select(.value.product != null)
        | select(.value.product as $p | $v.products | any(. == $p) | not)
        | "UNKNOWN_PRODUCT\t" + .key),
      ($missing | split("\n")[] | select(length > 0) | "MISSING_ROOT\t" + .)') ||
    die 'the vocabulary check failed'
  n=$(printf '%s\n' "$folders" | grep -c . || true)
  if test -n "$findings"; then printf '%s\n' "$findings"; fi
  undecided=$(jq -r '.components | to_entries[] | select(.value.product == null)
    | "NOTE\t" + .key + " product " + (.value.productStatus // "unset")' "$vocab")
  if test -n "$undecided"; then printf '%s\n' "$undecided"; fi
  printf 'total\t%s folders\t%s findings\n' "$n" "$(printf '%s' "$findings" | grep -c . || true)"
  test -z "$findings"
  exit
fi

test -n "$input" || usage
test -r "$input" || die "cannot read $input"
test -r "$reqs" || die "cannot read requirements $reqs"
if test "$mode" = seed; then
  test -n "$out" || die 'seed needs --out DIR'
  jq -s -e 'length > 0 and all(.[]; type == "object" and (.oldId | type) == "string")' "$input" \
    > /dev/null 2>&1 || die "malformed seed input $input: one object with oldId per line"
  payloads=$(jq -s -c . "$input")
else
  test "$apply" = 0 || test -n "$request" || die '--apply needs --request-id ID'
  jq -e 'type == "object"' "$input" > /dev/null 2>&1 || die "malformed payload $input"
  payloads=$(jq -c '[.]' "$input")
fi
pol=null
if test -r "$policy"; then
  pol=$(jq -c . "$policy" 2> /dev/null) || die "malformed policy $policy"
fi
req=$(awk -F'\t' 'NR > 1 && $1 != "" { print $1 }' "$reqs" | jq -R -s -c 'split("\n")
  | map(select(length > 0))')

sha6() {
  if command -v sha256sum > /dev/null 2>&1; then sha256sum; else shasum -a 256; fi | cut -c1-6
}
# Keys over 60 bytes keep 53 bytes, a hyphen and 6 hex digits of the full key's SHA-256.
hashes='{}'
longkeys=$(printf '%s' "$payloads" | jq -r '.[] | .identityKey? // empty | strings
  | select(test("^[a-z0-9][a-z0-9.-]*$") and utf8bytelength > 60)')
for k in $longkeys; do
  s="$(printf '%s' "$k" | head -c 53)-$(printf '%s' "$k" | sha6)"
  hashes=$(printf '%s' "$hashes" | jq -c --arg k "$k" --arg s "$s" '.[$k] = $s')
done
# Touch paths of one segment: a directory in the repository is refused (TTX-V1-011).
dirs='[]'
single=$(printf '%s' "$payloads" | jq -r '.[] | .effects?.touchPaths? // [] | .[] | strings
  | select(test("^[A-Za-z0-9._-]+$"))' | sort -u)
for p in $single; do
  if test -d "$repo/$p"; then dirs=$(printf '%s' "$dirs" | jq -c --arg p "$p" '. + [$p]'); fi
done

idx='[]'
if test -n "$index"; then
  test -r "$index" || die "cannot read index $index"
  idx=$(jq -R -s -c 'split("\n") | map(select(length > 0) | split("\t")
    | {id: .[0], status: .[1], key: .[2], title: (.[3] // "")})' "$index")
elif test "$mode" = create; then
  stored=$(printf '%s' "$payloads" | jq -r --argjson h "$hashes" '.[0] | (.identityKey? // null)
    as $f | if ($f | type) == "string" then ($h[$f] // $f)
    else ([.labels? // [] | .[] | strings | select(startswith("key:"))][0] // "" | ltrimstr("key:"))
    end')
  if test -n "$stored"; then
    found=$(cd "$store" && corvint-tasks ticket search --label "key:$stored" --limit 1000) ||
      die "holder search failed in $store; DUPLICATE_KEY cannot be checked"
    idx=$(printf '%s' "$found" | jq -c --arg k "$stored" '[.items[]? | {id: .ticketId,
      status: .status, key: $k, title: (.title // "")}]') || die 'malformed holder search result'
  fi
fi

result=$(printf '%s' "$payloads" | jq -r --slurpfile v "$vocab" --argjson req "$req" \
  --argjson pol "$pol" --argjson idx "$idx" --argjson hash "$hashes" --argjson dirs "$dirs" \
  --arg mode "$mode" '
($v[0]) as $v |
def inarr($a; $x): $a | any(. == $x);
def facet($ls; $ns): [$ls[] | strings | select(startswith($ns + ":")) | .[($ns | length) + 1:]];
def openst: . != "COMPLETED" and . != "ARCHIVED";
def hasline($body; $l): $body | test("(^|\n)" + $l + ":[ \t]*[^ \t\n]");
def loc($x): ($x | type) == "string"
  and ($v.locusPrefixes | any(. as $pre | $x | startswith($pre) and length > ($pre | length)));
def keyerr($k; $t; $c; $refs; $run):
  if ($k | type) != "string" or $t == null then empty
  elif ($k | test("^[a-z0-9][a-z0-9.-]*$") | not) then "key \($k) is not lower-case and dot-joined"
  else ($k | split(".")) as $s
  | if $t == "delivery" then
      if ($s | length) < 2 or ($s | length) > 3
        then "key \($k) is not <requirement-id>.<slice>[.<runner>]"
      elif ($refs | map(strings | ascii_downcase) | any(. == $s[0]) | not)
        then "key requirement \($s[0]) is not in requirementRefs"
      elif (inarr($v.slices; $s[1]) | not) then "key slice \($s[1]) is not in the vocabulary"
      elif ($s | length) == 3 and ((inarr($v.runners; $s[2]) and inarr($run; $s[2])) | not)
        then "key runner \($s[2]) is not a runner label of the ticket"
      else empty end
    elif $s[0] != $c then "key component \($s[0]) is not the component \($c)"
    elif $t == "bug" then
      if ($s | length) != 3 or (inarr($v.bugClasses; $s[1]) | not) or (loc($s[2]) | not)
        then "key \($k) is not <component>.<class>.<locus>" else empty end
    elif $t == "test-gap" then
      if ($s | length) != 3 or $s[1] != "gap" or (loc($s[2]) | not)
        then "key \($k) is not <component>.gap.<locus>" else empty end
    elif $t == "optimization" then
      if ($s | length) != 3 or (inarr(["slow", "large"]; $s[1]) | not) or (loc($s[2]) | not)
        then "key \($k) is not <component>.slow|large.<locus>" else empty end
    elif $t == "decision" then
      if ($s | length) != 3 or $s[1] != "decide" or ($s[2] | length) == 0
        then "key \($k) is not <component>.decide.<topic>" else empty end
    elif $t == "chore" then
      if ($s | length) != 4 or $s[1] != "chore" or (inarr($v.choreActions; $s[2]) | not)
        or ($s[3] | length) == 0
        then "key \($k) is not <component>.chore.<action>.<object>" else empty end
    else empty end
  end;
def policydiff:
  if ($pol | type) == "object" and ($pol.facets? != null) then
    [["components", ($v.components | keys)], ["products", $v.products],
     ["types", ($v.types | keys)], ["runners", $v.runners], ["agents", $v.agents]][] as [$n, $want]
    | ($pol.facets[$n]?) as $have
    | (if ($have | type) == "object" then ($have | keys)
       elif ($have | type) == "array" then ($have | map(tostring)) else null end) as $h
    | select($h == null or ($h | sort | unique) != ($want | sort | unique))
    | {code: "VOCAB_POLICY", detail: "policy facets.\($n) differs from the vocabulary"}
  else empty end;
def check($p; $index):
  ($p.labels // [] | if type == "array" then . else [] end) as $ls
  | ($p.body // "" | tostring) as $body0
  | facet($ls; "component") as $comp | facet($ls; "product") as $prod
  | facet($ls; "type") as $typ | facet($ls; "runner") as $run | facet($ls; "agent") as $agt
  | facet($ls; "key") as $klab
  | ($comp[0] // null) as $c | ($typ[0] // null) as $t
  | (if $c == null then null else $v.components[$c] end) as $cv
  | (if $t == null then null else $v.types[$t] end) as $tv
  | ($p.requirementRefs // [] | if type == "array" then . else [] end) as $refs
  | ($p.identityKey // null) as $full
  | (if ($full | type) == "string" then ($hash[$full] // $full) else ($klab[0] // null) end)
    as $stored
  | ($p.distinctFrom // [] | if type == "array" then . else [] end) as $df
  | ($body0 + ($df | map("\n\($v.interim.distinctFromLine) \(.id): \(.reason)") | join("")))
    as $body
  | ($index | map(select($stored != null and .key == $stored))) as $holders
  | ($holders | map(select(.status | openst))) as $open
  | ($open | map(select(.id as $i
      | $df | any(.id == $i and ((.reason // "") | tostring | test("[^ \t]"))) | not))) as $left
  | ($p.effects // {}) as $e | ($e.touchPaths // []) as $tp
  | ($p.title // "" | tostring) as $ti
  | [
      ($ls[] | . as $l
        | if (type != "string") or (utf8bytelength > $v.limits.labelBytes)
            then {code: "FACET_VALUE", detail: "label \($l)"}
          elif test("^(component|product|type|runner|agent):") then
            (split(":")[0]) as $ns | .[($ns | length) + 1:] as $val
            | (if $ns == "component" then ($v.components | has($val))
               elif $ns == "product" then inarr($v.products; $val)
               elif $ns == "type" then ($v.types | has($val))
               elif $ns == "runner" then inarr($v.runners; $val)
               else inarr($v.agents; $val) end) as $ok
            | select($ok | not) | {code: "FACET_VALUE", detail: "label \($l)"}
          elif startswith($v.interim.keyLabelPrefix) or inarr($v.otherLabels; $l) then empty
          else {code: "FACET_VALUE", detail: "label \($l)"} end),
      (if ($comp | length) == 0 then {code: "FACET_REQUIRED", detail: "component"}
       elif ($comp | length) > 1 then {code: "FACET_VALUE", detail: "component: more than one"}
       else empty end),
      (if ($typ | length) == 0 then {code: "FACET_REQUIRED", detail: "type"}
       elif ($typ | length) > 1 then {code: "FACET_VALUE", detail: "type: more than one"}
       else empty end),
      (if $cv == null then empty
       elif $cv.product == null then {code: "FACET_VALUE", detail: ("product: the product of "
         + "component \($c) is \($cv.productStatus // "NOT_DECIDED") in the vocabulary")}
       elif ($prod | length) > 0 and $prod != [$cv.product]
         then {code: "FACET_VALUE", detail: "product: component \($c) has product \($cv.product)"}
       else empty end),
      (if $cv != null and $cv.milestone != null and $p.milestone != $cv.milestone
         then {code: "FACET_VALUE", detail: "milestone: component \($c) needs \($cv.milestone)"}
       elif inarr($v.milestones; $p.milestone) | not
         then {code: "FACET_VALUE", detail: "milestone \($p.milestone)"}
       else empty end),
      (if $tv != null and $p.kind != $tv.kind
         then {code: "FACET_VALUE", detail: "kind: type \($t) needs \($tv.kind)"} else empty end),
      (if $cv != null and $cv.product == "testing" and ($run | length) == 0
         then {code: "FACET_REQUIRED", detail: "runner"} else empty end),
      (if $c == "agents" and ($agt | length) == 0
         then {code: "FACET_REQUIRED", detail: "agent"} else empty end),
      (if ($refs | length) == 0 then {code: "ANCHOR_REQUIRED", detail: "requirementRefs is empty"}
       elif ($t == "delivery" or $p.milestone == "v1-0")
         and ($refs | any(strings | startswith("PROD-")) | not)
         then {code: "ANCHOR_REQUIRED", detail: "a delivery or v1-0 ticket names no PROD-*"}
       else empty end),
      (if inarr($refs; "UNPLANNED") then
         (if $p.milestone != "post-1-0"
            then {code: "ANCHOR_REQUIRED", detail: "UNPLANNED needs milestone post-1-0"}
          else empty end),
         (if $t != null and (inarr($v.unplannedTypes; $t) | not)
            then {code: "ANCHOR_REQUIRED", detail: "UNPLANNED is not allowed for type \($t)"}
          else empty end),
         (if hasline($body0; $v.interim.unplannedReasonLine) | not
            then {code: "ANCHOR_REQUIRED",
                  detail: "UNPLANNED needs a body line Unplanned: <reason>"}
          else empty end)
       else empty end),
      ($refs[] | select(. != "UNPLANNED") | select(inarr($req; .) | not)
        | {code: "ANCHOR_UNKNOWN", detail: tostring}),
      (if $full == null and ($klab | length) == 0
         then {code: "KEY_REQUIRED", detail: "no identityKey and no key: label"}
       elif ($klab | length) > 1 then {code: "KEY_MISMATCH", detail: "more than one key: label"}
       elif ($full | type) == "string" and ($klab | length) == 1 and $klab[0] != $stored
         then {code: "KEY_MISMATCH",
               detail: "label key:\($klab[0]) is not the stored key \($stored)"}
       elif ($stored | type) == "string" and ($stored | utf8bytelength) > $v.limits.keyBytes
         then {code: "KEY_MISMATCH", detail: "key \($stored) is over \($v.limits.keyBytes) bytes"}
       else empty end),
      (keyerr($full // $stored; $t; $c; $refs; $run) | {code: "KEY_MISMATCH", detail: .}),
      (if ($holders | length) > 0 and (($left | length) > 0 or ($df | length) == 0) then
         {code: "DUPLICATE_KEY", detail: ("key \($stored) is held by "
           + ($holders | map("\(.id) \(.status) \"\(.title)\"") | join("; "))
           + (if ($left | length) > 0 then "; distinctFrom leaves out "
                + ($left | map(.id) | join(", ")) else "" end)
           + "; the same problem: " + ($holders | map(if (.status | openst)
               then "corvint-tasks ticket recur \(.id) --context TEXT --request-id ID"
                 + " or ticket attach-evidence --target \(.id)"
               else "corvint-tasks ticket reopen --target \(.id), then recur" end)
             | join("; ")))}
       else empty end),
      (if $e.coverage != "QUALIFIED"
         then {code: "EFFECTS_REQUIRED", detail: "coverage \($e.coverage)"} else empty end),
      (if ($tp | length) == 0 then {code: "EFFECTS_REQUIRED", detail: "no touch path"}
       else empty end),
      ($tp[] | select(type != "string" or . == "" or . == "." or test("^[^/]+/$")
          or (test("^[^/]+$") and inarr($dirs; .)))
        | {code: "EFFECTS_REQUIRED", detail: "touch path \(.) is a single top-level directory"}),
      (if $tv == null then empty else ($tv.bodyLines[] | . as $l
        | select(hasline($body0; $l) | not)
        | if $l == "Repro" and inarr($ls; "suspected") and (inarr(["P0", "P1"]; $p.priority) | not)
            then empty
          elif $l == "Repro"
            then {code: "TEMPLATE_FIELD", detail: "Repro (or label suspected below P1)"}
          else {code: "TEMPLATE_FIELD", detail: $l} end) end),
      (if ($body | utf8bytelength) > $v.limits.bodyBytes
         then {code: "BODY_TOO_LONG", detail: "\($body | utf8bytelength) bytes"} else empty end),
      (if ($ti | utf8bytelength) == 0 or ($ti | utf8bytelength) > $v.limits.titleBytes
          or ($ti | test("^[^\\s:]+:")) or ($ti | test("(^|[^A-Za-z0-9-])V1-[0-9]{4}"))
         then {code: "TEMPLATE_FIELD", detail: "title"} else empty end),
      (if ($p.acceptanceCriteria // [] | length) == 0
         then {code: "TEMPLATE_FIELD", detail: "acceptanceCriteria"} else empty end),
      policydiff
    ] as $refusals
  | ($stored // "" | split(".")) as $s
  | {refusals: $refusals, key: $stored, holders: $holders,
     siblings: (if $t == "delivery" or ($s | length) < 3 then [] else
       [$index[] | select(.status | openst) | select(.key != $stored) | . as $h
        | (.key | split(".")) as $o
        | select(($o | length) >= 3 and $o[0] == $s[0] and $o[-1] == $s[-1] and $o[1] != $s[1])]
       end),
     payload: ($p | del(.identityKey, .distinctFrom, .oldId)
       | .labels = ($ls
         + (if ($klab | length) == 0 and $stored != null then ["key:\($stored)"] else [] end)
         + (if ($prod | length) == 0 and $cv != null and $cv.product != null
              then ["product:\($cv.product)"] else [] end) | unique)
       | if (.requirementRefs | type) == "array" then .requirementRefs |= unique else . end
       | .body = $body)};
def lines($r):
  ($r.siblings[] | "SIBLING\t\(.id)\t\(.key)\t\(.title)"),
  (if ($r.refusals | length) > 0 then ($r.refusals[] | "REFUSED\t\(.code)\t\(.detail)")
   else "PAYLOAD\t\($r.payload | tojson)" end);
if $mode == "create" then lines(check(.[0]; $idx))
else
  reduce .[] as $p ({seeds: [], held: [], folded: []};
    . as $st
    | ($idx + ($st.seeds | map({id: .oldId, status: "OPEN", key: .key, title: .payload.title,
        df: .df})))
      as $index
    | check($p; $index) as $r
    | ($r.refusals | map(select(.code != "DUPLICATE_KEY"))) as $other
    | ($r.holders | map(select(.df != null))) as $seedholders
    | ($seedholders | map(select(.df | any(.id == $p.oldId) | not))) as $unnamed
    | if ($other | length) > 0 then
        .held += [{oldId: $p.oldId, codes: ($r.refusals | map(.code) | unique | join(",")),
          detail: ($r.refusals | map("\(.code): \(.detail)") | join(" | "))}]
      elif ($r.refusals | length) > 0 or ($unnamed | length) > 0 then
        .folded += [{oldId: $p.oldId, into: ($r.holders[0].id), key: $r.key}]
      else .seeds += [{oldId: $p.oldId, key: $r.key, payload: $r.payload,
        df: ($p.distinctFrom // [])}] end)
  | (.seeds[] | "SEED\t\({oldId, payload} | tojson)"),
    (.held[] | "HELD\t\(.oldId)\t\(.codes)\t\(.detail | gsub("[\t\n]"; " "))"),
    (.folded[] | "FOLDED\t\(.oldId)\t\(.into)\t\(.key)")
end') || die 'the gate program failed on its input'

if test "$mode" = seed; then
  mkdir -p "$out"
  printf '%s\n' "$result" | sed -n 's/^SEED	//p' > "$out/seed.jsonl"
  printf '%s\n' "$result" | sed -n 's/^HELD	//p' > "$out/held.tsv"
  printf '%s\n' "$result" | sed -n 's/^FOLDED	//p' > "$out/folded.tsv"
  printf 'seed\t%s\nheld\t%s\nfolded\t%s\n' "$(grep -c . "$out/seed.jsonl" || true)" \
    "$(grep -c . "$out/held.tsv" || true)" "$(grep -c . "$out/folded.tsv" || true)"
  exit 0
fi
printf '%s\n' "$result" | grep '^SIBLING	' >&2 || true
if printf '%s\n' "$result" | grep -q '^REFUSED	'; then
  printf '%s\n' "$result" | grep '^REFUSED	'
  exit 1
fi
printf '%s\n' "$result" | grep '^PAYLOAD	'
if test "$apply" = 1; then
  printf '%s\n' "$result" | sed -n 's/^PAYLOAD	//p' |
    (cd "$store" && corvint-tasks ticket create --request-id "$request" --payload-stdin)
fi
