#!/bin/sh
set -eu

# DCG-V0-801 (V1-3384): one command for every mechanical repin an output-neutral source edit needs.
# It runs the analyzer-schema bump (IDX-SNAP-V0-107: an input edit whose audited output is
# unchanged repins only the digests), stages the two pin files when the bump rewrote them, since
# the citation gate reads the Git index, and then runs the line-citation --repin, which rewrites
# and stages every citation whose cited text moved unchanged (DCG-V0-048). Anything left is a
# citation whose cited text changed, which a person must read; the final check names it.
#
#   script/repin.sh [--base REF]    (make repin [BASE=REF]; REF defaults to origin/main)

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

pins="internal/contextindex/analyzer_schema.go internal/contextindex/analyzer_schema_test.go"
# shellcheck disable=SC2086
before=$(git hash-object -- $pins)
script/analyzer-schema.sh bump "$@"
# shellcheck disable=SC2086
after=$(git hash-object -- $pins)
if [ "$before" != "$after" ]; then
    # shellcheck disable=SC2086
    git add -- $pins
    printf 'repin: staged the analyzer-schema pins the bump rewrote\n'
fi
script/check-line-citations.sh --repin
