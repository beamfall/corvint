#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Covers ASD-V0-583 and LVT-V0-006 (V1-4252) on a fixture index: a Mkfifo file under `unix`
# without `!solaris`, or with no constraint and no GOOS suffix, fails and is named; an explicit
# GOOS list, `unix && !solaris`, a GOOS suffix and a //go:build line after the package clause are
# told apart; staged content is read, not the worktree; and the files are read in one batch, so the
# git spawn count does not grow with the number of files.
set -euo pipefail

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-mkfifo-build-tags-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

repo=$test_root/repo
mkdir -p "$repo/script" "$repo/pkg" "$test_root/shim"
cp "$source_root/script/check-mkfifo-build-tags.sh" "$repo/script/"
git -C "$repo" init -q
real_git=$(command -v git)
printf '#!/bin/sh\necho git >> "%s"\nexec "%s" "$@"\n' "$test_root/spawns" "$real_git" > "$test_root/shim/git"
sed '2i\
if [ "${LANE_BAD_BATCH:-}" = 1 ] && [ "$1 $2" = "cat-file --batch" ]; then exit 23; fi
' "$test_root/shim/git" > "$test_root/shim/newgit"
mv "$test_root/shim/newgit" "$test_root/shim/git"
chmod +x "$test_root/shim/git"
run() { (cd "$repo" && PATH="$test_root/shim:$PATH" script/check-mkfifo-build-tags.sh 2>&1); }
fifo() { printf '%s\npackage pkg\n\nimport "syscall"\n\nfunc f() { _ = syscall.Mkfifo("x", 0o600) }\n' "$2" > "$repo/pkg/$1"; }

fifo bad_unix.go '//go:build unix'
fifo good_unix.go '//go:build unix && !solaris'
fifo good_list.go '//go:build darwin || linux'
fifo none.go ''
fifo suffix_linux.go ''
printf 'package pkg\n//go:build unix\n\nimport "syscall"\n\nvar _ = syscall.Mkfifo\n' > "$repo/pkg/late.go"
git -C "$repo" add -A
status=0
out=$(run) || status=$?
[[ $status == 1 ]] || fail "violations passed (exit $status): $out"
[[ $out == *'FAIL pkg/bad_unix.go: calls syscall.Mkfifo under //go:build unix, which includes solaris'* ]] || fail "unix without !solaris not named: $out"
[[ $out == *'FAIL pkg/none.go: calls syscall.Mkfifo with no //go:build constraint'* ]] || fail "missing constraint not named: $out"
[[ $out == *'FAIL pkg/late.go: calls syscall.Mkfifo with no //go:build constraint'* ]] || fail "a constraint after the package clause counted: $out"
[[ $(grep -c FAIL <<< "$out") == 3 ]] || fail "a passing file was reported: $out"

# Fixed files pass; the worktree alone does not change the verdict until it is staged.
fifo bad_unix.go '//go:build unix && !solaris'
fifo none.go '//go:build linux'
fifo late.go '//go:build darwin'
status=0
out=$(run) || status=$?
[[ $status == 1 ]] || fail "unstaged fixes changed the verdict: $out"
git -C "$repo" add -A
out=$(run) || fail "fixed files still fail: $out"

# One batch: forty more files cost no more git spawns than six.
: > "$test_root/spawns"
run > /dev/null || fail "the six-file run failed"
few=$(grep -c . "$test_root/spawns")
for i in $(seq 1 40); do fifo "more_$i.go" '//go:build unix && !solaris'; done
git -C "$repo" add -A
: > "$test_root/spawns"
run > /dev/null || fail "the forty-six-file run failed"
many=$(grep -c . "$test_root/spawns")
[[ $many == "$few" && $many -le 4 ]] || fail "git spawns grew with the file count: $few for 6 files, $many for 46"
status=0
LANE_BAD_BATCH=1 run > /dev/null 2>&1 || status=$?
[[ $status -ne 0 ]] || fail "an unavailable batch read declared the index valid"
printf 'check-mkfifo-build-tags_test.sh: ok (%d git spawns)\n' "$many"
