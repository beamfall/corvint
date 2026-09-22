#!/bin/sh
set -eu

# AHI-020 (docs/specs/agent-harness-integration-v0.md): each host package's own manifest
# version MUST advance whenever that package's shipped content changes since the version was
# last bumped. A host that caches an installed package by version never re-stages changed
# content under an unchanged version (docs/build-log/2026-09-10.md); the Claude Code plugin
# shipped a Python-to-native rewrite under an unbumped 0.1.4 before this gate existed.
#
# Everything here is read from committed history with `git show <rev>:<path>` and
# `git log -- <path>`, never from the worktree: a gate must measure what a fresh clone of HEAD
# contains, not a dirty tree.
#
# For each package below, "the commit that last changed the version field" is the newest
# commit (by committer date) touching a version file whose field value differs from the value
# at that commit's first parent (a file's first commit always counts, since the field goes
# from absent to present). "The last commit that changed any other shipped file" is the newest
# commit (by committer date) touching any tracked path in the package's shipped set, which
# excludes the version file(s) themselves and, unless a package declares its own `files` list,
# excludes README.md. The shipped set is named by the package directory with those files excluded,
# not by a file list, so a shipped file added later (a new skill, say) is covered without editing
# this script. The gate fails when a version file's last field-change commit is older
# than the shipped set's last content-change commit.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

# git_field_at REV PATH JQFILTER: the field value at REV, or the sentinel "\x01ABSENT" when
# the path does not exist at REV (git show exits non-zero) so "absent" can never collide with
# a real (possibly empty-string) field value.
git_field_at() {
    rev=$1 path=$2 filter=$3
    content=$(git show "$rev:$path" 2>/dev/null) || { printf '\001ABSENT'; return 0; }
    printf '%s' "$content" | jq -r "$filter" 2>/dev/null || printf '\001ABSENT'
}

# last_field_change_commit PATH JQFILTER: the hash of the newest commit at which PATH's
# JQFILTER value differs from its value at that commit's first parent. Exits 1 with a message
# on stderr if PATH is not tracked at HEAD. A commit hash, not a timestamp, is what lets the
# caller decide "last bumped at or after the shipped set's last change" by ancestry rather than
# by committer-second equality, which two distinct commits (e.g. either side of a rebase) can
# share.
last_field_change_commit() {
    path=$1 filter=$2
    commits=$(git log --format=%H -- "$path")
    if [ -z "$commits" ]; then
        printf 'host package versions: %s is not tracked in history\n' "$path" >&2
        exit 1
    fi
    for commit in $commits; do
        here=$(git_field_at "$commit" "$path" "$filter")
        if git rev-parse -q --verify "$commit^" >/dev/null 2>&1; then
            before=$(git_field_at "$commit^" "$path" "$filter")
        else
            before=$(printf '\001ABSENT') # root commit: nothing came before it
        fi
        if [ "$here" != "$before" ]; then
            printf '%s' "$commit"
            return 0
        fi
    done
    # No touching commit ever changed the field's value (should not happen: the oldest
    # touching commit always introduces the path, which counts as a change from absent).
    printf '%s' "$(echo "$commits" | tail -n1)"
}

# last_content_change_commit: the hash of the newest commit touching any of the given
# pathspecs, or empty if none of them are tracked.
last_content_change_commit() {
    git log -1 --format=%H -- "$@" 2>/dev/null || true
}

status=0

# check_package NAME VERSION_FIELDS_FILE... -- SHIPPED_PATHS...
# VERSION_FIELDS entries are "path|jqfilter" pairs, one per manifest that must independently
# track the shipped set.
check_package() {
    name=$1
    shift
    fields=""
    while [ "$1" != "--" ]; do
        fields="$fields $1"
        shift
    done
    shift # consume --
    shipped_commit=$(last_content_change_commit "$@")
    if [ -z "$shipped_commit" ]; then
        return 0 # no shipped files tracked (should not happen for a real package)
    fi
    for field in $fields; do
        path=${field%%|*}
        filter=${field#*|}
        bump_commit=$(last_field_change_commit "$path" "$filter")
        # The version file is current only when its own last bump is the shipped set's last
        # change (both landed in one commit) or a descendant of it (bumped afterward). A
        # committer-second timestamp comparison cannot tell those cases apart from two distinct
        # commits sharing a second (a rebase can produce this), so ancestry decides it instead.
        if [ "$bump_commit" != "$shipped_commit" ] && ! git merge-base --is-ancestor "$shipped_commit" "$bump_commit" 2>/dev/null; then
            printf 'host package versions: %s: %s last bumped at %s, older than shipped content changed at %s\n' \
                "$name" "$path" "$bump_commit" "$shipped_commit" >&2
            status=1
        fi
    done
}

check_package claude-code \
    'integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json|.version' \
    'integrations/claude-code/.claude-plugin/marketplace.json|.plugins[]|select(.name=="corvint")|.version' \
    -- \
    integrations/claude-code/plugins/corvint \
    ':(exclude)integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json' \
    ':(exclude)integrations/claude-code/plugins/corvint/README.md'

check_package codex \
    'integrations/codex/plugins/corvint/.codex-plugin/plugin.json|.version' \
    -- \
    integrations/codex/plugins/corvint \
    ':(exclude)integrations/codex/plugins/corvint/.codex-plugin/plugin.json' \
    ':(exclude)integrations/codex/plugins/corvint/README.md'

check_package gemini-cli \
    'integrations/gemini-cli/gemini-extension.json|.version' \
    -- \
    integrations/gemini-cli \
    ':(exclude)integrations/gemini-cli/gemini-extension.json' \
    ':(exclude)integrations/gemini-cli/README.md'

# opencode declares its own published `files` list, README.md included, so that list (not the
# README exclusion above) defines its shipped set.
check_package opencode \
    'integrations/opencode/package.json|.version' \
    -- \
    integrations/opencode/src \
    integrations/opencode/README.md \
    integrations/opencode/opencode.example.json


check_package pi \
    'integrations/pi/package.json|.version' \
    'integrations/pi/package.json|.corvintIntegration.adapterVersion' \
    -- \
    integrations/pi/index.ts integrations/pi/runtime.js integrations/pi/README.md integrations/pi/compatibility.json

exit $status
