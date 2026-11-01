#!/bin/sh
set -eu

# Repin use-case receipts whose subjects changed (V1-0268, docs/specs/use-case-conformance-v0.md).
#
# A receipt under conformance/use-cases-v0/receipts/ pins each subject file by the SHA-256 of its
# exact bytes, and the ledger pins each receipt the same way (UCV0-004, UCV0-005). A change that
# edits a pinned subject therefore fails `go run ./conformance/use-cases-v0` with
# `subject-digest-mismatch` until the receipt is repinned. This script does the repin that used to
# be done by hand: for every receipt with a changed subject it writes the subject's new digest and
# `repositoryRevision` = `git rev-parse HEAD` into the receipt, then the receipt's new digest into
# the ledger. It edits only those digest strings, so every other byte stays as committed.
#
#   script/repin-use-case-receipts.sh           repin every stale receipt and report each one
#   script/repin-use-case-receipts.sh --check   list stale receipts; exit 1 if any, write nothing
#   script/repin-use-case-receipts.sh --merged  also adopt a receipt git holds after a merge
#   ... [--changed PATH...]                     take the changed paths from the arguments
#
# It refuses, writing nothing, when a receipt or subject is missing, when a receipt's bytes do not
# match its ledger pin (a hand edit that needs review, not a repin), or when an old digest is not
# a unique string in the file it would be replaced in (for example a sealed-benchmark seal that
# also names the subject digest, UCV0-007). It reads the worktree, as the conformance runner does.
# A receipt that left its ledger pin is refused with the exact `git restore --source=REV` and
# repin command when a recent commit holds the pinned bytes (UCV0-019).
#
# --merged (ASD-V0-917, V1-3936) is the integrator's step after merging lanes, and the batch-regen
# full run's use-case-receipts step: a receipt whose bytes left its ledger pin is adopted, not
# refused, when its worktree bytes equal its index blob, that is when git holds them (a merge
# result, committed or staged) rather than an unstaged edit. Its ledger pin is then rewritten to
# its bytes, after its subjects are repinned, and it is listed as "adopted". Two lanes that each
# repinned one receipt merge, through script/merge-use-case-pins.sh, to ours' pins, which this
# mode recomputes from the merged subjects. An unstaged receipt edit is still refused.
#
# It also lists, for information only, each changed path that no receipt pins (UCV0-018): the
# paths `git diff --name-only HEAD` names, or the `--changed` paths. "All receipts current" says
# nothing about such a path, for example help text no receipt covers. The listing never changes
# the exit status.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"

usage() { printf 'usage: %s [--check | --merged] [--changed PATH...]\n' "$0" >&2; exit 64; }
mode=write
source=diff
while [ $# -gt 0 ]; do
    case $1 in
    --check) [ "$mode" = write ] || usage; mode=check; shift ;;
    --merged) [ "$mode" = write ] || usage; mode=merged; shift ;;
    --changed) source=args; shift; break ;;
    *) usage ;;
    esac
done

revision=$(git rev-parse HEAD)

perl - "$mode" "$revision" "$source" "$@" <<'PERL'
use strict;
use warnings;
use Digest::SHA qw(sha256_hex);
use JSON::PP ();

my ($mode, $revision, $source, @given) = @ARGV;
my $ledger_path = 'conformance/use-cases-v0/ledger.json';

sub slurp {
    my ($path) = @_;
    return undef if -l $path || !-f $path;
    open my $fh, '<:raw', $path or return undef;
    local $/;
    my $raw = <$fh>;
    close $fh;
    return $raw;
}

# replace_once swaps the one occurrence of a quoted 64-hex digest, or returns undef when the
# digest is absent or ambiguous, so a replacement can never touch a second, unrelated field.
sub replace_once {
    my ($text, $old, $new) = @_;
    my $count = () = $text =~ /"\Q$old\E"/g;
    return undef if $count != 1;
    $text =~ s/"\Q$old\E"/"$new"/;
    return $text;
}

# One shell word for a path in a printed command: single-quoted, each embedded quote as '\''.
sub shell_quote {
    my ($text) = @_;
    $text =~ s/'/'\\''/g;
    return "'$text'";
}

# pinned_recovery names the recovery for a receipt whose bytes left its ledger pin (UCV0-019): a
# cherry-pick or merge that carries a repinned receipt without its ledger leaves the pinned bytes
# in history, so the newest of the last 50 commits touching the path whose blob has the pinned
# digest is named with the exact restore-and-repin command. Nothing is restored automatically;
# with no such commit the edit needs review.
sub pinned_recovery {
    my ($path, $pin) = @_;
    my @revisions;
    if (open my $log, '-|', 'git', 'log', '--format=%H', '-n', '50', 'HEAD', '--', $path) {
        chomp(@revisions = <$log>);
        close $log;
    }
    for my $revision (@revisions) {
        open my $blob, '-|', 'git', 'cat-file', 'blob', "$revision:$path" or next;
        binmode $blob;
        local $/;
        my $bytes = <$blob>;
        close $blob or next;
        next unless defined $bytes && sha256_hex($bytes) eq $pin;
        return "fix: the pinned bytes are at $revision; restore them and repin: "
            . "git restore --source=$revision -- " . shell_quote($path) . " && script/repin-use-case-receipts.sh";
    }
    return "fix: no commit among the last 50 touching $path has the pinned bytes; review the edit, "
        . "then restore the receipt or record the change by hand";
}

# index_blob returns the bytes git holds for a path in the index, or undef when it has none.
sub index_blob {
    my ($path) = @_;
    open my $blob, '-|', 'git', 'cat-file', 'blob', ":$path" or return undef;
    binmode $blob;
    local $/;
    my $bytes = <$blob>;
    close $blob or return undef;
    return $bytes;
}

my $ledger_raw = slurp($ledger_path) // die "repin: $ledger_path is missing\n";
my $ledger = JSON::PP->new->decode($ledger_raw);

my @refusals;
my @stale;    # [receipt path, pinned receipt digest, receipt raw, [[subject, old, new], ...]]
my %adopted;  # --merged: receipts whose git-held bytes left their ledger pin
my $receipt_count = 0;
my %pinned = ($ledger_path => 1);    # the ledger, every receipt, subject and clause-extract spec
for my $row (@{ $ledger->{useCases} }) {
    for my $class (@{ $ledger->{evidenceClasses} }) {
        my $ref = $row->{evidence}{$class} or next;
        $receipt_count++;
        my $path = $ref->{path};
        $pinned{$path} = 1;
        my $raw = slurp($path);
        if (!defined $raw) {
            push @refusals, "$path: receipt is missing";
            next;
        }
        if (sha256_hex($raw) ne $ref->{sha256} && $mode eq 'merged'
            && defined(my $held = index_blob($path))) {
            $adopted{$path} = 1 if $held eq $raw;
        }
        if (sha256_hex($raw) ne $ref->{sha256} && !$adopted{$path}) {
            push @refusals, "$path: receipt bytes do not match the ledger pin (hand edit?)\n    "
                . pinned_recovery($path, $ref->{sha256});
            next;
        }
        my @changed;
        for my $subject (@{ JSON::PP->new->decode($raw)->{subjects} }) {
            $pinned{ $subject->{path} } = 1;
            my $bytes = slurp($subject->{path});
            if (!defined $bytes) {
                push @refusals, "$path: subject $subject->{path} is missing";
                next;
            }
            # A clause extract (UCV0-016) pins clauses of its spec, so the spec counts as pinned.
            my $extract = $subject->{path} =~ /\.json\z/ ? eval { JSON::PP->new->decode($bytes) } : undef;
            if (ref $extract eq 'HASH' && ($extract->{profile} // '') eq 'corvint-use-case-clauses/0'
                && defined $extract->{spec} && !ref $extract->{spec}) {
                $pinned{ $extract->{spec} } = 1;
            }
            my $digest = sha256_hex($bytes);
            push @changed, [$subject->{path}, $subject->{sha256}, $digest] if $digest ne $subject->{sha256};
        }
        push @stale, [$path, $ref->{sha256}, $raw, \@changed] if @changed || $adopted{$path};
    }
}

my %updated;
my $new_ledger = $ledger_raw;
for my $entry (@stale) {
    my ($path, $pinned, $raw, $changed) = @$entry;
    my $text = $raw =~ s/("repositoryRevision":\s*")[0-9a-f]+"/$1$revision"/r;
    my @ambiguous = grep { !defined replace_once($text, $_->[1], $_->[2]) } @$changed;
    push @refusals, map { "$path: digest of $_->[0] is not a unique string in the receipt" } @ambiguous;
    next if @ambiguous;
    $text = replace_once($text, $_->[1], $_->[2]) for @$changed;
    $new_ledger = replace_once($new_ledger, $pinned, sha256_hex($text));
    die "repin: $ledger_path does not pin $path by a unique digest\n" unless defined $new_ledger;
    $updated{$path} = $text;
}

if (@refusals) {
    print STDERR "repin: refused, nothing written:\n";
    print STDERR "  $_\n" for @refusals;
    exit 1;
}

# Informational (UCV0-018): changed paths no receipt pins. A failed diff is stated, never read as
# "nothing changed", and nothing here changes the exit status.
my @changed_paths;
my $diff_failed;
if ($source eq 'args') {
    @changed_paths = map { s{\A\./}{}r } @given;
} elsif (open my $diff, '-|', 'git', 'diff', '--name-only', '--no-renames', '-z', 'HEAD', '--') {
    local $/;
    my $names = <$diff> // '';
    $diff_failed = !close $diff;
    @changed_paths = grep { length } split /\0/, $names;
} else {
    $diff_failed = 1;
}
my %seen;
my @unpinned = grep { !$pinned{$_} } grep { !$seen{$_}++ } sort @changed_paths;
if ($diff_failed) {
    print "use-case receipts: changed paths not listed: git diff against HEAD failed (informational)\n";
} else {
    my $from = $source eq 'args' ? '--changed' : 'the worktree against HEAD';
    print "use-case receipts: " . scalar(keys %seen) . " changed path(s) in $from, "
        . scalar(@unpinned) . " not pinned by any receipt (informational)\n";
    print "  unpinned $_\n" for @unpinned;
}

if (!@stale) {
    print "use-case receipts: all $receipt_count receipts current\n";
    exit 0;
}

my $verb = $mode eq 'check' ? 'stale' : 'repinned';
for my $entry (@stale) {
    my ($path, undef, undef, $changed) = @$entry;
    print "$verb $path\n";
    print "  adopted the merged receipt bytes (--merged)\n" if $adopted{$path};
    print "  subject $_->[0]\n" for @$changed;
}

if ($mode eq 'check') {
    print STDERR "use-case receipts: " . scalar(@stale) . " receipt(s) pin changed subjects; "
        . "run script/repin-use-case-receipts.sh and commit the result\n";
    exit 1;
}

sub spew {
    my ($path, $text) = @_;
    my $tmp = "$path.repin.$$";
    open my $fh, '>:raw', $tmp or die "repin: $tmp: $!\n";
    print {$fh} $text or die "repin: $tmp: $!\n";
    close $fh or die "repin: $tmp: $!\n";
    rename $tmp, $path or die "repin: $path: $!\n";
}
spew($_, $updated{$_}) for sort keys %updated;
spew($ledger_path, $new_ledger);
print "use-case receipts: " . scalar(@stale) . " receipt(s) repinned at $revision; "
    . "verify with go run ./conformance/use-cases-v0\n";
PERL
