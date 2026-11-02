#!/usr/bin/env bash
# Tests for script/batch-assemble.sh (ASD-V0-785..786, NEW-D1): one pass merges sealed lanes into
# the batch with attribute-free ort, keeps the batch side of a conflict with -Xours and lists its
# paths, skips what -Xours cannot resolve and what is already merged, never runs a configured merge
# driver, writes each --dispositions line into its ort-ours merge message, and refuses a dirty
# worktree, a detached HEAD, an unknown branch or a bad disposition without moving the branch.
set -uo pipefail
export LC_ALL=C
script=$(cd "$(dirname "$0")" && pwd)/batch-assemble.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-batch-assemble-test.XXXXXX") || exit 1
tmp=$(cd "$tmp" && pwd)
trap '/bin/rm -rf "$tmp"' EXIT
failed=0
fail() {
  failed=1
  printf 'batch-assemble_test: FAIL line %s: %s\n' "${BASH_LINENO[0]}" "$*" >&2
  [[ ! -f $tmp/out ]] || sed 's/^/  | /' "$tmp/out" >&2
}
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid \
  GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
repo=$tmp/repo
git init -q -b batch "$repo" || exit 1
cd "$repo" || exit 1
git config merge.marker.driver "touch '$tmp/driver-ran'; exit 0"
printf 'notes.txt merge=marker\n' > .gitattributes
printf 'base\n' > notes.txt
printf 'one\ntwo\n' > code.txt
git add -A && git commit -qm base
base=$(git rev-parse HEAD)
lane() { # lane NAME: a branch from base with the staged edits the caller makes
  git checkout -q -b "$1" "$base"
}
lane lane-a && printf 'base\nlane a\n' > notes.txt && printf 'a\n' > a.txt && git add -A && git commit -qm a
lane lane-b && printf 'base\nlane b\n' > notes.txt && git add -A && git commit -qm b
lane lane-c && printf 'c\n' > c.txt && git add -A && git commit -qm c
lane lane-e && printf 'one\ntwo e\n' > code.txt && git add -A && git commit -qm e
lane lane-d && git rm -q code.txt && git commit -qm d
git checkout -q batch

# 1. Refusals leave the branch where it was and merge nothing.
"$script" lane-a no-such-branch > "$tmp/out" 2>&1
[[ $? == 2 && $(git rev-parse HEAD) == "$base" ]] && grep -q 'REFUSE unknown-branch no-such-branch' "$tmp/out" ||
  fail 'an unknown branch must refuse before any merge'
printf 'dirty\n' >> code.txt
"$script" lane-a > "$tmp/out" 2>&1
[[ $? == 2 && $(git rev-parse HEAD) == "$base" ]] && grep -q 'REFUSE dirty-worktree' "$tmp/out" ||
  fail 'a dirty tracked file must refuse'
git checkout -q -- code.txt
git checkout -q --detach
"$script" lane-a > "$tmp/out" 2>&1
[[ $? == 2 ]] && grep -q 'REFUSE detached-head' "$tmp/out" || fail 'a detached HEAD must refuse'
git checkout -q batch
"$script" > "$tmp/out" 2>&1
[[ $? == 2 ]] && grep -q 'REFUSE usage' "$tmp/out" || fail 'no branch must refuse with usage'

# 2. One pass: a, b (conflicts with a on notes.txt), c, e (clean), d (modify/delete against e).
"$script" lane-a lane-b lane-c lane-e lane-d > "$tmp/out" 2>&1 || fail "the pass must exit 0, got $?"
head=$(git rev-parse HEAD)
manifest=$(git rev-parse --absolute-git-dir)/corvint/batch-assemble-manifest.txt
grep -q "^batch-assemble: BASE $base branch=batch$" "$tmp/out" || fail 'BASE line'
for name in lane-a lane-c lane-e; do
  grep -Eq "^batch-assemble: MERGE [0-9a-f]{40} branch=$name lane=$(git rev-parse "$name") derivation=ort$" "$tmp/out" ||
    fail "$name must merge with derivation=ort"
done
grep -Eq "^batch-assemble: MERGE [0-9a-f]{40} branch=lane-b lane=$(git rev-parse lane-b) derivation=ort-ours conflicted=1 paths=notes.txt$" "$tmp/out" ||
  fail 'lane-b must merge with derivation=ort-ours and list notes.txt'
grep -q "^batch-assemble: SKIP branch=lane-d lane=$(git rev-parse lane-d) reason=ours-unresolved paths=code.txt$" "$tmp/out" ||
  fail 'lane-d (modify/delete) must be skipped as ours-unresolved'
grep -q "^batch-assemble: DONE head=$head merges=4 ort=3 ours=1 skipped=1$" "$tmp/out" || fail 'DONE line'
grep -q "^batch-assemble: NEXT restore: .* BASE=$head (DCW-V0-073)$" "$tmp/out" || fail 'NEXT restore line'
grep -q "^batch-assemble: NEXT verify: build and run the affected selection at $head (corvint affected --base $base); " "$tmp/out" ||
  fail 'NEXT verify line'
grep -q '^batch-assemble: NEXT rebind lane-d: ' "$tmp/out" || fail 'NEXT rebind line for lane-d'
grep -q "^batch-assemble: NEXT check: make dogfood-batch-check BASE=$base$" "$tmp/out" || fail 'NEXT check line'
[[ $(grep -c '^batch-assemble: ' "$manifest") == $(grep -c '^batch-assemble: [A-Z][A-Z]* ' "$tmp/out" | tr -d ' ') ]] &&
  grep -q "^batch-assemble: DONE head=$head " "$manifest" || fail 'the manifest must hold every BASE/MERGE/SKIP/DONE/NEXT line'
[[ ! -e $tmp/driver-ran ]] || fail 'the configured merge driver ran'
[[ $(git show HEAD:notes.txt) == $'base\nlane a' ]] || fail 'notes.txt must keep the batch side, not a driver result'
[[ -z $(git status --porcelain) ]] || fail 'the worktree must be clean after the fast-forward'
[[ $(git rev-list --count "$base..HEAD") == 8 ]] || fail 'four lane commits and four merges'
git log --merges --format='%H %P' "$base..HEAD" > "$tmp/merges"
[[ $(wc -l < "$tmp/merges" | tr -d ' ') == 4 ]] || fail 'four two-parent merges'
# Each merge tree is what an attribute-free merge-tree derives from its parents, with or without
# -Xours (the derivations dogfood batch-check accepts, DCW-V0-039, DCW-V0-073).
# The check derives them in its own bare repository that borrows the objects, as batch-check does.
git init -q --bare "$tmp/derive.git" && printf '%s\n' "$repo/.git/objects" > "$tmp/derive.git/objects/info/alternates" ||
  fail 'derivation scratch'
derive() { git --git-dir="$tmp/derive.git" -c core.attributesFile=/dev/null merge-tree --write-tree "$@" 2> /dev/null | sed -n 1p; }
while read -r merge first second; do
  tree=$(git rev-parse "$merge^{tree}")
  plain=$(derive "$first" "$second")
  ours=$(derive -Xours "$first" "$second")
  [[ $tree == "$plain" || $tree == "$ours" ]] || fail "merge $merge is not an attribute-free ort or ort-ours tree"
done < "$tmp/merges"
git log -1 --format=%B "$(git log --merges --format=%H --grep='Merge lane-b' "$base..HEAD")" | grep -q 'Kept the batch side of each conflicting hunk (-Xours) in: notes.txt.' ||
  fail 'the ort-ours merge message must name the kept paths'
git log --format=%B "$base..HEAD" | grep -q '^# Conflicts:' && fail 'no merge message may keep a # Conflicts: block'

# 3. A rerun skips what is already merged and leaves HEAD alone.
"$script" lane-a lane-c > "$tmp/out" 2>&1 || fail 'the rerun must exit 0'
[[ $(git rev-parse HEAD) == "$head" ]] || fail 'an already merged rerun must not move HEAD'
grep -q "^batch-assemble: DONE head=$head merges=0 ort=0 ours=0 skipped=2$" "$tmp/out" &&
  grep -q 'SKIP branch=lane-a .* reason=already-merged' "$tmp/out" || fail 'already-merged skips'
grep -q 'NEXT verify\|NEXT restore' "$tmp/out" && fail 'a pass with no merge owes no build or restore'

# 4. --dispositions: a hunk the restore change cannot hold verbatim is dispositioned in the ort-ours
# merge's message; a malformed line, an unknown branch or a disposition that reached no ort-ours
# merge refuses and leaves the branch where it was.
git checkout -q -b batch2 "$base"
id=$(printf '%064d' 0 | tr 0 a)
printf '# comment\n\nlane-x %s reason\n' "$id" > "$tmp/disp"
"$script" --dispositions "$tmp/disp" lane-a lane-b > "$tmp/out" 2>&1
[[ $? == 2 && $(git rev-parse HEAD) == "$base" ]] && grep -q "REFUSE dispositions-unknown-branch $tmp/disp:3: lane-x" "$tmp/out" ||
  fail 'a disposition for a branch not named must refuse before any merge'
printf 'lane-b abc reason\n' > "$tmp/disp"
"$script" --dispositions "$tmp/disp" lane-a lane-b > "$tmp/out" 2>&1
[[ $? == 2 && $(git rev-parse HEAD) == "$base" ]] && grep -q "REFUSE dispositions-malformed $tmp/disp:1: " "$tmp/out" ||
  fail 'a malformed hunk id must refuse'
printf 'lane-b %s\n' "$id" > "$tmp/disp"
"$script" --dispositions "$tmp/disp" lane-a lane-b > "$tmp/out" 2>&1
[[ $? == 2 ]] && grep -q 'REFUSE dispositions-malformed' "$tmp/out" || fail 'a disposition without a reason must refuse'
"$script" --dispositions "$tmp/missing" lane-a > "$tmp/out" 2>&1
[[ $? == 2 ]] && grep -q 'REFUSE dispositions-unreadable' "$tmp/out" || fail 'an unreadable FILE must refuse'
printf 'lane-c %s generated\n' "$id" > "$tmp/disp"
"$script" --dispositions "$tmp/disp" lane-a lane-b lane-c > "$tmp/out" 2>&1
[[ $? == 2 && $(git rev-parse HEAD) == "$base" ]] && grep -q 'REFUSE disposition-unused lane-c: ' "$tmp/out" ||
  fail 'a disposition for an ort merge must refuse unused and leave the branch at base'
printf 'lane-b %s regenerated in the restore change\n' "$id" > "$tmp/disp"
"$script" --dispositions "$tmp/disp" lane-a lane-b lane-c > "$tmp/out" 2>&1 || fail "the disposition pass must exit 0, got $?"
grep -Eq "^batch-assemble: MERGE [0-9a-f]{40} branch=lane-b lane=$(git rev-parse lane-b) derivation=ort-ours conflicted=1 paths=notes.txt dispositions=1$" "$tmp/out" ||
  fail 'the lane-b MERGE line must count its disposition'
merge_b=$(git log --merges --format=%H --grep='Merge lane-b' "$base..HEAD")
git log -1 --format=%B "$merge_b" | grep -qx "Corvint-Dropped-Hunk: $id regenerated in the restore change" ||
  fail 'the lane-b merge message must carry the Corvint-Dropped-Hunk line'
[[ $(git rev-parse "$merge_b^{tree}") == $(git rev-parse "$(git log --merges --format=%H --grep='Merge lane-b' batch)^{tree}") ]] ||
  fail 'a disposition changes the message only, never the merge tree'
git log --format=%B "$base..HEAD" | grep -c '^Corvint-Dropped-Hunk:' | grep -qx 1 || fail 'only the ort-ours merge carries the line'

if [[ $failed != 0 ]]; then
  printf 'batch-assemble_test: FAIL\n' >&2
  exit 1
fi
printf 'batch-assemble_test: PASS\n'
