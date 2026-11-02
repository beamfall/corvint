#!/bin/sh
set -eu

# With no argument the gate reads the Git index. TTG-V0-950 (V1-3586): `--range BASE..HEAD` checks
# the traceability rows of every commit in the range against the test functions declared at HEAD,
# the change head, so a spec-first commit may cite a test its change adds in a later commit, while
# a cited test that never lands still fails. The range is its first-parent commits, the change's
# own, and each unresolved row is named once, at the first commit that holds it.
range=
case ${1:-} in
  '') ;;
  --range)
    test $# = 2 && test -n "$2" || { printf 'usage: %s [--range BASE..HEAD]\n' "$0" >&2; exit 2; }
    range=$2
    ;;
  *) printf 'usage: %s [--range BASE..HEAD]\n' "$0" >&2; exit 2 ;;
esac

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

head= commits=
if test -n "$range"; then
  case $range in *..*) ;; *) printf '%s: --range must be BASE..HEAD, got %s\n' "$0" "$range" >&2; exit 2 ;; esac
  head=$(git rev-parse --verify -q "${range#*..}^{commit}") ||
    { printf '%s: --range head %s is not a commit\n' "$0" "${range#*..}" >&2; exit 2; }
  git rev-parse --verify -q "${range%%..*}^{commit}" > /dev/null ||
    { printf '%s: --range base %s is not a commit\n' "$0" "${range%%..*}" >&2; exit 2; }
  commits=$(git rev-list --first-parent --reverse "${range%%..*}..$head") ||
    { printf '%s: git rev-list %s failed\n' "$0" "$range" >&2; exit 2; }
  test -n "$commits" || { printf '%s: --range %s holds no commit\n' "$0" "$range" >&2; exit 2; }
fi

# shellcheck disable=SC2086
perl - "$head" $commits <<'PERL'
use strict;
use warnings;
use IPC::Open2 qw(open2);

# $test_ref is the tree whose test functions resolve a row ('' is the Git index); @spec_refs are the
# trees whose spec rows are checked, in commit order ('' alone is the index).
my ($test_ref, @spec_refs) = @ARGV;
@spec_refs = ('') unless @spec_refs;

# Every index read goes through one `git cat-file --batch` co-process, so a scan forks git once
# instead of once per file. A path the batch cannot serve (a newline breaks its request framing;
# a missing or non-blob entry) is read by a one-shot `git cat-file blob`, which reports and exits
# exactly as the per-file read always did.
my ($batch_out, $batch_in, $batch_pid);
sub index_blob_once {
    my ($path, $ref) = @_;
    $ref //= '';
    open my $fh, '-|', 'git', 'cat-file', 'blob', "$ref:$path" or die "$path: $!\n";
    my $blob = do { local $/; <$fh> } // '';
    close $fh
        or die $! ? "git cat-file blob $ref:$path: $!\n" : "git cat-file blob $ref:$path failed: exit " . ($? >> 8) . "\n";
    return $blob;
}

sub index_blob {
    my ($path, $ref) = @_;
    $ref //= '';
    local $/ = "\n";
    return index_blob_once($path, $ref) if $path =~ /\n/;
    local $SIG{PIPE} = 'IGNORE';    # a dead co-process surfaces as a short read, not a signal
    $batch_pid //= open2($batch_out, $batch_in, 'git', 'cat-file', '--batch');
    print {$batch_in} "$ref:$path\n";
    $batch_in->flush;
    my $header = <$batch_out> // batch_failed();
    my ($type, $size) = $header =~ /\A[0-9a-f]+ (\S+) (\d+)\n\z/ or return index_blob_once($path, $ref);
    my $read = read($batch_out, my $blob, $size + 1) // die "git cat-file --batch: $!\n";
    batch_failed() unless $read == $size + 1 && substr($blob, -1, 1, '') eq "\n";
    return $type eq 'blob' ? $blob : index_blob_once($path, $ref);
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

# tracked_paths lists REF's tree, or the Git index when REF is ''.
sub tracked_paths {
    my ($ref, @pathspec) = @_;
    return $ref eq ''
        ? ('git', 'ls-files', '-z', '--cached', '--', @pathspec)
        : ('git', 'ls-tree', '-r', '-z', '--name-only', $ref, '--', @pathspec);
}

sub tracked_spec_docs {
    my ($ref) = @_;
    open my $ls, '-|', tracked_paths($ref, 'docs/specs')
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
    return split /^/, index_blob($_[0], $_[1]);
}

my %tests;
open my $paths, '-|', tracked_paths($test_ref)
    or die "git ls-files: $!\n";
{
    local $/ = "\0";
    while (my $file = <$paths>) {
        chomp $file;
        next unless $file =~ /_test\.go\z/;
        my $source = index_blob($file, $test_ref);
        $tests{$1} = 1 while $source =~ /^\s*func\s+(Test[A-Za-z0-9_]+)\s*\(/mg;
    }
}
close $paths
    or die $! ? "git ls-files: $!\n" : 'git ls-files failed: exit ' . ($? >> 8) . "\n";

# TTG-V0-012 (V1-3235): the set above is an index snapshot by design, so an unresolved name stays
# unresolved; when an untracked or unstaged working-tree test file declares it, the refusal names
# that file and says to stage it, rather than reporting a test that exists as missing.
my %unstaged;
sub unstaged_hint {
    my ($name) = @_;
    unless (%unstaged) {
        $unstaged{''} = [];
        open my $ls, '-|', 'git', 'ls-files', '-z', '--others', '--modified', '--exclude-standard'
            or die "git ls-files: $!\n";
        local $/ = "\0";
        while (my $file = <$ls>) {
            chomp $file;
            next unless $file =~ /_test\.go\z/ && open my $fh, '<', $file;
            my $source = do { local $/; <$fh> } // '';
            close $fh;
            push @{ $unstaged{$1} }, $file while $source =~ /^\s*func\s+(Test[A-Za-z0-9_]+)\s*\(/mg;
        }
        close $ls
            or die $! ? "git ls-files: $!\n" : 'git ls-files failed: exit ' . ($? >> 8) . "\n";
    }
    return '' if $test_ref ne '';
    my $files = $unstaged{$name} or return '';
    my %seen;
    my @files = grep { !$seen{$_}++ } @$files;
    return " (declared only in the unstaged working-tree file @files; the check reads the index, so stage it: git add -- @files)";
}

my (@missing, %reported);
my $planned = 0;
my $wildcards = 0;
for my $spec_ref (@spec_refs) {
    my $at = $spec_ref eq '' ? '' : substr($spec_ref, 0, 12) . ' ';
    for my $file (tracked_spec_docs($spec_ref)) {
        my ($active, $level, $line_number) = (0, 0, 0);
        for my $line (index_lines($file, $spec_ref)) {
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
                # TTG-V0-917 (V1-4048): a word of the test-name shape that names no test, such as a fixture
                # `TestLine`, is written with `(not a test)` right after it and is not checked or counted.
                next if $after =~ /^`?[ \t]*\(not a test\)/;
                # DCW-V0-356 (V1-2855): a wildcard such as `TestX_*` is never expanded; the refusal says
                # so and names the declared functions it would have matched, so the row can list them.
                if ($after =~ /^\*/) {
                    my @matches = sort grep { index($_, $name) == 0 } keys %tests;
                    my $shown = join(', ', @matches[0 .. ($#matches < 4 ? $#matches : 4)])
                        . (@matches > 5 ? ' and ' . (@matches - 5) . ' more' : '');
                    push @missing, "$at$file:$line_number: $name* is a wildcard; name each test function in full ("
                        . (@matches ? scalar(@matches) . " declared: $shown" : 'no declared function has this prefix') . ')';
                    ++$wildcards;
                    next;
                }
                next if $tests{$name};
            my $first = $reported{"$file\0$name"} //= $spec_ref;
            push @missing, "$at$file:$line_number: $name" . unstaged_hint($name) if $first eq $spec_ref;
            }
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
    # TTG-V0-950: a spec-first commit may cite a test that a later commit of its change adds.
    print STDERR "traceability tests: a spec-first commit whose test lands in a later commit of the same change "
        . "passes `make traceability-tests-check TRACE_RANGE=BASE..HEAD`, which resolves every commit's rows "
        . "against the change head (TTG-V0-950)\n"
        if $test_ref eq '';
    exit 1;
}
PERL
