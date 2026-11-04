#!/usr/bin/env bash
# usage: script/long-lines-check.sh
#
# ASD-V0-573 (V1-3461): fails when a tracked text file holds a line over 4096 bytes and is neither
# under a glob in .corvint/agent-grep-exclude (hidden from script/agent-grep.sh) nor listed by
# exact path in .corvint/long-line-allow (searched, accepted as long). Each such file is named with
# its fix, so a new generated or data file cannot silently flood agent searches. Binary files are
# skipped. Exit 1 on a finding, 2 outside a checkout.
set -uo pipefail
export LC_ALL=C
me=long-lines-check
top=$(git rev-parse --show-toplevel 2> /dev/null) || { printf '%s: not inside a Git checkout\n' "$me" >&2; exit 2; }
cd "$top" || exit 2
excludes=()
while IFS= read -r glob; do
  [[ -z $glob || $glob == \#* ]] && continue
  excludes+=(":(top,exclude,glob)$glob")
done < .corvint/agent-grep-exclude
allow=$(grep -v -e '^#' -e '^$' .corvint/long-line-allow 2> /dev/null)
found=$(git ls-files --eol -z -- . ${excludes[@]+"${excludes[@]}"} | tr '\0' '\n' | awk -F'\t' '$1 !~ /^i\/-text/ { print $2 }' |
  tr '\n' '\0' | xargs -0 awk 'length($0) > 4096 { print FILENAME; nextfile }' 2> /dev/null |
  grep -vxF -e "${allow:-/}")
[[ -z $found ]] && { printf '%s: ok\n' "$me"; exit 0; }
while IFS= read -r path; do
  printf '%s: %s has a line over 4096 bytes; fix: add a glob for it to .corvint/agent-grep-exclude when it is generated or data, or its path to .corvint/long-line-allow when agents must search it\n' "$me" "$path" >&2
done <<< "$found"
exit 1
