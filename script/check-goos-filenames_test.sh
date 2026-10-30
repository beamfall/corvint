#!/usr/bin/env bash
# Tests for script/check-goos-filenames.sh (V1-1910) on a temporary Git fixture.
set -uo pipefail
export LC_ALL=C
script=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-goos-filenames-test.XXXXXX") || exit 1
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'check-goos-filenames_test: FAIL %s\n' "$*" >&2; exit 1; }

git init -q "$tmp/repo" || fail "git init"
mkdir -p "$tmp/repo/internal/a" "$tmp/repo/cmd/b" "$tmp/repo/script" "$tmp/repo/tools"
for f in internal/a/validity_jsts.go internal/a/validity_js_flow.go internal/a/proc_darwin.go \
  internal/a/proc_linux_arm64.go internal/a/proc_windows_test.go cmd/b/js.go tools/x_js.go; do
  : > "$tmp/repo/$f"
done
git -C "$tmp/repo" add -A
"$script/check-goos-filenames.sh" "$tmp/repo" > "$tmp/out" 2>&1 || fail "clean fixture refused: $(cat "$tmp/out")"

for f in internal/a/validity_js.go internal/a/rule_wasm_test.go cmd/b/run_js_wasm.go internal/a/x_plan9_386.go; do
  : > "$tmp/repo/$f"
done
git -C "$tmp/repo" add -A
"$script/check-goos-filenames.sh" "$tmp/repo" > "$tmp/out" 2>&1
[[ $? == 1 ]] || fail "constrained names accepted: $(cat "$tmp/out")"
grep -q '^check-goos-filenames: internal/a/validity_js.go builds only for js .*internal/a/validity_jsts.go' "$tmp/out" || fail "js: $(cat "$tmp/out")"
grep -q 'internal/a/rule_wasm_test.go builds only for wasm .*internal/a/rule_wasmts_test.go' "$tmp/out" || fail "wasm test: $(cat "$tmp/out")"
grep -q 'cmd/b/run_js_wasm.go builds only for js/wasm' "$tmp/out" || fail "pair: $(cat "$tmp/out")"
grep -q 'internal/a/x_plan9_386.go builds only for plan9/386' "$tmp/out" || fail "plan9: $(cat "$tmp/out")"
(( $(wc -l < "$tmp/out") == 4 )) || fail "unexpected lines: $(cat "$tmp/out")"

printf '# comment\ninternal/a/validity_js.go\ninternal/a/rule_wasm_test.go\ncmd/b/run_js_wasm.go\ninternal/a/x_plan9_386.go\n' > "$tmp/repo/script/goos-filename-allowlist.txt"
"$script/check-goos-filenames.sh" "$tmp/repo" > "$tmp/out" 2>&1 || fail "allowlisted names refused: $(cat "$tmp/out")"
printf 'check-goos-filenames_test: PASS\n'
