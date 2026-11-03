#!/usr/bin/env bash
# usage: script/check-stray-backups.sh
#
# ASD-V0-2370 (V1-4352): no editor, merge or patch leftover is ever tracked. Over the index of the
# current checkout, every path whose last component ends in `.bak` or `.orig` (any letter case)
# prints `check-stray-backups: FAIL stray-backup PATH: ...` with the fix. There is no allow list:
# a fixture that needs such a name generates it at test time. It exits 1 on any failure, 2 when the
# index cannot be read, and 0 otherwise. It is a doc gate (Makefile DOC_GATE_TARGETS, so CI's
# `make doc-gates` runs it) and a static fail-fast gate of the batch precheck.
set -uo pipefail
export LC_ALL=C
top=$(git rev-parse --show-toplevel 2> /dev/null) && cd "$top" || { printf 'check-stray-backups: not in a Git worktree\n' >&2; exit 2; }

listing=$(git -c core.quotePath=false ls-files -z -- ':(icase)*.bak' ':(icase)*.orig' | tr '\0' '\n') ||
  { printf 'check-stray-backups: cannot read the index\n' >&2; exit 2; }
failed=0
while IFS= read -r path; do
  [[ -n $path ]] || continue
  printf 'check-stray-backups: FAIL stray-backup %s: a .bak or .orig file is an editor, merge or patch leftover; git rm --cached it (and delete it), or rename a deliberate fixture\n' "$path"
  failed=1
done <<< "$listing"
((failed)) && exit 1
printf 'check-stray-backups: no tracked .bak or .orig file\n'
