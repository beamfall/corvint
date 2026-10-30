#!/bin/sh
set -euf

# AHI-173 (docs/specs/agent-harness-integration-v0.md): bump or verify every version location of
# one host package together, so a shipped-content change needs one command rather than a hand edit
# of each manifest that host-package-versions-check (AHI-020) and the AHI-010 matrix test then
# compare (V1-2145).
#
#   script/host-package-version.sh verify [PKG...]   every location equals the manifest version
#   script/host-package-version.sh bump PKG [X.Y.Z]  set every location (default: next patch)
#
# Edits are textual and format-preserving: only the one quoted value at each location changes,
# and each JSON edit is checked against jq's setpath of the same path before it is written.
# Recorded evidence fields that name an older adapter version (lastValidation, lastConformance,
# hostVersionEvidenceAdapterVersion) are never touched. A lit:NAME filter is a source-code
# constant (AHI-240): the one quoted string assigned to NAME, as in NAME='X' or NAME = "X". A
# tuple:FIELD filter is a spec's declared host tuple (AHI-522, V1-3170): the quoted FIELD value
# inside the one `"tuple": {...}` object on a single line, as in the Pi capability inventory.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
for tool in jq perl awk; do
    command -v "$tool" >/dev/null 2>&1 || { printf 'host package version: %s is required\n' "$tool" >&2; exit 1; }
done

packages="claude-code codex gemini-cli opencode pi speckit openspec"

# known PKG: refuse a package outside the list before any command substitution swallows the exit.
known() {
    case " $packages " in
        *" $1 "*) ;;
        *) printf 'host package version: unknown package %s (one of: %s)\n' "$1" "$packages" >&2; exit 2 ;;
    esac
}

# locations PKG: one "path|filter" line per version location; the first is the package manifest.
# A filter is a jq path expression, yaml:PARENT.CHILD, lit:NAME or tuple:FIELD.
locations() {
    matrix='integrations/compatibility.json|.entries[]|select(.host=="'$1'")|.adapterVersion'
    case $1 in
        claude-code) printf '%s\n' \
            'integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json|.version' \
            'integrations/claude-code/.claude-plugin/marketplace.json|.plugins[]|select(.name=="corvint")|.version' \
            'integrations/claude-code/plugins/corvint/compatibility.json|.adapter.version' "$matrix" ;;
        codex) printf '%s\n' \
            'integrations/codex/plugins/corvint/.codex-plugin/plugin.json|.version' \
            'integrations/codex/plugins/corvint/compatibility.json|.adapter.version' "$matrix" ;;
        gemini-cli) printf '%s\n' \
            'integrations/gemini-cli/gemini-extension.json|.version' \
            'integrations/gemini-cli/compatibility.json|.adapterVersion' "$matrix" ;;
        opencode) printf '%s\n' \
            'integrations/opencode/package.json|.version' \
            'integrations/opencode/package.json|.corvintIntegration.adapterVersion' "$matrix" \
            'integrations/opencode/src/runtime.js|lit:ADAPTER_VERSION' ;;
        pi) printf '%s\n' \
            'integrations/pi/package.json|.version' \
            'integrations/pi/package.json|.corvintIntegration.adapterVersion' \
            'integrations/pi/compatibility.json|.adapterVersion' "$matrix" \
            'integrations/pi/runtime.js|lit:PI_ADAPTER_VERSION' \
            'cmd/corvint/pi_adapter.go|lit:piAdapterVersion' \
            'docs/specs/pi-workflow-v0.md|tuple:adapterVersion' ;;
        speckit) printf '%s\n' 'protocol/integrations/speckit/corvint/extension.yml|yaml:extension.version' ;;
        openspec) printf '%s\n' 'protocol/integrations/openspec/skills/corvint-openspec/SKILL.md|yaml:metadata.version' ;;
        *) printf 'host package version: unknown package %s (one of: %s)\n' "$1" "$packages" >&2; exit 2 ;;
    esac
}

# yaml_field PARENT CHILD FILE: the unquoted scalar of CHILD in FILE's first top-level PARENT: block.
yaml_field() {
    awk -v parent="$1" -v child="$2" '
        inside && /^[^[:space:]]/ { exit }
        $0 ~ "^" parent ":[[:space:]]*$" { inside = 1; next }
        inside && $0 ~ "^[[:space:]]+" child ":" {
            value = $0
            sub("^[[:space:]]+" child ":[[:space:]]*", "", value)
            sub("[[:space:]]+$", "", value)
            gsub(/^"|"$/, "", value)
            print value
            found = 1
            exit
        }
        END { if (!found) exit 1 }' "$3"
}

# lit_field NAME FILE: the quoted string assigned to NAME, only when exactly one assignment exists.
lit_field() {
    N=$1 perl -0ne '@m = /\b\Q$ENV{N}\E\s*=\s*["\x27]([^"\x27\n]*)["\x27]/g; exit 1 if @m != 1; print $m[0]' "$2"
}

# tuple_field FIELD FILE: the quoted FIELD value of the one single-line "tuple" object.
tuple_field() {
    F=$1 perl -0ne '@m = /"tuple":\s*\{[^}\n]*"\Q$ENV{F}\E":\s*"([^"\n]*)"/g; exit 1 if @m != 1; print $m[0]' "$2"
}

# read_field PATH FILTER: the value at one location, or a refusal naming it.
read_field() {
    case $2 in
        lit:*) value=$(lit_field "${2#lit:}" "$1") || value= ;;
        tuple:*) value=$(tuple_field "${2#tuple:}" "$1") || value= ;;
        yaml:*) key=${2#yaml:}; value=$(yaml_field "${key%%.*}" "${key#*.}" "$1") || value= ;;
        *) value=$(jq -er "[$2] | if length == 1 then .[0] else error end" "$1" 2>/dev/null) || value= ;;
    esac
    [ -n "$value" ] || { printf 'host package version: %s: no single value at %s\n' "$1" "$2" >&2; exit 1; }
    printf '%s' "$value"
}

# set_field PATH FILTER OLD NEW: rewrite only the quoted OLD at that location to NEW.
set_field() {
    file=$1 filter=$2 old=$3 new=$4
    tmp="$file.version-tmp.$$"
    case $filter in
        lit:*)
            N=${filter#lit:} O=$old V=$new perl -0pe 's/(\b\Q$ENV{N}\E\s*=\s*["\x27])\Q$ENV{O}\E(["\x27])/$1$ENV{V}$2/' "$file" >"$tmp"
            ;;
        tuple:*)
            F=${filter#tuple:} O=$old V=$new perl -0pe 's/("tuple":\s*\{[^}\n]*"\Q$ENV{F}\E":\s*")\Q$ENV{O}\E"/$1$ENV{V}"/' "$file" >"$tmp"
            ;;
        yaml:*)
            key=${filter#yaml:}
            awk -v parent="${key%%.*}" -v child="${key#*.}" -v old="$old" -v new="$new" '
                done || !(inside && $0 ~ "^[[:space:]]+" child ":") {
                    if (inside && /^[^[:space:]]/) inside = 0
                    if ($0 ~ "^" parent ":[[:space:]]*$") inside = 1
                    print; next
                }
                { i = index($0, old); print substr($0, 1, i - 1) new substr($0, i + length(old)); done = 1 }
            ' "$file" >"$tmp"
            ;;
        *)
            key=$(jq -r "path($filter) | last" "$file")
            want=$(jq -S --argjson p "$(jq -c "path($filter)" "$file")" --arg v "$new" 'setpath($p; $v)' "$file")
            count=$(K=$key O=$old perl -0ne '$n = () = /"\Q$ENV{K}\E"\s*:\s*"\Q$ENV{O}\E"/g; print $n' "$file")
            i=0
            while [ "$i" -lt "$count" ]; do
                i=$((i + 1))
                K=$key O=$old N=$new I=$i perl -0pe '$c = 0; s/("\Q$ENV{K}\E"\s*:\s*")\Q$ENV{O}\E"/++$c == $ENV{I} ? "$1$ENV{N}\"" : $&/ge' "$file" >"$tmp"
                [ "$(jq -S . "$tmp")" = "$want" ] && break
                rm -f "$tmp"
            done
            ;;
    esac
    if [ ! -f "$tmp" ] || [ "$(filter_value "$tmp" "$filter")" != "$new" ]; then
        rm -f "$tmp"
        printf 'host package version: %s: cannot rewrite %s at %s\n' "$file" "$old" "$filter" >&2
        exit 1
    fi
    chmod "$(stat -c %a "$file" 2>/dev/null || stat -f %Lp "$file")" "$tmp"
    mv -f "$tmp" "$file"
}

filter_value() { (read_field "$1" "$2") 2>/dev/null || true; }

# verify_package PKG: prints "PKG VERSION" when every location agrees; otherwise names each
# disagreeing location and returns 1.
verify_package() {
    manifest= version= ok=0
    for location in $(locations "$1"); do
        path=${location%%|*}
        value=$(read_field "$path" "${location#*|}")
        if [ -z "$version" ]; then
            manifest=$path version=$value
        elif [ "$value" != "$version" ]; then
            printf 'host package version: %s: %s has %s, manifest %s has %s\n' "$1" "$path" "$value" "$manifest" "$version" >&2
            ok=1
        fi
    done
    [ "$ok" -eq 0 ] && printf '%s %s\n' "$1" "$version"
    return "$ok"
}

case ${1:-} in
    verify)
        shift
        [ "$#" -gt 0 ] || set -- $packages
        status=0
        for package in "$@"; do known "$package"; done
        for package in "$@"; do verify_package "$package" || status=1; done
        exit $status
        ;;
    bump)
        [ "$#" -ge 2 ] && [ "$#" -le 3 ] || { printf 'usage: %s bump PKG [X.Y.Z]\n' "$0" >&2; exit 2; }
        package=$2
        known "$package"
        current=$(verify_package "$package") || { printf 'host package version: %s: locations disagree; fix them before bumping\n' "$package" >&2; exit 1; }
        current=${current#* }
        new=${3:-$(printf '%s' "$current" | awk -F. '{ printf "%d.%d.%d", $1, $2, $3 + 1 }')}
        printf '%s' "$new" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || { printf 'host package version: %s is not X.Y.Z\n' "$new" >&2; exit 2; }
        [ "$new" != "$current" ] || { printf 'host package version: %s is already %s\n' "$package" "$new" >&2; exit 1; }
        for location in $(locations "$package"); do
            set_field "${location%%|*}" "${location#*|}" "$current" "$new"
        done
        verify_package "$package" >/dev/null
        printf 'host package version: %s %s -> %s\n' "$package" "$current" "$new"
        ;;
    *)
        printf 'usage: %s verify [PKG...] | bump PKG [X.Y.Z]\n' "$0" >&2
        exit 2
        ;;
esac
