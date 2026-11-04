#!/usr/bin/env bash
# Tests for the .github split check (script/github-split-check.sh, ASD-V0-706) on a temporary Git
# fixture only. Run by `make lane-scripts-test`. No Go build runs.
set -uo pipefail
export LC_ALL=C
unset $(git rev-parse --local-env-vars)

script=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-github-split-check-test.XXXXXX") || exit 1
trap '/bin/rm -rf "$tmp"' EXIT
failed=0 passed=0
fail() { printf 'github-split-check_test: FAIL %s\n' "$*" >&2; failed=$((failed + 1)); }
expect_line() { grep -qE -- "$2" "$1" || fail "$3: no line matching /$2/ in: $(cat "$1")"; }
reject_line() { ! grep -qE -- "$2" "$1" || fail "$3: unexpected line matching /$2/ in: $(cat "$1")"; }
check_rc() { [[ $1 == "$2" ]] && passed=$((passed + 1)) || fail "$3: exit $1, want $2"; }

g() { git -C "$repo" -c user.name=t -c user.email=t@example.invalid -c commit.gpgsign=false "$@"; }
repo=$tmp/repo
out=$tmp/out
git init -q -b main "$repo" || exit 1
mkdir -p "$repo/internal/foo" "$repo/script"
printf 'package foo\n' > "$repo/internal/foo/a.go"
/bin/cp -f "$script/github-split-check.sh" "$repo/script/github-split-check.sh"
g add -A && g commit -q -m 'init' || exit 1
g update-ref refs/remotes/origin/main HEAD

# A .github-only or code-only range passes; a mixed range names the .github commit (V1-3292).
g checkout -q -b gh-split origin/main
mkdir -p "$repo/.github"
printf 'x: 1\n' > "$repo/.github/ci.yml" && g add -A && g commit -q -m 'ci: tune' && ghc=$(g rev-parse HEAD)
(cd "$repo" && script/github-split-check.sh origin/main) > "$out" 2>&1
check_rc $? 0 'github-only range'
expect_line "$out" '^github-split-check: ok github-commits=1 ' 'github-only ok'
printf 'package foo\n\nvar Y = 2\n' > "$repo/internal/foo/a.go" && g commit -q -am 'feat: code'
(cd "$repo" && script/github-split-check.sh) > "$out" 2>&1
check_rc $? 1 'mixed range'
expect_line "$out" "^refuse github-in-code-range $ghc ci: tune\$" 'names the .github commit'
expect_line "$out" 'fix: move each named commit to its own side branch' 'split recovery'
# A log the parser cannot classify refuses instead of passing the mixed range.
mkdir -p "$tmp/gitjunk"
printf '#!/bin/sh\nfor a; do [ "$a" = log ] && { printf %s; exit 0; }; done\nexec %s "$@"\n' "'junk\\0'" "$(command -v git)" > "$tmp/gitjunk/git"
chmod +x "$tmp/gitjunk/git"
(cd "$repo" && PATH=$tmp/gitjunk:$PATH script/github-split-check.sh) > "$out" 2>&1
check_rc $? 2 'split parser failure'
expect_line "$out" 'REFUSE parse-failed ' 'parser failure refuses'
reject_line "$out" 'github-split-check: ok' 'no ok on parser failure'
# A log that silently drops commits (empty output, exit 0) refuses too.
mkdir -p "$tmp/gitempty"
printf '#!/bin/sh\nfor a; do [ "$a" = log ] && exit 0; done\nexec %s "$@"\n' "$(command -v git)" > "$tmp/gitempty/git"
chmod +x "$tmp/gitempty/git"
(cd "$repo" && PATH=$tmp/gitempty:$PATH script/github-split-check.sh) > "$out" 2>&1
check_rc $? 2 'split dropped log'
(cd "$repo" && script/github-split-check.sh HEAD~1) > "$out" 2>&1
check_rc $? 0 'code-only range'
(cd "$repo" && script/github-split-check.sh nosuchref) > "$out" 2>&1
check_rc $? 2 'split unknown base'

# Paths are read NUL-delimited, so a workflow path Git would display-quote (non-ASCII, a tab, a
# newline) is still a .github path in a mixed range.
for name in $'\xc3\xa9' $'t\tb' $'n\nl'; do
  g checkout -q -B gh-quoted origin/main
  mkdir -p "$repo/.github/workflows"
  printf 'x: 1\n' > "$repo/.github/workflows/$name.yml" && g add -A && g commit -q -m 'ci: quoted path' && qc=$(g rev-parse HEAD)
  printf 'package foo\n\nvar Z = 3\n' > "$repo/internal/foo/a.go" && g commit -q -am 'feat: code'
  (cd "$repo" && script/github-split-check.sh origin/main) > "$out" 2>&1
  check_rc $? 1 "mixed range with quoted workflow path $(printf %q "$name")"
  expect_line "$out" "^refuse github-in-code-range $qc ci: quoted path\$" "names the quoted-path commit $(printf %q "$name")"
done
# The other direction: a .github-only range with a quoted path, and a code-only range with one.
g checkout -q -B gh-quoted-only origin/main
mkdir -p "$repo/.github/workflows" && printf 'x: 1\n' > "$repo/.github/workflows/"$'\xc3\xa9'".yml"
g add -A && g commit -q -m 'ci: quoted only'
(cd "$repo" && script/github-split-check.sh origin/main) > "$out" 2>&1
check_rc $? 0 'github-only range with a quoted path'
expect_line "$out" '^github-split-check: ok github-commits=1 ' 'quoted github-only counts its commit'
g checkout -q -B code-quoted-only origin/main
printf 'package foo\n' > "$repo/internal/foo/"$'\xc3\xa9'".go" && g add -A && g commit -q -m 'feat: quoted code'
(cd "$repo" && script/github-split-check.sh origin/main) > "$out" 2>&1
check_rc $? 0 'code-only range with a quoted path'
expect_line "$out" '^github-split-check: ok github-commits=0 ' 'quoted code-only ok'
# An empty range passes.
(cd "$repo" && script/github-split-check.sh HEAD) > "$out" 2>&1
check_rc $? 0 'empty range'
expect_line "$out" '^github-split-check: ok github-commits=0 ' 'empty range ok'

if ((failed)); then
  printf 'github-split-check_test: %d checks passed, %d failed\n' "$passed" "$failed" >&2
  exit 1
fi
printf 'github-split-check_test: %d checks passed\n' "$passed"
