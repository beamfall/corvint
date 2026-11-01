#!/usr/bin/env bash
# DCW-V0-811: the grep -P stand-in that script/test-rg-fallback.sh writes for hosts without ripgrep
# answers the subset the dogfood shell tests use exactly as ripgrep does, and refuses anything else.
set -euo pipefail

source_root=$(cd "$(dirname "$0")/.." && pwd)
dir=$(mktemp -d "${TMPDIR:-/tmp}/corvint-rg-fallback-test.XXXXXX")
trap 'rm -rf "$dir"' EXIT
fail() { printf 'test-rg-fallback_test: FAIL %s\n' "$1" >&2; exit 1; }
# shellcheck source=script/test-rg-fallback.sh
source "$source_root/script/test-rg-fallback.sh"
# ASD-V0-915 (V1-3786): a shell test that runs rg sources this fallback (or defines its own rg
# function), so a host without ripgrep never stops it with a bare "rg: command not found".
unguarded=
for test_file in "$source_root"/script/*_test.sh; do
  grep -v '^[[:space:]]*#' "$test_file" | grep -qE '(^|[^A-Za-z0-9_./-])rg( |$)' || continue
  grep -q 'test-rg-fallback\.sh"\?$' "$test_file" && continue
  grep -qE '^[[:space:]]*rg\(\)' "$test_file" && continue
  unguarded="$unguarded ${test_file#"$source_root"/}"
done
[ -z "$unguarded" ] || fail "rg without the test-rg-fallback.sh guard:$unguarded"
if ! printf 'x\n' | grep -qP '^x$' 2> /dev/null; then
  printf 'test-rg-fallback_test: SKIP grep has no -P on this host\n'
  exit 0
fi
test_rg_write_stand_in "$dir"
rg=$dir/rg
printf 'alpha 123\nx.y\n"sha256:%s"\n' "$(printf 'a%.0s' $(seq 64))" > "$dir/in.txt"

"$rg" -q '\d{3}' "$dir/in.txt" || fail "-q PCRE did not match"
"$rg" -Fxq -- 'x.y' "$dir/in.txt" || fail "-Fxq exact line did not match"
! "$rg" -Fxq -- 'x' "$dir/in.txt" || fail "-Fxq matched a partial line"
! "$rg" -Fq 'x?y' "$dir/in.txt" || fail "-F read the pattern as a regex"
"$rg" -q '"sha256:[0-9a-f]{64}"' "$dir/in.txt" || fail "a bounded repeat did not match"
test "$("$rg" -c 'a' "$dir/in.txt")" = 2 || fail "-c did not count matching lines"
printf 'x.y\n' | "$rg" -xq 'x.y' || fail "stdin input did not match"
"$rg" -qF 'alpha' "$dir/in.txt" || fail "-qF did not match"
for bad in -i -n --color=never -qi; do
  status=0
  err=$("$rg" "$bad" 'a' "$dir/in.txt" 2>&1) || status=$?
  test "$status" = 2 || fail "unsupported $bad exited $status, not 2"
  case $err in "rg stand-in: unsupported argument"*) ;; *) fail "unsupported $bad did not name itself: $err" ;; esac
done
status=0
"$rg" -q a b c > /dev/null 2>&1 || status=$?
test "$status" = 2 || fail "three operands exited $status, not 2"
printf 'test-rg-fallback_test: ok\n'
