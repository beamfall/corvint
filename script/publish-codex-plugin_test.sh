#!/bin/sh
set -eu

# AHI-951 / V1-2495: the committed Codex plugin passes the manifest-reference check, a package whose
# plugin.json references a missing mcpServers file is refused, and a publish from a committed
# revision carries .mcp.json, the qualified hooks and PUBLISH.json while omitting scripts/.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
script=$source_root/script/publish-codex-plugin.sh
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-publish-codex-plugin-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

plugin=$source_root/integrations/codex/plugins/corvint
"$script" check "$plugin" || fail "committed Codex plugin fails the manifest-reference check"

cp -R "$plugin" "$test_root/missing"
rm "$test_root/missing/.mcp.json"
if "$script" check "$test_root/missing" 2>"$test_root/missing.err"; then
    fail "a package without its declared .mcp.json passed"
fi
grep -q 'declares mcpServers ./.mcp.json, which is missing' "$test_root/missing.err" || fail "refusal does not name the missing mcpServers file: $(cat "$test_root/missing.err")"

cp -R "$plugin" "$test_root/empty"
echo '{}' >"$test_root/empty/.mcp.json"
if "$script" check "$test_root/empty" 2>/dev/null; then
    fail "an .mcp.json with no mcpServers object passed"
fi

cp -R "$plugin" "$test_root/escape"
jq '.mcpServers = "../outside.json"' "$plugin/.codex-plugin/plugin.json" >"$test_root/escape/.codex-plugin/plugin.json"
if "$script" check "$test_root/escape" 2>/dev/null; then
    fail "a manifest path leaving the plugin passed"
fi

echo '{"hooks":{"SessionStart":[]}}' >"$test_root/qualified-hooks.json"
revision=$(git -C "$source_root" rev-parse HEAD)
out=$("$script" publish "$revision" "$test_root/qualified-hooks.json" "$test_root/market")
[ "$out" = "$test_root/market" ] || fail "publish printed $out"
published=$test_root/market/plugins/corvint
[ -f "$published/.mcp.json" ] || fail "published package lacks .mcp.json"
jq -e '.mcpServers.corvint' "$published/.mcp.json" >/dev/null || fail "published .mcp.json lacks the corvint server"
cmp -s "$published/hooks/hooks.json" "$test_root/qualified-hooks.json" || fail "published hooks are not the qualified rendering"
[ ! -e "$published/scripts" ] || fail "published package carries scripts/"
jq -e --arg rev "$revision" '.sourceRevision == $rev and (.pluginVersion | type == "string")' "$test_root/market/PUBLISH.json" >/dev/null || fail "PUBLISH.json does not bind the revision"
jq -e '.plugins[0].source.path == "./plugins/corvint"' "$test_root/market/.agents/plugins/marketplace.json" >/dev/null || fail "marketplace manifest does not point at the plugin"

if "$script" publish "$revision" "$test_root/qualified-hooks.json" "$test_root/market" 2>/dev/null; then
    fail "publish overwrote an existing output"
fi
if "$script" publish "$revision" "$test_root/missing.err" "$test_root/market2" 2>/dev/null; then
    fail "publish accepted a non-hooks file"
fi
[ ! -e "$test_root/market2" ] || fail "a refused publish left output"
ls -a "$test_root" | grep -q '^\.publish-codex-plugin' && fail "a refused publish left its staging directory"

echo "publish-codex-plugin_test: PASS"
