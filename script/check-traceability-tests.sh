#!/bin/sh
set -eu

# With no argument the gate reads the Git index. TTG-V0-950 (V1-3586): `--range BASE..HEAD` checks
# the traceability rows of every commit in the range against the test functions declared at HEAD,
# the change head, so a spec-first commit may cite a test its change adds in a later commit, while
# a cited test that never lands still fails. The range is its first-parent commits, the change's
# own, and each unresolved row is named once, at the first commit that holds it.
# TTG-V0-960, TTG-V0-961 (V1-4803): a citation `TestName/REQ-ID` resolves only when a declaration of
# TestName holds REQ-ID in its body. TTG-V0-962, TTG-V0-963: the range form also compares BASE's tree
# with HEAD's and refuses a requirement whose last test citation the range removed, unless its row
# states `(untested: REASON)`.
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

base='' head='' commits=''
if test -n "$range"; then
  case $range in *..*) ;; *) printf '%s: --range must be BASE..HEAD, got %s\n' "$0" "$range" >&2; exit 2 ;; esac
  head=$(git rev-parse --verify -q "${range#*..}^{commit}") ||
    { printf '%s: --range head %s is not a commit\n' "$0" "${range#*..}" >&2; exit 2; }
  base=$(git rev-parse --verify -q "${range%%..*}^{commit}") ||
    { printf '%s: --range base %s is not a commit\n' "$0" "${range%%..*}" >&2; exit 2; }
  commits=$(git rev-list --first-parent --reverse "${range%%..*}..$head") ||
    { printf '%s: git rev-list %s failed\n' "$0" "$range" >&2; exit 2; }
  test -n "$commits" || { printf '%s: --range %s holds no commit\n' "$0" "$range" >&2; exit 2; }
fi

# shellcheck disable=SC2086
perl - "$base" "$head" $commits <<'PERL'
use strict;
use warnings;
use IPC::Open2 qw(open2);

# $base_ref is the range base whose cited requirements HEAD must keep ('' in the index form);
# $test_ref is the tree whose test functions resolve a row ('' is the Git index); @spec_refs are the
# trees whose spec rows are checked, in commit order ('' alone is the index).
my ($base_ref, $test_ref, @spec_refs) = @ARGV;
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

# load_tests maps each top-level `func TestName(` declared in REF's tracked test files (TTG-V0-001)
# to the files that declare it; the files are kept so a subtest citation can read the bodies.
sub load_tests {
    my ($ref) = @_;
    my %decl;
    open my $paths, '-|', tracked_paths($ref)
        or die "git ls-files: $!\n";
    {
        local $/ = "\0";
        while (my $file = <$paths>) {
            chomp $file;
            next unless $file =~ /_test\.go\z/;
            my $source = index_blob($file, $ref);
            my %seen;
            while ($source =~ /^\s*func\s+(Test[A-Za-z0-9_]+)\s*\(/mg) {
                push @{ $decl{$1} }, $file unless $seen{$1}++;
            }
        }
    }
    close $paths
        or die $! ? "git ls-files: $!\n" : 'git ls-files failed: exit ' . ($? >> 8) . "\n";
    return { ref => $ref, decl => \%decl, has => {} };
}

my $head_tests = load_tests($test_ref);
my %tests = map { $_ => 1 } keys %{ $head_tests->{decl} };

# TTG-V0-960: a requirement ID is the longest text matching this shape at its position.
my $req_id = qr/[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-[0-9]+/;

# TTG-V0-961: the body of a declaration runs from its `func TestName(` line to the first later line
# that is exactly `}` in column 0 (gofmt closes a top-level function there), or to the end of the
# file. The ID counts only when no letter, digit or `-` precedes it and no letter or digit follows.
# Any one declaration of the name in any tracked test file of the tree satisfies the citation.
sub body_has_id {
    my ($tests, $name, $id) = @_;
    my $key = "$name\0$id";
    return $tests->{has}{$key} if exists $tests->{has}{$key};
    my $found = 0;
    FILE: for my $file (@{ $tests->{decl}{$name} || [] }) {
        my $source = index_blob($file, $tests->{ref});
        while ($source =~ /^\s*func\s+\Q$name\E\s*\(/mg) {
            my $start = $-[0];
            my $eol = index($source, "\n", $start);
            my $body = substr($source, $start);
            if ($eol >= 0 && substr($source, $eol) =~ /\n\}(?:\n|\z)/) {
                $body = substr($source, $start, $eol - $start + $+[0]);
            }
            if ($body =~ /(?<![A-Za-z0-9-])\Q$id\E(?![A-Za-z0-9])/) {
                $found = 1;
                last FILE;
            }
        }
    }
    return $tests->{has}{$key} = $found;
}

# is_retired reads a spec's header for TTG-V0-951.
sub is_retired {
    my ($lines) = @_;
    my $last = $#$lines < 39 ? $#$lines : 39;
    return scalar grep { /^(?:- )?\**Delivery status:\**\s*retired\b/ } @$lines[0 .. $last];
}

# scan_rows calls $row->($file, $line_number, $line) for every table row of a traceability section
# (TTG-V0-003, TTG-V0-004, TTG-V0-007) in REF's spec files that are not retired, and $spec->($file,
# \@lines) once per such file when given. It returns the number of retired specs it skipped.
sub scan_rows {
    my ($ref, $row, $spec) = @_;
    my $retired = 0;
    for my $file (tracked_spec_docs($ref)) {
        my ($active, $level, $line_number) = (0, 0, 0);
        my @lines = index_lines($file, $ref);
        # TTG-V0-951: a spec whose header says `Delivery status: retired` keeps its body as a record
        # of removed code, so its rows name tests that left with that code; it is not checked.
        if (is_retired(\@lines)) {
            ++$retired;
            next;
        }
        $spec->($file, \@lines) if $spec;
        for my $line (@lines) {
            ++$line_number;
            if ($line =~ /^(#{1,6})\s+(.+)/) {
                my ($next_level, $title) = (length($1), $2);
                $active = 0 if $active && $next_level <= $level;
                ($active, $level) = (1, $next_level) if !$active && $title =~ /traceability/i;
                next;
            }
            next unless $active && $line =~ /^\s*\|/;
            $row->($file, $line_number, $line);
        }
    }
    return $retired;
}

# row_names lists each `Test` word of a row with what the gate does with it: 'selector' (TTG-V0-011),
# 'planned' (TTG-V0-006), 'notatest' (TTG-V0-917), 'removed' (TTG-V0-952), 'wildcard' (DCW-V0-356)
# or 'name', a checked name; a checked name followed at once by `/REQ-ID` also carries that ID as a
# subtest citation (TTG-V0-960).
sub row_names {
    my ($line) = @_;
    my @names;
    while ($line =~ /\b(Test[A-Za-z0-9_]+)\b/g) {
        my $name = $1;
        # TTG-V0-011 (V1-3143): a selector such as `toolgroup.TestValidity` or `(*T).TestRun` names
        # a Go identifier of another package or type, never a cited test function, which a row
        # always names bare; it is not checked.
        my $start = $-[1];
        my $after = substr($line, pos($line));
        my $kind = $start >= 2 && substr($line, $start - 1, 1) eq '.' && substr($line, $start - 2, 1) =~ /[\w)]/
            ? 'selector'
            : $after =~ /^`?[ \t]*\(PLANNED\)/ ? 'planned'
            # TTG-V0-917 (V1-4048): a word of the test-name shape that names no test, such as a
            # fixture `TestLine`, is written with `(not a test)` right after it.
            : $after =~ /^`?[ \t]*\(not a test\)/ ? 'notatest'
            # TTG-V0-952: a live spec row that records a test removed with its code writes
            # `(removed)` right after the name.
            : $after =~ /^`?[ \t]*\(removed\)/ ? 'removed'
            : $after =~ /^\*/ ? 'wildcard'
            : 'name';
        my $sub = $kind eq 'name' && $after =~ m{^/($req_id)} ? $1 : undef;
        push @names, { name => $name, kind => $kind, sub => $sub };
    }
    return @names;
}

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

my (@missing, %reported, %subtests);
my $planned = 0;
my $wildcards = 0;
my ($retired, $removed) = (0, 0);
for my $spec_ref (@spec_refs) {
    my $at = $spec_ref eq '' ? '' : substr($spec_ref, 0, 12) . ' ';
    $retired += scan_rows($spec_ref, sub {
        my ($file, $line_number, $line) = @_;
        for my $cited (row_names($line)) {
            my ($name, $kind, $id) = @{$cited}{qw(name kind sub)};
            next if $kind eq 'selector' || $kind eq 'notatest';
            if ($kind eq 'planned') {
                ++$planned;
                next;
            }
            # TTG-V0-952: the marker is refused while the test is declared.
            if ($kind eq 'removed') {
                push @missing, "$at$file:$line_number: $name is marked (removed) but is declared; drop the marker"
                    if $tests{$name};
                ++$removed;
                next;
            }
            # DCW-V0-356 (V1-2855): a wildcard such as `TestX_*` is never expanded; the refusal says
            # so and names the declared functions it would have matched, so the row can list them.
            if ($kind eq 'wildcard') {
                my @matches = sort grep { index($_, $name) == 0 } keys %tests;
                my $shown = join(', ', @matches[0 .. ($#matches < 4 ? $#matches : 4)])
                    . (@matches > 5 ? ' and ' . (@matches - 5) . ' more' : '');
                push @missing, "$at$file:$line_number: $name* is a wildcard; name each test function in full ("
                    . (@matches ? scalar(@matches) . " declared: $shown" : 'no declared function has this prefix') . ')';
                ++$wildcards;
                next;
            }
            if ($tests{$name}) {
                # TTG-V0-961: a subtest citation also needs the ID in a body of the declared name;
                # each spec path and citation is counted and reported once.
                next unless defined $id;
                $subtests{"$file\0$name/$id"} = 1;
                next if body_has_id($head_tests, $name, $id);
                my $first = $reported{"$file\0$name/$id"} //= $spec_ref;
                push @missing, "$at$file:$line_number: $name/$id (no declaration of $name contains $id)"
                    if $first eq $spec_ref;
                next;
            }
            my $first = $reported{"$file\0$name"} //= $spec_ref;
            push @missing, "$at$file:$line_number: $name" . unstaged_hint($name) if $first eq $spec_ref;
        }
    });
}

# cited_requirements reads REF's tree for TTG-V0-962: a requirement is cited when a traceability row
# holds its ID in full in the first cell beside a checked test name that resolves in that tree, or
# when a resolving subtest citation names it. It also returns the IDs whose row states
# `(untested: REASON)` with a non-blank reason (TTG-V0-963) and the text of the specs not retired.
sub cited_requirements {
    my ($tests) = @_;
    my (%cited, %untested, $text);
    scan_rows($tests->{ref}, sub {
        my ($file, $line_number, $line) = @_;
        my ($cell) = $line =~ /^\s*\|([^|]*)/;
        my @ids = ($cell // '') =~ /(?<![A-Za-z0-9-])($req_id)/g;
        if ($line =~ /\(untested:([^)]*)\)/ && $1 =~ /\S/) {
            $untested{$_} = 1 for @ids;
        }
        for my $cited (row_names($line)) {
            my ($name, $kind, $id) = @{$cited}{qw(name kind sub)};
            next unless $kind eq 'name' && $tests->{decl}{$name};
            next if defined $id && !body_has_id($tests, $name, $id);
            $cited{$_} //= "$file:$line_number: $name" for @ids, defined $id ? $id : ();
        }
    }, sub { $text .= join '', @{ $_[1] } });
    return (\%cited, \%untested, $text // '');
}

# TTG-V0-962: a requirement cited at BASE and not at HEAD fails unless HEAD's specs no longer state
# it or its row states `(untested: REASON)` (TTG-V0-963).
my (@lost, $base_count, $untested_count);
if ($base_ref ne '') {
    my ($base_cited) = cited_requirements(load_tests($base_ref));
    my ($head_cited, $head_untested, $head_text) = cited_requirements($head_tests);
    my $at = substr($base_ref, 0, 12);
    ($base_count, $untested_count) = (scalar(keys %$base_cited), 0);
    for my $id (sort keys %$base_cited) {
        next if $head_cited->{$id};
        if ($head_untested->{$id}) {
            ++$untested_count;
            next;
        }
        next unless $head_text =~ /(?<![A-Za-z0-9-])\Q$id\E(?![A-Za-z0-9])/;
        push @lost, "$id: last test citation removed ($at $base_cited->{$id})";
    }
}

close_index();
print "traceability tests: $planned planned\n";
print "traceability tests: $removed marked removed; $retired retired spec(s) not checked\n" if $removed || $retired;
print 'traceability subtest citations: ' . scalar(keys %subtests) . " checked\n";
print "traceability last citations: $base_count requirements cited at BASE, $untested_count untested\n"
    if $base_ref ne '';
if (@lost) {
    print STDERR "traceability tests: requirements whose last test citation the range removed:\n";
    print STDERR "  $_\n" for @lost;
    print STDERR "traceability tests: keep a test that cites the requirement, or state the removal in its row "
        . "with (untested: REASON) (TTG-V0-962, TTG-V0-963)\n";
}
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
}
exit 1 if @missing || @lost;
PERL
