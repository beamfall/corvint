#!/bin/sh
set -eu

# Decision 0061 makes the root `.gitattributes` `* -text` with no exemptions, which means Git
# normalises nothing on the way in: a contributor whose checkout writes CRLF commits CRLF, and
# nothing corrects it. Worktree byte-exactness feeds the snapshot status digest, the archive
# gate's dirty-tree refusal and `dogfood-check` repository-wide, so the drift has to be caught
# where it lands. See
# docs/decisions/0061-windows-is-a-deferred-target-and-gitattributes-is-repository-wide-2026-09-05.md
# ("a `git ls-files --eol` drift check is a test backlog item").
#
# Two things are checked over the tracked set, which is what gets committed and is exactly what
# `git ls-files` names:
#
#   1. Every tracked path reports `attr/-text`. A path that reports anything else has escaped the
#      repository-wide rule, whether by a narrower `.gitattributes` elsewhere in the tree or by the
#      root file being edited or deleted.
#   2. No tracked blob's index bytes are CRLF or mixed, except the two intentional interop fixtures
#      below. Those two exist to carry non-LF endings and are the reason decision 0061 rejected
#      `text=auto`; they are listed by full path so a third one cannot arrive unnoticed.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
cd "$root"

perl - <<'PERL'
use strict;
use warnings;

my %exempt = map { $_ => 1 } (
    'interop/cem-0.1/repository/base/src/ending.txt',
    'interop/cem-0.1/patches/line-ending.patch',
);

open my $listing, '-|', 'git', 'ls-files', '--eol', '-z'
    or die "git ls-files: $!\n";
local $/ = "\0";

my (@attribute_drift, @ending_drift);
my ($tracked, $exempted) = (0, 0);
while (my $record = <$listing>) {
    chomp $record;
    next if $record eq '';
    my ($fields, $path) = split /\t/, $record, 2;
    die "eol policy: unparsable `git ls-files --eol` record: $record\n" unless defined $path;
    my ($index_eol, undef, $attr) = split ' ', $fields, 3;
    $attr =~ s/\s+\z// if defined $attr;
    $attr = '' unless defined $attr;
    ++$tracked;

    push @attribute_drift, "$path ($attr)" unless $attr eq 'attr/-text';

    if ($exempt{$path}) {
        ++$exempted;
        next;
    }
    push @ending_drift, "$path ($index_eol)" if $index_eol eq 'i/crlf' || $index_eol eq 'i/mixed';
}
close $listing
    or die $! ? "eol policy: git ls-files: $!\n" : 'eol policy: git ls-files failed: exit ' . ($? >> 8) . "\n";

my $status = 0;
if (@attribute_drift) {
    print STDERR "eol policy: tracked path(s) do not report attr/-text:\n";
    print STDERR "  $_\n" for @attribute_drift;
    print STDERR "the root .gitattributes must stay `* -text` (decision 0061)\n";
    $status = 1;
}
if (@ending_drift) {
    print STDERR "eol policy: tracked path(s) commit CRLF or mixed endings:\n";
    print STDERR "  $_\n" for @ending_drift;
    print STDERR "commit LF bytes, or add the path to the exemption list in script/check-eol-policy.sh\n";
    $status = 1;
}
exit $status if $status;

print "eol policy: $tracked tracked paths clean, $exempted exempt fixture(s)\n";
PERL
