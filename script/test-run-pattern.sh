#!/usr/bin/env bash
# usage: test-run-pattern.sh BASE
# ASD-V0-1025..1027 (V1-3892), ASD-V0-3832 (V1-5007), ASD-V0-5133..5135: the Go tests a change can
# break through the files and requirement IDs they read, the test code it edits or removes and the
# package inputs it edits, as `-run` patterns a lane adds to its focused set
# (docs/LANE-ORCHESTRATION.md section 13, step 3). It complements `corvint test --dry-run`, which
# selects by Go reachability.
# Keys, from BASE to the working tree (committed, staged, unstaged and untracked):
#   - each changed path's base name, matched as the last segment of a Go string literal
#     ("Makefile", "../../Makefile", "docs/specs/x.md", filepath.Join(..., "x.md")); a base name
#     two tracked files share (main.go) also needs its parent ("corvint/main.go", "corvint", "main.go");
#   - each requirement ID on a changed line (PREFIX-Vn-NNN), matched as a word, and its compact
#     form (the hyphens dropped), matched inside a name such as a Test function's.
# A Test, Example or Fuzz function of a *_test.go file is selected when its text holds a key, when
# a changed or removed line of the file lies in it, or when it names (as a word) a function, method,
# var or const of the same package's test files whose text holds a key, is changed, or names such a
# symbol (to a fixed point). The package runs whole (fail closed) for a key in or a change to
# TestMain, init or any other top-level declaration; for a removed line outside a Test, Benchmark,
# Example or Fuzz function whose declaration is gone (removed-NAME; the line is BASE's); for a
# deleted *_test.go file (deleted-test-file); for a changed non-test .go, .s, .c, .h, .cc, .cpp or
# .m file of a package with tests (source-edit; heavy.sh admits the whole package for a .go file
# without HEAVY_WHOLE_REASON, ASD-V0-2478) or any other changed non-testdata file in such a
# package's directory (package-file); and, for a changed go.mod or go.sum, every package of that
# module the script would otherwise narrow (module-edit). Prints, sorted by package:
#   run: script/heavy.sh go test -count=1 -run '^(TestA|TestB)$' ./PKG
#     why TestA FILE:LINE KEY            (or FILE:LINE changed, or ... via HELPER)
#   whole: ./PKG reason=source-edit FILE | package-file FILE | module-edit FILE | deleted-test-file FILE
#          | KEY-in-TestMain|init|top-level FILE:LINE KEY | changed-... FILE:LINE | removed-... FILE:LINE
#   test-run-pattern: SUMMARY keys=paths:P,ids:I packages=N tests=T whole=W sources=S
# Over-selection is expected (a common base name or symbol name, a changed var block, a new test
# file with a type declaration). Not covered: a key read by a helper in another package or a
# non-test file, a path built from pieces, a .go file under testdata, an embedded file in a
# subdirectory without tests, and non-Go tests. Git paths are read unquoted (core.quotepath=off)
# with fixed a/ b/ prefixes; a path holding a tab, quote, backslash or newline is still quoted
# by Git and not matched.
# Reads Git and the checkout only; writes nothing. Exit 0, or 2 with `REFUSE CODE`.
set -uo pipefail
export LC_ALL=C

refuse() {
  printf 'test-run-pattern: REFUSE %s\n' "$*" >&2
  exit 2
}
if [[ ${1-} == -h || ${1-} == --help ]]; then
  sed -n '2,36p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
fi
[[ $# == 1 ]] || refuse "usage: test-run-pattern.sh BASE"
top=$(git rev-parse --show-toplevel 2> /dev/null) || refuse "not-a-git-checkout $PWD; fix: run from the lane checkout"
cd "$top" || exit 2
base=$(git rev-parse --verify -q "$1^{commit}") || refuse "unknown-base $1; fix: git fetch origin, then pass the lane BASE sha"
command -v perl > /dev/null || refuse "perl-missing; fix: install perl, which this script needs"

# ASD-V0-5134: every Git listing prints paths unquoted, and the one diff whose headers are parsed
# carries fixed prefixes, so diff.noprefix, diff.mnemonicPrefix and core.quotepath do not change
# what the script reads.
gitq() { git -c core.quotepath=off "$@"; }

work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-test-run-pattern.XXXXXX") || exit 2
trap '/bin/rm -rf "$work"' EXIT
{ gitq diff --no-renames --name-only "$base" -- && gitq ls-files --others --exclude-standard; } |
  sort -u > "$work/paths" || refuse "diff-failed base=$base; fix: rerun after git fsck"
gitq diff --no-renames -U0 "$base" -- | perl -ne '
  next if /^(?:\+\+\+|---) / || !/^[-+]/;
  print "$1\n" while /\b([A-Z][A-Z0-9]*-V[0-9]+-[0-9]+)\b/g;
' | sort -u > "$work/ids"
gitq ls-files --others --exclude-standard -z | xargs -0 -r perl -ne 'print "$1\n" while /\b([A-Z][A-Z0-9]*-V[0-9]+-[0-9]+)\b/g' 2> /dev/null |
  sort -u >> "$work/ids"
sort -u -o "$work/ids" "$work/ids"
# Test files: tracked and untracked, so a new test file takes part (ASD-V0-3832).
{ gitq ls-files -- '*_test.go' && gitq ls-files --others --exclude-standard -- '*_test.go'; } | sort -u | tr '\n' '\0' > "$work/tests"
gitq ls-files | sed 's,.*/,,' | sort | uniq -d > "$work/shared-bases"
# ASD-V0-3832: changed non-test Go source (a .go file under testdata is data, not package source).
grep -E '\.go$' "$work/paths" | grep -v -E '_test\.go$' | grep -v -E '(^|/)testdata/' > "$work/sources" || true
# ASD-V0-3832, ASD-V0-5133: each hunk of a changed test file as the tab-separated FILE NEWSTART
# NEWCOUNT OLDSTART OLDCOUNT; an untracked test file is changed throughout (FILE 1 all 0 0); a
# deleted test file is FILE deleted. Git ends a header path that holds a space with a tab.
gitq diff --no-renames --src-prefix=a/ --dst-prefix=b/ -U0 "$base" -- '*_test.go' | perl -ne '
  s/\t$//;
  if (m{^--- a/(.*)$}) { $old = $1; next }
  if (m{^--- /dev/null}) { undef $old; next }
  if (m{^\+\+\+ b/(.*)$}) { $f = $1; next }
  if (m{^\+\+\+ /dev/null}) { print "$old\tdeleted\n" if defined $old; undef $old; undef $f; next }
  next unless defined $f;
  print join("\t", $f, $3, (defined $4 ? $4 : 1), $1, (defined $2 ? $2 : 1)), "\n" if /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/;
' > "$work/hunks"
gitq ls-files --others --exclude-standard -- '*_test.go' | sed 's/$/\t1\tall\t0\t0/' >> "$work/hunks"

perl -e '
use strict; use warnings;
my ($paths, $ids, $tests, $shared, $sources, $hunks, $base) = @ARGV;
my (%bases, %shared, @idkeys, @allpaths);
open my $sh, "<", $shared or die; while (<$sh>) { chomp; $shared{$_} = 1 } close $sh;
# A base name that names one tracked file is its key; a shared one (main.go, README.md) needs its
# parent directory too, as "parent/base" or as Join pieces "parent", "base".
my @joins;
open my $p, "<", $paths or die;
while (<$p>) {
  chomp; next unless length;
  push @allpaths, $_;
  my @seg = split m{/};
  if (!$shared{$seg[-1]} || @seg == 1) { $bases{$seg[-1]} = 1; next }
  $bases{"$seg[-2]/$seg[-1]"} = 1;
  push @joins, "\"" . quotemeta($seg[-2]) . "\",\\s*\"" . quotemeta($seg[-1]) . "\"";
}
close $p;
my @compact;
open my $i, "<", $ids or die; while (<$i>) { chomp; next unless length; push @idkeys, $_; (my $c = $_) =~ s/-//g; push @compact, $c } close $i;
# ASD-V0-3832: changed package source, by package directory (the first path by sort names it).
my (%srcwhole, $nsources);
open my $s, "<", $sources or die;
while (<$s>) { chomp; next unless length; $nsources++; (my $dir = $_) =~ s{/[^/]*$}{}; $dir = "." if $dir eq $_; $srcwhole{$dir} //= "source-edit $_" }
close $s;
$nsources //= 0;
# ASD-V0-3832, ASD-V0-5133: the hunks of each test file, new and old side; deleted test files.
my (%hunks, %deleted);
open my $h, "<", $hunks or die;
while (<$h>) {
  chomp; my ($f, @r) = split /\t/;
  if (@r == 1 && $r[0] eq "deleted") { $deleted{$f} = 1; next }
  next unless @r == 4;
  push @{ $hunks{$f} }, [@r];
}
close $h;
my @pathre = map { quotemeta } sort keys %bases;
my @alts;
push @alts, "(?<=[\"/`])(?:" . join("|", @pathre) . ")(?=[\"`])" if @pathre;
push @alts, @joins;
# An ID as a word; its compact form also inside a test name (TestABCV01020Name).
push @alts, "(?<![A-Za-z0-9-])(?:" . join("|", map { quotemeta } @idkeys) . ")(?![0-9])" if @idkeys;
push @alts, "(?<![A-Z0-9])(?:" . join("|", map { quotemeta } @compact) . ")(?![0-9])" if @compact;
my $npaths = keys %bases; my $nids = @idkeys;
my $key = @alts ? qr/(@{[join "|", @alts]})/ : qr/(?!)/;
local $/ = "\0";
open my $t, "<", $tests or die; my @files = map { chomp; $_ } <$t>; close $t;
$/ = "\n";

# parse_decls(\@lines, FILE): the top-level declarations of one test file with their line ranges
# (a doc comment belongs to the declaration that follows it), the lines some declaration spans,
# the inert lines (blank, comment-only, package and import) and the top-level lines holding a key.
sub parse_decls {
  my ($l, $f) = @_;
  my ($cur, $doc, $docstart, $in_import);
  my (@decls, %covered, %inert, @toplevel);
  for my $n (0 .. $#$l) {
    my $line = $l->[$n];
    if (!$cur) {
      if ($line =~ m{^\s*//}) { $docstart //= $n + 1; $doc .= $line; $inert{$n + 1} = 1; next }
      if ($line =~ /^\s*$/) { undef $doc; undef $docstart; $inert{$n + 1} = 1; next }
      if ($in_import) { $inert{$n + 1} = 1; $in_import = 0 if $line =~ /^\)/; next }
      if ($line =~ /^package\s/ || $line =~ /^import\s+"/) { undef $doc; undef $docstart; $inert{$n + 1} = 1; next }
      if ($line =~ /^import\s*\(/) { undef $doc; undef $docstart; $inert{$n + 1} = 1; $in_import = 1; next }
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
        push @toplevel, sprintf "top-level %s:%d %s", $f, $n + 1, $1;
        next;
      } else { undef $doc; undef $docstart; next }
      if (defined $doc) { $cur->{text} = $doc; $cur->{start} = $docstart; $cur->{head} = $n + 1 }
      undef $doc; undef $docstart;
    }
    $cur->{text} .= $line;
    if ($cur->{kind} eq "block") {
      if ($line =~ /^\t(\w+(?:\s*,\s*\w+)*)\b/ && $line !~ /^\t\/\//) { $cur->{lastnames} = [split /\s*,\s*/, $1] }
      if ($line =~ $key && $cur->{lastnames}) {
        push @decls, { name => $cur->{lastnames}, kind => "var", file => $f, start => $n + 1, end => $n + 1, text => $line, direct => $1 };
      }
      if ($line =~ /^\)/) { $cur->{end} = $n + 1; push @decls, $cur; undef $cur }
      next;
    }
    my $done = $line =~ /^[})]/ || ($n + 1 == ($cur->{head} // $cur->{start}) && ($line =~ /\}\s*$/ || ($cur->{kind} eq "var" && $line !~ /[{(\[]\s*$/)));
    if ($done) {
      $cur->{direct} = $1 if $cur->{text} =~ $key;
      $cur->{end} = $n + 1;
      push @decls, $cur; undef $cur;
    }
  }
  if ($cur) { $cur->{end} = scalar @$l; push @decls, $cur }
  for my $x (@decls) { $covered{$_} = 1 for $x->{start} .. $x->{end} }
  return (\@decls, \%covered, \%inert, \@toplevel);
}

# old_side(FILE): the BASE version of a test file, parsed; empty when BASE has no such file.
sub old_side {
  my ($f) = @_;
  my @l;
  if (open my $g, "-|", "git", "-c", "core.quotepath=off", "show", "$base:$f") { @l = <$g>; close $g }
  my ($decls, $covered, $inert) = parse_decls(\@l, $f);
  return { lines => \@l, decls => $decls, covered => $covered, inert => $inert };
}

my %pkg;    # dir => { decls => [ {name, kind, file, start, end, text, direct, changed} ], whole => [...] }
my %testdirs;
for my $f (@files) {
  (my $dir = $f) =~ s{/[^/]*$}{}; $dir = "." if $dir eq $f;
  $testdirs{$dir} = 1;
  next unless $hunks{$f} || @alts;
  open my $fh, "<", $f or next; my @l = <$fh>; close $fh;
  next unless $hunks{$f} || grep { /$key/ } @l;
  my ($decls, $covered, $inert, $toplevel) = parse_decls(\@l, $f);
  my @decls = @$decls;
  push @{ $pkg{$dir}{whole} }, @$toplevel;
  my %byname;
  for my $x (@decls) { next if $x->{kind} eq "block"; $byname{$_} //= $x for @{ $x->{name} } }
  # ASD-V0-3832: attribute each changed new-side line to the declaration spanning it. A changed
  # line no declaration spans, unless inert, and a changed var/const block run the package whole.
  # ASD-V0-5133: a removed line is read from BASE and attributed the same way; a removed
  # declaration other than a Test, Benchmark, Example or Fuzz function runs the package whole.
  my $old;
  for my $hunk (@{ $hunks{$f} || [] }) {
    my ($start, $count, $ostart, $ocount) = @$hunk;
    my @lines = $count eq "all" ? (1 .. scalar @l) : $count == 0 ? () : ($start .. $start + $count - 1);
    for my $ln (@lines) {
      next if $ln < 1 || $ln > @l;
      if (!$covered->{$ln}) {
        next if $inert->{$ln};
        push @{ $pkg{$dir}{whole} }, sprintf "changed-top-level %s:%d", $f, $ln;
        next;
      }
      for my $x (@decls) {
        next if $ln < $x->{start} || $ln > $x->{end} || defined $x->{changed};
        if ($x->{kind} eq "block" && $count ne "all") { push @{ $pkg{$dir}{whole} }, sprintf "changed-block %s:%d", $f, $ln; next }
        $x->{changed} = "$f:$ln changed" unless $x->{kind} eq "block";
      }
    }
    next if $count eq "all" || $ocount == 0;
    $old //= old_side($f);
    for my $ln ($ostart .. $ostart + $ocount - 1) {
      next if $ln < 1 || $ln > @{ $old->{lines} };
      if (!$old->{covered}{$ln}) {
        next if $old->{inert}{$ln};
        push @{ $pkg{$dir}{whole} }, sprintf "removed-top-level %s:%d", $f, $ln;
        next;
      }
      for my $x (@{ $old->{decls} }) {
        next if $ln < $x->{start} || $ln > $x->{end};
        if ($x->{kind} eq "block") { push @{ $pkg{$dir}{whole} }, sprintf "removed-block %s:%d", $f, $ln; next }
        my $name = $x->{name}[0];
        my $now = $byname{$name};
        if ($now && $now->{kind} eq $x->{kind}) { $now->{changed} //= sprintf "%s:%d changed", $f, $start > 0 ? $start : 1; next }
        next if $x->{kind} eq "test" || $name =~ /^Benchmark(?:[^a-z]|$)/;
        push @{ $pkg{$dir}{whole} }, sprintf "removed-%s %s:%d", $name, $f, $ln;
      }
    }
  }
  push @{ $pkg{$dir}{decls} }, grep { $_->{kind} ne "block" } @decls;
}
# ASD-V0-5133: a deleted test file runs its package whole while the package still has Go files.
for my $f (sort keys %deleted) {
  (my $dir = $f) =~ s{/[^/]*$}{}; $dir = "." if $dir eq $f;
  opendir(my $dh, $dir) or next;
  my @go = grep { /\.go$/ && -f "$dir/$_" } readdir $dh;
  closedir $dh;
  next unless @go;
  push @{ $pkg{$dir}{whole} }, "deleted-test-file $f";
}
# ASD-V0-5135: a changed compiled non-Go source or any other non-testdata file in the directory
# of a package with tests runs it whole; a changed go.mod or go.sum turns every narrowed package
# of its module into a whole run.
my %compiled = map { $_ => 1 } qw(.s .S .c .h .cc .cpp .cxx .hh .hpp .m .mm .syso);
my (%pkgfile, @modules);
for my $p (@allpaths) {
  next if $p =~ /\.go$/ || $p =~ m{(^|/)testdata/};
  if ($p =~ m{^(?:(.*)/)?go\.(?:mod|sum)$}) { push @modules, [defined $1 ? $1 : ".", $p]; next }
  (my $dir = $p) =~ s{/[^/]*$}{}; $dir = "." if $dir eq $p;
  next unless $testdirs{$dir};
  my ($ext) = $p =~ /(\.[^.\/]+)$/;
  $pkgfile{$dir} //= ((defined $ext && $compiled{$ext}) ? "source-edit" : "package-file") . " $p";
}
@modules = sort { length $b->[0] <=> length $a->[0] || $a->[1] cmp $b->[1] } @modules;
sub module_of {
  my ($dir) = @_;
  for my $m (@modules) { return $m->[1] if $m->[0] eq "." || $dir eq $m->[0] || index($dir, "$m->[0]/") == 0 }
  return;
}
for my $dir (keys %testdirs) {
  my @first = grep { defined } $srcwhole{$dir}, $pkgfile{$dir};
  next unless @first;
  unshift @{ $pkg{$dir}{whole} }, @first;
}
my ($tests_total, $whole_total, @out) = (0, 0);
for my $dir (sort keys %pkg) {
  my $d = $pkg{$dir};
  my @decls = @{ $d->{decls} || [] };
  my @whole = @{ $d->{whole} || [] };
  my %marked;    # symbol => "FILE:LINE KEY"
  for my $x (@decls) {
    next unless defined $x->{direct} || defined $x->{changed};
    if (defined $x->{direct}) {
      my $line = $x->{start}; my @tl = split /\n/, $x->{text};
      for my $k (0 .. $#tl) { if ($tl[$k] =~ $key) { $line = $x->{start} + $k; last } }
      $x->{why} = "$x->{file}:$line $x->{direct}";
    } else { $x->{why} = $x->{changed} }
    if ($x->{kind} eq "main") { push @whole, (defined $x->{direct} ? "$x->{name}[0]-reads-key" : "$x->{name}[0]-changed") . " $x->{why}"; next }
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
  if (defined(my $module = module_of($dir))) { push @out, "whole: ./$dir reason=module-edit $module"; $whole_total++; next }
  my %seen; @sel = grep { !$seen{$_->[0]}++ } sort { $a->[0] cmp $b->[0] } @sel;
  $tests_total += @sel;
  push @out, sprintf "run: script/heavy.sh go test -count=1 -run %s ./%s", "\x27^(" . join("|", map { $_->[0] } @sel) . ")\$\x27", $dir;
  push @out, map { "  why $_->[0] $_->[1]" } @sel;
}
print "$_\n" for @out;
printf "test-run-pattern: SUMMARY keys=paths:%d,ids:%d packages=%d tests=%d whole=%d sources=%d\n", $npaths, $nids,
  scalar(grep { /^(?:run|whole):/ } @out), $tests_total, $whole_total, $nsources;
' "$work/paths" "$work/ids" "$work/tests" "$work/shared-bases" "$work/sources" "$work/hunks" "$base"
