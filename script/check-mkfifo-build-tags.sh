#!/usr/bin/env bash
# usage: script/check-mkfifo-build-tags.sh
#
# ASD-V0-583 (V1-3373): syscall.Mkfifo is undefined on solaris and illumos, so a Go file that calls
# it must exclude them. `make cross-vet` (GOOS=solaris) catches the regression only in the nightly
# gate; this doc gate catches it in seconds. For every tracked .go file in the index that names
# syscall.Mkfifo it prints `check-mkfifo-build-tags: FAIL PATH: ...` with the fix when
#   1. its //go:build line names `unix` without `!solaris` (unix includes solaris and illumos), or
#   2. it has no //go:build line and no GOOS file-name suffix, so it builds on every platform.
# An explicit GOOS list such as `darwin || linux` or `unix && !solaris` passes. It exits 1 on any
# failure, 2 when the index cannot be read, and 0 otherwise.
set -uo pipefail
export LC_ALL=C
cd "$(git rev-parse --show-toplevel)" || { printf 'check-mkfifo-build-tags: not in a Git worktree\n' >&2; exit 2; }

files=$(git grep -l --cached -F 'syscall.Mkfifo' -- '*.go')
status=$?
((status <= 1)) || { printf 'check-mkfifo-build-tags: cannot read the index\n' >&2; exit 2; }
failed=0
goos_suffix='_(aix|android|darwin|dragonfly|freebsd|hurd|ios|js|linux|netbsd|openbsd|plan9|wasip1|windows)(_[a-z0-9]+)?(_test)?\.go$'
[[ -n $files ]] || exit 0
# LVT-V0-006 (V1-4252): one `git cat-file --batch` reads every named file's staged blob, in order,
# and prints `PATH<TAB>CONSTRAINT` (the first //go:build line before the package clause, empty when
# there is none), instead of one `git show` per file; a missing or unparsable object exits 2.
list=$(mktemp "${TMPDIR:-/tmp}/check-mkfifo-build-tags.XXXXXX") || exit 2
trap '/bin/rm -f "$list"' EXIT
printf '%s\n' "$files" > "$list"
# shellcheck disable=SC2016 # the Perl program is single-quoted on purpose.
constraints=$(sed 's/^/:/' "$list" | git cat-file --batch | perl -e '
  open(my $paths, "<", $ARGV[0]) or exit 2;
  binmode STDIN;
  while (my $path = <$paths>) {
    chomp $path;
    my $header = <STDIN>;
    defined $header or exit 2;
    my (undef, $type, $size) = split / /, $header;
    defined $size && $type eq "blob" && $size =~ /^[0-9]+$/ or exit 2;
    my $got = read(STDIN, my $text, $size);
    defined $got && $got == $size or exit 2;
    defined(<STDIN>) or exit 2;
    my $constraint = "";
    for my $line (split /\n/, $text) {
      last if $line =~ /^package /;
      if ($line =~ s{^//go:build }{}) { $constraint = $line; last }
    }
    print "$path\t$constraint\n";
  }' "$list") || { printf 'check-mkfifo-build-tags: cannot read the index\n' >&2; exit 2; }
while IFS=$'\t' read -r path constraint; do
  [[ -n $path ]] || continue
  if [[ -z $constraint ]]; then
    [[ $path =~ $goos_suffix ]] && continue
    printf 'check-mkfifo-build-tags: FAIL %s: calls syscall.Mkfifo with no //go:build constraint; add //go:build unix && !solaris (Mkfifo is undefined on solaris, illumos and windows)\n' "$path"
    failed=1
  elif [[ " $constraint " =~ [^!a-z0-9_]unix[^a-z0-9_] && $constraint != *'!solaris'* ]]; then
    printf 'check-mkfifo-build-tags: FAIL %s: calls syscall.Mkfifo under //go:build %s, which includes solaris and illumos where Mkfifo is undefined; use //go:build unix && !solaris\n' "$path" "$constraint"
    failed=1
  fi
done <<< "$constraints"
exit "$failed"
