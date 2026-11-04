#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Cheap README row shape check (ASD-V0-2011, V1-4332). Run at the repository root:
# script/check-spec-readme.sh. Reads worktree README.md and INDEX.json only; no Go build.
# Status, claims and header agreement remain owned by internal/specindex.
set -eu
# A workflow-only fixture with neither file has no spec corpus, so the dogfood-closeout static phase
# skips this check as the batch-precheck one does (ASD-V0-2010, V1-4553).
if [ ! -e docs/specs/README.md ] && [ ! -e docs/specs/INDEX.json ]; then
  echo 'spec-readme-check: SKIP no spec corpus (neither docs/specs/README.md nor docs/specs/INDEX.json)'
  exit 0
fi
perl - <<'PERL'
use strict;
use warnings;
use JSON::PP ();
my $readme = 'docs/specs/README.md';
open my $index, '<', 'docs/specs/INDEX.json' or die "spec-readme-check: INDEX.json: $!\n";
my $entries = JSON::PP::decode_json(do { local $/; <$index> });
close $index or die "spec-readme-check: INDEX.json: $!\n";
my %expected = map { $_->{path} => 1 } @$entries;
open my $fh, '<', $readme or die "spec-readme-check: $readme: $!\n";
my (%rows, %refs, %exact, @errors);
while (my $line = <$fh>) {
    next unless $line =~ /^\|/;
    my @cells = split /\|/, $line, -1;
    next unless @cells >= 7;
    next unless $cells[2] =~ /\]\(([^)#]+\.md)(#[^)]*)?\)/;
    my ($path, $anchor) = ($1, $2);
    next if $path =~ m{^\.\./}; # hand-owned product/decision contract rows are not spec rows
    $path = "docs/specs/$path";
    push @{$refs{$path}}, $.;
    push @{$rows{$path}}, $. unless defined $anchor;
    push @{$exact{$line}}, $.;
}
close $fh or die "spec-readme-check: $readme: $!\n";
for my $line (sort { $exact{$a}[0] <=> $exact{$b}[0] } keys %exact) {
    next unless @{$exact{$line}} > 1;
    my $numbers = join(', ', @{$exact{$line}});
    chomp $line;
    push @errors, "fix: $readme: lines $numbers: duplicate row (remove extras): $line";
}
for my $path (sort keys %expected) {
    my @lines = @{$rows{$path} // []};
    next if @lines == 1;
    push @errors, "$readme: $path has " . scalar(@lines) . ' unanchored rows, want exactly 1; lines '
        . (@lines ? join(', ', @lines) : @{$refs{$path} // []} ? join(', ', @{$refs{$path}}) . ' (anchored only)' : '(missing)');
}
for my $path (sort keys %rows) {
    push @errors, "$readme: lines " . join(', ', @{$rows{$path}}) . ": unregistered spec row $path"
        unless $expected{$path};
}
if (@errors) {
    print STDERR "spec-readme-check: $_\n" for @errors;
    die "spec-readme-check: fix: remove the extra named rows at the reported line numbers; keep exactly one unanchored file row per INDEX.json spec (add the missing row or remove its #anchor); then stage docs/specs and run make spec-requirements\n";
}
print "spec-readme-check: PASS\n";
PERL
