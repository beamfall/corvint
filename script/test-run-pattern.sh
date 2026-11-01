#!/usr/bin/env bash
# usage: test-run-pattern.sh BASE
# ASD-V0-1025..1027 (V1-3892): the Go tests a change can break through the files and requirement
# IDs they read, as `-run` patterns a lane adds to its focused set (docs/LANE-ORCHESTRATION.md
# section 13, step 3). It complements `corvint test --dry-run`, which selects by Go reachability.
# Keys, from BASE to the working tree (committed, staged, unstaged and untracked):
#   - each changed path's base name, matched as the last segment of a Go string literal
#     ("Makefile", "../../Makefile", "docs/specs/x.md", filepath.Join(..., "x.md")); a base name
#     two tracked files share (main.go) also needs its parent ("corvint/main.go", "corvint", "main.go");
#   - each requirement ID on a changed line (PREFIX-Vn-NNN), matched as a word, and its compact
#     form (the hyphens dropped), matched inside a name such as a Test function's.
# A Test, Example or Fuzz function of a tracked *_test.go file is selected when its text holds a
# key, or names (as a word) a function, method, var or const of the same package's test files
# whose text holds a key or names such a symbol (to a fixed point). A key in TestMain, init or any
# other top-level declaration runs the package whole. Prints, sorted by package:
#   run: script/heavy.sh go test -count=1 -run '^(TestA|TestB)$' ./PKG
#     why TestA FILE:LINE KEY
#   whole: ./PKG reason=KEY-in-TestMain|init|top-level FILE:LINE KEY
#   test-run-pattern: SUMMARY keys=paths:P,ids:I packages=N tests=T whole=W
# Over-selection is expected (a common base name or symbol name). Not covered: a key read by a
# helper in another package or a non-test file, a path built from pieces, and non-Go tests.
# Reads Git and the checkout only; writes nothing. Exit 0, or 2 with `REFUSE CODE`.
set -uo pipefail
export LC_ALL=C

refuse() {
  printf 'test-run-pattern: REFUSE %s\n' "$*" >&2
  exit 2
}
if [[ ${1-} == -h || ${1-} == --help ]]; then
  sed -n '2,21p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
fi
[[ $# == 1 ]] || refuse "usage: test-run-pattern.sh BASE"
top=$(git rev-parse --show-toplevel 2> /dev/null) || refuse "not-a-git-checkout $PWD; fix: run from the lane checkout"
cd "$top" || exit 2
base=$(git rev-parse --verify -q "$1^{commit}") || refuse "unknown-base $1; fix: git fetch origin, then pass the lane BASE sha"
command -v perl > /dev/null || refuse "perl-missing; fix: install perl, which this script needs"

work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-test-run-pattern.XXXXXX") || exit 2
trap '/bin/rm -rf "$work"' EXIT
{ git diff --no-renames --name-only "$base" -- && git ls-files --others --exclude-standard; } |
  sort -u > "$work/paths" || refuse "diff-failed base=$base; fix: rerun after git fsck"
git diff --no-renames -U0 "$base" -- | perl -ne '
  next if /^(?:\+\+\+|---) / || !/^[-+]/;
  print "$1\n" while /\b([A-Z][A-Z0-9]*-V[0-9]+-[0-9]+)\b/g;
' | sort -u > "$work/ids"
git ls-files --others --exclude-standard -z | xargs -0 -r perl -ne 'print "$1\n" while /\b([A-Z][A-Z0-9]*-V[0-9]+-[0-9]+)\b/g' 2> /dev/null |
  sort -u >> "$work/ids"
sort -u -o "$work/ids" "$work/ids"
git ls-files -z -- '*_test.go' > "$work/tests"
git ls-files | sed 's,.*/,,' | sort | uniq -d > "$work/shared-bases"

perl -e '
use strict; use warnings;
my ($paths, $ids, $tests, $shared) = @ARGV;
my (%bases, %shared, @idkeys);
open my $sh, "<", $shared or die; while (<$sh>) { chomp; $shared{$_} = 1 } close $sh;
# A base name that names one tracked file is its key; a shared one (main.go, README.md) needs its
# parent directory too, as "parent/base" or as Join pieces "parent", "base".
my @joins;
open my $p, "<", $paths or die;
while (<$p>) {
  chomp; next unless length;
  my @seg = split m{/};
  if (!$shared{$seg[-1]} || @seg == 1) { $bases{$seg[-1]} = 1; next }
  $bases{"$seg[-2]/$seg[-1]"} = 1;
  push @joins, "\"" . quotemeta($seg[-2]) . "\",\\s*\"" . quotemeta($seg[-1]) . "\"";
}
close $p;
my @compact;
open my $i, "<", $ids or die; while (<$i>) { chomp; next unless length; push @idkeys, $_; (my $c = $_) =~ s/-//g; push @compact, $c } close $i;
my @pathre = map { quotemeta } sort keys %bases;
my @alts;
push @alts, "(?<=[\"/`])(?:" . join("|", @pathre) . ")(?=[\"`])" if @pathre;
push @alts, @joins;
# An ID as a word; its compact form also inside a test name (TestABCV01020Name).
push @alts, "(?<![A-Za-z0-9-])(?:" . join("|", map { quotemeta } @idkeys) . ")(?![0-9])" if @idkeys;
push @alts, "(?<![A-Z0-9])(?:" . join("|", map { quotemeta } @compact) . ")(?![0-9])" if @compact;
my $npaths = keys %bases; my $nids = @idkeys;
unless (@alts) { printf "test-run-pattern: SUMMARY keys=paths:%d,ids:%d packages=0 tests=0 whole=0\n", $npaths, $nids; exit 0 }
my $key = qr/(@{[join "|", @alts]})/;
local $/ = "\0";
open my $t, "<", $tests or die; my @files = map { chomp; $_ } <$t>; close $t;
$/ = "\n";
my %pkg;    # dir => { decls => [ {name, test, kind, file, start, lines} ] }
for my $f (@files) {
  open my $fh, "<", $f or next; my @l = <$fh>; close $fh;
  next unless grep { /$key/ } @l;
  (my $dir = $f) =~ s{/[^/]*$}{}; $dir = "." if $dir eq $f;
  my ($cur, $doc, $docstart);
  for my $n (0 .. $#l) {
    my $line = $l[$n];
    if (!$cur) {
      # A top-level comment belongs to the declaration that follows it.
      if ($line =~ m{^\s*//}) { $docstart //= $n + 1; $doc .= $line; next }
      if ($line =~ /^\s*$/) { undef $doc; undef $docstart; next }
      if ($line =~ /^func\s+(?:\([^)]*\)\s*)?(\w+)/) {
        my $name = $1;
        my $kind = $name eq "TestMain" || $name eq "init" ? "main" : $name =~ /^(?:Test|Example|Fuzz)(?:[^a-z]|$)/ && $line !~ /^func\s+\(/ ? "test" : "func";
        $cur = { name => [$name], kind => $kind, file => $f, start => $n + 1, text => "" };
      } elsif ($line =~ /^(?:var|const)\s+\($/) {
        $cur = { name => [], kind => "block", file => $f, start => $n + 1, text => "", lines => [] };
      } elsif ($line =~ /^(?:var|const)\s+(\w+(?:\s*,\s*\w+)*)/) {
        my @names = split /\s*,\s*/, $1;
        $cur = { name => [@names], kind => "var", file => $f, start => $n + 1, text => "" };
      } elsif ($line =~ $key) {
        push @{ $pkg{$dir}{whole} }, sprintf "top-level %s:%d %s", $f, $n + 1, $1;
        next;
      } else { undef $doc; undef $docstart; next }
      if (defined $doc) { $cur->{text} = $doc; $cur->{start} = $docstart; $cur->{head} = $n + 1 }
      undef $doc; undef $docstart;
    }
    $cur->{text} .= $line;
    if ($cur->{kind} eq "block") {
      if ($line =~ /^\t(\w+(?:\s*,\s*\w+)*)\b/ && $line !~ /^\t\/\//) { $cur->{lastnames} = [split /\s*,\s*/, $1] }
      if ($line =~ $key && $cur->{lastnames}) {
        push @{ $pkg{$dir}{decls} }, { name => $cur->{lastnames}, kind => "var", file => $f, start => $n + 1, text => $line, direct => $1 };
      }
      if ($line =~ /^\)/) { undef $cur }
      next;
    }
    my $done = $line =~ /^[})]/ || ($n + 1 == ($cur->{head} // $cur->{start}) && ($line =~ /\}\s*$/ || ($cur->{kind} eq "var" && $line !~ /[{(\[]\s*$/)));
    if ($done) {
      $cur->{direct} = $1 if $cur->{text} =~ $key;
      push @{ $pkg{$dir}{decls} }, $cur; undef $cur;
    }
  }
}
my ($tests_total, $whole_total, @out) = (0, 0);
for my $dir (sort keys %pkg) {
  my $d = $pkg{$dir};
  my @decls = @{ $d->{decls} || [] };
  my @whole = @{ $d->{whole} || [] };
  my %marked;    # symbol => "FILE:LINE KEY"
  for my $x (@decls) {
    next unless defined $x->{direct};
    my $line = $x->{start}; my @tl = split /\n/, $x->{text};
    for my $k (0 .. $#tl) { if ($tl[$k] =~ $key) { $line = $x->{start} + $k; last } }
    $x->{why} = "$x->{file}:$line $x->{direct}";
    if ($x->{kind} eq "main") { push @whole, "$x->{name}[0]-reads-key $x->{why}"; next }
    next if $x->{kind} eq "test";
    $marked{$_} //= $x->{why} for @{ $x->{name} };
  }
  for (my $changed = 1; $changed && %marked;) {
    $changed = 0;
    my $re = "\\b(" . join("|", map { quotemeta } sort keys %marked) . ")\\b";
    for my $x (@decls) {
      next if $x->{kind} eq "test" || $x->{why};
      next unless $x->{text} =~ /$re/;
      my $via = $1;
      $x->{why} = $marked{$via};
      if ($x->{kind} eq "main") { push @whole, "$x->{name}[0]-reads-key $x->{why}"; next }
      for (@{ $x->{name} }) { next if $marked{$_}; $marked{$_} = $x->{why}; $changed = 1 }
    }
  }
  if (@whole) { push @out, "whole: ./$dir reason=$whole[0]"; $whole_total++; next }
  my $re = %marked ? "\\b(" . join("|", map { quotemeta } sort keys %marked) . ")\\b" : undef;
  my @sel;
  for my $x (@decls) {
    next unless $x->{kind} eq "test";
    my $why = $x->{why};
    if (!$why && defined $re && $x->{text} =~ /$re/) { $why = "$marked{$1} via $1" }
    next unless $why;
    push @sel, [$x->{name}[0], $why];
  }
  next unless @sel;
  my %seen; @sel = grep { !$seen{$_->[0]}++ } sort { $a->[0] cmp $b->[0] } @sel;
  $tests_total += @sel;
  push @out, sprintf "run: script/heavy.sh go test -count=1 -run %s ./%s", "\x27^(" . join("|", map { $_->[0] } @sel) . ")\$\x27", $dir;
  push @out, map { "  why $_->[0] $_->[1]" } @sel;
}
print "$_\n" for @out;
printf "test-run-pattern: SUMMARY keys=paths:%d,ids:%d packages=%d tests=%d whole=%d\n", $npaths, $nids,
  scalar(grep { /^(?:run|whole):/ } @out), $tests_total, $whole_total;
' "$work/paths" "$work/ids" "$work/tests" "$work/shared-bases"
