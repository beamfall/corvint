#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# STA-V0-012 (docs/specs/spec-toolchain-adapters-v0.md): compare the pinned upstream releases in
# interop/spec-toolchains/versions.json with the npm registry's `latest` dist-tag (OpenSpec) and
# the GitHub latest release (Spec Kit). Operator-run; it needs the network and is never part of a
# gate. One line per toolchain:
#
#   <name> <pinned> CURRENT
#   <name> <pinned> BEHIND <latest>
#   <name> <pinned> UNAVAILABLE <reason>
#
# BEHIND means the registry's latest differs from the pin. It exits 1 only when some toolchain is
# BEHIND; UNAVAILABLE (offline, rate-limited, an unreadable answer) is reported and exits 0, never
# read as CURRENT. A setup error prints no verdict lines and exits 2: a missing jq or curl, or a
# manifest that is missing, empty, unparsable or lists no complete toolchain entry, so a broken
# manifest is never a silent success. Moving to a new release is a manifest edit, a green spec-toolchain-conformance
# job and a traced spec update in one change; this script never edits the manifest.
#
# SPEC_TOOLCHAIN_LATEST_RESPONSES=DIR reads `<name>.json` from DIR instead of the network (a missing
# file is UNAVAILABLE), so the verdicts are testable offline (script/spec-toolchain-latest_test.sh).

set -euo pipefail

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
manifest=${SPEC_TOOLCHAIN_MANIFEST:-$root/interop/spec-toolchains/versions.json}
responses=${SPEC_TOOLCHAIN_LATEST_RESPONSES:-}

command -v jq >/dev/null 2>&1 || { echo "spec-toolchain-latest: jq is required" >&2; exit 2; }
[ -n "$responses" ] || command -v curl >/dev/null 2>&1 || { echo "spec-toolchain-latest: curl is required" >&2; exit 2; }

fetch() { # fetch NAME URL -> response body on stdout, or nonzero
    if [ -n "$responses" ]; then
        cat -- "$responses/$1.json" 2>/dev/null
    else
        curl -fsSL --proto '=https' --max-time 30 -H 'Accept: application/json' "$2"
    fi
}

rows_filter='.toolchains as $t | if ($t | type) == "array" and ($t | length) > 0
    and all($t[]; [.name, .version, .source.kind, (.source.package // .source.repository)]
        | all(type == "string" and . != "" and (test("[\t\n]") | not)))
    then $t[] | [.name, .version, .source.kind, (.source.package // .source.repository)] | @tsv
    else error("no complete toolchain entries") end'
rows=$(jq -r "$rows_filter" "$manifest" 2>/dev/null) && [ -n "$rows" ] || {
    echo "spec-toolchain-latest: unreadable manifest $manifest (missing, empty, unparsable or without complete toolchain entries)" >&2
    exit 2
}

behind=0
while IFS=$'\t' read -r name pinned kind source; do
    case $kind in
        npm)
            url="https://registry.npmjs.org/-/package/$source/dist-tags"
            filter='.latest | strings'
            ;;
        git-tag)
            url="https://api.github.com/repos/$source/releases/latest"
            filter='select((.draft | not) and (.prerelease | not)) | .tag_name | strings | ltrimstr("v")'
            ;;
        *)
            printf '%s %s UNAVAILABLE unsupported-source-kind:%s\n' "$name" "$pinned" "$kind"
            continue
            ;;
    esac
    if ! body=$(fetch "$name" "$url"); then
        printf '%s %s UNAVAILABLE fetch-failed\n' "$name" "$pinned"
        continue
    fi
    latest=$(jq -r "$filter" <<<"$body" 2>/dev/null || true)
    if ! [[ $latest =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
        printf '%s %s UNAVAILABLE unreadable-response\n' "$name" "$pinned"
    elif [ "$latest" = "$pinned" ]; then
        printf '%s %s CURRENT\n' "$name" "$pinned"
    else
        printf '%s %s BEHIND %s\n' "$name" "$pinned" "$latest"
        behind=1
    fi
done <<<"$rows"

exit "$behind"
