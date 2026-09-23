#!/bin/sh
set -eu

# Drives script/check-install-lifecycle.sh against two builds of this checkout stamped as build 1
# and build 2: the binary path with a real upgrade, the archive path with the release layout,
# a tampered archive refused at the checksum step before anything runs, the usage refusal, and
# the report file. Every run happens in the wrapper's own temporary directory; nothing here
# writes into the checkout.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-install-lifecycle-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
test_root=$(cd "$test_root" && pwd -P)

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
script="$source_root/script/check-install-lifecycle.sh"

(cd "$source_root" && GOTOOLCHAIN=local go build -trimpath -ldflags "-X main.build=1" -o "$test_root/corvint-1" ./cmd/corvint) || fail "build 1 failed"
(cd "$source_root" && GOTOOLCHAIN=local go build -trimpath -ldflags "-X main.build=2" -o "$test_root/corvint-2" ./cmd/corvint) || fail "build 2 failed"

digest() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

# 1. Binary path with a genuine upgrade: every step passes and the upgrade is not same-bytes.
output=$(CORVINT_LIFECYCLE_BINARY="$test_root/corvint-1" CORVINT_LIFECYCLE_UPGRADE_BINARY="$test_root/corvint-2" \
  CORVINT_LIFECYCLE_REPORT="$test_root/report.txt" "$script") || fail "binary lifecycle failed: $output"
for step in install-a first-index upgrade-b rollback-a uninstall backup-restore corrupt-truncate corrupt-overwrite; do
  case $output in
    *"step $step: ok"*) ;;
    *) fail "step $step did not pass: $output" ;;
  esac
done
case $output in
  *"step upgrade-b: ok version=Corvint "*"(build 2)"*) ;;
  *) fail "the upgrade did not run build 2: $output" ;;
esac
case $output in
  *same-bytes*) fail "a genuine upgrade was reported as same-bytes" ;;
esac
case $output in
  *"SUMMARY status=PASS version_a=Corvint_"*"_(build_1) version_b=Corvint_"*"_(build_2)"*) ;;
  *) fail "summary line missing or wrong: $output" ;;
esac
test "$(tail -n 1 "$test_root/report.txt")" = "$(printf '%s\n' "$output" | tail -n 1)" || fail "report file does not end with the summary"

# 2. Archive path: the release layout `<root>/corvint` + `<root>/SHA256SUMS` in one tar.gz.
mkdir -p "$test_root/assembly/corvint_test_host"
cp "$test_root/corvint-1" "$test_root/assembly/corvint_test_host/corvint"
printf '%s  corvint\n' "$(digest "$test_root/corvint-1")" > "$test_root/assembly/corvint_test_host/SHA256SUMS"
(cd "$test_root/assembly" && tar -czf "$test_root/corvint_test_host.tar.gz" corvint_test_host)
output=$(CORVINT_LIFECYCLE_ARCHIVE="$test_root/corvint_test_host.tar.gz" "$script") || fail "archive lifecycle failed: $output"
case $output in
  *"archive $test_root/corvint_test_host.tar.gz"*"step upgrade-b: ok version=Corvint "*"(build 1) same-bytes"*"SUMMARY status=PASS"*) ;;
  *) fail "archive run did not report the archive, the same-bytes upgrade and PASS: $output" ;;
esac

# 3. A tampered archive fails at install-a, before any index or read runs.
printf '%s  corvint\n' "$(digest "$test_root/corvint-2")" > "$test_root/assembly/corvint_test_host/SHA256SUMS"
(cd "$test_root/assembly" && tar -czf "$test_root/tampered.tar.gz" corvint_test_host)
status=0
output=$(CORVINT_LIFECYCLE_ARCHIVE="$test_root/tampered.tar.gz" "$script" 2>&1) || status=$?
test "$status" -eq 1 || fail "a tampered archive exited $status, not 1"
case $output in
  *"step install-a: FAIL"*"SUMMARY status=FAIL step=install-a"*) ;;
  *) fail "a tampered archive was not refused at install-a: $output" ;;
esac
case $output in
  *"step first-index"*) fail "a tampered archive still reached first-index" ;;
esac

# 4. No input and an extra argument are usage errors (exit 2).
status=0
env -u CORVINT_LIFECYCLE_ARCHIVE -u CORVINT_LIFECYCLE_BINARY "$script" >/dev/null 2>&1 || status=$?
test "$status" -eq 2 || fail "no input exited $status, not 2"
status=0
CORVINT_LIFECYCLE_BINARY="$test_root/corvint-1" "$script" extra >/dev/null 2>&1 || status=$?
test "$status" -eq 2 || fail "an extra argument exited $status, not 2"

echo "check-install-lifecycle_test.sh: ok"
