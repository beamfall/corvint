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

# scan reads grep -Hn lines (path:line:text) and drops only those whose path field is exactly the
# external-conformance script, so a renamed copy, a longer path or a line elsewhere naming it is
# still scanned.
scan() {
  awk -v p="$external_conformance" 'index($0, p ":") != 1 || substr($0, length(p) + 2) !~ /^[0-9]+:/' | unexempted
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

printf 'no-python-runtime-dependency: build/test/hook/packaging paths name no Python interpreter\n'
