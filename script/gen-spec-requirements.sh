#!/bin/sh
set -eu

# Enumerate tracked paths and read their bytes from the Git index, not the worktree: `make
# gate` must measure the set a fresh clone contains (`Makefile:14`), and `spec-requirements-check`
# byte-compares this output against the committed index. A worktree listing broke that both ways:
# an untracked spec added a row no clone can reproduce, and a tracked spec deleted from the
# worktree alone silently dropped its rows. `git add` is what publishes a spec to this index.
# `decision-number-gate-v0.md` pins the rule; `check-decision-numbers.sh` is the reference.

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
        or die $! ? "git cat-file :$path: $!\n" : "git cat-file :$path failed: exit " . ($? >> 8) . "\n";
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

my @specs;
{
    open my $paths, '-|', 'git', 'ls-files', '-z', '--', 'docs/specs'
        or die "git ls-files: $!\n";
    local $/ = "\0";
    while (my $path = <$paths>) {
        chomp $path;
        next unless $path =~ m{\Adocs/specs/[^/]+\.md\z};
        next if $path eq 'docs/specs/README.md';
        push @specs, $path;
    }
    close $paths
        or die $! ? "git ls-files: $!\n" : 'git ls-files failed: exit ' . ($? >> 8) . "\n";
}

my %best;
for my $file (sort @specs) {
    my $line_number = 0;
    for my $line (split /^/, index_blob($file)) {
        ++$line_number;
        while ($line =~ /\b([A-Z][A-Z0-9-]*-[0-9]{3})\b/g) {
            my $id = $1;
            my $quoted = quotemeta($id);
            my $is_period = $line =~ /^\s*(?:[-*]\s+)?(?:\*\*)?`?$quoted`?\.(?:\*\*)?\s/;
            my $rank = $is_period ? 0
                     : $line =~ /^\s*(?:[-*]\s+)?(?:\*\*)?`?$quoted`?(?:\*\*)?\s*:/ ? 0
                     : $line =~ /^\s*\|\s*`?$quoted`?(?:\s|\||\.)/ ? 1
                     : 2;
            next if $rank > 1;
            my $key = join("\0", sprintf('%d', $rank), $file, sprintf('%09d', $line_number));
            next if exists $best{$id} && $best{$id}{key} le $key;

            my $title = $line;
            chomp $title;
            $title =~ s/\r$//;
            if ($rank == 0 && $is_period) {
                $title =~ s/^\s*(?:[-*]\s+)?(?:\*\*)?`?$quoted`?\.(?:\*\*)?\s*//;
            } elsif ($rank == 0) {
                $title =~ s/^\s*(?:[-*]\s+)?(?:\*\*)?`?$quoted`?(?:\*\*)?\s*:(?:\*\*)?\s*//;
            }
            $title =~ s/[\t\r\n]+/ /g;
            $title =~ s/^\s+|\s+$//g;
            # SRG-V0-005 counts characters: decode UTF-8 only around the cut so a multibyte
            # sequence is never split and the whitespace trims keep their byte semantics. Bytes
            # that are not valid UTF-8 cannot be decoded and are cut by byte.
            my $decoded = utf8::decode($title);
            $title = substr($title, 0, 80) if length($title) > 80;
            utf8::encode($title) if $decoded;
            $title =~ s/\s+$//;
            $best{$id} = {
                key => $key,
                file => $file,
                line => $line_number,
                title => $title,
            };
        }
    }
}
close_index();

print "id\tfile\tline\ttitle\n";
for my $id (sort keys %best) {
    my $row = $best{$id};
    print join("\t", $id, $row->{file}, $row->{line}, $row->{title}), "\n";
}
PERL
