#!/bin/sh
set -eu

# Every backticked `path:line` citation in the DCG-V0-001 document set (docs/specs,
# docs/agent-memory, README.md, docs/DOGFOOD.md, docs/AGENT-ROUTES.md, docs/SELF-DEVELOPMENT.md,
# docs/SPEC-TOOLCHAIN-INTEGRATION.md, docs/RELEASE-NOTES-*.md, and conformance/*/README.md) must name a file that exists, a
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
# citation in a later document requires one. The committed unpinned count ceiling and the
# per-citation inventory in script/line-citation-unpinned-inventory.txt prevent the legacy debt
# from growing or drifting without requiring a mechanical bulk repin. Write an anchor with --hash:
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
# repository's `src/`), and anything inside a fenced code block. Out of scope (DCG-V0-922): a
# `path:N` in a code comment (Go, shell or any other source file), since only the document set
# above is scanned, and whether an unpinned legacy citation still names the line it meant.
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
# `docs/BUILD-LOG.md` was appended to, apart from a few entries inserted at its top, and decision
# 0423 closes it to new entries, which go to write-once `docs/build-log/` files; every
# `docs/agent-memory/*.md` backlog lists entries newest first and removes them when done. A bare
# line citation into one of them (from any other document) names a position, not an entry, and
# the three checks above cannot see an in-place edit, a removal or an out-of-order insertion move
# the text under it. A citation into one of these files is therefore REQUIRED to carry the
# `@<hash>` content anchor described above (`--hash docs/agent-memory/fixes.md:108-110`, say); an
# unpinned one is a failure. The pin binds the citation to the lines it cites, so any later
# change to them fails it, forcing a repin (and a read) instead of silently citing the wrong text.
# Self-citation (a citation inside `docs/BUILD-LOG.md` pointing at `docs/BUILD-LOG.md` itself, or
# inside a given `docs/agent-memory/X.md` pointing at that same `X.md`) is exempt from this rule.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
# DCG-V0-048: --repin reruns this script after staging what it rewrote.
CHECK_LINE_CITATIONS_SELF=$(CDPATH='' cd -- "$(dirname "$0")" && pwd)/${0##*/}
export CHECK_LINE_CITATIONS_SELF
cd "$root"

# The program arrives on standard input, so the caller's standard input (the --show-changed
# answers, DCG-V0-630) is passed on descriptor 3.
perl - "$@" 3<&0 <<'PERL'
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
    my ($path, $object) = @_;
    $object //= ":$path";
    open my $fh, '-|', 'git', 'cat-file', 'blob', $object or die "$path: $!\n";
    my $blob = do { local $/; <$fh> } // '';
    close $fh
        or die $! ? "git cat-file $object: $!\n" : "git cat-file $object failed: exit " . ($? >> 8) . "\n";
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

# DCG-V0-043 (V1-2152): a path with unmerged index entries has no stage-0 blob to read. Every
# read of one refuses with the unmerged stages named, instead of the bare `git cat-file` exit.
my %unmerged;
{
    open my $entries, '-|', 'git', 'ls-files', '-u', '-z' or die "git ls-files -u: $!\n";
    local $/ = "\0";
    while (my $entry = <$entries>) {
        chomp $entry;
        my ($stage, $path) = $entry =~ /\A\d+ [0-9a-f]+ (\d)\t(.*)\z/s or next;
        push @{ $unmerged{$path} }, $stage;
    }
    close $entries
        or die $! ? "git ls-files -u: $!\n" : 'git ls-files -u failed: exit ' . ($? >> 8) . "\n";
}

my %lines_of;
sub lines_of {
    my ($path) = @_;
    return $lines_of{$path} if exists $lines_of{$path};
    return $lines_of{$path} = undef unless $tracked{$path};
    die "$path is unmerged in the Git index (stages " . join(',', @{ $unmerged{$path} })
        . ", no stage 0): resolve the conflict and git add it first\n"
        if $unmerged{$path};
    my @lines = split /^/, index_blob($path);
    chomp @lines;
    return $lines_of{$path} = \@lines;
}

# DCG-V0-800 (V1-3384): a decision record is written once (decision 0423 and every lane charter
# forbid editing one), so a citation in it names the code as it stood when the record was written.
# Each docs/decisions document whose index copy equals its blob in the newest commit that wrote it
# resolves its citations against that commit's tree, never the moving index: an output-neutral
# edit of a cited file then needs no repin of a record nobody may edit. A decision this change
# writes or edits, one no commit has written yet, and a clone whose history lacks the commit keep
# the index, as before.
my %written_at;
my $written_scanned;
sub decision_commit {
    my ($doc) = @_;
    return undef unless $doc =~ m{\Adocs/decisions/[^/]+\.md\z};
    unless ($written_scanned) {
        $written_scanned = 1;
        my %index_blob;
        if (open my $staged, '-|', 'git', 'ls-files', '-s', '-z', '--', 'docs/decisions') {
            local $/ = "\0";
            while (my $entry = <$staged>) {
                chomp $entry;
                my ($blob, $path) = $entry =~ /\A\d+ ([0-9a-f]+) 0\t(.*)\z/s or next;
                $index_blob{$path} = $blob;
            }
            close $staged;
        }
        if (open my $log, '-|', 'git', 'log', '--no-renames', '--format=@%H', '--raw', '--no-abbrev',
            '--diff-filter=AM', '--', 'docs/decisions') {
            local $/ = "\n";
            my ($commit, %seen);
            while (my $line = <$log>) {
                chomp $line;
                if ($line =~ /\A\@([0-9a-f]+)\z/) { $commit = $1; next; }
                my ($blob, $path) = $line =~ /\A:\d+ \d+ [0-9a-f]+ ([0-9a-f]+) [AM]\t(.*)\z/ or next;
                next if $seen{$path}++;
                $written_at{$path} = $commit if defined $commit && ($index_blob{$path} // '') eq $blob;
            }
            close $log;
        }
    }
    return $written_at{$doc};
}

# lines_at returns PATH's lines in COMMIT's tree, or undef when that tree has no such blob.
my %lines_at;
sub lines_at {
    my ($commit, $path) = @_;
    my $key = "$commit:$path";
    return $lines_at{$key} if exists $lines_at{$key};
    return $lines_at{$key} = undef if $path =~ /\n/;
    open my $fh, '-|', 'git', 'cat-file', '-p', $key or return $lines_at{$key} = undef;
    binmode $fh;
    my $blob = do { local $/; <$fh> };
    return $lines_at{$key} = undef unless close $fh;
    my @lines = split /^/, $blob // '';
    chomp @lines;
    return $lines_at{$key} = \@lines;
}

# lines_for is the content DOC's citation of PATH is checked against (DCG-V0-800).
sub lines_for {
    my ($doc, $path) = @_;
    my $commit = decision_commit($doc);
    return lines_of($path) unless defined $commit && $tracked{$path};
    return lines_at($commit, $path) // lines_of($path);
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

# The span text of a citation, as --hash takes it: N, N-M or N,M[,M...].
sub anchored_span {
    my ($first, $last, @extra) = @_;
    return join(',', $first, @extra) . (defined $last ? "-$last" : '');
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

# A log or backlog document: a bare line number into one names a position, not an entry, so
# a citation into it must be pinned. See the header comment for the reasoning.
sub log_or_backlog_doc {
    my ($path) = @_;
    return $path eq 'docs/BUILD-LOG.md' || $path =~ m{^docs/agent-memory/[^/]+\.md$};
}

sub scanned_doc {
    my ($path) = @_;
    return $path =~ m{^(?:docs/(?:specs|agent-memory|decisions)/[^/]+\.md|README\.md|docs/(?:DOGFOOD|AGENT-ROUTES|GLOSSARY|AUTOMATION|SELF-DEVELOPMENT|SPEC-TOOLCHAIN-INTEGRATION|RELEASE-NOTES-[^/]*)\.md|conformance/[^/]+/README\.md)$};
}

my @failures;
my @repins;
# DCG-V0-045: under --repin, the commit whose diff to the index proves a whole-span move, and the
# changed citations whose span that diff maps to new lines (suggested, never written).
my ($repin_base, $repin_base_label, @suggestions);
# DCG-V0-920/921: --repin --changed, and the files --selects REV names as changed.
my ($repin_changed, %selects_changed);
# DCG-V0-630: --repin --show-changed, and the changed spans it shows for a confirmation.
my ($repin_show, @show_changed);
# DCG-V0-921: the pinned citations into a file --selects REV names as changed.
my @selected;
# DCG-V0-920: the changed spans --repin --changed rewrote, printed for a read.
my @changed_repins;
# DCG-V0-046: root-basename skips are counted, and listed only under --verbose.
my @skipped;
# The byte offset of the token being checked in its document line, so --repin rewrites exactly
# that occurrence and never another identical token on the same line.
my $token_at;
my %legacy_document;
my (%unpinned_inventory, %unpinned_seen);
my %drift_waiver;
my $unpinned_count = 0;
# This is a ratchet, not a target. Lowering it is free; raising it requires an explicit contract diff.
my $unpinned_ceiling = 941;
# DCG-V0-044: the non-whitespace length a lone substantive line needs before a unique match moves it.
my $distinctive_line = 20;

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
    # DCG-V0-921: selected before the target is resolved, so a deleted cited file still selects.
    push @selected, "$doc:$line_number  $token" if defined $pin && $selects_changed{$path};
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
    if (!defined $pin && $doc ne $path && log_or_backlog_doc($path)) {
        push @failures, "$doc:$line_number  $token  $path is a log or backlog file: "
            . "pin this citation with \@<hash> (script/check-line-citations.sh --hash $path:$first"
            . (defined $last ? "-$last" : '') . ") or cite a stable anchor instead of a line number";
        return;
    }
    my $lines = lines_for($doc, $path);
    if (!$lines) {
        push @failures, "$doc:$line_number  $token  no such file in the Git index: $path";
        return;
    }
    my $waived = $drift_waiver{drift_waiver_key($doc, $path, $first, $last)};
    # DCG-V0-022: a pin whose text moved unchanged is reported as a move before any range or
    # blank-line finding, since those describe the old position, not the cited text.
    if (defined $pin && !$waived) {
        my @numbers = anchored_numbers($first, $last, @extra);
        my $in_range = !grep { $_ < 1 || $_ > @$lines } @numbers;
        if (!$in_range || anchor_of($lines, @numbers) !~ /^\Q$pin\E/) {
            # DCG-V0-920: a span the diff from the base deletes is never moved, even when an identical
            # block survives elsewhere; the deletion guard runs before the hash relocation.
            my $deleted = defined $repin_base && diff_deletes($path, $pin, undef, @numbers);
            my @shifts = $deleted ? () : relocations($lines, $pin, @numbers);
            my $shift = @shifts == 1 && relocatable($lines, $shifts[0], @numbers) ? $shifts[0] : undef;
            $shift //= diff_relocation($path, $lines, $pin, @numbers) if defined $repin_base && !$deleted;
            $shift //= history_relocation($path, $lines, $pin, @numbers) if defined $repin_base && !$deleted;
            if (defined $shift) {
                my $moved = shifted_token($token, $shift);
                # DCG-V0-841 (V1-3671): the plain check has no base, so a surviving copy of lines the
                # index deleted since HEAD looks like a move; --repin refuses that case, so the hint
                # names the recovery instead of --repin.
                my $head = defined $repin_base_label ? undef : head_commit();
                if (defined $head && diff_deletes($path, $pin, $head, @numbers)) {
                    my ($span) = $moved =~ /^`?([^`\@]+)/;
                    push @failures, "$doc:$line_number  $token  cited lines were deleted since HEAD; an identical copy "
                        . "survives at $moved, which --repin refuses to move to: restore the deleted lines, or cite the "
                        . "surviving copy with a new anchor from `script/check-line-citations.sh --hash $span`";
                    return;
                }
                push @failures, "$doc:$line_number  $token  cited content moved unchanged to $moved "
                    . "(script/check-line-citations.sh --repin rewrites it)";
                push @repins, [$doc, $line_number, $token, $moved, $token_at];
                return;
            }
            # DCG-V0-047 (V1-3124): a span whose lines changed only in the citation tokens they
            # carry (an earlier --repin of the cited document) is re-anchored, so a transitive
            # chain resolves by staging and rerunning --repin, never by a hand-run --hash.
            my $anchor = defined $repin_base && $in_range ? token_reanchor($path, $lines, $pin, @numbers) : undef;
            if (defined $anchor) {
                (my $reanchored = $token) =~ s{\@\Q$pin\E`\z}{'@' . substr($anchor, 0, length $pin) . '`'}e;
                push @failures, "$doc:$line_number  $token  cited lines changed only in their citation tokens; "
                    . "re-anchored as $reanchored (script/check-line-citations.sh --repin rewrites it)";
                push @repins, [$doc, $line_number, $token, $reanchored, $token_at];
                return;
            }
            if (defined $repin_base) {
                my ($suggested, $old_numbers, $new_numbers) = diff_suggestion($token, $path, $lines, $pin, $first, $last, @extra);
                # DCG-V0-630: --show-changed asks, on the first pass only, before writing it.
                if (defined $suggested && $repin_show && ($ENV{CHECK_LINE_CITATIONS_REPIN_PASS} // 1) eq '1') {
                    push @show_changed, [$doc, $line_number, $token, $suggested, $token_at, $path, $lines, $old_numbers, $new_numbers];
                    return;
                }
                # DCG-V0-920 (V1-3342, V1-3413): --repin --changed writes the diff's mapping and the
                # new anchor too, each listed for a read; without --changed it is only suggested.
                if (defined $suggested && $repin_changed) {
                    push @failures, "$doc:$line_number  $token  cited content changed; --repin --changed re-anchors it as $suggested "
                        . "(script/check-line-citations.sh --repin rewrites it)";
                    push @repins, [$doc, $line_number, $token, $suggested, $token_at];
                    push @changed_repins, [$doc, $line_number, $token, $suggested];
                    return;
                }
                push @suggestions, [$doc, $line_number, $token, $suggested] if defined $suggested;
            }
        }
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
    for my $number ($first, @extra) {
        push @failures, "$doc:$line_number  $token  $path:$number is a blank or bracket-only line"
            if trivial($lines->[$number - 1]) && !$waived;
    }
    if (!defined $pin) {
        # DCG-V0-041: a legacy unpinned citation passes only while it matches an inventory row; it
        # is reached only once the citation resolves, so a missing file or line is reported as such.
        my $span = join(',', $first, @extra) . (defined $last ? "-$last" : '');
        my $key = "$doc\t$path:$span";
        if (++$unpinned_seen{$key} > ($unpinned_inventory{$key} // 0)) {
            push @failures, "$doc:$line_number  $token  unpinned citation is new or edited (not in "
                . "script/line-citation-unpinned-inventory.txt): read $path:$span and pin it with "
                . "script/check-line-citations.sh --hash $path:$span";
        }
        return;
    }
    return if $waived;
    my @numbers = anchored_numbers($first, $last, @extra);
    my $actual = anchor_of($lines, @numbers);
    return if $actual =~ /^\Q$pin\E/;
    my @shifts = relocations($lines, $pin, @numbers);
    my $why = '';
    if (@shifts > 1) {
        $why = '; the anchored text now appears at ' . scalar(@shifts) . ' positions ('
            . candidates($token, @shifts) . '), so it is not relocated';
    } elsif (@shifts == 1) {
        $why = '; the anchored text appears once at ' . shifted_token($token, $shifts[0])
            . ', but a span with fewer than two non-blank lines is not relocated unless its one line '
            . "holds at least $distinctive_line non-whitespace characters";
    }
    push @failures, "$doc:$line_number  $token  cited content changed: anchor \@$pin, now \@" . substr($actual, 0, 8)
        . $why
        . " (read the lines before repinning; when only citation tokens in them changed, "
        . "script/check-line-citations.sh --repin re-anchors it, DCG-V0-047; when the sentence still holds, "
        . "print the new anchor with script/check-line-citations.sh --hash $path:" . anchored_span($first, $last, @extra)
        . ", or let script/check-line-citations.sh --repin --changed --base <rev> rewrite every such span the diff "
        . "from <rev> maps, DCG-V0-920)";
}

# DCG-V0-022: every whole-span shift under which the cited lines hash to the pin again. A pin
# shorter than eight hex characters is never relocated: at 16 bits a chance match is likely.
sub relocations {
    my ($lines, $pin, @numbers) = @_;
    return () if length($pin) < 8;
    my ($low, $high) = (sort { $a <=> $b } @numbers)[0, -1];
    my @shifts;
    for my $shift (1 - $low .. @$lines - $high) {
        next if $shift == 0;
        push @shifts, $shift if anchor_of($lines, map { $_ + $shift } @numbers) =~ /^\Q$pin\E/;
    }
    return @shifts;
}

# A unique hash match alone does not prove a move: when a cited `return nil` changes, another
# `return nil` elsewhere becomes the only match. A relocation also needs the matched span to hold
# at least two lines that are neither blank nor bracket-only, or (DCG-V0-044) exactly one such
# line that is distinctive: at least $distinctive_line non-whitespace characters, so one common
# short line never qualifies.
sub distinctive {
    my ($text) = @_;
    return 0 if trivial($text);
    (my $dense = $text) =~ s/\s+//g;
    return length($dense) >= $distinctive_line;
}
sub relocatable {
    my ($lines, $shift, @numbers) = @_;
    my %seen;
    my @substantive = grep { !$seen{$_}++ && !trivial($lines->[$_ + $shift - 1]) } @numbers;
    return 1 if @substantive >= 2;
    return @substantive == 1 && distinctive($lines->[$substantive[0] + $shift - 1]);
}

# DCW-V0-354: the positions an unrelocated anchor matches, named so a reader can check them.
sub candidates {
    my ($token, @shifts) = @_;
    my @named = map { shifted_token($token, $_) } @shifts[0 .. ($#shifts < 4 ? $#shifts : 4)];
    return join(', ', @named) . (@shifts > 5 ? ' and ' . (@shifts - 5) . ' more' : '');
}

# DCG-V0-045: the -U0 hunks of `git diff --cached <base> -- <path>`, each as [old start, old count,
# new start, new count], so a line the diff leaves unchanged maps to its index line exactly. The
# diff is the move evidence the hash search lacks when the cited text repeats or is one short line.
my (%hunks_of, %base_lines_of);
sub base_hunks {
    my ($path, $commit) = @_;
    $commit //= $repin_base;
    my $key = "$commit\0$path";
    return $hunks_of{$key} if exists $hunks_of{$key};
    return $hunks_of{$key} = undef if $path =~ /\n/;
    local $ENV{GIT_OPTIONAL_LOCKS} = '0';
    local $/ = "\n";
    my @hunks;
    open my $diff, '-|', 'git', 'diff', '--cached', '--no-ext-diff', '--no-textconv', '--no-color',
        '--no-renames', '-U0', $commit, '--', $path or die "git diff $commit -- $path: $!\n";
    while (my $row = <$diff>) {
        push @hunks, [$1, $2 // 1, $3, $4 // 1] if $row =~ /^\@\@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? \@\@/;
    }
    close $diff
        or die $! ? "git diff $commit -- $path: $!\n" : "git diff $commit -- $path failed: exit " . ($? >> 8) . "\n";
    return $hunks_of{$key} = \@hunks;
}

# The cited file's lines at the base commit, or undef when the base does not hold it as a blob.
sub base_lines {
    my ($path, $commit) = @_;
    $commit //= $repin_base;
    my $key = "$commit\0$path";
    return $base_lines_of{$key} if exists $base_lines_of{$key};
    return $base_lines_of{$key} = undef if $path =~ /\n/;
    local $/ = "\n";
    my $entry = do {
        local $/;
        open my $tree, '-|', 'git', 'ls-tree', '-z', $commit, '--', $path or die "git ls-tree: $!\n";
        my $listed = <$tree> // '';
        close $tree or die "git ls-tree $commit -- $path failed\n";
        $listed;
    };
    return $base_lines_of{$key} = undef unless $entry =~ /\A\d+ blob [0-9a-f]+\t/;
    my @lines = split /^/, index_blob_once($path, "$commit:$path");
    chomp @lines;
    return $base_lines_of{$key} = \@lines;
}

# Maps base line $n through the hunks: (index line) when the diff leaves it unchanged, or
# (undef, new start, new count) of the hunk that rewrote it.
sub map_line {
    my ($hunks, $n) = @_;
    my $offset = 0;
    for my $hunk (@$hunks) {
        my ($old, $old_count, $new, $new_count) = @$hunk;
        if ($old_count == 0) {    # a pure insertion after base line $old
            last if $n <= $old;
            $offset += $new_count;
            next;
        }
        last if $n < $old;
        return (undef, $new, $new_count) if $n < $old + $old_count;
        $offset += $new_count - $old_count;
    }
    return ($n + $offset);
}

# The constant shift the diff proves for a pinned span: the pin matched these lines at the base,
# the diff leaves every one of them unchanged, all move by the same nonzero offset, and the
# moved lines still hash to the pin in the index.
sub diff_relocation {
    my ($path, $lines, $pin, @numbers) = @_;
    return diff_relocation_from($repin_base, $path, $lines, $pin, @numbers);
}
sub diff_relocation_from {
    my ($commit, $path, $lines, $pin, @numbers) = @_;
    return undef if length($pin) < 8;
    my $base = base_lines($path, $commit) or return undef;
    return undef if grep { $_ < 1 || $_ > @$base } @numbers;
    return undef unless anchor_of($base, @numbers) =~ /^\Q$pin\E/;
    my $hunks = base_hunks($path, $commit) or return undef;
    my %shifts;
    for my $number (@numbers) {
        my ($moved) = map_line($hunks, $number);
        return undef unless defined $moved;
        $shifts{$moved - $number} = 1;
    }
    my ($shift, @more) = keys %shifts;
    return undef if @more || $shift == 0;
    my @moved = map { $_ + $shift } @numbers;
    return undef if grep { $_ < 1 || $_ > @$lines } @moved;
    return anchor_of($lines, @moved) =~ /^\Q$pin\E/ ? $shift : undef;
}

# DCG-V0-810 (V1-3467): when HEAD's diff proves nothing, typically because the insertion above a
# cited line was committed before --repin ran, the move is proved by the diff from the newest
# commit at or before the base, among the last $history_depth commits that touched the cited file,
# whose cited lines hash to the pin. That commit is where the citation's text last sat at its
# cited numbers, so the diff tracks those very lines, a repeated or short line included; a span
# whose own text changed since then is never relocated.
my $history_depth = 50;
my %history_of;
sub path_history {
    my ($path) = @_;
    return @{ $history_of{$path} } if exists $history_of{$path};
    return @{ $history_of{$path} = [] } if $path =~ /\n/;
    local $ENV{GIT_OPTIONAL_LOCKS} = '0';
    local $/ = "\n";
    open my $log, '-|', 'git', 'log', '--format=%H', "--max-count=$history_depth", $repin_base, '--', $path
        or die "git log -- $path: $!\n";
    chomp(my @commits = <$log>);
    close $log or die "git log $repin_base_label -- $path failed\n";
    return @{ $history_of{$path} = \@commits };
}
sub history_relocation {
    my ($path, $lines, $pin, @numbers) = @_;
    return undef if length($pin) < 8;
    for my $commit (path_history($path)) {
        my $base = base_lines($path, $commit) or next;
        next if grep { $_ < 1 || $_ > @$base } @numbers;
        next unless anchor_of($base, @numbers) =~ /^\Q$pin\E/;
        return diff_relocation_from($commit, $path, $lines, $pin, @numbers);
    }
    return undef;
}
# DCG-V0-920: whether the pin matched these lines at the base and a hunk with no new lines (a
# pure deletion) removed any of them.
sub diff_deletes {
    my ($path, $pin, $commit, @numbers) = @_;
    my $base = base_lines($path, $commit) or return 0;
    return 0 if grep { $_ < 1 || $_ > @$base } @numbers;
    return 0 unless anchor_of($base, @numbers) =~ /^\Q$pin\E/;
    my $hunks = base_hunks($path, $commit) or return 0;
    for my $number (@numbers) {
        my ($moved, undef, $count) = map_line($hunks, $number);
        return 1 if !defined $moved && !$count;
    }
    return 0;
}

# DCG-V0-841: HEAD's commit for the plain check's deletion hint, or undef on an unborn HEAD.
my $head_commit;
sub head_commit {
    return $head_commit eq q() ? undef : $head_commit if defined $head_commit;
    open my $resolve, '-|', 'git', 'rev-parse', '--verify', '--quiet', 'HEAD^{commit}' or die "git rev-parse: $!\n";
    chomp(my $commit = <$resolve> // '');
    close $resolve;
    $head_commit = $commit =~ /\A[0-9a-f]{40,64}\z/ ? $commit : '';
    return $head_commit eq '' ? undef : $head_commit;
}

# For a pinned span whose text changed since the base, the span the diff maps it to and that
# span's current anchor: an unchanged endpoint maps exactly, and one inside a rewritten hunk
# widens to that hunk's new lines. It is printed for the author to read, never written.
sub diff_suggestion {
    my ($token, $path, $lines, $pin, $first, $last, @extra) = @_;
    my @numbers = anchored_numbers($first, $last, @extra);
    my $base = base_lines($path) or return undef;
    return undef if grep { $_ < 1 || $_ > @$base } @numbers;
    return undef unless anchor_of($base, @numbers) =~ /^\Q$pin\E/;
    my $hunks = base_hunks($path) or return undef;
    # DCG-V0-920: a cited line in a hunk with no new lines was deleted; it maps to nothing, so the
    # citation stays a failure instead of re-anchoring to the next surviving line.
    my $deleted;
    my $start = sub {
        my ($moved, $new, $count) = map_line($hunks, $_[0]);
        $deleted = 1 if !defined $moved && !$count;
        return defined $moved ? $moved : $count ? $new : $new + 1;
    };
    my $end = sub {
        my ($moved, $new, $count) = map_line($hunks, $_[0]);
        $deleted = 1 if !defined $moved && !$count;
        return defined $moved ? $moved : $count ? $new + $count - 1 : $new;
    };
    my ($span, @now);
    if (defined $last) {
        my ($from, $to) = ($start->($first), $end->($last));
        return undef if $to < $from;
        ($span, @now) = ("$from-$to", $from .. $to);
    } else {
        @now = map { $start->($_) } $first, @extra;
        $span = join ',', @now;
    }
    return undef if $deleted;
    return undef if grep { $_ < 1 || $_ > @$lines } @now;
    (my $suggested = $token) =~ s{:\d[\d,-]*\@[0-9a-f]+`\z}{":$span\@" . substr(anchor_of($lines, @now), 0, 8) . '`'}e;
    return wantarray ? ($suggested, \@numbers, \@now) : $suggested;
}

# DCG-V0-047: whether the text before a backticked `:N` names a citation's file: empty for a
# continuation, else path-shaped or a tracked path.
sub citation_prefix {
    my ($prefix) = @_;
    return $prefix eq '' || $prefix =~ m{/} || $prefix =~ /\.[A-Za-z0-9]+\z/ || $tracked{$prefix};
}

# DCG-V0-047: the current full anchor of a pinned span whose lines, at the same numbers, hashed to
# the pin at the base and differ from the base only inside citation tokens' line spans and
# anchors; undef for any other change, which still needs a reader.
sub token_reanchor {
    my ($path, $lines, $pin, @numbers) = @_;
    return undef if length($pin) < 8;
    my $base = base_lines($path) or return undef;
    return undef if grep { $_ < 1 || $_ > @$base } @numbers;
    return undef unless anchor_of($base, @numbers) =~ /^\Q$pin\E/;
    # Only real citation tokens lose their spans and anchors: a continuation (`:7@abcdef01`) or
    # a path-shaped prefix (a `/`, a file extension, or a tracked path). `retry:3` is prose, so a
    # change to its number still needs a reader.
    my $strip = sub {
        (my $line = $_[0]) =~ s{`([^`\s]*?)(:\d[\d,]*(?:-\d+)?(?:\@[0-9a-f]{4,64})?)`}{citation_prefix($1) ? "`$1:`" : "`$1$2`"}ge;
        $line =~ s/\s+$//;
        $line;
    };
    for my $number (@numbers) {
        return undef unless $strip->($base->[$number - 1]) eq $strip->($lines->[$number - 1]);
    }
    return anchor_of($lines, @numbers);
}

# The citation token with every number in its line span moved by $shift; the pin is unchanged.
sub shifted_token {
    my ($token, $shift) = @_;
    $token =~ s{:(\d[\d,-]*)(\@[0-9a-f]+`)\z}{my $span = $1; my $tail = $2; $span =~ s/(\d+)/$1 + $shift/ge; ":$span$tail"}e;
    return $token;
}

# --hash prints one citation with its anchor appended, so an author can pin a citation
# without computing the digest by hand.
my $usage = "usage: check-line-citations.sh [--verbose] [--repin [--changed | --show-changed] [--base <rev>]] | --selects <rev> | --hash <path:line[-line][,line]> ...\n";
my ($repin, $verbose, $base_arg, $selects_base);
my @original_argv = @ARGV;
if (@ARGV && $ARGV[0] ne '--hash') {
    my @flags = @ARGV;
    while (@flags) {
        my $flag = shift @flags;
        if ($flag eq '--repin') { $repin = 1 }
        elsif ($flag eq '--verbose') { $verbose = 1 }
        elsif ($flag eq '--base' && @flags) { $base_arg = shift @flags }
        elsif ($flag eq '--changed') { $repin_changed = 1 }
        elsif ($flag eq '--show-changed') { $repin_show = 1 }
        elsif ($flag eq '--selects' && @flags) { $selects_base = shift @flags }
        else { die $usage }
    }
    die "--base needs --repin\n$usage" if defined $base_arg && !$repin;
    die "--changed needs --repin\n$usage" if $repin_changed && !$repin;
    die "--show-changed needs --repin and excludes --changed\n$usage" if $repin_show && (!$repin || $repin_changed);
    die "--selects takes no other flag\n$usage" if defined $selects_base && ($repin || $verbose);
}
# DCG-V0-045: --repin proves moves against the diff from --base (default HEAD) to the index. An
# unborn HEAD leaves only the hash search; a named base that does not resolve refuses.
if ($repin) {
    $repin_base_label = $base_arg // 'HEAD';
    open my $resolve, '-|', 'git', 'rev-parse', '--verify', '--quiet', '--end-of-options', "$repin_base_label^{commit}"
        or die "git rev-parse: $!\n";
    chomp(my $commit = <$resolve> // '');
    close $resolve;
    if ($commit =~ /\A[0-9a-f]{40,64}\z/) {
        $repin_base = $commit;
    } elsif (defined $base_arg) {
        die "--base $base_arg does not name a commit\n";
    }
}
# DCG-V0-921 (V1-3323): --selects REV lists the pinned citations whose cited file differs between
# REV and the index, so a lane whose change touches a cited file knows to run line-citations-check.
if (defined $selects_base) {
    open my $diff, '-|', 'git', 'diff', '--cached', '--name-only', '--no-renames', '--end-of-options', $selects_base, '--'
        or die "git diff: $!\n";
    while (my $path = <$diff>) { chomp $path; $selects_changed{$path} = 1; }
    close $diff or die "--selects $selects_base: not a revision git diff can read\n";
}
if (@ARGV && !defined $repin && !$verbose && !defined $selects_base) {
    my ($flag, @tokens) = @ARGV;
    die $usage unless $flag eq '--hash' && @tokens;
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

# DCG-V0-041: every unpinned citation a legacy document may still carry, one row per occurrence,
# as `<document>\t<path>:<span>`. Rows are only ever deleted: a new or edited unpinned citation
# has no row and fails, so a legacy table cannot drift silently onto a different non-blank line.
my $inventory_rows = lines_of('script/line-citation-unpinned-inventory.txt')
    or die "script/line-citation-unpinned-inventory.txt: not readable from the Git index\n";
for my $row (@$inventory_rows) {
    next if $row eq '' || $row =~ /^#/;
    my ($doc, $citation) = $row =~ /^([^\t]+)\t([^\t]+:\d[\d,-]*)$/
        or die "script/line-citation-unpinned-inventory.txt: malformed row: $row\n";
    die "script/line-citation-unpinned-inventory.txt: $doc is not on the legacy allowlist\n"
        unless $legacy_document{$doc};
    ++$unpinned_inventory{"$doc\t$citation"};
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
            $token_at = $-[0];
            my $rest = substr($line, pos($line));
            if (defined $9) {
                my $spec = $spec_of_id{$9};
                $current = $spec if $spec && $spec ne $doc && $rest =~ /^(?:\([a-z]\))?,?\s*\(?`:\d/;
                next;
            }
            if (defined $10) {
                my ($name, $span, $pin) = ($10, $11, $12);
                # DCG-V0-921: a changed, deleted root file such as README.md still reaches check() under --selects.
                if (($tracked{$name} || $selects_changed{$name}) && $span =~ /^(\d+)((?:,\d+)*)(?:-(\d+))?$/) {
                    $current = $name;
                    my @extra = grep { length } split /,/, $2;
                    check($doc, $line_number, $token, $name, $pin, $1, $3, @extra);
                    next;
                }
                push @skipped, "line citations: note: $doc:$line_number: skipped $token: basename is not tracked at repository root; use a repository-relative path\n";
                $current = $name =~ /\.md$/ && $tracked{"docs/specs/$name"} ? "docs/specs/$name" : undef;
                next;
            }
            if (defined $13) {
                # An untracked token is prose, so it leaves the paragraph anchor exactly as
                # it was: before this alternative existed the regex did not match it at all.
                # DCG-V0-921: so does a changed, deleted extensionless file such as Makefile.
                next unless $tracked{$13} || $selects_changed{$13};
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

# DCG-V0-041: the inventory only shrinks. A row no citation matches is a failure, not a note, so a
# retired row cannot later admit a new unpinned citation with the same document, path and span.
for my $key (sort keys %unpinned_inventory) {
    my $unused = $unpinned_inventory{$key} - ($unpinned_seen{$key} // 0);
    next if $unused <= 0;
    my ($doc, $citation) = split /\t/, $key;
    push @failures, "script/line-citation-unpinned-inventory.txt  $doc\t$citation  $unused row(s) match no "
        . "unpinned citation that resolves: delete them (rows are only deleted, never reused)";
}

push @failures, "unpinned citation count $unpinned_count exceeds committed ceiling $unpinned_ceiling; read and pin citations or lower the ceiling"
    if $unpinned_count > $unpinned_ceiling;

if (defined $selects_base) {
    close_index();
    if (@selected) {
        print "line citations: selects line-citations-check: " . scalar(@selected)
            . " pinned citation(s) cite a file changed since $selects_base; run make line-citations-check\n";
        # DCG-V0-980 (lane cit-u18 friction): each selected citation the index already breaks is
        # labelled moved (a mechanical --repin) or with its reason, and one in a document the change
        # does not edit is marked, so a lane sees before the seal which out-of-scope specs it moves.
        my %reason;
        for my $failure (@failures) {
            my ($key, $why) = $failure =~ /\A(\S+  `[^`]*`)  (.*)\z/s or next;
            $reason{$key} //= $why;
        }
        my ($broken, $moved, $outside) = (0, 0, 0);
        for my $entry (@selected) {
            my ($doc) = $entry =~ /\A(.*):\d+  /;
            my $line = "  $entry";
            if (defined(my $why = $reason{$entry})) {
                $broken++;
                if ($why =~ /--repin rewrites it\)\z/) { $moved++; $line .= "  [moved: --repin rewrites it]" }
                else { $line .= "  [breaks: $why]" }
                if (!$selects_changed{$doc}) { $outside++; $line .= "  [outside this change]" }
            }
            print "$line\n";
        }
        print "line citations: selects hint: $broken of " . scalar(@selected) . " selected citation(s) break at the "
            . "index ($moved moved, " . ($broken - $moved) . " need a read); $outside sit in documents this change does "
            . "not edit; run script/check-line-citations.sh --repin and commit the rewritten documents before sealing "
            . "(DCG-V0-980)\n" if $broken;
    } else {
        print "line citations: no pinned citation cites a file changed since $selects_base\n";
    }
    exit 0;
}

# DCG-V0-022: --repin rewrites the worktree copy of each document whose citations moved
# unchanged, and only when that copy is byte-identical to the index copy the scan read. DCG-V0-048:
# it then stages exactly those rewritten documents (the index held their pre-rewrite bytes, so
# nothing else is staged) and reruns itself until a pass rewrites nothing, at most five passes, so a
# transitive chain resolves in one call and the index-reading check passes with no manual staging.
# DCG-V0-842: every other finding is printed with its reason and fails the run.
if ($repin) {
    # DCG-V0-630: show each changed span old (at the base) and new (in the index) and write the
    # mapped token only on a `y` answer; any other answer leaves it as a DCG-V0-045 suggestion.
    my $answers;
    open $answers, '<&=', 3 or die "--show-changed: no answer input: $!\n" if @show_changed;
    for my $shown (@show_changed) {
        my ($doc, $line_number, $from, $to, $at, $path, $lines, $old, $new) = @$shown;
        my $base = base_lines($path);
        print "$doc:$line_number  $from changed since $repin_base_label; the diff maps it to $to\n";
        print "  - $_: " . ($base->[$_ - 1] =~ s/\r?\n\z//r) . "\n" for @$old;
        print "  + $_: " . ($lines->[$_ - 1] =~ s/\r?\n\z//r) . "\n" for @$new;
        print "write $to? [y/N] ";
        my $answer = <$answers> // '';
        print "\n" unless length $answer;
        if ($answer =~ /\A\s*y(?:es)?\s*\z/i) {
            push @repins, [$doc, $line_number, $from, $to, $at];
            push @changed_repins, [$doc, $line_number, $from, $to];
        } else {
            push @suggestions, [$doc, $line_number, $from, $to];
        }
    }
    my (%by_doc, @refused);
    push @{ $by_doc{$_->[0]} }, $_ for @repins;
    for my $doc (sort keys %by_doc) {
        my $indexed = index_blob($doc);
        my $worktree = do { local $/; open my $in, '<', $doc or die "$doc: $!\n"; <$in> } // '';
        if ($worktree ne $indexed) {
            print STDERR "line citations: --repin refused $doc: the worktree copy differs from the index; "
                . "run `git add -- $doc` (or discard that edit), then rerun --repin\n";
            push @refused, $doc;
            next;
        }
        my @doc_lines = split /^/, $worktree;
        # Rewrite each reported occurrence at its own offset, right to left within a line, so an
        # identical token elsewhere on the line (a `:N` continuation resolving to another file)
        # stays as written.
        for my $move (sort { $a->[1] <=> $b->[1] || $b->[4] <=> $a->[4] } @{ $by_doc{$doc} }) {
            my (undef, $line_number, $from, $to, $at) = @$move;
            die "$doc:$line_number: --repin offset $at no longer holds $from\n"
                unless substr($doc_lines[$line_number - 1], $at, length $from) eq $from;
            substr($doc_lines[$line_number - 1], $at, length $from) = $to;
            print "$doc:$line_number  $from -> $to\n";
        }
        my $temporary = "$doc.repin.$$";
        open my $out, '>', $temporary or die "$temporary: $!\n";
        print {$out} @doc_lines or die "$temporary: $!\n";
        close $out or die "$temporary: $!\n";
        chmod((stat $doc)[2] & 07777, $temporary) or die "$temporary: $!\n";
        rename $temporary, $doc or die "$doc: $!\n";
    }
    close_index();
    for my $changed (@changed_repins) {
        my ($doc, $line_number, $from, $to) = @$changed;
        print "$doc:$line_number  $from changed since $repin_base_label; " . ($repin_show ? 'confirmed, written' : '--changed re-anchored it')
            . " as $to (read those lines: the anchor now asserts this content)\n";
    }
    # DCG-V0-045: a changed span is never rewritten; the diff's mapping and the new anchor are
    # printed so the author reads those lines and pastes the token only if the sentence holds.
    for my $suggestion (@suggestions) {
        my ($doc, $line_number, $from, $to) = @$suggestion;
        print "$doc:$line_number  $from changed since $repin_base_label; the diff maps it to $to "
            . "(read those lines; paste it only if the sentence still holds)\n";
    }
    my @rewritten = grep { my $doc = $_; !grep { $_ eq $doc } @refused } sort keys %by_doc;
    my $count = grep { my $doc = $_->[0]; grep { $_ eq $doc } @rewritten } @repins;
    my $pass = ($ENV{CHECK_LINE_CITATIONS_REPIN_PASS} // 1) =~ /\A[1-9]\z/ ? $ENV{CHECK_LINE_CITATIONS_REPIN_PASS} // 1 : 1;
    # DCG-V0-840 (V1-3613): the citations earlier passes rewrote, so the last pass, which rewrites
    # nothing, still reports them instead of saying nothing moved.
    my $earlier = ($ENV{CHECK_LINE_CITATIONS_REPIN_TOTAL} // '') =~ /\A\d{1,9}\z/ ? $ENV{CHECK_LINE_CITATIONS_REPIN_TOTAL} : 0;
    my $total = $earlier + (@rewritten ? $count : 0);
    my $staged = 0;
    if (@rewritten) {
        # Literal pathspecs: a document named docs/[ab].md must not also stage docs/a.md.
        $staged = system('git', '--literal-pathspecs', 'add', '--', @rewritten) == 0;
        print STDERR "line citations: --repin rewrote $count citation(s) in " . scalar(@rewritten) . " document(s) and "
            . ($staged ? "staged them (git add -- @rewritten)\n"
                : "could not stage them; run `git add -- @rewritten` and rerun the check\n");
    }
    if (!@repins && $total) {
        print STDERR "line citations: --repin rewrote $total citation(s) across $pass pass(es); this pass found no further move\n";
    } elsif (!@repins) {
        print STDERR "line citations: --repin found no citation whose content moved unchanged\n";
    }
    print_unstaged('--repin', @rewritten);
    print_skipped();
    # DCG-V0-842 (V1-3672): every citation --repin did not resolve is printed with its reason, and
    # the run fails, so a deletion found under the base is never reported as a clean repin. They
    # print before every exit below, so a refused or unconverged document elsewhere cannot hide
    # them; the rerun pass recomputes them from the staged rewrite.
    my @unresolved = grep { !/--repin rewrites it/ } @failures;
    my $print_unresolved = sub {
        return unless @unresolved;
        print STDERR "line citations: --repin left " . scalar(@unresolved) . " citation(s) that do not resolve:\n";
        print STDERR "  $_\n" for @unresolved;
    };
    if (@refused || @rewritten && !$staged) {
        # DCG-V0-048: an unstaged rewrite leaves the index-reading check failing, so it fails too.
        $print_unresolved->();
        exit 1;
    }
    # A rewrite can unpin a citation into the rewritten document; the next pass reads the staged
    # rewrite and resolves that link of the chain. A fifth pass that still rewrote did not converge.
    if (@rewritten) {
        if ($pass >= 5 || !defined $ENV{CHECK_LINE_CITATIONS_SELF}) {
            print STDERR "line citations: --repin still rewrote citations in pass $pass of at most 5; the chain did not converge, so read the rewritten documents and rerun --repin\n";
            $print_unresolved->();
            exit 1;
        }
        print STDERR "line citations: --repin pass " . ($pass + 1) . " rereads the staged rewrite\n";
        $ENV{CHECK_LINE_CITATIONS_REPIN_PASS} = $pass + 1;
        $ENV{CHECK_LINE_CITATIONS_REPIN_TOTAL} = $total;
        exec 'bash', $ENV{CHECK_LINE_CITATIONS_SELF}, @original_argv or die "rerun --repin: $!\n";
    }
    $print_unresolved->();
    exit(@unresolved ? 1 : 0);
}

close_index();

# DCG-V0-046: failures print first, then the notes; the exit status is decided by failures alone.
if (@failures) {
    print STDERR "line citations: " . scalar(@failures) . " citation(s) do not resolve:\n";
    print STDERR "  $_\n" for @failures;
    # DCW-V0-353 (V1-2836): the one command that clears every moved-unchanged failure.
    my $movable = grep { /--repin rewrites it/ } @failures;
    print STDERR "line citations: fix: $movable citation(s) moved unchanged; run script/check-line-citations.sh --repin (it stages the documents it rewrites), then rerun make line-citations-check\n"
        if $movable;
}
print_skipped();
print_unstaged('this gate');
exit 1 if @failures;

# DCG-V0-046: one summary line for every root-basename skip, or each skip under --verbose.
sub print_skipped {
    return unless @skipped;
    if ($verbose) {
        print STDERR @skipped;
        return;
    }
    print STDERR "line citations: note: skipped " . scalar(@skipped) . " citation(s) whose basename is not "
        . "tracked at repository root (--verbose lists them)\n";
}

# DCG-V0-021: the gate reads the index, so an unstaged edit to a file it read is invisible to it.
# Name those files instead of letting a pass read as a check of the worktree. This is a note, not
# a failure: a failed `git diff` is reported and leaves the result alone, and GIT_OPTIONAL_LOCKS=0
# keeps it from refreshing the index on disk. Under --repin (DCG-V0-045) it names the exact
# staging step, since a move in an unstaged file is invisible to --repin as well.
sub print_unstaged {
    my ($reader, @rewritten) = @_;
    my %unstaged;
    local $ENV{GIT_OPTIONAL_LOCKS} = '0';
    if (open my $diff, '-|', 'git', 'diff', '--name-only', '-z') {
        local $/ = "\0";
        while (my $path = <$diff>) {
            chomp $path;
            $unstaged{$path} = 1 if defined $lines_of{$path};
        }
        close $diff
            or print STDERR 'line citations: note: could not list unstaged files ('
                . ($! ? "git diff: $!" : 'git diff exit ' . ($? >> 8)) . ")\n";
    } else {
        print STDERR "line citations: note: could not list unstaged files (git diff: $!)\n";
    }
    # The documents --repin just rewrote are named by its own staging line above.
    delete @unstaged{@rewritten};
    return unless %unstaged;
    my @paths = sort keys %unstaged;
    print STDERR "line citations: note: $reader reads the Git index, so unstaged edits to "
        . scalar(@paths) . " file(s) it read were not checked (git add them to include them):\n"
        . join('', map { "  $_\n" } @paths)
        . ($reader eq '--repin' ? "  run `git add -- @paths`, then rerun --repin\n" : '');
}
PERL

# conformance/divergence-register.md is deliberately not in scanned_doc (V1-0267): it carries about
# 90 legacy bare citations, so scanning it would raise the unpinned ceiling, which only a contract
# diff may do. Its GPK-V0 range citations are repinned by hand instead.
