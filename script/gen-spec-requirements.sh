#!/bin/sh
set -eu

# Enumerate tracked paths and read their bytes from the Git index, not the worktree: `make
# gate` must measure the set a fresh clone contains (`Makefile:14`), and `spec-requirements-check`
# byte-compares this output against the committed index. A worktree listing broke that both ways:
# an untracked spec added a row no clone can reproduce, and a tracked spec deleted from the
# worktree alone silently dropped its rows. `git add` is what publishes a spec to this index.
# `decision-number-gate-v0.md` pins the rule; `check-decision-numbers.sh` is the reference.
#
# With no argument the script prints docs/specs/REQUIREMENTS.tsv. `--index` prints
# docs/specs/INDEX.json and `--readme` prints docs/specs/README.md, each projected from the
# staged spec headers onto the staged file (SRG-V0-012). `--lines` prints the uncommitted locator
# view: the same rows with each definition's line number, ordered by id (SRG-V0-015). Those modes
# write stdout only. `--all DIR` writes REQUIREMENTS.tsv, INDEX.json and README.md into the existing
# directory DIR from one read of the staged specs, each byte for byte what its own mode prints
# (SRG-V0-018); `make spec-requirements` and `spec-requirements-check` use it.

usage() { printf 'usage: %s [--index | --readme | --lines | --all DIR]\n' "$0" >&2; exit 2; }
out_dir=
case ${1:-} in
  '') mode=tsv ;;
  --index) mode=index ;;
  --readme) mode=readme ;;
  --lines) mode=lines ;;
  --all)
    test $# = 2 && test -n "$2" || usage
    mode=all
    out_dir=$(CDPATH= cd -- "$2" && pwd) || { printf '%s: --all DIR must be an existing directory\n' "$0" >&2; exit 2; }
    ;;
  *) usage ;;
esac

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

# DCW-V0-352 (V1-2835): `--all` reads the staged specs, so a docs/specs edit left unstaged is
# silently not in its output. Name each one on stderr (at most 20, then a count) before generating;
# the output and exit status are unchanged. Only the docs/specs/*.md files the generator reads are
# named; README.md is an output, never an input. A caller that points GIT_INDEX_FILE at a private
# snapshot index (dogfood-finish.sh) has no staging step to warn about, so it gets no warning.
if test "$mode" = all && test -z "${GIT_INDEX_FILE:-}"; then
  ignored=$(GIT_OPTIONAL_LOCKS=0 git -c core.quotepath=off diff --name-only -- docs/specs 2>/dev/null
    GIT_OPTIONAL_LOCKS=0 git -c core.quotepath=off ls-files --others --exclude-standard -- docs/specs 2>/dev/null) ||
    printf '%s: WARN could not list unstaged docs/specs edits\n' "$0" >&2
  printf '%s\n' "$ignored" | grep -E '^docs/specs/[^/]+\.md$' | grep -v -x docs/specs/README.md | sort -u | awk -v self="$0" '
    NR <= 20 { printf "%s: WARN %s is not staged, so this run ignored it; git add it, then rerun make spec-requirements\n", self, $0 }
    END { if (NR > 20) printf "%s: WARN %d more unstaged docs/specs edits were ignored\n", self, NR - 20 }' >&2
fi

perl - "$mode" "$out_dir" <<'PERL'
use strict;
use warnings;
use IPC::Open2 qw(open2);
use POSIX ();
use JSON::PP ();

my ($mode, $out_dir) = @ARGV;

# SRG-V0-018: every index read of one invocation goes through one `git cat-file --batch --buffer`
# co-process, pipelined: a forked writer sends every request while this process reads the replies,
# so neither side waits on a round trip and neither can fill the other's pipe. A path the batch
# cannot serve (a newline breaks its request framing; a missing or non-blob entry) is read by a
# one-shot `git cat-file blob`, which reports and exits exactly as the per-file read always did.
sub index_blob_once {
    my ($path) = @_;
    open my $fh, '-|', 'git', 'cat-file', 'blob', ":$path" or die "$path: $!\n";
    my $blob = do { local $/; <$fh> } // '';
    close $fh
        or die $! ? "git cat-file :$path: $!\n" : "git cat-file :$path failed: exit " . ($? >> 8) . "\n";
    return $blob;
}

sub index_blobs {
    my @paths = @_;
    my %blob;
    my @batch = grep { !/\n/ } @paths;
    my %fallback = map { $_ => 1 } grep { /\n/ } @paths;
    if (@batch) {
        local $SIG{PIPE} = 'IGNORE';    # a dead co-process surfaces as a short read, not a signal
        local $/ = "\n";
        my $pid = open2(my $out, my $in, 'git', 'cat-file', '--batch', '--buffer');
        my $writer = fork // die "fork: $!\n";
        if ($writer == 0) {
            close $out;
            print {$in} map { ":$_\n" } @batch;
            POSIX::_exit(close $in ? 0 : 1);
        }
        close $in;
        my $short = 0;
        for my $path (@batch) {
            my $header = <$out>;
            $short = 1, last unless defined $header;
            my ($type, $size) = $header =~ /\A[0-9a-f]+ (\S+) (\d+)\n\z/ or ($fallback{$path} = 1, next);
            my $read = read($out, my $body, $size + 1) // die "git cat-file --batch: $!\n";
            $short = 1, last unless $read == $size + 1 && substr($body, -1, 1, '') eq "\n";
            $type eq 'blob' ? ($blob{$path} = $body) : ($fallback{$path} = 1);
        }
        close $out;
        waitpid $writer, 0;
        waitpid $pid, 0;
        # Reports the co-process status the way a closed `-|` pipe does; $! is cleared so `die`
        # exits with that status rather than a stale errno.
        $! = 0;
        die 'git cat-file --batch failed: exit ' . ($? >> 8) . "\n" if $?;
        die "git cat-file --batch failed: short read\n" if $short;
    }
    $blob{$_} = index_blob_once($_) for grep { $fallback{$_} } @paths;
    return \%blob;
}

my @specs;
my %tracked;
{
    open my $paths, '-|', 'git', 'ls-files', '-z', '--', 'docs/specs'
        or die "git ls-files: $!\n";
    local $/ = "\0";
    while (my $path = <$paths>) {
        chomp $path;
        $tracked{$path} = 1;
        next unless $path =~ m{\Adocs/specs/[^/]+\.md\z};
        next if $path eq 'docs/specs/README.md';
        push @specs, $path;
    }
    close $paths
        or die $! ? "git ls-files: $!\n" : 'git ls-files failed: exit ' . ($? >> 8) . "\n";
}

# SRG-V0-009: a rank 0 definition records how its text was accepted in its leading parenthetical.
# `standing` names a standing approval and wins over every other word, so a promotion to `owner`
# must remove it; `owner` needs an affirmative accepted/approved or "owner decision" and an anchor,
# a decision number or a date. A negated phrase such as "not (yet) accepted" or "not an owner
# decision" does not count, even across an inner parenthesis. Anything else, including no
# parenthetical, is `unrecorded` and inherits the spec header's intent status.
sub acceptance {
    my ($text) = @_;
    return ('unrecorded', '-') unless $text =~ /\A\(/;
    my ($depth, $end) = (0, length $text);
    for my $i (0 .. length($text) - 1) {
        my $char = substr($text, $i, 1);
        if ($char eq '(') {
            ++$depth;
        } elsif ($char eq ')' && --$depth == 0) {
            $end = $i;
            last;
        }
    }
    my $marker = substr($text, 1, $end - 1);
    my $mode = 'unrecorded';
    if ($marker =~ /\bstanding\b/i) {
        $mode = 'standing';
    } else {
        (my $affirmed = $marker) =~ s/\b(?:not|never)\b[^;,]*?\b(?:accepted|approved|owner decision)\b//gi;
        $mode = 'owner'
            if $affirmed =~ /\b(?:accepted|approved|owner decision)\b/i
            && $affirmed =~ /\bdecisions?\s+[0-9]{4}(?![-0-9])|\b[0-9]{4}-[0-9]{2}-[0-9]{2}\b/i;
    }
    return ($mode, '-') if $mode eq 'unrecorded';
    my (@anchors, %seen);
    while ($marker =~ /\bdecisions?\s+([0-9]{4})(?![-0-9])|\b([0-9]{4}-[0-9]{2}-[0-9]{2})\b|\b(V[0-9]+-[0-9]{4})\b|(#[0-9]+)\b/gi) {
        my $anchor = defined $1 ? "decision $1" : $2 // $3 // $4;
        push @anchors, $anchor unless $seen{$anchor}++;
    }
    return ($mode, @anchors ? join('; ', @anchors) : '-');
}

# SRG-V0-042: a definition whose text opens with an acceptance note the SRG-V0-009 reader does not
# classify as intended is refused, naming the accepted forms, instead of silently recording
# `unrecorded` or `owner`. Three near misses are refused: an auto-acceptance that omits the word
# `standing` (it would read as `owner` or `unrecorded`); a leading parenthetical that affirms
# acceptance but records no decision number or date; and an unparenthesized leading `accepted` or
# `approved` note, which the reader never sees. SRG-V0-044 adds a trailing `Status: (accepted ...)`
# note on the definition line, also never seen. SRG-V0-046: a leading note still open at the end of
# the definition line is refused for that reason, since its continuation is never read. An empty
# return is no near miss.
sub acceptance_near_miss {
    my ($text) = @_;
    if ($text =~ /\A\(/) {
        my ($depth, $end) = (0, length $text);
        for my $i (0 .. length($text) - 1) {
            my $char = substr($text, $i, 1);
            if ($char eq '(') {
                ++$depth;
            } elsif ($char eq ')' && --$depth == 0) {
                $end = $i;
                last;
            }
        }
        my $marker = substr($text, 1, $end - 1);
        my ($mode) = acceptance($text);
        (my $affirmed = $marker) =~ s/\b(?:not|never)\b[^;,]*?\b(?:accepted|approved|owner decision)\b//gi;
        my $auto = $marker =~ /\bauto-?accept/i && $mode ne 'standing';
        my $unanchored = $mode eq 'unrecorded' && $affirmed =~ /\b(?:accepted|approved|owner decision)\b/i;
        return 'the acceptance note is still open at the end of the definition line, and only that '
            . 'line is read, so a decision number, date or `standing` on a later line is not seen; '
            . 'close the note on the line of the id, for example '
            . '`(accepted by the owner, owner chat YYYY-MM-DD; V1-NNNN)`'
            if $depth > 0 && ($auto || $unanchored);
        return 'an auto-acceptance note must name the standing approval' if $auto;
        return 'the acceptance note records no decision number or date; add one, for example '
            . '`(accepted, decision NNNN; V1-NNNN)` or `(accepted YYYY-MM-DD; V1-NNNN)`'
            if $unanchored;
        return '';
    }
    return 'the acceptance note is not parenthesized, so it is not read'
        if $text =~ /\A(?:accepted|approved)(?:,|:|\s+(?:by|under|on)\b|\s+\(|\s+[0-9]{4}-[0-9]{2}-[0-9]{2}\b)/i;
    # SRG-V0-044: a trailing `Status: (accepted ...)` note on the definition line is never read.
    return 'the acceptance note trails the definition as `Status: (...)`, so it is not read'
        if $text =~ /\bStatus:\s*\((?:accepted|approved)\b/i;
    return '';
}

sub trim {
    my ($text) = @_;
    $text =~ s/\A\s+|\s+\z//g;
    return $text;
}

# The header and digest readers mirror `header` and `checkDigest` in internal/specindex, which
# test that INDEX.json and README.md agree with them: the first line anywhere in the file that,
# after an optional "- ", starts with the plain or bold key, and the `Claim` of the Agent digest.
sub header_value {
    my ($blob, $key) = @_;
    (my $bold = $key) =~ s/:\z//;
    $bold = "**$bold:**";
    return $blob =~ /^[^\S\n]*(?:- )?[^\S\n]*(?:\Q$key\E|\Q$bold\E)([^\n]*)/m ? trim($1) : undef;
}

sub digest_values {
    my ($blob) = @_;
    $blob =~ /^[^\S\n]*## Agent digest[^\S\n]*(?:\n|\z)/mg or return {};
    my %values;
    for my $line (split /\n/, substr($blob, pos $blob)) {
        my $text = trim($line);
        last if $text eq q();
        next unless $text =~ s/\A- //;
        my ($key, $value) = split /: /, $text, 2;
        $values{$key} = $value if defined $value;
    }
    return \%values;
}

my @reads = ((sort @specs), grep { $tracked{$_} } 'docs/specs/INTENT-STATUS-FROZEN.tsv');
push @reads, 'docs/specs/INDEX.json'
    if ($mode eq 'index' || $mode eq 'all') && $tracked{'docs/specs/INDEX.json'};
push @reads, 'docs/specs/README.md'
    if ($mode eq 'readme' || $mode eq 'all') && $tracked{'docs/specs/README.md'};
my $blobs = index_blobs(@reads);

my (%best, %header, %definitions);
for my $file (sort @specs) {
    my $blob = $blobs->{$file};
    # Every mode reads the header: SRG-V0-040 bounds the claim even where only the TSV is printed.
    my ($title) = $blob =~ /^# ([^\n]+)/m;
    $title = trim($title) if defined $title;
    my $prefix = header_value($blob, 'Requirement prefix:');
    $prefix =~ s/`//g if defined $prefix;
    $header{$file} = {
        title => $title,
        prefix => $prefix,
        intent => header_value($blob, 'Intent status:'),
        delivery => header_value($blob, 'Delivery status:'),
        claim => digest_values($blob)->{Claim}, status => digest_values($blob)->{Status},
    };
    my $line_number = 0;
    my $previous_blank = 1;
    my $rollback = 0;
    my ($fence, $continues) = ('', 0);
    for my $line (split /^/, $blob) {
        ++$line_number;
        # SRG-V0-003: unmarked definitions start paragraphs, not wrapped prose.
        my $definition_boundary = $previous_blank || $line =~ /^\s*(?:[-*]\s+|\*\*|`)/;
        # SRG-V0-548: under RDG-V0-023 (script/check-requirement-definitions.sh, whose fence
        # tracking this mirrors) an unbulleted line that directly follows a non-blank, non-heading
        # line outside a fence continues that paragraph or list item, so a marked id that starts it
        # may be a wrapped citation such as `ID`. ending a sentence. Such a line still ranks 0, but
        # after every unwrapped rank 0 line, and it is never counted as a second definition.
        my $wrapped = $continues && $line !~ /^\s*[-*]\s/ ? 1 : 0;
        $previous_blank = $line =~ /^\s*$/;
        my ($run, $rest) = $line =~ /^ {0,3}(`{3,}|~{3,})(.*?)\r?\n?\z/ ? ($1, $2) : ('', '');
        if ($fence ne '') {
            $fence = '' if $run ne '' && substr($run, 0, 1) eq substr($fence, 0, 1)
                && length($run) >= length($fence) && $rest =~ /\A[ \t]*\z/;
        } elsif ($run ne '' && !($run =~ /^`/ && $rest =~ /`/)) {
            $fence = $run;
        }
        $continues = $fence eq '' && $run eq '' && $line =~ /\S/ && $line !~ /^ {0,3}#/;
        # SRG-V0-966 (V1-3660): a rollback section restates IDs (`ID: revert ...`) without
        # defining them, so its lines never rank 0; the next heading ends the section.
        $rollback = $1 =~ /^Rollback\b/i ? 1 : 0 if $fence eq '' && $line =~ /^ {0,3}#{2,6}\s+(.*)/;

        while ($line =~ /\b([A-Z][A-Z0-9-]*-[0-9]{3})\b/g) {
            my $id = $1;
            my $quoted = quotemeta($id);
            my $is_period = $definition_boundary && $line =~ /^\s*(?:[-*]\s+)?(?:\*\*)?`?$quoted`?\.(?:\*\*)?\s/;
            my $rank = $is_period ? 0
                     : $definition_boundary && $line =~ /^\s*(?:[-*]\s+)?(?:\*\*)?`?$quoted`?(?:\*\*)?\s*:/ ? 0
                     : $line =~ /^\s*\|\s*`?$quoted`?(?:\s|\||\.)/ ? 1
                     : 2;
            $rank = 2 if $rank == 0 && $rollback;
            next if $rank > 1;
            push @{$definitions{$id}}, "$file:$line_number"
                if $rank == 0 && !$wrapped && !grep { $_ eq "$file:$line_number" } @{$definitions{$id} // []};
            my $key = join("\0", sprintf('%d', $rank), $rank == 0 ? $wrapped : 0, $file, sprintf('%09d', $line_number));
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
            my ($acceptance, $accepted_by) = $rank == 0 ? acceptance($title) : ('unrecorded', '-');
            my $near_miss = $rank == 0 ? acceptance_near_miss($title) : '';
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
                acceptance => $acceptance,
                accepted_by => $accepted_by,
                near_miss => $near_miss,
            };
        }
    }
}

# SRG-V0-015: the committed index carries no line number and orders its rows by file, then by
# definition position, so an edit that shifts a spec's lines rewrites no row and two changes that
# add requirements to different sections of one spec insert their rows at different places.
# `--lines` is the derived locator view with the line column, ordered by id; it is never committed.
sub tsv_text {
    my @ids = sort {
        $best{$a}{file} cmp $best{$b}{file} || $best{$a}{line} <=> $best{$b}{line} || $a cmp $b
    } keys %best;
    return join '', "id\tfile\ttitle\tacceptance\tacceptedBy\n",
        map { join("\t", $_, @{$best{$_}}{qw(file title acceptance accepted_by)}) . "\n" } @ids;
}
# SRG-V0-040: the specindex claim bound (1..160 characters), refused in every mode, before the
# TSV and --lines modes exit, with the path and character length, so it fails at the first spec
# step instead of at spec-requirements-check.
for my $file (sort @specs) {
    next unless defined(my $claim = $header{$file}{claim});
    utf8::decode($claim);
    length($claim) <= 160
        or die "$file: the Agent digest Claim is " . length($claim) . " characters; the limit is 160\n";
}
# status_ids returns the requirement IDs a status names: each SRG-V0-003 token and each `..NNN` or
# `/NNN` suffix after it, read with the token's prefix, so `X-V0-001..003` and `X-V0-001/002` name
# X-V0-003 and X-V0-002 too.
sub status_ids {
    my ($text) = @_;
    my @ids;
    while ($text =~ /\b([A-Z][A-Z0-9-]*-)([0-9]{3})((?:(?:\.\.|\/)[0-9]{3}\b)*)\b/g) {
        my ($prefix, $first, $rest) = ($1, $2, $3);
        push @ids, map { "$prefix$_" } $first, $rest =~ /([0-9]{3})/g;
    }
    return @ids;
}
# SRG-V0-980 (V1-3458): the header `Intent status:` and digest `Status:` are frozen. A spec may name
# in them only the requirement IDs its row in INTENT-STATUS-FROZEN.tsv lists, and its header may
# exceed 160 characters only up to that row's length, so an amendment is recorded on the
# requirement's own line (SRG-V0-981) and two lanes never append to one header line.
{
    my %frozen;
    my $frozen_text = $blobs->{'docs/specs/INTENT-STATUS-FROZEN.tsv'} // '';
    for my $row (split /\n/, $frozen_text) {
        next if $row =~ /\Apath\t/;
        my ($path, $chars, $ids) = split /\t/, $row, -1;
        defined $ids && $chars =~ /\A[0-9]+\z/
            or die "docs/specs/INTENT-STATUS-FROZEN.tsv: malformed row: $row\n";
        $frozen{$path} = { chars => $chars, ids => { map { $_ => 1 } split / /, $ids } };
    }
    my @refused;
    for my $file (sort @specs) {
        my $row = $frozen{$file} // { chars => 0, ids => {} };
        for my $field (['Intent status', $header{$file}{intent}], ['Agent digest Status', $header{$file}{status}]) {
            my ($name, $value) = @$field;
            next unless defined $value;
            my %seen;
            push @refused, "$file: its $name names $_, which its frozen header text does not"
                for grep { !$row->{ids}{$_} && !$seen{$_}++ } status_ids($value);
        }
        next unless defined(my $intent = $header{$file}{intent});
        utf8::decode($intent);
        push @refused, "$file: its Intent status is " . length($intent) . " characters; the limit is "
            . ($row->{chars} > 160 ? "its frozen length, $row->{chars}" : '160')
            if length($intent) > 160 && length($intent) > $row->{chars};
    }
    if (@refused) {
        print STDERR "$_\n" for @refused;
        die "the Intent status header and digest Status hold the current status only; record an "
            . "acceptance or amendment in the requirement's own leading note, for example "
            . "`- `ID`: (accepted under the owner's standing auto-accept (YYYY-MM-DD); V1-NNNN) ...`, "
            . "which REQUIREMENTS.tsv carries in its acceptance and acceptedBy columns, and in its "
            . "build-log entry (SRG-V0-009, SRG-V0-981), then remove the new header text and rerun "
            . "make spec-requirements; a renamed spec moves its INTENT-STATUS-FROZEN.tsv row to the new "
            . "path; see docs/SPEC-DRIVEN-DEVELOPMENT.md (SRG-V0-980)\n";
    }
}
# SRG-V0-547 (V1-3049): a requirement ID defined by more than one rank 0 line, in one spec or
# across specs, is refused in every mode, naming every definition, instead of keeping the first
# and silently dropping the rest. Two lanes that each define the same ID collide here.
{
    my @duplicate = grep { @{$definitions{$_}} > 1 } sort keys %definitions;
    if (@duplicate) {
        print STDERR "$_: defined more than once: @{$definitions{$_}}\n" for @duplicate;
        die "a requirement ID must have exactly one definition line across docs/specs; renumber or "
            . "remove the extra definitions, then rerun make spec-requirements (SRG-V0-547)\n";
    }
}
{
    my @near = grep { $best{$_}{near_miss} ne '' } sort keys %best;
    if (@near) {
        print STDERR "$best{$_}{file}:$best{$_}{line}: $_: $best{$_}{near_miss}\n" for @near;
        die "write the note in parentheses right after the id, closed on the id's line: "
            . "`(accepted, decision NNNN; V1-NNNN)`, `(accepted YYYY-MM-DD; V1-NNNN)` or "
            . "`(accepted under the owner's standing auto-accept (YYYY-MM-DD); V1-NNNN)`; a longer "
            . "clause may carry the anchor, as in `(accepted by the owner, amending X-V0-001 (owner "
            . "chat, YYYY-MM-DD); V1-NNNN)`; see docs/SPEC-DRIVEN-DEVELOPMENT.md (SRG-V0-042)\n";
    }
}
if ($mode eq 'tsv') {
    print tsv_text();
    exit 0;
}
if ($mode eq 'lines') {
    print "id\tfile\tline\ttitle\tacceptance\tacceptedBy\n";
    for my $id (sort keys %best) {
        my $row = $best{$id};
        print join("\t", $id, @{$row}{qw(file line title acceptance accepted_by)}), "\n";
    }
    exit 0;
}

# SRG-V0-012: INDEX.json and the README.md spec table are projections. A spec header owns its
# intent and delivery status and its Agent digest owns the claim; INDEX.json owns every other
# field and README.md every other cell, which the projection keeps byte for byte. A spec with no
# entry or row gains one built from its header, appended after the existing ones.
for my $file (sort @specs) {
    my $spec = $header{$file};
    defined $spec->{intent} && defined $spec->{delivery}
        or die "$file: declare Intent status and Delivery status before regenerating\n";
    for my $field (qw(intent delivery claim)) {
        next unless defined $spec->{$field};
        utf8::decode($spec->{$field}) or die "$file: $field is not valid UTF-8\n";
        $spec->{$field} =~ /[|\x00-\x1f]/
            and die "$file: $field holds a table pipe or control character\n";
    }
    # The README description is the claim then any tail; a one-sentence claim is what lets the
    # projection find where the old claim ends without reading INDEX.json.
    next unless defined $spec->{claim};
    $spec->{claim} =~ /[.?!]\z/ && $spec->{claim} !~ /[.?!]\s/
        or die "$file: the Agent digest Claim must be one sentence ending in . ? or !\n";
}

sub default_prefix {
    my ($file) = @_;
    return $header{$file}{prefix} if defined $header{$file}{prefix};
    my ($id) = grep { $best{$_}{file} eq $file && $best{$_}{key} =~ /\A0\0/ } sort keys %best;
    return defined $id ? $id =~ s/-[0-9]{3}\z//r : '';
}

sub new_spec_title {
    my ($file) = @_;
    my $title = $header{$file}{title} // die "$file: a new spec needs a `# Title` line\n";
    utf8::decode($title) or die "$file: title is not valid UTF-8\n";
    defined $header{$file}{claim} or die "$file: a new spec needs an Agent digest Claim\n";
    return $title;
}

sub index_text {
    my @keys = qw(path title reqPrefix intent delivery claim dependsOn supersededBy closableToday implementation);
    my $raw = $tracked{'docs/specs/INDEX.json'} ? $blobs->{'docs/specs/INDEX.json'} : "[]\n";
    my $entries = eval { JSON::PP->new->utf8->decode($raw) };
    ref $entries eq 'ARRAY' or die "docs/specs/INDEX.json: not a JSON array\n";
    my %listed;
    for my $entry (@$entries) {
        ref $entry eq 'HASH' && join(',', sort keys %$entry) eq join(',', sort @keys)
            or die "docs/specs/INDEX.json: every entry needs exactly the keys @keys\n";
        my $path = $entry->{path};
        !$listed{$path}++ or die "docs/specs/INDEX.json: duplicate entry $path\n";
        my $spec = $header{$path} or die "docs/specs/INDEX.json: $path is not a tracked spec\n";
        $entry->{$_} = $spec->{$_} for grep { defined $spec->{$_} } qw(intent delivery claim);
    }
    for my $file (grep { !$listed{$_} } sort @specs) {
        my $prefix = default_prefix($file);
        utf8::decode($prefix);
        push @$entries, {
            path => $file,
            title => new_spec_title($file),
            reqPrefix => $prefix,
            intent => $header{$file}{intent},
            delivery => $header{$file}{delivery},
            claim => $header{$file}{claim},
            dependsOn => [],
            supersededBy => undef,
            closableToday => JSON::PP::false,
            implementation => [],
        };
    }
    my $json_string = sub {
        my ($text) = @_;
        $text =~ s/(["\\])/\\$1/g;
        $text =~ s/\n/\\n/g;
        $text =~ s/\r/\\r/g;
        $text =~ s/\t/\\t/g;
        $text =~ s/([\x00-\x1f])/sprintf('\\u%04x', ord $1)/ge;
        return qq("$text");
    };
    my $json_value = sub {
        my ($value) = @_;
        return 'null' unless defined $value;
        return $value ? 'true' : 'false' if JSON::PP::is_bool($value);
        return $json_string->($value) unless ref $value;
        ref $value eq 'ARRAY' && !grep { ref || !defined } @$value
            or die "docs/specs/INDEX.json: a value is neither a string, a boolean, null nor a string list\n";
        return '[]' unless @$value;
        return "[\n" . join(",\n", map { '      ' . $json_string->($_) } @$value) . "\n    ]";
    };
    my @objects = map {
        my $entry = $_;
        "  {\n" . join(",\n", map { qq(    "$_": ) . $json_value->($entry->{$_}) } @keys) . "\n  }";
    } @$entries;
    my $out = "[\n" . join(",\n", @objects) . "\n]\n";
    utf8::encode($out);
    return $out;
}

# README mode: rewrite the Intent and Delivery cells and the leading claim of the description in
# every table row whose Contract cell links a spec without an anchor, the rows `readREADME` in
# internal/specindex reads.
sub readme_text {
    $tracked{'docs/specs/README.md'} or die "docs/specs/README.md is not in the Git index\n";
    my @rows = split /^/, $blobs->{'docs/specs/README.md'};
    my ($table) = grep { $rows[$_] =~ /\A\| Scope \| Contract \| Intent \| Delivery \|/ } 0 .. $#rows;
    defined $table or die "docs/specs/README.md: no `| Scope | Contract | Intent | Delivery |` table\n";
    my $end = $table;
    ++$end while $end + 1 <= $#rows && $rows[$end + 1] =~ /\A\|/;
    my %rowed;
    my $cell = sub {
        my ($column, $value) = @_;
        $column =~ /\A(\s*)(.*?)(\s*)\z/s;
        return "$1$value$3";
    };
    for my $i ($table + 2 .. $end) {
        my $row = $rows[$i];
        utf8::decode($row) or die "docs/specs/README.md:" . ($i + 1) . ": not valid UTF-8\n";
        my $newline = $row =~ s/\n\z// ? "\n" : '';
        my @columns = split /\|/, $row, -1;
        next if @columns < 7;
        my ($link, $anchor) = $columns[2] =~ /\]\(([^)#]+\.md)(#[^)]*)?\)/ or next;
        next if defined $anchor;
        my $path = $link =~ m{\A\.\./} ? $link : "docs/specs/$link";
        my $spec = $header{$path} or next;
        $rowed{$path} = 1;
        # SRG-V0-043: a requirement range written right after the link, alone or before a `;`, is
        # the span of the ids of that prefix the spec defines, so it cannot drift from the TSV.
        if ($columns[2] =~ /\A(.*?\]\([^)]+\)\s+)([A-Z][A-Z0-9-]*-)([0-9]{3})(?:\.\.([0-9]{3}))?(?=\s*(?:;|\z))/s) {
            my ($lead, $prefix) = ($1, $2);
            my @numbers = sort map { substr $_, length $prefix }
                grep { $best{$_}{file} eq $path && /\A\Q$prefix\E[0-9]{3}\z/ } keys %best;
            @numbers or die "$path: its README row shows a $prefix range but the spec defines no $prefix id\n";
            my $range = $prefix . $numbers[0] . ($numbers[-1] ne $numbers[0] ? "..$numbers[-1]" : '');
            substr($columns[2], length $lead, $+[0] - length $lead) = $range;
        }
        $columns[3] = $cell->($columns[3], $spec->{intent});
        $columns[4] = $cell->($columns[4], $spec->{delivery});
        if (defined $spec->{claim}) {
            my $description = trim($columns[5]);
            $description =~ s/\A.*?[.?!](?=\s|\z)//s
                or die "$path: its README row description does not start with a one-sentence claim\n";
            $columns[5] = $cell->($columns[5], $spec->{claim} . $description);
        }
        $row = join('|', @columns) . $newline;
        utf8::encode($row);
        $rows[$i] = $row;
    }
    my @added;
    for my $file (grep { !$rowed{$_} } sort @specs) {
        my $name = $file =~ s{\Adocs/specs/}{}r;
        my $row = sprintf("| %s | [`%s`](%s) | %s | %s | %s |\n",
            new_spec_title($file), $name, $name, @{$header{$file}}{qw(intent delivery claim)});
        utf8::encode($row);
        push @added, $row;
    }
    $rows[$end] .= "\n" if @added && $rows[$end] !~ /\n\z/;
    splice @rows, $end + 1, 0, @added;
    return join '', @rows;
}

if ($mode eq 'index') {
    print index_text();
} elsif ($mode eq 'readme') {
    print readme_text();
} else {
    # Every output is built before any is written, so a refused projection leaves DIR untouched.
    my %out = ('REQUIREMENTS.tsv' => tsv_text(), 'INDEX.json' => index_text(), 'README.md' => readme_text());
    for my $name (sort keys %out) {
        open my $fh, '>', "$out_dir/$name" or die "$out_dir/$name: $!\n";
        print {$fh} $out{$name} or die "$out_dir/$name: $!\n";
        close $fh or die "$out_dir/$name: $!\n";
    }
}
PERL
