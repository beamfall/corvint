#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
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

# No pathspec: GIT_GLOB_PATHSPECS or GIT_LITERAL_PATHSPECS in the caller's environment would change
# what one matches. The whole index is read as NUL-separated records, so a name holding a newline
# stays one path. A path ends in `.bak` or `.orig` exactly when its last component does. The listing
# ends with one empty record, which Git never emits for a path: without it the index was unreadable.
failed=0 complete=0
while IFS= read -r -d '' path; do
  [[ -z $path ]] && { complete=1; continue; }
  case $path in
    *.[bB][aA][kK] | *.[oO][rR][iI][gG])
      printf 'check-stray-backups: FAIL stray-backup %s: a .bak or .orig file is an editor, merge or patch leftover; git rm --cached it (and delete it), or rename a deliberate fixture\n' "$path"
      failed=1
      ;;
  esac
done < <(git ls-files -z && printf '\0')
((complete)) || { printf 'check-stray-backups: cannot read the index\n' >&2; exit 2; }
((failed)) && exit 1
printf 'check-stray-backups: no tracked .bak or .orig file\n'
