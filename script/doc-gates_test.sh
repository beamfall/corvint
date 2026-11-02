#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# ASD-V0-779 (V1-3615): script/doc-gates.sh runs every gate in one call, a failing gate stops no
# other (the conformance gate included), each gate prints as one block, and the run ends naming
# every failed gate with the command that reruns only those.
set -uo pipefail
script=$(cd "$(dirname "$0")" && pwd)/doc-gates.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/doc-gates-test.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT
fail() { printf 'doc-gates_test: FAIL %s\n' "$*" >&2; exit 1; }

repo=$tmp/repo
mkdir -p "$repo" && cd "$repo" || exit 2
git init -q .
printf 'ok:\n\t@echo fine one\n\t@echo fine two\nbad1:\n\t@echo broken one\n\t@exit 1\nbad2:\n\t@echo broken two\n\t@exit 3\n' > Makefile
printf '#!/bin/sh\necho "conformance $*"\nexit 4\n' > "$tmp/go"
chmod +x "$tmp/go"

out=$(DOC_GATES_GO=$tmp/go DOC_GATES_JOBS=3 "$script" bad1 ok bad2 conformance/use-cases-v0 2>&1)
rc=$?
[[ $rc == 1 ]] || fail "a run with failed gates exited $rc, want 1: $out"
for want in 'doc-gates: PASS ok (' 'doc-gates: FAIL bad1 (' 'doc-gates: FAIL bad2 (' 'doc-gates: FAIL conformance/use-cases-v0 (' \
  '  | broken one' '  | broken two' '  | conformance run ./conformance/use-cases-v0' \
  'doc-gates: FAIL 3 of 4 gates: bad1 bad2 conformance/use-cases-v0' \
  "rerun only those: make doc-gates DOC_GATES_RUN='bad1 bad2 conformance/use-cases-v0'"; do
  grep -qF -- "$want" <<< "$out" || fail "missing '$want' in: $out"
done
# Each block is whole: the ok gate's two lines follow its own header.
awk '/^doc-gates: PASS ok /{getline a; getline b; exit !(a == "  | fine one" && b == "  | fine two")}' <<< "$out" || fail "the ok block is split: $out"
timings=$(git rev-parse --git-path doc-gates.timings.tsv)
[[ $(cut -f1,2 "$timings" | tr '\t\n' ' ,') == 'target exit,bad1 2,ok 0,bad2 2,conformance/use-cases-v0 4,' ]] || fail "timings: $(cat "$timings")"

out=$("$script" ok 2>&1) || fail "a passing run failed: $out"
grep -qx 'doc-gates: PASS 1 gates' <<< "$out" || fail "no pass line: $out"
"$script" > /dev/null 2>&1
[[ $? == 2 ]] || fail "no gate: want usage exit 2"
printf 'doc-gates_test: PASS\n'
