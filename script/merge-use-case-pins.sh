#!/bin/sh
set -eu

# script/merge-use-case-pins.sh (ASD-V0-917, V1-3936): the git merge driver for the use-case ledger
# and receipts (.gitattributes `merge=use-case-pins`). Two lanes that each repinned the same
# receipt change the same `repositoryRevision`, subject `sha256` and ledger `sha256` lines, so a
# plain text merge stops on a conflict whose two sides differ only in pinned digests. This driver
# runs the ordinary three-way `git merge-file` and resolves each conflict hunk whose sides are equal
# once every quoted 40- or 64-hex string is masked, by keeping ours: those strings are derived, and
# `script/repin-use-case-receipts.sh --merged` (the batch-regen use-case-receipts step) recomputes
# them from the merged tree. Any other conflict hunk stays conflicted and the driver exits 1, so a
# real content conflict (an added subject, a new use-case row, a changed claim) is never resolved.
#
# Install once per checkout (an unset driver falls back to the ordinary text merge):
#   git config merge.use-case-pins.driver 'script/merge-use-case-pins.sh %O %A %B %L'
#
#   script/merge-use-case-pins.sh BASE OURS THEIRS [MARKER-SIZE]   writes the result over OURS

[ $# -ge 3 ] || { printf 'usage: %s BASE OURS THEIRS [MARKER-SIZE]\n' "$0" >&2; exit 64; }

perl - "$1" "$2" "$3" "${4:-7}" <<'PERL'
use strict;
use warnings;

my ($base, $ours, $theirs, $size) = @ARGV;
die "merge-use-case-pins: marker size $size is not a number\n" unless $size =~ /\A[0-9]+\z/;
open my $merge, '-|', 'git', 'merge-file', '-p', "--marker-size=$size",
    '-L', 'ours', '-L', 'base', '-L', 'theirs', $ours, $base, $theirs
    or die "merge-use-case-pins: git merge-file: $!\n";
binmode $merge;
my @lines = <$merge>;
close $merge;
# git merge-file exits with the conflict count, or a negative status (255 here) on an error.
my $status = $? >> 8;
die "merge-use-case-pins: git merge-file failed\n" if $? & 127 || $status > 127;

sub masked { join('', @_) =~ s/"[0-9a-f]{40}(?:[0-9a-f]{24})?"/"PIN"/gr }

my (@out, @a, @base_part, @b, $state);
my $unresolved = 0;
for my $line (@lines) {
    if (!defined $state && $line =~ /\A<{$size} ours\r?\n?\z/) {
        $state = 'a';
        @a = @base_part = @b = ();
    } elsif (defined $state && $state eq 'a' && $line =~ /\A\|{$size}(?: |\r?\n?\z)/) {
        $state = 'base';
        push @base_part, $line;
    } elsif (defined $state && $state ne 'b' && $line =~ /\A={$size}\r?\n?\z/) {
        $state = 'b';
        push @base_part, $line;
    } elsif (defined $state && $state eq 'b' && $line =~ /\A>{$size} theirs\r?\n?\z/) {
        if (masked(@a) eq masked(@b)) {
            push @out, @a;
        } else {
            $unresolved++;
            push @out, "<" x $size . " ours\n", @a, @base_part, @b, $line;
        }
        undef $state;
    } elsif (!defined $state) {
        push @out, $line;
    } elsif ($state eq 'a') {
        push @a, $line;
    } elsif ($state eq 'base') {
        push @base_part, $line;
    } else {
        push @b, $line;
    }
}
die "merge-use-case-pins: unterminated conflict hunk in the merge-file output\n" if defined $state;

open my $fh, '>:raw', $ours or die "merge-use-case-pins: $ours: $!\n";
print {$fh} @out or die "merge-use-case-pins: $ours: $!\n";
close $fh or die "merge-use-case-pins: $ours: $!\n";
exit($unresolved ? 1 : 0);
PERL
