#!/bin/sh
set -eu

# Two cases for the `vscode` completion check, `npm --prefix extensions/vscode run check`, which
# is enrolled with no preceding install step. A copy of the tracked extension with no
# `node_modules` must pass, because `check` installs from the lockfile itself; before that it
# exited 127 on `biome: command not found`. A copy without `package-lock.json` must refuse before
# installing anything, so dependencies are never resolved unpinned. The first case needs npm's
# cache or registry access; it is not a `make gate` prerequisite, since the gate runs no npm.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-vscode-check-install-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

# The extension's tests also read the shared test-validity vectors outside the extension.
(cd "$source_root" && git ls-files -z -- extensions/vscode conformance/test-validity-v0 |
    tar --null -T - -cf -) | tar -xf - -C "$test_root"
extension="$test_root/extensions/vscode"
test ! -e "$extension/node_modules" || fail "the fixture copy already has node_modules"

# 1. No node_modules: check installs from the lockfile, then lints, typechecks and tests.
status=0
output=$(npm --prefix "$extension" run check 2>&1) || status=$?
test "$status" -eq 0 || fail "check without node_modules exited $status: $(printf '%s' "$output" | tail -n 20)"
test -x "$extension/node_modules/.bin/biome" || fail "check passed without installing biome"

# 2. No lockfile: check refuses before installing anything.
rm -rf "$extension/node_modules" "$extension/package-lock.json"
if output=$(npm --prefix "$extension" run check 2>&1); then
    fail "check passed without a lockfile"
fi
case $output in
    *package-lock.json*) ;;
    *) fail "the refusal did not name the missing lockfile: $output" ;;
esac
test ! -e "$extension/node_modules" || fail "check installed dependencies without a lockfile"

printf 'vscode check install test: 2 cases passed\n'
