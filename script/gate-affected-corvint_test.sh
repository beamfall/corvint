#!/bin/sh
set -eu

# Exercises script/gate-affected-corvint.sh (STR-V0-012 rollout step 2) on a two-package Go
# repository with real go test: a clean tree runs nothing, an edit runs a narrowed set, a
# dirty go.mod runs FULL, BASE joins the committed range, an unresolvable BASE is a usage
# error, a failing test fails the run, the receipt lands under the Git directory, and SIGTERM
# reaches the running corvint.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-gate-affected-corvint-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

(cd "$source_root" && GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go build -o "$test_root/corvint" ./cmd/corvint)

repository="$test_root/repo"
mkdir -p "$repository/a" "$repository/b" "$repository/.github/cishards"
printf 'module example.test/gate\n\ngo 1.27\n' > "$repository/go.mod"
printf 'package a\n\nfunc A() int { return 1 }\n' > "$repository/a/a.go"
printf 'package a\n\nimport "testing"\n\nfunc TestA(t *testing.T) {\n\tif A() != 1 {\n\t\tt.Fatal(A())\n\t}\n}\n' > "$repository/a/a_test.go"
printf 'package b\n\nimport "example.test/gate/a"\n\nfunc B() int { return a.A() + 1 }\n' > "$repository/b/b.go"
printf 'package b\n\nimport "testing"\n\nfunc TestB(t *testing.T) { _ = B() }\n' > "$repository/b/b_test.go"
cat > "$repository/.github/cishards/test-commands.json" <<'EOF'
{
  "profile": "corvint-test-commands/0",
  "go": {"env": {"GOTOOLCHAIN": "local"}, "flags": ["-count=1"]},
  "languages": {}
}
EOF
git -C "$repository" init -q -b main
git -C "$repository" config user.name test
git -C "$repository" config user.email test@example.invalid
git -C "$repository" config maintenance.auto false
git -C "$repository" config gc.auto 0
git -C "$repository" add .
git -C "$repository" commit -qm fixture
first=$(git -C "$repository" rev-parse HEAD)

run_gate() {
  status=0
  (cd "$repository" && CORVINT_TEST_BINARY="$test_root/corvint" GOCACHE=/tmp/corvint-go-build-cache \
    "$source_root/script/gate-affected-corvint.sh" "$@" > "$test_root/stdout" 2> "$test_root/stderr") || status=$?
}

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  sed 's/^/  stdout: /' "$test_root/stdout" >&2
  sed 's/^/  stderr: /' "$test_root/stderr" >&2
  exit 1
}

expect_status() { test "$status" -eq "$1" || fail "exit $status, want $1 ($2)"; }
expect_err() { grep -qF -- "$1" "$test_root/stderr" || fail "stderr lacks '$1'"; }
expect_out() { grep -qF -- "$1" "$test_root/stdout" || fail "stdout lacks '$1'"; }
# go test reports one `ok|FAIL <package>` line per package it ran.
ran() { grep -qE "^(ok|FAIL)[[:space:]]+$1([[:space:]]|\$)" "$test_root/stdout" || fail "$1 did not run"; }
not_ran() { if grep -qE "^(ok|FAIL)[[:space:]]+$1([[:space:]]|\$)" "$test_root/stdout"; then fail "$1 ran"; fi; }
reset_repository() { git -C "$repository" checkout -q -- . && git -C "$repository" clean -qfd; }

# A clean tree with an empty BASE (HEAD) tests nothing and passes.
run_gate ""
expect_status 0 "clean"
expect_out "gate-affected: base $first (HEAD)"
expect_err "corvint-test: NOTHING"
test -f "$repository/.git/corvint/test-run.json" || fail "no receipt under the Git directory"

# Editing a leaf of the import graph narrows to it; b reaches a, a does not reach b.
printf '\n// edited\n' >> "$repository/b/b.go"
run_gate ""
expect_status 0 "narrowed"
expect_err "corvint-test: NARROWED"
ran example.test/gate/b
not_ran example.test/gate/a
reset_repository

# A dirty root module definition runs FULL over every package.
printf '\n// edited\n' >> "$repository/go.mod"
run_gate ""
expect_status 0 "go.mod FULL"
expect_err "corvint-test: FALLBACK"
ran example.test/gate/a
ran example.test/gate/b
reset_repository

# BASE joins the committed range: a committed change to a reaches a and b on a clean tree.
printf 'package a\n\nfunc A() int { return 1 + 0 }\n' > "$repository/a/a.go"
git -C "$repository" commit -qam "change a"
run_gate "$first"
expect_status 0 "committed range"
expect_out "gate-affected: base $first ($first)"
expect_err "corvint-test: NARROWED"
ran example.test/gate/a
ran example.test/gate/b
run_gate main
expect_status 0 "clean against main"
expect_err "corvint-test: NOTHING"

# A failing selected test fails the run with the runner's FAIL status.
printf 'package a\n\nfunc A() int { return 2 }\n' > "$repository/a/a.go"
run_gate ""
expect_status 1 "failing test"
reset_repository

# An unresolvable or option-shaped BASE is a usage error and runs nothing.
run_gate no-such-revision
expect_status 2 "bad base"
expect_err "gate-affected: base no-such-revision does not resolve to a commit"
run_gate --base
expect_status 2 "option base"
expect_err "gate-affected: base --base is not a revision"

# SIGTERM to the gate is forwarded to the running corvint, which owns and kills its process
# groups, and the gate exits 143 without starting anything else.
# shellcheck disable=SC2016
printf '#!/bin/sh\ntrap '"'"'echo forwarded > "%s/forwarded"; exit 143'"'"' TERM\nkill -TERM "$PPID"\nsleep 30 &\nwait\n' "$test_root" > "$test_root/terminated-corvint"
chmod +x "$test_root/terminated-corvint"
status=0
(cd "$repository" && CORVINT_TEST_BINARY="$test_root/terminated-corvint" \
  "$source_root/script/gate-affected-corvint.sh" > "$test_root/stdout" 2> "$test_root/stderr") || status=$?
expect_status 143 "terminated"
test -f "$test_root/forwarded" || fail "SIGTERM was not forwarded to corvint"

echo "gate-affected-corvint tests passed"
