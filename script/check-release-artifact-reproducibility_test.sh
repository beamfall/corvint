#!/bin/sh
set -eu

# Drives the wrapper with a stub `go` on PATH: the output-location refusal, the exact delegated
# command line and toolchain pin, exit-status and stdout propagation, and the caller-CWD limit.
# The default `/tmp/corvint-release-artifact` output is never selected here, because running it
# would create that directory on the host outside this fixture.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-release-wrapper-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

# The wrapper derives its root with `cd && pwd`, so the fixture root must be spelled the same
# way for the lexical containment cases to compare like for like.
test_root=$(cd "$test_root" && pwd)
root="$test_root/repo"
mkdir -p "$root/script" "$test_root/bin" "$test_root/elsewhere"
cp "$source_root/script/check-release-artifact-reproducibility.sh" "$root/script/"
log="$test_root/go.log"
cat > "$test_root/bin/go" <<'STUB'
#!/bin/sh
printf 'GOTOOLCHAIN=%s cwd=%s args=%s\n' "${GOTOOLCHAIN:-unset}" "$(pwd)" "$*" > "$STUB_LOG"
echo stub-stdout
exit "${STUB_EXIT:-0}"
STUB
chmod +x "$test_root/bin/go"

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

run() {
  status=0
  rm -f "$log"
  output=$(cd "${RUN_FROM:-$root}" && PATH="$test_root/bin:$PATH" STUB_LOG="$log" \
    CORVINT_RELEASE_ARTIFACT_OUTPUT="$1" "$root/script/check-release-artifact-reproducibility.sh" 2>&1) || status=$?
  printf '%s' "$output"
  return $status
}

# 1. An output equal to the root, or below it, is refused with exit 2 before go runs.
for output in "$root" "$root/out"; do
  status=0
  message=$(run "$output") || status=$?
  test "$status" -eq 2 || fail "output $output exited $status, not 2"
  case $message in
    *"output must be outside $root"*) ;;
    *) fail "the refusal did not name the root: $message" ;;
  esac
  test ! -e "$log" || fail "a refused run still invoked go"
done

# 2. A sibling path sharing the root's prefix is not refused: the check is lexical on `root/`.
run "${root}2" >/dev/null || fail "a sibling path sharing the root prefix was refused"

# 3. An outside output is created, and go receives the pinned toolchain and exact arguments.
out="$test_root/out"
output=$(run "$out") || fail "an outside output failed"
test -d "$out" || fail "the output directory was not created"
test "$output" = "stub-stdout" || fail "delegated stdout was not preserved: $output"
expected="GOTOOLCHAIN=local cwd=$root args=run ./conformance/release-artifact-v0 --root $root --manifest $root/conformance/release-artifact-v0/manifest.json --output $out --report $out/report.json"
test "$(cat "$log")" = "$expected" || fail "delegated invocation differed: $(cat "$log")"

# 4. The delegated exit status is the wrapper's exit status. (Overrides are exported inside a
#    subshell: `VAR=x run` would leak past the function in POSIX-mode shells.)
status=0
(export STUB_EXIT=1; run "$out" >/dev/null) || status=$?
test "$status" -eq 1 || fail "a delegated exit 1 surfaced as $status"

# 5. The wrapper does not change directory: go runs in the caller's CWD, so the relative
#    package path resolves there and not at the root.
(export RUN_FROM="$test_root/elsewhere"; run "$out" >/dev/null) || fail "a run from elsewhere failed"
case $(cat "$log") in
  *"cwd=$test_root/elsewhere "*) ;;
  *) fail "the delegated command did not run in the caller's directory: $(cat "$log")" ;;
esac

# 6. A regular file already at the output path fails `mkdir -p`, and go never runs (OACS-V0-006).
blocked="$test_root/blocked"
: > "$blocked"
status=0
run "$blocked" >/dev/null 2>&1 || status=$?
test "$status" -ne 0 || fail "an output path blocked by a file passed"
test ! -e "$log" || fail "a failing mkdir still invoked go"

echo "check-release-artifact-reproducibility_test.sh: ok"
