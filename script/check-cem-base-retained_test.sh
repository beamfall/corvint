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

run_check() {
  [[ ! -d .corvint ]] || git add -A -- .corvint
  "$check" "$@"
}

# 1. No map and no register: passes.
run_check > "$tmp/out" 2>&1 || fail "empty corpus refused: $(cat "$tmp/out")"

# CEM-V1-7901 also covers an unsealed committed sidecar, before its archive rename.
mkdir -p .corvint
printf '{"baseRevision":"0123456789abcdef0123456789abcdef01234567"}\n' > .corvint/change.cem.json
git add .corvint/change.cem.json
run_check > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q '.corvint/change.cem.json: base' "$tmp/out" || fail "unsealed sidecar bypassed retention"
printf '{"baseRevision":"%s"}\n' "$retained" > .corvint/change.cem.json
"$check" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q '.corvint/change.cem.json: base' "$tmp/out" || fail "working bytes hid an unretained staged base"
git rm --cached -f -q .corvint/change.cem.json
rm .corvint/change.cem.json

# 2. A map whose base origin/main reaches passes.
target=$(commit target)
map "$target" "$retained"
git add .corvint && git commit -q -m seal
run_check > "$tmp/out" 2>&1 || fail "retained base refused: $(cat "$tmp/out")"

# 3. A new map on a base only a deleted branch held is refused, naming the map and the fix.
git checkout -q -b lineage "$retained"
orphan=$(commit orphan-base)
git checkout -q - && git branch -q -D lineage
map aaaa "$orphan"
git add .corvint && git commit -q -m seal-orphan
run_check > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q "\.corvint/changes/aaaa.cem.json: base $orphan is not reachable from a retained ref" "$tmp/out" &&
  grep -q 'fix: rebind the change on a base' "$tmp/out" || fail "unretained base not refused: $(cat "$tmp/out")"
# A base object missing from the repository is refused the same way.
map bbbb 0123456789abcdef0123456789abcdef01234567
git add .corvint && git commit -q -m seal-missing
run_check > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q 'bbbb.cem.json: base 0123456789abcdef0123456789abcdef01234567 is not reachable' "$tmp/out" ||
  fail "missing base not refused: $(cat "$tmp/out")"

# CEM-V1-7901: a newly supplied label cannot turn an unretained base into retained evidence.
printf 'aaaa\t%s\tarchive-lineage\tarchive@invented\n' "$orphan" > .corvint/cem-lineage.tsv
run_check > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q 'aaaa.cem.json: base' "$tmp/out" || fail "new label bypassed retention: $(cat "$tmp/out")"

# 4. Historical labels admit exactly the labelled base, without claiming retention.
printf '# target\tbase\tlabel\tretained\naaaa\t%s\tarchive-lineage\tarchive@x\nbbbb\t0123456789abcdef0123456789abcdef01234567\tbase-unavailable\tnone\n' "$orphan" > .corvint/cem-lineage.tsv
git add .corvint/cem-lineage.tsv && git commit -q -m legacy-labels
git update-ref refs/remotes/origin/main HEAD
run_check > "$tmp/out" 2>&1 && grep -q '2 labelled' "$tmp/out" || fail "labelled maps refused: $(cat "$tmp/out")"
printf 'aaaa\t%s\tarchive-lineage\tarchive@x\nbbbb\t%s\tbase-unavailable\tnone\n' "$retained" "$retained" > .corvint/cem-lineage.tsv
run_check > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q "labels aaaa with base $retained, but the map's base is $orphan" "$tmp/out" || fail "mismatched label passed: $(cat "$tmp/out")"
printf 'aaaa\t%s\tarchive-lineage\tarchive@x\nbbbb\t0123456789abcdef0123456789abcdef01234567\tbase-unavailable\tnone\ncccc\t%s\trebased-lane-base\tnone\n' "$orphan" "$retained" > .corvint/cem-lineage.tsv
run_check > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q 'labels cccc, but .corvint/changes/cccc.cem.json is not tracked' "$tmp/out" || fail "stale label passed: $(cat "$tmp/out")"
printf 'aaaa\t%s\ttrusted\tarchive@x\n' "$orphan" > .corvint/cem-lineage.tsv
run_check > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q "aaaa has unknown label 'trusted'" "$tmp/out" || fail "unknown label passed: $(cat "$tmp/out")"

# CEM-V1-7901: a new label can name a real retained ref; a raw SHA or wrong ref cannot.
map cccc "$orphan"
git add .corvint/changes/cccc.cem.json
printf 'aaaa\t%s\tarchive-lineage\tarchive@x\nbbbb\t0123456789abcdef0123456789abcdef01234567\tbase-unavailable\tnone\ncccc\t%s\tarchive-lineage\trefs/archive/cem-base\n' "$orphan" "$orphan" > .corvint/cem-lineage.tsv
git update-ref refs/archive/cem-base "$orphan"
run_check > "$tmp/out" 2>&1 || fail "real retained ref refused: $(cat "$tmp/out")"
sed "s,refs/archive/cem-base,$orphan," .corvint/cem-lineage.tsv > "$tmp/register"
cp "$tmp/register" .corvint/cem-lineage.tsv
run_check > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q 'cccc.cem.json: base' "$tmp/out" || fail "raw object ID counted as retained"
git rm --cached -f -q .corvint/changes/cccc.cem.json
rm .corvint/changes/cccc.cem.json

# 5. Without a resolvable retained ref the check is skipped, saying so; a bad argument refuses.
git update-ref -d refs/remotes/origin/main
run_check > "$tmp/out" 2>&1 && grep -q 'skipped: no retained ref resolves here' "$tmp/out" || fail "no retained ref: $(cat "$tmp/out")"
run_check --retained > "$tmp/out" 2>&1
[[ $? == 2 ]] || fail "--retained without a ref did not refuse: $(cat "$tmp/out")"
# An explicit --retained ref replaces the default: the orphan base it reaches passes, the missing one does not.
rm .corvint/cem-lineage.tsv
run_check --retained "$orphan" > "$tmp/out" 2>&1
[[ $? == 1 ]] && ! grep -q 'aaaa.cem.json' "$tmp/out" && grep -q 'bbbb.cem.json' "$tmp/out" ||
  fail "explicit --retained: $(cat "$tmp/out")"
echo "check-cem-base-retained_test: PASS"
