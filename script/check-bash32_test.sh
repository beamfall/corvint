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
test-variable-set	[[ -n ${d}x && -""v y ]]
case-modification	echo "${d}{name,,}"
globstar	shopt -s glob""star
co""proc	co""proc cat
pipe-stderr	make |""& tee log
append-both	make &"">> log
case-fallthrough	a) echo a ;;""&
automatic-fd	exec {f""d}> log
automatic-fd	: {f""d}>log
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

# 6. Quoted text and comments are not code: no rule fires on a word in a string or a comment.
{
  echo '#!/usr/bin/env bash'
  printf '%s\n' "echo \"map""file is unavailable\"" "echo x # map""file is Bash 4" "echo 'wa""it -n' \"co""proc\""
  printf '%s\n' "msg='a" "dec""lare -A in a multi-line string'"
  printf '%s\n' '[[ $(command -''v git) == x ]] || echo none'
} > "$repo/ok.sh"
run_gate || { cat "$tmp/out" >&2; fail "quoted text or a comment refused"; }
expect 'check-bash32: ok, 1 scripts' "$tmp/out"

# 7. An apostrophe in here-document prose does not hide the code after the terminator.
{
  echo '#!/usr/bin/env bash'
  echo 'cat <<EOF'
  echo "don't stop"
  echo 'EOF'
  printf '%s\n' "map""file -t lines < f"
} > "$repo/ok.sh"
if run_gate; then fail "here-document apostrophe hid a construct"; fi
expect "check-bash32: FAIL map""file ok.sh:5:" "$tmp/out"

# 8. An incomplete scan refuses and never prints ok: an unreadable script exits 2.
printf '#!/bin/bash\necho ok\n' > "$repo/ok.sh"
git -C "$repo" add -A
chmod 000 "$repo/ok.sh"
if [[ -r $repo/ok.sh ]]; then
  echo 'check-bash32_test: SKIP unreadable-script case (running as a user who can read mode 000)'
else
  set +e
  (cd "$repo" && script/check-bash32.sh) > "$tmp/out" 2>&1
  rc=$?
  set -e
  [[ $rc == 2 ]] || { cat "$tmp/out" >&2; fail "unreadable script exited $rc, want 2"; }
  expect 'check-bash32: FAIL incomplete scan' "$tmp/out"
  absent 'check-bash32: ok' "$tmp/out"
fi
chmod 644 "$repo/ok.sh"

# 9. The gate needs no temporary file: with TMPDIR unusable it still refuses a construct (the
# Bash 3.2 here-string failure that once skipped every rule and printed ok).
printf '#!/bin/bash\n%s\n' "map""file x" > "$repo/ok.sh"
git -C "$repo" add -A
set +e
(cd "$repo" && TMPDIR="$tmp/missing" script/check-bash32.sh) > "$tmp/out" 2>&1
rc=$?
set -e
[[ $rc == 1 ]] || { cat "$tmp/out" >&2; fail "TMPDIR unusable exited $rc, want 1"; }
expect "check-bash32: FAIL map""file ok.sh:2:" "$tmp/out"
/bin/rm -f "$repo/ok.sh"

# 11. A scan whose inputs match the last passing scan is reused with a reason line; a changed
# script, a new bash-shebang script and a changed checker each force a full scan, and
# CORVINT_GATE_LEDGER=off always rescans (ASD-V0-601, V1-4100).
printf '#!/bin/bash\necho ok\n' > "$repo/ok.sh"
run_gate || { cat "$tmp/out" >&2; fail "clean script refused"; }
absent '(reused:' "$tmp/out"
run_gate || { cat "$tmp/out" >&2; fail "unchanged rerun refused"; }
expect 'check-bash32: ok, 1 scripts (reused: ' "$tmp/out"
printf '#!/bin/bash\n%s\n' "map""file x" > "$repo/ok.sh"
if run_gate; then fail "a changed script reused the last pass"; fi
expect "check-bash32: FAIL map""file ok.sh:2:" "$tmp/out"
printf '#!/bin/bash\necho ok\n' > "$repo/ok.sh"
run_gate || { cat "$tmp/out" >&2; fail "restored script refused"; }
printf '#!/usr/bin/env bash\n%s\n' "map""file y" > "$repo/script/tool2"
if run_gate; then fail "a new bash-shebang script reused the last pass"; fi
expect "check-bash32: FAIL map""file script/tool2:2:" "$tmp/out"
/bin/rm -f "$repo/script/tool2"
run_gate || { cat "$tmp/out" >&2; fail "tree without tool2 refused"; }
run_gate || { cat "$tmp/out" >&2; fail "unchanged rerun refused"; }
expect '(reused: ' "$tmp/out"
echo '# a checker change' >> "$repo/script/check-bash32.sh"
run_gate || { cat "$tmp/out" >&2; fail "changed checker refused"; }
absent '(reused:' "$tmp/out"
expect 'check-bash32: ok, 1 scripts' "$tmp/out"
git -C "$repo" add -A
set +e
(cd "$repo" && CORVINT_GATE_LEDGER=off script/check-bash32.sh) > "$tmp/out" 2>&1
rc=$?
set -e
[[ $rc == 0 ]] || { cat "$tmp/out" >&2; fail "CORVINT_GATE_LEDGER=off exited $rc"; }
absent '(reused:' "$tmp/out"
/bin/rm -f "$repo/ok.sh"

# 10. The gate is a doc gate, so make doc-gates and batch-precheck run it (ASD-V0-750).
grep -qE '^DOC_GATE_TARGETS := (.* )?bash32-check( |$)' "$root/Makefile" || fail "bash32-check is not in DOC_GATE_TARGETS"
# GATE_STEPS lists one step per line (STR-V0-920), so read the whole assignment.
awk '/^GATE_STEPS = /,/end of GATE_STEPS/' "$root/Makefile" | grep -qE '(^|[[:space:]])bash32-check([[:space:]]|$)' || fail "bash32-check is not in GATE_STEPS"

echo 'check-bash32_test: PASS'
