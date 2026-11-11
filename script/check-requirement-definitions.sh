#!/bin/sh
set -eu

# Every requirement ID in docs/specs/REQUIREMENTS.tsv must have a definition line in
# some spec body. The generator falls back to a table row when no definition exists,
# so a clause body deleted by accident keeps its row and every other gate stays green:
# that is how commit dfd02ea removed eighteen GPK clause bodies without failing a gate.
#
# CEM-CB-* is exempt. docs/specs/cem-0.2-canonical-binding.md states that family in
# tables by design, so those IDs have no definition line and never had one.
#
# Definition grammar (RDG-V0-004): after optional leading whitespace, an optional `-`/`*`
# bullet and optional `**`, the ID (backticks balanced) is followed by `:` or by a single `.`.
# A `..` is a prose range such as `AMAP-V0-024..026 are`, never a definition.
# An unbulleted line that directly follows a non-blank, non-heading line outside a fence is a
# wrapped continuation, so an id starting it is a citation, never a definition (RDG-V0-023).
#
# Outside-section ratchet (RDG-V0-021): the OCM intent reader enumerates only the one
# `## Requirements` section, so an OCM-form definition (- `ID`: , - **ID:** , - **ID.** )
# placed after that section falls out of every OCM. Such a definition must be listed in
# script/requirement-definitions-outside-section.txt, a frozen baseline that may only shrink.
#
# Cited-id scan (RDG-V0-9980, V1-5324): every id of the same shape cited in a tracked Go file, a
# Makefile or AGENTS.md (testdata/, vendor/, docs/build-log/, docs/decisions/ and generated Go
# files skipped) must have a definition in some spec. A definition in a retired spec (header
# `Delivery status: retired`) counts as defined; those ids are only counted, as information. A
# cited id ends at its first three-digit group. An undefined cited id fails unless it is listed in
# script/requirement-citations-undefined.txt, a frozen baseline that may only shrink, or in
# script/requirement-citations-allowlist.tsv, the reviewed `id<TAB>reason` list of tokens that
# are not requirement ids (a checksum name, a standard, a test fixture id). A baseline line that is
# defined, no longer cited or allowlisted fails as stale; an allowlist line with no reason fails,
# and one whose id is defined or no longer cited fails as stale. CEM-CB-* stays exempt, as above.
# Four-digit ids are outside the id shape, so this scan does not see them.

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

# RDG-V0-9980: the index files whose requirement citations are read. The scan is bounded to tracked
# Go files, Makefiles and the root AGENTS.md, never fixture, vendored or record directories.
sub tracked_citing_files {
    open my $ls, '-|', 'git', 'ls-files', '-z', '--cached'
        or die "git ls-files: $!\n";
    local $/ = "\0";
    my @files;
    while (my $path = <$ls>) {
        chomp $path;
        next unless $path eq 'AGENTS.md' || $path =~ m{(?:\A|/)Makefile\z} || $path =~ /\.go\z/;
        next if $path =~ m{(?:\A|/)(?:testdata|vendor)/} || $path =~ m{\Adocs/(?:build-log|decisions)/};
        push @files, $path;
    }
    close $ls
        or die $! ? "git ls-files: $!\n" : 'git ls-files failed: exit ' . ($? >> 8) . "\n";
    return sort @files;
}

sub index_lines {
    return split /^/, index_blob($_[0]);
}

my (%defined, %live, %outside, @misshapen);
for my $file (tracked_spec_docs()) {
    next if $file eq 'docs/specs/README.md';
    my $line_number = 0;
    my @lines = index_lines($file);
    # RDG-V0-9980: a spec whose header says `Delivery status: retired` is a record of removed code
    # (the TTG-V0-951 rule), so its definitions are not live for the cited-id scan.
    my $last = $#lines < 39 ? $#lines : 39;
    my $is_live = !grep { /^(?:- )?\**Delivery status:\**\s*retired\b/ } @lines[0 .. $last];
    # A fence is tracked as OCM's fencedLines does (internal/lrfrepo/ocm.go): a run of at least
    # three backticks or tildes indented at most three spaces opens one (a backtick fence's info
    # string holds no backtick), and only a run of the same character at least as long, followed
    # by nothing but spaces or tabs, closes it.
    my ($fence, $in_section, $sections, $rollback) = ('', 0, 0, 0);
    my @outside;
    my $continues = 0;
    for my $line (@lines) {
        ++$line_number;
        # RDG-V0-023: an unbulleted line that directly follows a non-blank, non-heading line
        # outside a fence continues that paragraph or list item, so an id that starts it (a
        # wrapped citation such as `ID`. ending a sentence) is never a definition.
        my $wrapped = $continues && $line !~ /^\s*[-*]\s/;
        my ($run, $rest) = $line =~ /^ {0,3}(`{3,}|~{3,})(.*?)\r?\n?\z/ ? ($1, $2) : ('', '');
        if ($fence ne '') {
            $fence = '' if $run ne '' && substr($run, 0, 1) eq substr($fence, 0, 1)
                && length($run) >= length($fence) && $rest =~ /\A[ \t]*\z/;
        } elsif ($run ne '' && !($run =~ /^`/ && $rest =~ /`/)) {
            $fence = $run;
        } elsif ($line =~ /^## Requirements\r?\n?\z/) {
            ($in_section, $sections) = (1, $sections + 1);
        } elsif ($line =~ /^##(?: |\r?\n?\z)/) {
            $in_section = 0;
        } elsif (!$in_section && !$rollback
            && $line =~ /^- (?:`([A-Z][A-Z0-9-]*-[0-9]{3})`: |\*\*([A-Z][A-Z0-9-]*-[0-9]{3})[:.]\*\* )/) {
            push @outside, "$file " . ($1 // $2);
        }
        $continues = $fence eq '' && $run eq '' && $line =~ /\S/ && $line !~ /^ {0,3}#/;
        # SRG-V0-966 (V1-3660): a heading whose text starts with Rollback opens a rollback section
        # (any level from ##), which the next heading ends; the generator tracks it the same way.
        $rollback = $1 =~ /^Rollback\b/i ? 1 : 0 if $fence eq '' && $run eq '' && $line =~ /^ {0,3}#{2,6}\s+(.*)/;
        # RDG-V0-022: a definition-shaped line with a parenthetical between the id and its colon,
        # such as - `ID` (V1-x): text, is no definition under the grammar above, so an id with no
        # other definition would silently fall back to a table row or vanish. It fails, naming the
        # accepted forms. A traceability line for an id defined elsewhere keeps passing.
        if ($line =~ /^\s*(?:[-*]\s+)?(?:\*\*)?(`?)([A-Z][A-Z0-9-]*-[0-9]{3})\1(?:\*\*)?\s*\((?:[^()]|\([^()]*\))*\)(?:\*\*)?\s*:/) {
            push @misshapen, [$2, "$file:$line_number $2"];
        }
        # The backticks must balance. With both optional and independent, an opening
        # backtick matched while the closing one did not, so a prose range like
        # `PUB-V0-011..015`. read as a second definition of PUB-V0-011. The
        # backreference makes the closing backtick required exactly when one opened.
        next if $wrapped;
        # SRG-V0-966 (V1-3660): an ID restated under a Rollback heading is no definition.
        next if $rollback;
        next unless $line =~ /^\s*(?:[-*]\s+)?(?:\*\*)?(`?)([A-Z][A-Z0-9-]*-[0-9]{3})\1(?:\*\*)?(?:\s*:|\.(?!\.))/;
        push @{ $defined{$2} }, "$file:$line_number";
        $live{$2} = 1 if $is_live;
    }
    # Only a spec with exactly one Requirements section is an OCM intent; others refuse anyway.
    $outside{$_} = 1 for $sections == 1 ? @outside : ();
}

my %baseline;
if (system("git cat-file -e :script/requirement-definitions-outside-section.txt 2>/dev/null") == 0) {
    for my $row (split /^/, index_blob('script/requirement-definitions-outside-section.txt')) {
        chomp $row;
        next if $row =~ /^\s*(?:#|\z)/;
        $baseline{$row} = 1;
    }
}
my @new_outside = grep { !$baseline{$_} } sort keys %outside;
my @stale_baseline = grep { !$outside{$_} } sort keys %baseline;

# RDG-V0-9980: the first citing path:line of every id cited in scope. The id must not continue a
# longer token on either side, so a four-digit id is never read as its three-digit prefix, and it
# ends at its first three-digit group: no segment before that group is three or more digits, so
# `ABC-V0-001..003`, `ABC-V0-001-002` and `ABC-V0-001/002` each read as `ABC-V0-001` alone.
my %cited;
for my $file (tracked_citing_files()) {
    my $blob = index_blob($file);
    next if $file =~ /\.go\z/ && $blob =~ m{\A// Code generated .* DO NOT EDIT\.\r?\n};
    while ($blob =~ /(?<![A-Za-z0-9-])([A-Z][A-Z0-9]*(?:-(?![0-9]{3,}(?![A-Z0-9]))[A-Z0-9]+)*-[0-9]{3})(?![0-9])/g) {
        my $id = $1;
        next if $cited{$id} || $id =~ /^CEM-CB-/;
        my $line_number = 1 + (substr($blob, 0, $-[0]) =~ tr/\n//);
        $cited{$id} = "$file:$line_number";
    }
}
my %citation_baseline;
if (system("git cat-file -e :script/requirement-citations-undefined.txt 2>/dev/null") == 0) {
    for my $row (split /^/, index_blob('script/requirement-citations-undefined.txt')) {
        chomp $row;
        next if $row =~ /^\s*(?:#|\z)/;
        $citation_baseline{$row} = 1;
    }
}
# The allowlist names cited tokens that are not requirement ids, each with the reviewed reason.
my (%allowlist, @reasonless);
if (system("git cat-file -e :script/requirement-citations-allowlist.tsv 2>/dev/null") == 0) {
    my $row_number = 0;
    for my $row (split /^/, index_blob('script/requirement-citations-allowlist.tsv')) {
        ++$row_number;
        chomp $row;
        next if $row =~ /^\s*(?:#|\z)/;
        my ($id, $reason) = $row =~ /\A([^\t]+)\t(.*)\z/ ? ($1, $2) : ($row, '');
        if ($reason !~ /\S/) {
            # The file name is a separate literal: joined with the interpolation it reads as a
            # constructed path under script/ to the shell caller completeness scan (AZU-V0-8901).
            push @reasonless, 'script/requirement-citations-allowlist.tsv:' . "$row_number $id";
            next;
        }
        $allowlist{$id} = 1;
    }
}
my @undefined_cited = grep { !$defined{$_} && !$citation_baseline{$_} && !$allowlist{$_} } sort keys %cited;
my @stale_citation_baseline = grep { $defined{$_} || !$cited{$_} || $allowlist{$_} } sort keys %citation_baseline;
my @stale_allowlist = grep { $defined{$_} || !$cited{$_} } sort keys %allowlist;
my $retired_only = grep { $defined{$_} && !$live{$_} } keys %cited;

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
    # RDG-V0-741 (V1-3699): REQUIREMENTS.tsv is generated from the staged specs.
    print STDERR "  if the spec defines them now, stage the spec, then run make spec-requirements to regenerate "
        . "docs/specs/REQUIREMENTS.tsv; otherwise restore their definitions (RDG-V0-741)\n";
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
if (@new_outside) {
    $failed = 1;
    print STDERR "requirement definitions: defined outside the spec's ## Requirements section, so OCM cannot enumerate them; move each into that section:\n";
    print STDERR "  $_\n" for @new_outside;
}
if (@stale_baseline) {
    $failed = 1;
    print STDERR "requirement definitions: no longer outside ## Requirements; delete from script/requirement-definitions-outside-section.txt:\n";
    print STDERR "  $_\n" for @stale_baseline;
}
if (@undefined_cited) {
    $failed = 1;
    print STDERR "requirement definitions: these ids are cited in Go code, a Makefile or AGENTS.md but no spec defines them; "
        . "for each, define the requirement in its spec's ## Requirements section, or fix the citation to the id that is defined, "
        . "or, only for a token that is not a requirement id, add an `id<TAB>reason` line to "
        . "script/requirement-citations-allowlist.tsv (RDG-V0-9980):\n";
    print STDERR "  $_ $cited{$_}\n" for @undefined_cited;
}
if (@stale_citation_baseline) {
    $failed = 1;
    print STDERR "requirement definitions: these script/requirement-citations-undefined.txt lines are stale, "
        . "now defined in a spec, no longer cited or allowlisted; delete them (RDG-V0-9980):\n";
    print STDERR "  $_\n" for @stale_citation_baseline;
}
if (@reasonless) {
    $failed = 1;
    print STDERR "requirement definitions: these script/requirement-citations-allowlist.tsv lines give no reason; "
        . "write `id<TAB>reason` saying why the token is not a requirement id (RDG-V0-9980):\n";
    print STDERR "  $_\n" for @reasonless;
}
if (@stale_allowlist) {
    $failed = 1;
    print STDERR "requirement definitions: these script/requirement-citations-allowlist.tsv lines are stale, "
        . "now defined in a spec or no longer cited; delete them (RDG-V0-9980):\n";
    print STDERR "  $_\n" for @stale_allowlist;
}
print "requirement definitions: $retired_only cited id(s) are defined only in a retired spec (RDG-V0-9980, information only)\n"
    if $retired_only;
@misshapen = map { $_->[1] } grep { !$defined{$_->[0]} } @misshapen;
if (@misshapen) {
    $failed = 1;
    print STDERR "requirement definitions: these lines look like definitions but put a note between the id and its colon, so no definition is read; write `- `ID`: (note) text`, `- **ID:** (note) text` or `- **ID.** (note) text` (docs/SPEC-DRIVEN-DEVELOPMENT.md):\n";
    print STDERR "  $_\n" for @misshapen;
}
exit($failed);
PERL
