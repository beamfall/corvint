#!/bin/sh
# GOC-V0-008 falsifying assertion: Corvint's own build, tests, hooks, packaging and default
# execution paths (Makefile, script/*.sh, integrations/**/*.mjs and integrations/**/*.json)
# must carry no literal Python-interpreter invocation. Two things are never flagged: the
# CORVINT_TEST_EXTERNAL_PYTEST opt-in, which lets an explicitly requested command run against a
# user's own Python project, and the `corvint-analyzer-python` Go tool family (its name embeds
# "python" but it is Go source that analyzes Python as fixture data, not a Python interpreter
# dependency of Corvint's own build/tests/hooks/packaging).
set -eu

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
pattern='\<python3?\>|\<pip\>|\<pytest\>'
allowed='CORVINT_TEST_EXTERNAL_PYTEST|analyzer-python'

self=$(CDPATH='' cd -- "$(dirname "$0")" && pwd -P)/$(basename "$0")
targets=$(
  printf '%s\n' "$source_root/Makefile"
  find "$source_root/script" -maxdepth 1 -type f -name '*.sh' ! -path "$self"
  find "$source_root/integrations" -type f \( -name '*.mjs' -o -name '*.json' \) 2>/dev/null
)

violations=$(printf '%s\n' "$targets" | xargs grep -nE "$pattern" 2>/dev/null | grep -vE "$allowed" || true)
if [ -n "$violations" ]; then
  printf 'GOC-V0-008: a build/test/hook/packaging path names a Python interpreter:\n%s\n' "$violations" >&2
  exit 1
fi

# Falsifying self-test: prove the scan genuinely fires on a real invocation and genuinely
# allows both carved-out exceptions, so this is not a vacuous always-pass grep.
fixture=$(mktemp "${TMPDIR:-/tmp}/corvint-no-python-dep.XXXXXX")
trap 'rm -f "$fixture"' EXIT

printf 'python3 build.py\n' > "$fixture"
if ! grep -nE "$pattern" "$fixture" | grep -vE "$allowed" >/dev/null; then
  echo "self-test: scan failed to detect a genuine python3 invocation" >&2
  exit 1
fi

# shellcheck disable=SC2016
printf 'if [ -n "$CORVINT_TEST_EXTERNAL_PYTEST" ]; then python3 -m pytest; fi\n' > "$fixture"
if grep -nE "$pattern" "$fixture" | grep -vE "$allowed" >/dev/null; then
  echo "self-test: the CORVINT_TEST_EXTERNAL_PYTEST opt-in path was not allowed" >&2
  exit 1
fi

printf 'go build -o corvint-analyzer-python ./cmd/corvint-analyzer-python\n' > "$fixture"
if grep -nE "$pattern" "$fixture" | grep -vE "$allowed" >/dev/null; then
  echo "self-test: the corvint-analyzer-python Go tool name was not allowed" >&2
  exit 1
fi

printf 'no-python-runtime-dependency: build/test/hook/packaging paths name no Python interpreter\n'
