#!/usr/bin/env bash
# usage: script/check-test-exec-writes.sh
#
# WQO-V0-921 (V1-2587): a test that writes a file with os.WriteFile and then executes it can fail
# with ETXTBSY when a parallel test forks while the write descriptor is open; execfile.WriteFile
# (WQO-V0-093) closes that race. For every indexed *_test.go file it prints
# `check-test-exec-writes: FAIL PATH:LINE: ...` with the fix for each os.WriteFile call whose
# permission argument is an octal literal (bare, or wrapped in os.FileMode or fs.FileMode) with an
# execute bit. internal/execfile itself, testdata trees and nested Go modules, which cannot import
# internal/execfile, are exempt; so is internal/tasks/cli until its sweep lands (V1-2587). A
# permission held in a variable is not seen. It exits 1 on any failure, 2 when the index cannot be
# read, and 0 otherwise.
set -uo pipefail
export LC_ALL=C
cd "$(git rev-parse --show-toplevel)" || { printf 'check-test-exec-writes: not in a Git worktree\n' >&2; exit 2; }

modules=$(git ls-files --cached -- '*go.mod') || { printf 'check-test-exec-writes: cannot read the index\n' >&2; exit 2; }
files=$(git grep -l --cached -F 'os.WriteFile' -- '*_test.go')
status=$?
((status <= 1)) || { printf 'check-test-exec-writes: cannot read the index\n' >&2; exit 2; }
nested=$(printf '%s\n' "$modules" | sed -n 's|/go.mod$|/|p')
failed=0
while IFS= read -r path; do
  [[ -n $path ]] || continue
  case $path in internal/execfile/* | */testdata/* | testdata/* | internal/tasks/cli/*) continue ;; esac
  skip=0
  while IFS= read -r prefix; do
    [[ -n $prefix && $path == "$prefix"* ]] && { skip=1; break; }
  done <<< "$nested"
  ((skip)) && continue
  git show ":$path" | perl -e '
    my $path = shift;
    my $src = do { local $/; <STDIN> };
    my $args = qr{(\((?:"(?:[^"\\\n]|\\.)*"|`[^`]*`|'"'"'(?:[^'"'"'\\\n]|\\.)+'"'"'|//[^\n]*|/\*.*?\*/|[^()"`'"'"'/]++|/|(?1))*\))}s;
    my $perm = qr{,\s*(?:(?:os|fs)\.FileMode\(\s*)?(0o?[0-7]{3,4})\s*\)?\s*,?\s*\)\z};
    my $bad = 0;
    while ($src =~ /\bos\.WriteFile$args/g) {
        my ($call, $at) = ($1, $-[0]);
        next unless $call =~ $perm;
        my $raw = $1;
        (my $mode = $raw) =~ s/^0o/0/;
        next unless oct($mode) & 0111;
        my $line = 1 + (substr($src, 0, $at) =~ tr/\n//);
        print "check-test-exec-writes: FAIL $path:$line: os.WriteFile writes an executable (mode $raw), which a parallel fork can leave busy (ETXTBSY) when the test runs it; use execfile.WriteFile from internal/execfile (WQO-V0-921)\n";
        $bad = 1;
    }
    exit $bad;
  ' "$path" || failed=1
done <<< "$files"
exit "$failed"
