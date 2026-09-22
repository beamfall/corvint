#!/bin/sh
set -eu

# ARTIFACT-V0-008 (docs/specs/release-artifact-integrity-v0.md) requires pull-request CI to
# grant only `contents: read`, disable persisted checkout credentials, and pin every action to
# a full commit SHA. The static test that proved this lived in the deleted
# `tests/test_release_artifact.py` (removed with the Python wheel checker by `54735d98`) and had
# no Go or shell replacement -- this script is that replacement. See
# docs/agent-memory/tests.md's ARTIFACT-V0-008 history before this script existed.
#
# This checks exactly the three properties named in the requirement, no more:
#   1. the workflow-level `permissions:` block is exactly `contents: read` (nothing broader,
#      nothing else declared alongside it), and no job-level block grants more;
#   2. every `actions/checkout` step sets `persist-credentials: false`;
#   3. every `uses:` reference is pinned to a full 40-character commit SHA, not a tag or branch.
#
# It reads the workflow's bytes from the Git index (`git cat-file blob :<path>`), like the other
# gate scripts. `make gate` must measure what a fresh clone contains (`Makefile:14`): reading the
# worktree file passed a violation that was committed but edited away on disk, and failed on an
# uncommitted edit no clone would carry. The index, not HEAD, so a fix staged in the same change
# is what the gate measures.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

workflow=.github/workflows/ci.yml
git ls-files --error-unmatch -- "$workflow" >/dev/null 2>&1 ||
    { echo "ci least-privilege: $workflow is not tracked" >&2; exit 1; }

perl - "$workflow" <<'PERL'
use strict;
use warnings;

my ($path) = @ARGV;
open my $fh, '-|', 'git', 'cat-file', 'blob', ":$path" or die "$path: $!\n";
my @lines = <$fh>;
close $fh
    or die $! ? "git cat-file blob :$path: $!\n" : "git cat-file blob :$path failed: exit " . ($? >> 8) . "\n";

my @failures;

# --- 1. workflow-level permissions is exactly `contents: read` ---------------------------
my @top_level_blocks;
for (my $i = 0; $i < @lines; $i++) {
    next unless $lines[$i] =~ /^permissions:\s*(?:#.*)?$/;
    my @block;
    my $j = $i + 1;
    while ($j < @lines && $lines[$j] =~ /^(\s+)\S/) {
        push @block, $lines[$j] unless $lines[$j] =~ /^\s*#/;
        $j++;
    }
    push @top_level_blocks, [$i + 1, \@block];
}

if (@top_level_blocks != 1) {
    push @failures, sprintf(
        "expected exactly one workflow-level `permissions:` block, found %d",
        scalar @top_level_blocks,
    );
} else {
    my ($line_number, $block) = @{ $top_level_blocks[0] };
    my @entries = grep { /\S/ } @$block;
    if (@entries != 1 || $entries[0] !~ /^\s{2}contents:\s*read\s*(?:#.*)?$/) {
        push @failures, sprintf(
            "%s:%d: workflow-level `permissions:` must be exactly `contents: read`, found: %s",
            $path, $line_number, join(' / ', map { s/^\s+|\s+$//gr } @entries) || '(empty)',
        );
    }
}

# --- 1b. no job-level `permissions:` grants more than `contents: read` ----------------------
# A job-level block replaces the workflow-level one for that job, so it is held to the same
# exact `contents: read`; only the inline `{}` (no permission at all) is narrower still.
for (my $i = 0; $i < @lines; $i++) {
    next unless $lines[$i] =~ /^(\s+)permissions:\s*(.*?)\s*$/;
    my ($indent, $inline) = (length($1), $2);
    $inline =~ s/\s*#.*$//;
    next if $inline eq '{}';
    my @entries;
    if ($inline eq '') {
        for (my $j = $i + 1; $j < @lines; $j++) {
            next if $lines[$j] =~ /^\s*(?:#.*)?$/;
            last if length(($lines[$j] =~ /^(\s*)/)[0]) <= $indent;
            push @entries, $lines[$j] =~ s/^\s+|\s+$//gr;
        }
    } else {
        @entries = ($inline);
    }
    next if @entries == 1 && $entries[0] =~ /^contents:\s*read\s*(?:#.*)?$/;
    push @failures, sprintf(
        "%s:%d: job-level `permissions:` must be exactly `contents: read`, found: %s",
        $path, $i + 1, join(' / ', @entries) || '(empty)',
    );
}

# --- 2. every actions/checkout step disables persisted credentials -----------------------
# A step is the list item that holds the `uses:` key, whichever key the item opens with
# (`- uses:` or `- name:` then `uses:`); its extent ends at the next line indented no deeper
# than the item's dash.
for (my $i = 0; $i < @lines; $i++) {
    next unless $lines[$i] =~ /^(\s*)(?:-\s+)?uses:\s*actions\/checkout@/;
    my $uses_indent = length($1);
    my $line_number = $i + 1;
    my $start = $i;
    while ($start >= 0) {
        last if $lines[$start] =~ /^(\s*)-\s/ && ($start == $i || length($1) < $uses_indent);
        $start--;
    }
    my $found = 0;
    if ($start >= 0) {
        my ($dash_indent) = map { length } $lines[$start] =~ /^(\s*)/;
        for (my $j = $start + 1; $j < @lines; $j++) {
            my $line = $lines[$j];
            next if $line =~ /^\s*(?:#.*)?$/;
            last if length(($line =~ /^(\s*)/)[0]) <= $dash_indent;
            if ($line =~ /^\s+persist-credentials:\s*false\s*(?:#.*)?$/) {
                $found = 1;
                last;
            }
        }
    }
    push @failures, sprintf(
        "%s:%d: actions/checkout step is missing `persist-credentials: false`",
        $path, $line_number,
    ) unless $found;
}

# --- 3. every `uses:` reference is pinned to a full 40-character commit SHA --------------
for (my $i = 0; $i < @lines; $i++) {
    next unless $lines[$i] =~ /uses:\s*(\S+)@(\S+)/;
    my ($action, $ref) = ($1, $2);
    unless ($ref =~ /^[0-9a-fA-F]{40}$/) {
        push @failures, sprintf(
            "%s:%d: `uses: %s\@%s` is not pinned to a full 40-character commit SHA",
            $path, $i + 1, $action, $ref,
        );
    }
}

if (@failures) {
    print STDERR "ci least-privilege (ARTIFACT-V0-008):\n";
    print STDERR "  $_\n" for @failures;
    exit 1;
}
PERL
