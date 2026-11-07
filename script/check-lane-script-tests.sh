#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# usage: script/check-lane-script-tests.sh BASE
# Repository dogfood check/seal preflight. Required runs ignore optional group selectors.
set -euo pipefail
base=${1:?usage: script/check-lane-script-tests.sh BASE}
changed=$(git -c core.quotepath=false diff --name-only --no-renames "$base" -- 'script/lane-*') || exit 2
untracked=$(git -c core.quotepath=false ls-files --others --exclude-standard -- 'script/lane-*') || exit 2
if [[ -n $changed$untracked ]]; then
  printf 'dogfood-check: lane-scripts-test required (script/lane-* differs from %s)\n' "$base"
  env -u LANE_SCRIPTS_GROUPS make --no-print-directory lane-scripts-test LANE_SCRIPTS_GROUPS=all || {
    printf 'dogfood-check: REFUSE lane-scripts-test failed; fix: repair the failing lane-script tests and rerun\n' >&2
    exit 1
  }
fi
# With no script/lane-* change it prints nothing: dogfood check's own output, which callers and
# tests match exactly, stays unchanged (ASD-V0-640).
