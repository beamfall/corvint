#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

# Decision 0048 grandfathers exactly this pair: both full filenames are cited from frozen evidence.
grandfathered='docs/decisions/0016-change-anchored-packet-2026-09-01.md docs/decisions/0016-packet-5-current-pin-requalification-2026-08-31.md'

# Enumerate tracked paths, not the worktree: `make gate` must measure the set a fresh clone
# contains (Makefile:14). A worktree-only file would otherwise report a collision no commit
# carries, and deleting one member of a real collision from the worktree alone would hide it.
#
# The enumeration is captured to a file and checked on its own, rather than piped straight
# into the rest of the pipeline: plain `sh` has no `pipefail`, so a pipeline's exit status is
# its last command's, and a `git ls-files` failure would otherwise be swallowed and reported
# as zero collisions found.
listing=$(mktemp "${TMPDIR:-/tmp}/corvint-decision-numbers.XXXXXX")
filtered=$(mktemp "${TMPDIR:-/tmp}/corvint-decision-numbers-filtered.XXXXXX")
trap 'rm -f "$listing" "$filtered"' EXIT

if ! git ls-files -z -- docs/decisions >"$listing"; then
    printf 'decision numbers: git ls-files failed to enumerate docs/decisions\n' >&2
    exit 1
fi

tr '\0' '\n' <"$listing" |
    grep -E '^docs/decisions/[0-9]{4}-[^/]*\.md$' |
    LC_ALL=C sort >"$filtered"

awk -v grandfathered="$grandfathered" '
{
    path = $0
    name = path
    sub(/^.*\//, "", name)
    number = substr(name, 1, 4)
    count[number]++
    files[number, count[number]] = path
    if (count[number] == 1) {
        order[++order_count] = number
    }
}
END {
    status = 0
    for (i = 1; i <= order_count; i++) {
        number = order[i]
        if (count[number] < 2) {
            continue
        }
        joined = files[number, 1]
        for (j = 2; j <= count[number]; j++) {
            joined = joined " " files[number, j]
        }
        if (joined == grandfathered) {
            continue
        }
        if (status == 0) {
            print "decision number collisions:"
        }
        print number ":"
        for (j = 1; j <= count[number]; j++) {
            print "  " files[number, j]
        }
        status = 1
    }
    exit status
}
' <"$filtered" >&2

# docs/decisions/README.md indexes every tracked decision file exactly once (V1-0265): a
# missing row leaves a decision undiscoverable from the index, and a duplicate row means one
# file's row was probably meant for another file.
readme=docs/decisions/README.md
indexed=$(mktemp "${TMPDIR:-/tmp}/corvint-decision-numbers-indexed.XXXXXX")
trap 'rm -f "$listing" "$filtered" "$indexed"' EXIT

grep -oE '^\| \[`[0-9]{4}-[^`]+\.md`\]' "$readme" |
    sed -E 's/^\| \[`//; s/`\]$//' |
    LC_ALL=C sort >"$indexed"

dupes=$(LC_ALL=C uniq -d "$indexed")
if [ -n "$dupes" ]; then
    printf 'decision index duplicate rows in %s:\n' "$readme" >&2
    printf '%s\n' "$dupes" | sed 's/^/  /' >&2
    exit 1
fi

missing=$(sed -E 's#^docs/decisions/##' "$filtered" | LC_ALL=C sort | LC_ALL=C comm -23 - "$indexed")
if [ -n "$missing" ]; then
    printf 'decision index missing rows in %s:\n' "$readme" >&2
    printf '%s\n' "$missing" | sed 's/^/  /' >&2
    exit 1
fi
