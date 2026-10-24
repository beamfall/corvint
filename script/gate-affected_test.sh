#!/bin/sh
set -eu

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-gate-affected-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

(cd "$source_root" && GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go build -o "$test_root/corvint" ./cmd/corvint)

repository="$test_root/repo"
cp -R "$source_root/internal/liveverify/affected/testdata/fixture" "$repository"
git -C "$repository" init -q -b main
git -C "$repository" -c user.name=test -c user.email=test@example.invalid add .
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qm fixture

# The recorder stands in for go test: it prints `go-test` then one package pattern per line.
# fail names the broken expectation and shows what the gate printed, so a failed run never
# exits silently under set -e.
fail() {
  printf 'FAIL: %s\n' "$1" >&2
  printf '%s\n' "$output" | sed 's/^/  stdout: /' >&2
  sed 's/^/  stderr: /' "$test_root/stderr" >&2
  exit 1
}

# The planner's default graph cache (AFP-V0-436) lives under the test root, never the
# real user cache directory.
run_gate() {
  status=0
  output=$(cd "$repository" && CORVINT_PLANNER="env HOME=$test_root/home XDG_CACHE_HOME=$test_root/cache $test_root/corvint" GO_TEST_COMMAND='printf "%s\n" go-test' \
    "$source_root/script/gate-affected.sh" "$@" 2> "$test_root/stderr") || status=$?
  test "$status" -eq 0 || fail "gate-affected.sh $* exited $status"
}

# has_line is the predicate; expect_line fails the test, naming the missing line.
has_line() { printf '%s\n' "$output" | grep -qxF -- "$1"; }
expect_line() { has_line "$1" || fail "missing line: $1"; }
# A package the planner already selected is printed once as `selected`; the selector adds a
# `frontier` line only for a package the plan did not name (AFP-V0-011, AFP-V0-012 rule (a)).
expect_reached() {
  printf '%s\n' "$output" | grep -qxE -- "gate-affected: (selected $1|frontier $1 <- $2)" ||
    fail "missing line: gate-affected: selected $1 (or frontier $1 <- $2)"
  expect_line "$1"
}
expect_fallback() {
  grep -qF -- "gate-affected: FALLBACK $1" "$test_root/stderr" || fail "missing stderr line: gate-affected: FALLBACK $1"
  expect_line ./...
}
reset_repository() { git -C "$repository" checkout -q -- . && git -C "$repository" clean -qfd; }

run_gate
expect_line 'gate-affected: no dirty or committed path; no Go package to test'
if has_line go-test; then fail "a clean tree ran go test"; fi

printf '\n// edited\n' >> "$repository/core/core.go"
run_gate
expect_line 'gate-affected: selected example.com/fixture/core'
expect_line 'gate-affected: selected example.com/fixture/leaf'
expect_line example.com/fixture/core
if grep -q FALLBACK "$test_root/stderr"; then fail "a core edit fell back"; fi
reset_repository

# A deleted source still reaches its package and that package's importers.
rm "$repository/core/core.go"
run_gate
expect_reached example.com/fixture/core core/core.go
expect_reached example.com/fixture/mid core/core.go
expect_reached example.com/fixture/leaf core/core.go
reset_repository

# No fixture package encloses or names NOTES.md, so it narrows nothing and widens nothing.
printf 'notes\n' > "$repository/NOTES.md"
printf '\n// edited\n' >> "$repository/core/core.go"
run_gate
expect_line 'gate-affected: data NOTES.md'
expect_line example.com/fixture/core
if grep -q FALLBACK "$test_root/stderr" || has_line example.com/fixture/solo; then fail "an unread document widened the run"; fi
reset_repository

printf '\n// require nothing\n' >> "$repository/go.mod"
run_gate
expect_fallback 'root module definition is dirty: go.mod'
reset_repository

base=$(git -C "$repository" rev-parse HEAD)
printf '\n// committed\n' >> "$repository/core/core.go"
git -C "$repository" -c user.name=test -c user.email=test@example.invalid commit -qam 'edit core'
run_gate "$base"
expect_line "gate-affected: base $base ($base)"
expect_line 'gate-affected: selected example.com/fixture/core'
expect_line example.com/fixture/core

run_gate no-such-ref
expect_fallback 'base no-such-ref does not resolve to a commit'

# An interrupt while planning stops the gate; it does not fall back to the full go-test run.
# shellcheck disable=SC2016
printf '#!/bin/sh\nkill -INT "$PPID"\nexit 130\n' > "$test_root/interrupted-planner"
chmod +x "$test_root/interrupted-planner"
status=0
output=$(cd "$repository" && CORVINT_PLANNER="$test_root/interrupted-planner" GO_TEST_COMMAND='printf "%s\n" go-test' \
  "$source_root/script/gate-affected.sh" 2> "$test_root/stderr") || status=$?
if test "$status" -eq 0 || has_line go-test; then fail "an interrupted plan ran go test (exit $status)"; fi

echo "gate-affected_test.sh: ok"
