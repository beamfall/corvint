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
cp "$source_root/script/gen-spec-requirements.sh" "$source_root/script/spec-requirements-resolve.sh" "$repo/script/"
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
multi_phony() { awk -v file="$1" '/^\.PHONY:/ && NF > 2 { printf "%s:%d: .PHONY declares %d targets\n", file, NR, NF - 1 }' "$1"; }
shared_phony=$(multi_phony "$source_root/Makefile")
test -z "$shared_phony" || fail "declare each phony target on its own .PHONY: line next to its rule (SRG-V0-982): $shared_phony"

# SRG-V0-983 replay. The base adds a Makefile with one `.PHONY:` line per target and a control
# makefile with one shared line.
git checkout -q -b replay "$sections"
printf '.PHONY: alpha\nalpha:\n\t@echo alpha\n\n.PHONY: mid\nmid:\n\t@echo mid\n\n.PHONY: omega\nomega:\n\t@echo omega\n' > "$repo/Makefile"
printf '.PHONY: alpha omega\nalpha:\n\t@echo alpha\n\nomega:\n\t@echo omega\n' > "$repo/shared.mk"
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

# Controls: the same amendments as header segments are refused by the generator (SRG-V0-980) and
# conflict, and the same targets appended to one shared `.PHONY:` line conflict.
control_lane() {
  git checkout -q -b "$1" "$replay"
  sed "s/^Intent status: accepted\$/Intent status: accepted; $2 amended under the owner's standing auto-accept (2026-10-31; $3)/" \
    "$repo/docs/specs/sections.md" > "$test_root/spec" && cp "$test_root/spec" "$repo/docs/specs/sections.md"
  sed "1s/\$/ $4/" "$repo/shared.mk" > "$test_root/mk" && printf '\n%s:\n\t@echo %s\n' "$4" "$4" >> "$test_root/mk" &&
    cp "$test_root/mk" "$repo/shared.mk"
  git add -- docs/specs/sections.md shared.mk
  ctl_err=$( (cd "$repo" && script/gen-spec-requirements.sh) 2>&1 > /dev/null) &&
    fail "a header segment naming $2 was not refused"
  case $ctl_err in
    *"docs/specs/sections.md: its Intent status names $2"*'(SRG-V0-980)'*) ;;
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

printf 'ok\n'
