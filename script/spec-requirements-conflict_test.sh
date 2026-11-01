#!/bin/sh
set -eu

# Covers SRG-V0-013: two lanes that each add one requirement to the same spec and run
# `make spec-requirements` conflict only inside that spec's requirement list and the
# REQUIREMENTS.tsv rows, never in the spec header, the Agent digest, INDEX.json or README.md.
# The TSV conflict resolves by regeneration, and a lane on another spec merges cleanly.
# Covers SRG-V0-016: lanes that add requirements to different specs, or to different sections of
# one spec so that each shifts the other's definition lines, or that only edit prose no indexed
# field reads, merge in both orders with no conflict in any file, and every merged generated file
# is the regenerated one. A reflow that changes a requirement's first line, and so its title, is
# the recorded limit: next to an inserted row it conflicts in REQUIREMENTS.tsv. The test drives the real Makefile target against a temporary repository
# holding a copy of the generator.
# Covers SRG-V0-981..983 (V1-3458): two lanes that amend different requirements of one spec on
# their own definition lines and add different phony make targets, each on its own `.PHONY:` line,
# merge in both orders with no conflict, while the same amendments as header segments and the same
# targets on one shared `.PHONY:` line conflict; the repository Makefile has no shared line.
# Covers SRG-V0-920..922 (V1-3448, V1-3338, V1-3437): spec-requirements-resolve regenerates and
# exits 0 when only a spec conflicted and when no merge is in progress; two lanes that each add a
# new spec and repin a different use-case receipt conflict in INDEX.json, README.md and the ledger,
# make spec-requirements refuses that merge with one line naming spec-requirements-resolve (also
# once the conflicted INDEX.json is staged with its markers), and the resolve alone finishes it.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-spec-conflict-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

# The merges must not depend on the caller's Git configuration (conflict style, signing, hooks).
GIT_CONFIG_GLOBAL=/dev/null
GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL GIT_CONFIG_NOSYSTEM
repo=$test_root/repo
mkdir -p "$repo/script" "$repo/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$source_root/script/spec-requirements-resolve.sh" \
  "$source_root/script/repin-use-case-receipts.sh" "$repo/script/"
git() { command git -C "$repo" -c user.name=t -c user.email=t@example.invalid -c merge.conflictStyle=merge "$@"; }
git init -q -b main

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

regen() {
  git add -A -- docs/specs
  make -s --no-print-directory -f "$source_root/Makefile" -C "$repo" spec-requirements
  git add -A -- docs/specs
}

cat > "$repo/docs/specs/shared.md" <<'SPEC'
# Shared spec

Intent status: accepted (history: REQUIREMENTS.tsv acceptance columns and docs/build-log)
Delivery status: experimental

## Agent digest
- Claim: The shared spec is where two lanes add requirements at once.
- Status: accepted (history: REQUIREMENTS.tsv acceptance columns and docs/build-log) / experimental

## Requirements

- SHR-V0-001: (accepted, decision 0001) The first clause.
- SHR-V0-002: (accepted, decision 0001) The second clause.

## Failure modes

None yet.
SPEC
cat > "$repo/docs/specs/other.md" <<'SPEC'
# Other spec

Intent status: accepted
Delivery status: implemented

## Agent digest
- Claim: The other spec belongs to a third lane.

## Requirements

- OTH-V0-001: (accepted, decision 0002) The other clause.
SPEC
cat > "$repo/docs/specs/README.md" <<'MD'
# Specs

| Scope | Contract | Intent | Delivery | Binding evidence or next gate |
| --- | --- | --- | --- | --- |
MD
regen
git commit -q -m base
base=$(git rev-parse HEAD)

# add_requirement BRANCH SPEC AFTER-ID LINE [FROM]: one lane, branched from FROM (default the
# base commit), adds one requirement after AFTER-ID.
add_requirement() {
  git checkout -q -b "$1" "${5:-$base}"
  awk -v after="$3" -v line="$4" '{ print } index($0, "- " after ":") == 1 { print line }' \
    "$repo/docs/specs/$2" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/$2"
  regen
  git commit -q -m "$1"
}
add_requirement lane-a shared.md SHR-V0-002 \
  "- SHR-V0-003: (accepted under the owner's standing auto-accept (2026-10-11); V1-9001) Lane A's clause."
add_requirement lane-b shared.md SHR-V0-002 \
  "- SHR-V0-004: (accepted under the owner's standing auto-accept (2026-10-11); V1-9002) Lane B's clause."
add_requirement lane-c other.md OTH-V0-001 \
  "- OTH-V0-002: (accepted under the owner's standing auto-accept (2026-10-11); V1-9003) Lane C's clause."

git checkout -q lane-a
if git merge -q --no-edit lane-b > /dev/null 2>&1; then fail "two lanes on one spec merged without a conflict"; fi
conflicted=$(git diff --name-only --diff-filter=U | tr '\n' ' ')
test "$conflicted" = "docs/specs/REQUIREMENTS.tsv docs/specs/shared.md " ||
  fail "the conflict touched more than the spec and REQUIREMENTS.tsv: $conflicted"

# Every conflict marker in the spec lies inside its requirement list.
awk '
  /^## Requirements$/ { inside = 1; next }
  /^## / { inside = 0 }
  /^(<<<<<<<|=======|>>>>>>>)/ { markers++; if (!inside) bad = 1 }
  END { exit !(markers == 3 && !bad) }
' "$repo/docs/specs/shared.md" || fail "a spec conflict fell outside the requirement list"
# Every conflicted TSV line is a row of the two new requirements.
awk '
  /^<<<<<<</ { inside = 1; next }
  /^>>>>>>>/ { inside = 0; next }
  /^=======$/ { next }
  inside && !/^SHR-V0-00[34]\t/ { bad = 1 }
  END { exit bad }
' "$repo/docs/specs/REQUIREMENTS.tsv" || fail "a REQUIREMENTS.tsv conflict held more than the new rows"

# Resolution: keep both requirement lines and stage the spec; the resolve target refuses until
# then, and afterwards regenerates the TSV rather than hand-merging it (SRG-V0-041).
resolve() { make -s --no-print-directory -f "$source_root/Makefile" -C "$repo" spec-requirements-resolve > /dev/null 2>&1; }
resolve && fail "spec-requirements-resolve ran with the spec still conflicted"
sed -e '/^<<<<<<< /d' -e '/^=======$/d' -e '/^>>>>>>> /d' "$repo/docs/specs/shared.md" > "$test_root/spec"
cp "$test_root/spec" "$repo/docs/specs/shared.md"
git add -- docs/specs/shared.md
resolve || fail "spec-requirements-resolve refused a merge whose only conflict left is the TSV"
git commit -q --no-edit
for id in SHR-V0-003 SHR-V0-004; do
  grep -q "^$id	docs/specs/shared.md	.*	standing	" "$repo/docs/specs/REQUIREMENTS.tsv" || fail "$id lost in the merge"
done

# A lane on another spec merges with no conflict at all.
git merge -q --no-edit lane-c > /dev/null 2>&1 || fail "a lane on another spec conflicted"
for file in REQUIREMENTS.tsv INDEX.json README.md; do
  case $file in
    REQUIREMENTS.tsv) flag= ;;
    INDEX.json) flag=--index ;;
    README.md) flag=--readme ;;
  esac
  (cd "$repo" && script/gen-spec-requirements.sh $flag) | cmp - "$repo/docs/specs/$file" ||
    fail "the merged $file is not the regenerated one"
done
grep -q '^OTH-V0-002	' "$repo/docs/specs/REQUIREMENTS.tsv" || fail "OTH-V0-002 lost in the merge"

# SRG-V0-016: a spec with two requirement sections, separated by prose. Every lane below branches
# from this commit.
git checkout -q -b sections "$base"
cat > "$repo/docs/specs/sections.md" <<'SPEC'
# Sections spec

Intent status: accepted
Delivery status: experimental

## Agent digest
- Claim: The sections spec has two requirement sections.

## Introduction

The introduction is prose.

## Requirements

- SEC-V0-001: (accepted, decision 0003) The first clause.
- SEC-V0-002: (accepted, decision 0003) The second clause.

## Later requirements

Prose separates the two sections.

- SEC-V0-010: (accepted, decision 0003) The tenth clause.
- SEC-V0-011: (accepted, decision 0003) The eleventh clause.

## Failure modes

None yet.
SPEC
regen
git commit -q -m sections
sections=$(git rev-parse HEAD)
standing="(accepted under the owner's standing auto-accept (2026-10-11); V1-9004)"
# Lane X adds to the first section, which shifts every definition line of the second.
add_requirement lane-x sections.md SEC-V0-002 "- SEC-V0-003: $standing Lane X's clause." "$sections"
# Lane Y adds to the second section.
add_requirement lane-y sections.md SEC-V0-011 "- SEC-V0-012: $standing Lane Y's clause." "$sections"
# Lane O adds to another spec.
add_requirement lane-o other.md OTH-V0-001 "- OTH-V0-003: $standing Lane O's clause." "$sections"
# Lane Z edits only the spec's prose, above every definition: it shifts every line and adds no row.
git checkout -q -b lane-z "$sections"
awk '{ print } $0 == "The introduction is prose." { print ""; print "Lane Z adds a paragraph."; print "It is two lines long." }' \
  "$repo/docs/specs/sections.md" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/sections.md"
regen
git commit -q -m lane-z

# merges_cleanly A B IDS: B merges into A, and A into B, with no conflict; each merged tree's
# generated files are the regenerated ones and keep every listed requirement.
merges_cleanly() {
  for order in "$1 $2" "$2 $1"; do
    set -- $order "$3"
    git checkout -q -b "merge-$1-$2" "$1"
    git merge -q --no-edit "$2" > /dev/null 2>&1 || fail "$2 conflicted when merged into $1: $(git diff --name-only --diff-filter=U | tr '\n' ' ')"
    for file in REQUIREMENTS.tsv INDEX.json README.md; do
      case $file in
        REQUIREMENTS.tsv) flag= ;;
        INDEX.json) flag=--index ;;
        README.md) flag=--readme ;;
      esac
      (cd "$repo" && script/gen-spec-requirements.sh $flag) | cmp -s - "$repo/docs/specs/$file" ||
        fail "merging $2 into $1 left a $file that is not the regenerated one"
    done
    for id in $3; do
      grep -q "^$id	" "$repo/docs/specs/REQUIREMENTS.tsv" || fail "$id lost merging $2 into $1"
    done
  done
}
merges_cleanly lane-x lane-y 'SEC-V0-003 SEC-V0-012'
merges_cleanly lane-x lane-z 'SEC-V0-003'
merges_cleanly lane-y lane-z 'SEC-V0-012'
merges_cleanly lane-x lane-o 'SEC-V0-003 OTH-V0-003'

# SRG-V0-016 limit: a reflow that leaves a requirement's meaning but changes its first line changes
# that row's title, so it is not a prose-only edit under SRG-V0-016. Lane W inserts a requirement
# right after the reflowed one: the spec merges cleanly (an unchanged continuation line separates
# the two edits), but the two REQUIREMENTS.tsv rows are adjacent and conflict.
git checkout -q -b wrapped "$sections"
awk '$0 == "- SEC-V0-010: (accepted, decision 0003) The tenth clause." {
       print "- SEC-V0-010: (accepted, decision 0003) The tenth clause"; print "  continues here"; print "  and ends here."; next }
     { print }' "$repo/docs/specs/sections.md" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/sections.md"
regen
git commit -q -m wrapped
wrapped=$(git rev-parse HEAD)
git checkout -q -b lane-r "$wrapped"
awk '$0 == "- SEC-V0-010: (accepted, decision 0003) The tenth clause" { print "- SEC-V0-010: (accepted, decision 0003) The tenth clause continues"; next }
     $0 == "  continues here" { print "  here"; next }
     { print }' "$repo/docs/specs/sections.md" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/sections.md"
regen
git commit -q -m lane-r
git diff --quiet "$wrapped" lane-r -- docs/specs/REQUIREMENTS.tsv && fail "lane R's reflow left the SEC-V0-010 title unchanged"
git checkout -q -b lane-w "$wrapped"
awk '{ print } $0 == "  and ends here." { print "- SEC-V0-013: '"$standing"' Lane W'"'"'s clause." }' \
  "$repo/docs/specs/sections.md" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/sections.md"
regen
git commit -q -m lane-w
for order in 'lane-r lane-w' 'lane-w lane-r'; do
  set -- $order
  git checkout -q -b "merge-$1-$2" "$1"
  if git merge -q --no-edit "$2" > /dev/null 2>&1; then fail "a reflowed row and a row inserted after it merged without a conflict"; fi
  conflicted=$(git diff --name-only --diff-filter=U | tr '\n' ' ')
  test "$conflicted" = "docs/specs/REQUIREMENTS.tsv " ||
    fail "merging $2 into $1 conflicted in $conflicted, want only REQUIREMENTS.tsv"
  # Resolution: the spec merged; regenerate the index rather than hand-merge it (SRG-V0-013).
  git checkout -q --theirs -- docs/specs/REQUIREMENTS.tsv
  regen
  git commit -q --no-edit
  grep -q '^SEC-V0-010	docs/specs/sections.md	.*The tenth clause continues	' "$repo/docs/specs/REQUIREMENTS.tsv" &&
    grep -q '^SEC-V0-013	' "$repo/docs/specs/REQUIREMENTS.tsv" || fail "the regenerated merge of $2 into $1 lost a row"
done

# SRG-V0-041: two lanes that each add a new spec conflict in INDEX.json and README.md (and in
# REQUIREMENTS.tsv when the new rows are adjacent); one command regenerates and stages all three,
# in either merge order, to the fresh generation from the merged specs.
new_spec() {
  git checkout -q -b "$1" "$base"
  printf '# %s spec\n\nIntent status: accepted\nDelivery status: experimental\n\n## Agent digest\n- Claim: %s adds a spec.\n\n## Requirements\n\n- %s: (accepted, decision 0004) The clause.\n' \
    "$2" "$2" "$3" > "$repo/docs/specs/$2.md"
  regen
  git commit -q -m "$1"
}
new_spec lane-n1 newone NEW-V0-001
new_spec lane-n2 newtwo NWT-V0-001
for order in 'lane-n1 lane-n2' 'lane-n2 lane-n1'; do
  set -- $order
  git checkout -q -b "merge-$1-$2" "$1"
  if git merge -q --no-edit "$2" > /dev/null 2>&1; then fail "two new specs merged without a conflict; drop the SRG-V0-016 limit"; fi
  for path in $(git diff --name-only --diff-filter=U); do
    case $path in
      docs/specs/INDEX.json | docs/specs/README.md | docs/specs/REQUIREMENTS.tsv) ;;
      *) fail "two new specs conflicted in $path" ;;
    esac
  done
  resolve || fail "spec-requirements-resolve refused two new specs merging $2 into $1"
  test -z "$(git diff --name-only --diff-filter=U)" || fail "spec-requirements-resolve left a conflict"
  git commit -q --no-edit
  for file in REQUIREMENTS.tsv INDEX.json README.md; do
    case $file in
      REQUIREMENTS.tsv) flag= ;;
      INDEX.json) flag=--index ;;
      README.md) flag=--readme ;;
    esac
    (cd "$repo" && script/gen-spec-requirements.sh $flag) | cmp -s - "$repo/docs/specs/$file" ||
      fail "the resolved merge of $2 into $1 left a $file that is not the regenerated one"
  done
  for id in NEW-V0-001 NWT-V0-001; do
    grep -q "^$id	" "$repo/docs/specs/REQUIREMENTS.tsv" || fail "$id lost merging $2 into $1"
  done
  grep -q 'newone.md' "$repo/docs/specs/INDEX.json" && grep -q 'newtwo.md' "$repo/docs/specs/README.md" ||
    fail "the resolved merge of $2 into $1 lost a new spec's INDEX.json entry or README.md row"
done


# SRG-V0-982: each phony target has its own `.PHONY:` line. multi_phony FILE names every line that
# declares more than one.
multi_phony() {
  awk -v file="$1" '/^[ \t]*\.PHONY[ \t]*:/ { line = $0; sub(/#.*/, "", line); sub(/^[^:]*:/, "", line); n = split(line, words, /[ \t]+/)
    for (i = 1; i <= n; i++) if (words[i] == "") --count; count += n
    if (count > 1) printf "%s:%d: .PHONY declares %d targets\n", file, NR, count; count = 0 }' "$1"
}
shared_phony=$(multi_phony "$source_root/Makefile")
test -z "$shared_phony" || fail "declare each phony target on its own .PHONY: line next to its rule (SRG-V0-982): $shared_phony"

# SRG-V0-983 replay. The base adds a Makefile with one `.PHONY:` line per target and a control
# makefile with one shared line.
git checkout -q -b replay "$sections"
printf '.PHONY: alpha\nalpha:\n\t@echo alpha\n\n.PHONY: mid\nmid:\n\t@echo mid\n\n.PHONY: omega\nomega:\n\t@echo omega\n' > "$repo/Makefile"
printf '.PHONY:alpha omega # shared\nalpha:\n\t@echo alpha\n\nomega:\n\t@echo omega\n' > "$repo/shared.mk"
test "$(multi_phony "$repo/shared.mk")" = "$repo/shared.mk:1: .PHONY declares 2 targets" ||
  fail "a shared .PHONY: line was not named: $(multi_phony "$repo/shared.mk")"
git add -- Makefile shared.mk
git commit -q -m replay
replay=$(git rev-parse HEAD)

# amend_lane BRANCH ID TICKET TARGET AFTER: one lane amends ID's own note (SRG-V0-981) and adds the
# phony TARGET after the rule of AFTER.
amend_lane() {
  git checkout -q -b "$1" "$replay"
  awk -v id="$2" -v ticket="$3" 'index($0, "- " id ": (accepted, decision 0003) ") == 1 {
         sub(/\(accepted, decision 0003\)/, "(accepted, decision 0003; amended under the owner'"'"'s standing auto-accept (2026-10-31); " ticket ")")
         sub(/\.$/, ", amended.") }
       { print }' "$repo/docs/specs/sections.md" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/sections.md"
  awk -v target="$4" -v after="$5" '{ print } $0 == "\t@echo " after { printf "\n.PHONY: %s\n%s:\n\t@echo %s\n", target, target, target }' \
    "$repo/Makefile" > "$test_root/mk" && cp "$test_root/mk" "$repo/Makefile"
  git add -- Makefile
  regen
  git commit -q -m "$1"
}
amend_lane lane-am1 SEC-V0-001 V1-9101 beta alpha
amend_lane lane-am2 SEC-V0-011 V1-9102 psi mid
merges_cleanly lane-am1 lane-am2 'SEC-V0-001 SEC-V0-011'
for pair in 'SEC-V0-001 V1-9101' 'SEC-V0-011 V1-9102'; do
  set -- $pair
  grep -q "^$1	docs/specs/sections.md	.*	standing	decision 0003; 2026-10-31; $2\$" "$repo/docs/specs/REQUIREMENTS.tsv" ||
    fail "the amended $1 row does not carry standing and its anchors: $(grep "^$1	" "$repo/docs/specs/REQUIREMENTS.tsv")"
done
test "$(make -s --no-print-directory -C "$repo" alpha beta mid psi omega | tr '\n' ' ')" = 'alpha beta mid psi omega ' ||
  fail "the merged Makefile lost a target"
# SRG-V0-983 limit: amendments to adjacent definition lines still conflict, only in those spec lines
# and the two REQUIREMENTS.tsv rows.
amend_lane lane-am3 SEC-V0-002 V1-9103 gamma omega
git checkout -q -b merge-adjacent lane-am1
if git merge -q --no-edit lane-am3 > /dev/null 2>&1; then fail "adjacent amendments merged without a conflict; drop the SRG-V0-983 limit"; fi
conflicted=$(git diff --name-only --diff-filter=U | tr '\n' ' ')
test "$conflicted" = "docs/specs/REQUIREMENTS.tsv docs/specs/sections.md " ||
  fail "adjacent amendments conflicted in $conflicted, want only the spec and REQUIREMENTS.tsv"
git merge --abort

# Controls: the same amendments as header segments are refused by the generator (SRG-V0-980) and
# conflict, and the same targets appended to one shared `.PHONY:` line conflict.
control_lane() {
  git checkout -q -b "$1" "$replay"
  sed "s/^Intent status: accepted\$/Intent status: accepted; $2 amended under the owner's standing auto-accept (2026-10-31; $3)/" \
    "$repo/docs/specs/sections.md" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/sections.md"
  sed "1s/ #/ $4 #/" "$repo/shared.mk" > "$test_root/mk" && printf '\n%s:\n\t@echo %s\n' "$4" "$4" >> "$test_root/mk" &&
    cp "$test_root/mk" "$repo/shared.mk"
  make -np -f "$repo/shared.mk" 2> /dev/null | grep '^\.PHONY:' | tr ' ' '\n' | grep -qx "$4" ||
    fail "the control did not add $4 to the shared .PHONY: list"
  git add -- docs/specs/sections.md shared.mk
  ctl_err=$( (cd "$repo" && script/gen-spec-requirements.sh) 2>&1 > /dev/null) &&
    fail "a header segment naming $2 was not refused"
  case $ctl_err in
    *"docs/specs/sections.md:"[0-9]*": its Intent status names $2"*'(SRG-V0-980)'*) ;;
    *) fail "the header-segment refusal does not name $2 and the rule: $ctl_err" ;;
  esac
  git commit -q -m "$1"
}
control_lane lane-hs1 SEC-V0-001 V1-9101 beta
control_lane lane-hs2 SEC-V0-011 V1-9102 psi
git checkout -q -b merge-hs lane-hs1
if git merge -q --no-edit lane-hs2 > /dev/null 2>&1; then fail "header segments and a shared .PHONY: line merged without a conflict"; fi
conflicted=$(git diff --name-only --diff-filter=U | tr '\n' ' ')
test "$conflicted" = "docs/specs/sections.md shared.mk " ||
  fail "the controls conflicted in $conflicted, want the spec header and the shared .PHONY: line"
git merge --abort

# SRG-V0-920 (V1-3448): two lanes reword the same prose line of one spec; only the spec conflicts.
prose_lane() {
  git checkout -q -b "$1" "$base"
  sed "s/^None yet\.\$/$2/" "$repo/docs/specs/shared.md" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/shared.md"
  regen
  git commit -q -m "$1"
}
prose_lane lane-p1 'Lane one failure.'
prose_lane lane-p2 'Lane two failure.'
git checkout -q -b merge-prose lane-p1
if git merge -q --no-edit lane-p2 > /dev/null 2>&1; then fail "two prose rewordings merged without a conflict"; fi
conflicted=$(git diff --name-only --diff-filter=U | tr '\n' ' ')
test "$conflicted" = "docs/specs/shared.md " || fail "the prose conflict touched $conflicted, want only the spec"
sed -e '/^<<<<<<< /d' -e '/^=======$/d' -e '/^>>>>>>> /d' -e '/^Lane two failure\.$/d' "$repo/docs/specs/shared.md" > "$test_root/spec"
cp "$test_root/spec" "$repo/docs/specs/shared.md"
git add -- docs/specs/shared.md
out=$(make -s --no-print-directory -f "$source_root/Makefile" -C "$repo" spec-requirements-resolve 2>&1) ||
  fail "spec-requirements-resolve failed with no unmerged path: $out"
case $out in *'no unmerged path; regenerating anyway'*'regenerated and staged'*) ;; *) fail "resolve output: $out" ;; esac
git commit -q --no-edit
# No merge in progress: a staged spec edit leaves the index stale; the resolve regenerates it.
sed 's/^- SHR-V0-002: (accepted, decision 0001) The second clause\.$/- SHR-V0-002: (accepted, decision 0001) The second clause, reworded./' \
  "$repo/docs/specs/shared.md" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/shared.md"
git add -- docs/specs/shared.md
make -s --no-print-directory -f "$source_root/Makefile" -C "$repo" spec-requirements-resolve > /dev/null 2>&1 ||
  fail "spec-requirements-resolve failed outside a merge"
grep -q 'reworded' "$repo/docs/specs/REQUIREMENTS.tsv" && git diff --quiet -- docs/specs ||
  fail "spec-requirements-resolve outside a merge did not regenerate and stage the index"
git reset -q --hard

# SRG-V0-921/922 (V1-3338, V1-3437): two lanes each add a new spec and repin a different receipt.
git checkout -q -b receipts "$base"
uc=conformance/use-cases-v0
mkdir -p "$repo/$uc/receipts/UC-A" "$repo/$uc/receipts/UC-B" "$repo/docs/subjects"
sha() { perl -MDigest::SHA=sha256_hex -0777 -ne 'print sha256_hex($_)' "$repo/$1"; }
for x in A B; do
  printf 'subject %s v1\n' "$x" > "$repo/docs/subjects/$x.md"
  printf '{\n  "repositoryRevision": "0000000000000000000000000000000000000000",\n  "subjects": [\n    {\n      "path": "docs/subjects/%s.md",\n      "sha256": "%s"\n    }\n  ]\n}\n' \
    "$x" "$(sha "docs/subjects/$x.md")" > "$repo/$uc/receipts/UC-$x/contract.json"
done
printf '{\n  "evidenceClasses": ["contract"],\n  "useCases": [\n    {"evidence": {"contract": {"path": "%s", "sha256": "%s"}}, "id": "UC-A"},\n    {"evidence": {"contract": {"path": "%s", "sha256": "%s"}}, "id": "UC-B"}\n  ]\n}\n' \
  "$uc/receipts/UC-A/contract.json" "$(sha "$uc/receipts/UC-A/contract.json")" \
  "$uc/receipts/UC-B/contract.json" "$(sha "$uc/receipts/UC-B/contract.json")" > "$repo/$uc/ledger.json"
git add -A && git commit -q -m receipts
rbase=$(git rev-parse HEAD)
receipt_lane() {
  git checkout -q -b "$1" "$rbase"
  printf '# %s spec\n\nIntent status: accepted\nDelivery status: experimental\n\n## Agent digest\n- Claim: %s.\n\n## Requirements\n\n- %s-V0-001: (accepted, decision 0004) The clause.\n' \
    "$2" "$2" "$3" > "$repo/docs/specs/$1.md"
  printf 'subject %s v2\n' "$4" > "$repo/docs/subjects/$4.md"
  (cd "$repo" && script/repin-use-case-receipts.sh > /dev/null) || fail "$1: the fixture repin failed"
  regen
  git add -A && git commit -q -m "$1"
}
receipt_lane lane-r1 Alpha ALP A
receipt_lane lane-r2 Beta BET B
git checkout -q -b merge-receipts lane-r1
if git merge -q --no-edit lane-r2 > /dev/null 2>&1; then fail "two new specs and two receipt repins merged without a conflict"; fi
conflicted=$(git diff --name-only --diff-filter=U | tr '\n' ' ')
case $conflicted in *docs/specs/INDEX.json*) ;; *) fail "the new-spec merge did not conflict in INDEX.json: $conflicted" ;; esac
case $conflicted in *"$uc/ledger.json"*) ;; *) fail "the receipt merge did not conflict in the ledger: $conflicted" ;; esac
srq() { make -s --no-print-directory -f "$source_root/Makefile" -C "$repo" spec-requirements 2>&1 > /dev/null; }
err=$(srq) && fail "make spec-requirements ran over an unmerged INDEX.json"
# One line of its own; make adds its usual `make: *** ... Error 2` line.
test "$(printf '%s\n' "$err" | grep -vc '^make: \*\*\* ')" = 1 && case $err in *'docs/specs/INDEX.json'*'make spec-requirements-resolve'*) true ;; *) false ;; esac ||
  fail "the unmerged refusal is not one line naming spec-requirements-resolve: $err"
git add -- docs/specs/INDEX.json
err=$(srq) && fail "make spec-requirements ran over a staged INDEX.json holding conflict markers"
case $err in *'make spec-requirements-resolve'*) ;; *) fail "the staged-marker refusal does not name spec-requirements-resolve: $err" ;; esac
out=$(make -s --no-print-directory -f "$source_root/Makefile" -C "$repo" spec-requirements-resolve 2>&1) ||
  fail "spec-requirements-resolve did not finish the receipt merge: $out"
test -z "$(git diff --name-only --diff-filter=U)" || fail "paths still unmerged after the resolve"
git commit -q --no-edit
for file in REQUIREMENTS.tsv INDEX.json README.md; do
  case $file in REQUIREMENTS.tsv) flag= ;; INDEX.json) flag=--index ;; README.md) flag=--readme ;; esac
  (cd "$repo" && script/gen-spec-requirements.sh $flag) | cmp -s - "$repo/docs/specs/$file" || fail "the resolved $file is not the regenerated one"
done
grep -q '^ALP-V0-001	' "$repo/docs/specs/REQUIREMENTS.tsv" && grep -q '^BET-V0-001	' "$repo/docs/specs/REQUIREMENTS.tsv" ||
  fail "a new spec's requirement was lost in the resolve"
check=$( (cd "$repo" && script/repin-use-case-receipts.sh --check) 2>&1) || fail "receipts stale after the resolve: $check"
case $check in *'all 2 receipts current'*) ;; *) fail "receipt check after the resolve: $check" ;; esac

# SRG-V0-921 (review fix): a ledger conflict whose other side changed more than digests (here a
# claim beside UC-B's pin) is refused for a hand merge, writing nothing, instead of `--ours`
# silently dropping that change.
receipt_lane lane-r3 Gamma GAM A
git checkout -q -b lane-r4 "$rbase"
perl -0pi -e 's/("id": "UC-B")/"claim": "PROVEN", $1/' "$repo/$uc/ledger.json"
grep -q '"claim": "PROVEN"' "$repo/$uc/ledger.json" || fail "the fixture claim edit did not apply"
git add -A && git commit -q -m lane-r4
git checkout -q -b merge-claim lane-r3
if git merge -q --no-edit lane-r4 > /dev/null 2>&1; then fail "a digest repin and a claim edit merged without a ledger conflict"; fi
before=$(git diff | git hash-object --stdin)
rc=0
out=$(make -s --no-print-directory -f "$source_root/Makefile" -C "$repo" spec-requirements-resolve 2>&1) || rc=$?
[ "$rc" != 0 ] && case $out in *'changed more than digests in: '"$uc/ledger.json"*) true ;; *) false ;; esac ||
  fail "spec-requirements-resolve took --ours over a non-digest ledger change (exit $rc): $out"
test "$(git diff --name-only --diff-filter=U)" = "$uc/ledger.json" && test "$(git diff | git hash-object --stdin)" = "$before" ||
  fail "the refused resolve changed the merge state"
git merge --abort

printf 'ok\n'
