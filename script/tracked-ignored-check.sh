#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/tracked-ignored-check.sh
# DTL-V0-012: fixture copies relying on ignore rules must retain every tracked file.
set -uo pipefail
paths=$(git ls-files -ci --exclude-standard) || { echo 'tracked-ignored-check: FAIL git ls-files; fix: run inside the repository and repair the Git read error' >&2; exit 2; }
if [[ -n $paths ]]; then
  printf 'tracked-ignored-check: FAIL tracked ignored paths:\n%s\nfix: narrow the ignore rules so tracked paths are included, then rerun make tracked-ignored-check\n' "$paths" >&2
  exit 1
fi
echo 'tracked-ignored-check: PASS'
