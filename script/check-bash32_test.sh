#!/usr/bin/env bash
# Temp-fixture tests for script/check-bash32.sh (ASD-V0-750, V1-3812). The fixtures spell each
# Bash 4+ construct from split fragments, so this file itself passes the gate it tests.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/check-bash32-test.XXXXXX")
trap '/bin/rm -rf "$tmp"' EXIT INT TERM
fail() { printf 'check-bash32_test: FAIL %s\n' "$*" >&2; exit 1; }
expect() { grep -qF -- "$1" "$2" || { cat "$2" >&2; fail "missing: $1"; }; }
absent() { ! grep -qF -- "$1" "$2" || { cat "$2" >&2; fail "unexpected: $1"; }; }

repo="$tmp/repo"
mkdir -p "$repo/script"
cp "$here/check-bash32.sh" "$repo/script/check-bash32.sh"
git -C "$repo" init -q
# run_gate prints the gate's output to $tmp/out and returns its exit status.
run_gate() {
  git -C "$repo" add -A
  set +e
  (cd "$repo" && script/check-bash32.sh) > "$tmp/out" 2>&1
  local rc=$?
  set -e
  return "$rc"
}

# 1. A clean tree passes; the gate does not count itself.
run_gate || { cat "$tmp/out" >&2; fail "clean tree refused"; }
expect 'check-bash32: ok, 0 scripts' "$tmp/out"

# 2. declare -A is refused naming file:line (the ticket's fixture).
{
  echo '#!/usr/bin/env bash'
  echo 'set -eu'
  printf '%s\n' "dec""lare -A seen=()"
} > "$repo/bad.sh"
if run_gate; then fail "associative array passed"; fi
expect 'check-bash32: FAIL associative-array bad.sh:3: Bash 4+ construct; macOS /bin/bash is 3.2' "$tmp/out"
expect '1 Bash 4+ construct(s) in 1 scripts' "$tmp/out"
/bin/rm -f "$repo/bad.sh"

# 3. Each other rule refuses its construct; the label names the rule.
d='$'
cases="map""file	map""file -t lines < f
case-modification	echo ${d}{name,,}
case-modification	echo ${d}{name^}
parameter-transformation	echo ${d}{name@Q}
negative-array-index	echo ${d}{list[-1]}
nameref	local -""n ref=x
test-variable-set	[[ -""v name ]]
globstar	shopt -s glob""star
co""proc	co""proc cat
pipe-stderr	make |""& tee log
append-both	make &"">> log
case-fallthrough	a) echo a ;;""&
automatic-fd	exec {f""d}> log
wait-next	wa""it -n
brace-step	echo {1..9..""2}"
while IFS=$'\t' read -r label text; do
  printf '#!/bin/bash\n%s\n' "$text" > "$repo/case.sh"
  if run_gate; then fail "$label passed: $text"; fi
  expect "check-bash32: FAIL $label case.sh:2:" "$tmp/out"
done <<< "$cases"
/bin/rm -f "$repo/case.sh"

# 4. A file without .sh whose first line names bash is scanned; a non-bash script is not.
printf '#!/usr/bin/env bash\n%s\n' "map""file x" > "$repo/script/tool"
printf '#!/usr/bin/env python3\n%s\n' "map""file = 1" > "$repo/script/tool.py"
if run_gate; then fail "bash shebang file not scanned"; fi
expect "FAIL map""file script/tool:2:" "$tmp/out"
absent 'tool.py' "$tmp/out"
/bin/rm -f "$repo/script/tool" "$repo/script/tool.py"

# 5. Comment lines, quoted operators and the allow marker pass; Bash 3.2 forms pass.
{
  echo '#!/usr/bin/env bash'
  printf '%s\n' "  # dec""lare -A is Bash 4"
  printf '%s\n' "echo 'a |""& b' \"c &"">> d\""
  printf '%s\n' "dec""lare -A fixture=() # bash32: allow"
  printf '%s\n' 'set -- "${a[@]+"${a[@]}"}"' 'x=$(echo "$y" | tr "[:upper:]" "[:lower:]")' 'echo {1..9}'
} > "$repo/ok.sh"
run_gate || { cat "$tmp/out" >&2; fail "Bash 3.2 forms refused"; }
expect 'check-bash32: ok, 1 scripts' "$tmp/out"

# 6. The gate is a doc gate, so make doc-gates and batch-precheck run it (ASD-V0-750).
grep -qE '^DOC_GATE_TARGETS := (.* )?bash32-check( |$)' "$root/Makefile" || fail "bash32-check is not in DOC_GATE_TARGETS"
grep -qE '^GATE_STEPS = (.* )?bash32-check( |$)' "$root/Makefile" || fail "bash32-check is not in GATE_STEPS"

echo 'check-bash32_test: PASS'
