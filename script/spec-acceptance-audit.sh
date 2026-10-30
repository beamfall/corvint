#!/bin/sh
set -eu

# SRG-V0-010: list the requirements of one acceptance mode from docs/specs/REQUIREMENTS.tsv,
# grouped by spec with counts. The default mode, `standing`, is every requirement accepted under
# a standing approval that no owner read-back has promoted to `owner`. A spec whose INDEX.json
# delivery status is `validated` while it still holds a `standing` requirement is reported as
# NOT_PROMOTABLE whatever mode is listed. A requirement whose leading parenthetical records no
# mode but whose definition block holds an `accepted under ... standing` marker elsewhere (a
# trailing marker) is counted as UNPARSED_STANDING and, in the standing listing, named: it is
# `unrecorded` until its marker moves to the front. The audit reports and exits 0; it gates nothing.
# `make spec-requirements-check` is what binds both files to the staged specs. The committed index
# carries no line number (SRG-V0-015), so each listed location takes its line from the derived
# `script/gen-spec-requirements.sh --lines` view of the same staged specs (SRG-V0-017), and a row
# whose file is not the file that view defines the id in fails rather than citing the wrong file.

usage() { printf 'usage: %s [--acceptance standing|owner|unrecorded]\n' "$0" >&2; exit 2; }
mode=standing
case $# in
  0) ;;
  2) test "$1" = --acceptance || usage; mode=$2 ;;
  *) usage ;;
esac
case $mode in
  standing|owner|unrecorded) ;;
  *) usage ;;
esac

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

perl - "$mode" <<'PERL' || exit 1
use strict;
use warnings;
use JSON::PP ();

my $mode = shift @ARGV;
my $tsv = 'docs/specs/REQUIREMENTS.tsv';
open my $in, '<', $tsv or die "$tsv: $!\n";
my $header = <$in> // die "$tsv: empty\n";
$header eq "id\tfile\ttitle\tacceptance\tacceptedBy\n"
    or die "$tsv: header is not the committed id, file, title and acceptance columns; run make spec-requirements\n";
my (%count, %rows, %standing, %spec_lines, @unparsed, %line);

open my $view, '-|', 'script/gen-spec-requirements.sh', '--lines'
    or die "script/gen-spec-requirements.sh --lines: $!\n";
<$view> // die "script/gen-spec-requirements.sh --lines: no header\n";
while (my $row = <$view>) {
    my ($id, $file, $number) = split /\t/, $row;
    $line{$id} = [$file, $number];
}
close $view or die "script/gen-spec-requirements.sh --lines failed: exit " . ($? >> 8) . "\n";

# The definition block is the definition line and its continuation lines, up to a blank line, the
# next list item or a heading. A table row (a rank 1 winner) has no block.
sub standing_elsewhere {
    my ($file, $number) = @_;
    my $lines = $spec_lines{$file} //= do {
        open my $spec, '<', $file or die "$file: $!\n";
        [<$spec>];
    };
    my $i = $number - 1;
    return 0 if $i > $#$lines || $lines->[$i] =~ /\A\s*\|/;
    my $block = $lines->[$i++];
    $block .= $lines->[$i++] while $i <= $#$lines && $lines->[$i] !~ /\A\s*(?:\z|[-*] |#)/;
    return $block =~ /\baccepted\s+under\b[^.;()]*\bstanding\b/i;
}
while (my $line = <$in>) {
    chomp $line;
    my @fields = split /\t/, $line, -1;
    @fields == 5 or die "$tsv:$.: want 5 columns, got " . @fields . "\n";
    my ($id, $file, $title, $acceptance, $by) = @fields;
    $id =~ /\A[A-Z][A-Z0-9-]*-[0-9]{3}\z/
        && $file =~ m{\Adocs/specs/[^/]+\.md\z}
        or die "$tsv:$.: want a requirement id and a docs/specs/*.md file\n";
    my ($defined, $number) = @{$line{$id} // die "$tsv:$.: $id is not defined by the staged specs; run make spec-requirements\n"};
    $defined eq $file
        or die "$tsv:$.: $id is defined in $defined, not $file; run make spec-requirements\n";
    $acceptance =~ /\A(?:owner|standing|unrecorded)\z/
        or die "$tsv:$.: acceptance '$acceptance' is not owner, standing or unrecorded\n";
    ++$count{$acceptance};
    ++$standing{$file} if $acceptance eq 'standing';
    push @{$rows{$file}}, "  $id\t$file:$number\t$by" if $acceptance eq $mode;
    push @unparsed, "UNPARSED_STANDING\t$id\t$file:$number\n"
        if $acceptance eq 'unrecorded' && standing_elsewhere($file, $number);
}
close $in;

my $index = 'docs/specs/INDEX.json';
open my $json, '<', $index or die "$index: $!\n";
my $entries = eval { local $/; JSON::PP->new->utf8->decode(<$json>) };
ref $entries eq 'ARRAY' or die "$index: not a JSON array\n";
my %delivery = map { $_->{path} => $_->{delivery} } @$entries;

print "$mode requirements by spec\n";
for my $file (sort keys %rows) {
    printf "%s\t%d\n", $file, scalar @{$rows{$file}};
    print map { "$_\n" } sort @{$rows{$file}};
}
print sort @unparsed if $mode eq 'standing';
for my $file (sort keys %standing) {
    next unless ($delivery{$file} // '') =~ /\Avalidated\b/;
    printf "NOT_PROMOTABLE\t%s\tdelivery validated\t%d standing\n", $file, $standing{$file};
}
printf "total\t%d %s in %d specs\towner %d\tstanding %d\tunrecorded %d\tUNPARSED_STANDING %d\n",
    $count{$mode} // 0, $mode, scalar keys %rows, (map { $count{$_} // 0 } qw(owner standing unrecorded)),
    scalar @unparsed;
PERL
