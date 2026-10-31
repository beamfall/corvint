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
while IFS= read -r path; do
  [[ -n $path ]] || continue
  constraint=$(git show ":$path" | sed -n '/^package /q; s/^\/\/go:build //p' | head -1)
  if [[ -z $constraint ]]; then
    [[ $path =~ $goos_suffix ]] && continue
    printf 'check-mkfifo-build-tags: FAIL %s: calls syscall.Mkfifo with no //go:build constraint; add //go:build unix && !solaris (Mkfifo is undefined on solaris, illumos and windows)\n' "$path"
    failed=1
  elif [[ " $constraint " =~ [^!a-z0-9_]unix[^a-z0-9_] && $constraint != *'!solaris'* ]]; then
    printf 'check-mkfifo-build-tags: FAIL %s: calls syscall.Mkfifo under //go:build %s, which includes solaris and illumos where Mkfifo is undefined; use //go:build unix && !solaris\n' "$path" "$constraint"
    failed=1
  fi
done <<< "$files"
exit "$failed"
