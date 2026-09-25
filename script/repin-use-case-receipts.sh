#!/bin/sh
set -eu

# Repin use-case receipts whose subjects changed (V1-0268, docs/specs/use-case-conformance-v0.md).
#
# A receipt under conformance/use-cases-v0/receipts/ pins each subject file by the SHA-256 of its
# exact bytes, and the ledger pins each receipt the same way (UCV0-004, UCV0-005). A change that
# edits a pinned subject therefore fails `go run ./conformance/use-cases-v0` with
# `subject-digest-mismatch` until the receipt is repinned. This script does the repin that used to
# be done by hand: for every receipt with a changed subject it writes the subject's new digest and
# `repositoryRevision` = `git rev-parse HEAD` into the receipt, then the receipt's new digest into
# the ledger. It edits only those digest strings, so every other byte stays as committed.
#
#   script/repin-use-case-receipts.sh           repin every stale receipt and report each one
#   script/repin-use-case-receipts.sh --check   list stale receipts; exit 1 if any, write nothing
#
# It refuses, writing nothing, when a receipt or subject is missing, when a receipt's bytes do not
# match its ledger pin (a hand edit that needs review, not a repin), or when an old digest is not
# a unique string in the file it would be replaced in (for example a sealed-benchmark seal that
# also names the subject digest, UCV0-007). It reads the worktree, as the conformance runner does.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

mode=write
case ${1:-} in
"") ;;
--check) mode=check ;;
*) printf 'usage: %s [--check]\n' "$0" >&2; exit 64 ;;
esac

revision=$(git rev-parse HEAD)

perl - "$mode" "$revision" <<'PERL'
use strict;
use warnings;
use Digest::SHA qw(sha256_hex);
use JSON::PP ();

my ($mode, $revision) = @ARGV;
my $ledger_path = 'conformance/use-cases-v0/ledger.json';

sub slurp {
    my ($path) = @_;
    return undef if -l $path || !-f $path;
    open my $fh, '<:raw', $path or return undef;
    local $/;
    my $raw = <$fh>;
    close $fh;
    return $raw;
}

# replace_once swaps the one occurrence of a quoted 64-hex digest, or returns undef when the
# digest is absent or ambiguous, so a replacement can never touch a second, unrelated field.
sub replace_once {
    my ($text, $old, $new) = @_;
    my $count = () = $text =~ /"\Q$old\E"/g;
    return undef if $count != 1;
    $text =~ s/"\Q$old\E"/"$new"/;
    return $text;
}

my $ledger_raw = slurp($ledger_path) // die "repin: $ledger_path is missing\n";
my $ledger = JSON::PP->new->decode($ledger_raw);

my @refusals;
my @stale;    # [receipt path, pinned receipt digest, receipt raw, [[subject, old, new], ...]]
my $receipt_count = 0;
for my $row (@{ $ledger->{useCases} }) {
    for my $class (@{ $ledger->{evidenceClasses} }) {
        my $ref = $row->{evidence}{$class} or next;
        $receipt_count++;
        my $path = $ref->{path};
        my $raw = slurp($path);
        if (!defined $raw) {
            push @refusals, "$path: receipt is missing";
            next;
        }
        if (sha256_hex($raw) ne $ref->{sha256}) {
            push @refusals, "$path: receipt bytes do not match the ledger pin (hand edit?)";
            next;
        }
        my @changed;
        for my $subject (@{ JSON::PP->new->decode($raw)->{subjects} }) {
            my $bytes = slurp($subject->{path});
            if (!defined $bytes) {
                push @refusals, "$path: subject $subject->{path} is missing";
                next;
            }
            my $digest = sha256_hex($bytes);
            push @changed, [$subject->{path}, $subject->{sha256}, $digest] if $digest ne $subject->{sha256};
        }
        push @stale, [$path, $ref->{sha256}, $raw, \@changed] if @changed;
    }
}

my %updated;
my $new_ledger = $ledger_raw;
for my $entry (@stale) {
    my ($path, $pinned, $raw, $changed) = @$entry;
    my $text = $raw =~ s/("repositoryRevision":\s*")[0-9a-f]+"/$1$revision"/r;
    my @ambiguous = grep { !defined replace_once($text, $_->[1], $_->[2]) } @$changed;
    push @refusals, map { "$path: digest of $_->[0] is not a unique string in the receipt" } @ambiguous;
    next if @ambiguous;
    $text = replace_once($text, $_->[1], $_->[2]) for @$changed;
    $new_ledger = replace_once($new_ledger, $pinned, sha256_hex($text));
    die "repin: $ledger_path does not pin $path by a unique digest\n" unless defined $new_ledger;
    $updated{$path} = $text;
}

if (@refusals) {
    print STDERR "repin: refused, nothing written:\n";
    print STDERR "  $_\n" for @refusals;
    exit 1;
}

if (!@stale) {
    print "use-case receipts: all $receipt_count receipts current\n";
    exit 0;
}

my $verb = $mode eq 'check' ? 'stale' : 'repinned';
for my $entry (@stale) {
    my ($path, undef, undef, $changed) = @$entry;
    print "$verb $path\n";
    print "  subject $_->[0]\n" for @$changed;
}

if ($mode eq 'check') {
    print STDERR "use-case receipts: " . scalar(@stale) . " receipt(s) pin changed subjects; "
        . "run script/repin-use-case-receipts.sh and commit the result\n";
    exit 1;
}

sub spew {
    my ($path, $text) = @_;
    my $tmp = "$path.repin.$$";
    open my $fh, '>:raw', $tmp or die "repin: $tmp: $!\n";
    print {$fh} $text or die "repin: $tmp: $!\n";
    close $fh or die "repin: $tmp: $!\n";
    rename $tmp, $path or die "repin: $path: $!\n";
}
spew($_, $updated{$_}) for sort keys %updated;
spew($ledger_path, $new_ledger);
print "use-case receipts: " . scalar(@stale) . " receipt(s) repinned at $revision; "
    . "verify with go run ./conformance/use-cases-v0\n";
PERL
