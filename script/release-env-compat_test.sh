#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd -P)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/corvint-release-env-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT INT TERM
mkdir "$scratch/bin"
for tool in git npm; do
  printf '#!/bin/sh\necho invoked >> "$CORVINT_ADMISSION_MARKER"\nexit 1\n' > "$scratch/bin/$tool"
  chmod +x "$scratch/bin/$tool"
done
export CORVINT_ADMISSION_MARKER="$scratch/marker"
# Extract only the shared admission function: no build or fixture effects.
for wrapper in corvint-companion-release-gate public-release-check; do
  eval "$(sed -n '/^resolve_release_setting() {/,/^}/p' "$root/script/$wrapper")"
  unset CORVINT_FIXTURE
  resolve_release_setting CORVINT_FIXTURE
  [ "${CORVINT_FIXTURE+x}" != x ]
  CORVINT_FIXTURE=current; resolve_release_setting CORVINT_FIXTURE; [ "$CORVINT_FIXTURE" = current ]
  CORVINT_FIXTURE=
  if resolve_release_setting CORVINT_FIXTURE 2>/dev/null; then exit 1; fi
  unset CORVINT_FIXTURE
  rm -f "$CORVINT_ADMISSION_MARKER"
  if CORVINT_NPM_CACHE= PATH="$scratch/bin:$PATH" bash "$root/script/$wrapper" >"$scratch/out" 2>"$scratch/error"; then exit 1; fi
  grep -Fxq 'CORVINT_NPM_CACHE must not be empty' "$scratch/error"
  [ ! -e "$CORVINT_ADMISSION_MARKER" ]
done

# A nonempty explicit source wins before consulting the ambient source setting.
for primary in '' /one; do
  rm -f "$CORVINT_ADMISSION_MARKER"
  if CORVINT_TASKMAN_REPO="$primary" CORVINT_NPM_CACHE=/cache PATH="$scratch/bin:$PATH" bash "$root/script/corvint-companion-release-gate" /explicit >"$scratch/out" 2>"$scratch/error"; then exit 1; fi
  [ -s "$CORVINT_ADMISSION_MARKER" ]
  grep -Fq '/explicit is not a git checkout' "$scratch/error"
  ! grep -Eq 'must not be empty' "$scratch/error"
done
printf 'release environment compatibility: PASS\n'
