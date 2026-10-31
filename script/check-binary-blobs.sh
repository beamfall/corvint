#!/usr/bin/env bash
# usage: script/check-binary-blobs.sh
#
# ASD-V0-582 (V1-3364): a built executable or other binary blob is never committed outside a declared
# fixture path. Two tree-wide rules over the index of the current checkout:
#   1. every path whose indexed content Git classifies as binary (`git ls-files --eol` reports
#      `i/-text`) lies under a declared fixture path: a `testdata/` path component or
#      `assets/brand/`. Otherwise it prints
#      `check-binary-blobs: FAIL binary-blob PATH mode=MODE: ...` with the fix;
#   2. every `tools/NAME/` directory holding a `package main` Go file has its default
#      `go build ./tools/NAME` output `/NAME` ignored at the repository root. Otherwise it prints
#      `check-binary-blobs: FAIL tool-output-not-ignored NAME: ...` with the fix.
# It exits 1 on any failure, 2 when the index cannot be read, and 0 otherwise. It is a doc gate
# (Makefile DOC_GATE_TARGETS), so CI and make batch-precheck both run it.
set -uo pipefail
export LC_ALL=C
cd "$(git rev-parse --show-toplevel)" || { printf 'check-binary-blobs: not in a Git worktree\n' >&2; exit 2; }

fixture_path() {
  [[ $1 == testdata/* || $1 == */testdata/* || $1 == assets/brand/* ]]
}

listing=$(git ls-files -s --eol -z | tr '\0' '\n') || { printf 'check-binary-blobs: cannot read the index\n' >&2; exit 2; }
failed=0
while IFS= read -r line; do
  [[ -n $line ]] || continue
  # MODE OID STAGE\tINDEX-EOL WORKTREE-EOL ATTR\tPATH
  meta=${line%%$'\t'*}
  rest=${line#*$'\t'}
  path=${rest#*$'\t'}
  eol=${rest%%[[:space:]]*}
  mode=${meta%% *}
  [[ $eol == i/-text && $mode != 160000 ]] || continue
  fixture_path "$path" && continue
  printf 'check-binary-blobs: FAIL binary-blob %s mode=%s: binary content is allowed only under a testdata/ directory or assets/brand/; git rm --cached it, ignore its build output in .gitignore, or move a real fixture under testdata/\n' "$path" "$mode"
  failed=1
done <<< "$listing"

while IFS= read -r dir; do
  name=${dir#tools/}
  [[ -n $name && $name != */* ]] || continue
  grep -qs '^package main$' "$dir"/*.go || continue
  git check-ignore -q --no-index -- "$name" && continue
  printf 'check-binary-blobs: FAIL tool-output-not-ignored %s: go build ./tools/%s writes ./%s at the repository root; add /%s to the root .gitignore\n' "$name" "$name" "$name" "$name"
  failed=1
done < <(git ls-files -- 'tools/*.go' | sed -n 's|^\(tools/[^/]*\)/[^/]*\.go$|\1|p' | sort -u)

exit "$failed"
