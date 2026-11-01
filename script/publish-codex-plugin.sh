#!/bin/sh
set -eu

# AHI-951 / V1-2495: publish the Codex plugin from a committed revision into a local marketplace,
# and refuse any plugin tree whose .codex-plugin/plugin.json references a missing file.
#
# usage: script/publish-codex-plugin.sh check PLUGIN_DIR
#        script/publish-codex-plugin.sh publish REVISION QUALIFIED_HOOKS OUTPUT
#
# check refuses (exit 1) when a skills, mcpServers or hooks path declared by the manifest does not
# exist under PLUGIN_DIR or resolves, through a symlink, outside it. publish extracts integrations/codex/plugins/corvint at REVISION (so an
# uncommitted file can never ship), replaces hooks/hooks.json with QUALIFIED_HOOKS (the reviewed
# prepare-qualified-hooks rendering), omits scripts/, writes the marketplace manifest and
# PUBLISH.json (which hashes the staged hooks copy), and runs check on the result before it is visible. OUTPUT must not exist.

usage() { echo "usage: $0 check PLUGIN_DIR | publish REVISION QUALIFIED_HOOKS OUTPUT" >&2; exit 2; }

for tool in jq git; do
    command -v "$tool" >/dev/null 2>&1 || { echo "publish-codex-plugin: $tool is required" >&2; exit 2; }
done

# inside_plugin PLUGIN_DIR REF: REF, resolved physically, names a path inside PLUGIN_DIR. A
# directory resolves through every symlink with cd -P; a file resolves its parent that way and
# must not itself be a symlink, so no link can carry a reference out of the plugin.
inside_plugin() {
    root=$(CDPATH='' cd -P -- "$1" && pwd -P) || return 1
    target=$1/$2
    if [ -d "$target" ]; then
        resolved=$(CDPATH='' cd -P -- "$target" && pwd -P) || return 1
    else
        [ ! -L "$target" ] || return 1
        parent=$(CDPATH='' cd -P -- "$(dirname -- "$target")" && pwd -P) || return 1
        resolved=$parent/$(basename -- "$target")
    fi
    case $resolved in
        "$root" | "$root"/*) return 0 ;;
    esac
    return 1
}

# check_plugin PLUGIN_DIR: every manifest-declared path resolves physically inside PLUGIN_DIR.
check_plugin() {
    manifest=$1/.codex-plugin/plugin.json
    [ -f "$manifest" ] || { echo "publish-codex-plugin: $manifest is missing" >&2; return 1; }
    jq -e 'type == "object"' "$manifest" >/dev/null 2>&1 || { echo "publish-codex-plugin: $manifest is not a JSON object" >&2; return 1; }
    status=0
    for key in skills mcpServers hooks; do
        ref=$(jq -r --arg k "$key" '.[$k] // empty | if type == "string" then . else "NON_STRING" end' "$manifest")
        [ -n "$ref" ] || continue
        case $ref in
            NON_STRING) echo "publish-codex-plugin: $manifest $key is not a path string" >&2; status=1; continue ;;
            /* | *..*) echo "publish-codex-plugin: $manifest $key path $ref leaves the plugin" >&2; status=1; continue ;;
        esac
        if [ ! -e "$1/$ref" ]; then
            echo "publish-codex-plugin: $manifest declares $key $ref, which is missing from the package" >&2
            status=1
        elif ! inside_plugin "$1" "$ref"; then
            echo "publish-codex-plugin: $manifest $key path $ref resolves outside the plugin" >&2
            status=1
        elif [ "$key" = mcpServers ] && ! jq -e '.mcpServers | type == "object" and length > 0' "$1/$ref" >/dev/null 2>&1; then
            echo "publish-codex-plugin: $1/$ref declares no mcpServers object" >&2
            status=1
        fi
    done
    return $status
}

[ $# -ge 1 ] || usage
case $1 in
    check)
        [ $# -eq 2 ] || usage
        check_plugin "$2"
        ;;
    publish)
        [ $# -eq 4 ] || usage
        revision=$2 hooks=$3 output=$4
        source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
        commit=$(git -C "$source_root" rev-parse --verify --quiet "$revision^{commit}") || { echo "publish-codex-plugin: $revision is not a commit" >&2; exit 1; }
        jq -e '.hooks | type == "object"' "$hooks" >/dev/null 2>&1 || { echo "publish-codex-plugin: $hooks is not a hooks configuration" >&2; exit 1; }
        [ ! -e "$output" ] || { echo "publish-codex-plugin: $output already exists" >&2; exit 1; }
        parent=$(dirname "$output")
        stage=$(mktemp -d "$parent/.publish-codex-plugin.XXXXXX")
        trap 'rm -rf "$stage"' EXIT
        trap 'exit 129' HUP
        trap 'exit 130' INT
        trap 'exit 143' TERM
        mkdir -p "$stage/plugins" "$stage/.agents/plugins"
        git -C "$source_root" archive --format=tar "$commit" integrations/codex/plugins/corvint | tar -x -C "$stage"
        mv "$stage/integrations/codex/plugins/corvint" "$stage/plugins/corvint"
        rm -rf "$stage/integrations" "$stage/plugins/corvint/scripts"
        staged_hooks=$stage/plugins/corvint/hooks/hooks.json
        cp "$hooks" "$staged_hooks"
        # The staged copy is the published byte stream, so it is the one validated and hashed.
        jq -e '.hooks | type == "object"' "$staged_hooks" >/dev/null 2>&1 || { echo "publish-codex-plugin: the staged hooks are not a hooks configuration" >&2; exit 1; }
        hooks_blob=$(git hash-object -- "$staged_hooks") && [ -n "$hooks_blob" ] || { echo "publish-codex-plugin: cannot hash the staged hooks" >&2; exit 1; }
        check_plugin "$stage/plugins/corvint"
        version=$(jq -er '.version' "$stage/plugins/corvint/.codex-plugin/plugin.json")
        jq -n '{name: "corvint-full-candidate", interface: {displayName: "Corvint (qualified candidate)"}, plugins: [{name: "corvint", source: {source: "local", path: "./plugins/corvint"}, policy: {installation: "AVAILABLE", authentication: "ON_INSTALL"}, category: "Productivity"}]}' >"$stage/.agents/plugins/marketplace.json"
        jq -n --arg rev "$commit" --arg version "$version" --arg hooks "$hooks_blob" \
            '{sourceRevision: $rev, pluginVersion: $version, transforms: ["hooks/hooks.json replaced by the prepare-qualified-hooks rendering (git blob " + $hooks + ")", "scripts/ omitted: the adapter lives in the root release"], checked: "AHI-951 manifest references resolve"}' >"$stage/PUBLISH.json"
        mv "$stage" "$output"
        trap - EXIT
        echo "$output"
        ;;
    *) usage ;;
esac
