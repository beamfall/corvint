#!/bin/sh
set -eu

# SRG-V0-041: resolves a merge whose only remaining conflicts are in the generated spec index
# files (docs/specs/REQUIREMENTS.tsv, INDEX.json, README.md), as when two branches append adjacent
# requirement rows or each add a new spec. It refuses, naming them, while any other path is
# unmerged: resolve and stage a conflicted spec first. Each conflicted generated file restarts
# from this side's version (`--ours`), then all three are regenerated from the staged specs, as
# `make spec-requirements` does, and staged. A new spec the other side added gets the INDEX.json
# entry and README.md row its header derives; hand-set fields of that entry are re-applied by hand.
# SRG-V0-920 (V1-3448): with no unmerged path at all (the generated files merged cleanly, or no
# merge is in progress) it still regenerates and stages the three, and exits 0.
# SRG-V0-921 (V1-3338): the use-case receipts and their ledger (conformance/use-cases-v0/) are
# derived too: a conflicted one restarts from `--ours` and, after the spec regeneration,
# script/repin-use-case-receipts.sh repins every receipt whose subject changed, and they are staged.

# SRG-V0-921 (review fix): `--ours` drops the other side's whole file, and the reconciliation below
# carries only digests, so a receipt or ledger restarts from `--ours` only when the other side's
# change to it is digest-only: its stage 3 equals the merge base (stage 1, else stage 2) once every
# "sha256" and "repositoryRevision" value is blanked. Any other change is left for a hand merge.

generated='docs/specs/REQUIREMENTS.tsv docs/specs/INDEX.json docs/specs/README.md'
receipts=conformance/use-cases-v0
blank_digests() { sed -E 's/("(sha256|repositoryRevision)"[[:space:]]*:[[:space:]]*")[0-9a-f]*"/\1"/g'; }
digest_only() {
  git cat-file -e ":2:$1" 2> /dev/null && git cat-file -e ":3:$1" 2> /dev/null || return 1
  base=":1:$1"
  git cat-file -e "$base" 2> /dev/null || base=":2:$1"
  test "$(git show "$base" | blank_digests)" = "$(git show ":3:$1" | blank_digests)"
}
unmerged=$(git diff --name-only --diff-filter=U)
other= repin= manual=
for path in $unmerged; do
  case " $generated " in
    *" $path "*) continue ;;
  esac
  case $path in
    "$receipts/ledger.json" | "$receipts/receipts/"*)
      if digest_only "$path"; then repin=1; else manual="$manual $path"; fi ;;
    *) other="$other $path" ;;
  esac
done
if test -n "$other"; then
  echo "spec-requirements-resolve: resolve and stage these first:$other" >&2
  exit 1
fi
if test -n "$manual"; then
  echo "spec-requirements-resolve: the other side changed more than digests in:$manual; merge them by hand and stage them, then rerun" >&2
  exit 1
fi
for path in $unmerged; do
  git checkout -q --ours -- "$path"
  git add -- "$path"
done
# SRG-V0-922 (V1-3437): a generated file staged with its conflict markers restarts from HEAD.
for path in $(git grep -l --cached -e '^<<<<<<< ' -e '^>>>>>>> ' -- $generated 2> /dev/null || true); do
  git checkout -q HEAD -- "$path"
done
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
script/gen-spec-requirements.sh --all "$tmp"
for file in REQUIREMENTS.tsv INDEX.json README.md; do
  cp "$tmp/$file" "docs/specs/$file"
done
git add -- $generated
if test -n "$repin"; then
  # The ledger restarted from this side; a receipt the other side changed is pinned there. Take that
  # pin when the receipt's bytes are exactly the ones the other side's ledger pins, then repin.
  git show MERGE_HEAD:"$receipts/ledger.json" > "$tmp/theirs.json" 2> /dev/null || : > "$tmp/theirs.json"
  perl - "$receipts/ledger.json" "$tmp/theirs.json" <<'PERL' || exit 1
use strict;
use warnings;
use Digest::SHA qw(sha256_hex);
use JSON::PP ();
my ($path, $theirs_path) = @ARGV;
sub slurp { open my $fh, '<:raw', $_[0] or return undef; local $/; return scalar <$fh>; }
my $text = slurp($path);
my $ours = JSON::PP->new->decode($text);
my $raw = slurp($theirs_path);
my $theirs = length($raw // '') ? JSON::PP->new->decode($raw) : { useCases => [] };
my %their_pin;
for my $row (@{ $theirs->{useCases} }) {
    for my $class (keys %{ $row->{evidence} || {} }) {
        my $ref = $row->{evidence}{$class};
        $their_pin{"$ref->{path}\0$ref->{sha256}"} = 1;
    }
}
for my $row (@{ $ours->{useCases} }) {
    for my $class (sort keys %{ $row->{evidence} || {} }) {
        my $ref = $row->{evidence}{$class};
        my $bytes = slurp($ref->{path});
        next unless defined $bytes;
        my $digest = sha256_hex($bytes);
        next if $digest eq $ref->{sha256} || !$their_pin{"$ref->{path}\0$digest"};
        my $count = () = $text =~ /\Q$ref->{sha256}\E/g;
        die "spec-requirements-resolve: $ref->{path}: pin $ref->{sha256} is not unique in $path\n" if $count != 1;
        $text =~ s/\Q$ref->{sha256}\E/$digest/;
        print "spec-requirements-resolve: took the other side's pin for $ref->{path}\n";
    }
}
open my $out, '>:raw', $path or die "$path: $!\n";
print $out $text;
close $out or die "$path: $!\n";
PERL
  script/repin-use-case-receipts.sh
  git add -u -- "$receipts"
fi
test -n "$unmerged" || echo "spec-requirements-resolve: no unmerged path; regenerating anyway"
echo "spec-requirements-resolve: regenerated and staged $generated${repin:+ and repinned $receipts}"
