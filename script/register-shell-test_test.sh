#!/usr/bin/env bash
# ASD-V0-569 (V1-3319): script/register-shell-test.sh registers a shell test in the rule, GATE_STEPS
# and shell-test lists of a fixture Makefile in one run, idempotently, and --hint names an
# unregistered shell test added since BASE with docs/SHELL-TEST-REGISTRATION.md.
set -uo pipefail
script=$(cd "$(dirname "$0")" && pwd)/register-shell-test.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/register-shell-test-test.XXXXXX") || exit 2
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'register-shell-test_test: FAIL %s\n' "$1" >&2; exit 1; }
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
cd "$tmp" || exit 2
git init -q . && git config core.hooksPath /dev/null
mkdir script
printf '#!/bin/sh\n' > script/aaa_test.sh
cat > Makefile <<'EOF'
GATE_STEPS = \
  s1 \
  s2 \
  s3 \
  s4 \
  s5 \
  aaa-test \
  zzz-check \
  # end of GATE_STEPS: add a step above

.PHONY: aaa-test
aaa-test:
	@script/aaa_test.sh

shell-test: \
  aaa-test \
  # end of shell-test prerequisites
EOF
git add . && git commit -q -m base || exit 2
base=$(git rev-parse HEAD)

# 1. --hint is silent while every added shell test is registered.
out=$("$script" --hint "$base" 2>&1) || fail "hint rc"
[[ -z $out ]] || fail "registered tree hinted: $out"

# 2. A new unregistered shell test is hinted with the command and the doc.
printf '#!/bin/sh\n' > script/mmm_test.sh
out=$("$script" --hint "$base" 2>&1) || fail "hint rc"
[[ $out == 'hint: unregistered-shell-test script/mmm_test.sh: run make shell-test-register TEST=script/mmm_test.sh, then see docs/SHELL-TEST-REGISTRATION.md item 3 (unbounded-readers)' ]] || fail "hint: $out"

# 3. One command adds the rule and both sorted list entries.
out=$("$script" script/mmm_test.sh) || fail "register rc: $out"
want_gate=$'  s5 \\\n  aaa-test \\\n  mmm-test \\\n  zzz-check \\'
[[ $(cat Makefile) == *"$want_gate"* ]] || fail "GATE_STEPS order: $(cat Makefile)"
awk '/^shell-test: \\$/ { on = 1; next } on && /# end of/ { exit } on { print }' Makefile | grep -qxF '  mmm-test \' || fail "shell-test entry"
grep -qxF $'\t@script/mmm_test.sh' Makefile && grep -qxF '.PHONY: mmm-test' Makefile || fail "rule"
grep -q '^next: docs/SHELL-TEST-REGISTRATION.md item 3' <<< "$out" || fail "next step: $out"
out=$("$script" --hint "$base" 2>&1)
[[ -z $out ]] || fail "registered test still hinted: $out"

# 4. A second run changes nothing; a GATE_STEPS name sorting before the fixed five stays after them.
cp Makefile Makefile.before
"$script" script/mmm_test.sh > /dev/null || fail "rerun rc"
cmp -s Makefile Makefile.before || fail "rerun changed the Makefile"
printf '#!/bin/sh\n' > script/a0_test.sh
"$script" script/a0_test.sh > /dev/null || fail "a0 rc"
[[ $(cat Makefile) == *$'  s5 \\\n  a0-test \\'* ]] || fail "a0 placed among the fixed five"

# 5. Usage errors.
"$script" script/missing_test.sh > /dev/null 2>&1
[[ $? == 2 ]] || fail "missing file is not a usage error"
"$script" Makefile > /dev/null 2>&1
[[ $? == 2 ]] || fail "non-test path is not a usage error"
printf 'register-shell-test_test: ok\n'
