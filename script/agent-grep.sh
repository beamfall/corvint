#!/usr/bin/env bash
# usage: script/agent-grep.sh [GIT-GREP-OPTION...] PATTERN [REV...] [-- PATHSPEC...]
#
# ASD-V0-573 (V1-3461): git grep with every glob in .corvint/agent-grep-exclude excluded, so a
# repository search returns source, spec and test lines instead of multi-KB lines from generated
# CEMs, ticket records and test corpora. Arguments pass to git grep unchanged; the pathspec is the
# caller's (default .) plus the exclusions. Exit codes are git grep's.
set -uo pipefail
top=$(git rev-parse --show-toplevel 2> /dev/null) || { printf 'agent-grep: not inside a Git checkout\n' >&2; exit 2; }
args=() paths=()
while (($#)); do
  if [[ $1 == -- ]]; then shift; paths=("$@"); break; fi
  args+=("$1"); shift
done
((${#paths[@]})) || paths=(.)
excludes=()
while IFS= read -r glob; do
  [[ -z $glob || $glob == \#* ]] && continue
  excludes+=(":(top,exclude,glob)$glob")
done < "$top/.corvint/agent-grep-exclude"
exec git grep ${args[@]+"${args[@]}"} -- "${paths[@]}" ${excludes[@]+"${excludes[@]}"}
