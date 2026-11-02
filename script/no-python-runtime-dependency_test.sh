#!/bin/sh
# GOC-V0-008 falsifying assertion: Corvint's own build, tests, hooks, packaging and default
# execution paths (Makefile, script/*.sh, integrations/**/*.mjs and integrations/**/*.json)
# must carry no literal Python-interpreter invocation. Three things are never flagged: the
# CORVINT_TEST_EXTERNAL_PYTEST opt-in, which lets an explicitly requested command run against a
# user's own Python project, the `corvint-analyzer-python` Go tool family (its name embeds
# "python" but it is Go source that analyzes Python as fixture data, not a Python interpreter
# dependency of Corvint's own build/tests/hooks/packaging), and this check's own name, which the
# Makefile's gate step must spell. One file is exempt by exact repository path: the named
# external-conformance script script/spec-toolchain-conformance.sh (STA-V0-011), which installs
# third-party Spec Kit into a throwaway venv to compare upstream's view with Corvint's. It is not
# Corvint's build/test/hook/packaging path, so the check also fails if any gate step or other
# scanned path reaches it (GOC-V0-011).
set -eu

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
pattern='\<python3?\>|\<pip\>|\<pytest\>'
opt_in='CORVINT_TEST_EXTERNAL_PYTEST'
names='analyzer-python|no-python-runtime-dependency'
external_conformance='script/spec-toolchain-conformance.sh'

# unexempted keeps each matched line that still names an interpreter after the opt-in lines are
# dropped and the two allowed names are masked, so an allowed name elsewhere on a line or in its
# path never hides a real invocation. Paths are repository-relative so the checkout path cannot
# match or exempt anything.
unexempted() {
  grep -v "$opt_in" | sed -E "s/$names/[allowed-name]/g" | grep -E "$pattern"
}

# mask_fixture_data blanks the quoted arguments of a printf or echo command (V1-3971): text a shell
# test writes into a fixture is data, not an invocation. It reads the shell words of each line: a
# printf or echo in command position (line start, after | ; & ( : or a reserved word such as then)
# has its single-quoted arguments and its double-quoted ones without $( or a backquote replaced by
# empty quotes, up to the next unquoted | ; & or ). An unquoted name, a later command on the line and
# a command substitution inside double quotes are left as they are, so a real call still matches.
mask_fixture_data() {
  awk '
    function endword() {
      if (word == "") return
      if (cmdpos && (word == "printf" || word == "echo")) args = 1
      cmdpos = word ~ /^(then|do|else|elif|if|while|until|!|time)$/
      word = ""
    }
    {
      line = $0; n = length(line); out = ""; word = ""; cmdpos = 1; args = 0
      for (i = 1; i <= n; i++) {
        c = substr(line, i, 1)
        if (c == "\\") { out = out substr(line, i, 2); word = word "x"; i++; continue }
        if (c == "\047") {
          j = index(substr(line, i + 1), "\047")
          if (j == 0) { out = out substr(line, i); break }
          out = out (args ? "\047\047" : substr(line, i, j + 1)); i += j; word = word "x"; continue
        }
        if (c == "\"") {
          for (j = i + 1; j <= n && substr(line, j, 1) != "\""; j++) if (substr(line, j, 1) == "\\") j++
          s = substr(line, i, j - i + 1)
          out = out ((args && s !~ /\$\(|`/) ? "\"\"" : s); i = j; word = word "x"; continue
        }
        if (c ~ /[ \t|;&():]/) {
          endword()
          if (c ~ /[|;&(:]/) { cmdpos = 1; args = 0 } else if (c == ")") args = 0
        } else word = word c
        out = out c
      }
      print out
    }'
}

# scan reads grep -Hn lines (path:line:text) and drops only those whose path field is exactly the
# external-conformance script, so a renamed copy, a longer path or a line elsewhere naming it is
# still scanned. Quoted printf and echo fixture data is masked first (V1-3971).
scan() {
  awk -v p="$external_conformance" 'index($0, p ":") != 1 || substr($0, length(p) + 2) !~ /^[0-9]+:/' | mask_fixture_data | unexempted
}

# reaches_external reads Makefile text and names each way a gate step or another target reaches the
# exempt script: it may only be the recipe of its own target, which nothing lists as a step or
# prerequisite.
reaches_external() {
  awk -v p="${external_conformance##*/}" -v t=spec-toolchain-conformance '
    # make logical lines: GATE_STEPS and prerequisite lists continue one entry per line (STR-V0-920).
    { joined = joined $0 }
    /\\$/ { sub(/\\$/, " ", joined); next }
    { $0 = joined; joined = "" }
    /^GATE_STEPS *=/ { for (i = 3; i <= NF; i++) if ($i == t) print "GATE_STEPS lists " t; next }
    /^[^\t#][^:=]*:/ {
      target = substr($0, 1, index($0, ":") - 1); n = split(substr($0, index($0, ":") + 1), deps, /[ \t|]+/)
      for (i = 1; i <= n; i++) if (deps[i] == t && target != ".PHONY") print target " depends on " t
    }
    /^\t/ && index($0, p) && target != t { print "recipe of " target " runs " p }
  '
}

cd "$source_root"
targets=$(
  printf '%s\n' Makefile
  find script -maxdepth 1 -type f -name '*.sh' ! -name "$(basename "$0")"
  find integrations -type f \( -name '*.mjs' -o -name '*.json' \) 2>/dev/null
)

violations=$(printf '%s\n' "$targets" | xargs grep -HnE "$pattern" 2>/dev/null | scan || true)
via=$(
  reaches_external < Makefile
  printf '%s\n' "$targets" | grep -vx -e Makefile -e "$external_conformance" |
    xargs grep -Hn "${external_conformance##*/}" 2>/dev/null | grep -v '^[^:]*:[0-9]*: *#' || true
)
if [ -n "$via" ]; then
  printf 'GOC-V0-011: the external-conformance exemption is reachable from a scanned path:\n%s\n' "$via" >&2
  exit 1
fi
if [ -n "$violations" ]; then
  printf 'GOC-V0-008: a build/test/hook/packaging path names a Python interpreter:\n%s\n' "$violations" >&2
  exit 1
fi

# Falsifying self-test: prove the scan genuinely fires on a real invocation and genuinely
# allows every carved-out exception, so this is not a vacuous always-pass grep.
fixture=$(mktemp "${TMPDIR:-/tmp}/corvint-no-python-dep.XXXXXX")
trap 'rm -f "$fixture"' EXIT

printf 'python3 build.py\n' > "$fixture"
if ! grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: scan failed to detect a genuine python3 invocation" >&2
  exit 1
fi

# shellcheck disable=SC2016
printf 'if [ -n "$CORVINT_TEST_EXTERNAL_PYTEST" ]; then python3 -m pytest; fi\n' > "$fixture"
if grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: the CORVINT_TEST_EXTERNAL_PYTEST opt-in path was not allowed" >&2
  exit 1
fi

printf 'go build -o corvint-analyzer-python ./cmd/corvint-analyzer-python\n' > "$fixture"
if grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: the corvint-analyzer-python Go tool name was not allowed" >&2
  exit 1
fi

printf 'no-python-runtime-dependency-test:\n\t@script/no-python-runtime-dependency_test.sh\n' > "$fixture"
if grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: this check's own Makefile gate step was not allowed" >&2
  exit 1
fi

printf '\t@script/no-python-runtime-dependency_test.sh && python3 -c 1\n' > "$fixture"
if ! grep -nE "$pattern" "$fixture" | unexempted >/dev/null; then
  echo "self-test: an allowed name on the line hid a genuine python3 invocation" >&2
  exit 1
fi

# The external-conformance exemption is the exact path only: its own line is allowed, while the
# same invocation in another script, a longer path or a Makefile recipe naming it still fails.
fires() { printf '%s\n' "$1" | scan >/dev/null; }
if fires "$external_conformance:32:python=\${SPEC_TOOLCHAIN_PYTHON:-python3}"; then
  echo "self-test: the external-conformance script was not exempt by its exact path" >&2
  exit 1
fi
for line in "script/build.sh:32:python=\${SPEC_TOOLCHAIN_PYTHON:-python3}" \
  "$external_conformance.bak:1:python3 -m venv v" "x$external_conformance:1:pip install x" \
  "Makefile:282:	@$external_conformance && python3 -c 1"; do
  if ! fires "$line"; then
    echo "self-test: the external-conformance exemption hid a genuine invocation: $line" >&2
    exit 1
  fi
done

# Reaching the exempt script from a gate step, a prerequisite or another recipe is detected; its
# own non-gate target is not.
printf 'GATE_STEPS = go-test\nspec-toolchain-conformance:\n\t@%s\n' "$external_conformance" > "$fixture"
if [ -n "$(reaches_external < "$fixture")" ]; then
  echo "self-test: the external-conformance target itself was reported as reaching the exemption" >&2
  exit 1
fi
for makefile in 'GATE_STEPS = go-test spec-toolchain-conformance' 'go-test: build spec-toolchain-conformance' \
  "$(printf 'go-test:\n\t@%s' "$external_conformance")" \
  "$(printf 'GATE_STEPS = \\\n  go-test \\\n  spec-toolchain-conformance \\\n  # end')" \
  "$(printf 'shell-test: \\\n  build \\\n  spec-toolchain-conformance \\\n  # end')"; do
  printf '%s\n' "$makefile" > "$fixture"
  if [ -z "$(reaches_external < "$fixture")" ]; then
    echo "self-test: a gate path reaching the external-conformance script was not detected: $makefile" >&2
    exit 1
  fi
done

# V1-3971: an interpreter name only inside a quoted printf or echo argument is fixture data; a real
# call (bare, after a pipe, after && or ;, inside $(...), unquoted after echo) is still reported.
# shellcheck disable=SC2016
for line in "script/x_test.sh:3:printf 'python3 -c 1\\n' > \"\$f\"" \
  "script/x_test.sh:4:  printf '%s\\n' 'pip install x' \"pytest -q\" >> \"\$f\"" \
  "script/x_test.sh:5:if true; then echo \"python3 build.py\"; fi" \
  "script/x_test.sh:6:out=\$(printf 'python3 -c 1' | wc -c)"; do
  if fires "$line"; then
    echo "self-test: quoted printf/echo fixture data was flagged: $line" >&2
    exit 1
  fi
  # The mutant that drops the quoting check (no mask_fixture_data stage) flags it, so the
  # cases above are not vacuous.
  if ! printf '%s\n' "$line" | unexempted >/dev/null; then
    echo "self-test: the fixture-data case does not depend on the quoting check: $line" >&2
    exit 1
  fi
done
# shellcheck disable=SC2016
for line in "script/x_test.sh:7:python3 -c 1" "script/x_test.sh:8:printf 'x' | python3 -" \
  "script/x_test.sh:9:printf 'x' && python3 -c 1" "script/x_test.sh:10:printf 'x'; pip install y" \
  "script/x_test.sh:11:out=\$(python3 -c 1)" "script/x_test.sh:12:printf '%s' \"\$(python3 -c 1)\"" \
  "script/x_test.sh:13:echo python3 -c 1" "script/x_test.sh:14:printf 'x' | sh -c 'pytest'" \
  "script/x_test.sh:15:echo 'x' | python3"; do
  if ! fires "$line"; then
    echo "self-test: a genuine invocation was hidden as fixture data: $line" >&2
    exit 1
  fi
done

printf 'no-python-runtime-dependency: build/test/hook/packaging paths name no Python interpreter\n'
