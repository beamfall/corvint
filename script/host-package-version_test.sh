#!/bin/sh
set -eu

# AHI-173 / V1-2145: script/host-package-version.sh bumps every version location of a package
# together and changes nothing else, verify names a disagreeing location, and bump refuses until
# the locations agree. It runs against copies of the real manifests, so a new location format
# or a moved file fails here.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-host-package-version-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

files="integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json
integrations/claude-code/.claude-plugin/marketplace.json
integrations/codex/plugins/corvint/.codex-plugin/plugin.json
protocol/integrations/speckit/corvint/extension.yml
protocol/integrations/openspec/skills/corvint-openspec/SKILL.md"
mkdir -p "$test_root/script"
cp "$source_root/script/host-package-version.sh" "$test_root/script/"
for f in $files; do
    mkdir -p "$test_root/$(dirname "$f")"
    cp "$source_root/$f" "$test_root/$f"
done
git -C "$test_root" init -q
git -C "$test_root" add -A
git -C "$test_root" -c user.email=fixture@example.test -c user.name=Fixture commit -q -m fixture

tool() { (cd "$test_root" && script/host-package-version.sh "$@"); }

# 1. The real manifests agree today.
tool verify >/dev/null || fail "verify refused the committed manifests"

# 2. Bumping each package changes exactly one line per location.
cases=1
for spec in claude-code:2 codex:1 speckit:1 openspec:1; do
    package=${spec%%:*} want=${spec#*:}
    old=$(tool verify "$package") || fail "$package did not verify before its bump"
    old=${old#* }
    tool bump "$package" >/dev/null || fail "$package bump failed"
    new=$(tool verify "$package") || fail "$package did not verify after its bump"
    new=${new#* }
    expected=$(printf '%s' "$old" | awk -F. '{ printf "%d.%d.%d", $1, $2, $3 + 1 }')
    [ "$new" = "$expected" ] || fail "$package bumped $old to $new, want $expected"
    changed=$(git -C "$test_root" diff -U0 | grep -c '^+[^+]' || true)
    [ "$changed" -eq "$want" ] || fail "$package bump changed $changed lines, want $want: $(git -C "$test_root" diff)"
    git -C "$test_root" -c user.email=fixture@example.test -c user.name=Fixture commit -qam "bump $package"
    cases=$((cases + 1))
done

# 3. An explicit version applies to every location.
tool bump claude-code 1.2.3 >/dev/null || fail "explicit claude-code bump failed"
[ "$(tool verify claude-code)" = "claude-code 1.2.3" ] || fail "explicit claude-code bump did not apply everywhere"

# 4. A disagreeing location is named by verify, and bump refuses until it is fixed.
(cd "$test_root" && git checkout -q -- . && jq '.plugins[0].version = "0.0.1"' integrations/claude-code/.claude-plugin/marketplace.json >m.tmp && mv m.tmp integrations/claude-code/.claude-plugin/marketplace.json)
if output=$(tool verify claude-code 2>&1); then fail "verify passed a disagreeing location"; fi
case $output in *'claude-code: integrations/claude-code/.claude-plugin/marketplace.json has 0.0.1'*) ;; *) fail "verify did not name the location: $output" ;; esac
if tool bump claude-code >/dev/null 2>&1; then fail "bump ran over disagreeing locations"; fi
(cd "$test_root" && git checkout -q -- .)
grep -A3 '^host-package-versions-check:' "$source_root/Makefile" | grep -q 'host-package-version.sh verify' ||
    fail "make host-package-versions-check does not run verify (AHI-240)"

# 5. An unknown package is a usage refusal.
status=0
tool verify nope >/dev/null 2>&1 || status=$?
[ "$status" -eq 2 ] || fail "unknown package exited $status, want 2"
printf 'host-package-version_test: %d cases passed\n' "$((cases + 3))"
