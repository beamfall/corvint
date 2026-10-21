#!/bin/sh
set -eu

# Analyzer schema maintenance (V1-2130, V1-1732; docs/specs/index-snapshot-v0.md
# IDX-SNAP-V0-060 and IDX-SNAP-V0-061).
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

# pins prints "N DIGEST" for the schema number in analyzer_schema.go and the digest in the test,
# read from a tree-ish (git show) or, with "-", from the worktree. It prints nothing when either
# pin is absent, so a caller can tell an unreadable base from a real pin.
pins() {
    if [ "$1" = - ]; then
        number=$(sed -n 's/^const analyzerSchemaID = "corvint-analyzer\/\([0-9][0-9]*\)"$/\1/p' "$schema_go")
        digest=$(sed -n 's/^[[:space:]]*const auditedSHA256 = "\([0-9a-f]\{64\}\)"$/\1/p' "$schema_test")
        tested=$(sed -n 's/^[[:space:]]*const auditedSchema = "corvint-analyzer\/\([0-9][0-9]*\)"$/\1/p' "$schema_test")
    else
        number=$(git show "$1:$schema_go" 2>/dev/null | sed -n 's/^const analyzerSchemaID = "corvint-analyzer\/\([0-9][0-9]*\)"$/\1/p')
        digest=$(git show "$1:$schema_test" 2>/dev/null | sed -n 's/^[[:space:]]*const auditedSHA256 = "\([0-9a-f]\{64\}\)"$/\1/p')
        tested=$number
    fi
    if [ -n "$number" ] && [ -n "$digest" ] && [ "$number" = "$tested" ]; then
        printf '%s %s\n' "$number" "$digest"
    fi
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
number=${current%% *}
pinned=${current#* }
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
    base_number=${base_pins%% *}
    if [ "$number" -le "$base_number" ]; then
        printf 'analyzer schema: this branch changed the analyzer pins but keeps corvint-analyzer/%s, while %s is at corvint-analyzer/%s with digest %s; a reused number would let one pack name two analyzer contents. Rebump to corvint-analyzer/%s with script/analyzer-schema.sh bump\n' \
            "$number" "$base" "$base_number" "${base_pins#* }" "$((base_number + 1))" >&2
        status=1
    fi
    exit $status
fi

[ -n "$base_pins" ] || {
    printf 'analyzer schema: base %s does not resolve to readable pins; pass --base REF\n' "$base" >&2
    exit 1
}
base_number=${base_pins%% *}
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
    if [ "$number" -le "$base_number" ]; then
        target=$((base_number + 1))
    fi
    perl -0pi -e "s/^const analyzerSchemaID = \"corvint-analyzer\\/$number\"\$/const analyzerSchemaID = \"corvint-analyzer\\/$target\"/m or die" "$schema_go"
    perl -0pi -e "s/^(\\s*const auditedSchema = )\"corvint-analyzer\\/$number\"\$/\${1}\"corvint-analyzer\\/$target\"/m or die; s/^(\\s*const auditedSHA256 = )\"$pinned\"\$/\${1}\"$actual\"/m or die" "$schema_test"
    # The schema file is itself an audited input, so the digest moves with the number.
    final=$(audit_digest)
    perl -0pi -e "s/^(\\s*const auditedSHA256 = )\"$actual\"\$/\${1}\"$final\"/m or die" "$schema_test"
    printf 'analyzer schema: corvint-analyzer/%s -> corvint-analyzer/%s, digest %s\n' "$number" "$target" "$final"
fi
script/repin-use-case-receipts.sh
