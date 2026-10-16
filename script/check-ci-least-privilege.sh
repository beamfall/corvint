#!/bin/sh
set -eu

# ARTIFACT-V0-008 (docs/specs/release-artifact-integrity-v0.md) requires CI workflows to grant
# only `contents: read`, disable persisted checkout credentials, and pin every action to a full
# commit SHA. The static test that proved this lived in the deleted
# `tests/test_release_artifact.py` (removed with the Python wheel checker by `54735d98`) and had
# no Go or shell replacement -- this script is that replacement. Since V1-1469 it covers every
# tracked `.github/workflows/*.yml` (and `*.yaml`), not only `ci.yml`.
#
# This checks exactly the three properties named in the requirement, no more:
#   1. each workflow's workflow-level `permissions:` block is exactly `contents: read` (nothing
#      broader, nothing else declared alongside it), and no job-level block grants more, unless
#      `script/ci-least-privilege-exceptions.tsv` declares that exact block with a reason;
#   2. every `actions/checkout` step sets `persist-credentials: false`;
#   3. every `uses:` reference is pinned to a full 40-character commit SHA, not a tag or branch.
#
# An exception row names one block (the workflow level, or one job) and the exact permission set
# it holds; the block must match it entry for entry, and a row that matches no block fails, so
# the list cannot go stale or silently widen.
#
# It reads the workflows and the exception list from the Git index (`git cat-file blob :<path>`),
# like the other gate scripts. `make gate` must measure what a fresh clone contains
# (`Makefile:14`): reading the worktree file passed a violation that was committed but edited
# away on disk, and failed on an uncommitted edit no clone would carry. The index, not HEAD, so a
# fix staged in the same change is what the gate measures.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

git ls-files --error-unmatch -- .github/workflows/ci.yml >/dev/null 2>&1 ||
    { echo "ci least-privilege: .github/workflows/ci.yml is not tracked" >&2; exit 1; }

exceptions=script/ci-least-privilege-exceptions.tsv
git ls-files --error-unmatch -- "$exceptions" >/dev/null 2>&1 || exceptions=

perl - "$exceptions" <<'PERL'
use strict;
use warnings;

my ($exceptions_path) = @ARGV;

# Tracked workflow files, NUL-separated so no path is split; `git ls-files` lists what the index
# holds, which is what the gate measures.
my @workflows = do {
    local $/ = "\0";
    open my $ls, '-|', 'git', 'ls-files', '-z', '--', '.github/workflows/*.yml', '.github/workflows/*.yaml'
        or die "git ls-files: $!\n";
    my @paths = map { s/\0\z//r } <$ls>;
    close $ls or die "git ls-files .github/workflows failed\n";
    @paths;
};

sub blob {
    my ($path) = @_;
    open my $fh, '-|', 'git', 'cat-file', 'blob', ":$path" or die "$path: $!\n";
    my @lines = <$fh>;
    close $fh
        or die $! ? "git cat-file blob :$path: $!\n" : "git cat-file blob :$path failed: exit " . ($? >> 8) . "\n";
    return @lines;
}

my @failures;

# --- exception list: workflow<TAB>block<TAB>permissions<TAB>reason ---------------------------
# block is `workflow` or `job:<id>`; permissions is the exact set, as space-separated
# `scope:level` tokens. Each row is consumed by the one block it names.
my %exceptions;
if ($exceptions_path ne '') {
    my $n = 0;
    for my $line (blob($exceptions_path)) {
        $n++;
        next if $line =~ /^\s*(?:#.*)?$/;
        chomp $line;
        my @fields = split /\t/, $line, -1;
        my ($workflow, $block, $perms, $reason) = @fields;
        if (@fields != 4 || $workflow !~ m{^\.github/workflows/\S+\.ya?ml$}
            || $block !~ /^(?:workflow|job:[A-Za-z_][A-Za-z0-9_-]*)$/
            || $perms !~ /^[a-z-]+:(?:read|write|none)(?: [a-z-]+:(?:read|write|none))*$/
            || $reason !~ /\S/) {
            push @failures, "$exceptions_path:$n: expected workflow<TAB>workflow|job:<id><TAB>scope:level ...<TAB>reason";
            next;
        }
        my $key = "$workflow\t$block";
        if (exists $exceptions{$key}) {
            push @failures, "$exceptions_path:$n: duplicate exception for $workflow $block";
            next;
        }
        $exceptions{$key} = { perms => join(' ', sort split / /, $perms), line => $n, used => 0 };
    }
}

# allowed(PATH, BLOCK, ENTRIES): true when ENTRIES is exactly `contents: read` or exactly the
# declared exception for that block. ENTRIES are trimmed `scope: level` lines.
sub allowed {
    my ($path, $block, @entries) = @_;
    return 1 if @entries == 1 && $entries[0] =~ /^contents:\s*read\s*(?:#.*)?$/;
    my $exception = $exceptions{"$path\t$block"} or return 0;
    my @tokens;
    for (@entries) {
        /^([a-z-]+):\s*(read|write|none)\s*(?:#.*)?$/ or return 0;
        push @tokens, "$1:$2";
    }
    return 0 unless join(' ', sort @tokens) eq $exception->{perms};
    $exception->{used} = 1;
    return 1;
}

for my $path (@workflows) {
    my @lines = blob($path);

    # --- 1. workflow-level permissions is exactly `contents: read` -----------------------
    my @top_level_blocks;
    for (my $i = 0; $i < @lines; $i++) {
        next unless $lines[$i] =~ /^(["']?)permissions\1:\s*(?:#.*)?$/;
        # The block runs to the next line indented no deeper than the key; blank and comment
        # lines inside it do not end it.
        my @block;
        for (my $j = $i + 1; $j < @lines; $j++) {
            next if $lines[$j] =~ /^\s*(?:#.*)?$/;
            last unless $lines[$j] =~ /^\s+\S/;
            push @block, $lines[$j];
        }
        push @top_level_blocks, [$i + 1, \@block];
    }

    if (@top_level_blocks != 1) {
        push @failures, sprintf(
            "%s: expected exactly one workflow-level `permissions:` block, found %d",
            $path, scalar @top_level_blocks,
        );
    } else {
        my ($line_number, $block) = @{ $top_level_blocks[0] };
        my @entries = map { s/^\s+|\s+$//gr } grep { /\S/ } @$block;
        my $indented = !grep { !/^\s{2}\S/ } grep { /\S/ } @$block;
        unless ($indented && allowed($path, 'workflow', @entries)) {
            push @failures, sprintf(
                "%s:%d: workflow-level `permissions:` must be exactly `contents: read` or a declared exception, found: %s",
                $path, $line_number, join(' / ', @entries) || '(empty)',
            );
        }
    }

    # --- 1b. no job-level `permissions:` grants more than `contents: read` ------------------
    # A job-level block replaces the workflow-level one for that job, so it is held to the same
    # exact `contents: read` unless declared; the inline `{}` (no permission at all) is narrower
    # still. The job is the nearest preceding two-space key after the top-level `jobs:`.
    my ($jobs_line, $job) = (undef, undef);
    for (my $i = 0; $i < @lines; $i++) {
        if ($lines[$i] =~ /^jobs:\s*(?:#.*)?$/) { $jobs_line = $i; next }
        if ($lines[$i] =~ /^\S/) { $jobs_line = undef; $job = undef; next }
        # Any two-space key under `jobs:` starts a new job; one whose id is not a plain YAML key
        # (quoted, say) is no job an exception can name, so its block can only be `contents: read`.
        if (defined $jobs_line && $lines[$i] =~ /^  \S/) {
            $job = $lines[$i] =~ /^  ([A-Za-z_][A-Za-z0-9_-]*):\s*(?:#.*)?$/ ? $1 : undef;
        }
        next unless $lines[$i] =~ /^(\s+)(["']?)permissions\2:\s*(.*?)\s*$/;
        my ($indent, $inline) = (length($1), $3);
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
        my $block = defined $job && $indent == 4 ? "job:$job" : '(not a job)';
        next if @entries && allowed($path, $block, @entries);
        push @failures, sprintf(
            "%s:%d: job-level `permissions:` (%s) must be exactly `contents: read` or a declared exception, found: %s",
            $path, $i + 1, $block, join(' / ', @entries) || '(empty)',
        );
    }

    # --- 2. every actions/checkout step disables persisted credentials -------------------
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

    # --- 3. every `uses:` reference is pinned to a full 40-character commit SHA ----------
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
}

# A declared exception that no block matched is stale or wrong; either way it must not stand.
for my $key (sort keys %exceptions) {
    next if $exceptions{$key}{used};
    my ($workflow, $block) = split /\t/, $key;
    push @failures, sprintf(
        "%s:%d: exception for %s %s matches no permissions block (stale, or the block changed)",
        $exceptions_path, $exceptions{$key}{line}, $workflow, $block,
    );
}

if (@failures) {
    print STDERR "ci least-privilege (ARTIFACT-V0-008):\n";
    print STDERR "  $_\n" for @failures;
    exit 1;
}
PERL
