#!/bin/sh
set -eu

# Every requirement ID in docs/specs/REQUIREMENTS.tsv must have a definition line in
# some spec body. The generator falls back to a table row when no definition exists,
# so a clause body deleted by accident keeps its row and every other gate stays green:
# that is how commit dfd02ea removed eighteen GPK clause bodies without failing a gate.
#
# CEM-CB-* is exempt. docs/specs/cem-0.2-canonical-binding.md states that family in
# tables by design, so those IDs have no definition line and never had one.

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

if git ls-files --error-unmatch docs/specs/REQUIREMENTS.tsv >/dev/null 2>&1; then
    :
else
    status=$?
    [ "$status" -eq 1 ] && exit 0
    echo "requirement definitions: git ls-files failed (exit $status)" >&2
    exit "$status"
fi

perl - <<'PERL'
use strict;
use warnings;
use IPC::Open2 qw(open2);

# Every index read goes through one `git cat-file --batch` co-process, so a scan forks git once
# instead of once per file. A path the batch cannot serve (a newline breaks its request framing;
# a missing or non-blob entry) is read by a one-shot `git cat-file blob`, which reports and exits
# exactly as the per-file read always did.
my ($batch_out, $batch_in, $batch_pid);
sub index_blob_once {
    my ($path) = @_;
    open my $fh, '-|', 'git', 'cat-file', 'blob', ":$path" or die "$path: $!\n";
    my $blob = do { local $/; <$fh> } // '';
    close $fh
        or die $! ? "git cat-file blob :$path: $!\n" : "git cat-file blob :$path failed: exit " . ($? >> 8) . "\n";
    return $blob;
}

sub index_blob {
    my ($path) = @_;
    local $/ = "\n";
    return index_blob_once($path) if $path =~ /\n/;
    local $SIG{PIPE} = 'IGNORE';    # a dead co-process surfaces as a short read, not a signal
    $batch_pid //= open2($batch_out, $batch_in, 'git', 'cat-file', '--batch');
    print {$batch_in} ":$path\n";
    $batch_in->flush;
    my $header = <$batch_out> // batch_failed();
    my ($type, $size) = $header =~ /\A[0-9a-f]+ (\S+) (\d+)\n\z/ or return index_blob_once($path);
    my $read = read($batch_out, my $blob, $size + 1) // die "git cat-file --batch: $!\n";
    batch_failed() unless $read == $size + 1 && substr($blob, -1, 1, '') eq "\n";
    return $type eq 'blob' ? $blob : index_blob_once($path);
}

# Reaps the batch co-process and fails on its nonzero exit, reporting the status the way a closed
# `-|` pipe does; $! is cleared so `die` exits with that status rather than a stale errno.
sub close_index {
    return unless defined $batch_pid;
    close $batch_in;
    waitpid $batch_pid, 0;
    undef $batch_pid;
    return unless $?;
    $! = 0;
    die 'git cat-file --batch failed: exit ' . ($? >> 8) . "\n";
}

# A batch stream that ends before a header or a full body means the co-process died mid-scan.
sub batch_failed {
    close_index();
    die "git cat-file --batch failed: short read\n";
}

sub tracked_spec_docs {
    open my $ls, '-|', 'git', 'ls-files', '-z', '--cached', '--', 'docs/specs'
        or die "git ls-files: $!\n";
    local $/ = "\0";
    my @files;
    while (my $path = <$ls>) {
        chomp $path;
        next unless $path =~ m{\Adocs/specs/[^/]+\.md\z};
        push @files, $path;
    }
    close $ls
        or die $! ? "git ls-files: $!\n" : 'git ls-files failed: exit ' . ($? >> 8) . "\n";
    return sort @files;
}

sub index_lines {
    return split /^/, index_blob($_[0]);
}

my %defined;
for my $file (tracked_spec_docs()) {
    next if $file eq 'docs/specs/README.md';
    my $line_number = 0;
    for my $line (index_lines($file)) {
        ++$line_number;
        # The backticks must balance. With both optional and independent, an opening
        # backtick matched while the closing one did not, so a prose range like
        # `PUB-V0-011..015`. read as a second definition of PUB-V0-011. The
        # backreference makes the closing backtick required exactly when one opened.
        next unless $line =~ /^\s*(?:[-*]\s+)?(?:\*\*)?(`?)([A-Z][A-Z0-9-]*-[0-9]{3})\1(?:\*\*)?\s*[:.]/;
        push @{ $defined{$2} }, "$file:$line_number";
    }
}

my @undefined;
for my $row (split /^/, index_blob('docs/specs/REQUIREMENTS.tsv')) {
    chomp $row;
    next unless length $row;
    my ($id) = split /\t/, $row;
    next unless defined $id && $id =~ /^[A-Z][A-Z0-9-]*-[0-9]{3}$/;
    next if $id =~ /^CEM-CB-/;
    push @undefined, $id unless $defined{$id};
}
close_index();

my @duplicated;
for my $id (sort keys %defined) {
    my @locs = @{ $defined{$id} };
    next if @locs <= 1;
    push @duplicated, [$id, @locs];
}

my $failed = 0;
if (@undefined) {
    $failed = 1;
    print STDERR "requirement definitions: no clause body defines these ids, only a table row:\n";
    print STDERR "  $_\n" for @undefined;
}
if (@duplicated) {
    $failed = 1;
    for my $dup (@duplicated) {
        my ($id, @locs) = @$dup;
        for my $i (1 .. $#locs) {
            print STDERR "duplicate definition: $id at $locs[0] and $locs[$i]\n";
        }
    }
}
exit($failed);
PERL
