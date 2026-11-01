#!/bin/sh
set -eu

# Analyzer schema maintenance (V1-2130, V1-1732, V1-2289, V1-3574; docs/specs/index-snapshot-v0.md
# IDX-SNAP-V0-060, IDX-SNAP-V0-061, IDX-SNAP-V0-107 and IDX-SNAP-V0-955).
#
# internal/contextindex/analyzer_schema.go names the analyzer schema (corvint-analyzer/N).
# TestAnalyzerSchemaInputs pins N (auditedSchema) and logs, without pinning, the digest of the
# audited input files; TestAnalyzerSchemaOutput pins auditedOutputSHA256, the digest of the pack a
# fixed corpus compiles to. The output pin is the one content pin (IDX-SNAP-V0-955), so an input
# edit that leaves the golden pack unchanged rewrites no pin line, and two branches that each make
# such an edit cannot conflict on one.
#
#   script/analyzer-schema.sh bump  [--base REF]   run TestAnalyzerSchemaOutput; when its digest
#                                                  differs from the pin or the number collides with
#                                                  REF's, set N to REF's N when the output equals
#                                                  REF's, else to REF's N + 1 (or keep this branch's
#                                                  N when it is already above REF's), rewrite the
#                                                  number and output pins and repin stale use-case
#                                                  receipts. One bump per branch.
#   script/analyzer-schema.sh check [--base REF]   name, on stderr and without failing, the audited
#                                                  inputs changed since the merge base with REF;
#                                                  refuse a branch whose output pin differs from
#                                                  REF's while its number is not above REF's.
#
# The Go test stays the authority that the pinned output digest is the real one.
# CORVINT_ANALYZER_OUTPUT_COMMAND replaces the command that prints "analyzer output SHA256=<hex>".
# REF defaults to origin/main. When REF does not resolve, `check` reports the base comparison
# NOT_RUN and `bump` refuses, so a bump never guesses its number.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

schema_go=internal/contextindex/analyzer_schema.go
schema_test=internal/contextindex/analyzer_schema_test.go

usage() { printf 'usage: %s bump|check [--base REF]\n' "$0" >&2; exit 64; }

[ $# -ge 1 ] || usage
mode=$1
shift
case $mode in bump | check) ;; *) usage ;; esac
base=origin/main
while [ $# -gt 0 ]; do
    case $1 in
    --base)
        [ $# -ge 2 ] || usage
        base=$2
        shift 2
        ;;
    *) usage ;;
    esac
done

# pins prints "N OUTPUT" for the schema number in analyzer_schema.go and the output digest in the
# test, read from a tree-ish (git show) or, with "-", from the worktree. It prints nothing when
# either is absent, or when the worktree's two number pins disagree, so a caller can tell an
# unreadable base from a real pin.
pins() {
    if [ "$1" = - ]; then
        number=$(sed -n 's/^const analyzerSchemaID = "corvint-analyzer\/\([0-9][0-9]*\)"$/\1/p' "$schema_go")
        test_source=$(cat "$schema_test")
        tested=$(printf '%s\n' "$test_source" | sed -n 's/^[[:space:]]*const auditedSchema = "corvint-analyzer\/\([0-9][0-9]*\)"$/\1/p')
    else
        number=$(git show "$1:$schema_go" 2>/dev/null | sed -n 's/^const analyzerSchemaID = "corvint-analyzer\/\([0-9][0-9]*\)"$/\1/p')
        test_source=$(git show "$1:$schema_test" 2>/dev/null) || test_source=
        tested=$number
    fi
    output=$(printf '%s\n' "$test_source" | sed -n 's/^[[:space:]]*const auditedOutputSHA256 = "\([0-9a-f]\{64\}\)"$/\1/p')
    if [ -n "$number" ] && [ -n "$output" ] && [ "$number" = "$tested" ]; then
        printf '%s %s\n' "$number" "$output"
    fi
}

# field I LINE prints the Ith space-separated field of a pins line.
field() { printf '%s\n' "$2" | cut -d' ' -f"$1"; }

# actual_output runs TestAnalyzerSchemaOutput and prints the output digest it reports.
actual_output() {
    command=${CORVINT_ANALYZER_OUTPUT_COMMAND:-"GOTOOLCHAIN=local go test -count=1 -run '^TestAnalyzerSchemaOutput\$' -v ./internal/contextindex/"}
    report=$(sh -c "$command" 2>&1) || true
    found=$(printf '%s\n' "$report" | sed -n 's/.*analyzer output SHA256=\([0-9a-f]\{64\}\).*/\1/p' | head -n 1)
    [ -n "$found" ] || {
        printf 'analyzer schema: TestAnalyzerSchemaOutput reported no output digest:\n%s\n' "$report" >&2
        exit 1
    }
    printf '%s\n' "$found"
}

# changed_inputs REV prints, comma-separated, the audited inputs (go.mod, go.sum and the non-test
# .go files directly under the internal/ directories TestAnalyzerSchemaInputs lists) that differ
# between REV and the worktree.
changed_inputs() {
    rev=$1
    set -- go.mod go.sum
    for directory in $(sed -n 's/.*for _, directory := range \[\]string{\([^}]*\)}.*/\1/p' "$schema_test" | tr ',' '\n' | tr -d '" '); do
        set -- "$@" ":(glob)internal/$directory/*.go"
    done
    git diff --name-only "$rev" -- "$@" 2>/dev/null | grep -v '_test\.go$' | paste -sd, - || true
}

current=$(pins -)
[ -n "$current" ] || {
    printf 'analyzer schema: cannot read matching pins from %s and %s\n' "$schema_go" "$schema_test" >&2
    exit 1
}
number=$(field 1 "$current")
pinned_output=$(field 2 "$current")

base_pins=
if git rev-parse --verify --quiet "$base^{commit}" >/dev/null; then
    base_pins=$(pins "$base")
fi
merge_base=
if [ -n "$base_pins" ]; then
    merge_base=$(git merge-base HEAD "$base" 2>/dev/null) || merge_base=
fi

if [ "$mode" = check ]; then
    if [ -z "$base_pins" ]; then
        printf 'analyzer schema: base %s does not resolve; base collision check NOT_RUN\n' "$base" >&2
        exit 0
    fi
    [ -n "$merge_base" ] || {
        printf 'analyzer schema: HEAD has no merge base with %s; base collision check NOT_RUN\n' "$base" >&2
        exit 0
    }
    changed=$(changed_inputs "$merge_base")
    if [ -n "$changed" ]; then
        printf 'analyzer schema: analyzer inputs changed since %s: %s; run TestAnalyzerSchemaOutput, and script/analyzer-schema.sh bump only if it fails (IDX-SNAP-V0-955)\n' "$merge_base" "$changed" >&2
    fi
    if [ "$current" = "$(pins "$merge_base")" ] || [ "$current" = "$base_pins" ]; then
        exit 0
    fi
    base_number=$(field 1 "$base_pins")
    base_output=$(field 2 "$base_pins")
    if [ "$number" -gt "$base_number" ]; then
        exit 0
    fi
    if [ "$number" -eq "$base_number" ] && [ "$pinned_output" = "$base_output" ]; then
        printf 'analyzer schema: corvint-analyzer/%s keeps the audited output digest of %s; no bump needed (IDX-SNAP-V0-107)\n' "$number" "$base"
        exit 0
    fi
    printf 'analyzer schema: this branch changed the analyzer output pin but keeps corvint-analyzer/%s, while %s is at corvint-analyzer/%s with output %s; a reused number would let one pack name two analyzer contents. Rebump to corvint-analyzer/%s with script/analyzer-schema.sh bump\n' \
        "$number" "$base" "$base_number" "$(field 2 "$base_pins")" "$((base_number + 1))" >&2
    exit 1
fi

[ -n "$base_pins" ] || {
    printf 'analyzer schema: base %s does not resolve to readable pins; pass --base REF\n' "$base" >&2
    exit 1
}
base_number=$(field 1 "$base_pins")
base_output=$(field 2 "$base_pins")
collides=false
if [ "$current" != "$base_pins" ] && [ "$number" -le "$base_number" ]; then
    if [ -z "$merge_base" ] || [ "$current" != "$(pins "$merge_base")" ]; then
        collides=true
    fi
fi
output=$(actual_output)
if [ "$output" = "$pinned_output" ] && [ "$collides" = false ]; then
    printf 'analyzer schema: output matches corvint-analyzer/%s; nothing to bump\n' "$number"
else
    target=$number
    if [ "$output" = "$base_output" ]; then
        # The corpus compiles to base's pack bytes, so base's number still names these facts.
        target=$base_number
    elif [ "$number" -le "$base_number" ]; then
        target=$((base_number + 1))
    fi
    perl -0pi -e "s/^const analyzerSchemaID = \"corvint-analyzer\\/$number\"\$/const analyzerSchemaID = \"corvint-analyzer\\/$target\"/m or die" "$schema_go"
    perl -0pi -e "s/^(\\s*const auditedSchema = )\"corvint-analyzer\\/$number\"\$/\${1}\"corvint-analyzer\\/$target\"/m or die; s/^(\\s*const auditedOutputSHA256 = )\"$pinned_output\"\$/\${1}\"$output\"/m or die" "$schema_test"
    printf 'analyzer schema: corvint-analyzer/%s -> corvint-analyzer/%s, output %s\n' "$number" "$target" "$output"
fi
script/repin-use-case-receipts.sh
