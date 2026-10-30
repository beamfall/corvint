#!/bin/bash
# GPK-V0-021 self-dogfood output-equality runner.
# usage: run-case.sh <case-id> <stdin-file-or-"-none-"> -- <args...>
# Runs oracle then candidate with identical argv/stdin on the pinned subject repo,
# captures stdout/stderr/rc, and hashes `git status --porcelain` before/between/after.
set -u
OUT=/private/tmp/corvint-integration/conformance/perf-v0/results/corvint-selfdogfood-2026-08-29/raw
SUBJ=/private/tmp/corvint-self
GO=/tmp/corvint-w12
export PYTHONPATH=/private/tmp/corvint-integration/src

CASE="$1"; shift
STDIN="$1"; shift
[ "$1" = "--" ] && shift

st() { ( cd "$SUBJ" && git status --porcelain | shasum -a 256 | awk '{print $1}' ); }

mkdir -p "$OUT"
printf '%s\n' "$*" > "$OUT/$CASE.argv"
S0=$(st)
if [ "$STDIN" = "-none-" ]; then
  ( cd "$SUBJ" && python3 -m corvint_cli "$@" ) >"$OUT/$CASE.oracle.stdout" 2>"$OUT/$CASE.oracle.stderr"; ORC=$?
else
  ( cd "$SUBJ" && python3 -m corvint_cli "$@" <"$STDIN" ) >"$OUT/$CASE.oracle.stdout" 2>"$OUT/$CASE.oracle.stderr"; ORC=$?
fi
S1=$(st)
if [ "$STDIN" = "-none-" ]; then
  ( cd "$SUBJ" && "$GO" "$@" ) >"$OUT/$CASE.cand.stdout" 2>"$OUT/$CASE.cand.stderr"; CRC=$?
else
  ( cd "$SUBJ" && "$GO" "$@" <"$STDIN" ) >"$OUT/$CASE.cand.stdout" 2>"$OUT/$CASE.cand.stderr"; CRC=$?
fi
S2=$(st)

OB=$(wc -c <"$OUT/$CASE.oracle.stdout" | tr -d ' ')
CB=$(wc -c <"$OUT/$CASE.cand.stdout" | tr -d ' ')
if cmp -s "$OUT/$CASE.oracle.stdout" "$OUT/$CASE.cand.stdout"; then EQ=yes; else EQ=no; fi
if [ "$S0" = "$S1" ] && [ "$S1" = "$S2" ]; then STB=yes; else STB=no; fi
printf '%s|%s|%s|%s|%s|%s|%s|%s|%s\n' "$CASE" "$ORC" "$CRC" "$OB" "$CB" "$EQ" "$STB" "$S0" "$S2" \
  | tee -a "$OUT/../summary.psv"
