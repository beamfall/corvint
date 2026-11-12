#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# OSC-V1-012 (V1-5399): run `openspec validate --all` from docs/specs with the OpenSpec command
# (@fission-ai/openspec) at the version the Makefile pins, with telemetry off. A change folder
# that fails validation fails the run. `--strict` warnings are printed with their count and fail
# nothing. When the command is absent or at another version, the run prints NOT_RUN with the
# reason and exits 0, except in CI (CI=true), where it fails: it never reports a pass it did not
# run. OPENSPEC names another command path and OPENSPEC_VALIDATE_ROOT another repository root
# (tests use both). The Go check tools/openspec-check needs none of this; no product binary calls
# the command.
# usage: script/openspec-validate.sh PINNED_VERSION   (make openspec-validate passes the pin)
set -eu
test $# = 1 && test -n "$1" || { printf 'usage: %s PINNED_VERSION\n' "$0" >&2; exit 2; }
pin=$1
root=${OPENSPEC_VALIDATE_ROOT:-$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)}
command=${OPENSPEC:-openspec}
install="npm install -g @fission-ai/openspec@$pin (Node 20.19 or later)"
unavailable() {
  if test "${CI:-}" = true; then
    printf 'openspec-validate: FAIL %s; in CI the pinned command is required: %s\n' "$1" "$install" >&2
    exit 1
  fi
  printf 'openspec-validate: NOT_RUN %s; to run it: %s\n' "$1" "$install"
  exit 0
}
command -v "$command" > /dev/null 2>&1 || unavailable "the OpenSpec command ($command) is not installed"
version=$(OPENSPEC_TELEMETRY=0 "$command" --version 2> /dev/null | tail -n 1 | tr -d '[:space:]') ||
  unavailable "$command --version failed"
test "$version" = "$pin" || unavailable "$command is version ${version:-unknown}, not the pinned $pin"
cd "$root/docs/specs"
work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-openspec-validate.XXXXXX")
trap 'rm -rf "$work"' EXIT INT TERM
status=0
OPENSPEC_TELEMETRY=0 "$command" validate --all > "$work/out" 2>&1 || status=$?
cat "$work/out"
if test "$status" != 0; then
  printf 'openspec-validate: FAIL openspec %s validate --all exited %s in docs/specs; fix each folder it names\n' "$pin" "$status" >&2
  exit 1
fi
OPENSPEC_TELEMETRY=0 "$command" validate --all --strict > "$work/strict" 2>&1 || true
warnings=$(grep -ci 'warn' "$work/strict" || true)
sed 's/^/openspec-validate: strict: /' "$work/strict"
printf 'openspec-validate: PASS openspec %s validate --all in docs/specs; --strict printed %s warning line(s), which fail nothing\n' "$pin" "$warnings"
