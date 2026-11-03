#!/usr/bin/env bash
# Tests for script/check-stray-backups.sh (ASD-V0-2370, V1-4352) on a fixture repository: a clean
# index passes; tracked `.bak` and `.orig` files at any depth and in any letter case each fail and
# are named; names that only contain `bak` or `orig` and untracked leftovers pass; a run outside a
# Git worktree exits 2.
set -uo pipefail
export LC_ALL=C
source_root=$(cd "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-stray-backups-test.XXXXXX") || exit 1
trap '/bin/rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
check=$source_root/script/check-stray-backups.sh
fail() { printf 'check-stray-backups_test: FAIL %s\n' "$*" >&2; exit 1; }

repo=$test_root/repo
mkdir -p "$repo/sub/deep" "$repo/orig" && git -C "$repo" init -q || fail "fixture init"
for f in bakery.go x.bak.go orig/file.txt sub/original.md notes.backup; do printf 'x\n' > "$repo/$f"; done
git -C "$repo" add -A || fail "fixture add"
out=$(cd "$repo/sub" && "$check" 2>&1); rc=$?
[[ $rc == 0 && $out == 'check-stray-backups: no tracked .bak or .orig file' ]] || fail "clean index (rc=$rc): $out"

# An untracked leftover is not the index's: it passes.
printf 'x\n' > "$repo/untracked.bak"
out=$(cd "$repo" && "$check" 2>&1); rc=$?
[[ $rc == 0 ]] || fail "an untracked .bak failed (rc=$rc): $out"

for f in sub/deep/node_identity_test.go.bak top.orig sub/MERGED.ORIG odd.Bak; do printf 'x\n' > "$repo/$f"; done
git -C "$repo" add -- sub/deep/node_identity_test.go.bak top.orig sub/MERGED.ORIG odd.Bak || fail "fixture add leftovers"
out=$(cd "$repo" && "$check" 2>&1); rc=$?
((rc == 1)) || fail "tracked leftovers did not exit 1 (rc=$rc): $out"
want=$(printf '%s\n' odd.Bak sub/MERGED.ORIG sub/deep/node_identity_test.go.bak top.orig)
got=$(sed -n 's/^check-stray-backups: FAIL stray-backup \([^:]*\): .* git rm --cached it .*/\1/p' <<< "$out")
[[ $got == "$want" ]] || fail "named leftovers: want [$want] got [$got] from: $out"
[[ $(grep -c . <<< "$out") == 4 ]] || fail "extra output lines: $out"

# A leftover removed from the index alone (still on disk) passes again.
git -C "$repo" rm -q --cached -- sub/deep/node_identity_test.go.bak top.orig sub/MERGED.ORIG odd.Bak || fail "fixture rm"
out=$(cd "$repo" && "$check" 2>&1); rc=$?
[[ $rc == 0 ]] || fail "index-removed leftovers still failed (rc=$rc): $out"

mkdir -p "$test_root/nogit"
out=$(cd "$test_root/nogit" && GIT_CEILING_DIRECTORIES=$test_root "$check" 2>&1); rc=$?
[[ $rc == 2 && $out == *'not in a Git worktree'* ]] || fail "outside a worktree (rc=$rc): $out"
printf 'check-stray-backups_test: PASS\n'
