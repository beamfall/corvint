#!/usr/bin/env bash
# ASD-V0-576 (V1-3390): script/changed-checks.sh runs line-citations-check,
# unbounded-readers-check and test-read-scopes-check only when the diff from BASE reaches their
# inputs, and fails naming each failed check. Temporary Git fixtures and a stub make only.
set -uo pipefail
script=$(cd "$(dirname "$0")" && pwd)/changed-checks.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/changed-checks-test.XXXXXX") || exit 2
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'changed-checks_test: FAIL %s\n' "$1" >&2; exit 1; }
printf '#!/bin/sh\necho "$1" >> "%s/ran"\n[ "$1" != "${STUB_FAIL-}" ]\n' "$tmp" > "$tmp/make" && chmod +x "$tmp/make"
export CHANGED_CHECKS_MAKE="$tmp/make"
mkdir "$tmp/repo" && cd "$tmp/repo" || exit 2
git init -q . && git config user.email t@example.invalid && git config user.name t || exit 2
mkdir -p docs internal/x script
printf 'See `internal/x/a.go:1`.\n' > docs/spec.md
printf 'package x\n' > internal/x/a.go
printf 'package x\n' > internal/x/b.go
printf '#!/bin/sh\n' > script/s.sh
git add -A && git commit -q -m base || exit 2
base=$(git rev-parse HEAD)
ran() { if [[ -f $tmp/ran ]]; then tr '\n' ' ' < "$tmp/ran"; fi; /bin/rm -f "$tmp/ran"; }

# 1. No change: nothing runs.
out=$("$script" "$base") || fail "no change: rc=$?"
[[ -z $(ran) ]] || fail "a check ran with no change"

# 2. A cited, uncommitted Go edit runs only line-citations-check.
printf 'package x // edited\n' > internal/x/a.go
out=$("$script" "$base") || fail "cited edit: rc=$? $out"
[[ $(ran) == 'line-citations-check ' ]] || fail "cited edit ran the wrong checks: $out"
grep -qF 'run line-citations-check (changed: internal/x/a.go)' <<< "$out" && grep -qF 'skip unbounded-readers-check' <<< "$out" || fail "cited edit output: $out"
git checkout -q -- internal/x/a.go

# 3. An uncited Go edit runs nothing; a committed script edit runs unbounded-readers-check.
printf 'package x // edited\n' > internal/x/b.go
"$script" "$base" > /dev/null || fail "uncited edit"
[[ -z $(ran) ]] || fail "an uncited edit ran a check"
printf '#!/bin/sh\n# edited\n' > script/s.sh && git commit -q -am script || exit 2
"$script" "$base" > /dev/null || fail "script edit"
[[ $(ran) == 'unbounded-readers-check ' ]] || fail "script edit ran the wrong checks"

# 4. A new untracked test file runs test-read-scopes-check; a failed check fails the run, named.
printf 'package x\n' > internal/x/b_test.go
out=$(STUB_FAIL=test-read-scopes-check "$script" "$base" 2>&1)
[[ $? == 1 ]] || fail "failed check: not exit 1: $out"
[[ $(ran) == 'unbounded-readers-check test-read-scopes-check ' ]] || fail "new test ran the wrong checks: $out"
grep -qF 'changed-checks: FAILED test-read-scopes-check' <<< "$out" || fail "failed check output: $out"

# 5. Usage errors.
"$script" > /dev/null 2>&1; [[ $? == 2 ]] || fail "no BASE is not a usage error"
"$script" not-a-rev > /dev/null 2>&1; [[ $? == 2 ]] || fail "a bad BASE is not a usage error"
printf 'changed-checks_test: ok\n'
