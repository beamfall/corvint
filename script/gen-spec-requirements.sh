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
# staged spec headers onto the staged file (SRG-V0-012); a superseded digest's Successor path
# becomes the INDEX.json supersededBy (SRG-V0-979). `--lines` prints the uncommitted locator
# view: the same rows with each definition's line number, ordered by id (SRG-V0-015). Those modes
# write stdout only. `--all DIR` writes REQUIREMENTS.tsv, INDEX.json and README.md into the existing
# directory DIR from one read of the staged specs, each byte for byte what its own mode prints
# (SRG-V0-018). SRG-V0-954: `--write` (`make spec-requirements`) writes the three into docs/specs/
# after refusing any unstaged or untracked spec, and `--check` (`make spec-requirements-check`)
# byte-compares them with docs/specs/, naming each stale file.
#
# Minimal fixture layout (SRG-V0-743, V1-3388), all read from the Git index, so stage each file:
# this script at script/gen-spec-requirements.sh and the tracked docs/specs/*.md specs, which is
# enough for the default TSV and --lines modes. --index starts from docs/specs/INDEX.json when it is
# tracked and from an empty list otherwise, and a spec new to it needs a `# Title` line and an
# Agent digest `- Claim:`. --readme, --all, --write and --check also need docs/specs/README.md with
# a `| Scope | Contract | Intent | Delivery |` table whose rows link each spec.
# docs/specs/INTENT-STATUS-FROZEN.tsv and script/requirement-definitions-outside-section.txt are
# optional: absent means no frozen header IDs and no baselined outside definitions.

usage() {
  printf 'usage: %s [--index | --readme | --lines | --all DIR | --write | --check]\n' "$0" >&2
  printf '  --write regenerates docs/specs/REQUIREMENTS.tsv, INDEX.json and README.md (make spec-requirements);\n' >&2
  printf '  --check compares them with a fresh generation (make spec-requirements-check)\n' >&2
  exit 2
}
out_dir=
case ${1:-} in
  '') mode=tsv ;;
  --index) mode=index ;;
  --readme) mode=readme ;;
  --lines) mode=lines ;;
  --write | --check) test $# = 1 || usage; mode=${1#--} ;;
  --all)
    test $# = 2 && test -n "$2" || usage
    mode=all
    out_dir=$(CDPATH= cd -- "$2" && pwd) || { printf '%s: --all DIR must be an existing directory\n' "$0" >&2; exit 2; }
    ;;
  *) usage ;;
esac
test "$mode" = all || test $# -le 1 || usage

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

# SRG-V0-954 (V1-3510): `--write` refuses instead, naming every unstaged or untracked spec and the
# `git add` that publishes it, so it never writes generated files that silently omit an edit.
if test "$mode" = write && test -z "${GIT_INDEX_FILE:-}"; then
  listed=$(GIT_OPTIONAL_LOCKS=0 git -c core.quotepath=off diff --name-only -- docs/specs &&
    GIT_OPTIONAL_LOCKS=0 git -c core.quotepath=off ls-files --others --exclude-standard -- docs/specs) ||
    { printf '%s: REFUSE could not list unstaged docs/specs edits\n' "$0" >&2; exit 1; }
  unstaged=$(printf '%s\n' "$listed" | grep -E '^docs/specs/[^/]+\.md$' | grep -v -x docs/specs/README.md | sort -u)
  if test -n "$unstaged"; then
    printf '%s\n' "$unstaged" | sed "s|^|$0: REFUSE unstaged-spec |" >&2
    printf '%s: the generator reads the Git index, so stage the specs first: git add -- %s; then rerun make spec-requirements (SRG-V0-954)\n' \
      "$0" "$(printf '%s\n' "$unstaged" | tr '\n' ' ' | sed 's/ $//')" >&2
    exit 1
  fi
fi
perl_mode=$mode
case $mode in
  write | check)
    perl_mode=all
    out_dir=$(mktemp -d "${TMPDIR:-/tmp}/gen-spec-requirements.XXXXXX") || exit 1
    trap 'rm -rf "$out_dir"' EXIT
    trap 'exit 129' HUP
    trap 'exit 130' INT
    trap 'exit 143' TERM
    ;;
esac

perl - "$perl_mode" "$out_dir" <<'PERL' || exit $?
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
    open my $paths, '-|', 'git', 'ls-files', '-z', '--', 'docs/specs', 'script/requirement-definitions-outside-section.txt'
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

# leading_note is the text of the parenthetical that opens TEXT, nested parentheses included, up to
# its matching close or the end of TEXT (SRG-V0-953), so `(owner chat (2026-10-27), accepted)` is
# read whole rather than cut at the inner close.
sub leading_note {
    my ($text) = @_;
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
    return substr($text, 1, $end - 1);
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

# field_line is the 1-based line number of the header line header_value reads, or of the Agent
# digest `- KEY:` line, so a refusal can name it as path:line (SRG-V0-950).
sub field_line {
    my ($blob, $key, $digest) = @_;
    my $offset;
    if ($digest) {
        $blob =~ /^[^\S\n]*## Agent digest[^\S\n]*(?:\n|\z)/mg or return 0;
        my $from = pos $blob;
        substr($blob, $from) =~ /^[^\S\n]*- \Q$key\E: /m or return 0;
        $offset = $from + $-[0];
    } else {
        (my $bold = $key) =~ s/:\z//;
        $bold = "**$bold:**";
        $blob =~ /^[^\S\n]*(?:- )?[^\S\n]*(?:\Q$key\E|\Q$bold\E)/m or return 0;
        $offset = $-[0];
    }
    return 1 + (substr($blob, 0, $offset) =~ tr/\n//);
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

my @reads = ((sort @specs), grep { $tracked{$_} } 'docs/specs/INTENT-STATUS-FROZEN.tsv',
    'script/requirement-definitions-outside-section.txt');
# SRG-V0-761 (V1-4001): README mode reads INDEX.json too, for the claim the row was projected from.
push @reads, 'docs/specs/INDEX.json'
    if ($mode eq 'index' || $mode eq 'readme' || $mode eq 'all') && $tracked{'docs/specs/INDEX.json'};
push @reads, 'docs/specs/README.md'
    if ($mode eq 'readme' || $mode eq 'all') && $tracked{'docs/specs/README.md'};
my $blobs = index_blobs(@reads);
my @noted_definitions;

my (%best, %header, %definitions, %section, @outside_section, @continuation_notes, @malformed, %obligations);
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
        successor => digest_values($blob)->{Successor},
        digest => scalar($blob =~ /^[^\S\n]*## Agent digest[^\S\n]*(?:\n|\z)/m),
        intent_line => field_line($blob, 'Intent status:'), status_line => field_line($blob, 'Status', 1),
        claim_line => field_line($blob, 'Claim', 1),
    };
    # SRG-V0-891 (V1-3295): the ## Requirements section's line range, tracked as
    # script/check-requirement-definitions.sh (RDG-V0-021) tracks it, so the generator refuses an
    # OCM-form definition after that section before doc-gates does.
    my ($in_section, $sections, $section_fence, @outside) = (0, 0, '');
    # SRG-V0-740: the OCM obligations of the section, counted as internal/lrfrepo/ocm.go counts them.
    my %section_ids;
    my $section_line = 0;
    my $line_number = 0;
    my $previous_blank = 1;
    my $rollback = 0;
    my @noted;
    my ($fence, $continues, $item_open, $pending) = ('', 0, 0, undef);
    for my $line (split /^/, $blob) {
        ++$line_number;
        # SRG-V0-953 (V1-3553): only the definition line is read, so an acceptance note that opens
        # the continuation line of a definition with no leading note of its own is refused.
        if (defined $pending) {
            push @continuation_notes, "$file:$line_number: $pending: the acceptance note starts on the "
                . "continuation line, and only the definition line is read"
                if $continues && $line !~ /^\s*[-*]\s/ && $line =~ /^\s*(\(.*)/s
                && leading_note($1) =~ /\b(?:accepted|approved|standing)\b|\bauto-?accept/i;
            undef $pending;
        }
        # SRG-V0-003: unmarked definitions start paragraphs, not wrapped prose.
        my $definition_boundary = $previous_blank || $line =~ /^\s*(?:[-*]\s+|\*\*|`)/;
        # SRG-V0-548: under RDG-V0-023 (script/check-requirement-definitions.sh, whose fence
        # tracking this mirrors) an unbulleted line that directly follows a non-blank, non-heading
        # line outside a fence continues that paragraph or list item, so a marked id that starts it
        # may be a wrapped citation such as `ID`. ending a sentence. Such a line still ranks 0, but
        # after every unwrapped rank 0 line, and it is never counted as a second definition.
        my $wrapped = $continues && $line !~ /^\s*[-*]\s/ ? 1 : 0;
        # SRG-V0-952 (V1-3563): an indented, unbulleted paragraph after a blank line inside a list
        # item continues that item, so a marked id that starts it is no second definition either.
        $wrapped = 1 if $previous_blank && $item_open && $fence eq '' && $line =~ /^ {2,}\S/ && $line !~ /^\s*[-*]\s/;
        $previous_blank = $line =~ /^\s*$/;
        my ($run, $rest) = $line =~ /^ {0,3}(`{3,}|~{3,})(.*?)\r?\n?\z/ ? ($1, $2) : ('', '');
        if ($fence ne '') {
            $fence = '' if $run ne '' && substr($run, 0, 1) eq substr($fence, 0, 1)
                && length($run) >= length($fence) && $rest =~ /\A[ \t]*\z/;
        } elsif ($run ne '' && !($run =~ /^`/ && $rest =~ /`/)) {
            $fence = $run;
        }
        $continues = $fence eq '' && $run eq '' && $line =~ /\S/ && $line !~ /^ {0,3}#/;
        # SRG-V0-966 (V1-3660): a rollback section restates IDs (`ID: revert ...`, bulleted or
        # not) without defining them, so its lines never rank 0 and never count as an
        # out-of-section definition (SRG-V0-891); the next heading ends the section.
        $rollback = $1 =~ /^Rollback\b/i ? 1 : 0 if $fence eq '' && $run eq '' && $line =~ /^ {0,3}#{2,6}\s+(.*)/;
        if ($fence eq '' && $run eq '') {
            if ($line =~ /^## Requirements\r?\n?\z/) {
                ($in_section, $sections) = (1, $sections + 1);
                $section{$file} = [$line_number, 0];
            } elsif ($line =~ /^##(?: |\r?\n?\z)/) {
                $section{$file}[1] = $line_number - 1 if $in_section;
                $in_section = 0;
            } elsif ($line =~ /^- (?:`([A-Z][A-Z0-9-]*-[0-9]{3,})`: |\*\*([A-Z][A-Z0-9-]*-[0-9]{3,})[:.]\*\* )/) {
                if ($in_section) { $section_ids{$1 // $2} = 1 }
                elsif (!$rollback) { push @outside, [$1 // $2, $line_number] }
            }
            # SRG-V0-791: a definition-shaped bold or backticked id the token scan cannot read; a lettered
            # sub-requirement (`VAF-009A`) stays unindexed under its parent row.
            if ($definition_boundary && !$wrapped
                && $line =~ /^\s*(?:[-*]\s+)?(?:\*\*([A-Z][A-Z0-9-]*-[0-9][A-Za-z0-9_-]*)[:.]\*\*|`([A-Z][A-Z0-9-]*-[0-9][A-Za-z0-9_-]*)`:\s)/) {
                my $id = $1 // $2;
                push @malformed, [$file, $line_number, $id] if $id !~ /-[0-9]{3,}[A-Z]?\z/;
            }
        }
        if ($fence ne '' || $run ne '' || $line =~ /^ {0,3}#/) {
            $item_open = 0;
        } elsif ($line =~ /^\s*[-*]\s/) {
            $item_open = 1;
        } elsif ($line =~ /^\S/) {
            $item_open = 0;
        }

        # SRG-V0-760 (V1-4000): a bulleted line whose note sits between the id and its colon, as in
        # `- `ID` (KEEP): text`, ranks no 0; it is refused below when the id has no definition.
        push @noted, [$1, $line_number, $in_section]
            if $fence eq '' && $run eq '' && !$rollback
            && $line =~ /^\s*[-*]\s+(?:\*\*)?`?([A-Z][A-Z0-9-]*-[0-9]{3,})`?(?:\*\*)?\s+\([^)\n]*\)(?:\*\*)?\s*:/;
        while ($line =~ /\b([A-Z][A-Z0-9-]*-[0-9]{3,})\b/g) {
            my $id = $1;
            my $quoted = quotemeta($id);
            my $is_period = $definition_boundary && $line =~ /^\s*(?:[-*]\s+)?(?:\*\*)?`?$quoted`?\.(?:\*\*)?\s/;
            my $rank = $is_period ? 0
                     : $definition_boundary && $line =~ /^\s*(?:[-*]\s+)?(?:\*\*)?`?$quoted`?(?:\*\*)?\s*:/ ? 0
                     : $line =~ /^\s*\|\s*`?$quoted`?(?:\s|\||\.)/ ? 1
                     : 2;
            $rank = 2 if $rank == 0 && $rollback;
            next if $rank > 1;
            if ($rank == 0 && !$wrapped && !grep { $_ eq "$file:$line_number" } @{$definitions{$id} // []}) {
                push @{$definitions{$id}}, "$file:$line_number";
                (my $text = $line) =~ s/^\s*(?:[-*]\s+)?(?:\*\*)?`?$quoted`?(?:\*\*)?\s*[.:](?:\*\*)?\s*//;
                $pending = $id if $text !~ /\A\(/;
            }
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
    push @noted_definitions, map { [$file, @$_] } grep { !$sections || $_->[2] } @noted;
    $section{$file}[1] ||= $line_number if $in_section;
    if ($sections == 1) {
        push @outside_section, [$file, @$_] for @outside;
        $obligations{$file} = scalar keys %section_ids;
    } else {
        delete $section{$file};
    }
}

# SRG-V0-790 (V1-3793): an id of four or more digits is a requirement only under a prefix that
# also defines a three-digit id on a rank 0 line, so ticket ids (`V1-0789`) and record ids
# (`DR-0046`) in tables and prose stay text, as they were when the scan read three digits only.
my %known_prefix;
for my $id (keys %best) {
    my ($prefix) = $id =~ /\A(.*-)[0-9]{3}\z/;
    $known_prefix{$prefix} = 1 if defined $prefix && $best{$id}{key} =~ /\A0\0/;
}
sub id_prefix { return $_[0] =~ /\A(.*-)[0-9][^-]*\z/ ? $1 : '' }
sub requirement_id { return $_[0] =~ /-[0-9]{3}\z/ || $known_prefix{id_prefix($_[0])} }
for my $id (grep { !requirement_id($_) } keys %best) {
    delete $best{$id};
    delete $definitions{$id};
}
@outside_section = grep { requirement_id($_->[1]) } @outside_section;
# SRG-V0-791 (V1-3793): a definition-shaped id under a known prefix that is not the prefix and three
# or more digits (`CIN-V0-01`, `CIN-V0-1010a`, `CIN-V0-1010-bad`) is refused with its file and line,
# never dropped. malformed_prefix is the longest known prefix the id starts with before a digit.
sub malformed_prefix {
    my ($id) = @_;
    for my $prefix (sort { length $b <=> length $a || $a cmp $b } keys %known_prefix) {
        return $prefix if $id =~ /\A\Q$prefix\E[0-9]/;
    }
    return '';
}
if (my @refused = grep { malformed_prefix($_->[2]) ne '' } @malformed) {
    print STDERR "$_->[0]:$_->[1]: $_->[2] looks like a requirement definition of prefix "
        . malformed_prefix($_->[2]) . " but is not that prefix followed by three or more digits\n" for @refused;
    die "write each requirement id as PREFIX-NNN with three or more digits, then rerun make "
        . "spec-requirements (SRG-V0-791)\n";
}
# id_order sorts requirement ids by prefix, then by number, so CIN-V0-999 precedes CIN-V0-1010.
sub id_order {
    my ($left, $right) = @_;
    my ($left_prefix, $left_number) = $left =~ /\A(.*-)([0-9]+)\z/ ? ($1, $2) : ($left, -1);
    my ($right_prefix, $right_number) = $right =~ /\A(.*-)([0-9]+)\z/ ? ($1, $2) : ($right, -1);
    return $left_prefix cmp $right_prefix || $left_number <=> $right_number || $left cmp $right;
}

# where_requirements FILE: where a new requirement of FILE is defined, for the refusals below.
sub where_requirements {
    my ($file) = @_;
    my $range = $section{$file};
    return $range ? "its own definition line inside ${file}'s ## Requirements section (lines $range->[0]..$range->[1])"
        : "its own definition line in ${file}'s ## Requirements section";
}

# SRG-V0-015: the committed index carries no line number and orders its rows by file, then by
# definition position, so an edit that shifts a spec's lines rewrites no row and two changes that
# add requirements to different sections of one spec insert their rows at different places.
# `--lines` is the derived locator view with the line column, ordered by id; it is never committed.
sub tsv_text {
    my @ids = sort {
        $best{$a}{file} cmp $best{$b}{file} || $best{$a}{line} <=> $best{$b}{line} || id_order($a, $b)
    } keys %best;
    return join '', "id\tfile\ttitle\tacceptance\tacceptedBy\n",
        map { join("\t", $_, @{$best{$_}}{qw(file title acceptance accepted_by)}) . "\n" } @ids;
}
# SRG-V0-951 (V1-3602): every refusal below is collected, and one run prints them all, each
# category's repair once after its lines, then exits non-zero before printing or writing anything.
my (@refusals, @repairs, %repaired);
sub refuse {
    my ($repair, @lines) = @_;
    return unless @lines;
    push @refusals, @lines;
    push @repairs, $repair if $repair ne '' && !$repaired{$repair}++;
}

# SRG-V0-040: the specindex claim bound (1..160 characters), refused in every mode, before the
# TSV and --lines modes exit, with the path and character length, so it fails at the first spec
# step instead of at spec-requirements-check.
for my $file (sort @specs) {
    next unless defined(my $claim = $header{$file}{claim});
    utf8::decode($claim);
    # SRG-V0-763 (V1-3306): the refusal names the Claim line and the template that states the limit.
    refuse("shorten each Claim to one sentence of at most 160 characters and move detail into the "
        . "spec body; docs/SPEC-DRIVEN-DEVELOPMENT.md \"Required capability-spec shape\" states the "
        . "limit (SRG-V0-040, SRG-V0-763)",
        "$file:$header{$file}{claim_line}: the Agent digest Claim is " . length($claim) . " characters; the limit is 160")
        if length($claim) > 160;
}
# status_ids returns the requirement IDs a status names: each SRG-V0-003 token and each `..NNN` or
# `/NNN` suffix after it, read with the token's prefix, so `X-V0-001..003` and `X-V0-001/002` name
# X-V0-003 and X-V0-002 too.
sub status_ids {
    my ($text) = @_;
    my @ids;
    while ($text =~ /\b([A-Z][A-Z0-9-]*-)([0-9]{3,})((?:(?:\.\.|\/)[0-9]{3,}\b)*)\b/g) {
        my ($prefix, $first, $rest) = ($1, $2, $3);
        push @ids, grep { requirement_id($_) } map { "$prefix$_" } $first, $rest =~ /([0-9]{3,})/g;
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
        # ASD-V0-844 (V1-3875): a spec with no frozen row (a new one) is held to the same rule,
        # but its refusal says so instead of naming frozen header text it never had.
        my $why = exists $frozen{$file}
            ? 'which its frozen header text does not; this line is frozen (docs/specs/INTENT-STATUS-FROZEN.tsv)'
            : 'and it has no docs/specs/INTENT-STATUS-FROZEN.tsv row, so, as for every new spec, this line holds the current status only and names no requirement ID';
        # SRG-V0-950 (V1-3575): each refusal names the frozen line as path:line.
        for my $field (['Intent status', $header{$file}{intent}, $header{$file}{intent_line}],
                ['Agent digest Status', $header{$file}{status}, $header{$file}{status_line}]) {
            my ($name, $value, $at) = @$field;
            next unless defined $value;
            my %seen;
            push @refused, "$file:$at: its $name names $_, $why, so $_ belongs on " . where_requirements($file)
                for grep { !$row->{ids}{$_} && !$seen{$_}++ } status_ids($value);
        }
        next unless defined(my $intent = $header{$file}{intent});
        utf8::decode($intent);
        push @refused, "$file:$header{$file}{intent_line}: its Intent status is " . length($intent)
            . " characters; the limit is " . ($row->{chars} > 160 ? "its frozen length, $row->{chars}" : '160')
            if length($intent) > 160 && length($intent) > $row->{chars};
    }
    refuse("the Intent status header and digest Status lines are frozen and hold the current status "
        . "only, so a new requirement ID or range never goes there: it belongs on each requirement's "
        . "own definition line, whose leading note records the acceptance, for example "
        . "`- `ID`: (accepted under the owner's standing auto-accept (YYYY-MM-DD); V1-NNNN) ...`, "
        . "which REQUIREMENTS.tsv carries in its acceptance and acceptedBy columns, in the spec's "
        . "acceptance and traceability rows, and in its build-log entry (SRG-V0-009, SRG-V0-981, "
        . "SRG-V0-950); remove the new header text and rerun make spec-requirements; a renamed spec "
        . "moves its INTENT-STATUS-FROZEN.tsv row to the new path; see docs/SPEC-DRIVEN-DEVELOPMENT.md "
        . "(SRG-V0-980)", @refused);
}
# SRG-V0-891 (V1-3295): an OCM-form definition after a spec's one ## Requirements section is refused
# in every mode, naming the line and the section range, unless the frozen RDG-V0-021 baseline
# script/requirement-definitions-outside-section.txt lists it.
{
    my %baseline;
    for my $row (split /\n/, $blobs->{'script/requirement-definitions-outside-section.txt'} // '') {
        next if $row =~ /^\s*(?:#|\z)/;
        $baseline{$row} = 1;
    }
    my @misplaced = grep { !$baseline{"$_->[0] $_->[1]"} } @outside_section;
    if (@misplaced) {
        print STDERR "$_->[0]:$_->[2]: $_->[1] is defined outside the ## Requirements section (lines "
            . "$section{$_->[0]}[0]..$section{$_->[0]}[1])\n" for @misplaced;
        die "a requirement is defined inside its spec's ## Requirements section, which OCM enumerates; "
            . "move each definition there, then rerun make spec-requirements (SRG-V0-891, RDG-V0-021)\n";
    }
}
# SRG-V0-968 (V1-3661): every live spec's Agent digest carries a Claim, so its README row's leading
# summary is projected from it (SRG-V0-012); a superseded digest (Successor and Why only, as
# internal/specindex requires) is exempt. Refused in every mode.
{
    my @claimless = grep { $header{$_}{digest} && !defined $header{$_}{claim} && !defined $header{$_}{successor} } sort @specs;
    if (@claimless) {
        print STDERR "$_: its ## Agent digest has no `- Claim:` line\n" for @claimless;
        die "every spec's Agent digest states a one-sentence `- Claim:`, which README.md and INDEX.json "
            . "project (a superseded spec's digest holds Successor and Why instead), then rerun make "
            . "spec-requirements (SRG-V0-968)\n";
    }
}
# SRG-V0-740 (V1-3769): `ocm prepare` refuses a ## Requirements section with more than 256
# obligations, which otherwise surfaces only at spec-requirements-check's Go test after the TSV is
# written (SRG-V0-965). Refuse it here in every mode, and warn from 240 so the companion spec is
# planned before the limit.
{
    my $route = 'define new requirements in a companion spec with its own requirement prefix, never '
        . 'in the frozen Intent status header (SRG-V0-980)';
    my @over = grep { $obligations{$_} > 256 } sort keys %obligations;
    print STDERR "$_: warning: its ## Requirements section holds $obligations{$_} of the 256 obligations "
        . "`ocm prepare` admits; $route (SRG-V0-740)\n"
        for grep { $obligations{$_} >= 240 && $obligations{$_} <= 256 } sort keys %obligations;
    if (@over) {
        print STDERR "$_: its ## Requirements section holds $obligations{$_} obligations; the limit is 256\n" for @over;
        die "`ocm prepare` refuses a Requirements section over 256 obligations; $route, then rerun "
            . "make spec-requirements (SRG-V0-740)\n";
    }
}
# SRG-V0-547 (V1-3049): a requirement ID defined by more than one rank 0 line, in one spec or
# across specs, is refused in every mode, naming every definition, instead of keeping the first
# and silently dropping the rest. Two lanes that each define the same ID collide here.
{
    my @duplicate = grep { @{$definitions{$_}} > 1 } sort keys %definitions;
    refuse("a requirement ID must have exactly one definition line across docs/specs; renumber or "
        . "remove the extra definitions, then rerun make spec-requirements (SRG-V0-547)",
        map { "$_: defined more than once: @{$definitions{$_}}" } @duplicate);
}
{
    my @near = grep { $best{$_}{near_miss} ne '' } sort keys %best;
    my $note_repair = "write the note in parentheses right after the id, closed on the id's line: "
        . "`(accepted, decision NNNN; V1-NNNN)`, `(accepted YYYY-MM-DD; V1-NNNN)` or "
        . "`(accepted under the owner's standing auto-accept (YYYY-MM-DD); V1-NNNN)`; a longer "
        . "clause may carry the anchor, as in `(accepted by the owner, amending X-V0-001 (owner "
        . "chat, YYYY-MM-DD); V1-NNNN)`; see docs/SPEC-DRIVEN-DEVELOPMENT.md (SRG-V0-042)";
    refuse($note_repair, map { "$best{$_}{file}:$best{$_}{line}: $_: $best{$_}{near_miss}" } @near);
    refuse("move the note to the start of the text on the id's line, as in "
        . "`- `ID`: (accepted YYYY-MM-DD; V1-NNNN) The requirement ...` (SRG-V0-953); " . $note_repair,
        @continuation_notes);
}
# SRG-V0-760 (V1-4000): inside ## Requirements (or anywhere in a spec without one), outside fences and
# rollback sections, a `- `ID` (note):` line whose id has no definition is refused naming the line,
# so the id is never silently missing from REQUIREMENTS.tsv (RDG-V0-022 names it at doc-gates).
refuse("a note between the id and its colon is no definition: write `- `ID`: (note) text`, then "
    . "rerun make spec-requirements; see docs/SPEC-DRIVEN-DEVELOPMENT.md \"Accepted forms\" (SRG-V0-760)",
    map { "$_->[0]:$_->[2]: $_->[1]: the note sits before the colon, so this line defines nothing" }
    grep { !$definitions{$_->[1]} } @noted_definitions);
if ($mode ne 'tsv' && $mode ne 'lines') {
    # SRG-V0-012: INDEX.json and the README.md spec table are projections. A spec header owns its
    # intent and delivery status and its Agent digest owns the claim; INDEX.json owns every other
    # field and README.md every other cell, which the projection keeps byte for byte. A spec with no
    # entry or row gains one built from its header, appended after the existing ones.
    for my $file (sort @specs) {
        my $spec = $header{$file};
        defined $spec->{intent} && defined $spec->{delivery}
            or refuse('', "$file: declare Intent status and Delivery status before regenerating"), next;
        for my $field (qw(intent delivery claim)) {
            next unless defined $spec->{$field};
            utf8::decode($spec->{$field}) or refuse('', "$file: $field is not valid UTF-8"), next;
            $spec->{$field} =~ /[|\x00-\x1f]/
                and refuse('', "$file: $field holds a table pipe or control character");
        }
        # The README description is the claim then any tail; a one-sentence claim is what lets the
        # projection find where the old claim ends without reading INDEX.json.
        next unless defined $spec->{claim};
        $spec->{claim} =~ /[.?!]\z/ && $spec->{claim} !~ /[.?!]\s/
            or refuse('', "$file: the Agent digest Claim must be one sentence ending in . ? or !");
    }
}
if (@refusals) {
    print STDERR "$_\n" for @refusals, @repairs;
    exit 1;
}
if ($mode eq 'tsv') {
    print tsv_text();
    exit 0;
}
if ($mode eq 'lines') {
    print "id\tfile\tline\ttitle\tacceptance\tacceptedBy\n";
    for my $id (sort { id_order($a, $b) } keys %best) {
        my $row = $best{$id};
        print join("\t", $id, @{$row}{qw(file line title acceptance accepted_by)}), "\n";
    }
    exit 0;
}


sub default_prefix {
    my ($file) = @_;
    return $header{$file}{prefix} if defined $header{$file}{prefix};
    my ($id) = grep { $best{$_}{file} eq $file && $best{$_}{key} =~ /\A0\0/ } sort keys %best;
    return defined $id ? $id =~ s/-[0-9]{3,}\z//r : '';
}

# successor_path is the repository path a digest `- Successor:` value names: its first backticked
# `*.md` path, with a bare file name taken as a sibling in docs/specs/ (SRG-V0-979), or undef.
sub successor_path {
    my ($value) = @_;
    return undef unless defined $value && $value =~ /`([^`\s]+\.md)`/;
    my $path = $1;
    return $path =~ m{/} ? $path : "docs/specs/$path";
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
        # SRG-V0-979 (V1-3797): a superseded digest's Successor owns supersededBy, so retiring a
        # spec needs no hand edit of INDEX.json for internal/specindex to accept it.
        my $successor = successor_path($spec->{successor});
        $entry->{supersededBy} = $successor if defined $successor;
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
            supersededBy => successor_path($header{$file}{successor}),
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

# index_entries: the staged INDEX.json entries by path, or none when it is not tracked.
sub index_entries {
    return {} unless $tracked{'docs/specs/INDEX.json'};
    my $entries = eval { JSON::PP->new->utf8->decode($blobs->{'docs/specs/INDEX.json'}) };
    ref $entries eq 'ARRAY' or die "docs/specs/INDEX.json: not a JSON array\n";
    return { map { ref $_ eq 'HASH' && defined $_->{path} ? ($_->{path} => $_) : () } @$entries };
}

# spec_range FILE PREFIX: the span of the PREFIX ids FILE defines, or '' when it defines none.
sub spec_range {
    my ($file, $prefix) = @_;
    my @numbers = sort map { substr $_, length $prefix }
        grep { $best{$_}{file} eq $file && /\A\Q$prefix\E[0-9]{3}\z/ } keys %best;
    return '' unless @numbers;
    return $prefix . $numbers[0] . ($numbers[-1] ne $numbers[0] ? "..$numbers[-1]" : '');
}

# README mode: rewrite the Intent and Delivery cells and the leading claim of the description in
# every table row whose Contract cell links a spec without an anchor, the rows `readREADME` in
# internal/specindex reads.
sub readme_text {
    $tracked{'docs/specs/README.md'} or die "docs/specs/README.md is not in the Git index; --readme, --all, "
        . "--write and --check need it with its spec table (the minimal fixture layout is the comment at the "
        . "top of script/gen-spec-requirements.sh, SRG-V0-743)\n";
    my @rows = split /^/, $blobs->{'docs/specs/README.md'};
    my ($table) = grep { $rows[$_] =~ /\A\| Scope \| Contract \| Intent \| Delivery \|/ } 0 .. $#rows;
    defined $table or die "docs/specs/README.md: no `| Scope | Contract | Intent | Delivery |` table\n";
    my $end = $table;
    ++$end while $end + 1 <= $#rows && $rows[$end + 1] =~ /\A\|/;
    my %rowed;
    my $indexed = index_entries();
    # SRG-V0-762 (V1-2310): the prefix a range-less row gains, INDEX.json's reqPrefix first.
    my $row_prefix = sub {
        my ($file) = @_;
        my $prefix = $indexed->{$file} && defined $indexed->{$file}{reqPrefix} ? $indexed->{$file}{reqPrefix} : '';
        $prefix = default_prefix($file) if $prefix eq '';
        utf8::decode($prefix);
        return $prefix eq '' ? '' : "$prefix-";
    };
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
        if ($columns[2] =~ /\A(.*?\]\([^)]+\)\s+)([A-Z][A-Z0-9-]*-)([0-9]{3,})(?:\.\.([0-9]{3,}))?(?=\s*(?:;|\z))/s) {
            my ($lead, $prefix) = ($1, $2);
            # SRG-V0-790: equal numbers of different widths (`001`, `0001`) tie-break as text, so the
            # range never follows hash order.
            my @numbers = sort { $a <=> $b || $a cmp $b } map { substr $_, length $prefix }
                grep { $best{$_}{file} eq $path && /\A\Q$prefix\E[0-9]{3,}\z/ } keys %best;
            @numbers or die "$path: its README row shows a $prefix range but the spec defines no $prefix id\n";
            my $range = $prefix . $numbers[0] . ($numbers[-1] ne $numbers[0] ? "..$numbers[-1]" : '');
            substr($columns[2], length $lead, $+[0] - length $lead) = $range;
        } elsif ($columns[2] =~ /\A(\s*\[[^\]]*\]\([^)]+\))(?=\s*(?:;|\z))/s) {
            # SRG-V0-762 (V1-2310): a Contract cell that is the bare link (alone or before a `;`)
            # gains the range of the ids the spec defines, which SRG-V0-043 then keeps current.
            my $prefix = $row_prefix->($path);
            my $range = $prefix eq '' ? '' : spec_range($path, $prefix);
            substr($columns[2], length $1, 0) = " $range" if $range ne '';
        }
        $columns[3] = $cell->($columns[3], $spec->{intent});
        $columns[4] = $cell->($columns[4], $spec->{delivery});
        if (defined $spec->{claim}) {
            # SRG-V0-761 (V1-4001): only the projected claim is replaced: the current Claim, or the
            # claim INDEX.json recorded when the row was last projected. A row whose first sentence
            # is neither was edited by hand and is refused rather than rewritten; a spec with no
            # INDEX.json entry keeps the first-sentence rule.
            my $description = trim($columns[5]);
            my $old = $indexed->{$path} ? $indexed->{$path}{claim} : undef;
            if ($description =~ s/\A\Q$spec->{claim}\E(?=\s|\z)//) {
            } elsif (defined $old && $old ne '' && $description =~ s/\A\Q$old\E(?=\s|\z)//) {
            } elsif (!$indexed->{$path}) {
                $description =~ s/\A.*?[.?!](?=\s|\z)//s
                    or die "$path: its README row description does not start with a one-sentence claim\n";
            } else {
                die "docs/specs/README.md:" . ($i + 1) . ": the $path row's description starts with neither "
                    . "the spec's Agent digest Claim nor the claim INDEX.json recorded, so it was edited by "
                    . "hand; restore the claim as its first sentence and put the change in the spec's "
                    . "`- Claim:` line, then rerun make spec-requirements (SRG-V0-761)\n";
            }
            $columns[5] = $cell->($columns[5], $spec->{claim} . $description);
        }
        $row = join('|', @columns) . $newline;
        utf8::encode($row);
        $rows[$i] = $row;
    }
    my @added;
    for my $file (grep { !$rowed{$_} } sort @specs) {
        my $name = $file =~ s{\Adocs/specs/}{}r;
        my $prefix = $row_prefix->($file);
        my $range = $prefix eq '' ? '' : spec_range($file, $prefix);
        my $row = sprintf("| %s | [`%s`](%s)%s | %s | %s | %s |\n",
            new_spec_title($file), $name, $name, $range eq '' ? '' : " $range", @{$header{$file}}{qw(intent delivery claim)});
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

generated='REQUIREMENTS.tsv INDEX.json README.md'
case $mode in
  write)
    # SRG-V0-967: one line names each rewritten file, or says nothing changed.
    updated=
    for file in $generated; do
      cmp -s "$out_dir/$file" "docs/specs/$file" && continue
      cp "$out_dir/$file" "docs/specs/$file" || exit 1
      updated="$updated $file"
    done
    # SRG-V0-820 (V1-4068): the regenerated files are staged with the specs they came from, so a
    # commit of the staged spec carries them; a caller with a private GIT_INDEX_FILE (dogfood-finish)
    # stages nothing.
    staged=
    if test -z "${GIT_INDEX_FILE:-}"; then
      for file in $generated; do
        git diff --quiet -- "docs/specs/$file" 2>/dev/null && git ls-files --error-unmatch -- "docs/specs/$file" >/dev/null 2>&1 && continue
        git add -- "docs/specs/$file" || { printf '%s: REFUSE could not stage docs/specs/%s\n' "$0" "$file" >&2; exit 1; }
        staged="$staged docs/specs/$file"
      done
    fi
    if test -n "$updated"; then echo "spec-requirements: updated$updated"; else echo "spec-requirements: no changes"; fi
    test -z "$staged" || echo "spec-requirements: staged$staged"
    ;;
  check)
    stale=
    for file in $generated; do
      cmp -s "$out_dir/$file" "docs/specs/$file" || stale="$stale docs/specs/$file"
    done
    if test -n "$stale"; then
      printf '%s: STALE%s: stage the changed spec, then run make spec-requirements (SRG-V0-954)\n' "$0" "$stale" >&2
      exit 1
    fi
    ;;
esac
