#!/bin/sh
set -eu

# DCG-V0-682 (V1-3746): resolve the merge conflicts that differ only in pinned citation line numbers.
#
# Two lanes that each insert lines into one cited file repin the same citation to different
# numbers, so every batch merge stops on Markdown conflict blocks whose two sides carry the same
# text and the same content anchors and differ only in `path:N@hex` spans. The numbers on either
# side are stale against the merged code anyway, so the side kept does not matter: --repin moves
# each pin to the one position where its anchored text now hashes unchanged.
#
# For every unmerged `*.md` path, this script reads the worktree copy's conflict blocks (Git's
# default 7-character markers, with or without a diff3 base section). When every block's two sides
# are equal once each pinned span is masked, it writes the current side (ours) of each block and
# stages the file. Any block that differs in other text, a different anchor included, refuses that
# file, naming the block's line, and leaves it untouched. When every file resolved, it runs
# `script/check-line-citations.sh --repin` and exits with its status; otherwise it exits 1 without
# repinning, since the merge is not finished. Non-Markdown paths are left to the caller.
#
#   script/line-citations-resolve.sh

root=$(git rev-parse --show-toplevel)
cd "$root"

status=0
resolved=0
paths=$(git diff --name-only --diff-filter=U -- '*.md')
IFS='
'
for path in $paths; do
  stages=$(git ls-files -u -- "$path" | awk '{ print $3 }' | sort -u | tr -d '\n')
  case $stages in
    123 | 23) ;;
    *)
      printf 'line citations: %s: index stages %s are not a two-sided content conflict; resolve it by hand\n' "$path" "$stages" >&2
      status=1
      continue
      ;;
  esac
  if perl -e '
    use strict;
    use warnings;
    my ($path) = @ARGV;
    open my $in, "<", $path or die "line citations: $path: $!\n";
    my @lines = <$in>;
    close $in;
    # A pinned span is the line part of `path:N@hex`, `path:N-M@hex`, `path:N,M@hex` or `:N@hex`.
    sub masked { my $text = join "", @_; $text =~ s/:\d[\d,-]*(?=\@[0-9a-f]{4,64}`)/:N/g; return $text }
    sub refuse { print STDERR "line citations: $path:$_[0]: $_[1]; resolve it by hand\n"; exit 1 }
    my ($state, $start, @ours, @theirs, @out);
    my $blocks = 0;
    for my $i (0 .. $#lines) {
        my $line = $lines[$i];
        my $number = $i + 1;
        if (!defined $state) {
            if ($line =~ /^<{7}(?: |\r?$)/) { ($state, $start, @ours, @theirs) = ("ours", $number); next }
            refuse($number, "conflict marker outside a block") if $line =~ /^(?:={7}|>{7}|\|{7})(?: |\r?$)/;
            push @out, $line;
            next;
        }
        if ($line =~ /^\|{7}(?: |\r?$)/) {
            refuse($number, "base marker out of order in the block at line $start") unless $state eq "ours";
            $state = "base";
            next;
        }
        if ($line =~ /^={7}\r?$/) {
            refuse($number, "separator out of order in the block at line $start") if $state eq "theirs";
            $state = "theirs";
            next;
        }
        if ($line =~ /^>{7}(?: |\r?$)/) {
            refuse($number, "end marker before the separator in the block at line $start") unless $state eq "theirs";
            refuse($start, "the two sides differ in more than pinned citation line numbers")
                unless masked(@ours) eq masked(@theirs);
            push @out, @ours;
            ++$blocks;
            undef $state;
            next;
        }
        refuse($number, "nested conflict marker in the block at line $start") if $line =~ /^<{7}(?: |\r?$)/;
        push @ours, $line if $state eq "ours";
        push @theirs, $line if $state eq "theirs";
    }
    refuse($start, "conflict block is not closed") if defined $state;
    refuse(1, "no conflict block with 7-character markers") unless $blocks;
    open my $out, ">", $path or die "line citations: $path: $!\n";
    print {$out} @out;
    close $out or die "line citations: $path: $!\n";
    print STDERR "line citations: $path: resolved $blocks block(s) that differed only in pinned line numbers\n";
  ' "$path"; then
    git add -- "$path"
    resolved=$((resolved + 1))
  else
    status=1
  fi
done

if [ "$status" != 0 ]; then
  printf 'line citations: some conflicts need a hand resolution; rerun this script after resolving them\n' >&2
  exit 1
fi
if [ "$resolved" = 0 ]; then
  printf 'line citations: no unmerged Markdown file to resolve\n' >&2
  exit 0
fi
exec script/check-line-citations.sh --repin
