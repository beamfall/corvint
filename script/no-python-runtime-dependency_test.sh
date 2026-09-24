#!/bin/sh
# GOC-V0-008 falsifying assertion: Corvint's own build, tests, hooks, packaging and default
# execution paths (Makefile, script/*.sh, integrations/**/*.mjs and integrations/**/*.json)
# must carry no literal Python-interpreter invocation. Three things are never flagged: the
# CORVINT_TEST_EXTERNAL_PYTEST opt-in, which lets an explicitly requested command run against a
# user's own Python project, the `corvint-analyzer-python` Go tool family (its name embeds
# "python" but it is Go source that analyzes Python as fixture data, not a Python interpreter
# dependency of Corvint's own build/tests/hooks/packaging), and this check's own name, which the
# Makefile's gate step must spell.
set -eu

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
pattern='\<python3?\>|\<pip\>|\<pytest\>'
opt_in='CORVINT_TEST_EXTERNAL_PYTEST'
names='analyzer-python|no-python-runtime-dependency'

# unexempted keeps each matched line that still names an interpreter after the opt-in lines are
# dropped and the two allowed names are masked, so an allowed name elsewhere on a line or in its
# path never hides a real invocation. Paths are repository-relative so the checkout path cannot
# match or exempt anything.
unexempted() {
  grep -v "$opt_in" | sed -E "s/$names/[allowed-name]/g" | grep -E "$pattern"
}

cd "$source_root"
targets=$(
  printf '%s\n' Makefile
  find script -maxdepth 1 -type f -name '*.sh' ! -name "$(basename "$0")"
  find integrations -type f \( -name '*.mjs' -o -name '*.json' \) 2>/dev/null
)

violations=$(printf '%s\n' "$targets" | xargs grep -nE "$pattern" 2>/dev/null | unexempted || true)
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

printf 'no-python-runtime-dependency: build/test/hook/packaging paths name no Python interpreter\n'
