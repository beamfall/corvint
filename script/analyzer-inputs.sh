#!/bin/sh
set -eu

# usage: script/analyzer-inputs.sh REV
# Prints the audited analyzer inputs that differ between REV and the worktree (IDX-SNAP-V0-890,
# IDX-SNAP-V0-891): go.mod, go.sum and the non-test .go files directly under the internal/
# directories TestAnalyzerSchemaInputs lists, read from that test so the two cannot disagree on the
# set. script/analyzer-schema.sh check notes them and script/dogfood-change.sh names them at change
# start; this script runs no test.

[ $# -eq 1 ] || { printf 'usage: %s REV\n' "$0" >&2; exit 64; }
rev=$1
root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
schema_test=internal/contextindex/analyzer_schema_test.go
directories=$(sed -n 's/.*for _, directory := range \[\]string{\([^}]*\)}.*/\1/p' "$schema_test" | tr ',' '\n' | tr -d '" \t')
[ -n "$directories" ] || { printf 'analyzer schema: cannot find the audited directory list in %s\n' "$schema_test" >&2; exit 1; }
set -- go.mod go.sum
for directory in $directories; do set -- "$@" "internal/$directory"; done
git diff --name-only "$rev" -- "$@" 2>/dev/null | grep -E '^go\.(mod|sum)$|^internal/[^/]+/[^/]+\.go$' | grep -v '_test\.go$' || true
