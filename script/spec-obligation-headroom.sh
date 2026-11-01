#!/usr/bin/env bash
# spec-obligation-headroom.sh counts the OCM obligations of every staged spec's `## Requirements`
# section the way `ocm prepare` does (scanRequirements in internal/lrfrepo/ocm.go) and reports the
# headroom before `make spec-requirements` writes anything (SRG-V0-680, V1-3769):
#
#   - a section holding 240 to 256 obligations prints one warning line naming the spec, its count
#     and the companion-spec route, and the run still passes;
#   - a section holding more than 256 is refused, naming each spec, and the run exits 1.
#
# The rule mirrored from ocm.go: a spec counts only when exactly one unfenced line is
# `## Requirements`; the section runs to the next unfenced `##` heading; an obligation is a line
# `- **ID:** `, `- **ID.** ` or `- `ID`: ` whose ID matches ^[A-Z][A-Z0-9-]{2,31}-[0-9]{3}$, fenced
# or not (ocm.go reads fences only to find headings), and a repeated ID counts once. A fence is a run of at least three backticks or tildes indented at most
# three spaces (a backtick fence's info string holds no backtick), closed by a run of the same
# character at least as long followed only by spaces or tabs. The Go check
# TestSpecsStayWithinTheOCMObligationBound stays the authority; this script only reports earlier.
#
# Specs are read from the Git index, like script/gen-spec-requirements.sh, so stage them first.
set -eu

usage() { printf 'usage: %s\n' "$0" >&2; exit 2; }
[ "$#" -eq 0 ] || usage
root=$(git rev-parse --show-toplevel)
cd "$root"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-spec-headroom.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
GIT_OPTIONAL_LOCKS=0 git -c core.quotepath=off ls-files -z -- 'docs/specs/*.md' > "$tmp/specs"

perl -e '
use strict;
use warnings;
my ($limit, $warn_from) = (256, 240);
my ($list, $requests) = @ARGV;
open my $names, "<", $list or die "$list: $!\n";
my @specs = do { local $/ = "\0"; grep { $_ ne "docs/specs/README.md" && m{\Adocs/specs/[^/]+\.md\z} } map { chomp; $_ } <$names> };
close $names;
open my $out, ">", $requests or die "$requests: $!\n";
print {$out} ":$_\n" for @specs;
close $out or die "$requests: $!\n";
open my $blobs, "-|", "sh", "-c", "exec git cat-file --batch < \"\$1\"", "sh", $requests or die "git cat-file: $!\n";
binmode $blobs;
my (@warned, @over);
for my $spec (@specs) {
    my $header = <$blobs>;
    defined $header && $header =~ /\A\S+ blob (\d+)\n\z/ or die "$spec: not readable from the Git index\n";
    my $size = $1;
    my $data = "";
    read($blobs, $data, $size) == $size or die "$spec: short read\n";
    read($blobs, my $newline, 1);
    my $count = obligations($data);
    next unless defined $count;
    push @over, [$spec, $count] if $count > $limit;
    push @warned, [$spec, $count] if $count >= $warn_from && $count <= $limit;
}
my $route = "define new requirements in a companion spec with its own requirement prefix, never in a "
    . "section of this one, then rerun make spec-requirements";
printf STDERR "%s: warning: its ## Requirements section holds %d of the %d obligations `ocm prepare` admits; "
    . "%s (SRG-V0-680)\n", $_->[0], $_->[1], $limit, $route for @warned;
if (@over) {
    printf STDERR "%s: its ## Requirements section holds %d obligations, over the %d that `ocm prepare` admits\n",
        $_->[0], $_->[1], $limit for @over;
    print STDERR "spec-requirements: refused before writing: $route (SRG-V0-680)\n";
    exit 1;
}
exit 0;

# fence_run returns the fence character, run length and the rest of a line, or () when the line
# opens no fence run (markdownFenceRun in internal/lrfrepo/ocm.go).
sub fence_run {
    my ($line) = @_;
    return () unless $line =~ /\A( {0,3})([`~])/;
    my $char = $2;
    $line =~ /\A {0,3}(\Q$char\E+)(.*)\z/s;
    return ($char, length $1, $2);
}

# obligations counts the requirement IDs of the one ## Requirements section, or undef when the spec
# has none or more than one, which `ocm prepare` refuses for another reason.
sub obligations {
    my ($data) = @_;
    my @lines = split /\n/, $data, -1;
    pop @lines if @lines && $lines[-1] eq "";
    my ($char, $length, @fenced) = ("", 0);
    for my $line (@lines) {
        $line =~ s/\r\z//;
        if ($length) {
            push @fenced, 1;
            my ($c, $n, $rest) = fence_run($line);
            $length = 0 if defined $c && $c eq $char && $n >= $length && $rest =~ /\A[ \t]*\z/;
            next;
        }
        my ($c, $n, $rest) = fence_run($line);
        if (defined $c && $n >= 3 && !($c eq "`" && index($rest, "`") >= 0)) {
            ($char, $length) = ($c, $n);
            push @fenced, 1;
        } else {
            push @fenced, 0;
        }
    }
    my @headings = grep { !$fenced[$_] && $lines[$_] eq "## Requirements" } 0 .. $#lines;
    return undef unless @headings == 1;
    my %seen;
    for my $index ($headings[0] + 1 .. $#lines) {
        my $line = $lines[$index];
        last if !$fenced[$index] && ($line eq "##" || $line =~ /\A## /);
        my ($start, @markers);
        if ($line =~ /\A- \*\*/) { ($start, @markers) = (4, ":** ", ".** ") }
        elsif ($line =~ /\A- `/) { ($start, @markers) = (3, "`: ") }
        else { next }
        my $closing = -1;
        for my $marker (@markers) {
            my $at = index(substr($line, $start), $marker);
            $closing = $at if $at >= 0 && ($closing < 0 || $at < $closing);
        }
        next if $closing < 0;
        my $id = substr($line, $start, $closing);
        $seen{$id} = 1 if $id =~ /\A[A-Z][A-Z0-9-]{2,31}-[0-9]{3}\z/;
    }
    return scalar keys %seen;
}
' "$tmp/specs" "$tmp/requests"
