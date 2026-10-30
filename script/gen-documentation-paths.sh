#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu

# ASD-V0-105 (V1-3174): generate, or check, .corvint/documentation-paths.json, the AFP-V0-160
# declaration of this repository's documentation paths, so it is never edited by hand.
#
#   script/gen-documentation-paths.sh          rewrite the declaration
#   script/gen-documentation-paths.sh --check  exit 1, naming the fix, when it is stale
#
# The declaration is every top-level docs/*.md file the Git index tracks, plus the documentation
# directories in DIRS, sorted in byte order, as one compact corvint-documentation-paths/0 line.
# Other docs/ subdirectories (agent-memory, experiments, plans, post-merge-workflow) hold notes or
# redirects, not product documentation, and stay out until DIRS names them. A new top-level guide
# is declared once it is staged: run `make documentation-paths` after `git add`.

DIRS='docs/build-log/
docs/decisions/
docs/specs/'
declaration=.corvint/documentation-paths.json

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
command -v jq >/dev/null 2>&1 || { printf 'documentation paths: jq is required\n' >&2; exit 1; }

generate() {
    # -z keeps a name with a quote, backslash or non-ASCII byte raw (git C-quotes it otherwise); a
    # name holding a newline cannot be one JSON line entry here, so it refuses.
    { git ls-files -z -- 'docs/*.md' |
        perl -0ne 'chomp; next unless m{\Adocs/[^/]+\.md\z}s; die "documentation paths: a docs/*.md name holds a newline\n" if /\n/; print "$_\n"'
        printf '%s\n' "$DIRS"; } |
        LC_ALL=C sort -u |
        jq -R . | jq -cs '{profile: "corvint-documentation-paths/0", paths: .}'
}

case ${1:-} in
    '')
        tmp="$declaration.tmp.$$"
        trap 'rm -f "$tmp"' EXIT
        generate >"$tmp"
        mv -f "$tmp" "$declaration"
        trap - EXIT
        ;;
    --check)
        want=$(mktemp "${TMPDIR:-/tmp}/corvint-documentation-paths.XXXXXX")
        trap 'rm -f "$want"' EXIT
        generate >"$want"
        if ! cmp -s "$want" "$declaration"; then
            printf 'documentation paths: %s is stale against the tracked docs/*.md files; run make documentation-paths and commit it\n' "$declaration" >&2
            exit 1
        fi
        ;;
    *)
        printf 'usage: %s [--check]\n' "$0" >&2
        exit 2
        ;;
esac
