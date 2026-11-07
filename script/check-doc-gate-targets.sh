#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Check the staged DOC_GATE_TARGETS list: ASD-V0-925 fast prefix, then sorted unique targets, one per line.
set -eu
git show :Makefile | perl -e '
use strict; use warnings;
my ($on, @targets, @errors);
while (<STDIN>) {
    if (/^DOC_GATE_TARGETS :=/) {
        push @errors, "line $.: use DOC_GATE_TARGETS := followed by a continuation" unless /^DOC_GATE_TARGETS := \\\s*$/;
        $on = 1; next;
    }
    next unless $on;
    if (/^  ([A-Za-z0-9][A-Za-z0-9_.-]*)( \\)?\s*$/) {
        push @targets, $1; $on = 0 unless defined $2;
    } else { push @errors, "line $.: expected one target per line with two spaces"; $on = 0; }
}
push @errors, "missing or unterminated DOC_GATE_TARGETS list" if $on || !@targets;
push @errors, "DOC_GATE_TARGETS must start with line-citations-check then spec-requirements-check (ASD-V0-925)"
    unless @targets >= 2 && $targets[0] eq "line-citations-check" && $targets[1] eq "spec-requirements-check";
my @rest = @targets > 2 ? @targets[2 .. $#targets] : ();
my %seen;
push @errors, "DOC_GATE_TARGETS must be duplicate-free with a bytewise sorted suffix after the fixed prefix"
    if (grep { $seen{$_}++ } @targets)
        || join(" ", @rest) ne join(" ", sort @rest);
if (@errors) { print STDERR "doc-gate-targets: FAIL $_; fix: keep line-citations-check then spec-requirements-check first; LC_ALL=C sort the remaining unique one-target-per-line entries in Makefile\n" for @errors; exit 1; }
print "doc-gate-targets: PASS\n";
'
