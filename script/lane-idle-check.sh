#!/usr/bin/env bash
# usage: script/lane-idle-check.sh [--idle-minutes N] [--now EPOCH] [--gaps] PATH...
#
# Read-only idle-lane watchdog (ASD-V0-017, docs/LANE-ORCHESTRATION.md). Each PATH is one lane's
# activity evidence: a transcript or output file, or a directory (its newest file counts).
#
# Default mode prints one line per PATH and exits 1 when any lane is idle:
#   LANE <ACTIVE|IDLE> idle=<minutes>m last=<UTC time> <PATH>
# Last activity is the newest "timestamp":"<RFC 3339>" value (Z or numeric offset) in a .jsonl
# transcript; a file without any "timestamp" key counts by its modification time. A transcript that
# cannot be read, or holds a timestamp that does not parse, makes the lane
#   LANE UNKNOWN idle=? last=? reason=<unreadable|unparsed-timestamp>:<file> <PATH>
# (reason=no-activity when nothing is found); UNKNOWN is flagged like IDLE. --now fixes the
# comparison time (default: the current time).
#
# --gaps replays .jsonl transcripts: every interval longer than the threshold between consecutive
# timestamps prints, with the time the watchdog would have flagged it (gap start + threshold):
#   GAP idle=<minutes>m from=<UTC> to=<UTC> flagged=<UTC> <PATH>
# and a transcript it cannot read or parse prints UNKNOWN reason=<reason> <PATH>. It exits 1 when
# any gap or UNKNOWN is found. Nothing is written anywhere.
set -uo pipefail

idle=30
now=
gaps=0
while [[ $# -gt 0 ]]; do
  case $1 in
    --idle-minutes) idle=${2-}; shift 2 || exit 2 ;;
    --now) now=${2-}; shift 2 || exit 2 ;;
    --gaps) gaps=1; shift ;;
    -h | --help) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    --) shift; break ;;
    -*) printf 'lane-idle-check: REFUSE unknown-option %s\n' "$1" >&2; exit 2 ;;
    *) break ;;
  esac
done
if [[ $# -eq 0 ]]; then
  printf 'lane-idle-check: REFUSE no-paths (name each lane transcript, output file or directory)\n' >&2
  exit 2
fi
[[ $idle =~ ^[1-9][0-9]*$ ]] || { printf 'lane-idle-check: REFUSE invalid-idle-minutes %s\n' "$idle" >&2; exit 2; }
[[ -z $now || $now =~ ^[0-9]+$ ]] || { printf 'lane-idle-check: REFUSE invalid-now %s\n' "$now" >&2; exit 2; }

exec perl -e '
use strict;
use warnings;
use File::Find ();
use POSIX qw(strftime);
use Time::Local qw(timegm);
my ($idle, $now, $gaps, @paths) = @ARGV;
$now = time if $now eq "";
my $limit = $idle * 60;
my $flagged = 0;
sub utc { strftime("%Y-%m-%dT%H:%M:%SZ", gmtime($_[0])) }
# stamps FILE: (undef, reason) when the transcript cannot be read or holds a "timestamp" value that
# is not an RFC 3339 date-time; otherwise the epoch seconds of every timestamp, offsets applied.
sub stamps {
    my ($file) = @_;
    my @t;
    open(my $fh, "<", $file) or return (undef, "unreadable");
    while (my $line = <$fh>) {
        my $keys = () = $line =~ /"timestamp"\s*:/g;
        my $parsed = 0;
        while ($line =~ /"timestamp"\s*:\s*"(\d{4})-(\d\d)-(\d\d)[Tt](\d\d):(\d\d):(\d\d)(?:\.\d+)?([Zz]|([+-])(\d\d):(\d\d))"/g) {
            my $off = defined $8 ? ($8 eq "-" ? -1 : 1) * ($9 * 3600 + $10 * 60) : 0;
            my $e = eval { timegm($6, $5, $4, $3, $2 - 1, $1) };
            return (undef, "unparsed-timestamp") unless defined $e;
            push @t, $e - $off;
            $parsed++;
        }
        return (undef, "unparsed-timestamp") if $parsed != $keys;
    }
    close $fh or return (undef, "unreadable");
    return (1, @t);
}
sub files {
    my ($path) = @_;
    return ($path) unless -d $path;
    my @f;
    File::Find::find({ no_chdir => 1, wanted => sub { push @f, $_ if -f $_ } }, $path);
    return sort @f;
}
for my $path (@paths) {
    unless (-e $path) { print STDERR "lane-idle-check: REFUSE missing-path $path\n"; exit 2 }
    if ($gaps) {
        for my $file (grep { /\.jsonl\z/ } files($path)) {
            my ($ok, @t) = stamps($file);
            unless ($ok) { printf "UNKNOWN reason=%s %s\n", $t[0], $file; $flagged = 1; next }
            @t = sort { $a <=> $b } @t;
            for my $i (1 .. $#t) {
                my $d = $t[$i] - $t[$i - 1];
                next unless $d > $limit;
                printf "GAP idle=%dm from=%s to=%s flagged=%s %s\n", $d / 60, utc($t[$i - 1]), utc($t[$i]), utc($t[$i - 1] + $limit), $file;
                $flagged = 1;
            }
        }
        next;
    }
    my ($last, $why);
    for my $file (files($path)) {
        my $max;
        if ($file =~ /\.jsonl\z/) {
            my ($ok, @t) = stamps($file);
            unless ($ok) { $why = "$t[0]:$file"; last }
            ($max) = sort { $b <=> $a } @t;
        }
        # A file with no "timestamp" key (an output log) counts by its modification time.
        $max = (stat $file)[9] unless defined $max;
        $last = $max if defined $max && (!defined $last || $max > $last);
    }
    $why = "no-activity" if !defined $why && !defined $last;
    if (defined $why) { printf "LANE UNKNOWN idle=? last=? reason=%s %s\n", $why, $path; $flagged = 1; next }
    my $age = $now - $last;
    my $state = $age > $limit ? "IDLE" : "ACTIVE";
    $flagged = 1 if $state eq "IDLE";
    printf "LANE %s idle=%dm last=%s %s\n", $state, ($age > 0 ? $age : 0) / 60, utc($last), $path;
}
exit $flagged;
' "$idle" "$now" "$gaps" "$@"
