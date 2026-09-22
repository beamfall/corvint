#!/bin/sh
set -eu
# Monitor mode only matters for case 10: without it, a POSIX shell auto-ignores SIGINT (and
# SIGQUIT) for any command backgrounded with `&` from a non-job-control shell, so `kill -INT`
# on the backgrounded wrapper below would silently do nothing instead of exercising the trap.
set -m

# Drives the wrapper with a stub `go` on PATH, so the cases exercise the wrapper's own logic:
# its refusals, the environment it hands to both invocations, the exact-line Core dependency
# check, the foreign-cache check with its main-module exemption, and the cleanup trap. No real
# toolchain runs here; the wrapper's real-build evidence is a separate manual run.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
real_mktemp=$(command -v mktemp)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-offline-build-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/bin" "$test_root/repo" "$test_root/tmp"
cp "$source_root/script/check-analyzer-python-offline-build.sh" "$test_root/script/"
log="$test_root/go.log"
cat > "$test_root/bin/go" <<'STUB'
#!/bin/sh
printf 'GOMODCACHE=%s GOCACHE=%s GOPROXY=%s GOSUMDB=%s GOWORK=%s GOFLAGS=%s args=%s\n' \
  "$GOMODCACHE" "$GOCACHE" "$GOPROXY" "$GOSUMDB" "$GOWORK" "$GOFLAGS" "$*" >> "$STUB_LOG"
case $1 in
  build)
    test -z "${STUB_BUILD_FAIL:-}" || exit 1
    if [ -n "${STUB_WAIT:-}" ]; then
      echo $$ > "$STUB_WAIT"
      exec sleep 30
    fi
    ;;
  list)
    test -z "${STUB_LIST_FAIL:-}" || exit 1
    printf 'fmt\n%s\n' "${STUB_DEP:-github.com/Beamfall/corvint/internal/other}"
    for path in ${STUB_CACHE:-}; do
      mkdir -p "$GOMODCACHE/cache/download/$path" && : > "$GOMODCACHE/cache/download/$path/list"
    done ;;
esac
STUB
chmod +x "$test_root/bin/go"

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

run() {
  status=0
  : > "$log"
  output=$(cd "$test_root" && PATH="$test_root/bin:$PATH" TMPDIR="$test_root/tmp" STUB_LOG="$log" \
    script/check-analyzer-python-offline-build.sh "$@" 2>&1) || status=$?
  printf '%s' "$output"
  return $status
}

# 1. A missing root argument and a root carrying vendor/ are both refused before any build.
run >/dev/null 2>&1 && fail "a missing root argument passed"
mkdir "$test_root/repo/vendor"
run repo >/dev/null 2>&1 && fail "a root with vendor/ passed"
test ! -s "$log" || fail "a refused run still invoked go"
rmdir "$test_root/repo/vendor"

# 2. A clean run passes, hands the pinned environment to both invocations, and cleans up.
run repo >/dev/null || fail "a clean stubbed run failed"
env='GOMODCACHE=[^ ]*/corvint-analyzer-python-modcache\.[^ ]* GOCACHE=[^ ]*/corvint-analyzer-python-buildcache\.[^ ]* GOPROXY=off GOSUMDB=off GOWORK=off GOFLAGS=-mod=mod'
grep -q "^$env args=build -o [^ ]*/corvint-analyzer-python-offline\.[^ ]* \./cmd/corvint-analyzer-python\$" "$log" ||
  fail "the build invocation did not carry the pinned environment: $(cat "$log")"
grep -q "^$env args=list -deps \./cmd/corvint\$" "$log" ||
  fail "the list invocation did not carry the pinned environment: $(cat "$log")"
test -z "$(ls -A "$test_root/tmp")" || fail "temporary caches survived a passing run: $(ls "$test_root/tmp")"

# Stub overrides are exported inside a subshell: `VAR=x run` would leak past the function in
# POSIX-mode shells.

# 3. A failing build fails the check, and the trap still removes the allocations.
(export STUB_BUILD_FAIL=1; run repo >/dev/null) && fail "a failing build passed"
test -z "$(ls -A "$test_root/tmp")" || fail "temporary caches survived a failing run"

# 4. An exact Core dependency line on the analyzer package is refused with a message (OACS-V0-002).
status=0
output=$(export STUB_DEP=github.com/Beamfall/corvint/internal/analyzerpython; run repo) || status=$?
test "$status" -ne 0 || fail "a Core dependency on the analyzer package passed"
case $output in
  *'check-analyzer-python-offline-build: Core build depends on github.com/Beamfall/corvint/internal/analyzerpython'*) ;;
  *) fail "the Core dependency refusal carried no message: $output" ;;
esac

# 5. A foreign module-cache download is refused; the main module's own cache entry is exempt.
# shellcheck disable=SC2030
(export STUB_CACHE=example.com/dep/@v; run repo >/dev/null) && fail "a foreign module-cache entry passed"
# shellcheck disable=SC2031
(export STUB_CACHE=github.com/!beamfall/corvint/@v; run repo >/dev/null) ||
  fail "the main module's own cache entry was not exempt"

# 6. A root argument that is not a directory is refused before any build (OACS-V0-001).
run no-such-root >/dev/null 2>&1 && fail "a nonexistent root passed"

# 7. A failing `go list -deps` fails the check, and the trap still removes the allocations
#    (OACS-V0-001).
(export STUB_LIST_FAIL=1; run repo >/dev/null) && fail "a failing list passed"
test -z "$(ls -A "$test_root/tmp")" || fail "temporary caches survived a failing list"

# 8. A failing first cache allocation exits before any go invocation runs, and leaves nothing to
#    clean up (OACS-V0-003 allocation-failure characterization).
cat > "$test_root/bin/mktemp" <<STUB
#!/bin/sh
test -z "\${STUB_MKTEMP_FAIL:-}" || exit 1
exec $real_mktemp "\$@"
STUB
chmod +x "$test_root/bin/mktemp"
(export STUB_MKTEMP_FAIL=1; run repo >/dev/null 2>&1) && fail "a failing module-cache allocation passed"
test -z "$(ls -A "$test_root/tmp")" || fail "a failed allocation left temporary files behind"
test ! -s "$log" || fail "a failed allocation still invoked go"

# 9. TERM during the delegated build still runs the trap and removes all three allocations
#    (OACS-V0-003 supervised-interruption characterization). The wrapper and its blocked go stub
#    are both owned here and signaled directly: a shell defers its own trap until a foreground
#    child exits, so killing only the wrapper would not reproduce the interrupted-build path.
wait_file="$test_root/waiting-stub"
rm -f "$wait_file"
(cd "$test_root" && PATH="$test_root/bin:$PATH" TMPDIR="$test_root/tmp" STUB_LOG="$log" \
    STUB_WAIT="$wait_file" exec script/check-analyzer-python-offline-build.sh repo >/dev/null 2>&1) &
wrapper_pid=$!
attempt=0
while [ ! -s "$wait_file" ] && [ "$attempt" -lt 200 ]; do
  sleep 0.05
  attempt=$((attempt + 1))
done
test -s "$wait_file" || fail "the interrupt fixture did not reach the delegated build"
stub_pid=$(cat "$wait_file")
kill -TERM "$wrapper_pid" 2>/dev/null || true
kill -TERM "$stub_pid" 2>/dev/null || true
wait "$wrapper_pid" 2>/dev/null || true
wrapper_pid=
attempt=0
while kill -0 "$stub_pid" 2>/dev/null && [ "$attempt" -lt 100 ]; do
  sleep 0.05
  attempt=$((attempt + 1))
done
if kill -0 "$stub_pid" 2>/dev/null; then
  fail "the interrupted build stub survived"
fi
stub_pid=
test -z "$(ls -A "$test_root/tmp")" ||
  fail "an interrupted run left temporary caches behind: $(ls "$test_root/tmp")"

# 10. SIGINT during the delegated build still runs the trap and removes all three allocations
#    (OACS-V0-003 supervised-interruption characterization, the INT half of `trap ... EXIT HUP
#    INT TERM`). `set -m` above puts the backgrounded wrapper in its own process group so the
#    signal is not auto-ignored, matching a real terminal's Ctrl-C to the whole foreground group;
#    a plain `kill -INT` at the wrapper's own pid would not, since backgrounded jobs in a
#    non-job-control shell inherit SIGINT already set to be ignored.
gopid_file="$test_root/go-int.pid"
stopfile="$test_root/stop-int"
rm -f "$gopid_file" "$stopfile"
: > "$log"
cat > "$test_root/bin/go" <<STUBEOF
#!/bin/sh
echo \$\$ > "$gopid_file"
i=0
while [ ! -f "$stopfile" ] && [ "\$i" -lt 100 ]; do
  sleep 0.1
  i=\$((i + 1))
done
STUBEOF
chmod +x "$test_root/bin/go"

PATH="$test_root/bin:$PATH" TMPDIR="$test_root/tmp" \
  "$test_root/script/check-analyzer-python-offline-build.sh" "$test_root/repo" >"$log" 2>&1 &
wpid=$!

attempt=0
while [ ! -s "$gopid_file" ] && [ "$attempt" -lt 100 ]; do
  sleep 0.1
  attempt=$((attempt + 1))
done
test -s "$gopid_file" || fail "the SIGINT case: go stub never started"
gopid=$(cat "$gopid_file")

kill -INT -- "-$wpid"

set +e
wait "$wpid"
ws=$?
set -e
test "$ws" -ne 0 || fail "a SIGINT-interrupted run reported success"
kill -0 "$gopid" 2>/dev/null && fail "the SIGINT case: go stub $gopid is still running"
test -z "$(ls -A "$test_root/tmp")" || fail "the SIGINT case left temporary state behind: $(ls "$test_root/tmp")"

# 11. TERM to the wrapper alone while its delegated `go list` still succeeds exits 143 (OACS-V0-003),
#     instead of resuming after cleanup to pass without the removed dependency list.
listpid_file="$test_root/go-list.pid"
rm -f "$listpid_file" "$stopfile"
cat > "$test_root/bin/go" <<STUBEOF
#!/bin/sh
[ "\$1" = list ] || exit 0
echo \$\$ > "$listpid_file"
i=0
while [ ! -f "$stopfile" ] && [ "\$i" -lt 200 ]; do
  sleep 0.05
  i=\$((i + 1))
done
printf 'fmt\ngithub.com/Beamfall/corvint/internal/other\n'
STUBEOF
PATH="$test_root/bin:$PATH" TMPDIR="$test_root/tmp" \
  "$test_root/script/check-analyzer-python-offline-build.sh" "$test_root/repo" >"$log" 2>&1 &
wpid=$!
attempt=0
while [ ! -s "$listpid_file" ] && [ "$attempt" -lt 100 ]; do
  sleep 0.1
  attempt=$((attempt + 1))
done
test -s "$listpid_file" || fail "the signal case: go list stub never started"
kill -TERM "$wpid"
: > "$stopfile"
set +e
wait "$wpid"
ws=$?
set -e
test "$ws" -eq 143 || fail "a wrapper signalled during a passing go list exited $ws, not 143"
test -z "$(ls -A "$test_root/tmp")" || fail "the signal case left temporary state behind: $(ls "$test_root/tmp")"

echo "check-analyzer-python-offline-build_test.sh: ok"
