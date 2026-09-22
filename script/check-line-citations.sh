#!/bin/sh
set -eu

# Every backticked `path:line` citation in the DCG-V0-001 document set (docs/specs,
# docs/agent-memory, README.md, docs/DOGFOOD.md, docs/AGENT-ROUTES.md,
# docs/SELF-DEVELOPMENT.md, and conformance/*/README.md) must name a file that exists, a
# line inside it, and a line that carries something to cite. Specs are written against a
# working tree and committed later in the same change as the code they cite, so a citation
# can be stale the moment it lands: the 2026-09-05 audit repinned 330 of them across nine
# documents, none out of range, most pointing at a blank line, a lone brace, or the wrong
# function. This gate catches the three checkable failure shapes (missing file, line past
# the end, reversed range) and the strongest cheap drift signal (a citation that lands on a
# blank or bracket-only line).
#
# A citation carries a content anchor as `path:line@<hex>`, where <hex> is a prefix of
# the sha256 of the cited lines. The checks above are proxies for drift and cannot
# see the shape that audit found most often: a citation that still lands on a real line but
# no longer on the line it meant. An anchor makes that mechanical. Anchors are opt-in per
# citation in the legacy documents frozen in script/line-citation-legacy-documents.txt; every
# citation in a later document requires one. The committed unpinned count ceiling prevents the
# legacy debt from growing without requiring a mechanical bulk repin. Write an anchor with --hash:
#
#   script/check-line-citations.sh --hash internal/contextindex/index.go:473
#
# which prints the citation with its anchor appended. When an anchor stops matching, do not
# paste the new hash back unread: a matching anchor asserts the cited content is unchanged,
# not that it was ever the right content to cite, so the repair is to read the lines and
# decide whether the sentence still holds.
#
# Covered: `dir/file.ext:N`, `dir/file.ext:N-M`, `dir/file.ext:N,M`, an extensionless path
# the Git index tracks such as `Makefile:18`, a basename the Git index tracks at the
# repository root such as `ROADMAP.md:342`, and a bare `:N` or `:N-M` continuation, which is
# resolved to the last cited file in the same paragraph or, when a requirement id such as
# `CF-V0-014` intervened, to that id's spec from docs/specs/REQUIREMENTS.tsv. A root basename
# names exactly one path, so it is checked as that path even where docs/specs holds a file of
# the same name (`README.md`). Any other bare basename, such as `prove.go:767`, is not
# checked: its directory is a reading of the surrounding prose, not of the token.
# A token whose prefix is an exact tracked path followed by `:N` is citation-like even when
# the rest is malformed; the gate reports it instead of silently treating it as prose.
#
# Exempt: Go standard-library paths cited as `src/syscall/...` (GOROOT-relative, not this
# repository's `src/`), and anything inside a fenced code block.
#
# Every path this gate reads is resolved against the Git index, never the working
# filesystem: the scanned document set and the tracked set come from `git ls-files`, and
# each file's bytes from `:<path>` through `git cat-file --batch`. `make gate` must measure what a fresh
# clone contains (`Makefile:18`), and resolving with `-f $path` did not: a citation into a
# file that existed only on the author's disk passed, and a tracked file deleted from the
# worktree alone failed. The index rather than HEAD is the reference, so a file staged for
# addition in the same change as the document citing it resolves, which is the polarity
# `script/check-decision-numbers.sh` already uses. A dirty worktree therefore cannot change
# this gate's result in either direction; `git add` is what publishes a change to it.
#
# `docs/BUILD-LOG.md` and every `docs/agent-memory/*.md` backlog file are written
# newest-entry-on-top: prepending an entry shifts every existing line down, so a bare line
# citation into one of them (from any other document) is wrong the moment the next entry
# lands, and the three checks above cannot see it -- the old line number still exists, still
# resolves to a real line, and is rarely blank. A citation into one of these files is
# therefore REQUIRED to carry the `@<hash>` content anchor described above (`--hash
# docs/agent-memory/fixes.md:108-110`, say); an unpinned one is now a failure. This reuses
# the anchor check as the drift detector: the pin fails the next time the file is prepended,
# forcing a repin (and a read) instead of silently citing the wrong entry. Self-citation
# (a citation inside `docs/BUILD-LOG.md` pointing at `docs/BUILD-LOG.md` itself, or inside a
# given `docs/agent-memory/X.md` pointing at that same `X.md`) is exempt from this rule.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

perl - "$@" <<'PERL'
use strict;
use warnings;
use Digest::SHA qw(sha256_hex);
use IPC::Open2 qw(open2);

my %tracked;
{
    open my $paths, '-|', 'git', 'ls-files', '-z' or die "git ls-files: $!\n";
    local $/ = "\0";
    while (my $path = <$paths>) {
        chomp $path;
        $tracked{$path} = 1 if length $path;
    }
    close $paths
        or die $! ? "git ls-files: $!\n" : 'git ls-files failed: exit ' . ($? >> 8) . "\n";
}

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

my %lines_of;
sub lines_of {
    my ($path) = @_;
    return $lines_of{$path} if exists $lines_of{$path};
    return $lines_of{$path} = undef unless $tracked{$path};
    my @lines = split /^/, index_blob($path);
    chomp @lines;
    return $lines_of{$path} = \@lines;
}

my %spec_of_id;
if (my $rows = lines_of('docs/specs/REQUIREMENTS.tsv')) {
    for my $row (@$rows) {
        my ($id, $file) = split /\t/, $row;
        $spec_of_id{$id} = $file if defined $file && $id =~ /^[A-Z][A-Z0-9-]*-[0-9]{3}$/;
    }
}

sub trivial {
    my ($text) = @_;
    return 1 unless defined $text;
    $text =~ s/^\s+|\s+$//g;
    return $text eq '' || $text =~ /^[\}\)\]\,;]+$/;
}

# The anchored line set is what the citation reads as one span: a range covers its endpoints,
# and a comma list covers the listed lines in written order. Each line is right-trimmed, so
# indentation stays part of the anchor and trailing whitespace churn does not break it.
sub anchored_numbers {
    my ($first, $last, @extra) = @_;
    return defined $last ? ($first .. $last) : ($first, @extra);
}

# Returns the full digest. A pin is any prefix of it, so a citation may be pinned at the
# eight characters --hash prints or at full length, and both compare the same way.
sub anchor_of {
    my ($lines, @numbers) = @_;
    my @text = map { my $line = $lines->[$_ - 1]; $line =~ s/\s+$//; $line } @numbers;
    return sha256_hex(join("\n", @text));
}

# A prepended backlog document: every insertion shifts existing lines down, so a bare line
# number into one is not a stable citation. See the header comment for the reasoning.
sub prepended_doc {
    my ($path) = @_;
    return $path eq 'docs/BUILD-LOG.md' || $path =~ m{^docs/agent-memory/[^/]+\.md$};
}

sub scanned_doc {
    my ($path) = @_;
    return $path =~ m{^(?:docs/(?:specs|agent-memory|decisions)/[^/]+\.md|README\.md|docs/(?:DOGFOOD|AGENT-ROUTES|SELF-DEVELOPMENT)\.md|conformance/[^/]+/README\.md)$};
}

my @failures;
my %legacy_document;
my %drift_waiver;
my $unpinned_count = 0;
# This is a ratchet, not a target. Lowering it is free; raising it requires an explicit contract diff.
my $unpinned_ceiling = 941;

# A drift waiver names one citation whose content-sensitive checks (trivial-line, anchor
# mismatch) are exempted because this change is forbidden from touching either the citing
# document or the cited lines (a different in-flight lane, or an off-limits document, owns
# them). It never exempts the file-existence or line-range checks. See
# script/line-citation-drift-waivers.txt and DCG-V0-020.
sub drift_waiver_key {
    my ($doc, $path, $first, $last) = @_;
    return "$doc\t$path:$first" . (defined $last ? "-$last" : '');
}
if (my $rows = lines_of('script/line-citation-drift-waivers.txt')) {
    for my $row (@$rows) {
        next if $row eq '' || $row =~ /^#/;
        my ($doc, $spec) = split /\t/, $row;
        die "script/line-citation-drift-waivers.txt: malformed row: $row\n"
            unless defined $spec && $spec =~ /^[^:]+:\d+(?:-\d+)?$/;
        die "script/line-citation-drift-waivers.txt: duplicate row: $row\n"
            if $drift_waiver{"$doc\t$spec"}++;
    }
}
sub check_malformed {
    my ($doc, $line_number, $token) = @_;
    my $content = substr($token, 1, -1);
    my ($path, $suffix) = $content =~ m{\A(\/?[A-Za-z0-9_][A-Za-z0-9_.\/-]*\/[A-Za-z0-9_.-]+\.[a-z0-9]+|[A-Za-z0-9_.-]+\.[a-z0-9]+|[A-Za-z0-9_][A-Za-z0-9_-]*(?:\/[A-Za-z0-9_-]+)*):\d+(.*)\z}
        or return;
    return unless $tracked{$path};
    return if $suffix =~ /\A(?:(?:,\d+)*|-\d+)(?:\@[0-9a-f]{4,64})?\z/;
    push @failures, "$doc:$line_number  $token  malformed citation-like token: expected "
        . "<tracked-path>:N, :N-M, or :N,M[,M...] with an optional \@<4-64 lowercase hex> anchor";
}

sub check {
    my ($doc, $line_number, $token, $path, $pin, $first, $last, @extra) = @_;
    return if $path =~ m{^src/syscall/};
    if (defined $last && $last < $first) {
        push @failures, "$doc:$line_number  $token  range end $last is before its start $first";
        return;
    }
    if (!defined $pin) {
        ++$unpinned_count;
        if (!$legacy_document{$doc}) {
            push @failures, "$doc:$line_number  $token  citations in documents outside the legacy allowlist require an \@<hex> content anchor";
            return;
        }
    }
    if (!defined $pin && $doc ne $path && prepended_doc($path)) {
        push @failures, "$doc:$line_number  $token  $path is prepended (newest entry on top): "
            . "pin this citation with \@<hash> (script/check-line-citations.sh --hash $path:$first"
            . (defined $last ? "-$last" : '') . ") or cite a stable anchor instead of a line number";
        return;
    }
    my $lines = lines_of($path);
    if (!$lines) {
        push @failures, "$doc:$line_number  $token  no such file in the Git index: $path";
        return;
    }
    for my $number ($first, @extra, (defined $last ? $last : ())) {
        if ($number < 1) {
            push @failures, "$doc:$line_number  $token  line $number is before the start of $path (lines are 1-based)";
            return;
        }
        if ($number > @$lines) {
            push @failures, "$doc:$line_number  $token  line $number is past the end of $path (" . scalar(@$lines) . " lines)";
            return;
        }
    }
    my $waived = $drift_waiver{drift_waiver_key($doc, $path, $first, $last)};
    for my $number ($first, @extra) {
        push @failures, "$doc:$line_number  $token  $path:$number is a blank or bracket-only line"
            if trivial($lines->[$number - 1]) && !$waived;
    }
    return unless defined $pin;
    return if $waived;
    my $actual = anchor_of($lines, anchored_numbers($first, $last, @extra));
    push @failures, "$doc:$line_number  $token  cited content changed: anchor \@$pin, now \@" . substr($actual, 0, 8) . " (read the lines before repinning)"
        unless $actual =~ /^\Q$pin\E/;
}

# --hash prints one citation with its anchor appended, so an author can pin a citation
# without computing the digest by hand.
if (@ARGV) {
    my ($flag, @tokens) = @ARGV;
    die "usage: check-line-citations.sh [--hash <path:line[-line][,line]> ...]\n"
        unless $flag eq '--hash' && @tokens;
    for my $token (@tokens) {
        my ($path, $first, $extra, $last) = $token =~ /^([^:]+):(\d+)((?:,\d+)*)(?:-(\d+))?$/
            or die "not a citation: $token\n";
        my $lines = lines_of($path) or die "no such file in the Git index (git add it first): $path\n";
        die "range end $last is before its start $first: $token\n" if defined $last && $last < $first;
        my @extra = grep { length } split /,/, $extra;
        for my $number ($first, @extra, (defined $last ? $last : ())) {
            die "line $number is before the start of $path (lines are 1-based)\n" if $number < 1;
            die "line $number is past the end of $path (" . scalar(@$lines) . " lines)\n" if $number > @$lines;
        }
        print "$token\@" . substr(anchor_of($lines, anchored_numbers($first, $last, @extra)), 0, 8) . "\n";
    }
    close_index();
    exit 0;
}

my $legacy_rows = lines_of('script/line-citation-legacy-documents.txt')
    or die "script/line-citation-legacy-documents.txt: not readable from the Git index\n";
for my $path (@$legacy_rows) {
    next if $path eq '' || $path =~ /^#/;
    die "script/line-citation-legacy-documents.txt: invalid scanned-document path: $path\n"
        unless scanned_doc($path);
    die "script/line-citation-legacy-documents.txt: duplicate path: $path\n"
        if $legacy_document{$path}++;
}

for my $doc (sort grep { scanned_doc($_) } keys %tracked) {
    my $doc_lines = lines_of($doc) or die "$doc: not readable from the Git index\n";
    my $line_number = 0;
    my $fenced = 0;
    my $current;
    for my $doc_line (@$doc_lines) {
        my $line = $doc_line;
        ++$line_number;
        if ($line =~ /^\s*```/) { $fenced = !$fenced; next; }
        next if $fenced;
        if ($line !~ /\S/) { undef $current; next; }
        while ($line =~ /`[^`]*`/g) {
            check_malformed($doc, $line_number, $&);
        }
        # A continuation resolves only to an anchor the same paragraph states outright: the
        # last full-path citation, a sibling spec cited by bare name, or a requirement id
        # written immediately before the continuation (`CF-V0-014`, `:180-184`). A basename
        # the index tracks at the repository root, such as `ROADMAP.md:342`, is a full path and
        # is checked as one. Any other bare basename such as `prove.go:767`, or a path
        # mentioned without a line, ends the anchor: what follows is a reading of the prose,
        # and the gate does not guess.
        # The last alternative is the DCG-V0-015 extensionless form, `Makefile:18`. It has no extension
        # to key on, so it is matched broadly and then kept only when the Git index tracks
        # the path; `exit:1` and the like stay prose.
        while ($line =~ /`(?:(\/?[A-Za-z0-9_][A-Za-z0-9_.\/-]*\/[A-Za-z0-9_.-]+\.[a-z0-9]+):(\d+)((?:,\d+)*)(?:-(\d+))?(?:\@([0-9a-f]{4,64}))?|:(\d+)(?:-(\d+))?(?:\@([0-9a-f]{4,64}))?|([A-Z][A-Z0-9-]*-[0-9]{3})|([A-Za-z0-9_.-]+\.[a-z0-9]+):(\d[\d,-]*)(?:\@([0-9a-f]{4,64}))?|[A-Za-z0-9_][A-Za-z0-9_.\/-]*\/[A-Za-z0-9_.-]+\.[a-z0-9]+|([A-Za-z0-9_][A-Za-z0-9_-]*(?:\/[A-Za-z0-9_-]+)*):(\d+)((?:,\d+)*)(?:-(\d+))?(?:\@([0-9a-f]{4,64}))?)`/g) {
            my $token = $&;
            my $rest = substr($line, pos($line));
            if (defined $9) {
                my $spec = $spec_of_id{$9};
                $current = $spec if $spec && $spec ne $doc && $rest =~ /^(?:\([a-z]\))?,?\s*\(?`:\d/;
                next;
            }
            if (defined $10) {
                my ($name, $span, $pin) = ($10, $11, $12);
                if ($tracked{$name} && $span =~ /^(\d+)((?:,\d+)*)(?:-(\d+))?$/) {
                    $current = $name;
                    my @extra = grep { length } split /,/, $2;
                    check($doc, $line_number, $token, $name, $pin, $1, $3, @extra);
                    next;
                }
                $current = $name =~ /\.md$/ && $tracked{"docs/specs/$name"} ? "docs/specs/$name" : undef;
                next;
            }
            if (defined $13) {
                # An untracked token is prose, so it leaves the paragraph anchor exactly as
                # it was: before this alternative existed the regex did not match it at all.
                next unless $tracked{$13};
                $current = $13;
                my @extra = grep { length } split /,/, ($15 // '');
                check($doc, $line_number, $token, $13, $17, $14, $16, @extra);
                next;
            }
            if (!defined $1 && !defined $6) { undef $current; next; }
            if (defined $1) {
                $current = $1;
                my @extra = grep { length } split /,/, ($3 // '');
                check($doc, $line_number, $token, $1, $5, $2, $4, @extra);
                next;
            }
            next unless defined $current;
            check($doc, $line_number, $token, $current, $8, $6, $7);
        }
    }
}

push @failures, "unpinned citation count $unpinned_count exceeds committed ceiling $unpinned_ceiling; read and pin citations or lower the ceiling"
    if $unpinned_count > $unpinned_ceiling;
close_index();
if (@failures) {
    print STDERR "line citations: " . scalar(@failures) . " citation(s) do not resolve:\n";
    print STDERR "  $_\n" for @failures;
    exit 1;
}
PERL
