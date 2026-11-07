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
# excludes the version file(s) themselves and README.md (AHI-954: every package, those with a
# `files` list included). The shipped set is named by the package directory with those files
# excluded, not by a file list, so a shipped file added later (a new skill, say) is covered without editing
# this script. The gate fails when a version file's last field-change commit is older
# than the shipped set's last content-change commit.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
# Without jq every version reads as absent, which reports a misleading stale bump.
command -v jq >/dev/null 2>&1 || { printf 'host package versions: jq is required\n' >&2; exit 1; }

# yaml_field PARENT CHILD: the scalar value of the indented CHILD key in the first top-level
# PARENT: block on stdin, unquoted; exits 1 when absent. Spec Kit's extension.yml and the
# OpenSpec skill's front matter carry their versions this way (V1-2184).
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
        END { if (!found) exit 1 }'
}

# git_field_at REV PATH FILTER: the field value at REV, or the sentinel "\x01ABSENT" when
# the path does not exist at REV (git show exits non-zero) so "absent" can never collide with
# a real (possibly empty-string) field value. FILTER is a jq filter, or yaml:PARENT.CHILD.
git_field_at() {
    rev=$1 path=$2 filter=$3
    content=$(git show "$rev:$path" 2>/dev/null) || { printf '\001ABSENT'; return 0; }
    case $filter in
        yaml:*)
            key=${filter#yaml:}
            printf '%s\n' "$content" | yaml_field "${key%%.*}" "${key#*.}" 2>/dev/null || printf '\001ABSENT'
            ;;
        *) printf '%s' "$content" | jq -r "$filter" 2>/dev/null || printf '\001ABSENT' ;;
    esac
}

# field_of_file FILE FILTER: git_field_at's value for a blob the batch read wrote to FILE, or the
# ABSENT sentinel when FILE.missing marks an object the batch could not find (no first parent, or no
# PATH at that commit), exactly as git_field_at reads a failed `git show`.
field_of_file() {
    file=$1 filter=$2
    [ ! -e "$file.missing" ] || { printf '\001ABSENT'; return 0; }
    content=$(cat "$file") || { printf '\001ABSENT'; return 0; }
    case $filter in
        yaml:*)
            key=${filter#yaml:}
            printf '%s\n' "$content" | yaml_field "${key%%.*}" "${key#*.}" 2>/dev/null || printf '\001ABSENT'
            ;;
        *) printf '%s' "$content" | jq -r "$filter" 2>/dev/null || printf '\001ABSENT' ;;
    esac
}

# batch_blobs DIR: reads `git cat-file --batch` output on stdin and writes the Nth object (from 0)
# to DIR/N, or creates DIR/N.missing for a missing one; exits non-zero unless it read COUNT objects.
batch_blobs() {
    perl -e '
        my ($dir, $count) = @ARGV;
        binmode STDIN;
        my $i = 0;
        while (defined(my $header = <STDIN>)) {
            chomp $header;
            my $out = "$dir/$i";
            if ($header =~ / (missing|ambiguous)$/) { $out .= ".missing"; open(my $f, ">", $out) or exit 3; close $f; $i++; next }
            my (undef, undef, $size) = split / /, $header;
            defined $size && $size =~ /^[0-9]+$/ or exit 3;
            my $got = read(STDIN, my $buf, $size);
            defined $got && $got == $size or exit 3;
            defined(my $nl = <STDIN>) or exit 3;
            open(my $f, ">", $out) or exit 3; binmode $f; print {$f} $buf; close $f or exit 3;
            $i++;
        }
        exit($i == $count ? 0 : 3);' "$1" "$2"
}

# last_field_change_commit PATH JQFILTER: the hash of the newest commit at which PATH's
# JQFILTER value differs from its value at that commit's first parent. Exits 1 with a message
# on stderr if PATH is not tracked at HEAD. A commit hash, not a timestamp, is what lets the
# caller decide "last bumped at or after the shipped set's last change" by ancestry rather than
# by committer-second equality, which two distinct commits (e.g. either side of a rebase) can
# share.
# LVT-V0-005 (V1-4252): one `git log` lists the touching commits and one `git cat-file --batch`
# reads PATH at each of them and at each first parent, instead of three git spawns per commit; a
# batch that cannot be read or parsed falls back to the per-commit reads, so the verdict is
# unchanged either way.
last_field_change_commit() {
    path=$1 filter=$2
    # LVT-V0-005: each path keeps its exact original history traversal; all paths share
    # one blob batch. The two Pi fields reuse the same history and immutable bytes.
    if [ -n "${histories:-}" ] && [ -f "$histories/ready" ]; then
        commits=$(cat "$histories/log/$path")
        if [ -n "$commits" ]; then
            i=$(cat "$histories/offset/$path")
            for commit in $commits; do
                here=$(field_of_file "$histories/$i" "$filter")
                before=$(field_of_file "$histories/$((i + 1))" "$filter")
                i=$((i + 2))
                if [ "$here" != "$before" ]; then printf '%s' "$commit"; return 0; fi
            done
            printf '%s' "$(echo "$commits" | tail -n1)"
            return 0
        fi
    fi
    commits=$(git log --format=%H -- "$path")
    if [ -z "$commits" ]; then
        printf 'host package versions: %s is not tracked in history\n' "$path" >&2
        exit 1
    fi
    blobs=$(mktemp -d "${TMPDIR:-/tmp}/corvint-host-package-versions.XXXXXX") || blobs=
    count=0
    if [ -n "$blobs" ]; then
        for commit in $commits; do
            printf '%s:%s\n%s^:%s\n' "$commit" "$path" "$commit" "$path"
            count=$((count + 2))
        done > "$blobs/request"
        if git cat-file --batch < "$blobs/request" 2>/dev/null | batch_blobs "$blobs" "$count"; then
            i=0
            for commit in $commits; do
                here=$(field_of_file "$blobs/$i" "$filter")
                before=$(field_of_file "$blobs/$((i + 1))" "$filter")
                i=$((i + 2))
                if [ "$here" != "$before" ]; then
                    rm -rf "$blobs"
                    printf '%s' "$commit"
                    return 0
                fi
            done
            rm -rf "$blobs"
            printf '%s' "$(echo "$commits" | tail -n1)"
            return 0
        fi
        rm -rf "$blobs"
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

# A conservative existence filter over every historical path. It only avoids a log for a
# package that has never shipped any matching path (deletions remain in this inventory).
# Unsupported pathspec syntax falls back to the complete per-package log.
history_has_paths() {
    perl -e '
        my $file = shift;
        my (@include, @exclude);
        for my $spec (@ARGV) {
            my $exclude = $spec =~ s/^:\(exclude\)//;
            exit 0 if $spec =~ /[:*?\[\]\\]/;
            push @{ $exclude ? \@exclude : \@include }, $spec;
        }
        open my $fh, "<", $file or exit 0;
        local $/ = "\0";
        while (my $path = <$fh>) {
            chomp $path;
            next unless grep { $path eq $_ || index($path, "$_/") == 0 } @include;
            next if grep { $path eq $_ || index($path, "$_/") == 0 } @exclude;
            exit 0;
        }
        exit 1; ' "$histories/shipped-paths" "$@"
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
    if [ "${collect_histories:-0}" = 1 ]; then
        shipped_commit=
        if history_has_paths "$@"; then shipped_commit=$(last_content_change_commit "$@"); fi
        printf '%s' "$shipped_commit" > "$histories/shipped/$name"
        if [ -n "$shipped_commit" ]; then
            for field in $fields; do printf '%s\n' "${field%%|*}"; done
        fi
        return 0
    fi
    if [ -n "${histories:-}" ] && [ -f "$histories/ready" ]; then
        shipped_commit=$(cat "$histories/shipped/$name")
    else
        shipped_commit=$(last_content_change_commit "$@")
    fi
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
            printf 'host package versions: %s: %s last bumped at %s, older than shipped content changed at %s; run make host-package-version-bump PKG=%s\n' \
                "$name" "$path" "$bump_commit" "$shipped_commit" "$name" >&2
            status=1
        fi
    done
}

packages() {
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

# opencode declares its own published `files` list, so that list defines its shipped set. Its
# README.md stays out, as for every package (AHI-954): a version-cached README changes no behaviour.
check_package opencode \
    'integrations/opencode/package.json|.version' \
    -- \
    integrations/opencode/src \
    integrations/opencode/opencode.example.json \
    integrations/opencode/mcp.example.json \
    integrations/opencode/skills


check_package pi \
    'integrations/pi/package.json|.version' \
    'integrations/pi/package.json|.corvintIntegration.adapterVersion' \
    -- \
    integrations/pi/index.ts integrations/pi/extension.js integrations/pi/tools.js integrations/pi/runtime.js integrations/pi/compatibility.json integrations/pi/process.js integrations/pi/core.js integrations/pi/tasks.js integrations/pi/operations.js integrations/pi/cockpit.js integrations/pi/workflow.js

# AHI-020 / V1-2184: the Spec Kit extension and the OpenSpec skill are cached by version too. The
# OpenSpec skill file is both its version file and its main shipped content, so it stays in its
# own shipped set: a body edit must land with a bump.
check_package speckit \
    'protocol/integrations/speckit/corvint/extension.yml|yaml:extension.version' \
    -- \
    protocol/integrations/speckit/corvint \
    ':(exclude)protocol/integrations/speckit/corvint/extension.yml' \
    ':(exclude)protocol/integrations/speckit/corvint/README.md'

check_package openspec \
    'protocol/integrations/openspec/skills/corvint-openspec/SKILL.md|yaml:metadata.version' \
    -- \
    protocol/integrations/openspec \
    ':(exclude)protocol/integrations/openspec/README.md'

}

# Preparing this private cache never narrows the check. Any failed allocation, enumeration or
# batch parse leaves it unready and last_field_change_commit runs its original complete walk.
histories=$(mktemp -d "${TMPDIR:-/tmp}/corvint-host-histories.XXXXXX") || histories=
if [ -n "$histories" ]; then
    trap 'rm -rf "$histories"' EXIT
    trap 'exit 129' HUP
    trap 'exit 130' INT
    trap 'exit 143' TERM
    prepare_histories() {
        mkdir -p "$histories/shipped" || return 1
        git log --format= --name-only -z -- integrations protocol/integrations > "$histories/shipped-paths" || return 1
        collect_histories=1 packages | LC_ALL=C sort -u > "$histories/paths" || return 1
        count=0
        : > "$histories/request"
        while IFS= read -r manifest; do
            mkdir -p "$histories/log/$(dirname "$manifest")" "$histories/offset/$(dirname "$manifest")" || return 1
            git log --format=%H -- "$manifest" > "$histories/log/$manifest" || return 1
            printf '%s\n' "$count" > "$histories/offset/$manifest" || return 1
            for commit in $(cat "$histories/log/$manifest"); do
                printf '%s:%s\n%s^:%s\n' "$commit" "$manifest" "$commit" "$manifest" >> "$histories/request" || return 1
                count=$((count + 2))
            done
        done < "$histories/paths"
        git cat-file --batch < "$histories/request" 2>/dev/null | batch_blobs "$histories" "$count" || return 1
        : > "$histories/ready"
    }
    prepare_histories || true
fi
packages

exit $status
