#!/bin/sh
set -eu

# Covers the three byte ceilings at equality and one byte over, the final-entry guards, and the
# evidence limit: an empty regular file is admitted as the binary because the check reads size
# and nothing else.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-ratchets-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

repo="$test_root/repo"
analyzer="$repo/internal/analyzerpython/analyzer.go"
command="$repo/cmd/corvint-analyzer-python/main.go"
binary="$test_root/analyzer"
mkdir -p "$test_root/script" "$repo/internal/analyzerpython" "$repo/cmd/corvint-analyzer-python"
cp "$source_root/script/check-analyzer-python-ratchets.sh" "$test_root/script/"

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
fill() { head -c "$1" /dev/zero > "$2"; }
run() { (cd "$test_root" && script/check-analyzer-python-ratchets.sh "$@" 2>/dev/null); }

# 1. Both positional arguments are required.
run "$repo" && fail "a missing binary argument passed"

# 2. Every file exactly at its ceiling passes.
fill 65536 "$analyzer"
fill 4096 "$command"
fill 6291456 "$binary"
run "$repo" "$binary" || fail "files at their ceilings failed"

# 3. One byte over any ceiling fails, and restoring it passes again.
for over in "65537 $analyzer 65536" "4097 $command 4096" "6291457 $binary 6291456"; do
  # shellcheck disable=SC2086
  set -- $over
  fill "$1" "$2"
  run "$repo" "$binary" && fail "$2 at $1 bytes passed its ceiling"
  fill "$3" "$2"
done
run "$repo" "$binary" || fail "restored ceilings failed"

# 4. A symlinked or missing final entry is refused, even when its target is within the ceiling.
mv "$binary" "$binary.real"
ln -s "$binary.real" "$binary"
run "$repo" "$binary" && fail "a symlinked binary passed"
rm "$binary"
run "$repo" "$binary" && fail "a missing binary passed"
mv "$binary.real" "$binary"
mv "$command" "$command.away"
run "$repo" "$binary" && fail "a missing command source passed"
mv "$command.away" "$command"

# 5. An empty regular binary passes: the check measures bytes, not content or provenance.
: > "$binary"
run "$repo" "$binary" || fail "an empty regular binary failed"

# 6. A non-regular, non-symlink final entry (a directory) is refused (OACS-V0-004).
rm "$binary"
mkdir "$binary"
run "$repo" "$binary" && fail "a directory in place of the binary passed"
rmdir "$binary"
: > "$binary"

echo "check-analyzer-python-ratchets_test.sh: ok"
