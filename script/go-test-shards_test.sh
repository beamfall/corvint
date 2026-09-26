#!/bin/sh
set -eu

# Four cases on fixture weights. 1: the greedy longest-first assignment is the expected one.
# 2: every input package lands in exactly one shard, including one with no weight, and a weight
# for a package not on the input adds nothing. 3: the input order does not change the output.
# 4: bad arguments, empty input and a missing weights file are refused.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-go-test-shards-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

shards="$source_root/script/go-test-shards.sh"

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

printf 'ex/a\t10\nex/b\t6\nex/c\t5\nex/d\t1\nex/gone\t99\n' >"$test_root/weights.tsv"
printf 'ex/a\nex/b\nex/c\nex/d\nex/new\n' >"$test_root/packages"
CORVINT_GO_TEST_SHARD_WEIGHTS="$test_root/weights.tsv"
export CORVINT_GO_TEST_SHARD_WEIGHTS

# 1. Order a10 b6 c5 d1 new1: a->0 (10), b->1 (6), c->1 (11), d->0 (11), new->0 (12, tie to 0).
"$shards" 0 2 <"$test_root/packages" >"$test_root/s0" 2>/dev/null
"$shards" 1 2 <"$test_root/packages" >"$test_root/s1" 2>/dev/null
test "$(cat "$test_root/s0")" = "$(printf 'ex/a\nex/d\nex/new')" || fail "shard 0: $(cat "$test_root/s0")"
test "$(cat "$test_root/s1")" = "$(printf 'ex/b\nex/c')" || fail "shard 1: $(cat "$test_root/s1")"

# 2. The shards partition the input exactly, for more shards than packages too.
for count in 1 2 3 7; do
    : >"$test_root/union"
    shard=0
    while [ "$shard" -lt "$count" ]; do
        "$shards" "$shard" "$count" <"$test_root/packages" >>"$test_root/union" 2>/dev/null
        shard=$((shard + 1))
    done
    LC_ALL=C sort "$test_root/union" >"$test_root/union.sorted"
    LC_ALL=C sort "$test_root/packages" >"$test_root/packages.sorted"
    cmp -s "$test_root/union.sorted" "$test_root/packages.sorted" ||
        fail "$count shards do not partition the input: $(tr '\n' ' ' <"$test_root/union.sorted")"
done

# 3. Reversed input order gives the same shard.
awk '{ line[NR] = $0 } END { for (i = NR; i > 0; i--) print line[i] }' "$test_root/packages" |
    "$shards" 0 2 >"$test_root/r0" 2>/dev/null
cmp -s "$test_root/s0" "$test_root/r0" || fail "input order changed shard 0: $(cat "$test_root/r0")"

# 4. Refusals exit nonzero and print no package.
refused() {
    status=0
    output=$("$@" <"$test_root/input" 2>/dev/null) || status=$?
    test "$status" -ne 0 || fail "accepted: $*"
    test -z "$output" || fail "printed packages on refusal: $*: $output"
}
cp "$test_root/packages" "$test_root/input"
refused "$shards" 2 2
refused "$shards" 0 0
refused "$shards" x 2
refused "$shards" 0
refused env CORVINT_GO_TEST_SHARD_WEIGHTS="$test_root/missing.tsv" "$shards" 0 2
: >"$test_root/input"
refused "$shards" 0 1

printf 'go-test-shards_test: 4 cases passed\n'
