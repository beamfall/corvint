#!/bin/sh
set -eu

# Two cases, on a fixture repository: a clean tracked docs/decisions set passes, and a
# `git ls-files` failure is refused loudly instead of being swallowed by the pipe and
# reported as zero collisions found (plain `sh` has no `pipefail`, so a pipeline's exit
# status is its last command's, not the enumeration's).

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-decision-numbers-test.XXXXXX")
fakebin=$(mktemp -d "${TMPDIR:-/tmp}/corvint-decision-numbers-fakebin.XXXXXX")
cleanup() { rm -rf "$test_root" "$fakebin"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/docs/decisions"
cp "$source_root/script/check-decision-numbers.sh" "$test_root/script/"

git -C "$test_root" init -q
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

run() {
    status=0
    output=$(cd "$test_root" && script/check-decision-numbers.sh 2>&1) || status=$?
    printf '%s' "$output"
    return $status
}

# 1. A clean tracked set with no collisions passes and says nothing.
printf '# one\n' > "$test_root/docs/decisions/0001-first-2026-09-01.md"
printf '# two\n' > "$test_root/docs/decisions/0002-second-2026-09-01.md"
git -C "$test_root" add -A
git -C "$test_root" commit -qm fixture
output=$(run) || fail "a clean tracked set did not pass: $output"
test -z "$output" || fail "a passing run printed output: $output"

# 2. A `git ls-files` failure is refused loudly, never reported as a passing check.
cat > "$fakebin/git" <<'EOF'
#!/bin/sh
if [ "$1" = "ls-files" ]; then
    echo "fatal: fake git ls-files failure" >&2
    exit 128
fi
exec /usr/bin/git "$@"
EOF
chmod +x "$fakebin/git"

status=0
output=$(cd "$test_root" && PATH="$fakebin:$PATH" script/check-decision-numbers.sh 2>&1) || status=$?
test "$status" -ne 0 || fail "a git ls-files failure passed the gate: $output"
case $output in
    *ls-files*) ;;
    *) fail "the failure did not name git ls-files: $output" ;;
esac

printf 'check-decision-numbers_test: 2 cases passed\n'
