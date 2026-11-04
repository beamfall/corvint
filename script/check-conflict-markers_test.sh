#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# DCI-V0-7902: each marker, governed path family, staged bytes and fixture exception.
set -euo pipefail
check=$(cd "$(dirname "$0")" && pwd)/check-conflict-markers.sh
scratch=$(mktemp -d "${TMPDIR:-/tmp}/corvint-conflict-markers-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
git init -q "$scratch/repo"
cd "$scratch/repo"
mkdir -p docs script testdata
fail() { echo "conflict-markers test: FAIL: $*" >&2; exit 1; }
"$check" > "$scratch/out" || fail 'empty tree'
for path in docs/contract.md source.go data.json script/tool testdata/marker.go; do
  for marker in '<<<<<<< HEAD' '=======' '>>>>>>> branch'; do
    printf '%s\n' "$marker" > "$path"
    git add "$path"
    # The clean working bytes must not hide staged markers.
    printf 'clean\n' > "$path"
    if "$check" > "$scratch/out" 2>&1; then fail "accepted $marker in $path"; fi
    grep -Fq "$path" "$scratch/out" || fail 'missing path diagnostic'
    git rm --cached -f -q "$path"
    rm "$path"
  done
done
printf '<<<<<<< HEAD\n=======\n>>>>>>> branch\n' > testdata/marker.go
printf 'testdata/marker.go\n' > script/conflict-marker-fixtures.txt
git add .
"$check" > "$scratch/out" || fail 'exact fixture exception refused'
printf '<<<<<<< HEAD\n' > testdata/other.go
git add testdata/other.go
if "$check" > "$scratch/out" 2>&1; then fail 'fixture exception leaked to sibling'; fi
printf 'testdata/marker.go\ntestdata/other.go\n' > script/conflict-marker-fixtures.txt
if "$check" > "$scratch/out" 2>&1; then fail 'unstaged allowlist concealed a marker'; fi
printf 'testdata/\n' > script/conflict-marker-fixtures.txt
git add script/conflict-marker-fixtures.txt
if "$check" > "$scratch/out" 2>&1; then fail 'directory exemption admitted'; fi
printf 'docs/contract.md\n' > script/conflict-marker-fixtures.txt
git add script/conflict-marker-fixtures.txt
if "$check" > "$scratch/out" 2>&1; then fail 'non-fixture allowlist admitted'; fi
printf 'testdata/*\n' > script/conflict-marker-fixtures.txt
git add script/conflict-marker-fixtures.txt
if "$check" > "$scratch/out" 2>&1; then fail 'wildcard admitted'; fi
echo 'conflict-markers test: PASS'
