#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# STA-V0-011 (docs/specs/spec-toolchain-adapters-v0.md): check Corvint's OpenSpec and Spec Kit
# intent forms against the pinned upstream CLIs. `make spec-toolchain-conformance` and the CI job
# of the same name both run this script. It needs the network for the npm registry, GitHub and
# PyPI, and Node >= 22.12, npm, Python >= 3.11, jq, git and (unless CORVINT_BIN is set) Go.
#
#   1. Install the STA-V0-001 versions with checksum verification: OpenSpec with `npm ci` from the
#      committed lockfile, whose integrity must equal interop/spec-toolchains/versions.json, and
#      Spec Kit from the pinned commit's source archive with `pip --require-hashes`.
#   2. Run the upstream validators on the STA-V0-010 fixtures: `openspec validate --all --strict`
#      and Spec Kit's `check-prerequisites.sh --json`.
#   3. Assert that `corvint ocm prepare --intent-form ...` derives exactly upstream's view: the
#      `openspec show --json` requirement names for every main spec and delta, and the spec
#      template's FR/SC line grammar for Spec Kit (also over upstream's own spec template).
#   4. Install the STA-V0-008 extension with `specify extension add --dev`, and load the
#      STA-V0-009 config snippet through `openspec instructions`.
#
# Any mismatch exits 1. Everything runs in one temporary directory that is removed on exit;
# nothing is installed globally and the repository is only read.
#
# CORVINT_BIN may name a corvint binary built from the current tree; otherwise the script builds
# one with `go build ./cmd/corvint`. SPEC_TOOLCHAIN_PYTHON overrides the python3 interpreter.

set -euo pipefail

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
toolchains=$root/interop/spec-toolchains
manifest=$toolchains/versions.json
fixtures=$toolchains/fixtures
python=${SPEC_TOOLCHAIN_PYTHON:-python3}

failures=0
fail() {
    printf 'spec-toolchain-conformance: FAIL: %s\n' "$*" >&2
    failures=$((failures + 1))
}
die() {
    printf 'spec-toolchain-conformance: %s\n' "$*" >&2
    exit 1
}
step() { printf '== %s\n' "$*"; }

for tool in node npm jq git "$python"; do
    command -v "$tool" >/dev/null 2>&1 || die "missing required tool: $tool"
done
if command -v sha256sum >/dev/null 2>&1; then
    sha256() { sha256sum | cut -d' ' -f1; }
else
    sha256() { shasum -a 256 | cut -d' ' -f1; }
fi
node -e 'const [a,b]=process.versions.node.split(".").map(Number); process.exit(a>22||(a===22&&b>=12)?0:1)' ||
    die "node $(node --version) is older than OpenSpec 1.16.0's engines.node >=22.12.0"
"$python" -I -c 'import sys; sys.exit(0 if sys.version_info >= (3, 11) else 1)' ||
    die "$python is older than Spec Kit's requires-python >=3.11"

work=$(mktemp -d "${TMPDIR:-/tmp}/spec-toolchain-conformance.XXXXXX")
cleanup() { rm -rf -- "$work"; }
trap cleanup EXIT
trap 'cleanup; exit 130' INT
trap 'cleanup; exit 143' TERM

# Upstream tools must not phone home or prompt; HOME and the caches stay inside $work.
export HOME=$work/home XDG_CONFIG_HOME=$work/home/.config XDG_CACHE_HOME=$work/home/.cache
export DO_NOT_TRACK=1 OPENSPEC_TELEMETRY=0 CI=${CI:-true} NO_COLOR=1
export npm_config_cache=$work/npm-cache npm_config_update_notifier=false npm_config_fund=false npm_config_audit=false
export PIP_CACHE_DIR=$work/pip-cache PIP_DISABLE_PIP_VERSION_CHECK=1 PIP_NO_INPUT=1
export GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=conformance GIT_AUTHOR_EMAIL=conformance@example.invalid
export GIT_COMMITTER_NAME=conformance GIT_COMMITTER_EMAIL=conformance@example.invalid
mkdir -p "$HOME"
export TMPDIR=$work/tmp
mkdir -p "$TMPDIR"

entry() { jq -er --arg name "$1" --arg field "$2" '.toolchains[] | select(.name == $name) | getpath($field | split("."))' "$manifest"; }

# --- 1. install the pinned versions with checksum verification ----------------------------------
step "install: OpenSpec $(entry openspec version) and Spec Kit $(entry spec-kit version) (versions.json)"

os_version=$(entry openspec version)
os_package=$(entry openspec source.package)
os_tarball=$(entry openspec source.tarball)
os_integrity=$(entry openspec npmIntegrity)
os_sha256=$(entry openspec tarballSha256)
lock=$toolchains/npm/package-lock.json
[ "$(jq -r --arg p "$os_package" '.dependencies[$p]' "$toolchains/npm/package.json")" = "$os_version" ] ||
    die "npm/package.json does not pin $os_package to exactly $os_version"
locked=$(jq -c --arg p "node_modules/$os_package" '.packages[$p] | {version, resolved, integrity}' "$lock")
[ "$locked" = "$(jq -nc --arg v "$os_version" --arg r "$os_tarball" --arg i "$os_integrity" '{version: $v, resolved: $r, integrity: $i}')" ] ||
    die "npm/package-lock.json entry $locked disagrees with versions.json"
# A full SRI SHA-512 is `sha512-` and 88 base64 characters that decode to exactly 64 bytes (the
# last data character carries 2 bits, so it is one of A, Q, g or w). A truncated or empty digest
# would let npm ci verify nothing, so it is refused before npm runs; TestSTAV0001_VersionsManifest
# enforces the same rule.
sri_sha512='^sha512-[A-Za-z0-9+/]{85}[AQgw]==$'
unpinned=$(jq -r --arg sri "$sri_sha512" '.packages | to_entries[] | select(.key != "")
    | select(((.value.integrity // "") | test($sri) | not)
        or ((.value.resolved // "") | startswith("https://registry.npmjs.org/") | not)) | .key' "$lock")
[ -z "$unpinned" ] || die "npm/package-lock.json entries without a full registry sha512 integrity: $unpinned"
[[ $os_integrity =~ $sri_sha512 ]] || die "versions.json OpenSpec npmIntegrity is not a full sha512 digest"

# The manifest's SHA-256 is cross-checked against the registry tarball, and the tarball against the
# lockfile integrity, so the three records cannot drift apart.
mkdir -p "$work/download"
curl -fsSL --proto '=https' -o "$work/download/openspec.tgz" "$os_tarball"
[ "$(sha256 <"$work/download/openspec.tgz")" = "$os_sha256" ] || die "OpenSpec tarball SHA-256 differs from versions.json"
[ "sha512-$(openssl dgst -sha512 -binary "$work/download/openspec.tgz" | openssl base64 -A)" = "$os_integrity" ] ||
    die "OpenSpec tarball sha512 differs from versions.json npmIntegrity"

mkdir -p "$work/npm"
cp "$toolchains/npm/package.json" "$lock" "$work/npm/"
(cd "$work/npm" && npm ci --ignore-scripts --no-audit --no-fund --registry=https://registry.npmjs.org/ </dev/null >/dev/null)
openspec=$work/npm/node_modules/.bin/openspec
[ "$("$openspec" --version </dev/null)" = "$os_version" ] || die "installed openspec does not report $os_version"

sk_version=$(entry spec-kit version)
sk_commit=$(entry spec-kit commit)
sk_archive=$(entry spec-kit source.archive)
sk_sha256=$(entry spec-kit archiveSha256)
requirements=$toolchains/speckit/requirements.txt
case $sk_archive in
    "https://github.com/github/spec-kit/archive/$sk_commit.tar.gz") ;;
    *) die "versions.json Spec Kit archive does not name the pinned commit $sk_commit" ;;
esac
grep -Fqx "specify-cli @ $sk_archive \\" "$requirements" ||
    die "speckit/requirements.txt does not install specify-cli from $sk_archive"
[ "$(grep -A1 -F "specify-cli @ $sk_archive" "$requirements" | sed -n 's/^ *--hash=sha256:\([0-9a-f]*\).*/\1/p')" = "$sk_sha256" ] ||
    die "speckit/requirements.txt specify-cli hash differs from versions.json archiveSha256"
curl -fsSL --proto '=https' -o "$work/download/spec-kit.tar.gz" "$sk_archive"
[ "$(sha256 <"$work/download/spec-kit.tar.gz")" = "$sk_sha256" ] || die "Spec Kit archive SHA-256 differs from versions.json"
[ "$(gzip -dc "$work/download/spec-kit.tar.gz" | git get-tar-commit-id)" = "$sk_commit" ] ||
    die "Spec Kit archive does not carry commit $sk_commit"

"$python" -I -m venv "$work/venv"
pip=("$work/venv/bin/python" -I -m pip install --quiet --require-hashes)
# The build backend is installed first, from hashes, so building specify-cli needs no unhashed
# isolated build environment; every other distribution must be a hashed wheel.
"${pip[@]}" --only-binary=:all: -r "$toolchains/speckit/build-requirements.txt" </dev/null
"${pip[@]}" --no-build-isolation --only-binary=:all: --no-binary=specify-cli -r "$requirements" </dev/null
specify=$work/venv/bin/specify
installed=$("$work/venv/bin/python" -I -c 'import importlib.metadata as m; print(m.version("specify-cli"))')
[ "$installed" = "$sk_version" ] || die "installed specify-cli is $installed, want $sk_version"
core_pack=$("$work/venv/bin/python" -I -c 'import os, specify_cli; print(os.path.join(os.path.dirname(specify_cli.__file__), "core_pack"))')
[ -d "$core_pack/scripts/bash" ] && [ -f "$core_pack/templates/spec-template.md" ] ||
    die "specify-cli $installed has no bundled core_pack scripts and spec template"
echo "installed openspec $os_version (npm ci, lockfile integrity) and specify-cli $installed (pip --require-hashes, commit $sk_commit)"

# --- 2. upstream validators on the fixtures -----------------------------------------------------
step "validate: openspec validate --all --strict; check-prerequisites.sh --json"

cp -R "$fixtures/openspec" "$work/openspec-project"
if (cd "$work/openspec-project" && "$openspec" validate --all --strict --json </dev/null >"$work/validate.json"); then
    jq -e '.summary.totals.failed == 0 and .summary.totals.items >= 3' "$work/validate.json" >/dev/null ||
        fail "openspec validate summary: $(jq -c .summary "$work/validate.json")"
    echo "openspec validate --all --strict: $(jq -c .summary.totals "$work/validate.json")"
else
    fail "openspec validate --all --strict failed: $(jq -c '[.items[] | select(.valid | not) | {id, issues}]' "$work/validate.json" 2>/dev/null || cat "$work/validate.json")"
fi

# Spec Kit has no project validator; check-prerequisites.sh is the check its commands run before
# they read a feature. `specify init` would copy the same bundled scripts into .specify/scripts/.
speckit_project=$work/speckit-project
cp -R "$fixtures/speckit" "$speckit_project"
mkdir -p "$speckit_project/.specify/scripts/bash"
cp "$core_pack"/scripts/bash/*.sh "$speckit_project/.specify/scripts/bash/"
git -C "$speckit_project" init -q -b main
feature_dir=$(jq -er .feature_directory "$speckit_project/.specify/feature.json")
if prereq=$(cd "$speckit_project" && bash .specify/scripts/bash/check-prerequisites.sh --json --require-tasks --include-tasks </dev/null); then
    jq -e --arg dir "$(cd "$speckit_project" && pwd -P)/$feature_dir" \
        '(.FEATURE_DIR | sub("^/private"; "")) == ($dir | sub("^/private"; "")) and (.AVAILABLE_DOCS | index("tasks.md"))' \
        <<<"$prereq" >/dev/null || fail "check-prerequisites.sh --json reported $prereq for $feature_dir"
    echo "check-prerequisites.sh --json: $prereq"
else
    fail "check-prerequisites.sh --json --require-tasks failed for $feature_dir"
fi

# --- 3. Corvint's derived obligations equal upstream's view -------------------------------------
step "obligations: corvint ocm prepare --intent-form against upstream"

if [ -n "${CORVINT_BIN:-}" ]; then
    corvint=$CORVINT_BIN
else
    corvint=$work/corvint
    (cd "$root" && go build -o "$corvint" ./cmd/corvint)
fi

repo=$work/repo
mkdir -p "$repo"
cp -R "$fixtures/openspec/." "$repo/"
cp -R "$fixtures/speckit/." "$repo/"
mkdir -p "$repo/specs/000-upstream-template"
cp "$core_pack/templates/spec-template.md" "$repo/specs/000-upstream-template/spec.md"
# Out-of-section IDs: the fixture plus an FR and an SC item under `## Assumptions`, which neither
# Corvint's form nor the template grammar may count (STA-V0-005).
out_of_section=specs/002-out-of-section/spec.md
mkdir -p "$repo/${out_of_section%/spec.md}"
{
    cat "$fixtures/speckit/specs/001-session-timeout/spec.md"
    printf '%s\n' '' '- **FR-999**: An assumption written as a requirement is not one.' \
        '- **SC-999**: An assumption written as a criterion is not one.'
} >"$repo/$out_of_section"
[ "$(grep '^## ' "$repo/$out_of_section" | tail -n 1)" = "## Assumptions" ] ||
    die "the Spec Kit fixture no longer ends in an ## Assumptions section"
printf 'base\n' >"$repo/conformance-marker.txt"
git -C "$repo" init -q -b main
git -C "$repo" add -A
git -C "$repo" -c commit.gpgsign=false commit -qm base
base=$(git -C "$repo" rev-parse HEAD)
printf 'target\n' >"$repo/conformance-marker.txt"
git -C "$repo" -c commit.gpgsign=false commit -qam target
target=$(git -C "$repo" rev-parse HEAD)
(cd "$repo" && "$corvint" cem prepare --spec cem/1.0 --base "$base" --target "$target" </dev/null >/dev/null)

maps=0
prepare() { # prepare INTENT FORM -> envelope JSON on stdout (runs in a subshell; callers count)
    (cd "$repo" && "$corvint" ocm prepare --map ".corvint/conformance-$(printf "%s" "$1" | sha256 | cut -c1-16).ocm.json" --intent "$1" \
        --intent-form "$2" --expected-base "$base" --target "$target" </dev/null)
}
# The STA-V0-002 obligation ID rule, recomputed independently for every native name.
check_ids() { # check_ids INTENT ENVELOPE
    local id name want
    while IFS=$'\t' read -r id name; do
        want="OSR-$(printf '%s' "$name" | sha256 | cut -c1-12 | tr '[:lower:]' '[:upper:]')"
        [ "$id" = "$want" ] || fail "$1: obligation $id for \"$name\" should be $want"
    done < <(jq -r '.nativeNames | to_entries[] | [.key, .value] | @tsv' <<<"$2")
}
same() { # same LABEL CORVINT_JSON UPSTREAM_JSON (compared as sorted arrays)
    if [ "$(jq -cS 'sort' <<<"$2")" = "$(jq -cS 'sort' <<<"$3")" ]; then
        printf 'ok   %s: %s\n' "$1" "$(jq -c 'length' <<<"$2") item(s)"
    else
        fail "$1: corvint $(jq -cS sort <<<"$2") != upstream $(jq -cS sort <<<"$3")"
    fi
}
names_of() { jq -c '[.worklist[].id as $id | .nativeNames[$id]]' <<<"$1"; }

while IFS= read -r spec; do
    id=${spec#openspec/specs/}
    id=${id%/spec.md}
    maps=$((maps + 1))
    envelope=$(prepare "$spec" openspec-spec) || { fail "$spec: corvint ocm prepare --intent-form openspec-spec refused"; continue; }
    upstream=$(cd "$work/openspec-project" && "$openspec" show "$id" --type spec --json </dev/null | jq -c '[.requirements[].name]')
    same "openspec-spec $spec" "$(names_of "$envelope")" "$upstream"
    check_ids "$spec" "$envelope"
done < <(cd "$repo" && find openspec/specs -name spec.md -type f | LC_ALL=C sort)

# A delta is shown through its change. OpenSpec shows only active changes, so an archived change
# is copied, unchanged, into a scratch project as an active change of the same id.
deltas=0
while IFS= read -r delta; do
    deltas=$((deltas + 1))
    change_dir=${delta%%/specs/*}
    capability=${delta#"$change_dir"/specs/}
    capability=${capability%/spec.md}
    change=${change_dir##*/}
    project=$work/openspec-project
    case $change_dir in
        openspec/changes/archive/*)
            change=${change#????-??-??-}
            project=$work/archived-$deltas
            mkdir -p "$project/openspec/changes"
            cp "$fixtures/openspec/openspec/config.yaml" "$project/openspec/"
            cp -R "$repo/$change_dir" "$project/openspec/changes/$change"
            ;;
    esac
    maps=$((maps + 1))
    envelope=$(prepare "$delta" openspec-delta) || { fail "$delta: corvint ocm prepare --intent-form openspec-delta refused"; continue; }
    shown=$(cd "$project" && "$openspec" show "$change" --type change --json </dev/null |
        jq -c --arg spec "$capability" '[.deltas[] | select(.spec == $spec)]')
    same "openspec-delta $delta obligations" "$(names_of "$envelope")" \
        "$(jq -c '[.[] | select(.operation == "ADDED" or .operation == "MODIFIED") | ((.requirements // []) + [.requirement // empty]) | map(.name) | unique[]]' <<<"$shown")"
    same "openspec-delta $delta excludedRequirements" "$(jq -c '.excludedRequirements' <<<"$envelope")" \
        "$(jq -c '[.[] | if .operation == "RENAMED" then {operation, from: .rename.from, to: .rename.to}
            elif .operation == "REMOVED" then ((.requirements // []) + [.requirement // empty]) | map(.name) | unique[] as $n | {operation: "REMOVED", name: $n}
            else empty end]' <<<"$shown")"
    check_ids "$delta" "$envelope"
done < <(cd "$repo" && find openspec/changes -path '*/specs/*' -name spec.md -type f | LC_ALL=C sort)
[ "$deltas" -ge 2 ] || fail "expected an active and an archived OpenSpec delta, found $deltas"

# Spec Kit has no parser for FR/SC IDs; its grammar is the spec template's
# `- **FR-001**: ` / `- **SC-001**: ` lines under `## Requirements` / `## Success Criteria`.
template=$core_pack/templates/spec-template.md
grep -Eq '^## Requirements' "$template" && grep -Eq '^## Success Criteria' "$template" &&
    grep -Eq '^- \*\*FR-[0-9]{3}\*\*: ' "$template" && grep -Eq '^- \*\*SC-[0-9]{3}\*\*: ' "$template" &&
    grep -Fq '[NEEDS CLARIFICATION' "$template" ||
    fail "upstream spec-template.md no longer uses the FR/SC line grammar STA-V0-005 reads"
grammar_ids() { # grammar_ids FILE CLARIFY(0|1): unfenced FR/SC item IDs, or only those needing clarification
    # FR lines count only inside `## Requirements` and SC lines only inside `## Success Criteria`
    # (each with its ### subsections, and the template's `*(mandatory)*` annotation); every other
    # level-2 heading ends the section.
    awk -v clarify="$2" '
        function flush() { if (id != "" && (clarify == 0 || text ~ /\[NEEDS CLARIFICATION/)) print id; id = ""; text = "" }
        /^ {0,3}(```|~~~)/ { flush(); fenced = !fenced; next }
        fenced { next }
        /^##( |$)/ {
            flush()
            section = ""
            if ($0 ~ /^## Requirements( \*\([^)]*\)\*)?[ \t]*$/) section = "FR"
            else if ($0 ~ /^## Success Criteria( \*\([^)]*\)\*)?[ \t]*$/) section = "SC"
            next
        }
        /^- \*\*(FR|SC)-[0-9][0-9][0-9]+\*\*: / {
            flush()
            if (substr($0, 5, 2) != section) next
            id = $0; sub(/^- \*\*/, "", id); sub(/\*\*:.*/, "", id); text = $0; next
        }
        id != "" && /^  / { text = text "\n" $0; next }
        { flush() }
        END { flush() }
    ' "$1" | jq -Rsc 'split("\n") | map(select(. != ""))'
}
while IFS= read -r spec; do
    maps=$((maps + 1))
    envelope=$(prepare "$spec" speckit-spec) || { fail "$spec: corvint ocm prepare --intent-form speckit-spec refused"; continue; }
    same "speckit-spec $spec FR/SC IDs" "$(jq -c '[.worklist[].id] + [.excludedRequirements[].id]' <<<"$envelope")" "$(grammar_ids "$repo/$spec" 0)"
    same "speckit-spec $spec needs-clarification" "$(jq -c '[.excludedRequirements[] | select(.reason == "needs-clarification") | .id]' <<<"$envelope")" "$(grammar_ids "$repo/$spec" 1)"
    if [ "$spec" = "$out_of_section" ]; then
        out_ids=$(jq -c --argjson oracle "$(grammar_ids "$repo/$spec" 0)" \
            '[([.worklist[].id] + [.excludedRequirements[].id] + $oracle)[] | select(. == "FR-999" or . == "SC-999")]' <<<"$envelope")
        if [ "$out_ids" = "[]" ]; then
            echo "ok   speckit-spec $spec: FR-999 and SC-999 under ## Assumptions excluded by both sides"
        else
            fail "$spec: out-of-section IDs counted: $out_ids"
        fi
    fi
done < <(cd "$repo" && find specs -mindepth 2 -maxdepth 2 -name spec.md -type f | LC_ALL=C sort)

# --- 4. host packaging against the upstream CLIs ------------------------------------------------
step "integrations: specify extension add --dev; openspec instructions with the config snippet"

if (cd "$speckit_project" && "$specify" extension add --dev "$root/protocol/integrations/speckit/corvint" </dev/null >"$work/extension-add.log" 2>&1); then
    listed=$(cd "$speckit_project" && "$specify" extension list --json </dev/null)
    jq -e '[.. | objects | select((.id // .name // "") == "corvint")] | length > 0' <<<"$listed" >/dev/null ||
        fail "specify extension list --json does not show corvint: $listed"
    hooks=$("$work/venv/bin/python" -I -c 'import json, sys, yaml; print(json.dumps(yaml.safe_load(open(sys.argv[1]))["hooks"]))' \
        "$speckit_project/.specify/extensions.yml")
    jq -e '(.before_implement | any(.extension == "corvint" and .command == "speckit.corvint.context" and .optional == true))
        and (.after_implement | any(.extension == "corvint" and .command == "speckit.corvint.review" and .optional == true))' \
        <<<"$hooks" >/dev/null || fail "registered hooks are not the optional before/after_implement pair: $hooks"
    echo "specify extension add --dev: installed corvint with hooks $(jq -c '{before_implement: [.before_implement[].command], after_implement: [.after_implement[].command]}' <<<"$hooks")"
else
    fail "specify extension add --dev failed: $(tail -n 20 "$work/extension-add.log")"
fi

snippet_project=$work/openspec-snippet
cp -R "$fixtures/openspec" "$snippet_project"
cp "$root/protocol/integrations/openspec/config.yaml" "$snippet_project/openspec/config.yaml"
active_change=$(cd "$snippet_project" && find openspec/changes -mindepth 1 -maxdepth 1 -type d ! -name archive | LC_ALL=C sort | head -n 1)
active_change=${active_change##*/}
if (cd "$snippet_project" && "$openspec" validate --all --strict --json </dev/null >/dev/null) &&
    apply=$(cd "$snippet_project" && "$openspec" instructions apply --change "$active_change" --json </dev/null) &&
    tasks=$(cd "$snippet_project" && "$openspec" instructions tasks --change "$active_change" --json </dev/null); then
    for check in 'apply .context' 'apply .operationGuidance' 'tasks .rules'; do
        set -- $check
        if [ "$1" = apply ]; then doc=$apply; else doc=$tasks; fi
        jq -e "($2 | tostring) as \$s | (\$s | contains(\"corvint context\")) and (\$s | contains(\"corvint review\"))" <<<"$doc" >/dev/null ||
            fail "openspec instructions $1 $2 does not carry the Corvint context and review steps"
    done
    echo "openspec instructions: config snippet reaches apply context, apply guidance and tasks rules"
else
    fail "openspec rejected the protocol/integrations/openspec/config.yaml snippet"
fi

if [ "$failures" -ne 0 ]; then
    printf 'spec-toolchain-conformance: %d mismatch(es)\n' "$failures" >&2
    exit 1
fi
echo "spec-toolchain-conformance: PASS (openspec $os_version, spec-kit $sk_version, $maps intent forms checked)"
