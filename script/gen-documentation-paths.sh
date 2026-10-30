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

# generate OUT: write the declaration to OUT. Each stage writes a file and is checked, since
# /bin/sh has no pipefail: a failed listing must refuse, never write a short declaration.
generate() {
    work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-documentation-paths.XXXXXX") || exit 1
    # -z keeps a name with a quote or non-ASCII byte raw (git C-quotes it otherwise). A name the
    # AFP-V0-160 entry grammar (ValidRelativePath) refuses, one holding a backslash, a newline or
    # bytes that are not UTF-8, refuses here too, so --check never passes a declaration the
    # planner would reject.
    git ls-files -z -- 'docs/*.md' >"$work/listed" ||
        { rm -rf "$work"; printf 'documentation paths: git ls-files failed\n' >&2; exit 1; }
    perl -0ne 'chomp; next unless m{\Adocs/[^/]+\.md\z}s;
        die "documentation paths: a docs/*.md name holds a newline\n" if /\n/;
        die "documentation paths: $_ holds a backslash, which a declaration entry cannot (AFP-V0-160); rename it\n" if /\\/;
        my $copy = $_; die "documentation paths: a docs/*.md name is not UTF-8; rename it\n" unless utf8::decode($copy);
        print "$_\n"' \
        "$work/listed" >"$work/paths" || { rm -rf "$work"; exit 1; }
    printf '%s\n' "$DIRS" >>"$work/paths"
    LC_ALL=C sort -u "$work/paths" >"$work/sorted" &&
        jq -R . "$work/sorted" >"$work/strings" &&
        jq -cs '{profile: "corvint-documentation-paths/0", paths: .}' "$work/strings" >"$1" ||
        { rm -rf "$work"; printf 'documentation paths: cannot build the declaration\n' >&2; exit 1; }
    rm -rf "$work"
}

case ${1:-} in
    '')
        tmp="$declaration.tmp.$$"
        trap 'rm -f "$tmp"' EXIT
        generate "$tmp"
        mv -f "$tmp" "$declaration"
        trap - EXIT
        ;;
    --check)
        want=$(mktemp "${TMPDIR:-/tmp}/corvint-documentation-paths.XXXXXX")
        trap 'rm -f "$want"' EXIT
        generate "$want"
        if ! cmp -s "$want" "$declaration"; then
            printf 'documentation paths: %s is stale against the tracked docs/*.md files; run make documentation-paths and commit it\n' "$declaration" >&2
            # Name each declared path the tree no longer has (V1-3174).
            if [ -f "$declaration" ]; then
                jq -r '.paths[]? | strings' "$declaration" 2>/dev/null | while IFS= read -r path; do
                    [ -e "$path" ] || printf '  missing: %s\n' "$path" >&2
                done
            fi
            exit 1
        fi
        ;;
    *)
        printf 'usage: %s [--check]\n' "$0" >&2
        exit 2
        ;;
esac
