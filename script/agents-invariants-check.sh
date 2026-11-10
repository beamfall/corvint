#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/agents-invariants-check.sh [FILE]
#
# ACR-V1-012: product invariants 3, 4, 5, 7 and 8 of AGENTS.md are edited only by the change
# `acceptance-record`. This check pins their text. It fails when invariant 8 lacks the words
# "human-owned intent", and when the pinned text differs, naming the last commit that touched the
# file. An edit by another change is such a difference: only the owning change may move the pin,
# and a rewording of invariant 8 goes to the owner (owner decisions H4 and H5, 2026-11-10).
set -uo pipefail
pin=b6a0e7c10fbc5c24ac4eebcadf65e1af7f6638baef8877729e76f541b0b45caf
file=${1:-AGENTS.md}
test -r "$file" || { printf 'agents-invariants-check: cannot read %s\n' "$file" >&2; exit 2; }
# One invariant: its numbered line and every line up to the next numbered line or section,
# later paragraphs included, without trailing blank lines, inside "## Product invariants".
block() {
  awk -v n="$1" '
    /^## / { on = ($0 == "## Product invariants"); take = 0; next }
    !on { next }
    /^[0-9]+\. / { take = (index($0, n ". ") == 1); blank = 0 }
    take && /^$/ { blank++; next }
    take { for (; blank > 0; blank--) print ""; print }' "$file"
}
last() { git log -1 --format=%H -- "$file" 2>/dev/null || true; }
status=0
eight=$(block 8 | tr '\n' ' ' | tr -s ' ')
case $eight in
  *'human-owned intent'*) ;;
  *) printf 'agents-invariants-check: invariant 8 lacks "human-owned intent" (ACR-V1-012); last commit on %s: %s\n' \
       "$file" "$(last)" >&2; status=1 ;;
esac
for n in 3 4 5 7 8; do
  test -n "$(block "$n")" || { printf 'agents-invariants-check: invariant %s not found in %s\n' "$n" "$file" >&2; exit 1; }
done
got=$(for n in 3 4 5 7 8; do block "$n"; done | shasum -a 256 | cut -d' ' -f1)
if [[ $got != "$pin" ]]; then
  printf 'agents-invariants-check: invariants 3, 4, 5, 7 or 8 differ from the text pinned by change acceptance-record (ACR-V1-012): got %s; last commit on %s: %s; fix: revert the edit, or make it in that change and move the pin\n' \
    "$got" "$file" "$(last)" >&2
  status=1
fi
test "$status" = 0 && printf 'agents-invariants-check: ok\n'
exit "$status"
