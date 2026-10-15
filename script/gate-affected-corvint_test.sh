#!/bin/sh
set -eu

# Exercises script/gate-affected-corvint.sh (STR-V0-012 rollout step 2) on a two-package Go
# repository with real go test: a clean tree runs nothing, an edit runs a narrowed set, a
# dirty go.mod runs FULL, BASE joins the committed range, an unresolvable BASE is a usage
# error, a failing test fails the run, the receipt lands under the Git directory, SIGTERM
# reaches the running corvint, and HUP, INT or TERM during the build stops the whole build.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-gate-affected-corvint-test.XXXXXX")
# cleanup also stops any build stand-in or sleeper a failed signal case left behind; a pid is
# killed only while it still names one of those commands, never a reused unrelated pid.
cleanup() {
  for marker in "$test_root"/*.pid; do
    [ -f "$marker" ] || continue
    pid=$(cat "$marker")
    case "$(ps -p "$pid" -o comm= 2> /dev/null)" in
      *sh | *go | *sleep) kill -s KILL "$pid" 2> /dev/null || true ;;
    esac
  done
  rm -rf "$test_root"
}
# gate_affected runs the wrapper, under GATE_AFFECTED_TEST_SHELL (for example dash) when it is
# set and otherwise through its own #!/bin/sh.
gate_affected() {
  if [ -n "${GATE_AFFECTED_TEST_SHELL:-}" ]; then
    "$GATE_AFFECTED_TEST_SHELL" "$source_root/script/gate-affected-corvint.sh" "$@"
  else
    "$source_root/script/gate-affected-corvint.sh" "$@"
  fi
}
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
    gate_affected "$@" > "$test_root/stdout" 2> "$test_root/stderr") || status=$?
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
# The stand-in stops its own sleeper when the forwarded signal arrives.
cat > "$test_root/terminated-corvint" <<'EOF_CORVINT'
#!/bin/sh
if [ -n "$GATE_TEST_IGNORE_TERM" ]; then
  (trap '' TERM && exec sleep 30) &
else
  sleep 30 &
fi
sleeper=$!
echo "$sleeper" > "$GATE_TEST_DIR/corvint-sleep.pid"
trap 'echo forwarded > "$GATE_TEST_DIR/forwarded"; kill "$sleeper"; exit 143' TERM
kill -TERM "$PPID"
wait
EOF_CORVINT
chmod +x "$test_root/terminated-corvint"
status=0
(cd "$repository" && CORVINT_TEST_BINARY="$test_root/terminated-corvint" GATE_TEST_DIR="$test_root" \
  gate_affected > "$test_root/stdout" 2> "$test_root/stderr") || status=$?
expect_status 143 "terminated"
test -f "$test_root/forwarded" || fail "SIGTERM was not forwarded to corvint"

# A hangup, interrupt or termination during the build of corvint stops the build and every
# process below it, and the gate exits 128+N without starting tests (V1-1310). The stand-in
# build records its pid and a sleeping descendant's, signals the gate, and waits; it writes
# build-finished only if it outlives its sleeper, which a deferred signal would allow. The last
# case's sleeper ignores TERM, so only the gate's bounded KILL stops it. In the no-ps case `ps`
# fails, so the gate walks a stand-in /proc tree that the build writes, with a command name
# holding a space, a parenthesis and a newline (V1-1400). No wall-clock bound is asserted, so a
# loaded host cannot fail the case.
mkdir -p "$test_root/fake-go" "$test_root/no-ps"
cat > "$test_root/fake-go/go" <<'EOF_GO'
#!/bin/sh
if [ -n "$GATE_TEST_IGNORE_TERM" ]; then
  (trap '' TERM && exec sleep 30) &
else
  sleep 30 &
fi
echo "$!" > "$GATE_TEST_DIR/sleep.pid"
echo "$$" > "$GATE_TEST_DIR/build.pid"
if [ -n "$CORVINT_TEST_PROC_ROOT" ]; then
  mkdir -p "$CORVINT_TEST_PROC_ROOT/$$" "$CORVINT_TEST_PROC_ROOT/$!"
  echo "$$ (go) S $PPID $$ 0" > "$CORVINT_TEST_PROC_ROOT/$$/stat"
  printf '%s (sl\n) eep) S %s %s 0\n' "$!" "$$" "$$" > "$CORVINT_TEST_PROC_ROOT/$!/stat"
fi
kill -s "$GATE_TEST_SIGNAL" "$PPID"
wait
echo finished > "$GATE_TEST_DIR/build-finished"
EOF_GO
chmod +x "$test_root/fake-go/go"
printf '#!/bin/sh
echo "ps: unavailable" >&2
exit 1
' > "$test_root/no-ps/ps"
chmod +x "$test_root/no-ps/ps"
alive() { kill -0 "$1" 2> /dev/null; }
for case in HUP:129:: INT:130:: TERM:143:: INT:130:ignore: TERM:143::no-ps; do
  IFS=: read -r signal code ignore no_ps << EOF_CASE
$case
EOF_CASE
  rm -f "$test_root/build.pid" "$test_root/sleep.pid" "$test_root/build-finished"
  rm -rf "$test_root/proc"
  search_path="$test_root/fake-go:$PATH"
  proc_root=""
  if [ -n "$no_ps" ]; then
    search_path="$test_root/no-ps:$search_path"
    proc_root="$test_root/proc"
  fi
  status=0
  (cd "$repository" && PATH="$search_path" GATE_TEST_DIR="$test_root" GATE_TEST_SIGNAL="$signal" GATE_TEST_IGNORE_TERM="$ignore" \
    CORVINT_TEST_PROC_ROOT="$proc_root" \
    gate_affected > "$test_root/stdout" 2> "$test_root/stderr") || status=$?
  expect_status "$code" "$signal during the build"
  if test -f "$test_root/build-finished"; then fail "$signal during the build waited for the build to finish"; fi
  test -s "$test_root/build.pid" && test -s "$test_root/sleep.pid" || fail "$signal: the build stand-in did not start"
  if grep -qF "could not be built" "$test_root/stderr" || grep -qF "corvint-test" "$test_root/stderr"; then
    fail "$signal during the build went on past the build"
  fi
  # The killed sleeper is reparented and reaped asynchronously; allow it two seconds.
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    if alive "$(cat "$test_root/build.pid")" || alive "$(cat "$test_root/sleep.pid")"; then sleep 0.2; else break; fi
  done
  if alive "$(cat "$test_root/build.pid")"; then fail "$signal: the build outlived the gate"; fi
  if alive "$(cat "$test_root/sleep.pid")"; then fail "$signal: a build descendant outlived the gate"; fi
  if grep -qF "cannot list processes" "$test_root/stderr"; then fail "$signal: the gate found no process table"; fi
done

echo "gate-affected-corvint tests passed"
