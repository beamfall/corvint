#!/bin/sh
set -eu

# Analyzer schema maintenance (V1-2130, V1-1732, V1-2289, V1-3574, V1-2976, V1-2356;
# docs/specs/index-snapshot-v0.md IDX-SNAP-V0-060, IDX-SNAP-V0-061, IDX-SNAP-V0-107 and
# IDX-SNAP-V0-890 to IDX-SNAP-V0-892).
#
# internal/contextindex/analyzer_schema.go names the analyzer schema (corvint-analyzer/N).
# TestAnalyzerSchemaOutput pins, next to that number, the digests of what a fixed golden corpus
# compiles to: auditedOutputSHA256 for the pack and auditedShardSHA256 for its blob shards. No pin
# moves on an input edit that leaves that output unchanged, so two branches that each edit an
# analyzer input without changing its output merge without touching the pins (IDX-SNAP-V0-890).
#
#   script/analyzer-schema.sh bump  [--base REF]   run the output test; when its digests differ
#                                                  from the pins, set N to REF's N when they equal
#                                                  REF's pins, else to REF's N + 1 (or keep this
#                                                  branch's N when it is already above REF's),
#                                                  rewrite the pins and repin stale use-case
#                                                  receipts. One bump per branch.
#   script/analyzer-schema.sh check [--base REF]   refuse when this branch changed the pins but
#                                                  kept a schema number that REF already uses
#                                                  with other output pins; note an analyzer input
#                                                  change, which `check --output` decides.
#   script/analyzer-schema.sh check --output [--base REF]
#                                                  also run the output test and refuse when its
#                                                  digests differ from the pins (IDX-SNAP-V0-891).
#
# The output test runs through script/heavy.sh, the host semaphore, when it is executable
# (IDX-SNAP-V0-892); CORVINT_ANALYZER_OUTPUT_COMMAND replaces the command, which must print
# "analyzer output SHA256=<hex>" and "analyzer shard SHA256=<hex>". The Go test stays the
# authority that the pinned digests are the real ones.
#
# REF defaults to origin/main. Two branches that both bump to the same N with different output can
# merge without a textual conflict on the number, so `check` compares against the base, not only
# the tree. When REF does not resolve, `check` reports the base comparison NOT_RUN and `bump`
# refuses, so a bump never guesses its number.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

schema_go=internal/contextindex/analyzer_schema.go
schema_test=internal/contextindex/analyzer_schema_test.go

usage() { printf 'usage: %s bump|check [--output] [--base REF]\n' "$0" >&2; exit 64; }

[ $# -ge 1 ] || usage
mode=$1
shift
case $mode in bump | check) ;; *) usage ;; esac
base=origin/main
run_output=false
while [ $# -gt 0 ]; do
    case $1 in
    --base)
        [ $# -ge 2 ] || usage
        base=$2
        shift 2
        ;;
    --output)
        [ "$mode" = check ] || usage
        run_output=true
        shift
        ;;
    *) usage ;;
    esac
done

# pins prints "N OUTPUT SHARD" for the schema number in analyzer_schema.go and the output and shard
# digests in the test, read from a tree-ish (git show) or, with "-", from the worktree. SHARD is
# "-" when the test pins no shard digest (a tree from before IDX-SNAP-V0-892). It prints nothing
# when N or OUTPUT is absent, or when the worktree test's auditedSchema disagrees with N, so a
# caller can tell an unreadable base from a real pin.
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
    shard=$(printf '%s\n' "$test_source" | sed -n 's/^[[:space:]]*const auditedShardSHA256 = "\([0-9a-f]\{64\}\)"$/\1/p')
    if [ -n "$number" ] && [ -n "$output" ] && [ "$number" = "$tested" ]; then
        printf '%s %s %s\n' "$number" "$output" "${shard:--}"
    fi
}

# field I LINE prints the Ith space-separated field of a pins line.
field() { printf '%s\n' "$2" | cut -d' ' -f"$1"; }

# same_output A B succeeds when the "OUTPUT SHARD" pairs A and B name the same output: equal pack
# digests and equal shard digests, where a side with no shard pin ("-") compares the pack only.
same_output() {
    [ "$(field 1 "$1")" = "$(field 1 "$2")" ] || return 1
    [ "$(field 2 "$1")" = - ] || [ "$(field 2 "$2")" = - ] || [ "$(field 2 "$1")" = "$(field 2 "$2")" ]
}

# actual_output runs TestAnalyzerSchemaOutput and prints "OUTPUT SHARD", the digests it reports.
actual_output() {
    heavy=
    [ -x script/heavy.sh ] && heavy=script/heavy.sh
    command=${CORVINT_ANALYZER_OUTPUT_COMMAND:-"$heavy env GOTOOLCHAIN=local go test -count=1 -run '^TestAnalyzerSchemaOutput\$' -v ./internal/contextindex/"}
    report=$(sh -c "$command" 2>&1) || true
    found=$(printf '%s\n' "$report" | sed -n 's/.*analyzer output SHA256=\([0-9a-f]\{64\}\).*/\1/p' | head -n 1)
    found_shard=$(printf '%s\n' "$report" | sed -n 's/.*analyzer shard SHA256=\([0-9a-f]\{64\}\).*/\1/p' | head -n 1)
    [ -n "$found" ] && [ -n "$found_shard" ] || {
        printf 'analyzer schema: TestAnalyzerSchemaOutput reported no output digest:\n%s\n' "$report" >&2
        exit 1
    }
    printf '%s %s\n' "$found" "$found_shard"
}

# changed_inputs REV prints the audited analyzer inputs that differ between REV and the worktree:
# go.mod, go.sum and the non-test .go files directly under the internal/ directories the input
# audit test lists (read from the test, so the two cannot disagree on the set).
changed_inputs() {
    rev=$1
    directories=$(sed -n 's/.*for _, directory := range \[\]string{\([^}]*\)}.*/\1/p' "$schema_test" | tr ',' '\n' | tr -d '" \t')
    [ -n "$directories" ] || { printf 'analyzer schema: cannot find the audited directory list in %s\n' "$schema_test" >&2; exit 1; }
    set -- go.mod go.sum
    for directory in $directories; do set -- "$@" "internal/$directory"; done
    git diff --name-only "$rev" -- "$@" 2>/dev/null | grep -E '^go\.(mod|sum)$|^internal/[^/]+/[^/]+\.go$' | grep -v '_test\.go$' || true
}

current=$(pins -)
[ -n "$current" ] || {
    printf 'analyzer schema: cannot read matching pins from %s and %s\n' "$schema_go" "$schema_test" >&2
    exit 1
}
number=$(field 1 "$current")
pinned_pair="$(field 2 "$current") $(field 3 "$current")"

base_pins=
if git rev-parse --verify --quiet "$base^{commit}" >/dev/null; then
    base_pins=$(pins "$base")
fi
merge_base=
[ -z "$base_pins" ] || merge_base=$(git merge-base HEAD "$base" 2>/dev/null) || merge_base=

if [ "$mode" = check ]; then
    status=0
    if [ "$run_output" = true ]; then
        actual_pair=$(actual_output)
        if [ "$actual_pair" != "$pinned_pair" ]; then
            printf 'analyzer schema: the golden corpus output drifted from its pins (pinned %s, actual %s); run script/analyzer-schema.sh bump\n' "$pinned_pair" "$actual_pair" >&2
            status=1
        fi
    fi
    if [ -z "$base_pins" ]; then
        printf 'analyzer schema: base %s does not resolve; base collision check NOT_RUN\n' "$base" >&2
        exit $status
    fi
    [ -n "$merge_base" ] || {
        printf 'analyzer schema: HEAD has no merge base with %s; base collision check NOT_RUN\n' "$base" >&2
        exit $status
    }
    inputs=$(changed_inputs "$merge_base")
    if [ -n "$inputs" ] && [ "$run_output" = false ]; then
        printf 'analyzer schema: NOTE %s analyzer input(s) changed since %s, first %s: TestAnalyzerSchemaOutput or script/analyzer-schema.sh check --output decides whether a bump is needed\n' \
            "$(printf '%s\n' "$inputs" | wc -l | tr -d ' ')" "$merge_base" "$(printf '%s\n' "$inputs" | head -n 1)"
    fi
    if [ "$current" = "$(pins "$merge_base")" ] || [ "$current" = "$base_pins" ]; then
        exit $status
    fi
    base_number=$(field 1 "$base_pins")
    base_pair="$(field 2 "$base_pins") $(field 3 "$base_pins")"
    if [ "$number" -eq "$base_number" ] && same_output "$pinned_pair" "$base_pair"; then
        printf 'analyzer schema: pins differ from %s under corvint-analyzer/%s but name the same golden output; no bump needed (IDX-SNAP-V0-107)\n' "$base" "$number"
        exit $status
    fi
    if [ "$number" -le "$base_number" ]; then
        printf 'analyzer schema: this branch changed the analyzer pins but keeps corvint-analyzer/%s, while %s is at corvint-analyzer/%s with output %s; a reused number would let one pack name two analyzer contents. Rebump to corvint-analyzer/%s with script/analyzer-schema.sh bump\n' \
            "$number" "$base" "$base_number" "$base_pair" "$((base_number + 1))" >&2
        status=1
    fi
    exit $status
fi

[ -n "$base_pins" ] || {
    printf 'analyzer schema: base %s does not resolve to readable pins; pass --base REF\n' "$base" >&2
    exit 1
}
base_number=$(field 1 "$base_pins")
base_pair="$(field 2 "$base_pins") $(field 3 "$base_pins")"
actual_pair=$(actual_output)
target=$number
if same_output "$actual_pair" "$base_pair"; then
    # The corpus compiles to base's output, so base's number still names these facts.
    target=$base_number
elif [ "$number" -le "$base_number" ]; then
    target=$((base_number + 1))
fi
if [ "$actual_pair" = "$pinned_pair" ] && [ "$target" = "$number" ]; then
    printf 'analyzer schema: the golden output matches corvint-analyzer/%s; nothing to bump\n' "$number"
else
    perl -0pi -e "s/^const analyzerSchemaID = \"corvint-analyzer\\/$number\"\$/const analyzerSchemaID = \"corvint-analyzer\\/$target\"/m or die" "$schema_go"
    perl -0pi -e "s/^(\\s*const auditedSchema = )\"corvint-analyzer\\/$number\"\$/\${1}\"corvint-analyzer\\/$target\"/m or die" "$schema_test"
    perl -0pi -e "s/^(\\s*const auditedOutputSHA256 = )\"[0-9a-f]{64}\"\$/\${1}\"$(field 1 "$actual_pair")\"/m or die; s/^(\\s*const auditedShardSHA256 = )\"[0-9a-f]{64}\"\$/\${1}\"$(field 2 "$actual_pair")\"/m or die" "$schema_test"
    printf 'analyzer schema: corvint-analyzer/%s -> corvint-analyzer/%s, output %s\n' "$number" "$target" "$actual_pair"
fi
script/repin-use-case-receipts.sh
