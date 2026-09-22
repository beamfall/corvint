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
trap 'rm -f "$listing"' EXIT

if ! git ls-files -z -- docs/decisions >"$listing"; then
    printf 'decision numbers: git ls-files failed to enumerate docs/decisions\n' >&2
    exit 1
fi

tr '\0' '\n' <"$listing" |
    grep -E '^docs/decisions/[0-9]{4}-[^/]*\.md$' |
    LC_ALL=C sort |
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
' >&2
