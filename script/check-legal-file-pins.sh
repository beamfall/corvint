#!/usr/bin/env bash
# usage: script/check-legal-file-pins.sh [MANIFEST]
#
# ASD-V0-073 (V1-3238): the release-artifact manifest (default
# conformance/release-artifact-v0/manifest.json) pins every legal file by sha256 (GPK-V0-020), and
# `make go-archive-gate` fails with `FAIL legal-file-altered` when a checkout's bytes differ. That
# gate runs only in CI, so batch-precheck runs this check first: for each `legalFiles` entry it
# hashes the worktree file and, on a missing file or a stale pin, prints
# `check-legal-file-pins: FAIL legal-file-altered PATH sha256 ACTUAL, MANIFEST pins EXPECTED` (or
# `legal-file-missing`) with the fix, and exits 1. A checkout without the manifest passes with a
# note; an unreadable manifest exits 2.
set -uo pipefail
manifest=${1:-conformance/release-artifact-v0/manifest.json}
if [[ ! -e $manifest ]]; then
  printf 'check-legal-file-pins: skipped: no %s in this checkout\n' "$manifest"
  exit 0
fi
MANIFEST=$manifest exec perl -MJSON::PP -MDigest::SHA=sha256_hex -e '
use strict;
use warnings;
my $manifest = $ENV{MANIFEST};
open my $in, "<", $manifest or do { print STDERR "check-legal-file-pins: cannot read $manifest: $!\n"; exit 2 };
my $doc = eval { local $/; JSON::PP->new->decode(scalar <$in>) };
my $files = ref $doc eq "HASH" ? $doc->{legalFiles} : undef;
unless (ref $files eq "ARRAY" && @$files) {
    print STDERR "check-legal-file-pins: $manifest has no legalFiles list\n";
    exit 2;
}
my $bad = 0;
for my $entry (@$files) {
    my ($path, $want) = ref $entry eq "HASH" ? @{$entry}{qw(path sha256)} : ();
    unless (defined $path && defined $want) {
        print STDERR "check-legal-file-pins: $manifest has a legalFiles entry without path and sha256\n";
        exit 2;
    }
    my $fh;
    unless (open $fh, "<:raw", $path) {
        print "check-legal-file-pins: FAIL legal-file-missing $path: $!; restore it or remove its $manifest legalFiles entry\n";
        $bad = 1;
        next;
    }
    my $got = sha256_hex(do { local $/; <$fh> } // "");
    next if $got eq $want;
    print "check-legal-file-pins: FAIL legal-file-altered $path sha256 $got, $manifest pins $want; "
        . "a legal-file edit must update that legalFiles sha256 in the same change, or make go-archive-gate fails in CI\n";
    $bad = 1;
}
printf "check-legal-file-pins: %d legal file(s) match %s\n", scalar @$files, $manifest unless $bad;
exit $bad;
'
