#!/bin/sh
set -eu

# Covers SRG-V0-013: two lanes that each add one requirement to the same spec and run
# `make spec-requirements` conflict only inside that spec's requirement list and the
# REQUIREMENTS.tsv rows, never in the spec header, the Agent digest, INDEX.json or README.md.
# The TSV conflict resolves by regeneration, and a lane on another spec merges cleanly. The test
# drives the real Makefile target against a temporary repository holding a copy of the generator.

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
cp "$source_root/script/gen-spec-requirements.sh" "$repo/script/"
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

# add_requirement BRANCH SPEC AFTER-ID LINE: one lane appends one requirement after AFTER-ID.
add_requirement() {
  git checkout -q -b "$1" "$base"
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

# Resolution: keep both requirement lines, then regenerate rather than hand-merge the TSV.
sed -e '/^<<<<<<< /d' -e '/^=======$/d' -e '/^>>>>>>> /d' "$repo/docs/specs/shared.md" > "$test_root/spec"
cp "$test_root/spec" "$repo/docs/specs/shared.md"
git checkout -q --theirs -- docs/specs/REQUIREMENTS.tsv
regen
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

printf 'ok\n'
