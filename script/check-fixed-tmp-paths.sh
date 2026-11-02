#!/usr/bin/env bash
# usage: script/check-fixed-tmp-paths.sh
#
# ASD-V0-720 (V1-3847): a test that writes under the fixed /tmp, or hands a child TMPDIR=/tmp,
# ignores the TMPDIR the lane or CI gave it; on a host whose /tmp is a full tmpfs the test fails
# with "no space left on device" while its own TMPDIR has room. For every indexed *_test.go line
# that calls MkdirTemp, CreateTemp, WriteFile, Mkdir, MkdirAll, Create or OpenFile on a "/tmp"
# literal, joins onto "/tmp", or sets TMPDIR to "/tmp" in an environment entry or Setenv, it
# prints `check-fixed-tmp-paths: FAIL PATH:LINE: ...` with the fix, unless the line carries
# `// fixed-tmp: REASON` or the line before it does. A literal may be a "..." or raw `...`
# string, and a call's argument may start on a later line; LINE is the line where the match
# starts. It exits 1 on any failure, 2 when the index cannot be read, and 0 otherwise.
set -uo pipefail
export LC_ALL=C
cd "$(git rev-parse --show-toplevel)" || { printf 'check-fixed-tmp-paths: not in a Git worktree\n' >&2; exit 2; }

# Candidate files hold a /tmp literal; the patterns then match across line breaks.
candidates=$(git grep -l -z --cached -F /tmp -- '*_test.go' | tr '\0' '\n')
status=$?
((status <= 1)) || { printf 'check-fixed-tmp-paths: cannot read the index\n' >&2; exit 2; }
[[ -n $candidates ]] || exit 0
# shellcheck disable=SC2016 # the Perl program is single-quoted on purpose.
perl -e '
  my ($q, $failed) = ("[\"`]", 0);
  my $pattern = qr{(?:MkdirTemp|CreateTemp|WriteFile|MkdirAll|Mkdir|Create|OpenFile)\(\s*$q/tmp[/"`]
    | filepath\.Join\(\s*$q/tmp$q | ${q}TMPDIR=/tmp[/"`] | Setenv\(\s*${q}TMPDIR$q,\s*$q/tmp[/"`]}x;
  while (my $path = <STDIN>) {
    chomp $path;
    open(my $blob, "-|", "git", "show", ":$path") or exit 2;
    my $text = do { local $/; <$blob> };
    close($blob) or exit 2;
    my @lines = split /\n/, $text, -1;
    while ($text =~ /$pattern/g) {
      my $line = 1 + (substr($text, 0, $-[0]) =~ tr/\n//);
      next if grep { defined && index($_, "// fixed-tmp:") >= 0 } @lines[$line - 1, $line > 1 ? $line - 2 : $line - 1];
      print "check-fixed-tmp-paths: FAIL $path:$line: writes under or exports the fixed /tmp; use t.TempDir() or os.TempDir(), or mark the line // fixed-tmp: REASON\n";
      $failed = 1;
    }
  }
  exit $failed;
' <<< "$candidates"
