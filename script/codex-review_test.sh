#!/usr/bin/env bash
# ASD-V0-064 (V1-3219): script/codex-review.sh checks `codex login status` once and reports
# NOT_AVAILABLE with the login fix before any review call; a logged-in CLI runs the read-only review.
# A stub codex records every call. No network and no real Codex run.
set -uo pipefail
export LC_ALL=C

script=$(cd "$(dirname "$0")" && pwd)/codex-review.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-codex-review-test.XXXXXX") || exit 1
trap '/bin/rm -rf "$tmp"' EXIT
failed=0
fail() {
  printf 'codex-review_test: FAIL %s\n' "$*" >&2
  failed=1
}

# The stub answers `login status` with $STUB_STATUS and exit $STUB_RC, and `exec` with its argv.
mkdir "$tmp/bin"
cat > "$tmp/bin/codex" << 'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$STUB_LOG"
if [[ $1 == login ]]; then
  printf '%s\n' "$STUB_STATUS"
  exit "$STUB_RC"
fi
printf 'reviewed:%s\n' "$*"
STUB
chmod 0755 "$tmp/bin/codex"
review() { STUB_LOG=$tmp/calls CODEX_BIN=$tmp/bin/codex "$script" "$@" 2>&1; }

: > "$tmp/calls"
out=$(STUB_STATUS='Not logged in' STUB_RC=1 review 'review this'); rc=$?
[[ $rc == 3 && $out == *'codex-review: NOT_AVAILABLE not-logged-in (codex login status exit 1: Not logged in); fix: run '"'codex login'"* ]] ||
  fail "logged out: rc=$rc out=$out"
[[ $(cat "$tmp/calls") == 'login status' ]] || fail "logged out: want one login check and no review, got: $(cat "$tmp/calls")"

: > "$tmp/calls"
out=$(STUB_STATUS='Not logged in' STUB_RC=0 review --preflight); rc=$?
[[ $rc == 3 && $out == *NOT_AVAILABLE* ]] || fail "exit 0 with 'Not logged in' was admitted: rc=$rc out=$out"

: > "$tmp/calls"
out=$(STUB_STATUS='Logged in using ChatGPT' STUB_RC=0 review --preflight); rc=$?
[[ $rc == 0 && $out == 'codex-review: OK logged-in (Logged in using ChatGPT)' ]] || fail "preflight: rc=$rc out=$out"
[[ $(cat "$tmp/calls") == 'login status' ]] || fail "preflight ran a review: $(cat "$tmp/calls")"

: > "$tmp/calls"
out=$(STUB_STATUS='Logged in using ChatGPT' STUB_RC=0 review 'review this'); rc=$?
[[ $rc == 0 && $out == 'reviewed:exec -m gpt-6.1-sol -s read-only review this' ]] || fail "review: rc=$rc out=$out"
[[ $(wc -l < "$tmp/calls" | tr -d ' ') == 2 ]] || fail "review: want one check and one exec, got: $(cat "$tmp/calls")"

out=$(STUB_STATUS='Logged in' STUB_RC=0 review --preflight extra); rc=$?
[[ $rc == 2 ]] || fail "preflight accepted an extra argument: rc=$rc out=$out"

out=$(CODEX_BIN=$tmp/missing "$script" --preflight 2>&1); rc=$?
[[ $rc == 3 && $out == *'NOT_AVAILABLE codex-not-installed'* ]] || fail "missing codex: rc=$rc out=$out"

[[ $failed == 0 ]] || exit 1
echo "codex-review_test.sh: ok"
