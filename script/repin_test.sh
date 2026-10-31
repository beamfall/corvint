#!/bin/sh
set -eu

# DCG-V0-801: `make repin` passes origin/main to script/repin.sh unless BASE is given on the
# command line or in the environment; the Makefile's own `BASE ?= main` default never reaches it.
root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
fail() { printf 'repin_test: FAIL %s\n' "$1" >&2; exit 1; }
plan() { (cd "$root" && env -u BASE make -n --no-print-directory "$@" repin); }

out=$(plan) || fail "make -n repin failed: $out"
[ "$out" = 'script/repin.sh --base "origin/main"' ] || fail "default base: $out"
out=$(plan BASE=0123abc) || fail "make -n repin BASE= failed: $out"
[ "$out" = 'script/repin.sh --base "0123abc"' ] || fail "command-line base: $out"
out=$(cd "$root" && BASE=feedbeef make -n --no-print-directory repin) || fail "environment base failed: $out"
[ "$out" = 'script/repin.sh --base "feedbeef"' ] || fail "environment base: $out"
printf 'repin_test: ok\n'
