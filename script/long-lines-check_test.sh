#!/usr/bin/env bash
# ASD-V0-573 (V1-3461): script/long-lines-check.sh fails on a tracked file with a line over 4096
# bytes outside .corvint/agent-grep-exclude and .corvint/long-line-allow, and script/agent-grep.sh
# keeps excluded paths out of a search. Temporary Git fixtures only.
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
check=$here/long-lines-check.sh grep=$here/agent-grep.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/long-lines-test.XXXXXX") || exit 2
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'long-lines-check_test: FAIL %s\n' "$1" >&2; exit 1; }
cd "$tmp" || exit 2
git init -q . && git config user.email t@example.invalid && git config user.name t || exit 2
mkdir -p .corvint data src
printf '# comment\n\ndata/**\n' > .corvint/agent-grep-exclude
long=$(head -c 5000 /dev/zero | tr '\0' x)
printf 'needle %s\n' "$long" > data/corpus.jsonl
printf 'needle short\n' > src/main.txt
printf 'needle %s\n' "$(head -c 4000 /dev/zero | tr '\0' y)" > src/under.txt
git add -A || exit 2

# 1. Long lines under an excluded glob and lines up to 4096 bytes pass.
out=$("$check" 2>&1) || fail "excluded long line: $out"
[[ $out == 'long-lines-check: ok' ]] || fail "ok output: $out"

# 2. A new tracked long-line file outside the excluded set fails, named with its fix.
printf '%s\n' "$long" > src/generated.json && git add src/generated.json
out=$("$check" 2>&1)
[[ $? == 1 ]] || fail "unlisted long line: not exit 1: $out"
grep -qF 'long-lines-check: src/generated.json has a line over 4096 bytes; fix:' <<< "$out" || fail "finding output: $out"
grep -q 'corpus\|under\|main' <<< "$out" && fail "an accepted file was reported: $out"

# 3. Listing it in .corvint/long-line-allow accepts it.
printf '# searched\nsrc/generated.json\n' > .corvint/long-line-allow
"$check" > /dev/null 2>&1 || fail "allowed long line still fails"

# 4. agent-grep leaves excluded paths out; a caller pathspec still applies.
out=$("$grep" -l needle) || fail "agent-grep: rc=$?"
[[ $out == $'src/main.txt\nsrc/under.txt' ]] || fail "agent-grep files: $out"
out=$("$grep" -l needle -- src/main.txt) || fail "agent-grep pathspec: rc=$?"
[[ $out == src/main.txt ]] || fail "agent-grep pathspec files: $out"

# 5. (review) A scan that does not finish refuses with exit 2 and never prints ok: a tracked file the
# scan cannot read (mode 000, which root still reads, so that case needs another user; and a tracked
# file deleted from the worktree, for every user), and an allow list that cannot be read.
unreadable() { # unreadable PATH LABEL: the check must exit 2 naming the unfinished scan, without ok
  out=$("$check" 2>&1)
  [[ $? == 2 ]] || fail "$2: not exit 2: $out"
  grep -qF 'long-lines-check: REFUSED the scan did not finish' <<< "$out" && grep -qF -- "$1" <<< "$out" || fail "$2: refusal output: $out"
  grep -q 'long-lines-check: ok' <<< "$out" && fail "$2: printed ok: $out"
  return 0
}
printf 'short\n' > src/hidden.txt && git add src/hidden.txt || exit 2
if (($(id -u) != 0)); then
  chmod 000 src/hidden.txt && unreadable src/hidden.txt 'unreadable tracked file'
  chmod 644 src/hidden.txt
fi
/bin/rm -f src/hidden.txt && unreadable src/hidden.txt 'tracked file missing from the worktree'
git rm -q --cached src/hidden.txt || exit 2
"$check" > /dev/null 2>&1 || fail "the check still fails once the unreadable file is gone"
if (($(id -u) != 0)); then
  chmod 000 .corvint/long-line-allow
  out=$("$check" 2>&1)
  rc=$?
  chmod 644 .corvint/long-line-allow
  [[ $rc == 2 ]] && grep -qF 'long-lines-check: REFUSED cannot read .corvint/long-line-allow' <<< "$out" || fail "unreadable allow list: rc=$rc $out"
fi
printf 'long-lines-check_test: ok\n'
