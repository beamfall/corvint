#!/bin/sh
set -eu

# Analyzer schema maintenance (V1-2130, V1-1732, V1-2289; docs/specs/index-snapshot-v0.md
# IDX-SNAP-V0-060, IDX-SNAP-V0-061 and IDX-SNAP-V0-107).
#
# internal/contextindex/analyzer_schema.go names the analyzer schema (corvint-analyzer/N) and
# TestAnalyzerSchemaInputs pins the SHA-256 audit digest of every analyzer input file next to that
# number (IDX-SNAP-V0-017). An edit to any input used to need three manual steps: bump N, paste
# the digest the failing test printed into the test, and run script/repin-use-case-receipts.sh.
#
#   script/analyzer-schema.sh bump  [--base REF]   recompute the digest; when it drifted, set N to
#                                                  base's N + 1 (or keep this branch's N when it is
#                                                  already above base's), rewrite both pins and
#                                                  repin stale use-case receipts. One bump per branch.
#   script/analyzer-schema.sh check [--base REF]   refuse when the pinned digest is stale, or when
#                                                  this branch changed the pins but kept a schema
#                                                  number that REF already uses with another digest.
#
# TestAnalyzerSchemaOutput also pins auditedOutputSHA256, the digest of the pack a fixed corpus
# compiles to (IDX-SNAP-V0-107). When the test pins one, `bump` runs that test to read the actual
# output digest: an input edit whose output equals REF's keeps REF's number and repins only the
# digests, and `check` accepts REF's number with another input digest when both output pins agree.
# The Go test stays the authority that the pinned output digest is the real one.
# CORVINT_ANALYZER_OUTPUT_COMMAND replaces the command that prints "analyzer output SHA256=<hex>".
#
# REF defaults to origin/main. Two branches that both bump to the same N with different digests
# can merge without a textual conflict on the number, so `check` compares against the base, not
# only the tree. When REF does not resolve, `check` reports the base comparison NOT_RUN (the
# digest check still runs) and `bump` refuses, so a bump never guesses its number.

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

# pins prints "N DIGEST OUTPUT" for the schema number in analyzer_schema.go and the input and
# output digests in the test, read from a tree-ish (git show) or, with "-", from the worktree.
# OUTPUT is "-" when the test pins no output digest. It prints nothing when N or DIGEST is absent,
# so a caller can tell an unreadable base from a real pin.
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
    digest=$(printf '%s\n' "$test_source" | sed -n 's/^[[:space:]]*const auditedSHA256 = "\([0-9a-f]\{64\}\)"$/\1/p')
    output=$(printf '%s\n' "$test_source" | sed -n 's/^[[:space:]]*const auditedOutputSHA256 = "\([0-9a-f]\{64\}\)"$/\1/p')
    if [ -n "$number" ] && [ -n "$digest" ] && [ "$number" = "$tested" ]; then
        printf '%s %s %s\n' "$number" "$digest" "${output:--}"
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

# audit_digest mirrors TestAnalyzerSchemaInputs: go.mod, go.sum when present, and every non-test
# .go file directly under the internal/ directories the test lists, sorted bytewise, each hashed
# as "path NUL content NUL". The directory list is read from the test itself, so the two cannot
# disagree on which inputs are audited; the test remains the authority and rejects a wrong pin.
audit_digest() {
    perl -e '
use strict;
use warnings;
use Digest::SHA;
my ($test) = @ARGV;
open my $fh, "<", $test or die "analyzer schema: cannot read $test\n";
my $source = do { local $/; <$fh> };
close $fh;
my ($list) = $source =~ /for _, directory := range \[\]string\{([^}]*)\}/
    or die "analyzer schema: cannot find the audited directory list in $test\n";
my @directories = $list =~ /"([^"]+)"/g;
@directories or die "analyzer schema: empty audited directory list in $test\n";
my @paths = ("go.mod");
push @paths, "go.sum" if -e "go.sum";
for my $directory (@directories) {
    my @matches = grep { !/_test\.go$/ } glob("internal/$directory/*.go");
    @matches or die "analyzer schema: no inputs in internal/$directory\n";
    push @paths, @matches;
}
my $digest = Digest::SHA->new(256);
for my $path (sort @paths) {
    open my $in, "<:raw", $path or die "analyzer schema: cannot read $path\n";
    my $content = do { local $/; <$in> };
    close $in;
    $digest->add("$path\0$content\0");
}
print $digest->hexdigest, "\n";
' "$schema_test"
}

current=$(pins -)
[ -n "$current" ] || {
    printf 'analyzer schema: cannot read matching pins from %s and %s\n' "$schema_go" "$schema_test" >&2
    exit 1
}
number=$(field 1 "$current")
pinned=$(field 2 "$current")
pinned_output=$(field 3 "$current")
actual=$(audit_digest)

base_pins=
if git rev-parse --verify --quiet "$base^{commit}" >/dev/null; then
    base_pins=$(pins "$base")
fi

if [ "$mode" = check ]; then
    status=0
    if [ "$actual" != "$pinned" ]; then
        printf 'analyzer schema: inputs drifted from the audited digest (pinned %s, actual %s); run script/analyzer-schema.sh bump\n' "$pinned" "$actual" >&2
        status=1
    fi
    if [ -z "$base_pins" ]; then
        printf 'analyzer schema: base %s does not resolve; base collision check NOT_RUN\n' "$base" >&2
        exit $status
    fi
    merge_base=$(git merge-base HEAD "$base" 2>/dev/null) || merge_base=
    [ -n "$merge_base" ] || {
        printf 'analyzer schema: HEAD has no merge base with %s; base collision check NOT_RUN\n' "$base" >&2
        exit $status
    }
    if [ "$current" = "$(pins "$merge_base")" ] || [ "$current" = "$base_pins" ]; then
        exit $status
    fi
    base_number=$(field 1 "$base_pins")
    base_output=$(field 3 "$base_pins")
    if [ "$number" -eq "$base_number" ] && [ "$pinned_output" != - ] && [ "$pinned_output" = "$base_output" ]; then
        printf 'analyzer schema: inputs differ from %s under corvint-analyzer/%s but the audited output digest is unchanged; no bump needed (IDX-SNAP-V0-107)\n' "$base" "$number"
        exit $status
    fi
    if [ "$number" -le "$base_number" ]; then
        printf 'analyzer schema: this branch changed the analyzer pins but keeps corvint-analyzer/%s, while %s is at corvint-analyzer/%s with digest %s; a reused number would let one pack name two analyzer contents. Rebump to corvint-analyzer/%s with script/analyzer-schema.sh bump\n' \
            "$number" "$base" "$base_number" "$(field 2 "$base_pins")" "$((base_number + 1))" >&2
        status=1
    fi
    exit $status
fi

[ -n "$base_pins" ] || {
    printf 'analyzer schema: base %s does not resolve to readable pins; pass --base REF\n' "$base" >&2
    exit 1
}
base_number=$(field 1 "$base_pins")
base_output=$(field 3 "$base_pins")
collides=false
if [ "$current" != "$base_pins" ] && [ "$number" -le "$base_number" ]; then
    merge_base=$(git merge-base HEAD "$base" 2>/dev/null) || merge_base=
    if [ -z "$merge_base" ] || [ "$current" != "$(pins "$merge_base")" ]; then
        collides=true
    fi
fi
if [ "$actual" = "$pinned" ] && [ "$collides" = false ]; then
    printf 'analyzer schema: inputs match corvint-analyzer/%s; nothing to bump\n' "$number"
else
    target=$number
    output=$pinned_output
    if [ "$pinned_output" != - ]; then
        output=$(actual_output)
    fi
    if [ "$output" != - ] && [ "$output" = "$base_output" ]; then
        # The corpus compiles to base's pack bytes, so base's number still names these facts.
        target=$base_number
    elif [ "$number" -le "$base_number" ]; then
        target=$((base_number + 1))
    fi
    perl -0pi -e "s/^const analyzerSchemaID = \"corvint-analyzer\\/$number\"\$/const analyzerSchemaID = \"corvint-analyzer\\/$target\"/m or die" "$schema_go"
    perl -0pi -e "s/^(\\s*const auditedSchema = )\"corvint-analyzer\\/$number\"\$/\${1}\"corvint-analyzer\\/$target\"/m or die; s/^(\\s*const auditedSHA256 = )\"$pinned\"\$/\${1}\"$actual\"/m or die" "$schema_test"
    if [ "$output" != - ]; then
        perl -0pi -e "s/^(\\s*const auditedOutputSHA256 = )\"$pinned_output\"\$/\${1}\"$output\"/m or die" "$schema_test"
    fi
    # The schema file is itself an audited input, so the digest moves with the number.
    final=$(audit_digest)
    perl -0pi -e "s/^(\\s*const auditedSHA256 = )\"$actual\"\$/\${1}\"$final\"/m or die" "$schema_test"
    printf 'analyzer schema: corvint-analyzer/%s -> corvint-analyzer/%s, digest %s, output %s\n' "$number" "$target" "$final" "$output"
fi
script/repin-use-case-receipts.sh
