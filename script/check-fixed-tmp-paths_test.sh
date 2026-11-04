#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Covers ASD-V0-720 and LVT-V0-006 (V1-4252) on a fixture index: a test that writes under the
# fixed /tmp, joins onto it or exports TMPDIR=/tmp fails with its path and line; a
# `// fixed-tmp: REASON` marker on the line or the line before exempts it; staged content is read,
# not the worktree; and the candidates are read in one batch, so the git spawn count does not grow
# with the number of files.
set -euo pipefail

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-fixed-tmp-paths-test.XXXXXX")
trap 'rm -rf "$test_root"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

repo=$test_root/repo
mkdir -p "$repo/script" "$repo/pkg" "$test_root/shim"
cp "$source_root/script/check-fixed-tmp-paths.sh" "$repo/script/"
git -C "$repo" init -q
real_git=$(command -v git)
printf '#!/bin/sh\necho git >> "%s"\nexec "%s" "$@"\n' "$test_root/spawns" "$real_git" > "$test_root/shim/git"
sed '2i\
if [ "${LANE_BAD_BATCH:-}" = 1 ] && [ "$1 $2" = "cat-file --batch" ]; then exit 23; fi
' "$test_root/shim/git" > "$test_root/shim/newgit"
mv "$test_root/shim/newgit" "$test_root/shim/git"
chmod +x "$test_root/shim/git"
run() { (cd "$repo" && PATH="$test_root/shim:$PATH" script/check-fixed-tmp-paths.sh 2>&1); }

cat > "$repo/pkg/a_test.go" <<'GO'
package pkg

func TestA(t *testing.T) {
	os.WriteFile("/tmp/x", nil, 0o600)
	dir := filepath.Join("/tmp", "y")
	cmd.Env = append(cmd.Env, "TMPDIR=/tmp/z")
	// fixed-tmp: the fixture names the literal on purpose
	os.MkdirAll("/tmp/marked", 0o700)
	os.MkdirTemp(
		"/tmp/later", "x")
}
GO
printf 'package pkg\n\nconst note = "/tmp is only named here"\n' > "$repo/pkg/b_test.go"
printf 'package pkg\n\nfunc f() { os.WriteFile("/tmp/x", nil, 0o600) }\n' > "$repo/pkg/c.go"
git -C "$repo" add -A
status=0
out=$(run) || status=$?
[[ $status == 1 ]] || fail "fixed /tmp writes passed (exit $status): $out"
for want in 'FAIL pkg/a_test.go:4:' 'FAIL pkg/a_test.go:5:' 'FAIL pkg/a_test.go:6:' 'FAIL pkg/a_test.go:9:'; do
  [[ $out == *"$want"* ]] || fail "missing '$want': $out"
done
[[ $(grep -c FAIL <<< "$out") == 4 ]] || fail "a marked line, a bare literal or a non-test file was reported: $out"

# The verdict follows the index: an unstaged fix keeps the failure, a staged one clears it.
printf 'package pkg\n\nfunc TestA(t *testing.T) { _ = t.TempDir() }\n' > "$repo/pkg/a_test.go"
status=0
out=$(run) || status=$?
[[ $status == 1 ]] || fail "an unstaged fix changed the verdict: $out"
git -C "$repo" add -A
out=$(run) || fail "a staged fix still fails: $out"

# One batch: forty more candidate files cost no more git spawns than one.
: > "$test_root/spawns"
run > /dev/null || fail "the one-candidate run failed"
few=$(grep -c . "$test_root/spawns")
for i in $(seq 1 40); do printf 'package pkg\n\nconst p%d = "/tmp/named"\n' "$i" > "$repo/pkg/m${i}_test.go"; done
git -C "$repo" add -A
: > "$test_root/spawns"
run > /dev/null || fail "the forty-one-candidate run failed"
many=$(grep -c . "$test_root/spawns")
[[ $many == "$few" && $many -le 4 ]] || fail "git spawns grew with the candidate count: $few for 1 file, $many for 41"
status=0
LANE_BAD_BATCH=1 run > /dev/null 2>&1 || status=$?
[[ $status -ne 0 ]] || fail "an unavailable batch read declared the index valid"
printf 'check-fixed-tmp-paths_test.sh: ok (%d git spawns)\n' "$many"
