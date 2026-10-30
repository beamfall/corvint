#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

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

my %tests;
open my $paths, '-|', 'git', 'ls-files', '-z', '--cached'
    or die "git ls-files: $!\n";
{
    local $/ = "\0";
    while (my $file = <$paths>) {
        chomp $file;
        next unless $file =~ /_test\.go\z/;
        my $source = index_blob($file);
        $tests{$1} = 1 while $source =~ /^\s*func\s+(Test[A-Za-z0-9_]+)\s*\(/mg;
    }
}
close $paths
    or die $! ? "git ls-files: $!\n" : 'git ls-files failed: exit ' . ($? >> 8) . "\n";

my @missing;
my $planned = 0;
my $wildcards = 0;
for my $file (tracked_spec_docs()) {
    my ($active, $level, $line_number) = (0, 0, 0);
    for my $line (index_lines($file)) {
        ++$line_number;
        if ($line =~ /^(#{1,6})\s+(.+)/) {
            my ($next_level, $title) = (length($1), $2);
            $active = 0 if $active && $next_level <= $level;
            ($active, $level) = (1, $next_level) if !$active && $title =~ /traceability/i;
            next;
        }
        next unless $active && $line =~ /^\s*\|/;
        while ($line =~ /\b(Test[A-Za-z0-9_]+)\b/g) {
            my $name = $1;
            # TTG-V0-011 (V1-3143): a selector such as `toolgroup.TestValidity` or `(*T).TestRun` names
            # a Go identifier of another package or type, never a cited test function, which a row
            # always names bare; it is not checked.
            my $start = $-[1];
            next if $start >= 2 && substr($line, $start - 1, 1) eq '.' && substr($line, $start - 2, 1) =~ /[\w)]/;
            my $after = substr($line, pos($line));
            if ($after =~ /^`?[ \t]*\(PLANNED\)/) {
                ++$planned;
                next;
            }
            # DCW-V0-356 (V1-2855): a wildcard such as `TestX_*` is never expanded; the refusal says
            # so and names the declared functions it would have matched, so the row can list them.
            if ($after =~ /^\*/) {
                my @matches = sort grep { index($_, $name) == 0 } keys %tests;
                my $shown = join(', ', @matches[0 .. ($#matches < 4 ? $#matches : 4)])
                    . (@matches > 5 ? ' and ' . (@matches - 5) . ' more' : '');
                push @missing, "$file:$line_number: $name* is a wildcard; name each test function in full ("
                    . (@matches ? scalar(@matches) . " declared: $shown" : 'no declared function has this prefix') . ')';
                ++$wildcards;
                next;
            }
            push @missing, "$file:$line_number: $name" unless $tests{$name};
        }
    }
}

close_index();
print "traceability tests: $planned planned\n";
if (@missing) {
    print STDERR "traceability tests: unresolved test functions:\n";
    print STDERR "  $_\n" for @missing;
    print STDERR "traceability tests: rows must name declared Test functions in full; wildcard and prefix forms are not resolved\n"
        if $wildcards;
    exit 1;
}
PERL
