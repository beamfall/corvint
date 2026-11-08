#!/usr/bin/env bash
# usage: script/long-lines-check.sh
#
# ASD-V0-573 (V1-3461): fails when a tracked text file holds a line over 4096 bytes and is neither
# under a glob in .corvint/agent-grep-exclude (hidden from script/agent-grep.sh) nor listed by
# exact path in .corvint/long-line-allow (searched, accepted as long). Each such file is named with
# its fix, so a new generated or data file cannot silently flood agent searches. Binary files are
# skipped. Exit 1 on a finding, 2 outside a checkout or when the scan does not finish: a failing
# tracked-file list, an unreadable tracked file or an unreadable allow list exits 2 with the error
# and a fix line, never `ok` (review, 2026-11-07).
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
# The allow list is optional; one that exists but cannot be read refuses. grep exits 1 when every
# line is a comment or blank, which is an empty list.
allow=
if [[ -e .corvint/long-line-allow ]]; then
  allow=$(grep -v -e '^#' -e '^$' .corvint/long-line-allow)
  rc=$?
  ((rc <= 1)) || { printf '%s: REFUSED cannot read .corvint/long-line-allow (grep exit %s); fix: make it readable, then rerun\n' "$me" "$rc" >&2; exit 2; }
fi
# pipefail: a failing file list or file read fails the whole scan, and its error stays on stderr.
scanned=$(git ls-files --eol -z -- . ${excludes[@]+"${excludes[@]}"} | tr '\0' '\n' | awk -F'\t' '$1 !~ /^i\/-text/ { print $2 }' |
  tr '\n' '\0' | xargs -0 awk 'length($0) > 4096 { print FILENAME; nextfile }') || {
  printf '%s: REFUSED the scan did not finish (exit %s): the tracked-file list or a file read failed, named above; fix: make every tracked text file readable (restore a deleted one with git restore, or git rm it), then rerun\n' "$me" "$?" >&2
  exit 2
}
found=
if [[ -n $scanned ]]; then
  found=$(grep -vxF -e "${allow:-/}" <<< "$scanned")
  rc=$?
  ((rc <= 1)) || { printf '%s: REFUSED cannot filter the findings by .corvint/long-line-allow (grep exit %s)\n' "$me" "$rc" >&2; exit 2; }
fi
[[ -z $found ]] && { printf '%s: ok\n' "$me"; exit 0; }
while IFS= read -r path; do
  printf '%s: %s has a line over 4096 bytes; fix: add a glob for it to .corvint/agent-grep-exclude when it is generated or data, or its path to .corvint/long-line-allow when agents must search it\n' "$me" "$path" >&2
done <<< "$found"
exit 1
