#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Offline cases for script/spec-toolchain-latest.sh (STA-V0-012), from recorded registry answers:
# both current; OpenSpec behind; a Spec Kit prerelease or draft answer is unreadable, not BEHIND;
# a missing answer is UNAVAILABLE and exits 0; BEHIND alone exits 1; a missing, empty, unparsable or
# incomplete manifest prints no verdict and exits 2.

set -euo pipefail

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
script=$source_root/script/spec-toolchain-latest.sh
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-spec-toolchain-latest-test.XXXXXX")
cleanup() { rm -rf -- "$test_root"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

check() { # check LABEL WANT_EXIT WANT_OUTPUT OPENSPEC_JSON SPECKIT_JSON ("" = no answer)
    local dir=$test_root/$1 output status
    mkdir -p "$dir"
    [ -z "$4" ] || printf '%s\n' "$4" >"$dir/openspec.json"
    [ -z "$5" ] || printf '%s\n' "$5" >"$dir/spec-kit.json"
    status=0
    output=$(SPEC_TOOLCHAIN_LATEST_RESPONSES=$dir "$script") || status=$?
    [ "$status" -eq "$2" ] || fail "$1: exit $status, want $2; output: $output"
    [ "$output" = "$3" ] || fail "$1: output
$output
want
$3"
}

pinned_os=$(jq -r '.toolchains[] | select(.name == "openspec") | .version' "$source_root/interop/spec-toolchains/versions.json")
pinned_sk=$(jq -r '.toolchains[] | select(.name == "spec-kit") | .version' "$source_root/interop/spec-toolchains/versions.json")
npm_answer() { printf '{"latest":"%s","next":"0.3.0"}' "$1"; }
gh_answer() { printf '{"tag_name":"v%s","draft":%s,"prerelease":%s}' "$1" "${2:-false}" "${3:-false}"; }

check current 0 "openspec $pinned_os CURRENT
spec-kit $pinned_sk CURRENT" "$(npm_answer "$pinned_os")" "$(gh_answer "$pinned_sk")"

check openspec-behind 1 "openspec $pinned_os BEHIND 9.0.0
spec-kit $pinned_sk CURRENT" "$(npm_answer 9.0.0)" "$(gh_answer "$pinned_sk")"

check speckit-behind 1 "openspec $pinned_os CURRENT
spec-kit $pinned_sk BEHIND 9.1.0" "$(npm_answer "$pinned_os")" "$(gh_answer 9.1.0)"

check prerelease-unreadable 0 "openspec $pinned_os CURRENT
spec-kit $pinned_sk UNAVAILABLE unreadable-response" "$(npm_answer "$pinned_os")" "$(gh_answer 9.1.0 false true)"

check missing-unavailable 0 "openspec $pinned_os UNAVAILABLE fetch-failed
spec-kit $pinned_sk UNAVAILABLE fetch-failed" "" ""

check garbage-unavailable 1 "openspec $pinned_os UNAVAILABLE unreadable-response
spec-kit $pinned_sk BEHIND 9.1.0" '{"message":"rate limited"}' "$(gh_answer 9.1.0)"

check_manifest() { # check_manifest LABEL [CONTENT] (no CONTENT = no manifest file): setup error, exit 2
    local dir=$test_root/manifest-$1 output status
    mkdir -p "$dir/answers"
    npm_answer "$pinned_os" >"$dir/answers/openspec.json"
    gh_answer "$pinned_sk" >"$dir/answers/spec-kit.json"
    [ "$#" -lt 2 ] || printf '%s' "$2" >"$dir/versions.json"
    status=0
    output=$(SPEC_TOOLCHAIN_MANIFEST=$dir/versions.json SPEC_TOOLCHAIN_LATEST_RESPONSES=$dir/answers "$script" 2>/dev/null) || status=$?
    [ "$status" -eq 2 ] || fail "manifest-$1: exit $status, want 2; output: $output"
    [ -z "$output" ] || fail "manifest-$1: printed verdicts for a broken manifest: $output"
}

check_manifest missing
check_manifest empty ''
check_manifest unparsable '{"toolchains": ['
check_manifest no-toolchains '{"toolchains": []}'
check_manifest incomplete-entry '{"toolchains": [{"name": "openspec", "source": {"kind": "npm", "package": "@fission-ai/openspec"}}]}'

echo "spec-toolchain-latest: 11 offline cases passed"
