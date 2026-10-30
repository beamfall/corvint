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
integrations/claude-code/plugins/corvint/compatibility.json
integrations/codex/plugins/corvint/.codex-plugin/plugin.json
integrations/codex/plugins/corvint/compatibility.json
integrations/gemini-cli/gemini-extension.json
integrations/gemini-cli/compatibility.json
integrations/opencode/package.json
integrations/opencode/src/runtime.js
integrations/pi/package.json
integrations/pi/compatibility.json
integrations/compatibility.json
integrations/pi/runtime.js
cmd/corvint/pi_adapter.go
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
evidence() {
    (cd "$test_root" && jq -c '[.lastValidation.adapterVersion]' integrations/claude-code/plugins/corvint/compatibility.json &&
        jq -c '[.lastConformance.adapterVersion]' integrations/pi/compatibility.json &&
        jq -c '[.entries[].hostVersionEvidenceAdapterVersion]' integrations/compatibility.json)
}

# 1. The real manifests agree today.
tool verify >/dev/null || fail "verify refused the committed manifests"
before=$(evidence)

# 2. Bumping each package changes exactly one line per location and leaves recorded evidence
# versions alone.
cases=1
for spec in claude-code:4 codex:3 gemini-cli:3 opencode:4 pi:6 speckit:1 openspec:1; do
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
[ "$(evidence)" = "$before" ] || fail "a bump rewrote a recorded evidence version"

# 3. An explicit version applies to every location.
tool bump pi 1.2.3 >/dev/null || fail "explicit pi bump failed"
[ "$(tool verify pi)" = "pi 1.2.3" ] || fail "explicit pi bump did not apply everywhere"

# 4. A disagreeing location is named by verify, and bump refuses until it is fixed.
(cd "$test_root" && jq '.adapterVersion = "0.0.1"' integrations/gemini-cli/compatibility.json >g.tmp && mv g.tmp integrations/gemini-cli/compatibility.json)
if output=$(tool verify gemini-cli 2>&1); then fail "verify passed a disagreeing location"; fi
case $output in *'gemini-cli: integrations/gemini-cli/compatibility.json has 0.0.1'*) ;; *) fail "verify did not name the location: $output" ;; esac
if tool bump gemini-cli >/dev/null 2>&1; then fail "bump ran over disagreeing locations"; fi

# 4b. AHI-240: a drifted Pi emitter or native literal is a location verify names.
# AHI-249: the OpenCode runtime ADAPTER_VERSION literal is a location too.
for spec in "pi|integrations/pi/runtime.js|PI_ADAPTER_VERSION='" "pi|cmd/corvint/pi_adapter.go|piAdapterVersion = \"" "opencode|integrations/opencode/src/runtime.js|ADAPTER_VERSION = \""; do
    package=${spec%%|*} spec=${spec#*|}
    path=${spec%%|*} prefix=${spec#*|}
    (cd "$test_root" && git checkout -q -- . && P=$prefix perl -0pi -e 's/\Q$ENV{P}\E[0-9.]+/$ENV{P}0.0.1/' "$path")
    if output=$(tool verify "$package" 2>&1); then fail "verify passed a drifted $path"; fi
    case $output in *"$package: $path has 0.0.1"*) ;; *) fail "verify did not name $path: $output" ;; esac
    if tool bump "$package" >/dev/null 2>&1; then fail "bump ran over a drifted $path"; fi
    cases=$((cases + 1))
done
(cd "$test_root" && git checkout -q -- .)
grep -A3 '^host-package-versions-check:' "$source_root/Makefile" | grep -q 'host-package-version.sh verify' ||
    fail "make host-package-versions-check does not run verify (AHI-240)"

# 5. An unknown package is a usage refusal.
status=0
tool verify nope >/dev/null 2>&1 || status=$?
[ "$status" -eq 2 ] || fail "unknown package exited $status, want 2"
printf 'host-package-version_test: %d cases passed\n' "$((cases + 3))"
