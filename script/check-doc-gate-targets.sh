#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Check the staged DOC_GATE_TARGETS list: sorted, unique, one target per continuation line.
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
push @errors, "DOC_GATE_TARGETS must be bytewise sorted and duplicate-free"
    if join(" ", @targets) ne join(" ", sort keys %{ {map { $_ => 1 } @targets} });
if (@errors) { print STDERR "doc-gate-targets: FAIL $_; fix: sort the one-target-per-line DOC_GATE_TARGETS list in Makefile\n" for @errors; exit 1; }
print "doc-gate-targets: PASS\n";
'
