#!/usr/bin/env bash
# Temp-fixture test for script/check-windows-main.sh (V1-3782, ASD-V0-974).
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
check="$here/check-windows-main.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/windows-main-test.XXXXXX")
trap '/bin/rm -rf "$tmp"' EXIT INT TERM
fail() { printf 'check-windows-main_test: FAIL %s\n' "$*" >&2; exit 1; }
export GOCACHE=${CORVINT_GOCACHE:-${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}} GOTOOLCHAIN=local
mkdir -p "$tmp/cmd/tool" "$tmp/cmd/plain"
printf 'module example.invalid/m\n\ngo 1.27\n' > "$tmp/go.mod"
printf '//go:build darwin || linux\n\npackage main\n\nfunc main() { usage() }\n' > "$tmp/cmd/tool/main.go"
printf 'package main\n\nfunc usage() {}\n' > "$tmp/cmd/tool/help.go"
printf 'package main\n\nfunc main() {}\n' > "$tmp/cmd/plain/main.go"

# 1. An untagged help.go beside a unix-only main.go is named, with the cause.
if out=$("$check" "$tmp" 2>&1); then fail "untagged help.go passed"; fi
grep -q 'cmd/tool builds for GOOS=windows from help.go, but none declares func main' <<< "$out" || fail "message: $out"
grep -q 'cmd/plain' <<< "$out" && fail "an untagged main package was flagged: $out"

# 2. Tagging help.go like main.go clears it.
printf '//go:build darwin || linux\n\npackage main\n\nfunc usage() {}\n' > "$tmp/cmd/tool/help.go"
"$check" "$tmp" > /dev/null 2>&1 || fail "tagged help.go still flagged"

# 3. A windows main beside an untagged file also clears it.
printf 'package main\n\nfunc usage() {}\n' > "$tmp/cmd/tool/help.go"
printf 'package main\n\nfunc main() { usage() }\n' > "$tmp/cmd/tool/main_windows.go"
"$check" "$tmp" > /dev/null 2>&1 || fail "a windows main was not seen"

# 4. A directory go list cannot read refuses with exit 2.
set +e; "$check" "$tmp/absent" > /dev/null 2>&1; rc=$?; set -e
[[ $rc == 2 ]] || fail "absent dir exit $rc, want 2"
echo "check-windows-main_test: ok"
