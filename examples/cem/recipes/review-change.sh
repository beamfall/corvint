#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Recipe 2: prepare a change-evidence review for base..HEAD. Composes the public commands the
# Corvint dogfood loop runs (docs/DOGFOOD.md): `corvint cem prepare`, strict `cem status`,
# `cem report`, and the advisory `corvint review --base`.
#
# Required environment:
#   CORVINT_BASE   full commit ID the change starts from; HEAD is the change
# Optional environment:
#   CORVINT_ROOT        repository checkout (default $PWD)
#   CORVINT_BIN         Corvint executable (default corvint)
#   CEM_MAX_UNKNOWN     accepted unknown hunks (default 0)
#   CEM_MAX_MECHANICAL  accepted mechanical hunks (default 0)
#   RECIPE_OUT          directory outside the repository for the receipts (default: a new temp dir)
#   RECIPE_TIMEOUT      per-step wall-clock bound in seconds (default 600)
#
# `cem prepare` is the only writer: it creates or resumes .corvint/change.cem.json and never
# replaces an outdated or invalid map. Exit 0 complete (every hunk within the caps, the map
# committed at HEAD, report rendered), 2 operational, 3 incomplete: a step refused, hunks remain
# over the caps, or the map is not committed. Receipts stay in RECIPE_OUT; `cem report` writes
# the rendered review to its private default under the Git directory and the outcome names it.

set -u
# shellcheck source-path=SCRIPTDIR source=bounded.sh
. "$(dirname "$0")/bounded.sh"

base=${CORVINT_BASE:-}
root=${CORVINT_ROOT:-$PWD}
corvint=${CORVINT_BIN:-corvint}
max_unknown=${CEM_MAX_UNKNOWN:-0}
max_mechanical=${CEM_MAX_MECHANICAL:-0}
map=.corvint/change.cem.json
[ -n "$base" ] || operational 'CORVINT_BASE is required'
case $max_unknown in ''|*[!0123456789]*) operational 'CEM_MAX_UNKNOWN must be a whole number' ;; esac
case $max_mechanical in ''|*[!0123456789]*) operational 'CEM_MAX_MECHANICAL must be a whole number' ;; esac

finish() {
  printf 'outcome=%s %s out=%s\n' "$1" "$2" "$recipe_out"
  exit "$3"
}

step_or_stop() {
  local status=0
  run_step "$@" || status=$?
  if [ "$status" -eq 124 ]; then finish operational "timeout=$1" 2; fi
  if [ "$status" -ne 0 ]; then finish incomplete "refused=$1" 3; fi
}

caps=(--expected-base "$base" --target HEAD --max-unknown "$max_unknown" --max-mechanical "$max_mechanical")

step_or_stop prepare "$corvint" --root "$root" cem prepare --base "$base" --target HEAD
step_or_stop status "$corvint" --root "$root" cem status --map "$map" "${caps[@]}"

# CI reads the map from the committed head tree, so an uncommitted or edited map is not done.
committed=yes
git -C "$root" cat-file -e "HEAD:$map" 2>/dev/null || committed=no
if [ -n "$(git -C "$root" status --porcelain -- "$map")" ]; then committed=no; fi
if [ "$committed" != yes ]; then finish incomplete "map-uncommitted=$map" 3; fi

step_or_stop report "$corvint" --root "$root" cem report --map "$map" "${caps[@]}"
step_or_stop review "$corvint" --root "$root" review --base "$base"
report=$(grep -o '"report":"[^"]*"' "$recipe_out/report.out" | cut -d'"' -f4)
finish complete "report=$report" 0
