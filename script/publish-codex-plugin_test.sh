#!/bin/sh
set -eu

# AHI-951 / V1-2495: the committed Codex plugin passes the manifest-reference check, a package whose
# plugin.json references a missing mcpServers file is refused, and a publish from a committed
# revision carries .mcp.json, the qualified hooks and PUBLISH.json while omitting scripts/. A
# reference that a symlink carries outside the plugin is refused, and PUBLISH.json records the
# blob of the hooks actually published; a failed hash refuses the publish.

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

mkdir "$test_root/outside"
echo '{"mcpServers":{"x":{}}}' >"$test_root/outside/mcp.json"
cp -R "$plugin" "$test_root/linkdir"
skills=$(jq -r '.skills' "$plugin/.codex-plugin/plugin.json")
rm -rf "$test_root/linkdir/$skills"
ln -s "$test_root/outside" "$test_root/linkdir/${skills%/}"
if "$script" check "$test_root/linkdir" 2>"$test_root/linkdir.err"; then
    fail "a skills directory symlinked outside the plugin passed"
fi
grep -q 'resolves outside the plugin' "$test_root/linkdir.err" || fail "symlink refusal does not name the escape: $(cat "$test_root/linkdir.err")"

cp -R "$plugin" "$test_root/linkfile"
rm "$test_root/linkfile/.mcp.json"
ln -s "$test_root/outside/mcp.json" "$test_root/linkfile/.mcp.json"
if "$script" check "$test_root/linkfile" 2>/dev/null; then
    fail "an .mcp.json symlinked outside the plugin passed"
fi

cp -R "$plugin" "$test_root/innerlink"
mv "$test_root/innerlink/${skills%/}" "$test_root/innerlink/real-skills"
ln -s real-skills "$test_root/innerlink/${skills%/}"
"$script" check "$test_root/innerlink" || fail "a skills directory symlinked inside the plugin was refused"

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
blob=$(git hash-object "$published/hooks/hooks.json")
jq -e --arg blob "$blob" '.transforms[0] | contains("git blob " + $blob)' "$test_root/market/PUBLISH.json" >/dev/null || fail "PUBLISH.json does not record the published hooks blob $blob"
jq -e '.plugins[0].source.path == "./plugins/corvint"' "$test_root/market/.agents/plugins/marketplace.json" >/dev/null || fail "marketplace manifest does not point at the plugin"

if "$script" publish "$revision" "$test_root/qualified-hooks.json" "$test_root/market" 2>/dev/null; then
    fail "publish overwrote an existing output"
fi
if "$script" publish "$revision" "$test_root/missing.err" "$test_root/market2" 2>/dev/null; then
    fail "publish accepted a non-hooks file"
fi
[ ! -e "$test_root/market2" ] || fail "a refused publish left output"
mkdir "$test_root/bin"
real_git=$(command -v git)
printf '#!/bin/sh\n[ "$1" = hash-object ] && exit 1\nexec "%s" "$@"\n' "$real_git" >"$test_root/bin/git"
chmod +x "$test_root/bin/git"
if PATH="$test_root/bin:$PATH" "$script" publish "$revision" "$test_root/qualified-hooks.json" "$test_root/market3" 2>"$test_root/hash.err"; then
    fail "publish succeeded although hashing the staged hooks failed"
fi
grep -q 'cannot hash the staged hooks' "$test_root/hash.err" || fail "hash failure is not named: $(cat "$test_root/hash.err")"
[ ! -e "$test_root/market3" ] || fail "a publish whose hooks hash failed left output"
ls -a "$test_root" | grep -q '^\.publish-codex-plugin' && fail "a refused publish left its staging directory"

echo "publish-codex-plugin_test: PASS"
