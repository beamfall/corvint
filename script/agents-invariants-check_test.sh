#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# ACR-V1-012: script/agents-invariants-check.sh passes on the repository's AGENTS.md, fails when
# invariant 8 loses the words "human-owned intent", and fails when invariant 3, 4, 5 or 7 is
# edited without moving the pin (an edit from another change). Temporary copies only.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
check=$here/agents-invariants-check.sh src=$here/../AGENTS.md
tmp=$(mktemp -d "${TMPDIR:-/tmp}/agents-invariants-test.XXXXXX") || exit 2
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'agents-invariants-check_test: FAIL %s\n' "$1" >&2; exit 1; }

# 1. The committed file carries the pinned text.
out=$("$check" "$src" 2>&1) || fail "repository AGENTS.md: $out"
[[ $out == 'agents-invariants-check: ok' ]] || fail "ok output: $out"

# 2. Invariant 8 without the phrase fails and says so.
sed 's/human-owned intent/intent/' "$src" > "$tmp/eight.md"
out=$("$check" "$tmp/eight.md" 2>&1)
[[ $? == 1 ]] || fail "dropped phrase: not exit 1: $out"
grep -qF 'invariant 8 lacks "human-owned intent" (ACR-V1-012)' <<< "$out" || fail "dropped phrase output: $out"

# 3. An edit of each other owned invariant fails as a difference from the pin.
for n in 3 4 5 7; do
  awk -v n="$n" 'index($0, n ". ") == 1 && !done { print $0 " Extra."; done = 1; next } { print }' \
    "$src" > "$tmp/edit-$n.md"
  cmp -s "$src" "$tmp/edit-$n.md" && fail "fixture for invariant $n did not change the file"
  out=$("$check" "$tmp/edit-$n.md" 2>&1)
  [[ $? == 1 ]] || fail "edited invariant $n: not exit 1: $out"
  grep -qF 'differ from the text pinned by change acceptance-record (ACR-V1-012)' <<< "$out" \
    || fail "edited invariant $n output: $out"
done

# 3b. A second paragraph added after a blank line is part of the invariant, in the list and at
# its end.
for n in 5 8; do
  awk -v n="$n" '
    index($0, n ". ") == 1 { in_block = 1; print; next }
    in_block && !/^   / { print ""; print "   A second paragraph."; in_block = 0 }
    { print }' "$src" > "$tmp/para-$n.md"
  cmp -s "$src" "$tmp/para-$n.md" && fail "fixture for a paragraph in invariant $n did not change the file"
  out=$("$check" "$tmp/para-$n.md" 2>&1)
  [[ $? == 1 ]] || fail "paragraph added to invariant $n: not exit 1: $out"
done

# 4. An edit of an invariant this change does not own, or of another section, passes.
sed 's/^1\. Evidence is pinned/1. Evidence stays pinned/; s/^## Verify$/## Verify it/' "$src" > "$tmp/other.md"
cmp -s "$src" "$tmp/other.md" && fail "fixture for an unowned edit did not change the file"
out=$("$check" "$tmp/other.md" 2>&1) || fail "unowned edit refused: $out"

# 5. A missing owned invariant fails; an unreadable file is a usage error.
awk 'index($0, "5. ") == 1 { skip = 1; next } skip && /^   / { next } { skip = 0; print }' "$src" > "$tmp/gone.md"
out=$("$check" "$tmp/gone.md" 2>&1)
[[ $? == 1 ]] || fail "missing invariant 5: not exit 1: $out"
"$check" "$tmp/absent.md" >/dev/null 2>&1
[[ $? == 2 ]] || fail "absent file: not exit 2"
printf 'agents-invariants-check_test: ok\n'
