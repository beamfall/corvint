#!/usr/bin/env bash
# Tests for script/check-cem-base-retained.sh (decision 0588, V1-3758) on a fixture repository: the check
# keeps to Bash 3.2 (macOS /bin/bash), a map whose
# base the retained ref reaches passes, a new map on an unretained base is refused naming the map and the
# fix, a register label admits exactly its base, a stale or mismatched register row and an unknown label
# are refused, and with no retained ref the check says it was skipped.
set -uo pipefail
export LC_ALL=C
script=$(cd "$(dirname "$0")" && pwd)
check=$script/check-cem-base-retained.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-cem-base-retained-test.XXXXXX") || exit 1
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'check-cem-base-retained_test: FAIL line %s: %s\n' "${BASH_LINENO[0]}" "$*" >&2; exit 1; }
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

# 0. macOS /bin/bash is 3.2: the check uses no Bash 4 construct (associative arrays, mapfile,
# case-changing expansion, |&, &>>, ;;&, coproc, [[ -v ]]).
if grep -nE 'declare -[Aln]|local -n|mapfile|readarray|\$\{[A-Za-z_]+(,,|\^\^)\}|\|&|&>>|;;&|coproc|\[\[ -v ' "$check"; then
  fail "check uses a Bash 4 construct"
fi

repo=$tmp/repo
git init -q "$repo" || fail "git init"
cd "$repo" || fail "cd"
commit() { git commit -q --allow-empty -m "$1" || fail "commit $1"; git rev-parse HEAD; }
map() { mkdir -p .corvint/changes && printf '{"artifacts":[],"baseRevision":"%s","spec":"cem/1.0"}\n' "$2" > ".corvint/changes/$1.cem.json"; }
retained=$(commit retained)
git update-ref refs/remotes/origin/main "$retained"

# 1. No map and no register: passes.
"$check" > "$tmp/out" 2>&1 || fail "empty corpus refused: $(cat "$tmp/out")"

# 2. A map whose base origin/main reaches passes.
target=$(commit target)
map "$target" "$retained"
git add .corvint && git commit -q -m seal
"$check" > "$tmp/out" 2>&1 || fail "retained base refused: $(cat "$tmp/out")"

# 3. A new map on a base only a deleted branch held is refused, naming the map and the fix.
git checkout -q -b lineage "$retained"
orphan=$(commit orphan-base)
git checkout -q - && git branch -q -D lineage
map aaaa "$orphan"
git add .corvint && git commit -q -m seal-orphan
"$check" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q "\.corvint/changes/aaaa.cem.json: base $orphan is not reachable from a retained ref" "$tmp/out" &&
  grep -q 'fix: rebind the change on a base' "$tmp/out" || fail "unretained base not refused: $(cat "$tmp/out")"
# A base object missing from the repository is refused the same way.
map bbbb 0123456789abcdef0123456789abcdef01234567
git add .corvint && git commit -q -m seal-missing
"$check" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q 'bbbb.cem.json: base 0123456789abcdef0123456789abcdef01234567 is not reachable' "$tmp/out" ||
  fail "missing base not refused: $(cat "$tmp/out")"

# 4. Register labels admit exactly the labelled base.
printf '# target\tbase\tlabel\tretained\naaaa\t%s\tarchive-lineage\tarchive@x\nbbbb\t0123456789abcdef0123456789abcdef01234567\tbase-unavailable\tnone\n' "$orphan" > .corvint/cem-lineage.tsv
"$check" > "$tmp/out" 2>&1 && grep -q '2 labelled' "$tmp/out" || fail "labelled maps refused: $(cat "$tmp/out")"
printf 'aaaa\t%s\tarchive-lineage\tarchive@x\nbbbb\t%s\tbase-unavailable\tnone\n' "$retained" "$retained" > .corvint/cem-lineage.tsv
"$check" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q "labels aaaa with base $retained, but the map's base is $orphan" "$tmp/out" || fail "mismatched label passed: $(cat "$tmp/out")"
printf 'aaaa\t%s\tarchive-lineage\tarchive@x\nbbbb\t0123456789abcdef0123456789abcdef01234567\tbase-unavailable\tnone\ncccc\t%s\trebased-lane-base\tnone\n' "$orphan" "$retained" > .corvint/cem-lineage.tsv
"$check" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q 'labels cccc, but .corvint/changes/cccc.cem.json is not tracked' "$tmp/out" || fail "stale label passed: $(cat "$tmp/out")"
printf 'aaaa\t%s\ttrusted\tarchive@x\n' "$orphan" > .corvint/cem-lineage.tsv
"$check" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q "aaaa has unknown label 'trusted'" "$tmp/out" || fail "unknown label passed: $(cat "$tmp/out")"

# 5. Without a resolvable retained ref the check is skipped, saying so; a bad argument refuses.
git update-ref -d refs/remotes/origin/main
"$check" > "$tmp/out" 2>&1 && grep -q 'skipped: no retained ref resolves here' "$tmp/out" || fail "no retained ref: $(cat "$tmp/out")"
"$check" --retained > "$tmp/out" 2>&1
[[ $? == 2 ]] || fail "--retained without a ref did not refuse: $(cat "$tmp/out")"
# An explicit --retained ref replaces the default: the orphan base it reaches passes, the missing one does not.
rm .corvint/cem-lineage.tsv
"$check" --retained "$orphan" > "$tmp/out" 2>&1
[[ $? == 1 ]] && ! grep -q 'aaaa.cem.json' "$tmp/out" && grep -q 'bbbb.cem.json' "$tmp/out" ||
  fail "explicit --retained: $(cat "$tmp/out")"
echo "check-cem-base-retained_test: PASS"
