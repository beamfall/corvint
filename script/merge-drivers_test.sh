#!/usr/bin/env bash
# ASD-V0-571 (V1-4167): script/merge-drivers.sh reports a custom .gitattributes merge driver that
# this checkout's Git config lacks, --install sets it from the script/merge-NAME.sh convention, and
# Git's built-in drivers need no config. Temporary Git fixtures only.
set -uo pipefail
script=$(cd "$(dirname "$0")" && pwd)/merge-drivers.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/merge-drivers-test.XXXXXX") || exit 2
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'merge-drivers_test: FAIL %s\n' "$1" >&2; exit 1; }
cd "$tmp" || exit 2
git init -q . || exit 2
mkdir script
printf '* -text\n# merge=commented\ndocs/README.md merge=union\n.corvint/use-cases/** merge=use-case-pins\n' > .gitattributes

# 1. An unset driver whose script is absent is reported with its fix.
out=$("$script" 2>&1)
[[ $? == 1 ]] || fail "missing driver and script: not exit 1: $out"
grep -qF 'MISSING merge=use-case-pins' <<< "$out" && grep -qF 'neither merge.use-case-pins.driver nor script/merge-use-case-pins.sh' <<< "$out" || fail "missing script output: $out"
grep -q 'union\|commented' <<< "$out" && fail "a built-in or commented driver was reported: $out"

# 2. With the script present, the check names --install, and --install sets the conventional driver.
printf '#!/bin/sh\n' > script/merge-use-case-pins.sh && chmod +x script/merge-use-case-pins.sh
out=$("$script" 2>&1)
[[ $? == 1 ]] && grep -qF 'fix: script/merge-drivers.sh --install' <<< "$out" || fail "unset driver: $out"
out=$("$script" --install 2>&1) || fail "install: rc=$? $out"
[[ $(git config --get merge.use-case-pins.driver) == 'script/merge-use-case-pins.sh %O %A %B %L' ]] || fail "installed driver: $(git config --get merge.use-case-pins.driver)"
out=$("$script" 2>&1) || fail "check after install: $out"
[[ $out == 'merge-drivers: ok merge=use-case-pins' ]] || fail "check after install output: $out"

# 3. --install never overwrites a driver the checkout already set.
git config merge.use-case-pins.driver 'custom %O %A %B'
"$script" --install > /dev/null 2>&1 || fail "install over a set driver"
[[ $(git config --get merge.use-case-pins.driver) == 'custom %O %A %B' ]] || fail "a set driver was overwritten"

# 4. Usage errors.
"$script" --bogus > /dev/null 2>&1
[[ $? == 2 ]] || fail "unknown flag is not a usage error"
printf 'merge-drivers_test: ok\n'
