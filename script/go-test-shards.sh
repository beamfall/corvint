#!/bin/sh
set -eu

# Usage: script/go-test-shards.sh SHARD COUNT <packages
#
# Prints the Go packages of shard SHARD (0-based) of COUNT, one per line, from the package list
# on standard input (`go list ./...`). The `go-product-shard` CI job (decision 0418) runs
# `go test` over this list. Every input package lands in exactly one shard, and the output is
# the same for the same input, weights and COUNT.
#
# Balance: greedy longest-first over script/go-test-shard-weights.tsv (package TAB seconds,
# from a `go test -json` run on main). Packages go in descending weight, ties by name, each to
# the least-loaded shard, ties to the lowest index. A package missing from the weights weighs
# 1 second, and a weight for a package not on the input is ignored, so stale weights can only
# unbalance the shards, never drop or add a package.

fail() { printf 'go-test-shards: %s\n' "$1" >&2; exit 2; }

test $# -eq 2 || fail 'usage: go-test-shards.sh SHARD COUNT <packages'
shard=$1
count=$2
case $shard in '' | *[!0-9]*) fail "SHARD must be a non-negative integer: $shard" ;; esac
case $count in '' | *[!0-9]*) fail "COUNT must be a positive integer: $count" ;; esac
test "$count" -gt 0 || fail "COUNT must be a positive integer: $count"
test "$shard" -lt "$count" || fail "SHARD $shard is not below COUNT $count"

weights=${CORVINT_GO_TEST_SHARD_WEIGHTS:-$(CDPATH='' cd -- "$(dirname "$0")" && pwd)/go-test-shard-weights.tsv}
test -r "$weights" || fail "weights file is not readable: $weights"

# Temporary files, not one pipeline: plain `sh` has no `pipefail`, so a failed stage would
# otherwise pass an empty or partial list on as a successful one.
work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-go-test-shards.XXXXXX")
trap 'rm -rf "$work"' EXIT

awk -F '\t' 'NR == FNR { weight[$1] = $2; next }
    $0 != "" { print (($0 in weight) ? weight[$0] : 1) "\t" $0 }' "$weights" - >"$work/weighted"
test -s "$work/weighted" || fail 'no packages on standard input'
LC_ALL=C sort -t "$(printf '\t')" -k1,1nr -k2,2 "$work/weighted" >"$work/ordered"

awk -F '\t' -v shard="$shard" -v count="$count" '
{
    best = 0
    for (s = 1; s < count; s++) {
        if (load[s] < load[best]) best = s
    }
    load[best] += $1
    packages[best]++
    if (best == shard) print $2
}
END {
    printf "go-test-shards: shard %d of %d: %d packages, predicted %d s\n",
        shard, count, packages[shard], load[shard] > "/dev/stderr"
}' "$work/ordered"
